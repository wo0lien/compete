package web

import (
	"errors"
	"net/http"

	"github.com/wo0lien/compete/store"
)

func (s *Server) account(w http.ResponseWriter, r *http.Request, u store.User) {
	s.render(w, r, http.StatusOK, "account.html", nil)
}

func (s *Server) setPawn(w http.ResponseWriter, r *http.Request, u store.User) {
	err := s.store.SetPawn(u.ID, r.FormValue("pawn"))
	if errors.Is(err, store.ErrBadPawn) {
		s.render(w, r, http.StatusUnprocessableEntity, "account.html", map[string]any{"Error": err.Error()})
		return
	}
	if err != nil {
		fail(w, err)
		return
	}
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

// deleteAccount asks for the password again: deletion is permanent.
func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request, u store.User) {
	if _, err := s.store.Authenticate(u.Username, r.FormValue("password")); err != nil {
		s.render(w, r, http.StatusUnauthorized, "account.html", map[string]any{"Error": "Wrong password, account kept."})
		return
	}
	if err := s.store.DeleteUser(u.ID); err != nil {
		fail(w, err)
		return
	}
	s.clearSession(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) resetForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, "reset.html", map[string]any{"Token": r.PathValue("token")})
}

func (s *Server) reset(w http.ResponseWriter, r *http.Request) {
	tok := r.PathValue("token")
	err := s.store.ResetPassword(tok, r.FormValue("password"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.message(w, r, http.StatusNotFound, "Link expired", "This reset link was already used or has expired. Ask your admin for a new one.")
	case errors.Is(err, store.ErrWeakPassword):
		s.render(w, r, http.StatusUnprocessableEntity, "reset.html", map[string]any{"Token": tok, "Error": err.Error()})
	case err != nil:
		fail(w, err)
	default:
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
