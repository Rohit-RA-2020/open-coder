package ui

import (
	"fmt"
	"open-coder/pkg/agentic"
	"open-coder/pkg/conversations"
	"open-coder/pkg/lsp"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
)

// Model is the main Bubble Tea model for the application
type Model struct {
	// Dimensions
	width  int
	height int
	ready  bool

	// UI Components
	viewport   viewport.Model
	textarea   textarea.Model
	spinner    spinner.Model
	fileTree   *FileTree
	codePanel  *CodePanel
	filePicker *FilePicker

	// State
	view           ViewState
	panelFocus     PanelFocus // Current focused panel
	messages       []ChatMessage
	currentTool    *ToolCallStartMsg
	styles         Styles
	isDarkTheme    bool
	accentColor    AccentColor // Current accent color
	settingsCursor int         // Cursor position in settings
	showSidebar    bool        // Toggle for sidebar visibility
	showCodePanel  bool        // Toggle for code panel visibility

	// Glamour renderer for markdown
	renderer *glamour.TermRenderer

	// Backend interface
	agent AgentInterface

	// Status info
	modelName         string
	serverCount       int
	toolCount         int
	isProcessing      bool
	hasUserInteracted bool // Track if user has sent a message

	// Error state
	lastError error

	// File picker state
	filePickerPath string

	// Approval state
	pendingApproval *ApprovalRequestMsg

	// Indexing state
	indexingCurrent int
	indexingTotal   int
	indexingFile    string

	// Diff panel state
	diffPanel *DiffPanel

	// Token stats
	inputTokens        int
	outputTokens       int
	totalContextTokens int
	avgTokensPerSecond float64

	// Command autocomplete state
	showCommandMenu   bool // Whether command dropdown is visible
	commandMenuCursor int  // Currently selected command index

	// Conversation history state
	conversationList   []ConversationSummaryInfo
	conversationCursor int

	// Inline diff preview panel
	previewPanel *PreviewPanel

	// LSP Client
	lspClient *lsp.GoClient

	// Agentic Mode
	taskPanel     *TaskPanel
	proposalPanel *ProposalPanel
	taskListPanel *TaskListPanel
	isAgenticMode bool
}

// AgentInterface defines the backend operations the UI needs
type AgentInterface interface {
	ProcessMessage(content string) tea.Cmd
	CancelCurrentTool()
	ClearConversation()
	GetModelName() string
	GetServerCount() int
	GetToolCount() int
	IndexCodebase() tea.Cmd
	ToggleTerminalApproval()
	IsTerminalApprovalRequired() bool
	GenerateCommitMessage(diffContent string, filesChanged, additions, deletions int) tea.Cmd
	// Conversation persistence
	SaveConversation() error
	LoadConversation(id string) error
	ListConversations() ([]conversations.ConversationSummary, error)
	NewConversationSession()
	GetCurrentConversationID() string
	GetCurrentConversationTitle() string
	DeleteConversation(id string) error
	// Undo/Redo
	Undo() tea.Cmd
	Redo() tea.Cmd
	GetUndoCount() int
	GetRedoCount() int
}

// New creates a new Model with the given agent backend
func New(agent AgentInterface) Model {
	// Initialize textarea for input
	ta := textarea.New()
	ta.Placeholder = "Type your message... (@ to browse files, /help for commands)"
	ta.Focus()
	ta.Prompt = "▸ " // Show prompt on same line as input
	ta.CharLimit = 0 // No limit
	ta.SetWidth(80)
	ta.SetHeight(1) // Single line input
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // Enter sends, not newline

	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#58a6ff"))

	// Create markdown renderer with custom dark style that:
	// 1. Properly renders headers without showing ## prefix
	// 2. Uses app's blue accent color for headers
	// 3. Compact spacing for chat display
	markdownStyle := ansi.StyleConfig{
		Document: ansi.StyleBlock{
			Margin: uintPtr(0),
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "▌ ",
				Color:       stringPtr("#58a6ff"),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "",
				Color:  stringPtr("#58a6ff"),
				Bold:   boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "",
				Color:  stringPtr("#58a6ff"),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "",
				Color:  stringPtr("#79c0ff"),
				Bold:   boolPtr(true),
			},
		},
		Paragraph: ansi.StyleBlock{
			Margin: uintPtr(0),
		},
		List: ansi.StyleList{
			StyleBlock: ansi.StyleBlock{
				Margin: uintPtr(0),
			},
			LevelIndent: 2,
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: stringPtr("#ffa657"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				Margin: uintPtr(0),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: stringPtr("#c9d1d9"),
				},
				Keyword: ansi.StylePrimitive{
					Color: stringPtr("#ff7b72"),
				},
				Name: ansi.StylePrimitive{
					Color: stringPtr("#79c0ff"),
				},
				LiteralString: ansi.StylePrimitive{
					Color: stringPtr("#a5d6ff"),
				},
			},
		},
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(markdownStyle),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		// Log error but continue - nil renderer will be handled in renderMessage
		renderer = nil
	}

	// Initialize styles
	styles := NewStyles(AdaptiveTheme)

	// Get working directory for file tree
	wd, _ := os.Getwd()

	fp := NewFilePicker(styles)
	fp.Width = 80
	fp.Height = 20

	return Model{
		textarea:      ta,
		spinner:       sp,
		fileTree:      NewFileTree(wd, styles),
		codePanel:     NewCodePanel(styles),
		filePicker:    &fp,
		diffPanel:     NewDiffPanel(styles),
		previewPanel:  NewPreviewPanel(styles),
		taskPanel:     NewTaskPanel(styles),
		proposalPanel: NewProposalPanel(styles),
		taskListPanel: NewTaskListPanel(styles),
		view:          ViewChat,
		panelFocus:    PanelChat,
		messages:      []ChatMessage{},
		styles:        styles,
		isDarkTheme:   true,
		renderer:      renderer,
		agent:         agent,
		showSidebar:   true,
		showCodePanel: true,
	}
}

// Init implements tea.Model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		m.startLSPCmd(),
	)
}

// startLSPCmd creates a command to start the LSP server
func (m *Model) startLSPCmd() tea.Cmd {
	return func() tea.Msg {
		cwd, err := os.Getwd()
		if err != nil {
			return LSPStartedMsg{Error: err}
		}

		client, err := lsp.NewGoClient(cwd)
		if err != nil {
			return LSPStartedMsg{Error: err}
		}

		// Return the client in the message
		return LSPStartedMsg{Client: client, Error: nil}
	}
}

