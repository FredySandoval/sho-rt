package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestValidation(t *testing.T) {
	for _, v := range []string{"https://example.com/a?q=x", "http://localhost:3000", "https://[::1]/"} {
		if e := ValidateURL(v); e != nil {
			t.Errorf("%s: %v", v, e)
		}
	}
	for _, v := range []string{"", "example.com", "javascript:alert(1)", "ftp://example.com", "https://", "https://user:pass@example.com", "https://example.com\n"} {
		if ValidateURL(v) == nil {
			t.Errorf("accepted %q", v)
		}
	}
	for _, a := range []string{"../x", "a/b", "a b", "", "é"} {
		if ValidateAlias(a) == nil {
			t.Errorf("accepted alias %q", a)
		}
	}
}
func TestAliases(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		a, e := GenerateAlias()
		if e != nil || ValidateAlias(a) != nil || seen[a] {
			t.Fatalf("bad alias %s: %v", a, e)
		}
		seen[a] = true
	}
}
func TestPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "links.db")
	s, e := Open(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Save(ctx, 0, "https://example.com", "test"); e != nil {
		t.Fatal(e)
	}
	if s.Save(ctx, 0, "https://example.org", "test") == nil {
		t.Fatal("duplicate accepted")
	}
	if _, e = s.Resolve(ctx, "test", true); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ls, e := s.List(ctx, "TEST")
	if e != nil || len(ls) != 1 || ls[0].Clicks != 1 {
		t.Fatalf("persistence: %v %v", ls, e)
	}
	st, e := s.Stats(ctx)
	if e != nil || st.Clicks != 1 || st.Recent != 1 {
		t.Fatalf("stats: %+v %v", st, e)
	}
	if e = s.Toggle(ctx, ls[0].ID); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Resolve(ctx, "test", true); e != ErrDisabled {
		t.Fatal(e)
	}
	newPath := filepath.Join(t.TempDir(), "moved.db")
	if e = s.Relocate(newPath); e != nil {
		t.Fatal(e)
	}
	if e = s.Relocate(newPath); e == nil {
		t.Fatal("overwrote existing database")
	}
	if e = s.Delete(ctx, ls[0].ID); e != nil {
		t.Fatal(e)
	}
	ls, e = s.List(ctx, "")
	if e != nil || len(ls) != 0 {
		t.Fatal(ls, e)
	}
}
