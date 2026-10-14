package store

import (
	"errors"
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
