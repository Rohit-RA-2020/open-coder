package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme represents a color theme (dark or light)
type Theme struct {
	Name string

	// Base colors
	Background lipgloss.Color
	Foreground lipgloss.Color
	Subtle     lipgloss.Color
	Highlight  lipgloss.Color
	Special    lipgloss.Color

	// Semantic colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Error     lipgloss.Color
	Info      lipgloss.Color

	// Chat colors
	UserMessage      lipgloss.Color
	AssistantMessage lipgloss.Color
	SystemMessage    lipgloss.Color
	ToolMessage      lipgloss.Color

	// UI element colors
	Border        lipgloss.Color
	BorderFocused lipgloss.Color
	StatusBar     lipgloss.Color
	StatusBarText lipgloss.Color
	InputBg       lipgloss.Color
	InputText     lipgloss.Color
	Placeholder   lipgloss.Color
}

// DarkTheme is the default dark color scheme
var DarkTheme = Theme{
	Name: "dark",

	// Base colors - Deep, rich dark theme
	Background: lipgloss.Color("#0d1117"),
	Foreground: lipgloss.Color("#e6edf3"),
	Subtle:     lipgloss.Color("#7d8590"),
	Highlight:  lipgloss.Color("#58a6ff"),
	Special:    lipgloss.Color("#a371f7"),

	// Semantic colors
	Primary:   lipgloss.Color("#58a6ff"),
	Secondary: lipgloss.Color("#a371f7"),
	Success:   lipgloss.Color("#3fb950"),
	Warning:   lipgloss.Color("#d29922"),
	Error:     lipgloss.Color("#f85149"),
	Info:      lipgloss.Color("#58a6ff"),

	// Chat colors
	UserMessage:      lipgloss.Color("#79c0ff"),
	AssistantMessage: lipgloss.Color("#e6edf3"),
	SystemMessage:    lipgloss.Color("#7d8590"),
	ToolMessage:      lipgloss.Color("#3fb950"),

	// UI element colors
	Border:        lipgloss.Color("#30363d"),
	BorderFocused: lipgloss.Color("#58a6ff"),
	StatusBar:     lipgloss.Color("#161b22"),
	StatusBarText: lipgloss.Color("#7d8590"),
	InputBg:       lipgloss.Color("#161b22"),
	InputText:     lipgloss.Color("#e6edf3"),
	Placeholder:   lipgloss.Color("#484f58"),
}

// LightTheme is the light color scheme
var LightTheme = Theme{
	Name: "light",

	// Base colors - Clean, modern light theme
	Background: lipgloss.Color("#ffffff"),
	Foreground: lipgloss.Color("#1f2328"),
	Subtle:     lipgloss.Color("#656d76"),
	Highlight:  lipgloss.Color("#0969da"),
	Special:    lipgloss.Color("#8250df"),

	// Semantic colors
	Primary:   lipgloss.Color("#0969da"),
	Secondary: lipgloss.Color("#8250df"),
	Success:   lipgloss.Color("#1a7f37"),
	Warning:   lipgloss.Color("#9a6700"),
	Error:     lipgloss.Color("#cf222e"),
	Info:      lipgloss.Color("#0969da"),

	// Chat colors
	UserMessage:      lipgloss.Color("#0550ae"),
	AssistantMessage: lipgloss.Color("#1f2328"),
	SystemMessage:    lipgloss.Color("#656d76"),
	ToolMessage:      lipgloss.Color("#1a7f37"),

	// UI element colors
	Border:        lipgloss.Color("#d0d7de"),
	BorderFocused: lipgloss.Color("#0969da"),
	StatusBar:     lipgloss.Color("#f6f8fa"),
	StatusBarText: lipgloss.Color("#656d76"),
	InputBg:       lipgloss.Color("#f6f8fa"),
	InputText:     lipgloss.Color("#1f2328"),
	Placeholder:   lipgloss.Color("#6e7781"),
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
		Background(lipgloss.Color("#21262d")).
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
		Background(lipgloss.Color("#21262d"))

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
		Background(lipgloss.Color("#21262d"))

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
	var diffAddedBg, diffRemovedBg lipgloss.Color
	if theme.Name == "dark" {
		diffAddedBg = lipgloss.Color("#1c3a1c")   // Dark green background
		diffRemovedBg = lipgloss.Color("#3d1a1a") // Dark red background
	} else {
		diffAddedBg = lipgloss.Color("#d4edda")   // Light green background
		diffRemovedBg = lipgloss.Color("#f8d7da") // Light red background
	}

	s.DiffAddedLine = lipgloss.NewStyle().
		Foreground(theme.Success).
		Background(diffAddedBg)

	s.DiffAddedPrefix = lipgloss.NewStyle().
		Foreground(theme.Success).
		Background(diffAddedBg).
		Bold(true)

	s.DiffRemovedLine = lipgloss.NewStyle().
		Foreground(theme.Error).
		Background(diffRemovedBg)

	s.DiffRemovedPrefix = lipgloss.NewStyle().
		Foreground(theme.Error).
		Background(diffRemovedBg).
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

	return s
}

// DefaultStyles returns styles with the dark theme
func DefaultStyles() Styles {
	return NewStyles(DarkTheme)
}
