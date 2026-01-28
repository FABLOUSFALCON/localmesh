package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FormField represents a single form field.
type FormField struct {
	Label       string
	Placeholder string
	Value       string
	Required    bool
	input       textinput.Model
}

// ServiceForm is a form for adding/editing services.
type ServiceForm struct {
	fields       []FormField
	focusedField int
	title        string
	isEdit       bool
	serviceID    string

	width  int
	height int

	styles Styles
	keymap KeyMap
}

// NewServiceForm creates a new service form.
func NewServiceForm(title string) *ServiceForm {
	f := &ServiceForm{
		title:  title,
		styles: NewStyles(DefaultTheme),
		keymap: DefaultKeyMap(),
	}

	// Define form fields
	f.fields = []FormField{
		{Label: "Name", Placeholder: "my-service", Required: true},
		{Label: "URL", Placeholder: "http://localhost:8080", Required: true},
		{Label: "Zone", Placeholder: "campus", Required: false},
		{Label: "Description", Placeholder: "Service description", Required: false},
	}

	// Initialize text inputs
	for i := range f.fields {
		ti := textinput.New()
		ti.Placeholder = f.fields[i].Placeholder
		ti.CharLimit = 256
		if i == 0 {
			ti.Focus()
		}
		f.fields[i].input = ti
	}

	return f
}

// SetService populates the form with existing service data.
func (f *ServiceForm) SetService(id, name, url, zone, desc string) {
	f.isEdit = true
	f.serviceID = id
	if len(f.fields) >= 4 {
		f.fields[0].input.SetValue(name)
		f.fields[1].input.SetValue(url)
		f.fields[2].input.SetValue(zone)
		f.fields[3].input.SetValue(desc)
	}
}

// GetValues returns the form values.
func (f *ServiceForm) GetValues() (name, url, zone, desc string) {
	if len(f.fields) >= 4 {
		return f.fields[0].input.Value(),
			f.fields[1].input.Value(),
			f.fields[2].input.Value(),
			f.fields[3].input.Value()
	}
	return "", "", "", ""
}

// IsValid checks if required fields are filled.
func (f *ServiceForm) IsValid() bool {
	for _, field := range f.fields {
		if field.Required && strings.TrimSpace(field.input.Value()) == "" {
			return false
		}
	}
	return true
}

// SetSize sets form dimensions.
func (f *ServiceForm) SetSize(width, height int) {
	f.width = width
	f.height = height
}

// Update handles input.
func (f *ServiceForm) Update(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, f.keymap.Up):
			f.prevField()
		case key.Matches(msg, f.keymap.Down):
			f.nextField()
		case msg.String() == "tab":
			f.nextField()
		case msg.String() == "shift+tab":
			f.prevField()
		default:
			// Update the focused field
			var cmd tea.Cmd
			f.fields[f.focusedField].input, cmd = f.fields[f.focusedField].input.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return tea.Batch(cmds...)
}

func (f *ServiceForm) nextField() {
	f.fields[f.focusedField].input.Blur()
	f.focusedField = (f.focusedField + 1) % len(f.fields)
	f.fields[f.focusedField].input.Focus()
}

func (f *ServiceForm) prevField() {
	f.fields[f.focusedField].input.Blur()
	f.focusedField = (f.focusedField - 1 + len(f.fields)) % len(f.fields)
	f.fields[f.focusedField].input.Focus()
}

// View renders the form.
func (f *ServiceForm) View() string {
	var b strings.Builder

	// Title
	b.WriteString(f.styles.ModalTitle.Render(f.title))
	b.WriteString("\n\n")

	// Fields
	for i, field := range f.fields {
		label := field.Label
		if field.Required {
			label += " *"
		}

		labelStyle := f.styles.Muted
		inputStyle := f.styles.Input
		if i == f.focusedField {
			labelStyle = f.styles.Bold
			inputStyle = f.styles.InputFocused
		}

		b.WriteString(labelStyle.Render(label))
		b.WriteString("\n")
		b.WriteString(inputStyle.Width(f.width - 4).Render(field.input.View()))
		b.WriteString("\n\n")
	}

	// Buttons hint
	b.WriteString("\n")
	saveBtn := f.styles.ButtonPrimary.Render(" Save (Ctrl+S) ")
	cancelBtn := f.styles.Button.Render(" Cancel (Esc) ")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, saveBtn, "  ", cancelBtn))

	return f.styles.Modal.
		Width(f.width).
		Height(f.height).
		Render(b.String())
}

// ConfirmDialog is a confirmation dialog.
type ConfirmDialog struct {
	title    string
	message  string
	focused  int // 0 = cancel, 1 = confirm
	onYes    func()
	onNo     func()
	styles   Styles
	keymap   KeyMap
	width    int
	height   int
	isDanger bool
}

// NewConfirmDialog creates a new confirmation dialog.
func NewConfirmDialog(title, message string, isDanger bool) *ConfirmDialog {
	return &ConfirmDialog{
		title:    title,
		message:  message,
		isDanger: isDanger,
		styles:   NewStyles(DefaultTheme),
		keymap:   DefaultKeyMap(),
	}
}

// SetSize sets dialog dimensions.
func (d *ConfirmDialog) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// SetCallbacks sets the confirmation callbacks.
func (d *ConfirmDialog) SetCallbacks(onYes, onNo func()) {
	d.onYes = onYes
	d.onNo = onNo
}