// Update implements tea.Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case StreamChunkMsg:
		return m.handleStreamChunk(msg)

	case StreamErrorMsg:
		m.isProcessing = false
		m.lastError = msg.Err
		return m, nil

	case ToolCallStartMsg:
		m.currentTool = &msg
		m.messages = append(m.messages, ChatMessage{
			Role:     RoleTool,
			Content:  fmt.Sprintf("🔧 Calling: %s", msg.ToolName),
			ToolName: msg.ToolName,
			ToolID:   msg.ToolID,
		})
		m.updateViewport()
		return m, m.spinner.Tick

	case ToolCallResultMsg:
		m.currentTool = nil
		content := msg.Result
		if msg.Error != nil {
			content = fmt.Sprintf("❌ Error: %v", msg.Error)
		} else {
			content = fmt.Sprintf("✅ %s completed", msg.ToolName)
		}
		m.messages = append(m.messages, ChatMessage{
			Role:     RoleTool,
			Content:  content,
			ToolName: msg.ToolName,
			ToolID:   msg.ToolID,
		})
		m.updateViewport()
		return m, nil

	case ToolCallCancelledMsg:
		m.currentTool = nil
		m.messages = append(m.messages, ChatMessage{
			Role:     RoleSystem,
			Content:  fmt.Sprintf("⏹️ Tool cancelled: %s", msg.ToolName),
			ToolName: msg.ToolName,
		})
		m.updateViewport()
		return m, nil

	case ApprovalRequestMsg:
		m.view = ViewApproval
		m.pendingApproval = &msg
		return m, nil

	case MCPConnectedMsg:
		m.serverCount = msg.ServerCount
		m.toolCount = msg.ToolCount
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: fmt.Sprintf("✅ Connected: %d servers, %d tools", msg.ServerCount, msg.ToolCount),
		})
		m.updateViewport()
		return m, nil

	case InitCompleteMsg:
		m.ready = true
		if m.agent != nil {
			m.modelName = m.agent.GetModelName()
			m.serverCount = m.agent.GetServerCount()
			m.toolCount = m.agent.GetToolCount()
		}
		return m, nil

	case userMessageMsg:
		if m.agent != nil && msg.Content != "" {
			m.isProcessing = true
			m.hasUserInteracted = true // User has started chatting
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleUser,
				Content: msg.Content,
			})
			m.updateViewport()
			return m, m.agent.ProcessMessage(msg.Content)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case FileSelectedMsg:
		m.view = ViewChat
		// Insert file path into textarea
		currentText := m.textarea.Value()
		m.textarea.SetValue(currentText + " `" + msg.Path + "` ")
		m.textarea.Focus()
		return m, nil

	case FilePickerCancelledMsg:
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil

	case ThemeChangedMsg:
		m.styles = NewStyles(msg.Theme)
		m.isDarkTheme = msg.Theme.Name == "dark"
		return m, nil

	case IndexingProgressMsg:
		// Update indexing progress state
		m.indexingCurrent = msg.Current
		m.indexingTotal = msg.Total
		m.indexingFile = msg.File

		// Generate progress bar
		progress := float64(msg.Current) / float64(msg.Total)
		barWidth := 20
		filled := int(progress * float64(barWidth))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

		// Update or add progress message in chat
		progressMsg := fmt.Sprintf("📊 Indexing [%s] %d/%d - %s", bar, msg.Current, msg.Total, msg.File)

		// Replace last message if it's a progress update, or add new
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleSystem &&
			strings.HasPrefix(m.messages[len(m.messages)-1].Content, "📊 Indexing") {
			m.messages[len(m.messages)-1].Content = progressMsg
		} else {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: progressMsg,
			})
		}
		m.updateViewport()
		return m, nil

	case IndexingCompleteMsg:
		// Clear the progress message and show completion
		if msg.AlreadyIndexed {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("ℹ️ Directory already indexed (%d files, %d chunks)", msg.FilesProcessed, msg.ChunksProcessed),
			})
		} else {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("✅ Indexing complete! %d files → %d chunks", msg.FilesProcessed, msg.ChunksProcessed),
			})
		}
		m.indexingCurrent = 0
		m.indexingTotal = 0
		m.indexingFile = ""
		m.updateViewport()
		return m, nil

	case IndexingErrorMsg:
		m.lastError = msg.Err
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: fmt.Sprintf("❌ Indexing failed: %v", msg.Err),
		})
		m.updateViewport()
		return m, nil

	case FileSelectedInTreeMsg:
		// Load file into code panel
		if m.codePanel != nil {
			if err := m.codePanel.LoadFile(msg.Path); err != nil {
				m.messages = append(m.messages, ChatMessage{
					Role:    RoleSystem,
					Content: fmt.Sprintf("❌ Failed to load file: %v", err),
				})
				m.updateViewport()
			} else {
				// Switch focus to code panel when file is loaded
				m.panelFocus = PanelCode
				m.messages = append(m.messages, ChatMessage{
					Role:    RoleSystem,
					Content: fmt.Sprintf("✅ Loaded: %s", msg.Path),
				})
				m.updateViewport()
				return m, m.requestDiagnosticsCmd(msg.Path)
			}
		}
		return m, nil

	case CodePreviewUpdatedMsg:
		// Update code preview
		if m.codePanel != nil {
			m.codePanel.SetContent(msg.Content, msg.Language)
		}
		return m, nil

	case GitDiffResultMsg:
		// Handle git diff result
		if m.diffPanel != nil {
			if msg.Error != nil {
				m.diffPanel.Error = msg.Error
			} else {
				m.diffPanel.Error = nil
				m.diffPanel.Staged = msg.Staged
				m.diffPanel.ParseDiff(msg.Diff)
			}
			m.diffPanel.Width = m.width - 6
			m.diffPanel.Height = m.height - 4
		}
		return m, nil

	case CommitMessageRequestMsg:
		// Request AI to generate commit message
		if m.agent != nil {
			return m, m.agent.GenerateCommitMessage(msg.DiffContent, msg.FilesChanged, msg.Additions, msg.Deletions)
		}
		return m, nil

	case CommitMessageResultMsg:
		// Handle commit message result
		if m.diffPanel != nil {
			m.diffPanel.SetCommitMessage(msg.Message, msg.Error)
		}
		return m, nil

	case TokenStatsUpdatedMsg:
		// Update token stats display
		m.inputTokens = msg.InputTokens
		m.outputTokens = msg.OutputTokens
		m.totalContextTokens = msg.TotalContextTokens
		m.avgTokensPerSecond = msg.AvgTokensPerSecond
		return m, nil

	case ConversationSavedMsg:
		// Show save confirmation
		if msg.Error != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("❌ Failed to save conversation: %v", msg.Error),
			})
		} else {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("✅ Conversation saved (ID: %s)", msg.ID),
			})
		}
		m.updateViewport()
		return m, nil

	case ConversationLoadedMsg:
		// Handle loaded conversation
		if msg.Error != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("❌ Failed to load conversation: %v", msg.Error),
			})
		} else {
			m.messages = []ChatMessage{{
				Role:    RoleSystem,
				Content: fmt.Sprintf("📂 Loaded conversation: %s (ID: %s)", msg.Title, msg.ID),
			}}
			m.hasUserInteracted = true
		}
		m.updateViewport()
		return m, nil

	case ConversationListMsg:
		// Store conversation list for history view
		if msg.Error != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("❌ Failed to list conversations: %v", msg.Error),
			})
			m.view = ViewChat
		} else {
			m.conversationList = msg.Conversations
			m.conversationCursor = 0
		}
		m.updateViewport()
		return m, nil

	case UndoResultMsg:
		if msg.Error != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("⚠️ %v", msg.Error),
			})
		} else {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("↩️ Undo successful (%d more available)", msg.UndoCount),
			})
		}
		m.updateViewport()
		return m, nil

	case RedoResultMsg:
		if msg.Error != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("⚠️ %v", msg.Error),
			})
		} else {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("↪️ Redo successful (%d more available)", msg.RedoCount),
			})
		}
		m.updateViewport()
		return m, nil

	case PreviewAcceptedMsg:
		// Apply the accepted change
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: fmt.Sprintf("✅ Applied change to %s", msg.Change.Path),
		})
		// Check if more changes pending
		if m.previewPanel != nil && !m.previewPanel.HasPendingChanges() {
			m.view = ViewChat
			m.textarea.Focus()
		}
		m.updateViewport()
		return m, nil

	case PreviewRejectedMsg:
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: fmt.Sprintf("❌ Rejected change to %s", msg.Change.Path),
		})
		if m.previewPanel != nil && !m.previewPanel.HasPendingChanges() {
			m.view = ViewChat
			m.textarea.Focus()
		}
		m.updateViewport()
		return m, nil

	case PreviewAcceptAllMsg:
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: fmt.Sprintf("✅ Applied %d changes", len(msg.Changes)),
		})
		m.view = ViewChat
		m.textarea.Focus()
		m.updateViewport()
		return m, nil

	case PreviewRejectedAllMsg:
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: "❌ Rejected all pending changes",
		})
		m.view = ViewChat
		m.textarea.Focus()
		m.updateViewport()
		return m, nil

	case PreviewRequestedMsg:
		// Add change to preview and switch to preview view
		if m.previewPanel != nil {
			m.previewPanel.AddChange(msg.Path, msg.Original, msg.New, msg.Operation, msg.ToolName)
			m.view = ViewPreview
		}
		return m, nil

	case LSPStartedMsg:
		if msg.Error != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("⚠️ LSP failed to start: %v", msg.Error),
			})
		} else {
			if client, ok := msg.Client.(*lsp.GoClient); ok {
				m.lspClient = client
				m.messages = append(m.messages, ChatMessage{
					Role:    RoleSystem,
					Content: "✅ LSP (gopls) started",
				})
			}
		}
		return m, nil

	case DiagnosticMsg:
		if m.codePanel != nil && m.codePanel.FilePath == msg.FilePath {
			if diags, ok := msg.Diagnostics.([]lsp.Diagnostic); ok {
				m.codePanel.SetDiagnostics(diags)
			}
		}
		return m, nil

	case HoverMsg:
		if m.codePanel != nil {
			if msg.Error != nil {
				// Optionally show error or just ignore
			} else {
				m.codePanel.SetHoverContent(msg.Content)
			}
		}
		return m, nil

	case DefinitionMsg:
		if msg.Error != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("⚠️ Definition lookup failed: %v", msg.Error),
			})
		} else {
			// Open the file at the definition
			if msg.FilePath != "" && m.codePanel != nil {
				if err := m.codePanel.LoadFile(msg.FilePath); err == nil {
					// Navigate to line (LSP is 0-based, CodePanel is 1-based)
					line := msg.Line + 1
					// We need a method to scroll to line in CodePanel, let's manually set it for now
					m.codePanel.CursorLine = line
					// Adjust offset to center
					visibleHeight := m.codePanel.Height - 4
					newOffset := line - (visibleHeight / 2)
					if newOffset < 0 {
						newOffset = 0
					}
					m.codePanel.Offset = newOffset
					m.panelFocus = PanelCode
				}
			}
		}
		return m, nil

	// ============================================
	// Agentic Mode Messages
	// ============================================

	case AgenticModeStartMsg:
		// Start agentic task planning - stay in chat view, show progress inline
		m.isAgenticMode = true
		m.hasUserInteracted = true

		// Show task starting message in chat
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleUser,
			Content: msg.Request,
		})
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: "🎯 **Starting Task**\n\n🔍 Analyzing project structure...",
		})
		m.isProcessing = true
		m.updateViewport()
		if agent, ok := m.agent.(*Agent); ok {
			return m, agent.StartAgenticTask(msg.Request)
		}
		return m, nil

	case AgenticTaskCreatedMsg:
		// Task plan created - stay in chat, show summary
		if m.taskPanel != nil {
			if agent, ok := m.agent.(*Agent); ok {
				m.taskPanel.SetTask(agent.GetAgenticTask())
				m.taskPanel.SetSize(m.width-4, m.height-4)
			}
		}
		// Show plan summary in chat
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: fmt.Sprintf("📋 **Plan: %s**\n\n%d steps ready. Type `Y` to approve, `N` to cancel, or `/taskview` to see details.", msg.Title, msg.TodoCount),
		})
		m.isProcessing = false
		m.updateViewport()
		// Don't auto-start - wait for approval
		return m, nil

	case AgenticTaskUpdateMsg:
		// Update task panel with progress
		if m.taskPanel != nil {
			// The task panel will auto-refresh since it references the task
		}
		return m, nil

	case AgenticPhaseChangedMsg:
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: fmt.Sprintf("📍 Phase: %s", msg.NewPhase),
		})
		m.updateViewport()
		return m, nil

	case AgenticModeExitMsg:
		m.isAgenticMode = false
		// Keep task panel visible after completion so user can review
		// Only auto-close on cancellation
		if msg.Completed {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: "✅ Agentic task completed! (Press Esc to close task panel)",
			})
			// Stay in ViewAgenticTask so user can review
		} else if msg.Cancelled {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: "⛔ Agentic task cancelled",
			})
			m.view = ViewChat
			m.textarea.Focus()
		}
		m.updateViewport()
		return m, nil

	case AgenticTaskErrorMsg:
		m.isAgenticMode = false
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: fmt.Sprintf("❌ Agentic error: %v", msg.Error),
		})
		// Keep task panel visible so user can see what failed
		m.updateViewport()
		return m, nil

	case AgenticPauseMsg:
		if agent, ok := m.agent.(*Agent); ok {
			agent.PauseAgenticTask()
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: "⏸️ Task paused",
			})
			m.updateViewport()
		}
		return m, nil

	case AgenticResumeMsg:
		if agent, ok := m.agent.(*Agent); ok {
			agent.ResumeAgenticTask()
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: "▶️ Task resumed",
			})
			m.updateViewport()
		}
		return m, nil

	case AgenticCancelMsg:
		if agent, ok := m.agent.(*Agent); ok {
			agent.CancelAgenticTask()
		}
		return m, nil

	case AgenticSkipTodoMsg:
		if agent, ok := m.agent.(*Agent); ok {
			agent.SkipAgenticTodo(msg.TodoID)
		}
		return m, nil

	// ============================================
	// Proposal & Task List Messages
	// ============================================

	case AgenticProposalReadyMsg:
		// Show proposal for approval
		if m.proposalPanel != nil {
			if proposal, ok := msg.Proposal.(*agentic.TaskProposal); ok {
				m.proposalPanel.SetProposal(proposal)
				m.proposalPanel.SetSize(m.width-10, m.height-6)
			}
		}
		m.view = ViewAgenticProposal
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: "📝 Task proposal ready for review",
		})
		m.updateViewport()
		return m, nil

	case AgenticProposalApprovedMsg:
		// User approved, start execution
		m.view = ViewAgenticTask
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: "✅ Proposal approved, starting execution...",
		})
		m.updateViewport()
		if agent, ok := m.agent.(*Agent); ok {
			return m, agent.ExecuteApprovedTask()
		}
		return m, nil

	case AgenticProposalRejectedMsg:
		// User rejected, return to chat
		m.isAgenticMode = false
		m.view = ViewChat
		m.hasUserInteracted = false
		m.textarea.Focus()
		m.messages = append(m.messages, ChatMessage{
			Role:    RoleSystem,
			Content: "❌ Proposal rejected",
		})
		m.updateViewport()
		return m, nil

	case AgenticLoadTaskListMsg:
		// Load task list
		m.view = ViewAgenticTaskList
		if m.taskListPanel != nil {
			m.taskListPanel.Loading = true
			m.taskListPanel.SetSize(m.width-10, m.height-6)
		}
		if agent, ok := m.agent.(*Agent); ok {
			return m, agent.LoadTaskList()
		}
		return m, nil

	case AgenticTaskListLoadedMsg:
		// Populate task list panel
		if m.taskListPanel != nil {
			if msg.Error != nil {
				m.taskListPanel.Error = msg.Error
				m.taskListPanel.Loading = false
			} else {
				var summaries []agentic.TaskSummary
				for _, t := range msg.Tasks {
					if summary, ok := t.(agentic.TaskSummary); ok {
						summaries = append(summaries, summary)
					}
				}
				m.taskListPanel.SetTasks(summaries)
			}
		}
		return m, nil

	case AgenticTaskSelectedMsg:
		// Load and show selected task
		if agent, ok := m.agent.(*Agent); ok {
			return m, agent.LoadAndShowTask(msg.TaskID)
		}
		return m, nil

	case AgenticDeleteTaskMsg:
		// Delete a task
		if agent, ok := m.agent.(*Agent); ok {
			return m, agent.DeleteTask(msg.TaskID)
		}
		return m, nil

	case AgenticTaskDeletedMsg:
		if msg.Error != nil {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("❌ Failed to delete task: %v", msg.Error),
			})
		} else {
			m.messages = append(m.messages, ChatMessage{
				Role:    RoleSystem,
				Content: fmt.Sprintf("🗑️ Task %s deleted", msg.TaskID),
			})
			// Refresh list
			if agent, ok := m.agent.(*Agent); ok {
				return m, agent.LoadTaskList()
			}
		}
		m.updateViewport()
		return m, nil
	}

	// Update textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case ViewApproval:
		return m.handleApprovalKeys(msg)
	case ViewHelp:
		if msg.Type == tea.KeyEsc || msg.String() == "q" {
			m.view = ViewChat
			m.textarea.Focus()
		}
		return m, nil
	case ViewFilePicker:
		return m.handleFilePickerKeys(msg)
	case ViewSettings:
		return m.handleSettingsKeys(msg)
	case ViewDiff:
		return m.handleDiffKeys(msg)
	case ViewHistory:
		return m.handleHistoryKeys(msg)
	case ViewPreview:
		return m.handlePreviewKeys(msg)
	case ViewAgenticTask:
		return m.handleAgenticKeys(msg)
	case ViewAgenticProposal:
		return m.handleProposalKeys(msg)
	case ViewAgenticTaskList:
		return m.handleTaskListKeys(msg)
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		if m.currentTool != nil && m.agent != nil {
			m.agent.CancelCurrentTool()
			return m, nil
		}
		return m, tea.Quit

	case tea.KeyCtrlQ:
		return m, tea.Quit

	case tea.KeyCtrlZ:
		// Undo last file change
		if m.agent != nil {
			return m, m.agent.Undo()
		}
		return m, nil

	case tea.KeyCtrlY:
		// Redo last undone change
		if m.agent != nil {
			return m, m.agent.Redo()
		}
		return m, nil

	case tea.KeyEsc:
		// Close command menu if open
		if m.showCommandMenu {
			m.showCommandMenu = false
			return m, nil
		}

	case tea.KeyUp:
		// Navigate command menu
		if m.showCommandMenu {
			filtered := m.getFilteredCommands()
			if len(filtered) > 0 {
				m.commandMenuCursor--
				if m.commandMenuCursor < 0 {
					m.commandMenuCursor = len(filtered) - 1
				}
			}
			return m, nil
		}

	case tea.KeyDown:
		// Navigate command menu
		if m.showCommandMenu {
			filtered := m.getFilteredCommands()
			if len(filtered) > 0 {
				m.commandMenuCursor++
				if m.commandMenuCursor >= len(filtered) {
					m.commandMenuCursor = 0
				}
			}
			return m, nil
		}

	case tea.KeyEnter:
		// Select from command menu
		if m.showCommandMenu {
			filtered := m.getFilteredCommands()
			if len(filtered) > 0 && m.commandMenuCursor < len(filtered) {
				command := filtered[m.commandMenuCursor].Command
				m.textarea.Reset()
				m.showCommandMenu = false
				m.commandMenuCursor = 0
				newView, cmd := m.handleCommand(command)
				m.view = newView
				return m, cmd
			}
		}
		// Normal message submission
		if m.view == ViewChat && !m.isProcessing && m.panelFocus == PanelChat {
			content := strings.TrimSpace(m.textarea.Value())
			if content != "" {
				m.textarea.Reset()
				m.showCommandMenu = false
				newView, cmd := m.handleCommand(content)
				m.view = newView
				return m, cmd
			}
		}

	case tea.KeyCtrlL:
		// Clear chat
		m.messages = []ChatMessage{}
		m.updateViewport()
		return m, nil

	case tea.KeyPgUp:
		m.viewport.ViewUp()
		return m, nil

	case tea.KeyPgDown:
		m.viewport.ViewDown()
		return m, nil

	case tea.KeyTab:
		// Autocomplete from command menu if open
		if m.showCommandMenu {
			filtered := m.getFilteredCommands()
			if len(filtered) > 0 && m.commandMenuCursor < len(filtered) {
				m.textarea.SetValue(filtered[m.commandMenuCursor].Command + " ")
				m.showCommandMenu = false
				m.commandMenuCursor = 0
			}
			return m, nil
		}
		// Switch panel focus
		m.panelFocus++
		if m.panelFocus > PanelCode {
			m.panelFocus = PanelFileTree
		}
		return m, nil

	case tea.KeyShiftTab:
		// Switch panel focus reverse
		m.panelFocus--
		if m.panelFocus < PanelFileTree {
			m.panelFocus = PanelCode
		}
		return m, nil

	case tea.KeyF1:
		m.showSidebar = !m.showSidebar
		return m, nil

	case tea.KeyF2:
		m.showCodePanel = !m.showCodePanel
		return m, nil

	case tea.KeyF3:
		// Toggle diff view
		if m.view == ViewDiff {
			m.view = ViewChat
			m.textarea.Focus()
		} else {
			m.view = ViewDiff
			return m, FetchDiff(false, "")
		}
		return m, nil
	}

	// Handle panel-specific input
	switch m.panelFocus {
	case PanelFileTree:
		if m.fileTree != nil {
			newTree, cmd := m.fileTree.Update(msg)
			m.fileTree = newTree
			return m, cmd
		}
	case PanelCode:
		if m.codePanel != nil {
			// LSP shortcuts
			if m.lspClient != nil {
				if msg.Type == tea.KeyF12 {
					return m, m.requestDefinitionCmd(m.codePanel.FilePath, m.codePanel.CursorLine-1, m.codePanel.CursorCol)
				} else if msg.String() == "K" {
					return m, m.requestHoverCmd(m.codePanel.FilePath, m.codePanel.CursorLine-1, m.codePanel.CursorCol)
				}
			}
			newPanel, cmd := m.codePanel.Update(msg)
			m.codePanel = newPanel
			return m, cmd
		}
	case PanelChat:
		// Handle textarea input
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		m.updateCommandMenu()
		return m, cmd
	}

	// Default: handle textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	m.updateCommandMenu()
	return m, cmd
}

