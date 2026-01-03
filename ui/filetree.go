package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// FileTreeKeyMap defines key bindings for file tree
type FileTreeKeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Left         key.Binding
	Right        key.Binding
	Select       key.Binding
	Collapse     key.Binding
	Expand       key.Binding
	ToggleHidden key.Binding
}

// DefaultFileTreeKeyMap returns default key bindings
func DefaultFileTreeKeyMap() FileTreeKeyMap {
	return FileTreeKeyMap{
		Up:           key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:         key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:         key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "collapse")),
		Right:        key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "expand/enter")),
		Select:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select file")),
		Collapse:     key.NewBinding(key.WithKeys("shift+left"), key.WithHelp("shift+←", "collapse")),
		Expand:       key.NewBinding(key.WithKeys("shift+right"), key.WithHelp("shift+→", "expand")),
		ToggleHidden: key.NewBinding(key.WithKeys("."), key.WithHelp(".", "toggle hidden")),
	}
}

// TreeNode represents a node in the file tree
type TreeNode struct {
	Path       string
	Name       string
	IsDir      bool
	IsExpanded bool
	IsRoot     bool
	Children   []*TreeNode
	Depth      int
}

// FileTree is a file tree component
type FileTree struct {
	Root         *TreeNode
	FlatNodes    []*TreeNode // Flattened for rendering
	Cursor       int
	SelectedPath string
	ShowHidden   bool
	KeyMap       FileTreeKeyMap
	Styles       Styles
	Width        int
	Height       int
	offset       int // For scrolling
	program      tea.Program
}

// NewFileTree creates a new file tree
func NewFileTree(rootPath string, styles Styles) *FileTree {
	root := buildTree(rootPath, "", 0)
	flatNodes := flattenTree(root)

	return &FileTree{
		Root:       root,
		FlatNodes:  flatNodes,
		Cursor:     0,
		ShowHidden: false,
		KeyMap:     DefaultFileTreeKeyMap(),
		Styles:     styles,
		Width:      30,
		Height:     20,
		offset:     0,
	}
}

// SetProgram sets the tea.Program for sending messages
func (ft *FileTree) SetProgram(p *tea.Program) {
	ft.program = *p
}

// buildTree recursively builds the file tree
func buildTree(path string, name string, depth int) *TreeNode {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}

	node := &TreeNode{
		Path:   path,
		Name:   name,
		IsDir:  info.IsDir(),
		Depth:  depth,
		IsRoot: depth == 0,
	}

	if node.IsDir {
		entries, err := os.ReadDir(path)
		if err == nil {
			for _, entry := range entries {
				childPath := filepath.Join(path, entry.Name())
				child := buildTree(childPath, entry.Name(), depth+1)
				if child != nil {
					node.Children = append(node.Children, child)
				}
			}
		}
	}

	return node
}

// flattenTree creates a flattened list of visible nodes
func flattenTree(root *TreeNode) []*TreeNode {
	var nodes []*TreeNode
	var traverse func(*TreeNode)

	traverse = func(node *TreeNode) {
		nodes = append(nodes, node)
		if node.IsDir && node.IsExpanded && len(node.Children) > 0 {
			for _, child := range node.Children {
				traverse(child)
			}
		}
	}

	traverse(root)
	return nodes
}

// shouldShowNode checks if node should be visible based on showHidden setting
func (ft *FileTree) shouldShowNode(node *TreeNode) bool {
	if !node.IsRoot && strings.HasPrefix(node.Name, ".") {
		return ft.ShowHidden
	}
	return true
}

// Update handles input for file tree
func (ft *FileTree) Update(msg tea.Msg) (*FileTree, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return ft.handleKey(msg)
	}
	return ft, nil
}

