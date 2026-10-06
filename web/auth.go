package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/wo0lien/compete/store"
)

const sessionTTL = 90 * 24 * time.Hour

// auth wraps handlers that need a logged-in user; anonymous visitors are sent
// to the login page and come back afterwards.
func (s *Server) auth(h func(http.ResponseWriter, *http.Request, store.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFrom(r)
		if !ok {
			http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			return
		}
		h(w, r, u)
	}
}

// safeNext only allows local paths, so ?next= cannot redirect off-site.
// Browsers drop tabs/newlines and treat \ as /, so "/\t/evil.com" or "/\evil.com"
// would become "//evil.com": any control character or backslash is refused.
func safeNext(next string) string {
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") ||
		strings.ContainsRune(next, '\\') || strings.ContainsFunc(next, unicode.IsControl) {
		return "/"
	}
	return next
}

func (s *Server) setSession(w http.ResponseWriter, u store.User) error {
	tok, err := s.store.CreateSession(u.ID, sessionTTL)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: tok, Path: "/", MaxAge: int(sessionTTL / time.Second),
		HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
	return nil
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "session", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode})
}

func (s *Server) signupForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "signup.html", map[string]any{"Next": r.URL.Query().Get("next")})
}

func (s *Server) signup(w http.ResponseWriter, r *http.Request) {
	name, next := r.FormValue("username"), r.FormValue("next")
	u, err := s.store.CreateUser(name, r.FormValue("password"))
	if errors.Is(err, store.ErrBadUsername) || errors.Is(err, store.ErrWeakPassword) || errors.Is(err, store.ErrUsernameTaken) {
		s.render(w, r, http.StatusUnprocessableEntity, "signup.html", map[string]any{"Next": next, "Username": name, "Error": err.Error()})
		return
	}
	if err == nil {
		err = s.setSession(w, u)
	}
	if err != nil {
		fail(w, err)
		return
	}
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

func (s *Server) loginForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "login.html", map[string]any{"Next": r.URL.Query().Get("next")})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	name, next := r.FormValue("username"), r.FormValue("next")
	u, err := s.store.Authenticate(name, r.FormValue("password"))
	if errors.Is(err, store.ErrBadLogin) {
		s.render(w, r, http.StatusUnauthorized, "login.html", map[string]any{"Next": next, "Username": name, "Error": err.Error()})
		return
	}
	if err == nil {
		err = s.setSession(w, u)
	}
	if err != nil {
		fail(w, err)
		return
	}
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		s.store.DeleteSession(c.Value)
	}
	s.clearSession(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
