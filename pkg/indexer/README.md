# Indexer Package

A modular codebase indexing package for semantic code search using vector embeddings.

## Overview

This package provides functionality to:
- Scan and discover code files in a directory
- Automatically exclude binary files, images, and large files (>1MB)
- Respect .gitignore patterns for intelligent filtering
- Break files into overlapping chunks for better context
- Generate AI-powered summaries of code chunks
- Create vector embeddings for semantic search
- Store everything in Qdrant vector database

## Structure

```
pkg/indexer/
├── config.go       # Configuration management
├── scanner.go      # File discovery and filtering
├── chunker.go      # File chunking logic
├── indexer.go      # Main indexing orchestration
└── README.md       # This file
```

## Usage

### Basic Usage

```go
import "open-coder/pkg/indexer"

// Create configuration from JSON config file, environment variables, and .env file
config := indexer.LoadConfigFromEnv()

// Create indexer
idx, err := indexer.NewIndexer(config)
if err != nil {
    log.Fatal(err)
}

// Index a directory
err = idx.IndexDirectory(context.Background(), "/path/to/code")
```

### Configuration Loading

The indexer automatically loads configuration with the following priority (highest to lowest):

1. **JSON Config File** (`~/.open-coder/config`) - Primary configuration source
2. **Environment Variables** - Can override JSON config values
3. **`.env` File** - For backward compatibility
4. **Default Values** - Built-in defaults

The JSON config file is automatically created by the installation script when you have a `.env` file in your project directory, or you can create it manually.

The JSON config file should have this structure:

```json
{
  "indexer": {
    "embedding": {
      "base_url": "https://your-embedding-endpoint.openai.azure.com/openai/v1",
      "api_key": "your-embedding-api-key-here",
      "model": "text-embedding-3-small"
    },
    "summary": {
      "base_url": "https://your-summary-endpoint.openai.azure.com/openai/v1",
      "api_key": "your-summary-api-key-here",
      "model": "gpt-4o-mini"
    },
    "qdrant": {
      "host": "localhost",
      "port": "6334"
    },
    "chunking": {
      "size": "100",
      "overlap": "10"
    },
    "vector_dimensions": "1536"
  }
}
```

### Custom Configuration

```go
config := indexer.LoadConfigFromEnv()

// Modify chunking settings
config.ChunkSize = 150
config.ChunkOverlap = 20

// Add custom file extensions
config.AddCodeExtension(".sol")  // Solidity
config.AddCodeExtension(".ex")   // Elixir

// Add directories to ignore
config.AddIgnoreDir("generated")
config.AddIgnoreDir("migrations")

// Add file patterns to ignore
config.AddIgnorePattern("*.test.js")
config.AddIgnorePattern("*.spec.ts")

idx, err := indexer.NewIndexer(config)
```

## Configuration Options

### Chunking Settings
- **ChunkSize**: Number of lines per chunk (env: `CHUNK_SIZE`, default: 100)
- **ChunkOverlap**: Overlapping lines between chunks (env: `CHUNK_OVERLAP`, default: 10)

### File Filtering
- **CodeExtensions**: File extensions to index (default: common programming languages)
- **IgnoreDirs**: Directories to skip (default: node_modules, vendor, etc.)
- **IgnorePatterns**: File patterns to ignore (default: *.min.js, *.lock, etc.)

### Azure OpenAI Settings
- **SummaryEndpoint**: Endpoint for summary generation (env: `SUMMARY_BASE_URL`)
- **SummaryAPIKey**: API key for summary service (env: `SUMMARY_API_KEY`)
- **SummaryModel**: Model to use for summaries (env: `SUMMARY_MODEL`, default: gpt-4o-mini)
- **EmbeddingEndpoint**: Endpoint for embeddings (env: `EMBEDDING_BASE_URL`)
- **EmbeddingAPIKey**: API key for embedding service (env: `EMBEDDING_API_KEY`)
- **EmbeddingModel**: Model for embeddings (env: `EMBEDDING_MODEL`, default: text-embedding-3-small)

### Qdrant Settings
- **QdrantHost**: Qdrant server host (env: `QDRANT_HOST`, default: localhost)
- **QdrantPort**: Qdrant server port (env: `QDRANT_PORT`, default: 6334)
- **VectorDimensions**: Embedding dimensions (env: `VECTOR_DIMENSIONS`, default: 1536)

## Default Ignored Directories

- `node_modules`, `vendor`, `__pycache__`
- `.git`, `.svn`, `.hg`
- `dist`, `build`, `target`
- `.next`, `.nuxt`, `.output`
- `venv`, `env`, `.env`

