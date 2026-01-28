package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// View represents a distinct screen in the application.
type View int

const (
	ViewDashboard View = iota
	ViewServices
	ViewPlugins
	ViewNetwork
	ViewLogs
	ViewConfig
)

func (v View) String() string {
	return []string{
		"Dashboard",
		"Services",
		"Plugins",
		"Network",
		"Logs",
		"Config",
	}[v]
}

// FocusRegion represents focusable areas in the UI.
type FocusRegion int

const (
	FocusMainPanel FocusRegion = iota
	FocusSidePanel
	FocusLogs
	FocusInput
	FocusModal
)

// App is the main application model.
type App struct {
	// Dimensions
	width  int
	height int
	ready  bool

	// Navigation
	currentView   View
	focus         FocusRegion
	previousFocus FocusRegion

	// Components
	tabBar    *TabBar
	statusBar *StatusBar
	helpBar   *HelpBar

	// Lists for different views
	serviceList *List
	pluginList  *List
	networkList *List
	logList     *List

	// Panels
	detailPanel *Panel
	statsPanel  *Panel
	logsPanel   *Panel

	// Input
	input        textinput.Model
	inputActive  bool
	inputPurpose string

	// Modal
	modalVisible bool
	modalTitle   string
	modalContent string
	modalType    ModalType

	// Data
	services []ServiceItem
	plugins  []PluginItem
	networks []NetworkItem
	logs     []LogEntry
	stats    SystemStats

	// State
	lastUpdate time.Time
	ticker     *time.Ticker
	quitting   bool

	// Styling
	theme  Theme
	styles Styles
	keymap KeyMap
}

// ModalType represents different modal dialogs.
type ModalType int

const (
	ModalInfo ModalType = iota
	ModalConfirm
	ModalInput
	ModalForm
)

// Message types for tea.Cmd
type tickMsg time.Time
type refreshMsg struct{}
type serviceUpdateMsg []ServiceItem
type logUpdateMsg []LogEntry
type statsUpdateMsg SystemStats

// ServiceItem implements ListItem for services.
type ServiceItem struct {
	ID          string
	Name        string
	Desc        string
	URL         string
	Zone        string
	StatusStr   string
	Latency     time.Duration
	RequestCnt  int64
	LastChecked time.Time
}

func (s ServiceItem) FilterValue() string { return s.Name + " " + s.Desc }
func (s ServiceItem) Title() string       { return s.Name }
func (s ServiceItem) Description() string { return s.URL }
func (s ServiceItem) Status() string      { return s.StatusStr }

// PluginItem implements ListItem for plugins.
type PluginItem struct {
	Name      string
	Desc      string
	Version   string
	StatusStr string
	Loaded    bool
}

func (p PluginItem) FilterValue() string { return p.Name + " " + p.Desc }
func (p PluginItem) Title() string       { return p.Name }
func (p PluginItem) Description() string { return p.Desc + " v" + p.Version }
func (p PluginItem) Status() string      { return p.StatusStr }

// NetworkItem implements ListItem for networks.
type NetworkItem struct {
	SSID      string
	Zone      string
	IP        string
	StatusStr string
	Users     int
}

func (n NetworkItem) FilterValue() string { return n.SSID + " " + n.Zone }
func (n NetworkItem) Title() string       { return n.SSID }
func (n NetworkItem) Description() string { return n.Zone + " • " + n.IP }
func (n NetworkItem) Status() string      { return n.StatusStr }

// LogEntry represents a log line.
type LogEntry struct {
	Time    time.Time
	Level   string
	Source  string
	Message string
}

func (l LogEntry) FilterValue() string { return l.Source + " " + l.Message }
func (l LogEntry) Title() string       { return fmt.Sprintf("[%s] %s", l.Level, l.Source) }
func (l LogEntry) Description() string { return l.Message }
func (l LogEntry) Status() string      { return l.Level }

// SystemStats holds system statistics.
type SystemStats struct {
	Uptime         time.Duration
	TotalRequests  int64
	ActiveServices int
	HealthyNodes   int
	TotalNodes     int
	NetworkID      string
	Zone           string
}

