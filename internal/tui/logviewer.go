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

// LogLevel represents a log severity level.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
	LogLevelAll
)

func (l LogLevel) String() string {
	return []string{"DEBUG", "INFO", "WARN", "ERROR", "ALL"}[l]
}

// LogLine represents a single log entry.
type LogLine struct {
	Timestamp time.Time
	Level     LogLevel
	Source    string
	Message   string
}

// LogViewer is an interactive log viewing component.
type LogViewer struct {
	logs     []LogLine
	filtered []int // indices into logs

	// View state
	offset      int
	cursor      int
	follow      bool
	showDetails bool

	// Filtering
	filterInput  textinput.Model
	filterMode   bool
	filterText   string
	minLevel     LogLevel
	sourceFilter string

	// Dimensions
	width   int
	height  int
	focused bool

	// Configuration
	maxLines   int
	autoScroll bool

	styles Styles
	keymap KeyMap
}

// NewLogViewer creates a new log viewer.
func NewLogViewer() *LogViewer {
	ti := textinput.New()
	ti.Placeholder = "Filter logs..."
	ti.CharLimit = 100

	return &LogViewer{
		filterInput: ti,
		minLevel:    LogLevelAll,
		maxLines:    10000,
		autoScroll:  true,
		follow:      true,
		styles:      NewStyles(DefaultTheme),
		keymap:      DefaultKeyMap(),
	}
}

// SetSize sets the viewer dimensions.
func (v *LogViewer) SetSize(width, height int) {
	v.width = width
	v.height = height
}

// SetFocused sets focus state.
func (v *LogViewer) SetFocused(focused bool) {
	v.focused = focused
}

// IsFocused returns focus state.
func (v *LogViewer) IsFocused() bool {
	return v.focused
}

// AddLog adds a new log entry.
func (v *LogViewer) AddLog(level LogLevel, source, message string) {
	log := LogLine{
		Timestamp: time.Now(),
		Level:     level,
		Source:    source,
		Message:   message,
	}

	v.logs = append(v.logs, log)

	// Trim old logs if needed
	if len(v.logs) > v.maxLines {
		v.logs = v.logs[len(v.logs)-v.maxLines:]
	}

	// Update filtered view
	if v.matchesFilter(log) {
		v.filtered = append(v.filtered, len(v.logs)-1)
	}

	// Auto-scroll if following
	if v.follow {
		v.goToEnd()
	}
}

// AddLogLine adds a pre-constructed log entry.
func (v *LogViewer) AddLogLine(log LogLine) {
	v.logs = append(v.logs, log)

	// Trim old logs if needed
	if len(v.logs) > v.maxLines {
		v.logs = v.logs[len(v.logs)-v.maxLines:]
	}

	// Update filtered view
	if v.matchesFilter(log) {
		v.filtered = append(v.filtered, len(v.logs)-1)
	}

	// Auto-scroll if following
	if v.follow {
		v.goToEnd()
	}
}

// Clear removes all logs.
func (v *LogViewer) Clear() {
	v.logs = nil
	v.filtered = nil
	v.cursor = 0
	v.offset = 0
}

// SetLevelFilter sets the minimum log level.
func (v *LogViewer) SetLevelFilter(level LogLevel) {
	v.minLevel = level
	v.updateFiltered()
}

// CycleLevelFilter cycles through log levels.
func (v *LogViewer) CycleLevelFilter() {
	v.minLevel = (v.minLevel + 1) % 5
	v.updateFiltered()
}

// SetSourceFilter filters by source.
func (v *LogViewer) SetSourceFilter(source string) {
	v.sourceFilter = source
	v.updateFiltered()
}

// ToggleFollow toggles follow mode.
func (v *LogViewer) ToggleFollow() {
	v.follow = !v.follow
	if v.follow {
		v.goToEnd()
	}
}

// IsFollowing returns follow mode state.
func (v *LogViewer) IsFollowing() bool {
	return v.follow
}

// matchesFilter checks if a log matches current filters.
func (v *LogViewer) matchesFilter(log LogLine) bool {
	// Level filter
	if v.minLevel != LogLevelAll && log.Level < v.minLevel {
		return false
	}

	// Source filter
	if v.sourceFilter != "" && !strings.EqualFold(log.Source, v.sourceFilter) {
		return false
	}

	// Text filter
	if v.filterText != "" {
		text := strings.ToLower(v.filterText)
		if !strings.Contains(strings.ToLower(log.Message), text) &&
			!strings.Contains(strings.ToLower(log.Source), text) {
			return false
		}
	}

	return true
}

