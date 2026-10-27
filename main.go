package main

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"fmt"
	"github.com/fredy/sho-rt/internal/config"
	"github.com/fredy/sho-rt/internal/daemon"
	"github.com/fredy/sho-rt/internal/server"
	"github.com/fredy/sho-rt/internal/store"
	"github.com/fredy/sho-rt/internal/tui"
	"os"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "sho-rt:", e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) == 3 && os.Args[1] == "--internal-service" {
		s, e := daemon.New(os.Args[2], true)
		if e != nil {
			return e
		}
		return s.Run()
	}
	path, e := config.Path()
	if e != nil {
		return e
	}
	c, e := config.Load(path)
	if e != nil {
		return e
	}
	db, e := store.Open(c.Database)
	if e != nil {
		return e
	}
	defer db.Close()
	r := &server.Runner{}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		r.Stop(ctx)
	}()
	_, e = tea.NewProgram(tui.New(path, c, db, r)).Run()
	return e
}
