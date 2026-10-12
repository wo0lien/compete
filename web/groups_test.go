package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/wo0lien/compete/store"
)

func TestCreateJoinKick(t *testing.T) {
	ts, st := newTestServer(t)
	alice, bob := newClient(t), newClient(t)
	signup(t, ts, alice, "alice")
	signup(t, ts, bob, "bob")

	resp, _ := post(t, alice, ts.URL+"/groups", url.Values{"name": {"Les Potes"}})
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/g/1" {
		t.Fatalf("create = %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	gs, _ := st.GroupsOf(1)
	code := gs[0].InviteCode

	if _, body := get(t, bob, ts.URL+"/join/"+code); !strings.Contains(body, "Les Potes") {
		t.Fatal("join page does not name the group")
	}
	if resp, _ := post(t, bob, ts.URL+"/join/"+code, nil); resp.Header.Get("Location") != "/g/1" {
		t.Fatalf("join → %q", resp.Header.Get("Location"))
	}
	if resp, _ := post(t, bob, ts.URL+"/g/1/kick/1", nil); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("member kicking owner = %d, want 403", resp.StatusCode)
	}
	if _, body := get(t, bob, ts.URL+"/g/1/settings"); !strings.Contains(body, "alice") {
		t.Fatal("settings does not list members")
	}
	if resp, _ := post(t, alice, ts.URL+"/g/1/kick/2", nil); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("kick without confirm = %d, want 422", resp.StatusCode)
	}
	if _, body := get(t, alice, ts.URL+"/g/1/settings"); !strings.Contains(body, `<dialog id="confirm-kick-2"`) {
		t.Fatal("kick button should open a confirm sheet")
	}
	if resp, _ := post(t, alice, ts.URL+"/g/1/kick/2", url.Values{"confirm": {"yes"}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("kick = %d", resp.StatusCode)
	}
	if resp, _ := get(t, bob, ts.URL+"/g/1/settings"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("kicked member sees settings: %d", resp.StatusCode)
	}
}

func TestJoinNeedsLoginAndValidCode(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, _ := get(t, newClient(t), ts.URL+"/join/abc")
	if resp.Header.Get("Location") != "/login?next=%2Fjoin%2Fabc" {
		t.Fatalf("anonymous join → %q", resp.Header.Get("Location"))
	}
	c := newClient(t)
	signup(t, ts, c, "carol")
	if resp, _ := get(t, c, ts.URL+"/join/abc"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("bad code = %d, want 404", resp.StatusCode)
	}
}

func TestOwnerActions(t *testing.T) {
	ts, st := newTestServer(t)
	alice, bob, carol := newClient(t), newClient(t), newClient(t)
	signup(t, ts, alice, "alice")
	signup(t, ts, bob, "bob")
	signup(t, ts, carol, "carol")
	post(t, alice, ts.URL+"/groups", url.Values{"name": {"Les Potes"}})
	gs, _ := st.GroupsOf(1)
	old := gs[0].InviteCode
	post(t, bob, ts.URL+"/join/"+old, nil)

	if resp, _ := get(t, carol, ts.URL+"/g/1/settings"); resp.StatusCode != http.StatusNotFound { // Review Focus 2
		t.Fatalf("outsider settings = %d, want 404", resp.StatusCode)
	}
	post(t, alice, ts.URL+"/g/1/code", nil)
	if resp, _ := get(t, carol, ts.URL+"/join/"+old); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("old invite still works: %d", resp.StatusCode)
	}
	if resp, _ := post(t, alice, ts.URL+"/g/1/rename", url.Values{"name": {"  "}}); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("blank rename = %d, want 422", resp.StatusCode)
	}
	if resp, _ := post(t, alice, ts.URL+"/g/1/leave", nil); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("owner leave = %d, want 422", resp.StatusCode)
	}
	if resp, _ := post(t, bob, ts.URL+"/g/1/leave", nil); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("member leave = %d", resp.StatusCode)
	}
	if resp, _ := post(t, alice, ts.URL+"/g/1/delete", nil); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("delete without confirm = %d, want 422", resp.StatusCode)
	}
	if resp, _ := post(t, alice, ts.URL+"/g/1/delete", url.Values{"confirm": {"yes"}}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("delete = %d", resp.StatusCode)
	}
	if resp, _ := get(t, alice, ts.URL+"/g/1/settings"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("deleted group still there: %d", resp.StatusCode)
	}
}

