package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const tusmoAlice = "TUSMO #70 3/6 - 0:35\n\n🟥🟥🟥⬛🟨🟨🟨⬛\n🟥🟥🟥🟥🟥🟥🟥🟥"
const tusmoBob = "TUSMO #70 4/6 - 1:10\n\n🟥⬛⬛⬛⬛⬛⬛⬛\n🟥🟥🟥🟥🟥🟥🟥🟥"

func submit(t *testing.T, ts *httptest.Server, c *http.Client, text string) *http.Response {
	t.Helper()
	resp, _ := post(t, c, ts.URL+"/results", url.Values{"text": {text}, "back": {"/g/1"}})
	return resp
}

// twoPlayers returns alice (owner) and bob in group 1.
func twoPlayers(t *testing.T) (*httptest.Server, *http.Client, *http.Client) {
	t.Helper()
	ts, st := newTestServer(t)
	alice, bob := newClient(t), newClient(t)
	signup(t, ts, alice, "alice")
	signup(t, ts, bob, "bob")
	post(t, alice, ts.URL+"/groups", url.Values{"name": {"Les Potes"}})
	gs, _ := st.GroupsOf(1)
	post(t, bob, ts.URL+"/join/"+gs[0].InviteCode, nil)
	return ts, alice, bob
}

func TestBoardHiddenThenRevealed(t *testing.T) { // Review Focus 1
	ts, alice, bob := twoPlayers(t)
	if resp := submit(t, ts, alice, tusmoAlice); resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/g/1" {
		t.Fatalf("submit = %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	_, body := get(t, bob, ts.URL+"/g/1")
	if !strings.Contains(body, "Tusmo #70") || !strings.Contains(body, "hidden-tile") {
		t.Fatalf("bob should see a hidden Tusmo board:\n%s", body)
	}
	if strings.Contains(body, "🟥🟥🟥⬛🟨🟨🟨⬛") || strings.Contains(body, "3/6") {
		t.Fatal("hidden board leaks alice's grid or score")
	}
	submit(t, ts, bob, tusmoBob)
	_, body = get(t, bob, ts.URL+"/g/1")
	if !strings.Contains(body, "🟥🟥🟥⬛🟨🟨🟨⬛") || !strings.Contains(body, "3/6") || !strings.Contains(body, "0:35") {
		t.Fatalf("revealed board misses alice's result:\n%s", body)
	}
	if strings.Index(body, "alice") > strings.Index(body, "4/6") {
		t.Fatal("alice (3/6) should rank above bob (4/6)")
	}
}

func TestSubmitErrors(t *testing.T) {
	ts, alice, _ := twoPlayers(t)
	resp, body := post(t, alice, ts.URL+"/results", url.Values{"text": {"hello"}, "back": {"/g/1"}})
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "supported yet") {
		t.Fatalf("unsupported = %d\n%s", resp.StatusCode, body)
	}
	submit(t, ts, alice, tusmoAlice)
	resp, body = post(t, alice, ts.URL+"/results", url.Values{"text": {tusmoAlice}, "back": {"/g/1"}})
	if resp.StatusCode != http.StatusConflict || !strings.Contains(body, "Already submitted Tusmo #70") {
		t.Fatalf("duplicate = %d\n%s", resp.StatusCode, body)
	}
	resp, _ = post(t, alice, ts.URL+"/results", url.Values{"text": {strings.Replace(tusmoAlice, "#70", "#71", 1)}, "back": {"//evil.com"}})
	if resp.Header.Get("Location") != "/" { // Review Focus 4
		t.Fatalf("back=//evil.com → %q, want /", resp.Header.Get("Location"))
	}
}

func TestGroupPageOutsider(t *testing.T) { // Review Focus 2
	ts, _, _ := twoPlayers(t)
	c := newClient(t)
	signup(t, ts, c, "carol")
	if resp, _ := get(t, c, ts.URL+"/g/1"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("outsider = %d, want 404", resp.StatusCode)
	}
}

func TestShareTarget(t *testing.T) { // Review Focus 3
	ts, alice, _ := twoPlayers(t)
	q := url.Values{"title": {"Tusmo"}, "text": {"TUSMO #70 3/6 - 0:35"}, "url": {"https://www.tusmo.xyz"}}
	_, body := get(t, alice, ts.URL+"/share?"+q.Encode())
	if !strings.Contains(body, "TUSMO #70 3/6 - 0:35\nTusmo\nhttps://www.tusmo.xyz</textarea>") {
		t.Fatalf("share page should prefill text first:\n%s", body)
	}
	resp, _ := get(t, newClient(t), ts.URL+"/share?"+q.Encode())
	if !strings.HasPrefix(resp.Header.Get("Location"), "/login?next=%2Fshare%3F") {
		t.Fatalf("anonymous share → %q", resp.Header.Get("Location"))
	}
}

func TestSubmitTooLarge(t *testing.T) {
	ts, alice, _ := twoPlayers(t)
	resp, body := post(t, alice, ts.URL+"/results", url.Values{"text": {strings.Repeat("x", maxBody)}, "back": {"/g/1"}})
	if resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(body, "Too long") || !strings.Contains(body, `class="logo"`) {
		t.Fatalf("oversized paste = %d, want a 413 page:\n%s", resp.StatusCode, body)
	}
}
