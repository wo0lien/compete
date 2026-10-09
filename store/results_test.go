package store

import (
	"errors"
	"testing"

	"github.com/wo0lien/compete/games"
)

func res(game, variant string, puzzle int, score, tiebreak *int) games.Result {
	return games.Result{Game: game, Variant: variant, PuzzleID: puzzle, Score: score, Tiebreak: tiebreak,
		Detail: map[string]any{"grid": "🟩"}}
}

// crew is a group of alice (owner), bob and carol, plus dave who is outside it.
type crew struct {
	s                       *Store
	g                       Group
	alice, bob, carol, dave User
}

func newCrew(t *testing.T) crew {
	t.Helper()
	s := newStore(t)
	c := crew{s: s, alice: mustUser(t, s, "alice"), bob: mustUser(t, s, "bob"),
		carol: mustUser(t, s, "carol"), dave: mustUser(t, s, "dave")}
	c.g, _ = s.CreateGroup(c.alice.ID, "Friends")
	s.JoinByCode(c.bob.ID, c.g.InviteCode)
	s.JoinByCode(c.carol.ID, c.g.InviteCode)
	return c
}

func (c crew) add(t *testing.T, u User, rs ...games.Result) {
	t.Helper()
	if _, err := c.s.AddResults(u.ID, "raw", rs); err != nil {
		t.Fatal(err)
	}
}

func names(b Board) []string {
	var out []string
	for _, e := range b.Entries {
		out = append(out, e.Username)
	}
	return out
}

func TestAddResultsSkipsDuplicates(t *testing.T) { // Review Focus 2
	c := newCrew(t)
	all, rock, hiphop := res("songless", "All", 404, new(3), nil), res("songless", "Rock", 404, new(2), nil), res("songless", "Hip Hop", 404, nil, nil)
	if added, err := c.s.AddResults(c.alice.ID, "raw", []games.Result{all, rock}); err != nil || len(added) != 2 {
		t.Fatalf("first share: added %d, err %v", len(added), err)
	}
	added, err := c.s.AddResults(c.alice.ID, "raw", []games.Result{all, rock, hiphop})
	if err != nil || len(added) != 1 || added[0].Variant != "Hip Hop" {
		t.Fatalf("re-share: added %+v, err %v; want only Hip Hop", added, err)
	}
	if _, err := c.s.AddResults(c.alice.ID, "raw", []games.Result{all}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("full duplicate err = %v, want ErrDuplicate", err)
	}
}

func TestBoardHiddenUntilViewerSubmits(t *testing.T) { // Review Focus 3
	c := newCrew(t)
	c.add(t, c.carol, res("tusmo", "", 70, new(2), new(30)))
	c.add(t, c.bob, res("tusmo", "", 70, new(4), new(30)))
	c.add(t, c.dave, res("tusmo", "", 70, new(1), new(5))) // outsider

	b, err := c.s.Board(c.g.ID, c.alice.ID, "tusmo", "", 70)
	if err != nil {
		t.Fatal(err)
	}
	if b.Revealed {
		t.Fatal("board revealed before alice submitted")
	}
	// played members alphabetically (not by rank), then non-players; dave absent
	if got := names(b); len(got) != 3 || got[0] != "bob" || got[1] != "carol" || got[2] != "alice" {
		t.Fatalf("hidden order = %v, want [bob carol alice]", got)
	}
	for _, e := range b.Entries[:2] {
		if !e.Hidden || e.Score != nil || e.Tiebreak != nil || e.Grid != "" {
			t.Fatalf("hidden entry leaks data: %+v", e)
		}
	}

	c.add(t, c.alice, res("tusmo", "", 70, new(3), new(10)))
	b, _ = c.s.Board(c.g.ID, c.alice.ID, "tusmo", "", 70)
	if !b.Revealed {
		t.Fatal("board still hidden after alice submitted")
	}
	if got := names(b); got[0] != "carol" || got[1] != "alice" || got[2] != "bob" {
		t.Fatalf("revealed order = %v, want [carol alice bob]", got)
	}
	if e := b.Entries[0]; e.Hidden || e.Score == nil || *e.Score != 2 || e.Grid != "🟩" {
		t.Fatalf("revealed entry = %+v", e)
	}
}

