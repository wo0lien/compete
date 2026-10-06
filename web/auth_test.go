package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// signup creates an account through the UI and leaves c logged in.
func signup(t *testing.T, ts *httptest.Server, c *http.Client, name string) {
	t.Helper()
	resp, body := post(t, c, ts.URL+"/signup", url.Values{"username": {name}, "password": {"correct horse"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("signup %s = %d\n%s", name, resp.StatusCode, body)
	}
}

func TestSignupLoginLogout(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t)
	signup(t, ts, c, "alice")
	if _, body := get(t, c, ts.URL+"/"); !strings.Contains(body, "alice") {
		t.Fatal("home does not show the logged-in user")
	}
	if resp, _ := post(t, c, ts.URL+"/logout", nil); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("logout = %d", resp.StatusCode)
	}
	if _, body := get(t, c, ts.URL+"/"); !strings.Contains(body, `href="/signup"`) {
		t.Fatal("still logged in after logout")
	}
	resp, body := post(t, c, ts.URL+"/login", url.Values{"username": {"alice"}, "password": {"wrong password"}})
	if resp.StatusCode != http.StatusUnauthorized || !strings.Contains(body, "wrong username or password") {
		t.Fatalf("bad login = %d\n%s", resp.StatusCode, body)
	}
	resp, _ = post(t, c, ts.URL+"/login", url.Values{"username": {"alice"}, "password": {"correct horse"}, "next": {"/g/1"}})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/g/1" {
		t.Fatalf("login = %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestSignupErrors(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, body := post(t, newClient(t), ts.URL+"/signup", url.Values{"username": {"bob"}, "password": {"short"}})
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "at least 10 characters") {
		t.Fatalf("weak password = %d\n%s", resp.StatusCode, body)
	}
}

func TestSafeNext(t *testing.T) { // Review Focus 4
	cases := map[string]string{"/g/1": "/g/1", "/share?text=a%20b": "/share?text=a%20b", "": "/", "//evil.com": "/",
		"https://evil.com": "/", `/\evil.com`: "/", "/\t/evil.com": "/", "/\n/evil.com": "/", "/\x7f/evil.com": "/"}
	for in, want := range cases {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
