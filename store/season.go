package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/wo0lien/compete/games"
)

// ErrSeasonDays: a season lasts short (3), normal (7) or long (14) days.
var ErrSeasonDays = errors.New("season length must be 3, 7 or 14 days")

var seasonLengths = []int{3, 7, 14}

// seasonN is the finish line: what a strong player (beating 3/4 of the group
// on every puzzle) earns in days, at least 1. Integer form of
// ceil(days × games × (1 + 3/4 × (members − 1))).
func seasonN(days, games, members int) int {
	return max(1, (days*games*(3*members+1)+3)/4)
}

// SeasonDays is the length the group's next season is sized for.
func (s *Store) SeasonDays(groupID int64) (int, error) {
	var d int
	err := s.db.QueryRow("SELECT season_days FROM groups WHERE id = ?", groupID).Scan(&d)
	return d, err
}

// SetSeasonDays changes the length of the next seasons; the running one keeps its N.
func (s *Store) SetSeasonDays(groupID int64, days int) error {
	if !slices.Contains(seasonLengths, days) {
		return ErrSeasonDays
	}
	_, err := s.db.Exec("UPDATE groups SET season_days = ? WHERE id = ?", days, groupID)
	return err
}

// scored is one counted puzzle: its day and each member's points on it.
type scored struct {
	day string
	pts map[int64]int
}

// points scores one puzzle: 1 for playing plus 1 per member strictly beaten.
// Members who did not play are beaten by everyone who did. ranks maps each
// member who played to their RANK() (1 = best, ties share).
func points(ranks map[int64]int, members int) map[int64]int {
	out := make(map[int64]int, len(ranks))
	for u, rk := range ranks {
		p := 1 + members - len(ranks)
		for _, other := range ranks {
			if other > rk {
				p++
			}
		}
		out[u] = p
	}
	return out
}

// walk adds counted puzzles up in day order. The first day at whose end
// someone's total reaches n is the final day; later days belong to the next
// season and are left out.
func walk(ps []scored, n int) (totals map[int64]int, final string) {
	ps = slices.Clone(ps)
	slices.SortStableFunc(ps, func(a, b scored) int { return strings.Compare(a.day, b.day) })
	totals = map[int64]int{}
	for i, p := range ps {
		for u, v := range p.pts {
			totals[u] += v
		}
		if i+1 < len(ps) && ps[i+1].day == p.day {
			continue // the day isn't over yet
		}
		for _, v := range totals {
			if v >= n {
				return totals, p.day
			}
		}
	}
	return totals, ""
}

// winners are the members with the top total, by id; nobody on an empty race.
func winners(totals map[int64]int) []int64 {
	top := 0
	for _, v := range totals {
		top = max(top, v)
	}
	var out []int64
	for u, v := range totals {
		if v == top && top > 0 {
			out = append(out, u)
		}
	}
	slices.Sort(out)
	return out
}

// Season is a group's running race: first to N points (see the spec, #54).
type Season struct {
	Number   int
	N        int
	Start    string   // YYYY-MM-DD
	FinalDay string   // "" until someone reaches N
	Racers   []Racer  // every member, points desc then username
	Past     []Winner // last 3 decided seasons, newest first
}

// Winner is a decided season and who won it (co-winners on a tie).
type Winner struct {
	Number    int
	Usernames []string
}