func TestBoardOrdersTiebreakThenFails(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), new(40)))
	c.add(t, c.bob, res("tusmo", "", 70, new(3), new(20)))
	c.add(t, c.carol, res("tusmo", "", 70, nil, new(5)))
	b, _ := c.s.Board(c.g.ID, c.alice.ID, "tusmo", "", 70)
	if got := names(b); got[0] != "bob" || got[1] != "alice" || got[2] != "carol" {
		t.Fatalf("order = %v, want [bob alice carol]", got)
	}
}

func TestRevealIsPerVariant(t *testing.T) { // Review Focus 4
	c := newCrew(t)
	c.add(t, c.bob, res("songless", "All", 404, new(1), nil), res("songless", "Rock", 404, new(1), nil))
	c.add(t, c.alice, res("songless", "Rock", 404, new(2), nil))
	rock, _ := c.s.Board(c.g.ID, c.alice.ID, "songless", "Rock", 404)
	all, _ := c.s.Board(c.g.ID, c.alice.ID, "songless", "All", 404)
	if !rock.Revealed || all.Revealed {
		t.Fatalf("Rock revealed=%v (want true), All revealed=%v (want false)", rock.Revealed, all.Revealed)
	}
}

func TestCurrentBoardsAndPuzzles(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.bob, res("tusmo", "", 70, new(3), nil))
	c.add(t, c.carol, res("tusmo", "", 71, new(3), nil))
	c.add(t, c.bob, res("travle", "usa", 12, new(0), nil))
	c.add(t, c.dave, res("tusmo", "", 99, new(1), nil)) // outsider must not move "current"
	bs, err := c.s.CurrentBoards(c.g.ID, c.alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 2 || bs[0].Game != "travle" || bs[0].Variant != "usa" || bs[1].Game != "tusmo" || bs[1].PuzzleID != 71 {
		t.Fatalf("CurrentBoards = %+v", bs)
	}
	ps, _ := c.s.Puzzles(c.g.ID, "tusmo", "")
	if len(ps) != 2 || ps[0] != 71 || ps[1] != 70 {
		t.Fatalf("Puzzles = %v, want [71 70]", ps)
	}
}

// Ranks follow the leaderboard's RANK(): equal score and tiebreak share a rank,
// the next one skips (1, 1, 3); members who did not play have none.
func TestBoardRanksTies(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), new(20)))
	c.add(t, c.bob, res("tusmo", "", 70, new(3), new(20)))
	c.add(t, c.alice, res("tusmo", "", 71, new(3), new(40)))
	c.add(t, c.bob, res("tusmo", "", 71, new(3), new(20)))
	c.add(t, c.carol, res("tusmo", "", 71, nil, new(5)))
	for _, tc := range []struct {
		puzzle int
		want   map[string]int
	}{
		{70, map[string]int{"alice": 1, "bob": 1, "carol": 0}},
		{71, map[string]int{"bob": 1, "alice": 2, "carol": 3}},
	} {
		b, _ := c.s.Board(c.g.ID, c.alice.ID, "tusmo", "", tc.puzzle)
		for _, e := range b.Entries {
			if e.Rank != tc.want[e.Username] {
				t.Errorf("puzzle %d: %s rank %d, want %d", tc.puzzle, e.Username, e.Rank, tc.want[e.Username])
			}
		}
	}
}

func TestAddResultsEmpty(t *testing.T) {
	c := newCrew(t)
	if _, err := c.s.AddResults(c.alice.ID, "raw", nil); !errors.Is(err, ErrNoResults) {
		t.Fatalf("AddResults(nil) = %v, want ErrNoResults", err)
	}
}
