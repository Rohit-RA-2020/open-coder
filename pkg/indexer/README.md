# Indexer Package

A modular codebase indexing package for semantic code search using vector embeddings.

## Overview

This package provides functionality to:
- Scan and discover code files in a directory
- Automatically exclude binary files, images, and large files (>1MB)
- Respect .gitignore patterns for intelligent filtering
- **AST-based semantic chunking** for Go, JavaScript/TypeScript, and Python (extracts functions, classes, methods)
- Fallback to line-based overlapping chunks for unsupported languages
- Split large functions/classes into sub-chunks with overlap
- Generate AI-powered summaries of code chunks
- Create vector embeddings for semantic search
- Store everything in Qdrant vector database with rich metadata

## Structure

```
pkg/indexer/
├── config.go       # Configuration management
├── scanner.go      # File discovery and filtering
├── chunker.go      # File chunking router and line-based chunking
├── ast_chunker.go  # AST-based semantic chunking using tree-sitter
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
      "overlap": "10",
      "mode": "ast",
      "max_node_lines": "200"
    },
    "vector_dimensions": "1536",
    "ast_languages": [".go", ".js", ".jsx", ".ts", ".tsx", ".py"]
  }
}
```

### Custom Configuration

```go
config := indexer.LoadConfigFromEnv()

// Modify chunking settings
config.ChunkSize = 150
config.ChunkOverlap = 20

// AST chunking options
config.ChunkMode = "ast"           // or "lines" for line-based only
config.MaxChunkLines = 200         // Split functions larger than this
config.ASTLanguages = []string{".go", ".js", ".ts", ".py"}  // Languages for AST chunking

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
- **ChunkMode**: Chunking strategy - `"ast"` or `"lines"` (env: `CHUNK_MODE`, default: `"ast"`)
- **ChunkSize**: Number of lines per chunk for line-based mode and sub-chunk splitting (env: `CHUNK_SIZE`, default: 100)
- **ChunkOverlap**: Overlapping lines between chunks (env: `CHUNK_OVERLAP`, default: 10)
- **MaxChunkLines**: Max lines before splitting a large function/class into sub-chunks (env: `MAX_CHUNK_LINES`, default: 200)
- **ASTLanguages**: File extensions for AST-based chunking (default: `[".go", ".js", ".jsx", ".ts", ".tsx", ".py"]`)

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

// Use AST chunking (default) - extracts functions, classes, methods
config.ChunkMode = "ast"
config.MaxChunkLines = 150  // Split large components

// Ignore test files
config.AddIgnorePattern("*.test.ts")
config.AddIgnorePattern("*.test.tsx")
config.AddIgnorePattern("*.spec.ts")

// Ignore build artifacts
config.AddIgnoreDir("dist")
config.AddIgnoreDir("coverage")

// Smaller sub-chunks for split large functions
config.ChunkSize = 50
config.ChunkOverlap = 5

idx, _ := indexer.NewIndexer(config)
idx.IndexDirectory(ctx, "./my-react-app")
```

## Example: Force Line-Based Chunking

```go
// Disable AST chunking for legacy codebase
config := indexer.DefaultConfig()
config.ChunkMode = "lines"  // Force line-based chunking
config.ChunkSize = 100
config.ChunkOverlap = 10

idx, _ := indexer.NewIndexer(config)
idx.IndexDirectory(ctx, "./legacy-project")
```

## How It Works

### 1. File Discovery (scanner.go)
- Recursively walks the directory tree
- Filters files by extension and patterns
- Skips ignored directories
- Excludes binary files and images automatically
- Respects .gitignore patterns
- Skips files larger than 1MB

### 2. File Chunking (chunker.go + ast_chunker.go)

The indexer uses a smart chunking router that selects the best strategy:

#### AST-Based Chunking (Default for Go, JS/TS, Python)
Uses [tree-sitter](https://tree-sitter.github.io/) to parse code and extract semantic units:

- **Functions**: `function_declaration`, `method_declaration`, `arrow_function`
- **Classes**: `class_declaration`, `class_definition`
- **Methods**: `method_definition` (including nested methods inside classes)
- **Types**: `type_declaration`, `interface_declaration`, `type_alias_declaration`
- **Constants/Variables**: `const_declaration`, `var_declaration`, `variable_declaration`

**Benefits:**
- Chunks respect code structure (never splits mid-function)
- Captures symbol names, kinds, and parent relationships
- Attaches leading comments/docstrings to their declarations
- Large functions (>200 lines) are split into sub-chunks with overlap

**Chunk Metadata:**
- `Symbol`: Function/class name (e.g., "handleRequest", "UserService")
- `Kind`: Declaration type ("function", "class", "method", "interface", etc.)
- `Parent`: Parent class/receiver for methods
- `Language`: Source language ("go", "javascript", "typescript", "python")
- `Part`: Sub-chunk indicator (e.g., "1/3") for split large nodes

#### Line-Based Chunking (Fallback)
For unsupported languages or when AST parsing fails:
- Splits into fixed-size overlapping chunks
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
- Includes metadata: filename, line range, summary, symbol, kind, parent, language, part

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


## AST-Supported Languages

The following languages use semantic AST-based chunking:

| Language | Extensions | Extracted Symbols |
|----------|------------|-------------------|
| Go | `.go` | functions, methods, types, const, var |
| JavaScript | `.js`, `.jsx` | functions, classes, methods, variables, exports |
| TypeScript | `.ts`, `.tsx` | functions, classes, methods, interfaces, types, enums, variables |
| Python | `.py` | functions, classes, methods (including decorated) |

All other languages fall back to line-based chunking.

## Adding Support for New Languages

To add AST-based chunking for a new language (e.g., Rust, Ruby, Java):

### Step 1: Add the tree-sitter grammar dependency

```bash
go get github.com/smacker/go-tree-sitter/rust
```

Available grammars: [github.com/smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) (rust, ruby, java, c, cpp, c_sharp, php, etc.)

### Step 2: Update `ast_chunker.go`

**1. Add the import:**

```go
import (
    // ... existing imports
    "github.com/smacker/go-tree-sitter/rust"
)
```

**2. Register the language in `langMap`:**

```go
var langMap = map[string]*sitter.Language{
    // ... existing languages
    ".rs": rust.GetLanguage(),
}
```

**3. Add language name mapping in `langNameMap`:**

```go
var langNameMap = map[string]string{
    // ... existing mappings
    ".rs": "rust",
}
```

**4. Define declaration types in `declarationTypes`:**

```go
var declarationTypes = map[string]map[string]string{
    // ... existing languages
    "rust": {
        "function_item":     "function",
        "impl_item":         "impl",
        "struct_item":       "struct",
        "enum_item":         "enum",
        "trait_item":        "trait",
        "const_item":        "const",
        "static_item":       "variable",
        "mod_item":          "module",
    },
}
```

### Step 3: Update config defaults

In `config.go`, add the extension to `ASTLanguages`:

```go
ASTLanguages: []string{".go", ".js", ".jsx", ".ts", ".tsx", ".py", ".rs"},
```

### Finding Node Types

To discover the correct node types for a language:

1. **Use tree-sitter playground**: [tree-sitter.github.io/tree-sitter/playground](https://tree-sitter.github.io/tree-sitter/playground)
2. **Check grammar source**: Look at the grammar's `grammar.js` in the tree-sitter repo
3. **Print AST**: Add debug logging in `extractDeclarations` to see node types:
   ```go
   fmt.Printf("Node: %s at line %d\n", node.Type(), node.StartPoint().Row)
   ```

### Example: Adding Ruby Support

```go
// 1. Import
import "github.com/smacker/go-tree-sitter/ruby"

// 2. langMap
".rb": ruby.GetLanguage(),

// 3. langNameMap  
".rb": "ruby",

// 4. declarationTypes
"ruby": {
    "method":           "method",
    "singleton_method": "method",
    "class":            "class",
    "module":           "module",
    "assignment":       "variable",
},
```

Then run `go mod tidy` and rebuild.