// CurrentSeason returns the group's running season with each member's points.
// It starts season 1 on first use and closes a decided season (its final day
// is over for every game), so reading is enough to move seasons on: they only
// depend on whole days, never on when this runs.
func (s *Store) CurrentSeason(groupID int64) (Season, error) {
	for {
		now := s.Now()
		members, err := s.Members(groupID)
		if err != nil {
			return Season{}, err
		}
		ids, err := s.GroupGames(groupID)
		if err != nil {
			return Season{}, err
		}
		id, se, err := s.runningSeason(groupID, now, len(ids), len(members))
		if err != nil {
			return Season{}, err
		}
		ps, err := s.counted(groupID, se.Start, now)
		if err != nil {
			return Season{}, err
		}
		totals, final := walk(ps, se.N)
		if final != "" && dayOver(ids, final, now) {
			if err := s.closeSeason(groupID, id, se.Number, final, winners(totals)); err != nil {
				return Season{}, err
			}
			continue // the next season may already hold points
		}
		se.FinalDay = final
		for _, m := range members {
			se.Racers = append(se.Racers, Racer{Username: m.Username, Pawn: m.Pawn, Points: totals[m.UserID]})
		}
		slices.SortStableFunc(se.Racers, func(a, b Racer) int {
			if a.Points != b.Points {
				return b.Points - a.Points
			}
			return strings.Compare(strings.ToLower(a.Username), strings.ToLower(b.Username))
		})
		se.Past, err = s.pastWinners(groupID)
		return se, err
	}
}

// runningSeason loads the open season, starting the first one today if none.
func (s *Store) runningSeason(groupID int64, now time.Time, nGames, members int) (int64, Season, error) {
	var id int64
	var se Season
	q := "SELECT id, number, start_day, n FROM seasons WHERE group_id = ? AND end_day IS NULL"
	err := s.db.QueryRow(q, groupID).Scan(&id, &se.Number, &se.Start, &se.N)
	if !errors.Is(err, sql.ErrNoRows) {
		return id, se, err
	}
	days, err := s.SeasonDays(groupID)
	if err != nil {
		return 0, se, err
	}
	if err := s.startSeason(groupID, now.UTC().Format(time.DateOnly), seasonN(days, nGames, members)); err != nil {
		return 0, se, err
	}
	err = s.db.QueryRow(q, groupID).Scan(&id, &se.Number, &se.Start, &se.N)
	return id, se, err
}

