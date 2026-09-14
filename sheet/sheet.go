package sheet

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
)

type CellCoord struct {
	Col int
	Row int
}

// GraphConfig is HasuCalc's terminal/PNG graph (one per sheet).
// It is persisted only in .hwk/.hwkz. XLSX and ODS omit charts by design:
// Excel/ODS chart models are not a 1:1 match, and a partial mapping is worse
// than exporting sheets without charts.
type GraphConfig struct {
	Type   string                     `json:"type"` // "BAR", "LINE", "STACKED"
	Title  string                     `json:"title"`
	RangeX *coord.RangeRef            `json:"range_x,omitempty"`
	Series map[string]*coord.RangeRef `json:"series"`
}

type Sheet struct {
	name            string
	workbook        *Workbook
	cells           map[CellCoord]*cell.Cell
	colWidths       map[int]int
	defaultColWidth int
	globalFormat    cell.CellFormat
	recalcMode      string // "AUTO", "MANUAL"
	needsRecalc     bool
	modified        bool
	namedRanges     map[string]any // CellRef or RangeRef
	graph           GraphConfig
	frozenRows      int
	frozenCols      int
	maxRows         int
	maxCols         int
	maxPopulatedRow int
	maxPopulatedCol int
	evalGen         uint64
	suspendRecalc   bool
}

func NewSheet() *Sheet {
	return &Sheet{
		name:            "Sheet1",
		cells:           make(map[CellCoord]*cell.Cell),
		colWidths:       make(map[int]int),
		defaultColWidth: 9,
		globalFormat:    cell.CellFormat{Type: cell.FmtGeneral},
		recalcMode:      "AUTO",
		needsRecalc:     false,
		modified:        false,
		namedRanges:     make(map[string]any),
		graph: GraphConfig{
			Type:   "LINE",
			Title:  "",
			Series: make(map[string]*coord.RangeRef),
		},
		maxRows:         1048576,
		maxCols:         16384,
		maxPopulatedRow: 0,
		maxPopulatedCol: 0,
	}
}

func (s *Sheet) FrozenRows() int {
	return s.frozenRows
}

func (s *Sheet) SetFrozenRows(r int) {
	if r < 0 {
		r = 0
	}
	s.frozenRows = r
	s.modified = true
}

func (s *Sheet) FrozenCols() int {
	return s.frozenCols
}

func (s *Sheet) SetFrozenCols(c int) {
	if c < 0 {
		c = 0
	}
	s.frozenCols = c
	s.modified = true
}

func (s *Sheet) Name() string {
	if s.name == "" {
		return "Sheet1"
	}
	return s.name
}

func (s *Sheet) SetName(name string) {
	name = strings.TrimSpace(name)
	if s.name == name {
		return
	}
	s.name = name
	s.modified = true
}

func (s *Sheet) Workbook() *Workbook {
	return s.workbook
}

func (s *Sheet) SetWorkbook(wb *Workbook) {
	s.workbook = wb
}

type SheetSnapshot struct {
	cells           map[CellCoord]*cell.Cell
	colWidths       map[int]int
	globalFormat    cell.CellFormat
	graph           GraphConfig
	frozenRows      int
	frozenCols      int
	namedRanges     map[string]any
	defaultColWidth int
	recalcMode      string
}

func (s *Sheet) CreateSnapshot() SheetSnapshot {
	cellsCopy := make(map[CellCoord]*cell.Cell, len(s.cells))
	for pt, c := range s.cells {
		if c != nil {
			var fmtCopy *cell.CellFormat
			if c.FormatSpec != nil {
				f := *c.FormatSpec
				fmtCopy = &f
			}
			cellsCopy[pt] = &cell.Cell{
				RawInput:   c.RawInput,
				Type:       c.Type,
				Alignment:  c.Alignment,
				FormatSpec: fmtCopy,
				Value:      c.Value,
			}
		}
	}
	widthsCopy := make(map[int]int, len(s.colWidths))
	for k, v := range s.colWidths {
		widthsCopy[k] = v
	}
	seriesCopy := make(map[string]*coord.RangeRef, len(s.graph.Series))
	for k, v := range s.graph.Series {
		if v != nil {
			vr := *v
			seriesCopy[k] = &vr
		}
	}
	var rxCopy *coord.RangeRef
	if s.graph.RangeX != nil {
		rx := *s.graph.RangeX
		rxCopy = &rx
	}
	namedCopy := make(map[string]any, len(s.namedRanges))
	for k, v := range s.namedRanges {
		namedCopy[k] = v
	}
	return SheetSnapshot{
		cells:        cellsCopy,
		colWidths:    widthsCopy,
		globalFormat: s.globalFormat,
		graph: GraphConfig{
			Type:   s.graph.Type,
			Title:  s.graph.Title,
			RangeX: rxCopy,
			Series: seriesCopy,
		},
		frozenRows:      s.frozenRows,
		frozenCols:      s.frozenCols,
		namedRanges:     namedCopy,
		defaultColWidth: s.defaultColWidth,
		recalcMode:      s.recalcMode,
	}
}

func (s *Sheet) RestoreSnapshot(snap SheetSnapshot) {
	s.RestoreSnapshotNoRecalc(snap)
	s.Recalculate()
}

// RestoreSnapshotNoRecalc restores cells/layout without recalculating (for batch workbook undo).
func (s *Sheet) RestoreSnapshotNoRecalc(snap SheetSnapshot) {
	s.cells = snap.cells
	s.colWidths = snap.colWidths
	s.globalFormat = snap.globalFormat
	s.graph = snap.graph
	s.frozenRows = snap.frozenRows
	s.frozenCols = snap.frozenCols
	if snap.namedRanges != nil {
		s.namedRanges = snap.namedRanges
	} else {
		s.namedRanges = make(map[string]any)
	}
	if snap.defaultColWidth > 0 {
		s.defaultColWidth = snap.defaultColWidth
	}
	if snap.recalcMode != "" {
		s.recalcMode = snap.recalcMode
	}
	s.recomputePopulatedBounds()
	s.needsRecalc = true
	s.modified = true
}

func ParseFlexibleDate(s string) (time.Time, string, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "\"")
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, "", false
	}

	formats := []struct {
		layout string
		outFmt string
	}{
		{"2006-01-02", "2006-01-02"},
		{"2006-1-2", "2006-01-02"},
		{"2006/01/02", "2006/01/02"},
		{"2006/1/2", "2006/01/02"},
		{"2006.01.02", "2006.01.02"},
		{"2006.1.2", "2006.01.02"},
		{"01/02/2006", "01/02/2006"},
		{"1/2/2006", "01/02/2006"},
		{"02-Jan-2006", "02-Jan-2006"},
		{"2-Jan-2006", "02-Jan-2006"},
		{"2006-01", "2006-01"},
		{"2006/01", "2006/01"},
	}

	for _, f := range formats {
		if t, err := time.Parse(f.layout, s); err == nil {
			return t, f.outFmt, true
		}
	}
	return time.Time{}, "", false
}

