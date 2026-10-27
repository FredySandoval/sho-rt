package daemon

import (
	"github.com/fredy/sho-rt/internal/config"
	"net"
	"path/filepath"
	"testing"
)

func TestProgramLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	c, e := config.Load(path)
	if e != nil {
		t.Fatal(e)
	}
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	c.Port = l.Addr().(*net.TCPAddr).Port
	l.Close()
	if e = config.Save(path, c); e != nil {
		t.Fatal(e)
	}
	p := &Program{Path: path}
	if e = p.Start(nil); e != nil {
		t.Fatal(e)
	}
	if e = p.Stop(nil); e != nil {
		t.Fatal(e)
	}
	p = &Program{Path: path}
	if e = p.Start(nil); e != nil {
		t.Fatal(e)
	}
	if e = p.Stop(nil); e != nil {
		t.Fatal(e)
	}
}