// startSeason opens the group's next season. OR IGNORE: a concurrent first
// read already opened one.
func (s *Store) startSeason(groupID int64, start string, n int) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO seasons (group_id, number, start_day, n)
		SELECT ?, COALESCE(MAX(number), 0) + 1, ?, ? FROM seasons WHERE group_id = ?`, groupID, start, n, groupID)
	return err
}

// counted returns the puzzles that count in a season starting on start: one
// per game a day (its MainVariant), of the group's games, on or after start.
// A result scores only if pasted before its puzzle closed; a member counts on
// puzzles from the day they joined. A puzzle counts once closed, or earlier
// when every member it counts has played.
// ponytail: reads every result of the group's games, all time; filter by
// submitted_at once groups have years of history.
func (s *Store) counted(groupID int64, start string, now time.Time) ([]scored, error) {
	joined := map[int64]string{} // member → day they joined
	mrows, err := s.db.Query("SELECT user_id, joined_at FROM memberships WHERE group_id = ?", groupID)
	if err != nil {
		return nil, err
	}
	for mrows.Next() {
		var uid, at int64
		if err := mrows.Scan(&uid, &at); err != nil {
			mrows.Close()
			return nil, err
		}
		joined[uid] = time.Unix(at, 0).UTC().Format(time.DateOnly)
	}
	mrows.Close()
	if err := mrows.Err(); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`
		SELECT r.game, r.variant, r.puzzle_id, r.user_id, r.score, r.tiebreak, r.submitted_at
		FROM results r
		JOIN memberships m ON m.user_id = r.user_id AND m.group_id = ?1
		JOIN group_games gg ON gg.game = r.game AND gg.group_id = ?1`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type puzzle struct {
		game string
		id   int
	}
	type entry struct {
		user            int64
		score, tiebreak *int
	}
	plays := map[puzzle][]entry{}
	var order []puzzle
	for rows.Next() {
		var p puzzle
		var variant string
		var e entry
		var at int64
		if err := rows.Scan(&p.game, &variant, &p.id, &e.user, &e.score, &e.tiebreak, &at); err != nil {
			return nil, err
		}
		g, _ := games.ByID(p.game)
		if variant != g.MainVariant {
			continue
		}
		if today, ok := g.Today(variant, time.Unix(at, 0)); !ok || today > p.id {
			continue // pasted after the puzzle closed, once its board was out
		}
		if plays[p] == nil {
			order = append(order, p)
		}
		plays[p] = append(plays[p], e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []scored
	for _, p := range order {
		g, _ := games.ByID(p.game)
		d, ok := g.Day(g.MainVariant, p.id)
		day := d.Format(time.DateOnly)
		if !ok || day < start {
			continue
		}
		ranks := map[int64]int{}
		for _, e := range plays[p] {
			rk := 1
			for _, o := range plays[p] {
				if better(o.score, o.tiebreak, e.score, e.tiebreak) {
					rk++
				}
			}
			ranks[e.user] = rk
		}
		members := len(ranks) // who played, plus who was in the group that day
		for u, j := range joined {
			if _, played := ranks[u]; !played && j <= day {
				members++
			}
		}
		if today, _ := g.Today(g.MainVariant, now); p.id >= today && len(ranks) < members {
			continue // still open and someone has yet to play: no spoilers
		}
		out = append(out, scored{day, points(ranks, members)})
	}
	return out, nil
}

// better: result a ranks above b, in the boards' order (fails last, then
// score, then tiebreak, missing tiebreak last; lower is better).
func better(as, at, bs, bt *int) bool {
	if (as == nil) != (bs == nil) {
		return bs == nil
	}
	if as != nil && *as != *bs {
		return *as < *bs
	}
	if (at == nil) != (bt == nil) {
		return bt == nil
	}
	return at != nil && *at < *bt
}

// dayOver: every scheduled game of the group has moved past day.
func dayOver(gameIDs []string, day string, now time.Time) bool {
	for _, id := range gameIDs {
		g, _ := games.ByID(id)
		today, ok := g.Today("", now)
		if !ok {
			continue
		}
		if d, _ := g.Day("", today); d.Format(time.DateOnly) <= day {
			return false
		}
	}
	return true
}

// closeSeason freezes season id on final with its winners and opens the next
// one the day after. A second closer finds end_day already set and does nothing.
func (s *Store) closeSeason(groupID, id int64, number int, final string, won []int64) error {
	members, err := s.Members(groupID)
	if err != nil {
		return err
	}
	ids, err := s.GroupGames(groupID)
	if err != nil {
		return err
	}
	days, err := s.SeasonDays(groupID)
	if err != nil {
		return err
	}
	f, err := time.Parse(time.DateOnly, final)
	if err != nil {
		return err
	}
	w, _ := json.Marshal(won)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec("UPDATE seasons SET end_day = ?, winners = ? WHERE id = ? AND end_day IS NULL", final, string(w), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil // already closed by a concurrent read
	}
	if _, err := tx.Exec("INSERT INTO seasons (group_id, number, start_day, n) VALUES (?, ?, ?, ?)",
		groupID, number+1, f.AddDate(0, 0, 1).Format(time.DateOnly), seasonN(days, len(ids), len(members))); err != nil {
		return err
	}
	return tx.Commit()
}

// pastWinners lists the last 3 decided seasons, newest first.
func (s *Store) pastWinners(groupID int64) ([]Winner, error) {
	rows, err := s.db.Query(`
		SELECT se.number, u.username FROM seasons se, json_each(se.winners) w JOIN users u ON u.id = w.value
		WHERE se.group_id = ? AND se.end_day IS NOT NULL
		  AND se.number > (SELECT MAX(number) FROM seasons WHERE group_id = ?) - 4
		ORDER BY se.number DESC, u.username COLLATE NOCASE`, groupID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Winner
	for rows.Next() {
		var n int
		var name string
		if err := rows.Scan(&n, &name); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Number != n {
			out = append(out, Winner{Number: n})
		}
		out[len(out)-1].Usernames = append(out[len(out)-1].Usernames, name)
	}
	return out, rows.Err()
}
