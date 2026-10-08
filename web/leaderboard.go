package web

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

type trackCell struct {
	Label, Class string
	Pawns        []string // pawn colors standing on this cell
}

// track lays racers on 9 cells, GO to ★. Positions are scaled to the leader so
// the track never grows; the legend shows the real win counts.
func track(racers []store.Racer) []trackCell {
	cells := make([]trackCell, 9)
	for i := range cells {
		cells[i].Label = strconv.Itoa(i)
	}
	cells[0].Label, cells[0].Class = "GO", "start"
	cells[8].Label, cells[8].Class = "★", "goal"
	most := 0
	for _, r := range racers {
		most = max(most, r.Wins)
	}
	for _, r := range racers {
		pos := 0
		if most > 0 {
			pos = r.Wins * 8 / most
		}
		cells[pos].Pawns = append(cells[pos].Pawns, r.Pawn)
	}
	return cells
}

// raceData builds the "race" partial's input for a game filter (nil = all games).
func (s *Server) raceData(groupID int64, gameIDs []string) (map[string]any, error) {
	racers, err := s.store.Race(groupID, gameIDs)
	if err != nil {
		return nil, err
	}
	return map[string]any{"Track": track(racers), "Racers": racers, "GroupID": groupID}, nil
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
	race, err := s.raceData(g.ID, ids)
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
	ids, err := s.store.Puzzles(g.ID, q.Get("game"), q.Get("variant"))
	if err != nil {
		s.oops(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "history.html", map[string]any{
		"Group": g, "Game": q.Get("game"), "Variant": q.Get("variant"), "Puzzles": ids,
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
