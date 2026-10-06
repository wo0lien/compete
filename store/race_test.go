package store

import "testing"

func TestRaceCountsClosedWinsAcrossGames(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(2), nil))
	c.add(t, c.bob, res("tusmo", "", 70, new(3), nil), res("tusmo", "", 71, new(1), nil)) // 71 is open
	c.add(t, c.bob, res("songless", "All", 404, new(1), nil))
	c.add(t, c.carol, res("songless", "All", 404, new(2), nil), res("songless", "All", 405, new(1), nil))
	c.add(t, c.dave, res("tusmo", "", 70, new(1), nil)) // outsider

	all, err := c.s.Race(c.g.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name string
		wins int
	}{{"alice", 1}, {"bob", 1}, {"carol", 0}}
	if len(all) != len(want) {
		t.Fatalf("Race = %+v", all)
	}
	for i, w := range want {
		if all[i].Username != w.name || all[i].Wins != w.wins || all[i].Pawn == "" {
			t.Fatalf("Race[%d] = %+v, want %s with %d wins", i, all[i], w.name, w.wins)
		}
	}

	music, _ := c.s.Race(c.g.ID, []string{"songless"})
	if music[0].Username != "bob" || music[0].Wins != 1 || music[1].Wins != 0 {
		t.Fatalf("Race(songless) = %+v, want bob first with 1 win", music)
	}
}

func TestBoardMarksViewerAndPawns(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(2), nil))
	b, _ := c.s.Board(c.g.ID, c.alice.ID, "tusmo", "", 70)
	if !b.Entries[0].Me || b.Entries[0].Pawn == "" || b.Entries[1].Me {
		t.Fatalf("entries = %+v", b.Entries)
	}
	ms, _ := c.s.Members(c.g.ID)
	if ms[0].Pawn == "" {
		t.Fatalf("member pawn empty: %+v", ms[0])
	}
}
