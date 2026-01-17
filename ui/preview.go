package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PreviewChange represents a pending file change
type PreviewChange struct {
	Path            string
	OriginalContent string
	NewContent      string
	Operation       string // "create", "modify", "delete"
	ToolName        string
	DiffLines       []PreviewDiffLine
}

// PreviewDiffLine represents a line in the diff preview
type PreviewDiffLine struct {
	Type      string // "add", "remove", "context", "header"
	OldLineNo int
	NewLineNo int
	Content   string
}

// PreviewPanel displays pending file changes for approval
type PreviewPanel struct {
	Changes      []PreviewChange
	CurrentIndex int
	Offset       int
	Width        int
	Height       int
	Styles       Styles
	ApprovalChan chan bool
	ShowingDiff  bool
	LineOffset   int // For scrolling within a diff
}

// NewPreviewPanel creates a new preview panel
func NewPreviewPanel(styles Styles) *PreviewPanel {
	return &PreviewPanel{
		Changes:      make([]PreviewChange, 0),
		CurrentIndex: 0,
		Offset:       0,
		Styles:       styles,
		ShowingDiff:  false,
		LineOffset:   0,
	}
}

// AddChange adds a pending change for preview
func (p *PreviewPanel) AddChange(path, original, newContent, operation, toolName string) {
	change := PreviewChange{
		Path:            path,
		OriginalContent: original,
		NewContent:      newContent,
		Operation:       operation,
		ToolName:        toolName,
		DiffLines:       p.generateDiff(original, newContent),
	}
	p.Changes = append(p.Changes, change)
}

// generateDiff creates a simple diff between original and new content
func (p *PreviewPanel) generateDiff(original, newContent string) []PreviewDiffLine {
	var lines []PreviewDiffLine

	oldLines := strings.Split(original, "\n")
	newLines := strings.Split(newContent, "\n")

	// Simple line-by-line diff (not a full diff algorithm, but good for preview)
	// For a real implementation, you'd use a proper diff library

	maxLen := len(oldLines)
	if len(newLines) > maxLen {
		maxLen = len(newLines)
	}

	oldIdx, newIdx := 0, 0
	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		if oldIdx >= len(oldLines) {
			// New lines added
			lines = append(lines, PreviewDiffLine{
				Type:      "add",
				NewLineNo: newIdx + 1,
				Content:   newLines[newIdx],
			})
			newIdx++
		} else if newIdx >= len(newLines) {
			// Old lines removed
			lines = append(lines, PreviewDiffLine{
				Type:      "remove",
				OldLineNo: oldIdx + 1,
				Content:   oldLines[oldIdx],
			})
			oldIdx++
		} else if oldLines[oldIdx] == newLines[newIdx] {
			// Lines are the same
			lines = append(lines, PreviewDiffLine{
				Type:      "context",
				OldLineNo: oldIdx + 1,
				NewLineNo: newIdx + 1,
				Content:   oldLines[oldIdx],
			})
			oldIdx++
			newIdx++
		} else {
			// Lines are different - show as remove then add
			lines = append(lines, PreviewDiffLine{
				Type:      "remove",
				OldLineNo: oldIdx + 1,
				Content:   oldLines[oldIdx],
			})
			lines = append(lines, PreviewDiffLine{
				Type:      "add",
				NewLineNo: newIdx + 1,
				Content:   newLines[newIdx],
			})
			oldIdx++
			newIdx++
		}
	}

	return lines
}

// HasPendingChanges returns true if there are pending changes
func (p *PreviewPanel) HasPendingChanges() bool {
	return len(p.Changes) > 0
}

// GetCurrentChange returns the current change being previewed
func (p *PreviewPanel) GetCurrentChange() *PreviewChange {
	if p.CurrentIndex >= 0 && p.CurrentIndex < len(p.Changes) {
		return &p.Changes[p.CurrentIndex]
	}
	return nil
}

