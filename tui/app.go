package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
)

var (
	colLetterCache [1024]string
	spaceCache     [128]string
)

func init() {
	for i := 0; i < 1024; i++ {
		colLetterCache[i] = coord.ColToLetter(i)
	}
	for i := 0; i < 128; i++ {
		spaceCache[i] = strings.Repeat(" ", i)
	}
}

func getSpaces(w int) string {
	if w <= 0 {
		return ""
	}
	if w < 128 {
		return spaceCache[w]
	}
	return strings.Repeat(" ", w)
}

func getColLetter(col int) string {
	if col >= 0 && col < 1024 {
		return colLetterCache[col]
	}
	return coord.ColToLetter(col)
}

func getRowHeaderWidth(maxRow int) int {
	r := maxRow + 1
	digits := 1
	for r >= 10 {
		digits++
		r /= 10
	}
	if digits < 3 {
		digits = 3
	}
	return digits + 1 // trailing space
}

func drawRowNumFast(s tcell.Screen, y int, rowIdx int, width int, style tcell.Style) {
	val := rowIdx + 1
	var buf [16]rune
	n := 0
	for val > 0 {
		buf[n] = rune('0' + (val % 10))
		n++
		val /= 10
	}
	padSpaces := (width - 1) - n
	if padSpaces < 0 {
		padSpaces = 0
	}
	for x := 0; x < padSpaces; x++ {
		s.SetContent(x, y, ' ', nil, style)
	}
	curX := padSpaces
	for i := n - 1; i >= 0; i-- {
		s.SetContent(curX, y, buf[i], nil, style)
		curX++
	}
	for curX < width {
		s.SetContent(curX, y, ' ', nil, style)
		curX++
	}
}

var (
	statRightNormal = "[Ctrl+Z:Undo Ctrl+K:Cmds /:Menu]"
	statRightCalc   = "[CALC] [Ctrl+Z:Undo Ctrl+K:Cmds /:Menu]"
)

func formatHotKeyLabel(name, key string) string {
	if key == "" {
		return name
	}
	return fmt.Sprintf("%s[%s]", name, strings.ToUpper(key))
}

// topMenuLabel returns a menu-bar label. When short is true, long names are
// abbreviated so File…Help fits on classic 80-column terminals.
func topMenuLabel(item *MenuItem, short bool) string {
	name := item.Name
	if short {
		switch item.Name {
		case "Format":
			name = "Fmt"
		case "Column":
			name = "Col"
		case "Formula":
			name = "Fx"
		}
	}
	return formatHotKeyLabel(name, item.Key)
}

func menuBarWidth(items []*MenuItem, short bool) int {
	if len(items) == 0 {
		return 0
	}
	w := 0
	for i, it := range items {
		if i > 0 {
			w++
		}
		w += runewidth.StringWidth(topMenuLabel(it, short))
	}
	return w
}

func useShortMenuBar(items []*MenuItem, termW int) bool {
	return menuBarWidth(items, false) >= termW && menuBarWidth(items, true) < termW
}

type colLayout struct {
	colIdx int
	x      int
	width  int
}

type ConfirmAction int

const (
	ConfirmNone ConfirmAction = iota
	ConfirmQuit
	ConfirmNew
)

type App struct {
	screen   tcell.Screen
	workbook *sheet.Workbook
	sheet    *sheet.Sheet
	styles   Styles

	cursorCol int
	cursorRow int
	leftCol   int
	topRow    int

	visibleCols []colLayout

	mode           string // "READY", "INPUT", "EDIT", "MENU", "POINT", "PROMPT"
	inputBuffer    []rune
	inputCursorPos int

	rootMenu  *MenuItem
	menuStack []*menuState

	promptChain      []PromptItem
	currentPromptIdx int
	promptResults    map[string]string
	promptHandler    string
	promptText       string

	selectionAnchor   *[2]int // [col, row]
	selectedRange     *coord.RangeRef
	clipboardRange    *coord.RangeRef
	clipboard         *ClipboardData
	hasClipboard      bool
	cutUndoID         uint64
	savedCutClipboard *ClipboardData
	cutRedoPending    bool

	pointAnchor    *[2]int // [col, row]
	pointOriginCol int
	pointOriginRow int

	palette     *CommandPalette
	undoManager *sheet.UndoManager
	filePicker  *FilePicker

	pendingExportRange *coord.RangeRef
	lastSearchQuery    string
	lastReplaceText    string
	replaceScopeAll    bool
	sheetTabBounds     []tabBound
	isMouseDown        bool
	mouseAnchor        *[2]int

	statusMessage  string
	filename       string
	endMode        bool
	lastTimeUpdate time.Time
	cachedTimeStr  string

	confirmAction ConfirmAction
	running       bool

	sortDataRange  *coord.RangeRef
	sortPrimaryCol int
	sortPrimaryAsc bool

	sheetViews map[string]sheetViewState
}

type sheetViewState struct {
	cursorCol, cursorRow int
	leftCol, topRow      int
}

type tabBound struct {
	sheetIdx int
	startX   int
	endX     int
}

type menuState struct {
	menu        *MenuItem
	selectedIdx int
	xPos        int
}

func (a *App) currentWorkbook() *sheet.Workbook {
	if a.workbook == nil {
		a.workbook = sheet.NewWorkbook(a.filename)
		if a.sheet != nil {
			a.workbook.Sheets[0] = a.sheet
			a.sheet.SetWorkbook(a.workbook)
		} else {
			a.sheet = a.workbook.GetActiveSheet()
		}
	}
	return a.workbook
}

func (a *App) saveSheetView(name string) {
	if a.sheetViews == nil {
		a.sheetViews = make(map[string]sheetViewState)
	}
	a.sheetViews[name] = sheetViewState{
		cursorCol: a.cursorCol,
		cursorRow: a.cursorRow,
		leftCol:   a.leftCol,
		topRow:    a.topRow,
	}
}

func (a *App) restoreSheetView(name string) {
	vs, ok := a.sheetViews[name]
	if !ok {
		a.cursorCol, a.cursorRow = 0, 0
		a.leftCol, a.topRow = 0, 0
		return
	}
	a.cursorCol = vs.cursorCol
	a.cursorRow = vs.cursorRow
	a.leftCol = vs.leftCol
	a.topRow = vs.topRow
	if a.sheet != nil {
		if a.cursorCol >= a.sheet.MaxCols() {
			a.cursorCol = a.sheet.MaxCols() - 1
		}
		if a.cursorRow >= a.sheet.MaxRows() {
			a.cursorRow = a.sheet.MaxRows() - 1
		}
	}
}

func (a *App) switchSheet(idx int) {
	wb := a.currentWorkbook()
	if idx < 0 || idx >= len(wb.Sheets) {
		return
	}
	if a.sheet != nil {
		a.saveSheetView(a.sheet.Name())
	}
	wb.ActiveSheetIndex = idx
	a.sheet = wb.Sheets[idx]
	a.sheet.SetWorkbook(wb)
	// Only recalc if something is marked dirty — switching alone shouldn't recompute the book.
	needs := false
	for _, s := range wb.Sheets {
		if s != nil && s.NeedsRecalc() {
			needs = true
			break
		}
	}
	if needs {
		wb.RecalculateAll()
	}
	a.restoreSheetView(a.sheet.Name())
	a.clearSelection()
	a.statusMessage = fmt.Sprintf("Switched to sheet [%s]", a.sheet.Name())
}

func NewApp(screen tcell.Screen, sh *sheet.Sheet, filename string) *App {
	if filename == "" || filename == "DATA.WK3" {
		filename = GenerateUnusedFilename(".", "DATA", ".hwk")
	} else {
		filename = NormalizeHwkSaveFilename(filename)
	}
	filename = AbsolutePath(filename)
	var wb *sheet.Workbook
	if sh != nil && sh.Workbook() != nil {
		wb = sh.Workbook()
	} else {
		wb = sheet.NewWorkbook(filename)
		if sh != nil {
			wb.Sheets[0] = sh
			sh.SetWorkbook(wb)
		} else {
			sh = wb.GetActiveSheet()
		}
	}
	app := &App{
		screen:         screen,
		workbook:       wb,
		sheet:          sh,
		styles:         InitStyles(),
		cursorCol:      0,
		cursorRow:      0,
		leftCol:        0,
		topRow:         0,
		visibleCols:    make([]colLayout, 0, 32),
		mode:           "READY",
		inputBuffer:    nil,
		inputCursorPos: 0,
		rootMenu:       BuildMenuTree(),
		menuStack:      nil,
		promptResults:  make(map[string]string),
		palette:        NewCommandPalette(),
		undoManager:    sheet.NewUndoManager(50),
		filePicker:     NewFilePicker(),
		filename:       filename,
		confirmAction:  ConfirmNone,
		running:        true,
		sortPrimaryAsc: true,
		sheetViews:     make(map[string]sheetViewState),
	}
	screen.EnableMouse(tcell.MouseButtonEvents | tcell.MouseMotionEvents)
	return app
}

func (a *App) LoadFileForTest(fn string) {
	a.loadFile(fn)
}

func (a *App) RunOnceForTest() {
	a.adjustViewport()
	a.drawScreen()
	a.screen.Show()
}

func (a *App) HandleEventForTest(ev tcell.Event) {
	a.processEvent(ev)
}

func (a *App) GetCursorForTest() (col, row int) {
	return a.cursorCol, a.cursorRow
}

// ImportCSVFileForTest runs CSV import (same path as the file picker) for tests.
func (a *App) ImportCSVFileForTest(fn string) {
	a.importCSVFile(fn)
}

func (a *App) GetModeForTest() string {
	return a.mode
}

func (a *App) GetViewportForTest() (leftCol, topRow int) {
	return a.leftCol, a.topRow
}

func (a *App) SetCursorForTest(col, row int) {
	a.cursorCol, a.cursorRow = col, row
}

func (a *App) SetViewportForTest(leftCol, topRow int) {
	a.leftCol, a.topRow = leftCol, topRow
}

func (a *App) SelectRangeForTest(r *coord.RangeRef) {
	a.selectedRange = r
}

func (a *App) TriggerAutoFunctionForTest(fnName string) {
	a.doAutoFunction(fnName)
}

func (a *App) TriggerAutoFillForTest() {
	a.doAutoFill()
}

func (a *App) GetInputBufferForTest() string {
	return string(a.inputBuffer)
}

func (a *App) GetStatusMessageForTest() string {
	return a.statusMessage
}

func (a *App) GetFilenameForTest() string {
	return a.filename
}

func (a *App) Run() error {
	for a.running {
		a.adjustViewport()
		a.drawScreen()
		a.screen.Show()

		ev := a.screen.PollEvent()
		if ev == nil {
			break
		}
		a.processEvent(ev)
	}
	return nil
}

func (a *App) ProcessEventForTest(ev tcell.Event) {
	a.processEvent(ev)
	a.adjustViewport()
	a.drawScreen()
	a.screen.Show()
}

func (a *App) SwitchSheetForTest(idx int) {
	a.switchSheet(idx)
}

func (a *App) ExecuteActionHandlerForTest(action string, res map[string]string) {
	a.dispatchAction(action, res)
}

func (a *App) processEvent(ev tcell.Event) {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		a.handleKeyEvent(tev)
	case *tcell.EventMouse:
		a.handleMouseEvent(tev)
	case *tcell.EventResize:
		a.screen.Sync()
	}
}

func (a *App) handleMouseEvent(ev *tcell.EventMouse) {
	if a.filePicker.IsActive() || a.palette.IsActive() || a.confirmAction != ConfirmNone {
		return
	}
	if a.mode == "MENU" || a.mode == "PROMPT" {
		return
	}

	mx, my := ev.Position()
	buttons := ev.Buttons()

	_, h := a.screen.Size()

	// 1. Mouse Wheel Scrolling (3 rows per tick)
	if buttons&tcell.WheelUp != 0 {
		a.cursorRow -= 3
		if a.cursorRow < 0 {
			a.cursorRow = 0
		}
		a.clearSelection()
		a.adjustViewport()
		return
	}
	if buttons&tcell.WheelDown != 0 {
		a.cursorRow += 3
		if a.cursorRow >= a.sheet.MaxRows() {
			a.cursorRow = a.sheet.MaxRows() - 1
		}
		a.clearSelection()
		a.adjustViewport()
		return
	}

	// 2. Button release (mouse button released)
	if buttons&tcell.Button1 == 0 {
		a.isMouseDown = false
		a.mouseAnchor = nil
		return
	}

	// 3. Sheet Tab Click (when multiple sheets, tab bar is at h-2)
	wb := a.currentWorkbook()
	hasMultipleSheets := len(wb.Sheets) > 1
	if hasMultipleSheets && my == h-2 && (buttons&tcell.Button1 != 0) {
		for _, b := range a.sheetTabBounds {
			if mx >= b.startX && mx <= b.endX && b.sheetIdx < len(wb.Sheets) {
				a.switchSheet(b.sheetIdx)
				return
			}
		}
	}

	// 4. Grid Cell Click / Drag Selection
	gridStartY := 3
	numGridRows := a.getNumGridRows(h)
	frozenR := a.displayFrozenRows(numGridRows)

	if my >= gridStartY && my < gridStartY+numGridRows {
		rOff := my - gridStartY
		var targetRow int
		if rOff < frozenR {
			targetRow = rOff
		} else {
			targetRow = a.topRow + (rOff - frozenR)
		}

		var targetCol int = -1
		for _, cl := range a.visibleCols {
			if mx >= cl.x && mx < cl.x+cl.width {
				targetCol = cl.colIdx
				break
			}
		}

		if targetCol != -1 && targetRow >= 0 && targetRow < a.sheet.MaxRows() {
			if !a.isMouseDown {
				// Initial mouse press on cell
				a.commitInputBuffer()
				a.isMouseDown = true
				a.mouseAnchor = &[2]int{targetCol, targetRow}
				a.cursorCol = targetCol
				a.cursorRow = targetRow
				a.clearSelection()
				a.adjustViewport()
			} else if a.mouseAnchor != nil {
				// Dragging across cells
				a.cursorCol = targetCol
				a.cursorRow = targetRow
				if targetCol != a.mouseAnchor[0] || targetRow != a.mouseAnchor[1] {
					minC := a.mouseAnchor[0]
					if targetCol < minC {
						minC = targetCol
					}
					maxC := a.mouseAnchor[0]
					if targetCol > maxC {
						maxC = targetCol
					}
					minR := a.mouseAnchor[1]
					if targetRow < minR {
						minR = targetRow
					}
					maxR := a.mouseAnchor[1]
					if targetRow > maxR {
						maxR = targetRow
					}
					rRef := coord.RangeRef{
						Start: coord.CellRef{Col: minC, Row: minR},
						End:   coord.CellRef{Col: maxC, Row: maxR},
					}
					a.selectedRange = &rRef
				} else {
					a.clearSelection()
				}
				a.adjustViewport()
			}
		}
	}
}

func (a *App) commitInputBuffer() {
	if a.mode != "INPUT" && a.mode != "EDIT" && a.mode != "POINT" {
		return
	}
	if len(a.inputBuffer) > 0 {
		a.pushUndo()
		a.sheet.SetCellInput(a.cursorCol, a.cursorRow, string(a.inputBuffer), nil)
	}
	a.inputBuffer = nil
	a.inputCursorPos = 0
	a.pointAnchor = nil
	a.mode = "READY"
}

func (a *App) getNumGridRows(h int) int {
	wb := a.currentWorkbook()
	hasMultipleSheets := len(wb.Sheets) > 1
	gridStartY := 3
	gridEndY := h - 2
	if hasMultipleSheets {
		gridEndY = h - 3
	}
	numGridRows := gridEndY - gridStartY + 1
	if numGridRows < 1 {
		numGridRows = 1
	}
	return numGridRows
}

func (a *App) displayFrozenRows(numGridRows int) int {
	frozenR := a.sheet.FrozenRows()
	if frozenR >= numGridRows && numGridRows > 0 {
		return numGridRows - 1
	}
	return frozenR
}

func (a *App) getRowHeaderW(h int) int {
	maxVisibleRow := a.topRow + a.getNumGridRows(h)
	if a.cursorRow > maxVisibleRow {
		maxVisibleRow = a.cursorRow
	}
	return getRowHeaderWidth(maxVisibleRow)
}

func (a *App) adjustViewport() {
	w, h := a.screen.Size()
	numGridRows := a.getNumGridRows(h)
	frozenR := a.displayFrozenRows(numGridRows)
	frozenC := a.sheet.FrozenCols()

	if a.topRow < frozenR {
		a.topRow = frozenR
	}
	if a.leftCol < frozenC {
		a.leftCol = frozenC
	}

	numScrolledRows := numGridRows - frozenR
	if numScrolledRows < 1 {
		numScrolledRows = 1
	}

	if a.cursorRow >= frozenR {
		if a.cursorRow < a.topRow {
			a.topRow = a.cursorRow
		} else if a.cursorRow >= a.topRow+numScrolledRows {
			a.topRow = a.cursorRow - numScrolledRows + 1
		}
	}

	rowHeaderW := a.getRowHeaderW(h)
	if a.cursorCol >= frozenC {
		if a.cursorCol < a.leftCol {
			a.leftCol = a.cursorCol
		} else {
			for a.leftCol < a.cursorCol {
				curX := rowHeaderW
				for c := 0; c < frozenC; c++ {
					curX += a.sheet.GetColWidth(c)
				}
				for c := a.leftCol; c <= a.cursorCol; c++ {
					curX += a.sheet.GetColWidth(c)
				}
				if curX <= w {
					break
				}
				a.leftCol++
			}
		}
	}
}

func (a *App) getFormattedTime() string {
	if a.cachedTimeStr == "" || time.Since(a.lastTimeUpdate) > time.Second {
		a.lastTimeUpdate = time.Now()
		a.cachedTimeStr = a.lastTimeUpdate.Format("2006/01/02  03:04 PM")
	}
	return a.cachedTimeStr
}

func (a *App) invalidateCutRedo() {
	a.cutRedoPending = false
	a.savedCutClipboard = nil
}

func (a *App) pushUndo() {
	a.invalidateCutRedo()
	a.undoManager.Push(a.sheet)
}

func (a *App) pushUndoSheets(sheets []*sheet.Sheet) {
	a.invalidateCutRedo()
	a.undoManager.PushMulti(sheets)
}

func (a *App) pushUndoWorkbook() {
	a.invalidateCutRedo()
	if wb := a.currentWorkbook(); wb != nil {
		a.undoManager.PushWorkbook(wb)
	} else {
		a.undoManager.Push(a.sheet)
	}
}

func (a *App) doUndo() {
	wb := a.currentWorkbook()
	resolve := func(name string) *sheet.Sheet {
		if wb == nil {
			return a.sheet
		}
		sh := wb.GetSheet(name)
		if sh == nil {
			return nil // deleted sheet — do not restore onto another sheet
		}
		if idx := wb.GetSheetIndex(sh); idx >= 0 {
			a.switchSheet(idx)
		}
		return sh
	}
	afterWB := func() {
		if wb == nil {
			return
		}
		a.sheet = wb.GetActiveSheet()
		a.clearSelection()
	}
	undoneID := a.undoManager.PeekUndoID()
	if a.undoManager.Undo(a.sheet, resolve, wb, afterWB) {
		a.clearSelection()
		if wb != nil {
			a.sheet = wb.GetActiveSheet()
		}
		if a.clipboard != nil && a.clipboard.IsCut && undoneID != 0 && undoneID == a.cutUndoID {
			a.savedCutClipboard = a.clipboard
			a.clipboard = nil
			a.hasClipboard = false
			a.clipboardRange = nil
			a.cutRedoPending = true
		}
		a.statusMessage = "Undo action completed."
	} else {
		a.statusMessage = "Nothing to undo."
	}
}

func (a *App) doRedo() {
	wb := a.currentWorkbook()
	resolve := func(name string) *sheet.Sheet {
		if wb == nil {
			return a.sheet
		}
		sh := wb.GetSheet(name)
		if sh == nil {
			return nil
		}
		if idx := wb.GetSheetIndex(sh); idx >= 0 {
			a.switchSheet(idx)
		}
		return sh
	}
	afterWB := func() {
		if wb == nil {
			return
		}
		a.sheet = wb.GetActiveSheet()
		a.clearSelection()
	}
	if a.undoManager.Redo(a.sheet, resolve, wb, afterWB) {
		a.clearSelection()
		if wb != nil {
			a.sheet = wb.GetActiveSheet()
		}
		if a.cutRedoPending && a.savedCutClipboard != nil {
			a.clipboard = a.savedCutClipboard
			a.hasClipboard = true
			rng := a.clipboard.Range
			a.clipboardRange = &rng
			a.cutUndoID = a.undoManager.PeekUndoID()
			a.cutRedoPending = false
			a.savedCutClipboard = nil
		}
		a.statusMessage = "Redo action completed."
	} else {
		a.statusMessage = "Nothing to redo."
	}
}

func clipRangeToSheetBounds(r coord.RangeRef, maxCols, maxRows int) coord.RangeRef {
	if r.Start.Col < 0 {
		r.Start.Col = 0
	}
	if r.Start.Row < 0 {
		r.Start.Row = 0
	}
	if r.Start.Col >= maxCols {
		r.Start.Col = maxCols - 1
	}
	if r.Start.Row >= maxRows {
		r.Start.Row = maxRows - 1
	}
	if r.End.Col < 0 {
		r.End.Col = 0
	}
	if r.End.Row < 0 {
		r.End.Row = 0
	}
	if r.End.Col >= maxCols {
		r.End.Col = maxCols - 1
	}
	if r.End.Row >= maxRows {
		r.End.Row = maxRows - 1
	}
	return r
}

