package tui

import (
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"context"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/fredy/sho-rt/internal/config"
	"github.com/fredy/sho-rt/internal/daemon"
	"github.com/fredy/sho-rt/internal/server"
	"github.com/fredy/sho-rt/internal/store"
	"strconv"
	"strings"
	"time"
)

// Wireframe palette: charcoal canvas, cool outlines, and a warm footer.
var canvas = lipgloss.Color("#353535")
var foreground = lipgloss.Color("#D6D6D6")
var outline = lipgloss.Color("#AEBBD8")
var warm = lipgloss.Color("#D6D6A8")
var muted = foreground
var accent = lipgloss.NewStyle().Foreground(foreground)
var selected = lipgloss.NewStyle().Background(lipgloss.Color("#DEDEDE")).Foreground(canvas)
var border = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(outline).Padding(0, 1)

type tick time.Time
type noticeExpired time.Time

func pulse() tea.Cmd { return tea.Tick(3*time.Second, func(t time.Time) tea.Msg { return tick(t) }) }
func clearNoticeAfter(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(t time.Time) tea.Msg { return noticeExpired(t) })
}

type Model struct {
	path           string
	c              config.Config
	db             *store.Store
	runner         *server.Runner
	w, h, page     int
	nav            bool
	table          table.Model
	search         textinput.Model
	searching      bool
	links          []store.Link
	stats          store.Stats
	notice         string
	form           *huh.Form
	formKind       string
	editID         int64
	confirm        string
	detail         bool
	serviceStatus  string
	settingsAction int
}

func New(path string, c config.Config, db *store.Store, r *server.Runner) *Model {
	m := &Model{path: path, c: c, db: db, runner: r, w: 110, h: 32}
	m.search = textinput.New()
	m.search.Placeholder = "Type to filter links"
	m.table = table.New(table.WithFocused(true))
	s := table.DefaultStyles()
	s.Header = s.Header.Foreground(foreground).Bold(false)
	s.Cell = s.Cell.PaddingBottom(0)
	s.Selected = selected
	m.table.SetStyles(s)
	m.refresh()
	return m
}
func (m *Model) Init() tea.Cmd { return pulse() }
func (m *Model) refresh() {
	var e error
	m.links, e = m.db.List(context.Background(), m.search.Value())
	if e != nil {
		m.notice = e.Error()
	}
	m.stats, e = m.db.Stats(context.Background())
	if e != nil {
		m.notice = e.Error()
	}
	m.serviceStatus = daemon.Info(m.path)
	m.resize()
}

// titledBorder centers a padded title without changing the panel's width.
func titledBorder(panel, title string, style lipgloss.Style) string {
	lines := strings.Split(panel, "\n")
	edge := style.GetBorderStyle()
	innerWidth := lipgloss.Width(lines[0]) - 2
	heading := " " + ansi.Truncate(title, max(0, innerWidth-2), "…") + " "
	remaining := max(0, innerWidth-lipgloss.Width(heading))
	left := remaining / 2
	lines[0] = lipgloss.NewStyle().Foreground(style.GetBorderTopForeground()).Render(
		edge.TopLeft + strings.Repeat(edge.Top, left) + heading + strings.Repeat(edge.Top, remaining-left) + edge.TopRight,
	)
	return strings.Join(lines, "\n")
}

func (m *Model) sidebarWidth() int { return max(24, m.w/5) }

