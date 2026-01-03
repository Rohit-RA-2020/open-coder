package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Message types for async operations

// StreamChunkMsg is sent when a new chunk arrives from OpenAI streaming
type StreamChunkMsg struct {
	Content string
	Done    bool
}

// StreamErrorMsg is sent when streaming encounters an error
type StreamErrorMsg struct {
	Err error
}

// ToolCallStartMsg indicates a tool call is starting
type ToolCallStartMsg struct {
	ToolName string
	ToolID   string
	Args     map[string]any
}

// ToolCallResultMsg contains the result of a tool execution
type ToolCallResultMsg struct {
	ToolName string
	ToolID   string
	Result   string
	Error    error
}

// ToolCallCancelledMsg indicates a tool call was cancelled
type ToolCallCancelledMsg struct {
	ToolName string
	ToolID   string
}

// ApprovalRequestMsg asks user to approve a terminal command
type ApprovalRequestMsg struct {
	ToolName string
	ToolID   string
	Command  string
	Args     map[string]any
}

// ApprovalResponseMsg is the user's response to an approval request
type ApprovalResponseMsg struct {
	ToolID   string
	Approved bool
}

// MCPConnectedMsg indicates MCP servers connected successfully
type MCPConnectedMsg struct {
	ServerCount int
	ToolCount   int
}

// MCPErrorMsg indicates an MCP connection error
type MCPErrorMsg struct {
	Err error
}

// InitCompleteMsg signals initialization is complete
type InitCompleteMsg struct{}

// ErrorMsg is a generic error message
type ErrorMsg struct {
	Err error
}

// FileSelectedMsg is sent when a file is selected in the file picker
type FileSelectedMsg struct {
	Path string
}

// FilePickerCancelledMsg is sent when file picker is cancelled
type FilePickerCancelledMsg struct{}

// ThemeChangedMsg indicates the theme was changed
type ThemeChangedMsg struct {
	Theme Theme
}

// AccentColor represents available accent colors
type AccentColor int

const (
	AccentBlue AccentColor = iota
	AccentPurple
	AccentGreen
	AccentOrange
	AccentPink
	AccentCyan
	AccentRed
	AccentYellow
	AccentTeal
	AccentIndigo
)

// AccentColorName returns the display name for an accent color
func (a AccentColor) Name() string {
	switch a {
	case AccentBlue:
		return "Blue 💙"
	case AccentPurple:
		return "Purple 💜"
	case AccentGreen:
		return "Green 💚"
	case AccentOrange:
		return "Orange 🧡"
	case AccentPink:
		return "Pink 💗"
	case AccentCyan:
		return "Cyan 🩵"
	case AccentRed:
		return "Red ❤️"
	case AccentYellow:
		return "Yellow 💛"
	case AccentTeal:
		return "Teal 🌊"
	case AccentIndigo:
		return "Indigo 💎"
	default:
		return "Blue 💙"
	}
}

// AccentColorChangedMsg indicates accent color was changed
type AccentColorChangedMsg struct {
	Accent AccentColor
}

// Hex returns the hex color code for an accent color
func (a AccentColor) Hex() string {
	switch a {
	case AccentBlue:
		return "#58a6ff"
	case AccentPurple:
		return "#a371f7"
	case AccentGreen:
		return "#3fb950"
	case AccentOrange:
		return "#f0883e"
	case AccentPink:
		return "#f778ba"
	case AccentCyan:
		return "#56d4dd"
	case AccentRed:
		return "#f85149"
	case AccentYellow:
		return "#d29922"
	case AccentTeal:
		return "#2ea043"
	case AccentIndigo:
		return "#6e40c9"
	default:
		return "#58a6ff"
	}
}

// IndexingStartMsg indicates codebase indexing has started
type IndexingStartMsg struct {
	Directory string
}

// IndexingProgressMsg provides indexing progress updates
type IndexingProgressMsg struct {
	Current int
	Total   int
	File    string
}

// IndexingCompleteMsg indicates indexing is complete
type IndexingCompleteMsg struct {
	ChunksProcessed int
	FilesProcessed  int
	AlreadyIndexed  bool // True if directory was already indexed
}

// IndexingErrorMsg indicates an indexing error
type IndexingErrorMsg struct {
	Err error
}

// WindowSizeMsg is sent when terminal size changes
type WindowSizeMsg struct {
	Width  int
	Height int
}

// TickMsg is for animations and periodic updates
type TickMsg struct{}

// SendMessageCmd creates a command to process user input
func SendMessageCmd(message string) tea.Cmd {
	return func() tea.Msg {
		return userMessageMsg{Content: message}
	}
}

// userMessageMsg is internal - user submitted a message
type userMessageMsg struct {
	Content string
}

// ViewState represents the current UI view
type ViewState int

const (
	ViewChat ViewState = iota
	ViewFilePicker
	ViewSettings
	ViewApproval
	ViewIndexing
	ViewHelp
)

// PanelFocus represents which panel is currently focused in split-pane layout
type PanelFocus int

const (
	PanelFileTree PanelFocus = iota
	PanelChat
	PanelCode
)

// PanelSwitchedMsg indicates panel focus has changed
type PanelSwitchedMsg struct {
	NewFocus PanelFocus
}

// FileTreeToggledMsg indicates a directory was expanded/collapsed
type FileTreeToggledMsg struct {
	Path string
}

// FileSelectedInTreeMsg indicates a file was selected in the file tree
type FileSelectedInTreeMsg struct {
	Path string
}

// CodePreviewUpdatedMsg indicates the code preview was updated
type CodePreviewUpdatedMsg struct {
	FilePath string
	Content  string
	Language string
}

// CodeActionRequestedMsg indicates a code action was requested (apply, edit, diff)
type CodeActionRequestedMsg struct {
	Action   string // "apply", "edit", "diff"
	FilePath string
	Line     int
	Content  string
}

// ChatMessageRole identifies the sender of a message
type ChatMessageRole int

const (
	RoleUser ChatMessageRole = iota
	RoleAssistant
	RoleSystem
	RoleTool
)

// ChatMessage represents a message in the chat history
type ChatMessage struct {
	Role      ChatMessageRole
	Content   string
	ToolName  string // For tool messages
	ToolID    string // For tool messages
	Streaming bool   // Currently being streamed
}
