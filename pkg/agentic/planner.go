package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openai/openai-go/v3"
)

// Planner handles AI-powered task breakdown
type Planner struct {
	client *openai.Client
	model  string
}

// NewPlanner creates a new task planner
func NewPlanner(client *openai.Client, model string) *Planner {
	return &Planner{
		client: client,
		model:  model,
	}
}

// analysisSystemPrompt guides AI to analyze a project structure
const analysisSystemPrompt = `You are an expert software engineer analyzing a codebase.

Given the project structure and file contents, provide a concise analysis that covers:

1. PROJECT TYPE: What kind of project is this (web app, CLI tool, library, etc.)?
2. TECH STACK: Languages, frameworks, key dependencies
3. ARCHITECTURE: Main components, modules, or packages
4. KEY FILES: Entry points, configuration files, important modules
5. PATTERNS: Coding patterns, conventions, or standards used

Keep your analysis focused and concise (max 500 words). Focus on information that would help plan coding tasks.`

// planningSystemPrompt guides the AI to generate structured task breakdowns
const planningSystemPrompt = `You are an expert software engineer tasked with breaking down coding requests into actionable steps.

You have been provided with:
1. A coding request from the user
2. Complete analysis of the project structure (already analyzed - DO NOT include "analyze project" as a step)

IMPORTANT: The project has already been analyzed. Do NOT include steps like "analyze project structure" or "go through the project" - that's already done.

Using this context, create a structured plan with three phases:

1. PLANNING: Preparation steps like reading specific relevant files to understand implementation details
2. EXECUTION: Implementation steps (creating files, writing code, modifying existing code)
3. VERIFICATION: Testing and validation (running tests, manual checks, reviewing changes)

Respond ONLY with valid JSON in this exact format:
{
  "title": "Short descriptive title for the task",
  "description": "Brief description of what will be accomplished",
  "planning_steps": ["Step 1", "Step 2", ...],
  "execution_steps": ["Step 1", "Step 2", ...],
  "verification_steps": ["Step 1", "Step 2", ...]
}

Guidelines:
- Keep steps concise and actionable (max 80 chars each)
- Reference actual file paths from the provided project structure
- Follow the project's existing patterns and conventions
- 1-3 planning steps (focus on reading specific files, NOT analyzing the project)
- 3-6 execution steps typically
- 1-3 verification steps typically
- Be specific about files and functions when possible
- Order steps logically (dependencies first)`

// ProjectAnalysis contains the result of analyzing a project
type ProjectAnalysis struct {
	Summary     string   `json:"summary"`
	ProjectType string   `json:"project_type"`
	TechStack   []string `json:"tech_stack"`
	KeyFiles    []string `json:"key_files"`
	FileTree    string   `json:"file_tree"`
	Patterns    string   `json:"patterns"`
}

// AnalyzeProject analyzes the codebase to provide context for planning
func (p *Planner) AnalyzeProject(ctx context.Context, projectPath string, request string) (*ProjectAnalysis, error) {
	// Get project structure
	fileTree := getProjectFileTree(projectPath, 3) // max depth 3

	// Read key files (package.json, go.mod, requirements.txt, README, etc.)
	keyFileContents := readKeyFiles(projectPath)

	// Build analysis prompt
	prompt := fmt.Sprintf(`Please analyze this project to help plan: "%s"

Project Structure:
%s

Key File Contents:
%s

Provide a concise analysis that will help plan this task.`, request, fileTree, keyFileContents)

	// Call AI for analysis
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(analysisSystemPrompt),
			openai.UserMessage(prompt),
		},
		Model:       openai.ChatModel(p.model),
		Temperature: openai.Float(0.3),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze project: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from AI for analysis")
	}

	analysis := &ProjectAnalysis{
		Summary:  strings.TrimSpace(resp.Choices[0].Message.Content),
		FileTree: fileTree,
	}

	return analysis, nil
}

