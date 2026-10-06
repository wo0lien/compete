package web

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wo0lien/compete/store"
)

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(st, false, false))
	t.Cleanup(func() { ts.Close(); st.Close() })
	return ts, st
}

// newClient keeps cookies and does not follow redirects, so tests can assert Location.
func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func get(t *testing.T, c *http.Client, u string) (*http.Response, string) {
	t.Helper()
	resp, err := c.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	return resp, readAll(t, resp)
}

func post(t *testing.T, c *http.Client, u string, form url.Values) (*http.Response, string) {
	t.Helper()
	resp, err := c.PostForm(u, form)
	if err != nil {
		t.Fatal(err)
	}
	return resp, readAll(t, resp)
}

func TestHomeLoggedOut(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, body := get(t, newClient(t), ts.URL+"/")
	if resp.StatusCode != 200 || !strings.Contains(body, `href="/signup"`) || !strings.Contains(body, "compete") {
		t.Fatalf("GET / = %d\n%s", resp.StatusCode, body)
	}
}

func TestStaticAssets(t *testing.T) {
	ts, _ := newTestServer(t)
	for _, p := range []string{"/static/app.css", "/static/htmx.min.js", "/static/fonts/nunito.woff2", "/static/fonts/lilita-one.woff2"} {
		if resp, _ := get(t, newClient(t), ts.URL+p); resp.StatusCode != 200 {
			t.Errorf("GET %s = %d", p, resp.StatusCode)
		}
	}
}

func TestCrossSitePostRejected(t *testing.T) {
	ts, _ := newTestServer(t)
	req, _ := http.NewRequest("POST", ts.URL+"/login", strings.NewReader("username=a&password=b"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	resp, err := newClient(t).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-site POST = %d, want 403", resp.StatusCode)
	}
}

func TestLimiter(t *testing.T) {
	l := newLimiter(2, time.Hour)
	if !l.allow("1.2.3.4") || !l.allow("1.2.3.4") || l.allow("1.2.3.4") {
		t.Fatal("third hit in the window should be refused")
	}
	if !l.allow("5.6.7.8") {
		t.Fatal("other IPs have their own budget")
	}
}
