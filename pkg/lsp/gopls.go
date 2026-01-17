package lsp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GoClient is a specialized LSP client for Go using gopls
type GoClient struct {
	*Client
}

// NewGoClient creates a new Go LSP client using gopls
func NewGoClient(rootPath string) (*GoClient, error) {
	// Check if gopls is available
	goplsPath, err := exec.LookPath("gopls")
	if err != nil {
		// Try looking in default GOPATH bin
		home, err := os.UserHomeDir()
		if err == nil {
			possiblePath := filepath.Join(home, "go", "bin", "gopls")
			if _, err := os.Stat(possiblePath); err == nil {
				goplsPath = possiblePath
			}
		}
	}

	if goplsPath == "" {
		return nil, fmt.Errorf("gopls not found in PATH or ~/go/bin. Install with: go install golang.org/x/tools/gopls@latest")
	}

	client, err := NewClient(goplsPath, []string{"-remote=auto", "-logfile=auto"}, rootPath)
	if err != nil {
		return nil, err
	}

	goClient := &GoClient{Client: client}

	// Initialize the LSP connection
	if err := client.Initialize(); err != nil {
		client.Close()
		return nil, err
	}

	return goClient, nil
}

// OpenGoFile opens a Go file and notifies gopls
func (c *GoClient) OpenGoFile(filePath, content string) error {
	uri := FilePathToURI(filePath)
	return c.DidOpen(uri, "go", content)
}

// GoToDefinition gets the definition location for a symbol
func (c *GoClient) GoToDefinition(filePath string, line, character int) (*Location, error) {
	uri := FilePathToURI(filePath)
	locations, err := c.Definition(uri, line, character)
	if err != nil {
		return nil, err
	}

	if len(locations) == 0 {
		return nil, nil
	}

	return &locations[0], nil
}

// GetHoverInfo gets hover information at a position
func (c *GoClient) GetHoverInfo(filePath string, line, character int) (string, error) {
	uri := FilePathToURI(filePath)
	return c.Hover(uri, line, character)
}

// GetFileDiagnostics returns diagnostics for a specific file
func (c *GoClient) GetFileDiagnostics(filePath string) []Diagnostic {
	uri := FilePathToURI(filePath)
	return c.GetDiagnostics(uri)
}

// IsGoFile checks if a file is a Go file
func IsGoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".go"
}

// GetGoProjectRoot finds the Go module root
func GetGoProjectRoot(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	dir := absPath
	if !isDir(dir) {
		dir = filepath.Dir(dir)
	}

	// Walk up looking for go.mod
	for {
		modPath := filepath.Join(dir, "go.mod")
		if fileExists(modPath) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root
			break
		}
		dir = parent
	}

	return absPath
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := exec.Command("test", "-f", path).Output()
	return err == nil
}

// isDir checks if a path is a directory
func isDir(path string) bool {
	_, err := exec.Command("test", "-d", path).Output()
	return err == nil
}
