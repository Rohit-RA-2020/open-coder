package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// Client represents an LSP client connection
type Client struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	requestID int64
	pending   map[int64]chan *Response
	pendingMu sync.Mutex

	notifications chan *Notification
	diagnostics   map[string][]Diagnostic
	diagnosticsMu sync.RWMutex

	initialized bool
	rootPath    string
	ctx         context.Context
	cancel      context.CancelFunc
}

// Request represents an LSP request
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response represents an LSP response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// ResponseError represents an LSP error
type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Notification represents an LSP notification
type Notification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Diagnostic represents an LSP diagnostic
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// Range represents a text range
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position represents a text position
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Location represents a location in a file
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams identifies a position in a text document
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// PublishDiagnosticsParams is sent from server to client
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// NewClient creates a new LSP client
func NewClient(command string, args []string, rootPath string) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = rootPath
	cmd.Env = append(os.Environ(), "GOPRIVATE=*")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	client := &Client{
		cmd:           cmd,
		stdin:         stdin,
		stdout:        stdout,
		stderr:        stderr,
		pending:       make(map[int64]chan *Response),
		notifications: make(chan *Notification, 100),
		diagnostics:   make(map[string][]Diagnostic),
		rootPath:      rootPath,
		ctx:           ctx,
		cancel:        cancel,
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start LSP server: %w", err)
	}

	// Start reading responses
	go client.readResponses()
	go client.readStderr()

	return client, nil
}

// Initialize sends the initialize request to the LSP server
func (c *Client) Initialize() error {
	rootURI := "file://" + c.rootPath

	params := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   rootURI,
		"capabilities": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"hover": map[string]interface{}{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"definition": map[string]interface{}{
					"linkSupport": true,
				},
				"publishDiagnostics": map[string]interface{}{
					"relatedInformation": true,
				},
			},
		},
	}

	resp, err := c.Call("initialize", params)
	if err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize error: %s", resp.Error.Message)
	}

	// Send initialized notification
	c.Notify("initialized", struct{}{})
	c.initialized = true

	return nil
}

// Call sends a request and waits for response
func (c *Client) Call(method string, params interface{}) (*Response, error) {
	id := atomic.AddInt64(&c.requestID, 1)

	req := &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	respChan := make(chan *Response, 1)
	c.pendingMu.Lock()
	c.pending[id] = respChan
	c.pendingMu.Unlock()

	if err := c.send(req); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, err
	}

	select {
	case resp := <-respChan:
		return resp, nil
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// Notify sends a notification (no response expected)
func (c *Client) Notify(method string, params interface{}) error {
	req := &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	return c.send(req)
}

// send writes a request to the LSP server
func (c *Client) send(req *Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := c.stdin.Write(data); err != nil {
		return err
	}

	return nil
}

// readResponses reads responses from the LSP server
func (c *Client) readResponses() {
	reader := bufio.NewReader(c.stdout)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		// Read headers
		contentLength := 0
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(line, "Content-Length:") {
				length := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				contentLength, _ = strconv.Atoi(length)
			}
		}

		if contentLength == 0 {
			continue
		}

		// Read body
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return
		}

		// Try to parse as response or notification
		var baseMsg struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(body, &baseMsg); err != nil {
			continue
		}

		if baseMsg.ID != nil {
			// It's a response
			var resp Response
			if err := json.Unmarshal(body, &resp); err != nil {
				continue
			}

			c.pendingMu.Lock()
			if ch, ok := c.pending[resp.ID]; ok {
				ch <- &resp
				delete(c.pending, resp.ID)
			}
			c.pendingMu.Unlock()
		} else if baseMsg.Method != "" {
			// It's a notification
			c.handleNotification(baseMsg.Method, baseMsg.Params)
		}
	}
}

// handleNotification processes incoming notifications
func (c *Client) handleNotification(method string, params json.RawMessage) {
	switch method {
	case "textDocument/publishDiagnostics":
		var diagParams PublishDiagnosticsParams
		if err := json.Unmarshal(params, &diagParams); err == nil {
			c.diagnosticsMu.Lock()
			c.diagnostics[diagParams.URI] = diagParams.Diagnostics
			c.diagnosticsMu.Unlock()
		}
	}
}

// readStderr reads stderr from the LSP server
func (c *Client) readStderr() {
	reader := bufio.NewReader(c.stderr)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		// Log stderr for debugging if needed
		_ = line
	}
}

// DidOpen notifies the server about an opened file
func (c *Client) DidOpen(uri, languageID, text string) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": languageID,
			"version":    1,
			"text":       text,
		},
	}
	return c.Notify("textDocument/didOpen", params)
}

// DidClose notifies the server about a closed file
func (c *Client) DidClose(uri string) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": uri,
		},
	}
	return c.Notify("textDocument/didClose", params)
}

// DidChange notifies the server about file changes
func (c *Client) DidChange(uri, text string, version int) error {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     uri,
			"version": version,
		},
		"contentChanges": []map[string]interface{}{
			{"text": text},
		},
	}
	return c.Notify("textDocument/didChange", params)
}

// Definition requests go-to-definition
func (c *Client) Definition(uri string, line, character int) ([]Location, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	resp, err := c.Call("textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("definition error: %s", resp.Error.Message)
	}

	// Can be Location, []Location, or null
	var locations []Location
	if err := json.Unmarshal(resp.Result, &locations); err != nil {
		// Try single location
		var loc Location
		if err := json.Unmarshal(resp.Result, &loc); err == nil && loc.URI != "" {
			locations = []Location{loc}
		}
	}

	return locations, nil
}

// Hover requests hover information
func (c *Client) Hover(uri string, line, character int) (string, error) {
	params := TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: line, Character: character},
	}

	resp, err := c.Call("textDocument/hover", params)
	if err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("hover error: %s", resp.Error.Message)
	}

	var hover struct {
		Contents interface{} `json:"contents"`
	}
	if err := json.Unmarshal(resp.Result, &hover); err != nil {
		return "", err
	}

	// Contents can be string, MarkedString, []MarkedString, or MarkupContent
	switch v := hover.Contents.(type) {
	case string:
		return v, nil
	case map[string]interface{}:
		if value, ok := v["value"].(string); ok {
			return value, nil
		}
	}

	return "", nil
}

// GetDiagnostics returns cached diagnostics for a URI
func (c *Client) GetDiagnostics(uri string) []Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()
	return c.diagnostics[uri]
}

// GetAllDiagnostics returns all cached diagnostics
func (c *Client) GetAllDiagnostics() map[string][]Diagnostic {
	c.diagnosticsMu.RLock()
	defer c.diagnosticsMu.RUnlock()
	result := make(map[string][]Diagnostic)
	for k, v := range c.diagnostics {
		result[k] = v
	}
	return result
}

// Close shuts down the LSP client
func (c *Client) Close() error {
	if c.initialized {
		c.Call("shutdown", nil)
		c.Notify("exit", nil)
	}
	c.cancel()
	return c.cmd.Wait()
}

// FilePathToURI converts a file path to a URI
func FilePathToURI(path string) string {
	absPath, _ := filepath.Abs(path)
	return "file://" + absPath
}

// URIToFilePath converts a URI to a file path
func URIToFilePath(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

// DiagnosticSeverityString returns a human-readable severity
func DiagnosticSeverityString(severity int) string {
	switch severity {
	case 1:
		return "Error"
	case 2:
		return "Warning"
	case 3:
		return "Information"
	case 4:
		return "Hint"
	default:
		return "Unknown"
	}
}
