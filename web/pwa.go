package web

import (
	"net/http"

	"github.com/wo0lien/compete/games"
)

// serveAsset serves an embedded static file at a fixed path with an explicit type
// (the manifest's type is not in Go's mime table; the service worker must live at
// the root to control every page).
func serveAsset(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := assets.ReadFile("static/" + name)
		if err != nil {
			fail(w, err)
			return
		}
		w.Header().Set("Content-Type", contentType)
		w.Write(b)
	}
}

func (s *Server) gamesPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "games.html", map[string]any{"Games": games.All})
}
