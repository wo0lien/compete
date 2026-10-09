// Package store owns the SQLite database and every query, including the
// privacy rules (reveal rule, closed-puzzle aggregates) any UI must respect.
package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
	// Now is the clock that decides which puzzle is today's; tests pin it.
	Now func() time.Time
}

// Open opens (creating if needed) the database at path and applies pending migrations.
func Open(path string) (*Store, error) {
	// _txlock=immediate: transactions that read then write (group cap, join) take the
	// write lock up front; a deferred read->write upgrade fails with SQLITE_BUSY in WAL.
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, Now: time.Now}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// migrate runs migrations/NNN_*.sql in order, tracking progress in PRAGMA user_version.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql") // sorted lexically
	if err != nil {
		return err
	}
	if version > len(names) {
		return fmt.Errorf("database schema v%d is newer than this binary (v%d)", version, len(names))
	}
	for i := version; i < len(names); i++ {
		sqlText, err := migrations.ReadFile(names[i])
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(sqlText)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", names[i], err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func isUnique(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
