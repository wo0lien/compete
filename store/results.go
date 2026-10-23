package store

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/wo0lien/compete/games"
)

var ErrDuplicate = errors.New("already submitted")

// ErrNoResults is AddResults called with nothing to store.
var ErrNoResults = errors.New("no results")

// ErrFuturePuzzle is a result for a puzzle not out yet (above today+1).
var ErrFuturePuzzle = errors.New("puzzle not out yet")

// AddResults stores parsed results for a user and returns the ones that were new.
// Already-stored results are skipped, so re-sharing a Songless text after playing
// one more category only adds that category. ErrDuplicate if nothing was new.
func (s *Store) AddResults(userID int64, raw string, rs []games.Result) ([]games.Result, error) {
	if len(rs) == 0 {
		return nil, ErrNoResults
	}
	now := s.Now()
	for _, r := range rs {
		g, _ := games.ByID(r.Game)
		if today, ok := g.Today(r.Variant, now); ok && r.PuzzleID > today+1 {
			return nil, ErrFuturePuzzle
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var added []games.Result
	for _, r := range rs {
		var detail any // NULL when there is no detail
		if r.Detail != nil {
			b, err := json.Marshal(r.Detail)
			if err != nil {
				return nil, err
			}
			detail = string(b)
		}
		res, err := tx.Exec(`
			INSERT INTO results(user_id, game, variant, puzzle_id, score, tiebreak, detail, raw, submitted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
			userID, r.Game, r.Variant, r.PuzzleID, r.Score, r.Tiebreak, detail, raw, s.Now().Unix())
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			added = append(added, r)
		}
	}
	if len(added) == 0 {
		return nil, ErrDuplicate
	}
	return added, tx.Commit()
}

type Entry struct {
	Username string
	Played   bool
	Hidden   bool // played, but the viewer has not submitted this puzzle yet
	Score    *int
	Tiebreak *int
	Grid     string
	Pawn     string
	Me       bool // the viewer's own row
	Rank     int  // 1-based, shared on ties like the leaderboard's RANK(); 0 if not played
}

type Board struct {
	Game, Variant string
	PuzzleID      int
	Revealed      bool // the viewer has submitted this puzzle
	Entries       []Entry
}

// Board ranks every group member on one puzzle. Until the viewer has submitted
// that same (game, variant, puzzle), other members' results are hidden and
// listed alphabetically, because rank order alone would leak scores.
func (s *Store) Board(groupID, viewerID int64, game, variant string, puzzleID int) (Board, error) {
	b := Board{Game: game, Variant: variant, PuzzleID: puzzleID}
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.pawn, r.id IS NOT NULL, r.score, r.tiebreak,
		       COALESCE(json_extract(r.detail, '$.grid'), '')
		FROM memberships m
		JOIN users u ON u.id = m.user_id
		LEFT JOIN results r ON r.user_id = m.user_id
		     AND r.game = ? AND r.variant = ? AND r.puzzle_id = ?
		WHERE m.group_id = ?
		ORDER BY r.id IS NULL, r.score IS NULL, r.score, r.tiebreak IS NULL, r.tiebreak,
		         u.username COLLATE NOCASE`,
		game, variant, puzzleID, groupID)
	if err != nil {
		return Board{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var e Entry
		var uid int64
		if err := rows.Scan(&uid, &e.Username, &e.Pawn, &e.Played, &e.Score, &e.Tiebreak, &e.Grid); err != nil {
			return Board{}, err
		}
		e.Me = uid == viewerID
		if e.Me && e.Played {
			b.Revealed = true
		}
		if e.Played { // rows come in rank order
			e.Rank = len(b.Entries) + 1
			if n := len(b.Entries); n > 0 && b.Entries[n-1].Played &&
				eqInt(b.Entries[n-1].Score, e.Score) && eqInt(b.Entries[n-1].Tiebreak, e.Tiebreak) {
				e.Rank = b.Entries[n-1].Rank
			}
		}
		b.Entries = append(b.Entries, e)
	}
	if err := rows.Err(); err != nil {
		return Board{}, err
	}
	if !b.Revealed {
		for i := range b.Entries {
			if e := &b.Entries[i]; e.Played {
				e.Hidden, e.Score, e.Tiebreak, e.Grid, e.Rank = true, nil, nil, "", 0
			}
		}
		sort.SliceStable(b.Entries, func(i, j int) bool {
			a, c := b.Entries[i], b.Entries[j]
			if a.Me != c.Me { // the viewer first: their tile is the way to play
				return a.Me
			}
			if a.Played != c.Played {
				return a.Played
			}
			return strings.ToLower(a.Username) < strings.ToLower(c.Username)
		})
	}
	return b, nil
}

// eqInt compares nullable ints; two NULLs are equal, as in SQL's RANK().
func eqInt(a, b *int) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// CurrentBoards returns, for each game/variant of the group's games played in it, the board of
// today's puzzle, from the game's schedule: it shows before anyone has played it.
// Without a schedule, "current" is the newest puzzle_id among members' results.
// A newer id than today's (today+1, a player already past midnight) wins; one
// further ahead is ignored.
func (s *Store) CurrentBoards(groupID, viewerID int64) ([]Board, error) {
	rows, err := s.db.Query(`
		SELECT r.game, r.variant, MAX(r.puzzle_id) FROM results r
		JOIN memberships m ON m.user_id = r.user_id AND m.group_id = ?
		JOIN group_games gg ON gg.group_id = m.group_id AND gg.game = r.game
		GROUP BY r.game, r.variant ORDER BY r.game, r.variant`, groupID)
	if err != nil {
		return nil, err
	}
	var keys []Board
	for rows.Next() {
		var k Board
		if err := rows.Scan(&k.Game, &k.Variant, &k.PuzzleID); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// ponytail: one query per board; batch if groups ever play dozens of games.
	boards := make([]Board, 0, len(keys))
	now := s.Now()
	for _, k := range keys {
		g, _ := games.ByID(k.Game)
		// Ids beyond today+1 can only be rows stored before AddResults refused them (#4).
		if today, ok := g.Today(k.Variant, now); ok && (today > k.PuzzleID || k.PuzzleID > today+1) {
			k.PuzzleID = today
		}
		b, err := s.Board(groupID, viewerID, k.Game, k.Variant, k.PuzzleID)
		if err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, nil
}

// Past is one puzzle on a group's history page.
type Past struct {
	ID       int
	Revealed bool    // the viewer played it, so Winners show (Board's rule)
	Winners  []Entry // the best result's members, ties shared; none if hidden or everyone failed
}

// History lists the puzzles of a game/variant played in the group, newest
// first, with their winners.
func (s *Store) History(groupID, viewerID int64, game, variant string) ([]Past, error) {
	rows, err := s.db.Query(`
		SELECT r.puzzle_id, u.id, u.username, u.pawn, r.score, r.tiebreak FROM results r
		JOIN memberships m ON m.user_id = r.user_id AND m.group_id = ?
		JOIN users u ON u.id = r.user_id
		WHERE r.game = ? AND r.variant = ?
		ORDER BY r.puzzle_id DESC, r.score IS NULL, r.score, r.tiebreak IS NULL, r.tiebreak,
		         u.username COLLATE NOCASE`, groupID, game, variant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ps []Past
	var best Entry // the current puzzle's first row: rows come in rank order
	for rows.Next() {
		var id int
		var uid int64
		var e Entry
		if err := rows.Scan(&id, &uid, &e.Username, &e.Pawn, &e.Score, &e.Tiebreak); err != nil {
			return nil, err
		}
		if len(ps) == 0 || ps[len(ps)-1].ID != id {
			ps = append(ps, Past{ID: id})
			best = e
		}
		p := &ps[len(ps)-1]
		if uid == viewerID {
			p.Revealed = true
		}
		if e.Score != nil && eqInt(e.Score, best.Score) && eqInt(e.Tiebreak, best.Tiebreak) {
			p.Winners = append(p.Winners, e)
		}
	}
	for i := range ps {
		if !ps[i].Revealed {
			ps[i].Winners = nil
		}
	}
	return ps, rows.Err()
}

// recent is how far back a member's games count as theirs (ToPlay, PlayedRecently).
const recent = 14 * 24 * time.Hour

// PlayedRecently: the user submitted any result in the recent window.
func (s *Store) PlayedRecently(userID int64) (bool, error) {
	var ok bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM results WHERE user_id = ? AND submitted_at > ?)`,
		userID, s.Now().Add(-recent).Unix()).Scan(&ok)
	return ok, err
}

// ToPlay lists the games (ids, in games.All order) the user still has to play
// today: their own games of the last 14 days, per variant, and every game of
// their groups, where any variant played today counts. Games without a known
// schedule are left out.
func (s *Store) ToPlay(userID int64) ([]string, error) {
	now := s.Now()
	type key struct{ game, variant string }
	scan := func(q string, args ...any) ([]key, error) {
		rows, err := s.db.Query(q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var ks []key
		for rows.Next() {
			var k key
			if err := rows.Scan(&k.game, &k.variant); err != nil {
				return nil, err
			}
			ks = append(ks, k)
		}
		return ks, rows.Err()
	}
	own, err := scan(`SELECT DISTINCT game, variant FROM results WHERE user_id = ? AND submitted_at > ?`,
		userID, now.Add(-recent).Unix())
	if err != nil {
		return nil, err
	}
	group, err := scan(`SELECT DISTINCT gg.game, '' FROM group_games gg JOIN memberships m USING (group_id) WHERE m.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	// playedToday: a result for game+variant at today's puzzle or later (a today+1 share counts too)
	playedToday := func(k key) (bool, error) {
		g, _ := games.ByID(k.game)
		today, ok := g.Today(k.variant, now)
		if !ok {
			return false, nil
		}
		var played bool
		err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM results WHERE user_id = ? AND game = ? AND variant = ? AND puzzle_id >= ?)`,
			userID, k.game, k.variant, today).Scan(&played)
		return played, err
	}
	todo := map[string]bool{}
	for _, k := range own {
		g, _ := games.ByID(k.game)
		if _, ok := g.Today(k.variant, now); !ok || todo[k.game] {
			continue
		}
		played, err := playedToday(k)
		if err != nil {
			return nil, err
		}
		todo[k.game] = !played
	}
	for _, k := range group {
		g, ok := games.ByID(k.game)
		if !ok || todo[k.game] {
			continue
		}
		if _, ok := g.Today("", now); !ok {
			continue
		}
		vs, err := scan(`SELECT DISTINCT game, variant FROM results WHERE user_id = ? AND game = ?`, userID, k.game)
		if err != nil {
			return nil, err
		}
		played := false
		for _, v := range vs {
			if played, err = playedToday(v); err != nil {
				return nil, err
			} else if played {
				break
			}
		}
		todo[k.game] = !played
	}
	var out []string
	for _, g := range games.All {
		if todo[g.ID] {
			out = append(out, g.ID)
		}
	}
	return out, nil
}