// handleApprovalKeys handles key input during approval dialog
func (m Model) handleApprovalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.pendingApproval != nil {
			m.view = ViewChat
			approval := m.pendingApproval
			m.pendingApproval = nil
			return m, func() tea.Msg {
				return ApprovalResponseMsg{ToolID: approval.ToolID, Approved: true}
			}
		}
	case "n", "N", "esc":
		if m.pendingApproval != nil {
			m.view = ViewChat
			approval := m.pendingApproval
			m.pendingApproval = nil
			return m, func() tea.Msg {
				return ApprovalResponseMsg{ToolID: approval.ToolID, Approved: false}
			}
		}
	}
	return m, nil
}

// handleFilePickerKeys handles key input when file picker is active
func (m Model) handleFilePickerKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filePicker == nil {
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil
	}

	if msg.Type == tea.KeyEsc || msg.String() == "q" {
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil
	}

	newPicker, cmd := m.filePicker.Update(msg)
	m.filePicker = &newPicker

	return m, cmd
}

// handleSettingsKeys handles key input in settings view
func (m Model) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil

	case "left", "h", "up", "k":
		// Cycle accent color backward
		if m.accentColor > 0 {
			m.accentColor--
		} else {
			m.accentColor = AccentIndigo // Wrap to last
		}
		return m, m.applyAccentColor()

	case "right", "l", "down", "j", "enter", " ":
		// Cycle accent color forward
		if m.accentColor < AccentIndigo {
			m.accentColor++
		} else {
			m.accentColor = AccentBlue // Wrap to first
		}
		return m, m.applyAccentColor()
	}

	return m, nil
}

