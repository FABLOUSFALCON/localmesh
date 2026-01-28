// Package tui provides a professional terminal dashboard for LocalMesh.
package tui

import "github.com/charmbracelet/lipgloss"

// Theme defines the color palette and styling for the TUI.
type Theme struct {
	// Primary colors
	Primary    lipgloss.AdaptiveColor
	Secondary  lipgloss.AdaptiveColor
	Accent     lipgloss.AdaptiveColor
	Background lipgloss.AdaptiveColor
	Foreground lipgloss.AdaptiveColor

	// Status colors
	Success lipgloss.AdaptiveColor
	Warning lipgloss.AdaptiveColor
	Error   lipgloss.AdaptiveColor
	Info    lipgloss.AdaptiveColor

	// UI colors
	Border       lipgloss.AdaptiveColor
	BorderActive lipgloss.AdaptiveColor
	Muted        lipgloss.AdaptiveColor
	Highlight    lipgloss.AdaptiveColor
}

// DefaultTheme is the default LocalMesh color scheme.
var DefaultTheme = Theme{
	Primary:    lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"},
	Secondary:  lipgloss.AdaptiveColor{Light: "#0EA5E9", Dark: "#38BDF8"},
	Accent:     lipgloss.AdaptiveColor{Light: "#EC4899", Dark: "#F472B6"},
	Background: lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#0F0F1A"},
	Foreground: lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#E5E7EB"},

	Success: lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#34D399"},
	Warning: lipgloss.AdaptiveColor{Light: "#F59E0B", Dark: "#FBBF24"},
	Error:   lipgloss.AdaptiveColor{Light: "#EF4444", Dark: "#F87171"},
	Info:    lipgloss.AdaptiveColor{Light: "#3B82F6", Dark: "#60A5FA"},

	Border:       lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"},
	BorderActive: lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"},
	Muted:        lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"},
	Highlight:    lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#1F2937"},
}

// Styles contains all pre-configured lipgloss styles.
type Styles struct {
	// Base styles
	Base      lipgloss.Style
	Bold      lipgloss.Style
	Muted     lipgloss.Style
	Highlight lipgloss.Style

	// Header/Title styles
	Logo       lipgloss.Style
	Title      lipgloss.Style
	Subtitle   lipgloss.Style
	StatusBar  lipgloss.Style
	StatusText lipgloss.Style

	// Panel styles
	Panel           lipgloss.Style
	PanelActive     lipgloss.Style
	PanelHeader     lipgloss.Style
	PanelContent    lipgloss.Style
	PanelFooter     lipgloss.Style
	PanelTitle      lipgloss.Style
	PanelTitleFocus lipgloss.Style

	// List styles
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ListItemActive   lipgloss.Style

	// Status indicators
	StatusOnline   lipgloss.Style
	StatusOffline  lipgloss.Style
	StatusDegraded lipgloss.Style
	StatusUnknown  lipgloss.Style

	// Log levels
	LogDebug  lipgloss.Style
	LogInfo   lipgloss.Style
	LogWarn   lipgloss.Style
	LogError  lipgloss.Style
	LogSource lipgloss.Style

	// Tabs
	Tab          lipgloss.Style
	TabActive    lipgloss.Style
	TabInactive  lipgloss.Style
	TabSeparator lipgloss.Style

	// Modal/Dialog
	Modal        lipgloss.Style
	ModalTitle   lipgloss.Style
	ModalContent lipgloss.Style
	ModalFooter  lipgloss.Style

	// Input
	Input        lipgloss.Style
	InputFocused lipgloss.Style
	InputLabel   lipgloss.Style

	// Button
	Button        lipgloss.Style
	ButtonPrimary lipgloss.Style
	ButtonDanger  lipgloss.Style
	ButtonFocused lipgloss.Style

	// Help
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style
	HelpBar  lipgloss.Style

	// Spinner/Progress
	Spinner  lipgloss.Style
	Progress lipgloss.Style

	// Scrollbar
	ScrollThumb lipgloss.Style
	ScrollTrack lipgloss.Style
}

