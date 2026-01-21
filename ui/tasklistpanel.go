package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"open-coder/pkg/agentic"
)

// TaskListPanel displays all agentic tasks in the project
type TaskListPanel struct {
	Tasks         []agentic.TaskSummary
	SelectedIndex int
	ScrollOffset  int
	Width         int
	Height        int
	Styles        Styles
	FilterStatus  agentic.TaskStatus // "" for all
	Loading       bool
	Error         error
}

// NewTaskListPanel creates a new task list panel
func NewTaskListPanel(styles Styles) *TaskListPanel {
	return &TaskListPanel{
		Styles:        styles,
		SelectedIndex: 0,
		ScrollOffset:  0,
	}
}

// SetTasks sets the tasks to display
func (p *TaskListPanel) SetTasks(tasks []agentic.TaskSummary) {
	p.Tasks = tasks
	p.SelectedIndex = 0
	p.ScrollOffset = 0
	p.Loading = false
	p.Error = nil
}

// SetSize updates the panel dimensions
func (p *TaskListPanel) SetSize(width, height int) {
	p.Width = width
	p.Height = height
}

// Update handles input for task list panel
func (p *TaskListPanel) Update(msg tea.Msg) (*TaskListPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return p.handleKey(msg)
	}
	return p, nil
}

// handleKey processes keyboard input
func (p *TaskListPanel) handleKey(msg tea.KeyMsg) (*TaskListPanel, tea.Cmd) {
	if len(p.Tasks) == 0 {
		return p, nil
	}

	switch msg.String() {
	case "j", "down":
		if p.SelectedIndex < len(p.Tasks)-1 {
			p.SelectedIndex++
			p.ensureVisible()
		}
	case "k", "up":
		if p.SelectedIndex > 0 {
			p.SelectedIndex--
			p.ensureVisible()
		}
	case "g":
		p.SelectedIndex = 0
		p.ScrollOffset = 0
	case "G":
		p.SelectedIndex = len(p.Tasks) - 1
		p.ensureVisible()
	case "enter", " ":
		if p.SelectedIndex < len(p.Tasks) {
			taskID := p.Tasks[p.SelectedIndex].ID
			return p, func() tea.Msg {
				return AgenticTaskSelectedMsg{TaskID: taskID}
			}
		}
	case "d":
		// Delete selected task
		if p.SelectedIndex < len(p.Tasks) {
			taskID := p.Tasks[p.SelectedIndex].ID
			return p, func() tea.Msg {
				return AgenticDeleteTaskMsg{TaskID: taskID}
			}
		}
	case "r":
		// Refresh list
		return p, func() tea.Msg {
			return AgenticLoadTaskListMsg{}
		}
	case "f":
		// Cycle through filters
		switch p.FilterStatus {
		case "":
			p.FilterStatus = agentic.TaskCompleted
		case agentic.TaskCompleted:
			p.FilterStatus = agentic.TaskFailed
		case agentic.TaskFailed:
			p.FilterStatus = agentic.TaskPaused
		case agentic.TaskPaused:
			p.FilterStatus = ""
		default:
			p.FilterStatus = ""
		}
	}
	return p, nil
}

// ensureVisible adjusts scroll to keep selected item visible
func (p *TaskListPanel) ensureVisible() {
	visibleLines := p.Height - 10
	if visibleLines < 3 {
		visibleLines = 3
	}

	if p.SelectedIndex < p.ScrollOffset {
		p.ScrollOffset = p.SelectedIndex
	} else if p.SelectedIndex >= p.ScrollOffset+visibleLines {
		p.ScrollOffset = p.SelectedIndex - visibleLines + 1
	}
}

// View renders the task list panel
func (p *TaskListPanel) View() string {
	var sections []string

	// Header
	sections = append(sections, p.renderHeader())

	// Task list or empty state
	if p.Loading {
		sections = append(sections, p.renderLoading())
	} else if p.Error != nil {
		sections = append(sections, p.renderError())
	} else if len(p.Tasks) == 0 {
		sections = append(sections, p.renderEmpty())
	} else {
		sections = append(sections, p.renderTaskList())
	}

	// Footer
	sections = append(sections, p.renderFooter())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Apply panel style
	panelStyle := lipgloss.NewStyle().
		Width(p.Width).
		Height(p.Height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(p.Styles.Theme.Primary).
		Padding(1, 2)

	return panelStyle.Render(content)
}

// renderHeader renders the panel header
func (p *TaskListPanel) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Primary).
		PaddingBottom(1)

	title := titleStyle.Render("📚 Agentic Task History")

	// Filter indicator
	var filterText string
	if p.FilterStatus != "" {
		filterStyle := lipgloss.NewStyle().
			Foreground(p.Styles.Theme.Subtle).
			Italic(true)
		filterText = filterStyle.Render(fmt.Sprintf("  (Showing: %s)", p.FilterStatus))
	}

	// Count
	countStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Subtle)
	count := countStyle.Render(fmt.Sprintf("  [%d tasks]", len(p.Tasks)))

	header := lipgloss.JoinHorizontal(lipgloss.Top, title, filterText, count)

	separator := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Border).
		Render(strings.Repeat("─", p.Width-6))

	return lipgloss.JoinVertical(lipgloss.Left, header, separator)
}

// renderLoading renders loading state
func (p *TaskListPanel) renderLoading() string {
	loadingStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Primary).
		Align(lipgloss.Center).
		Width(p.Width - 6).
		PaddingTop(3)

	return loadingStyle.Render("⏳ Loading tasks...")
}

