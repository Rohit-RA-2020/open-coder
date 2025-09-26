# File Operations MCP Tool

A comprehensive Model Context Protocol (MCP) tool that provides file and directory operations that can be executed from any directory.

## Features

This tool provides 6 different file and directory operations:

### 1. `read_file`
Read the contents of a file with optional line range parameters.

**Parameters:**
- `path` (required): Path to the file to read (relative to current directory)
- `offset` (optional): Line number to start reading from (1-based, default: 1)
- `limit` (optional): Number of lines to read (default: reads entire file)

### 2. `write_file`
Write content to a file (creates or overwrites existing files).

**Parameters:**
- `path` (required): Path to the file to write (relative to current directory)
- `content` (required): Content to write to the file

### 3. `list_directory`
List contents of a directory with optional recursive listing.

**Parameters:**
- `path` (optional): Path to the directory to list (relative to current directory, defaults to current directory)
- `recursive` (optional): Whether to list contents recursively (default: false)

### 4. `search_files`
Search for files by name pattern using glob patterns.

**Parameters:**
- `pattern` (required): Glob pattern to match file names (e.g., '*.txt', '**/test_*.go')
- `path` (optional): Base directory to search in (relative to current directory, defaults to current directory)

### 5. `search_content`
Search for text content within files with context lines.

**Parameters:**
- `pattern` (required): Text pattern to search for
- `path` (optional): File or directory to search in (relative to current directory)
- `recursive` (optional): Whether to search recursively in subdirectories (default: false)
- `context_lines` (optional): Number of context lines to show before and after matches (default: 2)

### 6. `delete_file`
Delete a file or directory with optional recursive deletion.

**Parameters:**
- `path` (required): Path to the file or directory to delete (relative to current directory)
- `recursive` (optional): Whether to delete directories recursively (use with caution!)

## Usage

1. **Build the tool:**
   ```bash
   cd tools/file-access
   go build -o file-ops-cli main.go
   ```

2. **Run the MCP server:**
   ```bash
   ./file-ops-cli
   ```

3. **Use with MCP clients:**
   The tool communicates via standard input/output using JSON-RPC 2.0 protocol.

## Examples

### List current directory
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_directory",
    "arguments": {}
  }
}
```

### Read a file
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "read_file",
    "arguments": {
      "path": "example.txt"
    }
  }
}
```

### Search for files
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "search_files",
    "arguments": {
      "pattern": "*.go"
    }
  }
}
```

### Search content in files
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "search_content",
    "arguments": {
      "pattern": "TODO",
      "context_lines": 3
    }
  }
}
```

## Notes

- All paths are resolved relative to the current working directory where the binary is executed
- The tool automatically creates parent directories when writing files
- Directory listing shows files with 📄 emoji and directories with 📁 emoji
- Recursive operations are supported but should be used with caution, especially for deletion
- The tool uses standard Go file operations and respects file permissions

## Error Handling

The tool provides comprehensive error handling and returns descriptive error messages for common issues like:
- Invalid or non-existent paths
- Permission errors
- Missing required parameters
- File system errors
