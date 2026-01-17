package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FileChange represents a single file modification
type FileChange struct {
	Path            string    `json:"path"`
	OriginalContent string    `json:"original_content"`
	NewContent      string    `json:"new_content"`
	Operation       string    `json:"operation"` // "create", "modify", "delete"
	Timestamp       time.Time `json:"timestamp"`
}

// EditTransaction represents a group of related file changes
type EditTransaction struct {
	ID          string       `json:"id"`
	Description string       `json:"description"`
	Changes     []FileChange `json:"changes"`
	Timestamp   time.Time    `json:"timestamp"`
	ToolName    string       `json:"tool_name,omitempty"`
	Applied     bool         `json:"applied"`
}

// HistoryManager manages the undo/redo stack
type HistoryManager struct {
	mu           sync.RWMutex
	undoStack    []EditTransaction
	redoStack    []EditTransaction
	maxStackSize int
	storageDir   string

	// Current transaction being built
	currentTx *EditTransaction
}

// NewHistoryManager creates a new history manager
func NewHistoryManager() (*HistoryManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storageDir := filepath.Join(homeDir, ".open-coder", "history")
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	mgr := &HistoryManager{
		undoStack:    make([]EditTransaction, 0),
		redoStack:    make([]EditTransaction, 0),
		maxStackSize: 100, // Keep last 100 transactions
		storageDir:   storageDir,
	}

	// Load existing history
	_ = mgr.load()

	return mgr, nil
}

// BeginTransaction starts a new edit transaction
func (h *HistoryManager) BeginTransaction(description, toolName string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.currentTx = &EditTransaction{
		ID:          uuid.New().String()[:8],
		Description: description,
		Changes:     make([]FileChange, 0),
		Timestamp:   time.Now(),
		ToolName:    toolName,
		Applied:     false,
	}
}

// RecordChange records a file change in the current transaction
func (h *HistoryManager) RecordChange(path, originalContent, newContent, operation string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.currentTx == nil {
		// Auto-create a transaction if none exists
		h.currentTx = &EditTransaction{
			ID:          uuid.New().String()[:8],
			Description: "File modification",
			Changes:     make([]FileChange, 0),
			Timestamp:   time.Now(),
			Applied:     false,
		}
	}

	change := FileChange{
		Path:            path,
		OriginalContent: originalContent,
		NewContent:      newContent,
		Operation:       operation,
		Timestamp:       time.Now(),
	}

	h.currentTx.Changes = append(h.currentTx.Changes, change)
	return nil
}

// CommitTransaction commits the current transaction to the undo stack
func (h *HistoryManager) CommitTransaction() *EditTransaction {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.currentTx == nil || len(h.currentTx.Changes) == 0 {
		h.currentTx = nil
		return nil
	}

	tx := h.currentTx
	tx.Applied = true
	h.currentTx = nil

	// Add to undo stack
	h.undoStack = append(h.undoStack, *tx)

	// Clear redo stack when new changes are made
	h.redoStack = make([]EditTransaction, 0)

	// Limit stack size
	if len(h.undoStack) > h.maxStackSize {
		h.undoStack = h.undoStack[1:]
	}

	// Persist to disk
	_ = h.save()

	return tx
}

// AbortTransaction discards the current transaction
func (h *HistoryManager) AbortTransaction() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.currentTx = nil
}

// Undo reverts the last transaction and returns it
func (h *HistoryManager) Undo() (*EditTransaction, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.undoStack) == 0 {
		return nil, fmt.Errorf("nothing to undo")
	}

	// Pop from undo stack
	tx := h.undoStack[len(h.undoStack)-1]
	h.undoStack = h.undoStack[:len(h.undoStack)-1]

	// Revert changes (in reverse order)
	for i := len(tx.Changes) - 1; i >= 0; i-- {
		change := tx.Changes[i]
		if err := h.revertChange(change); err != nil {
			return &tx, fmt.Errorf("failed to revert change to %s: %w", change.Path, err)
		}
	}

	// Push to redo stack
	h.redoStack = append(h.redoStack, tx)

	// Persist
	_ = h.save()

	return &tx, nil
}

