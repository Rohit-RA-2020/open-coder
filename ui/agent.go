package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	tiktoken "github.com/pkoukk/tiktoken-go"

	"open-coder/pkg/agentic"
	"open-coder/pkg/conversations"
	"open-coder/pkg/history"
	"open-coder/pkg/indexer"
)

// Agent wraps the backend logic and provides the AgentInterface
type Agent struct {
	ctx          context.Context
	cancel       context.CancelFunc
	openaiClient *openai.Client
	model        string
	apiKey       string
	baseURL      string

	// MCP
	mcpClient *mcp.Client
	servers   []*MCPServer
	tools     []openai.ChatCompletionToolUnionParam

	// Conversation
	messages     []openai.ChatCompletionMessageParamUnion
	systemPrompt string

	// State
	mu                      sync.Mutex
	currentToolCancel       context.CancelFunc
	requireTerminalApproval bool

	// Program reference for sending messages
	program *tea.Program

	// Token tracking
	tokenEncoder      *tiktoken.Tiktoken
	inputTokens       int
	outputTokens      int
	totalResponseTime time.Duration
	responseCount     int

	// Conversation persistence
	conversationMgr *conversations.Manager

	// Undo/Redo history
	historyMgr *history.HistoryManager
}

// MCPServer represents a connected MCP server
type MCPServer struct {
	Name    string
	Command string
	Args    []string
	Session *mcp.ClientSession
}

// NewAgent creates a new Agent instance
func NewAgent(ctx context.Context, model, apiKey, baseURL string) *Agent {
	ctx, cancel := context.WithCancel(ctx)

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	// Initialize tiktoken encoder for token counting
	tke, err := tiktoken.EncodingForModel(model)
	if err != nil || tke == nil {
		// Fall back to cl100k_base for unknown models (covers GPT-4, Claude, etc.)
		tke, _ = tiktoken.GetEncoding("cl100k_base")
	}

	// Initialize conversation manager
	cwd, _ := os.Getwd()
	convMgr, _ := conversations.NewManager(cwd)

	// Initialize history manager for undo/redo
	histMgr, _ := history.NewHistoryManager()

	agent := &Agent{
		ctx:                     ctx,
		cancel:                  cancel,
		openaiClient:            &client,
		model:                   model,
		apiKey:                  apiKey,
		baseURL:                 baseURL,
		mcpClient:               mcp.NewClient(&mcp.Implementation{Name: "open-coder", Version: "v2.0.0"}, nil),
		servers:                 make([]*MCPServer, 0),
		tools:                   make([]openai.ChatCompletionToolUnionParam, 0),
		messages:                make([]openai.ChatCompletionMessageParamUnion, 0),
		requireTerminalApproval: true,
		tokenEncoder:            tke,
		conversationMgr:         convMgr,
		historyMgr:              histMgr,
	}

	// Create a new conversation
	if convMgr != nil {
		cwd, _ := os.Getwd()
		convMgr.NewConversation(model, cwd)
	}

	return agent
}

// SetProgram sets the tea.Program reference for sending messages
func (a *Agent) SetProgram(p *tea.Program) {
	a.program = p
}

// InitConversation initializes the conversation with a system prompt
func (a *Agent) InitConversation(systemPrompt string) {
	a.systemPrompt = systemPrompt
	a.messages = []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
	}
}

// ClearConversation resets the conversation to start fresh
func (a *Agent) ClearConversation() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages = []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(a.systemPrompt),
	}
	// Reset token stats
	a.inputTokens = 0
	a.outputTokens = 0
	a.totalResponseTime = 0
	a.responseCount = 0
}

// TruncateContext reduces conversation history to fit within token limits
// Keeps the system message, and the most recent messages up to maxTokens
func (a *Agent) TruncateContext(maxTokens int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.messages) <= 2 {
		return // Nothing to truncate
	}

	// Estimate current token usage
	totalTokens := 0
	for _, msg := range a.messages {
		// Rough estimate: 4 chars per token
		totalTokens += len(fmt.Sprintf("%v", msg)) / 4
	}

	if totalTokens <= maxTokens {
		return // No truncation needed
	}

	// Keep system message (first) and truncate from the beginning
	// Keep at least last 10 messages for context
	minKeep := 10
	if len(a.messages) <= minKeep+1 {
		return
	}

	// Remove older messages (after system prompt) until under limit
	for totalTokens > maxTokens && len(a.messages) > minKeep+1 {
		// Remove the second message (first after system)
		removedMsg := a.messages[1]
		a.messages = append(a.messages[:1], a.messages[2:]...)
		totalTokens -= len(fmt.Sprintf("%v", removedMsg)) / 4
	}
}

