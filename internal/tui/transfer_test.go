package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTransferPaths(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "links.json")
	if err := os.WriteFile(target, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "alias.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if got, err := resolveTransferPath(link, true); err != nil || got != target {
		t.Fatalf("import symlink: %s %v", got, err)
	}
	if _, err := resolveTransferPath(link, false); err == nil {
		t.Fatal("export accepted symlink")
	}
	broken := filepath.Join(dir, "broken.json")
	if err := os.Symlink(filepath.Join(dir, "missing"), broken); err != nil {
		t.Fatal(err)
	}
	for _, importing := range []bool{true, false} {
		if _, err := resolveTransferPath(broken, importing); err == nil {
			t.Fatal("accepted broken symlink")
		}
	}
	directory := filepath.Join(dir, "directory")
	if err := os.Symlink(dir, directory); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveTransferPath(filepath.Join(directory, "new.json"), false); err != nil || got != filepath.Join(dir, "new.json") {
		t.Fatalf("directory symlink: %s %v", got, err)
	}
	t.Setenv("HOME", dir)
	if got, err := resolveTransferPath("~/links.json", true); err != nil || got != target {
		t.Fatalf("home: %s %v", got, err)
	}
	if got, err := resolveTransferPath("links.json", false); err != nil || !filepath.IsAbs(got) {
		t.Fatalf("relative: %s %v", got, err)
	}
}
