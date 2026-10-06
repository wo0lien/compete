package web

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/wo0lien/compete/store"
)

// message renders a small page with a title and a sentence (errors, confirmations).
func (s *Server) message(w http.ResponseWriter, r *http.Request, status int, title, text string) {
	s.render(w, r, status, "message.html", map[string]any{"Title": title, "Text": text})
}

// groupFor loads group {id} if u is a member. Anyone else gets 404, so
// outsiders can't tell whether a group exists.
func (s *Server) groupFor(w http.ResponseWriter, r *http.Request, u store.User) (store.Group, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		g, err := s.store.GroupFor(u.ID, id)
		if err == nil {
			return g, true
		}
		if !errors.Is(err, store.ErrNotFound) {
			s.oops(w, r, err)
			return store.Group{}, false
		}
	}
	s.message(w, r, http.StatusNotFound, "Not found", "This group doesn't exist or you're not in it.")
	return store.Group{}, false
}

// ownerFor is groupFor plus 403 for members who are not the owner.
func (s *Server) ownerFor(w http.ResponseWriter, r *http.Request, u store.User) (store.Group, bool) {
	g, ok := s.groupFor(w, r, u)
	if ok && g.Role != "owner" {
		s.message(w, r, http.StatusForbidden, "Owner only", "Only the group owner can do that.")
		return g, false
	}
	return g, ok
}

func settingsURL(g store.Group) string { return fmt.Sprintf("/g/%d/settings", g.ID) }

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request, u store.User) {
	g, err := s.store.CreateGroup(u.ID, r.FormValue("name"))
	if errors.Is(err, store.ErrBadGroupName) || errors.Is(err, store.ErrTooManyGroups) {
		s.message(w, r, http.StatusUnprocessableEntity, "Can't create group", err.Error())
		return
	}
	if err != nil {
		s.oops(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/g/%d", g.ID), http.StatusSeeOther)
}

// joinForm asks before joining: GET never changes state.
func (s *Server) joinForm(w http.ResponseWriter, r *http.Request, u store.User) {
	g, err := s.store.GroupByCode(r.PathValue("code"))
	if errors.Is(err, store.ErrNotFound) {
		s.message(w, r, http.StatusNotFound, "Invite expired", "This invite link is invalid or was replaced. Ask for a new one.")
		return
	}
	if err != nil {
		s.oops(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, "join.html", map[string]any{"Group": g, "Code": r.PathValue("code")})
}

func (s *Server) join(w http.ResponseWriter, r *http.Request, u store.User) {
	g, err := s.store.JoinByCode(u.ID, r.PathValue("code"))
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.message(w, r, http.StatusNotFound, "Invite expired", "This invite link is invalid or was replaced. Ask for a new one.")
	case errors.Is(err, store.ErrTooManyGroups):
		s.message(w, r, http.StatusUnprocessableEntity, "Too many groups", err.Error())
	case err != nil:
		s.oops(w, r, err)
	default:
		http.Redirect(w, r, fmt.Sprintf("/g/%d", g.ID), http.StatusSeeOther)
	}
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.groupFor(w, r, u)
	if !ok {
		return
	}
	members, err := s.store.Members(g.ID)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	scheme := "https"
	if !s.secure {
		scheme = "http"
	}
	s.render(w, r, http.StatusOK, "settings.html", map[string]any{
		"Group": g, "Members": members, "Me": u.ID,
		"InviteURL": scheme + "://" + r.Host + "/join/" + g.InviteCode,
	})
}

func (s *Server) rename(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.ownerFor(w, r, u)
	if !ok {
		return
	}
	err := s.store.RenameGroup(g.ID, r.FormValue("name"))
	if errors.Is(err, store.ErrBadGroupName) {
		s.message(w, r, http.StatusUnprocessableEntity, "Can't rename", err.Error())
		return
	}
	if err != nil {
		s.oops(w, r, err)
		return
	}
	http.Redirect(w, r, settingsURL(g), http.StatusSeeOther)
}

func (s *Server) newCode(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.ownerFor(w, r, u)
	if !ok {
		return
	}
	if _, err := s.store.RegenerateCode(g.ID); err != nil {
		s.oops(w, r, err)
		return
	}
	http.Redirect(w, r, settingsURL(g), http.StatusSeeOther)
}

func (s *Server) kick(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.ownerFor(w, r, u)
	if !ok {
		return
	}
	if r.FormValue("confirm") != "yes" {
		s.message(w, r, http.StatusUnprocessableEntity, "Not kicked", "Confirm the kick to remove this player.")
		return
	}
	uid, err := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if err != nil {
		s.message(w, r, http.StatusNotFound, "Not found", "No such member.")
		return
	}
	if err := s.store.RemoveMember(g.ID, uid); err != nil {
		s.oops(w, r, err)
		return
	}
	http.Redirect(w, r, settingsURL(g), http.StatusSeeOther)
}

func (s *Server) leave(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.groupFor(w, r, u)
	if !ok {
		return
	}
	if g.Role == "owner" {
		s.message(w, r, http.StatusUnprocessableEntity, "You own this group", "Owners can't leave; delete the group instead.")
		return
	}
	if err := s.store.RemoveMember(g.ID, u.ID); err != nil {
		s.oops(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request, u store.User) {
	g, ok := s.ownerFor(w, r, u)
	if !ok {
		return
	}
	// Deleting removes everyone's group: require the confirm box (works without JS).
	if r.FormValue("confirm") != "yes" {
		s.message(w, r, http.StatusUnprocessableEntity, "Not deleted", "Tick the confirmation box to delete the group for everyone.")
		return
	}
	if err := s.store.DeleteGroup(g.ID); err != nil {
		s.oops(w, r, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
