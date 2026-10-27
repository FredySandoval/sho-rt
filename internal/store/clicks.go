package store

import (
	"context"
	"database/sql"
	"time"
)

// Keep at most 90 days of hourly buckets per link. Lifetime totals stay in links.
func migrateClickTotals(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`
 CREATE TABLE IF NOT EXISTS click_totals (
  link_id INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE,
  hour TEXT NOT NULL,
  count INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(link_id, hour)
 );
 CREATE INDEX IF NOT EXISTS click_totals_hour ON click_totals(hour);
 INSERT INTO click_totals(link_id, hour, count)
 SELECT link_id, strftime('%Y-%m-%d %H:00:00', happened), count(*) FROM clicks
 WHERE happened >= strftime('%Y-%m-%d %H:00:00','now','-90 days')
 GROUP BY link_id, strftime('%Y-%m-%d %H:00:00', happened);
 DROP TABLE clicks;
 PRAGMA user_version=2;
 `)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) trackClick(ctx context.Context, id int64) error {
	ctx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE links SET clicks=clicks+1 WHERE id=?`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO click_totals(link_id,hour,count)
 VALUES (?,strftime('%Y-%m-%d %H:00:00','now'),1)
 ON CONFLICT(link_id,hour) DO UPDATE SET count=count+1`, id); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM click_totals WHERE hour < strftime('%Y-%m-%d %H:00:00','now','-90 days')`); err != nil {
		return err
	}
	return tx.Commit()
}
