package ui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DiffLineType represents the type of a diff line
type DiffLineType int

const (
	DiffLineContext DiffLineType = iota
	DiffLineAdded
	DiffLineRemoved
	DiffLineFileHeader
	DiffLineHunkHeader
	DiffLineEmpty
)

// DiffLine represents a single line in a diff
type DiffLine struct {
	Type      DiffLineType
	Content   string
	OldLineNo int // Line number in old file (-1 if N/A)
	NewLineNo int // Line number in new file (-1 if N/A)
}

// DiffHunk represents a hunk in a diff (a section of changes)
type DiffHunk struct {
	Header     string
	OldStart   int
	OldCount   int
	NewStart   int
	NewCount   int
	Lines      []DiffLine
	StartIndex int // Index of first line in the full diff
}

// DiffFile represents a single file's diff
type DiffFile struct {
	OldPath      string
	NewPath      string
	Hunks        []DiffHunk
	IsNew        bool
	IsDeleted    bool
	Lines        []DiffLine // Lines for this specific file
	AddedCount   int
	RemovedCount int
}

// DiffFocus represents which panel is focused in the diff view
type DiffFocus int

const (
	DiffFocusFileList DiffFocus = iota
	DiffFocusContent
)

// DiffPanel displays git diff with syntax highlighting and file sidebar
type DiffPanel struct {
	Files        []DiffFile
	SelectedFile int       // Currently selected file in sidebar
	Focus        DiffFocus // Current focus (file list or content)
	RawDiff      string
	Staged       bool
	Offset       int   // Content scroll offset
	CursorLine   int   // Content cursor line
	FileOffset   int   // File list scroll offset
	HunkIndices  []int // Indices of hunk headers in current file
	Styles       Styles
	Width        int
	Height       int
	SidebarWidth int // Width of file sidebar
	Error        error
}

// NewDiffPanel creates a new diff panel
func NewDiffPanel(styles Styles) *DiffPanel {
	return &DiffPanel{
		Files:        []DiffFile{},
		SelectedFile: 0,
		Focus:        DiffFocusFileList,
		HunkIndices:  []int{},
		Styles:       styles,
		Width:        80,
		Height:       20,
		SidebarWidth: 35,
		Offset:       0,
		CursorLine:   0,
		FileOffset:   0,
	}
}

