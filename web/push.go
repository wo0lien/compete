package web

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/wo0lien/compete/store"
)

type pushSub struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// pushSubscribe stores this device's subscription (JSON from PushSubscription.toJSON).
func (s *Server) pushSubscribe(w http.ResponseWriter, r *http.Request, u store.User) {
	var sub pushSub
	if s.Push == nil || json.NewDecoder(r.Body).Decode(&sub) != nil || !validSub(sub) {
		http.Error(w, "bad subscription", http.StatusBadRequest)
		return
	}
	err := s.store.SaveSubscription(store.Subscription{Endpoint: sub.Endpoint, P256dh: sub.Keys.P256dh, Auth: sub.Keys.Auth, UserID: u.ID})
	if err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pushHosts are the browsers' push services. The server POSTs to a member's
// endpoint, so any other host (localhost, the LAN) is refused.
// ponytail: a fixed list; add a host when a browser ships a new push service.
var pushHosts = []string{"fcm.googleapis.com", ".google.com", ".push.services.mozilla.com", ".push.apple.com", ".notify.windows.com"}

func pushHost(host string) bool {
	for _, h := range pushHosts {
		if host == h || (strings.HasPrefix(h, ".") && strings.HasSuffix(host, h)) {
			return true
		}
	}
	return false
}

// validSub: an https endpoint on a known push service, default port, and keys
// of the sizes the Push API produces (65-byte P-256 point, 16-byte auth
// secret), base64url.
func validSub(sub pushSub) bool {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Scheme != "https" || u.Port() != "" || !pushHost(u.Hostname()) || len(sub.Endpoint) >= 2048 {
		return false
	}
	p, err1 := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.P256dh, "="))
	a, err2 := base64.RawURLEncoding.DecodeString(strings.TrimRight(sub.Keys.Auth, "="))
	return err1 == nil && err2 == nil && len(p) == 65 && len(a) == 16
}

func (s *Server) pushUnsubscribe(w http.ResponseWriter, r *http.Request, u store.User) {
	var sub pushSub
	if json.NewDecoder(r.Body).Decode(&sub) != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteUserSubscription(u.ID, sub.Endpoint); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setNotifications(w http.ResponseWriter, r *http.Request, u store.User) {
	on := func(k string) bool { return r.FormValue(k) == "1" }
	err := s.store.SetNotifyPrefs(u.ID, store.Prefs{Digest: on("digest"), Reminder: on("reminder"), Social: on("social"), Friends: on("friends")})
	if err != nil {
		s.oops(w, r, err)
		return
	}
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}
