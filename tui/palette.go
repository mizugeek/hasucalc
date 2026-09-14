package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

type PaletteItem struct {
	Category string
	Name     string
	Shortcut string
	Action   string
}

var DefaultPaletteItems = []PaletteItem{
	{"Formula", "AutoSum (=SUM for selected range / column)", "Alt+=", "doPaletteAutoSum"},
	{"Formula", "Browse & Insert Function", "", "doOpenFunctionPicker"},
	{"Formula", "Average (=AVERAGE for selected range)", "", "doPaletteAverage"},
	{"Formula", "Count (=COUNT for range)", "", "doPaletteCount"},
	{"Formula", "Max (=MAX for range)", "", "doPaletteMax"},
	{"Formula", "Min (=MIN for range)", "", "doPaletteMin"},
	{"Formula", "Today (=TODAY() current date)", "", "doPaletteToday"},
	{"Formula", "Now (=NOW() current date and time)", "", "doPaletteNow"},
	{"Formula", "Recalculate All Formulas", "F9", "doPaletteRecalc"},

	{"Edit", "Edit Active Cell Formula / Value", "Ctrl+E / F2", "doEditActiveCell"},
	{"Edit", "Undo Last Action", "Ctrl+Z", "doPaletteUndo"},
	{"Edit", "Redo Action", "Ctrl+Y", "doPaletteRedo"},
	{"Edit", "Cut Selection / Cell", "Ctrl+X", "doPaletteCut"},
	{"Edit", "Copy Selection / Cell", "Ctrl+C", "doPaletteCopy"},
	{"Edit", "Paste Clipboard", "Ctrl+V", "doPalettePaste"},
	{"Edit", "Paste Link (Reference to Source Cells)", "Ctrl+L", "doPalettePasteLink"},
	{"Edit", "Paste Values Only (No Formulas)", "", "doPalettePasteValues"},
	{"Edit", "Paste Transpose (Flip Rows & Columns)", "", "doPasteTranspose"},
	{"Edit", "Clear Selection / Cell", "Del", "doPaletteClear"},
	{"Edit", "Select All Data Block", "Ctrl+A", "doPaletteSelectAll"},
	{"Edit", "Go to Cell / Range / Sheet", "F5", "doPaletteGoto"},
	{"Edit", "Find Text / Number / Formula", "Ctrl+F", "doPaletteFind"},
	{"Edit", "Find and Replace", "Ctrl+H", "doPaletteReplace"},
	{"Edit", "Search in All Sheets", "", "doPaletteFindAll"},
	{"Edit", "Find Next Match", "F3", "doFindNext"},
	{"Edit", "Find Previous Match", "Shift+F3", "doFindPrev"},

	{"Format", "Currency Format (choose symbol; mix per cell OK)", "", "doPaletteFormatCurrency"},
	{"Format", "Percent Format (12.3%)", "", "doPaletteFormatPercent"},
	{"Format", "Fixed Decimal Format", "", "doPaletteFormatFixed"},
	{"Format", "Comma Format (1,234.56)", "", "doPaletteFormatComma"},
	{"Format", "Date Format", "", "doPaletteFormatDate"},
	{"Format", "Scientific Format", "", "doPaletteFormatScientific"},
	{"Format", "General Format", "", "doPaletteFormatGeneral"},
	{"Format", "Line-Single (\\- repeat line)", "", "doPaletteLineSingle"},
	{"Format", "Line-Double (\\= repeat line)", "", "doPaletteLineDouble"},

	{"Row", "Insert Row at cursor", "", "doPaletteInsertRow"},
	{"Row", "Delete Row at cursor", "", "doPaletteDeleteRow"},

	{"Column", "Insert Column at cursor", "", "doPaletteInsertCol"},
	{"Column", "Delete Column at cursor", "", "doPaletteDeleteCol"},
	{"Column", "Set Column Width", "", "doPaletteSetColWidth"},
	{"Column", "Reset Column Width to default", "", "doResetColWidth"},
	{"Column", "Global Column Width", "", "doPaletteGlobalColWidth"},

	{"Data", "AutoFill Series across Selection", "", "doAutoFill"},
	{"Data", "Fill Range with Sequence", "", "doPaletteDataFill"},
	{"Data", "Fill Down", "Ctrl+D", "doFillDown"},
	{"Data", "Fill Right", "Ctrl+R", "doFillRight"},
	{"Data", "Transpose Range Rows & Columns", "", "doPaletteTranspose"},
	{"Data", "Sort Range Ascending (A-Z, 0-9)", "", "doPaletteSortAsc"},
	{"Data", "Sort Range Descending (Z-A, 9-0)", "", "doPaletteSortDesc"},
	{"Data", "Sort Columns Left-to-Right Ascending (by Row)", "", "doPaletteSortHorizAsc"},
	{"Data", "Sort Columns Left-to-Right Descending (by Row)", "", "doPaletteSortHorizDesc"},
	{"Data", "Show Range Names List", "", "doRangeNameList"},

	{"Sheet", "Freeze-Panes: Both Rows & Columns", "", "doFreezeBoth"},
	{"Sheet", "Freeze-Panes: Horizontal Rows at Top", "", "doFreezeHorizontal"},
	{"Sheet", "Freeze-Panes: Vertical Columns at Left", "", "doFreezeVertical"},
	{"Sheet", "Freeze-Panes: Clear & Unfreeze", "", "doFreezeClear"},
	{"Sheet", "Switch Active Worksheet", "Ctrl+T", "doSheetSwitchModal"},
	{"Sheet", "Add Worksheet", "", "doPaletteAddSheet"},
	{"Sheet", "Delete Active Worksheet", "", "doWorksheetDelete"},
	{"Sheet", "Rename Active Worksheet", "", "doPaletteRenameSheet"},

	{"Chart", "View Chart", "F10", "doPaletteGraphView"},
	{"Chart", "Set Chart Type: Line", "", "doPaletteGraphLine"},
	{"Chart", "Set Chart Type: Bar", "", "doPaletteGraphBar"},
	{"Chart", "Set Chart Type: Stacked-Bar", "", "doPaletteGraphStacked"},
	{"Chart", "Set Chart Type: Pie", "", "doPaletteGraphPie"},
	{"Chart", "Set X-Axis Range", "", "doPaletteGraphSetX"},
	{"Chart", "Set Series A Range", "", "doPaletteGraphSetA"},
	{"Chart", "Set Series B Range", "", "doPaletteGraphSetB"},
	{"Chart", "Set Series C Range", "", "doPaletteGraphSetC"},
	{"Chart", "Set Series D Range", "", "doPaletteGraphSetD"},
	{"Chart", "Set Series E Range", "", "doPaletteGraphSetE"},
	{"Chart", "Set Series F Range", "", "doPaletteGraphSetF"},
	{"Chart", "Set Chart Title", "", "doPaletteGraphSetTitle"},
	{"Chart", "Show Chart Settings & Series Status", "", "doPaletteGraphStatus"},
	{"Chart", "Export Chart Image (.png)", "", "doGraphSavePNG"},

	{"File", "New Worksheet (Blank)", "", "doWorksheetErase"},
	{"File", "Open File (.hwk, .xlsx, .ods, .csv, .md, .html, .json)", "Ctrl+O", "doFileOpenDialog"},
	{"File", "Save Worksheet", "Ctrl+S", "doFileSaveDialog"},
	{"File", "Export CSV File (Active Sheet)", "", "doFileExportCSVFullDialog"},
	{"File", "Export Excel (.xlsx) Workbook (All Sheets)", "", "doFileExportXLSXFullDialog"},
	{"File", "Export OpenDocument (.ods) Workbook (All Sheets)", "", "doFileExportODSFullDialog"},
	{"File", "Export Markdown Table (.md)", "", "doFileExportMarkdownDialog"},
	{"File", "Export Range to CSV (Values Only)", "", "doFileExportCSVRangeDialog"},
	{"File", "Export Range to Excel (.xlsx) (Values Only)", "", "doFileExportXLSXRangeDialog"},
	{"File", "Export Range to OpenDocument (.ods) (Values Only)", "", "doFileExportODSRangeDialog"},
	{"File", "Export Range to Markdown (.md)", "", "doFileExportMarkdownRangeDialog"},
	{"File", "Quit Application", "Ctrl+Q", "doQuitApp"},

	{"Help", "About HasuCalc, Version, MIT License", "", "doOpenAbout"},
	{"Help", "Show Keybindings & Help", "F1", "doPaletteHelp"},
	{"Help", "Open Command Palette", "Ctrl+K", "doOpenPalette"},
}

