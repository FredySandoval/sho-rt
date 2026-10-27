package tui

import (
	lipgloss "charm.land/lipgloss/v2"
	"strings"
)

var settingsActions = []struct{ label, key, description string }{
	{"Check for updates", "c", "Check GitHub for a newer stable release."},
	{"Update application", "p", "Install a verified release. Restart the dashboard and background service afterward."},
	{"Import links from JSON", "o", "Read a JSON file, preview changes, and skip or overwrite conflicting aliases."},
	{"Export links to JSON", "f", "Export all links (without analytics) to a portable JSON file."},
	{"Edit configuration", "e", "Edit the listen address, port, and public domain."},
	{"Start local", "s", "Start the local HTTP server. It stops when you exit sho-rt."},
	{"Stop local", "x", "Stop the local HTTP server without exiting sho-rt."},
	{"Install service", "i", "Install the service (Linux: unprivileged systemd user service). Keep the binary in a permanent location."},
	{"Remove service", "r", "Remove the installed native service."},
	{"Start daemon", "b", "Start the native service in the background. It continues after sho-rt exits."},
	{"Stop daemon", "z", "Stop the native background service."},
	{"Restart daemon", "u", "Restart the native background service."},
	{"Enable boot startup", "v", "Linux: enable startup before login and operation after logout. Requires confirmation; sudo may prompt."},
	{"Disable legacy service", "g", "Linux: stop and disable the old system-wide service before migrating. Preserves its data; sudo may prompt."},
}

func (m *Model) settingsMenu() string {
	cell := func(i int) string {
		text := " " + settingsActions[i].label + " "
		if i == m.settingsAction {
			text = selected.Render(text)
		}
		return lipgloss.NewStyle().Width(25).Render(text)
	}
	lines := []string{cell(0), ""}
	for i := 1; i < len(settingsActions); i += 2 {
		line := cell(i)
		if i+1 < len(settingsActions) {
			if m.w-m.sidebarWidth()-5 < 50 {
				lines = append(lines, line)
				line = cell(i + 1)
			} else {
				line += cell(i + 1)
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