// AddMCPServer connects to an MCP server
func (a *Agent) AddMCPServer(name, command string, args []string) error {
	server := &MCPServer{
		Name:    name,
		Command: command,
		Args:    args,
	}

	transport := &mcp.CommandTransport{Command: exec.Command(command, args...)}
	session, err := a.mcpClient.Connect(a.ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server %s: %w", name, err)
	}

	server.Session = session
	a.servers = append(a.servers, server)
	return nil
}

// RefreshTools loads tools from all connected MCP servers
func (a *Agent) RefreshTools() error {
	var allTools []openai.ChatCompletionToolUnionParam

	for _, server := range a.servers {
		tools, err := a.buildOpenAIToolsFromMCP(server.Session)
		if err != nil {
			continue
		}
		allTools = append(allTools, tools...)
	}

	a.tools = allTools
	return nil
}

// buildOpenAIToolsFromMCP converts MCP tools to OpenAI format
func (a *Agent) buildOpenAIToolsFromMCP(session *mcp.ClientSession) ([]openai.ChatCompletionToolUnionParam, error) {
	res, err := session.ListTools(a.ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}

	out := make([]openai.ChatCompletionToolUnionParam, 0, len(res.Tools))
	for _, t := range res.Tools {
		var paramsObj map[string]any
		if t.InputSchema != nil {
			raw, err := json.Marshal(t.InputSchema)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(raw, &paramsObj); err != nil {
				continue
			}
		} else {
			paramsObj = map[string]any{"type": "object", "properties": map[string]any{}}
		}

		// Normalize schema
		if paramsObj == nil {
			paramsObj = map[string]any{}
		}
		if v, ok := paramsObj["type"]; !ok || v != "object" {
			paramsObj["type"] = "object"
		}
		if _, ok := paramsObj["properties"]; !ok {
			paramsObj["properties"] = map[string]any{}
		}

		// Filter out 'uid' parameter
		if props, ok := paramsObj["properties"].(map[string]any); ok {
			delete(props, "uid")
		}

		tool := openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        t.Name,
			Description: openai.String(t.Description),
			Parameters:  openai.FunctionParameters(paramsObj),
		})
		out = append(out, tool)
	}
	return out, nil
}

// GetModelName returns the model name
func (a *Agent) GetModelName() string {
	return a.model
}

// GetServerCount returns the number of connected servers
func (a *Agent) GetServerCount() int {
	return len(a.servers)
}

// GetToolCount returns the number of available tools
func (a *Agent) GetToolCount() int {
	return len(a.tools)
}

// countTokens counts tokens for a given text using tiktoken
func (a *Agent) countTokens(text string) int {
	if a.tokenEncoder == nil {
		return 0
	}
	return len(a.tokenEncoder.Encode(text, nil, nil))
}

// GetTokenStats returns current token statistics
func (a *Agent) GetTokenStats() (input, output, context int, avgTPS float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.responseCount > 0 && a.totalResponseTime.Seconds() > 0 {
		avgTPS = float64(a.outputTokens) / a.totalResponseTime.Seconds()
	}
	return a.inputTokens, a.outputTokens, a.inputTokens + a.outputTokens, avgTPS
}

// IsTerminalApprovalRequired returns whether terminal approval is required
func (a *Agent) IsTerminalApprovalRequired() bool {
	return a.requireTerminalApproval
}

// ToggleTerminalApproval toggles terminal approval requirement
func (a *Agent) ToggleTerminalApproval() {
	a.requireTerminalApproval = !a.requireTerminalApproval
}

// CancelCurrentTool cancels the currently running tool
func (a *Agent) CancelCurrentTool() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentToolCancel != nil {
		a.currentToolCancel()
		a.currentToolCancel = nil
	}
}

// ProcessMessage implements AgentInterface - processes user input and returns a command
func (a *Agent) ProcessMessage(content string) tea.Cmd {
	return func() tea.Msg {
		// Count input tokens
		tokens := a.countTokens(content)
		a.mu.Lock()
		a.inputTokens += tokens
		a.mu.Unlock()

		// Check if this should trigger agentic mode
		if a.shouldUseAgenticMode(content) {
			// Return a message to trigger agentic mode
			return AgenticModeStartMsg{Request: content}
		}

		// Track user message for persistence
		a.trackConversationMessage("user", content, "", "")

		a.messages = append(a.messages, openai.UserMessage(content))
		return a.processConversation()
	}
}