type CommandPalette struct {
	active      bool
	query       []rune
	items       []PaletteItem
	filtered    []PaletteItem
	selectedIdx int
	topIdx      int
}
func NewCommandPalette() *CommandPalette {
	return &CommandPalette{
		active:   false,
		query:    nil,
		items:    DefaultPaletteItems,
		filtered: DefaultPaletteItems,
	}
}

func (cp *CommandPalette) Open() {
	cp.active = true
	cp.query = nil
	cp.filter()
	cp.selectedIdx = 0
	cp.topIdx = 0
}

func (cp *CommandPalette) Close() {
	cp.active = false
	cp.query = nil
}

func (cp *CommandPalette) IsActive() bool {
	return cp.active
}

func (cp *CommandPalette) filter() {
	q := strings.ToLower(strings.TrimSpace(string(cp.query)))
	if q == "" {
		cp.filtered = cp.items
		if cp.selectedIdx >= len(cp.filtered) {
			cp.selectedIdx = 0
		}
		cp.topIdx = 0
		return
	}

	terms := strings.Fields(q)
	var res []PaletteItem
	for _, item := range cp.items {
		target := strings.ToLower(item.Category + " " + item.Name + " " + item.Shortcut)
		match := true
		for _, term := range terms {
			if !strings.Contains(target, term) {
				match = false
				break
			}
		}
		if match {
			res = append(res, item)
		}
	}
	cp.filtered = res
	if cp.selectedIdx >= len(cp.filtered) {
		cp.selectedIdx = 0
	}
	cp.topIdx = 0
}

