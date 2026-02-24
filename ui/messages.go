package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// CommandInfo represents a command with its description for autocomplete
type CommandInfo struct {
	Command     string
	Description string
}

// AvailableCommands returns all available commands for autocomplete
func AvailableCommands() []CommandInfo {
	return []CommandInfo{
		{Command: "/help", Description: "Show help screen"},
		{Command: "/settings", Description: "Open settings panel"},
		{Command: "/clear", Description: "Clear chat history"},
		{Command: "/new", Description: "Start new conversation"},
		{Command: "/index", Description: "Index current codebase"},
		{Command: "/diff", Description: "Show git diff (unstaged)"},
		{Command: "/diff --staged", Description: "Show staged changes"},
		{Command: "/history", Description: "Browse conversation history"},
		{Command: "/save", Description: "Save current conversation"},
		{Command: "/load", Description: "Load a conversation by ID"},
		{Command: "/undo", Description: "Undo last file change (Ctrl+Z)"},
		{Command: "/redo", Description: "Redo last undone change (Ctrl+Y)"},
		{Command: "/task", Description: "Start agentic task mode"},
		{Command: "/plan", Description: "Plan a coding task with AI"},
		{Command: "/taskview", Description: "View current/last agentic task"},
		{Command: "/tasks", Description: "Browse all agentic tasks"},
	}
}

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

// GitDiffResultMsg contains the result of a git diff command
type GitDiffResultMsg struct {
	Diff   string
	Error  error
	Staged bool
}

// CommitMessageRequestMsg requests AI to generate a commit message
type CommitMessageRequestMsg struct {
	DiffContent  string
	FilesChanged int
	Additions    int
	Deletions    int
}

// CommitMessageResultMsg contains the AI-generated commit message
type CommitMessageResultMsg struct {
	Message string
	Error   error
}

// WindowSizeMsg is sent when terminal size changes
type WindowSizeMsg struct {
	Width  int
	Height int
}

// TickMsg is for animations and periodic updates
type TickMsg struct{}

// TokenStatsUpdatedMsg contains updated token statistics
type TokenStatsUpdatedMsg struct {
	InputTokens        int     // Total input tokens sent
	OutputTokens       int     // Total output tokens received
	TotalContextTokens int     // Current context window size
	AvgTokensPerSecond float64 // Average tokens/second across all responses
}

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
	ViewDiff
	ViewHistory         // Conversation history browser
	ViewPreview         // Inline diff preview
	ViewAgenticTask     // Agentic task panel
	ViewAgenticProposal // Task proposal for approval
	ViewAgenticTaskList // Browse all agentic tasks
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
	Role            ChatMessageRole
	Content         string
	Streaming       bool   // Currently being streamed
	ToolName        string // For tool messages
	ToolID          string // For tool messages
	Args            map[string]interface{}
	RenderedContent string // Cache for rendered markdown
}

// ConversationSavedMsg indicates a conversation was saved
type ConversationSavedMsg struct {
	ID    string
	Title string
	Error error
}

// ConversationLoadedMsg indicates a conversation was loaded
type ConversationLoadedMsg struct {
	ID       string
	Title    string
	Messages []ChatMessage
	Error    error
}

// ConversationListMsg contains the list of saved conversations
type ConversationListMsg struct {
	Conversations []ConversationSummaryInfo
	Error         error
}

// ConversationSummaryInfo contains summary info for a conversation
type ConversationSummaryInfo struct {
	ID           string
	Title        string
	Preview      string
	MessageCount int
	UpdatedAt    string
}

// ConversationDeletedMsg indicates a conversation was deleted
type ConversationDeletedMsg struct {
	ID    string
	Error error
}

// UndoResultMsg contains the result of an undo operation
type UndoResultMsg struct {
	Transaction interface{} // *history.EditTransaction
	UndoCount   int
	RedoCount   int
	Error       error
}

// RedoResultMsg contains the result of a redo operation
type RedoResultMsg struct {
	Transaction interface{} // *history.EditTransaction
	UndoCount   int
	RedoCount   int
	Error       error
}