// NewApp creates a new application.
func NewApp() *App {
	theme := DefaultTheme
	styles := NewStyles(theme)
	keymap := DefaultKeyMap()

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = "Type here..."
	ti.CharLimit = 256

	app := &App{
		theme:  theme,
		styles: styles,
		keymap: keymap,

		currentView: ViewDashboard,
		focus:       FocusMainPanel,

		tabBar: NewTabBar([]string{
			"Dashboard", "Services", "Plugins", "Network", "Logs", "Config",
		}),
		statusBar: NewStatusBar(),
		helpBar:   NewHelpBar(nil),

		detailPanel: NewPanel("Details"),
		statsPanel:  NewPanel("Statistics"),
		logsPanel:   NewPanel("Logs"),

		input: ti,

		lastUpdate: time.Now(),
	}

	// Initialize with sample data
	app.initializeSampleData()
	app.createLists()
	app.updateHelpBindings()

	return app
}

func (a *App) initializeSampleData() {
	// Sample services
	a.services = []ServiceItem{
		{ID: "1", Name: "Attendance API", Desc: "Student attendance system", URL: "http://localhost:8081", Zone: "campus", StatusStr: "healthy", Latency: 45 * time.Millisecond, RequestCnt: 1234},
		{ID: "2", Name: "Library Portal", Desc: "Book management", URL: "http://localhost:8082", Zone: "library", StatusStr: "healthy", Latency: 32 * time.Millisecond, RequestCnt: 567},
		{ID: "3", Name: "Cafeteria Menu", Desc: "Daily menu & orders", URL: "http://localhost:8083", Zone: "campus", StatusStr: "degraded", Latency: 250 * time.Millisecond, RequestCnt: 89},
		{ID: "4", Name: "Lab Booking", Desc: "Reserve lab slots", URL: "http://localhost:8084", Zone: "labs", StatusStr: "offline", Latency: 0, RequestCnt: 45},
	}

	// Sample plugins
	a.plugins = []PluginItem{
		{Name: "auth-ldap", Desc: "LDAP authentication provider", Version: "1.2.0", StatusStr: "running", Loaded: true},
		{Name: "metrics", Desc: "Prometheus metrics exporter", Version: "2.0.1", StatusStr: "running", Loaded: true},
		{Name: "rate-limiter", Desc: "Request rate limiting", Version: "1.0.0", StatusStr: "stopped", Loaded: false},
	}

	// Sample networks
	a.networks = []NetworkItem{
		{SSID: "Campus-Main", Zone: "campus", IP: "192.168.1.0/24", StatusStr: "online", Users: 234},
		{SSID: "Library-WiFi", Zone: "library", IP: "192.168.2.0/24", StatusStr: "online", Users: 45},
		{SSID: "Lab-Network", Zone: "labs", IP: "192.168.3.0/24", StatusStr: "online", Users: 12},
		{SSID: "Guest-WiFi", Zone: "guest", IP: "10.0.0.0/24", StatusStr: "degraded", Users: 89},
	}

	// Sample logs
	a.logs = []LogEntry{
		{Time: time.Now().Add(-5 * time.Minute), Level: "INFO", Source: "gateway", Message: "Started HTTP server on :8080"},
		{Time: time.Now().Add(-4 * time.Minute), Level: "INFO", Source: "mdns", Message: "Discovered service: attendance-api"},
		{Time: time.Now().Add(-3 * time.Minute), Level: "WARN", Source: "proxy", Message: "High latency detected for cafeteria-menu (250ms)"},
		{Time: time.Now().Add(-2 * time.Minute), Level: "ERROR", Source: "health", Message: "Service lab-booking failed health check"},
		{Time: time.Now().Add(-1 * time.Minute), Level: "INFO", Source: "auth", Message: "User student123 authenticated from campus zone"},
		{Time: time.Now(), Level: "DEBUG", Source: "registry", Message: "Service metrics updated"},
	}

	// Stats
	a.stats = SystemStats{
		Uptime:         2*time.Hour + 34*time.Minute,
		TotalRequests:  15789,
		ActiveServices: 3,
		HealthyNodes:   3,
		TotalNodes:     4,
		NetworkID:      "campus-mesh-001",
		Zone:           "campus",
	}
}

