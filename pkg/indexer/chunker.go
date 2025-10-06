package indexer

import (
	"fmt"
	"os"
	"strings"
)

// FileChunk represents a chunk of a file with metadata
type FileChunk struct {
	FilePath  string
	StartLine int
	EndLine   int
	Content   string
}

// ChunkFile breaks a file into chunks of specified size with overlap
func ChunkFile(filePath string, chunkSize, overlap int) ([]FileChunk, error) {
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
