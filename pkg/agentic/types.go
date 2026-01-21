package agentic

import (
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the lifecycle of a task
type TaskStatus string

const (
	TaskPending          TaskStatus = "pending"
	TaskPlanning         TaskStatus = "planning"
	TaskAwaitingApproval TaskStatus = "awaiting_approval"
	TaskExecuting        TaskStatus = "executing"
	TaskVerifying        TaskStatus = "verifying"
	TaskCompleted        TaskStatus = "completed"
	TaskFailed           TaskStatus = "failed"
	TaskCancelled        TaskStatus = "cancelled"
	TaskPaused           TaskStatus = "paused"
)

// TaskPhase represents the current phase of task execution
type TaskPhase string

const (
	PhasePlanning     TaskPhase = "planning"
	PhaseExecution    TaskPhase = "execution"
	PhaseVerification TaskPhase = "verification"
	PhaseCompleted    TaskPhase = "completed"
)

// TodoStatus represents individual todo item state
type TodoStatus string

const (
	TodoPending    TodoStatus = "pending"
	TodoInProgress TodoStatus = "in_progress"
	TodoCompleted  TodoStatus = "completed"
	TodoFailed     TodoStatus = "failed"
	TodoSkipped    TodoStatus = "skipped"
)

// StatusIcon returns the emoji icon for a todo status
func (s TodoStatus) Icon() string {
	switch s {
	case TodoPending:
		return "⏳"
	case TodoInProgress:
		return "🔄"
	case TodoCompleted:
		return "✅"
	case TodoFailed:
		return "❌"
	case TodoSkipped:
		return "⏭️"
	default:
		return "○"
	}
}

// Todo represents a single task item
type Todo struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      TodoStatus `json:"status"`
	Order       int        `json:"order"`
	Phase       TaskPhase  `json:"phase"`
	Children    []Todo     `json:"children,omitempty"`
	StartedAt   time.Time  `json:"started_at,omitempty"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	Output      string     `json:"output,omitempty"`
}

// NewTodo creates a new todo item
func NewTodo(title string, phase TaskPhase, order int) *Todo {
	return &Todo{
		ID:     uuid.New().String()[:8],
		Title:  title,
		Status: TodoPending,
		Phase:  phase,
		Order:  order,
	}
}

// IsComplete returns true if the todo is in a terminal state
func (t *Todo) IsComplete() bool {
	return t.Status == TodoCompleted || t.Status == TodoFailed || t.Status == TodoSkipped
}

// VerificationStep represents a verification action
type VerificationStep struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Command  string     `json:"command,omitempty"`
	Status   TodoStatus `json:"status"`
	Result   string     `json:"result,omitempty"`
	IsManual bool       `json:"is_manual"`
	Passed   bool       `json:"passed"`
}

// NewVerificationStep creates a new verification step
func NewVerificationStep(title string, isManual bool) *VerificationStep {
	return &VerificationStep{
		ID:       uuid.New().String()[:8],
		Title:    title,
		Status:   TodoPending,
		IsManual: isManual,
	}
}

// Task represents a complete agentic task
type Task struct {
	ID                string             `json:"id"`
	Title             string             `json:"title"`
	Description       string             `json:"description"`
	OriginalRequest   string             `json:"original_request"`
	Status            TaskStatus         `json:"status"`
	Phase             TaskPhase          `json:"phase"`
	Todos             []Todo             `json:"todos"`
	VerificationSteps []VerificationStep `json:"verification_steps"`
	ExecutionLog      []LogEntry         `json:"execution_log"`
	CreatedAt         time.Time          `json:"created_at"`
	UpdatedAt         time.Time          `json:"updated_at"`
	CompletedAt       time.Time          `json:"completed_at,omitempty"`
}

// LogEntry represents an entry in the execution log
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // "info", "success", "warning", "error"
	Message   string    `json:"message"`
	TodoID    string    `json:"todo_id,omitempty"`
}

// NewTask creates a new task from a user request
func NewTask(request string) *Task {
	return &Task{
		ID:              uuid.New().String()[:8],
		OriginalRequest: request,
		Status:          TaskPending,
		Phase:           PhasePlanning,
		Todos:           make([]Todo, 0),
		ExecutionLog:    make([]LogEntry, 0),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// AddTodo adds a todo to the task
func (t *Task) AddTodo(todo *Todo) {
	t.Todos = append(t.Todos, *todo)
	t.UpdatedAt = time.Now()
}

// AddVerificationStep adds a verification step
func (t *Task) AddVerificationStep(step *VerificationStep) {
	t.VerificationSteps = append(t.VerificationSteps, *step)
	t.UpdatedAt = time.Now()
}

// AddLog adds an entry to the execution log
func (t *Task) AddLog(level, message string, todoID string) {
	t.ExecutionLog = append(t.ExecutionLog, LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		TodoID:    todoID,
	})
	t.UpdatedAt = time.Now()
}

// GetTodoByID finds a todo by its ID
func (t *Task) GetTodoByID(id string) *Todo {
	for i := range t.Todos {
		if t.Todos[i].ID == id {
			return &t.Todos[i]
		}
		// Check children
		for j := range t.Todos[i].Children {
			if t.Todos[i].Children[j].ID == id {
				return &t.Todos[i].Children[j]
			}
		}
	}
	return nil
}

// UpdateTodoStatus updates a todo's status
func (t *Task) UpdateTodoStatus(id string, status TodoStatus, output string) bool {
	todo := t.GetTodoByID(id)
	if todo == nil {
		return false
	}

	todo.Status = status
	todo.Output = output
	if status == TodoInProgress && todo.StartedAt.IsZero() {
		todo.StartedAt = time.Now()
	}
	if status == TodoCompleted || status == TodoFailed || status == TodoSkipped {
		todo.CompletedAt = time.Now()
	}
	t.UpdatedAt = time.Now()
	return true
}

// GetProgress returns completed/total counts
func (t *Task) GetProgress() (completed, total int) {
	for _, todo := range t.Todos {
		total++
		if todo.IsComplete() {
			completed++
		}
		for _, child := range todo.Children {
			total++
			if child.IsComplete() {
				completed++
			}
		}
	}
	return
}

// GetProgressPercent returns completion percentage
func (t *Task) GetProgressPercent() float64 {
	completed, total := t.GetProgress()
	if total == 0 {
		return 0
	}
	return float64(completed) / float64(total) * 100
}

// GetCurrentTodo returns the next pending todo
func (t *Task) GetCurrentTodo() *Todo {
	for i := range t.Todos {
		if t.Todos[i].Status == TodoPending || t.Todos[i].Status == TodoInProgress {
			return &t.Todos[i]
		}
	}
	return nil
}

// GetTodosByPhase returns todos filtered by phase
func (t *Task) GetTodosByPhase(phase TaskPhase) []Todo {
	var result []Todo
	for _, todo := range t.Todos {
		if todo.Phase == phase {
			result = append(result, todo)
		}
	}
	return result
}

// IsPhaseComplete checks if all todos in a phase are complete
func (t *Task) IsPhaseComplete(phase TaskPhase) bool {
	todos := t.GetTodosByPhase(phase)
	if len(todos) == 0 {
		return true
	}
	for _, todo := range todos {
		if !todo.IsComplete() {
			return false
		}
	}
	return true
}

// AdvancePhase moves to the next phase if current is complete
func (t *Task) AdvancePhase() bool {
	switch t.Phase {
	case PhasePlanning:
		if t.IsPhaseComplete(PhasePlanning) {
			t.Phase = PhaseExecution
			t.AddLog("info", "Moving to execution phase", "")
			return true
		}
	case PhaseExecution:
		if t.IsPhaseComplete(PhaseExecution) {
			t.Phase = PhaseVerification
			t.AddLog("info", "Moving to verification phase", "")
			return true
		}
	case PhaseVerification:
		if t.IsPhaseComplete(PhaseVerification) {
			t.Phase = PhaseCompleted
			t.Status = TaskCompleted
			t.CompletedAt = time.Now()
			t.AddLog("success", "Task completed!", "")
			return true
		}
	}
	return false
}

// PlanningResult represents the AI's task breakdown
type PlanningResult struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	PlanningSteps     []string `json:"planning_steps"`
	ExecutionSteps    []string `json:"execution_steps"`
	VerificationSteps []string `json:"verification_steps"`
}

// TaskSummary provides a quick overview for task history lists
type TaskSummary struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Status      TaskStatus `json:"status"`
	Phase       TaskPhase  `json:"phase"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
	TodoCount   int        `json:"todo_count"`
	Progress    float64    `json:"progress"`
}

// ProposedChange represents a file change in the proposal
type ProposedChange struct {
	FilePath    string `json:"file_path"`
	Operation   string `json:"operation"` // "create", "modify", "delete"
	Description string `json:"description"`
	Preview     string `json:"preview,omitempty"` // First ~10 lines of change
}

// TaskProposal is shown to user for approval before execution
type TaskProposal struct {
	TaskID         string           `json:"task_id"`
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	EstimatedTime  string           `json:"estimated_time"`
	Changes        []ProposedChange `json:"changes"`
	PlanningSteps  []string         `json:"planning_steps"`
	ExecutionSteps []string         `json:"execution_steps"`
	VerifySteps    []string         `json:"verification_steps"`
}

// ToSummary converts a Task to a TaskSummary
func (t *Task) ToSummary() TaskSummary {
	return TaskSummary{
		ID:          t.ID,
		Title:       t.Title,
		Status:      t.Status,
		Phase:       t.Phase,
		CreatedAt:   t.CreatedAt,
		CompletedAt: t.CompletedAt,
		TodoCount:   len(t.Todos),
		Progress:    t.GetProgressPercent(),
	}
}