func (s *Sheet) DataFill(target coord.RangeRef, startStr, stepStr, stopStr string) error {
	startStr = strings.TrimSpace(startStr)
	stepStr = strings.TrimSpace(stepStr)
	stopStr = strings.TrimSpace(stopStr)

	// Check if date format
	if t, outFmt, isDate := ParseFlexibleDate(startStr); isDate {
		stepDays := 1
		stepMonths := 0
		stepYears := 0
		stepLower := strings.ToLower(stepStr)

		if strings.HasSuffix(stepLower, "y") {
			if n, err := strconv.Atoi(strings.TrimSuffix(stepLower, "y")); err == nil {
				stepYears = n
				stepDays = 0
			}
		} else if strings.HasSuffix(stepLower, "m") {
			if n, err := strconv.Atoi(strings.TrimSuffix(stepLower, "m")); err == nil {
				stepMonths = n
				stepDays = 0
			}
		} else if strings.HasSuffix(stepLower, "w") {
			if n, err := strconv.Atoi(strings.TrimSuffix(stepLower, "w")); err == nil {
				stepDays = n * 7
			}
		} else if strings.HasSuffix(stepLower, "d") {
			if n, err := strconv.Atoi(strings.TrimSuffix(stepLower, "d")); err == nil {
				stepDays = n
			}
		} else if n, err := strconv.Atoi(stepStr); err == nil {
			stepDays = n
		}

		var stopTime *time.Time
		if stopStr != "" {
			if st, _, ok := ParseFlexibleDate(stopStr); ok {
				stopTime = &st
			}
		}

		advanceDate := func(cur time.Time) time.Time {
			if stepYears != 0 {
				return cur.AddDate(stepYears, 0, 0)
			}
			if stepMonths != 0 {
				return cur.AddDate(0, stepMonths, 0)
			}
			return cur.AddDate(0, 0, stepDays)
		}

		if target.MaxRow() > target.MinRow() && target.MaxCol() > target.MinCol() {
			for c := target.MinCol(); c <= target.MaxCol(); c++ {
				curTime := t
				for r := target.MinRow(); r <= target.MaxRow(); r++ {
					if stopTime != nil && curTime.After(*stopTime) {
						break
					}
					s.SetCellInput(c, r, fmt.Sprintf("'%s", curTime.Format(outFmt)), nil)
					curTime = advanceDate(curTime)
				}
			}
		} else if target.MaxRow() > target.MinRow() {
			curTime := t
			for r := target.MinRow(); r <= target.MaxRow(); r++ {
				for c := target.MinCol(); c <= target.MaxCol(); c++ {
					if stopTime != nil && curTime.After(*stopTime) {
						break
					}
					s.SetCellInput(c, r, fmt.Sprintf("'%s", curTime.Format(outFmt)), nil)
					curTime = advanceDate(curTime)
				}
			}
		} else {
			curTime := t
			for c := target.MinCol(); c <= target.MaxCol(); c++ {
				if stopTime != nil && curTime.After(*stopTime) {
					break
				}
				s.SetCellInput(c, target.MinRow(), fmt.Sprintf("'%s", curTime.Format(outFmt)), nil)
				curTime = advanceDate(curTime)
			}
		}
		s.Recalculate()
		return nil
	}

	// Numeric sequence
	startVal, err := strconv.ParseFloat(startStr, 64)
	if err != nil {
		startVal = 1.0
	}
	stepVal := 1.0
	if stepStr != "" {
		if sv, err := strconv.ParseFloat(stepStr, 64); err == nil {
			stepVal = sv
		}
	}
	var stopVal *float64
	if stopStr != "" {
		if sv, err := strconv.ParseFloat(stopStr, 64); err == nil {
			stopVal = &sv
		}
	}

	if target.MaxRow() > target.MinRow() && target.MaxCol() > target.MinCol() {
		for c := target.MinCol(); c <= target.MaxCol(); c++ {
			curVal := startVal
			for r := target.MinRow(); r <= target.MaxRow(); r++ {
				if stopVal != nil {
					if stepVal > 0 && curVal > *stopVal {
						break
					} else if stepVal < 0 && curVal < *stopVal {
						break
					}
				}
				valStr := fmt.Sprintf("%g", curVal)
				s.SetCellInput(c, r, valStr, nil)
				curVal += stepVal
			}
		}
	} else if target.MaxRow() > target.MinRow() {
		curVal := startVal
		for r := target.MinRow(); r <= target.MaxRow(); r++ {
			for c := target.MinCol(); c <= target.MaxCol(); c++ {
				if stopVal != nil {
					if stepVal > 0 && curVal > *stopVal {
						break
					} else if stepVal < 0 && curVal < *stopVal {
						break
					}
				}
				valStr := fmt.Sprintf("%g", curVal)
				s.SetCellInput(c, r, valStr, nil)
				curVal += stepVal
			}
		}
	} else {
		curVal := startVal
		for c := target.MinCol(); c <= target.MaxCol(); c++ {
			if stopVal != nil {
				if stepVal > 0 && curVal > *stopVal {
					break
				} else if stepVal < 0 && curVal < *stopVal {
					break
				}
			}
			valStr := fmt.Sprintf("%g", curVal)
			s.SetCellInput(c, target.MinRow(), valStr, nil)
			curVal += stepVal
		}
	}
	s.Recalculate()
	return nil
}

func (s *Sheet) TransposeRange(source coord.RangeRef, target coord.CellRef) error {
	minC, minR := source.MinCol(), source.MinRow()
	maxC, maxR := source.MaxCol(), source.MaxRow()

	type copiedCell struct {
		dCol      int
		dRow      int
		raw       string
		fmt       *cell.CellFormat
		isFormula bool
	}
	var copied []copiedCell

	for r := minR; r <= maxR; r++ {
		for c := minC; c <= maxC; c++ {
			cData := s.GetCell(c, r)
			if cData != nil && cData.Type != cell.TypeEmpty {
				var fmtCopy *cell.CellFormat
				if cData.FormatSpec != nil {
					f := *cData.FormatSpec
					fmtCopy = &f
				}
				copied = append(copied, copiedCell{
					dCol:      c - minC,
					dRow:      r - minR,
					raw:       cData.RawInput,
					fmt:       fmtCopy,
					isFormula: cData.Type == cell.TypeFormula,
				})
			}
		}
	}

	// If transposing in place, clear the source range first
	if target.Col == minC && target.Row == minR {
		for r := minR; r <= maxR; r++ {
			for c := minC; c <= maxC; c++ {
				s.ClearCell(c, r)
			}
		}
	}

	for _, cc := range copied {
		dstCol := target.Col + cc.dRow
		dstRow := target.Row + cc.dCol
		if dstCol < s.MaxCols() && dstRow < s.MaxRows() {
			raw := cc.raw
			if cc.isFormula {
				origC := minC + cc.dCol
				origR := minR + cc.dRow
				raw = formula.TransposeFormulaReferences(cc.raw, origC, origR, dstCol, dstRow)
			}
			s.SetCellInput(dstCol, dstRow, raw, cc.fmt)
		}
	}
	s.Recalculate()
	return nil
}

// BoundingBox returns the minimum and maximum row and column indices that contain cells with values.
func (s *Sheet) BoundingBox() (minCol, minRow, maxCol, maxRow int) {
	if len(s.cells) == 0 {
		return 0, 0, 0, 0
	}
	hasCells := false
	for pt, c := range s.cells {
		if c == nil || c.Type == cell.TypeEmpty {
			continue
		}
		if !hasCells {
			minCol, maxCol = pt.Col, pt.Col
			minRow, maxRow = pt.Row, pt.Row
			hasCells = true
		} else {
			if pt.Col < minCol {
				minCol = pt.Col
			}
			if pt.Col > maxCol {
				maxCol = pt.Col
			}
			if pt.Row < minRow {
				minRow = pt.Row
			}
			if pt.Row > maxRow {
				maxRow = pt.Row
			}
		}
	}
	if !hasCells {
		return 0, 0, 0, 0
	}
	return minCol, minRow, maxCol, maxRow
}

type undoSheetSnap struct {
	sheetName string
	snap      SheetSnapshot
}

// WorkbookSnapshot captures sheet order, names, content, and workbook-level names for structural undo.
type WorkbookSnapshot struct {
	Sheets           []undoSheetSnap
	ActiveSheetIndex int
	NamedRanges      map[string]any
}

type undoEntry struct {
	id     uint64
	sheets []undoSheetSnap
	wb     *WorkbookSnapshot // when set, undo/redo restores workbook structure
}

type UndoManager struct {
	undoStack []undoEntry
	redoStack []undoEntry
	maxLevels int
	nextID    uint64
}

func NewUndoManager(maxLevels int) *UndoManager {
	if maxLevels <= 0 {
		maxLevels = 50
	}
	return &UndoManager{maxLevels: maxLevels}
}

func (um *UndoManager) Push(s *Sheet) {
	if s == nil {
		return
	}
	um.PushMulti([]*Sheet{s})
}

// PushMulti records one undo step covering multiple sheets (e.g. replace-all).
func (um *UndoManager) PushMulti(sheets []*Sheet) {
	if len(sheets) == 0 {
		return
	}
	entry := undoEntry{sheets: make([]undoSheetSnap, 0, len(sheets))}
	seen := make(map[string]bool, len(sheets))
	for _, s := range sheets {
		if s == nil {
			continue
		}
		name := s.Name()
		if seen[name] {
			continue
		}
		seen[name] = true
		entry.sheets = append(entry.sheets, undoSheetSnap{sheetName: name, snap: s.CreateSnapshot()})
	}
	if len(entry.sheets) == 0 {
		return
	}
	um.pushEntry(entry)
}

