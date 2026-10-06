package web

import (
	"strings"
	"testing"
)

func TestPWAFiles(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, body := get(t, newClient(t), ts.URL+"/manifest.webmanifest")
	if resp.Header.Get("Content-Type") != "application/manifest+json" || !strings.Contains(body, `"share_target"`) {
		t.Fatalf("manifest = %q\n%s", resp.Header.Get("Content-Type"), body)
	}
	resp, _ = get(t, newClient(t), ts.URL+"/sw.js")
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/javascript") {
		t.Fatalf("sw.js content type = %q", resp.Header.Get("Content-Type"))
	}
}

func TestGamesPage(t *testing.T) {
	ts, _ := newTestServer(t)
	_, body := get(t, newClient(t), ts.URL+"/games")
	for _, want := range []string{"Tusmo", "https://www.tusmo.xyz", "Songless", "Travle", "geography", "fr"} {
		if !strings.Contains(body, want) {
			t.Errorf("games page misses %q", want)
		}
	}
}