// handleDiffKeys handles key input in diff view
func (m Model) handleDiffKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if commit message panel is open - let diff panel handle esc/q first
	if m.diffPanel != nil && m.diffPanel.ShowCommitMessage {
		newPanel, cmd := m.diffPanel.Update(msg)
		m.diffPanel = newPanel
		return m, cmd
	}

	switch msg.String() {
	case "esc", "q":
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil
	}

	// Delegate to diff panel for navigation
	if m.diffPanel != nil {
		newPanel, cmd := m.diffPanel.Update(msg)
		m.diffPanel = newPanel
		return m, cmd
	}

	return m, nil
}

// handleHistoryKeys handles key input in history view
func (m Model) handleHistoryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil

	case "up", "k":
		if m.conversationCursor > 0 {
			m.conversationCursor--
		}
		return m, nil

	case "down", "j":
		if m.conversationCursor < len(m.conversationList)-1 {
			m.conversationCursor++
		}
		return m, nil

	case "enter":
		if len(m.conversationList) > 0 && m.conversationCursor < len(m.conversationList) {
			id := m.conversationList[m.conversationCursor].ID
			m.view = ViewChat
			return m, m.loadConversationCmd(id)
		}
		return m, nil

	case "d":
		// Delete selected conversation
		if len(m.conversationList) > 0 && m.conversationCursor < len(m.conversationList) {
			id := m.conversationList[m.conversationCursor].ID
			if m.agent != nil {
				_ = m.agent.DeleteConversation(id)
				// Refresh the list
				return m, m.listConversationsCmd()
			}
		}
		return m, nil
	}

	return m, nil
}

// handlePreviewKeys handles key input in diff preview view
func (m Model) handlePreviewKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" || msg.String() == "q" {
		// Close preview without accepting/rejecting
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil
	}

	// Delegate to preview panel
	if m.previewPanel != nil {
		newPanel, cmd := m.previewPanel.Update(msg)
		m.previewPanel = newPanel
		return m, cmd
	}

	return m, nil
}

// handleAgenticKeys handles key input in agentic task view
func (m Model) handleAgenticKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// If task is not actively running, return to welcome screen
		if m.taskPanel != nil && m.taskPanel.Task != nil {
			status := m.taskPanel.Task.Status
			if status == agentic.TaskCompleted ||
				status == agentic.TaskCancelled ||
				status == agentic.TaskFailed {
				// Task is done, return to welcome screen
				m.view = ViewChat
				m.hasUserInteracted = false // Shows welcome screen
				m.textarea.Focus()
				return m, nil
			}
		}
		// If actively running, just minimize to chat view (keep running)
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil
	case "q":
		// Only q exits without going to welcome
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil
	}

	// Delegate to task panel
	if m.taskPanel != nil {
		newPanel, cmd := m.taskPanel.Update(msg)
		m.taskPanel = newPanel
		return m, cmd
	}

	return m, nil
}

