package web

import (
	"io/fs"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/wo0lien/compete/games"
)

func TestResolveLang(t *testing.T) {
	for _, c := range []struct{ account, cookie, accept, want string }{
		{"", "", "", "en"},
		{"", "", "fr-FR,fr;q=0.9,en;q=0.8", "fr"},
		{"", "", "de-DE,fr;q=0.8", "fr"},
		{"", "", "de-DE,es;q=0.8", "en"}, // neither: English
		{"", "", "garbage;;q=x", "en"},
		{"", "fr", "en-US", "fr"}, // cookie beats browser
		{"en", "fr", "fr", "en"},  // account beats cookie
		{"", "de", "fr", "fr"},    // unsupported cookie ignored
		{"xx", "", "en-GB", "en"}, // unsupported account ignored
	} {
		if got := resolveLang(c.account, c.cookie, c.accept); got != c.want {
			t.Errorf("resolveLang(%q, %q, %q) = %q, want %q", c.account, c.cookie, c.accept, got, c.want)
		}
	}
}

func TestNum(t *testing.T) {
	if got := num("fr", 3.44); got != "3,4" {
		t.Errorf("num fr = %q", got)
	}
	if got := num("en", 3.44); got != "3.4" {
		t.Errorf("num en = %q", got)
	}
}

// messageIDs flattens a locale file to its dotted message IDs.
func messageIDs(t *testing.T, lang string) []string {
	t.Helper()
	var m map[string]any
	if _, err := toml.DecodeFS(assets, "locales/"+lang+".toml", &m); err != nil {
		t.Fatal(err)
	}
	var ids []string
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for k, v := range m {
			if sub, ok := v.(map[string]any); ok {
				walk(prefix+k+".", sub)
			} else {
				ids = append(ids, prefix+k)
			}
		}
	}
	walk("", m)
	slices.Sort(ids)
	return ids
}

func TestLocalesComplete(t *testing.T) {
	en, fr := messageIDs(t, "en"), messageIDs(t, "fr")
	if !slices.Equal(en, fr) {
		t.Fatalf("en and fr message IDs differ:\nen only: %v\nfr only: %v", missing(en, fr), missing(fr, en))
	}
	for _, id := range en {
		for _, lang := range []string{"en", "fr"} {
			if got := tr(lang, id, "Game", "X", "N", 2); got == id {
				t.Errorf("%s: %q does not translate", lang, id)
			}
		}
	}
}

