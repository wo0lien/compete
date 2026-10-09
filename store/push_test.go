package store

import (
	"testing"
	"time"
)

func TestPushStore(t *testing.T) {
	s := newStore(t)
	alice, bob := mustUser(t, s, "alice"), mustUser(t, s, "bob")

	if v, err := s.Setting("vapid"); v != "" || err != nil {
		t.Fatalf("missing setting = %q, %v", v, err)
	}
	s.SetSetting("vapid", "k1")
	s.SetSetting("vapid", "k2")
	if v, _ := s.Setting("vapid"); v != "k2" {
		t.Fatalf("setting = %q, want k2", v)
	}

	phone := Subscription{Endpoint: "https://push.example/a", P256dh: "p", Auth: "a", UserID: alice.ID}
	laptop := Subscription{Endpoint: "https://push.example/b", P256dh: "p", Auth: "a", UserID: alice.ID}
	s.SaveSubscription(phone)
	s.SaveSubscription(laptop)
	r, _ := s.Recipient(alice.ID)
	if len(r.Subs) != 2 || r.Prefs != (Prefs{Digest: true, Reminder: true, Social: true}) {
		t.Fatalf("recipient = %+v, want 2 subs and default prefs", r)
	}
	if ids, _ := s.PushUsers(); len(ids) != 1 || ids[0] != alice.ID {
		t.Fatalf("PushUsers = %v, want [alice]", ids)
	}

	// the same device subscribing under bob moves to bob
	phone.UserID = bob.ID
	s.SaveSubscription(phone)
	if r, _ := s.Recipient(alice.ID); len(r.Subs) != 1 {
		t.Fatalf("alice subs = %d, want 1 after the phone moved", len(r.Subs))
	}
	s.DeleteUserSubscription(alice.ID, phone.Endpoint) // not alice's any more: no-op
	if r, _ := s.Recipient(bob.ID); len(r.Subs) != 1 {
		t.Fatal("alice could delete bob's subscription")
	}
	s.DeleteSubscription(phone.Endpoint)
	if r, _ := s.Recipient(bob.ID); len(r.Subs) != 0 {
		t.Fatal("DeleteSubscription left the row")
	}

	s.SetNotifyPrefs(alice.ID, Prefs{Friends: true})
	if p, _ := s.NotifyPrefs(alice.ID); p != (Prefs{Friends: true}) {
		t.Fatalf("prefs = %+v", p)
	}

	if first, _ := s.MarkSent(alice.ID, "digest:2026-10-06"); !first {
		t.Fatal("first MarkSent should be first")
	}
	if first, _ := s.MarkSent(alice.ID, "digest:2026-10-06"); first {
		t.Fatal("second MarkSent should not be first")
	}
	s.db.Exec("UPDATE push_sent SET sent_at = ?", fixtureDay.Add(-8*24*time.Hour).Unix())
	s.PruneSent(fixtureDay.Add(-7 * 24 * time.Hour))
	if first, _ := s.MarkSent(alice.ID, "digest:2026-10-06"); !first {
		t.Fatal("pruned key should be sendable again")
	}

	// account deletion cascades
	if err := s.DeleteUser(alice.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow("SELECT (SELECT COUNT(*) FROM push_subscriptions WHERE user_id = ?) + (SELECT COUNT(*) FROM push_sent WHERE user_id = ?)", alice.ID, alice.ID).Scan(&n)
	if n != 0 {
		t.Fatalf("%d push rows left after deleting alice", n)
	}
}