// shouldUseAgenticMode determines if a request should trigger agentic planning mode
// It detects task-like requests vs simple questions/conversations
func (a *Agent) shouldUseAgenticMode(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))

	// Skip short messages (likely simple questions)
	if len(lower) < 15 {
		return false
	}

	// Skip if it's clearly a question
	questionStarters := []string{"what ", "what's", "how ", "why ", "when ", "where ", "who ", "can you explain", "tell me about", "describe "}
	for _, q := range questionStarters {
		if strings.HasPrefix(lower, q) {
			return false
		}
	}

	// Skip if ends with question mark
	if strings.HasSuffix(strings.TrimSpace(content), "?") {
		return false
	}

	// Task-like action verbs that indicate implementation work
	taskVerbs := []string{
		"create ", "add ", "implement ", "build ", "make ",
		"write ", "develop ", "setup ", "set up ", "configure ",
		"refactor ", "fix ", "update ", "modify ", "change ",
		"remove ", "delete ", "migrate ", "convert ", "transform ",
		"integrate ", "connect ", "install ", "deploy ",
		"design ", "architect ", "restructure ", "optimize ",
	}

	for _, verb := range taskVerbs {
		if strings.Contains(lower, verb) {
			return true
		}
	}

	// Check for specific patterns that indicate a task
	taskPatterns := []string{
		"i want to ", "i need to ", "please ", "could you ",
		"let's ", "we need to ", "we should ",
	}

	for _, pattern := range taskPatterns {
		if strings.HasPrefix(lower, pattern) {
			// Check if followed by an action verb
			rest := strings.TrimPrefix(lower, pattern)
			for _, verb := range taskVerbs {
				if strings.HasPrefix(rest, strings.TrimSpace(verb)) {
					return true
				}
			}
		}
	}

	return false
}

// processConversation handles the conversation loop with streaming
func (a *Agent) processConversation() tea.Msg {
	// Auto-truncate context to prevent overflow (keep under ~100k tokens)
	a.TruncateContext(80000)

	responseStart := time.Now()
	var currentOutputTokens int

	log.Printf("Starting conversation processing with model: %s", a.model)

	for {
		// Create streaming request
		log.Printf("Creating streaming request with %d messages", len(a.messages))
		stream := a.openaiClient.Chat.Completions.NewStreaming(a.ctx, openai.ChatCompletionNewParams{
			Messages:          a.messages,
			Model:             openai.ChatModel(a.model),
			Tools:             a.tools,
			ParallelToolCalls: openai.Bool(false),
		})

		acc := openai.ChatCompletionAccumulator{}

		// Stream the response
		log.Println("Starting stream loop")
		for stream.Next() {
			current := stream.Current()
			acc.AddChunk(current)

			// Send content chunks to UI and count output tokens
			if len(current.Choices) > 0 && current.Choices[0].Delta.Content != "" {
				chunkContent := current.Choices[0].Delta.Content
				currentOutputTokens += a.countTokens(chunkContent)

				if a.program != nil {
					a.program.Send(StreamChunkMsg{
						Content: chunkContent,
						Done:    false,
					})
				}
			}
		}

		if err := stream.Err(); err != nil {
			log.Printf("Stream error: %v", err)
			return StreamErrorMsg{Err: err}
		}

		log.Println("Stream loop completed")

		// Update token stats after streaming completes
		responseDuration := time.Since(responseStart)
		a.mu.Lock()
		a.outputTokens += currentOutputTokens
		a.totalResponseTime += responseDuration
		a.responseCount++
		a.mu.Unlock()

		// Send token stats update to UI
		if a.program != nil {
			log.Printf("Sending token stats update: output=%d", currentOutputTokens)
			input, output, ctx, avgTPS := a.GetTokenStats()
			a.program.Send(TokenStatsUpdatedMsg{
				InputTokens:        input,
				OutputTokens:       output,
				TotalContextTokens: ctx,
				AvgTokensPerSecond: avgTPS,
			})
		}

		// Check for tool calls
		if len(acc.Choices) > 0 && len(acc.Choices[0].Message.ToolCalls) > 0 {
			log.Printf("Tool calls detected: %d", len(acc.Choices[0].Message.ToolCalls))
			a.messages = append(a.messages, acc.Choices[0].Message.ToParam())

			// Execute tool calls
			for _, tc := range acc.Choices[0].Message.ToolCalls {
				if tc.Function.Name != "" && tc.ID != "" {
					log.Printf("Executing tool: %s", tc.Function.Name)
					// Convert to our ToolCall type
					toolCall := ToolCall{
						ID:        tc.ID,
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					}
					result, err := a.executeTool(toolCall)

					var resultStr string
					if err != nil {
						resultStr = fmt.Sprintf("Error: %v", err)
						log.Printf("Tool execution error: %v", err)
					} else {
						resultStr = fmt.Sprintf("%v", result)
						log.Printf("Tool execution successful")
					}

					toolMessage := openai.ToolMessage(resultStr, tc.ID)
					a.messages = append(a.messages, toolMessage)
				}
			}

			// Reset for next iteration and continue the loop
			responseStart = time.Now()
			currentOutputTokens = 0
			continue
		}

		// No more tool calls - add final message and finish
		if len(acc.Choices) > 0 {
			a.messages = append(a.messages, acc.Choices[0].Message.ToParam())
			// Track assistant response for persistence
			a.trackConversationMessage("assistant", acc.Choices[0].Message.Content, "", "")
		}

		// Signal completion
		log.Println("Conversation turn complete")
		return StreamChunkMsg{Done: true}
	}
}

