package config

import (
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	c, e := Load(p)
	if e != nil {
		t.Fatal(e)
	}
	if c.Port != 3000 {
		t.Fatal(c)
	}
	c.Port = 4000
	c.AutoStart = true
	if e = Save(p, c); e != nil {
		t.Fatal(e)
	}
	next, e := Load(p)
	if e != nil || next != c {
		t.Fatal(next, e)
	}
	c.Port = -1
	if Save(p, c) == nil {
		t.Fatal("invalid port accepted")
	}
}