// renderError renders error state
func (p *TaskListPanel) renderError() string {
	errorStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Error).
		Align(lipgloss.Center).
		Width(p.Width - 6).
		PaddingTop(3)

	return errorStyle.Render(fmt.Sprintf("❌ Error: %v", p.Error))
}

// renderEmpty renders empty state
func (p *TaskListPanel) renderEmpty() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Subtle).
		Align(lipgloss.Center).
		Width(p.Width - 6).
		PaddingTop(3)

	content := `No agentic tasks yet.

Use /task <description> to create your first task.

Example:
  /task add user authentication with JWT tokens
  /task refactor the payment module for better performance`

	return emptyStyle.Render(content)
}

// renderTaskList renders the list of tasks
func (p *TaskListPanel) renderTaskList() string {
	var lines []string

	// Table header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Subtle)

	header := headerStyle.Render(fmt.Sprintf("  %-8s %-30s %-12s %-10s %s",
		"ID", "Title", "Status", "Progress", "Created"))
	lines = append(lines, header)

	// Separator
	lines = append(lines, lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Border).
		Render(strings.Repeat("─", p.Width-8)))

	// Filter tasks if filter is set
	visibleTasks := p.Tasks
	if p.FilterStatus != "" {
		var filtered []agentic.TaskSummary
		for _, t := range p.Tasks {
			if t.Status == p.FilterStatus {
				filtered = append(filtered, t)
			}
		}
		visibleTasks = filtered
	}

	// Calculate visible range
	visibleLines := p.Height - 12
	if visibleLines < 3 {
		visibleLines = 3
	}

	startIdx := p.ScrollOffset
	endIdx := startIdx + visibleLines
	if endIdx > len(visibleTasks) {
		endIdx = len(visibleTasks)
	}

	for i := startIdx; i < endIdx; i++ {
		task := visibleTasks[i]
		isSelected := i == p.SelectedIndex

		line := p.renderTaskRow(task, isSelected)
		lines = append(lines, line)
	}

	// Scroll indicator
	if len(visibleTasks) > visibleLines {
		scrollInfo := lipgloss.NewStyle().
			Foreground(p.Styles.Theme.Subtle).
			Italic(true).
			Render(fmt.Sprintf("  [%d-%d of %d]", startIdx+1, endIdx, len(visibleTasks)))
		lines = append(lines, scrollInfo)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderTaskRow renders a single task row
func (p *TaskListPanel) renderTaskRow(task agentic.TaskSummary, selected bool) string {
	// Determine row style
	var style lipgloss.Style
	if selected {
		style = lipgloss.NewStyle().
			Bold(true).
			Foreground(p.Styles.Theme.Primary).
			Background(lipgloss.Color("#21262d"))
	} else {
		style = lipgloss.NewStyle().
			Foreground(p.Styles.Theme.Foreground)
	}

	// Status icon and color
	var statusIcon string
	var statusColor lipgloss.TerminalColor
	switch task.Status {
	case agentic.TaskCompleted:
		statusIcon = "✅"
		statusColor = p.Styles.Theme.Success
	case agentic.TaskFailed:
		statusIcon = "❌"
		statusColor = p.Styles.Theme.Error
	case agentic.TaskPaused:
		statusIcon = "⏸️"
		statusColor = p.Styles.Theme.Warning
	case agentic.TaskCancelled:
		statusIcon = "⛔"
		statusColor = p.Styles.Theme.Error
	case agentic.TaskExecuting:
		statusIcon = "🔄"
		statusColor = p.Styles.Theme.Primary
	case agentic.TaskAwaitingApproval:
		statusIcon = "📝"
		statusColor = p.Styles.Theme.Info
	default:
		statusIcon = "⏳"
		statusColor = p.Styles.Theme.Subtle
	}

	// Truncate title if needed
	title := task.Title
	if len(title) > 28 {
		title = title[:25] + "..."
	}

	// Format progress
	progress := fmt.Sprintf("%.0f%%", task.Progress)

	// Format time
	timeStr := formatRelativeTime(task.CreatedAt)

	// Build row
	prefix := "  "
	if selected {
		prefix = "▸ "
	}

	row := fmt.Sprintf("%s%-8s %-30s %s %-12s %-10s %s",
		prefix,
		task.ID,
		title,
		statusIcon,
		lipgloss.NewStyle().Foreground(statusColor).Render(string(task.Status)),
		progress,
		timeStr,
	)

	return style.Render(row)
}

// renderFooter renders keyboard shortcuts
func (p *TaskListPanel) renderFooter() string {
	separator := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Border).
		Render(strings.Repeat("─", p.Width-6))

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Highlight)

	descStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Subtle)

	hints := []string{
		keyStyle.Render("↑↓/jk") + descStyle.Render(" nav"),
		keyStyle.Render("Enter") + descStyle.Render(" view"),
		keyStyle.Render("d") + descStyle.Render(" delete"),
		keyStyle.Render("f") + descStyle.Render(" filter"),
		keyStyle.Render("r") + descStyle.Render(" refresh"),
		keyStyle.Render("Esc") + descStyle.Render(" close"),
	}

	helpLine := lipgloss.NewStyle().
		PaddingTop(1).
		Render(strings.Join(hints, "  "))

	return lipgloss.JoinVertical(lipgloss.Left, separator, helpLine)
}

// formatRelativeTime formats a time as relative to now
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}

	diff := time.Since(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
	return t.Format("Jan 2")
}

// GetSelectedTask returns the currently selected task
func (p *TaskListPanel) GetSelectedTask() *agentic.TaskSummary {
	if len(p.Tasks) == 0 || p.SelectedIndex >= len(p.Tasks) {
		return nil
	}
	return &p.Tasks[p.SelectedIndex]
}
