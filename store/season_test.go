package store

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// N = ceil(days × games × (1 + 3/4 × (members − 1))), at least 1.
func TestSeasonN(t *testing.T) {
	for _, c := range []struct{ days, games, members, want int }{
		{7, 3, 3, 53}, // spec example
		{7, 1, 3, 18},
		{3, 1, 3, 8},
		{7, 2, 1, 14}, // solo: perfect play for the whole length
		{14, 1, 2, 25},
		{7, 0, 3, 1}, // no games: still a finish line
	} {
		if got := seasonN(c.days, c.games, c.members); got != c.want {
			t.Errorf("seasonN(%d, %d, %d) = %d, want %d", c.days, c.games, c.members, got, c.want)
		}
	}
}

func TestSeasonDays(t *testing.T) {
	c := newCrew(t)
	if d, err := c.s.SeasonDays(c.g.ID); err != nil || d != 7 {
		t.Fatalf("default = %d, %v; want 7", d, err)
	}
	if err := c.s.SetSeasonDays(c.g.ID, 14); err != nil {
		t.Fatal(err)
	}
	if d, _ := c.s.SeasonDays(c.g.ID); d != 14 {
		t.Fatalf("after set = %d, want 14", d)
	}
	if err := c.s.SetSeasonDays(c.g.ID, 5); !errors.Is(err, ErrSeasonDays) {
		t.Fatalf("set 5 = %v, want ErrSeasonDays", err)
	}
}