func (a *App) createLists() {
	// Convert services to ListItems
	serviceItems := make([]ListItem, len(a.services))
	for i := range a.services {
		serviceItems[i] = a.services[i]
	}
	a.serviceList = NewList("Services", serviceItems)

	// Convert plugins to ListItems
	pluginItems := make([]ListItem, len(a.plugins))
	for i := range a.plugins {
		pluginItems[i] = a.plugins[i]
	}
	a.pluginList = NewList("Plugins", pluginItems)

	// Convert networks to ListItems
	networkItems := make([]ListItem, len(a.networks))
	for i := range a.networks {
		networkItems[i] = a.networks[i]
	}
	a.networkList = NewList("Networks", networkItems)

	// Convert logs to ListItems
	logItems := make([]ListItem, len(a.logs))
	for i := range a.logs {
		logItems[i] = a.logs[i]
	}
	a.logList = NewList("Logs", logItems)
}

func (a *App) updateHelpBindings() {
	var bindings []key.Binding
	switch a.currentView {
	case ViewDashboard:
		bindings = []key.Binding{
			a.keymap.Quit,
			a.keymap.Help,
			a.keymap.NextPanel,
			a.keymap.Refresh,
		}
	case ViewServices:
		bindings = []key.Binding{
			a.keymap.New,
			a.keymap.Edit,
			a.keymap.Delete,
			a.keymap.ServiceStart,
			a.keymap.ServiceStop,
		}
	case ViewLogs:
		bindings = []key.Binding{
			a.keymap.LogFollow,
			a.keymap.LogFilter,
			a.keymap.LogLevel,
			a.keymap.LogClear,
		}
	default:
		bindings = []key.Binding{
			a.keymap.Up,
			a.keymap.Down,
			a.keymap.Select,
			a.keymap.Back,
		}
	}
	a.helpBar.SetBindings(bindings)
}

// Init initializes the application.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		a.tickCmd(),
	)
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.updateLayout()
		return a, nil

	case tickMsg:
		a.lastUpdate = time.Time(msg)
		a.stats.Uptime += time.Second
		cmds = append(cmds, a.tickCmd())

	case tea.KeyMsg:
		// Global keys
		if a.modalVisible {
			return a.handleModalInput(msg)
		}

		if a.inputActive {
			return a.handleTextInput(msg)
		}

		switch {
		case key.Matches(msg, a.keymap.Quit):
			a.quitting = true
			return a, tea.Quit

		case key.Matches(msg, a.keymap.Help):
			a.showHelp()

		case key.Matches(msg, a.keymap.NextPanel):
			a.nextFocus()

		case key.Matches(msg, a.keymap.PrevPanel):
			a.prevFocusRegion()

		case key.Matches(msg, a.keymap.FocusMain):
			if a.focus != FocusMainPanel {
				a.focus = FocusMainPanel
			}

		// View switching with numbers (using letter shortcuts)
		case key.Matches(msg, a.keymap.ViewDashboard):
			a.switchView(ViewDashboard)
		case key.Matches(msg, a.keymap.ViewServices):
			a.switchView(ViewServices)
		case key.Matches(msg, a.keymap.ViewPlugins):
			a.switchView(ViewPlugins)
		case key.Matches(msg, a.keymap.ViewNetwork):
			a.switchView(ViewNetwork)
		case key.Matches(msg, a.keymap.ViewLogs):
			a.switchView(ViewLogs)
		case key.Matches(msg, a.keymap.ViewConfig):
			a.switchView(ViewConfig)

		// CRUD operations
		case key.Matches(msg, a.keymap.New):
			a.showNewModal()
		case key.Matches(msg, a.keymap.Edit):
			a.showEditModal()
		case key.Matches(msg, a.keymap.Delete):
			a.showDeleteConfirm()
		case key.Matches(msg, a.keymap.Refresh):
			cmds = append(cmds, a.refreshData())

		// Navigation
		default:
			cmds = append(cmds, a.handleViewInput(msg))
		}
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleViewInput(msg tea.KeyMsg) tea.Cmd {
	var cmd tea.Cmd
	switch a.currentView {
	case ViewServices:
		cmd = a.serviceList.Update(msg)
		a.updateDetailPanel()
	case ViewPlugins:
		cmd = a.pluginList.Update(msg)
	case ViewNetwork:
		cmd = a.networkList.Update(msg)
	case ViewLogs:
		cmd = a.logList.Update(msg)
	}
	return cmd
}

