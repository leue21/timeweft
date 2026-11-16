// Package store persists events in a local SQLite database.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/leue21/timeweft/internal/events"

	_ "modernc.org/sqlite"
)

// migrations are applied in order; the index+1 is the schema version.
// Each must be idempotent: several hooks may open a fresh database at once.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS events (
		id      INTEGER PRIMARY KEY,
		ts      INTEGER NOT NULL, -- unix milliseconds, UTC
		source  TEXT NOT NULL,
		kind    TEXT NOT NULL,
		session TEXT NOT NULL,
		repo    TEXT NOT NULL,
		branch  TEXT NOT NULL,
		cwd     TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS events_ts ON events(ts);`,
}

type Store struct{ db *sql.DB }

// DefaultPath is $TIMEWEFT_DB, else $XDG_DATA_HOME/timeweft/timeweft.db,
// else ~/.local/share/timeweft/timeweft.db (%LOCALAPPDATA%\timeweft on Windows).
func DefaultPath() (string, error) {
	if p := os.Getenv("TIMEWEFT_DB"); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" && runtime.GOOS == "windows" {
		base = os.Getenv("LOCALAPPDATA")
	}
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "timeweft", "timeweft.db"), nil
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate %s: %w", path, err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return err
	}
	for i := v; i < len(migrations); i++ {
		if _, err := s.db.Exec(migrations[i]); err != nil {
			return err
		}
		if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Add(e events.Event) error {
	_, err := s.db.Exec(
		`INSERT INTO events (ts, source, kind, session, repo, branch, cwd) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Time.UnixMilli(), e.Source, string(e.Kind), e.Session, e.Repo, e.Branch, e.Cwd)
	return err
}

// Between returns events with from <= time < to, oldest first.
func (s *Store) Between(from, to time.Time) ([]events.Event, error) {
	rows, err := s.db.Query(
		`SELECT id, ts, source, kind, session, repo, branch, cwd FROM events
		 WHERE ts >= ? AND ts < ? ORDER BY ts, id`,
		from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []events.Event
	for rows.Next() {
		var e events.Event
		var ts int64
		var kind string
		if err := rows.Scan(&e.ID, &ts, &e.Source, &kind, &e.Session, &e.Repo, &e.Branch, &e.Cwd); err != nil {
			return nil, err
		}
		e.Time = time.UnixMilli(ts)
		e.Kind = events.Kind(kind)
		out = append(out, e)
	}
	return out, rows.Err()
}