// ToolCall represents the data we need from a tool call
type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

// executeTool executes a single tool call
func (a *Agent) executeTool(toolCall ToolCall) (interface{}, error) {
	// Parse arguments
	var args map[string]any
	if err := json.Unmarshal([]byte(toolCall.Arguments), &args); err != nil {
		return nil, err
	}

	// Send tool start message
	if a.program != nil {
		a.program.Send(ToolCallStartMsg{
			ToolName: toolCall.Name,
			ToolID:   toolCall.ID,
			Args:     args,
		})
	}

	// Check if terminal approval is needed
	if a.requireTerminalApproval && a.isTerminalCommandTool(toolCall.Name) {
		// For now, we'll auto-approve in this version
		// A more complete implementation would use channels for approval
	}

	// Create cancellable context for this tool
	toolCtx, toolCancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.currentToolCancel = toolCancel
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.currentToolCancel = nil
		a.mu.Unlock()
	}()

	// Execute the tool
	result, err := a.callTool(toolCtx, toolCall.Name, args)

	// Send result message
	if a.program != nil {
		if toolCtx.Err() != nil {
			a.program.Send(ToolCallCancelledMsg{
				ToolName: toolCall.Name,
				ToolID:   toolCall.ID,
			})
			return nil, fmt.Errorf("cancelled")
		}

		a.program.Send(ToolCallResultMsg{
			ToolName: toolCall.Name,
			ToolID:   toolCall.ID,
			Result:   fmt.Sprintf("%v", result),
			Error:    err,
		})
	}

	return result, err
}

// callTool calls an MCP tool
func (a *Agent) callTool(ctx context.Context, toolName string, args map[string]any) (interface{}, error) {
	// Check if this is a file-modifying tool and record original content for undo
	var filePath string
	var originalContent string
	var isFileModifying bool

	switch toolName {
	case "write_file", "create_file", "edit_file", "patch_file", "replace_in_file":
		if path, ok := args["path"].(string); ok {
			filePath = path
			isFileModifying = true
			// Read original content before modification
			if content, err := os.ReadFile(path); err == nil {
				originalContent = string(content)
			}
			// Start transaction
			a.BeginFileTransaction(fmt.Sprintf("Tool: %s", toolName), toolName)
		}
	case "delete_file", "remove_file":
		if path, ok := args["path"].(string); ok {
			filePath = path
			isFileModifying = true
			// Read original content before deletion
			if content, err := os.ReadFile(path); err == nil {
				originalContent = string(content)
			}
			a.BeginFileTransaction(fmt.Sprintf("Tool: %s", toolName), toolName)
		}
	}

	for _, server := range a.servers {
		params := &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		}

		res, err := server.Session.CallTool(ctx, params)
		if err == nil {
			// Record file change after successful modification
			if isFileModifying && filePath != "" {
				var newContent string
				var operation string

				switch toolName {
				case "delete_file", "remove_file":
					operation = "delete"
					newContent = ""
				case "write_file", "create_file":
					if originalContent == "" {
						operation = "create"
					} else {
						operation = "modify"
					}
					// Read new content
					if content, err := os.ReadFile(filePath); err == nil {
						newContent = string(content)
					}
				default:
					operation = "modify"
					if content, err := os.ReadFile(filePath); err == nil {
						newContent = string(content)
					}
				}

				a.RecordFileChange(filePath, originalContent, newContent, operation)
				a.CommitFileTransaction()
			}

			if len(res.Content) > 0 {
				return res.Content[0], nil
			}
			return "Tool executed successfully", nil
		}
	}

	// Abort transaction on failure
	if isFileModifying && a.historyMgr != nil {
		a.historyMgr.AbortTransaction()
	}

	return nil, fmt.Errorf("tool %s not found in any connected server", toolName)
}

// isTerminalCommandTool checks if a tool is a terminal command
func (a *Agent) isTerminalCommandTool(toolName string) bool {
	switch toolName {
	case "run_command", "run_command_with_env", "run_command_in_dir":
		return true
	default:
		return false
	}
}

