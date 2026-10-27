package store

import "database/sql"

// Settings returns the portable configuration stored with the links.
func (s *Store) Settings() ([]byte, error) {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS app_settings (id INTEGER PRIMARY KEY CHECK (id = 1), value TEXT NOT NULL)`); err != nil {
		return nil, err
	}
	var value []byte
	err := s.db.QueryRow(`SELECT value FROM app_settings WHERE id = 1`).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return value, err
}

func (s *Store) SaveSettings(value []byte) error {
	if _, err := s.Settings(); err != nil {
		return err
	}
	_, err := s.db.Exec(`INSERT INTO app_settings(id, value) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET value = excluded.value`, string(value))
	return err
}
