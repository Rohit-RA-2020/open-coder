package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
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
}

// New creates a new Model with the given agent backend
func New(agent AgentInterface) Model {
	// Initialize textarea for input
	ta := textarea.New()
	ta.Placeholder = "Type your message... (@ to browse files, /help for commands)"
	ta.Focus()
	ta.Prompt = ""
	ta.CharLimit = 0 // No limit
	ta.SetWidth(80)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // Enter sends, not newline

	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#58a6ff"))

	// Create markdown renderer
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	// Initialize styles
	styles := NewStyles(DarkTheme)

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
	)
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

	case tea.KeyEnter:
		if m.view == ViewChat && !m.isProcessing && m.panelFocus == PanelChat {
			content := strings.TrimSpace(m.textarea.Value())
			if content != "" {
				m.textarea.Reset()
				newView, cmd := m.handleCommand(content)
				m.view = newView
				return m, cmd
			}
		}

	case tea.KeyCtrlT:
		// Toggle theme
		if m.isDarkTheme {
			m.styles = NewStyles(LightTheme)
			m.isDarkTheme = false
		} else {
			m.styles = NewStyles(DarkTheme)
			m.isDarkTheme = true
		}
		return m, nil

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
			newPanel, cmd := m.codePanel.Update(msg)
			m.codePanel = newPanel
			return m, cmd
		}
	case PanelChat:
		// Handle textarea input
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		return m, cmd
	}

	// Default: handle textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
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

// handleCommand processes special commands
func (m Model) handleCommand(content string) (ViewState, tea.Cmd) {
	lower := strings.ToLower(content)

	switch {
	case lower == "/help" || lower == "help":
		return ViewHelp, nil

	case lower == "/settings" || lower == "settings":
		return ViewSettings, nil

	case lower == "/clear" || lower == "/new":
		m.messages = []ChatMessage{}
		m.hasUserInteracted = false // Show welcome screen again
		if m.agent != nil {
			m.agent.ClearConversation()
		}
		m.updateViewport()
		return ViewChat, nil

	case lower == "/theme" || lower == "/toggle-theme":
		if m.isDarkTheme {
			return ViewChat, func() tea.Msg { return ThemeChangedMsg{Theme: LightTheme} }
		}
		return ViewChat, func() tea.Msg { return ThemeChangedMsg{Theme: DarkTheme} }

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

	case strings.HasPrefix(content, "@"):
		return ViewFilePicker, nil

	default:
		return ViewChat, SendMessageCmd(content)
	}
}

