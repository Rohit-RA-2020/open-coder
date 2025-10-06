package indexer

// This file contains example configurations for different use cases

// PythonProjectConfig returns a config optimized for Python projects
func PythonProjectConfig() *Config {
	config := LoadConfigFromEnv()

	// Only Python files
	config.CodeExtensions = []string{".py"}

	// Add Python-specific ignores
	config.AddIgnoreDir("__pycache__")
	config.AddIgnoreDir(".pytest_cache")
	config.AddIgnoreDir("venv")
	config.AddIgnoreDir("env")
	config.AddIgnoreDir(".venv")
	config.AddIgnoreDir("htmlcov")

	config.AddIgnorePattern("*.pyc")
	config.AddIgnorePattern("*.pyo")
	config.AddIgnorePattern("*.pyd")

	return config
}

// JavaScriptProjectConfig returns a config optimized for JavaScript/TypeScript projects
func JavaScriptProjectConfig() *Config {
	config := LoadConfigFromEnv()

	// JS/TS files only
	config.CodeExtensions = []string{".js", ".jsx", ".ts", ".tsx", ".mjs"}

	// Add JS-specific ignores
	config.AddIgnoreDir("node_modules")
	config.AddIgnoreDir("dist")
	config.AddIgnoreDir("build")
	config.AddIgnoreDir(".next")
	config.AddIgnoreDir(".nuxt")
	config.AddIgnoreDir("coverage")

	config.AddIgnorePattern("*.min.js")
	config.AddIgnorePattern("*.bundle.js")
	config.AddIgnorePattern("*.test.js")
	config.AddIgnorePattern("*.spec.ts")

	return config
}

// GoProjectConfig returns a config optimized for Go projects
func GoProjectConfig() *Config {
	config := LoadConfigFromEnv()

	// Only Go files
	config.CodeExtensions = []string{".go"}

	// Add Go-specific ignores
	config.AddIgnoreDir("vendor")
	config.AddIgnorePattern("*_test.go")

	return config
}

// WebProjectConfig returns a config for full-stack web projects
func WebProjectConfig() *Config {
	config := LoadConfigFromEnv()

	// Web languages
	config.CodeExtensions = []string{
		".js", ".jsx", ".ts", ".tsx",
		".py", ".go", ".java",
		".html", ".css", ".scss", ".sass",
		".vue", ".svelte",
	}

	// Common web ignores
	config.AddIgnoreDir("node_modules")
	config.AddIgnoreDir("dist")
	config.AddIgnoreDir("build")
	config.AddIgnoreDir("public")
	config.AddIgnoreDir("static")

	return config
}

// LargeCodebaseConfig returns a config optimized for large codebases
func LargeCodebaseConfig() *Config {
	config := LoadConfigFromEnv()

	// Larger chunks to reduce API calls
	config.ChunkSize = 200
	config.ChunkOverlap = 20

	// More aggressive filtering
	config.AddIgnorePattern("*.test.*")
	config.AddIgnorePattern("*.spec.*")
	config.AddIgnorePattern("*.mock.*")

	return config
}

// DetailedIndexConfig returns a config for detailed, granular indexing
func DetailedIndexConfig() *Config {
	config := LoadConfigFromEnv()

	// Smaller chunks for more detailed search
	config.ChunkSize = 50
	config.ChunkOverlap = 10

	return config
}
