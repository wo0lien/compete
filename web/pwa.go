package web

import (
	"encoding/json"
	"net/http"

	"github.com/wo0lien/compete/games"
)

// serveAsset serves an embedded static file at a fixed path with an explicit type
// (the service worker must live at the root to control every page).
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

// manifest serves the PWA manifest with its description in the visitor's language.
func manifest(w http.ResponseWriter, r *http.Request) {
	b, err := assets.ReadFile("static/manifest.webmanifest")
	if err != nil {
		fail(w, err)
		return
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		fail(w, err)
		return
	}
	m["description"] = tr(langFrom(r), "manifest.about")
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Vary", "Accept-Language, Cookie")
	json.NewEncoder(w).Encode(m)
}

func (s *Server) gamesPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "games.html", map[string]any{"Games": games.All})
}
