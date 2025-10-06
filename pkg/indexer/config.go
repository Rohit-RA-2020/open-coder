package indexer

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// OpenCoderConfig represents the JSON configuration structure
type OpenCoderConfig struct {
	Indexer struct {
		Embedding struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
			Model   string `json:"model"`
		} `json:"embedding"`
		Summary struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
			Model   string `json:"model"`
		} `json:"summary"`
		Qdrant struct {
			Host string `json:"host"`
			Port string `json:"port"`
		} `json:"qdrant"`
		Chunking struct {
			Size    string `json:"size"`
			Overlap string `json:"overlap"`
		} `json:"chunking"`
		VectorDimensions string `json:"vector_dimensions"`
	} `json:"indexer"`
}

// Config holds the configuration for the indexer
type Config struct {
	// Chunking settings
	ChunkSize    int // Number of lines per chunk
	ChunkOverlap int // Number of overlapping lines between chunks

	// File filtering
	CodeExtensions []string // File extensions to index
	IgnoreDirs     []string // Directories to ignore
	IgnorePatterns []string // File patterns to ignore

	// Azure OpenAI endpoints
	SummaryEndpoint string
	SummaryAPIKey   string
	SummaryModel    string

	EmbeddingEndpoint string
	EmbeddingAPIKey   string
	EmbeddingModel    string

	// Qdrant settings
	QdrantHost string
	QdrantPort int

	// Vector settings
	VectorDimensions int
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		// Chunking settings
		ChunkSize:    100,
		ChunkOverlap: 10,

		// Common code file extensions
		CodeExtensions: []string{
			".go", ".py", ".js", ".ts", ".tsx", ".jsx",
			".java", ".c", ".cpp", ".h", ".hpp",
			".rs", ".rb", ".php", ".cs", ".swift", ".kt",
			".scala", ".r", ".m", ".sh", ".bash",
		},

		// Common directories to ignore
		IgnoreDirs: []string{
			"node_modules", "vendor", "__pycache__",
			".git", ".svn", ".hg",
			"dist", "build", "target",
			".next", ".nuxt", ".output",
			"venv", "env", ".env",
		},

		// File patterns to ignore
		IgnorePatterns: []string{
			"*.min.js", "*.min.css",
			"*.map", "*.lock",
			"package-lock.json", "yarn.lock",
			".DS_Store",
			// Binary and image files
			"*.jpg", "*.jpeg", "*.png", "*.gif", "*.bmp", "*.tiff", "*.webp", "*.ico",
			"*.mp4", "*.avi", "*.mov", "*.wmv", "*.flv", "*.webm", "*.mkv",
			"*.mp3", "*.wav", "*.flac", "*.aac", "*.ogg", "*.wma",
			"*.zip", "*.rar", "*.7z", "*.tar", "*.gz", "*.bz2",
			"*.exe", "*.dll", "*.so", "*.dylib", "*.bin", "*.app",
			"*.pdf", "*.doc", "*.docx", "*.xls", "*.xlsx", "*.ppt", "*.pptx",
			"*.db", "*.sqlite", "*.sqlite3", "*.dat", "*.obj", "*.class",
			"*.jar", "*.war", "*.ear", "*.pyc", "*.pyo", "*.o", "*.a", "*.lib",
		},

		// Azure OpenAI settings - Summary
		SummaryEndpoint: "",
		SummaryAPIKey:   "",
		SummaryModel:    "",

		// Azure OpenAI settings - Embeddings
		EmbeddingEndpoint: "",
		EmbeddingAPIKey:   "",
		EmbeddingModel:    "",

		// Qdrant settings
		QdrantHost: "localhost",
		QdrantPort: 6334,

		// Vector settings
		VectorDimensions: 1536,
	}
}

// AddCodeExtension adds a new code extension to the config
func (c *Config) AddCodeExtension(ext string) {
	c.CodeExtensions = append(c.CodeExtensions, ext)
}

// AddIgnoreDir adds a new directory to ignore
func (c *Config) AddIgnoreDir(dir string) {
	c.IgnoreDirs = append(c.IgnoreDirs, dir)
}