func missing(a, b []string) []string {
	var out []string
	for _, x := range a {
		if !slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// Every ID used in a template ({{t "id"}}) or in Go code (msg constants
// passed to tr, message and friends: any "word.word" string literal) must exist.
func TestMessageIDsUsedExist(t *testing.T) {
	known := map[string]bool{}
	for _, id := range messageIDs(t, "en") {
		known[id] = true
	}
	used := map[string]string{}
	tmplRe := regexp.MustCompile(`\bt "([a-z0-9_.]+)"`)
	fs.WalkDir(assets, "templates", func(p string, d fs.DirEntry, _ error) error {
		if d.IsDir() {
			return nil
		}
		b, _ := fs.ReadFile(assets, p)
		for _, m := range tmplRe.FindAllStringSubmatch(string(b), -1) {
			used[m[1]] = p
		}
		return nil
	})
	if len(used) == 0 {
		t.Fatal("no t calls found in templates")
	}
	for _, id := range slices.Sorted(maps.Keys(used)) {
		if !known[id] {
			t.Errorf("%s uses unknown message %q", used[id], id)
		}
	}
	// Go code passes IDs as literals under these prefixes.
	goRe := regexp.MustCompile(`"((?:err|msg|score|nav)\.[a-z0-9_.]+)"`)
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range goRe.FindAllStringSubmatch(string(b), -1) {
			if !known[m[1]] {
				t.Errorf("%s uses unknown message %q", f, m[1])
			}
		}
	}
	// Tag labels are looked up as tag.<tag>.
	for _, tag := range games.AllTags() {
		if !known["tag."+tag] {
			t.Errorf("no label tag.%s", tag)
		}
	}
}

func TestLangSwitch(t *testing.T) {
	ts, st := newTestServer(t)
	c := newClient(t)

	// Browser asks for French: the page is French.
	req, _ := http.NewRequest("GET", ts.URL+"/login", nil)
	req.Header.Set("Accept-Language", "fr-FR")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if body := readAll(t, resp); !strings.Contains(body, `<html lang="fr">`) {
		t.Fatalf("Accept-Language fr: page not French:\n%s", body)
	}

	resp, _ = post(t, c, ts.URL+"/lang", url.Values{"lang": {"de"}, "back": {"/login"}})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("lang=de: status %d, want 400", resp.StatusCode)
	}
	resp, _ = post(t, c, ts.URL+"/lang", url.Values{"lang": {"fr"}, "back": {"//evil.example"}})
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Fatalf("foreign back: Location %q, want /", loc)
	}
	resp, _ = post(t, c, ts.URL+"/lang", url.Values{"lang": {"fr"}, "back": {"/login?next=%2Fg%2F1"}})
	if loc := resp.Header.Get("Location"); loc != "/login?next=%2Fg%2F1" {
		t.Fatalf("Location %q, want back", loc)
	}
	if _, body := get(t, c, ts.URL+"/login"); !strings.Contains(body, `<html lang="fr">`) {
		t.Fatal("cookie fr: page not French")
	}

	// Logged in, the choice is saved on the account and follows to a new device.
	signup(t, ts, c, "alice")
	post(t, c, ts.URL+"/lang", url.Values{"lang": {"en"}, "back": {"/"}})
	u, err := st.Authenticate("alice", "correct horse")
	if err != nil || u.Lang != "en" {
		t.Fatalf("account lang = %q, %v; want en", u.Lang, err)
	}
	post(t, c, ts.URL+"/lang", url.Values{"lang": {"fr"}, "back": {"/"}})
	other := newClient(t)
	post(t, other, ts.URL+"/login", url.Values{"username": {"alice"}, "password": {"correct horse"}})
	if _, body := get(t, other, ts.URL+"/"); !strings.Contains(body, `<html lang="fr">`) {
		t.Fatal("account fr: new device not French")
	}
}

// Signup keeps the language in use, so a French visitor gets a French account.
func TestSignupKeepsLang(t *testing.T) {
	ts, st := newTestServer(t)
	c := newClient(t)
	post(t, c, ts.URL+"/lang", url.Values{"lang": {"fr"}, "back": {"/"}})
	signup(t, ts, c, "alice")
	if u, err := st.Authenticate("alice", "correct horse"); err != nil || u.Lang != "fr" {
		t.Fatalf("account lang = %q, %v; want fr", u.Lang, err)
	}
}

// A French player gets French pages end to end: validation errors, the
// duplicate message, tag labels and decimal commas.
func TestFrenchPages(t *testing.T) {
	ts, alice, _ := twoPlayers(t)
	post(t, alice, ts.URL+"/lang", url.Values{"lang": {"fr"}, "back": {"/"}})

	submit(t, ts, alice, tusmoAlice)
	_, body := post(t, alice, ts.URL+"/results", url.Values{"text": {tusmoAlice}, "back": {"/g/1"}})
	if !strings.Contains(body, "Déjà envoyé : Tusmo #70") {
		t.Fatalf("duplicate not French:\n%s", body)
	}
	if _, body := get(t, alice, ts.URL+"/g/1/leaderboard?tag=music"); !strings.Contains(body, `aria-current="page">musique</a>`) {
		t.Fatalf("tag label not French:\n%s", body)
	}

	c := newClient(t)
	post(t, c, ts.URL+"/lang", url.Values{"lang": {"fr"}, "back": {"/"}})
	_, body = post(t, c, ts.URL+"/signup", url.Values{"username": {"bob2"}, "password": {"short"}})
	if !strings.Contains(body, "au moins 10 caractères") {
		t.Fatalf("signup error not French:\n%s", body)
	}
}
