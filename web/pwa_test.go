package web

import (
	"encoding/json"
	"net/http"
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

// The manifest description follows the visitor's language; the maskable icon
// is its own entry so browsers never crop the plain one.
func TestManifestLanguage(t *testing.T) {
	ts, _ := newTestServer(t)
	req, _ := http.NewRequest("GET", ts.URL+"/manifest.webmanifest", nil)
	req.Header.Set("Accept-Language", "fr-FR")
	resp, err := newClient(t).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Description string
		Icons       []struct{ Purpose string }
	}
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if want := Translate("fr", "manifest.about"); m.Description != want || want == Translate("en", "manifest.about") {
		t.Fatalf("description = %q, want the French %q", m.Description, want)
	}
	if !strings.Contains(resp.Header.Get("Vary"), "Accept-Language") {
		t.Fatalf("Vary = %q, caches would mix languages", resp.Header.Get("Vary"))
	}
	for _, ic := range m.Icons {
		if strings.Contains(ic.Purpose, " ") {
			t.Fatalf("icon purpose %q: split any and maskable", ic.Purpose)
		}
	}
}
