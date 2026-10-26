package store

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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
	c.g, _ = s.CreateGroup(c.alice.ID, "Friends", []string{"tusmo"})
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
	// the viewer first, then played members alphabetically (not by rank), then non-players; dave absent
	if got := names(b); len(got) != 3 || got[0] != "alice" || got[1] != "bob" || got[2] != "carol" {
		t.Fatalf("hidden order = %v, want [alice bob carol]", got)
	}
	for _, e := range b.Entries[1:] {
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

// Travle's Perfect is stored as -1, so it ranks above a plain +0.
func TestBoardPerfectBeatsZero(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("travle", "challenge", 140, new(0), nil))
	c.add(t, c.bob, res("travle", "challenge", 140, new(-1), nil))
	b, _ := c.s.Board(c.g.ID, c.alice.ID, "travle", "challenge", 140)
	if got := names(b); got[0] != "bob" || got[1] != "alice" {
		t.Fatalf("order = %v, want [bob alice]", got)
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
	c.s.SetGroupGames(c.g.ID, []string{"tusmo", "travle"})
	c.add(t, c.bob, res("tusmo", "", 69, new(3), nil))
	c.add(t, c.carol, res("tusmo", "", 70, new(3), nil))
	c.add(t, c.bob, res("travle", "usa", 1205, new(0), nil))
	c.add(t, c.dave, res("tusmo", "", 71, new(1), nil))       // outsider must not move "current"
	c.add(t, c.bob, res("songless", "All", 404, new(3), nil)) // not a game of the group: no board
	bs, err := c.s.CurrentBoards(c.g.ID, c.alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 2 || bs[0].Game != "travle" || bs[0].Variant != "usa" || bs[1].Game != "tusmo" || bs[1].PuzzleID != 70 {
		t.Fatalf("CurrentBoards = %+v", bs)
	}
	ps, _ := c.s.History(c.g.ID, c.alice.ID, "tusmo", "")
	if len(ps) != 2 || ps[0].ID != 70 || ps[1].ID != 69 {
		t.Fatalf("History = %+v, want 70 then 69", ps)
	}
}

// History names each puzzle's winners (ties share, failures never win), only
// on puzzles the viewer played: the same rule as Board.
func TestHistoryWinners(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), new(20)))
	c.add(t, c.bob, res("tusmo", "", 70, new(3), new(20)))
	c.add(t, c.carol, res("tusmo", "", 70, nil, nil))
	c.add(t, c.bob, res("tusmo", "", 69, new(2), nil)) // alice did not play it
	c.add(t, c.alice, res("tusmo", "", 68, nil, nil))
	c.add(t, c.bob, res("tusmo", "", 68, nil, nil))
	c.add(t, c.dave, res("tusmo", "", 67, new(1), nil)) // outsider
	ps, err := c.s.History(c.g.ID, c.alice.ID, "tusmo", "")
	if err != nil {
		t.Fatal(err)
	}
	names := func(p Past) string {
		var out []string
		for _, w := range p.Winners {
			out = append(out, w.Username)
		}
		return strings.Join(out, ",")
	}
	got := fmt.Sprint(len(ps))
	for _, p := range ps {
		got += fmt.Sprintf(" #%d:%v:%s", p.ID, p.Revealed, names(p))
	}
	if want := "3 #70:true:alice,bob #69:false: #68:true:"; got != want {
		t.Fatalf("History = %q, want %q", got, want)
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

// The day after everyone played, today's puzzle is current even with no result
// yet; an older archive share does not move it back.
func TestCurrentBoardIsToday(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	c.s.Now = func() time.Time { return fixtureDay.Add(24 * time.Hour) } // Tusmo #71
	c.add(t, c.bob, res("tusmo", "", 60, new(2), nil))                   // from the archive
	bs, _ := c.s.CurrentBoards(c.g.ID, c.alice.ID)
	if len(bs) != 1 || bs[0].PuzzleID != 71 || bs[0].Revealed {
		t.Fatalf("CurrentBoards = %+v, want today's unrevealed Tusmo #71", bs)
	}
	for _, e := range bs[0].Entries {
		if e.Played {
			t.Fatalf("nobody played #71 yet: %+v", e)
		}
	}
}

// A player past midnight in their own timezone may share today+1; anything
// beyond is refused and never becomes current.
func TestFuturePuzzle(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 71, new(3), nil)) // today (#70) + 1
	if _, err := c.s.AddResults(c.bob.ID, "raw", []games.Result{res("tusmo", "", 72, new(3), nil)}); !errors.Is(err, ErrFuturePuzzle) {
		t.Fatalf("today+2 = %v, want ErrFuturePuzzle", err)
	}
	bs, _ := c.s.CurrentBoards(c.g.ID, c.alice.ID)
	if len(bs) != 1 || bs[0].PuzzleID != 71 {
		t.Fatalf("CurrentBoards = %+v, want #71", bs)
	}
}

// A variant without a schedule keeps "newest id seen".
func TestCurrentBoardUnscheduled(t *testing.T) {
	c := newCrew(t)
	c.s.SetGroupGames(c.g.ID, []string{"travle"})
	c.add(t, c.alice, res("travle", "challenge", 139, new(0), nil))
	c.add(t, c.bob, res("travle", "challenge", 140, new(1), nil))
	bs, _ := c.s.CurrentBoards(c.g.ID, c.alice.ID)
	if len(bs) != 1 || bs[0].PuzzleID != 140 {
		t.Fatalf("CurrentBoards = %+v, want challenge #140", bs)
	}
}