// hunkHeaderRegex matches hunk headers like @@ -1,5 +1,6 @@
var hunkHeaderRegex = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)$`)

// ParseDiff parses unified diff format into structured data
func (dp *DiffPanel) ParseDiff(diffText string) {
	dp.RawDiff = diffText
	dp.Files = []DiffFile{}
	dp.SelectedFile = 0
	dp.Offset = 0
	dp.CursorLine = 0
	dp.FileOffset = 0

	if diffText == "" {
		return
	}

	lines := strings.Split(diffText, "\n")
	var currentFile *DiffFile
	var currentHunk *DiffHunk
	oldLineNo, newLineNo := 0, 0

	for _, line := range lines {
		// Check for file headers
		if strings.HasPrefix(line, "diff --git") {
			// Save previous file if exists
			if currentFile != nil {
				if currentHunk != nil {
					currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
				}
				dp.countFileChanges(currentFile)
				dp.Files = append(dp.Files, *currentFile)
			}
			currentFile = &DiffFile{Lines: []DiffLine{}}
			currentHunk = nil

			currentFile.Lines = append(currentFile.Lines, DiffLine{
				Type:      DiffLineFileHeader,
				Content:   line,
				OldLineNo: -1,
				NewLineNo: -1,
			})
			continue
		}

		if currentFile == nil {
			continue
		}

		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			if strings.HasPrefix(line, "--- a/") {
				currentFile.OldPath = strings.TrimPrefix(line, "--- a/")
			} else if strings.HasPrefix(line, "+++ b/") {
				currentFile.NewPath = strings.TrimPrefix(line, "+++ b/")
			} else if strings.HasPrefix(line, "--- /dev/null") {
				currentFile.IsNew = true
			} else if strings.HasPrefix(line, "+++ /dev/null") {
				currentFile.IsDeleted = true
			}

			currentFile.Lines = append(currentFile.Lines, DiffLine{
				Type:      DiffLineFileHeader,
				Content:   line,
				OldLineNo: -1,
				NewLineNo: -1,
			})
			continue
		}

		// Check for hunk headers
		if matches := hunkHeaderRegex.FindStringSubmatch(line); matches != nil {
			// Save previous hunk
			if currentHunk != nil {
				currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
			}

			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}

			currentHunk = &DiffHunk{
				Header:     line,
				OldStart:   oldStart,
				OldCount:   oldCount,
				NewStart:   newStart,
				NewCount:   newCount,
				StartIndex: len(currentFile.Lines),
			}
			oldLineNo = oldStart
			newLineNo = newStart

			currentFile.Lines = append(currentFile.Lines, DiffLine{
				Type:      DiffLineHunkHeader,
				Content:   line,
				OldLineNo: -1,
				NewLineNo: -1,
			})
			continue
		}

		// Process diff content lines
		if currentHunk != nil {
			var diffLine DiffLine

			if strings.HasPrefix(line, "+") {
				diffLine = DiffLine{
					Type:      DiffLineAdded,
					Content:   line,
					OldLineNo: -1,
					NewLineNo: newLineNo,
				}
				newLineNo++
			} else if strings.HasPrefix(line, "-") {
				diffLine = DiffLine{
					Type:      DiffLineRemoved,
					Content:   line,
					OldLineNo: oldLineNo,
					NewLineNo: -1,
				}
				oldLineNo++
			} else if strings.HasPrefix(line, " ") || line == "" {
				diffLine = DiffLine{
					Type:      DiffLineContext,
					Content:   line,
					OldLineNo: oldLineNo,
					NewLineNo: newLineNo,
				}
				if line != "" {
					oldLineNo++
					newLineNo++
				}
			} else {
				// Other lines (like "\ No newline at end of file")
				diffLine = DiffLine{
					Type:      DiffLineContext,
					Content:   line,
					OldLineNo: -1,
					NewLineNo: -1,
				}
			}

			currentHunk.Lines = append(currentHunk.Lines, diffLine)
			currentFile.Lines = append(currentFile.Lines, diffLine)
		}
	}

	// Save last file and hunk
	if currentFile != nil {
		if currentHunk != nil {
			currentFile.Hunks = append(currentFile.Hunks, *currentHunk)
		}
		if currentFile.OldPath != "" || currentFile.NewPath != "" {
			dp.countFileChanges(currentFile)
			dp.Files = append(dp.Files, *currentFile)
		}
	}

	// Build hunk indices for current file
	dp.updateHunkIndices()
}

// countFileChanges counts added and removed lines for a file
func (dp *DiffPanel) countFileChanges(file *DiffFile) {
	for _, line := range file.Lines {
		switch line.Type {
		case DiffLineAdded:
			file.AddedCount++
		case DiffLineRemoved:
			file.RemovedCount++
		}
	}
}

// updateHunkIndices builds hunk indices for current selected file
func (dp *DiffPanel) updateHunkIndices() {
	dp.HunkIndices = []int{}
	if dp.SelectedFile >= 0 && dp.SelectedFile < len(dp.Files) {
		file := dp.Files[dp.SelectedFile]
		for i, line := range file.Lines {
			if line.Type == DiffLineHunkHeader {
				dp.HunkIndices = append(dp.HunkIndices, i)
			}
		}
	}
}

// FetchDiff runs git diff and parses the result
func FetchDiff(staged bool, path string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"diff", "--no-color"}
		if staged {
			args = append(args, "--cached")
		}
		if path != "" {
			args = append(args, "--", path)
		}

		cmd := exec.Command("git", args...)
		output, err := cmd.Output()

		return GitDiffResultMsg{
			Diff:   string(output),
			Error:  err,
			Staged: staged,
		}
	}
}

// Update handles input for diff panel
func (dp *DiffPanel) Update(msg tea.Msg) (*DiffPanel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return dp.handleKey(msg)
	}
	return dp, nil
}

// handleKey processes keyboard input
func (dp *DiffPanel) handleKey(msg tea.KeyMsg) (*DiffPanel, tea.Cmd) {
	switch msg.String() {
	case "left", "h":
		// Switch to file list
		dp.Focus = DiffFocusFileList
		return dp, nil

	case "right", "l":
		// Switch to content
		if len(dp.Files) > 0 {
			dp.Focus = DiffFocusContent
		}
		return dp, nil

	case "tab":
		// Toggle between staged/unstaged
		dp.Staged = !dp.Staged
		return dp, FetchDiff(dp.Staged, "")
	}

	if dp.Focus == DiffFocusFileList {
		return dp.handleFileListKey(msg)
	}
	return dp.handleContentKey(msg)
}

// handleFileListKey handles keys when file list is focused
func (dp *DiffPanel) handleFileListKey(msg tea.KeyMsg) (*DiffPanel, tea.Cmd) {
	if len(dp.Files) == 0 {
		return dp, nil
	}

	visibleHeight := dp.Height - 6

	switch msg.String() {
	case "up", "k":
		if dp.SelectedFile > 0 {
			dp.SelectedFile--
			if dp.SelectedFile < dp.FileOffset {
				dp.FileOffset = dp.SelectedFile
			}
			dp.Offset = 0
			dp.CursorLine = 0
			dp.updateHunkIndices()
		}

	case "down", "j":
		if dp.SelectedFile < len(dp.Files)-1 {
			dp.SelectedFile++
			if dp.SelectedFile >= dp.FileOffset+visibleHeight {
				dp.FileOffset = dp.SelectedFile - visibleHeight + 1
			}
			dp.Offset = 0
			dp.CursorLine = 0
			dp.updateHunkIndices()
		}

	case "enter":
		// Switch to content view
		dp.Focus = DiffFocusContent
		dp.Offset = 0
		dp.CursorLine = 0

	case "home", "g":
		dp.SelectedFile = 0
		dp.FileOffset = 0
		dp.Offset = 0
		dp.CursorLine = 0
		dp.updateHunkIndices()

	case "end", "G":
		dp.SelectedFile = len(dp.Files) - 1
		maxOffset := len(dp.Files) - visibleHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		dp.FileOffset = maxOffset
		dp.Offset = 0
		dp.CursorLine = 0
		dp.updateHunkIndices()
	}

	return dp, nil
}

// handleContentKey handles keys when content is focused
func (dp *DiffPanel) handleContentKey(msg tea.KeyMsg) (*DiffPanel, tea.Cmd) {
	if dp.SelectedFile >= len(dp.Files) || dp.SelectedFile < 0 {
		return dp, nil
	}

	file := dp.Files[dp.SelectedFile]
	if len(file.Lines) == 0 {
		return dp, nil
	}

	visibleHeight := dp.Height - 6

	switch msg.String() {
	case "up", "k":
		if dp.CursorLine > 0 {
			dp.CursorLine--
			if dp.CursorLine < dp.Offset {
				dp.Offset = dp.CursorLine
			}
		}

	case "down", "j":
		if dp.CursorLine < len(file.Lines)-1 {
			dp.CursorLine++
			if dp.CursorLine >= dp.Offset+visibleHeight {
				dp.Offset = dp.CursorLine - visibleHeight + 1
			}
		}

	case "pgup":
		dp.CursorLine -= visibleHeight / 2
		if dp.CursorLine < 0 {
			dp.CursorLine = 0
		}
		dp.Offset -= visibleHeight / 2
		if dp.Offset < 0 {
			dp.Offset = 0
		}

	case "pgdown":
		dp.CursorLine += visibleHeight / 2
		if dp.CursorLine >= len(file.Lines) {
			dp.CursorLine = len(file.Lines) - 1
		}
		dp.Offset += visibleHeight / 2
		maxOffset := len(file.Lines) - visibleHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		if dp.Offset > maxOffset {
			dp.Offset = maxOffset
		}

	case "home", "g":
		dp.Offset = 0
		dp.CursorLine = 0

	case "end", "G":
		dp.CursorLine = len(file.Lines) - 1
		maxOffset := len(file.Lines) - visibleHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		dp.Offset = maxOffset

	case "n":
		// Jump to next hunk
		for _, idx := range dp.HunkIndices {
			if idx > dp.CursorLine {
				dp.CursorLine = idx
				if dp.CursorLine >= dp.Offset+visibleHeight {
					dp.Offset = dp.CursorLine - 2
					if dp.Offset < 0 {
						dp.Offset = 0
					}
				}
				break
			}
		}

	case "N":
		// Jump to previous hunk
		for i := len(dp.HunkIndices) - 1; i >= 0; i-- {
			if dp.HunkIndices[i] < dp.CursorLine {
				dp.CursorLine = dp.HunkIndices[i]
				if dp.CursorLine < dp.Offset {
					dp.Offset = dp.CursorLine
				}
				break
			}
		}
	}

	return dp, nil
}

// View renders the diff panel with split layout
func (dp *DiffPanel) View() string {
	if dp.Error != nil {
		return dp.Styles.Error.Render(fmt.Sprintf("Error: %v", dp.Error))
	}

	if len(dp.Files) == 0 {
		stagedText := "unstaged"
		if dp.Staged {
			stagedText = "staged"
		}
		return dp.Styles.Help.Render(fmt.Sprintf("No %s changes found. Press Tab to toggle staged/unstaged.", stagedText))
	}

	// Calculate layout
	contentWidth := dp.Width - dp.SidebarWidth - 3

	// Build sidebar (file list)
	sidebar := dp.renderFileList()

	// Build content panel
	content := dp.renderContent(contentWidth)

	// Join sidebar and content horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebar,
		content,
	)
}

// renderFileList renders the file list sidebar
func (dp *DiffPanel) renderFileList() string {
	var b strings.Builder

	// Header
	stagedIndicator := "Unstaged"
	if dp.Staged {
		stagedIndicator = "Staged"
	}
	header := fmt.Sprintf("📁 Files [%s]", stagedIndicator)
	b.WriteString(dp.Styles.ModalTitle.Render(header))
	b.WriteString("\n")

	divider := strings.Repeat("─", dp.SidebarWidth-4)
	b.WriteString(dp.Styles.Divider.Render(divider))
	b.WriteString("\n")

	// Calculate visible range
	visibleHeight := dp.Height - 8
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	start := dp.FileOffset
	end := start + visibleHeight
	if end > len(dp.Files) {
		end = len(dp.Files)
	}

	// Render file entries
	for i := start; i < end; i++ {
		file := dp.Files[i]
		isSelected := i == dp.SelectedFile

		// Get filename
		name := file.NewPath
		if name == "" {
			name = file.OldPath
		}
		// Show just the basename for compactness
		displayName := filepath.Base(name)

		// Status icon
		icon := "M" // Modified
		if file.IsNew {
			icon = "A" // Added
		} else if file.IsDeleted {
			icon = "D" // Deleted
		}

		// Change stats
		stats := fmt.Sprintf("+%d -%d", file.AddedCount, file.RemovedCount)

		// Build line
		var line string
		if isSelected {
			focusIndicator := "▸"
			if dp.Focus == DiffFocusFileList {
				focusIndicator = "▶"
			}
			line = fmt.Sprintf("%s %s %s", focusIndicator, icon, displayName)

			// Truncate if needed
			maxLen := dp.SidebarWidth - 12
			if len(line) > maxLen {
				line = line[:maxLen-1] + "…"
			}

			// Pad and add stats
			padding := dp.SidebarWidth - 6 - len(line) - len(stats)
			if padding < 1 {
				padding = 1
			}
			line = line + strings.Repeat(" ", padding) + stats

			// Apply selected style
			selectedStyle := dp.Styles.FilePickerSelected
			if dp.Focus == DiffFocusFileList {
				selectedStyle = selectedStyle.Background(lipgloss.Color("#21262d"))
			}
			b.WriteString(selectedStyle.Render(line))
		} else {
			line = fmt.Sprintf("  %s %s", icon, displayName)

			// Truncate if needed
			maxLen := dp.SidebarWidth - 12
			if len(line) > maxLen {
				line = line[:maxLen-1] + "…"
			}

			// Pad and add stats
			padding := dp.SidebarWidth - 6 - len(line) - len(stats)
			if padding < 1 {
				padding = 1
			}
			line = line + strings.Repeat(" ", padding)

			// Color-coded stats
			statsStyled := dp.Styles.Success.Render(fmt.Sprintf("+%d", file.AddedCount)) + " " +
				dp.Styles.Error.Render(fmt.Sprintf("-%d", file.RemovedCount))

			// Icon color based on type
			iconStyle := dp.Styles.Warning // Modified - yellow
			if file.IsNew {
				iconStyle = dp.Styles.Success // Added - green
			} else if file.IsDeleted {
				iconStyle = dp.Styles.Error // Deleted - red
			}

			iconPart := iconStyle.Render(icon)
			namePart := dp.Styles.FilePickerFile.Render(displayName)
			b.WriteString(fmt.Sprintf("  %s %s%s%s", iconPart, namePart, strings.Repeat(" ", padding), statsStyled))
		}
		b.WriteString("\n")
	}

	// Pad remaining lines
	for i := end - start; i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	// Footer
	b.WriteString(dp.Styles.Divider.Render(divider))
	b.WriteString("\n")

	totalAdded, totalRemoved := dp.countTotalChanges()
	totalStats := dp.Styles.Success.Render(fmt.Sprintf("+%d", totalAdded)) + " " +
		dp.Styles.Error.Render(fmt.Sprintf("-%d", totalRemoved))
	b.WriteString(totalStats + " │ " + dp.Styles.Help.Render(fmt.Sprintf("%d files", len(dp.Files))))

	// Apply border based on focus
	sidebarStyle := dp.Styles.SidebarPanel.Width(dp.SidebarWidth)
	if dp.Focus == DiffFocusFileList {
		sidebarStyle = sidebarStyle.BorderForeground(dp.Styles.Theme.BorderFocused)
	}

	return sidebarStyle.Height(dp.Height - 2).Render(b.String())
}

// renderContent renders the diff content panel
func (dp *DiffPanel) renderContent(width int) string {
	if dp.SelectedFile >= len(dp.Files) || dp.SelectedFile < 0 {
		return dp.Styles.Help.Render("Select a file to view changes")
	}

	file := dp.Files[dp.SelectedFile]
	var b strings.Builder

	// Header with filename
	fileName := file.NewPath
	if fileName == "" {
		fileName = file.OldPath
	}

	// Status
	status := "modified"
	if file.IsNew {
		status = "new file"
	} else if file.IsDeleted {
		status = "deleted"
	}

	header := fmt.Sprintf("📄 %s (%s)", fileName, status)
	b.WriteString(dp.Styles.ModalTitle.Render(header))
	b.WriteString("\n")

	divider := strings.Repeat("─", width-4)
	b.WriteString(dp.Styles.Divider.Render(divider))
	b.WriteString("\n")

	// Calculate visible range
	visibleHeight := dp.Height - 8
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	start := dp.Offset
	end := start + visibleHeight
	if end > len(file.Lines) {
		end = len(file.Lines)
	}

	// Render visible lines
	for i := start; i < end; i++ {
		line := file.Lines[i]
		renderedLine := dp.renderDiffLine(line, i == dp.CursorLine, width-4)
		b.WriteString(renderedLine)
		b.WriteString("\n")
	}

	// Pad remaining lines
	for i := end - start; i < visibleHeight; i++ {
		b.WriteString("\n")
	}

	// Footer
	b.WriteString(dp.Styles.Divider.Render(divider))
	b.WriteString("\n")

	// File stats
	fileStats := dp.Styles.Success.Render(fmt.Sprintf("+%d", file.AddedCount)) + " " +
		dp.Styles.Error.Render(fmt.Sprintf("-%d", file.RemovedCount))
	position := fmt.Sprintf("Line %d/%d", dp.CursorLine+1, len(file.Lines))
	footer := dp.Styles.Help.Render("j/k: scroll · n/N: hunk · ←: files · Tab: staged")

	b.WriteString(fileStats + " │ " + dp.Styles.Help.Render(position) + " │ " + footer)

	// Apply border based on focus
	contentStyle := dp.Styles.CodePanel.Width(width)
	if dp.Focus == DiffFocusContent {
		contentStyle = contentStyle.BorderForeground(dp.Styles.Theme.BorderFocused)
	}

	return contentStyle.Height(dp.Height - 2).Render(b.String())
}

// renderDiffLine renders a single diff line with appropriate styling
func (dp *DiffPanel) renderDiffLine(line DiffLine, isCursor bool, maxWidth int) string {
	// Build line number column
	lineNums := dp.formatLineNumbers(line)

	// Get content
	content := line.Content

	// Apply line-specific styling
	var lineStyle lipgloss.Style
	var prefixStyle lipgloss.Style

	switch line.Type {
	case DiffLineAdded:
		lineStyle = dp.Styles.DiffAddedLine
		prefixStyle = dp.Styles.DiffAddedPrefix
	case DiffLineRemoved:
		lineStyle = dp.Styles.DiffRemovedLine
		prefixStyle = dp.Styles.DiffRemovedPrefix
	case DiffLineHunkHeader:
		lineStyle = dp.Styles.DiffHunkHeader
		prefixStyle = dp.Styles.DiffHunkHeader
	case DiffLineFileHeader:
		lineStyle = dp.Styles.DiffFileHeader
		prefixStyle = dp.Styles.DiffFileHeader
	default:
		lineStyle = dp.Styles.DiffContextLine
		prefixStyle = dp.Styles.DiffContextLine
	}

	// Highlight cursor line
	if isCursor && dp.Focus == DiffFocusContent {
		lineStyle = lineStyle.Background(lipgloss.Color("#21262d"))
	}

	// Format the line
	var prefix, rest string
	if len(content) > 0 && (line.Type == DiffLineAdded || line.Type == DiffLineRemoved || line.Type == DiffLineContext) {
		prefix = string(content[0])
		if len(content) > 1 {
			rest = content[1:]
		}
	} else {
		rest = content
	}

	// Truncate content to fit width
	maxContentWidth := maxWidth - 16
	if maxContentWidth > 0 && len(rest) > maxContentWidth {
		rest = rest[:maxContentWidth]
	}

	// Build final line
	lineNumStyle := dp.Styles.DiffLineNumber
	formattedLine := lineNumStyle.Render(lineNums) + prefixStyle.Render(prefix) + lineStyle.Render(rest)

	return formattedLine
}

// formatLineNumbers formats the old and new line numbers
func (dp *DiffPanel) formatLineNumbers(line DiffLine) string {
	oldNum := "    "
	newNum := "    "

	if line.OldLineNo > 0 {
		oldNum = fmt.Sprintf("%4d", line.OldLineNo)
	}
	if line.NewLineNo > 0 {
		newNum = fmt.Sprintf("%4d", line.NewLineNo)
	}

	if line.Type == DiffLineFileHeader || line.Type == DiffLineHunkHeader {
		return "         "
	}

	return oldNum + " " + newNum + " │"
}

// countTotalChanges counts total added and removed lines
func (dp *DiffPanel) countTotalChanges() (added, removed int) {
	for _, file := range dp.Files {
		added += file.AddedCount
		removed += file.RemovedCount
	}
	return
}

// HasChanges returns true if there are any files with changes
func (dp *DiffPanel) HasChanges() bool {
	return len(dp.Files) > 0
}

// GetFileCount returns the number of files in the diff
func (dp *DiffPanel) GetFileCount() int {
	return len(dp.Files)
}
