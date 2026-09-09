package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

type FilePickerMode int

const (
	FilePickerModeOpen FilePickerMode = iota
	FilePickerModeSave
	FilePickerModeImportCSV
	FilePickerModeExportCSV
	FilePickerModeExportXLSX
	FilePickerModeExportODS
	FilePickerModeExportMarkdown
)

type FileEntry struct {
	Name    string
	IsDir   bool
	Size    int64
	ModTime time.Time
}

type FilePicker struct {
	active           bool
	mode             FilePickerMode
	title            string
	currentDir       string
	entries          []FileEntry
	selectedIdx      int
	topIdx           int
	inputBuffer      []rune
	cursorPos        int
	inputActive      bool
	quitOnSave       bool
	newOnSave        bool
	confirmOverwrite bool
	pendingSavePath  string
	errorMessage     string
	typeAhead        string
	lastTypeAheadTime time.Time
}

func NewFilePicker() *FilePicker {
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	return &FilePicker{
		active:           false,
		currentDir:       dir,
		quitOnSave:       false,
		newOnSave:        false,
		confirmOverwrite: false,
		pendingSavePath:  "",
		errorMessage:     "",
	}
}

func (fp *FilePicker) IsActive() bool {
	return fp.active
}

func (fp *FilePicker) InputBuffer() []rune {
	return fp.inputBuffer
}

func (fp *FilePicker) CursorPos() int {
	return fp.cursorPos
}

func (fp *FilePicker) QuitOnSave() bool {
	return fp.quitOnSave
}

func (fp *FilePicker) SetQuitOnSave(q bool) {
	fp.quitOnSave = q
}

func (fp *FilePicker) NewOnSave() bool {
	return fp.newOnSave
}

func (fp *FilePicker) SetNewOnSave(n bool) {
	fp.newOnSave = n
}

func (fp *FilePicker) GetSelectedEntryForTest() FileEntry {
	if fp.selectedIdx >= 0 && fp.selectedIdx < len(fp.entries) {
		return fp.entries[fp.selectedIdx]
	}
	return FileEntry{}
}

func (fp *FilePicker) isSaveOrExport() bool {
	return fp.mode == FilePickerModeSave || fp.mode == FilePickerModeExportCSV || fp.mode == FilePickerModeExportXLSX || fp.mode == FilePickerModeExportODS || fp.mode == FilePickerModeExportMarkdown
}

var invalidFilenameChars = []rune{
	'/', '\\', ':', '*', '?', '"', '<', '>', '|', '\x00',
}

var reservedDeviceNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// ValidateFilename checks if a filename is valid across OS platforms.
func ValidateFilename(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("Filename cannot be empty")
	}
	if strings.HasPrefix(name, " ") || strings.HasSuffix(name, " ") {
		return fmt.Errorf("Filename cannot start or end with a space")
	}
	if strings.HasSuffix(name, ".") {
		return fmt.Errorf("Filename cannot end with a dot")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("Filename '%s' is not allowed", name)
	}
	if len(name) > 255 {
		return fmt.Errorf("Filename is too long (max 255 characters)")
	}

	for _, ch := range name {
		if ch < 32 || ch == 127 {
			return fmt.Errorf("Filename contains control characters")
		}
		for _, inv := range invalidFilenameChars {
			if ch == inv {
				return fmt.Errorf("Filename contains invalid character '%c'", ch)
			}
		}
	}

	base := strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name)))
	if reservedDeviceNames[base] {
		return fmt.Errorf("Filename '%s' is a reserved system name", base)
	}

	return nil
}

