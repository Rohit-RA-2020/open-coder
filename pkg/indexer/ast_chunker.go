package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
)

// Parser pool for reuse across multiple files
var parserPool = sync.Pool{
	New: func() interface{} {
		return sitter.NewParser()
	},
}

// langMap maps file extensions to Tree-sitter languages
var langMap = map[string]*sitter.Language{
	".js":  javascript.GetLanguage(),
	".jsx": javascript.GetLanguage(),
	".ts":  javascript.GetLanguage(),
	".tsx": javascript.GetLanguage(),
	".py":  python.GetLanguage(),
	".go":  golang.GetLanguage(),
}

// langNameMap maps file extensions to language names
var langNameMap = map[string]string{
	".js":  "javascript",
	".jsx": "javascript",
	".ts":  "typescript",
	".tsx": "typescript",
	".py":  "python",
	".go":  "go",
}

// declarationTypes maps node types to chunk kinds per language
var declarationTypes = map[string]map[string]string{
	"go": {
		"function_declaration": "function",
		"method_declaration":   "method",
		"type_declaration":     "type",
		"const_declaration":    "const",
		"var_declaration":      "variable",
	},
	"javascript": {
		"function_declaration":       "function",
		"class_declaration":          "class",
		"method_definition":          "method",
		"arrow_function":             "function",
		"variable_declaration":       "variable",
		"lexical_declaration":        "variable",
		"export_statement":           "export",
		"function_expression":        "function",
		"generator_function":         "function",
		"async_function_declaration": "function",
	},
	"typescript": {
		"function_declaration":       "function",
		"class_declaration":          "class",
		"method_definition":          "method",
		"arrow_function":             "function",
		"variable_declaration":       "variable",
		"lexical_declaration":        "variable",
		"export_statement":           "export",
		"interface_declaration":      "interface",
		"type_alias_declaration":     "type",
		"enum_declaration":           "enum",
		"function_expression":        "function",
		"async_function_declaration": "function",
	},
	"python": {
		"function_definition":       "function",
		"class_definition":          "class",
		"decorated_definition":      "decorated",
		"async_function_definition": "function",
	},
}

// ASTChunker handles AST-based file chunking
type ASTChunker struct {
	maxChunkLines int
	chunkSize     int
	chunkOverlap  int
}

// NewASTChunker creates a new AST chunker with the given config
func NewASTChunker(maxChunkLines, chunkSize, chunkOverlap int) *ASTChunker {
	return &ASTChunker{
		maxChunkLines: maxChunkLines,
		chunkSize:     chunkSize,
		chunkOverlap:  chunkOverlap,
	}
}

// SupportsLanguage checks if the given file extension is supported for AST chunking
func SupportsLanguage(ext string) bool {
	_, ok := langMap[strings.ToLower(ext)]
	return ok
}

// ChunkFileAST parses a file using tree-sitter and extracts semantic chunks
func (ac *ASTChunker) ChunkFileAST(filePath string) ([]FileChunk, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	lang, ok := langMap[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}

	code, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Get parser from pool
	parser := parserPool.Get().(*sitter.Parser)
	defer parserPool.Put(parser)

	parser.SetLanguage(lang)
	tree := parser.Parse(nil, code)
	if tree == nil {
		return nil, fmt.Errorf("failed to parse file: tree-sitter returned nil")
	}
	defer tree.Close()

	langName := langNameMap[ext]
	root := tree.RootNode()
	lines := strings.Split(string(code), "\n")

	var chunks []FileChunk
	ac.extractDeclarations(root, code, lines, filePath, langName, "", &chunks)

	// If no declarations found, fall back to treating whole file as one chunk
	if len(chunks) == 0 {
		chunks = append(chunks, FileChunk{
			FilePath:  filePath,
			StartLine: 0,
			EndLine:   len(lines),
			Content:   string(code),
			Symbol:    filepath.Base(filePath),
			Kind:      "file",
			Language:  langName,
		})
	}

	return chunks, nil
}