func (m *Model) resize() {
	// Subtract the sidebar, one-column gap, and the panel's borders/padding.
	width := max(25, m.w-m.sidebarWidth()-5)
	cursor := m.table.Cursor()
	// Column updates render immediately; discard rows with the old column shape.
	m.table.SetRows(nil)
	m.table.SetWidth(width)
	m.table.SetColumns([]table.Column{{Title: "ID", Width: 4}, {Title: "Short Link", Width: max(10, (width-41)/2)}, {Title: "Destination URL", Width: max(10, width-41-(width-41)/2)}, {Title: "Clicks", Width: 7}, {Title: "Status", Width: 8}, {Title: "Created", Width: 10}})
	if m.w < 100 {
		m.table.SetColumns([]table.Column{{Title: "Alias", Width: max(6, width-22)}, {Title: "Clicks", Width: 6}, {Title: "Status", Width: 8}})
	}
	rows := []table.Row{}
	for _, l := range m.links {
		status := "Active"
		if !l.Enabled {
			status = "Disabled"
		}
		if m.w < 100 {
			rows = append(rows, table.Row{l.Alias, fmt.Sprint(l.Clicks), status})
		} else {
			rows = append(rows, table.Row{fmt.Sprint(l.ID), m.c.Domain + "/" + l.Alias, l.Destination, fmt.Sprint(l.Clicks), status, l.Created[:10]})
		}
	}
	m.table.SetRows(rows)
	m.table.SetCursor(max(0, min(cursor, len(rows)-1)))
	m.table.SetHeight(max(3, m.h-11))
	m.search.SetWidth(max(10, width-12))
	if m.form != nil {
		m.form.WithWidth(max(25, width-6)).WithHeight(max(8, m.h-10))
	}
}
func (m *Model) chosen() *store.Link {
	i := m.table.Cursor()
	if i < 0 || i >= len(m.links) {
		return nil
	}
	return &m.links[i]
}
func (m *Model) linkForm(edit bool) tea.Cmd {
	dest, alias := "", ""
	m.editID = 0
	if edit {
		l := m.chosen()
		if l == nil {
			return nil
		}
		dest = l.Destination
		alias = l.Alias
		m.editID = l.ID
	}
	m.formKind = "link"
	m.form = huh.NewForm(huh.NewGroup(huh.NewInput().Key("destination").Title("Destination URL").Value(&dest).Validate(store.ValidateURL), huh.NewInput().Key("alias").Title("Custom alias (optional)").Value(&alias).Validate(func(v string) error {
		if v == "" {
			return nil
		}
		return store.ValidateAlias(v)
	}))).WithWidth(max(25, m.w-40)).WithHeight(max(8, m.h-10))
	return m.form.Init()
}
func (m *Model) settingsForm() tea.Cmd {
	m.formKind = "settings"
	address, port, domain := m.c.Address, strconv.Itoa(m.c.Port), m.c.Domain
	m.form = huh.NewForm(huh.NewGroup(huh.NewInput().Key("address").Title("Listen IP address").Value(&address), huh.NewInput().Key("port").Title("Port").Value(&port).Validate(func(s string) error {
		n, e := strconv.Atoi(s)
		if e != nil || n < 1 || n > 65535 {
			return fmt.Errorf("enter port 1–65535")
		}
		return nil
	}), huh.NewInput().Key("domain").Title("Public domain").Value(&domain))).WithWidth(max(25, m.w-40)).WithHeight(max(8, m.h-10))
	return m.form.Init()
}
func (m *Model) stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.runner.Stop(ctx)
}
func (m *Model) applySettings() error {
	old, e := config.Load(m.path)
	if e != nil {
		return e
	}
	next := m.c
	next.Address = m.form.GetString("address")
	next.Port, _ = strconv.Atoi(m.form.GetString("port"))
	next.Domain = m.form.GetString("domain")
	next.Database = old.Database
	m.c = old
	if e = next.Validate(); e != nil {
		return e
	}
	running, _, _ := m.runner.Status()
	serviceRunning := daemon.Status(m.path) == "running"

	if running {
		if e = m.stop(); e != nil {
			return e
		}
	}
	if e = config.Save(m.path, next); e != nil {
		if running {
			_ = m.runner.Start(old.Listen(), m.db)
		}
		return e
	}

	m.c = next
	if running {
		if e = m.runner.Start(next.Listen(), m.db); e != nil {
			// Restore the working listener if the new address cannot be bound.
			next.Address, next.Port = old.Address, old.Port
			_ = config.Save(m.path, next)
			m.c = next
			_ = m.runner.Start(old.Listen(), m.db)
			return e
		}
		return nil
	}
	if serviceRunning {
		return daemon.Control(m.path, "restart", next.AutoStart)
	}
	return nil
}

