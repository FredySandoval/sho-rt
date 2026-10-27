package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestClickAggregationAndFailure(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "links.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if err = s.Save(ctx, 0, "https://example.com", "test"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err = s.Resolve(ctx, "test", true); err != nil {
			t.Fatal(err)
		}
	}
	var rows, total int
	if err = s.db.QueryRow("SELECT count(*),sum(count) FROM click_totals").Scan(&rows, &total); err != nil {
		t.Fatal(err)
	}
	if rows != 1 || total != 20 {
		t.Fatalf("rows=%d total=%d", rows, total)
	}
	if _, err = s.db.Exec(`INSERT INTO click_totals VALUES(1,'2000-01-01 00:00:00',99)`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Resolve(ctx, "test", true); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow("SELECT count(*) FROM click_totals").Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("retention: %d %v", rows, err)
	}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_tracking BEFORE INSERT ON click_totals BEGIN SELECT RAISE(FAIL,'simulated disk failure'); END`); err != nil {
		t.Fatal(err)
	}
	dest, err := s.Resolve(ctx, "test", true)
	if err != nil || dest != "https://example.com" {
		t.Fatalf("redirect failed: %s %v", dest, err)
	}
	stats, err := s.Stats(ctx)
	if err != nil || stats.Clicks != 21 {
		t.Fatalf("tracking rollback: %+v %v", stats, err)
	}
}

func TestLegacyClickMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "links.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO links(alias,destination,clicks) VALUES('test','https://example.com',3);
 INSERT INTO clicks(link_id) VALUES(1),(1);
 INSERT INTO clicks(link_id,happened) VALUES(1,'2000-01-01');`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := s.Stats(context.Background())
	if err != nil || stats.Clicks != 3 || stats.Recent != 2 {
		t.Fatalf("migration: %+v %v", stats, err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stats, err = s.Stats(context.Background())
	if err != nil || stats.Recent != 2 {
		t.Fatalf("reopen: %+v %v", stats, err)
	}
}