// IndexCodebase implements AgentInterface - starts codebase indexing
func (a *Agent) IndexCodebase() tea.Cmd {
	return func() tea.Msg {
		cwd, err := os.Getwd()
		if err != nil {
			return IndexingErrorMsg{Err: err}
		}

		// Check if already indexed
		if marker, indexed := indexer.IsDirectoryIndexed(cwd); indexed {
			return IndexingCompleteMsg{
				FilesProcessed:  marker.FilesProcessed,
				ChunksProcessed: marker.ChunksCreated,
				AlreadyIndexed:  true,
			}
		}

		if a.program != nil {
			a.program.Send(IndexingStartMsg{Directory: cwd})
		}

		// Load indexer config
		config := indexer.LoadConfigFromEnv()

		// Create indexer
		idx, err := indexer.NewIndexer(config)
		if err != nil {
			return IndexingErrorMsg{Err: fmt.Errorf("failed to create indexer: %w", err)}
		}

		// Progress callback
		onProgress := func(current, total int, filename string) {
			if a.program != nil {
				a.program.Send(IndexingProgressMsg{
					Current: current,
					Total:   total,
					File:    filename,
				})
			}
		}

		// Run indexing with progress
		filesProcessed, chunksProcessed, err := idx.IndexDirectoryWithProgress(a.ctx, cwd, onProgress)
		if err != nil {
			return IndexingErrorMsg{Err: err}
		}

		return IndexingCompleteMsg{
			FilesProcessed:  filesProcessed,
			ChunksProcessed: chunksProcessed,
		}
	}
}

// GenerateCommitMessage generates a commit message using AI based on the diff
func (a *Agent) GenerateCommitMessage(diffContent string, filesChanged, additions, deletions int) tea.Cmd {
	return func() tea.Msg {
		// Build a prompt for generating commit message
		prompt := fmt.Sprintf(`Based on the following git diff, generate a concise and descriptive commit message.
The commit message should follow conventional commit format (e.g., feat:, fix:, refactor:, docs:, etc.).
Keep the first line under 72 characters. Add a blank line and bullet points for details if there are multiple significant changes.

Statistics:
- Files changed: %d
- Lines added: %d
- Lines deleted: %d

Diff:
%s

Generate ONLY the commit message, nothing else.`, filesChanged, additions, deletions, diffContent)

		// Create a one-shot request (not using the conversation history)
		resp, err := a.openaiClient.Chat.Completions.New(a.ctx, openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage("You are a helpful assistant that generates clear, concise git commit messages following conventional commit format."),
				openai.UserMessage(prompt),
			},
			Model: openai.ChatModel(a.model),
		})

		if err != nil {
			return CommitMessageResultMsg{
				Message: "",
				Error:   err,
			}
		}

		if len(resp.Choices) == 0 {
			return CommitMessageResultMsg{
				Message: "",
				Error:   fmt.Errorf("no response from AI"),
			}
		}

		commitMessage := strings.TrimSpace(resp.Choices[0].Message.Content)
		return CommitMessageResultMsg{
			Message: commitMessage,
			Error:   nil,
		}
	}
}

// Close cleans up resources
func (a *Agent) Close() {
	a.cancel()
	for _, s := range a.servers {
		if s.Session != nil {
			_ = s.Session.Close()
		}
	}
}

// DiscoverAndConnectServers auto-discovers MCP servers in ~/.open-coder/
func (a *Agent) DiscoverAndConnectServers() (int, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, err
	}

	installDir := filepath.Join(homeDir, ".open-coder")
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return 0, err
	}

	connected := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-cli") {
			continue
		}

		serverName := strings.TrimSuffix(entry.Name(), "-cli")
		serverPath := filepath.Join(installDir, entry.Name())

		// Check if executable
		info, err := entry.Info()
		if err != nil || info.Mode()&0111 == 0 {
			continue
		}

		if err := a.AddMCPServer(serverName, serverPath, []string{}); err == nil {
			connected++
		}
	}

	return connected, nil
}

// SaveConversation saves the current conversation to disk
func (a *Agent) SaveConversation() error {
	if a.conversationMgr == nil {
		return fmt.Errorf("conversation manager not initialized")
	}
	return a.conversationMgr.Save()
}

// LoadConversation loads a conversation by ID and restores it
func (a *Agent) LoadConversation(id string) error {
	if a.conversationMgr == nil {
		return fmt.Errorf("conversation manager not initialized")
	}

	conv, err := a.conversationMgr.Load(id)
	if err != nil {
		return err
	}

	// Rebuild OpenAI messages from saved conversation
	a.mu.Lock()
	defer a.mu.Unlock()

	a.messages = []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(a.systemPrompt),
	}

	for _, msg := range conv.Messages {
		switch msg.Role {
		case "user":
			a.messages = append(a.messages, openai.UserMessage(msg.Content))
		case "assistant":
			a.messages = append(a.messages, openai.AssistantMessage(msg.Content))
		case "tool":
			a.messages = append(a.messages, openai.ToolMessage(msg.Content, msg.ToolID))
		}
	}

	return nil
}

