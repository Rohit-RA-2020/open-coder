package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FilePickerKeyMap defines the key bindings for the file picker
type FilePickerKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Select key.Binding
	Back   key.Binding
	Home   key.Binding
	Root   key.Binding
	Toggle key.Binding
	Cancel key.Binding
}

// DefaultFilePickerKeyMap returns the default key bindings
func DefaultFilePickerKeyMap() FilePickerKeyMap {
	return FilePickerKeyMap{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:   key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "parent")),
		Right:  key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "open")),
		Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Back:   key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "back")),
		Home:   key.NewBinding(key.WithKeys("~"), key.WithHelp("~", "home")),
		Root:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "root")),
		Toggle: key.NewBinding(key.WithKeys("."), key.WithHelp(".", "toggle hidden")),
		Cancel: key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc/q", "cancel")),
	}
}

// FileEntry represents a file or directory in the picker
type FileEntry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime string
}

// FilePicker is a file browser component
type FilePicker struct {
	CurrentPath string
	Entries     []FileEntry
	Cursor      int
	ShowHidden  bool
	KeyMap      FilePickerKeyMap
	Styles      Styles
	Width       int
	Height      int
	offset      int // For scrolling
}

// NewFilePicker creates a new file picker
func NewFilePicker(styles Styles) FilePicker {
	// Use current working directory instead of home
	currentDir, err := os.Getwd()
	if err != nil {
		currentDir, _ = os.UserHomeDir()
		if currentDir == "" {
			currentDir = "/"
		}
	}

	fp := FilePicker{
		CurrentPath: currentDir,
		ShowHidden:  false,
		KeyMap:      DefaultFilePickerKeyMap(),
		Styles:      styles,
		Width:       60,
		Height:      20,
	}
	fp.readDir()
	return fp
}

// readDir reads the current directory
func (fp *FilePicker) readDir() {
	entries, err := os.ReadDir(fp.CurrentPath)
	if err != nil {
		fp.Entries = []FileEntry{}
		return
	}

	fp.Entries = make([]FileEntry, 0, len(entries))

	// Add parent directory entry if not at root
	if fp.CurrentPath != "/" {
		fp.Entries = append(fp.Entries, FileEntry{
			Name:  "..",
			Path:  filepath.Dir(fp.CurrentPath),
			IsDir: true,
		})
	}

	// Separate dirs and files for sorting
	var dirs, files []FileEntry

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files if not showing them
		if !fp.ShowHidden && strings.HasPrefix(name, ".") {
			continue
		}

		info, _ := entry.Info()
		var size int64
		var modTime string
		if info != nil {
			size = info.Size()
			modTime = info.ModTime().Format("Jan 02 15:04")
		}

		fe := FileEntry{
			Name:    name,
			Path:    filepath.Join(fp.CurrentPath, name),
			IsDir:   entry.IsDir(),
			Size:    size,
			ModTime: modTime,
		}

		if entry.IsDir() {
			dirs = append(dirs, fe)
		} else {
			files = append(files, fe)
		}
	}

	// Sort alphabetically
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// Dirs first, then files
	fp.Entries = append(fp.Entries, dirs...)
	fp.Entries = append(fp.Entries, files...)

	fp.Cursor = 0
	fp.offset = 0
}

