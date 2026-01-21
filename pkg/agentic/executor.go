package agentic

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ExecutorState represents the current state of the executor
type ExecutorState int

const (
	ExecutorIdle ExecutorState = iota
	ExecutorRunning
	ExecutorPaused
	ExecutorCancelled
)

// TaskUpdateCallback is called when task/todo status changes
type TaskUpdateCallback func(update TaskUpdate)

// TaskUpdate contains information about a task/todo state change
type TaskUpdate struct {
	TaskID    string
	TodoID    string
	Phase     TaskPhase
	Status    TodoStatus
	Message   string
	Output    string
	Timestamp time.Time
}

// ExecutorInterface defines what the executor needs from the agent
type ExecutorInterface interface {
	// SendTodoPrompt sends a prompt to the AI and returns the response
	SendTodoPrompt(ctx context.Context, prompt string) (string, error)
	// ExecuteVerification runs a verification command
	ExecuteVerification(ctx context.Context, command string) (string, error)
}

// Executor runs agentic tasks step-by-step
type Executor struct {
	mu       sync.Mutex
	planner  *Planner
	agent    ExecutorInterface
	state    ExecutorState
	task     *Task
	onUpdate TaskUpdateCallback

	// Control channels
	pauseCh  chan struct{}
	resumeCh chan struct{}
	cancelCh chan struct{}
}

// NewExecutor creates a new task executor
func NewExecutor(planner *Planner, agent ExecutorInterface, onUpdate TaskUpdateCallback) *Executor {
	return &Executor{
		planner:  planner,
		agent:    agent,
		state:    ExecutorIdle,
		onUpdate: onUpdate,
		pauseCh:  make(chan struct{}),
		resumeCh: make(chan struct{}),
		cancelCh: make(chan struct{}),
	}
}

// GetState returns the current executor state
func (e *Executor) GetState() ExecutorState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state
}

// GetTask returns the current task
func (e *Executor) GetTask() *Task {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.task
}

