package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ListItem represents an item that can be displayed in a list.
type ListItem interface {
	FilterValue() string
	Title() string
	Description() string
	Status() string
}

// List is an interactive, scrollable list component.
type List struct {
	items    []ListItem
	filtered []int // indices of filtered items
	cursor   int
	offset   int // scroll offset

	// Dimensions
	width  int
	height int

	// State
	focused  bool
	filter   string
	showHelp bool

	// Configuration
	title      string
	emptyText  string
	showStatus bool
	showDesc   bool
	wrap       bool

	// Styling
	styles Styles
	keymap KeyMap
}

// NewList creates a new list component.
func NewList(title string, items []ListItem) *List {
	l := &List{
		items:      items,
		title:      title,
		emptyText:  "No items",
		showStatus: true,
		showDesc:   true,
		styles:     NewStyles(DefaultTheme),
		keymap:     DefaultKeyMap(),
	}
	l.updateFiltered()
	return l
}

// SetItems updates the list items.
func (l *List) SetItems(items []ListItem) {
	l.items = items
	l.updateFiltered()
	if l.cursor >= len(l.filtered) {
		l.cursor = max(0, len(l.filtered)-1)
	}
}

// SetSize sets the list dimensions.
func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
}

// SetFocused sets focus state.
func (l *List) SetFocused(focused bool) {
	l.focused = focused
}

// IsFocused returns focus state.
func (l *List) IsFocused() bool {
	return l.focused
}

// SelectedItem returns the currently selected item.
func (l *List) SelectedItem() ListItem {
	if len(l.filtered) == 0 {
		return nil
	}
	return l.items[l.filtered[l.cursor]]
}

// SelectedIndex returns the index of the selected item.
func (l *List) SelectedIndex() int {
	if len(l.filtered) == 0 {
		return -1
	}
	return l.filtered[l.cursor]
}

// SetFilter sets the filter string.
func (l *List) SetFilter(filter string) {
	l.filter = strings.ToLower(filter)
	l.updateFiltered()
	l.cursor = 0
	l.offset = 0
}

// updateFiltered updates the filtered indices.
func (l *List) updateFiltered() {
	l.filtered = nil
	for i, item := range l.items {
		if l.filter == "" || strings.Contains(strings.ToLower(item.FilterValue()), l.filter) {
			l.filtered = append(l.filtered, i)
		}
	}
}

// Update handles input.
func (l *List) Update(msg tea.Msg) tea.Cmd {
	if !l.focused {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, l.keymap.Up):
			l.moveUp()
		case key.Matches(msg, l.keymap.Down):
			l.moveDown()
		case key.Matches(msg, l.keymap.PageUp):
			l.pageUp()
		case key.Matches(msg, l.keymap.PageDn):
			l.pageDown()
		case key.Matches(msg, l.keymap.Home):
			l.goToStart()
		case key.Matches(msg, l.keymap.End):
			l.goToEnd()
		}
	}

	return nil
}

func (l *List) moveUp() {
	if l.cursor > 0 {
		l.cursor--
		if l.cursor < l.offset {
			l.offset = l.cursor
		}
	}
}

func (l *List) moveDown() {
	if l.cursor < len(l.filtered)-1 {
		l.cursor++
		visibleHeight := l.visibleHeight()
		if l.cursor >= l.offset+visibleHeight {
			l.offset = l.cursor - visibleHeight + 1
		}
	}
}

func (l *List) pageUp() {
	l.cursor = max(0, l.cursor-l.visibleHeight())
	l.offset = max(0, l.offset-l.visibleHeight())
}

func (l *List) pageDown() {
	l.cursor = min(len(l.filtered)-1, l.cursor+l.visibleHeight())
	l.offset = min(max(0, len(l.filtered)-l.visibleHeight()), l.offset+l.visibleHeight())
}

func (l *List) goToStart() {
	l.cursor = 0
	l.offset = 0
}

func (l *List) goToEnd() {
	l.cursor = max(0, len(l.filtered)-1)
	l.offset = max(0, len(l.filtered)-l.visibleHeight())
}

func (l *List) visibleHeight() int {
	return max(1, l.height-4) // Account for title and borders
}

