// Package web serves compete's server-rendered UI. Every page is a full HTML
// document; htmx (hx-boost) only turns links and forms into in-place swaps.
package web

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

//go:embed templates static
var assets embed.FS

// maxBody caps request bodies: share texts and forms are tiny.
const maxBody = 64 << 10

type Server struct {
	store      *store.Store
	pages      map[string]*template.Template
	handler    http.Handler
	limit      *limiter
	secure     bool // Secure cookies; false only for local http and tests
	trustProxy bool // client IP from X-Forwarded-For (behind Caddy)
}

var funcs = template.FuncMap{
	"gameName": func(id string) string {
		if g, ok := games.ByID(id); ok {
			return g.Name
		}
		return id
	},
	"gameLang":     func(id string) string { g, _ := games.ByID(id); return g.Lang },
	"scoreText":    func(id string, score *int) string { g, _ := games.ByID(id); return g.ShowScore(score) },
	"tiebreakText": func(id string, t *int) string { g, _ := games.ByID(id); return g.ShowTiebreak(t) },
	"inc":          func(i int) int { return i + 1 },
}

func New(st *store.Store, secure, trustProxy bool) *Server {
	s := &Server{store: st, secure: secure, trustProxy: trustProxy,
		limit: newLimiter(20, time.Minute), pages: map[string]*template.Template{}}
	names, err := fs.Glob(assets, "templates/*.html")
	if err != nil {
		panic(err)
	}
	for _, p := range names {
		name := path.Base(p)
		if name == "base.html" || name == "partials.html" {
			continue
		}
		s.pages[name] = template.Must(template.New("").Funcs(funcs).
			ParseFS(assets, "templates/base.html", "templates/partials.html", p))
	}
	mux := http.NewServeMux()
	s.routes(mux)
	s.handler = http.NewCrossOriginProtection().Handler(s.withUser(mux))
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	s.handler.ServeHTTP(w, r)
}

func (s *Server) routes(mux *http.ServeMux) {
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	mux.HandleFunc("GET /{$}", s.home)
	mux.HandleFunc("GET /signup", s.signupForm)
	mux.HandleFunc("POST /signup", s.limited(s.signup))
	mux.HandleFunc("GET /login", s.loginForm)
	mux.HandleFunc("POST /login", s.limited(s.login))
	mux.HandleFunc("POST /logout", s.logout)
	mux.HandleFunc("POST /groups", s.auth(s.createGroup))
	mux.HandleFunc("GET /join/{code}", s.auth(s.joinForm))
	mux.HandleFunc("POST /join/{code}", s.auth(s.join))
	mux.HandleFunc("GET /g/{id}/settings", s.auth(s.settings))
	mux.HandleFunc("POST /g/{id}/rename", s.auth(s.rename))
	mux.HandleFunc("POST /g/{id}/code", s.auth(s.newCode))
	mux.HandleFunc("POST /g/{id}/kick/{uid}", s.auth(s.kick))
	mux.HandleFunc("POST /g/{id}/leave", s.auth(s.leave))
	mux.HandleFunc("POST /g/{id}/delete", s.auth(s.deleteGroup))
	mux.HandleFunc("GET /g/{id}", s.auth(s.groupPage))
	mux.HandleFunc("POST /results", s.limited(s.auth(s.submit)))
	mux.HandleFunc("GET /share", s.auth(s.share))
	mux.HandleFunc("GET /g/{id}/leaderboard", s.auth(s.leaderboard))
	mux.HandleFunc("GET /g/{id}/history", s.auth(s.history))
	mux.HandleFunc("GET /g/{id}/board", s.auth(s.onePuzzle))
	mux.HandleFunc("GET /account", s.auth(s.account))
	mux.HandleFunc("POST /account/pawn", s.auth(s.setPawn))
	mux.HandleFunc("POST /account/delete", s.auth(s.deleteAccount))
	mux.HandleFunc("GET /reset/{token}", s.resetForm)
	mux.HandleFunc("POST /reset/{token}", s.limited(s.reset))
	mux.HandleFunc("GET /manifest.webmanifest", serveAsset("manifest.webmanifest", "application/manifest+json"))
	mux.HandleFunc("GET /sw.js", serveAsset("sw.js", "text/javascript; charset=utf-8"))
	mux.HandleFunc("GET /games", s.gamesPage)
}

type ctxKey struct{}

// withUser attaches the logged-in user, if any, to the request context.
func (s *Server) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err == nil {
			if u, err := s.store.UserBySession(c.Value); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func userFrom(r *http.Request) (store.User, bool) {
	u, ok := r.Context().Value(ctxKey{}).(store.User)
	return u, ok
}

// render writes a full page; data always gets the logged-in user as .User.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page string, data map[string]any) {
	if data == nil {
		data = map[string]any{}
	}
	if u, ok := userFrom(r); ok {
		data["User"] = u
	}
	var buf bytes.Buffer
	if err := s.pages[page].ExecuteTemplate(&buf, "base", data); err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// fail logs an unexpected error and answers 500 without details.
func fail(w http.ResponseWriter, err error) {
	log.Print(err)
	http.Error(w, "Something went wrong.", http.StatusInternalServerError)
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	u, ok := userFrom(r)
	if !ok {
		s.render(w, r, http.StatusOK, "home.html", nil)
		return
	}
	groups, err := s.store.GroupsOf(u.ID)
	if err != nil {
		fail(w, err)
		return
	}
	s.render(w, r, http.StatusOK, "home.html", map[string]any{"Groups": groups})
}