// PushWorkbook records the full workbook (sheet list + contents) for add/delete/rename undo.
func (um *UndoManager) PushWorkbook(wb *Workbook) {
	if wb == nil || len(wb.Sheets) == 0 {
		return
	}
	um.pushEntry(undoEntry{wb: CaptureWorkbookSnapshot(wb)})
}

func CaptureWorkbookSnapshot(wb *Workbook) *WorkbookSnapshot {
	if wb == nil {
		return nil
	}
	snap := &WorkbookSnapshot{
		Sheets:           make([]undoSheetSnap, 0, len(wb.Sheets)),
		ActiveSheetIndex: wb.ActiveSheetIndex,
		NamedRanges:      make(map[string]any, len(wb.NamedRanges)),
	}
	for _, s := range wb.Sheets {
		if s == nil {
			continue
		}
		snap.Sheets = append(snap.Sheets, undoSheetSnap{sheetName: s.Name(), snap: s.CreateSnapshot()})
	}
	for k, v := range wb.NamedRanges {
		snap.NamedRanges[k] = v
	}
	return snap
}

func (um *UndoManager) assignID(entry undoEntry) undoEntry {
	um.nextID++
	entry.id = um.nextID
	return entry
}

// PeekUndoID returns the id of the next undo entry, or 0 if the stack is empty.
func (um *UndoManager) PeekUndoID() uint64 {
	if um == nil || len(um.undoStack) == 0 {
		return 0
	}
	return um.undoStack[len(um.undoStack)-1].id
}

func (um *UndoManager) pushEntry(entry undoEntry) {
	entry = um.assignID(entry)
	um.undoStack = append(um.undoStack, entry)
	if len(um.undoStack) > um.maxLevels {
		um.undoStack = um.undoStack[1:]
	}
	um.redoStack = nil
}

// ApplyWorkbookSnapshot replaces workbook sheets/order/names from a snapshot.
func ApplyWorkbookSnapshot(wb *Workbook, snap *WorkbookSnapshot) {
	if wb == nil || snap == nil {
		return
	}
	existingMap := make(map[string]*Sheet)
	for _, s := range wb.Sheets {
		if s != nil {
			existingMap[s.Name()] = s
		}
	}

	newSheets := make([]*Sheet, 0, len(snap.Sheets))
	for _, ss := range snap.Sheets {
		sh, exists := existingMap[ss.sheetName]
		if !exists {
			sh = NewSheet()
			sh.SetName(ss.sheetName)
			sh.SetWorkbook(wb)
		}
		sh.RestoreSnapshotNoRecalc(ss.snap)
		newSheets = append(newSheets, sh)
	}
	wb.Sheets = newSheets
	wb.ActiveSheetIndex = snap.ActiveSheetIndex
	if wb.ActiveSheetIndex < 0 || wb.ActiveSheetIndex >= len(wb.Sheets) {
		wb.ActiveSheetIndex = 0
	}
	if snap.NamedRanges != nil {
		wb.NamedRanges = make(map[string]any, len(snap.NamedRanges))
		for k, v := range snap.NamedRanges {
			wb.NamedRanges[k] = v
		}
	} else {
		wb.NamedRanges = make(map[string]any)
	}
	wb.RecalculateAll()
}

// Undo restores the last snapshot(s). resolve returns the sheet by name (and may switch UI focus).
// When the entry is a workbook snapshot, wb must be non-nil; afterApply is called after structural restore.
func (um *UndoManager) Undo(current *Sheet, resolve func(name string) *Sheet, wb *Workbook, afterWorkbook func()) bool {
	if len(um.undoStack) == 0 {
		return false
	}
	entry := um.undoStack[len(um.undoStack)-1]

	if entry.wb != nil {
		if wb == nil {
			um.undoStack = um.undoStack[:len(um.undoStack)-1]
			return false
		}
		um.undoStack = um.undoStack[:len(um.undoStack)-1]
		um.redoStack = append(um.redoStack, um.assignID(undoEntry{wb: CaptureWorkbookSnapshot(wb)}))
		ApplyWorkbookSnapshot(wb, entry.wb)
		if afterWorkbook != nil {
			afterWorkbook()
		}
		return true
	}

	type restore struct {
		target *Sheet
		snap   SheetSnapshot
	}
	var restores []restore
	for _, ss := range entry.sheets {
		var target *Sheet
		if resolve != nil {
			target = resolve(ss.sheetName)
		} else if current != nil && (ss.sheetName == "" || current.Name() == ss.sheetName) {
			target = current
		}
		if target != nil {
			restores = append(restores, restore{target: target, snap: ss.snap})
		}
	}
	if len(restores) == 0 {
		um.undoStack = um.undoStack[:len(um.undoStack)-1]
		return um.Undo(current, resolve, wb, afterWorkbook)
	}
	um.undoStack = um.undoStack[:len(um.undoStack)-1]

	redoEntry := undoEntry{sheets: make([]undoSheetSnap, 0, len(restores))}
	for _, r := range restores {
		redoEntry.sheets = append(redoEntry.sheets, undoSheetSnap{sheetName: r.target.Name(), snap: r.target.CreateSnapshot()})
	}
	um.redoStack = append(um.redoStack, um.assignID(redoEntry))
	for _, r := range restores {
		r.target.RestoreSnapshot(r.snap)
	}
	return true
}

func (um *UndoManager) Redo(current *Sheet, resolve func(name string) *Sheet, wb *Workbook, afterWorkbook func()) bool {
	if len(um.redoStack) == 0 {
		return false
	}
	entry := um.redoStack[len(um.redoStack)-1]

	if entry.wb != nil {
		if wb == nil {
			um.redoStack = um.redoStack[:len(um.redoStack)-1]
			return false
		}
		um.redoStack = um.redoStack[:len(um.redoStack)-1]
		um.undoStack = append(um.undoStack, um.assignID(undoEntry{wb: CaptureWorkbookSnapshot(wb)}))
		ApplyWorkbookSnapshot(wb, entry.wb)
		if afterWorkbook != nil {
			afterWorkbook()
		}
		return true
	}

	type restore struct {
		target *Sheet
		snap   SheetSnapshot
	}
	var restores []restore
	for _, ss := range entry.sheets {
		var target *Sheet
		if resolve != nil {
			target = resolve(ss.sheetName)
		} else if current != nil && (ss.sheetName == "" || current.Name() == ss.sheetName) {
			target = current
		}
		if target != nil {
			restores = append(restores, restore{target: target, snap: ss.snap})
		}
	}
	if len(restores) == 0 {
		um.redoStack = um.redoStack[:len(um.redoStack)-1]
		return um.Redo(current, resolve, wb, afterWorkbook)
	}
	um.redoStack = um.redoStack[:len(um.redoStack)-1]

	undoEntryOut := undoEntry{sheets: make([]undoSheetSnap, 0, len(restores))}
	for _, r := range restores {
		undoEntryOut.sheets = append(undoEntryOut.sheets, undoSheetSnap{sheetName: r.target.Name(), snap: r.target.CreateSnapshot()})
	}
	um.undoStack = append(um.undoStack, um.assignID(undoEntryOut))
	for _, r := range restores {
		r.target.RestoreSnapshot(r.snap)
	}
	return true
}

func (s *Sheet) GetColWidth(col int) int {
	if w, ok := s.colWidths[col]; ok {
		return w
	}
	return s.defaultColWidth
}

func (s *Sheet) SetColWidth(col, width int) {
	if width < 1 {
		width = 1
	}
	if width > 72 {
		width = 72
	}
	s.colWidths[col] = width
}

func (s *Sheet) ResetColWidth(col int) {
	delete(s.colWidths, col)
}

func (s *Sheet) DefaultColWidth() int {
	return s.defaultColWidth
}

func (s *Sheet) SetDefaultColWidth(w int) {
	if w < 1 {
		w = 1
	}
	if w > 72 {
		w = 72
	}
	s.defaultColWidth = w
}

func (s *Sheet) GlobalFormat() cell.CellFormat {
	return s.globalFormat
}

func (s *Sheet) SetGlobalFormat(fmtSpec cell.CellFormat) {
	s.globalFormat = fmtSpec
}

func (s *Sheet) RecalcMode() string {
	return s.recalcMode
}

func (s *Sheet) SetRecalcMode(mode string) {
	s.recalcMode = mode
}