func (a *App) drawScreen() {
	a.screen.Clear()
	w, h := a.screen.Size()
	if w < 10 || h < 4 {
		if w > 0 && h > 0 {
			drawTextFast(a.screen, 0, 0, "Too small", a.styles.Default, w)
			a.screen.Show()
		}
		return
	}

	currCell := a.sheet.GetCell(a.cursorCol, a.cursorRow)

	// ----------------------------------------------------
	// Line 0: Header status / Top Bar - Authentic Lotus 1-2-3 Layout
	// ----------------------------------------------------
	sheetNamePrefix := ""
	wbHeader := a.currentWorkbook()
	if wbHeader != nil && len(wbHeader.Sheets) > 1 {
		sheetNamePrefix = fmt.Sprintf("%s!", a.sheet.Name())
	}
	colLetter := getColLetter(a.cursorCol)
	rowNum := a.cursorRow + 1
	coordStr := fmt.Sprintf("%s%s%d", sheetNamePrefix, colLetter, rowNum)

	formatStr := ""
	if currCell != nil && currCell.FormatSpec != nil {
		formatStr = fmt.Sprintf("[%s] ", currCell.FormatSpec.String())
	}

	colWidthStr := fmt.Sprintf("[W%d] ", a.sheet.GetColWidth(a.cursorCol))

	var rawValueStr string
	if currCell != nil {
		rawValueStr = currCell.RawInput
	}

	leftHeader := fmt.Sprintf("%s: %s%s%s", coordStr, formatStr, colWidthStr, rawValueStr)
	drawTextFast(a.screen, 0, 0, leftHeader, a.styles.Default, w)

	modeStr := fmt.Sprintf("[%s]", a.mode)
	if a.endMode {
		modeStr = "[END]"
	}
	modeX := w - len(modeStr) - 1
	if modeX > 0 {
		drawTextFast(a.screen, modeX, 0, modeStr, a.styles.ModeBox, w)
	}

	// ----------------------------------------------------
	// Line 1: Cell contents / Input buffer / Slash Menu Bar
	// ----------------------------------------------------
	if a.mode == "MENU" && len(a.menuStack) > 0 {
		topState := a.menuStack[0]
		short := useShortMenuBar(a.rootMenu.Children, w)
		curX := 0
		for idx, topChild := range a.rootMenu.Children {
			lbl := topMenuLabel(topChild, short)
			lw := runewidth.StringWidth(lbl)
			gap := 0
			if curX > 0 {
				gap = 1
			}
			if curX+gap+lw >= w {
				// Extremely narrow terminals: draw only what fits.
				break
			}
			curX += gap
			if idx == topState.selectedIdx {
				drawTextFast(a.screen, curX, 1, lbl, a.styles.MenuSel, w)
			} else {
				drawTextFast(a.screen, curX, 1, lbl, a.styles.MenuText, w)
			}
			curX += lw
		}
	} else if a.mode == "PROMPT" {
		promptFull := fmt.Sprintf("%s%s", a.promptText, string(a.inputBuffer))
		drawTextFast(a.screen, 0, 1, promptFull, a.styles.Header, w)
	} else if a.mode == "INPUT" || a.mode == "EDIT" || a.mode == "POINT" {
		drawTextFast(a.screen, 0, 1, string(a.inputBuffer), a.styles.Default, w)
	} else {
		cellContent := ""
		if currCell != nil {
			cellContent = currCell.RawInput
		}
		drawTextFast(a.screen, 0, 1, cellContent, a.styles.Default, w)
	}

	// ----------------------------------------------------
	// Line 2: Column Headers
	// ----------------------------------------------------
	rowHeaderW := a.getRowHeaderW(h)
	for x := 0; x < rowHeaderW; x++ {
		a.screen.SetContent(x, 2, ' ', nil, a.styles.Header)
	}

	a.visibleCols = a.visibleCols[:0]
	curX := rowHeaderW
	frozenC := a.sheet.FrozenCols()

	// 1. Frozen columns
	for fc := 0; fc < frozenC && curX < w; fc++ {
		cw := a.sheet.GetColWidth(fc)
		if curX+cw > w {
			cw = w - curX
		}
		colLet := getColLetter(fc)
		cellText := cell.AlignAndPad(colLet, cw, cell.AlignCenter)
		drawTextFast(a.screen, curX, 2, cellText, a.styles.Header, w)
		a.visibleCols = append(a.visibleCols, colLayout{colIdx: fc, x: curX, width: cw})
		curX += cw
	}

	// 2. Scrolled columns
	cIdx := a.leftCol
	if cIdx < frozenC {
		cIdx = frozenC
	}
	for curX < w && cIdx < a.sheet.MaxCols() {
		cw := a.sheet.GetColWidth(cIdx)
		if curX+cw > w {
			cw = w - curX
		}
		colLet := getColLetter(cIdx)
		cellText := cell.AlignAndPad(colLet, cw, cell.AlignCenter)
		drawTextFast(a.screen, curX, 2, cellText, a.styles.Header, w)
		a.visibleCols = append(a.visibleCols, colLayout{colIdx: cIdx, x: curX, width: cw})
		curX += cw
		cIdx++
	}
	for x := curX; x < w; x++ {
		a.screen.SetContent(x, 2, ' ', nil, a.styles.Header)
	}

	// ----------------------------------------------------
	// Lines 3 to gridEndY: Grid Cells & Row Headers
	// ----------------------------------------------------
	wb := a.currentWorkbook()
	hasMultipleSheets := len(wb.Sheets) > 1

	gridStartY := 3
	numGridRows := a.getNumGridRows(h)
	frozenR := a.displayFrozenRows(numGridRows)

	for rOff := 0; rOff < numGridRows; rOff++ {
		gridY := gridStartY + rOff
		var rowIdx int
		if rOff < frozenR {
			rowIdx = rOff
		} else {
			rowIdx = a.topRow + (rOff - frozenR)
		}

		drawRowNumFast(a.screen, gridY, rowIdx, rowHeaderW, a.styles.Header)

		renderedCols := a.getRenderedRowCols(rowIdx)

		curGridX := rowHeaderW
		for idx, cl := range a.visibleCols {
			c := a.sheet.GetCell(cl.colIdx, rowIdx)
			rendered := renderedCols[idx]

			isCursor := (cl.colIdx == a.cursorCol && rowIdx == a.cursorRow)
			isSelected := a.selectedRange != nil && a.selectedRange.Contains(cl.colIdx, rowIdx)

			style := a.styles.Default
			if c != nil && (c.Type == cell.TypeNumber || (c.Type == cell.TypeFormula && c.Value != nil)) {
				if _, ok := c.Value.(float64); ok {
					style = a.styles.GridNumber
				} else if _, ok := c.Value.(int); ok {
					style = a.styles.GridNumber
				}
			}

			if isCursor {
				style = a.styles.CellCursor
			} else if isSelected {
				style = a.styles.RangeSel
			}

			drawTextFast(a.screen, cl.x, gridY, rendered, style, w)
			curGridX = cl.x + cl.width
		}
		for x := curGridX; x < w; x++ {
			a.screen.SetContent(x, gridY, ' ', nil, a.styles.Default)
		}
	}

	// ----------------------------------------------------
	// Line H-2 (if multiple sheets): Sheet Tab Bar with Scrolling Window
	// ----------------------------------------------------
	if hasMultipleSheets {
		tabY := h - 2
		for x := 0; x < w; x++ {
			a.screen.SetContent(x, tabY, ' ', nil, a.styles.Default)
		}
		curTabX := 1
		prefix := fmt.Sprintf("Sheets (%d/%d) [Ctrl+PgUp/PgDn, Ctrl+T]: ", wb.ActiveSheetIndex+1, len(wb.Sheets))
		curTabX = drawTextFast(a.screen, curTabX, tabY, prefix, a.styles.Header, w)

		availW := w - curTabX - 6
		activeIdx := wb.ActiveSheetIndex
		if activeIdx < 0 || activeIdx >= len(wb.Sheets) {
			activeIdx = 0
		}

		// Calculate sliding window [startIdx, endIdx] to keep active tab centered and visible
		startIdx := activeIdx
		endIdx := activeIdx
		activeLabelW := runewidth.StringWidth(fmt.Sprintf(" %s ", wb.Sheets[activeIdx].Name()))
		usedW := activeLabelW

		for {
			expanded := false
			if startIdx > 0 {
				leftW := runewidth.StringWidth(fmt.Sprintf(" %s ", wb.Sheets[startIdx-1].Name())) + 1
				if usedW+leftW+4 <= availW {
					startIdx--
					usedW += leftW
					expanded = true
				}
			}
			if endIdx < len(wb.Sheets)-1 {
				rightW := runewidth.StringWidth(fmt.Sprintf(" %s ", wb.Sheets[endIdx+1].Name())) + 1
				if usedW+rightW+4 <= availW {
					endIdx++
					usedW += rightW
					expanded = true
				}
			}
			if !expanded {
				break
			}
		}

		if startIdx > 0 {
			curTabX = drawTextFast(a.screen, curTabX, tabY, "◄ ", a.styles.TabArrow, w)
		}

		a.sheetTabBounds = a.sheetTabBounds[:0]
		for idx := startIdx; idx <= endIdx; idx++ {
			sh := wb.Sheets[idx]
			tabLabel := fmt.Sprintf(" %s ", sh.Name())
			startX := curTabX
			if idx == wb.ActiveSheetIndex {
				curTabX = drawTextFast(a.screen, curTabX, tabY, tabLabel, a.styles.ActiveTab, w)
			} else {
				curTabX = drawTextFast(a.screen, curTabX, tabY, tabLabel, a.styles.InactiveTab, w)
			}
			a.sheetTabBounds = append(a.sheetTabBounds, tabBound{sheetIdx: idx, startX: startX, endX: curTabX - 1})
			curTabX = drawTextFast(a.screen, curTabX, tabY, " ", a.styles.Default, w)
		}

		if endIdx < len(wb.Sheets)-1 {
			drawTextFast(a.screen, curTabX, tabY, "►", a.styles.TabArrow, w)
		}
	}

	// ----------------------------------------------------
	// Line H-1: Status Bar (Filename / Sheet / Time / CALC / Guides)
	// ----------------------------------------------------
	statusY := h - 1

	// Left: Filename, Active Sheet, & Date/Time
	displayName := filepath.Base(a.filename)
	fileInfo := displayName
	if hasMultipleSheets {
		fileInfo = fmt.Sprintf("%s [%s] (%d/%d)", displayName, a.sheet.Name(), wb.ActiveSheetIndex+1, len(wb.Sheets))
	}
	curStatX := drawTextFast(a.screen, 1, statusY, fileInfo, a.styles.Status, w)
	curStatX = drawTextFast(a.screen, curStatX, statusY, "  ", a.styles.Status, w)
	curStatX = drawTextFast(a.screen, curStatX, statusY, a.getFormattedTime(), a.styles.Status, w)

	// Right: CALC indicator and F-key guides
	rightStatus := statRightNormal
	if a.sheet.NeedsRecalc() {
		rightStatus = statRightCalc
	}
	rightW := len(rightStatus)
	rightX := w - rightW - 1

	for x := curStatX; x < rightX; x++ {
		a.screen.SetContent(x, statusY, ' ', nil, a.styles.Status)
	}

	// Center: Message
	if a.statusMessage != "" {
		msgW := runewidth.StringWidth(a.statusMessage)
		msgX := (w - msgW) / 2
		if msgX > curStatX+1 && msgX+msgW < rightX {
			drawTextFast(a.screen, msgX, statusY, a.statusMessage, a.styles.Error, w)
		}
	}

	if rightX > 0 {
		drawTextFast(a.screen, rightX, statusY, rightStatus, a.styles.ModeBox, w)
		for x := rightX + rightW; x < w; x++ {
			a.screen.SetContent(x, statusY, ' ', nil, a.styles.Status)
		}
	}

	// Draw Dropdown Submenu Box if in MENU mode
	if a.mode == "MENU" && len(a.menuStack) > 1 {
		a.drawDropdownSubmenu(w, h)
	}

	// Draw Command Palette Modal on top if active
	if a.palette.IsActive() {
		a.palette.Draw(a.screen, a.styles)
	}

	// Draw File Picker Modal on top if active
	if a.filePicker.IsActive() {
		a.filePicker.Draw(a.screen, a.styles)
	}

	// Draw Confirm Modal on top if active
	if a.confirmAction != ConfirmNone {
		a.drawConfirmModal(w, h)
	}

	// Hardware Cursor Placement
	if a.confirmAction != ConfirmNone || a.filePicker.IsActive() || a.palette.IsActive() {
		a.screen.HideCursor()
	} else if a.mode == "INPUT" || a.mode == "EDIT" {
		curPos := a.inputCursorPos
		runesBefore := a.inputBuffer[:curPos]
		cx := runewidth.StringWidth(string(runesBefore))
		if cx < w {
			a.screen.ShowCursor(cx, 1)
		}
	} else if a.mode == "PROMPT" {
		promptLen := runewidth.StringWidth(a.promptText)
		runesBefore := a.inputBuffer[:a.inputCursorPos]
		cx := promptLen + runewidth.StringWidth(string(runesBefore))
		if cx < w {
			a.screen.ShowCursor(cx, 1)
		}
	} else {
		a.screen.HideCursor()
	}
}