func (a *App) handleModalInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keymap.Back):
		a.modalVisible = false
		return a, nil
	case key.Matches(msg, a.keymap.Select):
		a.modalVisible = false
		// Handle confirmation
		return a, nil
	}
	return a, nil
}

func (a *App) handleTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keymap.Cancel):
		a.inputActive = false
		return a, nil
	case key.Matches(msg, a.keymap.Select):
		a.inputActive = false
		a.processInput()
		return a, nil
	}

	var cmd tea.Cmd
	a.input, cmd = a.input.Update(msg)
	return a, cmd
}

func (a *App) switchView(view View) {
	a.currentView = view
	a.tabBar.SetActive(int(view))
	a.updateHelpBindings()
	a.updateFocusForView()
}

func (a *App) updateFocusForView() {
	switch a.currentView {
	case ViewServices:
		a.serviceList.SetFocused(true)
		a.pluginList.SetFocused(false)
		a.networkList.SetFocused(false)
		a.logList.SetFocused(false)
	case ViewPlugins:
		a.serviceList.SetFocused(false)
		a.pluginList.SetFocused(true)
		a.networkList.SetFocused(false)
		a.logList.SetFocused(false)
	case ViewNetwork:
		a.serviceList.SetFocused(false)
		a.pluginList.SetFocused(false)
		a.networkList.SetFocused(true)
		a.logList.SetFocused(false)
	case ViewLogs:
		a.serviceList.SetFocused(false)
		a.pluginList.SetFocused(false)
		a.networkList.SetFocused(false)
		a.logList.SetFocused(true)
	}
}

func (a *App) nextFocus() {
	switch a.currentView {
	case ViewDashboard:
		// Cycle through dashboard panels: stats -> services -> networks -> logs
		a.focus = (a.focus + 1) % 4
	case ViewServices:
		// Toggle between list and detail panel
		if a.focus == FocusMainPanel {
			a.focus = FocusSidePanel
			a.serviceList.SetFocused(false)
			a.detailPanel.SetFocused(true)
		} else {
			a.focus = FocusMainPanel
			a.serviceList.SetFocused(true)
			a.detailPanel.SetFocused(false)
		}
	default:
		a.focus = (a.focus + 1) % 3
	}
}

func (a *App) prevFocusRegion() {
	switch a.currentView {
	case ViewDashboard:
		a.focus = (a.focus - 1 + 4) % 4
	case ViewServices:
		if a.focus == FocusSidePanel {
			a.focus = FocusMainPanel
			a.serviceList.SetFocused(true)
			a.detailPanel.SetFocused(false)
		} else {
			a.focus = FocusSidePanel
			a.serviceList.SetFocused(false)
			a.detailPanel.SetFocused(true)
		}
	default:
		a.focus = (a.focus - 1 + 3) % 3
	}
}

func (a *App) showHelp() {
	a.modalVisible = true
	a.modalTitle = "Keyboard Shortcuts"
	a.modalType = ModalInfo
	a.modalContent = a.buildHelpContent()
}

func (a *App) buildHelpContent() string {
	return `Navigation
  ↑/k    Move up
  ↓/j    Move down
  Tab    Next panel
  Esc    Back/Cancel

Views
  D      Dashboard
  S      Services
  P      Plugins
  N      Network
  L      Logs
  C      Config

Actions
  n      New item
  e      Edit
  d      Delete
  r      Refresh
  /      Search
  ?      Help`
}

func (a *App) showNewModal() {
	a.modalVisible = true
	a.modalTitle = "Add New Service"
	a.modalType = ModalForm
	a.modalContent = "Enter service details..."
}