func (s *Sheet) NeedsRecalc() bool {
	return s.needsRecalc
}

func (s *Sheet) IsModified() bool {
	return s.modified
}

func (s *Sheet) SetModified(m bool) {
	s.modified = m
}

func (s *Sheet) Graph() *GraphConfig {
	return &s.graph
}

func (s *Sheet) MaxRows() int {
	return s.maxRows
}

func (s *Sheet) MaxCols() int {
	return s.maxCols
}

func (s *Sheet) MaxPopulatedRow() int {
	return s.maxPopulatedRow
}

func (s *Sheet) MaxPopulatedCol() int {
	return s.maxPopulatedCol
}

func (s *Sheet) EvalGeneration() uint64 {
	return s.evalGen
}

func (s *Sheet) NamedRanges() map[string]any {
	return s.namedRanges
}

func (s *Sheet) SetNamedRange(name string, target any) {
	if s.namedRanges == nil {
		s.namedRanges = make(map[string]any)
	}
	s.namedRanges[strings.ToUpper(name)] = target
	s.modified = true
}

func (s *Sheet) DeleteNamedRange(name string) {
	if s.namedRanges != nil {
		delete(s.namedRanges, strings.ToUpper(name))
		s.modified = true
	}
}

func (s *Sheet) GetPopulatedCoords() map[CellCoord]struct{} {
	coords := make(map[CellCoord]struct{}, len(s.cells))
	for pt := range s.cells {
		coords[pt] = struct{}{}
	}
	return coords
}

func (s *Sheet) GetCell(col, row int) *cell.Cell {
	return s.cells[CellCoord{Col: col, Row: row}]
}

func (s *Sheet) GetRawCell(col, row int) *cell.Cell {
	return s.cells[CellCoord{Col: col, Row: row}]
}

func (s *Sheet) GetCellValue(col, row int) any {
	c := s.cells[CellCoord{Col: col, Row: row}]
	if c == nil {
		return nil
	}
	return c.Value
}

func (s *Sheet) GetCrossSheetCell(sheetName string, col, row int) *cell.Cell {
	if sheetName == "" || s.workbook == nil || strings.EqualFold(sheetName, s.Name()) {
		return s.GetCell(col, row)
	}
	targetSheet := s.workbook.GetSheet(sheetName)
	if targetSheet == nil {
		return nil
	}
	return targetSheet.GetCell(col, row)
}

func (s *Sheet) GetCrossSheetCellValue(sheetName string, col, row int) any {
	if sheetName == "" || s.workbook == nil || strings.EqualFold(sheetName, s.Name()) {
		return s.GetCellValue(col, row)
	}
	targetSheet := s.workbook.GetSheet(sheetName)
	if targetSheet == nil {
		return nil
	}
	return targetSheet.GetCellValue(col, row)
}

func (s *Sheet) GetRangeValues(r coord.RangeRef) [][]any {
	targetSheet := s
	if r.Sheet != "" && s.workbook != nil && !strings.EqualFold(r.Sheet, s.Name()) {
		if ts := s.workbook.GetSheet(r.Sheet); ts != nil {
			targetSheet = ts
		} else {
			return nil
		}
	}

	minRow := r.MinRow()
	maxRow := r.MaxRow()
	minCol := r.MinCol()
	maxCol := r.MaxCol()

	// If whole-column range (e.g. A:B), cap maxRow to maximum populated row in targetSheet
	if maxRow >= 100000 {
		maxRow = targetSheet.maxPopulatedRow
		if maxRow < minRow {
			maxRow = minRow
		}
	}

	// If whole-row range (e.g. 1:10), cap maxCol to maximum populated col in targetSheet
	if maxCol >= 10000 {
		maxCol = targetSheet.maxPopulatedCol
		if maxCol < minCol {
			maxCol = minCol
		}
	}

	numRows := maxRow - minRow + 1
	numCols := maxCol - minCol + 1
	if numRows <= 0 || numCols <= 0 {
		return nil
	}

	// Cached values only — live formula evaluation belongs in Evaluator.RangeRefNode
	// (calling Evaluate here re-enters GetCrossSheetRangeValues and overflows the stack).
	grid := make([][]any, numRows)
	for i := 0; i < numRows; i++ {
		row := minRow + i
		rowVals := make([]any, numCols)
		for j := 0; j < numCols; j++ {
			col := minCol + j
			if c, ok := targetSheet.cells[CellCoord{Col: col, Row: row}]; ok && c != nil {
				rowVals[j] = c.Value
			}
		}
		grid[i] = rowVals
	}
	return grid
}

func (s *Sheet) GetCrossSheetRangeValues(sheetName string, r coord.RangeRef) [][]any {
	r.Sheet = sheetName
	return s.GetRangeValues(r)
}

func (s *Sheet) GetNamedRange(name string) (any, bool) {
	name = strings.ToUpper(name)
	if v, ok := s.namedRanges[name]; ok {
		return v, true
	}
	if s.workbook != nil {
		if v, ok := s.workbook.GetNamedRange(name); ok {
			return v, true
		}
	}
	return nil, false
}

func (s *Sheet) GetCurrentSheetName() string {
	return s.name
}

func (s *Sheet) GetCurrentSheetIndex() int {
	if s.workbook != nil {
		for i, sh := range s.workbook.Sheets {
			if sh == s {
				return i + 1
			}
		}
	}
	return 1
}

func (s *Sheet) GetSheetName(index int) string {
	if s.workbook != nil {
		if index >= 1 && index <= len(s.workbook.Sheets) {
			return s.workbook.Sheets[index-1].Name()
		}
	}
	if index == 1 {
		return s.name
	}
	return ""
}

func (s *Sheet) SheetContext(sheetName string) formula.EvaluationContext {
	if sheetName == "" || strings.EqualFold(sheetName, s.Name()) {
		return s
	}
	if s.workbook == nil {
		return nil
	}
	ts := s.workbook.GetSheet(sheetName)
	if ts == nil {
		return nil
	}
	return ts
}

func (s *Sheet) recomputePopulatedBounds() {
	s.maxPopulatedRow = 0
	s.maxPopulatedCol = 0
	for pt, c := range s.cells {
		if c == nil || c.Type == cell.TypeEmpty {
			continue
		}
		if pt.Col > s.maxPopulatedCol {
			s.maxPopulatedCol = pt.Col
		}
		if pt.Row > s.maxPopulatedRow {
			s.maxPopulatedRow = pt.Row
		}
	}
}

func (s *Sheet) TriggerAutoRecalc() {
	if s.suspendRecalc {
		s.needsRecalc = true
		return
	}
	if s.recalcMode == "AUTO" {
		if s.workbook != nil {
			s.workbook.RecalculateAll()
		} else {
			s.Recalculate()
		}
	} else {
		s.needsRecalc = true
	}
}

// SuspendRecalc defers TriggerAutoRecalc until EndSuspendRecalc (for batch paste/replace).
func (s *Sheet) SuspendRecalc() {
	s.suspendRecalc = true
}

func (s *Sheet) EndSuspendRecalc() {
	if !s.suspendRecalc {
		return
	}
	s.suspendRecalc = false
	if s.needsRecalc {
		s.TriggerAutoRecalc()
	}
}

// ClearSuspendRecalc turns off suspend without recalculating (caller will recalc).
func (s *Sheet) ClearSuspendRecalc() {
	s.suspendRecalc = false
}

func (s *Sheet) SetCell(col, row int, c *cell.Cell) {
	if col < 0 || row < 0 || col >= s.maxCols || row >= s.maxRows {
		return
	}
	if c == nil {
		s.ClearCell(col, row)
		return
	}
	pt := CellCoord{Col: col, Row: row}
	s.cells[pt] = c
	if col > s.maxPopulatedCol {
		s.maxPopulatedCol = col
	}
	if row > s.maxPopulatedRow {
		s.maxPopulatedRow = row
	}
	s.modified = true
	s.TriggerAutoRecalc()
}

func (s *Sheet) SetCellInput(col, row int, rawText string, fmtSpec *cell.CellFormat) {
	if col < 0 || row < 0 || col >= s.maxCols || row >= s.maxRows {
		return
	}
	if strings.TrimSpace(rawText) == "" {
		s.ClearCell(col, row)
		return
	}

	pt := CellCoord{Col: col, Row: row}
	existing := s.cells[pt]
	appliedFmt := fmtSpec
	if appliedFmt == nil && existing != nil {
		appliedFmt = existing.FormatSpec
	}

	c := cell.NewCell(rawText, appliedFmt)
	s.cells[pt] = c
	if col > s.maxPopulatedCol {
		s.maxPopulatedCol = col
	}
	if row > s.maxPopulatedRow {
		s.maxPopulatedRow = row
	}
	s.modified = true
	s.TriggerAutoRecalc()
}