// handleWindowSize updates layout when terminal is resized
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	headerHeight := 2
	statusHeight := 1
	inputHeight := 5
	chatHeight := m.height - headerHeight - statusHeight - inputHeight

	if !m.ready {
		m.viewport = viewport.New(m.width, chatHeight)
		m.viewport.YPosition = headerHeight
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

	for _, msg := range m.messages {
		content.WriteString(m.renderMessage(msg))
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
	var label, content string

	// Calculate max width for content (leave room for label)
	maxWidth := m.width - 20
	if maxWidth < 40 {
		maxWidth = 40
	}

	switch msg.Role {
	case RoleUser:
		label = m.styles.MessageLabel.Foreground(m.styles.Theme.UserMessage).Render("You ▸")
		content = m.styles.MessageUser.Width(maxWidth).Render(msg.Content)

	case RoleAssistant:
		label = m.styles.MessageLabel.Foreground(m.styles.Theme.AssistantMessage).Render("Assistant ▸")
		// Try to render as markdown
		rendered, err := m.renderer.Render(msg.Content)
		if err != nil {
			content = m.styles.MessageAssist.Width(maxWidth).Render(msg.Content)
		} else {
			content = strings.TrimSpace(rendered)
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

	return fmt.Sprintf("%s %s", label, content)
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

	// Calculate layout dimensions
	sidebarWidth := 30
	if !m.showSidebar {
		sidebarWidth = 0
	}

	codePanelWidth := 40
	if !m.showCodePanel {
		codePanelWidth = 0
	}

	chatWidth := m.width - sidebarWidth - codePanelWidth - 4
	if chatWidth < 40 {
		chatWidth = 40
		codePanelWidth = m.width - sidebarWidth - 44
		if codePanelWidth < 30 {
			codePanelWidth = 0
		}
	}

	// Build sidebar (file tree)
	var sidebar string
	if m.showSidebar && m.fileTree != nil {
		m.fileTree.Width = sidebarWidth - 2
		m.fileTree.Height = m.height - headerHeight() - inputHeight() - 4
		sidebarStyle := m.styles.SidebarPanel
		if m.panelFocus == PanelFileTree {
			sidebarStyle = sidebarStyle.BorderForeground(m.styles.Theme.BorderFocused)
		}
		sidebar = sidebarStyle.Width(sidebarWidth).MaxHeight(m.height - headerHeight() - inputHeight() - 2).Render(
			m.styles.SidebarTitle.Render("📁 Files") + "\n" + m.fileTree.View(),
		)
	}

	// Build code panel
	var codePanelStr string
	if m.showCodePanel && m.codePanel != nil {
		m.codePanel.Width = codePanelWidth - 2
		m.codePanel.Height = m.height - headerHeight() - inputHeight() - 4
		codePanelStyle := m.styles.CodePanel
		if m.panelFocus == PanelCode {
			codePanelStyle = codePanelStyle.BorderForeground(m.styles.Theme.BorderFocused)
		}
		codePanelStr = codePanelStyle.Width(codePanelWidth).MaxHeight(m.height - headerHeight() - inputHeight() - 2).Render(m.codePanel.View())
	}

	// Chat viewport
	m.viewport.Width = chatWidth
	m.viewport.Height = m.height - headerHeight() - inputHeight() - 4
	chatViewStyle := m.styles.PanelBorder
	if m.panelFocus == PanelChat {
		chatViewStyle = m.styles.PanelBorderFocus
	}
	chatView := chatViewStyle.Width(chatWidth).MaxHeight(m.height - headerHeight() - inputHeight() - 2).Render(m.viewport.View())

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

	// Input area - highlight when chat is focused
	inputStyle := m.styles.InputContainer
	if m.panelFocus == PanelChat {
		inputStyle = inputStyle.BorderForeground(m.styles.Theme.BorderFocused)
	}
	inputBox := inputStyle.
		Width(m.width - 4).
		Render(m.styles.InputPrompt.Render("▸ ") + m.textarea.View())

	// Status bar
	statusBar := m.renderStatusBar()

	// Combine all elements
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

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	title := m.styles.HeaderTitle.Render("🤖 Open Coder")

	var info string
	if m.modelName != "" {
		info = m.styles.HeaderInfo.Render(fmt.Sprintf(" · %s", m.modelName))
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, info)

	themeIcon := "🌙"
	if !m.isDarkTheme {
		themeIcon = "☀️"
	}
	right := m.styles.HeaderInfo.Render(themeIcon)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if gap < 0 {
		gap = 0
	}

	return m.styles.Header.Width(m.width).Render(
		left + strings.Repeat(" ", gap) + right,
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
	items = append(items, m.styles.StatusItem.Render(
		fmt.Sprintf("📌 %s", focusLabel),
	))

	// Server status
	if m.serverCount > 0 {
		items = append(items, m.styles.StatusItem.Render(
			fmt.Sprintf("⚡ %d servers", m.serverCount),
		))
	}

	// Tool count
	if m.toolCount > 0 {
		items = append(items, m.styles.StatusItem.Render(
			fmt.Sprintf("🔧 %d tools", m.toolCount),
		))
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
| Ctrl+T | Toggle dark/light theme |
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
| /theme | Toggle theme |
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
   ___  ____  ____  _  __     _____ ___  ____  _____ ____  
  / _ \|  _ \| ___|| |/ /    / ____/ _ \|  _ \| ____|  _ \ 
 | | | | |_) |  _| |   /____| |   | | | | | | |  _| | |_) |
 | |_| |  __/| |___| . \____| |___| |_| | |_| | |___|  _ < 
  \___/|_|   |_____|_|\_\    \____|\___/|____/|_____|_| \_\
                                                           
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
		modelInfo := m.styles.StatusValue.Align(lipgloss.Center).Render("🤖 " + m.modelName)
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

	// Input area
	inputBox := m.styles.InputContainer.
		Width(70).
		BorderForeground(m.styles.Theme.BorderFocused).
		Render(m.styles.InputPrompt.Render("▸ ") + m.textarea.View())

	// Combine logo and input
	combined := lipgloss.JoinVertical(
		lipgloss.Center,
		contentBox,
		"",
		inputBox,
	)

	// Center everything on screen
	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		combined,
	)
}
