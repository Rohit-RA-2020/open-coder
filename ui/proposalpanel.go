package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"open-coder/pkg/agentic"
)

// ProposalPanel shows the task proposal for user approval before execution
type ProposalPanel struct {
	Proposal        *agentic.TaskProposal
	Width           int
	Height          int
	Styles          Styles
	ScrollOffset    int
	SelectedSection int // 0=overview, 1=changes, 2=steps
	ButtonFocus     int // 0=approve, 1=reject, 2=edit
}

// NewProposalPanel creates a new proposal panel
func NewProposalPanel(styles Styles) *ProposalPanel {
	return &ProposalPanel{
		Styles:          styles,
		SelectedSection: 0,
		ButtonFocus:     0,
	}
}

// SetProposal sets the proposal to display
func (p *ProposalPanel) SetProposal(proposal *agentic.TaskProposal) {
	p.Proposal = proposal
	p.ScrollOffset = 0
	p.SelectedSection = 0
	p.ButtonFocus = 0
}

// SetSize updates the panel dimensions
func (p *ProposalPanel) SetSize(width, height int) {
	p.Width = width
	p.Height = height
}

// Update handles input for proposal panel
func (p *ProposalPanel) Update(msg tea.Msg) (*ProposalPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return p.handleKey(msg)
	}
	return p, nil
}

// handleKey processes keyboard input
func (p *ProposalPanel) handleKey(msg tea.KeyMsg) (*ProposalPanel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		p.ScrollOffset++
	case "k", "up":
		if p.ScrollOffset > 0 {
			p.ScrollOffset--
		}
	case "tab":
		p.ButtonFocus = (p.ButtonFocus + 1) % 3
	case "shift+tab":
		p.ButtonFocus = (p.ButtonFocus + 2) % 3
	case "left", "h":
		if p.ButtonFocus > 0 {
			p.ButtonFocus--
		}
	case "right", "l":
		if p.ButtonFocus < 2 {
			p.ButtonFocus++
		}
	case "enter", " ":
		switch p.ButtonFocus {
		case 0: // Approve
			return p, func() tea.Msg {
				return AgenticProposalApprovedMsg{TaskID: p.Proposal.TaskID}
			}
		case 1: // Reject
			return p, func() tea.Msg {
				return AgenticProposalRejectedMsg{TaskID: p.Proposal.TaskID}
			}
		case 2: // Edit
			return p, func() tea.Msg {
				return AgenticProposalEditMsg{TaskID: p.Proposal.TaskID}
			}
		}
	case "y", "Y":
		// Quick approve
		return p, func() tea.Msg {
			return AgenticProposalApprovedMsg{TaskID: p.Proposal.TaskID}
		}
	case "n", "N":
		// Quick reject
		return p, func() tea.Msg {
			return AgenticProposalRejectedMsg{TaskID: p.Proposal.TaskID}
		}
	}
	return p, nil
}

// View renders the proposal panel
func (p *ProposalPanel) View() string {
	if p.Proposal == nil {
		return p.renderEmpty()
	}

	var sections []string

	// Header
	sections = append(sections, p.renderHeader())

	// Overview section
	sections = append(sections, p.renderOverview())

	// File changes section
	sections = append(sections, p.renderChanges())

	// Execution plan section
	sections = append(sections, p.renderSteps())

	// Approval buttons
	sections = append(sections, p.renderButtons())

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Apply panel style with modern glassmorphism feel
	panelStyle := lipgloss.NewStyle().
		Width(p.Width).
		Height(p.Height).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(p.Styles.Theme.Primary).
		Padding(1, 2)

	return panelStyle.Render(content)
}

// renderEmpty renders placeholder when no proposal is available
func (p *ProposalPanel) renderEmpty() string {
	emptyStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Subtle).
		Align(lipgloss.Center).
		Width(p.Width).
		Height(p.Height)

	return emptyStyle.Render("No proposal available")
}

// renderHeader renders the proposal header
func (p *ProposalPanel) renderHeader() string {
	// Title with icon
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Primary).
		PaddingBottom(1)

	title := titleStyle.Render(fmt.Sprintf("📝 Task Proposal: %s", p.Proposal.Title))

	// Separator line
	separator := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Border).
		Render(strings.Repeat("─", p.Width-6))

	return lipgloss.JoinVertical(lipgloss.Left, title, separator)
}

// renderOverview renders the overview section
func (p *ProposalPanel) renderOverview() string {
	sectionStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Foreground).
		PaddingTop(1).
		PaddingBottom(1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Highlight)

	valueStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Foreground)

	subtleStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Subtle).
		Italic(true)

	header := labelStyle.Render("📊 Overview")

	var lines []string
	lines = append(lines, header)

	// Description
	if p.Proposal.Description != "" {
		desc := p.Proposal.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		lines = append(lines, subtleStyle.Render("   "+desc))
	}

	// Time estimate
	lines = append(lines, valueStyle.Render(fmt.Sprintf("   ⏱️  Estimated time: %s", p.Proposal.EstimatedTime)))

	// Step counts
	totalSteps := len(p.Proposal.PlanningSteps) + len(p.Proposal.ExecutionSteps) + len(p.Proposal.VerifySteps)
	lines = append(lines, valueStyle.Render(fmt.Sprintf("   📋 Total steps: %d", totalSteps)))

	return sectionStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderChanges renders the file changes section
