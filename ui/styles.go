package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme represents a color theme (adapts to light/dark)
type Theme struct {
	Name string

	// Base colors
	Background lipgloss.TerminalColor
	Foreground lipgloss.TerminalColor
	Subtle     lipgloss.TerminalColor
	Highlight  lipgloss.TerminalColor
	Special    lipgloss.TerminalColor

	// Semantic colors
	Primary   lipgloss.TerminalColor
	Secondary lipgloss.TerminalColor
	Success   lipgloss.TerminalColor
	Warning   lipgloss.TerminalColor
	Error     lipgloss.TerminalColor
	Info      lipgloss.TerminalColor

	// Chat colors
	UserMessage      lipgloss.TerminalColor
	AssistantMessage lipgloss.TerminalColor
	SystemMessage    lipgloss.TerminalColor
	ToolMessage      lipgloss.TerminalColor

	// UI element colors
	Border        lipgloss.TerminalColor
	BorderFocused lipgloss.TerminalColor
	StatusBar     lipgloss.TerminalColor
	StatusBarText lipgloss.TerminalColor
	InputBg       lipgloss.TerminalColor
	InputText     lipgloss.TerminalColor
	Placeholder   lipgloss.TerminalColor

	// Diff colors
	DiffAddedBg   lipgloss.TerminalColor
	DiffRemovedBg lipgloss.TerminalColor
}

// AdaptiveTheme is the auto-adapting color scheme
var AdaptiveTheme = Theme{
	Name: "adaptive",

	// Base colors
	Background: lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#0d1117"},
	Foreground: lipgloss.AdaptiveColor{Light: "#1f2328", Dark: "#e6edf3"},
	Subtle:     lipgloss.AdaptiveColor{Light: "#656d76", Dark: "#7d8590"},
	Highlight:  lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"},
	Special:    lipgloss.AdaptiveColor{Light: "#8250df", Dark: "#a371f7"},

	// Semantic colors
	Primary:   lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"},
	Secondary: lipgloss.AdaptiveColor{Light: "#8250df", Dark: "#a371f7"},
	Success:   lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#3fb950"},
	Warning:   lipgloss.AdaptiveColor{Light: "#9a6700", Dark: "#d29922"},
	Error:     lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f85149"},
	Info:      lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"},

	// Chat colors
	UserMessage:      lipgloss.AdaptiveColor{Light: "#0550ae", Dark: "#79c0ff"},
	AssistantMessage: lipgloss.AdaptiveColor{Light: "#1f2328", Dark: "#e6edf3"},
	SystemMessage:    lipgloss.AdaptiveColor{Light: "#656d76", Dark: "#7d8590"},
	ToolMessage:      lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#3fb950"},

	// UI element colors
	Border:        lipgloss.AdaptiveColor{Light: "#d0d7de", Dark: "#30363d"},
	BorderFocused: lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#58a6ff"},
	StatusBar:     lipgloss.AdaptiveColor{Light: "#f6f8fa", Dark: "#161b22"},
	StatusBarText: lipgloss.AdaptiveColor{Light: "#656d76", Dark: "#7d8590"},
	InputBg:       lipgloss.AdaptiveColor{Light: "#f6f8fa", Dark: "#161b22"},
	InputText:     lipgloss.AdaptiveColor{Light: "#1f2328", Dark: "#e6edf3"},
	Placeholder:   lipgloss.AdaptiveColor{Light: "#6e7781", Dark: "#484f58"},

	// Diff colors
	DiffAddedBg:   lipgloss.AdaptiveColor{Light: "#d4edda", Dark: "#1c3a1c"},
	DiffRemovedBg: lipgloss.AdaptiveColor{Light: "#f8d7da", Dark: "#3d1a1a"},
}