// LSP Message types

// DiagnosticMsg contains diagnostics for a file
type DiagnosticMsg struct {
	FilePath    string
	Diagnostics interface{} // []lsp.Diagnostic
}

// HoverMsg contains hover information
type HoverMsg struct {
	Content string
	Error   error
}

// DefinitionMsg contains definition location
type DefinitionMsg struct {
	FilePath string
	Line     int
	Col      int
	Error    error
}

// LSPStartedMsg indicates LSP server started
type LSPStartedMsg struct {
	Client interface{} // *lsp.GoClient
	Error  error
}

// ============================================
// Agentic Mode Messages
// ============================================

// AgenticModeStartMsg starts agentic mode for a request
type AgenticModeStartMsg struct {
	Request string
}

// AgenticTaskCreatedMsg indicates a task plan was created
type AgenticTaskCreatedMsg struct {
	TaskID      string
	Title       string
	Description string
	TodoCount   int
}

// AgenticTaskUpdateMsg provides task/todo status updates
type AgenticTaskUpdateMsg struct {
	TaskID  string
	TodoID  string
	Phase   string
	Status  string
	Message string
}

// AgenticTaskUpdatedMsg provides a full task sync update
type AgenticTaskUpdatedMsg struct {
	Task interface{} // *agentic.Task
}

// AgenticPhaseChangedMsg indicates phase transition
type AgenticPhaseChangedMsg struct {
	TaskID   string
	NewPhase string // "planning", "execution", "verification", "completed"
}

// AgenticTaskProgressMsg provides progress percentage
type AgenticTaskProgressMsg struct {
	TaskID  string
	Percent float64
	Current int
	Total   int
}

// AgenticVerificationResultMsg contains verification outcome
type AgenticVerificationResultMsg struct {
	StepID string
	Passed bool
	Output string
}

// AgenticModeExitMsg exits agentic mode
type AgenticModeExitMsg struct {
	TaskID    string
	Completed bool
	Cancelled bool
}

// AgenticPauseMsg pauses task execution
type AgenticPauseMsg struct{}

// AgenticResumeMsg resumes paused execution
type AgenticResumeMsg struct{}

// AgenticCancelMsg cancels the current task
type AgenticCancelMsg struct{}

// AgenticSkipTodoMsg skips the current todo
type AgenticSkipTodoMsg struct {
	TodoID string
}

// AgenticTaskErrorMsg indicates an error in agentic mode
type AgenticTaskErrorMsg struct {
	TaskID string
	Error  error
}

// ============================================
// Proposal & Task List Messages
// ============================================

// AgenticProposalReadyMsg indicates a proposal is ready for review
type AgenticProposalReadyMsg struct {
	TaskID   string
	Proposal interface{} // *agentic.TaskProposal
}

// AgenticProposalApprovedMsg indicates user approved the proposal
type AgenticProposalApprovedMsg struct {
	TaskID string
}

// AgenticProposalRejectedMsg indicates user rejected the proposal
type AgenticProposalRejectedMsg struct {
	TaskID string
}

// AgenticProposalEditMsg indicates user wants to edit the proposal
type AgenticProposalEditMsg struct {
	TaskID   string
	Feedback string
}

// AgenticLoadTaskListMsg requests loading the task list
type AgenticLoadTaskListMsg struct{}

// AgenticTaskListLoadedMsg contains the loaded task list
type AgenticTaskListLoadedMsg struct {
	Tasks []interface{} // []agentic.TaskSummary
	Error error
}

// AgenticTaskSelectedMsg indicates a task was selected from the list
type AgenticTaskSelectedMsg struct {
	TaskID string
}

// AgenticDeleteTaskMsg requests deletion of a task
type AgenticDeleteTaskMsg struct {
	TaskID string
}

// AgenticTaskDeletedMsg indicates a task was deleted
type AgenticTaskDeletedMsg struct {
	TaskID string
	Error  error
}