// Clear removes all pending changes
func (p *PreviewPanel) Clear() {
	p.Changes = make([]PreviewChange, 0)
	p.CurrentIndex = 0
	p.Offset = 0
	p.LineOffset = 0
}

// AcceptCurrent accepts the current change
func (p *PreviewPanel) AcceptCurrent() *PreviewChange {
	if p.CurrentIndex < len(p.Changes) {
		change := p.Changes[p.CurrentIndex]
		// Remove from list
		p.Changes = append(p.Changes[:p.CurrentIndex], p.Changes[p.CurrentIndex+1:]...)
		if p.CurrentIndex >= len(p.Changes) && p.CurrentIndex > 0 {
			p.CurrentIndex--
		}
		return &change
	}
	return nil
}

// RejectCurrent rejects the current change
func (p *PreviewPanel) RejectCurrent() *PreviewChange {
	if p.CurrentIndex < len(p.Changes) {
		change := p.Changes[p.CurrentIndex]
		// Remove from list
		p.Changes = append(p.Changes[:p.CurrentIndex], p.Changes[p.CurrentIndex+1:]...)
		if p.CurrentIndex >= len(p.Changes) && p.CurrentIndex > 0 {
			p.CurrentIndex--
		}
		return &change
	}
	return nil
}

// AcceptAll accepts all pending changes
func (p *PreviewPanel) AcceptAll() []PreviewChange {
	changes := p.Changes
	p.Clear()
	return changes
}

// RejectAll rejects all pending changes
func (p *PreviewPanel) RejectAll() {
	p.Clear()
}

// Update handles input for the preview panel
func (p *PreviewPanel) Update(msg tea.Msg) (*PreviewPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return p.handleKey(msg)
	}
	return p, nil
}

// handleKey processes keyboard input
func (p *PreviewPanel) handleKey(msg tea.KeyMsg) (*PreviewPanel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if p.LineOffset > 0 {
			p.LineOffset--
		}
		return p, nil

	case "down", "j":
		change := p.GetCurrentChange()
		if change != nil && p.LineOffset < len(change.DiffLines)-1 {
			p.LineOffset++
		}
		return p, nil

	case "left", "h":
		if p.CurrentIndex > 0 {
			p.CurrentIndex--
			p.LineOffset = 0
		}
		return p, nil

	case "right", "l":
		if p.CurrentIndex < len(p.Changes)-1 {
			p.CurrentIndex++
			p.LineOffset = 0
		}
		return p, nil

	case "y", "enter":
		// Accept current change
		change := p.AcceptCurrent()
		if change != nil {
			return p, func() tea.Msg {
				return PreviewAcceptedMsg{Change: *change}
			}
		}
		return p, nil

	case "n", "backspace":
		// Reject current change
		change := p.RejectCurrent()
		if change != nil {
			return p, func() tea.Msg {
				return PreviewRejectedMsg{Change: *change}
			}
		}
		return p, nil

	case "a":
		// Accept all
		changes := p.AcceptAll()
		return p, func() tea.Msg {
			return PreviewAcceptAllMsg{Changes: changes}
		}

	case "r":
		// Reject all
		p.RejectAll()
		return p, func() tea.Msg {
			return PreviewRejectedAllMsg{}
		}
	}

	return p, nil
}