// GenerateUnusedFilename returns candidate filename with baseName and ext that does not exist in dir.
func GenerateUnusedFilename(dir, baseName, ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if dir == "" {
		dir = "."
	}
	firstCandidate := fmt.Sprintf("%s%s", baseName, ext)
	if _, err := os.Stat(filepath.Join(dir, firstCandidate)); os.IsNotExist(err) {
		return firstCandidate
	}
	for i := 1; i < 10000; i++ {
		candidate := fmt.Sprintf("%s%d%s", baseName, i, ext)
		if _, err := os.Stat(filepath.Join(dir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
	return firstCandidate
}

// NormalizeHwkSaveFilename ensures appropriate HasuCalc extension (.hwk, .hwkz, or .hwk.gz).
func NormalizeHwkSaveFilename(fn string) string {
	fn = strings.TrimSpace(fn)
	lower := strings.ToLower(fn)
	if strings.HasSuffix(lower, ".hwk.gz") {
		return fn
	}
	if strings.HasSuffix(lower, ".hwkz") {
		return fn
	}
	if strings.HasSuffix(lower, ".hwk") {
		return fn
	}
	if strings.HasSuffix(lower, ".gz") {
		base := strings.TrimSuffix(fn, filepath.Ext(fn))
		if !strings.HasSuffix(strings.ToLower(base), ".hwk") {
			base += ".hwk"
		}
		return base + ".gz"
	}
	if filepath.Ext(fn) != "" {
		base := strings.TrimSuffix(fn, filepath.Ext(fn))
		return base + ".hwk"
	}
	return fn + ".hwk"
}

func (fp *FilePicker) Open(mode FilePickerMode, initialPath string) {
	fp.active = true
	fp.mode = mode
	fp.quitOnSave = false
	fp.newOnSave = false
	fp.confirmOverwrite = false
	fp.pendingSavePath = ""
	fp.errorMessage = ""

	if initialPath != "" {
		absPath, err := filepath.Abs(initialPath)
		if err == nil {
			stat, err := os.Stat(absPath)
			if err == nil && stat.IsDir() {
				fp.currentDir = absPath
				fp.inputBuffer = nil
			} else {
				fp.currentDir = filepath.Dir(absPath)
				base := filepath.Base(absPath)
				if base != "." && base != "/" {
					fp.inputBuffer = []rune(base)
				}
			}
		}
	}

	if fp.currentDir == "" {
		dir, _ := os.Getwd()
		fp.currentDir = dir
	}

	switch fp.mode {
	case FilePickerModeOpen:
		fp.title = " OPEN WORKSHEET "
		fp.inputActive = false
	case FilePickerModeSave:
		fp.title = " SAVE WORKSHEET "
		if len(fp.inputBuffer) == 0 {
			fp.inputBuffer = []rune(GenerateUnusedFilename(fp.currentDir, "DATA", ".hwk"))
		} else {
			curStr := string(fp.inputBuffer)
			fp.inputBuffer = []rune(NormalizeHwkSaveFilename(curStr))
		}
		fp.inputActive = true
	case FilePickerModeImportCSV:
		fp.title = " IMPORT CSV FILE "
		fp.inputActive = false
	case FilePickerModeExportCSV:
		fp.title = " EXPORT CSV FILE "
		if len(fp.inputBuffer) == 0 {
			fp.inputBuffer = []rune("sheet.csv")
		}
		fp.inputActive = true
	case FilePickerModeExportXLSX:
		fp.title = " EXPORT EXCEL (.XLSX) FILE "
		if len(fp.inputBuffer) == 0 {
			fp.inputBuffer = []rune("workbook.xlsx")
		}
		fp.inputActive = true
	case FilePickerModeExportODS:
		fp.title = " EXPORT OPENDOCUMENT (.ODS) FILE "
		if len(fp.inputBuffer) == 0 {
			fp.inputBuffer = []rune("workbook.ods")
		}
		fp.inputActive = true
	case FilePickerModeExportMarkdown:
		fp.title = " EXPORT MARKDOWN (.MD) TABLE "
		if len(fp.inputBuffer) == 0 {
			fp.inputBuffer = []rune("table.md")
		}
		fp.inputActive = true
	}

	fp.typeAhead = ""
	fp.lastTypeAheadTime = time.Time{}
	fp.readDirectory()
	fp.selectedIdx = 0
	fp.topIdx = 0
	fp.cursorPos = len(fp.inputBuffer)
}

func (fp *FilePicker) Close() {
	fp.active = false
	fp.typeAhead = ""
	fp.lastTypeAheadTime = time.Time{}
}

func (fp *FilePicker) readDirectory() {
	fp.entries = nil
	fp.typeAhead = ""
	fp.lastTypeAheadTime = time.Time{}

	// Add parent directory entry unless we are at root
	parent := filepath.Dir(fp.currentDir)
	if parent != fp.currentDir {
		fp.entries = append(fp.entries, FileEntry{
			Name:  "..",
			IsDir: true,
		})
	}

	dirEntries, err := os.ReadDir(fp.currentDir)
	if err != nil {
		return
	}

	var dirs []FileEntry
	var files []FileEntry

	for _, de := range dirEntries {
		name := de.Name()
		// Hide hidden files starting with . except ..
		if strings.HasPrefix(name, ".") {
			continue
		}

		info, err := de.Info()
		if err != nil {
			continue
		}

		if de.IsDir() {
			dirs = append(dirs, FileEntry{
				Name:    name,
				IsDir:   true,
				ModTime: info.ModTime(),
			})
		} else {
			// Extension check
			ext := strings.ToLower(filepath.Ext(name))
			match := true
			switch fp.mode {
			case FilePickerModeOpen:
				match = (ext == ".hwk" || ext == ".hwkz" || ext == ".gz" || ext == ".json" || ext == ".csv" || ext == ".tsv" || ext == ".xlsx" || ext == ".xlsm" || ext == ".ods" || ext == ".ots" || ext == ".md" || ext == ".markdown" || ext == ".html" || ext == ".htm" || ext == ".wk3" || ext == ".123" || strings.HasSuffix(name, ".123.json") || strings.HasSuffix(name, ".hwk.gz"))
			case FilePickerModeSave:
				match = (ext == ".hwk" || ext == ".hwkz" || ext == ".gz" || ext == ".json" || strings.HasSuffix(name, ".123.json") || strings.HasSuffix(name, ".hwk.gz"))
			case FilePickerModeImportCSV:
				match = (ext == ".csv" || ext == ".tsv" || ext == ".txt")
			case FilePickerModeExportCSV:
				match = (ext == ".csv")
			case FilePickerModeExportXLSX:
				match = (ext == ".xlsx" || ext == ".xlsm")
			case FilePickerModeExportODS:
				match = (ext == ".ods" || ext == ".ots")
			case FilePickerModeExportMarkdown:
				match = (ext == ".md" || ext == ".markdown")
			}

			if match {
				files = append(files, FileEntry{
					Name:    name,
					IsDir:   false,
					Size:    info.Size(),
					ModTime: info.ModTime(),
				})
			}
		}
	}

	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	fp.entries = append(fp.entries, dirs...)
	fp.entries = append(fp.entries, files...)

	if fp.selectedIdx >= len(fp.entries) {
		fp.selectedIdx = 0
	}
	fp.topIdx = 0
}

func (fp *FilePicker) adjustViewport(maxVisible int) {
	if maxVisible <= 0 {
		return
	}
	if fp.selectedIdx < fp.topIdx {
		fp.topIdx = fp.selectedIdx
	} else if fp.selectedIdx >= fp.topIdx+maxVisible {
		fp.topIdx = fp.selectedIdx - maxVisible + 1
	}
	if fp.topIdx > len(fp.entries)-maxVisible {
		fp.topIdx = len(fp.entries) - maxVisible
	}
	if fp.topIdx < 0 {
		fp.topIdx = 0
	}
}

func (fp *FilePicker) HandleKey(ev *tcell.EventKey) (done bool, selectedPath string, canceled bool) {
	if !fp.active {
		return false, "", false
	}

	// 1. If Overwrite confirmation dialog is active, handle Y / N / Esc
	if fp.confirmOverwrite {
		switch ev.Key() {
		case tcell.KeyEscape:
			fp.confirmOverwrite = false
			fp.pendingSavePath = ""
			return false, "", false
		case tcell.KeyRune:
			switch ev.Rune() {
			case 'y', 'Y':
				target := fp.pendingSavePath
				fp.confirmOverwrite = false
				fp.pendingSavePath = ""
				fp.Close()
				return true, target, false
			case 'n', 'N', 'c', 'C':
				fp.confirmOverwrite = false
				fp.pendingSavePath = ""
				return false, "", false
			}
		case tcell.KeyEnter:
			target := fp.pendingSavePath
			fp.confirmOverwrite = false
			fp.pendingSavePath = ""
			fp.Close()
			return true, target, false
		}
		return false, "", false
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		fp.typeAhead = ""
		fp.Close()
		return true, "", true

	case tcell.KeyTab:
		fp.errorMessage = ""
		fp.typeAhead = ""
		// Toggle input box focus in Save/Export modes
		if fp.isSaveOrExport() {
			fp.inputActive = !fp.inputActive
		}
		return false, "", false

	case tcell.KeyUp, tcell.KeyCtrlP:
		fp.errorMessage = ""
		fp.typeAhead = ""
		if !fp.inputActive && len(fp.entries) > 0 {
			if fp.selectedIdx > 0 {
				fp.selectedIdx--
				if fp.isSaveOrExport() && !fp.entries[fp.selectedIdx].IsDir {
					fp.inputBuffer = []rune(fp.entries[fp.selectedIdx].Name)
					fp.cursorPos = len(fp.inputBuffer)
				}
			}
		}
		return false, "", false

	case tcell.KeyDown, tcell.KeyCtrlN:
		fp.errorMessage = ""
		fp.typeAhead = ""
		if !fp.inputActive && len(fp.entries) > 0 {
			if fp.selectedIdx < len(fp.entries)-1 {
				fp.selectedIdx++
				if fp.isSaveOrExport() && !fp.entries[fp.selectedIdx].IsDir {
					fp.inputBuffer = []rune(fp.entries[fp.selectedIdx].Name)
					fp.cursorPos = len(fp.inputBuffer)
				}
			}
		}
		return false, "", false

	case tcell.KeyLeft, tcell.KeyCtrlB:
		fp.errorMessage = ""
		if fp.inputActive {
			if fp.cursorPos > 0 {
				fp.cursorPos--
			}
		}
		return false, "", false

	case tcell.KeyRight, tcell.KeyCtrlF:
		fp.errorMessage = ""
		if fp.inputActive {
			if fp.cursorPos < len(fp.inputBuffer) {
				fp.cursorPos++
			}
		}
		return false, "", false

	case tcell.KeyHome, tcell.KeyCtrlA:
		fp.errorMessage = ""
		fp.typeAhead = ""
		if fp.inputActive {
			fp.cursorPos = 0
		}
		return false, "", false

	case tcell.KeyEnd, tcell.KeyCtrlE:
		fp.errorMessage = ""
		fp.typeAhead = ""
		if fp.inputActive {
			fp.cursorPos = len(fp.inputBuffer)
		}
		return false, "", false

	case tcell.KeyDelete, tcell.KeyCtrlD:
		fp.errorMessage = ""
		if fp.inputActive {
			if fp.cursorPos < len(fp.inputBuffer) {
				fp.inputBuffer = append(fp.inputBuffer[:fp.cursorPos], fp.inputBuffer[fp.cursorPos+1:]...)
			}
		}
		return false, "", false

	case tcell.KeyCtrlU:
		fp.errorMessage = ""
		if fp.inputActive {
			fp.inputBuffer = nil
			fp.cursorPos = 0
		}
		return false, "", false

	case tcell.KeyCtrlK:
		fp.errorMessage = ""
		if fp.inputActive {
			if fp.cursorPos <= len(fp.inputBuffer) {
				fp.inputBuffer = fp.inputBuffer[:fp.cursorPos]
			}
		}
		return false, "", false

	case tcell.KeyPgUp:
		fp.errorMessage = ""
		fp.typeAhead = ""
		if !fp.inputActive && len(fp.entries) > 0 {
			fp.selectedIdx -= 8
			if fp.selectedIdx < 0 {
				fp.selectedIdx = 0
			}
			if fp.isSaveOrExport() && !fp.entries[fp.selectedIdx].IsDir {
				fp.inputBuffer = []rune(fp.entries[fp.selectedIdx].Name)
				fp.cursorPos = len(fp.inputBuffer)
			}
		}
		return false, "", false

	case tcell.KeyPgDn:
		fp.errorMessage = ""
		fp.typeAhead = ""
		if !fp.inputActive && len(fp.entries) > 0 {
			fp.selectedIdx += 8
			if fp.selectedIdx >= len(fp.entries) {
				fp.selectedIdx = len(fp.entries) - 1
			}
			if fp.isSaveOrExport() && !fp.entries[fp.selectedIdx].IsDir {
				fp.inputBuffer = []rune(fp.entries[fp.selectedIdx].Name)
				fp.cursorPos = len(fp.inputBuffer)
			}
		}
		return false, "", false

	case tcell.KeyEnter:
		fp.errorMessage = ""
		fp.typeAhead = ""
		if fp.inputActive && fp.isSaveOrExport() {
			fn := strings.TrimSpace(string(fp.inputBuffer))
			if err := ValidateFilename(fn); err != nil {
				fp.errorMessage = err.Error()
				return false, "", false
			}
			if fp.mode == FilePickerModeSave {
				fn = NormalizeHwkSaveFilename(fn)
			} else if fp.mode == FilePickerModeExportCSV && filepath.Ext(fn) == "" {
				fn += ".csv"
			} else if fp.mode == FilePickerModeExportXLSX && filepath.Ext(fn) == "" {
				fn += ".xlsx"
			} else if fp.mode == FilePickerModeExportODS && filepath.Ext(fn) == "" {
				fn += ".ods"
			} else if fp.mode == FilePickerModeExportMarkdown && filepath.Ext(fn) == "" {
				fn += ".md"
			}
			full := filepath.Join(fp.currentDir, fn)
			if _, err := os.Stat(full); err == nil {
				// File exists! Prompt for overwrite confirmation
				fp.confirmOverwrite = true
				fp.pendingSavePath = full
				return false, "", false
			}
			fp.Close()
			return true, full, false
		}

		if len(fp.entries) > 0 && fp.selectedIdx >= 0 && fp.selectedIdx < len(fp.entries) {
			entry := fp.entries[fp.selectedIdx]
			if entry.IsDir {
				if entry.Name == ".." {
					fp.currentDir = filepath.Dir(fp.currentDir)
				} else {
					fp.currentDir = filepath.Join(fp.currentDir, entry.Name)
				}
				fp.readDirectory()
				fp.selectedIdx = 0
				return false, "", false
			} else {
				if fp.isSaveOrExport() {
					if err := ValidateFilename(entry.Name); err != nil {
						fp.errorMessage = err.Error()
						return false, "", false
					}
					fp.inputBuffer = []rune(entry.Name)
					fp.cursorPos = len(fp.inputBuffer)
					full := filepath.Join(fp.currentDir, entry.Name)
					if _, err := os.Stat(full); err == nil {
						// File exists! Prompt for overwrite confirmation
						fp.confirmOverwrite = true
						fp.pendingSavePath = full
						return false, "", false
					}
					fp.Close()
					return true, full, false
				}
				full := filepath.Join(fp.currentDir, entry.Name)
				fp.Close()
				return true, full, false
			}
		}
		return false, "", false

	case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyCtrlH:
		fp.errorMessage = ""
		if fp.inputActive {
			if fp.cursorPos > 0 && len(fp.inputBuffer) > 0 {
				fp.inputBuffer = append(fp.inputBuffer[:fp.cursorPos-1], fp.inputBuffer[fp.cursorPos:]...)
				fp.cursorPos--
			}
		} else {
			if len(fp.typeAhead) > 0 {
				fp.typeAhead = fp.typeAhead[:len(fp.typeAhead)-1]
				if len(fp.typeAhead) > 0 {
					prefixLower := strings.ToLower(fp.typeAhead)
					for i, e := range fp.entries {
						if strings.HasPrefix(strings.ToLower(e.Name), prefixLower) {
							fp.selectedIdx = i
							if fp.isSaveOrExport() && !e.IsDir {
								fp.inputBuffer = []rune(e.Name)
								fp.cursorPos = len(fp.inputBuffer)
							}
							break
						}
					}
				}
				return false, "", false
			}
			// Navigate to parent directory
			parent := filepath.Dir(fp.currentDir)
			if parent != fp.currentDir {
				fp.currentDir = parent
				fp.readDirectory()
				fp.selectedIdx = 0
			}
		}
		return false, "", false

	case tcell.KeyRune:
		fp.errorMessage = ""
		r := ev.Rune()
		if fp.inputActive {
			if fp.cursorPos < 0 {
				fp.cursorPos = 0
			}
			if fp.cursorPos > len(fp.inputBuffer) {
				fp.cursorPos = len(fp.inputBuffer)
			}
			fp.inputBuffer = append(fp.inputBuffer[:fp.cursorPos], append([]rune{r}, fp.inputBuffer[fp.cursorPos:]...)...)
			fp.cursorPos++
		} else {
			// Multi-character incremental prefix search (Type-Ahead)
			if time.Since(fp.lastTypeAheadTime) > 1500*time.Millisecond || fp.typeAhead == "" {
				fp.typeAhead = string(r)
			} else {
				fp.typeAhead += string(r)
			}
			fp.lastTypeAheadTime = time.Now()

			prefixLower := strings.ToLower(fp.typeAhead)
			found := false
			for i, e := range fp.entries {
				if strings.HasPrefix(strings.ToLower(e.Name), prefixLower) {
					fp.selectedIdx = i
					if fp.isSaveOrExport() && !e.IsDir {
						fp.inputBuffer = []rune(e.Name)
						fp.cursorPos = len(fp.inputBuffer)
					}
					found = true
					break
				}
			}
			// If not found with extended buffer, try restarting search with just the current rune
			if !found && len(fp.typeAhead) > 1 {
				singleChar := string(r)
				for i, e := range fp.entries {
					if strings.HasPrefix(strings.ToLower(e.Name), strings.ToLower(singleChar)) {
						fp.typeAhead = singleChar
						fp.selectedIdx = i
						if fp.isSaveOrExport() && !e.IsDir {
							fp.inputBuffer = []rune(e.Name)
							fp.cursorPos = len(fp.inputBuffer)
						}
						found = true
						break
					}
				}
			}
		}
		return false, "", false
	}

	return false, "", false
}

func (fp *FilePicker) Draw(s tcell.Screen, styles Styles) {
	if !fp.active {
		return
	}

	w, h := s.Size()
	modalW := 74
	if modalW > w-4 {
		modalW = w - 4
	}
	modalH := 18
	if modalH > h-4 {
		modalH = h - 4
	}

	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2
	if modalY < 1 {
		modalY = 1
	}

	boxStyle := styles.Header
	itemStyle := styles.Default
	selStyle := styles.MenuSel
	dirStyle := styles.GridNumber
	subStyle := styles.Status

	// 1. Draw Background Box
	for y := modalY; y < modalY+modalH; y++ {
		for x := modalX; x < modalX+modalW; x++ {
			s.SetContent(x, y, ' ', nil, itemStyle)
		}
	}

	// 2. Borders
	for x := modalX; x < modalX+modalW; x++ {
		s.SetContent(x, modalY, '═', nil, boxStyle)
		s.SetContent(x, modalY+2, '═', nil, boxStyle)
		s.SetContent(x, modalY+modalH-2, '═', nil, boxStyle)
		s.SetContent(x, modalY+modalH-1, '═', nil, boxStyle)
	}
	for y := modalY; y < modalY+modalH; y++ {
		s.SetContent(modalX, y, '║', nil, boxStyle)
		s.SetContent(modalX+modalW-1, y, '║', nil, boxStyle)
	}
	s.SetContent(modalX, modalY, '╔', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY, '╗', nil, boxStyle)
	s.SetContent(modalX, modalY+2, '╠', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY+2, '╣', nil, boxStyle)
	s.SetContent(modalX, modalY+modalH-2, '╠', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY+modalH-2, '╣', nil, boxStyle)
	s.SetContent(modalX, modalY+modalH-1, '╚', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY+modalH-1, '╝', nil, boxStyle)

	// 3. Title
	drawTextFast(s, modalX+(modalW-len(fp.title))/2, modalY, fp.title, boxStyle, w)

	// 4. Current Directory Bar (Line 1)
	dirDisplay := "Dir: " + runewidth.Truncate(fp.currentDir, modalW-8, "...")
	drawTextFast(s, modalX+2, modalY+1, dirDisplay, dirStyle, modalX+modalW-2)

	// 5. In Save/Export Mode: Show filename input line (Line 3)
	listStartY := modalY + 3
	listHeight := modalH - 5

	if fp.isSaveOrExport() {
		listStartY = modalY + 4
		listHeight = modalH - 6

		inputPrompt := "Filename: "
		drawTextFast(s, modalX+2, modalY+3, inputPrompt, dirStyle, modalX+modalW-2)

		inX := modalX + 12
		maxInW := modalW - 15

		inpBoxStyle := styles.Default
		if fp.inputActive {
			inpBoxStyle = tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorWhite)
		}
		cursorStyle := tcell.StyleDefault.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack).Bold(true)

		for x := inX; x < inX+maxInW; x++ {
			s.SetContent(x, modalY+3, ' ', nil, inpBoxStyle)
		}

		if fp.cursorPos < 0 {
			fp.cursorPos = 0
		}
		if fp.cursorPos > len(fp.inputBuffer) {
			fp.cursorPos = len(fp.inputBuffer)
		}

		curX := inX
		cursorVisualX := inX
		for i, r := range fp.inputBuffer {
			rw := runewidth.RuneWidth(r)
			if curX+rw > inX+maxInW {
				break
			}
			style := inpBoxStyle
			if fp.inputActive && i == fp.cursorPos {
				style = cursorStyle
				cursorVisualX = curX
			}
			s.SetContent(curX, modalY+3, r, nil, style)
			for w := 1; w < rw; w++ {
				s.SetContent(curX+w, modalY+3, ' ', nil, style)
			}
			curX += rw
		}

		if fp.inputActive && fp.cursorPos >= len(fp.inputBuffer) && curX < inX+maxInW {
			s.SetContent(curX, modalY+3, ' ', nil, cursorStyle)
			cursorVisualX = curX
		}

		if fp.inputActive {
			s.ShowCursor(cursorVisualX, modalY+3)
		}
	}

	// 6. File List Items
	maxVisible := listHeight
	if maxVisible < 1 {
		maxVisible = 1
	}

	fp.adjustViewport(maxVisible)
	startIdx := fp.topIdx

	for i := 0; i < maxVisible; i++ {
		idx := startIdx + i
		itemY := listStartY + i
		if idx >= len(fp.entries) {
			break
		}

		entry := fp.entries[idx]
		isSelected := (idx == fp.selectedIdx && !fp.inputActive)
		lineStyle := itemStyle
		if isSelected {
			lineStyle = selStyle
		}

		for x := modalX + 1; x < modalX+modalW-1; x++ {
			s.SetContent(x, itemY, ' ', nil, lineStyle)
		}

		// Icon & Name (ASCII only — emoji breaks on terminals without emoji fonts, e.g. Raspberry Pi)
		prefix := "  "
		nameStyle := lineStyle
		if entry.IsDir {
			prefix = "+ "
			if !isSelected {
				nameStyle = dirStyle
			}
		}

		displayName := prefix + entry.Name
		nameW := modalW - 30
		drawTextFast(s, modalX+2, itemY, runewidth.Truncate(displayName, nameW, ".."), nameStyle, modalX+modalW-2)

		// Size column
		sizeStr := "<DIR>"
		if !entry.IsDir {
			sizeStr = formatFileSize(entry.Size)
		}
		drawTextFast(s, modalX+modalW-24, itemY, sizeStr, subStyle, modalX+modalW-2)

		// Modified Date column
		if !entry.ModTime.IsZero() {
			dateStr := entry.ModTime.Format("01/02 15:04")
			drawTextFast(s, modalX+modalW-14, itemY, dateStr, subStyle, modalX+modalW-2)
		}
	}

	if fp.errorMessage != "" {
		drawTextFast(s, modalX+2, modalY+modalH-2, "Error: "+fp.errorMessage, styles.Error, modalX+modalW-2)
	} else if fp.typeAhead != "" && !fp.inputActive && time.Since(fp.lastTypeAheadTime) < 3*time.Second {
		drawTextFast(s, modalX+2, modalY+modalH-2, fmt.Sprintf("Jump to: [%s]", fp.typeAhead), styles.MenuSel, modalX+modalW-2)
	} else if len(fp.entries) == 0 {
		drawTextFast(s, modalX+4, listStartY+1, "No matching files or directories found.", styles.Error, modalX+modalW-2)
	}

	// 7. Footer Guides (Bottom line)
	footer := "[Enter: Open/Select]  [Tab: Filename]  [Esc: Cancel]"
	if fp.isSaveOrExport() {
		footer = "[Enter: Save/Export]  [←/→: Move Cursor]  [Tab: List Focus]  [Esc: Cancel]"
	} else if fp.mode == FilePickerModeOpen || fp.mode == FilePickerModeImportCSV {
		footer = "[Enter: Open File / Enter Dir]  [Backspace: Up Dir]  [Esc: Cancel]"
	}
	drawTextFast(s, modalX+2, modalY+modalH-1, footer, boxStyle, modalX+modalW-2)

	// 8. Draw Overwrite Confirmation Dialog if active
	if fp.confirmOverwrite {
		fp.drawOverwriteModal(s, styles, w, h)
	}
}

func (fp *FilePicker) drawOverwriteModal(s tcell.Screen, styles Styles, w, h int) {
	cW := 60
	cH := 8
	if cW > w-4 {
		cW = w - 4
	}
	if cH > h-2 {
		cH = h - 2
	}
	cX := (w - cW) / 2
	cY := (h - cH) / 2

	boxStyle := styles.Header
	bgStyle := styles.Default
	hotKeyStyle := styles.ModeBox
	itemStyle := styles.Default

	// Clear background
	for y := cY; y < cY+cH; y++ {
		for x := cX; x < cX+cW; x++ {
			s.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Double Border: ╔ ╗ ╚ ╝ ║ ═
	for x := cX; x < cX+cW; x++ {
		s.SetContent(x, cY, '═', nil, boxStyle)
		s.SetContent(x, cY+cH-1, '═', nil, boxStyle)
	}
	for y := cY; y < cY+cH; y++ {
		s.SetContent(cX, y, '║', nil, boxStyle)
		s.SetContent(cX+cW-1, y, '║', nil, boxStyle)
	}
	s.SetContent(cX, cY, '╔', nil, boxStyle)
	s.SetContent(cX+cW-1, cY, '╗', nil, boxStyle)
	s.SetContent(cX, cY+cH-1, '╚', nil, boxStyle)
	s.SetContent(cX+cW-1, cY+cH-1, '╝', nil, boxStyle)

	title := " OVERWRITE FILE "
	titleX := cX + (cW-len(title))/2
	drawTextFast(s, titleX, cY, title, boxStyle, w)

	baseName := filepath.Base(fp.pendingSavePath)
	msg1 := fmt.Sprintf("File '%s' already exists.", baseName)
	msg2 := "Do you want to overwrite this file?"
	drawTextFast(s, cX+4, cY+2, msg1, styles.Error, w)
	drawTextFast(s, cX+4, cY+3, msg2, itemStyle, w)

	// Buttons
	btnY := cY + 5
	bX := cX + 6
	bX = drawTextFast(s, bX, btnY, "[", itemStyle, w)
	bX = drawTextFast(s, bX, btnY, "Y", hotKeyStyle, w)
	bX = drawTextFast(s, bX, btnY, "] Overwrite        ", itemStyle, w)

	bX = drawTextFast(s, bX, btnY, "[", itemStyle, w)
	bX = drawTextFast(s, bX, btnY, "N / Esc", hotKeyStyle, w)
	drawTextFast(s, bX, btnY, "] Cancel", itemStyle, w)
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024.0)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024.0*1024.0))
}