// handleProposalKeys handles key input in proposal view
func (m Model) handleProposalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel and return to chat with welcome
		m.view = ViewChat
		m.hasUserInteracted = false
		m.textarea.Focus()
		return m, nil
	}

	// Delegate to proposal panel
	if m.proposalPanel != nil {
		newPanel, cmd := m.proposalPanel.Update(msg)
		m.proposalPanel = newPanel
		return m, cmd
	}

	return m, nil
}

// handleTaskListKeys handles key input in task list view
func (m Model) handleTaskListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.view = ViewChat
		m.textarea.Focus()
		return m, nil
	}

	// Delegate to task list panel
	if m.taskListPanel != nil {
		newPanel, cmd := m.taskListPanel.Update(msg)
		m.taskListPanel = newPanel
		return m, cmd
	}

	return m, nil
}

// applyAccentColor applies the current accent color to the theme

func (m *Model) applyAccentColor() tea.Cmd {
	color := lipgloss.Color(m.accentColor.Hex())

	// Update theme accent colors
	m.styles.Theme.Primary = color
	m.styles.Theme.Highlight = color
	m.styles.Theme.BorderFocused = color
	m.styles.Theme.Info = color

	// Rebuild styles with updated theme
	m.styles = NewStyles(m.styles.Theme)

	return nil
}

// getFilteredCommands returns commands that match the current input
func (m *Model) getFilteredCommands() []CommandInfo {
	input := strings.TrimSpace(m.textarea.Value())
	if !strings.HasPrefix(input, "/") {
		return []CommandInfo{}
	}

	var filtered []CommandInfo
	for _, cmd := range AvailableCommands() {
		if strings.HasPrefix(strings.ToLower(cmd.Command), strings.ToLower(input)) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

// updateCommandMenu updates the command menu visibility based on input
func (m *Model) updateCommandMenu() {
	input := strings.TrimSpace(m.textarea.Value())

	// Show menu when input starts with /
	if strings.HasPrefix(input, "/") && len(input) >= 1 {
		m.showCommandMenu = true
		// Reset cursor if filtered list changed
		filtered := m.getFilteredCommands()
		if m.commandMenuCursor >= len(filtered) {
			m.commandMenuCursor = 0
		}
	} else {
		m.showCommandMenu = false
		m.commandMenuCursor = 0
	}
}

// handleCommand processes special commands
func (m Model) handleCommand(content string) (ViewState, tea.Cmd) {
	lower := strings.ToLower(content)

	// Handle inline approval for agentic mode
	if m.isAgenticMode && m.taskPanel != nil && m.taskPanel.Task != nil {
		if m.taskPanel.Task.Status == agentic.TaskAwaitingApproval {
			switch lower {
			case "y", "yes":
				// Approve and execute
				m.messages = append(m.messages, ChatMessage{
					Role:    RoleUser,
					Content: "Y",
				})
				return ViewChat, func() tea.Msg {
					return AgenticProposalApprovedMsg{TaskID: m.taskPanel.Task.ID}
				}
			case "n", "no":
				// Reject
				m.messages = append(m.messages, ChatMessage{
					Role:    RoleUser,
					Content: "N",
				})
				return ViewChat, func() tea.Msg {
					return AgenticProposalRejectedMsg{TaskID: m.taskPanel.Task.ID}
				}
			}
		}
	}

	switch {
	case lower == "/help" || lower == "help":
		return ViewHelp, nil

	case lower == "/settings" || lower == "settings":
		return ViewSettings, nil

	case lower == "/clear":
		m.messages = []ChatMessage{}
		m.hasUserInteracted = false // Show welcome screen again
		if m.agent != nil {
			m.agent.ClearConversation()
		}
		m.updateViewport()
		return ViewChat, nil

	case lower == "/new":
		m.messages = []ChatMessage{}
		m.hasUserInteracted = false // Show welcome screen again
		if m.agent != nil {
			m.agent.NewConversationSession()
		}
		m.updateViewport()
		return ViewChat, nil

	case lower == "/index":
		if m.agent != nil {
			return ViewChat, m.agent.IndexCodebase()
		}
		return ViewChat, nil

	case lower == "exit" || lower == "quit" || lower == "bye":
		return ViewChat, tea.Quit

	case lower == "/diff":
		// Show unstaged changes
		return ViewDiff, FetchDiff(false, "")

	case lower == "/diff --staged" || lower == "/diff -s":
		// Show staged changes
		return ViewDiff, FetchDiff(true, "")

	case lower == "/history":
		// Show conversation history
		return ViewHistory, m.listConversationsCmd()

	case lower == "/save":
		// Save current conversation
		return ViewChat, m.saveConversationCmd()

	case strings.HasPrefix(lower, "/load "):
		// Load conversation by ID
		id := strings.TrimSpace(content[6:])
		return ViewChat, m.loadConversationCmd(id)

	case lower == "/undo":
		// Undo last file change
		if m.agent != nil {
			return ViewChat, m.agent.Undo()
		}
		return ViewChat, nil

	case lower == "/redo":
		// Redo last undone change
		if m.agent != nil {
			return ViewChat, m.agent.Redo()
		}
		return ViewChat, nil

	case strings.HasPrefix(lower, "/task "):
		// Start agentic task mode
		request := strings.TrimSpace(content[6:])
		if request == "" {
			return ViewChat, nil
		}
		return ViewAgenticTask, func() tea.Msg {
			return AgenticModeStartMsg{Request: request}
		}

	case strings.HasPrefix(lower, "/plan "):
		// Plan a task (same as /task for now)
		request := strings.TrimSpace(content[6:])
		if request == "" {
			return ViewChat, nil
		}
		return ViewAgenticTask, func() tea.Msg {
			return AgenticModeStartMsg{Request: request}
		}

	case lower == "/taskview":
		// View current/last agentic task
		if m.taskPanel != nil && m.taskPanel.Task != nil {
			return ViewAgenticTask, nil
		}
		// No task to view
		return ViewChat, nil

	case lower == "/tasks":
		// Show all agentic tasks
		return ViewAgenticTaskList, func() tea.Msg {
			return AgenticLoadTaskListMsg{}
		}

	case strings.HasPrefix(content, "@"):
		return ViewFilePicker, nil

	default:
		return ViewChat, SendMessageCmd(content)
	}
}

// listConversationsCmd creates a command to list saved conversations
func (m Model) listConversationsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.agent == nil {
			return ConversationListMsg{Error: fmt.Errorf("agent not initialized")}
		}

		list, err := m.agent.ListConversations()
		if err != nil {
			return ConversationListMsg{Error: err}
		}

		summaries := make([]ConversationSummaryInfo, len(list))
		for i, c := range list {
			summaries[i] = ConversationSummaryInfo{
				ID:           c.ID,
				Title:        c.Title,
				Preview:      c.Preview,
				MessageCount: c.MessageCount,
				UpdatedAt:    c.UpdatedAt.Format("Jan 2 15:04"),
			}
		}

		return ConversationListMsg{Conversations: summaries}
	}
}

// saveConversationCmd creates a command to save the current conversation
func (m Model) saveConversationCmd() tea.Cmd {
	return func() tea.Msg {
		if m.agent == nil {
			return ConversationSavedMsg{Error: fmt.Errorf("agent not initialized")}
		}

		err := m.agent.SaveConversation()
		if err != nil {
			return ConversationSavedMsg{Error: err}
		}

		return ConversationSavedMsg{
			ID:    m.agent.GetCurrentConversationID(),
			Title: m.agent.GetCurrentConversationTitle(),
		}
	}
}

// loadConversationCmd creates a command to load a conversation
func (m Model) loadConversationCmd(id string) tea.Cmd {
	return func() tea.Msg {
		if m.agent == nil {
			return ConversationLoadedMsg{Error: fmt.Errorf("agent not initialized")}
		}

		err := m.agent.LoadConversation(id)
		if err != nil {
			return ConversationLoadedMsg{Error: err}
		}

		return ConversationLoadedMsg{
			ID:    m.agent.GetCurrentConversationID(),
			Title: m.agent.GetCurrentConversationTitle(),
		}
	}
}

// handleWindowSize updates layout when terminal is resized
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	// Use consistent height calculation with renderChat
	// Account for: header (2) + input area (5) + status bar (1) + panel borders (4)
	chatHeight := m.height - headerHeight() - inputHeight() - 1 - 4
	if chatHeight < 10 {
		chatHeight = 10
	}

	if !m.ready {
		m.viewport = viewport.New(m.width, chatHeight)
		m.viewport.YPosition = headerHeight()
		m.ready = true
	} else {
		m.viewport.Width = m.width
		m.viewport.Height = chatHeight
	}

	m.textarea.SetWidth(m.width - 4)
	m.updateViewport()

	return m, nil
}

