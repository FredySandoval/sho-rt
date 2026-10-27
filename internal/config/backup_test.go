package config

import (
	"github.com/fredy/sho-rt/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableSettingsRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("unexpected config.json: %v", err)
	}
	if c.Domain != "example.com" {
		t.Fatal(c.Domain)
	}
	c.Domain = "short.example.org"
	c.Address = "0.0.0.0"
	c.Port = 8088
	c.AutoStart = true
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	restoredDir := t.TempDir()
	restoredDB := filepath.Join(restoredDir, "links.db")
	db, err := store.Open(c.Database)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Relocate(restoredDB); err != nil {
		t.Fatal(err)
	}
	restored, err := Load(filepath.Join(restoredDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	c.Database = restoredDB
	if restored != c {
		t.Fatalf("restored %+v; want %+v", restored, c)
	}
}
