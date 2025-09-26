# Terminal Command Executor MCP Server

A powerful MCP (Model Context Protocol) server that provides terminal command execution capabilities with comprehensive output capture and environment control.

## 🚀 Features

- **Execute any terminal command** with full output capture
- **Environment variable control** for custom execution contexts
- **Directory-specific execution** to run commands in specific locations
- **Timeout management** to prevent hanging commands
- **Detailed execution reporting** with timing, exit codes, and visual indicators

## 🔧 Available Tools

### 1. `run_command`
Execute a terminal command with optional arguments.

**Parameters:**
- `command` (required): The terminal command to execute
- `args` (optional): Arguments for the command as a JSON string array (e.g., `["-la", "/tmp"]`)
- `capture_output` (optional): Whether to capture output (default: true)
- `timeout` (optional): Command timeout in seconds (default: 30)

**Example:**
```json
{
  "tool": "run_command",
  "arguments": {
    "command": "ls",
    "args": "[\"-la\", \"/tmp\"]",
    "timeout": 10
  }
}
```

### 2. `run_command_with_env`
Execute a command with custom environment variables.

**Parameters:**
- `command` (required): The terminal command to execute
- `args` (optional): Arguments for the command as a JSON string array (e.g., `["$HOME", "$USER"]`)
- `env` (optional): Environment variables as a JSON string object (e.g., `{"HOME": "/custom/home", "USER": "testuser"}`)
- `capture_output` (optional): Whether to capture output (default: true)
- `timeout` (optional): Command timeout in seconds (default: 30)

**Example:**
```json
{
  "tool": "run_command_with_env",
  "arguments": {
    "command": "echo",
    "args": "[\"$HOME\", \"$USER\"]",
    "env": "{\"HOME\": \"/custom/home\", \"USER\": \"testuser\"}"
  }
}
```

### 3. `run_command_in_dir`
Execute a command in a specific directory.

**Parameters:**
- `command` (required): The terminal command to execute
- `args` (optional): Arguments for the command as a JSON string array (e.g., `["build"]`)
- `directory` (required): Directory to run the command in
- `capture_output` (optional): Whether to capture output (default: true)
- `timeout` (optional): Command timeout in seconds (default: 30)

**Example:**
```json
{
  "tool": "run_command_in_dir",
  "arguments": {
    "command": "make",
    "args": "[\"build\"]",
    "directory": "/path/to/project"
  }
}
```

## 📊 Output Format

The server returns detailed execution information including:

- **Command executed** with all arguments
- **Execution time** for performance monitoring
- **Working directory** (if specified)
- **Environment variables** (if custom ones were set)
- **Exit code** indicating success/failure
- **Standard output** (stdout)
- **Standard error** (stderr)
- **Success indicator** with visual markers

## 🛡️ Safety Features

- **Timeout protection**: Prevents commands from running indefinitely
- **Output capture control**: Option to disable output capture for memory efficiency
- **Error handling**: Comprehensive error reporting and validation
- **Path validation**: Safe handling of directory paths

## 💡 Common Use Cases

1. **System Administration**: Run system commands like `ps`, `top`, `df`
2. **Development Tools**: Execute build commands, tests, or deployment scripts
3. **File Operations**: Use commands like `find`, `grep`, `sed` for complex operations
4. **Network Tools**: Run commands like `ping`, `curl`, `wget`
5. **Process Management**: Monitor and control system processes

## 🔍 Example Usage

### List files in a directory
```json
{
  "tool": "run_command",
  "arguments": {
    "command": "ls",
    "args": "[\"-la\", \"/home/user\"]"
  }
}
```

### Check system information
```json
{
  "tool": "run_command",
  "arguments": {
    "command": "uname",
    "args": "[\"-a\"]"
  }
}
```

### Run a command in a specific directory with environment variables
```json
{
  "tool": "run_command_in_dir",
  "arguments": {
    "command": "npm",
    "args": "[\"install\"]",
    "directory": "/path/to/node/project"
  }
}
```

## 🚦 Exit Codes

- **0**: Command completed successfully
- **Non-zero**: Command failed (specific meaning depends on the command)
- **Timeout**: Command was terminated due to timeout

## ⚡ Performance Tips

1. **Use appropriate timeouts** for long-running commands
2. **Disable output capture** for commands with large output to save memory
3. **Use specific directories** instead of changing directories in commands
4. **Batch related commands** to reduce overhead

## 🐛 Troubleshooting

1. **Command not found**: Ensure the command is in PATH or provide full path
2. **Permission denied**: Check file permissions and user privileges
3. **Timeout errors**: Increase timeout value for long-running commands
4. **Environment issues**: Verify environment variable syntax and values

This MCP server provides a robust interface for terminal command execution, making it perfect for AI assistants that need to interact with the system shell safely and effectively.