## Default Ignored Patterns

- `*.min.js`, `*.min.css`
- `*.map`, `*.lock`
- `package-lock.json`, `yarn.lock`
- `.DS_Store`
- **Binary files**: `*.jpg`, `*.png`, `*.gif`, `*.mp4`, `*.zip`, `*.exe`, etc.
- **Documents**: `*.pdf`, `*.doc`, `*.docx`, `*.xls`, `*.xlsx`, etc.
- **Archives**: `*.zip`, `*.rar`, `*.7z`, `*.tar`, `*.gz`, etc.
- **Executables**: `*.exe`, `*.dll`, `*.so`, `*.dylib`, `*.bin`, etc.

## Automatic Filtering

The indexer automatically excludes:

### Binary Files and Images
- Detects binary content by checking for null bytes
- Recognizes common binary extensions (images, videos, audio, archives, executables)
- Skips files that can't be processed as text

### Large Files
- Automatically skips files larger than 1MB
- Prevents memory issues and improves performance
- Configurable size limit in the future

### Git Repositories
- Skips `.git` directories and their contents
- Respects `.gitignore` patterns throughout the directory tree
- Supports nested `.gitignore` files

### Dependencies
- Excludes common dependency directories: `node_modules`, `vendor`, `__pycache__`
- Skips build artifacts: `dist`, `build`, `target`
- Ignores environment directories: `venv`, `env`, `.env`

### .gitignore Support
- Automatically loads and respects `.gitignore` patterns
- Supports wildcards (`*`), directory patterns (`dir/`), and exact matches
- Caches compiled patterns for performance
- Walks up the directory tree to find all relevant `.gitignore` files

## Supported Languages (by default)

Go, Python, JavaScript, TypeScript, JSX, TSX, Java, C, C++, Rust, Ruby, PHP, C#, Swift, Kotlin, Scala, R, MATLAB, Shell scripts

## Example: Custom Indexer for Specific Project

```go
// Create custom config for a React + TypeScript project
config := indexer.DefaultConfig()

// Only index specific files
config.CodeExtensions = []string{".ts", ".tsx", ".js", ".jsx"}

// Ignore test files
config.AddIgnorePattern("*.test.ts")
config.AddIgnorePattern("*.test.tsx")
config.AddIgnorePattern("*.spec.ts")

// Ignore build artifacts
config.AddIgnoreDir("dist")
config.AddIgnoreDir("coverage")

// Smaller chunks for detailed indexing
config.ChunkSize = 50
config.ChunkOverlap = 5

idx, _ := indexer.NewIndexer(config)
idx.IndexDirectory(ctx, "./my-react-app")
```

## How It Works

### 1. File Discovery (scanner.go)
- Recursively walks the directory tree
- Filters files by extension and patterns
- Skips ignored directories
- Excludes binary files and images automatically
- Respects .gitignore patterns
- Skips files larger than 1MB

### 2. File Chunking (chunker.go)
- Reads file content
- Splits into lines
- Creates overlapping chunks
- Example: Lines 0-100, 90-190, 180-280...

### 3. Summary Generation (indexer.go)
- Sends code chunk to Azure OpenAI
- Generates description, key functions, and pseudo code
- Optimized for semantic search

### 4. Embedding Creation (indexer.go)
- Converts summary to vector embedding
- Uses Azure OpenAI embedding model
- 1536-dimensional vectors

### 5. Vector Storage (indexer.go)
- Stores in Qdrant collection
- Collection named after directory path
- Includes metadata: filename, line range, summary

## Collection Naming

Collections are automatically named based on the absolute path:
- `/home/user/project` → `codebase_home_user_project`
- `C:\Users\Dev\App` → `codebase_C__Users_Dev_App`

## Error Handling

The indexer is resilient to errors:
- Individual file failures don't stop the process
- Errors are logged with pterm colors
- Progress bar shows overall completion
- Final summary shows success metrics

## Performance Tips

1. **Adjust Chunk Size**: Larger chunks = fewer API calls but less granular search
2. **Use Patterns**: Filter out unnecessary files early
3. **Batch Processing**: The indexer processes files sequentially with progress tracking
4. **Parallel Potential**: Could be extended for parallel processing

## Future Enhancements

- [ ] Parallel file processing
- [ ] Incremental indexing (only changed files)
- [ ] Custom summarization prompts
- [ ] Multiple embedding models
- [ ] Configuration file support (.indexer.yaml)
- [ ] Configurable file size limits
- [ ] Language-specific chunking strategies
- [ ] Advanced binary file detection (MIME type checking)

