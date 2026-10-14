package web

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

type trackCell struct {
	Label, Class string
	Pawns        []string // pawn colors standing on this cell
}

// track lays the season's racers on 9 cells, GO to ★. Positions are absolute:
// points × 8 / N, and only those who reached N stand on ★.
func track(se store.Season) []trackCell {
	cells := make([]trackCell, 9)
	for i := range cells {
		cells[i].Label = strconv.Itoa(i)
	}
	cells[0].Label, cells[0].Class = "GO", "start"
	cells[8].Label, cells[8].Class = "★", "goal"
	for _, r := range se.Racers {
		pos := 8
		if r.Points < se.N {
			pos = min(r.Points*8/se.N, 7)
		}
		cells[pos].Pawns = append(cells[pos].Pawns, r.Pawn)
	}
	return cells
}

// raceData builds the "race" partial's input: the group's running season.
func (s *Server) raceData(groupID int64) (map[string]any, error) {
	se, err := s.store.CurrentSeason(groupID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"Track": track(se), "Season": se, "GroupID": groupID}, nil
}

func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.groupFor(w, r, u)
	if !ok {
		return
	}
	tag := r.URL.Query().Get("tag")
	var ids []string // nil = every game
	if tag != "" {
		ids = games.WithTag(tag)
		if ids == nil {
			ids = []string{} // unknown tag: nothing matches
		}
	}
	race, err := s.raceData(g.ID)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	stats, err := s.store.Leaderboard(g.ID)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	if ids != nil {
		stats = slices.DeleteFunc(stats, func(gs store.GameStats) bool { return !slices.Contains(ids, gs.Game) })
	}
	s.render(w, r, http.StatusOK, "leaderboard.html", map[string]any{
		"Group": g, "Race": race, "Stats": stats, "Tags": games.AllTags(), "Tag": tag,
	})
}

func (s *Server) history(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.groupFor(w, r, u)
	if !ok {
		return
	}
	q := r.URL.Query()
	game, variant := q.Get("game"), q.Get("variant")
	ps, err := s.store.History(g.ID, u.ID, game, variant)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	type row struct {
		store.Past
		Day string // "" without a known schedule
	}
	gm, _ := games.ByID(game)
	today, _ := gm.Today(variant, s.store.Now())
	lang := langFrom(r)
	rows := make([]row, len(ps))
	for i, p := range ps {
		rows[i].Past = p
		switch day, ok := gm.Day(variant, p.ID); {
		case !ok:
		case p.ID == today:
			rows[i].Day = tr(lang, "history.today")
		case p.ID == today-1:
			rows[i].Day = tr(lang, "history.yesterday")
		default:
			rows[i].Day = fmt.Sprintf("%d %s", day.Day(), strings.Fields(tr(lang, "history.months"))[day.Month()-1])
		}
	}
	s.render(w, r, http.StatusOK, "history.html", map[string]any{
		"Group": g, "Game": game, "Variant": variant, "Puzzles": rows,
	})
}

func (s *Server) onePuzzle(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.groupFor(w, r, u)
	if !ok {
		return
	}
	q := r.URL.Query()
	puzzle, err := strconv.Atoi(q.Get("puzzle"))
	if err != nil {
		s.message(w, r, http.StatusNotFound, "msg.not_found", "msg.no_puzzle")
		return
	}
	b, err := s.store.Board(g.ID, u.ID, q.Get("game"), q.Get("variant"), puzzle)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "board.html", map[string]any{"Group": g, "Board": b})
}
