// Package store owns the SQLite database: opening, migrating, nightly backups,
// and the two generic helpers every handler uses (Rows, Exec). Handlers write
// SQL; this package keeps the connection rules in one place.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	db      *sql.DB
	dataDir string
}

// Open opens ${dataDir}/potok.db in WAL mode and applies pending migrations.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir data dir: %w", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dataDir, "potok.db"))
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	// modernc's driver is one connection under the hood for file DBs; keep it
	// to one to avoid "database is locked" churn.
	db.SetMaxOpenConns(1)
	for _, p := range []string{"PRAGMA journal_mode=WAL;", "PRAGMA foreign_keys=ON;"} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}
	s := &Store{db: db, dataDir: dataDir}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error                   { return s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// migrate applies every migrations/*.sql not yet recorded, in filename order,
// one transaction each. Hand-rolled — no framework.
func (s *Store) migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (datetime('now')))`); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, n := range names {
		var done int
		if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=?`, n).Scan(&done); err != nil {
			return err
		}
		if done > 0 {
			continue
		}
		body, err := migrationsFS.ReadFile("migrations/" + n)
		if err != nil {
			return err
		}
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", n, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, n); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Printf("migrated %s", n)
	}
	return nil
}

// Row is one result row keyed by column name — JSON-ready. SQLite integers
// arrive as int64, text as string, NULL as nil.
type Row = map[string]any

// Rows runs a query and returns every row as a map.
func (s *Store) Rows(ctx context.Context, q string, args ...any) ([]Row, error) {
	rs, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, err
	}
	out := []Row{}
	for rs.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, err
		}
		r := Row{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				r[c] = string(b)
			} else {
				r[c] = vals[i]
			}
		}
		out = append(out, r)
	}
	return out, rs.Err()
}

// One returns the first row or nil.
func (s *Store) One(ctx context.Context, q string, args ...any) (Row, error) {
	rows, err := s.Rows(ctx, q, args...)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// Exec runs a statement and returns the last insert id.
func (s *Store) Exec(ctx context.Context, q string, args ...any) (int64, error) {
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// Backup writes a consistent copy with VACUUM INTO (safe on a live WAL db) to
// ${dataDir}/backups/potok-YYYY-MM-DD.db and keeps the newest `keep` files.
func (s *Store) Backup(ctx context.Context, keep int) (string, error) {
	dir := filepath.Join(s.dataDir, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "potok-"+time.Now().UTC().Format("2006-01-02")+".db")
	os.Remove(path)
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("vacuum into: %w", err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "potok-*.db"))
	sort.Strings(files)
	for len(files) > keep {
		os.Remove(files[0])
		files = files[1:]
	}
	return path, nil
}

// RunNightlyBackups blocks, taking a backup once per day at 03:00 UTC.
func (s *Store) RunNightlyBackups(ctx context.Context) {
	for {
		now := time.Now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
		}
		if p, err := s.Backup(ctx, 14); err != nil {
			log.Printf("backup failed: %v", err)
		} else {
			log.Printf("backup written: %s", p)
		}
	}
}
