package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2/lexers"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"open-coder/pkg/lsp"
)

// CodePanel displays code with line numbers
type CodePanel struct {
	FilePath   string
	Content    []string
	Language   string
	StartLine  int // 1-based
	CursorLine int // 1-based
	CursorCol  int // 0-based
	Offset     int
	Styles     Styles
	Width      int
	Height     int
	program    tea.Program

	// LSP support
	Diagnostics []lsp.Diagnostic
	HoverText   string
	ShowHover   bool
}

// NewCodePanel creates a new code panel
func NewCodePanel(styles Styles) *CodePanel {
	return &CodePanel{
		FilePath:   "",
		Content:    []string{},
		Language:   "",
		StartLine:  1,
		CursorLine: 1,
		CursorCol:  0,
		Offset:     0,
		Styles:     styles,
		Width:      80, // Increased for better readability
		Height:     30, // Increased for more visible content
	}
}

// SetProgram sets tea.Program for sending messages
func (cp *CodePanel) SetProgram(p *tea.Program) {
	cp.program = *p
}

// LoadFile loads a file into code panel
func (cp *CodePanel) LoadFile(filePath string) error {
	// Check if it's an image file - open with system viewer instead
	if isImageFile(filePath) {
		return openWithSystemViewer(filePath)
	}

	// Check if it's a binary file
	if isBinaryFile(filePath) {
		cp.FilePath = filePath
		cp.Content = []string{"[Binary file - cannot display]"}
		cp.Language = "Binary"
		cp.StartLine = 1
		cp.CursorLine = 1
		cp.Offset = 0
		return nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	cp.FilePath = filePath
	cp.Content = strings.Split(string(content), "\n")
	cp.Language = getLanguage(filePath)
	cp.StartLine = 1
	cp.CursorLine = 1
	cp.Offset = 0

	return nil
}

// SetContent sets code content directly
func (cp *CodePanel) SetContent(content string, language string) {
	cp.Content = strings.Split(content, "\n")
	cp.Language = language
	cp.StartLine = 1
	cp.CursorLine = 1
	cp.Offset = 0
}

// getLanguage returns the lexer name for a file
func getLanguage(filePath string) string {
	lexer := lexers.Match(filePath)
	if lexer != nil {
		return lexer.Config().Name
	}

	// Handle files without extensions
	lastDot := strings.LastIndex(filePath, ".")
	if lastDot == -1 || lastDot == len(filePath)-1 {
		return "PlainText"
	}

	ext := strings.ToLower(filePath[lastDot:])
	switch ext {
	case ".go":
		return "Go"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".py":
		return "Python"
	case ".rs":
		return "Rust"
	case ".java":
		return "Java"
	case ".c", ".h":
		return "C"
	case ".cpp", ".cc", ".hpp":
		return "C++"
	case ".css":
		return "CSS"
	case ".html":
		return "HTML"
	case ".json":
		return "JSON"
	case ".yaml", ".yml":
		return "YAML"
	case ".sh", ".bash":
		return "Bash"
	case ".sql":
		return "SQL"
	case ".md":
		return "Markdown"
	default:
		return "PlainText"
	}
}

// isImageFile checks if a file is an image based on extension
func isImageFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	imageExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
		".bmp": true, ".svg": true, ".webp": true, ".ico": true,
		".tiff": true, ".tif": true, ".heic": true, ".heif": true,
	}
	return imageExts[ext]
}

// isBinaryFile checks if a file is binary by reading first bytes
func isBinaryFile(filePath string) bool {
	// Check by extension first
	ext := strings.ToLower(filepath.Ext(filePath))
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".bin": true, ".obj": true, ".o": true, ".a": true,
		".zip": true, ".tar": true, ".gz": true, ".rar": true, ".7z": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wav": true,
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
	}
	if binaryExts[ext] {
		return true
	}

	// Read first 512 bytes to check for binary content
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil {
		return false
	}

	// Check for null bytes (common in binary files)
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}
	return false
}

// openWithSystemViewer opens a file with the system default application
func openWithSystemViewer(filePath string) error {
	cmd := exec.Command("xdg-open", filePath)
	return cmd.Start()
}

// Update handles input for code panel
func (cp *CodePanel) Update(msg tea.Msg) (*CodePanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return cp.handleKey(msg)
	}
	return cp, nil
}

