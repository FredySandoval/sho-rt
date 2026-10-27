package server

import (
	"context"
	"database/sql"
	"errors"
	"github.com/fredy/sho-rt/internal/store"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func Handler(db *store.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", 405)
			return
		}
		alias := strings.TrimPrefix(r.URL.Path, "/")
		if store.ValidateAlias(alias) != nil {
			http.NotFound(w, r)
			return
		}
		dest, e := db.Resolve(r.Context(), alias, r.Method == "GET")
		switch {
		case errors.Is(e, sql.ErrNoRows):
			http.NotFound(w, r)
		case errors.Is(e, store.ErrDisabled):
			http.Error(w, "link disabled", 410)
		case e != nil:
			http.Error(w, "storage unavailable", 503)
		default:
			http.Redirect(w, r, dest, http.StatusFound)
		}
	})
}

type Runner struct {
	mu      sync.Mutex
	srv     *http.Server
	started time.Time
	address string
}

func (r *Runner) Start(address string, db *store.Store) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.srv != nil {
		return errors.New("server already running")
	}
	l, e := net.Listen("tcp", address)
	if e != nil {
		return e
	}
	r.srv = &http.Server{Handler: Handler(db), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	r.started = time.Now()
	r.address = address
	go r.srv.Serve(l)
	return nil
}
func (r *Runner) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.srv == nil {
		return nil
	}
	e := r.srv.Shutdown(ctx)
	if e != nil {
		r.srv.Close()
	}
	r.srv = nil
	return e
}
func (r *Runner) Status() (bool, time.Duration, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.srv == nil {
		return false, 0, ""
	}
	return true, time.Since(r.started).Round(time.Second), r.address
}
