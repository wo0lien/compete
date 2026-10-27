package store

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/wo0lien/compete/games"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s.Now = func() time.Time { return fixtureDay }
	return s
}

// fixtureDay is when the repo's samples were shared: Tusmo #70, Songless #404, Travle #1392.
var fixtureDay = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

func TestOpenMigratesOnceAndEnablesForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for range 2 { // the second Open must find nothing to migrate
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var version, fk int
		s.db.QueryRow("PRAGMA user_version").Scan(&version)
		s.db.QueryRow("PRAGMA foreign_keys").Scan(&fk)
		if version != 7 || fk != 1 {
			t.Fatalf("user_version=%d foreign_keys=%d, want 7 1", version, fk)
		}
		s.Close()
	}
}

// Migration 005 seeds each existing group from its members' last 14 days of
// results; a group with none gets every game (Review Focus 5).
func TestMigrateSeedsGroupGames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	played, _ := s.CreateGroup(alice.ID, "Played", []string{"tusmo"})
	quiet, _ := s.CreateGroup(bob.ID, "Quiet", []string{"tusmo"})
	s.AddResults(alice.ID, "raw", []games.Result{{Game: "travle", Variant: "fra", PuzzleID: 1, Score: new(1)}})
	s.AddResults(alice.ID, "raw", []games.Result{{Game: "songless", Variant: "Rock", PuzzleID: 404, Score: new(1)}})
	s.db.Exec(`UPDATE results SET submitted_at = unixepoch() - 20*86400 WHERE game = 'songless'`) // too old
	// back to schema v4, as a production DB before this release
	if _, err := s.db.Exec(`DROP TABLE badges; ALTER TABLE memberships DROP COLUMN badges_seen_at; ALTER TABLE groups DROP COLUMN badges_through;
		DROP TABLE group_games; DROP TABLE seasons; ALTER TABLE groups DROP COLUMN season_days; PRAGMA user_version = 4`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, _ := s.GroupGames(played.ID); !reflect.DeepEqual(got, []string{"travle"}) {
		t.Errorf("played group = %v, want [travle]", got)
	}
	if got, _ := s.GroupGames(quiet.ID); !reflect.DeepEqual(got, []string{"tusmo", "songless", "travle"}) {
		t.Errorf("quiet group = %v, want every game", got)
	}
}

// Migration 007: existing groups start awarding from yesterday (no backfill),
// existing members have nothing new.
func TestMigrateSeedsBadges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	alice := mustUser(t, s, "alice")
	g, _ := s.CreateGroup(alice.ID, "Friends", []string{"tusmo"})
	if _, err := s.db.Exec(`DROP TABLE badges; ALTER TABLE memberships DROP COLUMN badges_seen_at;
		ALTER TABLE groups DROP COLUMN badges_through; PRAGMA user_version = 6`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var through string
	var seen int64
	s.db.QueryRow("SELECT badges_through FROM groups WHERE id = ?", g.ID).Scan(&through)
	s.db.QueryRow("SELECT badges_seen_at FROM memberships WHERE group_id = ?", g.ID).Scan(&seen)
	if want := time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly); through != want {
		t.Errorf("badges_through = %q, want yesterday %q", through, want)
	}
	if seen < time.Now().Unix()-60 {
		t.Errorf("badges_seen_at = %d, want about now", seen)
	}
}