// handleStreamChunk processes streaming content from AI
func (m Model) handleStreamChunk(msg StreamChunkMsg) (tea.Model, tea.Cmd) {
	if msg.Done {
		m.isProcessing = false
		// Mark the last assistant message as not streaming
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleAssistant {
			m.messages[len(m.messages)-1].Streaming = false
		}
		m.updateViewport()
		return m, nil
	}

	// Append to existing assistant message or create new one
	if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleAssistant && m.messages[len(m.messages)-1].Streaming {
		m.messages[len(m.messages)-1].Content += msg.Content
	} else {
		m.messages = append(m.messages, ChatMessage{
			Role:      RoleAssistant,
			Content:   msg.Content,
			Streaming: true,
		})
	}
	m.updateViewport()

	return m, nil
}

// updateViewport refreshes the chat viewport content
func (m *Model) updateViewport() {
	var content strings.Builder

	for i := range m.messages {
		// Use pointer to update cache in place
		msg := &m.messages[i]

		// If we have cached content and not streaming, use it
		if msg.RenderedContent != "" && !msg.Streaming {
			content.WriteString(msg.RenderedContent)
		} else {
			// Render and cache
			rendered := m.renderMessage(*msg)
			if !msg.Streaming {
				msg.RenderedContent = rendered
			}
			content.WriteString(rendered)
		}
		content.WriteString("\n")
	}

	if m.currentTool != nil {
		content.WriteString(m.renderToolPanel())
	}

	m.viewport.SetContent(content.String())
	m.viewport.GotoBottom()
}

// renderMessage renders a single chat message
func (m *Model) renderMessage(msg ChatMessage) string {
	// Use cached content if available and not streaming
	if msg.RenderedContent != "" && !msg.Streaming {
		return msg.RenderedContent
	}

	var label, content string

	// Calculate max width for content (leave room for label)
	maxWidth := m.width - 20
	if maxWidth < 40 {
		maxWidth = 40
	}

	switch msg.Role {
	case RoleUser:
		label = m.styles.MessageLabel.Foreground(m.styles.Theme.UserMessage).Render("You ▸")
		// User messages get a subtle left border to stand out (theme-aware)
		userStyle := lipgloss.NewStyle().
			Foreground(m.styles.Theme.UserMessage).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(m.styles.Theme.Primary).
			PaddingLeft(1).
			Width(maxWidth)
		content = userStyle.Render(msg.Content)

	case RoleAssistant:
		label = m.styles.MessageLabel.Foreground(m.styles.Theme.AssistantMessage).Render("Assistant ▸")
		// Try to render as markdown (with nil check for renderer)
		if m.renderer != nil {
			rendered, err := m.renderer.Render(msg.Content)
			if err == nil {
				content = strings.TrimSpace(rendered)
			} else {
				content = m.styles.MessageAssist.Width(maxWidth).Render(msg.Content)
			}
		} else {
			content = m.styles.MessageAssist.Width(maxWidth).Render(msg.Content)
		}
		if msg.Streaming {
			content += m.spinner.View()
		}

	case RoleSystem:
		label = m.styles.MessageLabel.Foreground(m.styles.Theme.SystemMessage).Render("System ▸")
		content = m.styles.MessageSystem.Width(maxWidth).Render(msg.Content)

	case RoleTool:
		label = m.styles.MessageLabel.Foreground(m.styles.Theme.ToolMessage).Render("Tool ▸")
		content = m.styles.MessageTool.Width(maxWidth).Render(msg.Content)
	}

	rendered := fmt.Sprintf("%s %s", label, content)

	// Cache the result if not streaming
	// We can't modify msg here since it's passed by value, but the caller (updateViewport)
	// will use the returned string. To actually cache, we need to update the slice in Model.
	// However, since this function is pure rendering, we'll update the cache in updateViewport loop instead.
	return rendered
}

// renderToolPanel renders the current tool execution panel
func (m *Model) renderToolPanel() string {
	if m.currentTool == nil {
		return ""
	}

	header := m.styles.ToolHeader.Render(fmt.Sprintf("🔧 Running: %s %s", m.currentTool.ToolName, m.spinner.View()))

	var argsContent string
	if len(m.currentTool.Args) > 0 {
		var args []string
		for k, v := range m.currentTool.Args {
			args = append(args, fmt.Sprintf("%s: %v", k, v))
		}
		argsContent = m.styles.ToolArgs.Render(strings.Join(args, "\n"))
	}

	content := header
	if argsContent != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, header, argsContent)
	}

	return m.styles.ToolPanel.Render(content)
}

// View implements tea.Model
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	switch m.view {
	case ViewHelp:
		return m.renderHelp()
	case ViewApproval:
		return m.renderApproval()
	case ViewFilePicker:
		return m.renderFilePicker()
	case ViewSettings:
		return m.renderSettings()
	case ViewDiff:
		return m.renderDiff()
	case ViewHistory:
		return m.renderHistory()
	case ViewPreview:
		return m.renderPreview()
	case ViewAgenticTask:
		return m.renderAgenticTask()
	case ViewAgenticProposal:
		return m.renderAgenticProposal()
	case ViewAgenticTaskList:
		return m.renderAgenticTaskList()
	default:

		// Show welcome screen if user hasn't started chatting yet
		if !m.hasUserInteracted && !m.isProcessing {
			return m.renderWelcome()
		}
		return m.renderChat()
	}
}

// renderChat renders the main chat view with split-pane layout
func (m Model) renderChat() string {
	// Header
	header := m.renderHeader()

	// Calculate available content height (accounting for header, input, status, borders)
	contentHeight := m.height - headerHeight() - inputHeight() - 1 - 4
	if contentHeight < 10 {
		contentHeight = 10
	}

	// Calculate layout dimensions with better proportions
	// Sidebar: ~20% of width (min 25, max 35)
	sidebarWidth := m.width / 5
	if sidebarWidth < 25 {
		sidebarWidth = 25
	}
	if sidebarWidth > 35 {
		sidebarWidth = 35
	}
	if !m.showSidebar {
		sidebarWidth = 0
	}

	// Code panel: ~35% of width (min 50 for readability)
	codePanelWidth := m.width * 35 / 100
	if codePanelWidth < 50 {
		codePanelWidth = 50
	}
	if !m.showCodePanel {
		codePanelWidth = 0
	}

	// Chat takes remaining space (minimum 45 chars for readability)
	chatWidth := m.width - sidebarWidth - codePanelWidth - 4
	if chatWidth < 45 {
		// If chat is too small, reduce code panel first
		neededSpace := 45 - chatWidth
		codePanelWidth -= neededSpace
		if codePanelWidth < 40 {
			codePanelWidth = 0 // Hide code panel if too cramped
		}
		chatWidth = m.width - sidebarWidth - codePanelWidth - 4
	}

	// Build sidebar (file tree)
	var sidebar string
	if m.showSidebar && m.fileTree != nil {
		m.fileTree.Width = sidebarWidth - 2
		m.fileTree.Height = contentHeight
		sidebarStyle := m.styles.SidebarPanel
		if m.panelFocus == PanelFileTree {
			sidebarStyle = sidebarStyle.BorderForeground(m.styles.Theme.BorderFocused)
		}
		sidebar = sidebarStyle.Width(sidebarWidth).Height(contentHeight + 2).Render(
			m.styles.SidebarTitle.Render("📁 Files") + "\n" + m.fileTree.View(),
		)
	}

	// Build code panel with proper sizing
	var codePanelStr string
	if m.showCodePanel && m.codePanel != nil && codePanelWidth > 0 {
		m.codePanel.Width = codePanelWidth - 2
		m.codePanel.Height = contentHeight
		codePanelStyle := m.styles.CodePanel
		if m.panelFocus == PanelCode {
			codePanelStyle = codePanelStyle.BorderForeground(m.styles.Theme.BorderFocused)
		}
		codePanelStr = codePanelStyle.Width(codePanelWidth).Height(contentHeight + 2).Render(m.codePanel.View())
	}

	// Chat viewport - use consistent height calculation
	m.viewport.Width = chatWidth - 2 // Account for border
	m.viewport.Height = contentHeight
	chatViewStyle := m.styles.PanelBorder
	if m.panelFocus == PanelChat {
		chatViewStyle = m.styles.PanelBorderFocus
	}
	chatView := chatViewStyle.Width(chatWidth).Height(contentHeight + 2).Render(m.viewport.View())

	// Combine middle section (sidebar, chat, code panel)
	middleSection := ""
	if sidebar != "" && codePanelStr != "" {
		middleSection = lipgloss.JoinHorizontal(
			lipgloss.Top,
			sidebar,
			chatView,
			codePanelStr,
		)
	} else if sidebar != "" {
		middleSection = lipgloss.JoinHorizontal(
			lipgloss.Top,
			sidebar,
			chatView,
		)
	} else if codePanelStr != "" {
		middleSection = lipgloss.JoinHorizontal(
			lipgloss.Top,
			chatView,
			codePanelStr,
		)
	} else {
		middleSection = chatView
	}

	// Command autocomplete dropdown
	var commandMenu string
	if m.showCommandMenu {
		commandMenu = m.renderCommandMenu()
	}

	// Input area - highlight when chat is focused
	inputStyle := m.styles.InputContainer
	if m.panelFocus == PanelChat {
		inputStyle = inputStyle.BorderForeground(m.styles.Theme.BorderFocused)
	}
	inputBox := inputStyle.
		Width(m.width - 4).
		Render(m.textarea.View())

	// Status bar
	statusBar := m.renderStatusBar()

	// Combine all elements
	if commandMenu != "" {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			header,
			middleSection,
			commandMenu,
			inputBox,
			statusBar,
		)
	}
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		middleSection,
		inputBox,
		statusBar,
	)
}