func (s *Sheet) ClearCell(col, row int) {
	pt := CellCoord{Col: col, Row: row}
	if _, ok := s.cells[pt]; ok {
		delete(s.cells, pt)
		s.modified = true
		s.recomputePopulatedBounds()
		s.TriggerAutoRecalc()
	}
}

func (s *Sheet) ClearRange(r coord.RangeRef) {
	if r.MaxRow()-r.MinRow() > 10000 || r.MaxCol()-r.MinCol() > 1000 {
		for pt := range s.cells {
			if r.Contains(pt.Col, pt.Row) {
				delete(s.cells, pt)
			}
		}
	} else {
		for row := r.MinRow(); row <= r.MaxRow(); row++ {
			for col := r.MinCol(); col <= r.MaxCol(); col++ {
				delete(s.cells, CellCoord{Col: col, Row: row})
			}
		}
	}
	s.modified = true
	s.recomputePopulatedBounds()
	s.TriggerAutoRecalc()
}

func (s *Sheet) EraseAll() {
	s.cells = make(map[CellCoord]*cell.Cell)
	s.colWidths = make(map[int]int)
	s.namedRanges = make(map[string]any)
	s.globalFormat = cell.CellFormat{Type: cell.FmtGeneral}
	s.recalcMode = "AUTO"
	s.needsRecalc = false
	s.modified = true
	s.frozenRows = 0
	s.frozenCols = 0
	s.maxPopulatedRow = 0
	s.maxPopulatedCol = 0
	s.graph = GraphConfig{
		Type:   "LINE",
		Title:  "Graph",
		Series: make(map[string]*coord.RangeRef),
	}
}

func (s *Sheet) Recalculate() {
	s.evalGen++
	evaluator := formula.NewEvaluator(s)
	type formulaCell struct {
		col, row int
		c        *cell.Cell
	}
	var formulas []formulaCell
	for pt, c := range s.cells {
		if c != nil && c.Type == cell.TypeFormula {
			formulas = append(formulas, formulaCell{pt.Col, pt.Row, c})
		}
	}
	// Row-major order so relative named formulas (e.g. DayOfWeek → OFFSET above)
	// usually see already-computed neighbors instead of false circular refs.
	sort.Slice(formulas, func(i, j int) bool {
		if formulas[i].row != formulas[j].row {
			return formulas[i].row < formulas[j].row
		}
		return formulas[i].col < formulas[j].col
	})
	for _, f := range formulas {
		val := evaluator.EvaluateCellAt(f.col, f.row, f.c.RawInput)
		f.c.SetValue(val)
		f.c.EvalGen = s.evalGen
	}
	s.needsRecalc = false
}

func (s *Sheet) CopyRangeFrom(srcSheet *Sheet, fromR, toR coord.RangeRef) {
	if srcSheet == nil {
		srcSheet = s
	}
	srcW := fromR.MaxCol() - fromR.MinCol() + 1
	srcH := fromR.MaxRow() - fromR.MinRow() + 1

	snapshot := make(map[CellCoord]*cell.Cell)
	for rOff := 0; rOff < srcH; rOff++ {
		for cOff := 0; cOff < srcW; cOff++ {
			sc := fromR.MinCol() + cOff
			sr := fromR.MinRow() + rOff
			if c, ok := srcSheet.cells[CellCoord{Col: sc, Row: sr}]; ok && c != nil {
				var fmtCopy *cell.CellFormat
				if c.FormatSpec != nil {
					f := *c.FormatSpec
					fmtCopy = &f
				}
				snapshot[CellCoord{Col: cOff, Row: rOff}] = &cell.Cell{
					RawInput:   c.RawInput,
					Type:       c.Type,
					Alignment:  c.Alignment,
					FormatSpec: fmtCopy,
					Value:      c.Value,
				}
			}
		}
	}

	for destR := toR.MinRow(); destR <= toR.MaxRow(); destR++ {
		for destC := toR.MinCol(); destC <= toR.MaxCol(); destC++ {
			cOff := (destC - toR.MinCol()) % srcW
			rOff := (destR - toR.MinRow()) % srcH
			srcCell, ok := snapshot[CellCoord{Col: cOff, Row: rOff}]
			if !ok || srcCell == nil {
				delete(s.cells, CellCoord{Col: destC, Row: destR})
				continue
			}

			origC := fromR.MinCol() + cOff
			origR := fromR.MinRow() + rOff
			dCol := destC - origC
			dRow := destR - origR

			if srcCell.Type == cell.TypeFormula {
				adj := formula.AdjustFormulaReferences(srcCell.RawInput, dCol, dRow)
				s.cells[CellCoord{Col: destC, Row: destR}] = cell.NewCell(adj, srcCell.FormatSpec)
			} else {
				var fmtCopy *cell.CellFormat
				if srcCell.FormatSpec != nil {
					f := *srcCell.FormatSpec
					fmtCopy = &f
				}
				s.cells[CellCoord{Col: destC, Row: destR}] = &cell.Cell{
					RawInput:   srcCell.RawInput,
					Type:       srcCell.Type,
					Alignment:  srcCell.Alignment,
					FormatSpec: fmtCopy,
					Value:      srcCell.Value,
				}
			}
		}
	}

	s.modified = true
	s.recomputePopulatedBounds()
	s.TriggerAutoRecalc()
}

func (s *Sheet) CopyRange(fromR, toR coord.RangeRef) {
	s.CopyRangeFrom(s, fromR, toR)
}

func (s *Sheet) MoveRange(fromR, toR coord.RangeRef) {
	deltaCol := toR.MinCol() - fromR.MinCol()
	deltaRow := toR.MinRow() - fromR.MinRow()

	snapshot := make(map[CellCoord]*cell.Cell)
	for r := fromR.MinRow(); r <= fromR.MaxRow(); r++ {
		for c := fromR.MinCol(); c <= fromR.MaxCol(); c++ {
			pt := CellCoord{Col: c, Row: r}
			if cellVal, ok := s.cells[pt]; ok {
				snapshot[CellCoord{Col: c - fromR.MinCol(), Row: r - fromR.MinRow()}] = cellVal
				delete(s.cells, pt)
			}
		}
	}

	for offPt, cellVal := range snapshot {
		destC := toR.MinCol() + offPt.Col
		destR := toR.MinRow() + offPt.Row
		if cellVal != nil && cellVal.Type == cell.TypeFormula {
			adj := formula.AdjustFormulaReferences(cellVal.RawInput, deltaCol, deltaRow)
			fmtCopy := cellVal.FormatSpec
			s.cells[CellCoord{Col: destC, Row: destR}] = cell.NewCell(adj, fmtCopy)
		} else {
			s.cells[CellCoord{Col: destC, Row: destR}] = cellVal
		}
	}

	s.recomputePopulatedBounds()
	s.modified = true
	s.retargetAllFormulas(fromR, deltaCol, deltaRow, s.Name(), true)
	s.TriggerAutoRecalc()
}

func (s *Sheet) InsertRow(atRow, count int) {
	newCells := make(map[CellCoord]*cell.Cell)
	for pt, c := range s.cells {
		if pt.Row >= atRow {
			newRow := pt.Row + count
			if newRow >= s.maxRows {
				continue
			}
			newCells[CellCoord{Col: pt.Col, Row: newRow}] = c
		} else {
			newCells[pt] = c
		}
	}
	s.cells = newCells
	if s.frozenRows > 0 && atRow < s.frozenRows {
		s.frozenRows += count
	}
	s.adjustAllFormulasForRowChange(atRow, count)
	s.adjustNamedForRowChange(atRow, count)
	s.adjustGraphForRowChange(atRow, count, false)
	s.recomputePopulatedBounds()
	s.modified = true
	s.TriggerAutoRecalc()
}