// View renders the preview panel
func (p *PreviewPanel) View() string {
	if len(p.Changes) == 0 {
		return p.Styles.Help.Render("No pending changes to preview")
	}

	var content strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Primary).
		MarginBottom(1)

	content.WriteString(headerStyle.Render(fmt.Sprintf("📝 Preview Changes (%d/%d)", p.CurrentIndex+1, len(p.Changes))))
	content.WriteString("\n\n")

	// Current change info
	change := p.GetCurrentChange()
	if change != nil {
		// File info
		opStyle := lipgloss.NewStyle().Bold(true)
		switch change.Operation {
		case "create":
			opStyle = opStyle.Foreground(lipgloss.Color("#3fb950"))
			content.WriteString(opStyle.Render("[NEW] "))
		case "delete":
			opStyle = opStyle.Foreground(lipgloss.Color("#f85149"))
			content.WriteString(opStyle.Render("[DELETE] "))
		default:
			opStyle = opStyle.Foreground(lipgloss.Color("#d29922"))
			content.WriteString(opStyle.Render("[MODIFY] "))
		}

		pathStyle := lipgloss.NewStyle().Foreground(p.Styles.Theme.Foreground)
		content.WriteString(pathStyle.Render(change.Path))
		content.WriteString("\n")

		if change.ToolName != "" {
			toolStyle := lipgloss.NewStyle().Foreground(p.Styles.Theme.Subtle)
			content.WriteString(toolStyle.Render(fmt.Sprintf("via %s", change.ToolName)))
			content.WriteString("\n")
		}

		content.WriteString("\n")

		// Diff content
		visibleLines := p.Height - 10
		if visibleLines < 5 {
			visibleLines = 5
		}

		endIdx := p.LineOffset + visibleLines
		if endIdx > len(change.DiffLines) {
			endIdx = len(change.DiffLines)
		}

		for i := p.LineOffset; i < endIdx; i++ {
			line := change.DiffLines[i]
			content.WriteString(p.renderDiffLine(line))
			content.WriteString("\n")
		}

		// Show scroll indicator if needed
		if len(change.DiffLines) > visibleLines {
			scrollInfo := lipgloss.NewStyle().Foreground(p.Styles.Theme.Subtle)
			content.WriteString(scrollInfo.Render(fmt.Sprintf("\n... %d more lines (↑↓ to scroll)", len(change.DiffLines)-endIdx)))
			content.WriteString("\n")
		}
	}

	// Help
	content.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(p.Styles.Theme.Subtle)
	content.WriteString(helpStyle.Render("y/Enter accept · n/Backspace reject · a accept all · r reject all · ←→ navigate · q close"))

	return content.String()
}

// renderDiffLine renders a single diff line
func (p *PreviewPanel) renderDiffLine(line PreviewDiffLine) string {
	var style lipgloss.Style
	var prefix string

	maxWidth := p.Width - 12
	if maxWidth < 20 {
		maxWidth = 20
	}

	switch line.Type {
	case "add":
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3fb950")).
			Background(lipgloss.Color("#0d1d0d"))
		prefix = fmt.Sprintf("+%4d ", line.NewLineNo)
	case "remove":
		style = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f85149")).
			Background(lipgloss.Color("#1d0d0d"))
		prefix = fmt.Sprintf("-%4d ", line.OldLineNo)
	case "context":
		style = lipgloss.NewStyle().Foreground(p.Styles.Theme.Subtle)
		prefix = fmt.Sprintf(" %4d ", line.OldLineNo)
	default:
		style = lipgloss.NewStyle().Bold(true).Foreground(p.Styles.Theme.Primary)
		prefix = "      "
	}

	// Truncate line if too long
	content := line.Content
	if len(content) > maxWidth {
		content = content[:maxWidth-3] + "..."
	}

	lineNumStyle := lipgloss.NewStyle().Foreground(p.Styles.Theme.Subtle)
	return lineNumStyle.Render(prefix) + style.Render(content)
}

// PreviewAcceptedMsg indicates a change was accepted
type PreviewAcceptedMsg struct {
	Change PreviewChange
}

// PreviewRejectedMsg indicates a change was rejected
type PreviewRejectedMsg struct {
	Change PreviewChange
}

// PreviewAcceptAllMsg indicates all changes were accepted
type PreviewAcceptAllMsg struct {
	Changes []PreviewChange
}

// PreviewRejectedAllMsg indicates all changes were rejected
type PreviewRejectedAllMsg struct{}

// PreviewRequestedMsg requests showing a preview
type PreviewRequestedMsg struct {
	Path      string
	Original  string
	New       string
	Operation string
	ToolName  string
}