// A game stays "to play" until today's puzzle is submitted for every variant
// the member played recently; unscheduled variants and stale games are left out.
func TestToPlay(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 69, new(3), nil))            // yesterday
	c.add(t, c.alice, res("travle", "challenge", 140, new(0), nil)) // no schedule
	c.add(t, c.alice, res("songless", "Rock", 404, new(2), nil))    // today
	c.add(t, c.alice, res("songless", "All", 403, new(2), nil))     // yesterday
	got, err := c.s.ToPlay(c.alice.ID)
	if err != nil || !reflect.DeepEqual(got, []string{"tusmo", "songless"}) {
		t.Fatalf("ToPlay = %v, %v; want [tusmo songless]", got, err)
	}
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	c.add(t, c.alice, res("songless", "All", 404, new(4), nil))
	if got, _ := c.s.ToPlay(c.alice.ID); len(got) != 0 {
		t.Fatalf("after playing today: ToPlay = %v, want none", got)
	}
	// submitted_at is the real insert time: pin it, then move the clock 15 days on.
	c.s.db.Exec("UPDATE results SET submitted_at = ?", fixtureDay.Unix())
	c.s.Now = func() time.Time { return fixtureDay.Add(15 * 24 * time.Hour) }
	// her own games are stale; the group's Tusmo still counts
	if got, _ := c.s.ToPlay(c.alice.ID); !reflect.DeepEqual(got, []string{"tusmo"}) {
		t.Fatalf("stale games: ToPlay = %v, want only the group's [tusmo]", got)
	}
}

// An id stored before the future-puzzle guard existed (#4) must not stay current.
func TestCurrentBoardIgnoresStoredFutureIds(t *testing.T) {
	c := newCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	if _, err := c.s.db.Exec(`INSERT INTO results(user_id, game, variant, puzzle_id, score, raw) VALUES (?, 'tusmo', '', 99, 1, 'raw')`, c.bob.ID); err != nil {
		t.Fatal(err)
	}
	bs, _ := c.s.CurrentBoards(c.g.ID, c.alice.ID)
	if len(bs) != 1 || bs[0].PuzzleID != 70 {
		t.Fatalf("CurrentBoards = %+v, want today's #70, not the stored #99", bs)
	}
}

// Waiting row: the viewer first, then who played, then who hasn't (names A→Z).
func TestBoardWaitingViewerFirst(t *testing.T) {
	c := newCrew(t) // alice (viewer), bob, carol in the group
	c.add(t, c.carol, res("tusmo", "", 70, new(2), nil))
	b, err := c.s.Board(c.g.ID, c.alice.ID, "tusmo", "", 70)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(b); !reflect.DeepEqual(got, []string{"alice", "carol", "bob"}) {
		t.Fatalf("waiting order = %v, want [alice carol bob]", got)
	}
}

// Alone in a group: just the viewer.
func TestBoardWaitingAlone(t *testing.T) {
	s := newStore(t)
	u := mustUser(t, s, "solo")
	g, _ := s.CreateGroup(u.ID, "Me", []string{"tusmo"})
	b, err := s.Board(g.ID, u.ID, "tusmo", "", 70)
	if err != nil || len(b.Entries) != 1 || !b.Entries[0].Me {
		t.Fatalf("entries = %+v, err %v; want only the viewer", b.Entries, err)
	}
}

// PlayedRecently: any result in the last 14 days (the same window as ToPlay).
func TestPlayedRecently(t *testing.T) {
	c := newCrew(t)
	if ok, err := c.s.PlayedRecently(c.alice.ID); err != nil || ok {
		t.Fatalf("no results: PlayedRecently = %v, %v", ok, err)
	}
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	if ok, err := c.s.PlayedRecently(c.alice.ID); err != nil || !ok {
		t.Fatalf("after a result: PlayedRecently = %v, %v", ok, err)
	}
}

// Group games are suggested even if never played; any variant played today
// counts; one chip per game across groups (Review Focus 1, 3, 4).
func TestToPlayGroupGames(t *testing.T) {
	c := newCrew(t) // alice, bob, carol in "Friends", which plays tusmo
	other, _ := c.s.CreateGroup(c.bob.ID, "Other", []string{"tusmo", "travle"})
	c.s.JoinByCode(c.dave.ID, other.InviteCode)
	c.s.db.Exec(`INSERT INTO group_games(group_id, game) VALUES (?, 'gone')`, other.ID)

	// dave never played anything: both group games
	if got, _ := c.s.ToPlay(c.dave.ID); !reflect.DeepEqual(got, []string{"tusmo", "travle"}) {
		t.Fatalf("dave = %v, want [tusmo travle]", got)
	}
	// bob is in both groups: tusmo once
	if got, _ := c.s.ToPlay(c.bob.ID); !reflect.DeepEqual(got, []string{"tusmo", "travle"}) {
		t.Fatalf("bob = %v, want [tusmo travle]", got)
	}
	// dave plays today's Tusmo and today's French Travle map: nothing left
	travle, _ := games.ByID("travle")
	fra, _ := travle.Today("fra", fixtureDay)
	c.add(t, c.dave, res("tusmo", "", 70, new(3), nil), res("travle", "fra", fra, new(1), nil))
	if got, _ := c.s.ToPlay(c.dave.ID); len(got) != 0 {
		t.Fatalf("dave after playing = %v, want none", got)
	}
	// an own game outside any group still shows (alice: songless yesterday)
	c.add(t, c.alice, res("songless", "All", 403, new(2), nil))
	if got, _ := c.s.ToPlay(c.alice.ID); !reflect.DeepEqual(got, []string{"tusmo", "songless"}) {
		t.Fatalf("alice = %v, want [tusmo songless]", got)
	}
}