// NewStyles creates styles based on the given theme.
func NewStyles(t Theme) Styles {
	return Styles{
		// Base
		Base:      lipgloss.NewStyle().Foreground(t.Foreground),
		Bold:      lipgloss.NewStyle().Bold(true).Foreground(t.Foreground),
		Muted:     lipgloss.NewStyle().Foreground(t.Muted),
		Highlight: lipgloss.NewStyle().Background(t.Highlight),

		// Logo and headers
		Logo: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(t.Primary).
			Padding(0, 2).
			MarginRight(1),

		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary),

		Subtitle: lipgloss.NewStyle().
			Foreground(t.Muted),

		StatusBar: lipgloss.NewStyle().
			Background(t.Highlight).
			Padding(0, 1),

		StatusText: lipgloss.NewStyle().
			Foreground(t.Muted).
			Padding(0, 1),

		// Panels
		Panel: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		PanelActive: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(t.Primary).
			Padding(0, 1),

		PanelHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary).
			Padding(0, 0, 1, 0),

		PanelContent: lipgloss.NewStyle().
			Padding(0, 1),

		PanelFooter: lipgloss.NewStyle().
			Foreground(t.Muted).
			Padding(1, 0, 0, 0),

		PanelTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Muted).
			Padding(0, 1).
			MarginBottom(1),

		PanelTitleFocus: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary).
			Background(t.Highlight).
			Padding(0, 1).
			MarginBottom(1),

		// List items
		ListItem: lipgloss.NewStyle().
			Padding(0, 1),

		ListItemSelected: lipgloss.NewStyle().
			Background(t.Highlight).
			Foreground(t.Primary).
			Bold(true).
			Padding(0, 1),

		ListItemActive: lipgloss.NewStyle().
			Foreground(t.Primary).
			Padding(0, 1),

		// Status
		StatusOnline: lipgloss.NewStyle().
			Foreground(t.Success).
			Bold(true),

		StatusOffline: lipgloss.NewStyle().
			Foreground(t.Error).
			Bold(true),

		StatusDegraded: lipgloss.NewStyle().
			Foreground(t.Warning).
			Bold(true),

		StatusUnknown: lipgloss.NewStyle().
			Foreground(t.Muted),

		// Log levels
		LogDebug:  lipgloss.NewStyle().Foreground(t.Muted),
		LogInfo:   lipgloss.NewStyle().Foreground(t.Info),
		LogWarn:   lipgloss.NewStyle().Foreground(t.Warning),
		LogError:  lipgloss.NewStyle().Foreground(t.Error),
		LogSource: lipgloss.NewStyle().Foreground(t.Secondary).Bold(true),

		// Tabs
		Tab: lipgloss.NewStyle().
			Padding(0, 2),

		TabActive: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary).
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(t.Primary).
			Padding(0, 2),

		TabInactive: lipgloss.NewStyle().
			Foreground(t.Muted).
			Padding(0, 2),

		TabSeparator: lipgloss.NewStyle().
			Foreground(t.Muted).
			SetString("│"),

		// Modal
		Modal: lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(t.Primary).
			Padding(1, 2).
			Background(t.Background),

		ModalTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary).
			Padding(0, 0, 1, 0),

		ModalContent: lipgloss.NewStyle().
			Padding(1, 0),

		ModalFooter: lipgloss.NewStyle().
			Foreground(t.Muted).
			Padding(1, 0, 0, 0),

		// Input
		Input: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		InputFocused: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Primary).
			Padding(0, 1),

		InputLabel: lipgloss.NewStyle().
			Foreground(t.Muted).
			MarginBottom(1),

		// Button
		Button: lipgloss.NewStyle().
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border),

		ButtonPrimary: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(t.Primary).
			Padding(0, 2),

		ButtonDanger: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(t.Error).
			Padding(0, 2),

		ButtonFocused: lipgloss.NewStyle().
			Bold(true).
			Foreground(t.Primary).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Primary).
			Padding(0, 2),

		// Help
		HelpKey: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(t.Muted),

		HelpBar: lipgloss.NewStyle().
			Foreground(t.Muted).
			Padding(0, 1),

		// Spinner
		Spinner: lipgloss.NewStyle().
			Foreground(t.Primary),

		Progress: lipgloss.NewStyle().
			Foreground(t.Primary),

		// Scrollbar
		ScrollThumb: lipgloss.NewStyle().
			Background(t.Primary),

		ScrollTrack: lipgloss.NewStyle().
			Background(t.Border),
	}
}