// extractDeclarations recursively walks the AST and extracts declaration nodes
func (ac *ASTChunker) extractDeclarations(node *sitter.Node, code []byte, lines []string, filePath, langName, parent string, chunks *[]FileChunk) {
	nodeType := node.Type()

	// Check if this is a declaration we care about
	declTypes, langExists := declarationTypes[langName]
	if !langExists {
		declTypes = declarationTypes["javascript"] // fallback
	}

	if kind, isDecl := declTypes[nodeType]; isDecl {
		chunk := ac.nodeToChunk(node, code, lines, filePath, langName, kind, parent)
		if chunk != nil {
			*chunks = append(*chunks, chunk...)
		}

		// For classes, recurse to find methods with class as parent
		if kind == "class" {
			symbolName := ac.extractSymbolName(node, code, langName)
			ac.extractClassMembers(node, code, lines, filePath, langName, symbolName, chunks)
		}
		return // Don't recurse further for this declaration
	}

	// Recurse into children
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		ac.extractDeclarations(child, code, lines, filePath, langName, parent, chunks)
	}
}

// extractClassMembers extracts methods and other members from a class node
func (ac *ASTChunker) extractClassMembers(classNode *sitter.Node, code []byte, lines []string, filePath, langName, className string, chunks *[]FileChunk) {
	// Find the class body
	var bodyNode *sitter.Node
	childCount := int(classNode.ChildCount())
	for i := 0; i < childCount; i++ {
		child := classNode.Child(i)
		childType := child.Type()
		if childType == "class_body" || childType == "block" || childType == "declaration_list" {
			bodyNode = child
			break
		}
	}

	if bodyNode == nil {
		return
	}

	// Extract methods from class body
	memberCount := int(bodyNode.ChildCount())
	for i := 0; i < memberCount; i++ {
		member := bodyNode.Child(i)
		memberType := member.Type()

		// Check for method types
		kind := ""
		switch memberType {
		case "method_definition", "function_definition", "method_declaration":
			kind = "method"
		case "function_item": // Rust
			kind = "method"
		case "decorated_definition":
			kind = "method"
		}

		if kind != "" {
			chunk := ac.nodeToChunk(member, code, lines, filePath, langName, kind, className)
			if chunk != nil {
				*chunks = append(*chunks, chunk...)
			}
		}
	}
}

// nodeToChunk converts an AST node to one or more FileChunks
func (ac *ASTChunker) nodeToChunk(node *sitter.Node, code []byte, lines []string, filePath, langName, kind, parent string) []FileChunk {
	startLine := int(node.StartPoint().Row)
	endLine := int(node.EndPoint().Row) + 1 // Make end exclusive

	// Capture leading comments/docstrings
	startLine = ac.findLeadingCommentStart(lines, startLine)

	// Clamp to valid range
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine < 0 {
		startLine = 0
	}

	nodeLines := endLine - startLine
	symbolName := ac.extractSymbolName(node, code, langName)

	// If node is small enough, return as single chunk
	if nodeLines <= ac.maxChunkLines {
		content := strings.Join(lines[startLine:endLine], "\n")
		return []FileChunk{{
			FilePath:  filePath,
			StartLine: startLine,
			EndLine:   endLine,
			Content:   content,
			Symbol:    symbolName,
			Kind:      kind,
			Parent:    parent,
			Language:  langName,
		}}
	}

	// Split large node into sub-chunks
	return ac.splitLargeNode(lines, startLine, endLine, filePath, langName, symbolName, kind, parent)
}

// splitLargeNode splits a large declaration into overlapping sub-chunks
func (ac *ASTChunker) splitLargeNode(lines []string, startLine, endLine int, filePath, langName, symbol, kind, parent string) []FileChunk {
	var chunks []FileChunk

	totalParts := (endLine - startLine + ac.chunkSize - ac.chunkOverlap - 1) / (ac.chunkSize - ac.chunkOverlap)
	if totalParts < 1 {
		totalParts = 1
	}

	partNum := 1
	for chunkStart := startLine; chunkStart < endLine; chunkStart += (ac.chunkSize - ac.chunkOverlap) {
		chunkEnd := chunkStart + ac.chunkSize
		if chunkEnd > endLine {
			chunkEnd = endLine
		}

		content := strings.Join(lines[chunkStart:chunkEnd], "\n")
		part := fmt.Sprintf("%d/%d", partNum, totalParts)

		chunks = append(chunks, FileChunk{
			FilePath:  filePath,
			StartLine: chunkStart,
			EndLine:   chunkEnd,
			Content:   content,
			Symbol:    symbol,
			Kind:      kind,
			Parent:    parent,
			Language:  langName,
			Part:      part,
		})

		partNum++
		if chunkEnd >= endLine {
			break
		}
	}

	return chunks
}