// updateFiltered rebuilds the filtered indices.
func (v *LogViewer) updateFiltered() {
	v.filtered = nil
	for i, log := range v.logs {
		if v.matchesFilter(log) {
			v.filtered = append(v.filtered, i)
		}
	}

	// Adjust cursor
	if v.cursor >= len(v.filtered) {
		v.cursor = max(0, len(v.filtered)-1)
	}
}

// Update handles input.
func (v *LogViewer) Update(msg tea.Msg) tea.Cmd {
	if !v.focused {
		return nil
	}

	// Handle filter input mode
	if v.filterMode {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, v.keymap.Back):
				v.filterMode = false
				v.filterInput.Blur()
				return nil
			case key.Matches(msg, v.keymap.Select):
				v.filterText = v.filterInput.Value()
				v.filterMode = false
				v.filterInput.Blur()
				v.updateFiltered()
				return nil
			}
		}
		var cmd tea.Cmd
		v.filterInput, cmd = v.filterInput.Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, v.keymap.Up):
			v.moveUp()
			v.follow = false
		case key.Matches(msg, v.keymap.Down):
			v.moveDown()
		case key.Matches(msg, v.keymap.PageUp):
			v.pageUp()
			v.follow = false
		case key.Matches(msg, v.keymap.PageDn):
			v.pageDown()
		case key.Matches(msg, v.keymap.Home):
			v.goToStart()
			v.follow = false
		case key.Matches(msg, v.keymap.End):
			v.goToEnd()
			v.follow = true
		case key.Matches(msg, v.keymap.LogFollow):
			v.ToggleFollow()
		case key.Matches(msg, v.keymap.LogFilter):
			v.filterMode = true
			v.filterInput.Focus()
		case key.Matches(msg, v.keymap.LogLevel):
			v.CycleLevelFilter()
		case key.Matches(msg, v.keymap.LogClear):
			v.Clear()
		case key.Matches(msg, v.keymap.Search):
			v.filterMode = true
			v.filterInput.Focus()
		}
	}

	return nil
}

func (v *LogViewer) moveUp() {
	if v.cursor > 0 {
		v.cursor--
		if v.cursor < v.offset {
			v.offset = v.cursor
		}
	}
}

func (v *LogViewer) moveDown() {
	if v.cursor < len(v.filtered)-1 {
		v.cursor++
		visibleHeight := v.visibleHeight()
		if v.cursor >= v.offset+visibleHeight {
			v.offset = v.cursor - visibleHeight + 1
		}
	}
}

func (v *LogViewer) pageUp() {
	h := v.visibleHeight()
	v.cursor = max(0, v.cursor-h)
	v.offset = max(0, v.offset-h)
}

func (v *LogViewer) pageDown() {
	h := v.visibleHeight()
	v.cursor = min(len(v.filtered)-1, v.cursor+h)
	v.offset = min(max(0, len(v.filtered)-h), v.offset+h)
}

func (v *LogViewer) goToStart() {
	v.cursor = 0
	v.offset = 0
}

func (v *LogViewer) goToEnd() {
	v.cursor = max(0, len(v.filtered)-1)
	v.offset = max(0, len(v.filtered)-v.visibleHeight())
}

func (v *LogViewer) visibleHeight() int {
	return max(1, v.height-4) // Account for header and borders
}

// SelectedLog returns the currently selected log.
func (v *LogViewer) SelectedLog() *LogLine {
	if len(v.filtered) == 0 || v.cursor >= len(v.filtered) {
		return nil
	}
	return &v.logs[v.filtered[v.cursor]]
}

// View renders the log viewer.
func (v *LogViewer) View() string {
	var b strings.Builder

	// Header
	header := v.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Filter input (if active)
	if v.filterMode {
		b.WriteString(v.styles.InputFocused.Width(v.width - 4).Render(v.filterInput.View()))
		b.WriteString("\n")
	}

	// Empty state
	if len(v.filtered) == 0 {
		emptyMsg := "No logs"
		if v.filterText != "" || v.minLevel != LogLevelAll || v.sourceFilter != "" {
			emptyMsg = "No logs match current filters"
		}
		b.WriteString(v.styles.Muted.Render("  " + emptyMsg))
		return v.wrapInPanel(b.String())
	}

	// Log lines
	visibleHeight := v.visibleHeight()
	if v.filterMode {
		visibleHeight-- // Account for filter input
	}

	for i := v.offset; i < min(v.offset+visibleHeight, len(v.filtered)); i++ {
		log := v.logs[v.filtered[i]]
		line := v.renderLogLine(log, i == v.cursor)
		b.WriteString(line)
		if i < min(v.offset+visibleHeight, len(v.filtered))-1 {
			b.WriteString("\n")
		}
	}

	// Scrollbar hint
	if len(v.filtered) > visibleHeight {
		scrollInfo := fmt.Sprintf(" %d/%d ", v.cursor+1, len(v.filtered))
		if v.follow {
			scrollInfo += Icons.ArrowDown + " FOLLOW"
		}
		b.WriteString("\n")
		b.WriteString(v.styles.Muted.Render(scrollInfo))
	}

	return v.wrapInPanel(b.String())
}