// Icons for the TUI
var Icons = struct {
	// Views
	Dashboard  string
	Service    string
	Plugin     string
	Node       string
	Zone       string
	Log        string
	Logs       string
	Config     string
	Network    string
	Navigation string
	Views      string

	// Status
	Online   string
	Offline  string
	Degraded string
	Unknown  string
	Status   string

	// Arrows
	Arrow      string
	ArrowRight string
	ArrowLeft  string
	ArrowUp    string
	ArrowDown  string

	// Actions
	Check    string
	Cross    string
	Warning  string
	Info     string
	Spinner  []string
	Bullet   string
	Dash     string
	Ellipsis string
	Search   string
	Filter   string
	Plus     string
	Minus    string
	Edit     string
	Delete   string
	Refresh  string
	Save     string
	Cancel   string
	Expand   string
	Collapse string

	// Types
	External string
	Internal string
	Lock     string
	Unlock   string
	Stream   string
	Activity string

	// Stats
	Logo    string
	Clock   string
	Metrics string
	Users   string
}{
	// Views
	Dashboard:  "󰕮 ",
	Service:    "󰒍 ",
	Plugin:     "󰏓 ",
	Node:       "󰒋 ",
	Zone:       "󰖟 ",
	Log:        "󰷐 ",
	Logs:       "󰷐 ",
	Config:     "󰒓 ",
	Network:    "󰛳 ",
	Navigation: "󰆾 ",
	Views:      "󰈈 ",

	// Status
	Online:   "●",
	Offline:  "○",
	Degraded: "◐",
	Unknown:  "◌",
	Status:   "󰐾 ",

	// Arrows
	Arrow:      "→",
	ArrowRight: "▶",
	ArrowLeft:  "◀",
	ArrowUp:    "▲",
	ArrowDown:  "▼",

	// Actions
	Check:    "✓",
	Cross:    "✗",
	Warning:  "⚠",
	Info:     "ℹ",
	Spinner:  []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
	Bullet:   "•",
	Dash:     "─",
	Ellipsis: "…",
	Search:   "󰍉 ",
	Filter:   "󰈲 ",
	Plus:     "+",
	Minus:    "-",
	Edit:     "󰏫 ",
	Delete:   "󰆴 ",
	Refresh:  "󰑓 ",
	Save:     "󰆓 ",
	Cancel:   "󰜺 ",
	Expand:   "󰁅 ",
	Collapse: "󰁆 ",

	// Types
	External: "󰏌 ",
	Internal: "󰏗 ",
	Lock:     "󰌾 ",
	Unlock:   "󰌿 ",
	Stream:   "󰐌 ",
	Activity: "󱓞 ",

	// Stats
	Logo:    "󰣀 ",
	Clock:   "󰥔 ",
	Metrics: "󰄪 ",
	Users:   "󰡉 ",
}

// FallbackIcons for terminals without Nerd Fonts
var FallbackIcons = struct {
	Service  string
	Plugin   string
	Node     string
	Zone     string
	Log      string
	Config   string
	Network  string
	Online   string
	Offline  string
	Degraded string
	Unknown  string
	Arrow    string
	Check    string
	Cross    string
	Warning  string
	Info     string
	Search   string
	Plus     string
	Minus    string
}{
	Service:  "📦",
	Plugin:   "🔌",
	Node:     "💻",
	Zone:     "🌐",
	Log:      "📜",
	Config:   "⚙️",
	Network:  "📡",
	Online:   "●",
	Offline:  "○",
	Degraded: "◐",
	Unknown:  "?",
	Arrow:    "→",
	Check:    "✓",
	Cross:    "✗",
	Warning:  "⚠",
	Info:     "ℹ",
	Search:   "🔍",
	Plus:     "+",
	Minus:    "-",
}
