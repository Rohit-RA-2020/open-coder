package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileChunk represents a chunk of a file with metadata
type FileChunk struct {
	FilePath  string
	StartLine int
	EndLine   int
	Content   string
	// AST metadata (populated by AST chunker, empty for line-based)
	Symbol   string // e.g., "handleRequest", "UserService"
	Kind     string // "function", "class", "method", "interface", "const", "variable"
	Parent   string // e.g., "UserService" for methods
	Language string // "go", "javascript", "python"
	Part     string // e.g., "1/3" for split large nodes
}

// ChunkFileLines breaks a file into chunks of specified size with overlap (line-based)
// This is the fallback chunker for unsupported languages or when AST parsing fails
func ChunkFileLines(filePath string, chunkSize, overlap int) ([]FileChunk, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var chunks []FileChunk

	for start := 0; start < len(lines); start += (chunkSize - overlap) {
		end := start + chunkSize
		if end > len(lines) {
			end = len(lines)
		}

		chunkContent := strings.Join(lines[start:end], "\n")
		chunks = append(chunks, FileChunk{
			FilePath:  filePath,
			StartLine: start,
			EndLine:   end,
			Content:   chunkContent,
		})

		// If we've reached the end, break
		if end >= len(lines) {
			break
		}
	}

	return chunks, nil
}

// ChunkFile is the main entry point for file chunking.
// It routes to AST-based or line-based chunking based on config and file type.
// Falls back to line-based chunking if AST parsing fails.
func ChunkFile(filePath string, config *Config) ([]FileChunk, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	// Check if AST chunking is enabled and language is supported
	useAST := config.ChunkMode == "ast" && isASTLanguage(ext, config.ASTLanguages)

	if useAST {
		// Try AST-based chunking
		astChunker := NewASTChunker(config.MaxChunkLines, config.ChunkSize, config.ChunkOverlap)
		chunks, err := astChunker.ChunkFileAST(filePath)
		if err == nil && len(chunks) > 0 {
			return chunks, nil
		}
		// Fall back to line-based chunking on AST parsing failure
	}

	// Use line-based chunking as fallback or default
	return ChunkFileLines(filePath, config.ChunkSize, config.ChunkOverlap)
}

// isASTLanguage checks if the file extension is in the list of AST-supported languages
func isASTLanguage(ext string, astLanguages []string) bool {
	for _, lang := range astLanguages {
		if ext == lang {
			return true
		}
	}
	return false
}
