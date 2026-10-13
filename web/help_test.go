package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestPlatformOf(t *testing.T) {
	for _, c := range []struct{ ua, want string }{
		{"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Mobile Safari/537.36", "android-chrome"},
		{"Mozilla/5.0 (Linux; Android 14; SM-S911B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/26.0 Chrome/122.0.0.0 Mobile Safari/537.36", "android-other"},
		{"Mozilla/5.0 (Android 14; Mobile; rv:131.0) Gecko/131.0 Firefox/131.0", "android-other"},
		{"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Mobile Safari/537.36 EdgA/129.0.0.0", "android-other"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1", "ios"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/129.0.6668.69 Mobile/15E148 Safari/604.1", "ios"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36", "desktop"},
		{"", "desktop"},
	} {
		if got := platformOf(c.ua); got != c.want {
			t.Errorf("platformOf(%q) = %q, want %q", c.ua, got, c.want)
		}
	}
}

// /help opens on the visitor's platform, logged out too; the other platforms stay one tap away.
func TestHelpPage(t *testing.T) {
	ts, _ := newTestServer(t)
	req, _ := http.NewRequest("GET", ts.URL+"/help", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Mobile Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /help = %d", resp.StatusCode)
	}
	for _, want := range []string{
		`aria-selected="true" data-variant="android-chrome"`,
		`aria-selected="false" data-variant="ios"`,
		`data-panel="desktop"`,
		"Why Chrome on Android",
		`data-install hidden`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/help lacks %q", want)
		}
	}
}

// Members get the welcome card (app.js shows it until dismissed); everyone gets the foot link to /help.
func TestHomeWelcome(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t)
	if _, body := get(t, c, ts.URL+"/"); strings.Contains(body, "data-welcome") || !strings.Contains(body, `<a href="/help">How compete works</a>`) {
		t.Fatalf("logged out: want the foot link, no card:\n%s", body)
	}
	signup(t, ts, c, "alice")
	_, body := get(t, c, ts.URL+"/")
	if !strings.Contains(body, `class="panel welcome" data-welcome hidden`) || !strings.Contains(body, `href="/help" class="btn go" data-welcome-done`) {
		t.Fatalf("member: no hidden welcome card linking to /help:\n%s", body)
	}
}