// Styles holds all the lipgloss styles for the application
type Styles struct {
	Theme Theme

	// App container
	App lipgloss.Style

	// Header
	Header      lipgloss.Style
	HeaderTitle lipgloss.Style
	HeaderInfo  lipgloss.Style

	// Chat area
	ChatContainer lipgloss.Style
	MessageUser   lipgloss.Style
	MessageAssist lipgloss.Style
	MessageSystem lipgloss.Style
	MessageTool   lipgloss.Style
	MessageLabel  lipgloss.Style
	CodeBlock     lipgloss.Style

	// Input area
	InputContainer lipgloss.Style
	InputField     lipgloss.Style
	InputPrompt    lipgloss.Style

	// Status bar
	StatusBar   lipgloss.Style
	StatusItem  lipgloss.Style
	StatusKey   lipgloss.Style
	StatusValue lipgloss.Style

	// Tool panel
	ToolPanel   lipgloss.Style
	ToolHeader  lipgloss.Style
	ToolName    lipgloss.Style
	ToolArgs    lipgloss.Style
	ToolResult  lipgloss.Style
	ToolSpinner lipgloss.Style

	// Modal / Dialogs
	Modal        lipgloss.Style
	ModalTitle   lipgloss.Style
	ModalContent lipgloss.Style

	// File picker
	FilePickerItem     lipgloss.Style
	FilePickerSelected lipgloss.Style
	FilePickerDir      lipgloss.Style
	FilePickerFile     lipgloss.Style

	// IDE components
	SidebarPanel     lipgloss.Style
	SidebarTitle     lipgloss.Style
	CodePanel        lipgloss.Style
	CodePanelTitle   lipgloss.Style
	LineNumber       lipgloss.Style
	LineNumberSel    lipgloss.Style
	ActionButton     lipgloss.Style
	ActionBtnFocus   lipgloss.Style
	PanelBorder      lipgloss.Style
	PanelBorderFocus lipgloss.Style

	// Misc
	Spinner lipgloss.Style
	Error   lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Help    lipgloss.Style
	Divider lipgloss.Style

	// Diff panel styles
	DiffAddedLine     lipgloss.Style
	DiffAddedPrefix   lipgloss.Style
	DiffRemovedLine   lipgloss.Style
	DiffRemovedPrefix lipgloss.Style
	DiffContextLine   lipgloss.Style
	DiffHunkHeader    lipgloss.Style
	DiffFileHeader    lipgloss.Style
	DiffLineNumber    lipgloss.Style
	DiffPanel         lipgloss.Style

	// Agentic Mode styles
	TaskPanelHeader     lipgloss.Style
	TaskPhaseBadge      lipgloss.Style
	TaskProgressFilled  lipgloss.Style
	TaskProgressEmpty   lipgloss.Style
	TaskTodoCard        lipgloss.Style
	TaskTodoSelected    lipgloss.Style
	TaskTodoCompleted   lipgloss.Style
	TaskTodoFailed      lipgloss.Style
	TaskTodoInProgress  lipgloss.Style
	TaskStatusBadge     lipgloss.Style
	TaskSectionHeader   lipgloss.Style
	TaskTimeEstimate    lipgloss.Style
	ProposalPanel       lipgloss.Style
	ProposalTitle       lipgloss.Style
	ProposalChange      lipgloss.Style
	ProposalButton      lipgloss.Style
	ProposalButtonFocus lipgloss.Style
}

