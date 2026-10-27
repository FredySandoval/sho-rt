package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

// LegacyServiceActive checks the system manager, never the user manager.
func LegacyServiceActive() bool {
	for _, state := range []string{"is-active", "is-enabled"} {
		if exec.Command("systemctl", state, "sho-rt.service").Run() == nil {
			return true
		}
	}
	return false
}

// SetupCommand delegates authentication to sudo on the controlling terminal.
// No password is read or retained by the application.
func SetupCommand(action string) (*exec.Cmd, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("this setup action requires Linux with systemd")
	}
	if os.Geteuid() == 0 {
		return nil, fmt.Errorf("run sho-rt as your normal user, not root")
	}
	switch action {
	case "enable boot startup":
		u, err := user.Current()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(u.Username, "-") {
			return nil, fmt.Errorf("invalid username")
		}
		// Avoid asking for administrator access when lingering is already enabled.
		out, err := exec.Command("loginctl", "show-user", u.Uid, "--property=Linger", "--value").Output()
		if err == nil && strings.TrimSpace(string(out)) == "yes" {
			return exec.Command("systemctl", "--user", "enable", "sho-rt.service"), nil
		}
		return exec.Command("sudo", "--", "loginctl", "enable-linger", u.Username), nil
	case "disable legacy service":
		return exec.Command("sudo", "--", "systemctl", "disable", "--now", "sho-rt.service"), nil
	default:
		return nil, fmt.Errorf("unknown setup action")
	}
}

func EnableBootService() error {
	out, err := exec.Command("systemctl", "--user", "enable", "sho-rt.service").CombinedOutput()
	if err != nil {
		return fmt.Errorf("enable user service: %w: %s", err, out)
	}
	return nil
}
