package web

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wo0lien/compete/push"
	"github.com/wo0lien/compete/store"
)

func newPushServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.Now = func() time.Time { return time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC) }
	s := New(st, false, false)
	s.Push, err = push.New(st, Translate, "https://example.org")
	if err != nil {
		t.Fatal(err)
	}
	s.Push.Send = func(store.Subscription, push.Message) error { return nil }
	ts := httptest.NewServer(s)
	t.Cleanup(func() { ts.Close(); st.Close() })
	return ts, st
}

func postJSON(t *testing.T, c *http.Client, u string, v any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(v)
	resp, err := c.Post(u, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func TestPushSubscribe(t *testing.T) {
	ts, st := newPushServer(t)
	c := newClient(t)
	k, _ := ecdh.P256().GenerateKey(rand.Reader)
	enc := base64.RawURLEncoding.EncodeToString
	good := map[string]any{"endpoint": "https://push.example/d1", "keys": map[string]string{"p256dh": enc(k.PublicKey().Bytes()), "auth": enc(make([]byte, 16))}}

	if resp := postJSON(t, c, ts.URL+"/push/subscribe", good); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("anonymous subscribe = %d, want a login redirect", resp.StatusCode)
	}
	signup(t, ts, c, "alice")
	for _, bad := range []map[string]any{
		{"endpoint": "http://push.example/d1", "keys": good["keys"]},
		{"endpoint": "https://push.example/d1", "keys": map[string]string{"p256dh": "nope", "auth": "x"}},
	} {
		if resp := postJSON(t, c, ts.URL+"/push/subscribe", bad); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("bad subscription %v = %d, want 400", bad, resp.StatusCode)
		}
	}
	if resp := postJSON(t, c, ts.URL+"/push/subscribe", good); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("subscribe = %d", resp.StatusCode)
	}
	if r, _ := st.Recipient(1); len(r.Subs) != 1 {
		t.Fatalf("subs = %d, want 1", len(r.Subs))
	}

	_, body := get(t, c, ts.URL+"/account")
	if !strings.Contains(body, `data-push-key="`) || !strings.Contains(body, `name="digest" value="1" checked`) || strings.Contains(body, `name="friends" value="1" checked`) {
		t.Fatalf("account page misses the notifications section with defaults:\n%s", body)
	}
	post(t, c, ts.URL+"/account/notifications", url.Values{"friends": {"1"}})
	if p, _ := st.NotifyPrefs(1); p != (store.Prefs{Friends: true}) {
		t.Fatalf("prefs = %+v, want friends only", p)
	}

	postJSON(t, c, ts.URL+"/push/unsubscribe", map[string]string{"endpoint": "https://push.example/d1"})
	if r, _ := st.Recipient(1); len(r.Subs) != 0 {
		t.Fatal("unsubscribe left the subscription")
	}
}

func TestAccountWithoutPush(t *testing.T) {
	ts, _ := newTestServer(t) // Push nil
	c := newClient(t)
	signup(t, ts, c, "alice")
	if _, body := get(t, c, ts.URL+"/account"); strings.Contains(body, "data-push-key") {
		t.Fatal("notifications section shown without a notifier")
	}
}
