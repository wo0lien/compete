package push

import (
	"testing"

	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

// crew: alice, bob, carol in "Les Potes", each subscribed once.
func crew(t *testing.T) (*store.Store, *Notifier, *[]sent, map[string]store.User) {
	t.Helper()
	st := newTestStore(t)
	us := map[string]store.User{}
	for _, name := range []string{"alice", "bob", "carol"} {
		u, _ := st.CreateUser(name, "correct horse")
		us[name] = u
		st.SaveSubscription(store.Subscription{Endpoint: "https://push.example/" + name, P256dh: "p", Auth: "a", UserID: u.ID})
	}
	g, _ := st.CreateGroup(us["alice"].ID, "Les Potes")
	st.JoinByCode(us["bob"].ID, g.InviteCode)
	st.JoinByCode(us["carol"].ID, g.InviteCode)
	n, _ := New(st, echoText, "https://example.org")
	return st, n, capture(n), us
}

func submitAs(t *testing.T, st *store.Store, n *Notifier, u store.User, rs ...games.Result) {
	t.Helper()
	added, err := st.AddResults(u.ID, "raw", rs)
	if err != nil {
		t.Fatal(err)
	}
	n.Submitted(u.ID, added)
}

func bodies(got []sent) map[string]string {
	m := map[string]string{}
	for _, s := range got {
		m[s.endpoint[len("https://push.example/"):]] = s.msg.Body
	}
	return m
}

func TestSubmittedPushes(t *testing.T) {
	st, n, got, us := crew(t)
	st.SetNotifyPrefs(us["carol"].ID, store.Prefs{Friends: true}) // social off, friends on

	submitAs(t, st, n, us["alice"], games.Result{Game: "tusmo", PuzzleID: 70, Score: new(4)})
	if b := bodies(*got); len(b) != 1 || b["carol"] != "en:push.friend" {
		t.Fatalf("first submit: %v, want only carol's friend ping (bob has friends off)", b)
	}

	*got = nil
	submitAs(t, st, n, us["bob"], games.Result{Game: "tusmo", PuzzleID: 70, Score: new(2)})
	if b := bodies(*got); len(b) != 2 || b["alice"] != "en:push.overtaken" || b["carol"] != "en:push.friend" {
		t.Fatalf("bob beats alice: %v, want alice overtaken, carol friend", b)
	}

	*got = nil
	submitAs(t, st, n, us["carol"], games.Result{Game: "tusmo", PuzzleID: 70, Score: new(5)})
	if b := bodies(*got); len(b) != 2 || b["alice"] != "en:push.everyone" || b["bob"] != "en:push.everyone" {
		t.Fatalf("last member played: %v, want everyone-played to alice and bob", b)
	}
}

// Several Songless categories in one paste: still one push per member.
func TestSubmittedOnePushPerMember(t *testing.T) {
	st, n, got, us := crew(t)
	st.SetNotifyPrefs(us["bob"].ID, store.Prefs{Friends: true})
	submitAs(t, st, n, us["alice"],
		games.Result{Game: "songless", Variant: "All", PuzzleID: 404, Score: new(2)},
		games.Result{Game: "songless", Variant: "Rock", PuzzleID: 404, Score: new(3)})
	if len(*got) != 1 || (*got)[0].endpoint != "https://push.example/bob" {
		t.Fatalf("sent %+v, want one ping to bob", *got)
	}
}