// PlanTask takes a user request and returns a structured Task with todos
func (p *Planner) PlanTask(ctx context.Context, request string, codebaseContext string) (*Task, error) {
	task := NewTask(request)
	task.Status = TaskPlanning

	// Build the planning prompt
	prompt := fmt.Sprintf("Coding Request: %s", request)
	if codebaseContext != "" {
		prompt += fmt.Sprintf("\n\nCodebase Context:\n%s", codebaseContext)
	}

	// Call the AI for task breakdown
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(planningSystemPrompt),
			openai.UserMessage(prompt),
		},
		Model:       openai.ChatModel(p.model),
		Temperature: openai.Float(0.3), // Lower temperature for more consistent structure
	})

	if err != nil {
		task.Status = TaskFailed
		task.AddLog("error", fmt.Sprintf("Planning failed: %v", err), "")
		return task, fmt.Errorf("failed to plan task: %w", err)
	}

	if len(resp.Choices) == 0 {
		task.Status = TaskFailed
		return task, fmt.Errorf("no response from AI")
	}

	// Parse the AI response
	content := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Extract JSON from response (handle potential markdown code blocks)
	content = extractJSON(content)

	var planResult PlanningResult
	if err := json.Unmarshal([]byte(content), &planResult); err != nil {
		task.AddLog("error", fmt.Sprintf("Failed to parse plan: %v", err), "")
		// Try to create a basic plan from the raw response
		planResult = createFallbackPlan(request, content)
	}

	// Populate task from planning result
	task.Title = planResult.Title
	task.Description = planResult.Description

	order := 0

	// Add planning todos
	for _, step := range planResult.PlanningSteps {
		todo := NewTodo(step, PhasePlanning, order)
		task.AddTodo(todo)
		order++
	}

	// Add execution todos
	for _, step := range planResult.ExecutionSteps {
		todo := NewTodo(step, PhaseExecution, order)
		task.AddTodo(todo)
		order++
	}

	// Add verification steps
	for _, step := range planResult.VerificationSteps {
		isManual := strings.Contains(strings.ToLower(step), "manual") ||
			strings.Contains(strings.ToLower(step), "review") ||
			strings.Contains(strings.ToLower(step), "check")
		verifyStep := NewVerificationStep(step, isManual)
		task.AddVerificationStep(verifyStep)

		// Also add as todo for tracking
		todo := NewTodo(step, PhaseVerification, order)
		task.AddTodo(todo)
		order++
	}

	task.AddLog("success", fmt.Sprintf("Created plan with %d steps", len(task.Todos)), "")
	return task, nil
}

// RefineTask updates a task based on user feedback
func (p *Planner) RefineTask(ctx context.Context, task *Task, feedback string) (*Task, error) {
	prompt := fmt.Sprintf(`Current task plan:
Title: %s
Description: %s

Current steps:
%s

User feedback: %s

Please revise the plan based on this feedback. Respond with the same JSON format.`,
		task.Title,
		task.Description,
		formatTodosForPrompt(task.Todos),
		feedback,
	)

	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(planningSystemPrompt),
			openai.UserMessage(prompt),
		},
		Model:       openai.ChatModel(p.model),
		Temperature: openai.Float(0.3),
	})

	if err != nil {
		return task, fmt.Errorf("failed to refine task: %w", err)
	}

	if len(resp.Choices) == 0 {
		return task, fmt.Errorf("no response from AI")
	}

	content := extractJSON(strings.TrimSpace(resp.Choices[0].Message.Content))

	var planResult PlanningResult
	if err := json.Unmarshal([]byte(content), &planResult); err != nil {
		task.AddLog("warning", "Could not parse refined plan, keeping original", "")
		return task, nil
	}

	// Recreate todos from refined plan
	task.Todos = make([]Todo, 0)
	task.Title = planResult.Title
	task.Description = planResult.Description

	order := 0
	for _, step := range planResult.PlanningSteps {
		todo := NewTodo(step, PhasePlanning, order)
		task.AddTodo(todo)
		order++
	}
	for _, step := range planResult.ExecutionSteps {
		todo := NewTodo(step, PhaseExecution, order)
		task.AddTodo(todo)
		order++
	}
	for _, step := range planResult.VerificationSteps {
		todo := NewTodo(step, PhaseVerification, order)
		task.AddTodo(todo)
		order++
	}

	task.AddLog("info", "Plan refined based on feedback", "")
	return task, nil
}