// Redo re-applies the last undone transaction
func (h *HistoryManager) Redo() (*EditTransaction, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.redoStack) == 0 {
		return nil, fmt.Errorf("nothing to redo")
	}

	// Pop from redo stack
	tx := h.redoStack[len(h.redoStack)-1]
	h.redoStack = h.redoStack[:len(h.redoStack)-1]

	// Re-apply changes
	for _, change := range tx.Changes {
		if err := h.applyChange(change); err != nil {
			return &tx, fmt.Errorf("failed to re-apply change to %s: %w", change.Path, err)
		}
	}

	// Push back to undo stack
	h.undoStack = append(h.undoStack, tx)

	// Persist
	_ = h.save()

	return &tx, nil
}

// revertChange reverts a single file change
func (h *HistoryManager) revertChange(change FileChange) error {
	absPath, err := filepath.Abs(change.Path)
	if err != nil {
		return err
	}

	switch change.Operation {
	case "create":
		// File was created, delete it
		return os.Remove(absPath)

	case "delete":
		// File was deleted, restore it
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return os.WriteFile(absPath, []byte(change.OriginalContent), 0644)

	case "modify":
		// File was modified, restore original content
		return os.WriteFile(absPath, []byte(change.OriginalContent), 0644)

	default:
		return fmt.Errorf("unknown operation: %s", change.Operation)
	}
}

// applyChange applies a single file change
func (h *HistoryManager) applyChange(change FileChange) error {
	absPath, err := filepath.Abs(change.Path)
	if err != nil {
		return err
	}

	switch change.Operation {
	case "create", "modify":
		dir := filepath.Dir(absPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
		return os.WriteFile(absPath, []byte(change.NewContent), 0644)

	case "delete":
		return os.Remove(absPath)

	default:
		return fmt.Errorf("unknown operation: %s", change.Operation)
	}
}

// GetUndoCount returns the number of available undo operations
func (h *HistoryManager) GetUndoCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.undoStack)
}

// GetRedoCount returns the number of available redo operations
func (h *HistoryManager) GetRedoCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.redoStack)
}

// GetLastTransaction returns the last transaction without removing it
func (h *HistoryManager) GetLastTransaction() *EditTransaction {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.undoStack) == 0 {
		return nil
	}
	tx := h.undoStack[len(h.undoStack)-1]
	return &tx
}

// GetHistory returns a copy of the undo stack
func (h *HistoryManager) GetHistory() []EditTransaction {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]EditTransaction, len(h.undoStack))
	copy(result, h.undoStack)
	return result
}

// save persists the history to disk
func (h *HistoryManager) save() error {
	data := struct {
		UndoStack []EditTransaction `json:"undo_stack"`
		RedoStack []EditTransaction `json:"redo_stack"`
	}{
		UndoStack: h.undoStack,
		RedoStack: h.redoStack,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(h.storageDir, "history.json"), bytes, 0644)
}

// load restores history from disk
func (h *HistoryManager) load() error {
	data, err := os.ReadFile(filepath.Join(h.storageDir, "history.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No history yet
		}
		return err
	}

	var saved struct {
		UndoStack []EditTransaction `json:"undo_stack"`
		RedoStack []EditTransaction `json:"redo_stack"`
	}

	if err := json.Unmarshal(data, &saved); err != nil {
		return err
	}

	h.undoStack = saved.UndoStack
	h.redoStack = saved.RedoStack
	return nil
}

// Clear removes all history
func (h *HistoryManager) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.undoStack = make([]EditTransaction, 0)
	h.redoStack = make([]EditTransaction, 0)
	h.currentTx = nil

	_ = h.save()
}
