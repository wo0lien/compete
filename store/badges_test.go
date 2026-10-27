package store

import (
	"errors"
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

// oldCrew is newCrew whose members joined days ago: on the group's first day
// (its creation day) nobody counts yet, per the join rule.
func oldCrew(t *testing.T) crew {
	t.Helper()
	c := newCrew(t)
	if _, err := c.s.db.Exec("UPDATE memberships SET joined_at = joined_at - 3*86400"); err != nil {
		t.Fatal(err)
	}
	return c
}

func award(t *testing.T, c crew) {
	t.Helper()
	if err := c.s.AwardBadges(c.g.ID); err != nil {
		t.Fatal(err)
	}
}

func badgeDays(t *testing.T, c crew, u User) []string {
	t.Helper()
	bs, err := c.s.BadgesOf(c.g.ID, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, b := range bs {
		out = append(out, b.Day)
	}
	return out
}

func TestAwardLonely(t *testing.T) {
	c := oldCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	award(t, c)
	if got := badgeDays(t, c, c.alice); got != nil {
		t.Fatalf("day 0 still running: badges %v, want none", got)
	}
	c.s.Now = func() time.Time { return day(1) }
	award(t, c)
	bs, _ := c.s.BadgesOf(c.g.ID, c.alice.ID)
	if len(bs) != 1 || bs[0].Day != "2026-10-06" || bs[0].Kind != KindLonely || len(bs[0].Token) < 16 {
		t.Fatalf("alice badges = %+v, want one lonely for 2026-10-06 with a token", bs)
	}
	if got := badgeDays(t, c, c.bob); got != nil {
		t.Fatalf("bob badges = %v, want none", got)
	}
	if got := through(t, c.s, c.g.ID); got != "2026-10-06" {
		t.Fatalf("badges_through = %q, want 2026-10-06", got)
	}
}

// Review Focus 5: Songless resets at midnight New York, hours after Paris.
func TestAwardWaitsForDayOver(t *testing.T) {
	c := oldCrew(t)
	c.s.SetGroupGames(c.g.ID, []string{"tusmo", "songless"})
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil), res("songless", "All", 404, new(2), nil))
	c.s.Now = func() time.Time { return day(1).Add(-11 * time.Hour) } // 01:00 UTC
	award(t, c)
	if got := badgeDays(t, c, c.alice); got != nil {
		t.Fatalf("songless day not over: badges %v, want none", got)
	}
	c.s.Now = func() time.Time { return day(1) }
	award(t, c)
	if got := badgeDays(t, c, c.alice); !reflect.DeepEqual(got, []string{"2026-10-06"}) {
		t.Fatalf("badges = %v, want [2026-10-06]", got)
	}
}

// Review Focus 1: nobody opened the group for days.
func TestAwardCatchesUpSeveralDays(t *testing.T) {
	c := oldCrew(t)
	for i := range 3 {
		c.s.Now = func() time.Time { return day(i) }
		c.add(t, c.alice, res("tusmo", "", 70+i, new(3), nil))
	}
	c.s.Now = func() time.Time { return day(3) }
	award(t, c)
	if got := badgeDays(t, c, c.alice); !reflect.DeepEqual(got, []string{"2026-10-08", "2026-10-07", "2026-10-06"}) {
		t.Fatalf("badges = %v, want three days newest first", got)
	}
}

// Review Focus 2: two reads at once.
func TestAwardTwiceIsIdempotent(t *testing.T) {
	c := oldCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	c.s.Now = func() time.Time { return day(1) }
	award(t, c)
	c.s.db.Exec("UPDATE groups SET badges_through = '2026-10-05'") // a second reader that loaded the old mark
	award(t, c)
	if got := badgeDays(t, c, c.alice); len(got) != 1 {
		t.Fatalf("badges = %v, want exactly one", got)
	}
}

func TestAwardNoBackfill(t *testing.T) {
	c := oldCrew(t)                                      // members in by 2026-10-05: only badges_through keeps that day out
	c.add(t, c.alice, res("tusmo", "", 69, new(3), nil)) // day before the group existed
	c.s.Now = func() time.Time { return day(1) }
	award(t, c)
	if got := badgeDays(t, c, c.alice); got != nil {
		t.Fatalf("badges = %v, want none (2026-10-05 predates the group)", got)
	}
}

func TestUnseenAndSeen(t *testing.T) {
	c := oldCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	c.s.Now = func() time.Time { return day(1) }
	award(t, c)
	u, err := c.s.Unseen(c.alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Unseen{1, "2026-10-06", KindLonely}); u[c.g.ID] != want {
		t.Fatalf("alice unseen = %+v, want %+v", u[c.g.ID], want)
	}
	if u, _ := c.s.Unseen(c.bob.ID); len(u) != 0 {
		t.Fatalf("bob unseen = %v, want none (alice's badge is not his news)", u)
	}
	c.s.Now = func() time.Time { return day(1).Add(time.Second) }
	if err := c.s.MarkBadgesSeen(c.g.ID, c.alice.ID); err != nil {
		t.Fatal(err)
	}
	if u, _ := c.s.Unseen(c.alice.ID); len(u) != 0 {
		t.Fatalf("after seen = %v, want none", u)
	}
}

func TestUnseenAfterRejoin(t *testing.T) {
	c := oldCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	c.s.Now = func() time.Time { return day(1) }
	award(t, c)
	c.s.Now = func() time.Time { return day(1).Add(time.Second) }
	c.s.RemoveMember(c.g.ID, c.bob.ID) // alice owns the group: bob leaves instead
	c.s.db.Exec("UPDATE badges SET user_id = ?", c.bob.ID)
	c.s.JoinByCode(c.bob.ID, c.g.InviteCode)
	if u, _ := c.s.Unseen(c.bob.ID); len(u) != 0 {
		t.Fatalf("after rejoin unseen = %v, want none", u)
	}
}

func TestWall(t *testing.T) {
	c := oldCrew(t)
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	c.s.Now = func() time.Time { return day(1) }
	c.add(t, c.alice, res("tusmo", "", 71, new(3), nil))
	c.s.Now = func() time.Time { return day(2) }
	award(t, c)
	rows, err := c.s.Wall(c.g.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Username)
	}
	if !reflect.DeepEqual(got, []string{"alice", "bob", "carol"}) {
		t.Fatalf("wall = %v, want members in Members order, not dave", got)
	}
	if rows[0].Counts[KindLonely] != 2 || rows[0].Total != 2 || rows[0].Pawn == "" || rows[1].Total != 0 {
		t.Fatalf("wall rows = %+v", rows)
	}
}

func TestBadgeByToken(t *testing.T) {
	c := oldCrew(t)
	c.s.SetLang(c.alice.ID, "fr")
	c.add(t, c.alice, res("tusmo", "", 70, new(3), nil))
	c.s.Now = func() time.Time { return day(1) }
	award(t, c)
	bs, _ := c.s.BadgesOf(c.g.ID, c.alice.ID)
	b, err := c.s.BadgeByToken(bs[0].Token)
	if err != nil {
		t.Fatal(err)
	}
	if b.GroupName != "Friends" || b.GroupID != c.g.ID || b.Username != "alice" || b.Lang != "fr" || b.Day != "2026-10-06" {
		t.Fatalf("badge = %+v", b)
	}
	if _, err := c.s.BadgeByToken("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown token err = %v, want ErrNotFound", err)
	}
}