func (p *ProposalPanel) renderChanges() string {
	if len(p.Proposal.Changes) == 0 {
		return ""
	}

	sectionStyle := lipgloss.NewStyle().
		PaddingTop(1).
		PaddingBottom(1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Highlight)

	header := labelStyle.Render("📁 Files to Change")

	var lines []string
	lines = append(lines, header)

	for _, change := range p.Proposal.Changes {
		var icon, opStyle string
		switch change.Operation {
		case "create":
			icon = "+"
			opStyle = lipgloss.NewStyle().
				Foreground(p.Styles.Theme.Success).
				Bold(true).
				Render(fmt.Sprintf("   [NEW] %s", change.FilePath))
		case "delete":
			icon = "-"
			opStyle = lipgloss.NewStyle().
				Foreground(p.Styles.Theme.Error).
				Bold(true).
				Render(fmt.Sprintf("   [DEL] %s", change.FilePath))
		default:
			icon = "~"
			opStyle = lipgloss.NewStyle().
				Foreground(p.Styles.Theme.Warning).
				Bold(true).
				Render(fmt.Sprintf("   [MOD] %s", change.FilePath))
		}
		_ = icon // suppress unused warning
		lines = append(lines, opStyle)
	}

	return sectionStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderSteps renders the execution plan section
func (p *ProposalPanel) renderSteps() string {
	sectionStyle := lipgloss.NewStyle().
		PaddingTop(1).
		PaddingBottom(1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Styles.Theme.Highlight)

	stepStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Foreground)

	phaseStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Subtle).
		Bold(true)

	var lines []string
	lines = append(lines, labelStyle.Render("📋 Execution Plan"))

	// Planning steps
	if len(p.Proposal.PlanningSteps) > 0 {
		lines = append(lines, phaseStyle.Render("   Planning:"))
		for i, step := range p.Proposal.PlanningSteps {
			if len(step) > p.Width-15 {
				step = step[:p.Width-18] + "..."
			}
			lines = append(lines, stepStyle.Render(fmt.Sprintf("      %d. %s", i+1, step)))
		}
	}

	// Execution steps
	if len(p.Proposal.ExecutionSteps) > 0 {
		lines = append(lines, phaseStyle.Render("   Execution:"))
		for i, step := range p.Proposal.ExecutionSteps {
			if len(step) > p.Width-15 {
				step = step[:p.Width-18] + "..."
			}
			lines = append(lines, stepStyle.Render(fmt.Sprintf("      %d. %s", i+1, step)))
		}
	}

	// Verification steps
	if len(p.Proposal.VerifySteps) > 0 {
		lines = append(lines, phaseStyle.Render("   Verification:"))
		for i, step := range p.Proposal.VerifySteps {
			if len(step) > p.Width-15 {
				step = step[:p.Width-18] + "..."
			}
			lines = append(lines, stepStyle.Render(fmt.Sprintf("      %d. %s", i+1, step)))
		}
	}

	return sectionStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

// renderButtons renders the approval buttons
func (p *ProposalPanel) renderButtons() string {
	// Separator
	separator := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Border).
		Render(strings.Repeat("─", p.Width-6))

	// Button styles
	buttonStyle := lipgloss.NewStyle().
		Padding(0, 3).
		MarginRight(2)

	focusedButtonStyle := buttonStyle.
		Bold(true)

	approveBtn := buttonStyle.
		Foreground(p.Styles.Theme.Success)
	if p.ButtonFocus == 0 {
		approveBtn = focusedButtonStyle.
			Background(p.Styles.Theme.Success).
			Foreground(lipgloss.Color("#ffffff"))
	}

	rejectBtn := buttonStyle.
		Foreground(p.Styles.Theme.Error)
	if p.ButtonFocus == 1 {
		rejectBtn = focusedButtonStyle.
			Background(p.Styles.Theme.Error).
			Foreground(lipgloss.Color("#ffffff"))
	}

	editBtn := buttonStyle.
		Foreground(p.Styles.Theme.Warning)
	if p.ButtonFocus == 2 {
		editBtn = focusedButtonStyle.
			Background(p.Styles.Theme.Warning).
			Foreground(lipgloss.Color("#ffffff"))
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Center,
		approveBtn.Render("✓ Approve (Y)"),
		rejectBtn.Render("✗ Reject (N)"),
		editBtn.Render("✎ Edit"),
	)

	// Center the buttons
	buttonContainer := lipgloss.NewStyle().
		Width(p.Width - 6).
		Align(lipgloss.Center).
		PaddingTop(1).
		Render(buttons)

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(p.Styles.Theme.Subtle).
		Align(lipgloss.Center).
		Width(p.Width - 6)

	help := helpStyle.Render("Tab/←→: navigate buttons · Y/N: quick approve/reject · Esc: cancel")

	return lipgloss.JoinVertical(lipgloss.Left, separator, buttonContainer, help)
}