func (cp *CommandPalette) adjustViewport(maxVisible int) {
	if maxVisible <= 0 {
		return
	}
	if cp.selectedIdx < cp.topIdx {
		cp.topIdx = cp.selectedIdx
	} else if cp.selectedIdx >= cp.topIdx+maxVisible {
		cp.topIdx = cp.selectedIdx - maxVisible + 1
	}
	if cp.topIdx > len(cp.filtered)-maxVisible {
		cp.topIdx = len(cp.filtered) - maxVisible
	}
	if cp.topIdx < 0 {
		cp.topIdx = 0
	}
}

func (cp *CommandPalette) HandleKey(ev *tcell.EventKey) (bool, *PaletteItem) {
	if !cp.active {
		return false, nil
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		cp.Close()
		return true, nil

	case tcell.KeyUp, tcell.KeyCtrlP:
		if len(cp.filtered) > 0 && cp.selectedIdx > 0 {
			cp.selectedIdx--
		}
		return true, nil

	case tcell.KeyDown, tcell.KeyCtrlN:
		if len(cp.filtered) > 0 && cp.selectedIdx < len(cp.filtered)-1 {
			cp.selectedIdx++
		}
		return true, nil

	case tcell.KeyEnter:
		if len(cp.filtered) > 0 && cp.selectedIdx >= 0 && cp.selectedIdx < len(cp.filtered) {
			item := cp.filtered[cp.selectedIdx]
			cp.Close()
			return true, &item
		}
		cp.Close()
		return true, nil

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(cp.query) > 0 {
			cp.query = cp.query[:len(cp.query)-1]
			cp.filter()
		}
		return true, nil

	case tcell.KeyRune:
		cp.query = append(cp.query, ev.Rune())
		cp.filter()
		return true, nil
	}

	return true, nil
}

func (cp *CommandPalette) Draw(s tcell.Screen, styles Styles) {
	if !cp.active {
		return
	}

	w, h := s.Size()
	modalW := 68
	if modalW > w-4 {
		modalW = w - 4
	}
	maxVisibleItems := 8
	modalH := maxVisibleItems + 5
	if modalH > h-4 {
		modalH = h - 4
		maxVisibleItems = modalH - 5
		if maxVisibleItems < 1 {
			maxVisibleItems = 1
		}
	}

	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2
	if modalY < 1 {
		modalY = 1
	}

	boxStyle := styles.Header
	itemStyle := styles.Default
	selStyle := styles.MenuSel
	catStyle := styles.GridNumber

	// Draw Box Frame
	for y := modalY; y < modalY+modalH; y++ {
		for x := modalX; x < modalX+modalW; x++ {
			s.SetContent(x, y, ' ', nil, itemStyle)
		}
	}

	// Border
	for x := modalX; x < modalX+modalW; x++ {
		s.SetContent(x, modalY, '═', nil, boxStyle)
		s.SetContent(x, modalY+2, '═', nil, boxStyle)
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
	s.SetContent(modalX, modalY+modalH-1, '╚', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY+modalH-1, '╝', nil, boxStyle)

	// Title
	title := " COMMAND PALETTE (Ctrl+K) "
	drawTextFast(s, modalX+(modalW-len(title))/2, modalY, title, boxStyle, w)

	// Search Input Box
	queryStr := "> " + string(cp.query)
	drawTextFast(s, modalX+2, modalY+1, queryStr, boxStyle, modalX+modalW-2)
	s.ShowCursor(modalX+2+runewidth.StringWidth(queryStr), modalY+1)

	// Filtered Items
	cp.adjustViewport(maxVisibleItems)
	startIdx := cp.topIdx

	for i := 0; i < maxVisibleItems; i++ {
		idx := startIdx + i
		itemY := modalY + 3 + i
		if idx >= len(cp.filtered) {
			break
		}

		item := cp.filtered[idx]
		isSelected := (idx == cp.selectedIdx)
		lineStyle := itemStyle
		if isSelected {
			lineStyle = selStyle
		}

		// Fill line
		for x := modalX + 1; x < modalX+modalW-1; x++ {
			s.SetContent(x, itemY, ' ', nil, lineStyle)
		}

		catText := "[" + item.Category + "]"
		drawTextFast(s, modalX+2, itemY, catText, catStyle, modalX+modalW-2)

		nameX := modalX + 12
		maxNameW := modalW - 24
		nameText := runewidth.Truncate(item.Name, maxNameW, "..")
		drawTextFast(s, nameX, itemY, nameText, lineStyle, modalX+modalW-2)

		if item.Shortcut != "" {
			scX := modalX + modalW - 2 - len(item.Shortcut)
			drawTextFast(s, scX, itemY, item.Shortcut, lineStyle, modalX+modalW-2)
		}
	}

	if len(cp.filtered) == 0 {
		drawTextFast(s, modalX+4, modalY+4, "No matching commands found.", styles.Error, modalX+modalW-2)
	}
}
