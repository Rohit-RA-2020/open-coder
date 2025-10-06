package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/qdrant/go-client/qdrant"
)

// OpenCoderConfig represents the JSON configuration structure
type OpenCoderConfig struct {
	Indexer struct {
		Embedding struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
			Model   string `json:"model"`
		} `json:"embedding"`
		Qdrant struct {
			Host string `json:"host"`
			Port string `json:"port"`
		} `json:"qdrant"`
		VectorDimensions string `json:"vector_dimensions"`
	} `json:"indexer"`
}

// loadJSONConfig loads configuration from the JSON config file
func loadJSONConfig() (*OpenCoderConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".open-coder", "config")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config OpenCoderConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	// Create a new MCP server
	s := server.NewMCPServer(
		"File Operations CLI 🚀",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	// Add file operation tools
	s.AddTool(createReadFileTool(), readFileHandler)
	s.AddTool(createReadLineRangeTool(), readLineRangeHandler)
	s.AddTool(createWriteFileTool(), writeFileHandler)
	s.AddTool(createEditLineRangeTool(), editLineRangeHandler)
	s.AddTool(createListDirectoryTool(), listDirectoryHandler)
	s.AddTool(createSearchFilesTool(), searchFilesHandler)
	s.AddTool(createSearchContentTool(), searchContentHandler)
	s.AddTool(createDeleteFileTool(), deleteFileHandler)
	s.AddTool(createSemanticSearchTool(), semanticSearchHandler)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func createReadFileTool() mcp.Tool {
	return mcp.NewTool("read_file",
		mcp.WithDescription("Read the contents of a file with optional line numbers"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file to read (relative to current directory)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Line number to start reading from (1-based, optional)"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Number of lines to read (optional, reads entire file if not specified)"),
		),
		mcp.WithBoolean("show_line_numbers",
			mcp.Description("Whether to include line numbers in the output (default: false)"),
		),
	)
}

func createWriteFileTool() mcp.Tool {
	return mcp.NewTool("write_file",
		mcp.WithDescription("Write content to a file (creates or overwrites)"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file to write (relative to current directory)"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Content to write to the file"),
		),
	)
}

func createListDirectoryTool() mcp.Tool {
	return mcp.NewTool("list_directory",
		mcp.WithDescription("List contents of a directory"),
		mcp.WithString("path",
			mcp.Description("Path to the directory to list (relative to current directory, defaults to current directory)"),
		),
		mcp.WithBoolean("recursive",
			mcp.Description("Whether to list contents recursively (default: false)"),
		),
	)
}

func createSearchFilesTool() mcp.Tool {
	return mcp.NewTool("search_files",
		mcp.WithDescription("Search for files by name pattern"),
		mcp.WithString("pattern",
			mcp.Required(),
			mcp.Description("Glob pattern to match file names (e.g., '*.txt', '**/test_*.go')"),
		),
		mcp.WithString("path",
			mcp.Description("Base directory to search in (relative to current directory, defaults to current directory)"),
		),
	)
}

func createSearchContentTool() mcp.Tool {
	return mcp.NewTool("search_content",
		mcp.WithDescription("Search for text content within files"),
		mcp.WithString("pattern",
			mcp.Required(),
			mcp.Description("Text pattern to search for (supports regex)"),
		),
		mcp.WithString("path",
			mcp.Description("File or directory to search in (relative to current directory)"),
		),
		mcp.WithBoolean("recursive",
			mcp.Description("Whether to search recursively in subdirectories (default: false)"),
		),
		mcp.WithNumber("context_lines",
			mcp.Description("Number of context lines to show before and after matches (default: 2)"),
		),
	)
}

func createDeleteFileTool() mcp.Tool {
	return mcp.NewTool("delete_file",
		mcp.WithDescription("Delete a file or directory"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file or directory to delete (relative to current directory)"),
		),
		mcp.WithBoolean("recursive",
			mcp.Description("Whether to delete directories recursively (use with caution!)"),
		),
	)
}