func (s *Sheet) DeleteRow(atRow, count int) {
	newCells := make(map[CellCoord]*cell.Cell)
	for pt, c := range s.cells {
		if pt.Row >= atRow && pt.Row < atRow+count {
			continue // deleted
		} else if pt.Row >= atRow+count {
			newCells[CellCoord{Col: pt.Col, Row: pt.Row - count}] = c
		} else {
			newCells[pt] = c
		}
	}
	s.cells = newCells
	if s.frozenRows > 0 && atRow < s.frozenRows {
		deletedInFreeze := count
		if atRow+count > s.frozenRows {
			deletedInFreeze = s.frozenRows - atRow
		}
		s.frozenRows -= deletedInFreeze
		if s.frozenRows < 0 {
			s.frozenRows = 0
		}
	}
	s.adjustAllFormulasForRowDelete(atRow, count)
	s.adjustNamedForRowDelete(atRow, count)
	s.adjustGraphForRowChange(atRow, count, true)
	s.recomputePopulatedBounds()
	s.modified = true
	s.TriggerAutoRecalc()
}

func (s *Sheet) InsertCol(atCol, count int) {
	newCells := make(map[CellCoord]*cell.Cell)
	for pt, c := range s.cells {
		if pt.Col >= atCol {
			newCol := pt.Col + count
			if newCol >= s.maxCols {
				continue
			}
			newCells[CellCoord{Col: newCol, Row: pt.Row}] = c
		} else {
			newCells[pt] = c
		}
	}
	s.cells = newCells

	newWidths := make(map[int]int)
	for col, w := range s.colWidths {
		if col >= atCol {
			newWidths[col+count] = w
		} else {
			newWidths[col] = w
		}
	}
	s.colWidths = newWidths
	if s.frozenCols > 0 && atCol < s.frozenCols {
		s.frozenCols += count
	}
	s.adjustAllFormulasForColChange(atCol, count)
	s.adjustNamedForColChange(atCol, count)
	s.adjustGraphForColChange(atCol, count, false)
	s.recomputePopulatedBounds()
	s.modified = true
	s.TriggerAutoRecalc()
}

func (s *Sheet) DeleteCol(atCol, count int) {
	newCells := make(map[CellCoord]*cell.Cell)
	for pt, c := range s.cells {
		if pt.Col >= atCol && pt.Col < atCol+count {
			continue
		} else if pt.Col >= atCol+count {
			newCells[CellCoord{Col: pt.Col - count, Row: pt.Row}] = c
		} else {
			newCells[pt] = c
		}
	}
	s.cells = newCells

	newWidths := make(map[int]int)
	for col, w := range s.colWidths {
		if col >= atCol && col < atCol+count {
			continue
		} else if col >= atCol+count {
			newWidths[col-count] = w
		} else {
			newWidths[col] = w
		}
	}
	s.colWidths = newWidths
	if s.frozenCols > 0 && atCol < s.frozenCols {
		deletedInFreeze := count
		if atCol+count > s.frozenCols {
			deletedInFreeze = s.frozenCols - atCol
		}
		s.frozenCols -= deletedInFreeze
		if s.frozenCols < 0 {
			s.frozenCols = 0
		}
	}
	s.adjustAllFormulasForColDelete(atCol, count)
	s.adjustNamedForColDelete(atCol, count)
	s.adjustGraphForColChange(atCol, count, true)
	s.recomputePopulatedBounds()
	s.modified = true
	s.TriggerAutoRecalc()
}

func (s *Sheet) adjustAllFormulasForRowChange(atRow, delta int) {
	if delta == 0 {
		return
	}
	edited := s.Name()
	s.applyFormulaAdjust(func(raw string) string {
		return formula.AdjustFormulaReferencesForRowChangeOnSheet(raw, edited, true, atRow, delta)
	})
	if s.workbook != nil {
		for _, other := range s.workbook.Sheets {
			if other == s {
				continue
			}
			other.applyFormulaAdjust(func(raw string) string {
				return formula.AdjustFormulaReferencesForRowChangeOnSheet(raw, edited, false, atRow, delta)
			})
			other.modified = true
		}
	}
}

func (s *Sheet) adjustAllFormulasForColChange(atCol, delta int) {
	if delta == 0 {
		return
	}
	edited := s.Name()
	s.applyFormulaAdjust(func(raw string) string {
		return formula.AdjustFormulaReferencesForColChangeOnSheet(raw, edited, true, atCol, delta)
	})
	if s.workbook != nil {
		for _, other := range s.workbook.Sheets {
			if other == s {
				continue
			}
			other.applyFormulaAdjust(func(raw string) string {
				return formula.AdjustFormulaReferencesForColChangeOnSheet(raw, edited, false, atCol, delta)
			})
			other.modified = true
		}
	}
}

func (s *Sheet) adjustAllFormulasForRowDelete(atRow, count int) {
	if count <= 0 {
		return
	}
	edited := s.Name()
	s.applyFormulaAdjust(func(raw string) string {
		return formula.AdjustFormulaReferencesForRowDeleteOnSheet(raw, edited, true, atRow, count)
	})
	if s.workbook != nil {
		for _, other := range s.workbook.Sheets {
			if other == s {
				continue
			}
			other.applyFormulaAdjust(func(raw string) string {
				return formula.AdjustFormulaReferencesForRowDeleteOnSheet(raw, edited, false, atRow, count)
			})
			other.modified = true
		}
	}
}

func (s *Sheet) adjustAllFormulasForColDelete(atCol, count int) {
	if count <= 0 {
		return
	}
	edited := s.Name()
	s.applyFormulaAdjust(func(raw string) string {
		return formula.AdjustFormulaReferencesForColDeleteOnSheet(raw, edited, true, atCol, count)
	})
	if s.workbook != nil {
		for _, other := range s.workbook.Sheets {
			if other == s {
				continue
			}
			other.applyFormulaAdjust(func(raw string) string {
				return formula.AdjustFormulaReferencesForColDeleteOnSheet(raw, edited, false, atCol, count)
			})
			other.modified = true
		}
	}
}

func (s *Sheet) applyFormulaAdjust(fn func(raw string) string) {
	for pt, c := range s.cells {
		if c == nil || c.Type != cell.TypeFormula {
			continue
		}
		adj := fn(c.RawInput)
		if adj != c.RawInput {
			fmtCopy := c.FormatSpec
			nc := cell.NewCell(adj, fmtCopy)
			nc.Value = c.Value
			nc.EvalGen = c.EvalGen
			nc.Alignment = c.Alignment
			s.cells[pt] = nc
		}
	}
}

func formulaCellMoved(c *cell.Cell, dCol, dRow int) *cell.Cell {
	if c == nil || c.Type != cell.TypeFormula || (dCol == 0 && dRow == 0) {
		return c
	}
	adj := formula.AdjustFormulaReferences(c.RawInput, dCol, dRow)
	if adj == c.RawInput {
		return c
	}
	var fmtCopy *cell.CellFormat
	if c.FormatSpec != nil {
		f := *c.FormatSpec
		fmtCopy = &f
	}
	nc := cell.NewCell(adj, fmtCopy)
	nc.Alignment = c.Alignment
	nc.Value = c.Value
	nc.EvalGen = c.EvalGen
	return nc
}

func (s *Sheet) SortRange(dataR coord.RangeRef, primaryCol int, primaryAsc bool) {
	type rowItem struct {
		rowIdx int
		cells  []*cell.Cell
	}

	var rows []rowItem
	for r := dataR.MinRow(); r <= dataR.MaxRow(); r++ {
		var rowCells []*cell.Cell
		for c := dataR.MinCol(); c <= dataR.MaxCol(); c++ {
			rowCells = append(rowCells, s.cells[CellCoord{Col: c, Row: r}])
		}
		rows = append(rows, rowItem{rowIdx: r, cells: rowCells})
	}

	pOffset := primaryCol - dataR.MinCol()
	sort.SliceStable(rows, func(i, j int) bool {
		var v1, v2 any
		if pOffset >= 0 && pOffset < len(rows[i].cells) && rows[i].cells[pOffset] != nil {
			v1 = rows[i].cells[pOffset].Value
		}
		if pOffset >= 0 && pOffset < len(rows[j].cells) && rows[j].cells[pOffset] != nil {
			v2 = rows[j].cells[pOffset].Value
		}

		if v1 == nil && v2 == nil {
			return false
		}
		if v1 == nil {
			return false
		}
		if v2 == nil {
			return true
		}

		f1, ok1 := toFloat(v1)
		f2, ok2 := toFloat(v2)
		if ok1 && ok2 {
			if primaryAsc {
				return f1 < f2
			}
			return f1 > f2
		}
		if ok1 && !ok2 {
			return primaryAsc
		}
		if !ok1 && ok2 {
			return !primaryAsc
		}

		str1 := strings.ToLower(fmt.Sprintf("%v", v1))
		str2 := strings.ToLower(fmt.Sprintf("%v", v2))
		if primaryAsc {
			return str1 < str2
		}
		return str1 > str2
	})

	for i, rItem := range rows {
		targetR := dataR.MinRow() + i
		for j, cellVal := range rItem.cells {
			targetC := dataR.MinCol() + j
			pt := CellCoord{Col: targetC, Row: targetR}
			if cellVal != nil {
				s.cells[pt] = formulaCellMoved(cellVal, 0, targetR-rItem.rowIdx)
			} else {
				delete(s.cells, pt)
			}
		}
	}

	s.modified = true
	s.TriggerAutoRecalc()
}