func (a *App) drawConfirmModal(w, h int) {
	modalW := 64
	modalH := 8
	if modalW > w-4 {
		modalW = w - 4
	}
	if modalH > h-2 {
		modalH = h - 2
	}
	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2

	boxStyle := a.styles.Header
	bgStyle := a.styles.Default
	hotKeyStyle := a.styles.ModeBox
	itemStyle := a.styles.Default

	// Clear background
	for y := modalY; y < modalY+modalH; y++ {
		for x := modalX; x < modalX+modalW; x++ {
			a.screen.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Double Border: ╔ ╗ ╚ ╝ ║ ═
	for x := modalX; x < modalX+modalW; x++ {
		a.screen.SetContent(x, modalY, '═', nil, boxStyle)
		a.screen.SetContent(x, modalY+modalH-1, '═', nil, boxStyle)
	}
	for y := modalY; y < modalY+modalH; y++ {
		a.screen.SetContent(modalX, y, '║', nil, boxStyle)
		a.screen.SetContent(modalX+modalW-1, y, '║', nil, boxStyle)
	}
	a.screen.SetContent(modalX, modalY, '╔', nil, boxStyle)
	a.screen.SetContent(modalX+modalW-1, modalY, '╗', nil, boxStyle)
	a.screen.SetContent(modalX, modalY+modalH-1, '╚', nil, boxStyle)
	a.screen.SetContent(modalX+modalW-1, modalY+modalH-1, '╝', nil, boxStyle)

	title := " QUIT HASUCALC "
	msg1 := "Worksheet has unsaved changes."
	msg2 := "Do you want to save changes before closing?"
	saveBtnLabel := "] Save & Exit    "

	if a.confirmAction == ConfirmNew {
		title = " NEW WORKSHEET "
		msg2 = "Do you want to save changes before creating new sheet?"
		saveBtnLabel = "] Save & New     "
	}

	titleX := modalX + (modalW-len(title))/2
	drawTextFast(a.screen, titleX, modalY, title, boxStyle, w)

	// Message lines
	drawTextFast(a.screen, modalX+4, modalY+2, msg1, itemStyle, w)
	drawTextFast(a.screen, modalX+4, modalY+3, msg2, itemStyle, w)

	// Buttons
	btnY := modalY + 5
	bX := modalX + 4
	bX = drawTextFast(a.screen, bX, btnY, "[", itemStyle, w)
	bX = drawTextFast(a.screen, bX, btnY, "Y", hotKeyStyle, w)
	bX = drawTextFast(a.screen, bX, btnY, saveBtnLabel, itemStyle, w)

	bX = drawTextFast(a.screen, bX, btnY, "[", itemStyle, w)
	bX = drawTextFast(a.screen, bX, btnY, "N", hotKeyStyle, w)
	bX = drawTextFast(a.screen, bX, btnY, "] Don't Save    ", itemStyle, w)

	bX = drawTextFast(a.screen, bX, btnY, "[", itemStyle, w)
	bX = drawTextFast(a.screen, bX, btnY, "Esc", hotKeyStyle, w)
	drawTextFast(a.screen, bX, btnY, "] Cancel", itemStyle, w)
}

func isEmptyCell(c *cell.Cell) bool {
	if c == nil || c.Type == cell.TypeEmpty || c.Value == nil {
		return true
	}
	if str, ok := c.Value.(string); ok && str == "" {
		return true
	}
	return false
}

func isStringCell(c *cell.Cell) bool {
	if c == nil || c.Value == nil {
		return false
	}
	if c.Type == cell.TypeLabel {
		return true
	}
	if _, ok := c.Value.(string); ok {
		return true
	}
	return false
}

func (a *App) getRenderedRowCols(rowIdx int) []string {
	numCols := len(a.visibleCols)
	res := make([]string, numCols)
	if numCols == 0 {
		return res
	}

	startScanCol := 0
	if a.sheet.FrozenCols() == 0 && len(a.visibleCols) > 0 {
		startScanCol = a.visibleCols[0].colIdx - 10
		if startScanCol < 0 {
			startScanCol = 0
		}
	}
	endScanCol := a.visibleCols[numCols-1].colIdx

	var spillRunes []rune

	visColMap := make(map[int]int, numCols)
	for idx, cl := range a.visibleCols {
		visColMap[cl.colIdx] = idx
	}

	for cIdx := startScanCol; cIdx <= endScanCol; cIdx++ {
		colW := a.sheet.GetColWidth(cIdx)
		visIdx, isVis := visColMap[cIdx]
		if isVis {
			colW = a.visibleCols[visIdx].width
		}

		c := a.sheet.GetCell(cIdx, rowIdx)
		if len(spillRunes) > 0 {
			if isEmptyCell(c) {
				curW := 0
				splitIdx := 0
				for i, r := range spillRunes {
					rw := runewidth.RuneWidth(r)
					if curW+rw > colW {
						break
					}
					curW += rw
					splitIdx = i + 1
				}
				part := string(spillRunes[:splitIdx])
				if curW < colW {
					part += getSpaces(colW - curW)
				}
				if isVis {
					res[visIdx] = part
				}
				spillRunes = spillRunes[splitIdx:]
				continue
			} else {
				spillRunes = nil
			}
		}

		if !isEmptyCell(c) {
			if isStringCell(c) && c.Alignment != cell.AlignRepeat && (c.Alignment == cell.AlignLeft || c.Alignment == cell.AlignDefault) {
				strVal := fmt.Sprintf("%v", c.Value)
				valW := runewidth.StringWidth(strVal)
				if valW > colW {
					totalAvail := colW
					for nextCol := cIdx + 1; nextCol < a.sheet.MaxCols(); nextCol++ {
						nextCell := a.sheet.GetCell(nextCol, rowIdx)
						if !isEmptyCell(nextCell) {
							break
						}
						totalAvail += a.sheet.GetColWidth(nextCol)
						if totalAvail >= valW {
							break
						}
					}
					clipped := runewidth.Truncate(strVal, totalAvail, "")
					allRunes := []rune(clipped)
					curW := 0
					splitIdx := 0
					for i, r := range allRunes {
						rw := runewidth.RuneWidth(r)
						if curW+rw > colW {
							break
						}
						curW += rw
						splitIdx = i + 1
					}
					firstPart := string(allRunes[:splitIdx])
					if curW < colW {
						firstPart += getSpaces(colW - curW)
					}
					if isVis {
						res[visIdx] = firstPart
					}
					spillRunes = allRunes[splitIdx:]
				} else {
					if isVis {
						res[visIdx] = c.Render(colW, a.sheet.GlobalFormat())
					}
				}
			} else {
				if isVis {
					res[visIdx] = c.Render(colW, a.sheet.GlobalFormat())
				}
			}
		} else {
			if isVis {
				res[visIdx] = getSpaces(colW)
			}
		}
	}

	return res
}

func (a *App) drawDropdownSubmenu(w, h int) {
	curState := a.menuStack[len(a.menuStack)-1]
	items := curState.menu.Children
	if len(items) == 0 {
		return
	}

	topState := a.menuStack[0]
	// Calculate X position of top category (must match menu bar layout in draw)
	short := useShortMenuBar(a.rootMenu.Children, w)
	popX := 0
	for idx, topChild := range a.rootMenu.Children {
		if idx == topState.selectedIdx {
			break
		}
		if idx > 0 {
			popX++
		}
		popX += runewidth.StringWidth(topMenuLabel(topChild, short))
	}

	popW := 30
	for _, it := range items {
		lw := runewidth.StringWidth(formatHotKeyLabel(it.Name, it.Key)) + len(it.Shortcut) + 6
		if lw > popW {
			popW = lw
		}
	}
	if popX+popW >= w {
		popX = w - popW - 1
	}
	if popX < 1 {
		popX = 1
	}

	popY := 2
	popH := len(items) + 2

	boxStyle := a.styles.Header
	itemStyle := a.styles.Default
	selStyle := a.styles.MenuSel

	// Frame
	for y := popY; y < popY+popH; y++ {
		for x := popX; x < popX+popW; x++ {
			a.screen.SetContent(x, y, ' ', nil, itemStyle)
		}
	}
	for x := popX; x < popX+popW; x++ {
		a.screen.SetContent(x, popY, '═', nil, boxStyle)
		a.screen.SetContent(x, popY+popH-1, '═', nil, boxStyle)
	}
	for y := popY; y < popY+popH; y++ {
		a.screen.SetContent(popX, y, '║', nil, boxStyle)
		a.screen.SetContent(popX+popW-1, y, '║', nil, boxStyle)
	}
	a.screen.SetContent(popX, popY, '╔', nil, boxStyle)
	a.screen.SetContent(popX+popW-1, popY, '╗', nil, boxStyle)
	a.screen.SetContent(popX, popY+popH-1, '╚', nil, boxStyle)
	a.screen.SetContent(popX+popW-1, popY+popH-1, '╝', nil, boxStyle)

	// Items
	for idx, it := range items {
		iy := popY + 1 + idx
		isSelected := (idx == curState.selectedIdx)
		st := itemStyle
		if isSelected {
			st = selStyle
			for x := popX + 1; x < popX+popW-1; x++ {
				a.screen.SetContent(x, iy, ' ', nil, st)
			}
		}

		itemLbl := formatHotKeyLabel(it.Name, it.Key)
		drawTextFast(a.screen, popX+2, iy, itemLbl, st, popX+popW-2)
		if it.Shortcut != "" {
			scX := popX + popW - 2 - len(it.Shortcut)
			drawTextFast(a.screen, scX, iy, it.Shortcut, st, popX+popW-2)
		}
	}
}

func drawTextFast(s tcell.Screen, x, y int, text string, style tcell.Style, maxW int) int {
	curX := x
	var lastMain rune
	var lastComb []rune
	lastX := -1

	for _, r := range text {
		rw := 1
		if r >= 0x80 {
			rw = runewidth.RuneWidth(r)
		}
		if rw == 0 {
			if lastX >= 0 {
				lastComb = append(lastComb, r)
				s.SetContent(lastX, y, lastMain, lastComb, style)
			}
			continue
		}
		if curX+rw > maxW {
			break
		}
		lastMain = r
		lastComb = nil
		lastX = curX
		s.SetContent(curX, y, r, nil, style)
		curX += rw
	}
	return curX
}

func (a *App) handleKeyEvent(ev *tcell.EventKey) {
	// Confirmation dialog takes priority if active
	if a.confirmAction != ConfirmNone {
		switch ev.Key() {
		case tcell.KeyEscape:
			a.confirmAction = ConfirmNone
		case tcell.KeyRune:
			switch ev.Rune() {
			case 'y', 'Y':
				act := a.confirmAction
				a.confirmAction = ConfirmNone
				a.filePicker.Open(FilePickerModeSave, a.filename)
				if act == ConfirmQuit {
					a.filePicker.SetQuitOnSave(true)
				} else if act == ConfirmNew {
					a.filePicker.SetNewOnSave(true)
				}
			case 'n', 'N':
				act := a.confirmAction
				a.confirmAction = ConfirmNone
				if act == ConfirmQuit {
					a.running = false
				} else if act == ConfirmNew {
					a.createNewWorksheet()
				}
			case 'c', 'C':
				a.confirmAction = ConfirmNone
			}
		case tcell.KeyEnter:
			act := a.confirmAction
			a.confirmAction = ConfirmNone
			a.filePicker.Open(FilePickerModeSave, a.filename)
			if act == ConfirmQuit {
				a.filePicker.SetQuitOnSave(true)
			} else if act == ConfirmNew {
				a.filePicker.SetNewOnSave(true)
			}
		}
		return
	}

	// File Picker takes top priority if active
	if a.filePicker.IsActive() {
		done, selectedPath, canceled := a.filePicker.HandleKey(ev)
		if done {
			if canceled || selectedPath == "" {
				a.pendingExportRange = nil
				return
			}
			switch a.filePicker.mode {
			case FilePickerModeOpen:
				a.loadFile(selectedPath)
			case FilePickerModeSave:
				a.saveFile(selectedPath)
				if a.filePicker.QuitOnSave() {
					a.running = false
				} else if a.filePicker.NewOnSave() {
					a.createNewWorksheet()
				}
			case FilePickerModeImportCSV:
				a.importCSVFile(selectedPath)
			case FilePickerModeExportCSV:
				a.exportCSVFile(selectedPath)
			case FilePickerModeExportXLSX:
				a.exportXLSXFile(selectedPath)
			case FilePickerModeExportODS:
				a.exportODSFile(selectedPath)
			case FilePickerModeExportMarkdown:
				a.exportMarkdownFile(selectedPath)
			}
		}
		return
	}

	// Command Palette takes priority if active
	if a.palette.IsActive() {
		handled, item := a.palette.HandleKey(ev)
		if handled && item != nil {
			a.dispatchPaletteAction(item.Action)
		}
		return
	}

	// Global Shortcuts
	switch ev.Key() {
	case tcell.KeyCtrlQ:
		a.tryQuitApp()
		return
	case tcell.KeyCtrlK:
		a.palette.Open()
		return
	case tcell.KeyCtrlZ:
		a.doUndo()
		return
	case tcell.KeyCtrlY:
		a.doRedo()
		return
	case tcell.KeyCtrlL:
		if a.mode == "READY" {
			a.doPasteLink()
			return
		}
	case tcell.KeyCtrlT:
		wb := a.currentWorkbook()
		if len(wb.Sheets) > 1 {
			selected := ShowSheetPickerModal(a.screen, a.filename, wb.SheetNames(), a.styles)
			if selected >= 0 {
				a.switchSheet(selected)
			}
		}
		return
	case tcell.KeyF1:
		RenderHelpScreen(a.screen, a.styles)
		return
	case tcell.KeyF9:
		a.currentWorkbook().RecalculateAll()
		a.statusMessage = "Recalculated all worksheets."
		return
	case tcell.KeyF10:
		RenderGraphScreen(a.screen, a.sheet, a.styles, a.filename)
		return
	}

	switch a.mode {
	case "READY":
		a.handleReadyKey(ev)
	case "INPUT":
		a.handleInputKey(ev)
	case "EDIT":
		a.handleBufferEditing(ev)
	case "MENU":
		a.handleMenuKey(ev)
	case "PROMPT":
		a.handlePromptKey(ev)
	case "POINT":
		a.handlePointKey(ev)
	}
}

func (a *App) handleReadyKey(ev *tcell.EventKey) {
	if ev.Key() == tcell.KeyRune {
		if ev.Rune() == '/' {
			a.mode = "MENU"
			a.menuStack = []*menuState{
				{menu: a.rootMenu, selectedIdx: 0},
				{menu: a.rootMenu.Children[0], selectedIdx: 0},
			}
			return
		}
		if ev.Rune() == ':' {
			a.palette.Open()
			return
		}
	}

	endMode := a.endMode
	a.endMode = false

	isShift := (ev.Modifiers() & tcell.ModShift) != 0

	// Handle Shift+Arrow selection vs normal movement
	if isShift {
		switch ev.Key() {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyLeft, tcell.KeyRight:
			if a.selectionAnchor == nil {
				a.selectionAnchor = &[2]int{a.cursorCol, a.cursorRow}
			}
			switch ev.Key() {
			case tcell.KeyUp:
				if a.cursorRow > 0 {
					a.cursorRow--
				}
			case tcell.KeyDown:
				if a.cursorRow < a.sheet.MaxRows()-1 {
					a.cursorRow++
				}
			case tcell.KeyLeft:
				if a.cursorCol > 0 {
					a.cursorCol--
				}
			case tcell.KeyRight:
				if a.cursorCol < a.sheet.MaxCols()-1 {
					a.cursorCol++
				}
			}
			a.selectedRange = &coord.RangeRef{
				Start: coord.CellRef{Col: a.selectionAnchor[0], Row: a.selectionAnchor[1]},
				End:   coord.CellRef{Col: a.cursorCol, Row: a.cursorRow},
			}
			return
		}
	}

	switch ev.Key() {
	case tcell.KeyEnd:
		a.endMode = true
		return

	case tcell.KeyUp, tcell.KeyCtrlP:
		a.clearSelection()
		if endMode {
			a.jumpBoundary(0, -1)
		} else if a.cursorRow > 0 {
			a.cursorRow--
		}
	case tcell.KeyDown, tcell.KeyCtrlN, tcell.KeyEnter:
		a.clearSelection()
		if endMode {
			a.jumpBoundary(0, 1)
		} else if a.cursorRow < a.sheet.MaxRows()-1 {
			a.cursorRow++
		}
	case tcell.KeyLeft, tcell.KeyCtrlB, tcell.KeyBacktab:
		a.clearSelection()
		if endMode {
			a.jumpBoundary(-1, 0)
		} else if a.cursorCol > 0 {
			a.cursorCol--
		}
	case tcell.KeyRight:
		a.clearSelection()
		if endMode {
			a.jumpBoundary(1, 0)
		} else if a.cursorCol < a.sheet.MaxCols()-1 {
			a.cursorCol++
		}
	case tcell.KeyTab:
		a.clearSelection()
		if ev.Modifiers()&tcell.ModShift != 0 {
			if endMode {
				a.jumpBoundary(-1, 0)
			} else if a.cursorCol > 0 {
				a.cursorCol--
			}
		} else {
			if endMode {
				a.jumpBoundary(1, 0)
			} else if a.cursorCol < a.sheet.MaxCols()-1 {
				a.cursorCol++
			}
		}
	case tcell.KeyPgUp:
		if (ev.Modifiers() & tcell.ModCtrl) != 0 {
			wb := a.currentWorkbook()
			if len(wb.Sheets) > 1 && wb.ActiveSheetIndex > 0 {
				a.switchSheet(wb.ActiveSheetIndex - 1)
			}
			return
		}
		a.clearSelection()
		a.cursorRow -= 20
		if a.cursorRow < 0 {
			a.cursorRow = 0
		}
	case tcell.KeyPgDn:
		if (ev.Modifiers() & tcell.ModCtrl) != 0 {
			wb := a.currentWorkbook()
			if len(wb.Sheets) > 1 && wb.ActiveSheetIndex < len(wb.Sheets)-1 {
				a.switchSheet(wb.ActiveSheetIndex + 1)
			}
			return
		}
		a.clearSelection()
		a.cursorRow += 20
		if a.cursorRow >= a.sheet.MaxRows() {
			a.cursorRow = a.sheet.MaxRows() - 1
		}
	case tcell.KeyHome:
		a.clearSelection()
		a.cursorCol = 0
		a.cursorRow = 0

	// Modern Shortcuts: Ctrl+Z, Ctrl+Y, Ctrl+C, Ctrl+V, Ctrl+X, Ctrl+S, Ctrl+O, Ctrl+A, Ctrl+G, Ctrl+D, Ctrl+R, Ctrl+H
	case tcell.KeyCtrlZ:
		a.doUndo()
	case tcell.KeyCtrlY:
		a.doRedo()
	case tcell.KeyCtrlC:
		a.doCopySelection()
	case tcell.KeyCtrlV:
		a.doPasteClipboard()
	case tcell.KeyCtrlL:
		a.doPasteLink()
	case tcell.KeyCtrlX:
		a.doCutSelection()
	case tcell.KeyCtrlA:
		a.doSelectAll()
	case tcell.KeyCtrlD:
		a.doFillDown()
	case tcell.KeyCtrlR:
		a.doFillRight()
	case tcell.KeyCtrlH:
		a.startPromptChain([]PromptItem{
			{Key: "find", Prompt: "Find text/number/formula: ", Default: a.lastSearchQuery},
			{Key: "replace", Prompt: "Replace with: ", Default: a.lastReplaceText},
			{Key: "scope", Prompt: "Scope [C: Current Sheet / A: All Sheets]: ", Default: "C"},
		}, "doReplacePrompt")
	case tcell.KeyCtrlS:
		a.filePicker.Open(FilePickerModeSave, a.filename)
	case tcell.KeyCtrlO:
		a.filePicker.Open(FilePickerModeOpen, a.filename)
	case tcell.KeyCtrlF:
		a.startPromptChain([]PromptItem{{Key: "query", Prompt: "Find text/number/formula: ", Default: a.lastSearchQuery}}, "doFindPrompt")
	case tcell.KeyF3:
		if ev.Modifiers()&tcell.ModShift != 0 {
			a.findPrev()
		} else {
			a.findNext()
		}
	case tcell.KeyCtrlG, tcell.KeyF5:
		a.startPromptChain([]PromptItem{{Key: "cell", Prompt: "Enter address or name to go to (e.g. B10, Sheet2!A1, Total): "}}, "doGotoCell")

	case tcell.KeyDelete, tcell.KeyBackspace, tcell.KeyBackspace2:
		a.pushUndo()
		if a.selectedRange != nil {
			a.sheet.ClearRange(*a.selectedRange)
			a.statusMessage = fmt.Sprintf("Cleared range %s.", a.selectedRange)
			a.clearSelection()
		} else {
			a.sheet.ClearCell(a.cursorCol, a.cursorRow)
		}

	case tcell.KeyEscape:
		a.clearSelection()

	case tcell.KeyF2, tcell.KeyCtrlE: // EDIT cell content on line 1 (F2 or Ctrl+E)
		c := a.sheet.GetCell(a.cursorCol, a.cursorRow)
		raw := ""
		if c != nil {
			raw = c.RawInput
		}
		a.inputBuffer = []rune(raw)
		a.inputCursorPos = len(a.inputBuffer)
		a.mode = "EDIT"

	case tcell.KeyRune:
		// Alt+= AutoSum (Excel-like)
		if ev.Modifiers()&tcell.ModAlt != 0 && (ev.Rune() == '=' || ev.Rune() == '+') {
			a.doAutoSum()
			return
		}
		a.clearSelection()
		a.inputBuffer = []rune{ev.Rune()}
		a.inputCursorPos = 1
		a.mode = "INPUT"
	}
}

func (a *App) clearSelection() {
	a.selectionAnchor = nil
	a.selectedRange = nil
}

type ClipboardData struct {
	Width     int
	Height    int
	Cells     map[sheet.CellCoord]*cell.Cell
	Range     coord.RangeRef
	SheetName string
	IsCut     bool // Excel-like move: paste without relative formula shift
}

func (a *App) doCopySelection() {
	srcRange := coord.RangeRef{
		Start: coord.CellRef{Col: a.cursorCol, Row: a.cursorRow},
		End:   coord.CellRef{Col: a.cursorCol, Row: a.cursorRow},
	}
	if a.selectedRange != nil {
		srcRange = *a.selectedRange
	}

	srcW := srcRange.MaxCol() - srcRange.MinCol() + 1
	srcH := srcRange.MaxRow() - srcRange.MinRow() + 1

	snapshot := make(map[sheet.CellCoord]*cell.Cell)
	for rOff := 0; rOff < srcH; rOff++ {
		for cOff := 0; cOff < srcW; cOff++ {
			sc := srcRange.MinCol() + cOff
			sr := srcRange.MinRow() + rOff
			c := a.sheet.GetCell(sc, sr)
			if c != nil {
				var fmtCopy *cell.CellFormat
				if c.FormatSpec != nil {
					f := *c.FormatSpec
					fmtCopy = &f
				}
				snapshot[sheet.CellCoord{Col: cOff, Row: rOff}] = &cell.Cell{
					RawInput:   c.RawInput,
					Type:       c.Type,
					Alignment:  c.Alignment,
					FormatSpec: fmtCopy,
					Value:      c.Value,
				}
			}
		}
	}

	copyRef := srcRange
	a.clipboardRange = &copyRef
	a.clipboard = &ClipboardData{
		Width:     srcW,
		Height:    srcH,
		Cells:     snapshot,
		Range:     srcRange,
		SheetName: a.sheet.Name(),
		IsCut:     false,
	}
	a.hasClipboard = true
	a.statusMessage = fmt.Sprintf("Copied %s to clipboard.", srcRange)
}

func (a *App) doCutSelection() {
	a.pushUndoWorkbook()
	a.doCopySelection()
	if a.clipboard != nil {
		a.clipboard.IsCut = true
		a.cutUndoID = a.undoManager.PeekUndoID()
		a.cutRedoPending = false
		a.savedCutClipboard = nil
	}
	if a.selectedRange != nil {
		a.sheet.ClearRange(*a.selectedRange)
		a.clearSelection()
	} else {
		a.sheet.ClearCell(a.cursorCol, a.cursorRow)
	}
	a.statusMessage = "Cut selection to clipboard."
}

func (a *App) doPasteClipboard() {
	if !a.hasClipboard || a.clipboard == nil {
		a.statusMessage = "Clipboard is empty!"
		return
	}
	clip := a.clipboard
	if !clip.IsCut {
		a.pushUndoWorkbook()
	}
	srcW := clip.Width
	srcH := clip.Height
	if srcW <= 0 || srcH <= 0 {
		a.statusMessage = "Clipboard is empty!"
		return
	}
	toR := coord.RangeRef{
		Start: coord.CellRef{Col: a.cursorCol, Row: a.cursorRow},
		End:   coord.CellRef{Col: a.cursorCol + srcW - 1, Row: a.cursorRow + srcH - 1},
	}
	// Copy tiles into a larger selection; cut pastes once (Excel-like move).
	if a.selectedRange != nil && !clip.IsCut {
		toR = *a.selectedRange
	}
	toR = clipRangeToSheetBounds(toR, a.sheet.MaxCols(), a.sheet.MaxRows())
	a.clearSelection()

	a.sheet.SuspendRecalc()
	for destR := toR.MinRow(); destR <= toR.MaxRow(); destR++ {
		for destC := toR.MinCol(); destC <= toR.MaxCol(); destC++ {
			cOff := (destC - toR.MinCol()) % srcW
			rOff := (destR - toR.MinRow()) % srcH
			srcCell, ok := clip.Cells[sheet.CellCoord{Col: cOff, Row: rOff}]
			if !ok || srcCell == nil {
				a.sheet.ClearCell(destC, destR)
				continue
			}

			raw := srcCell.RawInput
			if srcCell.Type == cell.TypeFormula && !clip.IsCut {
				origC := clip.Range.MinCol() + cOff
				origR := clip.Range.MinRow() + rOff
				dCol := destC - origC
				dRow := destR - origR
				raw = formula.AdjustFormulaReferences(srcCell.RawInput, dCol, dRow)
			} else if srcCell.Type == cell.TypeFormula && clip.IsCut && !strings.EqualFold(clip.SheetName, a.sheet.Name()) {
				raw = formula.QualifyFormulaReferences(srcCell.RawInput, clip.SheetName)
			}
			var fmtCopy *cell.CellFormat
			if srcCell.FormatSpec != nil {
				f := *srcCell.FormatSpec
				fmtCopy = &f
			}
			a.sheet.SetCellInput(destC, destR, raw, fmtCopy)
		}
	}
	a.sheet.EndSuspendRecalc()

	a.finishCutPaste(toR)

	a.sheet.SetModified(true)
	a.statusMessage = fmt.Sprintf("Pasted to %s.", toR)
}

func (a *App) finishCutPaste(toR coord.RangeRef) {
	clip := a.clipboard
	if clip == nil || !clip.IsCut {
		return
	}
	dCol := toR.MinCol() - clip.Range.MinCol()
	dRow := toR.MinRow() - clip.Range.MinRow()
	srcSh := a.sheet
	if wb := a.currentWorkbook(); wb != nil {
		if sh := wb.GetSheet(clip.SheetName); sh != nil {
			srcSh = sh
		} else {
			a.hasClipboard = false
			a.clipboard = nil
			a.clipboardRange = nil
			return
		}
	}
	srcSh.RetargetFormulasAfterMoveTo(clip.Range, dCol, dRow, a.sheet.Name())
	if wb := a.currentWorkbook(); wb != nil {
		wb.RecalculateAll()
	} else {
		srcSh.TriggerAutoRecalc()
	}
	a.hasClipboard = false
	a.clipboard = nil
	a.clipboardRange = nil
}

func (a *App) retargetClipboardSheet(oldName, newName string) {
	if a.clipboard == nil || !strings.EqualFold(a.clipboard.SheetName, oldName) {
		return
	}
	a.clipboard.SheetName = newName
	a.clipboard.Range.Sheet = newName
	if a.clipboardRange != nil {
		a.clipboardRange.Sheet = newName
	}
}

func (a *App) discardCutClipboardForSheet(deletedName string) {
	if a.clipboard == nil || !strings.EqualFold(a.clipboard.SheetName, deletedName) {
		return
	}
	if !a.clipboard.IsCut {
		return
	}
	a.discardCutClipboard()
}

func (a *App) discardCutClipboard() {
	a.hasClipboard = false
	a.clipboard = nil
	a.clipboardRange = nil
	a.savedCutClipboard = nil
	a.cutUndoID = 0
}

func (a *App) cutClipboardOnActiveSheet() bool {
	return a.clipboard != nil && a.clipboard.IsCut && strings.EqualFold(a.clipboard.SheetName, a.sheet.Name())
}

func (a *App) shiftCutClipboardRows(delta int) {
	a.clipboard.Range.Start.Row += delta
	a.clipboard.Range.End.Row += delta
	if a.clipboardRange != nil {
		a.clipboardRange.Start.Row += delta
		a.clipboardRange.End.Row += delta
	}
}

func (a *App) shiftCutClipboardCols(delta int) {
	a.clipboard.Range.Start.Col += delta
	a.clipboard.Range.End.Col += delta
	if a.clipboardRange != nil {
		a.clipboardRange.Start.Col += delta
		a.clipboardRange.End.Col += delta
	}
}

func (a *App) adjustCutClipboardForRowInsert(atRow, count int) {
	if !a.cutClipboardOnActiveSheet() {
		return
	}
	minR, maxR := a.clipboard.Range.MinRow(), a.clipboard.Range.MaxRow()
	if maxR < atRow {
		return
	}
	if minR >= atRow {
		a.shiftCutClipboardRows(count)
		return
	}
	a.discardCutClipboard()
}

func (a *App) adjustCutClipboardForRowDelete(atRow, count int) {
	if !a.cutClipboardOnActiveSheet() {
		return
	}
	minR, maxR := a.clipboard.Range.MinRow(), a.clipboard.Range.MaxRow()
	delHi := atRow + count - 1
	if maxR < atRow {
		return
	}
	if minR > delHi {
		a.shiftCutClipboardRows(-count)
		return
	}
	a.discardCutClipboard()
}

func (a *App) adjustCutClipboardForColInsert(atCol, count int) {
	if !a.cutClipboardOnActiveSheet() {
		return
	}
	minC, maxC := a.clipboard.Range.MinCol(), a.clipboard.Range.MaxCol()
	if maxC < atCol {
		return
	}
	if minC >= atCol {
		a.shiftCutClipboardCols(count)
		return
	}
	a.discardCutClipboard()
}

func (a *App) adjustCutClipboardForColDelete(atCol, count int) {
	if !a.cutClipboardOnActiveSheet() {
		return
	}
	minC, maxC := a.clipboard.Range.MinCol(), a.clipboard.Range.MaxCol()
	delHi := atCol + count - 1
	if maxC < atCol {
		return
	}
	if minC > delHi {
		a.shiftCutClipboardCols(-count)
		return
	}
	a.discardCutClipboard()
}

func (a *App) doPasteLink() {
	if !a.hasClipboard || a.clipboard == nil {
		a.statusMessage = "Clipboard is empty!"
		return
	}
	if !a.clipboard.IsCut {
		a.pushUndo()
	}
	clip := a.clipboard
	srcW := clip.Width
	srcH := clip.Height
	if srcW <= 0 || srcH <= 0 {
		a.statusMessage = "Clipboard is empty!"
		return
	}
	toR := coord.RangeRef{
		Start: coord.CellRef{Col: a.cursorCol, Row: a.cursorRow},
		End:   coord.CellRef{Col: a.cursorCol + srcW - 1, Row: a.cursorRow + srcH - 1},
	}
	if a.selectedRange != nil {
		toR = *a.selectedRange
	}
	toR = clipRangeToSheetBounds(toR, a.sheet.MaxCols(), a.sheet.MaxRows())

	sheetRef := clip.SheetName
	quotedSheet := strings.TrimSuffix(coord.QuoteSheetPrefix(sheetRef), "!")

	a.sheet.SuspendRecalc()
	linkFormulas := map[sheet.CellCoord]string{}
	for destR := toR.MinRow(); destR <= toR.MaxRow(); destR++ {
		for destC := toR.MinCol(); destC <= toR.MaxCol(); destC++ {
			cOff := (destC - toR.MinCol()) % srcW
			rOff := (destR - toR.MinRow()) % srcH

			srcC := clip.Range.MinCol() + cOff
			srcR := clip.Range.MinRow() + rOff

			srcCellRef := coord.CellRef{Col: srcC, Row: srcR}.String()
			var formulaStr string
			if a.sheet.Name() == clip.SheetName {
				formulaStr = "=" + srcCellRef
			} else {
				formulaStr = "=" + quotedSheet + "!" + srcCellRef
			}

			var fmtCopy *cell.CellFormat
			if srcCell, ok := clip.Cells[sheet.CellCoord{Col: cOff, Row: rOff}]; ok && srcCell != nil && srcCell.FormatSpec != nil {
				f := *srcCell.FormatSpec
				fmtCopy = &f
			}

			a.sheet.SetCellInput(destC, destR, formulaStr, fmtCopy)
			linkFormulas[sheet.CellCoord{Col: destC, Row: destR}] = formulaStr
		}
	}
	a.sheet.EndSuspendRecalc()

	wasCut := clip.IsCut
	a.finishCutPaste(toR)
	if wasCut {
		for pt, raw := range linkFormulas {
			a.sheet.SetCellInput(pt.Col, pt.Row, raw, nil)
		}
		if wb := a.currentWorkbook(); wb != nil {
			wb.RecalculateAll()
		} else {
			a.sheet.TriggerAutoRecalc()
		}
	}
	a.sheet.SetModified(true)
	a.statusMessage = fmt.Sprintf("Pasted links to %s.", toR)
}

func (a *App) doPasteValues() {
	if !a.hasClipboard || a.clipboard == nil {
		a.statusMessage = "Clipboard is empty!"
		return
	}
	if !a.clipboard.IsCut {
		a.pushUndo()
	}
	clip := a.clipboard
	srcW := clip.Width
	srcH := clip.Height
	if srcW <= 0 || srcH <= 0 {
		a.statusMessage = "Clipboard is empty!"
		return
	}
	toR := coord.RangeRef{
		Start: coord.CellRef{Col: a.cursorCol, Row: a.cursorRow},
		End:   coord.CellRef{Col: a.cursorCol + srcW - 1, Row: a.cursorRow + srcH - 1},
	}
	if a.selectedRange != nil {
		toR = *a.selectedRange
	}
	toR = clipRangeToSheetBounds(toR, a.sheet.MaxCols(), a.sheet.MaxRows())

	a.sheet.SuspendRecalc()
	for destR := toR.MinRow(); destR <= toR.MaxRow(); destR++ {
		for destC := toR.MinCol(); destC <= toR.MaxCol(); destC++ {
			cOff := (destC - toR.MinCol()) % srcW
			rOff := (destR - toR.MinRow()) % srcH
			srcCell, ok := clip.Cells[sheet.CellCoord{Col: cOff, Row: rOff}]
			if !ok || srcCell == nil || srcCell.Value == nil {
				a.sheet.ClearCell(destC, destR)
				continue
			}

			var valStr string
			switch v := srcCell.Value.(type) {
			case float64:
				valStr = fmt.Sprintf("%v", v)
			case int:
				valStr = fmt.Sprintf("%d", v)
			case string:
				valStr = "'" + v
			case bool:
				if v {
					valStr = "TRUE"
				} else {
					valStr = "FALSE"
				}
			default:
				valStr = fmt.Sprintf("'%v", v)
			}

			var fmtCopy *cell.CellFormat
			if srcCell.FormatSpec != nil {
				f := *srcCell.FormatSpec
				fmtCopy = &f
			}
			a.sheet.SetCellInput(destC, destR, valStr, fmtCopy)
		}
	}
	a.sheet.EndSuspendRecalc()

	a.finishCutPaste(toR)
	a.sheet.SetModified(true)
	a.statusMessage = fmt.Sprintf("Pasted values to %s.", toR)
}

func (a *App) doPasteTranspose() {
	if !a.hasClipboard || a.clipboard == nil {
		a.statusMessage = "Clipboard is empty!"
		return
	}
	if !a.clipboard.IsCut {
		a.pushUndo()
	}
	clip := a.clipboard
	srcW := clip.Width
	srcH := clip.Height
	if srcW <= 0 || srcH <= 0 {
		a.statusMessage = "Clipboard is empty!"
		return
	}

	destTopC := a.cursorCol
	destTopR := a.cursorRow

	a.sheet.SuspendRecalc()
	for rOff := 0; rOff < srcH; rOff++ {
		for cOff := 0; cOff < srcW; cOff++ {
			destC := destTopC + rOff
			destR := destTopR + cOff
			if destC >= a.sheet.MaxCols() || destR >= a.sheet.MaxRows() {
				continue
			}

			srcCell, ok := clip.Cells[sheet.CellCoord{Col: cOff, Row: rOff}]
			if !ok || srcCell == nil {
				a.sheet.ClearCell(destC, destR)
				continue
			}

			origC := clip.Range.MinCol() + cOff
			origR := clip.Range.MinRow() + rOff

			if srcCell.Type == cell.TypeFormula {
				adj := formula.TransposeFormulaReferences(srcCell.RawInput, origC, origR, destC, destR)
				a.sheet.SetCellInput(destC, destR, adj, srcCell.FormatSpec)
			} else {
				var fmtCopy *cell.CellFormat
				if srcCell.FormatSpec != nil {
					f := *srcCell.FormatSpec
					fmtCopy = &f
				}
				a.sheet.SetCellInput(destC, destR, srcCell.RawInput, fmtCopy)
			}
		}
	}
	a.sheet.EndSuspendRecalc()

	if clip.IsCut {
		srcSh := a.sheet
		if wb := a.currentWorkbook(); wb != nil {
			if sh := wb.GetSheet(clip.SheetName); sh != nil {
				srcSh = sh
			}
		}
		srcSh.RetargetAfterTransposeMove(clip.Range, destTopC, destTopR, a.sheet.Name())
		if wb := a.currentWorkbook(); wb != nil {
			wb.RecalculateAll()
		} else {
			srcSh.TriggerAutoRecalc()
		}
		a.hasClipboard = false
		a.clipboard = nil
		a.clipboardRange = nil
	}
	a.sheet.SetModified(true)
	a.statusMessage = fmt.Sprintf("Pasted transposed (%dx%d -> %dx%d) to %s%d.", srcW, srcH, srcH, srcW, getColLetter(destTopC), destTopR+1)
}

func (a *App) doAutoFill() {
	if a.selectedRange == nil {
		a.statusMessage = "Select a range to AutoFill (e.g. A1..A10)."
		return
	}
	rng := *a.selectedRange
	minC, maxC := rng.MinCol(), rng.MaxCol()
	minR, maxR := rng.MinRow(), rng.MaxRow()

	if maxC == minC && maxR == minR {
		a.statusMessage = "Select a range larger than one cell to AutoFill."
		return
	}

	a.pushUndo()

	seed := a.sheet.GetCell(minC, minR)
	if maxC > minC && maxR > minR && seed != nil && seed.Type == cell.TypeFormula {
		onlySeed := cellHasRaw(seed)
		if onlySeed {
			for r := minR; r <= maxR && onlySeed; r++ {
				for c := minC; c <= maxC; c++ {
					if c == minC && r == minR {
						continue
					}
					if cellHasRaw(a.sheet.GetCell(c, r)) {
						onlySeed = false
						break
					}
				}
			}
		}
		if onlySeed {
			for r := minR; r <= maxR; r++ {
				for c := minC; c <= maxC; c++ {
					if c == minC && r == minR {
						continue
					}
					adj := formula.AdjustFormulaReferences(seed.RawInput, c-minC, r-minR)
					a.sheet.SetCellInput(c, r, adj, seed.FormatSpec)
				}
			}
			a.recalculateAll()
			a.statusMessage = "AutoFill: filled formulas with adjusted references."
			return
		}
	}

	if maxR == minR {
		a.autoFillRow(minR, minC, maxC)
	} else {
		for c := minC; c <= maxC; c++ {
			a.autoFillColumn(c, minR, maxR)
		}
	}
	a.recalculateAll()
	a.statusMessage = fmt.Sprintf("AutoFilled series across %s.", rng)
}

func (a *App) doSelectAll() {
	// Find min/max populated cells
	minC, minR := 0, 0
	maxC, maxR := 0, 0
	first := true
	for pt := range a.sheet.GetPopulatedCoords() {
		if first {
			minC, minR = pt.Col, pt.Row
			maxC, maxR = pt.Col, pt.Row
			first = false
		} else {
			if pt.Col < minC {
				minC = pt.Col
			}
			if pt.Row < minR {
				minR = pt.Row
			}
			if pt.Col > maxC {
				maxC = pt.Col
			}
			if pt.Row > maxR {
				maxR = pt.Row
			}
		}
	}
	if !first {
		a.selectedRange = &coord.RangeRef{
			Start: coord.CellRef{Col: minC, Row: minR},
			End:   coord.CellRef{Col: maxC, Row: maxR},
		}
		a.statusMessage = fmt.Sprintf("Selected all: %s", a.selectedRange)
	}
}

func (a *App) jumpBoundary(dCol, dRow int) {
	isPop := func(col, row int) bool {
		c := a.sheet.GetCell(col, row)
		return c != nil && c.Type != cell.TypeEmpty && c.RawInput != ""
	}

	if dCol != 0 {
		maxC := a.sheet.MaxCols()
		if dCol > 0 {
			if a.cursorCol >= maxC-1 {
				return
			}
			curPop := isPop(a.cursorCol, a.cursorRow)
			nextPop := isPop(a.cursorCol+1, a.cursorRow)
			if curPop && nextPop {
				for c := a.cursorCol + 1; c < maxC; c++ {
					if !isPop(c, a.cursorRow) {
						a.cursorCol = c - 1
						return
					}
				}
				a.cursorCol = maxC - 1
			} else {
				popMaxC := a.sheet.MaxPopulatedCol()
				limit := popMaxC
				if a.cursorCol >= limit {
					limit = maxC - 1
				}
				for c := a.cursorCol + 1; c <= limit; c++ {
					if isPop(c, a.cursorRow) {
						a.cursorCol = c
						return
					}
				}
				if a.cursorCol < popMaxC {
					a.cursorCol = popMaxC
				} else {
					a.cursorCol = maxC - 1
				}
			}
		} else { // dCol < 0
			if a.cursorCol <= 0 {
				return
			}
			curPop := isPop(a.cursorCol, a.cursorRow)
			prevPop := isPop(a.cursorCol-1, a.cursorRow)
			if curPop && prevPop {
				for c := a.cursorCol - 1; c >= 0; c-- {
					if !isPop(c, a.cursorRow) {
						a.cursorCol = c + 1
						return
					}
				}
				a.cursorCol = 0
			} else {
				for c := a.cursorCol - 1; c >= 0; c-- {
					if isPop(c, a.cursorRow) {
						a.cursorCol = c
						return
					}
				}
				a.cursorCol = 0
			}
		}
	}

	if dRow != 0 {
		maxR := a.sheet.MaxRows()
		if dRow > 0 {
			if a.cursorRow >= maxR-1 {
				return
			}
			curPop := isPop(a.cursorCol, a.cursorRow)
			nextPop := isPop(a.cursorCol, a.cursorRow+1)
			if curPop && nextPop {
				for r := a.cursorRow + 1; r < maxR; r++ {
					if !isPop(a.cursorCol, r) {
						a.cursorRow = r - 1
						return
					}
				}
				a.cursorRow = maxR - 1
			} else {
				popMaxR := a.sheet.MaxPopulatedRow()
				limit := popMaxR
				if a.cursorRow >= limit {
					limit = maxR - 1
				}
				for r := a.cursorRow + 1; r <= limit; r++ {
					if isPop(a.cursorCol, r) {
						a.cursorRow = r
						return
					}
				}
				if a.cursorRow < popMaxR {
					a.cursorRow = popMaxR
				} else {
					a.cursorRow = maxR - 1
				}
			}
		} else { // dRow < 0
			if a.cursorRow <= 0 {
				return
			}
			curPop := isPop(a.cursorCol, a.cursorRow)
			prevPop := isPop(a.cursorCol, a.cursorRow-1)
			if curPop && prevPop {
				for r := a.cursorRow - 1; r >= 0; r-- {
					if !isPop(a.cursorCol, r) {
						a.cursorRow = r + 1
						return
					}
				}
				a.cursorRow = 0
			} else {
				for r := a.cursorRow - 1; r >= 0; r-- {
					if isPop(a.cursorCol, r) {
						a.cursorRow = r
						return
					}
				}
				a.cursorRow = 0
			}
		}
	}
}

func (a *App) commitAndMove(dCol, dRow int) {
	a.pushUndo()
	a.sheet.SetCellInput(a.cursorCol, a.cursorRow, string(a.inputBuffer), nil)
	a.inputBuffer = nil
	a.inputCursorPos = 0
	a.mode = "READY"

	if dCol < 0 && a.cursorCol > 0 {
		a.cursorCol += dCol
	} else if dCol > 0 && a.cursorCol < a.sheet.MaxCols()-1 {
		a.cursorCol += dCol
	}

	if dRow < 0 && a.cursorRow > 0 {
		a.cursorRow += dRow
	} else if dRow > 0 && a.cursorRow < a.sheet.MaxRows()-1 {
		a.cursorRow += dRow
	}
}

func inputEndsWithPointTrigger(s string) bool {
	s = strings.TrimRight(s, " \t")
	if s == "" {
		return false
	}
	for _, suf := range []string{"..", "<>", "<=", ">=", "+", "-", "*", "/", "^", "&", "(", ",", "=", ":", "<", ">"} {
		if strings.HasSuffix(s, suf) {
			return true
		}
	}
	return false
}

func (a *App) handleInputKey(ev *tcell.EventKey) {
	// Point mode trigger on arrow keys after operator
	if ev.Key() == tcell.KeyUp || ev.Key() == tcell.KeyDown || ev.Key() == tcell.KeyLeft || ev.Key() == tcell.KeyRight {
		str := string(a.inputBuffer)
		if inputEndsWithPointTrigger(str) {
			a.mode = "POINT"
			a.pointOriginCol = a.cursorCol
			a.pointOriginRow = a.cursorRow
			a.pointAnchor = nil
			a.selectedRange = nil
			a.handlePointKey(ev)
			return
		}
	}
	a.handleBufferEditing(ev)
}

func (a *App) handleBufferEditing(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyEnter:
		a.pushUndo()
		a.sheet.SetCellInput(a.cursorCol, a.cursorRow, string(a.inputBuffer), nil)
		a.inputBuffer = nil
		a.inputCursorPos = 0
		a.mode = "READY"

	case tcell.KeyDown:
		a.commitAndMove(0, 1)

	case tcell.KeyUp:
		a.commitAndMove(0, -1)

	case tcell.KeyTab:
		if ev.Modifiers()&tcell.ModShift != 0 {
			a.commitAndMove(-1, 0)
		} else {
			a.commitAndMove(1, 0)
		}

	case tcell.KeyBacktab:
		a.commitAndMove(-1, 0)

	case tcell.KeyEscape:
		a.inputBuffer = nil
		a.inputCursorPos = 0
		a.mode = "READY"

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if a.inputCursorPos > 0 {
			a.inputBuffer = append(a.inputBuffer[:a.inputCursorPos-1], a.inputBuffer[a.inputCursorPos:]...)
			a.inputCursorPos--
		}

	case tcell.KeyDelete:
		if a.inputCursorPos < len(a.inputBuffer) {
			a.inputBuffer = append(a.inputBuffer[:a.inputCursorPos], a.inputBuffer[a.inputCursorPos+1:]...)
		}

	case tcell.KeyLeft:
		if a.inputCursorPos > 0 {
			a.inputCursorPos--
		}
	case tcell.KeyRight:
		if a.inputCursorPos < len(a.inputBuffer) {
			a.inputCursorPos++
		}
	case tcell.KeyHome:
		a.inputCursorPos = 0
	case tcell.KeyEnd:
		a.inputCursorPos = len(a.inputBuffer)

	case tcell.KeyF3: // Insert Function picker in INPUT/EDIT mode
		snippet := ShowFunctionPickerModal(a.screen, a.styles)
		if snippet != "" {
			snRunes := []rune(snippet)
			a.inputBuffer = append(a.inputBuffer[:a.inputCursorPos], append(snRunes, a.inputBuffer[a.inputCursorPos:]...)...)
			a.inputCursorPos += len(snRunes)
		}

	case tcell.KeyRune:
		r := ev.Rune()
		a.inputBuffer = append(a.inputBuffer[:a.inputCursorPos], append([]rune{r}, a.inputBuffer[a.inputCursorPos:]...)...)
		a.inputCursorPos++
	}
}

func (a *App) handlePointKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyUp:
		if a.cursorRow > 0 {
			a.cursorRow--
		}
	case tcell.KeyDown:
		if a.cursorRow < a.sheet.MaxRows()-1 {
			a.cursorRow++
		}
	case tcell.KeyLeft:
		if a.cursorCol > 0 {
			a.cursorCol--
		}
	case tcell.KeyRight:
		if a.cursorCol < a.sheet.MaxCols()-1 {
			a.cursorCol++
		}
	case tcell.KeyEscape:
		a.cursorCol = a.pointOriginCol
		a.cursorRow = a.pointOriginRow
		a.pointAnchor = nil
		a.selectedRange = nil
		a.mode = "INPUT"
		return

	case tcell.KeyRune:
		if ev.Rune() == '.' || ev.Rune() == ':' {
			if a.pointAnchor == nil {
				a.pointAnchor = &[2]int{a.cursorCol, a.cursorRow}
			} else {
				a.pointAnchor = nil
				a.selectedRange = nil
			}
			return
		}
		if strings.ContainsRune("+-*/^),;<>&=", ev.Rune()) {
			a.finishPoint(string(ev.Rune()))
			return
		}

	case tcell.KeyEnter:
		a.finishPoint("")
		return
	}

	if a.pointAnchor != nil {
		startRef := coord.CellRef{Col: a.pointAnchor[0], Row: a.pointAnchor[1]}
		endRef := coord.CellRef{Col: a.cursorCol, Row: a.cursorRow}
		a.selectedRange = &coord.RangeRef{Start: startRef, End: endRef}
	}
}