// AddIgnorePattern adds a new file pattern to ignore
func (c *Config) AddIgnorePattern(pattern string) {
	c.IgnorePatterns = append(c.IgnorePatterns, pattern)
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

// LoadConfigFromEnv loads configuration from JSON config file, environment variables, and .env file
func LoadConfigFromEnv() *Config {
	config := DefaultConfig()

	// First priority: JSON config file
	jsonConfig, err := loadJSONConfig()
	if err == nil {
		// Apply JSON config values
		if jsonConfig.Indexer.Embedding.BaseURL != "" {
			config.EmbeddingEndpoint = jsonConfig.Indexer.Embedding.BaseURL
		}
		if jsonConfig.Indexer.Embedding.APIKey != "" {
			config.EmbeddingAPIKey = jsonConfig.Indexer.Embedding.APIKey
		}
		if jsonConfig.Indexer.Embedding.Model != "" {
			config.EmbeddingModel = jsonConfig.Indexer.Embedding.Model
		}

		if jsonConfig.Indexer.Summary.BaseURL != "" {
			config.SummaryEndpoint = jsonConfig.Indexer.Summary.BaseURL
		}
		if jsonConfig.Indexer.Summary.APIKey != "" {
			config.SummaryAPIKey = jsonConfig.Indexer.Summary.APIKey
		}
		if jsonConfig.Indexer.Summary.Model != "" {
			config.SummaryModel = jsonConfig.Indexer.Summary.Model
		}

		if jsonConfig.Indexer.Qdrant.Host != "" {
			config.QdrantHost = jsonConfig.Indexer.Qdrant.Host
		}
		if jsonConfig.Indexer.Qdrant.Port != "" {
			if port, err := strconv.Atoi(jsonConfig.Indexer.Qdrant.Port); err == nil {
				config.QdrantPort = port
			}
		}

		if jsonConfig.Indexer.Chunking.Size != "" {
			if size, err := strconv.Atoi(jsonConfig.Indexer.Chunking.Size); err == nil {
				config.ChunkSize = size
			}
		}
		if jsonConfig.Indexer.Chunking.Overlap != "" {
			if overlap, err := strconv.Atoi(jsonConfig.Indexer.Chunking.Overlap); err == nil {
				config.ChunkOverlap = overlap
			}
		}

		if jsonConfig.Indexer.VectorDimensions != "" {
			if dims, err := strconv.Atoi(jsonConfig.Indexer.VectorDimensions); err == nil {
				config.VectorDimensions = dims
			}
		}
	}

	// Load .env file if it exists (for backward compatibility)
	loadEnvFile(".env")

	// Second priority: environment variables (can override JSON config)
	if val := os.Getenv("EMBEDDING_BASE_URL"); val != "" {
		config.EmbeddingEndpoint = val
	}
	if val := os.Getenv("EMBEDDING_API_KEY"); val != "" {
		config.EmbeddingAPIKey = val
	}
	if val := os.Getenv("EMBEDDING_MODEL"); val != "" {
		config.EmbeddingModel = val
	}
	if val := os.Getenv("SUMMARY_BASE_URL"); val != "" {
		config.SummaryEndpoint = val
	}
	if val := os.Getenv("SUMMARY_API_KEY"); val != "" {
		config.SummaryAPIKey = val
	}
	if val := os.Getenv("SUMMARY_MODEL"); val != "" {
		config.SummaryModel = val
	}
	if val := os.Getenv("QDRANT_HOST"); val != "" {
		config.QdrantHost = val
	}
	if val := os.Getenv("QDRANT_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.QdrantPort = port
		}
	}
	if val := os.Getenv("VECTOR_DIMENSIONS"); val != "" {
		if dims, err := strconv.Atoi(val); err == nil {
			config.VectorDimensions = dims
		}
	}
	if val := os.Getenv("CHUNK_SIZE"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			config.ChunkSize = size
		}
	}
	if val := os.Getenv("CHUNK_OVERLAP"); val != "" {
		if overlap, err := strconv.Atoi(val); err == nil {
			config.ChunkOverlap = overlap
		}
	}

	return config
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