func (s *Sheet) SortRangeHorizontal(dataR coord.RangeRef, primaryRow int, primaryAsc bool) {
	type colItem struct {
		colIdx int
		cells  []*cell.Cell
	}

	var columns []colItem
	for c := dataR.MinCol(); c <= dataR.MaxCol(); c++ {
		var colCells []*cell.Cell
		for r := dataR.MinRow(); r <= dataR.MaxRow(); r++ {
			colCells = append(colCells, s.cells[CellCoord{Col: c, Row: r}])
		}
		columns = append(columns, colItem{colIdx: c, cells: colCells})
	}

	pOffset := primaryRow - dataR.MinRow()
	sort.SliceStable(columns, func(i, j int) bool {
		var v1, v2 any
		if pOffset >= 0 && pOffset < len(columns[i].cells) && columns[i].cells[pOffset] != nil {
			v1 = columns[i].cells[pOffset].Value
		}
		if pOffset >= 0 && pOffset < len(columns[j].cells) && columns[j].cells[pOffset] != nil {
			v2 = columns[j].cells[pOffset].Value
		}

		if v1 == nil && v2 == nil {
			return false
		}
		if v1 == nil {
			return false
		}
		if v2 == nil {
			return true
		}

		f1, ok1 := toFloat(v1)
		f2, ok2 := toFloat(v2)
		if ok1 && ok2 {
			if primaryAsc {
				return f1 < f2
			}
			return f1 > f2
		}
		if ok1 && !ok2 {
			return primaryAsc
		}
		if !ok1 && ok2 {
			return !primaryAsc
		}

		str1 := strings.ToLower(fmt.Sprintf("%v", v1))
		str2 := strings.ToLower(fmt.Sprintf("%v", v2))
		if primaryAsc {
			return str1 < str2
		}
		return str1 > str2
	})

	for i, cItem := range columns {
		targetC := dataR.MinCol() + i
		for j, cellVal := range cItem.cells {
			targetR := dataR.MinRow() + j
			pt := CellCoord{Col: targetC, Row: targetR}
			if cellVal != nil {
				s.cells[pt] = formulaCellMoved(cellVal, targetC-cItem.colIdx, 0)
			} else {
				delete(s.cells, pt)
			}
		}
	}

	s.modified = true
	s.TriggerAutoRecalc()
}

func (s *Sheet) FormatRange(r coord.RangeRef, fmtSpec cell.CellFormat) {
	s.modified = true
	minR := r.MinRow()
	maxR := r.MaxRow()
	minC := r.MinCol()
	maxC := r.MaxCol()

	// If whole-column/row reference, cap maxRow/maxCol to populated area to prevent creating millions of empty cells
	if maxR >= 100000 {
		maxR = s.maxPopulatedRow
		if maxR < minR {
			maxR = minR
		}
	}
	if maxC >= 10000 {
		maxC = s.maxPopulatedCol
		if maxC < minC {
			maxC = minC
		}
	}

	for row := minR; row <= maxR; row++ {
		for col := minC; col <= maxC; col++ {
			pt := CellCoord{Col: col, Row: row}
			c := s.cells[pt]
			fmtCopy := fmtSpec
			if c == nil {
				c = cell.NewCell("", &fmtCopy)
				s.cells[pt] = c
			} else {
				c.FormatSpec = &fmtCopy
				c.InvalidateCache()
			}
		}
	}
}

func (s *Sheet) LabelAlignRange(r coord.RangeRef, align cell.Alignment) {
	s.modified = true
	if r.MaxRow()-r.MinRow() > 10000 || r.MaxCol()-r.MinCol() > 1000 {
		for pt, c := range s.cells {
			if r.Contains(pt.Col, pt.Row) && c != nil && c.Type == cell.TypeLabel {
				c.Alignment = align
				if strings.HasPrefix(c.RawInput, "'") || strings.HasPrefix(c.RawInput, `"`) || strings.HasPrefix(c.RawInput, "^") || strings.HasPrefix(c.RawInput, `\`) {
					c.RawInput = string(align) + c.RawInput[1:]
				} else {
					c.RawInput = string(align) + c.RawInput
				}
				c.InvalidateCache()
			}
		}
		return
	}

	for row := r.MinRow(); row <= r.MaxRow(); row++ {
		for col := r.MinCol(); col <= r.MaxCol(); col++ {
			pt := CellCoord{Col: col, Row: row}
			c := s.cells[pt]
			if c != nil && c.Type == cell.TypeLabel {
				c.Alignment = align
				if strings.HasPrefix(c.RawInput, "'") || strings.HasPrefix(c.RawInput, `"`) || strings.HasPrefix(c.RawInput, "^") || strings.HasPrefix(c.RawInput, `\`) {
					c.RawInput = string(align) + c.RawInput[1:]
				} else {
					c.RawInput = string(align) + c.RawInput
				}
				c.InvalidateCache()
			}
		}
	}
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	}
	return 0, false
}

// File I/O (JSON & CSV)

// File I/O (JSON & CSV)

type sheetJSONData struct {
	Name            string         `json:"name,omitempty"`
	Version         string         `json:"version,omitempty"`
	DefaultColWidth int            `json:"default_col_width,omitempty"`
	ColWidths       map[string]int `json:"col_widths,omitempty"`
	GlobalFormat    string         `json:"global_format,omitempty"`
	RecalcMode      string         `json:"recalc_mode,omitempty"`
	NamedRanges     map[string]any `json:"named_ranges,omitempty"`
	FrozenRows      int            `json:"frozen_rows,omitempty"`
	FrozenCols      int            `json:"frozen_cols,omitempty"`
	Graph           *graphJSONData `json:"graph,omitempty"`
	Cells           map[string]any `json:"cells"`
}

type graphJSONData struct {
	Type   string            `json:"type,omitempty"`
	Title  string            `json:"title,omitempty"`
	RangeX string            `json:"range_x,omitempty"`
	Series map[string]string `json:"series,omitempty"`
}

func ReadMaybeGzipFile(filepath string) ([]byte, error) {
	raw, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	if len(raw) >= 2 && raw[0] == 0x1f && raw[1] == 0x8b {
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		return io.ReadAll(gr)
	}
	return raw, nil
}

func WriteMaybeGzipFile(filepath string, data []byte) error {
	lower := strings.ToLower(filepath)
	if strings.HasSuffix(lower, ".hwkz") || strings.HasSuffix(lower, ".gz") || strings.HasSuffix(lower, ".hwk.gz") {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write(data); err != nil {
			return err
		}
		if err := gw.Close(); err != nil {
			return err
		}
		return os.WriteFile(filepath, buf.Bytes(), 0644)
	}
	return os.WriteFile(filepath, data, 0644)
}