func (a *App) finishPoint(appendOp string) {
	cStr := fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
	refStr := cStr
	if a.pointAnchor != nil {
		aStr := fmt.Sprintf("%s%d", getColLetter(a.pointAnchor[0]), a.pointAnchor[1]+1)
		refStr = fmt.Sprintf("%s..%s", aStr, cStr)
	}

	a.inputBuffer = append(a.inputBuffer, []rune(refStr)...)
	if appendOp != "" {
		a.inputBuffer = append(a.inputBuffer, []rune(appendOp)...)
	}
	a.inputCursorPos = len(a.inputBuffer)
	a.cursorCol = a.pointOriginCol
	a.cursorRow = a.pointOriginRow
	a.pointAnchor = nil
	a.selectedRange = nil
	a.mode = "INPUT"
}

func (a *App) handleMenuKey(ev *tcell.EventKey) {
	if len(a.menuStack) == 0 {
		a.mode = "READY"
		return
	}

	curState := a.menuStack[len(a.menuStack)-1]
	children := curState.menu.Children

	if ev.Key() == tcell.KeyEscape {
		a.menuStack = a.menuStack[:len(a.menuStack)-1]
		if len(a.menuStack) == 0 {
			a.mode = "READY"
		}
		return
	}

	switch ev.Key() {
	case tcell.KeyLeft:
		if len(a.menuStack) > 1 {
			// Switch top menu category
			topState := a.menuStack[0]
			topCategories := a.rootMenu.Children
			topState.selectedIdx = (topState.selectedIdx - 1 + len(topCategories)) % len(topCategories)
			a.menuStack = []*menuState{
				topState,
				{menu: topCategories[topState.selectedIdx], selectedIdx: 0},
			}
		} else {
			curState.selectedIdx = (curState.selectedIdx - 1 + len(children)) % len(children)
		}

	case tcell.KeyRight:
		if len(a.menuStack) > 1 {
			// Switch top menu category
			topState := a.menuStack[0]
			topCategories := a.rootMenu.Children
			topState.selectedIdx = (topState.selectedIdx + 1) % len(topCategories)
			a.menuStack = []*menuState{
				topState,
				{menu: topCategories[topState.selectedIdx], selectedIdx: 0},
			}
		} else {
			curState.selectedIdx = (curState.selectedIdx + 1) % len(children)
		}

	case tcell.KeyUp:
		curState.selectedIdx = (curState.selectedIdx - 1 + len(children)) % len(children)

	case tcell.KeyDown:
		curState.selectedIdx = (curState.selectedIdx + 1) % len(children)

	case tcell.KeyEnter:
		if len(children) > 0 && curState.selectedIdx >= 0 && curState.selectedIdx < len(children) {
			selectedItem := children[curState.selectedIdx]
			a.executeMenuItem(selectedItem)
		}

	case tcell.KeyRune:
		chUpper := strings.ToUpper(string(ev.Rune()))

		// 1. First, check if chUpper matches an item in the CURRENT dropdown submenu!
		for idx, item := range children {
			if strings.ToUpper(item.Key) == chUpper {
				curState.selectedIdx = idx
				a.executeMenuItem(item)
				return
			}
		}

		// 2. If not found in current submenu, check if chUpper matches a top category to switch to!
		topState := a.menuStack[0]
		for idx, topCat := range a.rootMenu.Children {
			if strings.ToUpper(topCat.Key) == chUpper {
				topState.selectedIdx = idx
				a.menuStack = []*menuState{
					topState,
					{menu: topCat, selectedIdx: 0},
				}
				return
			}
		}
	}
}

func (a *App) executeMenuItem(item *MenuItem) {
	if item.ActionType == ActionSubmenu && len(item.Children) > 0 {
		a.menuStack = append(a.menuStack, &menuState{menu: item, selectedIdx: 0})
	} else if item.ActionType == ActionPrompt {
		a.startPromptChain(item.PromptChain, item.ActionHandler)
	} else if item.ActionType == ActionExecute {
		a.mode = "READY"
		a.menuStack = nil
		a.dispatchAction(item.ActionHandler, nil)
	}
}

func (a *App) startPromptChain(chain []PromptItem, handler string) {
	a.promptChain = chain
	a.currentPromptIdx = 0
	a.promptResults = make(map[string]string)
	a.promptHandler = handler
	a.mode = "PROMPT"
	a.setupPrompt()
}