// handleKey processes keyboard input
func (cp *CodePanel) handleKey(msg tea.KeyMsg) (*CodePanel, tea.Cmd) {
	if len(cp.Content) == 0 {
		return cp, nil
	}

	switch msg.String() {
	case "up", "k":
		if cp.CursorLine > cp.StartLine {
			cp.CursorLine--
			if cp.CursorLine-cp.StartLine < cp.Offset {
				cp.Offset--
			}
		}

	case "down", "j":
		lastLine := cp.StartLine + len(cp.Content) - 1
		if cp.CursorLine < lastLine {
			cp.CursorLine++
			visibleHeight := cp.Height - 4
			if cp.CursorLine-cp.StartLine-cp.Offset >= visibleHeight {
				cp.Offset++
			}
		}

	case "pgup":
		cp.Offset -= (cp.Height - 4) / 2
		if cp.Offset < 0 {
			cp.Offset = 0
		}
		cp.CursorLine = cp.StartLine + cp.Offset

	case "pgdown":
		visibleHeight := cp.Height - 4
		cp.Offset += visibleHeight / 2
		maxOffset := len(cp.Content) - visibleHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		if cp.Offset > maxOffset {
			cp.Offset = maxOffset
		}
		cp.CursorLine = cp.StartLine + cp.Offset

	case "home", "g":
		cp.Offset = 0
		cp.CursorLine = cp.StartLine

	case "end", "G":
		visibleHeight := cp.Height - 4
		maxOffset := len(cp.Content) - visibleHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		cp.Offset = maxOffset
		cp.CursorLine = cp.StartLine + cp.Offset

	case "left", "h":
		if cp.CursorCol > 0 {
			cp.CursorCol--
		}

	case "right", "l":
		lineContent := cp.GetLineContent(cp.CursorLine)
		if cp.CursorCol < len(lineContent) {
			cp.CursorCol++
		}
	}

	return cp, nil
}

