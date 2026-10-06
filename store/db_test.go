package store

import (
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

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
		if version != 2 || fk != 1 {
			t.Fatalf("user_version=%d foreign_keys=%d, want 2 1", version, fk)
		}
		s.Close()
	}
}
