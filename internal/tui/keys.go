package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines all keyboard shortcuts for the TUI.
type KeyMap struct {
	// Navigation
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	PageUp key.Binding
	PageDn key.Binding
	Home   key.Binding
	End    key.Binding

	// Panel navigation
	NextPanel key.Binding
	PrevPanel key.Binding
	FocusMain key.Binding

	// Actions
	Select  key.Binding
	Back    key.Binding
	Refresh key.Binding
	Search  key.Binding
	Filter  key.Binding
	Help    key.Binding
	Quit    key.Binding

	// CRUD operations
	New    key.Binding
	Edit   key.Binding
	Delete key.Binding
	Save   key.Binding
	Cancel key.Binding

	// View switching
	ViewDashboard key.Binding
	ViewServices  key.Binding
	ViewPlugins   key.Binding
	ViewNetwork   key.Binding
	ViewLogs      key.Binding
	ViewConfig    key.Binding

	// Quick actions
	ToggleExpand  key.Binding
	ToggleDetails key.Binding
	Copy          key.Binding
	Yank          key.Binding

	// Log specific
	LogFollow key.Binding
	LogFilter key.Binding
	LogLevel  key.Binding
	LogClear  key.Binding
	LogExport key.Binding

	// Service specific
	ServiceStart   key.Binding
	ServiceStop    key.Binding
	ServiceRestart key.Binding
	ServiceLogs    key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation - vim style + arrows
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("PgUp", "page up"),
		),
		PageDn: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("PgDn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("Home/g", "go to top"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("End/G", "go to bottom"),
		),

		// Panel navigation
		NextPanel: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "next panel"),
		),
		PrevPanel: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-Tab", "prev panel"),
		),
		FocusMain: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "focus main"),
		),

		// Actions
		Select: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("Enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("Esc", "back"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r", "ctrl+r"),
			key.WithHelp("r", "refresh"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),

		// CRUD
		New: key.NewBinding(
			key.WithKeys("n", "a"),
			key.WithHelp("n", "new"),
		),
		Edit: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "edit"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d", "x"),
			key.WithHelp("d", "delete"),
		),
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("C-s", "save"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "cancel"),
		),

		// View switching (number keys)
		ViewDashboard: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "dashboard"),
		),
		ViewServices: key.NewBinding(
			key.WithKeys("2", "s"),
			key.WithHelp("2/s", "services"),
		),
		ViewPlugins: key.NewBinding(
			key.WithKeys("3", "p"),
			key.WithHelp("3/p", "plugins"),
		),
		ViewNetwork: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "network"),
		),
		ViewLogs: key.NewBinding(
			key.WithKeys("5", "L"),
			key.WithHelp("5/L", "logs"),
		),
		ViewConfig: key.NewBinding(
			key.WithKeys("6", "c"),
			key.WithHelp("6/c", "config"),
		),

		// Quick actions
		ToggleExpand: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "expand/collapse"),
		),
		ToggleDetails: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "details"),
		),
		Copy: key.NewBinding(
			key.WithKeys("y"),
			key.WithHelp("y", "copy"),
		),
		Yank: key.NewBinding(
			key.WithKeys("Y"),
			key.WithHelp("Y", "yank line"),
		),

		// Log specific
		LogFollow: key.NewBinding(
			key.WithKeys("F"),
			key.WithHelp("F", "follow"),
		),
		LogFilter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		LogLevel: key.NewBinding(
			key.WithKeys("v"),
			key.WithHelp("v", "level"),
		),
		LogClear: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("C-l", "clear"),
		),
		LogExport: key.NewBinding(
			key.WithKeys("ctrl+e"),
			key.WithHelp("C-e", "export"),
		),

		// Service specific
		ServiceStart: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "start"),
		),
		ServiceStop: key.NewBinding(
			key.WithKeys("X"),
			key.WithHelp("X", "stop"),
		),
		ServiceRestart: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "restart"),
		),
		ServiceLogs: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "logs"),
		),
	}
}

// ShortHelp returns keybindings for the mini help view.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Select, k.Help, k.Quit,
	}
}

// FullHelp returns keybindings for the expanded help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right, k.PageUp, k.PageDn},
		{k.NextPanel, k.PrevPanel, k.Select, k.Back},
		{k.New, k.Edit, k.Delete, k.Refresh},
		{k.Search, k.Filter, k.Help, k.Quit},
	}
}

// NavigationHelp returns navigation-specific help.
func (k KeyMap) NavigationHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.NextPanel, k.Select, k.Back,
	}
}

// ViewHelp returns view-switching help.
func (k KeyMap) ViewHelp() []key.Binding {
	return []key.Binding{
		k.ViewDashboard, k.ViewServices, k.ViewPlugins,
		k.ViewNetwork, k.ViewLogs, k.ViewConfig,
	}
}
