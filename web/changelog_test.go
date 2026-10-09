package web

import (
	"html"
	"net/http"
	"strings"
	"testing"
)

// Every release reads the same in both languages, newest first.
func TestChangelogFile(t *testing.T) {
	if len(changelog) == 0 {
		t.Fatal("no releases in changelog.toml")
	}
	for i, r := range changelog {
		if len(r.EN) == 0 || len(r.EN) != len(r.FR) {
			t.Errorf("%s: %d en / %d fr lines", r.Version, len(r.EN), len(r.FR))
		}
		for _, l := range append(r.EN, r.FR...) {
			if strings.TrimSpace(l) == "" {
				t.Errorf("%s: empty line", r.Version)
			}
		}
		if i > 0 && !versionLess(r.Version, changelog[i-1].Version) {
			t.Errorf("%s listed after %s: newest first", r.Version, changelog[i-1].Version)
		}
	}
}

func TestChangelogPageAndFooter(t *testing.T) {
	ts, _ := newTestServer(t) // Version is "" → shows "dev", which has no entry
	_, body := get(t, newClient(t), ts.URL+"/")
	if !strings.Contains(body, `<a href="/changelog">dev</a>`) {
		t.Fatalf("footer misses the version link:\n%s", body)
	}
	_, en := get(t, newClient(t), ts.URL+"/changelog")
	if !strings.Contains(en, "v0.1.0") || !strings.Contains(en, html.EscapeString(changelog[0].EN[0])) {
		t.Fatalf("changelog page misses releases:\n%s", en)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/changelog", nil)
	req.Header.Set("Accept-Language", "fr")
	resp, err := newClient(t).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if fr := readAll(t, resp); !strings.Contains(fr, html.EscapeString(changelog[0].FR[0])) {
		t.Fatalf("French changelog misses French lines:\n%s", fr)
	}
}
