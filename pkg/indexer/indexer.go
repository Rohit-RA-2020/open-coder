package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/qdrant/go-client/qdrant"
)

// ProgressCallback is called during indexing to report progress
type ProgressCallback func(current, total int, filename string)

// IndexMarker stores metadata about indexed directories
type IndexMarker struct {
	IndexedAt      time.Time `json:"indexed_at"`
	CollectionName string    `json:"collection_name"`
	FilesProcessed int       `json:"files_processed"`
	ChunksCreated  int       `json:"chunks_created"`
	Version        string    `json:"version"`
}

// MarkerFileName is the hidden file used to track indexed directories
const MarkerFileName = ".open-coder-index"

// Indexer handles the codebase indexing process
type Indexer struct {
	config          *Config
	summaryClient   *openai.Client
	embeddingClient *openai.Client
	qdrantClient    *qdrant.Client
	scanner         *FileScanner
}

// NewIndexer creates a new indexer instance
func NewIndexer(config *Config) (*Indexer, error) {
	// Initialize summary client
	summaryClient := openai.NewClient(
		option.WithBaseURL(config.SummaryEndpoint),
		option.WithAPIKey(config.SummaryAPIKey),
	)

	// Initialize embedding client
	embeddingClient := openai.NewClient(
		option.WithBaseURL(config.EmbeddingEndpoint),
		option.WithAPIKey(config.EmbeddingAPIKey),
	)

	// Initialize Qdrant client
	qdrantClient, err := qdrant.NewClient(&qdrant.Config{
		Host: config.QdrantHost,
		Port: config.QdrantPort,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	return &Indexer{
		config:          config,
		summaryClient:   &summaryClient,
		embeddingClient: &embeddingClient,
		qdrantClient:    qdrantClient,
		scanner:         NewFileScanner(config),
	}, nil
}

// IsDirectoryIndexed checks if a directory has already been indexed
func IsDirectoryIndexed(dirPath string) (*IndexMarker, bool) {
	markerPath := filepath.Join(dirPath, MarkerFileName)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return nil, false
	}

	var marker IndexMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, false
	}

	return &marker, true
}

// writeIndexMarker writes the index marker file
func writeIndexMarker(dirPath string, marker *IndexMarker) error {
	markerPath := filepath.Join(dirPath, MarkerFileName)
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(markerPath, data, 0600) // Only readable by owner
}

// IndexDirectory indexes an entire directory (no progress callback)
func (idx *Indexer) IndexDirectory(ctx context.Context, dirPath string) error {
	_, _, err := idx.IndexDirectoryWithProgress(ctx, dirPath, nil)
	return err
}

// IndexDirectoryWithProgress indexes a directory with progress callback
func (idx *Indexer) IndexDirectoryWithProgress(ctx context.Context, dirPath string, onProgress ProgressCallback) (int, int, error) {
	// Create collection name from absolute path
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	collectionName := idx.sanitizeCollectionName(absPath)

	// Create or verify collection
	if err := idx.ensureCollection(ctx, collectionName); err != nil {
		return 0, 0, err
	}

	// Discover files
	filesToProcess, err := idx.scanner.ScanDirectory(dirPath)
	if err != nil {
		return 0, 0, err
	}

	// Process files with callback
	totalChunks, err := idx.processFilesWithProgress(ctx, filesToProcess, collectionName, dirPath, onProgress)
	if err != nil {
		return len(filesToProcess), totalChunks, err
	}

	// Write index marker
	marker := &IndexMarker{
		IndexedAt:      time.Now(),
		CollectionName: collectionName,
		FilesProcessed: len(filesToProcess),
		ChunksCreated:  totalChunks,
		Version:        "2.0.0",
	}
	_ = writeIndexMarker(absPath, marker)

	return len(filesToProcess), totalChunks, nil
}

// sanitizeCollectionName creates a valid collection name from a path
func (idx *Indexer) sanitizeCollectionName(path string) string {
	name := strings.ReplaceAll(path, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ":", "_")
	return "codebase" + name
}

// ensureCollection creates a collection if it doesn't exist
func (idx *Indexer) ensureCollection(ctx context.Context, collectionName string) error {
	exists, err := idx.qdrantClient.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}

	if exists {
		return nil // Collection already exists
	}

	err = idx.qdrantClient.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     uint64(idx.config.VectorDimensions),
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}

// processFilesWithProgress processes all discovered files with progress callback
func (idx *Indexer) processFilesWithProgress(ctx context.Context, files []string, collectionName, basePath string, onProgress ProgressCallback) (int, error) {
	totalChunks := 0

	for i, filePath := range files {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return totalChunks, ctx.Err()
		default:
		}

		relPath, _ := filepath.Rel(basePath, filePath)

		// Call progress callback
		if onProgress != nil {
			onProgress(i+1, len(files), relPath)
		}

		// Chunk the file (uses AST-based or line-based chunking based on config)
		chunks, err := ChunkFile(filePath, idx.config)
		if err != nil {
			continue // Skip files that can't be chunked
		}

		// Process each chunk
		for _, chunk := range chunks {
			if err := idx.processChunk(ctx, chunk, relPath, collectionName); err != nil {
				continue // Skip chunks that fail
			}
			totalChunks++
		}
	}

	return totalChunks, nil
}

// processChunk processes a single file chunk
func (idx *Indexer) processChunk(ctx context.Context, chunk FileChunk, relPath, collectionName string) error {
	// Generate summary
	summary, err := idx.generateSummary(ctx, chunk.Content, relPath)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Generate embedding
	embedding, err := idx.generateEmbedding(ctx, summary)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Build payload with base fields
	payload := map[string]any{
		"filename":   relPath,
		"start_line": chunk.StartLine,
		"end_line":   chunk.EndLine,
		"summary":    summary,
	}

	// Add AST metadata if available
	if chunk.Symbol != "" {
		payload["symbol"] = chunk.Symbol
	}
	if chunk.Kind != "" {
		payload["kind"] = chunk.Kind
	}
	if chunk.Parent != "" {
		payload["parent"] = chunk.Parent
	}
	if chunk.Language != "" {
		payload["language"] = chunk.Language
	}
	if chunk.Part != "" {
		payload["part"] = chunk.Part
	}

	// Upsert to Qdrant
	_, err = idx.qdrantClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDUUID(uuid.NewString()),
				Vectors: qdrant.NewVectors(embedding...),
				Payload: qdrant.NewValueMap(payload),
			},
		},
	})

	return err
}

// generateSummary creates a summary with code explanation and pseudo code
func (idx *Indexer) generateSummary(ctx context.Context, code string, filename string) (string, error) {
	prompt := fmt.Sprintf(`Analyze the following code from file "%s" and provide:
1. A brief description of what the code does
2. Key functions/classes/structures
3. Pseudo code representation

Code:
%s

Provide a concise summary suitable for semantic search.`, filename, code)

	response, err := idx.summaryClient.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
		Model: openai.ChatModel(idx.config.SummaryModel),
	})

	if err != nil {
		return "", err
	}

	return response.Choices[0].Message.Content, nil
}

// generateEmbedding creates vector embeddings for text
func (idx *Indexer) generateEmbedding(ctx context.Context, text string) ([]float32, error) {
	embedding, err := idx.embeddingClient.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfString: openai.String(text),
		},
		Model:          openai.EmbeddingModel(idx.config.EmbeddingModel),
		Dimensions:     openai.Int(int64(idx.config.VectorDimensions)),
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
