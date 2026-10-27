package store

import (
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
