package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

// badgeGroup: alice, bob and carol joined group 1 (tusmo) three days before
// the fixture day; alice alone played Tusmo #70 that day, which is now over.
func badgeGroup(t *testing.T) (ts *httptest.Server, st *store.Store, alice, bob *http.Client) {
	t.Helper()
	ts, st = newTestServer(t)
	fixture := st.Now()
	st.Now = func() time.Time { return fixture.AddDate(0, 0, -3) }
	alice, bob, carol := newClient(t), newClient(t), newClient(t)
	signup(t, ts, alice, "alice")
	signup(t, ts, bob, "bob")
	signup(t, ts, carol, "carol")
	post(t, alice, ts.URL+"/groups", url.Values{"name": {"Les Potes"}, "game": {"tusmo"}})
	gs, _ := st.GroupsOf(1)
	post(t, bob, ts.URL+"/join/"+gs[0].InviteCode, nil)
	post(t, carol, ts.URL+"/join/"+gs[0].InviteCode, nil)
	st.Now = func() time.Time { return fixture }
	if _, err := st.AddResults(1, "raw", []games.Result{{Game: "tusmo", PuzzleID: 70, Score: new(3)}}); err != nil {
		t.Fatal(err)
	}
	st.Now = func() time.Time { return fixture.AddDate(0, 0, 1) }
	return ts, st, alice, bob
}

func TestBadgesTab(t *testing.T) {
	ts, _, alice, _ := badgeGroup(t)
	resp, body := get(t, alice, ts.URL+"/g/1/badges")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("badges tab = %d", resp.StatusCode)
	}
	for _, want := range []string{"À vaincre sans péril…", "×1", "/static/badges/lonely.svg", `href="/b/`, `aria-current="page">Badges`, "medal locked", "Tue 6 Oct"} {
		if !strings.Contains(body, want) {
			t.Errorf("badges tab lacks %q", want)
		}
	}
}

func TestBadgesTabMarksSeen(t *testing.T) {
	ts, st, alice, _ := badgeGroup(t)
	st.AwardBadges(1)
	if u, _ := st.Unseen(1); u[1].Count != 1 {
		t.Fatalf("before: unseen = %v, want 1", u)
	}
	st.Now = func() time.Time { return time.Date(2026, 10, 7, 12, 0, 1, 0, time.UTC) }
	get(t, alice, ts.URL+"/g/1/badges")
	if u, _ := st.Unseen(1); len(u) != 0 {
		t.Fatalf("after opening the tab: unseen = %v, want none", u)
	}
}

func TestBadgesTabOutsider404(t *testing.T) {
	ts, _, _, _ := badgeGroup(t)
	dave := newClient(t)
	signup(t, ts, dave, "dave")
	if resp, _ := get(t, dave, ts.URL+"/g/1/badges"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("outsider = %d, want 404", resp.StatusCode)
	}
}

func TestShortDate(t *testing.T) {
	if got := shortDate("en", "2026-10-26"); got != "Mon 26 Oct" {
		t.Errorf("en = %q", got)
	}
	if got := shortDate("fr", "2026-10-26"); got != "lun. 26 oct." {
		t.Errorf("fr = %q", got)
	}
}

func TestTodayNewsRow(t *testing.T) {
	ts, _, alice, bob := badgeGroup(t)
	_, body := get(t, alice, ts.URL+"/g/1")
	if !strings.Contains(body, `class="news-row" href="/g/1/badges"`) || !strings.Contains(body, "À vaincre sans péril, on triomphe sans gloire") {
		t.Fatal("alice's Today lacks the new-badge row")
	}
	if _, body := get(t, bob, ts.URL+"/g/1"); strings.Contains(body, "news-row") {
		t.Fatal("bob sees alice's badge as his news")
	}
}

func TestHomeGroupPill(t *testing.T) {
	ts, st, alice, _ := badgeGroup(t)
	_, body := get(t, alice, ts.URL+"/")
	if !strings.Contains(body, `<a class="group-row" href="/g/1/badges">`) || !strings.Contains(body, "news-pill") {
		t.Fatal("Home lacks the group's news pill linking to its badges")
	}
	later := st.Now().Add(time.Second)
	st.Now = func() time.Time { return later }
	get(t, alice, ts.URL+"/g/1/badges")
	_, body = get(t, alice, ts.URL+"/")
	if strings.Contains(body, "news-pill") || !strings.Contains(body, `<a class="group-row" href="/g/1">`) {
		t.Fatal("after opening Badges, Home still shows news")
	}
}

func shareToken(t *testing.T, st *store.Store) string {
	t.Helper()
	if err := st.AwardBadges(1); err != nil {
		t.Fatal(err)
	}
	bs, _ := st.BadgesOf(1, 1)
	if len(bs) != 1 {
		t.Fatalf("alice badges = %v, want 1", bs)
	}
	return bs[0].Token
}

// Review Focus 3: WhatsApp fetches the link with no cookie and no language.
func TestShareLoggedOutOG(t *testing.T) {
	ts, st, _, _ := badgeGroup(t)
	st.SetLang(1, "fr")
	token := shareToken(t, st)
	resp, err := http.Get(ts.URL + "/b/" + token)
	if err != nil {
		t.Fatal(err)
	}
	body := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("share = %d, want 200 without login", resp.StatusCode)
	}
	for _, want := range []string{
		`<meta property="og:title" content="alice a gagné « À vaincre sans péril, on triomphe sans gloire »">`,
		`<meta property="og:description" content="Seul·e à jouer le mar. 6 oct. dans Les Potes. Médaille d&#39;or, tribunes vides.">`,
		`<meta property="og:image" content="` + ts.URL + `/static/badges/lonely-og.png">`,
		`<meta property="og:url" content="` + ts.URL + `/b/` + token + `">`,
		`<meta name="twitter:card" content="summary_large_image">`,
		`href="/login?next=%2fb%2f` + token + `"`,
		"/static/badges/lonely.svg",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("share page lacks %s", want)
		}
	}
	if strings.Contains(body, `href="/g/1`) {
		t.Error("logged-out viewer gets a group link")
	}
}

func TestShareMemberAndOutsider(t *testing.T) {
	ts, st, _, bob := badgeGroup(t)
	token := shareToken(t, st)
	if _, body := get(t, bob, ts.URL+"/b/"+token); !strings.Contains(body, `href="/g/1/badges"`) {
		t.Error("member lacks the link to the group's badges")
	}
	dave := newClient(t)
	signup(t, ts, dave, "dave")
	_, body := get(t, dave, ts.URL+"/b/"+token)
	if strings.Contains(body, `href="/g/1`) || !strings.Contains(body, `class="btn go wide" href="/"`) {
		t.Error("outsider should get Home, not the group")
	}
}

func TestShareUnknownToken(t *testing.T) {
	ts, _, _, _ := badgeGroup(t)
	if resp, _ := get(t, newClient(t), ts.URL+"/b/nope"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown token = %d, want 404", resp.StatusCode)
	}
}
