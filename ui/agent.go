package ui

import (
	"context"
	"encoding/json"
	"fmt"
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

	return &Agent{
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
	}
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

		a.messages = append(a.messages, openai.UserMessage(content))
		return a.processConversation()
	}
}

// processConversation handles the conversation loop with streaming
func (a *Agent) processConversation() tea.Msg {
	responseStart := time.Now()
	var currentOutputTokens int

	for {
		// Create streaming request
		stream := a.openaiClient.Chat.Completions.NewStreaming(a.ctx, openai.ChatCompletionNewParams{
			Messages:          a.messages,
			Model:             openai.ChatModel(a.model),
			Tools:             a.tools,
			ParallelToolCalls: openai.Bool(false),
		})

		acc := openai.ChatCompletionAccumulator{}

		// Stream the response
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
			return StreamErrorMsg{Err: err}
		}

		// Update token stats after streaming completes
		responseDuration := time.Since(responseStart)
		a.mu.Lock()
		a.outputTokens += currentOutputTokens
		a.totalResponseTime += responseDuration
		a.responseCount++
		a.mu.Unlock()

		// Send token stats update to UI
		if a.program != nil {
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
			a.messages = append(a.messages, acc.Choices[0].Message.ToParam())

			// Execute tool calls
			for _, tc := range acc.Choices[0].Message.ToolCalls {
				if tc.Function.Name != "" && tc.ID != "" {
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
					} else {
						resultStr = fmt.Sprintf("%v", result)
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
		}

		// Signal completion
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
	for _, server := range a.servers {
		params := &mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		}

		res, err := server.Session.CallTool(ctx, params)
		if err == nil {
			if len(res.Content) > 0 {
				return res.Content[0], nil
			}
			return "Tool executed successfully", nil
		}
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