func (a *App) showEditModal() {
	a.modalVisible = true
	a.modalTitle = "Edit Service"
	a.modalType = ModalForm
	a.modalContent = "Modify service details..."
}

func (a *App) showDeleteConfirm() {
	a.modalVisible = true
	a.modalTitle = "Confirm Delete"
	a.modalType = ModalConfirm
	a.modalContent = "Are you sure you want to delete this item? This action cannot be undone."
}

func (a *App) processInput() {
	value := a.input.Value()
	switch a.inputPurpose {
	case "filter":
		switch a.currentView {
		case ViewServices:
			a.serviceList.SetFilter(value)
		case ViewLogs:
			a.logList.SetFilter(value)
		}
	}
	a.input.Reset()
}

func (a *App) refreshData() tea.Cmd {
	// In real implementation, this would fetch fresh data
	return func() tea.Msg {
		return refreshMsg{}
	}
}

func (a *App) updateLayout() {
	// Update component sizes based on window dimensions
	a.tabBar.SetWidth(a.width)
	a.statusBar.SetWidth(a.width)
	a.helpBar.SetWidth(a.width)

	// Calculate panel sizes
	mainWidth := a.width * 2 / 3
	sideWidth := a.width - mainWidth
	contentHeight := a.height - 6 // Account for tab bar, status bar, help bar

	a.serviceList.SetSize(mainWidth-2, contentHeight)
	a.pluginList.SetSize(mainWidth-2, contentHeight)
	a.networkList.SetSize(mainWidth-2, contentHeight)
	a.logList.SetSize(a.width-2, contentHeight)

	a.detailPanel.SetSize(sideWidth-2, contentHeight/2)
	a.statsPanel.SetSize(sideWidth-2, contentHeight/2)
}

func (a *App) updateDetailPanel() {
	if item := a.serviceList.SelectedItem(); item != nil {
		if svc, ok := item.(ServiceItem); ok {
			content := fmt.Sprintf(`%s %s

%s URL
  %s

%s Zone
  %s

%s Status
  %s • %v latency

%s Metrics
  %d requests total`,
				Icons.Service, svc.Name,
				Icons.Network, svc.URL,
				Icons.Zone, svc.Zone,
				Icons.Status, svc.StatusStr, svc.Latency,
				Icons.Metrics, svc.RequestCnt,
			)
			a.detailPanel.SetContent(content)
		}
	}
}

// View renders the application.
func (a *App) View() string {
	if !a.ready {
		return "Loading..."
	}

	if a.quitting {
		return ""
	}

	var content string

	// Tab bar
	tabBar := a.tabBar.View()

	// Main content based on current view
	switch a.currentView {
	case ViewDashboard:
		content = a.renderDashboard()
	case ViewServices:
		content = a.renderServicesView()
	case ViewPlugins:
		content = a.renderPluginsView()
	case ViewNetwork:
		content = a.renderNetworkView()
	case ViewLogs:
		content = a.renderLogsView()
	case ViewConfig:
		content = a.renderConfigView()
	}

	// Status bar - show current focus
	focusName := a.getFocusName()
	a.statusBar.SetLeft(fmt.Sprintf(" %s LocalMesh ", Icons.Logo))
	a.statusBar.SetCenter(fmt.Sprintf("%s %s  |  %s %s", Icons.Zone, a.stats.Zone, Icons.ArrowRight, focusName))
	a.statusBar.SetRight(fmt.Sprintf(" %s %d/%d nodes ", Icons.Network, a.stats.HealthyNodes, a.stats.TotalNodes))
	statusBar := a.statusBar.View()

	// Help bar
	helpBar := a.helpBar.View()

	// Combine all parts
	view := lipgloss.JoinVertical(
		lipgloss.Left,
		tabBar,
		content,
		statusBar,
		helpBar,
	)

	// Overlay modal if visible
	if a.modalVisible {
		view = a.overlayModal(view)
	}

	return view
}