// ListConversations returns a list of saved conversation summaries
func (a *Agent) ListConversations() ([]conversations.ConversationSummary, error) {
	if a.conversationMgr == nil {
		return nil, fmt.Errorf("conversation manager not initialized")
	}
	return a.conversationMgr.List()
}

// NewConversationSession starts a fresh conversation
func (a *Agent) NewConversationSession() {
	a.ClearConversation()
	if a.conversationMgr != nil {
		cwd, _ := os.Getwd()
		a.conversationMgr.NewConversation(a.model, cwd)
	}
}

// GetCurrentConversationID returns the current conversation ID
func (a *Agent) GetCurrentConversationID() string {
	if a.conversationMgr == nil || a.conversationMgr.GetCurrent() == nil {
		return ""
	}
	return a.conversationMgr.GetCurrent().ID
}

// GetCurrentConversationTitle returns the current conversation title
func (a *Agent) GetCurrentConversationTitle() string {
	if a.conversationMgr == nil || a.conversationMgr.GetCurrent() == nil {
		return ""
	}
	return a.conversationMgr.GetCurrent().Title
}

// trackConversationMessage tracks a message in the current conversation
func (a *Agent) trackConversationMessage(role, content, toolName, toolID string) {
	if a.conversationMgr != nil {
		a.conversationMgr.AddMessage(role, content, toolName, toolID)
		// Auto-save after each message
		_ = a.conversationMgr.Save()
	}
}

// DeleteConversation removes a saved conversation
func (a *Agent) DeleteConversation(id string) error {
	if a.conversationMgr == nil {
		return fmt.Errorf("conversation manager not initialized")
	}
	return a.conversationMgr.Delete(id)
}

// Undo reverts the last file change
func (a *Agent) Undo() tea.Cmd {
	return func() tea.Msg {
		if a.historyMgr == nil {
			return UndoResultMsg{Error: fmt.Errorf("history manager not initialized")}
		}

		tx, err := a.historyMgr.Undo()
		if err != nil {
			return UndoResultMsg{Error: err}
		}

		return UndoResultMsg{
			Transaction: tx,
			UndoCount:   a.historyMgr.GetUndoCount(),
			RedoCount:   a.historyMgr.GetRedoCount(),
		}
	}
}

// Redo re-applies the last undone change
func (a *Agent) Redo() tea.Cmd {
	return func() tea.Msg {
		if a.historyMgr == nil {
			return RedoResultMsg{Error: fmt.Errorf("history manager not initialized")}
		}

		tx, err := a.historyMgr.Redo()
		if err != nil {
			return RedoResultMsg{Error: err}
		}

		return RedoResultMsg{
			Transaction: tx,
			UndoCount:   a.historyMgr.GetUndoCount(),
			RedoCount:   a.historyMgr.GetRedoCount(),
		}
	}
}

// GetUndoCount returns number of available undo operations
func (a *Agent) GetUndoCount() int {
	if a.historyMgr == nil {
		return 0
	}
	return a.historyMgr.GetUndoCount()
}

// GetRedoCount returns number of available redo operations
func (a *Agent) GetRedoCount() int {
	if a.historyMgr == nil {
		return 0
	}
	return a.historyMgr.GetRedoCount()
}

// BeginFileTransaction starts tracking a file modification
func (a *Agent) BeginFileTransaction(description, toolName string) {
	if a.historyMgr != nil {
		a.historyMgr.BeginTransaction(description, toolName)
	}
}

// RecordFileChange records a file modification for undo/redo
func (a *Agent) RecordFileChange(path, originalContent, newContent, operation string) {
	if a.historyMgr != nil {
		_ = a.historyMgr.RecordChange(path, originalContent, newContent, operation)
	}
}

// CommitFileTransaction commits the current transaction
func (a *Agent) CommitFileTransaction() {
	if a.historyMgr != nil {
		a.historyMgr.CommitTransaction()
	}
}

// GetHistoryManager returns the history manager for direct access
func (a *Agent) GetHistoryManager() *history.HistoryManager {
	return a.historyMgr
}

// ============================================
// Agentic Mode
// ============================================

// agenticPlanner is the AI task planner
var agenticPlanner *agentic.Planner

// agenticExecutor is the task executor
var agenticExecutor *agentic.Executor

// agenticTask is the current agentic task
var agenticTask *agentic.Task

// taskStorage is for task persistence
var taskStorage *agentic.TaskStorage