// sendUpdate notifies listeners of a state change
func (e *Executor) sendUpdate(todoID string, status TodoStatus, message string) {
	if e.onUpdate == nil || e.task == nil {
		return
	}

	e.onUpdate(TaskUpdate{
		TaskID:    e.task.ID,
		TodoID:    todoID,
		Phase:     e.task.Phase,
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// ExecuteTask runs a task, updating todos as it progresses
func (e *Executor) ExecuteTask(ctx context.Context, task *Task) error {
	e.mu.Lock()
	if e.state == ExecutorRunning {
		e.mu.Unlock()
		return fmt.Errorf("executor is already running")
	}
	e.task = task
	e.state = ExecutorRunning
	e.cancelCh = make(chan struct{})
	e.pauseCh = make(chan struct{})
	e.resumeCh = make(chan struct{})
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		if e.state != ExecutorCancelled {
			e.state = ExecutorIdle
		}
		e.mu.Unlock()
	}()

	task.Status = TaskExecuting
	task.AddLog("info", "Starting task execution", "")

	// Execute todos phase by phase
	for {
		select {
		case <-ctx.Done():
			task.Status = TaskCancelled
			return ctx.Err()
		case <-e.cancelCh:
			task.Status = TaskCancelled
			task.AddLog("warning", "Task cancelled by user", "")
			return fmt.Errorf("task cancelled")
		default:
		}

		// Check for pause
		e.mu.Lock()
		if e.state == ExecutorPaused {
			e.mu.Unlock()
			e.sendUpdate("", TodoPending, "Execution paused")
			select {
			case <-e.resumeCh:
				e.mu.Lock()
				e.state = ExecutorRunning
				e.mu.Unlock()
				e.sendUpdate("", TodoInProgress, "Execution resumed")
			case <-e.cancelCh:
				task.Status = TaskCancelled
				return fmt.Errorf("task cancelled while paused")
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			e.mu.Unlock()
		}

		// Get next todo
		todo := task.GetCurrentTodo()
		if todo == nil {
			// Try to advance phase
			if !task.AdvancePhase() {
				break // All done
			}
			e.sendUpdate("", TodoCompleted, fmt.Sprintf("Completed %s phase", task.Phase))
			continue
		}

		// Execute the todo
		if err := e.ExecuteTodo(ctx, todo); err != nil {
			// Non-fatal error, log and continue
			task.AddLog("error", fmt.Sprintf("Todo failed: %v", err), todo.ID)
		}
	}

	// Final status
	if task.Status != TaskCancelled {
		if task.Phase == PhaseCompleted {
			task.Status = TaskCompleted
			task.CompletedAt = time.Now()
		}
	}

	e.sendUpdate("", TodoCompleted, "Task execution finished")
	return nil
}

// ExecuteTodo executes a single todo item
func (e *Executor) ExecuteTodo(ctx context.Context, todo *Todo) error {
	if todo == nil {
		return fmt.Errorf("nil todo")
	}

	// Mark as in progress
	e.task.UpdateTodoStatus(todo.ID, TodoInProgress, "")
	e.sendUpdate(todo.ID, TodoInProgress, fmt.Sprintf("Working on: %s", todo.Title))
	e.task.AddLog("info", fmt.Sprintf("Starting: %s", todo.Title), todo.ID)

	// Handle verification phase differently
	if todo.Phase == PhaseVerification {
		return e.executeVerificationTodo(ctx, todo)
	}

	// Generate prompt for this todo
	prompt := e.planner.GenerateTodoPrompt(e.task, todo)

	// Execute via agent
	result, err := e.agent.SendTodoPrompt(ctx, prompt)
	if err != nil {
		e.task.UpdateTodoStatus(todo.ID, TodoFailed, err.Error())
		e.sendUpdate(todo.ID, TodoFailed, fmt.Sprintf("Failed: %s", err.Error()))
		return err
	}

	// Mark as completed
	e.task.UpdateTodoStatus(todo.ID, TodoCompleted, result)
	e.sendUpdate(todo.ID, TodoCompleted, fmt.Sprintf("Completed: %s", todo.Title))
	e.task.AddLog("success", fmt.Sprintf("Completed: %s", todo.Title), todo.ID)

	return nil
}

// executeVerificationTodo handles verification-phase todos
func (e *Executor) executeVerificationTodo(ctx context.Context, todo *Todo) error {
	// Find matching verification step
	var verifyStep *VerificationStep
	for i := range e.task.VerificationSteps {
		if e.task.VerificationSteps[i].Title == todo.Title {
			verifyStep = &e.task.VerificationSteps[i]
			break
		}
	}

	if verifyStep != nil && verifyStep.Command != "" && !verifyStep.IsManual {
		// Automated verification
		result, err := e.agent.ExecuteVerification(ctx, verifyStep.Command)
		if err != nil {
			verifyStep.Status = TodoFailed
			verifyStep.Passed = false
			verifyStep.Result = err.Error()
			e.task.UpdateTodoStatus(todo.ID, TodoFailed, err.Error())
			e.sendUpdate(todo.ID, TodoFailed, "Verification failed")
			return err
		}

		verifyStep.Status = TodoCompleted
		verifyStep.Passed = true
		verifyStep.Result = result
		e.task.UpdateTodoStatus(todo.ID, TodoCompleted, result)
		e.sendUpdate(todo.ID, TodoCompleted, "Verification passed")
		return nil
	}

	// For manual verification or no command, just mark complete
	// In a full implementation, this would prompt the user
	e.task.UpdateTodoStatus(todo.ID, TodoCompleted, "Manual verification assumed passed")
	e.sendUpdate(todo.ID, TodoCompleted, "Verification step completed")
	return nil
}

// Pause pauses execution at the current step
func (e *Executor) Pause() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.state == ExecutorRunning {
		e.state = ExecutorPaused
		if e.task != nil {
			e.task.Status = TaskPaused
			e.task.AddLog("info", "Execution paused", "")
		}
	}
}

// Resume continues paused execution
func (e *Executor) Resume() {
	e.mu.Lock()
	if e.state == ExecutorPaused {
		e.mu.Unlock()
		select {
		case e.resumeCh <- struct{}{}:
		default:
		}
		if e.task != nil {
			e.task.Status = TaskExecuting
			e.task.AddLog("info", "Execution resumed", "")
		}
	} else {
		e.mu.Unlock()
	}
}

// Cancel cancels the current task
func (e *Executor) Cancel() {
	e.mu.Lock()
	if e.state == ExecutorRunning || e.state == ExecutorPaused {
		e.state = ExecutorCancelled
		e.mu.Unlock()
		close(e.cancelCh)
		if e.task != nil {
			e.task.Status = TaskCancelled
			e.task.AddLog("warning", "Task cancelled", "")
		}
	} else {
		e.mu.Unlock()
	}
}

// SkipTodo marks a todo as skipped and moves on
func (e *Executor) SkipTodo(todoID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.task == nil {
		return false
	}

	todo := e.task.GetTodoByID(todoID)
	if todo == nil {
		return false
	}

	if todo.Status == TodoInProgress || todo.Status == TodoPending {
		e.task.UpdateTodoStatus(todoID, TodoSkipped, "Skipped by user")
		e.task.AddLog("info", fmt.Sprintf("Skipped: %s", todo.Title), todoID)
		e.sendUpdate(todoID, TodoSkipped, "Todo skipped")
		return true
	}

	return false
}

// IsPaused returns whether execution is paused
func (e *Executor) IsPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state == ExecutorPaused
}

// IsRunning returns whether execution is running
func (e *Executor) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state == ExecutorRunning
}
