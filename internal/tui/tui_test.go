package tui

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"github.com/charmbracelet/x/ansi"
	"github.com/fredy/sho-rt/internal/config"
	"github.com/fredy/sho-rt/internal/server"
	"github.com/fredy/sho-rt/internal/store"
	"path/filepath"
	"strings"
	"testing"
)

func TestViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	db, e := store.Open(c.Database)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if err := db.Save(context.Background(), 0, "https://example.com", "demo"); err != nil {
		t.Fatal(err)
	}
	m := New(path, c, db, &server.Runner{})
	for _, size := range []tea.WindowSizeMsg{{Width: 60, Height: 18}, {Width: 110, Height: 32}, {Width: 160, Height: 45}} {
		m.Update(size)
		for page := 0; page < 3; page++ {
			m.page = page
			v := m.View()
			if !v.AltScreen || !strings.Contains(v.Content, "Link Shortener") {
				t.Fatal("missing dashboard")
			}
			if page == 0 || page == 1 {
				if !strings.Contains(ansi.Strip(v.Content), "demo") {
					t.Fatalf("page %d at width %d: saved link missing from table", page, size.Width)
				}
			}
			lines := strings.Split(v.Content, "\n")
			for _, line := range []string{lines[0], lines[len(lines)-1]} {
				if got := ansi.StringWidth(line); got != size.Width {
					t.Fatalf("page %d: border width = %d, want %d", page, got, size.Width)
				}
			}
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if !strings.Contains(m.View().Content, "resize") {
		t.Fatal("missing size guidance")
	}
}
func TestFormCancelDoesNotMutateConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	db, e := store.Open(c.Database)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	m := New(path, c, db, &server.Runner{})
	m.settingsForm()
	if m.c != c {
		t.Fatal("form mutated active config")
	}
}