// GenerateTodoPrompt creates a prompt for executing a specific todo
func (p *Planner) GenerateTodoPrompt(task *Task, todo *Todo) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are working on: %s\n\n", task.Title))
	sb.WriteString(fmt.Sprintf("Overall goal: %s\n\n", task.OriginalRequest))
	sb.WriteString(fmt.Sprintf("Current step: %s\n", todo.Title))

	if todo.Description != "" {
		sb.WriteString(fmt.Sprintf("Details: %s\n", todo.Description))
	}

	// Add context from completed todos
	completedTodos := getCompletedTodos(task.Todos)
	if len(completedTodos) > 0 {
		sb.WriteString("\nCompleted steps:\n")
		for _, ct := range completedTodos {
			sb.WriteString(fmt.Sprintf("- ✅ %s\n", ct.Title))
			if ct.Output != "" {
				// Truncate long outputs
				output := ct.Output
				if len(output) > 200 {
					output = output[:200] + "..."
				}
				sb.WriteString(fmt.Sprintf("  Result: %s\n", output))
			}
		}
	}

	sb.WriteString("\nPlease complete this step. Use available tools as needed.")

	return sb.String()
}

// extractJSON extracts JSON from a string that may contain markdown code blocks
func extractJSON(s string) string {
	// Remove markdown code block markers
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// Find JSON object boundaries
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}
	return s
}

// createFallbackPlan creates a basic plan when AI response can't be parsed
func createFallbackPlan(request, rawResponse string) PlanningResult {
	return PlanningResult{
		Title:       truncateString(request, 50),
		Description: "Auto-generated plan from request",
		PlanningSteps: []string{
			"Analyze the request",
			"Review relevant code",
		},
		ExecutionSteps: []string{
			"Implement the requested changes",
		},
		VerificationSteps: []string{
			"Review changes",
		},
	}
}

// formatTodosForPrompt formats todos for inclusion in a prompt
func formatTodosForPrompt(todos []Todo) string {
	var sb strings.Builder
	currentPhase := TaskPhase("")

	for _, todo := range todos {
		if todo.Phase != currentPhase {
			currentPhase = todo.Phase
			sb.WriteString(fmt.Sprintf("\n[%s]\n", strings.ToUpper(string(currentPhase))))
		}
		sb.WriteString(fmt.Sprintf("- %s %s\n", todo.Status.Icon(), todo.Title))
	}

	return sb.String()
}

// getCompletedTodos returns only completed todos
func getCompletedTodos(todos []Todo) []Todo {
	var result []Todo
	for _, todo := range todos {
		if todo.Status == TodoCompleted {
			result = append(result, todo)
		}
	}
	return result
}

// truncateString truncates a string to max length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// CreateProposal generates a detailed proposal from a task plan
func (p *Planner) CreateProposal(task *Task) *TaskProposal {
	proposal := &TaskProposal{
		TaskID:        task.ID,
		Title:         task.Title,
		Description:   task.Description,
		EstimatedTime: estimateTaskTime(task),
	}

	// Categorize steps and extract file changes
	for _, todo := range task.Todos {
		switch todo.Phase {
		case PhasePlanning:
			proposal.PlanningSteps = append(proposal.PlanningSteps, todo.Title)
		case PhaseExecution:
			proposal.ExecutionSteps = append(proposal.ExecutionSteps, todo.Title)
			// Try to detect file operations from step title
			change := analyzeStepForChanges(todo.Title)
			if change != nil {
				proposal.Changes = append(proposal.Changes, *change)
			}
		case PhaseVerification:
			proposal.VerifySteps = append(proposal.VerifySteps, todo.Title)
		}
	}

	return proposal
}

// estimateTaskTime provides a rough time estimate based on step count
func estimateTaskTime(task *Task) string {
	stepCount := len(task.Todos)
	if stepCount <= 3 {
		return "~2 minutes"
	} else if stepCount <= 6 {
		return "~5 minutes"
	} else if stepCount <= 10 {
		return "~10 minutes"
	}
	return "~15+ minutes"
}