// headerHeight returns the height of the header
func headerHeight() int {
	return 2
}

// inputHeight returns the height of the input area
func inputHeight() int {
	return 5
}

// renderCommandMenu renders the command autocomplete dropdown
func (m Model) renderCommandMenu() string {
	filtered := m.getFilteredCommands()
	if len(filtered) == 0 {
		return ""
	}

	var content strings.Builder
	content.WriteString(m.styles.Help.Render("Commands") + "\n")

	for i, cmd := range filtered {
		prefix := "  "
		style := m.styles.FilePickerItem
		if i == m.commandMenuCursor {
			prefix = "▸ "
			style = m.styles.FilePickerSelected
		}
		line := fmt.Sprintf("%s%s  %s", prefix, cmd.Command, m.styles.Help.Render(cmd.Description))
		content.WriteString(style.Render(line) + "\n")
	}

	content.WriteString(m.styles.Help.Render("↑↓ navigate · Enter select · Tab complete · Esc close"))

	menuStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Theme.Primary).
		Padding(0, 1).
		Width(m.width - 6)

	return menuStyle.Render(content.String())
}

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	title := m.styles.HeaderTitle.Render("Open Coder")

	var info string
	if m.modelName != "" {
		info = m.styles.HeaderInfo.Render(fmt.Sprintf(" · %s", m.modelName))
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, info)

	gap := m.width - lipgloss.Width(left) - 4
	if gap < 0 {
		gap = 0
	}

	return m.styles.Header.Width(m.width).Render(
		left + strings.Repeat(" ", gap),
	)
}

// renderStatusBar renders the bottom status bar
func (m Model) renderStatusBar() string {
	var items []string

	// Panel focus indicator
	var focusLabel string
	switch m.panelFocus {
	case PanelFileTree:
		focusLabel = "Files"
	case PanelChat:
		focusLabel = "Chat"
	case PanelCode:
		focusLabel = "Code"
	}
	items = append(items, m.styles.StatusItem.Render(focusLabel))

	// Error message (High priority)
	if m.lastError != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")).Bold(true)
		items = append(items, errorStyle.Render(fmt.Sprintf("ERROR: %v", m.lastError)))
	}

	// Server status
	if m.serverCount > 0 {
		items = append(items, m.styles.StatusItem.Render(
			fmt.Sprintf("%d servers", m.serverCount),
		))
	}

	// Tool count
	if m.toolCount > 0 {
		items = append(items, m.styles.StatusItem.Render(
			fmt.Sprintf("%d tools", m.toolCount),
		))
	}

	// Token stats (show if any tokens have been used)
	if m.inputTokens > 0 || m.outputTokens > 0 {
		tokenInfo := fmt.Sprintf("In:%d Out:%d Ctx:%d", m.inputTokens, m.outputTokens, m.totalContextTokens)
		if m.avgTokensPerSecond > 0 {
			tokenInfo += fmt.Sprintf(" %.1f tok/s", m.avgTokensPerSecond)
		}
		items = append(items, m.styles.StatusItem.Render(tokenInfo))
	}

	// Processing indicator
	if m.isProcessing {
		items = append(items, m.styles.StatusItem.Render(m.spinner.View()+" Processing..."))
	}

	// Help hint
	items = append(items, m.styles.Help.Render("Tab: panel · F1: files · F2: code · F3: diff · Ctrl+Q: quit"))

	return m.styles.StatusBar.Width(m.width).Render(
		strings.Join(items, " │ "),
	)
}

// renderHelp renders the help screen
func (m Model) renderHelp() string {
	helpContent := `
# Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Enter | Send message |
| Tab | Switch panel focus |
| Shift+Tab | Switch panel focus (reverse) |
| F1 | Toggle file tree sidebar |
| F2 | Toggle code panel |
| F3 | Toggle git diff view |
| Ctrl+C | Cancel current tool / Quit |
| Ctrl+Q | Quit |
| Ctrl+L | Clear chat history |
| PgUp/PgDown | Scroll chat |

# Panel Navigation

| Panel | Keys |
|-------|-------|
| File Tree | j/k: nav · Enter: select · h/l: collapse/expand |
| Chat | Type and Enter to send |
| Code Panel | j/k: nav · g/G: top/bottom · PgUp/PgDn: page |
| Diff View | h/l: switch panels · j/k: scroll · n/N: hunks |

# Commands

| Command | Description |
|---------|-------------|
| /help | Show this help |
| /clear | Clear chat history |
| /index | Index current codebase |
| /diff | Show git diff (unstaged) |
| /diff --staged | Show staged changes |
| @ | Open file browser |
| exit | Quit application |

Press Esc or Q to close this help.
`
	rendered, _ := m.renderer.Render(helpContent)

	return m.styles.Modal.
		Width(m.width - 10).
		Height(m.height - 4).
		Render(
			m.styles.ModalTitle.Render("📖 Help") + "\n" + rendered,
		)
}

// renderApproval renders the approval dialog for terminal commands
func (m Model) renderApproval() string {
	if m.pendingApproval == nil {
		return m.renderChat()
	}

	content := fmt.Sprintf(`
⚠️  Terminal Command Approval Required

Tool: %s
Command: %s

Press Y to approve, N to deny
`, m.pendingApproval.ToolName, m.pendingApproval.Command)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		m.styles.Modal.
			Width(60).
			BorderForeground(m.styles.Theme.Warning).
			Render(content),
	)
}

// renderFilePicker renders the file picker modal
func (m Model) renderFilePicker() string {
	if m.filePicker == nil {
		return m.renderChat()
	}

	m.filePicker.Width = m.width - 20
	m.filePicker.Height = m.height - 10

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		m.filePicker.View(),
	)
}

// renderDiff renders the git diff view
func (m Model) renderDiff() string {
	if m.diffPanel == nil {
		return m.renderChat()
	}

	m.diffPanel.Width = m.width - 6
	m.diffPanel.Height = m.height - 4
	m.diffPanel.Styles = m.styles

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		m.styles.DiffPanel.Render(m.diffPanel.View()),
	)
}

// renderSettings renders the settings panel
func (m Model) renderSettings() string {
	var content strings.Builder

	// Title
	content.WriteString(m.styles.ModalTitle.Render("⚙️ Settings"))
	content.WriteString("\n\n")

	// Accent Color heading
	content.WriteString(m.styles.FilePickerSelected.Render("▸ Accent Color: " + m.accentColor.Name()))
	content.WriteString("\n\n")

	// Color preview boxes - all 10 colors
	colors := []AccentColor{
		AccentBlue, AccentPurple, AccentGreen, AccentOrange, AccentPink,
		AccentCyan, AccentRed, AccentYellow, AccentTeal, AccentIndigo,
	}
	var colorBoxes []string
	for _, c := range colors {
		box := "●"
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
		if c == m.accentColor {
			box = "◉"
			style = style.Bold(true)
		}
		colorBoxes = append(colorBoxes, style.Render(box))
	}
	content.WriteString("  " + strings.Join(colorBoxes, " "))
	content.WriteString("\n\n")

	// Model info
	if m.modelName != "" {
		content.WriteString(m.styles.Help.Render("Model: " + m.modelName))
		content.WriteString("\n")
	}

	// Help hints
	content.WriteString("\n")
	content.WriteString(m.styles.Help.Render("←→ or ↑↓ change color · Esc close"))

	return m.styles.Modal.
		Width(m.width - 10).
		Height(m.height - 4).
		Render(content.String())
}

