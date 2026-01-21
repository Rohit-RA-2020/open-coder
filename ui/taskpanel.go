package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"open-coder/pkg/agentic"
)

// TaskPanel displays the agentic task view
type TaskPanel struct {
	Task            *agentic.Task
	SelectedIndex   int
	CollapsedPhases map[agentic.TaskPhase]bool
	ScrollOffset    int
	Width           int
	Height          int
	Styles          Styles
	ShowDetails     bool
	ShowLog         bool
	LogScrollOffset int
}

// NewTaskPanel creates a new task panel
func NewTaskPanel(styles Styles) *TaskPanel {
	return &TaskPanel{
		CollapsedPhases: make(map[agentic.TaskPhase]bool),
		Styles:          styles,
	}
}

// SetTask sets the current task to display
func (tp *TaskPanel) SetTask(task *agentic.Task) {
	tp.Task = task
	tp.SelectedIndex = 0
	tp.ScrollOffset = 0
}

// SetSize updates the panel dimensions
func (tp *TaskPanel) SetSize(width, height int) {
	tp.Width = width
	tp.Height = height
}

// Update handles input for task panel
func (tp *TaskPanel) Update(msg tea.Msg) (*TaskPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return tp.handleKey(msg)
	}
	return tp, nil
}

// handleKey processes keyboard input
func (tp *TaskPanel) handleKey(msg tea.KeyMsg) (*TaskPanel, tea.Cmd) {
	if tp.Task == nil {
		return tp, nil
	}

	totalItems := len(tp.Task.Todos)

	switch msg.String() {
	case "j", "down":
		if tp.SelectedIndex < totalItems-1 {
			tp.SelectedIndex++
			tp.ensureVisible()
		}
	case "k", "up":
		if tp.SelectedIndex > 0 {
			tp.SelectedIndex--
			tp.ensureVisible()
		}
	case "g":
		tp.SelectedIndex = 0
		tp.ScrollOffset = 0
	case "G":
		tp.SelectedIndex = totalItems - 1
		tp.ensureVisible()
	case "enter":
		tp.ShowDetails = !tp.ShowDetails
	case "l":
		tp.ShowLog = !tp.ShowLog
	case "tab":
		// Toggle collapse for current phase
		if tp.SelectedIndex < len(tp.Task.Todos) {
			phase := tp.Task.Todos[tp.SelectedIndex].Phase
			tp.CollapsedPhases[phase] = !tp.CollapsedPhases[phase]
		}
	case "p":
		return tp, func() tea.Msg { return AgenticPauseMsg{} }
	case "r":
		return tp, func() tea.Msg { return AgenticResumeMsg{} }
	case "s":
		// Skip current todo
		if tp.SelectedIndex < len(tp.Task.Todos) {
			todoID := tp.Task.Todos[tp.SelectedIndex].ID
			return tp, func() tea.Msg { return AgenticSkipTodoMsg{TodoID: todoID} }
		}
	case "c":
		return tp, func() tea.Msg { return AgenticCancelMsg{} }
	}

	return tp, nil
}

// ensureVisible adjusts scroll to keep selected item visible
func (tp *TaskPanel) ensureVisible() {
	visibleLines := tp.Height - 10 // Account for headers and footers
	if visibleLines < 5 {
		visibleLines = 5
	}

	if tp.SelectedIndex < tp.ScrollOffset {
		tp.ScrollOffset = tp.SelectedIndex
	} else if tp.SelectedIndex >= tp.ScrollOffset+visibleLines {
		tp.ScrollOffset = tp.SelectedIndex - visibleLines + 1
	}
}

// View renders the task panel
func (tp *TaskPanel) View() string {
	if tp.Task == nil {
		return tp.renderEmpty()
	}

	var sections []string

	// Header
	sections = append(sections, tp.renderHeader())

	// Phase sections
	sections = append(sections, tp.renderPhaseSection(agentic.PhasePlanning, "📋 PLANNING"))
	sections = append(sections, tp.renderPhaseSection(agentic.PhaseExecution, "🔧 EXECUTION"))
	sections = append(sections, tp.renderPhaseSection(agentic.PhaseVerification, "🧪 VERIFICATION"))

	// Execution log (if enabled)
	if tp.ShowLog && len(tp.Task.ExecutionLog) > 0 {
		sections = append(sections, tp.renderLog())
	}

	// Footer with key hints
	sections = append(sections, tp.renderFooter())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Apply panel style
	panelStyle := tp.Styles.ChatContainer.
		Width(tp.Width).
		Height(tp.Height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(tp.Styles.Theme.BorderFocused)

	return panelStyle.Render(content)
}

// renderEmpty renders placeholder when no task is active
func (tp *TaskPanel) renderEmpty() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(tp.Styles.Theme.Subtle).
		Align(lipgloss.Center).
		Width(tp.Width).
		Height(tp.Height)

	return emptyStyle.Render("No active task\n\nUse /task <description> to start")
}

