package store

import "testing"

func TestLeaderboardCountsClosedPuzzlesOnly(t *testing.T) { // Review Focus 5
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(2), nil), res("tusmo", "", 71, nil, nil), res("tusmo", "", 72, new(1), nil))
	c.add(t, c.bob, res("tusmo", "", 70, new(3), nil), res("tusmo", "", 71, new(4), nil))
	c.add(t, c.dave, res("tusmo", "", 70, new(1), nil))         // outsider: must not steal alice's win
	c.add(t, c.carol, res("songless", "All", 404, new(1), nil)) // only an open puzzle: no stats yet

	got, err := c.s.Leaderboard(c.g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Game != "tusmo" {
		t.Fatalf("Leaderboard = %+v, want only tusmo", got)
	}
	st := got[0].Stats
	if len(st) != 2 {
		t.Fatalf("stats = %+v, want alice and bob", st)
	}
	// both have 1 win; alice's average (2.0) beats bob's (3.5). #72 is open: excluded.
	a, b := st[0], st[1]
	if a.Username != "alice" || a.Plays != 2 || a.Wins != 1 || a.Fails != 1 || a.AvgScore.Float64 != 2 {
		t.Fatalf("alice = %+v", a)
	}
	if b.Username != "bob" || b.Plays != 2 || b.Wins != 1 || b.Fails != 0 || b.AvgScore.Float64 != 3.5 {
		t.Fatalf("bob = %+v", b)
	}
	if a.WinRate() != 50 {
		t.Fatalf("WinRate = %d, want 50", a.WinRate())
	}
}