// renderWelcome renders the welcome screen with ASCII branding
func (m Model) renderWelcome() string {
	// ASCII art logo
	logo := `
   ___  ____  _____ _   _        _____ ____  ____  _____ ____  
  / _ \|  _ \| ____| \ | |      / ____/ __ \|  _ \| ____|  _ \ 
 | | | | |_) |  _| |  \| |_____| |   | |  | | | | |  _| | |_) |
 | |_| |  __/| |___| |\  |_____| |___| |__| | |_| | |___|  _ < 
  \___/|_|   |_____|_| \_|      \____|\____/|____/|_____|_| \_\
                                                               
`
	// Get current working directory
	wd, _ := os.Getwd()

	// Build welcome content
	var content strings.Builder

	// Logo
	logoStyle := m.styles.HeaderTitle.Align(lipgloss.Center)
	content.WriteString(logoStyle.Render(logo))
	content.WriteString("\n\n")

	// Directory info
	dirInfo := m.styles.HeaderInfo.Align(lipgloss.Center).Render("📂 " + wd)
	content.WriteString(dirInfo)
	content.WriteString("\n\n")

	// Subtitle
	subtitle := m.styles.Help.Align(lipgloss.Center).Render("AI-Powered Terminal Code Editor")
	content.WriteString(subtitle)
	content.WriteString("\n\n")

	// Model info
	if m.modelName != "" {
		modelInfo := m.styles.StatusValue.Align(lipgloss.Center).Render(m.modelName)
		content.WriteString(modelInfo)
		content.WriteString("\n")
	}

	// Server/tool info
	if m.serverCount > 0 || m.toolCount > 0 {
		serverInfo := m.styles.Help.Align(lipgloss.Center).Render(
			fmt.Sprintf("⚡ %d servers · 🔧 %d tools", m.serverCount, m.toolCount),
		)
		content.WriteString(serverInfo)
		content.WriteString("\n")
	}

	content.WriteString("\n")

	// Help hints
	hints := m.styles.Help.Align(lipgloss.Center).Render(
		"Type a message to start · /help for commands · @ to browse files",
	)
	content.WriteString(hints)

	// Center the content box
	contentBox := m.styles.Modal.
		Width(70).
		Align(lipgloss.Center).
		Render(content.String())

	// Command autocomplete dropdown for welcome screen
	var commandMenu string
	if m.showCommandMenu {
		commandMenu = m.renderCommandMenu()
	}

	// Input area
	inputBox := m.styles.InputContainer.
		Width(70).
		BorderForeground(m.styles.Theme.BorderFocused).
		Render(m.textarea.View())

	// Combine logo, command menu (if shown), and input
	var combined string
	if commandMenu != "" {
		combined = lipgloss.JoinVertical(
			lipgloss.Center,
			contentBox,
			"",
			commandMenu,
			inputBox,
		)
	} else {
		combined = lipgloss.JoinVertical(
			lipgloss.Center,
			contentBox,
			"",
			inputBox,
		)
	}

	// Center everything on screen
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		combined,
	)
}

// renderHistory renders the conversation history browser
func (m Model) renderHistory() string {
	var content strings.Builder

	// Title
	title := m.styles.HeaderTitle.Render("📜 Conversation History")
	content.WriteString(title)
	content.WriteString("\n\n")

	if len(m.conversationList) == 0 {
		content.WriteString(m.styles.Help.Render("No saved conversations yet.\n\n"))
		content.WriteString(m.styles.Help.Render("Start chatting and your conversations will be auto-saved.\n"))
	} else {
		// Header
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(m.styles.Theme.Subtle)
		content.WriteString(headerStyle.Render(fmt.Sprintf("  %-10s %-35s %-15s %s\n", "ID", "Title", "Updated", "Messages")))
		content.WriteString(strings.Repeat("─", 70) + "\n")

		// List conversations
		for i, conv := range m.conversationList {
			prefix := "  "
			style := lipgloss.NewStyle().Foreground(m.styles.Theme.Foreground)

			if i == m.conversationCursor {
				prefix = "▸ "
				style = style.Bold(true).Foreground(m.styles.Theme.Primary)
			}

			// Truncate title if too long
			title := conv.Title
			if len(title) > 32 {
				title = title[:32] + "..."
			}

			line := fmt.Sprintf("%s%-10s %-35s %-15s %d msgs",
				prefix,
				conv.ID,
				title,
				conv.UpdatedAt,
				conv.MessageCount,
			)
			content.WriteString(style.Render(line))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(m.styles.Help.Render("↑↓/jk navigate · Enter load · d delete · q/Esc close"))

	// Create a modal box
	modalContent := m.styles.Modal.
		Width(75).
		Height(m.height - 10).
		Render(content.String())

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modalContent,
	)
}

// renderPreview renders the inline diff preview panel
func (m Model) renderPreview() string {
	if m.previewPanel == nil {
		return "No preview available"
	}

	// Update preview panel dimensions
	m.previewPanel.Width = m.width - 10
	m.previewPanel.Height = m.height - 6

	// Create modal box for preview
	modalContent := m.styles.Modal.
		Width(m.width - 8).
		Height(m.height - 4).
		Render(m.previewPanel.View())

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		modalContent,
	)
}

// renderAgenticTask renders the agentic task panel view
func (m Model) renderAgenticTask() string {
	if m.taskPanel == nil {
		return "No task panel available"
	}

	// Update task panel dimensions
	m.taskPanel.SetSize(m.width-4, m.height-4)

	// Render task panel centered
	taskContent := m.taskPanel.View()

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		taskContent,
	)
}

// renderAgenticProposal renders the task proposal for approval
func (m Model) renderAgenticProposal() string {
	if m.proposalPanel == nil {
		return "No proposal available"
	}

	// Update proposal panel dimensions
	m.proposalPanel.SetSize(m.width-8, m.height-4)

	// Render proposal panel centered
	proposalContent := m.proposalPanel.View()

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		proposalContent,
	)
}

// renderAgenticTaskList renders the task history list
func (m Model) renderAgenticTaskList() string {
	if m.taskListPanel == nil {
		return "No task list available"
	}

	// Update task list panel dimensions
	m.taskListPanel.SetSize(m.width-8, m.height-4)

	// Render task list panel centered
	listContent := m.taskListPanel.View()

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		listContent,
	)
}

// requestDefinitionCmd requests definition from LSP
func (m Model) requestDefinitionCmd(file string, line, col int) tea.Cmd {
	return func() tea.Msg {
		if m.lspClient == nil {
			return DefinitionMsg{Error: fmt.Errorf("LSP not available")}
		}
		loc, err := m.lspClient.GoToDefinition(file, line, col)
		if err != nil {
			return DefinitionMsg{Error: err}
		}
		if loc == nil {
			return DefinitionMsg{Error: fmt.Errorf("definition not found")}
		}
		return DefinitionMsg{
			FilePath: lsp.URIToFilePath(loc.URI),
			Line:     loc.Range.Start.Line,
			Col:      loc.Range.Start.Character,
		}
	}
}

// requestHoverCmd requests hover info from LSP
func (m Model) requestHoverCmd(file string, line, col int) tea.Cmd {
	return func() tea.Msg {
		if m.lspClient == nil {
			return HoverMsg{Error: fmt.Errorf("LSP not available")}
		}
		content, err := m.lspClient.GetHoverInfo(file, line, col)
		if err != nil {
			return HoverMsg{Error: err}
		}
		return HoverMsg{Content: content}
	}
}

// requestDiagnosticsCmd requests diagnostics from LSP
func (m Model) requestDiagnosticsCmd(file string) tea.Cmd {
	return func() tea.Msg {
		if m.lspClient == nil {
			return nil
		}
		// Notify LSP of file open
		content, err := os.ReadFile(file)
		if err == nil {
			m.lspClient.OpenGoFile(file, string(content))
		}

		// Return cached diagnostics if any
		diags := m.lspClient.GetFileDiagnostics(file)
		return DiagnosticMsg{FilePath: file, Diagnostics: diags}
	}
}

// Helper functions for glamour style configuration
func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }
func boolPtr(b bool) *bool       { return &b }
