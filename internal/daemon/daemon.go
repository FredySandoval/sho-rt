package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/fredy/sho-rt/internal/config"
	"github.com/fredy/sho-rt/internal/server"
	"github.com/fredy/sho-rt/internal/store"
	"github.com/kardianos/service"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type Program struct {
	Path   string
	runner server.Runner
	db     *store.Store
}

func (p *Program) Start(service.Service) error {
	c, e := config.Load(p.Path)
	if e != nil {
		return e
	}
	p.db, e = store.Open(c.Database)
	if e != nil {
		return e
	}
	if e = p.runner.Start(c.Listen(), p.db); e != nil {
		p.db.Close()
		return e
	}
	b, _ := json.Marshal(State{Started: time.Now(), Address: c.Listen()})
	if e = os.WriteFile(filepath.Join(filepath.Dir(p.Path), "service-state.json"), b, 0600); e != nil {
		_ = p.Stop(nil)
	}
	return e
}
func (p *Program) Stop(service.Service) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	e := p.runner.Stop(ctx)
	_ = os.Remove(filepath.Join(filepath.Dir(p.Path), "service-state.json"))
	if p.db != nil {
		p.db.Close()
	}
	return e
}
func New(path string, auto bool) (service.Service, error) {
	exe, e := os.Executable()
	if e != nil {
		return nil, e
	}
	startType := "manual"
	if auto {
		startType = "automatic"
	}
	return service.New(&Program{Path: path}, &service.Config{Name: "sho-rt", DisplayName: "sho-rt Link Shortener", Description: "Local short-link redirect server", Executable: exe, Arguments: []string{"--internal-service", path}, Option: service.KeyValue{"UserService": runtime.GOOS == "linux", "RunAtLoad": auto, "KeepAlive": false, "StartType": startType, "OnFailure": "restart"}})
}
func Control(path, action string, auto bool) error {
	if runtime.GOOS == "linux" {
		if os.Geteuid() == 0 {
			return fmt.Errorf("run daemon controls as your normal user, not root")
		}
		if (action == "install" || action == "start" || action == "restart") && LegacyServiceActive() {
			return fmt.Errorf("legacy system service is active or enabled; select Disable legacy service first")
		}
	}
	s, e := New(path, auto)
	if e == nil {
		if action == "uninstall" {
			_ = s.Stop()
		}
		e = service.Control(s, action)
		if e == nil && action == "install" && runtime.GOOS == "linux" && !auto {
			out, err := exec.Command("systemctl", "--user", "disable", "sho-rt.service").CombinedOutput()
			if err != nil {
				e = fmt.Errorf("disable startup: %w: %s", err, out)
			}
		}
	}
	if e != nil {
		return fmt.Errorf("service %s failed: %w (Linux requires a working systemd user session)", action, e)
	}
	return nil
}

// SetStartup reapplies the native service definition and preserves its running state.
func SetStartup(path string, auto bool) error {
	status := Status(path)
	if status != "running" && status != "stopped" {
		return nil
	}
	if err := Control(path, "uninstall", auto); err != nil {
		return err
	}
	if err := Control(path, "install", auto); err != nil {
		return err
	}
	if status == "running" {
		return Control(path, "start", auto)
	}
	return nil
}

type State struct {
	Started time.Time
	Address string
}

func Info(path string) string {
	status := Status(path)
	if status != "running" {
		return status
	}
	b, e := os.ReadFile(filepath.Join(filepath.Dir(path), "service-state.json"))
	if e != nil {
		return status
	}
	var state State
	if json.Unmarshal(b, &state) != nil || state.Started.IsZero() {
		return status
	}
	return fmt.Sprintf("running · %s · %s", time.Since(state.Started).Round(time.Second), state.Address)
}
func Status(path string) string {
	s, e := New(path, true)
	if e != nil {
		return e.Error()
	}
	st, e := s.Status()
	if e != nil {
		return "not installed / unavailable"
	}
	switch st {
	case service.StatusRunning:
		return "running"
	case service.StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}