// handleKey processes keyboard input
func (ft *FileTree) handleKey(msg tea.KeyMsg) (*FileTree, tea.Cmd) {
	if len(ft.FlatNodes) == 0 {
		return ft, nil
	}

	switch {
	case key.Matches(msg, ft.KeyMap.Up):
		if ft.Cursor > 0 {
			ft.Cursor--
			if ft.Cursor < ft.offset {
				ft.offset = ft.Cursor
			}
		}

	case key.Matches(msg, ft.KeyMap.Down):
		if ft.Cursor < len(ft.FlatNodes)-1 {
			ft.Cursor++
			visibleHeight := ft.Height - 2
			if ft.Cursor >= ft.offset+visibleHeight {
				ft.offset = ft.Cursor - visibleHeight + 1
			}
		}

	case key.Matches(msg, ft.KeyMap.Left), key.Matches(msg, ft.KeyMap.Collapse):
		if ft.Cursor >= 0 && ft.Cursor < len(ft.FlatNodes) {
			node := ft.FlatNodes[ft.Cursor]
			if node.IsDir && node.IsExpanded {
				node.IsExpanded = false
				ft.FlatNodes = flattenTree(ft.Root)
				ft.SelectedPath = ""
			}
		}

	case key.Matches(msg, ft.KeyMap.Right), key.Matches(msg, ft.KeyMap.Expand):
		if ft.Cursor >= 0 && ft.Cursor < len(ft.FlatNodes) {
			node := ft.FlatNodes[ft.Cursor]
			if node.IsDir && !node.IsExpanded {
				node.IsExpanded = true
				ft.FlatNodes = flattenTree(ft.Root)
			}
		}

	case key.Matches(msg, ft.KeyMap.Select):
		if ft.Cursor >= 0 && ft.Cursor < len(ft.FlatNodes) {
			node := ft.FlatNodes[ft.Cursor]
			if !node.IsDir {
				ft.SelectedPath = node.Path
				return ft, func() tea.Msg {
					return FileSelectedInTreeMsg{Path: node.Path}
				}
			}
		}

	case key.Matches(msg, ft.KeyMap.ToggleHidden):
		ft.ShowHidden = !ft.ShowHidden
		ft.FlatNodes = flattenTree(ft.Root)
		// Clamp cursor to valid range after filtering
		if ft.Cursor >= len(ft.FlatNodes) {
			ft.Cursor = len(ft.FlatNodes) - 1
		}
		if ft.Cursor < 0 {
			ft.Cursor = 0
		}
	}

	return ft, nil
}

// View renders the file tree
func (ft *FileTree) View() string {
	if ft.Root == nil {
		return ft.Styles.FilePickerFile.Render("No files found")
	}

	var b strings.Builder

	// Filter visible nodes
	visibleNodes := make([]*TreeNode, 0)
	for _, node := range ft.FlatNodes {
		if ft.shouldShowNode(node) {
			visibleNodes = append(visibleNodes, node)
		}
	}

	// Calculate visible range
	visibleHeight := ft.Height - 2
	start := ft.offset
	if start >= len(visibleNodes) {
		start = len(visibleNodes) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + visibleHeight
	if end > len(visibleNodes) {
		end = len(visibleNodes)
	}

	// Calculate actual cursor position in filtered list
	cursorPos := -1
	originalCursor := ft.FlatNodes[ft.Cursor]
	for i, node := range visibleNodes {
		if node.Path == originalCursor.Path {
			cursorPos = i
			break
		}
	}

	// Render nodes
	for i := start; i < end; i++ {
		if i >= len(visibleNodes) {
			break
		}
		node := visibleNodes[i]

		// Build prefix with depth indentation
		prefix := ""
		if node.Depth > 0 {
			prefix = strings.Repeat("  ", node.Depth-1)
			if node.IsDir {
				if node.IsExpanded {
					prefix += "▼ "
				} else {
					prefix += "▶ "
				}
			} else {
				prefix += "  "
			}
		}

		// Get icon
		icon := ft.getFileIcon(node)

		// Build line
		name := node.Name
		if node.IsRoot {
			name = filepath.Base(node.Path)
		}
		if len(name) > ft.Width-10 {
			name = name[:ft.Width-10] + ".."
		}
		line := prefix + icon + " " + name

		// Highlight cursor
		if i == cursorPos {
			line = ft.Styles.FilePickerSelected.Width(ft.Width - 2).Render(line)
		} else {
			line = ft.Styles.FilePickerFile.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

// getFileIcon returns an icon for a file based on its name/extension
func (ft *FileTree) getFileIcon(node *TreeNode) string {
	if node.IsDir {
		if node.IsExpanded {
			return "📂"
		}
		return "📁"
	}

	name := strings.ToLower(node.Name)
	ext := filepath.Ext(name)

	switch ext {
	case ".go":
		return "🐹"
	case ".js", ".jsx", ".ts", ".tsx":
		return "📜"
	case ".py":
		return "🐍"
	case ".rs":
		return "🦀"
	case ".java":
		return "☕"
	case ".c", ".cpp", ".cc", ".h", ".hpp":
		return "⚙️"
	case ".css":
		return "🎨"
	case ".html":
		return "🌐"
	case ".md":
		return "📝"
	case ".json", ".yaml", ".yml", ".toml":
		return "📋"
	case ".sql":
		return "🗃️"
	case ".sh", ".bash", ".zsh":
		return "💻"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg":
		return "🖼️"
	case ".pdf":
		return "📕"
	default:
		if strings.Contains(name, "readme") {
			return "📖"
		}
		if strings.Contains(name, "license") {
			return "📄"
		}
		if strings.Contains(name, "test") {
			return "🧪"
		}
		return "📄"
	}
}

// Refresh rebuilds the tree
func (ft *FileTree) Refresh(rootPath string) {
	ft.Root = buildTree(rootPath, "", 0)
	ft.FlatNodes = flattenTree(ft.Root)
	ft.Cursor = 0
	ft.offset = 0
	ft.SelectedPath = ""
}

// GetSelectedPath returns the currently selected file path
func (ft *FileTree) GetSelectedPath() string {
	return ft.SelectedPath
}