func (a *App) renderDashboard() string {
	contentHeight := a.height - 6
	panelWidth := a.width/3 - 2

	// Choose panel style based on focus
	statsPanelStyle := a.styles.Panel
	servicesPanelStyle := a.styles.Panel
	networksPanelStyle := a.styles.Panel
	logsPanelStyle := a.styles.Panel

	// Highlight focused panel
	switch a.focus {
	case 0: // Stats panel
		statsPanelStyle = a.styles.PanelActive
	case 1: // Services panel
		servicesPanelStyle = a.styles.PanelActive
	case 2: // Networks panel
		networksPanelStyle = a.styles.PanelActive
	case 3: // Logs panel
		logsPanelStyle = a.styles.PanelActive
	}

	// Stats section
	statsContent := a.renderStats()
	statsPanel := statsPanelStyle.
		Width(panelWidth).
		Height(contentHeight / 2).
		Render(statsContent)

	// Services summary
	servicesContent := a.renderServicesSummary()
	servicesPanel := servicesPanelStyle.
		Width(panelWidth).
		Height(contentHeight / 2).
		Render(servicesContent)

	// Network summary
	networkContent := a.renderNetworkSummary()
	networkPanel := networksPanelStyle.
		Width(panelWidth).
		Height(contentHeight / 2).
		Render(networkContent)

	// Recent logs
	logsContent := a.renderRecentLogs()
	logsPanel := logsPanelStyle.
		Width(a.width - 2).
		Height(contentHeight / 2).
		Render(logsContent)

	// Layout
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, statsPanel, servicesPanel, networkPanel)

	return lipgloss.JoinVertical(lipgloss.Left, topRow, logsPanel)
}

func (a *App) renderStats() string {
	return fmt.Sprintf(`%s System Statistics

%s Uptime
  %s

%s Total Requests
  %s

%s Network ID
  %s

%s Current Zone
  %s`,
		a.styles.PanelTitle.Render(Icons.Dashboard),
		Icons.Clock, formatDuration(a.stats.Uptime),
		Icons.Metrics, formatNumber(a.stats.TotalRequests),
		Icons.Network, a.stats.NetworkID,
		Icons.Zone, a.stats.Zone,
	)
}

func (a *App) renderServicesSummary() string {
	var b strings.Builder
	b.WriteString(a.styles.PanelTitle.Render(fmt.Sprintf("%s Services", Icons.Service)))
	b.WriteString("\n\n")

	healthy := 0
	degraded := 0
	offline := 0

	for _, svc := range a.services {
		switch svc.StatusStr {
		case "healthy":
			healthy++
		case "degraded":
			degraded++
		default:
			offline++
		}
	}

	b.WriteString(fmt.Sprintf("%s Healthy: %d\n", a.styles.StatusOnline.Render(Icons.Online), healthy))
	b.WriteString(fmt.Sprintf("%s Degraded: %d\n", a.styles.StatusDegraded.Render(Icons.Degraded), degraded))
	b.WriteString(fmt.Sprintf("%s Offline: %d\n", a.styles.StatusOffline.Render(Icons.Offline), offline))

	return b.String()
}

func (a *App) renderNetworkSummary() string {
	var b strings.Builder
	b.WriteString(a.styles.PanelTitle.Render(fmt.Sprintf("%s Networks", Icons.Network)))
	b.WriteString("\n\n")

	totalUsers := 0
	for _, net := range a.networks {
		totalUsers += net.Users
		status := a.styles.StatusOnline.Render(Icons.Online)
		if net.StatusStr != "online" {
			status = a.styles.StatusDegraded.Render(Icons.Degraded)
		}
		b.WriteString(fmt.Sprintf("%s %s (%d users)\n", status, net.SSID, net.Users))
	}

	b.WriteString(fmt.Sprintf("\n%s Total users: %d", Icons.Users, totalUsers))

	return b.String()
}

