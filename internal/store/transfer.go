package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Transfer is the portable link format; IDs and analytics intentionally stay local.
type Transfer struct {
	Version int            `json:"version"`
	Links   []TransferLink `json:"links"`
}
type TransferLink struct {
	Alias       string `json:"alias"`
	Destination string `json:"destination"`
	Enabled     *bool  `json:"enabled,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}
type ImportSummary struct{ Created, Unchanged, Conflicts, Updated int }

func (s ImportSummary) String() string {
	return fmt.Sprintf("%d new, %d unchanged, %d conflicts skipped, %d updated", s.Created, s.Unchanged, s.Conflicts, s.Updated)
}

func DecodeTransfer(r io.Reader) (Transfer, error) {
	var data Transfer
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(&data); err != nil {
		return data, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return data, fmt.Errorf("expected a single JSON document")
	}
	return data, data.Validate()
}
func (d Transfer) Validate() error {
	if d.Version != 1 {
		return fmt.Errorf("unsupported JSON version %d", d.Version)
	}
	if d.Links == nil {
		return fmt.Errorf("links must be an array")
	}
	seen := map[string]bool{}
	for _, l := range d.Links {
		if err := ValidateAlias(l.Alias); err != nil {
			return err
		}
		if seen[l.Alias] {
			return fmt.Errorf("duplicate alias %q in import", l.Alias)
		}
		seen[l.Alias] = true
		if err := ValidateURL(l.Destination); err != nil {
			return fmt.Errorf("%s: %w", l.Alias, err)
		}
		if l.CreatedAt != "" {
			if _, err := time.Parse(time.RFC3339, l.CreatedAt); err != nil {
				return fmt.Errorf("%s: created_at must be RFC3339", l.Alias)
			}
		}
	}
	return nil
}
func (s *Store) Export(ctx context.Context, w io.Writer) error {
	links, err := s.List(ctx, "")
	if err != nil {
		return err
	}
	data := Transfer{Version: 1, Links: []TransferLink{}}
	for _, l := range links {
		t, err := time.Parse("2006-01-02 15:04:05", l.Created)
		if err != nil {
			return err
		}
		enabled := l.Enabled
		data.Links = append(data.Links, TransferLink{l.Alias, l.Destination, &enabled, t.UTC().Format(time.RFC3339)})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// Import validates and applies the whole document atomically. Preview rolls back.
func (s *Store) Import(ctx context.Context, data Transfer, overwrite, preview bool) (ImportSummary, error) {
	var result ImportSummary
	if err := data.Validate(); err != nil {
		return result, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT alias,destination,enabled FROM links")
	if err != nil {
		return result, err
	}
	type existing struct {
		destination string
		enabled     bool
	}
	current := map[string]existing{}
	for rows.Next() {
		var a string
		var e existing
		if err = rows.Scan(&a, &e.destination, &e.enabled); err != nil {
			rows.Close()
			return result, err
		}
		current[a] = e
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	for _, l := range data.Links {
		enabled := true
		if l.Enabled != nil {
			enabled = *l.Enabled
		}
		if old, ok := current[l.Alias]; ok {
			if old.destination == l.Destination && old.enabled == enabled {
				result.Unchanged++
				continue
			}
			if !overwrite {
				result.Conflicts++
				continue
			}
			result.Updated++
			if !preview {
				_, err = tx.ExecContext(ctx, "UPDATE links SET destination=?,enabled=? WHERE alias=?", l.Destination, enabled, l.Alias)
			}
		} else {
			result.Created++
			created := time.Now().UTC()
			if l.CreatedAt != "" {
				created, _ = time.Parse(time.RFC3339, l.CreatedAt)
			}
			if !preview {
				_, err = tx.ExecContext(ctx, "INSERT INTO links(alias,destination,enabled,created) VALUES(?,?,?,?)", l.Alias, l.Destination, enabled, created.UTC().Format("2006-01-02 15:04:05"))
			}
		}
		if err != nil {
			return result, err
		}
	}
	if preview {
		return result, nil
	}
	return result, tx.Commit()
}
