package agentic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TaskStorage handles persistence of agentic tasks
type TaskStorage struct {
	basePath string
}

// NewTaskStorage creates a new task storage instance
func NewTaskStorage(basePath string) *TaskStorage {
	// Create the storage directory if it doesn't exist
	tasksDir := filepath.Join(basePath, ".open-coder", "tasks")
	os.MkdirAll(tasksDir, 0755)

	return &TaskStorage{
		basePath: tasksDir,
	}
}

// SaveTask persists a task to disk
func (s *TaskStorage) SaveTask(task *Task) error {
	if task == nil || task.ID == "" {
		return fmt.Errorf("invalid task")
	}

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	taskPath := filepath.Join(s.basePath, task.ID+".json")
	if err := os.WriteFile(taskPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write task file: %w", err)
	}

	return nil
}

// LoadTask loads a task from disk by ID
func (s *TaskStorage) LoadTask(id string) (*Task, error) {
	taskPath := filepath.Join(s.basePath, id+".json")

	data, err := os.ReadFile(taskPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read task file: %w", err)
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task: %w", err)
	}

	return &task, nil
}

// ListTasks returns summaries of all stored tasks, sorted by creation time (newest first)
func (s *TaskStorage) ListTasks() ([]TaskSummary, error) {
	entries, err := os.ReadDir(s.basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []TaskSummary{}, nil
		}
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	var summaries []TaskSummary

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimSuffix(entry.Name(), ".json")
		task, err := s.LoadTask(id)
		if err != nil {
			continue // Skip invalid tasks
		}

		summaries = append(summaries, task.ToSummary())
	}

	// Sort by creation time, newest first
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})

	return summaries, nil
}

// DeleteTask removes a task from storage
func (s *TaskStorage) DeleteTask(id string) error {
	taskPath := filepath.Join(s.basePath, id+".json")

	if err := os.Remove(taskPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("task not found: %s", id)
		}
		return fmt.Errorf("failed to delete task: %w", err)
	}

	return nil
}

// GetTasksDir returns the tasks storage directory path
func (s *TaskStorage) GetTasksDir() string {
	return s.basePath
}
