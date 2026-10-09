package push

import (
	"reflect"
	"testing"
	"time"

	"github.com/wo0lien/compete/games"
	"github.com/wo0lien/compete/store"
)

type sent struct {
	endpoint string
	msg      Message
}

// capture replaces delivery and records what would be sent.
func capture(n *Notifier) *[]sent {
	var out []sent
	n.Send = func(sub store.Subscription, m Message) error { out = append(out, sent{sub.Endpoint, m}); return nil }
	return &out
}

func paris(clock string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", "2026-10-06 "+clock, parisLoc)
	if err != nil {
		panic(err)
	}
	return t
}

func TestTimedPushes(t *testing.T) {
	st := newTestStore(t)
	alice, _ := st.CreateUser("alice", "correct horse") // plays Tusmo, two devices
	bob, _ := st.CreateUser("bob", "correct horse")     // already played today
	carol, _ := st.CreateUser("carol", "correct horse") // digest switched off
	st.SetLang(alice.ID, "fr")
	for _, u := range []store.User{alice, bob, carol} {
		st.AddResults(u.ID, "raw", []games.Result{{Game: "tusmo", PuzzleID: 69, Score: new(3)}}) // yesterday
		st.SaveSubscription(store.Subscription{Endpoint: "https://push.example/" + u.Username, P256dh: "p", Auth: "a", UserID: u.ID})
	}
	st.SaveSubscription(store.Subscription{Endpoint: "https://push.example/alice-laptop", P256dh: "p", Auth: "a", UserID: alice.ID})
	st.AddResults(bob.ID, "raw", []games.Result{{Game: "tusmo", PuzzleID: 70, Score: new(2)}}) // today
	st.SetNotifyPrefs(carol.ID, store.Prefs{Reminder: true})

	n, _ := New(st, echoText, "https://example.org")
	got := capture(n)

	n.Tick(paris("07:59"))
	if len(*got) != 0 {
		t.Fatalf("07:59 sent %v", *got)
	}
	n.Tick(paris("08:00"))
	want := []sent{
		{"https://push.example/alice", Message{"compete", "fr:push.digest", "/"}},
		{"https://push.example/alice-laptop", Message{"compete", "fr:push.digest", "/"}},
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("08:00 sent %+v, want %+v", *got, want)
	}
	n.Tick(paris("08:30")) // same window: no repeat
	n.Tick(paris("10:01")) // window over
	if len(*got) != 2 {
		t.Fatalf("repeat or late digest: %+v", *got)
	}
	*got = nil
	n.Tick(paris("20:00"))
	if len(*got) != 3 || (*got)[2].endpoint != "https://push.example/carol" || (*got)[2].msg.Body != "en:push.reminder" {
		t.Fatalf("20:00 sent %+v, want alice ×2 and carol", *got)
	}
}