// The paste forms carry a Paste button and the invite a Share/Copy button; both
// start hidden and app.js reveals them when the browser has the API.
func TestClipboardButtons(t *testing.T) {
	ts, alice, _ := twoPlayers(t)
	for _, p := range []string{"/", "/g/1"} {
		_, body := get(t, alice, ts.URL+p)
		if !strings.Contains(body, `<button type="button" class="btn" data-paste hidden>Paste</button>`) {
			t.Errorf("%s has no paste button:\n%s", p, body)
		}
		if !regexp.MustCompile(`<script src="/static/app\.js\?v=[0-9a-f]+" defer></script>`).MatchString(body) {
			t.Errorf("%s does not load app.js", p)
		}
	}
	_, body := get(t, alice, ts.URL+"/g/1/settings")
	if !regexp.MustCompile(`data-share="http://127\.0\.0\.1:\d+/join/[\w-]+" hidden>Share link</button>`).MatchString(body) {
		t.Errorf("settings has no share button with the invite URL:\n%s", body)
	}
}

func TestInviteURLUsesBaseURL(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := New(st, false, false)
	s.BaseURL = "https://play.example.org"
	ts := httptest.NewServer(s)
	defer ts.Close()
	c := newClient(t)
	signup(t, ts, c, "alice")
	post(t, c, ts.URL+"/groups", url.Values{"name": {"Les Potes"}})
	if _, body := get(t, c, ts.URL+"/g/1/settings"); !strings.Contains(body, `value="https://play.example.org/join/`) {
		t.Fatalf("invite link ignores BaseURL:\n%s", body)
	}
}

// Your own waiting tile is the way to play: it links to the game, no separate Play button.
func TestWaitingTileIsPlayLink(t *testing.T) {
	ts, alice, bob := twoPlayers(t)
	submit(t, ts, bob, tusmoBob)
	_, body := get(t, alice, ts.URL+"/g/1")
	if !strings.Contains(body, `class="start-tile play-tile" href="https://www.tusmo.xyz"`) || strings.Contains(body, `btn go play`) {
		t.Fatalf("own waiting tile is not the play link:\n%s", body)
	}
}

// The owner's settings carry one confirm sheet per destructive action; each Kick targets its own member.
func TestSettingsConfirmSheets(t *testing.T) {
	ts, st := newTestServer(t)
	owner, member := newClient(t), newClient(t)
	signup(t, ts, owner, "alice")
	signup(t, ts, member, "bob")
	post(t, owner, ts.URL+"/groups", url.Values{"name": {"G"}})
	gs, _ := st.GroupsOf(1)
	post(t, member, ts.URL+"/join/"+gs[0].InviteCode, nil)

	_, body := get(t, owner, ts.URL+"/g/1/settings")
	for _, want := range []string{
		`href="#confirm-new-link"`, `<dialog id="confirm-new-link"`,
		`href="#confirm-kick-2"`, `<dialog id="confirm-kick-2"`, `action="/g/1/kick/2"`,
		`href="#confirm-delete"`, `<dialog id="confirm-delete"`,
		`<input type="hidden" name="confirm" value="yes">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settings misses %q", want)
		}
	}
	if strings.Contains(body, `type="checkbox" name="confirm"`) {
		t.Error("delete still asks for a tick box")
	}

	// A member sees none of them.
	if _, body := get(t, member, ts.URL+"/g/1/settings"); strings.Contains(body, "<dialog") {
		t.Fatalf("member sees owner dialogs:\n%s", body)
	}

	// The delete sheet's own fields delete the group (no JS involved).
	post(t, owner, ts.URL+"/g/1/delete", url.Values{"confirm": {"yes"}})
	if resp, _ := get(t, owner, ts.URL+"/g/1"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("group still there after the sheet's delete: %d", resp.StatusCode)
	}
}

// The invite link reads as text; Copy and Share sit side by side in one row under it.
func TestInviteCopyButton(t *testing.T) {
	ts, st := newTestServer(t)
	c := newClient(t)
	signup(t, ts, c, "alice")
	post(t, c, ts.URL+"/groups", url.Values{"name": {"G"}})
	gs, _ := st.GroupsOf(1)
	_, body := get(t, c, ts.URL+"/g/1/settings")
	link := regexp.QuoteMeta(gs[0].InviteCode)
	if !regexp.MustCompile(`<input class="invite-link" readonly value="[^"]*/join/` + link + `"[^>]*>\s*<div class="invite-actions"><button type="button" class="btn" data-copy="[^"]*/join/` + link + `" data-copied="Copied" hidden>Copy</button><button type="button" class="btn" data-share="[^"]*/join/` + link + `" hidden>Share link</button></div>`).MatchString(body) {
		t.Fatalf("Copy and Share are not one row under the invite link:\n%s", body)
	}
	if !strings.Contains(body, `data-share="`) || strings.Contains(body, "data-copy-label") {
		t.Fatalf("Share should be its own button without a copy fallback:\n%s", body)
	}
}
