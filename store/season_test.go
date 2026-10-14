package store

import (
	"errors"
	"reflect"
	"testing"
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
