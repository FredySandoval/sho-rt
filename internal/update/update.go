// Package update checks and installs checksum-verified GitHub releases.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Version is set by the release build. Development builds never auto-update.
var Version = "dev"
var client = &http.Client{Timeout: 2 * time.Minute}

const repo = "https://github.com/FredySandoval/sho-rt/releases/download/"

var stable = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)

func newer(current, latest string) (bool, error) {
	a, b := stable.FindStringSubmatch(current), stable.FindStringSubmatch(latest)
	if a == nil || b == nil {
		return false, fmt.Errorf("cannot compare versions %q and %q; install a stable release first", current, latest)
	}
	for i := 1; i <= 3; i++ {
		x, e := strconv.ParseUint(a[i], 10, 64)
		if e != nil {
			return false, e
		}
		y, e := strconv.ParseUint(b[i], 10, 64)
		if e != nil {
			return false, e
		}
		if x != y {
			return y > x, nil
		}
	}
	return false, nil
}
func fetch(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s (%s)", res.Status, url)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("download exceeds size limit")
	}
	return b, nil
}
func Check(ctx context.Context) (string, bool, error) {
	b, err := fetch(ctx, "https://api.github.com/repos/FredySandoval/sho-rt/releases/latest", 1<<20)
	if err != nil {
		return "", false, err
	}
	var release struct {
		Tag string `json:"tag_name"`
	}
	if err = json.Unmarshal(b, &release); err != nil {
		return "", false, err
	}
	yes, err := newer(Version, release.Tag)
	return release.Tag, yes, err
}

// Run checks for a newer stable version, optionally installing it atomically.
// It does not touch data or restart services. Restart the dashboard after updating.
func Run(ctx context.Context, checkOnly bool) (string, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return "", fmt.Errorf("self-update supports Linux and macOS only")
	}
	latest, available, err := Check(ctx)
	if err != nil {
		return "", err
	}
	if !available {
		return fmt.Sprintf("Already up to date (%s).", Version), nil
	}
	if checkOnly {
		return fmt.Sprintf("Update available: %s → %s", Version, latest), nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	name := fmt.Sprintf("sho-rt_%s_%s_%s.tar.gz", strings.TrimPrefix(latest, "v"), runtime.GOOS, runtime.GOARCH)
	base := repo + latest + "/"
	sums, err := fetch(ctx, base+"checksums.txt", 1<<20)
	if err != nil {
		return "", err
	}
	expected := ""
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[1] == name {
			expected = f[0]
		}
	}
	if len(expected) != 64 {
		return "", fmt.Errorf("missing checksum for %s", name)
	}
	data, err := fetch(ctx, base+name, 128<<20)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != expected {
		return "", fmt.Errorf("checksum mismatch")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e != nil {
			return "", fmt.Errorf("binary missing from archive: %w", e)
		}
		if h.Name != "sho-rt" {
			continue
		}
		if h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > 256<<20 {
			return "", fmt.Errorf("invalid binary in archive")
		}
		break
	}
	f, err := os.CreateTemp(filepath.Dir(exe), ".sho-rt-update-*")
	if err != nil {
		return "", fmt.Errorf("cannot write binary directory (check permissions): %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err = io.Copy(f, tr); err != nil {
		return "", err
	}
	if err = f.Chmod(0755); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(f.Name(), exe); err != nil {
		return "", err
	}
	return fmt.Sprintf("Installed %s. Restart sho-rt and any running background service to use it.", latest), nil
}
