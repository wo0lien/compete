package web

import (
	"net/http"
	"regexp"
	"testing"
)

// Pages link static files as /static/<name>?v=<content hash>. That exact URL is
// cached for a year; any other /static/ request must revalidate, and the ETag
// turns revalidation into a 304.
func TestStaticCaching(t *testing.T) {
	ts, _ := newTestServer(t)
	c := newClient(t)
	_, body := get(t, c, ts.URL+"/login")
	m := regexp.MustCompile(`/static/app\.css\?v=([0-9a-f]+)`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("page does not link a versioned app.css:\n%s", body)
	}

	resp, _ := get(t, c, ts.URL+m[0])
	if cc := resp.Header.Get("Cache-Control"); cc != "public, max-age=31536000, immutable" {
		t.Errorf("versioned Cache-Control = %q", cc)
	}
	etag := resp.Header.Get("ETag")
	if etag != `"`+m[1]+`"` {
		t.Errorf("ETag = %q, want the hash %q", etag, m[1])
	}

	for _, u := range []string{"/static/app.css", "/static/app.css?v=stale", "/static/fonts/nunito.woff2"} {
		resp, _ := get(t, c, ts.URL+u)
		if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-cache" || resp.Header.Get("ETag") == "" {
			t.Errorf("%s: status %d, Cache-Control %q, ETag %q", u, resp.StatusCode, resp.Header.Get("Cache-Control"), resp.Header.Get("ETag"))
		}
	}

	req, _ := http.NewRequest("GET", ts.URL+"/static/app.css", nil)
	req.Header.Set("If-None-Match", etag)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("If-None-Match: status %d, want 304", resp.StatusCode)
	}
}