func (a *App) renderRecentLogs() string {
	var b strings.Builder
	b.WriteString(a.styles.PanelTitle.Render(fmt.Sprintf("%s Recent Activity", Icons.Logs)))
	b.WriteString("\n\n")

	maxLogs := 5
	if len(a.logs) < maxLogs {
		maxLogs = len(a.logs)
	}

	for i := 0; i < maxLogs; i++ {
		log := a.logs[i]
		levelStyle := a.styles.LogInfo
		switch log.Level {
		case "WARN":
			levelStyle = a.styles.LogWarn
		case "ERROR":
			levelStyle = a.styles.LogError
		case "DEBUG":
			levelStyle = a.styles.LogDebug
		}

		timeStr := log.Time.Format("15:04:05")
		b.WriteString(fmt.Sprintf("%s %s %s %s\n",
			a.styles.Muted.Render(timeStr),
			levelStyle.Render(fmt.Sprintf("[%s]", log.Level)),
			a.styles.LogSource.Render(log.Source),
			log.Message,
		))
	}

	return b.String()
}

func (a *App) renderServicesView() string {
	contentHeight := a.height - 6
	mainWidth := a.width * 2 / 3
	sideWidth := a.width - mainWidth

	a.updateDetailPanel()

	left := a.serviceList.View()

	// Detail panel on the right
	a.detailPanel.SetSize(sideWidth-2, contentHeight)
	right := a.detailPanel.View()

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (a *App) renderPluginsView() string {
	return a.pluginList.View()
}

func (a *App) renderNetworkView() string {
	return a.networkList.View()
}

func (a *App) renderLogsView() string {
	return a.logList.View()
}

func (a *App) renderConfigView() string {
	contentHeight := a.height - 6

	configContent := `# LocalMesh Configuration

gateway:
  port: 8080
  host: "0.0.0.0"
  tls:
    enabled: false

auth:
  session_duration: "24h"
  token_secret: "***"

services:
  - name: "attendance-api"
    url: "http://localhost:8081"
    zone: "campus"
  
  - name: "library-portal"
    url: "http://localhost:8082"
    zone: "library"

networks:
  campus:
    ssid: "Campus-Main"
    ip_range: "192.168.1.0/24"
  
  library:
    ssid: "Library-WiFi"
    ip_range: "192.168.2.0/24"`

	return a.styles.Panel.
		Width(a.width - 2).
		Height(contentHeight).
		Render(a.styles.PanelTitle.Render(fmt.Sprintf("%s Configuration", Icons.Config)) + "\n\n" + configContent)
}

func (a *App) overlayModal(base string) string {
	modalWidth := 50
	modalHeight := 20

	if a.width < 60 {
		modalWidth = a.width - 6
	}
	if a.height < 24 {
		modalHeight = a.height - 4
	}

	// Build content
	var content strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A78BFA"))
	content.WriteString(titleStyle.Render(a.modalTitle))
	content.WriteString("\n\n")

	contentLines := strings.Split(a.modalContent, "\n")
	maxLines := modalHeight - 6
	for i, line := range contentLines {
		if i >= maxLines {
			content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("..."))
			break
		}
		content.WriteString(line)
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("Press ESC to close"))

	modalStyle := lipgloss.NewStyle().
		Width(modalWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7C3AED")).
		Padding(1, 2)

	modal := modalStyle.Render(content.String())

	return lipgloss.Place(
		a.width,
		a.height,
		lipgloss.Center,
		lipgloss.Center,
		modal,
	)
}

// Helper functions

// getFocusName returns a human-readable name for the current focus
func (a *App) getFocusName() string {
	switch a.currentView {
	case ViewDashboard:
		switch a.focus {
		case 0:
			return "Statistics"
		case 1:
			return "Services"
		case 2:
			return "Networks"
		case 3:
			return "Logs"
		}
	case ViewServices:
		if a.focus == FocusSidePanel {
			return "Details"
		}
		return "Service List"
	case ViewPlugins:
		return "Plugin List"
	case ViewNetwork:
		return "Network List"
	case ViewLogs:
		return "Log Viewer"
	case ViewConfig:
		return "Configuration"
	}
	return "Main"
}

func formatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dh %dm %ds", h, m, s)
}

func formatNumber(n int64) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// Run starts the TUI application.
func Run() error {
	app := NewApp()
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())
	_, err := p.Run()
	return err
}