func (s *Sheet) buildSheetJSONData() sheetJSONData {
	var colWidthsMap map[string]int
	if len(s.colWidths) > 0 {
		colWidthsMap = make(map[string]int)
		for c, w := range s.colWidths {
			colWidthsMap[strconv.Itoa(c)] = w
		}
	}

	var namedMap map[string]any
	if len(s.namedRanges) > 0 {
		namedMap = encodeNamedMap(s.namedRanges)
	}

	var graphData *graphJSONData
	if s.graph.Type != "" || s.graph.Title != "" || s.graph.RangeX != nil || len(s.graph.Series) > 0 {
		g := graphJSONData{
			Type:   s.graph.Type,
			Title:  s.graph.Title,
			Series: make(map[string]string),
		}
		if s.graph.RangeX != nil {
			g.RangeX = s.graph.RangeX.String()
		}
		for k, v := range s.graph.Series {
			if v != nil {
				g.Series[k] = v.String()
			}
		}
		graphData = &g
	}

	cellsMap := make(map[string]any)
	for pt, c := range s.cells {
		coordStr := fmt.Sprintf("%s%d", coord.ColToLetter(pt.Col), pt.Row+1)

		if c.FormatSpec == nil {
			if num, ok := c.Value.(float64); ok && c.Type == cell.TypeNumber {
				if math.Abs(num-math.Round(num)) < 1e-9 && math.Abs(num) < 1e15 {
					cellsMap[coordStr] = int64(math.Round(num))
				} else {
					cellsMap[coordStr] = num
				}
			} else {
				cellsMap[coordStr] = c.RawInput
			}
		} else {
			cellsMap[coordStr] = map[string]any{
				"raw": c.RawInput,
				"fmt": c.FormatSpec.String(),
			}
		}
	}

	return sheetJSONData{
		Name:            s.Name(),
		Version:         "HasuCalc/2.0",
		DefaultColWidth: s.defaultColWidth,
		ColWidths:       colWidthsMap,
		GlobalFormat:    s.globalFormat.String(),
		RecalcMode:      s.recalcMode,
		NamedRanges:     namedMap,
		FrozenRows:      s.frozenRows,
		FrozenCols:      s.frozenCols,
		Graph:           graphData,
		Cells:           cellsMap,
	}
}

func (s *Sheet) SaveJSON(filepath string) error {
	data := s.buildSheetJSONData()
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	err = WriteMaybeGzipFile(filepath, bytes)
	if err == nil {
		s.modified = false
	}
	return err
}

func loadSheetFromData(data sheetJSONData) *Sheet {
	s := NewSheet()
	if data.Name != "" {
		s.SetName(data.Name)
	}
	if data.DefaultColWidth > 0 {
		s.defaultColWidth = data.DefaultColWidth
	}
	if data.RecalcMode != "" {
		s.recalcMode = data.RecalcMode
	}
	if data.GlobalFormat != "" {
		s.globalFormat = cell.ParseCellFormat(data.GlobalFormat)
	}

	for cStr, w := range data.ColWidths {
		if col, err := strconv.Atoi(cStr); err == nil {
			s.colWidths[col] = w
		}
	}

	if decoded := decodeNamedMap(data.NamedRanges); len(decoded) > 0 {
		s.namedRanges = decoded
	}

	if data.FrozenRows > 0 {
		s.frozenRows = data.FrozenRows
	}
	if data.FrozenCols > 0 {
		s.frozenCols = data.FrozenCols
	}

	if data.Graph != nil {
		s.graph.Type = data.Graph.Type
		s.graph.Title = data.Graph.Title
		if data.Graph.RangeX != "" {
			if r, err := coord.ParseRangeRef(data.Graph.RangeX); err == nil {
				s.graph.RangeX = &r
			}
		}
		for k, vStr := range data.Graph.Series {
			if r, err := coord.ParseRangeRef(vStr); err == nil {
				s.graph.Series[k] = &r
			}
		}
	}

	for coordStr, rawVal := range data.Cells {
		if cr, err := coord.ParseCellRef(coordStr); err == nil {
			var c *cell.Cell
			switch v := rawVal.(type) {
			case string:
				c = cell.NewCell(v, nil)
			case float64:
				if math.Abs(v-math.Round(v)) < 1e-9 && math.Abs(v) < 1e15 {
					c = cell.NewCell(fmt.Sprintf("%d", int64(math.Round(v))), nil)
				} else {
					c = cell.NewCell(strconv.FormatFloat(v, 'g', -1, 64), nil)
				}
				c.Value = v
			case int:
				c = cell.NewCell(strconv.Itoa(v), nil)
				c.Value = float64(v)
			case int64:
				c = cell.NewCell(strconv.FormatInt(v, 10), nil)
				c.Value = float64(v)
			case bool:
				if v {
					c = cell.NewCell("=TRUE()", nil)
				} else {
					c = cell.NewCell("=FALSE()", nil)
				}
			case map[string]any:
				rawStr := ""
				if r, ok := v["raw"].(string); ok {
					rawStr = r
				}
				var fmtSpec *cell.CellFormat
				if fStr, ok := v["fmt"].(string); ok && fStr != "" {
					f := cell.ParseCellFormat(fStr)
					fmtSpec = &f
				} else if fStr, ok := v["format"].(string); ok && fStr != "" {
					f := cell.ParseCellFormat(fStr)
					fmtSpec = &f
				}
				c = cell.NewCell(rawStr, fmtSpec)
				if val, ok := v["value"]; ok && val != nil {
					c.Value = val
				}
			}

			if c != nil {
				s.cells[CellCoord{Col: cr.Col, Row: cr.Row}] = c
				if cr.Col > s.maxPopulatedCol {
					s.maxPopulatedCol = cr.Col
				}
				if cr.Row > s.maxPopulatedRow {
					s.maxPopulatedRow = cr.Row
				}
			}
		}
	}

	s.SetModified(false)

	hasMissing := false
	for _, c := range s.cells {
		if c.Type == cell.TypeFormula && c.Value == nil {
			hasMissing = true
			break
		}
	}
	if hasMissing {
		s.Recalculate()
	}

	return s
}

func LoadSheetJSON(filepath string) (*Sheet, error) {
	bytes, err := ReadMaybeGzipFile(filepath)
	if err != nil {
		return nil, err
	}

	var wbData struct {
		Sheets []sheetJSONData `json:"sheets"`
	}
	if err := json.Unmarshal(bytes, &wbData); err == nil && len(wbData.Sheets) > 0 {
		return loadSheetFromData(wbData.Sheets[0]), nil
	}

	var data sheetJSONData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, err
	}

	return loadSheetFromData(data), nil
}

// ExportCSVRange exports a rectangular range of cells from the sheet as CSV (values only).
func (s *Sheet) ExportCSVRange(filepathStr string, minCol, minRow, maxCol, maxRow int) error {
	if minCol > maxCol {
		minCol, maxCol = maxCol, minCol
	}
	if minRow > maxRow {
		minRow, maxRow = maxRow, minRow
	}

	if dir := filepath.Dir(filepathStr); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	f, err := os.Create(filepathStr)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write UTF-8 BOM (\xEF\xBB\xBF) so Excel recognizes it as UTF-8
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}

	writer := csv.NewWriter(f)
	for r := minRow; r <= maxRow; r++ {
		var rowVals []string
		for c := minCol; c <= maxCol; c++ {
			cellVal := s.cells[CellCoord{Col: c, Row: r}]
			if cellVal != nil && cellVal.Value != nil && cellVal.Type != cell.TypeEmpty {
				valStr := formatMarkdownCellValue(cellVal, s.globalFormat)
				rowVals = append(rowVals, valStr)
			} else {
				rowVals = append(rowVals, "")
			}
		}
		if err := writer.Write(rowVals); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func (s *Sheet) ExportCSV(filepathStr string) error {
	if len(s.cells) == 0 {
		return s.ExportCSVRange(filepathStr, 0, 0, 0, 0)
	}
	minC, minR, maxC, maxR := s.BoundingBox()
	return s.ExportCSVRange(filepathStr, minC, minR, maxC, maxR)
}

func ImportSheetCSV(filepath string) (*Sheet, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	// Strip UTF-8 BOM if present
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	if strings.HasSuffix(strings.ToLower(filepath), ".tsv") {
		reader.Comma = '\t'
	} else if looksLikeTSV(data) {
		reader.Comma = '\t'
	}
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	s := NewSheet()
	for r, row := range records {
		for c, val := range row {
			if strings.TrimSpace(val) != "" {
				s.SetCellInput(c, r, val, nil)
			}
		}
	}
	s.Recalculate()
	s.SetModified(false)
	return s, nil
}

func looksLikeTSV(data []byte) bool {
	sample := data
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	nl := bytes.IndexByte(sample, '\n')
	if nl > 0 {
		sample = sample[:nl]
	}
	tabs := bytes.Count(sample, []byte{'\t'})
	commas := bytes.Count(sample, []byte{','})
	return tabs > 0 && tabs >= commas
}