// Update handles input.
func (d *ConfirmDialog) Update(msg tea.Msg) (bool, bool) { // confirmed, closed
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, d.keymap.Left):
			d.focused = 0
		case key.Matches(msg, d.keymap.Right):
			d.focused = 1
		case msg.String() == "tab":
			d.focused = (d.focused + 1) % 2
		case key.Matches(msg, d.keymap.Select):
			return d.focused == 1, true
		case key.Matches(msg, d.keymap.Back):
			return false, true
		case msg.String() == "y", msg.String() == "Y":
			return true, true
		case msg.String() == "n", msg.String() == "N":
			return false, true
		}
	}
	return false, false
}

// View renders the dialog.
func (d *ConfirmDialog) View() string {
	var b strings.Builder

	// Icon and title
	icon := Icons.Warning
	if d.isDanger {
		icon = Icons.Delete
	}
	b.WriteString(d.styles.ModalTitle.Render(icon + " " + d.title))
	b.WriteString("\n\n")

	// Message
	b.WriteString(d.styles.Base.Render(d.message))
	b.WriteString("\n\n")

	// Buttons
	cancelStyle := d.styles.Button
	confirmStyle := d.styles.ButtonPrimary
	if d.isDanger {
		confirmStyle = d.styles.ButtonDanger
	}

	if d.focused == 0 {
		cancelStyle = d.styles.ButtonFocused
	} else {
		confirmStyle = d.styles.ButtonFocused
	}

	cancelBtn := cancelStyle.Render(" No (n) ")
	confirmBtn := confirmStyle.Render(" Yes (y) ")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, cancelBtn, "  ", confirmBtn))

	return d.styles.Modal.
		Width(d.width).
		Height(d.height).
		Align(lipgloss.Center).
		Render(b.String())
}

// DetailView shows detailed information about an item.
type DetailView struct {
	title    string
	sections []DetailSection
	width    int
	height   int
	styles   Styles
}

// DetailSection is a section in the detail view.
type DetailSection struct {
	Title  string
	Fields []DetailField
}

// DetailField is a key-value field.
type DetailField struct {
	Key   string
	Value string
	Style lipgloss.Style
}

// NewDetailView creates a new detail view.
func NewDetailView(title string) *DetailView {
	return &DetailView{
		title:  title,
		styles: NewStyles(DefaultTheme),
	}
}

// SetSections sets the detail sections.
func (d *DetailView) SetSections(sections []DetailSection) {
	d.sections = sections
}

// SetSize sets the view dimensions.
func (d *DetailView) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// View renders the detail view.
func (d *DetailView) View() string {
	var b strings.Builder

	b.WriteString(d.styles.PanelTitle.Render(d.title))
	b.WriteString("\n")

	for _, section := range d.sections {
		b.WriteString("\n")
		b.WriteString(d.styles.Bold.Render(section.Title))
		b.WriteString("\n")

		for _, field := range section.Fields {
			valueStyle := field.Style
			if valueStyle.String() == "" {
				valueStyle = d.styles.Base
			}
			b.WriteString(fmt.Sprintf("  %s: %s\n",
				d.styles.Muted.Render(field.Key),
				valueStyle.Render(field.Value),
			))
		}
	}

	return d.styles.Panel.
		Width(d.width).
		Height(d.height).
		Render(b.String())
}

// ToastMessage represents a temporary notification.
type ToastMessage struct {
	Message  string
	Level    string // "info", "success", "warning", "error"
	Duration int    // in ticks
}

// Toast is a temporary notification component.
type Toast struct {
	message  string
	level    string
	visible  bool
	ticks    int
	maxTicks int
	width    int
	styles   Styles
}

// NewToast creates a new toast.
func NewToast() *Toast {
	return &Toast{
		styles: NewStyles(DefaultTheme),
	}
}

// Show displays a toast message.
func (t *Toast) Show(message, level string, duration int) {
	t.message = message
	t.level = level
	t.visible = true
	t.ticks = 0
	t.maxTicks = duration
}

// Tick advances the toast timer.
func (t *Toast) Tick() bool {
	if !t.visible {
		return false
	}
	t.ticks++
	if t.ticks >= t.maxTicks {
		t.visible = false
		return false
	}
	return true
}

// IsVisible returns whether the toast is visible.
func (t *Toast) IsVisible() bool {
	return t.visible
}

// SetWidth sets the toast width.
func (t *Toast) SetWidth(width int) {
	t.width = width
}

// View renders the toast.
func (t *Toast) View() string {
	if !t.visible {
		return ""
	}

	var icon string
	var style lipgloss.Style

	switch t.level {
	case "success":
		icon = Icons.Check
		style = t.styles.StatusOnline
	case "warning":
		icon = Icons.Warning
		style = t.styles.StatusDegraded
	case "error":
		icon = Icons.Cross
		style = t.styles.StatusOffline
	default:
		icon = Icons.Info
		style = t.styles.LogInfo
	}

	content := fmt.Sprintf(" %s %s ", icon, t.message)
	return style.
		Width(t.width).
		Align(lipgloss.Center).
		Render(content)
}

// Breadcrumb shows navigation path.
type Breadcrumb struct {
	items  []string
	styles Styles
}

// NewBreadcrumb creates a new breadcrumb.
func NewBreadcrumb() *Breadcrumb {
	return &Breadcrumb{
		styles: NewStyles(DefaultTheme),
	}
}

// SetItems sets the breadcrumb items.
func (b *Breadcrumb) SetItems(items []string) {
	b.items = items
}

// View renders the breadcrumb.
func (b *Breadcrumb) View() string {
	if len(b.items) == 0 {
		return ""
	}

	var parts []string
	for i, item := range b.items {
		style := b.styles.Muted
		if i == len(b.items)-1 {
			style = b.styles.Bold
		}
		parts = append(parts, style.Render(item))
	}

	return strings.Join(parts, b.styles.Muted.Render(" "+Icons.Arrow+" "))
}