// View renders the list.
func (l *List) View() string {
	var b strings.Builder

	// Title
	titleStyle := l.styles.PanelTitle
	if l.focused {
		titleStyle = l.styles.PanelTitleFocus
	}
	b.WriteString(titleStyle.Render(l.title))
	b.WriteString("\n")

	// Empty state
	if len(l.filtered) == 0 {
		emptyMsg := l.emptyText
		if l.filter != "" {
			emptyMsg = "No matches for \"" + l.filter + "\""
		}
		b.WriteString(l.styles.Muted.Render("  " + emptyMsg))
		return l.wrapInPanel(b.String())
	}

	// Items
	visibleHeight := l.visibleHeight()
	for i := l.offset; i < min(l.offset+visibleHeight, len(l.filtered)); i++ {
		item := l.items[l.filtered[i]]
		line := l.renderItem(item, i == l.cursor)
		b.WriteString(line)
		if i < min(l.offset+visibleHeight, len(l.filtered))-1 {
			b.WriteString("\n")
		}
	}

	// Scrollbar hint
	if len(l.filtered) > visibleHeight {
		scrollInfo := fmt.Sprintf(" %d/%d ", l.cursor+1, len(l.filtered))
		b.WriteString("\n")
		b.WriteString(l.styles.Muted.Render(scrollInfo))
	}

	return l.wrapInPanel(b.String())
}

func (l *List) renderItem(item ListItem, selected bool) string {
	var icon string
	switch item.Status() {
	case "running", "healthy", "online":
		icon = l.styles.StatusOnline.Render(Icons.Online)
	case "stopped", "offline", "error":
		icon = l.styles.StatusOffline.Render(Icons.Offline)
	case "degraded", "warning":
		icon = l.styles.StatusDegraded.Render(Icons.Degraded)
	default:
		icon = l.styles.StatusUnknown.Render(Icons.Unknown)
	}

	title := item.Title()
	if len(title) > l.width-10 {
		title = title[:l.width-13] + "..."
	}

	line := fmt.Sprintf(" %s %s", icon, title)

	if l.showDesc && item.Description() != "" {
		desc := item.Description()
		if len(desc) > l.width-len(title)-15 {
			desc = desc[:l.width-len(title)-18] + "..."
		}
		line += l.styles.Muted.Render(" " + desc)
	}

	if selected && l.focused {
		return l.styles.ListItemSelected.Render(line)
	} else if selected {
		return l.styles.ListItemActive.Render(line)
	}
	return l.styles.ListItem.Render(line)
}

func (l *List) wrapInPanel(content string) string {
	style := l.styles.Panel
	if l.focused {
		style = l.styles.PanelActive
	}
	return style.Width(l.width).Height(l.height).Render(content)
}

// --- Panel Component ---

// Panel is a bordered container with a title.
type Panel struct {
	title   string
	content string
	width   int
	height  int
	focused bool
	styles  Styles
}

// NewPanel creates a new panel.
func NewPanel(title string) *Panel {
	return &Panel{
		title:  title,
		styles: NewStyles(DefaultTheme),
	}
}

// SetContent sets the panel content.
func (p *Panel) SetContent(content string) {
	p.content = content
}