func (a *App) setupPrompt() {
	item := a.promptChain[a.currentPromptIdx]
	a.promptText = item.Prompt
	def := item.Default
	if item.Key == "width" {
		def = strconv.Itoa(a.sheet.GetColWidth(a.cursorCol))
	} else if item.Key == "col" {
		if def == "" {
			def = getColLetter(a.cursorCol)
		}
	} else if item.Key == "row" {
		if def == "" {
			def = strconv.Itoa(a.cursorRow + 1)
		}
	} else if a.promptHandler == "doDataFill" {
		if item.Key == "range" {
			if a.selectedRange != nil {
				def = a.selectedRange.String()
			} else if item.UseCursor {
				cStr := fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
				def = fmt.Sprintf("%s..%s", cStr, cStr)
			}
		} else if item.Key == "start" {
			targetRng := a.selectedRange
			if targetStr, ok := a.promptResults["range"]; ok && targetStr != "" {
				if r, err := coord.ParseRangeRef(targetStr); err == nil {
					targetRng = &r
				}
			}
			if targetRng != nil {
				c1 := a.sheet.GetCell(targetRng.MinCol(), targetRng.MinRow())
				if c1 != nil && c1.Value != nil {
					def = fmt.Sprintf("%v", c1.Value)
				}
			}
		} else if item.Key == "step" {
			targetRng := a.selectedRange
			if targetStr, ok := a.promptResults["range"]; ok && targetStr != "" {
				if r, err := coord.ParseRangeRef(targetStr); err == nil {
					targetRng = &r
				}
			}
			if targetRng != nil {
				minC, minR := targetRng.MinCol(), targetRng.MinRow()
				c1 := a.sheet.GetCell(minC, minR)
				var c2 *cell.Cell
				if targetRng.MaxRow() > minR {
					c2 = a.sheet.GetCell(minC, minR+1)
				} else if targetRng.MaxCol() > minC {
					c2 = a.sheet.GetCell(minC+1, minR)
				}
				if c1 != nil && c1.Value != nil && c2 != nil && c2.Value != nil {
					str1 := fmt.Sprintf("%v", c1.Value)
					str2 := fmt.Sprintf("%v", c2.Value)
					if t1, _, ok1 := sheet.ParseFlexibleDate(str1); ok1 {
						if t2, _, ok2 := sheet.ParseFlexibleDate(str2); ok2 {
							diffDays := int(t2.Sub(t1).Hours() / 24)
							if diffDays != 0 {
								def = fmt.Sprintf("%dd", diffDays)
							} else {
								def = "1d"
							}
						}
					} else if v1, err1 := strconv.ParseFloat(str1, 64); err1 == nil {
						if v2, err2 := strconv.ParseFloat(str2, 64); err2 == nil {
							def = fmt.Sprintf("%g", v2-v1)
						}
					}
				} else if c1 != nil && c1.Value != nil {
					str1 := fmt.Sprintf("%v", c1.Value)
					if _, _, ok := sheet.ParseFlexibleDate(str1); ok {
						def = "1d"
					}
				}
			}
		}
	} else if a.promptHandler == "doRangeTranspose" {
		if item.Key == "source" {
			if a.selectedRange != nil {
				def = a.selectedRange.String()
			} else if item.UseCursor {
				cStr := fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
				def = fmt.Sprintf("%s..%s", cStr, cStr)
			}
		} else if item.Key == "target" {
			if def == "" || def == "A1" {
				def = fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
			}
		}
	} else if item.Key == "range" && strings.HasPrefix(a.promptHandler, "doDataSort") {
		if a.selectedRange != nil {
			def = a.selectedRange.String()
		} else if a.sortDataRange != nil {
			def = a.sortDataRange.String()
		} else if item.UseCursor {
			cStr := fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
			def = fmt.Sprintf("%s..%s", cStr, cStr)
		}
	} else if strings.HasPrefix(a.promptHandler, "doGraphSet") {
		if a.promptHandler == "doGraphSetX" {
			if a.selectedRange != nil {
				def = a.selectedRange.String()
			} else if a.sheet.Graph().RangeX != nil {
				def = a.sheet.Graph().RangeX.String()
			} else if item.UseCursor {
				cStr := fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
				def = fmt.Sprintf("%s..%s", cStr, cStr)
			}
		} else {
			sKey := ""
			switch a.promptHandler {
			case "doGraphSetA":
				sKey = "A"
			case "doGraphSetB":
				sKey = "B"
			case "doGraphSetC":
				sKey = "C"
			case "doGraphSetD":
				sKey = "D"
			case "doGraphSetE":
				sKey = "E"
			case "doGraphSetF":
				sKey = "F"
			}
			if sKey != "" {
				if a.selectedRange != nil {
					def = a.selectedRange.String()
				} else if a.sheet.Graph().Series != nil && a.sheet.Graph().Series[sKey] != nil {
					def = a.sheet.Graph().Series[sKey].String()
				} else if item.UseCursor {
					cStr := fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
					def = fmt.Sprintf("%s..%s", cStr, cStr)
				}
			}
		}
	} else if a.promptHandler == "doExportCSVRange" || a.promptHandler == "doExportXLSXRange" || a.promptHandler == "doExportODSRange" || a.promptHandler == "doExportMarkdownRange" {
		if a.selectedRange != nil {
			def = a.selectedRange.String()
		} else {
			minC, minR, maxC, maxR := a.sheet.BoundingBox()
			if maxC >= minC && maxR >= minR {
				def = fmt.Sprintf("%s..%s", coord.CellRef{Col: minC, Row: minR}.String(), coord.CellRef{Col: maxC, Row: maxR}.String())
			} else if item.UseCursor {
				cStr := fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
				def = fmt.Sprintf("%s..%s", cStr, cStr)
			}
		}
	} else if item.UseCursor {
		if a.selectedRange != nil {
			def = a.selectedRange.String()
		} else {
			cStr := fmt.Sprintf("%s%d", getColLetter(a.cursorCol), a.cursorRow+1)
			def = fmt.Sprintf("%s..%s", cStr, cStr)
		}
	}
	a.inputBuffer = []rune(def)
	a.inputCursorPos = len(a.inputBuffer)
}

func (a *App) handlePromptKey(ev *tcell.EventKey) {
	if ev.Key() == tcell.KeyEscape {
		a.mode = "READY"
		a.menuStack = nil
		a.selectedRange = nil
		a.pendingExportRange = nil
		return
	}

	if ev.Key() == tcell.KeyEnter {
		item := a.promptChain[a.currentPromptIdx]
		a.promptResults[item.Key] = strings.TrimSpace(string(a.inputBuffer))

		a.currentPromptIdx++
		if a.currentPromptIdx < len(a.promptChain) {
			a.setupPrompt()
		} else {
			a.mode = "READY"
			a.menuStack = nil
			a.selectedRange = nil
			a.dispatchAction(a.promptHandler, a.promptResults)
		}
		return
	}

	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if a.inputCursorPos > 0 {
			a.inputBuffer = append(a.inputBuffer[:a.inputCursorPos-1], a.inputBuffer[a.inputCursorPos:]...)
			a.inputCursorPos--
		}
	case tcell.KeyDelete:
		if a.inputCursorPos < len(a.inputBuffer) {
			a.inputBuffer = append(a.inputBuffer[:a.inputCursorPos], a.inputBuffer[a.inputCursorPos+1:]...)
		}
	case tcell.KeyLeft:
		if a.inputCursorPos > 0 {
			a.inputCursorPos--
		}
	case tcell.KeyRight:
		if a.inputCursorPos < len(a.inputBuffer) {
			a.inputCursorPos++
		}
	case tcell.KeyRune:
		r := ev.Rune()
		a.inputBuffer = append(a.inputBuffer[:a.inputCursorPos], append([]rune{r}, a.inputBuffer[a.inputCursorPos:]...)...)
		a.inputCursorPos++
	}
}

func (a *App) dispatchPaletteAction(action string) {
	switch action {
	case "doPaletteUndo":
		a.doUndo()
	case "doPaletteRedo":
		a.doRedo()
	case "doPaletteAutoSum":
		a.doAutoSum()
	case "doPaletteAverage":
		a.doAverage()
	case "doPaletteCount":
		a.doCount()
	case "doPaletteFormatCurrency":
		a.startPromptChain([]PromptItem{
			{Key: "dec", Prompt: "Decimal places (0..15): ", Default: "2"},
			{Key: "symbol", Prompt: "Currency symbol (e.g. $, ¥, €, £): ", Default: "$"},
			{Key: "range", Prompt: "Range to format: ", UseCursor: true},
		}, "doRangeFormatCurrency")
	case "doPaletteFormatPercent":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.FormatRange(target, cell.CellFormat{Type: cell.FmtPercent, Decimals: 2})
		a.statusMessage = fmt.Sprintf("Formatted %s to Percent.", target)
	case "doPaletteFormatFixed":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.FormatRange(target, cell.CellFormat{Type: cell.FmtFixed, Decimals: 2})
		a.statusMessage = fmt.Sprintf("Formatted %s to Fixed (2).", target)
	case "doPaletteFormatComma":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.FormatRange(target, cell.CellFormat{Type: cell.FmtComma, Decimals: 2})
		a.statusMessage = fmt.Sprintf("Formatted %s to Comma.", target)
	case "doPaletteFormatDate":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.FormatRange(target, cell.CellFormat{Type: cell.FmtDate, DateFormat: 1})
		a.statusMessage = fmt.Sprintf("Formatted %s to Date.", target)
	case "doPaletteFormatScientific":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.FormatRange(target, cell.CellFormat{Type: cell.FmtScientific, Decimals: 2})
		a.statusMessage = fmt.Sprintf("Formatted %s to Scientific.", target)
	case "doPaletteFormatGeneral":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.FormatRange(target, cell.CellFormat{Type: cell.FmtGeneral})
		a.statusMessage = fmt.Sprintf("Formatted %s to General.", target)
	case "doPaletteSetColWidth":
		a.startPromptChain([]PromptItem{{Key: "width", Prompt: "Column width (1..72): ", Default: "9"}}, "doColumnSetWidth")
	case "doEditActiveCell":
		c := a.sheet.GetCell(a.cursorCol, a.cursorRow)
		raw := ""
		if c != nil {
			raw = c.RawInput
		}
		a.inputBuffer = []rune(raw)
		a.inputCursorPos = len(a.inputBuffer)
		a.mode = "EDIT"
	case "doPaletteCut":
		a.doCutSelection()
	case "doPaletteCopy":
		a.doCopySelection()
	case "doPalettePaste":
		a.doPasteClipboard()
	case "doPalettePasteLink":
		a.doPasteLink()
	case "doPalettePasteValues":
		a.doPasteValues()
	case "doPasteTranspose":
		a.doPasteTranspose()
	case "doAutoFill":
		a.doAutoFill()
	case "doPaletteClear":
		a.pushUndo()
		if a.selectedRange != nil {
			a.sheet.ClearRange(*a.selectedRange)
			a.clearSelection()
		} else {
			a.sheet.ClearCell(a.cursorCol, a.cursorRow)
		}
		a.statusMessage = "Cleared cells."
	case "doPaletteSelectAll":
		a.doSelectAll()
	case "doPaletteSortAsc":
		target := a.getTargetRange()
		a.startPromptChain([]PromptItem{
			{Key: "range", Prompt: "Enter range to sort: ", Default: target.String(), UseCursor: true},
			{Key: "col", Prompt: "Sort key column (A-Z) or cell: ", Default: getColLetter(a.cursorCol)},
		}, "doDataSortAscPrompt")
	case "doPaletteSortDesc":
		target := a.getTargetRange()
		a.startPromptChain([]PromptItem{
			{Key: "range", Prompt: "Enter range to sort: ", Default: target.String(), UseCursor: true},
			{Key: "col", Prompt: "Sort key column (A-Z) or cell: ", Default: getColLetter(a.cursorCol)},
		}, "doDataSortDescPrompt")
	case "doPaletteSortHorizAsc":
		target := a.getTargetRange()
		a.startPromptChain([]PromptItem{
			{Key: "range", Prompt: "Enter range to sort horizontally: ", Default: target.String(), UseCursor: true},
			{Key: "row", Prompt: "Sort key row number (e.g. 1, 2) or cell: ", Default: strconv.Itoa(a.cursorRow + 1)},
		}, "doDataSortHorizAscPrompt")
	case "doPaletteSortHorizDesc":
		target := a.getTargetRange()
		a.startPromptChain([]PromptItem{
			{Key: "range", Prompt: "Enter range to sort horizontally: ", Default: target.String(), UseCursor: true},
			{Key: "row", Prompt: "Sort key row number (e.g. 1, 2) or cell: ", Default: strconv.Itoa(a.cursorRow + 1)},
		}, "doDataSortHorizDescPrompt")
	case "doPaletteRecalc":
		a.sheet.Recalculate()
		a.statusMessage = "Recalculated worksheet."
	case "doPaletteGraphView":
		RenderGraphScreen(a.screen, a.sheet, a.styles, a.filename)
	case "doPaletteGraphLine":
		a.pushUndo()
		a.sheet.Graph().Type = "LINE"
		a.statusMessage = "Graph type: Line."
	case "doPaletteGraphBar":
		a.pushUndo()
		a.sheet.Graph().Type = "BAR"
		a.statusMessage = "Graph type: Bar."
	case "doPaletteGraphStacked":
		a.pushUndo()
		a.sheet.Graph().Type = "STACKED"
		a.statusMessage = "Graph type: Stacked-Bar."
	case "doPaletteGraphPie":
		a.pushUndo()
		a.sheet.Graph().Type = "PIE"
		a.statusMessage = "Graph type: Pie."
	case "doPaletteGraphSetX":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.Graph().RangeX = &target
		a.statusMessage = fmt.Sprintf("Graph X-range set to %s.", target)
	case "doPaletteGraphSetA":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.Graph().Series["A"] = &target
		a.statusMessage = fmt.Sprintf("Graph Series A set to %s.", target)
	case "doPaletteGraphSetB":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.Graph().Series["B"] = &target
		a.statusMessage = fmt.Sprintf("Graph Series B set to %s.", target)
	case "doPaletteGraphSetC":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.Graph().Series["C"] = &target
		a.statusMessage = fmt.Sprintf("Graph Series C set to %s.", target)
	case "doPaletteGraphSetD":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.Graph().Series["D"] = &target
		a.statusMessage = fmt.Sprintf("Graph Series D set to %s.", target)
	case "doPaletteGraphSetE":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.Graph().Series["E"] = &target
		a.statusMessage = fmt.Sprintf("Graph Series E set to %s.", target)
	case "doPaletteGraphSetF":
		a.pushUndo()
		target := a.getTargetRange()
		a.sheet.Graph().Series["F"] = &target
		a.statusMessage = fmt.Sprintf("Graph Series F set to %s.", target)
	case "doPaletteGraphSetTitle":
		a.startPromptChain([]PromptItem{
			{Key: "title", Prompt: "Enter graph title (or leave blank to clear): ", Default: a.sheet.Graph().Title},
		}, "doGraphSetTitle")
	case "doPaletteGraphStatus", "doGraphStatus":
		RenderGraphStatusScreen(a.screen, a.sheet, a.styles)
	case "doRangeNameList":
		RenderNamedRangesScreen(a.screen, a.sheet.NamedRanges(), a.styles)
	case "doSheetSwitchModal":
		a.doSheetSwitchModal()
	case "doPaletteSheetNext":
		a.doSheetNext()
	case "doPaletteSheetPrev":
		a.doSheetPrev()
	case "doWorksheetAdd":
		name := strings.TrimSpace(a.promptResults["name"])
		if name != "" {
			a.doAddSheet(name)
		}
	case "doWorksheetDelete":
		a.doDeleteSheet()
	case "doWorksheetRename":
		name := strings.TrimSpace(a.promptResults["name"])
		if name != "" {
			a.doRenameSheet(name)
		}
	case "doOpenFunctionPicker":
		a.doOpenFunctionPicker()
	case "doPaletteMax":
		a.doMax()
	case "doPaletteMin":
		a.doMin()
	case "doPaletteToday":
		a.doInsertToday()
	case "doPaletteNow":
		a.doInsertNow()
	case "doPaletteLineSingle":
		a.doInsertRepeatLine("-")
	case "doPaletteLineDouble":
		a.doInsertRepeatLine("=")
	case "doResetColWidth":
		a.doResetColWidth()
	case "doPaletteInsertRow":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Insert row range: ", UseCursor: true}}, "doInsertRow")
	case "doPaletteInsertCol":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Insert column range: ", UseCursor: true}}, "doInsertCol")
	case "doPaletteDeleteRow":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Delete row range: ", UseCursor: true}}, "doDeleteRow")
	case "doPaletteDeleteCol":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Delete column range: ", UseCursor: true}}, "doDeleteCol")
	case "doPaletteAddSheet":
		a.startPromptChain([]PromptItem{{Key: "name", Prompt: "Enter new worksheet name: "}}, "doWorksheetAdd")
	case "doPaletteRenameSheet":
		a.startPromptChain([]PromptItem{{Key: "name", Prompt: "Enter new worksheet name: "}}, "doWorksheetRename")
	case "doPaletteGlobalColWidth":
		a.startPromptChain([]PromptItem{{Key: "width", Prompt: "Enter global column width (1..72): ", Default: "9"}}, "doGlobalColWidth")
	case "doPaletteGoto":
		a.startPromptChain([]PromptItem{{Key: "cell", Prompt: "Enter address or name to go to (e.g. B10, Sheet2!A1, Total): "}}, "doGotoCell")
	case "doPaletteFind":
		a.startPromptChain([]PromptItem{{Key: "query", Prompt: "Find text/number/formula: ", Default: a.lastSearchQuery}}, "doFindPrompt")
	case "doPaletteReplace":
		a.startReplacePrompt()
	case "doPaletteFindAll":
		a.startPromptChain([]PromptItem{{Key: "query", Prompt: "Search all sheets for: ", Default: a.lastSearchQuery}}, "doFindAllPrompt")
	case "doPaletteDataFill":
		a.startPromptChain([]PromptItem{
			{Key: "range", Prompt: "Fill range: ", UseCursor: true},
			{Key: "start", Prompt: "Start value (number or YYYY-MM-DD): ", Default: "1"},
			{Key: "step", Prompt: "Step increment (e.g. 1, 5, 1d, 1m): ", Default: "1"},
			{Key: "stop", Prompt: "Stop value (optional): ", Default: ""},
		}, "doDataFill")
	case "doPaletteTranspose":
		a.startPromptChain([]PromptItem{
			{Key: "source", Prompt: "Source range to transpose: ", UseCursor: true},
			{Key: "target", Prompt: "Target top-left cell: ", Default: "A1"},
		}, "doRangeTranspose")
	case "doFreezeBoth":
		a.sheet.SetFrozenRows(a.cursorRow)
		a.sheet.SetFrozenCols(a.cursorCol)
		a.statusMessage = fmt.Sprintf("Frozen rows 1..%d and columns A..%s.", a.cursorRow, getColLetter(a.cursorCol-1))
	case "doFreezeHorizontal":
		a.sheet.SetFrozenRows(a.cursorRow)
		a.sheet.SetFrozenCols(0)
		a.statusMessage = fmt.Sprintf("Frozen rows 1..%d at top.", a.cursorRow)
	case "doFreezeVertical":
		a.sheet.SetFrozenRows(0)
		a.sheet.SetFrozenCols(a.cursorCol)
		a.statusMessage = fmt.Sprintf("Frozen columns A..%s at left.", getColLetter(a.cursorCol-1))
	case "doFreezeClear":
		a.sheet.SetFrozenRows(0)
		a.sheet.SetFrozenCols(0)
		a.statusMessage = "Cleared window titles (freeze panes)."
	case "doFindNext":
		a.findNext()
	case "doFindPrev":
		a.findPrev()
	case "doPaletteSave", "doFileSaveDialog":
		a.filePicker.Open(FilePickerModeSave, a.filename)
	case "doPaletteOpen", "doFileOpenDialog":
		a.filePicker.Open(FilePickerModeOpen, a.filename)
	case "doFileExportCSVFullDialog":
		a.pendingExportRange = nil
		a.filePicker.Open(FilePickerModeExportCSV, "sheet.csv")
	case "doFileExportXLSXFullDialog":
		a.pendingExportRange = nil
		a.filePicker.Open(FilePickerModeExportXLSX, "workbook.xlsx")
	case "doFileExportODSFullDialog":
		a.pendingExportRange = nil
		a.filePicker.Open(FilePickerModeExportODS, "workbook.ods")
	case "doPaletteExportCSV", "doFileExportCSVDialog", "doFileExportCSVRangeDialog":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Enter range to export (e.g. A1..D10): ", UseCursor: true}}, "doExportCSVRange")
	case "doPaletteExportXLSX", "doFileExportXLSXDialog", "doFileExportXLSXRangeDialog":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Enter range to export (e.g. A1..D10): ", UseCursor: true}}, "doExportXLSXRange")
	case "doPaletteExportODS", "doFileExportODSDialog", "doFileExportODSRangeDialog":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Enter range to export (e.g. A1..D10): ", UseCursor: true}}, "doExportODSRange")
	case "doPaletteExportMarkdown", "doFileExportMarkdownDialog", "doFileExportMarkdownRangeDialog":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Enter range to export (e.g. A1..D10): ", UseCursor: true}}, "doExportMarkdownRange")
	case "doPaletteImportCSV", "doFileImportCSVDialog":
		a.filePicker.Open(FilePickerModeImportCSV, "sheet.csv")
	case "doPaletteErase", "doWorksheetErase":
		a.tryNewWorksheet()
	case "doQuitApp":
		a.tryQuitApp()
	case "doPaletteHelp":
		RenderHelpScreen(a.screen, a.styles)
	case "doOpenAbout", "doPaletteAbout":
		RenderAboutScreen(a.screen, a.styles)
	}
}

func (a *App) tryQuitApp() {
	wb := a.currentWorkbook()
	dirty := a.sheet.IsModified()
	if wb != nil && wb.IsModified() {
		dirty = true
	}
	if !dirty {
		a.running = false
		return
	}
	a.confirmAction = ConfirmQuit
}

func (a *App) tryNewWorksheet() {
	wb := a.currentWorkbook()
	dirty := a.sheet.IsModified()
	if wb != nil && wb.IsModified() {
		dirty = true
	}
	if !dirty {
		a.createNewWorksheet()
		return
	}
	a.confirmAction = ConfirmNew
}

func (a *App) resetViewportAndViews() {
	a.cursorCol = 0
	a.cursorRow = 0
	a.leftCol = 0
	a.topRow = 0
	a.sheetViews = make(map[string]sheetViewState)
	a.clearSelection()
}

func (a *App) createNewWorksheet() {
	a.pushUndoWorkbook()
	a.filename = AbsolutePath(GenerateUnusedFilename(".", "DATA", ".hwk"))
	wb := sheet.NewWorkbook(a.filename)
	a.workbook = wb
	a.sheet = wb.GetActiveSheet()
	a.resetViewportAndViews()
	a.statusMessage = fmt.Sprintf("New workbook '%s'.", filepath.Base(a.filename))
}

func (a *App) getTargetRange() coord.RangeRef {
	if a.selectedRange != nil {
		return *a.selectedRange
	}
	return coord.RangeRef{
		Start: coord.CellRef{Col: a.cursorCol, Row: a.cursorRow},
		End:   coord.CellRef{Col: a.cursorCol, Row: a.cursorRow},
	}
}

