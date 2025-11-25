package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// CommandExecution tracks an ongoing command execution
type CommandExecution struct {
	cmd     *exec.Cmd
	cancel  context.CancelFunc
	done    chan struct{}
	timeout time.Duration
}

var (
	executionsMutex sync.Mutex
	executions      map[string]*CommandExecution
)

func init() {
	executions = make(map[string]*CommandExecution)
}

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"Terminal Command Executor 🚀",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Add terminal command tools
	s.AddTool(createRunCommandTool(), runCommandHandler)
	s.AddTool(createRunCommandWithEnvTool(), runCommandWithEnvHandler)
	s.AddTool(createRunCommandInDirTool(), runCommandInDirHandler)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func createRunCommandTool() mcp.Tool {
	return mcp.NewTool("run_command",
		mcp.WithDescription("Execute a terminal command and return the output"),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The terminal command to execute"),
		),
		mcp.WithString("args",
			mcp.Description("Arguments for the command as a JSON string array (optional)"),
		),
		mcp.WithBoolean("capture_output",
			mcp.Description("Whether to capture and return command output (default: true)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Command timeout in seconds (default: 30)"),
		),
	)
}

func createRunCommandWithEnvTool() mcp.Tool {
	return mcp.NewTool("run_command_with_env",
		mcp.WithDescription("Execute a command with custom environment variables"),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The terminal command to execute"),
		),
		mcp.WithString("args",
			mcp.Description("Arguments for the command as a JSON string array (optional)"),
		),
		mcp.WithString("env",
			mcp.Description("Environment variables as a JSON string object (optional)"),
		),
		mcp.WithBoolean("capture_output",
			mcp.Description("Whether to capture and return command output (default: true)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Command timeout in seconds (default: 30)"),
		),
	)
}

func createRunCommandInDirTool() mcp.Tool {
	return mcp.NewTool("run_command_in_dir",
		mcp.WithDescription("Execute a command in a specific directory"),
		mcp.WithString("command",
			mcp.Required(),
			mcp.Description("The terminal command to execute"),
		),
		mcp.WithString("args",
			mcp.Description("Arguments for the command as a JSON string array (optional)"),
		),
		mcp.WithString("directory",
			mcp.Required(),
			mcp.Description("Directory to run the command in"),
		),
		mcp.WithBoolean("capture_output",
			mcp.Description("Whether to capture and return command output (default: true)"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Command timeout in seconds (default: 30)"),
		),
	)
}

func runCommandHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := mcp.ParseString(request, "command", "")
	if command == "" {
		return mcp.NewToolResultError("command parameter is required"), nil
	}

	argsStr := mcp.ParseString(request, "args", "")
	captureOutput := mcp.ParseBoolean(request, "capture_output", true)
	timeout := mcp.ParseInt(request, "timeout", 30)

	// Convert args JSON string to slice
	var args []interface{}
	if argsStr != "" {
		var argsArray []string
		if err := json.Unmarshal([]byte(argsStr), &argsArray); err != nil {
			// If JSON parsing fails, treat as single argument
			args = append(args, argsStr)
		} else {
			args = make([]interface{}, len(argsArray))
			for i, arg := range argsArray {
				args[i] = arg
			}
		}
	}

	return executeCommand(ctx, command, args, nil, "", captureOutput, timeout)
}

func runCommandWithEnvHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := mcp.ParseString(request, "command", "")
	if command == "" {
		return mcp.NewToolResultError("command parameter is required"), nil
	}

	argsStr := mcp.ParseString(request, "args", "")
	envStr := mcp.ParseString(request, "env", "")
	captureOutput := mcp.ParseBoolean(request, "capture_output", true)
	timeout := mcp.ParseInt(request, "timeout", 30)

	// Convert args JSON string to slice
	var args []interface{}
	if argsStr != "" {
		var argsArray []string
		if err := json.Unmarshal([]byte(argsStr), &argsArray); err != nil {
			// If JSON parsing fails, treat as single argument
			args = append(args, argsStr)
		} else {
			args = make([]interface{}, len(argsArray))
			for i, arg := range argsArray {
				args[i] = arg
			}
		}
	}

	// Convert env JSON string to []string
	var envVars []string
	if envStr != "" {
		var envMap map[string]interface{}
		if err := json.Unmarshal([]byte(envStr), &envMap); err != nil {
			// If JSON parsing fails, treat as single env var
			envVars = append(envVars, envStr)
		} else {
			for key, value := range envMap {
				envVars = append(envVars, fmt.Sprintf("%s=%v", key, value))
			}
		}
	}

	return executeCommand(ctx, command, args, envVars, "", captureOutput, timeout)
}

func runCommandInDirHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	command := mcp.ParseString(request, "command", "")
	if command == "" {
		return mcp.NewToolResultError("command parameter is required"), nil
	}

	argsStr := mcp.ParseString(request, "args", "")
	directory := mcp.ParseString(request, "directory", "")
	if directory == "" {
		return mcp.NewToolResultError("directory parameter is required"), nil
	}

	captureOutput := mcp.ParseBoolean(request, "capture_output", true)
	timeout := mcp.ParseInt(request, "timeout", 30)

	// Convert args JSON string to slice
	var args []interface{}
	if argsStr != "" {
		var argsArray []string
		if err := json.Unmarshal([]byte(argsStr), &argsArray); err != nil {
			// If JSON parsing fails, treat as single argument
			args = append(args, argsStr)
		} else {
			args = make([]interface{}, len(argsArray))
			for i, arg := range argsArray {
				args[i] = arg
			}
		}
	}

	return executeCommand(ctx, command, args, nil, directory, captureOutput, timeout)
}

func executeCommand(parentCtx context.Context, command string, args []interface{}, envVars []string, directory string, captureOutput bool, timeoutSeconds int) (*mcp.CallToolResult, error) {
	// Handle case where command might contain arguments (e.g., "mkdir folder")
	var actualCommand string
	var stringArgs []string

	// If no args provided but command contains spaces, split it
	if len(args) == 0 && strings.Contains(command, " ") {
		parts := strings.Fields(command)
		if len(parts) > 1 {
			actualCommand = parts[0]
			stringArgs = parts[1:]
		} else {
			actualCommand = command
		}
	} else {
		actualCommand = command
		// Convert args to string slice
		for _, arg := range args {
			stringArgs = append(stringArgs, fmt.Sprintf("%v", arg))
		}
	}

	// Create a context for this command execution with timeout
	var ctx context.Context
	var cancel context.CancelFunc

	if timeoutSeconds > 0 {
		ctx, cancel = context.WithTimeout(parentCtx, time.Duration(timeoutSeconds)*time.Second)
	} else {
		ctx, cancel = context.WithCancel(parentCtx)
	}
	defer cancel()

	// Create the command
	var cmd *exec.Cmd
	if len(stringArgs) > 0 {
		cmd = exec.CommandContext(ctx, actualCommand, stringArgs...)
	} else {
		cmd = exec.CommandContext(ctx, actualCommand)
	}

	// Set environment variables if provided
	if len(envVars) > 0 {
		cmd.Env = append(os.Environ(), envVars...)
	}

	// Set working directory if provided
	if directory != "" {
		cmd.Dir = directory
	}

	// Create output pipes to capture in real-time if needed
	var stdout, stderr strings.Builder
	if captureOutput {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	// Execute the command
	startTime := time.Now()
	err := cmd.Run()
	executionTime := time.Since(startTime)

	// Build result
	var result strings.Builder

	// Command info
	result.WriteString(fmt.Sprintf("🔧 Command: %s", actualCommand))
	if len(stringArgs) > 0 {
		result.WriteString(fmt.Sprintf(" %s", strings.Join(stringArgs, " ")))
	}
	result.WriteString(fmt.Sprintf("\n⏱️  Execution Time: %v\n", executionTime))

	if directory != "" {
		result.WriteString(fmt.Sprintf("📁 Working Directory: %s\n", directory))
	}

	if len(envVars) > 0 {
		result.WriteString("🌍 Environment Variables:\n")
		for _, env := range envVars {
			result.WriteString(fmt.Sprintf("   %s\n", env))
		}
	}

	result.WriteString("----------------------------------------\n")

	// Handle context cancellation (Ctrl+C)
	if ctx.Err() == context.Canceled {
		result.WriteString("⚠️  Command execution was interrupted by user (Ctrl+C).\n")
		result.WriteString("The process has been terminated gracefully.\n")
		return mcp.NewToolResultText(result.String()), nil
	}

	// Handle timeout
	if ctx.Err() == context.DeadlineExceeded {
		result.WriteString(fmt.Sprintf("⏰ Command timed out after %d seconds\n", timeoutSeconds))
		// Try to kill the process if still running
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return mcp.NewToolResultText(result.String()), nil
	}

	// Exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			result.WriteString(fmt.Sprintf("❌ Error: %v\n", err))
			return mcp.NewToolResultText(result.String()), nil
		}
	}

	result.WriteString(fmt.Sprintf("📊 Exit Code: %d\n", exitCode))

	if captureOutput {
		// Standard output
		if stdout.Len() > 0 {
			result.WriteString("\n📤 Standard Output:\n")
			result.WriteString("----------------------------------------\n")
			result.WriteString(stdout.String())
		}

		// Standard error
		if stderr.Len() > 0 {
			result.WriteString("\n📥 Standard Error:\n")
			result.WriteString("----------------------------------------\n")
			result.WriteString(stderr.String())
		}
	} else {
		result.WriteString("\nℹ️  Output capture disabled\n")
	}

	// Success indicator
	if exitCode == 0 {
		result.WriteString("\n✅ Command completed successfully\n")
	} else {
		result.WriteString(fmt.Sprintf("\n⚠️  Command exited with code %d\n", exitCode))
	}

	return mcp.NewToolResultText(result.String()), nil
}