package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

func (s *Server) groupPage(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.groupFor(w, r, u)
	if !ok {
		return
	}
	boards, err := s.store.CurrentBoards(g.ID, u.ID)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	cs := cards(boards)
	for i := range cs {
		cs[i].Group = g.ID
	}
	// Cards still to play first: after a submit, the next game is on top.
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].ToPlay() && !cs[j].ToPlay() })
	race, err := s.raceData(g.ID, nil)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "group.html", map[string]any{"Group": g, "Cards": cs, "Race": race})
}

// submit parses a pasted share text and stores its results for the current user.
// Unrecognized text is never stored.
func (s *Server) submit(w http.ResponseWriter, r *http.Request, u store.User) {
	text, back := r.FormValue("text"), safeNext(r.FormValue("back"))
	data := map[string]any{"Text": text, "Back": back}
	if r.Header.Get("HX-Request") == "true" { // the page's own paste form: keep the page, show the warning under it
		data["Fragment"] = "paste-error"
		w.Header().Set("HX-Retarget", "#paste-error")
		w.Header().Set("HX-Reswap", "innerHTML show:none") // boosted swaps default to show:top, which would scroll the warning to the top
		w.Header().Set("HX-Push-Url", "false") // the address stays the page's, not /results
	}
	rs := games.Parse(text)
	if rs == nil {
		data["Error"], data["Unsupported"] = tr(langFrom(r), "err.unsupported"), true
		s.render(w, r, http.StatusUnprocessableEntity, "submit.html", data)
		return
	}
	added, err := s.store.AddResults(u.ID, games.Normalize(text), rs)
	g, _ := games.ByID(rs[0].Game)
	board := strings.TrimSpace(fmt.Sprintf("%s #%d %s", g.Name, rs[0].PuzzleID, rs[0].Variant))
	if errors.Is(err, store.ErrFuturePuzzle) {
		data["Error"] = tr(langFrom(r), "err.future_puzzle", "Board", board)
		s.render(w, r, http.StatusUnprocessableEntity, "submit.html", data)
		return
	}
	if errors.Is(err, store.ErrDuplicate) {
		data["Error"] = tr(langFrom(r), "err.already_submitted", "Board", board)
		s.render(w, r, http.StatusConflict, "submit.html", data)
		return
	}
	if err != nil {
		s.oops(w, r, err)
		return
	}
	if s.Push != nil {
		go s.Push.Submitted(u.ID, added)
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// shareText joins what a share target received. Apps spread the payload over
// text, title and url; text goes first so the anchored parsers still match.
func shareText(q url.Values) string {
	var parts []string
	for _, k := range []string{"text", "title", "url"} {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, "\n")
}

// share is the PWA share target: it only pre-fills the submit form. back is
// set when coming back from a paste sent with an expired session (loginNext).
func (s *Server) share(w http.ResponseWriter, r *http.Request, _ store.User) {
	q := r.URL.Query()
	s.render(w, r, http.StatusOK, "submit.html", map[string]any{"Text": shareText(q), "Back": safeNext(q.Get("back"))})
}