// StartAgenticTask initiates agentic mode for a request - analyzes project first, then shows proposal
func (a *Agent) StartAgenticTask(request string) tea.Cmd {
	return func() tea.Msg {
		// Initialize planner if needed
		if agenticPlanner == nil {
			agenticPlanner = agentic.NewPlanner(a.openaiClient, a.model)
		}

		// Initialize storage if needed
		cwd, _ := os.Getwd()
		if taskStorage == nil {
			taskStorage = agentic.NewTaskStorage(cwd)
		}

		// Step 1: Send status update - Analyzing project
		if a.program != nil {
			a.program.Send(StreamChunkMsg{
				Content: "🔍 Analyzing project structure...\n",
				Done:    false,
			})
		}

		// Step 2: Analyze the project
		analysis, err := agenticPlanner.AnalyzeProject(a.ctx, cwd, request)
		if err != nil {
			// Continue with basic context if analysis fails
			analysis = &agentic.ProjectAnalysis{
				Summary:  fmt.Sprintf("Project at %s (analysis unavailable)", cwd),
				FileTree: "",
			}
		}

		// Step 3: Send status update - Creating plan
		if a.program != nil {
			a.program.Send(StreamChunkMsg{
				Content: "📋 Creating task plan based on project analysis...\n",
				Done:    false,
			})
		}

		// Step 4: Create task plan with analysis context
		codebaseContext := fmt.Sprintf(`Working directory: %s

Project Analysis:
%s

Project Structure:
%s`, cwd, analysis.Summary, analysis.FileTree)

		task, err := agenticPlanner.PlanTask(a.ctx, request, codebaseContext)
		if err != nil {
			return AgenticTaskErrorMsg{Error: err}
		}

		// Set status to awaiting approval
		task.Status = agentic.TaskAwaitingApproval
		agenticTask = task

		// Save the task
		_ = taskStorage.SaveTask(task)

		// Create proposal for review
		proposal := agenticPlanner.CreateProposal(task)

		// Step 5: Send status update - Ready for review
		if a.program != nil {
			a.program.Send(StreamChunkMsg{
				Content: "✅ Proposal ready for your review\n",
				Done:    true,
			})
		}

		return AgenticProposalReadyMsg{
			TaskID:   task.ID,
			Proposal: proposal,
		}
	}
}

// ExecuteApprovedTask runs the task after user approval
func (a *Agent) ExecuteApprovedTask() tea.Cmd {
	return func() tea.Msg {
		if agenticTask == nil {
			return AgenticTaskErrorMsg{Error: fmt.Errorf("no task to execute")}
		}

		// Initialize executor
		agenticExecutor = agentic.NewExecutor(agenticPlanner, a, func(update agentic.TaskUpdate) {
			if a.program != nil {
				a.program.Send(AgenticTaskUpdateMsg{
					TaskID:  update.TaskID,
					TodoID:  update.TodoID,
					Phase:   string(update.Phase),
					Status:  string(update.Status),
					Message: update.Message,
				})
			}
			// Save task progress
			if taskStorage != nil {
				_ = taskStorage.SaveTask(agenticTask)
			}
		})

		// Send task created message first
		if a.program != nil {
			a.program.Send(AgenticTaskCreatedMsg{
				TaskID:      agenticTask.ID,
				Title:       agenticTask.Title,
				Description: agenticTask.Description,
				TodoCount:   len(agenticTask.Todos),
			})
		}

		// Execute the task
		err := agenticExecutor.ExecuteTask(a.ctx, agenticTask)

		// Save final state
		if taskStorage != nil {
			_ = taskStorage.SaveTask(agenticTask)
		}

		if err != nil {
			return AgenticModeExitMsg{
				TaskID:    agenticTask.ID,
				Cancelled: agenticTask.Status == agentic.TaskCancelled,
			}
		}

		return AgenticModeExitMsg{
			TaskID:    agenticTask.ID,
			Completed: agenticTask.Status == agentic.TaskCompleted,
		}
	}
}

// ExecuteAgenticTask runs the planned task (legacy - for tasks created without proposal)
func (a *Agent) ExecuteAgenticTask() tea.Cmd {
	return a.ExecuteApprovedTask()
}

// LoadTaskList loads all tasks from storage
func (a *Agent) LoadTaskList() tea.Cmd {
	return func() tea.Msg {
		// Initialize storage if needed
		if taskStorage == nil {
			cwd, _ := os.Getwd()
			taskStorage = agentic.NewTaskStorage(cwd)
		}

		summaries, err := taskStorage.ListTasks()
		if err != nil {
			return AgenticTaskListLoadedMsg{Error: err}
		}

		// Convert to interface slice
		tasks := make([]interface{}, len(summaries))
		for i, s := range summaries {
			tasks[i] = s
		}

		return AgenticTaskListLoadedMsg{Tasks: tasks}
	}
}