func (a *App) doAutoFunction(fnName string) {
	fnUpper := strings.ToUpper(fnName)
	// 1. If range selected, put formula just outside the selection (Excel-like).
	if a.selectedRange != nil {
		sel := *a.selectedRange
		destCol := a.cursorCol
		destRow := a.cursorRow
		// Prefer the cell below a vertical block, else right of a horizontal block.
		if sel.Contains(destCol, destRow) {
			if sel.MaxRow() == sel.MinRow() && sel.MaxCol() > sel.MinCol() {
				// Horizontal row block: place sum to the right
				if sel.MaxCol()+1 < a.sheet.MaxCols() {
					destCol = sel.MaxCol() + 1
					destRow = sel.MinRow()
				} else {
					a.statusMessage = "No empty cell adjacent to selection for AutoSum."
					return
				}
			} else {
				// Vertical block or multi-row block: place sum below
				if sel.MaxRow()+1 < a.sheet.MaxRows() {
					destRow = sel.MaxRow() + 1
					destCol = sel.MinCol()
				} else if sel.MaxCol()+1 < a.sheet.MaxCols() {
					destCol = sel.MaxCol() + 1
					destRow = sel.MinRow()
				} else {
					a.statusMessage = "No empty cell adjacent to selection for AutoSum."
					return
				}
			}
		}
		a.pushUndo()
		a.cursorCol = destCol
		a.cursorRow = destRow
		formulaStr := fmt.Sprintf("=%s(%s)", fnUpper, sel)
		a.sheet.SetCellInput(destCol, destRow, formulaStr, nil)
		a.statusMessage = fmt.Sprintf("Inserted %s.", formulaStr)
		a.clearSelection()
		return
	}

	// 2. Scan above
	c := a.cursorCol
	r := a.cursorRow
	if r > 0 && isNumberCell(a.sheet.GetCell(c, r-1)) {
		rTop := r - 1
		for rTop > 0 && isNumberCell(a.sheet.GetCell(c, rTop-1)) {
			rTop--
		}
		topRef := coord.CellRef{Col: c, Row: rTop}
		botRef := coord.CellRef{Col: c, Row: r - 1}
		formulaStr := fmt.Sprintf("=%s(%s..%s)", fnUpper, topRef, botRef)
		a.pushUndo()
		a.sheet.SetCellInput(c, r, formulaStr, nil)
		a.statusMessage = fmt.Sprintf("Inserted %s.", formulaStr)
		return
	}

	// 3. Scan left
	if c > 0 && isNumberCell(a.sheet.GetCell(c-1, r)) {
		cLeft := c - 1
		for cLeft > 0 && isNumberCell(a.sheet.GetCell(cLeft-1, r)) {
			cLeft--
		}
		leftRef := coord.CellRef{Col: cLeft, Row: r}
		rightRef := coord.CellRef{Col: c - 1, Row: r}
		formulaStr := fmt.Sprintf("=%s(%s..%s)", fnUpper, leftRef, rightRef)
		a.pushUndo()
		a.sheet.SetCellInput(c, r, formulaStr, nil)
		a.statusMessage = fmt.Sprintf("Inserted %s.", formulaStr)
		return
	}

	// 4. Default: open input buffer with =FUNC(
	a.inputBuffer = []rune(fmt.Sprintf("=%s(", fnUpper))
	a.inputCursorPos = len(a.inputBuffer)
	a.mode = "INPUT"
}

func (a *App) doAutoSum() {
	a.doAutoFunction("SUM")
}

func (a *App) doAverage() {
	a.doAutoFunction("AVERAGE")
}

func (a *App) doCount() {
	a.doAutoFunction("COUNT")
}

func (a *App) doMax() {
	a.doAutoFunction("MAX")
}

func (a *App) doMin() {
	a.doAutoFunction("MIN")
}

func (a *App) doInsertToday() {
	a.pushUndo()
	a.sheet.SetCellInput(a.cursorCol, a.cursorRow, "=TODAY()", &cell.CellFormat{Type: cell.FmtDate, DateFormat: 1})
	a.statusMessage = "Inserted =TODAY()."
}

func (a *App) doInsertNow() {
	a.pushUndo()
	a.sheet.SetCellInput(a.cursorCol, a.cursorRow, "=NOW()", &cell.CellFormat{Type: cell.FmtDate, DateFormat: 1})
	a.statusMessage = "Inserted =NOW()."
}

func (a *App) doInsertRepeatLine(ch string) {
	a.pushUndo()
	a.sheet.SetCellInput(a.cursorCol, a.cursorRow, "\\"+ch, nil)
	a.statusMessage = fmt.Sprintf("Inserted repeat line '\\%s'.", ch)
}

func (a *App) doResetColWidth() {
	a.pushUndo()
	a.sheet.ResetColWidth(a.cursorCol)
	a.statusMessage = fmt.Sprintf("Reset column %s width to default (%d).", coord.ColToLetter(a.cursorCol), a.sheet.DefaultColWidth())
}

func (a *App) doSheetSwitchModal() {
	wb := a.currentWorkbook()
	if len(wb.Sheets) <= 1 {
		a.statusMessage = "Workbook contains only 1 sheet."
		return
	}
	selected := ShowSheetPickerModal(a.screen, a.filename, wb.SheetNames(), a.styles)
	if selected >= 0 {
		a.switchSheet(selected)
	}
}

func (a *App) doSheetNext() {
	wb := a.currentWorkbook()
	if len(wb.Sheets) > 1 && wb.ActiveSheetIndex < len(wb.Sheets)-1 {
		a.switchSheet(wb.ActiveSheetIndex + 1)
	} else {
		a.statusMessage = "Already at the last sheet."
	}
}

func (a *App) doSheetPrev() {
	wb := a.currentWorkbook()
	if len(wb.Sheets) > 1 && wb.ActiveSheetIndex > 0 {
		a.switchSheet(wb.ActiveSheetIndex - 1)
	} else {
		a.statusMessage = "Already at the first sheet."
	}
}

func (a *App) doAddSheet(name string) {
	wb := a.currentWorkbook()
	a.pushUndoWorkbook()
	newSh := wb.AddSheet(name)
	a.switchSheet(len(wb.Sheets) - 1)
	a.statusMessage = fmt.Sprintf("Added and switched to sheet [%s].", newSh.Name())
}

func (a *App) doDeleteSheet() {
	wb := a.currentWorkbook()
	if len(wb.Sheets) <= 1 {
		a.statusMessage = "Cannot delete the only sheet in workbook."
		return
	}
	a.pushUndoWorkbook()
	delName := a.sheet.Name()
	delIdx := wb.ActiveSheetIndex
	if err := wb.DeleteSheet(delIdx); err != nil {
		a.statusMessage = fmt.Sprintf("Failed to delete sheet: %v", err)
		return
	}
	if a.sheetViews != nil {
		delete(a.sheetViews, delName)
	}
	newIdx := delIdx
	if newIdx >= len(wb.Sheets) {
		newIdx = len(wb.Sheets) - 1
	}
	a.switchSheet(newIdx)
	a.discardCutClipboardForSheet(delName)
	a.statusMessage = fmt.Sprintf("Deleted sheet [%s].", delName)
}

func (a *App) doRenameSheet(newName string) {
	wb := a.currentWorkbook()
	oldName := a.sheet.Name()
	a.pushUndoWorkbook()
	if err := wb.RenameSheet(oldName, newName); err != nil {
		a.statusMessage = fmt.Sprintf("Rename failed: %v", err)
		return
	}
	if a.sheetViews != nil {
		if vs, ok := a.sheetViews[oldName]; ok {
			delete(a.sheetViews, oldName)
			a.sheetViews[newName] = vs
		}
	}
	a.sheet = wb.GetSheet(newName)
	if a.sheet == nil {
		a.sheet = wb.GetActiveSheet()
	}
	a.retargetClipboardSheet(oldName, newName)
	a.statusMessage = fmt.Sprintf("Renamed sheet [%s] to [%s].", oldName, newName)
}

func isNumberCell(c *cell.Cell) bool {
	if c == nil {
		return false
	}
	if c.Type == cell.TypeNumber {
		return true
	}
	if c.Type == cell.TypeFormula && c.Value != nil {
		if _, ok := c.Value.(float64); ok {
			return true
		}
		if _, ok := c.Value.(int); ok {
			return true
		}
	}
	return false
}