// 1 for playing + 1 per member strictly beaten; non-players are beaten.
func TestPoints(t *testing.T) {
	for _, c := range []struct {
		name    string
		ranks   map[int64]int
		members int
		want    map[int64]int
	}{
		{"spec: alice 2/6, bob 4/6, carol absent", map[int64]int{1: 1, 2: 2}, 3, map[int64]int{1: 3, 2: 2}},
		{"tie beats nobody between them", map[int64]int{1: 1, 2: 1}, 3, map[int64]int{1: 2, 2: 2}},
		{"everyone played, fail last", map[int64]int{1: 1, 2: 2, 3: 3}, 3, map[int64]int{1: 3, 2: 2, 3: 1}},
		{"alone in a trio", map[int64]int{1: 1}, 3, map[int64]int{1: 3}},
		{"solo group", map[int64]int{1: 1}, 1, map[int64]int{1: 1}},
	} {
		if got := points(c.ranks, c.members); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: points = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestWalk(t *testing.T) {
	ps := []scored{
		{"2026-10-07", map[int64]int{1: 3, 2: 1}},
		{"2026-10-06", map[int64]int{1: 3, 2: 2}},
		{"2026-10-08", map[int64]int{1: 3, 2: 2}}, // day 3: alice reaches 9
		{"2026-10-08", map[int64]int{2: 7}},       // same day: bob overtakes, 12
		{"2026-10-09", map[int64]int{1: 3}},       // after the final day: next season
	}
	totals, final := walk(ps, 8)
	if final != "2026-10-08" || !reflect.DeepEqual(totals, map[int64]int{1: 9, 2: 12}) {
		t.Fatalf("walk = %v, %q; want {1:9 2:12}, 2026-10-08", totals, final)
	}
	if got := winners(totals); !reflect.DeepEqual(got, []int64{2}) {
		t.Fatalf("winners = %v, want bob (overtook on the final day)", got)
	}
	if _, final := walk(ps[:2], 8); final != "" {
		t.Fatalf("final = %q before anyone reaches N", final)
	}
	if got := winners(map[int64]int{3: 8, 1: 8, 2: 5}); !reflect.DeepEqual(got, []int64{1, 3}) {
		t.Fatalf("co-winners = %v, want [1 3]", got)
	}
}

// day d of the fixture week at noon UTC: day(0) = 2026-10-06, Tusmo #70.
func day(d int) time.Time { return fixtureDay.Add(time.Duration(d) * 24 * time.Hour) }

func points3(t *testing.T, se Season) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, r := range se.Racers {
		out[r.Username] = r.Points
	}
	return out
}

func mustSeason(t *testing.T, c crew) Season {
	t.Helper()
	se, err := c.s.CurrentSeason(c.g.ID)
	if err != nil {
		t.Fatal(err)
	}
	return se
}

// Season 1 starts on the first read; an open puzzle counts once everyone
// played it, off-list games and outsiders never count.
func TestSeasonCounting(t *testing.T) {
	c := newCrew(t)
	se := mustSeason(t, c)
	if se.Number != 1 || se.Start != "2026-10-06" || se.N != 18 || len(se.Racers) != 3 {
		t.Fatalf("first read = %+v, want season 1 from 2026-10-06, N 18, 3 racers", se)
	}
	c.add(t, c.alice, res("tusmo", "", 70, new(2), nil), res("songless", "All", 404, new(1), nil))
	c.add(t, c.bob, res("tusmo", "", 70, new(4), nil))
	c.add(t, c.dave, res("tusmo", "", 70, new(1), nil))
	if got := points3(t, mustSeason(t, c)); got["alice"] != 0 || got["bob"] != 0 {
		t.Fatalf("open #70, carol to play: %v, want nothing counted", got)
	}
	c.add(t, c.carol, res("tusmo", "", 70, nil, nil)) // fail: everyone played
	se = mustSeason(t, c)
	if got := points3(t, se); !reflect.DeepEqual(got, map[string]int{"alice": 3, "bob": 2, "carol": 1}) {
		t.Fatalf("all played #70: %v", got)
	}
	if se.Racers[0].Username != "alice" || se.Racers[0].Pawn == "" {
		t.Fatalf("racers not sorted by points: %+v", se.Racers)
	}
	c.add(t, c.alice, res("tusmo", "", 71, new(3), nil))
	c.s.Now = func() time.Time { return day(2) } // #71 closed
	if got := points3(t, mustSeason(t, c)); got["alice"] != 6 {
		t.Fatalf("closed #71 alone: alice %d, want 3+3", got["alice"])
	}
}

// Review Focus 1: a new member starts at 0 and must play open puzzles before they count early.
func TestSeasonJoinMidSeason(t *testing.T) {
	c := newCrew(t)
	mustSeason(t, c)
	for _, u := range []User{c.alice, c.bob, c.carol} {
		c.add(t, u, res("tusmo", "", 70, new(3), nil))
	}
	c.s.JoinByCode(c.dave.ID, c.g.InviteCode)
	se := mustSeason(t, c)
	if got := points3(t, se); got["dave"] != 0 || got["alice"] != 0 {
		t.Fatalf("dave joined before #70 closed: %v, want #70 waiting for dave", got)
	}
	c.s.Now = func() time.Time { return day(1) }
	if got := points3(t, mustSeason(t, c)); got["alice"] != 2 || got["dave"] != 0 {
		t.Fatalf("closed #70: %v, want 1 + dave beaten for each player", got)
	}
}

// Review Focus 2: removing a game drops its points; N stays frozen.
func TestSeasonGamesChangeKeepsN(t *testing.T) {
	c := newCrew(t)
	if err := c.s.SetGroupGames(c.g.ID, []string{"tusmo", "songless"}); err != nil {
		t.Fatal(err)
	}
	n := mustSeason(t, c).N
	c.add(t, c.alice, res("songless", "All", 404, new(1), nil))
	c.s.Now = func() time.Time { return day(1) }
	if got := points3(t, mustSeason(t, c)); got["alice"] != 3 {
		t.Fatalf("songless counted: alice %d, want 3", got["alice"])
	}
	c.s.SetGroupGames(c.g.ID, []string{"tusmo"})
	se := mustSeason(t, c)
	if se.N != n || points3(t, se)["alice"] != 0 {
		t.Fatalf("after removing songless: N %d (want %d), alice %d (want 0)", se.N, n, points3(t, se)["alice"])
	}
}

// A season runs to its final day, is decided once that day is over, freezes
// its co-winners, and the next one starts the day after.
func TestSeasonLifecycle(t *testing.T) {
	c := newCrew(t)
	c.s.SetSeasonDays(c.g.ID, 3) // N = 8
	mustSeason(t, c)
	// Review Focus 5: a game variant without a schedule never counts nor blocks.
	c.s.SetGroupGames(c.g.ID, []string{"tusmo", "travle"})
	c.add(t, c.alice, res("travle", "challenge", 12, new(0), nil))
	for id := 70; id <= 73; id++ { // alice and bob tie every day, carol fails: 2 each a day
		c.s.Now = func() time.Time { return day(id - 70) } // AddResults refuses puzzles not out yet
		c.add(t, c.alice, res("tusmo", "", id, new(3), nil))
		c.add(t, c.bob, res("tusmo", "", id, new(3), nil))
		if id != 72 { // carol skips #72 (still beaten), and pastes it late below
			c.add(t, c.carol, res("tusmo", "", id, nil, nil))
		}
	}
	c.s.Now = func() time.Time { return day(3) } // 2026-10-09: #73 played by all, day not over
	se := mustSeason(t, c)
	if se.Number != 1 || se.FinalDay != "2026-10-09" || points3(t, se)["alice"] != 8 {
		t.Fatalf("final day = %+v, want season 1 on its final day 2026-10-09 with alice 8", se)
	}
	c.s.Now = func() time.Time { return day(4) }
	c.add(t, c.alice, res("tusmo", "", 74, new(1), nil)) // the next day, alone
	c.s.Now = func() time.Time { return day(5) }         // 2026-10-11: #74 closed, 2026-10-09 over
	se = mustSeason(t, c)
	if se.Number != 2 || se.Start != "2026-10-10" || se.FinalDay != "" || se.N != seasonN(3, 2, 3) {
		t.Fatalf("after the final day = %+v, want season 2 from 2026-10-10", se)
	}
	if want := []Winner{{1, []string{"alice", "bob"}}}; !reflect.DeepEqual(se.Past, want) {
		t.Fatalf("past = %+v, want %+v", se.Past, want)
	}
	if got := points3(t, se); got["alice"] != 3 {
		t.Fatalf("#74 carries into season 2: alice %d, want 3", got["alice"])
	}
	// Review Focus 3: a late paste for a frozen day changes nothing.
	c.add(t, c.carol, res("tusmo", "", 72, new(1), nil))
	if again := mustSeason(t, c); !reflect.DeepEqual(again.Past, se.Past) || points3(t, again)["carol"] != 0 {
		t.Fatalf("late paste moved things: %+v", again)
	}
}

// Review Focus 4: a season is closed once, however many readers race to close it.
func TestCloseSeasonOnce(t *testing.T) {
	c := newCrew(t)
	se := mustSeason(t, c)
	id := c.s.runningSeasonID(t, c.g.ID)
	for range 2 {
		if err := c.s.closeSeason(c.g.ID, id, se.Number, "2026-10-06", []int64{c.alice.ID}); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	c.s.db.QueryRow("SELECT COUNT(*) FROM seasons WHERE group_id = ?", c.g.ID).Scan(&n)
	if n != 2 {
		t.Fatalf("%d seasons after two closes, want 2", n)
	}
}

// runningSeasonID is a test hook: the id of the group's running season.
func (s *Store) runningSeasonID(t *testing.T, groupID int64) int64 {
	t.Helper()
	var id int64
	if err := s.db.QueryRow("SELECT id FROM seasons WHERE group_id = ? AND end_day IS NULL", groupID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