// LoadAndShowTask loads a specific task and shows it
func (a *Agent) LoadAndShowTask(taskID string) tea.Cmd {
	return func() tea.Msg {
		if taskStorage == nil {
			cwd, _ := os.Getwd()
			taskStorage = agentic.NewTaskStorage(cwd)
		}

		task, err := taskStorage.LoadTask(taskID)
		if err != nil {
			return AgenticTaskErrorMsg{Error: err}
		}

		agenticTask = task

		return AgenticTaskCreatedMsg{
			TaskID:      task.ID,
			Title:       task.Title,
			Description: task.Description,
			TodoCount:   len(task.Todos),
		}
	}
}

// DeleteTask deletes a task from storage
func (a *Agent) DeleteTask(taskID string) tea.Cmd {
	return func() tea.Msg {
		if taskStorage == nil {
			cwd, _ := os.Getwd()
			taskStorage = agentic.NewTaskStorage(cwd)
		}

		err := taskStorage.DeleteTask(taskID)
		return AgenticTaskDeletedMsg{
			TaskID: taskID,
			Error:  err,
		}
	}
}

// SendTodoPrompt implements agentic.ExecutorInterface
func (a *Agent) SendTodoPrompt(ctx context.Context, prompt string) (string, error) {
	// Add the prompt as a user message and process
	a.messages = append(a.messages, openai.UserMessage(prompt))

	// Create streaming request
	stream := a.openaiClient.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Messages:          a.messages,
		Model:             openai.ChatModel(a.model),
		Tools:             a.tools,
		ParallelToolCalls: openai.Bool(false),
	})

	acc := openai.ChatCompletionAccumulator{}
	var result strings.Builder

	for stream.Next() {
		current := stream.Current()
		acc.AddChunk(current)

		if len(current.Choices) > 0 && current.Choices[0].Delta.Content != "" {
			result.WriteString(current.Choices[0].Delta.Content)
			if a.program != nil {
				a.program.Send(StreamChunkMsg{
					Content: current.Choices[0].Delta.Content,
					Done:    false,
				})
			}
		}
	}

	if err := stream.Err(); err != nil {
		return "", err
	}

	// Handle tool calls if any
	if len(acc.Choices) > 0 && len(acc.Choices[0].Message.ToolCalls) > 0 {
		a.messages = append(a.messages, acc.Choices[0].Message.ToParam())

		for _, tc := range acc.Choices[0].Message.ToolCalls {
			if tc.Function.Name != "" && tc.ID != "" {
				toolCall := ToolCall{
					ID:        tc.ID,
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				}
				toolResult, err := a.executeTool(toolCall)
				var resultStr string
				if err != nil {
					resultStr = fmt.Sprintf("Error: %v", err)
				} else {
					resultStr = fmt.Sprintf("%v", toolResult)
				}
				result.WriteString(fmt.Sprintf("\n[Tool: %s] %s", tc.Function.Name, resultStr))
				a.messages = append(a.messages, openai.ToolMessage(resultStr, tc.ID))
			}
		}
	}

	if len(acc.Choices) > 0 {
		a.messages = append(a.messages, acc.Choices[0].Message.ToParam())
	}

	return result.String(), nil
}

// ExecuteVerification implements agentic.ExecutorInterface
func (a *Agent) ExecuteVerification(ctx context.Context, command string) (string, error) {
	// Use the terminal tool to run the verification command
	args := map[string]any{"command": command}
	result, err := a.callTool(ctx, "run_terminal_cmd", args)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", result), nil
}

// PauseAgenticTask pauses execution
func (a *Agent) PauseAgenticTask() {
	if agenticExecutor != nil {
		agenticExecutor.Pause()
	}
}

// ResumeAgenticTask resumes execution
func (a *Agent) ResumeAgenticTask() {
	if agenticExecutor != nil {
		agenticExecutor.Resume()
	}
}

// CancelAgenticTask cancels the current task
func (a *Agent) CancelAgenticTask() {
	if agenticExecutor != nil {
		agenticExecutor.Cancel()
	}
}

// SkipAgenticTodo skips a todo item
func (a *Agent) SkipAgenticTodo(todoID string) bool {
	if agenticExecutor != nil {
		return agenticExecutor.SkipTodo(todoID)
	}
	return false
}

// GetAgenticTask returns the current agentic task
func (a *Agent) GetAgenticTask() *agentic.Task {
	return agenticTask
}

// IsAgenticModeActive returns whether agentic mode is running
func (a *Agent) IsAgenticModeActive() bool {
	return agenticExecutor != nil && agenticExecutor.IsRunning()
}

// IsAgenticModePaused returns whether agentic mode is paused
func (a *Agent) IsAgenticModePaused() bool {
	return agenticExecutor != nil && agenticExecutor.IsPaused()
}