func (v *LogViewer) renderHeader() string {
	title := Icons.Log + " Logs"

	// Status indicators
	var indicators []string

	// Level filter
	levelIndicator := fmt.Sprintf("[Level: %s]", v.minLevel.String())
	indicators = append(indicators, levelIndicator)

	// Follow mode
	if v.follow {
		indicators = append(indicators, v.styles.StatusOnline.Render("[FOLLOW]"))
	}

	// Active filters
	if v.filterText != "" {
		indicators = append(indicators, fmt.Sprintf("[Filter: %s]", v.filterText))
	}
	if v.sourceFilter != "" {
		indicators = append(indicators, fmt.Sprintf("[Source: %s]", v.sourceFilter))
	}

	titleStyle := v.styles.PanelTitle
	if v.focused {
		titleStyle = v.styles.PanelTitleFocus
	}

	return titleStyle.Render(title) + " " + v.styles.Muted.Render(strings.Join(indicators, " "))
}

func (v *LogViewer) renderLogLine(log LogLine, selected bool) string {
	// Timestamp
	timeStr := log.Timestamp.Format("15:04:05.000")

	// Level with color
	var levelStyle lipgloss.Style
	switch log.Level {
	case LogLevelDebug:
		levelStyle = v.styles.LogDebug
	case LogLevelInfo:
		levelStyle = v.styles.LogInfo
	case LogLevelWarn:
		levelStyle = v.styles.LogWarn
	case LogLevelError:
		levelStyle = v.styles.LogError
	default:
		levelStyle = v.styles.Muted
	}

	levelStr := fmt.Sprintf("[%-5s]", log.Level.String())

	// Format line
	line := fmt.Sprintf("%s %s %s %s",
		v.styles.Muted.Render(timeStr),
		levelStyle.Render(levelStr),
		v.styles.LogSource.Render(fmt.Sprintf("%-12s", log.Source)),
		log.Message,
	)

	// Truncate if needed
	maxLen := v.width - 4
	if len(line) > maxLen {
		line = line[:maxLen-3] + "..."
	}

	if selected && v.focused {
		return v.styles.ListItemSelected.Render(line)
	} else if selected {
		return v.styles.ListItemActive.Render(line)
	}
	return line
}

func (v *LogViewer) wrapInPanel(content string) string {
	style := v.styles.Panel
	if v.focused {
		style = v.styles.PanelActive
	}
	return style.Width(v.width).Height(v.height).Render(content)
}

// LogBuffer is a ring buffer for storing logs efficiently.
type LogBuffer struct {
	logs  []LogLine
	head  int
	tail  int
	count int
	cap   int
}

// NewLogBuffer creates a new log buffer with given capacity.
func NewLogBuffer(capacity int) *LogBuffer {
	return &LogBuffer{
		logs: make([]LogLine, capacity),
		cap:  capacity,
	}
}

// Push adds a log to the buffer.
func (b *LogBuffer) Push(log LogLine) {
	b.logs[b.tail] = log
	b.tail = (b.tail + 1) % b.cap
	if b.count < b.cap {
		b.count++
	} else {
		b.head = (b.head + 1) % b.cap
	}
}

// Get returns all logs in order.
func (b *LogBuffer) Get() []LogLine {
	result := make([]LogLine, b.count)
	for i := 0; i < b.count; i++ {
		result[i] = b.logs[(b.head+i)%b.cap]
	}
	return result
}

// Len returns the number of logs.
func (b *LogBuffer) Len() int {
	return b.count
}

// Clear empties the buffer.
func (b *LogBuffer) Clear() {
	b.head = 0
	b.tail = 0
	b.count = 0
}

// ParseLogLevel converts a string to LogLevel.
func ParseLogLevel(s string) LogLevel {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARN", "WARNING":
		return LogLevelWarn
	case "ERROR", "ERR":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}