// NewStyles creates a new Styles instance with the given theme
func NewStyles(theme Theme) Styles {
	s := Styles{Theme: theme}

	// App container - full terminal
	s.App = lipgloss.NewStyle().
		Background(theme.Background).
		Foreground(theme.Foreground)

	// Header
	s.Header = lipgloss.NewStyle().
		Background(theme.StatusBar).
		Foreground(theme.Foreground).
		Padding(0, 2).
		Bold(true)

	s.HeaderTitle = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	s.HeaderInfo = lipgloss.NewStyle().
		Foreground(theme.Subtle)

	// Chat container
	s.ChatContainer = lipgloss.NewStyle().
		Padding(1, 2)

	// Messages
	s.MessageUser = lipgloss.NewStyle().
		Foreground(theme.UserMessage).
		PaddingLeft(2)

	s.MessageAssist = lipgloss.NewStyle().
		Foreground(theme.AssistantMessage).
		PaddingLeft(2)

	s.MessageSystem = lipgloss.NewStyle().
		Foreground(theme.SystemMessage).
		Italic(true).
		PaddingLeft(2)

	s.MessageTool = lipgloss.NewStyle().
		Foreground(theme.ToolMessage).
		PaddingLeft(4)

	s.MessageLabel = lipgloss.NewStyle().
		Bold(true).
		MarginRight(1)

	s.CodeBlock = lipgloss.NewStyle().
		Background(lipgloss.AdaptiveColor{Light: "#f6f8fa", Dark: "#21262d"}).
		Foreground(theme.Foreground).
		Padding(1, 2).
		MarginTop(1).
		MarginBottom(1)

	// Input area - fixed at bottom (no background to avoid weird patches)
	s.InputContainer = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Border).
		Padding(0, 1)

	s.InputField = lipgloss.NewStyle().
		Foreground(theme.InputText)

	s.InputPrompt = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	// Status bar - bottom of screen
	s.StatusBar = lipgloss.NewStyle().
		Background(theme.StatusBar).
		Foreground(theme.StatusBarText).
		Padding(0, 1)

	s.StatusItem = lipgloss.NewStyle().
		Foreground(theme.StatusBarText).
		PaddingRight(2)

	s.StatusKey = lipgloss.NewStyle().
		Foreground(theme.Subtle)

	s.StatusValue = lipgloss.NewStyle().
		Foreground(theme.Highlight)

	// Tool panel
	s.ToolPanel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ToolMessage).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	s.ToolHeader = lipgloss.NewStyle().
		Foreground(theme.ToolMessage).
		Bold(true)

	s.ToolName = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	s.ToolArgs = lipgloss.NewStyle().
		Foreground(theme.Subtle)

	s.ToolResult = lipgloss.NewStyle().
		Foreground(theme.Foreground)

	s.ToolSpinner = lipgloss.NewStyle().
		Foreground(theme.Primary)

	// Modal
	s.Modal = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocused).
		Padding(1, 2).
		Background(theme.Background)

	s.ModalTitle = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		MarginBottom(1)

	s.ModalContent = lipgloss.NewStyle().
		Foreground(theme.Foreground)

	// File picker
	s.FilePickerItem = lipgloss.NewStyle().
		Foreground(theme.Foreground)

	s.FilePickerSelected = lipgloss.NewStyle().
		Foreground(theme.Highlight).
		Bold(true).
		Background(lipgloss.AdaptiveColor{Light: "#ddf4ff", Dark: "#21262d"})

	s.FilePickerDir = lipgloss.NewStyle().
		Foreground(theme.Primary)

	s.FilePickerFile = lipgloss.NewStyle().
		Foreground(theme.Foreground)

	// IDE components
	s.SidebarPanel = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Border).
		Background(theme.Background)

	s.SidebarTitle = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		Padding(0, 1)

	s.CodePanel = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Border).
		Background(theme.Background)

	s.CodePanelTitle = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	s.LineNumber = lipgloss.NewStyle().
		Foreground(theme.Subtle).
		PaddingRight(1)

	s.LineNumberSel = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true).
		PaddingRight(1).
		Background(lipgloss.AdaptiveColor{Light: "#ddf4ff", Dark: "#21262d"})

	s.ActionButton = lipgloss.NewStyle().
		Foreground(theme.Foreground).
		Padding(0, 2).
		MarginRight(1)

	s.ActionBtnFocus = lipgloss.NewStyle().
		Foreground(theme.Background).
		Background(theme.Primary).
		Padding(0, 2).
		MarginRight(1).
		Bold(true)

	s.PanelBorder = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Border)

	s.PanelBorderFocus = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.BorderFocused)

	// Misc
	s.Spinner = lipgloss.NewStyle().
		Foreground(theme.Primary)

	s.Error = lipgloss.NewStyle().
		Foreground(theme.Error)

	s.Success = lipgloss.NewStyle().
		Foreground(theme.Success)

	s.Warning = lipgloss.NewStyle().
		Foreground(theme.Warning)

	s.Help = lipgloss.NewStyle().
		Foreground(theme.Subtle)

	s.Divider = lipgloss.NewStyle().
		Foreground(theme.Border)

	// Diff panel styles - VS Code-inspired
	s.DiffAddedLine = lipgloss.NewStyle().
		Foreground(theme.Success).
		Background(theme.DiffAddedBg)

	s.DiffAddedPrefix = lipgloss.NewStyle().
		Foreground(theme.Success).
		Background(theme.DiffAddedBg).
		Bold(true)

	s.DiffRemovedLine = lipgloss.NewStyle().
		Foreground(theme.Error).
		Background(theme.DiffRemovedBg)

	s.DiffRemovedPrefix = lipgloss.NewStyle().
		Foreground(theme.Error).
		Background(theme.DiffRemovedBg).
		Bold(true)

	s.DiffContextLine = lipgloss.NewStyle().
		Foreground(theme.Subtle)

	s.DiffHunkHeader = lipgloss.NewStyle().
		Foreground(theme.Info).
		Bold(true)

	s.DiffFileHeader = lipgloss.NewStyle().
		Foreground(theme.Primary).
		Bold(true)

	s.DiffLineNumber = lipgloss.NewStyle().
		Foreground(theme.Subtle).
		PaddingRight(1)

	s.DiffPanel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocused).
		Padding(1, 2).
		Background(theme.Background)

	// Agentic Mode styles - Professional, modern design
	s.TaskPanelHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		PaddingBottom(1)

	s.TaskPhaseBadge = lipgloss.NewStyle().
		Bold(true).
		Padding(0, 2).
		MarginLeft(1)

	s.TaskProgressFilled = lipgloss.NewStyle().
		Foreground(theme.Success)

	s.TaskProgressEmpty = lipgloss.NewStyle().
		Foreground(theme.Subtle)

	s.TaskTodoCard = lipgloss.NewStyle().
		Foreground(theme.Foreground).
		PaddingLeft(2)

	s.TaskTodoSelected = lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary).
		Background(lipgloss.AdaptiveColor{Light: "#ddf4ff", Dark: "#21262d"}).
		PaddingLeft(2)

	s.TaskTodoCompleted = lipgloss.NewStyle().
		Foreground(theme.Success).
		Strikethrough(true).
		PaddingLeft(2)

	s.TaskTodoFailed = lipgloss.NewStyle().
		Foreground(theme.Error).
		PaddingLeft(2)

	s.TaskTodoInProgress = lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Warning).
		PaddingLeft(2)

	s.TaskStatusBadge = lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	s.TaskSectionHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Foreground).
		MarginTop(1)

	s.TaskTimeEstimate = lipgloss.NewStyle().
		Foreground(theme.Subtle).
		Italic(true)

	s.ProposalPanel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.Primary).
		Padding(1, 2).
		Background(theme.Background)

	s.ProposalTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.Primary)

	s.ProposalChange = lipgloss.NewStyle().
		Foreground(theme.Foreground).
		PaddingLeft(3)

	s.ProposalButton = lipgloss.NewStyle().
		Padding(0, 3).
		MarginRight(2)

	s.ProposalButtonFocus = lipgloss.NewStyle().
		Bold(true).
		Padding(0, 3).
		MarginRight(2)

	return s
}

// DefaultStyles returns styles with the adaptive theme
func DefaultStyles() Styles {
	return NewStyles(AdaptiveTheme)
}
