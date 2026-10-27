package tui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/fredy/sho-rt/internal/store"
)

func resolveTransferPath(input string, importing bool) (string, error) {
	if input == "~" || strings.HasPrefix(input, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if input == "~" {
			input = home
		} else {
			input = filepath.Join(home, strings.TrimPrefix(input, "~/"))
		}
	}
	if strings.TrimSpace(input) == "" {
		return "", fmt.Errorf("enter a file path")
	}
	path, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	if importing {
		return filepath.EvalSymlinks(path)
	}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("export path must not be a symbolic link")
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("export path must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(path)), nil
}
func (m *Model) transferForm(importing bool) tea.Cmd {
	m.formKind = "export-path"
	title := "Export all links to JSON"
	if importing {
		m.formKind = "import-path"
		title = "Import links from JSON"
	}
	m.form = huh.NewForm(huh.NewGroup(huh.NewInput().Key("file").Title(title).Description("Absolute, relative, or ~/ path").Validate(func(v string) error { _, err := resolveTransferPath(v, importing); return err })))
	m.resize()
	return m.form.Init()
}
func (m *Model) prepareTransfer() (tea.Cmd, error) {
	importing := m.formKind == "import-path"
	path, err := resolveTransferPath(m.form.GetString("file"), importing)
	if err != nil {
		return nil, err
	}
	m.transferPath = path
	if importing {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		data, err := store.DecodeTransfer(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		summary, err := m.db.Import(context.Background(), data, false, true)
		if err != nil {
			return nil, err
		}
		m.transferData = data
		m.formKind = "import-confirm"
		m.form = huh.NewForm(huh.NewGroup(
			huh.NewNote().Title("Import preview").Description(path+"\n"+summary.String()),
			huh.NewConfirm().Key("overwrite").Title("Overwrite conflicting links?").Affirmative("Overwrite").Negative("Skip conflicts"),
			huh.NewConfirm().Key("proceed").Title("Apply import?").Affirmative("Import").Negative("Cancel"),
		))
	} else {
		title := "Export all links?"
		if _, err := os.Lstat(path); err == nil {
			title = "Overwrite existing file with all links?"
		}
		m.formKind = "export-confirm"
		m.form = huh.NewForm(huh.NewGroup(huh.NewConfirm().Key("proceed").Title(title).Description(path).Affirmative("Export").Negative("Cancel")))
	}
	m.resize()
	return m.form.Init(), nil
}
func (m *Model) finishTransfer() error {
	if !m.form.GetBool("proceed") {
		m.notice = "Cancelled"
		return nil
	}
	if m.formKind == "import-confirm" {
		summary, err := m.db.Import(context.Background(), m.transferData, m.form.GetBool("overwrite"), false)
		if err == nil {
			m.notice = summary.String()
		}
		return err
	}
	var buf bytes.Buffer
	if err := m.db.Export(context.Background(), &buf); err != nil {
		return err
	}
	// Never open the destination for writing: a symlink cannot redirect the write.
	path, err := resolveTransferPath(m.transferPath, false)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".sho-rt-export-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(buf.Bytes()); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if _, err = resolveTransferPath(path, false); err != nil {
		return err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	m.notice = "Exported to " + path
	return nil
}