type serviceSetupResult struct {
	action string
	err    error
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if expired, ok := msg.(noticeExpired); ok {
		if time.Since(time.Time(expired)) <= time.Second {
			m.notice = ""
		}
		return m, nil
	}
	if result, ok := msg.(serviceSetupResult); ok {
		err := result.err
		if err == nil && result.action == "enable boot startup" {
			err = daemon.EnableBootService()
		}
		m.feedback(err)
		m.refresh()
		return m, nil
	}
	if s, ok := msg.(tea.WindowSizeMsg); ok {
		m.w = s.Width
		m.h = s.Height
		m.resize()
		return m, nil
	}
	if _, ok := msg.(tick); ok {
		if m.form == nil {
			m.refresh()
		}
		return m, pulse()
	}
	key, iskey := msg.(tea.KeyPressMsg)
	if iskey && key.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.form != nil {
		if iskey && key.String() == "esc" {
			m.form = nil
			m.c, _ = config.Load(m.path)
			return m, nil
		}
		f, cmd := m.form.Update(msg)
		m.form = f.(*huh.Form)
		if m.form.State == huh.StateCompleted {
			var e error
			if m.formKind == "link" {
				e = m.db.Save(context.Background(), m.editID, m.form.GetString("destination"), m.form.GetString("alias"))
			} else {
				e = m.applySettings()
			}
			m.form = nil
			m.notice = "Saved successfully"
			if e != nil {
				m.notice = e.Error()
			}
			m.refresh()
			if e == nil {
				return m, tea.Batch(cmd, clearNoticeAfter(3*time.Second))
			}
		}
		return m, cmd
	}
	if m.confirm != "" {
		if iskey && key.String() == "y" {
			action := m.confirm
			m.confirm = ""
			var e error
			if action == "enable boot startup" || action == "disable legacy service" {
				cmd, err := daemon.SetupCommand(action)
				if err != nil {
					m.feedback(err)
					return m, nil
				}
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return serviceSetupResult{action, err} })
			}
			if action == "delete" {
				if l := m.chosen(); l != nil {
					e = m.db.Delete(context.Background(), l.ID)
				}
			} else {
				e = daemon.Control(m.path, action, m.c.AutoStart)
			}
			m.feedback(e)
			m.refresh()
		} else if iskey {
			m.confirm = ""
		}
		return m, nil
	}
	if m.searching {
		if iskey && (key.String() == "esc" || key.String() == "enter") {
			m.searching = false
			m.search.Blur()
			return m, nil
		}
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		m.refresh()
		return m, cmd
	}
	if !iskey {
		return m, nil
	}
	k := key.String()
	if m.page == 2 && !m.nav {
		switch k {
		case "up", "k":
			m.settingsAction = (m.settingsAction + len(settingsActions) - 1) % len(settingsActions)
			m.notice = ""
			return m, nil
		case "down", "j":
			m.settingsAction = (m.settingsAction + 1) % len(settingsActions)
			m.notice = ""
			return m, nil
		case "enter":
			k = settingsActions[m.settingsAction].key
		}
	}
	switch k {
	case "q":
		return m, tea.Quit
	case "tab", "shift+tab":
		m.nav = !m.nav
		return m, nil
	case "left", "h":
		m.nav = true
		return m, nil
	case "right", "l":
		m.nav = false
		return m, nil
	case "esc":
		m.detail = false
		m.nav = false
		m.search.SetValue("")
		m.refresh()
		return m, nil
	case "1", "2", "3":
		n, _ := strconv.Atoi(k)
		m.page = n - 1
		m.detail = false
		return m, nil
	case "a":
		return m, m.linkForm(false)
	}
	if m.nav {
		switch k {
		case "up", "k":
			m.page = (m.page + 2) % 3
		case "down", "j":
			m.page = (m.page + 1) % 3
		case "enter":
			m.nav = false
		}
		return m, nil
	}
	if m.page == 0 || m.page == 1 {
		switch k {
		case "/":
			m.searching = true
			return m, m.search.Focus()
		case "e":
			return m, m.linkForm(true)
		case "d":
			if m.chosen() != nil {
				m.confirm = "delete"
			}
		case "t":
			if l := m.chosen(); l != nil {
				m.feedback(m.db.Toggle(context.Background(), l.ID))
				m.refresh()
			}
		case "enter":
			m.detail = !m.detail
		default:
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
	}
	if m.page == 2 {
		var e error
		switch k {
		case "e":
			return m, m.settingsForm()
		case "s":
			e = m.runner.Start(m.c.Listen(), m.db)
		case "x":
			e = m.stop()
		case "i":
			m.confirm = "install"
		case "r":
			m.confirm = "uninstall"
		case "b":
			e = m.stop()
			if e == nil {
				e = daemon.Control(m.path, "start", m.c.AutoStart)
			}
		case "z":
			e = daemon.Control(m.path, "stop", m.c.AutoStart)
		case "u":
			m.confirm = "restart"
		case "v":
			m.confirm = "enable boot startup"
		case "g":
			m.confirm = "disable legacy service"
		}
		m.feedback(e)
		m.refresh()
	}
	return m, nil
}
func (m *Model) feedback(e error) {
	m.notice = "Done"
	if e != nil {
		m.notice = e.Error()
	}
}
func (m *Model) View() tea.View {
	if m.w < 60 || m.h < 18 {
		v := tea.NewView("sho-rt\nPlease resize terminal to at least 60 × 18.\nCtrl+C to quit")
		v.AltScreen = true
		return v
	}
	height := m.h - 5
	sideWidth := m.sidebarWidth()
	labels := []string{"Links", "Statistics", "Settings"}
	side := ""
	for i, label := range labels {
		line := label
		if i == 0 {
			line += strings.Repeat(" ", max(1, sideWidth-6-lipgloss.Width(label)-len(fmt.Sprint(m.stats.Links)))) + fmt.Sprint(m.stats.Links)
		}
		if i == m.page {
			line = selected.Width(sideWidth-4).Padding(0, 1).Render(line)
		} else {
			line = lipgloss.NewStyle().Foreground(muted).Padding(0, 1).Render(line)
		}
		side += line + "\n\n"
	}
	running, up, addr := m.runner.Status()
	status := "Stopped"
	if running {
		status = "Local · " + up.String()
	}
	uptime := "Stopped"
	if running {
		uptime = up.Truncate(time.Second).String()
	}
	metric := func(label, value string) string {
		return lipgloss.NewStyle().Foreground(muted).Render(label) + strings.Repeat(" ", max(1, sideWidth-4-len(label)-len(value))) + value
	}
	summary := metric("Total Links", fmt.Sprint(m.stats.Links)) + "\n" + metric("Total Clicks", fmt.Sprint(m.stats.Clicks)) + "\n" + metric("This Month", fmt.Sprint(m.stats.Month)) + "\n" + metric("Uptime", uptime)
	side += strings.Repeat("\n", max(0, height-lipgloss.Height(side)-5)) + summary
	var content string
	if m.form != nil {
		content = m.form.View()
	} else if m.confirm != "" {
		content = "Confirm " + m.confirm + "?\n\n[y] Yes    [any other key] Cancel"
		if m.confirm == "enable boot startup" {
			content += "\n\nEnables your user service at boot and keeps your user manager running after logout.\nInstall the user service first. Sudo may request authentication in the terminal."
		} else if m.confirm == "disable legacy service" {
			content += "\n\nStops and disables the old system-wide sho-rt service (redirects may be interrupted).\nIts unit file and data are preserved. Sudo may request authentication."
		}
	} else {
		switch m.page {
		case 0:
			content = m.table.View()
			if m.searching || m.search.Value() != "" {
				search := "Search: " + m.search.View()
				content = lipgloss.NewStyle().Width(m.w-sideWidth-5).Align(lipgloss.Right).Render(search) + "\n\n" + content
			}
			if len(m.links) == 0 {
				content += "\nNo links found. Press A to create your first link."
			}
			if m.detail {
				if l := m.chosen(); l != nil {
					content = fmt.Sprintf("Link details\n\nhttps://%s/%s\n\n%s\n\nClicks: %d\nCreated: %s\nEnabled: %t\n\nEsc to return", m.c.Domain, l.Alias, l.Destination, l.Clicks, l.Created, l.Enabled)
				}
			}
		case 1:
			// The table component includes its full viewport height. Use a compact
			// viewport here so the statistics content stays inside the panel.
			m.table.SetHeight(max(3, m.h-18))
			content = fmt.Sprintf("Statistics\n\nTotal links     %d\nTotal clicks    %d\nLast 24 hours   %d\nThis month      %d\n\nPer-link clicks\n%s", m.stats.Links, m.stats.Clicks, m.stats.Recent, m.stats.Month, m.table.View())
			m.table.SetHeight(max(3, m.h-11))
		case 2:
			if !running {
				addr = m.c.Listen()
			}
			content = fmt.Sprintf("Settings\n\nHTTP address     %s\nLocal server     %s\nNative service   %s\nPublic domain    %s\nDatabase         %s\n\n%s", addr, status, m.serviceStatus, m.c.Domain, m.c.Database, m.settingsMenu())
		}
	}
	leftStyle, rightStyle := border, border
	active := border.Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("#C6D3EF"))
	leftFocused := m.nav && m.form == nil && m.confirm == "" && !m.searching
	if leftFocused {
		leftStyle = active
	} else {
		rightStyle = active
	}
	left := leftStyle.Width(sideWidth).Height(height).MaxHeight(height + 2).Render(side)
	right := rightStyle.Width(m.w - sideWidth - 1).Height(height).MaxHeight(height + 2).Render(content)
	left = titledBorder(left, "Link Shortener", leftStyle)
	right = titledBorder(right, labels[m.page], rightStyle)
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	help := "[ ↑k ↓j ←h →l ] Navigate   [ Enter ] View   [ A ] Add   [ E ] Edit   [ D ] Delete   [ / ] Search   [ Q ] Quit"
	if m.w < 120 {
		help = "↑k ↓j ←h →l Navigate · Enter View · A Add · E Edit · D Delete · / Search · Q Quit"
	}
	if m.form != nil {
		help = "Tab Next field • Shift+Tab Previous • Enter Submit • Esc Cancel"
	}
	footerTitle := "Controls"
	if m.page == 2 && m.form == nil && m.confirm == "" {
		footerTitle = "Description"
		help = settingsActions[m.settingsAction].description
	}
	footerText := lipgloss.NewStyle().Foreground(muted).Render(ansi.Truncate(help, m.w-6, "…"))
	if m.notice != "" {
		footerText = ansi.Truncate(m.notice, m.w-6, "…")
	}
	footerStyle := border.BorderForeground(warm)
	footer := footerStyle.Width(m.w).Height(5).Align(lipgloss.Center, lipgloss.Center).Render(footerText)
	footer = titledBorder(footer, footerTitle, footerStyle)
	v := tea.NewView(body + "\n" + footer)
	v.AltScreen = true
	v.BackgroundColor = canvas
	v.ForegroundColor = foreground
	v.WindowTitle = "sho-rt — Link Shortener"
	return v
}