func (a *App) dispatchAction(handler string, res map[string]string) {
	switch handler {
	case "doCancel":
		a.mode = "READY"
		a.menuStack = nil
	case "doQuitApp":
		a.tryQuitApp()
	case "doOpenPalette":
		a.palette.Open()
	case "doEditActiveCell":
		c := a.sheet.GetCell(a.cursorCol, a.cursorRow)
		raw := ""
		if c != nil {
			raw = c.RawInput
		}
		a.inputBuffer = []rune(raw)
		a.inputCursorPos = len(a.inputBuffer)
		a.mode = "EDIT"

	case "doPaletteUndo":
		a.doUndo()
	case "doPaletteRedo":
		a.doRedo()
	case "doPaletteCut":
		a.doCutSelection()
	case "doPaletteCopy":
		a.doCopySelection()
	case "doPalettePaste":
		a.doPasteClipboard()
	case "doPalettePasteLink":
		a.doPasteLink()
	case "doPalettePasteValues":
		a.doPasteValues()
	case "doSheetSwitchModal":
		a.doSheetSwitchModal()
	case "doPaletteSheetNext":
		a.doSheetNext()
	case "doPaletteSheetPrev":
		a.doSheetPrev()
	case "doWorksheetAdd":
		name := strings.TrimSpace(res["name"])
		if name != "" {
			a.doAddSheet(name)
		}
	case "doWorksheetDelete":
		a.doDeleteSheet()
	case "doWorksheetRename":
		name := strings.TrimSpace(res["name"])
		if name != "" {
			a.doRenameSheet(name)
		}
	case "doPaletteClear":
		a.pushUndo()
		if a.selectedRange != nil {
			a.sheet.ClearRange(*a.selectedRange)
			a.clearSelection()
		} else {
			a.sheet.ClearCell(a.cursorCol, a.cursorRow)
		}
	case "doPaletteSelectAll":
		a.doSelectAll()
	case "doPaletteAutoSum":
		a.doAutoSum()
	case "doPaletteAverage":
		a.doAverage()
	case "doPaletteCount":
		a.doCount()
	case "doPaletteMax":
		a.doMax()
	case "doPaletteMin":
		a.doMin()
	case "doPaletteToday":
		a.doInsertToday()
	case "doPaletteNow":
		a.doInsertNow()
	case "doPaletteLineSingle":
		a.doInsertRepeatLine("-")
	case "doPaletteLineDouble":
		a.doInsertRepeatLine("=")
	case "doResetColWidth":
		a.doResetColWidth()
	case "doDataSortSetRange":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.sortDataRange = &r
			a.statusMessage = fmt.Sprintf("Sort Data-Range: %s", r)
		}

	case "doDataSortSetPrimaryKey":
		colStr := res["col"]
		orderStr := strings.ToUpper(strings.TrimSpace(res["order"]))
		colIdx := parseColInput(colStr, a.cursorCol)
		a.sortPrimaryCol = colIdx
		a.sortPrimaryAsc = (orderStr != "D" && orderStr != "DESC")
		orderName := "Ascending"
		if !a.sortPrimaryAsc {
			orderName = "Descending"
		}
		a.statusMessage = fmt.Sprintf("Primary Key: Col %s (%s)", getColLetter(colIdx), orderName)

	case "doDataSortGo":
		target := a.sortDataRange
		if target == nil {
			if a.selectedRange != nil {
				target = a.selectedRange
			} else {
				a.statusMessage = "No Data-Range set! Use /DSD to set Data-Range."
				return
			}
		}
		a.pushUndo()
		a.sheet.SortRange(*target, a.sortPrimaryCol, a.sortPrimaryAsc)
		orderName := "ascending"
		if !a.sortPrimaryAsc {
			orderName = "descending"
		}
		a.statusMessage = fmt.Sprintf("Sorted %s by Col %s (%s).", *target, getColLetter(a.sortPrimaryCol), orderName)

	case "doDataSortAscPrompt":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			colIdx := parseColInput(res["col"], a.cursorCol)
			a.pushUndo()
			a.sheet.SortRange(r, colIdx, true)
			a.statusMessage = fmt.Sprintf("Sorted %s ascending by Col %s.", r, getColLetter(colIdx))
		}

	case "doDataSortDescPrompt":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			colIdx := parseColInput(res["col"], a.cursorCol)
			a.pushUndo()
			a.sheet.SortRange(r, colIdx, false)
			a.statusMessage = fmt.Sprintf("Sorted %s descending by Col %s.", r, getColLetter(colIdx))
		}

	case "doDataSortHorizAscPrompt":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			rowIdx := parseRowInput(res["row"], a.cursorRow)
			a.pushUndo()
			a.sheet.SortRangeHorizontal(r, rowIdx, true)
			a.statusMessage = fmt.Sprintf("Sorted %s horizontally ascending by Row %d.", r, rowIdx+1)
		}

	case "doDataSortHorizDescPrompt":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			rowIdx := parseRowInput(res["row"], a.cursorRow)
			a.pushUndo()
			a.sheet.SortRangeHorizontal(r, rowIdx, false)
			a.statusMessage = fmt.Sprintf("Sorted %s horizontally descending by Row %d.", r, rowIdx+1)
		}

	case "doDataSortReset":
		a.sortDataRange = nil
		a.sortPrimaryCol = 0
		a.sortPrimaryAsc = true
		a.statusMessage = "Sort settings reset."

	case "doPaletteRecalc":
		a.sheet.Recalculate()
		a.statusMessage = "Recalculated worksheet."
	case "doPaletteHelp":
		RenderHelpScreen(a.screen, a.styles)
	case "doOpenAbout", "doPaletteAbout":
		RenderAboutScreen(a.screen, a.styles)

	case "doWorksheetErase":
		a.tryNewWorksheet()

	case "doGlobalColWidth":
		if w, err := strconv.Atoi(res["width"]); err == nil {
			a.pushUndo()
			a.sheet.SetDefaultColWidth(w)
			a.statusMessage = fmt.Sprintf("Global column width set to %d.", w)
		}

	case "doInsertRow":
		a.pushUndoWorkbook()
		at, count := a.cursorRow, 1
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			at = r.MinRow()
			count = r.MaxRow() - r.MinRow() + 1
			a.sheet.InsertRow(at, count)
			a.statusMessage = fmt.Sprintf("Inserted %d row(s).", count)
		} else if cr, err := coord.ParseCellRef(res["range"]); err == nil {
			at = cr.Row
			a.sheet.InsertRow(at, 1)
			a.statusMessage = "Inserted 1 row."
		} else {
			a.sheet.InsertRow(at, 1)
			a.statusMessage = "Inserted 1 row."
		}
		a.adjustCutClipboardForRowInsert(at, count)

	case "doInsertCol":
		a.pushUndoWorkbook()
		at, count := a.cursorCol, 1
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			at = r.MinCol()
			count = r.MaxCol() - r.MinCol() + 1
			a.sheet.InsertCol(at, count)
			a.statusMessage = fmt.Sprintf("Inserted %d col(s).", count)
		} else if cr, err := coord.ParseCellRef(res["range"]); err == nil {
			at = cr.Col
			a.sheet.InsertCol(at, 1)
			a.statusMessage = "Inserted 1 col."
		} else {
			a.sheet.InsertCol(at, 1)
			a.statusMessage = "Inserted 1 col."
		}
		a.adjustCutClipboardForColInsert(at, count)

	case "doDeleteRow":
		a.pushUndoWorkbook()
		at, count := a.cursorRow, 1
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			at = r.MinRow()
			count = r.MaxRow() - r.MinRow() + 1
			a.sheet.DeleteRow(at, count)
			a.statusMessage = fmt.Sprintf("Deleted %d row(s).", count)
		} else if cr, err := coord.ParseCellRef(res["range"]); err == nil {
			at = cr.Row
			a.sheet.DeleteRow(at, 1)
			a.statusMessage = "Deleted 1 row."
		} else {
			a.sheet.DeleteRow(at, 1)
			a.statusMessage = "Deleted 1 row."
		}
		a.adjustCutClipboardForRowDelete(at, count)

	case "doDeleteCol":
		a.pushUndoWorkbook()
		at, count := a.cursorCol, 1
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			at = r.MinCol()
			count = r.MaxCol() - r.MinCol() + 1
			a.sheet.DeleteCol(at, count)
			a.statusMessage = fmt.Sprintf("Deleted %d col(s).", count)
		} else if cr, err := coord.ParseCellRef(res["range"]); err == nil {
			at = cr.Col
			a.sheet.DeleteCol(at, 1)
			a.statusMessage = "Deleted 1 col."
		} else {
			a.sheet.DeleteCol(at, 1)
			a.statusMessage = "Deleted 1 col."
		}
		a.adjustCutClipboardForColDelete(at, count)

	case "doColumnSetWidth":
		if w, err := strconv.Atoi(res["width"]); err == nil {
			a.pushUndo()
			a.sheet.SetColWidth(a.cursorCol, w)
			a.statusMessage = fmt.Sprintf("Column %s width set to %d.", getColLetter(a.cursorCol), w)
		}

	case "doColumnResetWidth":
		a.pushUndo()
		a.sheet.ResetColWidth(a.cursorCol)
		a.statusMessage = fmt.Sprintf("Column %s width reset.", getColLetter(a.cursorCol))

	case "doRangeFormatFixed":
		dec, _ := strconv.Atoi(res["dec"])
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.FormatRange(r, cell.CellFormat{Type: cell.FmtFixed, Decimals: dec})
			a.statusMessage = fmt.Sprintf("Formatted %s to Fixed (%d)", r, dec)
		}

	case "doRangeFormatCurrency":
		dec, _ := strconv.Atoi(res["dec"])
		sym := strings.TrimSpace(res["symbol"])
		if sym == "" {
			sym = "$"
		}
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.FormatRange(r, cell.CellFormat{Type: cell.FmtCurrency, Decimals: dec, CurrencySymbol: sym})
			a.statusMessage = fmt.Sprintf("Formatted %s to Currency %s (%d)", r, sym, dec)
		}

	case "doRangeFormatPercent":
		dec, _ := strconv.Atoi(res["dec"])
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.FormatRange(r, cell.CellFormat{Type: cell.FmtPercent, Decimals: dec})
			a.statusMessage = fmt.Sprintf("Formatted %s to Percent (%d)", r, dec)
		}

	case "doRangeFormatComma":
		dec, _ := strconv.Atoi(res["dec"])
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.FormatRange(r, cell.CellFormat{Type: cell.FmtComma, Decimals: dec})
			a.statusMessage = fmt.Sprintf("Formatted %s to Comma (%d)", r, dec)
		}

	case "doRangeFormatDate":
		df, _ := strconv.Atoi(res["type"])
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.FormatRange(r, cell.CellFormat{Type: cell.FmtDate, DateFormat: df})
			a.statusMessage = fmt.Sprintf("Formatted %s to Date (%d)", r, df)
		}

	case "doRangeFormatScientific":
		dec, _ := strconv.Atoi(res["dec"])
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.FormatRange(r, cell.CellFormat{Type: cell.FmtScientific, Decimals: dec})
			a.statusMessage = fmt.Sprintf("Formatted %s to Scientific (%d)", r, dec)
		}

	case "doRangeFormatGeneral":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.FormatRange(r, cell.CellFormat{Type: cell.FmtGeneral})
			a.statusMessage = fmt.Sprintf("Formatted %s to General", r)
		}

	case "doRangeLabelLeft":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.LabelAlignRange(r, cell.AlignLeft)
			a.statusMessage = fmt.Sprintf("Left aligned %s", r)
		}

	case "doRangeLabelRight":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.LabelAlignRange(r, cell.AlignRight)
			a.statusMessage = fmt.Sprintf("Right aligned %s", r)
		}

	case "doRangeLabelCenter":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.LabelAlignRange(r, cell.AlignCenter)
			a.statusMessage = fmt.Sprintf("Center aligned %s", r)
		}

	case "doRangeNameCreate":
		name := strings.ToUpper(strings.TrimSpace(res["name"]))
		if r, err := coord.ParseRangeRef(res["range"]); err == nil && name != "" {
			a.pushUndo()
			a.sheet.SetNamedRange(name, r)
			a.statusMessage = fmt.Sprintf("Created name '%s' for %s", name, r)
		}

	case "doRangeNameDelete":
		name := strings.ToUpper(strings.TrimSpace(res["name"]))
		if _, ok := a.sheet.NamedRanges()[name]; ok {
			a.pushUndo()
			a.sheet.DeleteNamedRange(name)
			a.statusMessage = fmt.Sprintf("Deleted name '%s'", name)
		}

	case "doRangeNameList":
		RenderNamedRangesScreen(a.screen, a.sheet.NamedRanges(), a.styles)

	case "doOpenFunctionPicker":
		a.doOpenFunctionPicker()

	case "doFileOpenDialog", "doFileRetrieve":
		a.filePicker.Open(FilePickerModeOpen, a.filename)

	case "doFileSaveDialog", "doFileSave":
		a.filePicker.Open(FilePickerModeSave, a.filename)

	case "doFileImportCSVDialog", "doFileImportCSV":
		a.filePicker.Open(FilePickerModeImportCSV, "sheet.csv")

	case "doFileExportCSVFullDialog":
		a.pendingExportRange = nil
		a.filePicker.Open(FilePickerModeExportCSV, "sheet.csv")

	case "doFileExportXLSXFullDialog":
		a.pendingExportRange = nil
		a.filePicker.Open(FilePickerModeExportXLSX, "workbook.xlsx")

	case "doFileExportODSFullDialog":
		a.pendingExportRange = nil
		a.filePicker.Open(FilePickerModeExportODS, "workbook.ods")

	case "doFileExportCSVRangeDialog", "doFileExportCSVDialog", "doFileExportCSV":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Enter range to export (e.g. A1..D10): ", UseCursor: true}}, "doExportCSVRange")

	case "doExportCSVRange":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pendingExportRange = &r
		} else {
			minC, minR, maxC, maxR := a.sheet.BoundingBox()
			a.pendingExportRange = &coord.RangeRef{
				Start: coord.CellRef{Col: minC, Row: minR},
				End:   coord.CellRef{Col: maxC, Row: maxR},
			}
		}
		a.filePicker.Open(FilePickerModeExportCSV, "sheet.csv")

	case "doFileExportXLSXRangeDialog", "doFileExportXLSXDialog", "doFileExportXLSX":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Enter range to export (e.g. A1..D10): ", UseCursor: true}}, "doExportXLSXRange")

	case "doExportXLSXRange":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pendingExportRange = &r
		} else {
			minC, minR, maxC, maxR := a.sheet.BoundingBox()
			a.pendingExportRange = &coord.RangeRef{
				Start: coord.CellRef{Col: minC, Row: minR},
				End:   coord.CellRef{Col: maxC, Row: maxR},
			}
		}
		a.filePicker.Open(FilePickerModeExportXLSX, "sheet.xlsx")

	case "doFileExportODSRangeDialog", "doFileExportODSDialog", "doFileExportODS":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Enter range to export (e.g. A1..D10): ", UseCursor: true}}, "doExportODSRange")

	case "doExportODSRange":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pendingExportRange = &r
		} else {
			minC, minR, maxC, maxR := a.sheet.BoundingBox()
			a.pendingExportRange = &coord.RangeRef{
				Start: coord.CellRef{Col: minC, Row: minR},
				End:   coord.CellRef{Col: maxC, Row: maxR},
			}
		}
		a.filePicker.Open(FilePickerModeExportODS, "sheet.ods")

	case "doFileExportMarkdownRangeDialog", "doFileExportMarkdownDialog", "doFileExportMarkdown":
		a.startPromptChain([]PromptItem{{Key: "range", Prompt: "Enter range to export (e.g. A1..D10): ", UseCursor: true}}, "doExportMarkdownRange")

	case "doExportMarkdownRange":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pendingExportRange = &r
		} else {
			minC, minR, maxC, maxR := a.sheet.BoundingBox()
			a.pendingExportRange = &coord.RangeRef{
				Start: coord.CellRef{Col: minC, Row: minR},
				End:   coord.CellRef{Col: maxC, Row: maxR},
			}
		}
		a.filePicker.Open(FilePickerModeExportMarkdown, "table.md")

	case "doGraphTypeLine":
		a.pushUndo()
		a.sheet.Graph().Type = "LINE"
		a.statusMessage = "Graph type: Line"
	case "doGraphTypeBar":
		a.pushUndo()
		a.sheet.Graph().Type = "BAR"
		a.statusMessage = "Graph type: Bar"
	case "doGraphTypeStacked":
		a.pushUndo()
		a.sheet.Graph().Type = "STACKED"
		a.statusMessage = "Graph type: Stacked-Bar"
	case "doGraphTypePie":
		a.pushUndo()
		a.sheet.Graph().Type = "PIE"
		a.statusMessage = "Graph type: Pie"

	case "doGraphSetX":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.Graph().RangeX = &r
			a.statusMessage = fmt.Sprintf("Graph X-range: %s", r)
		}
	case "doGraphSetA":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.Graph().Series["A"] = &r
			a.statusMessage = fmt.Sprintf("Graph Series A: %s", r)
		}
	case "doGraphSetB":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.Graph().Series["B"] = &r
			a.statusMessage = fmt.Sprintf("Graph Series B: %s", r)
		}
	case "doGraphSetC":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.Graph().Series["C"] = &r
			a.statusMessage = fmt.Sprintf("Graph Series C: %s", r)
		}
	case "doGraphSetD":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.Graph().Series["D"] = &r
			a.statusMessage = fmt.Sprintf("Graph Series D: %s", r)
		}
	case "doGraphSetE":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.Graph().Series["E"] = &r
			a.statusMessage = fmt.Sprintf("Graph Series E: %s", r)
		}
	case "doGraphSetF":
		if r, err := coord.ParseRangeRef(res["range"]); err == nil {
			a.pushUndo()
			a.sheet.Graph().Series["F"] = &r
			a.statusMessage = fmt.Sprintf("Graph Series F: %s", r)
		}

	case "doGraphView":
		RenderGraphScreen(a.screen, a.sheet, a.styles, a.filename)

	case "doGraphSetTitle":
		a.pushUndo()
		a.sheet.Graph().Title = strings.TrimSpace(res["title"])
		if a.sheet.Graph().Title == "" {
			a.statusMessage = "Graph title cleared."
		} else {
			a.statusMessage = fmt.Sprintf("Graph title set: %s", a.sheet.Graph().Title)
		}

	case "doGraphStatus":
		RenderGraphStatusScreen(a.screen, a.sheet, a.styles)

	case "doGraphSavePNG":
		fn := strings.TrimSpace(res["filename"])
		if fn == "" || fn == "graph.png" {
			fn = DefaultPNGFilename(a.sheet, a.filename)
		}
		if !strings.HasSuffix(strings.ToLower(fn), ".png") {
			fn += ".png"
		}
		if err := ExportGraphPNG(a.sheet, fn, 1280, 720); err == nil {
			a.statusMessage = fmt.Sprintf("Exported graph to '%s' (1280x720 PNG)", filepath.Base(fn))
		} else {
			a.statusMessage = fmt.Sprintf("Error exporting PNG: %v", err)
		}

	case "doFindPrompt":
		q := strings.TrimSpace(res["query"])
		if q != "" {
			a.replaceScopeAll = false
			a.executeFindScoped(q, true, true, false)
		} else {
			a.statusMessage = "Find query is empty."
		}

	case "doFindAllPrompt":
		q := strings.TrimSpace(res["query"])
		if q != "" {
			a.replaceScopeAll = true
			a.executeFindScoped(q, true, true, true)
		} else {
			a.statusMessage = "Search query is empty."
		}

	case "doReplacePrompt":
		f := res["find"]
		r := res["replace"]
		scope := strings.ToUpper(strings.TrimSpace(res["scope"]))
		if f == "" {
			a.statusMessage = "Find query is empty."
			return
		}
		a.lastSearchQuery = f
		a.lastReplaceText = r
		a.replaceScopeAll = (scope == "A" || scope == "ALL")
		a.startReplaceInteractive(f, r, a.replaceScopeAll)

	case "doReplaceConfirm":
		act := strings.ToUpper(strings.TrimSpace(res["action"]))
		findStr := a.lastSearchQuery
		repStr := a.lastReplaceText
		allSheets := a.replaceScopeAll

		if act == "A" || act == "ALL" {
			qLower := strings.ToLower(findStr)
			wb := a.currentWorkbook()
			sheetsToScan := []*sheet.Sheet{a.sheet}
			if allSheets && wb != nil && len(wb.Sheets) > 1 {
				sheetsToScan = wb.Sheets
			}
			a.pushUndoSheets(sheetsToScan)

			count := 0
			for _, sh := range sheetsToScan {
				sh.SuspendRecalc()
			}
			for _, sh := range sheetsToScan {
				minC, minR, maxC, maxR := sh.BoundingBox()
				for r := minR; r <= maxR; r++ {
					for c := minC; c <= maxC; c++ {
						cellData := sh.GetCell(c, r)
						if cellData == nil || cellData.Type == cell.TypeEmpty {
							continue
						}
						valStr := ""
						if cellData.Value != nil {
							valStr = fmt.Sprintf("%v", cellData.Value)
						}
						fmtValStr := cellData.FormattedValue(sh.GlobalFormat())

						rawLower := strings.ToLower(cellData.RawInput)
						valLower := strings.ToLower(valStr)
						fmtLower := strings.ToLower(fmtValStr)

						if cellData.Type == cell.TypeFormula {
							if newRaw, ok := formula.ReplaceInFormula(cellData.RawInput, findStr, repStr); ok {
								sh.SetCellInput(c, r, newRaw, cellData.FormatSpec)
								count++
								continue
							}
							// Displayed value: exact match only so "1" does not hit 10 or A1.
							if valStr != "" && strings.EqualFold(valStr, findStr) {
								newRaw := replaceCaseInsensitive(valStr, findStr, repStr)
								sh.SetCellInput(c, r, newRaw, cellData.FormatSpec)
								count++
							} else if fmtValStr != "" && strings.EqualFold(fmtValStr, findStr) {
								newRaw := replaceCaseInsensitive(fmtValStr, findStr, repStr)
								sh.SetCellInput(c, r, newRaw, cellData.FormatSpec)
								count++
							}
							continue
						}
						if strings.Contains(rawLower, qLower) {
							newRaw := replaceCaseInsensitive(cellData.RawInput, findStr, repStr)
							sh.SetCellInput(c, r, newRaw, cellData.FormatSpec)
							count++
						} else if valStr != "" && strings.Contains(valLower, qLower) {
							newRaw := replaceCaseInsensitive(valStr, findStr, repStr)
							sh.SetCellInput(c, r, newRaw, cellData.FormatSpec)
							count++
						} else if fmtValStr != "" && strings.Contains(fmtLower, qLower) {
							newRaw := replaceCaseInsensitive(fmtValStr, findStr, repStr)
							sh.SetCellInput(c, r, newRaw, cellData.FormatSpec)
							count++
						}
					}
				}
			}
			for _, sh := range sheetsToScan {
				sh.ClearSuspendRecalc()
			}
			a.recalculateAll()
			a.statusMessage = fmt.Sprintf("Replaced %d occurrences.", count)
			return
		}

		if act == "Y" || act == "YES" || act == "" {
			a.pushUndo()
			currCell := a.sheet.GetCell(a.cursorCol, a.cursorRow)
			if currCell != nil {
				valStr := ""
				if currCell.Value != nil {
					valStr = fmt.Sprintf("%v", currCell.Value)
				}
				fmtValStr := currCell.FormattedValue(a.sheet.GlobalFormat())
				rawLower := strings.ToLower(currCell.RawInput)
				valLower := strings.ToLower(valStr)
				fmtLower := strings.ToLower(fmtValStr)
				qLower := strings.ToLower(findStr)

				if currCell.Type == cell.TypeFormula {
					if newRaw, ok := formula.ReplaceInFormula(currCell.RawInput, findStr, repStr); ok {
						a.sheet.SetCellInput(a.cursorCol, a.cursorRow, newRaw, currCell.FormatSpec)
						a.recalculateAll()
					} else if valStr != "" && strings.EqualFold(valStr, findStr) {
						newRaw := replaceCaseInsensitive(valStr, findStr, repStr)
						a.sheet.SetCellInput(a.cursorCol, a.cursorRow, newRaw, currCell.FormatSpec)
						a.recalculateAll()
					} else if fmtValStr != "" && strings.EqualFold(fmtValStr, findStr) {
						newRaw := replaceCaseInsensitive(fmtValStr, findStr, repStr)
						a.sheet.SetCellInput(a.cursorCol, a.cursorRow, newRaw, currCell.FormatSpec)
						a.recalculateAll()
					}
				} else if strings.Contains(rawLower, qLower) {
					newRaw := replaceCaseInsensitive(currCell.RawInput, findStr, repStr)
					a.sheet.SetCellInput(a.cursorCol, a.cursorRow, newRaw, currCell.FormatSpec)
					a.recalculateAll()
				} else if valStr != "" && strings.Contains(valLower, qLower) {
					newRaw := replaceCaseInsensitive(valStr, findStr, repStr)
					a.sheet.SetCellInput(a.cursorCol, a.cursorRow, newRaw, currCell.FormatSpec)
					a.recalculateAll()
				} else if fmtValStr != "" && strings.Contains(fmtLower, qLower) {
					newRaw := replaceCaseInsensitive(fmtValStr, findStr, repStr)
					a.sheet.SetCellInput(a.cursorCol, a.cursorRow, newRaw, currCell.FormatSpec)
					a.recalculateAll()
				}
			}
			if !a.executeFindScoped(findStr, true, false, allSheets) {
				return
			}
			promptMsg := fmt.Sprintf("Replace \"%s\" with \"%s\" at %s%d? [Y: Yes / N: Skip / A: All / Esc: Cancel]: ", findStr, repStr, getColLetter(a.cursorCol), a.cursorRow+1)
			a.startPromptChain([]PromptItem{{Key: "action", Prompt: promptMsg, Default: "Y"}}, "doReplaceConfirm")
			return
		}

		if act == "N" || act == "NO" || act == "S" || act == "SKIP" {
			if !a.executeFindScoped(findStr, true, false, allSheets) {
				return
			}
			promptMsg := fmt.Sprintf("Replace \"%s\" with \"%s\" at %s%d? [Y: Yes / N: Skip / A: All / Esc: Cancel]: ", findStr, repStr, getColLetter(a.cursorCol), a.cursorRow+1)
			a.startPromptChain([]PromptItem{{Key: "action", Prompt: promptMsg, Default: "Y"}}, "doReplaceConfirm")
			return
		}

	case "doFreezeBoth":
		a.sheet.SetFrozenRows(a.cursorRow)
		a.sheet.SetFrozenCols(a.cursorCol)
		a.statusMessage = fmt.Sprintf("Frozen rows 1..%d and columns A..%s.", a.cursorRow, getColLetter(a.cursorCol-1))
	case "doFreezeHorizontal":
		a.sheet.SetFrozenRows(a.cursorRow)
		a.sheet.SetFrozenCols(0)
		a.statusMessage = fmt.Sprintf("Frozen rows 1..%d at top.", a.cursorRow)
	case "doFreezeVertical":
		a.sheet.SetFrozenRows(0)
		a.sheet.SetFrozenCols(a.cursorCol)
		a.statusMessage = fmt.Sprintf("Frozen columns A..%s at left.", getColLetter(a.cursorCol-1))
	case "doFreezeClear":
		a.sheet.SetFrozenRows(0)
		a.sheet.SetFrozenCols(0)
		a.statusMessage = "Cleared window titles (freeze panes)."

	case "doDataFill":
		targetStr := res["range"]
		startStr := res["start"]
		stepStr := res["step"]
		stopStr := res["stop"]
		if rr, err := coord.ParseRangeRef(strings.ToUpper(targetStr)); err == nil {
			a.pushUndo()
			if err := a.sheet.DataFill(rr, startStr, stepStr, stopStr); err == nil {
				a.recalculateAll()
				a.statusMessage = fmt.Sprintf("Filled sequence across %s.", rr)
			} else {
				a.statusMessage = fmt.Sprintf("Error in Data Fill: %v", err)
			}
		} else {
			a.statusMessage = fmt.Sprintf("Invalid range: %s", targetStr)
		}

	case "doAutoFill":
		a.doAutoFill()

	case "doPasteTranspose":
		a.doPasteTranspose()

	case "doRangeTranspose":
		srcStr := res["source"]
		dstStr := res["target"]
		sR, errS := coord.ParseRangeRef(strings.ToUpper(srcStr))
		dC, errD := coord.ParseCellRef(strings.ToUpper(dstStr))
		if errS == nil && errD == nil {
			a.pushUndo()
			if err := a.sheet.TransposeRange(sR, dC); err == nil {
				a.recalculateAll()
				a.statusMessage = fmt.Sprintf("Transposed %s to %s.", sR, dC)
			} else {
				a.statusMessage = fmt.Sprintf("Error transposing: %v", err)
			}
		} else {
			a.statusMessage = fmt.Sprintf("Invalid source or target coordinates.")
		}

	case "doFillDown":
		a.doFillDown()

	case "doFillRight":
		a.doFillRight()

	case "doFindNext":
		a.findNext()

	case "doFindPrev":
		a.findPrev()

	case "doGotoCell":
		s := strings.TrimSpace(res["cell"])
		if s == "" {
			return
		}

		// Handle cross-sheet reference e.g. Sheet2!B10 or 'My Sheet'!A1
		if idx := strings.LastIndex(s, "!"); idx != -1 {
			targetSheetName := coord.UnquoteSheetName(s[:idx])
			targetCoord := strings.TrimSpace(s[idx+1:])
			wb := a.currentWorkbook()
			for i, sh := range wb.Sheets {
				if strings.EqualFold(sh.Name(), targetSheetName) {
					a.switchSheet(i)
					a.statusMessage = fmt.Sprintf("Switched to sheet '%s'.", sh.Name())
					break
				}
			}
			s = targetCoord
		}

		sUpper := strings.ToUpper(s)
		if cr, err := coord.ParseCellRef(sUpper); err == nil {
			a.cursorCol = cr.Col
			a.cursorRow = cr.Row
			a.clearSelection()
			a.adjustViewport()
			a.statusMessage = fmt.Sprintf("Jumped to %s", cr)
		} else if rr, err := coord.ParseRangeRef(sUpper); err == nil {
			a.cursorCol = rr.MinCol()
			a.cursorRow = rr.MinRow()
			a.selectedRange = &rr
			a.adjustViewport()
			a.statusMessage = fmt.Sprintf("Jumped to range %s", rr)
		} else {
			if named, ok := a.sheet.GetNamedRange(s); ok {
				switch nr := named.(type) {
				case coord.CellRef:
					if nr.Sheet != "" {
						if wb := a.currentWorkbook(); wb != nil {
							if idx := wb.GetSheetIndex(wb.GetSheet(nr.Sheet)); idx >= 0 {
								a.switchSheet(idx)
							}
						}
					}
					a.cursorCol = nr.Col
					a.cursorRow = nr.Row
					a.clearSelection()
					a.adjustViewport()
					a.statusMessage = fmt.Sprintf("Jumped to named range '%s'", s)
					return
				case coord.RangeRef:
					if nr.Sheet != "" {
						if wb := a.currentWorkbook(); wb != nil {
							if idx := wb.GetSheetIndex(wb.GetSheet(nr.Sheet)); idx >= 0 {
								a.switchSheet(idx)
							}
						}
					}
					a.cursorCol = nr.MinCol()
					a.cursorRow = nr.MinRow()
					a.selectedRange = &nr
					a.adjustViewport()
					a.statusMessage = fmt.Sprintf("Jumped to named range '%s'", s)
					return
				}
			}
			foundName := false
			for name, val := range a.sheet.NamedRanges() {
				if strings.EqualFold(name, s) {
					switch nr := val.(type) {
					case coord.CellRef:
						a.cursorCol = nr.Col
						a.cursorRow = nr.Row
						a.clearSelection()
					case coord.RangeRef:
						a.cursorCol = nr.MinCol()
						a.cursorRow = nr.MinRow()
						a.selectedRange = &nr
					}
					a.adjustViewport()
					a.statusMessage = fmt.Sprintf("Jumped to named range '%s'", name)
					foundName = true
					break
				}
			}
			if !foundName {
				a.statusMessage = fmt.Sprintf("Invalid cell, range, or name: '%s'", s)
			}
		}
	}
}

func (a *App) startReplacePrompt() {
	a.startPromptChain([]PromptItem{
		{Key: "find", Prompt: "Find text/number/formula: ", Default: a.lastSearchQuery},
		{Key: "replace", Prompt: "Replace with: ", Default: a.lastReplaceText},
		{Key: "scope", Prompt: "Scope [C: Current Sheet / A: All Sheets]: ", Default: "C"},
	}, "doReplacePrompt")
}

func (a *App) startReplaceInteractive(findStr, repStr string, allSheets bool) {
	a.lastSearchQuery = findStr
	a.lastReplaceText = repStr
	a.replaceScopeAll = allSheets

	qLower := strings.ToLower(findStr)
	wb := a.currentWorkbook()
	sheetsToScan := []*sheet.Sheet{a.sheet}
	if allSheets && wb != nil && len(wb.Sheets) > 1 {
		sheetsToScan = wb.Sheets
	}

	matchCount := 0
	for _, sh := range sheetsToScan {
		minC, minR, maxC, maxR := sh.BoundingBox()
		for r := minR; r <= maxR; r++ {
			for c := minC; c <= maxC; c++ {
				cellData := sh.GetCell(c, r)
				if cellMatchesQuery(cellData, sh, qLower) {
					matchCount++
				}
			}
		}
	}

	if matchCount == 0 {
		a.statusMessage = fmt.Sprintf("Pattern not found: \"%s\"", findStr)
		return
	}

	a.executeFindScoped(findStr, true, true, allSheets)

	promptMsg := fmt.Sprintf("Replace \"%s\" with \"%s\" at %s%d? [Y: Yes / N: Skip / A: All / Esc: Cancel]: ", findStr, repStr, getColLetter(a.cursorCol), a.cursorRow+1)
	a.startPromptChain([]PromptItem{{Key: "action", Prompt: promptMsg, Default: "Y"}}, "doReplaceConfirm")
}

func replaceCaseInsensitive(src, find, rep string) string {
	if find == "" {
		return src
	}
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(find))
	if err != nil {
		return src
	}
	return re.ReplaceAllLiteralString(src, rep)
}

func (a *App) doFillDown() {
	a.pushUndo()
	if a.selectedRange != nil && a.selectedRange.MaxRow() > a.selectedRange.MinRow() {
		rng := *a.selectedRange
		topR := rng.MinRow()
		maxR := rng.MaxRow()
		if maxR >= 100000 {
			maxR = a.sheet.MaxPopulatedRow()
			if maxR <= topR {
				maxR = topR + 1
			}
		}
		for r := topR + 1; r <= maxR; r++ {
			for c := rng.MinCol(); c <= rng.MaxCol(); c++ {
				srcCell := a.sheet.GetCell(c, topR)
				if srcCell == nil || srcCell.Type == cell.TypeEmpty {
					a.sheet.ClearCell(c, r)
					continue
				}
				dRow := r - topR
				if srcCell.Type == cell.TypeFormula {
					adj := formula.AdjustFormulaReferences(srcCell.RawInput, 0, dRow)
					a.sheet.SetCellInput(c, r, adj, srcCell.FormatSpec)
				} else {
					a.sheet.SetCellInput(c, r, srcCell.RawInput, srcCell.FormatSpec)
				}
			}
		}
		a.recalculateAll()
		a.statusMessage = fmt.Sprintf("Filled down across %s.", rng)
	} else if a.cursorRow > 0 {
		srcCell := a.sheet.GetCell(a.cursorCol, a.cursorRow-1)
		if srcCell != nil {
			if srcCell.Type == cell.TypeFormula {
				adj := formula.AdjustFormulaReferences(srcCell.RawInput, 0, 1)
				a.sheet.SetCellInput(a.cursorCol, a.cursorRow, adj, srcCell.FormatSpec)
			} else {
				a.sheet.SetCellInput(a.cursorCol, a.cursorRow, srcCell.RawInput, srcCell.FormatSpec)
			}
			a.recalculateAll()
			a.statusMessage = fmt.Sprintf("Filled down to %s%d.", getColLetter(a.cursorCol), a.cursorRow+1)
		}
	}
}

func cellHasRaw(c *cell.Cell) bool {
	return c != nil && c.RawInput != ""
}

func (a *App) fillRightIntoEmpty(minC, maxC, row int) {
	a.fillRowFromSeed(row, minC, maxC, minC, true)
}

func (a *App) autoFillColumn(col, minR, maxR int) {
	top := a.sheet.GetCell(col, minR)
	bot := a.sheet.GetCell(col, maxR)
	fromBottom := !cellHasRaw(top) && cellHasRaw(bot)
	seedR := minR
	if fromBottom {
		seedR = maxR
	}
	seed := a.sheet.GetCell(col, seedR)
	if !cellHasRaw(seed) {
		return
	}
	if seed.Type == cell.TypeFormula {
		for r := minR; r <= maxR; r++ {
			if r == seedR {
				continue
			}
			adj := formula.AdjustFormulaReferences(seed.RawInput, 0, r-seedR)
			a.sheet.SetCellInput(col, r, adj, seed.FormatSpec)
		}
		return
	}
	otherR := seedR + 1
	if fromBottom {
		otherR = seedR - 1
	}
	var other *cell.Cell
	if otherR >= minR && otherR <= maxR {
		other = a.sheet.GetCell(col, otherR)
	}
	a.fillIndexSeries(func(i int) *cell.Cell {
		return a.sheet.GetCell(col, i)
	}, func(i int, raw string, fmtSpec *cell.CellFormat) {
		a.sheet.SetCellInput(col, i, raw, fmtSpec)
	}, minR, maxR, seedR, seed, other)
}

