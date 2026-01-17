package conversations

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Message represents a single message in a conversation
type Message struct {
	Role      string    `json:"role"` // "user", "assistant", "system", "tool"
	Content   string    `json:"content"`
	ToolName  string    `json:"tool_name,omitempty"`
	ToolID    string    `json:"tool_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Conversation represents a saved conversation session
type Conversation struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Messages   []Message `json:"messages"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	WorkingDir string    `json:"working_dir"`
	Model      string    `json:"model"`
	TokensUsed int       `json:"tokens_used"`
}

// ConversationSummary is a lightweight version for listing
type ConversationSummary struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
	Preview      string    `json:"preview"` // First user message preview
}

// Manager handles conversation persistence
type Manager struct {
	storageDir string
	rootDir    string
	current    *Conversation
}

// NewManager creates a new conversation manager
func NewManager(rootDir string) (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storageDir := filepath.Join(homeDir, ".open-coder", "conversations")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create conversations directory: %w", err)
	}

	// Normalize rootDir
	absRoot, err := filepath.Abs(rootDir)
	if err == nil {
		rootDir = absRoot
	}

	return &Manager{
		storageDir: storageDir,
		rootDir:    rootDir,
	}, nil
}

// NewConversation creates a new conversation
func (m *Manager) NewConversation(model, workingDir string) *Conversation {
	conv := &Conversation{
		ID:         uuid.New().String()[:8], // Short ID for easy reference
		Title:      "New Conversation",
		Messages:   make([]Message, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		WorkingDir: workingDir,
		Model:      model,
	}
	m.current = conv
	return conv
}

// GetCurrent returns the current conversation
func (m *Manager) GetCurrent() *Conversation {
	return m.current
}

// SetCurrent sets the current conversation
func (m *Manager) SetCurrent(conv *Conversation) {
	m.current = conv
}

// AddMessage adds a message to the current conversation
func (m *Manager) AddMessage(role, content, toolName, toolID string) {
	if m.current == nil {
		return
	}

	msg := Message{
		Role:      role,
		Content:   content,
		ToolName:  toolName,
		ToolID:    toolID,
		Timestamp: time.Now(),
	}

	m.current.Messages = append(m.current.Messages, msg)
	m.current.UpdatedAt = time.Now()

	// Auto-generate title from first user message
	if m.current.Title == "New Conversation" && role == "user" && content != "" {
		m.current.Title = generateTitle(content)
	}
}

// generateTitle creates a short title from the first user message
func generateTitle(content string) string {
	// Take first 50 characters, trim to last word boundary
	title := content
	if len(title) > 50 {
		title = title[:50]
		if idx := strings.LastIndex(title, " "); idx > 20 {
			title = title[:idx]
		}
		title += "..."
	}
	// Remove newlines
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.TrimSpace(title)
	return title
}

// Save saves the current conversation to disk
func (m *Manager) Save() error {
	if m.current == nil {
		return fmt.Errorf("no active conversation to save")
	}

	return m.SaveConversation(m.current)
}

// SaveConversation saves a specific conversation to disk
func (m *Manager) SaveConversation(conv *Conversation) error {
	if conv == nil {
		return fmt.Errorf("conversation is nil")
	}

	filePath := filepath.Join(m.storageDir, conv.ID+".json")
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal conversation: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write conversation file: %w", err)
	}

	return nil
}

// Load loads a conversation by ID
func (m *Manager) Load(id string) (*Conversation, error) {
	filePath := filepath.Join(m.storageDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read conversation file: %w", err)
	}

	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("failed to parse conversation: %w", err)
	}

	m.current = &conv
	return &conv, nil
}

// List returns summaries of all saved conversations
func (m *Manager) List() ([]ConversationSummary, error) {
	entries, err := os.ReadDir(m.storageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []ConversationSummary{}, nil
		}
		return nil, fmt.Errorf("failed to read conversations directory: %w", err)
	}

	summaries := make([]ConversationSummary, 0)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		filePath := filepath.Join(m.storageDir, entry.Name())

		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var conv Conversation
		if err := json.Unmarshal(data, &conv); err != nil {
			continue
		}

		// Filter by working directory
		// We want to verify if the conversation belongs to the current project
		// Use filepath.Clean/Abs to be safe, but exact match is usually sufficient for project root
		// if Normalized paths match
		convDir, err1 := filepath.Abs(conv.WorkingDir)
		rootDir, err2 := filepath.Abs(m.rootDir)
		if err1 == nil && err2 == nil && convDir != rootDir {
			continue
		}

		// Get preview from first user message
		preview := ""
		for _, msg := range conv.Messages {
			if msg.Role == "user" && msg.Content != "" {
				preview = msg.Content
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				break
			}
		}

		summaries = append(summaries, ConversationSummary{
			ID:           id,
			Title:        conv.Title,
			CreatedAt:    conv.CreatedAt,
			UpdatedAt:    conv.UpdatedAt,
			MessageCount: len(conv.Messages),
			Preview:      preview,
		})
	}

	// Sort by updated time, most recent first
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].UpdatedAt.After(summaries[j].UpdatedAt)
	})

	return summaries, nil
}

// Delete removes a conversation by ID
func (m *Manager) Delete(id string) error {
	filePath := filepath.Join(m.storageDir, id+".json")
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	// If we deleted the current conversation, clear it
	if m.current != nil && m.current.ID == id {
		m.current = nil
	}

	return nil
}

// GetStorageDir returns the storage directory path
func (m *Manager) GetStorageDir() string {
	return m.storageDir
}

// UpdateTokens updates the token count for the current conversation
func (m *Manager) UpdateTokens(tokens int) {
	if m.current != nil {
		m.current.TokensUsed = tokens
	}
}