// findLeadingCommentStart looks backwards from a line to find leading comments/docstrings
func (ac *ASTChunker) findLeadingCommentStart(lines []string, startLine int) int {
	if startLine <= 0 {
		return startLine
	}

	// Look backwards for consecutive comment lines
	commentStart := startLine
	for i := startLine - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])

		// Check for various comment patterns
		isComment := strings.HasPrefix(line, "//") ||
			strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "/*") ||
			strings.HasPrefix(line, "*") ||
			strings.HasPrefix(line, "*/") ||
			strings.HasPrefix(line, "\"\"\"") ||
			strings.HasPrefix(line, "'''") ||
			line == ""

		if isComment {
			if line != "" { // Only update if not empty
				commentStart = i
			}
		} else {
			break
		}
	}

	return commentStart
}

// extractSymbolName extracts the name of a declaration from an AST node
func (ac *ASTChunker) extractSymbolName(node *sitter.Node, code []byte, langName string) string {
	// Try to find identifier child
	childCount := int(node.ChildCount())
	for i := 0; i < childCount; i++ {
		child := node.Child(i)
		childType := child.Type()

		// Common identifier types
		switch childType {
		case "identifier", "name", "property_identifier", "field_identifier":
			return child.Content(code)
		case "type_identifier": // Go type declarations
			return child.Content(code)
		case "function_declarator": // C-style function declarations
			return ac.extractSymbolName(child, code, langName)
		}
	}

	// Language-specific handling
	switch langName {
	case "go":
		return ac.extractGoSymbolName(node, code)
	case "javascript", "typescript":
		return ac.extractJSSymbolName(node, code)
	case "python":
		return ac.extractPythonSymbolName(node, code)
	}

	return ""
}

// extractGoSymbolName extracts symbol name for Go declarations
func (ac *ASTChunker) extractGoSymbolName(node *sitter.Node, code []byte) string {
	nodeType := node.Type()

	switch nodeType {
	case "function_declaration":
		// func name(...) ...
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			return nameNode.Content(code)
		}
	case "method_declaration":
		// func (r Receiver) name(...) ...
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			return nameNode.Content(code)
		}
	case "type_declaration":
		// Look for type_spec children
		childCount := int(node.ChildCount())
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child.Type() == "type_spec" {
				if nameNode := child.ChildByFieldName("name"); nameNode != nil {
					return nameNode.Content(code)
				}
			}
		}
	case "const_declaration", "var_declaration":
		// Look for const_spec or var_spec
		childCount := int(node.ChildCount())
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child.Type() == "const_spec" || child.Type() == "var_spec" {
				if nameNode := child.ChildByFieldName("name"); nameNode != nil {
					return nameNode.Content(code)
				}
				// Try first identifier
				specChildCount := int(child.ChildCount())
				for j := 0; j < specChildCount; j++ {
					specChild := child.Child(j)
					if specChild.Type() == "identifier" {
						return specChild.Content(code)
					}
				}
			}
		}
	}

	return ""
}

// extractJSSymbolName extracts symbol name for JavaScript/TypeScript declarations
func (ac *ASTChunker) extractJSSymbolName(node *sitter.Node, code []byte) string {
	nodeType := node.Type()

	switch nodeType {
	case "function_declaration", "class_declaration", "interface_declaration",
		"type_alias_declaration", "enum_declaration", "async_function_declaration":
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			return nameNode.Content(code)
		}
	case "method_definition":
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			return nameNode.Content(code)
		}
	case "variable_declaration", "lexical_declaration":
		// Look for variable_declarator
		childCount := int(node.ChildCount())
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			if child.Type() == "variable_declarator" {
				if nameNode := child.ChildByFieldName("name"); nameNode != nil {
					return nameNode.Content(code)
				}
			}
		}
	case "arrow_function":
		// Arrow functions often don't have names, try parent assignment
		return ""
	case "export_statement":
		// Try to get name from declaration child
		if decl := node.ChildByFieldName("declaration"); decl != nil {
			return ac.extractJSSymbolName(decl, code)
		}
	}

	return ""
}

// extractPythonSymbolName extracts symbol name for Python declarations
func (ac *ASTChunker) extractPythonSymbolName(node *sitter.Node, code []byte) string {
	switch node.Type() {
	case "function_definition", "class_definition", "async_function_definition":
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			return nameNode.Content(code)
		}
	case "decorated_definition":
		// Look for function_definition or class_definition child
		childCount := int(node.ChildCount())
		for i := 0; i < childCount; i++ {
			child := node.Child(i)
			childType := child.Type()
			if childType == "function_definition" || childType == "class_definition" {
				return ac.extractPythonSymbolName(child, code)
			}
		}
	}

	return ""
}