func (a *App) autoFillRow(row, minC, maxC int) {
	left := a.sheet.GetCell(minC, row)
	right := a.sheet.GetCell(maxC, row)
	fromRight := !cellHasRaw(left) && cellHasRaw(right)
	seedC := minC
	if fromRight {
		seedC = maxC
	}
	seed := a.sheet.GetCell(seedC, row)
	if !cellHasRaw(seed) {
		return
	}
	hasOtherFormula := false
	for c := minC; c <= maxC; c++ {
		if c == seedC {
			continue
		}
		if cl := a.sheet.GetCell(c, row); cl != nil && cl.Type == cell.TypeFormula {
			hasOtherFormula = true
			break
		}
	}
	if seed.Type == cell.TypeFormula || hasOtherFormula {
		a.fillRowFromSeed(row, minC, maxC, seedC, true)
		return
	}
	otherC := seedC + 1
	if fromRight {
		otherC = seedC - 1
	}
	var other *cell.Cell
	if otherC >= minC && otherC <= maxC {
		other = a.sheet.GetCell(otherC, row)
	}
	a.fillIndexSeries(func(i int) *cell.Cell {
		return a.sheet.GetCell(i, row)
	}, func(i int, raw string, fmtSpec *cell.CellFormat) {
		a.sheet.SetCellInput(i, row, raw, fmtSpec)
	}, minC, maxC, seedC, seed, other)
}

func (a *App) fillRowFromSeed(row, minC, maxC, seedC int, emptyOnly bool) {
	seed := a.sheet.GetCell(seedC, row)
	if !cellHasRaw(seed) {
		return
	}
	for c := minC; c <= maxC; c++ {
		if c == seedC {
			continue
		}
		if emptyOnly && cellHasRaw(a.sheet.GetCell(c, row)) {
			continue
		}
		dCol := c - seedC
		if seed.Type == cell.TypeFormula {
			adj := formula.AdjustFormulaReferences(seed.RawInput, dCol, 0)
			a.sheet.SetCellInput(c, row, adj, seed.FormatSpec)
		} else {
			a.sheet.SetCellInput(c, row, seed.RawInput, seed.FormatSpec)
		}
	}
}

func (a *App) fillIndexSeries(
	get func(int) *cell.Cell,
	set func(int, string, *cell.CellFormat),
	lo, hi, seedIdx int,
	seed, other *cell.Cell,
) {
	if seed == nil || seed.Value == nil {
		raw := ""
		var fmtSpec *cell.CellFormat
		if seed != nil {
			raw = seed.RawInput
			fmtSpec = seed.FormatSpec
		}
		if raw == "" {
			return
		}
		for i := lo; i <= hi; i++ {
			if i == seedIdx {
				continue
			}
			set(i, raw, fmtSpec)
		}
		return
	}
	startStr := fmt.Sprintf("%v", seed.Value)
	if tSeed, outFmt, ok := sheet.ParseFlexibleDate(startStr); ok {
		stepDays := 1
		if other != nil && other.Value != nil {
			if tOther, _, ok2 := sheet.ParseFlexibleDate(fmt.Sprintf("%v", other.Value)); ok2 {
				den := seedIdx - (seedIdx - 1)
				otherIdx := seedIdx + 1
				if otherIdx > hi || (seedIdx > lo && get(seedIdx-1) == other) {
					otherIdx = seedIdx - 1
				}
				if other != get(otherIdx) {
					if seedIdx > lo && cellHasRaw(get(seedIdx-1)) {
						otherIdx = seedIdx - 1
					} else if seedIdx < hi {
						otherIdx = seedIdx + 1
					}
				}
				_ = den
				if otherIdx != seedIdx {
					diff := int(tSeed.Sub(tOther).Hours() / 24)
					stepDays = diff / (seedIdx - otherIdx)
					if stepDays == 0 {
						stepDays = 1
					}
				}
			}
		}
		for i := lo; i <= hi; i++ {
			cur := tSeed.AddDate(0, 0, (i-seedIdx)*stepDays)
			set(i, "'"+cur.Format(outFmt), seed.FormatSpec)
		}
		return
	}
	startVal, err := strconv.ParseFloat(startStr, 64)
	if err != nil {
		for i := lo; i <= hi; i++ {
			if i == seedIdx {
				continue
			}
			set(i, seed.RawInput, seed.FormatSpec)
		}
		return
	}
	step := 1.0
	if other != nil && other.Value != nil {
		if ov, err2 := strconv.ParseFloat(fmt.Sprintf("%v", other.Value), 64); err2 == nil {
			otherIdx := seedIdx + 1
			if otherIdx > hi || !cellHasRaw(get(otherIdx)) {
				if seedIdx > lo && cellHasRaw(get(seedIdx-1)) {
					otherIdx = seedIdx - 1
				}
			}
			if otherIdx != seedIdx && cellHasRaw(get(otherIdx)) {
				step = (startVal - ov) / float64(seedIdx-otherIdx)
			}
		}
	}
	for i := lo; i <= hi; i++ {
		set(i, fmt.Sprintf("%g", startVal+float64(i-seedIdx)*step), seed.FormatSpec)
	}
}

func (a *App) doFillRight() {
	a.pushUndo()
	if a.selectedRange != nil && a.selectedRange.MaxCol() > a.selectedRange.MinCol() {
		rng := *a.selectedRange
		leftC := rng.MinCol()
		maxC := rng.MaxCol()
		if maxC >= 10000 {
			maxC = a.sheet.MaxPopulatedCol()
			if maxC <= leftC {
				maxC = leftC + 1
			}
		}
		for c := leftC + 1; c <= maxC; c++ {
			for r := rng.MinRow(); r <= rng.MaxRow(); r++ {
				srcCell := a.sheet.GetCell(leftC, r)
				if srcCell == nil || srcCell.Type == cell.TypeEmpty {
					a.sheet.ClearCell(c, r)
					continue
				}
				dCol := c - leftC
				if srcCell.Type == cell.TypeFormula {
					adj := formula.AdjustFormulaReferences(srcCell.RawInput, dCol, 0)
					a.sheet.SetCellInput(c, r, adj, srcCell.FormatSpec)
				} else {
					a.sheet.SetCellInput(c, r, srcCell.RawInput, srcCell.FormatSpec)
				}
			}
		}
		a.recalculateAll()
		a.statusMessage = fmt.Sprintf("Filled right across %s.", rng)
	} else if a.cursorCol > 0 {
		srcCell := a.sheet.GetCell(a.cursorCol-1, a.cursorRow)
		if srcCell != nil {
			if srcCell.Type == cell.TypeFormula {
				adj := formula.AdjustFormulaReferences(srcCell.RawInput, 1, 0)
				a.sheet.SetCellInput(a.cursorCol, a.cursorRow, adj, srcCell.FormatSpec)
			} else {
				a.sheet.SetCellInput(a.cursorCol, a.cursorRow, srcCell.RawInput, srcCell.FormatSpec)
			}
			a.recalculateAll()
			a.statusMessage = fmt.Sprintf("Filled right to %s%d.", getColLetter(a.cursorCol), a.cursorRow+1)
		}
	}
}

func (a *App) recalculateAll() {
	a.sheet.SetModified(true)
	wb := a.currentWorkbook()
	if wb != nil {
		wb.RecalculateAll()
	} else {
		a.sheet.Recalculate()
	}
}

func (a *App) findNext() {
	if a.lastSearchQuery == "" {
		a.startPromptChain([]PromptItem{{Key: "query", Prompt: "Find text/number/formula: ", Default: a.lastSearchQuery}}, "doFindPrompt")
		return
	}
	a.executeFindScoped(a.lastSearchQuery, true, false, a.replaceScopeAll)
}

func (a *App) findPrev() {
	if a.lastSearchQuery == "" {
		a.startPromptChain([]PromptItem{{Key: "query", Prompt: "Find text/number/formula: ", Default: a.lastSearchQuery}}, "doFindPrompt")
		return
	}
	a.executeFindScoped(a.lastSearchQuery, false, false, a.replaceScopeAll)
}

func cellMatchesQuery(cellData *cell.Cell, sh *sheet.Sheet, qLower string) bool {
	if cellData == nil || cellData.Type == cell.TypeEmpty || qLower == "" {
		return false
	}
	if strings.Contains(strings.ToLower(cellData.RawInput), qLower) {
		return true
	}
	if cellData.Value != nil && strings.Contains(strings.ToLower(fmt.Sprintf("%v", cellData.Value)), qLower) {
		return true
	}
	if sh != nil && strings.Contains(strings.ToLower(cellData.FormattedValue(sh.GlobalFormat())), qLower) {
		return true
	}
	return false
}

func (a *App) executeFindScoped(query string, forward bool, includeCurrent bool, allSheets bool) bool {
	a.lastSearchQuery = query
	qLower := strings.ToLower(query)

	type matchPos struct {
		sheet *sheet.Sheet
		col   int
		row   int
	}
	var matches []matchPos

	wb := a.currentWorkbook()
	sheetsToScan := []*sheet.Sheet{a.sheet}
	if allSheets && wb != nil && len(wb.Sheets) > 1 {
		sheetsToScan = wb.Sheets
	}

	for _, sh := range sheetsToScan {
		minC, minR, maxC, maxR := sh.BoundingBox()
		for r := minR; r <= maxR; r++ {
			for c := minC; c <= maxC; c++ {
				cellData := sh.GetCell(c, r)
				if cellData == nil || cellData.Type == cell.TypeEmpty {
					continue
				}
				matched := cellMatchesQuery(cellData, sh, qLower)
				if matched {
					matches = append(matches, matchPos{sheet: sh, col: c, row: r})
				}
			}
		}
	}

	if len(matches) == 0 {
		a.statusMessage = fmt.Sprintf("Pattern not found: \"%s\"", query)
		return false
	}

	curRow, curCol := a.cursorRow, a.cursorCol
	curSheet := a.sheet
	selectedIdx := -1

	if forward {
		for i, m := range matches {
			if m.sheet != curSheet {
				if allSheets && wb != nil {
					curIdx := wb.GetSheetIndex(curSheet)
					mIdx := wb.GetSheetIndex(m.sheet)
					if mIdx > curIdx {
						selectedIdx = i
						break
					}
				}
			} else {
				if includeCurrent {
					if m.row > curRow || (m.row == curRow && m.col >= curCol) {
						selectedIdx = i
						break
					}
				} else {
					if m.row > curRow || (m.row == curRow && m.col > curCol) {
						selectedIdx = i
						break
					}
				}
			}
		}
		if selectedIdx == -1 {
			selectedIdx = 0
		}
	} else {
		for i := len(matches) - 1; i >= 0; i-- {
			m := matches[i]
			if m.sheet != curSheet {
				if allSheets && wb != nil {
					curIdx := wb.GetSheetIndex(curSheet)
					mIdx := wb.GetSheetIndex(m.sheet)
					if mIdx < curIdx {
						selectedIdx = i
						break
					}
				}
			} else {
				if includeCurrent {
					if m.row < curRow || (m.row == curRow && m.col <= curCol) {
						selectedIdx = i
						break
					}
				} else {
					if m.row < curRow || (m.row == curRow && m.col < curCol) {
						selectedIdx = i
						break
					}
				}
			}
		}
		if selectedIdx == -1 {
			selectedIdx = len(matches) - 1
		}
	}

	target := matches[selectedIdx]
	if target.sheet != a.sheet {
		if wb != nil {
			if idx := wb.GetSheetIndex(target.sheet); idx >= 0 {
				a.switchSheet(idx)
			} else {
				a.sheet = target.sheet
			}
		} else {
			a.sheet = target.sheet
		}
	}
	a.cursorCol = target.col
	a.cursorRow = target.row
	a.clearSelection()
	a.adjustViewport()
	if allSheets && wb != nil && len(wb.Sheets) > 1 {
		a.statusMessage = fmt.Sprintf("Found \"%s\" in '%s'!%s%d (match %d of %d in book). Press F3 for next.", query, a.sheet.Name(), getColLetter(target.col), target.row+1, selectedIdx+1, len(matches))
	} else {
		a.statusMessage = fmt.Sprintf("Found \"%s\" at %s%d (match %d of %d). Press F3 for next.", query, getColLetter(target.col), target.row+1, selectedIdx+1, len(matches))
	}
	return true
}

func (a *App) loadFile(fn string) {
	ext := strings.ToLower(filepath.Ext(fn))
	if ext == ".csv" || ext == ".tsv" {
		a.importCSVFile(fn)
		return
	}
	if ext == ".md" || ext == ".markdown" || ext == ".html" || ext == ".htm" {
		a.importMarkupFile(fn)
		return
	}
	if ext == ".xlsx" || ext == ".xlsm" {
		a.importXLSXFile(fn)
		return
	}
	if ext == ".ods" || ext == ".ots" {
		a.importODSFile(fn)
		return
	}
	RenderLoadingModal(a.screen, fn, "Opening worksheet...", a.styles)
	if wb, err := sheet.LoadWorkbookJSON(fn); err == nil {
		a.pushUndoWorkbook()
		a.workbook = wb
		a.sheet = wb.GetActiveSheet()
		a.filename = AbsolutePath(fn)
		a.resetViewportAndViews()
		a.statusMessage = fmt.Sprintf("Opened '%s' (%d sheets).", filepath.Base(fn), len(wb.Sheets))
	} else {
		a.statusMessage = fmt.Sprintf("Error opening file: %v", err)
	}
}

func (a *App) importXLSXFile(fn string) {
	onProgress := func(step string) {
		RenderLoadingModal(a.screen, fn, step, a.styles)
	}

	wb, err := sheet.ImportXLSXWorkbookWithProgress(fn, onProgress)
	if err != nil {
		// Fallback: Check if file is HasuCalc JSON saved with .xlsx extension
		if jsonWb, jsonErr := sheet.LoadWorkbookJSON(fn); jsonErr == nil && len(jsonWb.Sheets) > 0 {
			wb = jsonWb
		} else {
			a.statusMessage = fmt.Sprintf("Error importing xlsx: %v", err)
			return
		}
	}

	sheetIdx := 0
	if len(wb.Sheets) > 1 {
		selected := ShowSheetPickerModal(a.screen, fn, wb.SheetNames(), a.styles)
		if selected < 0 {
			a.statusMessage = "Import canceled."
			return
		}
		sheetIdx = selected
	}

	wb.ActiveSheetIndex = sheetIdx
	a.pushUndoWorkbook()
	a.workbook = wb
	a.sheet = wb.Sheets[sheetIdx]
	a.sheet.SetWorkbook(wb)

	sheetTitle := filepath.Base(fn)
	if len(wb.Sheets) > 1 {
		sheetTitle = fmt.Sprintf("%s [%s]", filepath.Base(fn), a.sheet.Name())
	}
	a.filename = SuggestedHwkBeside(fn)
	a.resetViewportAndViews()
	a.statusMessage = fmt.Sprintf("Imported '%s' (%d sheets).", sheetTitle, len(wb.Sheets))
}

func (a *App) importODSFile(fn string) {
	onProgress := func(step string) {
		RenderLoadingModal(a.screen, fn, step, a.styles)
	}

	wb, err := sheet.ImportODSWorkbookWithProgress(fn, onProgress)
	if err != nil {
		// Fallback: Check if file is HasuCalc JSON saved with .ods extension
		if jsonWb, jsonErr := sheet.LoadWorkbookJSON(fn); jsonErr == nil && len(jsonWb.Sheets) > 0 {
			wb = jsonWb
		} else {
			a.statusMessage = fmt.Sprintf("Error importing ods: %v", err)
			return
		}
	}

	sheetIdx := 0
	if len(wb.Sheets) > 1 {
		selected := ShowSheetPickerModal(a.screen, fn, wb.SheetNames(), a.styles)
		if selected < 0 {
			a.statusMessage = "Import canceled."
			return
		}
		sheetIdx = selected
	}

	wb.ActiveSheetIndex = sheetIdx
	a.pushUndoWorkbook()
	a.workbook = wb
	a.sheet = wb.Sheets[sheetIdx]
	a.sheet.SetWorkbook(wb)

	sheetTitle := filepath.Base(fn)
	if len(wb.Sheets) > 1 {
		sheetTitle = fmt.Sprintf("%s [%s]", filepath.Base(fn), a.sheet.Name())
	}
	a.filename = SuggestedHwkBeside(fn)
	a.resetViewportAndViews()
	a.statusMessage = fmt.Sprintf("Imported '%s' (%d sheets).", sheetTitle, len(wb.Sheets))
}

func (a *App) saveFile(fn string) {
	fn = NormalizeHwkSaveFilename(fn)
	wb := a.currentWorkbook()
	if err := wb.SaveJSON(fn); err == nil {
		a.filename = AbsolutePath(fn)
		a.statusMessage = fmt.Sprintf("Saved to '%s' (%d sheets).", filepath.Base(fn), len(wb.Sheets))
	} else {
		a.statusMessage = fmt.Sprintf("Error saving: %v", err)
	}
}

func (a *App) importCSVFile(fn string) {
	RenderLoadingModal(a.screen, fn, "Parsing CSV rows & columns...", a.styles)
	if s, err := sheet.ImportSheetCSV(fn); err == nil {
		a.pushUndoWorkbook()
		a.filename = SuggestedHwkBeside(fn)
		// Replace the workbook so SaveJSON writes the imported sheet (not the old one).
		wb := sheet.NewWorkbook(a.filename)
		s.SetName(wb.Sheets[0].Name())
		s.SetWorkbook(wb)
		wb.Sheets[0] = s
		s.SetModified(false)
		a.workbook = wb
		a.sheet = s
		a.resetViewportAndViews()
		a.statusMessage = fmt.Sprintf("Imported CSV from '%s'.", filepath.Base(fn))
	} else {
		a.statusMessage = fmt.Sprintf("Error importing CSV: %v", err)
	}
}

func (a *App) importMarkupFile(fn string) {
	RenderLoadingModal(a.screen, fn, "Parsing tables & text...", a.styles)
	if wb, err := sheet.ImportMarkupWorkbook(fn); err == nil {
		a.pushUndoWorkbook()
		a.filename = SuggestedHwkBeside(fn)
		wb.Name = filepath.Base(a.filename)
		a.workbook = wb
		a.sheet = wb.GetActiveSheet()
		a.resetViewportAndViews()
		a.statusMessage = fmt.Sprintf("Imported markup from '%s' (%d sheet(s)).", filepath.Base(fn), len(wb.Sheets))
	} else {
		a.statusMessage = fmt.Sprintf("Error importing markup: %v", err)
	}
}

func (a *App) exportCSVFile(fn string) {
	var err error
	if a.pendingExportRange != nil {
		err = a.sheet.ExportCSVRange(fn, a.pendingExportRange.MinCol(), a.pendingExportRange.MinRow(), a.pendingExportRange.MaxCol(), a.pendingExportRange.MaxRow())
	} else {
		err = a.sheet.ExportCSV(fn)
	}
	if err == nil {
		a.statusMessage = fmt.Sprintf("Exported CSV to '%s'.", filepath.Base(fn))
	} else {
		a.statusMessage = fmt.Sprintf("Error exporting CSV: %v", err)
	}
	a.pendingExportRange = nil
}

func (a *App) exportXLSXFile(fn string) {
	var err error
	if a.pendingExportRange != nil {
		err = a.sheet.ExportXLSXRange(fn, a.pendingExportRange.MinCol(), a.pendingExportRange.MinRow(), a.pendingExportRange.MaxCol(), a.pendingExportRange.MaxRow())
	} else {
		wb := a.currentWorkbook()
		err = wb.ExportXLSX(fn)
	}
	if err == nil {
		a.statusMessage = fmt.Sprintf("Exported Excel (.xlsx) to '%s'.", filepath.Base(fn))
	} else {
		a.statusMessage = fmt.Sprintf("Error exporting Excel: %v", err)
	}
	a.pendingExportRange = nil
}

func (a *App) exportODSFile(fn string) {
	var err error
	if a.pendingExportRange != nil {
		err = a.sheet.ExportODSRange(fn, a.pendingExportRange.MinCol(), a.pendingExportRange.MinRow(), a.pendingExportRange.MaxCol(), a.pendingExportRange.MaxRow())
	} else {
		wb := a.currentWorkbook()
		err = wb.ExportODS(fn)
	}
	if err == nil {
		a.statusMessage = fmt.Sprintf("Exported OpenDocument (.ods) to '%s'.", filepath.Base(fn))
	} else {
		a.statusMessage = fmt.Sprintf("Error exporting OpenDocument: %v", err)
	}
	a.pendingExportRange = nil
}

func (a *App) exportMarkdownFile(fn string) {
	var err error
	if a.pendingExportRange != nil {
		err = a.sheet.ExportMarkdownRange(fn, a.pendingExportRange.MinCol(), a.pendingExportRange.MinRow(), a.pendingExportRange.MaxCol(), a.pendingExportRange.MaxRow())
	} else {
		err = a.sheet.ExportMarkdown(fn)
	}
	if err == nil {
		a.statusMessage = fmt.Sprintf("Exported Markdown table to '%s'.", filepath.Base(fn))
	} else {
		a.statusMessage = fmt.Sprintf("Error exporting Markdown: %v", err)
	}
	a.pendingExportRange = nil
}

func parseColInput(s string, defaultCol int) int {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return defaultCol
	}
	if cr, err := coord.ParseCellRef(s); err == nil {
		return cr.Col
	}
	allLetters := true
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			allLetters = false
			break
		}
	}
	if allLetters && len(s) > 0 {
		col := 0
		for _, r := range s {
			col = col*26 + int(r-'A'+1)
		}
		return col - 1
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 1 {
		return n - 1
	}
	return defaultCol
}

func parseRowInput(s string, defaultRow int) int {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return defaultRow
	}
	if cr, err := coord.ParseCellRef(s); err == nil {
		return cr.Row
	}
	if n, err := strconv.Atoi(s); err == nil && n >= 1 {
		return n - 1
	}
	return defaultRow
}

func (a *App) doOpenFunctionPicker() {
	snippet := ShowFunctionPickerModal(a.screen, a.styles)
	if snippet != "" {
		if a.selectedRange != nil {
			snippet = fmt.Sprintf("%s%s)", snippet, a.selectedRange.String())
		}
		a.clearSelection()
		a.inputBuffer = []rune(snippet)
		a.inputCursorPos = len(a.inputBuffer)
		a.mode = "INPUT"
	}
}

func (a *App) CursorCol() int {
	return a.cursorCol
}

func (a *App) CursorRow() int {
	return a.cursorRow
}

func (a *App) SelectedRange() *coord.RangeRef {
	return a.selectedRange
}

func (a *App) ActiveSheet() *sheet.Sheet {
	return a.sheet
}

func (a *App) TopRow() int {
	return a.topRow
}