// SetSize sets panel dimensions.
func (p *Panel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SetFocused sets focus state.
func (p *Panel) SetFocused(focused bool) {
	p.focused = focused
}

// View renders the panel.
func (p *Panel) View() string {
	style := p.styles.Panel
	titleStyle := p.styles.PanelTitle
	if p.focused {
		style = p.styles.PanelActive
		titleStyle = p.styles.PanelTitleFocus
	}

	var b strings.Builder
	if p.title != "" {
		b.WriteString(titleStyle.Render(p.title))
		b.WriteString("\n")
	}
	b.WriteString(p.content)

	return style.Width(p.width).Height(p.height).Render(b.String())
}

// --- Tab Bar Component ---

// TabBar is a horizontal tab navigation component.
type TabBar struct {
	tabs    []string
	active  int
	width   int
	focused bool
	styles  Styles
}

// NewTabBar creates a new tab bar.
func NewTabBar(tabs []string) *TabBar {
	return &TabBar{
		tabs:   tabs,
		styles: NewStyles(DefaultTheme),
	}
}

// SetActive sets the active tab.
func (t *TabBar) SetActive(index int) {
	if index >= 0 && index < len(t.tabs) {
		t.active = index
	}
}

// Active returns the active tab index.
func (t *TabBar) Active() int {
	return t.active
}

// SetWidth sets the tab bar width.
func (t *TabBar) SetWidth(width int) {
	t.width = width
}

// Next moves to the next tab.
func (t *TabBar) Next() {
	t.active = (t.active + 1) % len(t.tabs)
}

// Prev moves to the previous tab.
func (t *TabBar) Prev() {
	t.active = (t.active - 1 + len(t.tabs)) % len(t.tabs)
}

// View renders the tab bar.
func (t *TabBar) View() string {
	var tabs []string
	for i, tab := range t.tabs {
		style := t.styles.TabInactive
		if i == t.active {
			style = t.styles.TabActive
		}
		tabs = append(tabs, style.Render(fmt.Sprintf(" %d %s ", i+1, tab)))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// --- Status Bar Component ---

// StatusBar displays status information at the bottom.
type StatusBar struct {
	left   string
	center string
	right  string
	width  int
	styles Styles
}

// NewStatusBar creates a new status bar.
func NewStatusBar() *StatusBar {
	return &StatusBar{
		styles: NewStyles(DefaultTheme),
	}
}

// SetLeft sets the left content.
func (s *StatusBar) SetLeft(content string) {
	s.left = content
}

// SetCenter sets the center content.
func (s *StatusBar) SetCenter(content string) {
	s.center = content
}

// SetRight sets the right content.
func (s *StatusBar) SetRight(content string) {
	s.right = content
}

// SetWidth sets the status bar width.
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// View renders the status bar.
func (s *StatusBar) View() string {
	leftLen := lipgloss.Width(s.left)
	rightLen := lipgloss.Width(s.right)
	centerLen := lipgloss.Width(s.center)

	// Calculate padding
	totalContent := leftLen + centerLen + rightLen
	if totalContent >= s.width {
		return s.styles.StatusBar.Width(s.width).Render(s.left + s.center + s.right)
	}

	// Distribute remaining space
	remaining := s.width - totalContent
	leftPad := remaining / 2
	rightPad := remaining - leftPad

	return s.styles.StatusBar.Width(s.width).Render(
		s.left +
			strings.Repeat(" ", leftPad) +
			s.center +
			strings.Repeat(" ", rightPad) +
			s.right,
	)
}

// --- Help Bar Component ---

// HelpBar displays keyboard shortcuts.
type HelpBar struct {
	bindings []key.Binding
	width    int
	styles   Styles
}

// NewHelpBar creates a new help bar.
func NewHelpBar(bindings []key.Binding) *HelpBar {
	return &HelpBar{
		bindings: bindings,
		styles:   NewStyles(DefaultTheme),
	}
}

// SetBindings updates the displayed bindings.
func (h *HelpBar) SetBindings(bindings []key.Binding) {
	h.bindings = bindings
}

// SetWidth sets the help bar width.
func (h *HelpBar) SetWidth(width int) {
	h.width = width
}

// View renders the help bar.
func (h *HelpBar) View() string {
	var parts []string
	for _, b := range h.bindings {
		if b.Enabled() {
			help := b.Help()
			part := h.styles.HelpKey.Render(help.Key) +
				h.styles.HelpDesc.Render(" "+help.Desc)
			parts = append(parts, part)
		}
	}
	return h.styles.HelpBar.Width(h.width).Render(
		strings.Join(parts, h.styles.Muted.Render(" • ")),
	)
}

// --- Spinner Component ---

// Spinner is an animated loading indicator.
type Spinner struct {
	frame  int
	styles Styles
}

// NewSpinner creates a new spinner.
func NewSpinner() *Spinner {
	return &Spinner{
		styles: NewStyles(DefaultTheme),
	}
}

// Tick advances the spinner animation.
func (s *Spinner) Tick() {
	s.frame = (s.frame + 1) % len(Icons.Spinner)
}

// View renders the spinner.
func (s *Spinner) View() string {
	return s.styles.Spinner.Render(Icons.Spinner[s.frame])
}

// --- Progress Bar Component ---

// ProgressBar displays a progress indicator.
type ProgressBar struct {
	percent float64
	width   int
	styles  Styles
}

// NewProgressBar creates a new progress bar.
func NewProgressBar() *ProgressBar {
	return &ProgressBar{
		styles: NewStyles(DefaultTheme),
	}
}

// SetPercent sets the progress percentage (0-1).
func (p *ProgressBar) SetPercent(percent float64) {
	if percent < 0 {
		p.percent = 0
	} else if percent > 1 {
		p.percent = 1
	} else {
		p.percent = percent
	}
}

// SetWidth sets the progress bar width.
func (p *ProgressBar) SetWidth(width int) {
	p.width = width
}

// View renders the progress bar.
func (p *ProgressBar) View() string {
	filled := int(float64(p.width) * p.percent)
	empty := p.width - filled

	bar := p.styles.Progress.Render(strings.Repeat("█", filled)) +
		p.styles.Muted.Render(strings.Repeat("░", empty))

	return bar
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
