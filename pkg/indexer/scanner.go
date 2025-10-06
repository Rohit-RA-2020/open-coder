package indexer

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileScanner handles file discovery
type FileScanner struct {
	config    *Config
	gitignore map[string][]*regexp.Regexp // Path -> compiled patterns
}

// NewFileScanner creates a new file scanner
func NewFileScanner(config *Config) *FileScanner {
	return &FileScanner{
		config:    config,
		gitignore: make(map[string][]*regexp.Regexp),
	}
}

// ScanDirectory scans a directory for code files
func (s *FileScanner) ScanDirectory(rootPath string) ([]string, error) {
	var filesToProcess []string

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if d.IsDir() {
			name := d.Name()
			if s.shouldIgnoreDir(name) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches ignore patterns
		if s.shouldIgnoreFile(d.Name()) {
			return nil
		}

		// Check .gitignore patterns
		if s.matchesGitignore(path, rootPath) {
			return nil
		}

		// Check file size limit (1MB)
		if s.isFileTooLarge(path) {
			return nil
		}

		// Check if file is binary/image
		if s.isBinaryFile(path) {
			return nil
		}

		// Only process code files
		ext := filepath.Ext(path)
		if s.isCodeFile(ext) {
			filesToProcess = append(filesToProcess, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return filesToProcess, nil
}

// shouldIgnoreDir checks if a directory should be ignored
func (s *FileScanner) shouldIgnoreDir(dirName string) bool {
	// Skip hidden directories
	if strings.HasPrefix(dirName, ".") {
		for _, ignoreDir := range s.config.IgnoreDirs {
			if !strings.HasPrefix(ignoreDir, ".") {
				// Only skip if it's in the ignore list
				continue
			}
			if dirName == ignoreDir {
				return true
			}
		}
		// Skip all other hidden dirs not explicitly in ignore list
		if dirName != "." {
			return true
		}
	}

	// Check against ignore list
	for _, ignoreDir := range s.config.IgnoreDirs {
		if dirName == ignoreDir {
			return true
		}
	}

	return false
}

// shouldIgnoreFile checks if a file should be ignored based on patterns
func (s *FileScanner) shouldIgnoreFile(filename string) bool {
	for _, pattern := range s.config.IgnorePatterns {
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// isCodeFile checks if a file extension is a code file
func (s *FileScanner) isCodeFile(ext string) bool {
	for _, codeExt := range s.config.CodeExtensions {
		if ext == codeExt {
			return true
		}
	}
	return false
}

// isFileTooLarge checks if a file exceeds the size limit (1MB)
func (s *FileScanner) isFileTooLarge(filePath string) bool {
	const maxSize = 1024 * 1024 // 1MB

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return true // Skip files we can't stat
	}

	return fileInfo.Size() > maxSize
}

// isBinaryFile checks if a file is binary by examining its content
func (s *FileScanner) isBinaryFile(filePath string) bool {
	// Check by extension first (faster)
	ext := strings.ToLower(filepath.Ext(filePath))
	binaryExtensions := []string{
		// Images
		".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp", ".svg", ".ico",
		// Videos
		".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".mkv", ".m4v",
		// Audio
		".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a",
		// Archives
		".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz", ".lz4",
		// Executables
		".exe", ".dll", ".so", ".dylib", ".bin", ".app", ".deb", ".rpm",
		// Documents
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		// Other binary formats
		".db", ".sqlite", ".sqlite3", ".dat", ".bin", ".obj", ".class",
		".jar", ".war", ".ear", ".pyc", ".pyo", ".o", ".a", ".lib",
	}

	for _, binaryExt := range binaryExtensions {
		if ext == binaryExt {
			return true
		}
	}

	// Check file content for binary data
	file, err := os.Open(filePath)
	if err != nil {
		return true // Skip files we can't open
	}
	defer file.Close()

	// Read first 512 bytes to check for null bytes
	reader := bufio.NewReader(file)
	buffer := make([]byte, 512)
	n, err := reader.Read(buffer)
	if err != nil && n == 0 {
		return true // Skip files we can't read
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			return true
		}
	}

	return false
}

// matchesGitignore checks if a file path matches any .gitignore patterns
func (s *FileScanner) matchesGitignore(filePath, rootPath string) bool {
	// Walk up the directory tree to find .gitignore files
	dir := filepath.Dir(filePath)
	relPath, err := filepath.Rel(rootPath, filePath)
	if err != nil {
		return false
	}

	for {
		gitignorePath := filepath.Join(dir, ".gitignore")
		if _, err := os.Stat(gitignorePath); err == nil {
			patterns := s.loadGitignorePatterns(gitignorePath, rootPath)
			for _, pattern := range patterns {
				if pattern.MatchString(relPath) || pattern.MatchString(filepath.Base(filePath)) {
					return true
				}
			}
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir || parent == rootPath {
			break
		}
		dir = parent
	}

	return false
}

// loadGitignorePatterns loads and compiles .gitignore patterns
func (s *FileScanner) loadGitignorePatterns(gitignorePath, rootPath string) []*regexp.Regexp {
	// Check if already loaded
	if patterns, exists := s.gitignore[gitignorePath]; exists {
		return patterns
	}

	file, err := os.Open(gitignorePath)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []*regexp.Regexp
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Convert gitignore pattern to regex
		pattern := s.gitignoreToRegex(line)
		if pattern != "" {
			if regex, err := regexp.Compile(pattern); err == nil {
				patterns = append(patterns, regex)
			}
		}
	}

	// Cache the patterns
	s.gitignore[gitignorePath] = patterns
	return patterns
}

// gitignoreToRegex converts a gitignore pattern to a regex pattern
func (s *FileScanner) gitignoreToRegex(pattern string) string {
	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		pattern = pattern[:len(pattern)-1]
		pattern = regexp.QuoteMeta(pattern) + ".*"
		return pattern
	}

	// Handle wildcards
	if strings.Contains(pattern, "*") {
		// Convert * to .* and escape other special characters
		escaped := regexp.QuoteMeta(pattern)
		escaped = strings.ReplaceAll(escaped, "\\*", ".*")
		return escaped
	}

	// Handle exact matches
	if !strings.Contains(pattern, "/") {
		// Pattern without path separator matches any file with this name
		return regexp.QuoteMeta(pattern) + "$"
	}

	// Pattern with path separator - match from beginning
	return "^" + regexp.QuoteMeta(pattern)
}