// renderHeader renders the task header with progress
func (tp *TaskPanel) renderHeader() string {
	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tp.Styles.Theme.Highlight).
		PaddingLeft(1)

	title := titleStyle.Render(fmt.Sprintf("📌 %s", tp.Task.Title))

	// Phase badge
	phaseStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1).
		MarginLeft(2)

	var phaseBadge string
	switch tp.Task.Phase {
	case agentic.PhasePlanning:
		phaseStyle = phaseStyle.Background(lipgloss.Color("#6e40c9")).Foreground(lipgloss.Color("#ffffff"))
		phaseBadge = phaseStyle.Render("PLANNING")
	case agentic.PhaseExecution:
		phaseStyle = phaseStyle.Background(lipgloss.Color("#f0883e")).Foreground(lipgloss.Color("#ffffff"))
		phaseBadge = phaseStyle.Render("EXECUTING")
	case agentic.PhaseVerification:
		phaseStyle = phaseStyle.Background(lipgloss.Color("#3fb950")).Foreground(lipgloss.Color("#ffffff"))
		phaseBadge = phaseStyle.Render("VERIFYING")
	case agentic.PhaseCompleted:
		phaseStyle = phaseStyle.Background(lipgloss.Color("#58a6ff")).Foreground(lipgloss.Color("#ffffff"))
		phaseBadge = phaseStyle.Render("COMPLETED")
	}

	// Progress bar
	progress := tp.Task.GetProgressPercent()
	completed, total := tp.Task.GetProgress()
	progressBar := tp.renderProgressBar(progress, tp.Width-30)
	progressText := fmt.Sprintf(" %d/%d (%.0f%%)", completed, total, progress)

	header := lipgloss.JoinHorizontal(lipgloss.Top, title, phaseBadge)

	// Status indicator
	var statusIndicator string
	switch tp.Task.Status {
	case agentic.TaskPaused:
		statusIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#d29922")).Render(" ⏸ PAUSED")
	case agentic.TaskCancelled:
		statusIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")).Render(" ⛔ CANCELLED")
	case agentic.TaskFailed:
		statusIndicator = lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")).Render(" ❌ FAILED")
	}

	if statusIndicator != "" {
		header = lipgloss.JoinHorizontal(lipgloss.Top, header, statusIndicator)
	}

	progressLine := lipgloss.NewStyle().
		Foreground(tp.Styles.Theme.Subtle).
		PaddingLeft(1).
		Render(progressBar + progressText)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		progressLine,
		lipgloss.NewStyle().Foreground(tp.Styles.Theme.Border).Render(strings.Repeat("─", tp.Width-4)),
	)
}

// renderProgressBar creates a visual progress bar
func (tp *TaskPanel) renderProgressBar(percent float64, width int) string {
	if width < 10 {
		width = 10
	}
	filled := int(float64(width) * percent / 100)
	empty := width - filled

	filledStyle := lipgloss.NewStyle().Foreground(tp.Styles.Theme.Success)
	emptyStyle := lipgloss.NewStyle().Foreground(tp.Styles.Theme.Subtle)

	return filledStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))
}

