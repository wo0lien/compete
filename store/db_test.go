package store

import (
	"path/filepath"
	"testing"
	"time"
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
		if version != 3 || fk != 1 {
			t.Fatalf("user_version=%d foreign_keys=%d, want 3 1", version, fk)
		}
		s.Close()
	}
}
