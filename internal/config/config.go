package config

import (
	"encoding/json"
	"fmt"
	"github.com/fredy/sho-rt/internal/store"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Address   string
	Port      int
	Database  string
	Domain    string
	AutoStart bool
}

func Path() (string, error) {
	d, e := os.UserConfigDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(d, "sho-rt", "links.db"), nil
}

// Load also accepts the old config.json location for installed service compatibility.
func Load(path string) (Config, error) {
	location := filepath.Join(filepath.Dir(path), "links.db")
	c := Config{Address: "127.0.0.1", Port: 3000, Database: location, Domain: "example.com"}
	legacy := filepath.Join(filepath.Dir(path), "config.json")
	// Never silently abandon a database previously stored elsewhere.
	b, err := os.ReadFile(legacy)
	if err != nil && !os.IsNotExist(err) {
		return c, err
	}
	if err == nil {
		if err = json.Unmarshal(b, &c); err != nil {
			return c, err
		}
		if c.Database != location {
			return c, fmt.Errorf("legacy database is at %s; stop the service and restore a SQLite backup to %s, then remove the legacy config.json", c.Database, location)
		}
	}
	db, err := store.Open(location)
	if err != nil {
		return c, err
	}
	data, err := db.Settings()
	db.Close()
	if err != nil {
		return c, err
	}
	if len(data) > 0 {
		if err = json.Unmarshal(data, &c); err != nil {
			return c, err
		}
	}
	c.Database = location
	if err = Save(path, c); err != nil {
		return c, err
	}
	if len(b) > 0 {
		// Retain the original as an inactive migration backup.
		if err = os.Rename(legacy, legacy+".migrated"); err != nil {
			return c, err
		}
	}
	return c, nil
}
func (c Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be 1–65535")
	}
	if net.ParseIP(c.Address) == nil && c.Address != "localhost" {
		return fmt.Errorf("listen address must be an IP or localhost")
	}
	if !filepath.IsAbs(c.Database) {
		return fmt.Errorf("database path must be absolute")
	}
	if c.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	return nil
}
func (c Config) Listen() string { return net.JoinHostPort(c.Address, strconv.Itoa(c.Port)) }
func Save(path string, c Config) error {
	if e := c.Validate(); e != nil {
		return e
	}
	if c.Database != filepath.Join(filepath.Dir(path), "links.db") {
		return fmt.Errorf("database location is fixed")
	}
	db, e := store.Open(c.Database)
	if e != nil {
		return e
	}
	defer db.Close()
	portable, e := json.Marshal(struct {
		Address   string
		Port      int
		Domain    string
		AutoStart bool
	}{c.Address, c.Port, c.Domain, c.AutoStart})
	if e != nil {
		return e
	}
	if e = db.SaveSettings(portable); e != nil {
		return e
	}
	return nil
}