// analyzeStepForChanges detects file operations from step descriptions
func analyzeStepForChanges(stepTitle string) *ProposedChange {
	lower := strings.ToLower(stepTitle)

	// Detect file creation patterns
	if strings.Contains(lower, "create") || strings.Contains(lower, "add new") {
		return &ProposedChange{
			FilePath:    extractFilePath(stepTitle),
			Operation:   "create",
			Description: stepTitle,
		}
	}

	// Detect file modification patterns
	if strings.Contains(lower, "modify") || strings.Contains(lower, "update") ||
		strings.Contains(lower, "edit") || strings.Contains(lower, "change") ||
		strings.Contains(lower, "add ") || strings.Contains(lower, "implement") {
		return &ProposedChange{
			FilePath:    extractFilePath(stepTitle),
			Operation:   "modify",
			Description: stepTitle,
		}
	}

	// Detect file deletion patterns
	if strings.Contains(lower, "delete") || strings.Contains(lower, "remove") {
		return &ProposedChange{
			FilePath:    extractFilePath(stepTitle),
			Operation:   "delete",
			Description: stepTitle,
		}
	}

	return nil
}

// extractFilePath attempts to extract a file path from a step description
func extractFilePath(step string) string {
	// Common file extensions to look for
	extensions := []string{".go", ".js", ".ts", ".py", ".rb", ".rs", ".java", ".c", ".cpp", ".h",
		".html", ".css", ".scss", ".json", ".yaml", ".yml", ".md", ".txt", ".sql"}

	words := strings.Fields(step)
	for _, word := range words {
		// Remove common punctuation
		word = strings.Trim(word, "(),.:;\"'`")
		for _, ext := range extensions {
			if strings.HasSuffix(word, ext) {
				return word
			}
		}
		// Check for path-like patterns
		if strings.Contains(word, "/") && len(word) > 3 {
			return word
		}
	}

	return "file"
}

// getProjectFileTree generates a file tree representation of the project
func getProjectFileTree(rootPath string, maxDepth int) string {
	var builder strings.Builder

	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil || relPath == "." {
			return nil
		}

		// Skip hidden files and directories (except .github, .vscode)
		base := filepath.Base(path)
		if strings.HasPrefix(base, ".") && base != ".github" && base != ".vscode" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip common non-essential directories
		skipDirs := []string{"node_modules", "vendor", "dist", "build", ".git", "__pycache__", ".next", ".nuxt"}
		for _, skip := range skipDirs {
			if base == skip {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Calculate depth
		depth := strings.Count(relPath, string(os.PathSeparator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Build tree with indentation
		indent := strings.Repeat("  ", depth)
		if d.IsDir() {
			builder.WriteString(fmt.Sprintf("%s📁 %s/\n", indent, base))
		} else {
			builder.WriteString(fmt.Sprintf("%s📄 %s\n", indent, base))
		}

		return nil
	})

	if err != nil {
		return fmt.Sprintf("Error scanning project: %v", err)
	}

	result := builder.String()
	if len(result) > 3000 {
		result = result[:3000] + "\n... (truncated)"
	}

	return result
}

// readKeyFiles reads important project configuration files
func readKeyFiles(rootPath string) string {
	keyFiles := []string{
		"go.mod",
		"go.sum",
		"package.json",
		"requirements.txt",
		"pyproject.toml",
		"Cargo.toml",
		"pom.xml",
		"build.gradle",
		"README.md",
		"README",
		"Makefile",
		"docker-compose.yml",
		"Dockerfile",
		".env.example",
	}

	var builder strings.Builder

	for _, file := range keyFiles {
		fullPath := filepath.Join(rootPath, file)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue // File doesn't exist
		}

		// Truncate large files
		contentStr := string(content)
		if len(contentStr) > 1500 {
			contentStr = contentStr[:1500] + "\n... (truncated)"
		}

		builder.WriteString(fmt.Sprintf("=== %s ===\n%s\n\n", file, contentStr))
	}

	result := builder.String()
	if result == "" {
		return "No key project files found."
	}

	return result
}