// View renders the code panel
func (cp *CodePanel) View() string {
	if cp.FilePath == "" && len(cp.Content) == 0 {
		return cp.Styles.FilePickerFile.Render("No file loaded")
	}

	var b strings.Builder

	// Header with file path and diagnostic count
	headerText := "📄 " + cp.FilePath
	diagCount := len(cp.Diagnostics)
	if diagCount > 0 {
		headerText += fmt.Sprintf(" (%d problems)", diagCount)
	}
	header := cp.Styles.ModalTitle.Render(headerText)

	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(cp.Styles.Divider.Render(strings.Repeat("─", cp.Width-2)))
	b.WriteString("\n")

	// Calculate visible range
	visibleHeight := cp.Height - 4
	start := cp.Offset
	end := start + visibleHeight
	if end > len(cp.Content) {
		end = len(cp.Content)
	}

	// Render lines with line numbers
	for i := start; i < end; i++ {
		lineNum := cp.StartLine + i

		// Check for diagnostics on this line
		var lineDiag *lsp.Diagnostic
		for _, d := range cp.Diagnostics {
			// LSP uses 0-based indexing, our StartLine is usually 1
			if d.Range.Start.Line == lineNum-1 {
				lineDiag = &d
				break
			}
		}

		// Format line number
		lineNumStr := fmt.Sprintf("%5d", lineNum)

		lineNumStyle := cp.Styles.FilePickerFile
		if lineNum == cp.CursorLine {
			lineNumStyle = cp.Styles.FilePickerSelected.Foreground(cp.Styles.Theme.Primary)
		}

		// Highlight cursor line background
		lineStyle := cp.Styles.MessageAssist
		if lineNum == cp.CursorLine {
			lineStyle = lineStyle.Background(lipgloss.Color("#21262d"))
		}

		// Apply diagnostic styling if present
		if lineDiag != nil {
			// Error color usually red/orange
			lineStyle = lineStyle.Underline(true).UnderlineSpaces(true)
			lineNumStyle = lineNumStyle.Foreground(lipgloss.Color("#f85149")) // Red for error
		}

		// Get line content
		lineContent := ""
		if i < len(cp.Content) {
			lineContent = cp.Content[i]
		}

		// Wrap or truncate line content to fit width
		maxLineWidth := cp.Width - 9 // Account for line number and separator
		if maxLineWidth > 0 && len(lineContent) > maxLineWidth {
			// Truncate with ellipsis for code panel (wrapping would break code)
			lineContent = lineContent[:maxLineWidth-1] + "…"
		}

		// Build line
		var renderedLine string
		if lineNum == cp.CursorLine {
			// Render cursor within line
			cursorCol := cp.CursorCol
			if cursorCol < 0 {
				cursorCol = 0
			}
			if cursorCol > len(lineContent) {
				cursorCol = len(lineContent)
			}

			before := ""
			cursor := " "
			after := ""

			if len(lineContent) > 0 {
				if cursorCol < len(lineContent) {
					before = lineContent[:cursorCol]
					cursor = string(lineContent[cursorCol])
					after = lineContent[cursorCol+1:]
				} else {
					before = lineContent
				}
			}

			cursorStyle := lipgloss.NewStyle().Background(lipgloss.Color("#58a6ff")).Foreground(lipgloss.Color("#0d1117"))
			renderedLine = lineStyle.Render(before) + cursorStyle.Render(cursor) + lineStyle.Render(after)
		} else {
			renderedLine = lineStyle.Render(lineContent)
		}

		line := lipgloss.JoinHorizontal(
			lipgloss.Top,
			lineNumStyle.Render(lineNumStr+" │"),
			renderedLine,
		)

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Pad remaining lines to fill visible height
	for i := end - start; i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	// Render hover if active
	// Render hover if active
	if cp.ShowHover && cp.HoverText != "" {
		// Logic moved to end of function for overlay return
	}

	// Footer with info
	b.WriteString(cp.Styles.Divider.Render(strings.Repeat("─", cp.Width-2)))
	b.WriteString("\n")

	infoText := ""
	if cp.Language != "" {
		infoText = cp.Styles.Help.Render("Lang: " + cp.Language)
	}

	// Show diagnostic message for current line in footer
	var currentLineDiag *lsp.Diagnostic
	for _, d := range cp.Diagnostics {
		if d.Range.Start.Line == cp.CursorLine-1 {
			currentLineDiag = &d
			break
		}
	}

	if currentLineDiag != nil {
		diagMsg := fmt.Sprintf("⚠️ %s", currentLineDiag.Message)
		if infoText != "" {
			infoText += " │ "
		}
		infoText += lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")).Render(diagMsg)
	} else if len(cp.Content) > 0 {
		if infoText != "" {
			infoText += " │ "
		}
		infoText += cp.Styles.Help.Render(fmt.Sprintf("Lines: %d", len(cp.Content)))
	}

	// Add key hint for LSP
	infoText += cp.Styles.Help.Render(" │ gd: def · K: hover")

	b.WriteString(infoText)

	// If hover is active, render it as an overlay (using lipgloss.Place usually, but here we simply return it
	// heavily styled if we want a 'popup' feel, or we rely on the parent Model to render the popup.
	// Let's rely on parent Model for the actual popup rendering if possible, but here we can render it
	// if we are the focus.
	// We'll stick to rendering it as part of the view if active.
	if cp.ShowHover && cp.HoverText != "" {
		return lipgloss.Place(
			cp.Width, cp.Height,
			lipgloss.Center, lipgloss.Center,
			cp.Styles.Modal.Render(cp.HoverText),
		)
	}

	return b.String()
}

// SetDiagnostics updates the diagnostics for the file
func (cp *CodePanel) SetDiagnostics(diags []lsp.Diagnostic) {
	cp.Diagnostics = diags
}

// SetHoverContent sets the hover text
func (cp *CodePanel) SetHoverContent(text string) {
	cp.HoverText = text
	if text != "" {
		cp.ShowHover = true
	} else {
		cp.ShowHover = false
	}
}

// ToggleHover toggles hover visibility
func (cp *CodePanel) ToggleHover() {
	if cp.HoverText != "" {
		cp.ShowHover = !cp.ShowHover
	}
}

// GetLineContent returns content at specific line
func (cp *CodePanel) GetLineContent(lineNum int) string {
	idx := lineNum - cp.StartLine
	if idx >= 0 && idx < len(cp.Content) {
		return cp.Content[idx]
	}
	return ""
}

// GetSelectedLine returns the currently selected line number
func (cp *CodePanel) GetSelectedLine() int {
	return cp.CursorLine
}
