package store

import (
	"reflect"
	"testing"
	"time"
)

func through(t *testing.T, s *Store, groupID int64) string {
	t.Helper()
	var d string
	if err := s.db.QueryRow("SELECT badges_through FROM groups WHERE id = ?", groupID).Scan(&d); err != nil {
		t.Fatal(err)
	}
	return d
}

// A new group evaluates from its creation day: nothing before it.
func TestCreateGroupSeedsBadgesThrough(t *testing.T) {
	c := newCrew(t) // fixtureDay 2026-10-06
	if got := through(t, c.s, c.g.ID); got != "2026-10-05" {
		t.Fatalf("badges_through = %q, want 2026-10-05", got)
	}
}

// Review Focus 4: a rejoin starts with everything seen.
func TestRejoinResetsSeen(t *testing.T) {
	c := newCrew(t)
	c.s.RemoveMember(c.g.ID, c.bob.ID)
	c.s.Now = func() time.Time { return day(2) }
	c.s.JoinByCode(c.bob.ID, c.g.InviteCode)
	var seen int64
	c.s.db.QueryRow("SELECT badges_seen_at FROM memberships WHERE group_id = ? AND user_id = ?", c.g.ID, c.bob.ID).Scan(&seen)
	if seen != day(2).Unix() {
		t.Fatalf("badges_seen_at = %d, want %d", seen, day(2).Unix())
	}
}

// plays builds one day's puzzle with the given players.
func plays(day, game string, users ...int64) puzzlePlays {
	p := puzzlePlays{game: game, day: day}
	for _, u := range users {
		p.entries = append(p.entries, entry{user: u})
	}
	return p
}

// Exactly one member played that day, and someone else could have.
func TestLonelyDays(t *testing.T) {
	trio := map[int64]string{1: "2026-10-01", 2: "2026-10-01", 3: "2026-10-01"}
	solo := map[int64]string{1: "2026-10-01"}
	late := map[int64]string{1: "2026-10-01", 4: "2026-10-06"}
	days := []string{"2026-10-05", "2026-10-06", "2026-10-07"}
	for _, c := range []struct {
		name   string
		ps     []puzzlePlays
		joined map[int64]string
		want   map[string]int64
	}{
		{"one player on two games", []puzzlePlays{plays("2026-10-05", "tusmo", 1), plays("2026-10-05", "songless", 1)}, trio, map[string]int64{"2026-10-05": 1}},
		{"two players", []puzzlePlays{plays("2026-10-05", "tusmo", 1), plays("2026-10-05", "songless", 2)}, trio, map[string]int64{}},
		{"nobody", nil, trio, map[string]int64{}},
		{"solo group", []puzzlePlays{plays("2026-10-05", "tusmo", 1)}, solo, map[string]int64{}},
		{"other member joined that day", []puzzlePlays{plays("2026-10-06", "tusmo", 1)}, late, map[string]int64{}},
		{"joiner counts the next day", []puzzlePlays{plays("2026-10-07", "tusmo", 1)}, late, map[string]int64{"2026-10-07": 1}},
		{"day outside days", []puzzlePlays{plays("2026-10-04", "tusmo", 1)}, trio, map[string]int64{}},
		{"player who left", []puzzlePlays{plays("2026-10-05", "tusmo", 9, 1)}, trio, map[string]int64{"2026-10-05": 1}},
	} {
		if got := lonelyDays(c.ps, c.joined, days); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: lonelyDays = %v, want %v", c.name, got, c.want)
		}
	}
}