func createReadLineRangeTool() mcp.Tool {
	return mcp.NewTool("read_line_range",
		mcp.WithDescription("Read specific lines or a range of lines from a file"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file to read (relative to current directory)"),
		),
		mcp.WithNumber("start_line",
			mcp.Required(),
			mcp.Description("Starting line number (1-based)"),
		),
		mcp.WithNumber("end_line",
			mcp.Description("Ending line number (1-based, optional - if not provided, reads only the start_line)"),
		),
		mcp.WithBoolean("show_line_numbers",
			mcp.Description("Whether to include line numbers in the output (default: true)"),
		),
	)
}

func createEditLineRangeTool() mcp.Tool {
	return mcp.NewTool("edit_line_range",
		mcp.WithDescription("Edit specific lines or a range of lines in a file"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file to edit (relative to current directory)"),
		),
		mcp.WithNumber("start_line",
			mcp.Required(),
			mcp.Description("Starting line number to edit (1-based)"),
		),
		mcp.WithNumber("end_line",
			mcp.Description("Ending line number to edit (1-based, optional - if not provided, edits only the start_line)"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("New content to replace the specified lines (use \\n for line breaks)"),
		),
		mcp.WithString("operation",
			mcp.Description("Operation type: 'replace' (default), 'insert_before', or 'insert_after'"),
		),
	)
}

func createSemanticSearchTool() mcp.Tool {
	return mcp.NewTool("semantic_search",
		mcp.WithDescription("Perform semantic search on indexed code using vector embeddings"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query to find semantically similar code"),
		),
		mcp.WithString("collection",
			mcp.Required(),
			mcp.Description("Collection name (directory path) to search in"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 2, max: 10)"),
		),
	)
}

func readFileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := mcp.ParseString(request, "path", "")
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	offset := mcp.ParseInt(request, "offset", 1)
	if offset < 1 {
		offset = 1
	}

	limit := mcp.ParseInt(request, "limit", -1)

	showLineNumbers := mcp.ParseBoolean(request, "show_line_numbers", false)

	// Resolve path relative to current working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	lines := strings.Split(string(content), "\n")

	// Adjust offset to 0-based indexing
	offset--

	// Handle offset bounds
	if offset >= len(lines) {
		return mcp.NewToolResultText(""), nil
	}

	end := len(lines)
	if limit > 0 && offset+limit < len(lines) {
		end = offset + limit
	}

	resultLines := lines[offset:end]

	var result string
	if showLineNumbers {
		var formattedLines []string
		for i, line := range resultLines {
			lineNum := offset + i + 1
			formattedLines = append(formattedLines, fmt.Sprintf("%4d: %s", lineNum, line))
		}
		result = strings.Join(formattedLines, "\n")
	} else {
		result = strings.Join(resultLines, "\n")
	}

	return mcp.NewToolResultText(result), nil
}

func writeFileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := mcp.ParseString(request, "path", "")
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	content := mcp.ParseString(request, "content", "")
	if content == "" {
		return mcp.NewToolResultError("content parameter is required"), nil
	}

	// Resolve path relative to current working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
	}

	// Ensure directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create directory: %v", err)), nil
	}

	err = os.WriteFile(absPath, []byte(content), 0644)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to write file: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully wrote %d bytes to %s", len(content), path)), nil
}

func listDirectoryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := mcp.ParseString(request, "path", ".")

	recursive := mcp.ParseBoolean(request, "recursive", false)

	// Resolve path relative to current working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Contents of %s:\n", absPath))
	result.WriteString("----------------------------------------\n")

	if recursive {
		err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Calculate relative path from the starting directory
			relPath, err := filepath.Rel(absPath, path)
			if err != nil {
				relPath = path
			}

			indent := strings.Repeat("  ", strings.Count(relPath, string(filepath.Separator)))
			if indent != "" {
				indent = "│" + indent[:len(indent)-1] + " "
			}

			fileType := "📄"
			if info.IsDir() {
				fileType = "📁"
			}

			result.WriteString(fmt.Sprintf("%s%s %s\n", indent, fileType, info.Name()))
			return nil
		})
	} else {
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read directory: %v", err)), nil
		}

		// Sort entries (directories first, then files)
		sort.Slice(entries, func(i, j int) bool {
			iIsDir := entries[i].IsDir()
			jIsDir := entries[j].IsDir()
			if iIsDir != jIsDir {
				return iIsDir
			}
			return entries[i].Name() < entries[j].Name()
		})

		for _, entry := range entries {
			fileType := "📄"
			if entry.IsDir() {
				fileType = "📁"
			}
			result.WriteString(fmt.Sprintf("%s %s\n", fileType, entry.Name()))
		}
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error during directory traversal: %v", err)), nil
	}

	return mcp.NewToolResultText(result.String()), nil
}

func searchFilesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pattern := mcp.ParseString(request, "pattern", "")
	if pattern == "" {
		return mcp.NewToolResultError("pattern parameter is required"), nil
	}

	basePath := mcp.ParseString(request, "path", ".")

	// Resolve base path relative to current working directory
	absBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid base path: %v", err)), nil
	}

	var matches []string
	err = filepath.Walk(absBasePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if the filename matches the pattern
		matched, err := filepath.Match(pattern, info.Name())
		if err != nil {
			return err
		}

		if matched {
			// Get relative path from base directory
			relPath, err := filepath.Rel(absBasePath, path)
			if err != nil {
				relPath = path
			}
			matches = append(matches, relPath)
		}

		return nil
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error during file search: %v", err)), nil
	}

	if len(matches) == 0 {
		return mcp.NewToolResultText("No files found matching pattern: " + pattern), nil
	}

	result := fmt.Sprintf("Found %d files matching pattern '%s':\n", len(matches), pattern)
	result += "----------------------------------------\n"
	for _, match := range matches {
		result += match + "\n"
	}

	return mcp.NewToolResultText(result), nil
}

func searchContentHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pattern := mcp.ParseString(request, "pattern", "")
	if pattern == "" {
		return mcp.NewToolResultError("pattern parameter is required"), nil
	}

	searchPath := mcp.ParseString(request, "path", ".")

	recursive := mcp.ParseBoolean(request, "recursive", false)

	contextLines := mcp.ParseInt(request, "context_lines", 2)
	if contextLines < 0 {
		contextLines = 0
	}

	// Resolve path relative to current working directory
	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Searching for pattern '%s' in %s", pattern, absPath))
	if recursive {
		result.WriteString(" (recursive)")
	}
	result.WriteString(":\n")
	result.WriteString("----------------------------------------\n")

	found := false
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories if not doing recursive search
		if info.IsDir() && !recursive && path != absPath {
			return filepath.SkipDir
		}

		// Only search in regular files
		if info.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files that can't be read
		}

		lines := strings.Split(string(content), "\n")
		lineNum := 1

		for i, line := range lines {
			if strings.Contains(line, pattern) {
				found = true

				// Get relative path
				relPath, err := filepath.Rel(absPath, path)
				if err != nil {
					relPath = path
				}

				result.WriteString(fmt.Sprintf("%s:%d\n", relPath, lineNum))

				// Show context lines
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				end := i + contextLines + 1
				if end > len(lines) {
					end = len(lines)
				}

				for j := start; j < end; j++ {
					marker := "  "
					if j == i {
						marker = "▶ "
					}
					result.WriteString(fmt.Sprintf("%s%d: %s\n", marker, j+1, lines[j]))
				}
				result.WriteString("\n")
			}
			lineNum++
		}

		return nil
	}

	if recursive {
		err = filepath.Walk(absPath, walkFn)
	} else {
		// Check if it's a file or directory
		info, err := os.Stat(absPath)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Cannot access path: %v", err)), nil
		}

		if info.IsDir() {
			entries, err := os.ReadDir(absPath)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Cannot read directory: %v", err)), nil
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					filePath := filepath.Join(absPath, entry.Name())
					fileInfo, err := entry.Info()
					if err != nil {
						continue
					}
					walkFn(filePath, fileInfo, nil)
				}
			}
		} else {
			walkFn(absPath, info, nil)
		}
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Error during content search: %v", err)), nil
	}

	if !found {
		result.WriteString("No matches found for pattern: " + pattern)
	}

	return mcp.NewToolResultText(result.String()), nil
}

func deleteFileHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := mcp.ParseString(request, "path", "")
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	recursive := mcp.ParseBoolean(request, "recursive", false)

	// Resolve path relative to current working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return mcp.NewToolResultError(fmt.Sprintf("Path does not exist: %s", path)), nil
	}

	if info.IsDir() && !recursive {
		return mcp.NewToolResultError(fmt.Sprintf("Cannot delete directory '%s' without recursive=true", path)), nil
	}

	var deletedPaths []string

	if recursive && info.IsDir() {
		// Delete directory recursively
		err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if err := os.Remove(path); err != nil {
				return err
			}

			relPath, err := filepath.Rel(absPath, path)
			if err != nil {
				relPath = path
			}
			deletedPaths = append(deletedPaths, relPath)
			return nil
		})

		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete directory recursively: %v", err)), nil
		}
	} else {
		// Delete single file
		if err := os.Remove(absPath); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete: %v", err)), nil
		}

		relPath, err := filepath.Rel(".", absPath)
		if err != nil {
			relPath = path
		}
		deletedPaths = append(deletedPaths, relPath)
	}

	result := fmt.Sprintf("Successfully deleted %d item(s):\n", len(deletedPaths))
	result += "----------------------------------------\n"
	for _, deletedPath := range deletedPaths {
		result += deletedPath + "\n"
	}

	return mcp.NewToolResultText(result), nil
}

func readLineRangeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := mcp.ParseString(request, "path", "")
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	startLine := mcp.ParseInt(request, "start_line", 1)
	if startLine < 1 {
		return mcp.NewToolResultError("start_line must be >= 1"), nil
	}

	endLine := mcp.ParseInt(request, "end_line", startLine)
	if endLine < startLine {
		return mcp.NewToolResultError("end_line must be >= start_line"), nil
	}

	showLineNumbers := mcp.ParseBoolean(request, "show_line_numbers", true)

	// Resolve path relative to current working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	lines := strings.Split(string(content), "\n")

	// Adjust to 0-based indexing
	startIdx := startLine - 1
	endIdx := endLine

	// Handle bounds
	if startIdx >= len(lines) {
		return mcp.NewToolResultText(""), nil
	}

	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	resultLines := lines[startIdx:endIdx]

	var result string
	if showLineNumbers {
		var formattedLines []string
		for i, line := range resultLines {
			lineNum := startIdx + i + 1
			formattedLines = append(formattedLines, fmt.Sprintf("%4d: %s", lineNum, line))
		}
		result = strings.Join(formattedLines, "\n")
	} else {
		result = strings.Join(resultLines, "\n")
	}

	return mcp.NewToolResultText(result), nil
}

func editLineRangeHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := mcp.ParseString(request, "path", "")
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	startLine := mcp.ParseInt(request, "start_line", 1)
	if startLine < 1 {
		return mcp.NewToolResultError("start_line must be >= 1"), nil
	}

	endLine := mcp.ParseInt(request, "end_line", startLine)
	if endLine < startLine {
		return mcp.NewToolResultError("end_line must be >= start_line"), nil
	}

	content := mcp.ParseString(request, "content", "")
	if content == "" {
		return mcp.NewToolResultError("content parameter is required"), nil
	}

	operation := mcp.ParseString(request, "operation", "replace")

	// Resolve path relative to current working directory
	absPath, err := filepath.Abs(path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid path: %v", err)), nil
	}

	// Read current file content
	currentContent, err := os.ReadFile(absPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read file: %v", err)), nil
	}

	lines := strings.Split(string(currentContent), "\n")
	newContentLines := strings.Split(content, "\n")

	// Adjust to 0-based indexing
	startIdx := startLine - 1
	endIdx := endLine

	// Handle bounds
	if startIdx > len(lines) {
		return mcp.NewToolResultError(fmt.Sprintf("start_line %d exceeds file length (%d lines)", startLine, len(lines))), nil
	}

	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	var resultLines []string

	switch operation {
	case "replace":
		// Replace the specified range with new content
		resultLines = append(resultLines, lines[:startIdx]...)
		resultLines = append(resultLines, newContentLines...)
		resultLines = append(resultLines, lines[endIdx:]...)

	case "insert_before":
		// Insert new content before the specified line
		resultLines = append(resultLines, lines[:startIdx]...)
		resultLines = append(resultLines, newContentLines...)
		resultLines = append(resultLines, lines[startIdx:]...)

	case "insert_after":
		// Insert new content after the specified line (or range)
		resultLines = append(resultLines, lines[:endIdx]...)
		resultLines = append(resultLines, newContentLines...)
		resultLines = append(resultLines, lines[endIdx:]...)

	default:
		return mcp.NewToolResultError(fmt.Sprintf("Invalid operation: %s. Must be 'replace', 'insert_before', or 'insert_after'", operation)), nil
	}

	result := strings.Join(resultLines, "\n")

	// Ensure directory exists
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create directory: %v", err)), nil
	}

	err = os.WriteFile(absPath, []byte(result), 0644)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to write file: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully edited lines %d-%d in %s using operation '%s'", startLine, endLine, path, operation)), nil
}

func semanticSearchHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := mcp.ParseString(request, "query", "")
	if query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	collection := mcp.ParseString(request, "collection", "")
	if collection == "" {
		return mcp.NewToolResultError("collection parameter is required"), nil
	}

	limit := mcp.ParseInt(request, "limit", 2)
	if limit < 1 {
		limit = 1
	}
	if limit > 10 {
		limit = 10
	}

	// If collection is ".", resolve to current directory absolute path
	if collection == "." {
		currentDir, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get current directory: %v", err)), nil
		}
		collection = sanitizeCollectionName(currentDir)
	}

	// Create embedding for the query
	embedding, err := createEmbeddingForQuery(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create embedding for query: %v", err)), nil
	}

	// Create Qdrant client using JSON config or environment variables
	var qdrantHost string
	var qdrantPort int

	// First priority: JSON config file
	jsonConfig, err := loadJSONConfig()
	if err == nil {
		qdrantHost = jsonConfig.Indexer.Qdrant.Host
		if jsonConfig.Indexer.Qdrant.Port != "" {
			if port, err := strconv.Atoi(jsonConfig.Indexer.Qdrant.Port); err == nil {
				qdrantPort = port
			}
		}
	}

	// Second priority: environment variables (can override JSON config)
	if qdrantHost == "" {
		qdrantHost = getEnv("QDRANT_HOST", "localhost")
	}
	if qdrantPort == 0 {
		qdrantPort = getEnvAsInt("QDRANT_PORT", 6334)
	}

	qdrantClient, err := qdrant.NewClient(&qdrant.Config{
		Host: qdrantHost,
		Port: qdrantPort,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to Qdrant: %v", err)), nil
	}

	// Perform semantic search
	limitUint64 := uint64(limit)
	results, err := qdrantClient.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQuery(embedding...),
		WithPayload:    qdrant.NewWithPayload(true),
		Limit:          &limitUint64,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to perform semantic search: %v", err)), nil
	}

	// Format results
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Semantic search results for query: '%s'\n", query))
	result.WriteString(fmt.Sprintf("Collection: %s\n", collection))
	result.WriteString("----------------------------------------\n\n")

	if len(results) == 0 {
		result.WriteString("No results found.")
		return mcp.NewToolResultText(result.String()), nil
	}

	for i, point := range results {
		result.WriteString(fmt.Sprintf("Result %d (Score: %.4f):\n", i+1, point.Score))

		// Extract payload information
		if point.Payload != nil {
			// Access payload values using the proper Qdrant value extraction
			if filenameVal, ok := point.Payload["filename"]; ok && filenameVal != nil {
				if filenameStr := filenameVal.GetStringValue(); filenameStr != "" {
					result.WriteString(fmt.Sprintf("  File: %s\n", filenameStr))
				}
			}

			var startLineValue, endLineValue float64
			if startLineVal, ok := point.Payload["start_line"]; ok && startLineVal != nil {
				startLineValue = startLineVal.GetDoubleValue()
			}
			if endLineVal, ok := point.Payload["end_line"]; ok && endLineVal != nil {
				endLineValue = endLineVal.GetDoubleValue()
			}

			if startLineValue > 0 {
				endLine := startLineValue
				if endLineValue > 0 {
					endLine = endLineValue
				}
				result.WriteString(fmt.Sprintf("  Lines: %.0f-%.0f\n", startLineValue, endLine))
			}

			if summaryVal, ok := point.Payload["summary"]; ok && summaryVal != nil {
				if summaryStr := summaryVal.GetStringValue(); summaryStr != "" {
					result.WriteString(fmt.Sprintf("  Summary: %s\n", summaryStr))
				}
			}
		}
		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// sanitizeCollectionName creates a valid collection name from a path (similar to indexer package)
func sanitizeCollectionName(path string) string {
	name := strings.ReplaceAll(path, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return "codebase" + name
}

// createEmbeddingForQuery creates an embedding for the search query using JSON config or environment credentials
func createEmbeddingForQuery(ctx context.Context, query string) ([]float32, error) {
	// Find and load .env file by walking up directory tree (for backward compatibility)
	loadEnvFileFromProjectRoot()

	// Initialize variables with defaults
	var embeddingEndpoint, embeddingAPIKey, embeddingModel string
	var vectorDimensions int

	// First priority: JSON config file
	jsonConfig, err := loadJSONConfig()
	if err == nil {
		embeddingEndpoint = jsonConfig.Indexer.Embedding.BaseURL
		embeddingAPIKey = jsonConfig.Indexer.Embedding.APIKey
		embeddingModel = jsonConfig.Indexer.Embedding.Model
		if jsonConfig.Indexer.VectorDimensions != "" {
			if dims, err := strconv.Atoi(jsonConfig.Indexer.VectorDimensions); err == nil {
				vectorDimensions = dims
			}
		}
	}

	// Second priority: environment variables (can override JSON config)
	if embeddingEndpoint == "" {
		embeddingEndpoint = getEnv("EMBEDDING_BASE_URL", "")
	}
	if embeddingAPIKey == "" {
		embeddingAPIKey = getEnv("EMBEDDING_API_KEY", "")
	}
	if embeddingModel == "" {
		embeddingModel = getEnv("EMBEDDING_MODEL", "text-embedding-3-small")
	}
	if vectorDimensions == 0 {
		vectorDimensions = getEnvAsInt("VECTOR_DIMENSIONS", 1536)
	}

	if embeddingEndpoint == "" {
		return nil, fmt.Errorf("EMBEDDING_BASE_URL not set in config file or environment")
	}

	if embeddingAPIKey == "" {
		return nil, fmt.Errorf("EMBEDDING_API_KEY not set in config file or environment")
	}

	embeddingClient := openai.NewClient(
		option.WithBaseURL(embeddingEndpoint),
		option.WithAPIKey(embeddingAPIKey),
	)

	embedding, err := embeddingClient.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(query),
		},
		Model:          openai.EmbeddingModel(embeddingModel),
		Dimensions:     openai.Int(int64(vectorDimensions)),
		EncodingFormat: openai.EmbeddingNewParamsEncodingFormatFloat,
	})

	if err != nil {
		return nil, err
	}

	embeddingData := embedding.Data[0].Embedding
	embedding32 := make([]float32, len(embeddingData))
	for i, v := range embeddingData {
		embedding32[i] = float32(v)
	}

	return embedding32, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as integer with a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// loadEnvFileFromProjectRoot finds and loads .env file by walking up directory tree
func loadEnvFileFromProjectRoot() {
	// Start from current directory and walk up until we find .env or reach root
	dir, err := os.Getwd()
	if err != nil {
		return
	}

	for {
		envPath := filepath.Join(dir, ".env")
		loadEnvFile(envPath)

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root directory
			break
		}
		dir = parent
	}
}

// loadEnvFile loads environment variables from a .env file
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		// .env file doesn't exist, that's okay
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// Remove quotes if present
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}

			os.Setenv(key, value)
		}
	}
}