// Update handles input for the file picker
func (fp FilePicker) Update(msg tea.Msg) (FilePicker, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, fp.KeyMap.Up):
			if fp.Cursor > 0 {
				fp.Cursor--
				// Scroll up if needed
				if fp.Cursor < fp.offset {
					fp.offset = fp.Cursor
				}
			}

		case key.Matches(msg, fp.KeyMap.Down):
			if fp.Cursor < len(fp.Entries)-1 {
				fp.Cursor++
				// Scroll down if needed
				visibleHeight := fp.Height - 4 // Account for header/footer
				if fp.Cursor >= fp.offset+visibleHeight {
					fp.offset = fp.Cursor - visibleHeight + 1
				}
			}

		case key.Matches(msg, fp.KeyMap.Left), key.Matches(msg, fp.KeyMap.Back):
			// Go to parent directory
			parent := filepath.Dir(fp.CurrentPath)
			if parent != fp.CurrentPath {
				fp.CurrentPath = parent
				fp.readDir()
			}

		case key.Matches(msg, fp.KeyMap.Right):
			// Enter directory
			if fp.Cursor < len(fp.Entries) {
				entry := fp.Entries[fp.Cursor]
				if entry.IsDir {
					fp.CurrentPath = entry.Path
					fp.readDir()
				}
			}

		case key.Matches(msg, fp.KeyMap.Select):
			// Select file or enter directory
			if fp.Cursor < len(fp.Entries) {
				entry := fp.Entries[fp.Cursor]
				if entry.IsDir {
					fp.CurrentPath = entry.Path
					fp.readDir()
				} else {
					// Return file selection message
					return fp, func() tea.Msg {
						return FileSelectedMsg{Path: entry.Path}
					}
				}
			}

		case key.Matches(msg, fp.KeyMap.Home):
			homeDir, _ := os.UserHomeDir()
			if homeDir != "" {
				fp.CurrentPath = homeDir
				fp.readDir()
			}

		case key.Matches(msg, fp.KeyMap.Root):
			fp.CurrentPath = "/"
			fp.readDir()

		case key.Matches(msg, fp.KeyMap.Toggle):
			fp.ShowHidden = !fp.ShowHidden
			fp.readDir()

		case key.Matches(msg, fp.KeyMap.Cancel):
			return fp, func() tea.Msg {
				return FilePickerCancelledMsg{}
			}
		}
	}

	return fp, nil
}

// View renders the file picker
func (fp FilePicker) View() string {
	var b strings.Builder

	// Header with current path
	header := fp.Styles.ModalTitle.Render("📁 " + fp.CurrentPath)
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(fp.Styles.Divider.Render(strings.Repeat("─", fp.Width-4)))
	b.WriteString("\n")

	// Calculate visible range
	visibleHeight := fp.Height - 6 // Account for header, footer, borders
	if visibleHeight < 1 {
		visibleHeight = 10
	}

	start := fp.offset
	end := start + visibleHeight
	if end > len(fp.Entries) {
		end = len(fp.Entries)
	}

	// Entries
	for i := start; i < end; i++ {
		entry := fp.Entries[i]

		// Build the line
		var icon string
		var nameStyle lipgloss.Style

		if entry.IsDir {
			icon = "📂 "
			nameStyle = fp.Styles.FilePickerDir
		} else {
			icon = "📄 "
			nameStyle = fp.Styles.FilePickerFile
		}

		name := truncateString(entry.Name, fp.Width-10)
		line := icon + name

		// Highlight cursor
		if i == fp.Cursor {
			line = fp.Styles.FilePickerSelected.Width(fp.Width - 4).Render(line)
		} else {
			line = nameStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Pad remaining space
	for i := end - start; i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	// Footer with help
	b.WriteString(fp.Styles.Divider.Render(strings.Repeat("─", fp.Width-4)))
	b.WriteString("\n")

	hiddenStatus := "off"
	if fp.ShowHidden {
		hiddenStatus = "on"
	}
	help := fp.Styles.Help.Render(
		"↑↓: navigate · enter: select · ←: parent · .: hidden (" + hiddenStatus + ") · esc: cancel",
	)
	b.WriteString(help)

	return fp.Styles.Modal.Width(fp.Width).Height(fp.Height).Render(b.String())
}

// GetSelectedPath returns the currently selected path
func (fp FilePicker) GetSelectedPath() string {
	if fp.Cursor < len(fp.Entries) {
		return fp.Entries[fp.Cursor].Path
	}
	return ""
}

// truncateString truncates a string to max length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
