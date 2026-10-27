package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed schema.sql
var schema string
var ErrDisabled = errors.New("link disabled")

type Link struct {
	ID                          int64
	Alias, Destination, Created string
	Enabled                     bool
	Clicks                      int64
}
type Stats struct{ Links, Clicks, Recent, Month int64 }
type Store struct {
	db              *sql.DB
	lastTrackingLog atomic.Int64
}

func Open(path string) (*Store, error) {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if _, e = db.Exec("PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); e != nil {
		db.Close()
		return nil, e
	}
	var version int
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil {
		db.Close()
		return nil, e
	}
	if version > 2 {
		db.Close()
		return nil, errors.New("database is newer than this application")
	}
	if version < 1 {
		tx, err := db.Begin()
		if err != nil {
			db.Close()
			return nil, err
		}
		if _, err = tx.Exec(schema); err != nil {
			tx.Rollback()
			db.Close()
			return nil, err
		}
		if err = tx.Commit(); err != nil {
			db.Close()
			return nil, err
		}
	}
	if version < 2 {
		if e = migrateClickTotals(db); e != nil {
			db.Close()
			return nil, e
		}
	}
	if _, e = db.Exec("DELETE FROM click_totals WHERE hour < strftime('%Y-%m-%d %H:00:00','now','-90 days')"); e != nil {
		db.Close()
		return nil, e
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }

// Relocate copies a consistent SQLite snapshot, never overwriting an existing file.
// Call only while the HTTP server is stopped.
func (s *Store) Relocate(path string) error {
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		return errors.New("new database path already exists or is inaccessible")
	}
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	if _, e := s.db.Exec("VACUUM INTO ?", path); e != nil {
		return e
	}
	next, e := Open(path)
	if e != nil {
		return e
	}
	old := s.db
	s.db = next.db
	return old.Close()
}
func ValidateURL(v string) error {
	u, e := url.Parse(v)
	if e != nil || u == nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return errors.New("enter an absolute http:// or https:// URL without credentials")
	}
	return nil
}

var aliasRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func ValidateAlias(a string) error {
	if !aliasRE.MatchString(a) {
		return errors.New("alias must contain 1–64 letters, numbers, underscores or hyphens")
	}
	return nil
}
func GenerateAlias() (string, error) {
	b := make([]byte, 5)
	_, e := rand.Read(b)
	return hex.EncodeToString(b), e
}
func (s *Store) Save(ctx context.Context, id int64, dest, alias string) error {
	dest = strings.TrimSpace(dest)
	alias = strings.TrimSpace(alias)
	if e := ValidateURL(dest); e != nil {
		return e
	}
	if alias == "" {
		var e error
		alias, e = GenerateAlias()
		if e != nil {
			return e
		}
	}
	if e := ValidateAlias(alias); e != nil {
		return e
	}
	var e error
	if id == 0 {
		_, e = s.db.ExecContext(ctx, "INSERT INTO links(alias,destination) VALUES(?,?)", alias, dest)
	} else {
		_, e = s.db.ExecContext(ctx, "UPDATE links SET alias=?,destination=? WHERE id=?", alias, dest, id)
	}
	if e != nil {
		return fmt.Errorf("cannot save link (alias may already exist): %w", e)
	}
	return nil
}
func (s *Store) List(ctx context.Context, search string) ([]Link, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,alias,destination,created,enabled,clicks FROM links WHERE instr(lower(alias),lower(?))>0 OR instr(lower(destination),lower(?))>0 ORDER BY id DESC", search, search)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var links []Link
	for rows.Next() {
		var l Link
		if e = rows.Scan(&l.ID, &l.Alias, &l.Destination, &l.Created, &l.Enabled, &l.Clicks); e != nil {
			return nil, e
		}
		links = append(links, l)
	}
	return links, rows.Err()
}
func (s *Store) Delete(ctx context.Context, id int64) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM links WHERE id=?", id)
	return e
}
func (s *Store) Toggle(ctx context.Context, id int64) error {
	_, e := s.db.ExecContext(ctx, "UPDATE links SET enabled=NOT enabled WHERE id=?", id)
	return e
}
func (s *Store) Resolve(ctx context.Context, alias string, track bool) (string, error) {
	var id int64
	var dest string
	var enabled bool
	if e := s.db.QueryRowContext(ctx, "SELECT id,destination,enabled FROM links WHERE alias=?", alias).Scan(&id, &dest, &enabled); e != nil {
		return "", e
	}
	if !enabled {
		return "", ErrDisabled
	}
	if track {
		if err := s.trackClick(ctx, id); err != nil {
			now := time.Now().Unix()
			last := s.lastTrackingLog.Load()
			if now-last >= 60 && s.lastTrackingLog.CompareAndSwap(last, now) {
				log.Printf("click tracking failed (redirect preserved): %v", err)
			}
		}
	}
	return dest, nil
}
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var v Stats
	e := s.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM links),(SELECT coalesce(sum(clicks),0) FROM links),(SELECT coalesce(sum(count),0) FROM click_totals WHERE hour>=strftime('%Y-%m-%d %H:00:00','now','-23 hours')),(SELECT coalesce(sum(count),0) FROM click_totals WHERE hour>=datetime('now','start of month'))`).Scan(&v.Links, &v.Clicks, &v.Recent, &v.Month)
	return v, e
}
