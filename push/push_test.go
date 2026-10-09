package push

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/wo0lien/compete/store"
)

// fixtureDay: Tusmo #70, Songless #404 (see games tests).
var fixtureDay = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	st.Now = func() time.Time { return fixtureDay }
	return st
}

func echoText(lang, id string, args ...any) string { return lang + ":" + id }

// deviceKeys returns a browser-like subscription key pair.
func deviceKeys(t *testing.T) (p256dh, auth string) {
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := make([]byte, 16)
	rand.Read(a)
	enc := base64.RawURLEncoding.EncodeToString
	return enc(k.PublicKey().Bytes()), enc(a)
}

func TestKeysPersist(t *testing.T) {
	st := newTestStore(t)
	a, err := New(st, echoText, "https://example.org")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := New(st, echoText, "https://example.org")
	if a.PublicKey() == "" || a.PublicKey() != b.PublicKey() {
		t.Fatalf("keys not persisted: %q vs %q", a.PublicKey(), b.PublicKey())
	}
}

// A push service answering 410 means the device unsubscribed: its row goes,
// the member's other device stays.
func TestDeliverGoneDeletesSubscription(t *testing.T) {
	st := newTestStore(t)
	u, _ := st.CreateUser("alice", "correct horse")
	gone := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusGone) }))
	defer gone.Close()
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) }))
	defer ok.Close()
	p, a := deviceKeys(t)
	st.SaveSubscription(store.Subscription{Endpoint: gone.URL + "/x", P256dh: p, Auth: a, UserID: u.ID})
	st.SaveSubscription(store.Subscription{Endpoint: ok.URL + "/y", P256dh: p, Auth: a, UserID: u.ID})
	n, _ := New(st, echoText, "https://example.org")
	r, _ := st.Recipient(u.ID)
	for _, sub := range r.Subs {
		n.Send(sub, Message{Title: "compete", Body: "hi", URL: "/"})
	}
	r, _ = st.Recipient(u.ID)
	if len(r.Subs) != 1 || r.Subs[0].Endpoint != ok.URL+"/y" {
		t.Fatalf("subs = %+v, want only the live one", r.Subs)
	}
}
