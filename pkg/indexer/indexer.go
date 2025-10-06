package indexer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/pterm/pterm"
	"github.com/qdrant/go-client/qdrant"
)

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

// IndexDirectory indexes an entire directory
func (idx *Indexer) IndexDirectory(ctx context.Context, dirPath string) error {
	// Create collection name from absolute path
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	collectionName := idx.sanitizeCollectionName(absPath)
	pterm.FgLightBlue.Printf("Collection name: %s\n", collectionName)

	// Create or verify collection
	if err := idx.ensureCollection(ctx, collectionName); err != nil {
		return err
	}

	// Discover files
	filesToProcess, err := idx.scanner.ScanDirectory(dirPath)
	if err != nil {
		return err
	}

	pterm.FgLightBlue.Printf("\n📊 Found %d code files to index\n\n", len(filesToProcess))

	// Process files with progress bar
	return idx.processFiles(ctx, filesToProcess, collectionName, dirPath)
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
		pterm.FgLightYellow.Println("⚠️  Collection already exists. Re-indexing will add new chunks.")
		return nil
	}

	pterm.FgLightBlue.Println("Creating new collection...")
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

	pterm.FgLightGreen.Println("✅ Collection created successfully")
	return nil
}

// processFiles processes all discovered files
func (idx *Indexer) processFiles(ctx context.Context, files []string, collectionName, basePath string) error {
	totalChunks := 0
	progressBar, _ := pterm.DefaultProgressbar.WithTotal(len(files)).WithTitle("Indexing files").Start()

	for _, filePath := range files {
		relPath, _ := filepath.Rel(basePath, filePath)

		// Chunk the file
		chunks, err := ChunkFile(filePath, idx.config.ChunkSize, idx.config.ChunkOverlap)
		if err != nil {
			pterm.FgLightRed.Printf("⚠️  Failed to chunk %s: %v\n", relPath, err)
			progressBar.Increment()
			continue
		}

		// Process each chunk
		for _, chunk := range chunks {
			if err := idx.processChunk(ctx, chunk, relPath, collectionName); err != nil {
				pterm.FgLightRed.Printf("⚠️  Failed to process %s:%d-%d: %v\n",
					relPath, chunk.StartLine, chunk.EndLine, err)
				continue
			}
			totalChunks++
		}

		progressBar.Increment()
	}

	progressBar.Stop()
	pterm.FgLightGreen.Printf("\n✅ Indexing complete! Processed %d chunks from %d files\n",
		totalChunks, len(files))
	pterm.FgLightBlue.Printf("📦 Collection: %s\n", collectionName)

	return nil
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

	// Upsert to Qdrant
	_, err = idx.qdrantClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDUUID(uuid.NewString()),
				Vectors: qdrant.NewVectors(embedding...),
				Payload: qdrant.NewValueMap(map[string]any{
					"filename":   relPath,
					"start_line": chunk.StartLine,
					"end_line":   chunk.EndLine,
					"summary":    summary,
				}),
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
