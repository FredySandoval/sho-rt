package server

import (
	"context"
	"github.com/fredy/sho-rt/internal/store"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestRedirects(t *testing.T) {
	db, e := store.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = db.Save(ctx, 0, "https://example.com/path?x=1", "go"); e != nil {
		t.Fatal(e)
	}
	h := Handler(db)
	for _, tc := range []struct {
		method, path string
		code         int
	}{{"GET", "/go", 302}, {"HEAD", "/go", 302}, {"GET", "/missing", 404}, {"POST", "/go", 405}, {"GET", "/", 404}, {"GET", "/a/b", 404}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.code {
			t.Errorf("%+v: got %d", tc, w.Code)
		}
		if tc.code == 302 && w.Header().Get("Location") != "https://example.com/path?x=1" {
			t.Fatal("bad redirect")
		}
	}
	ls, _ := db.List(ctx, "")
	if ls[0].Clicks != 1 {
		t.Fatal("HEAD counted as click")
	}
	db.Toggle(ctx, ls[0].ID)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/go", nil))
	if w.Code != 410 {
		t.Fatal(w.Code)
	}
}
func TestLifecycle(t *testing.T) {
	db, e := store.Open(filepath.Join(t.TempDir(), "db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r := &Runner{}
	if e = r.Start("127.0.0.1:0", db); e != nil {
		t.Fatal(e)
	}
	if e = r.Start("127.0.0.1:0", db); e == nil {
		t.Fatal("duplicate instance")
	}
	if running, _, _ := r.Status(); !running {
		t.Fatal("not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = r.Stop(ctx); e != nil {
		t.Fatal(e)
	}
	if e = r.Stop(ctx); e != nil {
		t.Fatal(e)
	}
	if running, _, _ := r.Status(); running {
		t.Fatal("still running")
	}
	if e = r.Start("127.0.0.1:0", db); e != nil {
		t.Fatal(e)
	}
	r.Stop(ctx)
}