// renderPhaseSection renders todos for a specific phase
func (tp *TaskPanel) renderPhaseSection(phase agentic.TaskPhase, label string) string {
	todos := tp.Task.GetTodosByPhase(phase)
	if len(todos) == 0 {
		return ""
	}

	// Check if collapsed
	if tp.CollapsedPhases[phase] {
		completed := 0
		for _, t := range todos {
			if t.IsComplete() {
				completed++
			}
		}
		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(tp.Styles.Theme.Foreground).
			PaddingLeft(1)

		countStyle := lipgloss.NewStyle().
			Foreground(tp.Styles.Theme.Subtle)

		return headerStyle.Render(fmt.Sprintf("▶ %s", label)) +
			countStyle.Render(fmt.Sprintf(" [%d/%d]", completed, len(todos)))
	}

	var lines []string

	// Phase header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tp.Styles.Theme.Foreground).
		PaddingLeft(1)

	completed := 0
	for _, t := range todos {
		if t.IsComplete() {
			completed++
		}
	}

	countStyle := lipgloss.NewStyle().
		Foreground(tp.Styles.Theme.Subtle)

	lines = append(lines,
		headerStyle.Render(fmt.Sprintf("▼ %s", label))+
			countStyle.Render(fmt.Sprintf(" [%d/%d]", completed, len(todos))))

	// Todo items
	for _, todo := range todos {
		line := tp.renderTodoItem(todo)
		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderTodoItem renders a single todo item
func (tp *TaskPanel) renderTodoItem(todo agentic.Todo) string {
	// Find global index for selection
	globalIndex := -1
	for i, t := range tp.Task.Todos {
		if t.ID == todo.ID {
			globalIndex = i
			break
		}
	}

	isSelected := globalIndex == tp.SelectedIndex
	isCurrent := todo.Status == agentic.TodoInProgress

	// Icon
	icon := todo.Status.Icon()

	// Text style based on status
	var textStyle lipgloss.Style
	switch todo.Status {
	case agentic.TodoCompleted:
		textStyle = lipgloss.NewStyle().
			Foreground(tp.Styles.Theme.Success).
			Strikethrough(true)
	case agentic.TodoFailed:
		textStyle = lipgloss.NewStyle().
			Foreground(tp.Styles.Theme.Error)
	case agentic.TodoSkipped:
		textStyle = lipgloss.NewStyle().
			Foreground(tp.Styles.Theme.Subtle).
			Italic(true)
	case agentic.TodoInProgress:
		textStyle = lipgloss.NewStyle().
			Foreground(tp.Styles.Theme.Warning).
			Bold(true)
	default:
		textStyle = lipgloss.NewStyle().
			Foreground(tp.Styles.Theme.Foreground)
	}

	// Selection indicator
	prefix := "  "
	if isSelected {
		prefix = "→ "
		textStyle = textStyle.Underline(true)
	} else if isCurrent {
		prefix = "▶ "
	}

	// Truncate title if needed
	maxWidth := tp.Width - 12
	title := todo.Title
	if len(title) > maxWidth {
		title = title[:maxWidth-3] + "..."
	}

	line := fmt.Sprintf("  %s%s %s", prefix, icon, textStyle.Render(title))

	// Show details if selected and details mode is on
	if isSelected && tp.ShowDetails && todo.Output != "" {
		detailStyle := lipgloss.NewStyle().
			Foreground(tp.Styles.Theme.Subtle).
			PaddingLeft(8)

		output := todo.Output
		if len(output) > 100 {
			output = output[:97] + "..."
		}
		line += "\n" + detailStyle.Render("└─ "+output)
	}

	return line
}

// renderLog renders the execution log
func (tp *TaskPanel) renderLog() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tp.Styles.Theme.Foreground).
		PaddingLeft(1)

	lines := []string{
		lipgloss.NewStyle().Foreground(tp.Styles.Theme.Border).Render(strings.Repeat("─", tp.Width-4)),
		headerStyle.Render("📜 EXECUTION LOG"),
	}

	// Show last N log entries
	maxEntries := 5
	startIdx := len(tp.Task.ExecutionLog) - maxEntries
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(tp.Task.ExecutionLog); i++ {
		entry := tp.Task.ExecutionLog[i]

		var levelStyle lipgloss.Style
		var levelIcon string
		switch entry.Level {
		case "success":
			levelStyle = lipgloss.NewStyle().Foreground(tp.Styles.Theme.Success)
			levelIcon = "✓"
		case "error":
			levelStyle = lipgloss.NewStyle().Foreground(tp.Styles.Theme.Error)
			levelIcon = "✗"
		case "warning":
			levelStyle = lipgloss.NewStyle().Foreground(tp.Styles.Theme.Warning)
			levelIcon = "⚠"
		default:
			levelStyle = lipgloss.NewStyle().Foreground(tp.Styles.Theme.Subtle)
			levelIcon = "•"
		}

		timeStr := entry.Timestamp.Format("15:04:05")
		msg := entry.Message
		if len(msg) > tp.Width-20 {
			msg = msg[:tp.Width-23] + "..."
		}

		logLine := fmt.Sprintf("  %s %s %s",
			lipgloss.NewStyle().Foreground(tp.Styles.Theme.Subtle).Render(timeStr),
			levelStyle.Render(levelIcon),
			msg,
		)
		lines = append(lines, logLine)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderFooter renders keyboard shortcuts
func (tp *TaskPanel) renderFooter() string {
	separator := lipgloss.NewStyle().
		Foreground(tp.Styles.Theme.Border).
		Render(strings.Repeat("─", tp.Width-4))

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(tp.Styles.Theme.Highlight)

	descStyle := lipgloss.NewStyle().
		Foreground(tp.Styles.Theme.Subtle)

	var hints []string

	if tp.Task.Status == agentic.TaskPaused {
		hints = append(hints, keyStyle.Render("[r]")+descStyle.Render("esume"))
	} else if tp.Task.Status == agentic.TaskExecuting {
		hints = append(hints, keyStyle.Render("[p]")+descStyle.Render("ause"))
	}

	hints = append(hints, keyStyle.Render("[s]")+descStyle.Render("kip"))
	hints = append(hints, keyStyle.Render("[c]")+descStyle.Render("ancel"))
	hints = append(hints, keyStyle.Render("[l]")+descStyle.Render("og"))
	hints = append(hints, keyStyle.Render("[Enter]")+descStyle.Render(" details"))
	hints = append(hints, keyStyle.Render("[Esc]")+descStyle.Render(" back"))

	helpLine := lipgloss.NewStyle().
		PaddingLeft(1).
		Render(strings.Join(hints, "  "))

	return lipgloss.JoinVertical(lipgloss.Left, separator, helpLine)
}

// GetSelectedTodo returns the currently selected todo
func (tp *TaskPanel) GetSelectedTodo() *agentic.Todo {
	if tp.Task == nil || tp.SelectedIndex >= len(tp.Task.Todos) {
		return nil
	}
	return &tp.Task.Todos[tp.SelectedIndex]
}
