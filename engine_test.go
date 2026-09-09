package main

import (
	"archive/zip"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
	"hasucalc/tui"
	"hasucalc/version"
)

// localIOFixture returns a path under an optional local fixture dir if present.
// Fixtures are never shipped; tests Skip when absent.
func localIOFixture(name string) string {
	candidates := []string{
		filepath.Join("testdata", "local", name),
		name,
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func TestCoordinatesAndRanges(t *testing.T) {
	if coord.ColToLetter(0) != "A" {
		t.Errorf("expected A, got %s", coord.ColToLetter(0))
	}
	if coord.ColToLetter(25) != "Z" {
		t.Errorf("expected Z, got %s", coord.ColToLetter(25))
	}
	if coord.ColToLetter(26) != "AA" {
		t.Errorf("expected AA, got %s", coord.ColToLetter(26))
	}

	c, err := coord.ParseCellRef("$B$10")
	if err != nil || c.Col != 1 || c.Row != 9 || !c.ColAbs || !c.RowAbs {
		t.Errorf("ParseCellRef failed: %+v, err: %v", c, err)
	}

	r, err := coord.ParseRangeRef("A1..C3")
	if err != nil || len(r.Cells()) != 9 {
		t.Errorf("ParseRangeRef failed: %+v, err: %v", r, err)
	}
}

func TestJapaneseRunewidthAlignment(t *testing.T) {
	text := "調査開始" // 4 kanji characters = 8 display columns
	w := runewidth.StringWidth(text)
	if w != 8 {
		t.Errorf("expected width 8 for 調査開始, got %d", w)
	}

	aligned := cell.AlignAndPad(text, 12, cell.AlignLeft)
	if runewidth.StringWidth(aligned) != 12 {
		t.Errorf("expected total width 12, got %d", runewidth.StringWidth(aligned))
	}
}

func TestFormulasAndCalculations(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "10", nil)        // A1
	sh.SetCellInput(0, 1, "20", nil)        // A2
	sh.SetCellInput(0, 2, "30", nil)        // A3
	sh.SetCellInput(0, 3, "+A1+A2*A3", nil) // A4 = 10 + 600 = 610
	sh.SetCellInput(0, 4, "@SUM(A1..A3)", nil)
	sh.SetCellInput(0, 5, "@AVG(A1..A3)", nil)
	sh.SetCellInput(0, 6, "@MIN(A1..A3)", nil)
	sh.SetCellInput(0, 7, "@MAX(A1..A3)", nil)
	sh.SetCellInput(0, 8, "@COUNT(A1..A3)", nil)
	sh.SetCellInput(0, 9, "@SQRT(A2*5)", nil) // sqrt(100) = 10

	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 610 {
		t.Errorf("expected A4=610, got %v", sh.GetCellValue(0, 3))
	}
	if v, ok := sh.GetCellValue(0, 4).(float64); !ok || v != 60 {
		t.Errorf("expected A5=60, got %v", sh.GetCellValue(0, 4))
	}
	if v, ok := sh.GetCellValue(0, 5).(float64); !ok || v != 20 {
		t.Errorf("expected A6=20, got %v", sh.GetCellValue(0, 5))
	}
	if v, ok := sh.GetCellValue(0, 6).(float64); !ok || v != 10 {
		t.Errorf("expected A7=10, got %v", sh.GetCellValue(0, 6))
	}
	if v, ok := sh.GetCellValue(0, 7).(float64); !ok || v != 30 {
		t.Errorf("expected A8=30, got %v", sh.GetCellValue(0, 7))
	}
	if v, ok := sh.GetCellValue(0, 8).(float64); !ok || v != 3 {
		t.Errorf("expected A9=3, got %v", sh.GetCellValue(0, 8))
	}
	if v, ok := sh.GetCellValue(0, 9).(float64); !ok || v != 10 {
		t.Errorf("expected A10=10, got %v", sh.GetCellValue(0, 9))
	}
}

func TestModernExcelFormulas(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "10", nil)              // A1
	sh.SetCellInput(0, 1, "20", nil)              // A2
	sh.SetCellInput(0, 2, "30", nil)              // A3
	sh.SetCellInput(1, 0, "=SUM(A1:A3)", nil)     // B1 = 60
	sh.SetCellInput(1, 1, "=AVERAGE(A1:A3)", nil) // B2 = 20
	sh.SetCellInput(1, 2, "=A1+A2*A3", nil)       // B3 = 610
	sh.SetCellInput(1, 3, `=IF(A1=10, "YES", "NO")`, nil)

	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 60 {
		t.Errorf("expected B1=60, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 20 {
		t.Errorf("expected B2=20, got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(1, 2).(float64); !ok || v != 610 {
		t.Errorf("expected B3=610, got %v", sh.GetCellValue(1, 2))
	}
	if v, ok := sh.GetCellValue(1, 3).(string); !ok || v != "YES" {
		t.Errorf("expected B4='YES', got %v", sh.GetCellValue(1, 3))
	}
}

func TestLogicalAndLookupFunctions(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "100", nil)
	sh.SetCellInput(0, 1, `@IF(A1>50, "Big", "Small")`, nil)
	sh.SetCellInput(0, 2, `@IF(A1=100, @TRUE, @FALSE)`, nil)
	sh.SetCellInput(0, 3, `@ISNUMBER(A1)`, nil)
	sh.SetCellInput(0, 4, `@ISSTRING(A2)`, nil)

	if v, ok := sh.GetCellValue(0, 1).(string); !ok || v != "Big" {
		t.Errorf("expected A2='Big', got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(bool); !ok || !v {
		t.Errorf("expected A3=true, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 1 {
		t.Errorf("expected A4=1, got %v", sh.GetCellValue(0, 3))
	}
	if v, ok := sh.GetCellValue(0, 4).(float64); !ok || v != 1 {
		t.Errorf("expected A5=1, got %v", sh.GetCellValue(0, 4))
	}
}

func TestCopyRelativeShift(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "10", nil)    // A1
	sh.SetCellInput(1, 0, "20", nil)    // B1
	sh.SetCellInput(0, 1, "+A1*2", nil) // A2

	fromR, _ := coord.ParseRangeRef("A2..A2")
	toR, _ := coord.ParseRangeRef("B2..B2")
	sh.CopyRange(fromR, toR)

	b2Cell := sh.GetCell(1, 1)
	if b2Cell == nil || b2Cell.Value != 40.0 {
		t.Errorf("expected B2 value 40, got %v", b2Cell)
	}
}

func TestDemoSheetDataAndGraph(t *testing.T) {
	sh := createDemoSheet()

	// Check Tokyo total (Row 1): 6.4 + 10.7 + 4.0 = 21.1
	tot1 := sh.GetCellValue(4, 1)
	if f, ok := tot1.(float64); !ok || math.Abs(f-21.1) > 0.001 {
		t.Errorf("expected Row 1 total 21.1, got %v", tot1)
	}

	// Check Graph config
	if sh.Graph().Type != "LINE" {
		t.Errorf("expected LINE graph, got %s", sh.Graph().Type)
	}
	if sh.Graph().Title != "" {
		t.Errorf("expected empty default title, got %s", sh.Graph().Title)
	}
}

func TestJSONAndCSVPersistence(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "100", nil)
	sh.SetCellInput(0, 1, "200", nil)
	sh.SetCellInput(0, 2, "@SUM(A1..A2)", nil)

	jsonFile := "./test_sheet.json"
	csvFile := "./test_sheet.csv"
	defer os.Remove(jsonFile)
	defer os.Remove(csvFile)

	if err := sh.SaveJSON(jsonFile); err != nil {
		t.Fatalf("SaveJSON error: %v", err)
	}
	loaded, err := sheet.LoadSheetJSON(jsonFile)
	if err != nil {
		t.Fatalf("LoadSheetJSON error: %v", err)
	}
	if v, ok := loaded.GetCellValue(0, 2).(float64); !ok || v != 300 {
		t.Errorf("expected loaded total 300, got %v", loaded.GetCellValue(0, 2))
	}

	if err := sh.ExportCSV(csvFile); err != nil {
		t.Fatalf("ExportCSV error: %v", err)
	}
	imported, err := sheet.ImportSheetCSV(csvFile)
	if err != nil {
		t.Fatalf("ImportSheetCSV error: %v", err)
	}
	if v, ok := imported.GetCellValue(0, 2).(float64); !ok || v != 300 {
		t.Errorf("expected imported total 300, got %v", imported.GetCellValue(0, 2))
	}
}

func TestModernExcelDimensions(t *testing.T) {
	sh := sheet.NewSheet()
	if sh.MaxRows() != 1048576 {
		t.Errorf("expected maxRows=1048576, got %d", sh.MaxRows())
	}
	if sh.MaxCols() != 16384 {
		t.Errorf("expected maxCols=16384, got %d", sh.MaxCols())
	}

	// Test Column XFD (16383)
	xfdCol := coord.LetterToCol("XFD")
	if xfdCol != 16383 {
		t.Errorf("expected XFD=16383, got %d", xfdCol)
	}
	xfdLet := coord.ColToLetter(16383)
	if xfdLet != "XFD" {
		t.Errorf("expected 16383=XFD, got %s", xfdLet)
	}

	// Test setting value at XFD1048576
	cr, err := coord.ParseCellRef("XFD1048576")
	if err != nil {
		t.Fatalf("ParseCellRef XFD1048576 error: %v", err)
	}
	sh.SetCellInput(cr.Col, cr.Row, "999.5", nil)
	val := sh.GetCellValue(cr.Col, cr.Row)
	if f, ok := val.(float64); !ok || f != 999.5 {
		t.Errorf("expected cell value 999.5, got %v", val)
	}
}

func TestCommandPaletteFiltering(t *testing.T) {
	p := tui.NewCommandPalette()
	p.Open()
	if !p.IsActive() {
		t.Errorf("expected palette active")
	}

	evS := tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)
	evU := tcell.NewEventKey(tcell.KeyRune, 'u', tcell.ModNone)
	evM := tcell.NewEventKey(tcell.KeyRune, 'm', tcell.ModNone)
	p.HandleKey(evS)
	p.HandleKey(evU)
	p.HandleKey(evM)

	evEnter := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	handled, item := p.HandleKey(evEnter)
	if !handled || item == nil || item.Action != "doPaletteAutoSum" {
		t.Errorf("expected AutoSum action, got %+v", item)
	}
}

func TestUndoRedoManager(t *testing.T) {
	sh := sheet.NewSheet()
	um := sheet.NewUndoManager(10)

	// Initial state
	sh.SetCellInput(0, 0, "100", nil) // A1 = 100
	sh.SetCellInput(0, 1, "200", nil) // A2 = 200

	// Push state before mutation
	um.Push(sh)

	// Mutate: edit A1, add A3 sum
	sh.SetCellInput(0, 0, "999", nil)
	sh.SetCellInput(0, 2, "=SUM(A1:A2)", nil)
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 1199 {
		t.Errorf("expected A3=1199, got %v", sh.GetCellValue(0, 2))
	}

	// Undo!
	if !um.Undo(sh, nil, nil, nil) {
		t.Fatalf("expected Undo to succeed")
	}
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 100 {
		t.Errorf("expected undone A1=100, got %v", sh.GetCellValue(0, 0))
	}
	if sh.GetCell(0, 2) != nil {
		t.Errorf("expected A3 to be nil after undo, got %v", sh.GetCell(0, 2))
	}

	// Redo!
	if !um.Redo(sh, nil, nil, nil) {
		t.Fatalf("expected Redo to succeed")
	}
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 999 {
		t.Errorf("expected redone A1=999, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 1199 {
		t.Errorf("expected redone A3=1199, got %v", sh.GetCellValue(0, 2))
	}
}

func TestFilePickerNavigation(t *testing.T) {
	fp := tui.NewFilePicker()
	fp.Open(tui.FilePickerModeOpen, ".")
	if !fp.IsActive() {
		t.Errorf("expected FilePicker active")
	}

	// Test Esc key cancel
	evEsc := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
	done, _, canceled := fp.HandleKey(evEsc)
	if !done || !canceled {
		t.Errorf("expected cancel on Esc")
	}

	// Test Save mode
	fp.Open(tui.FilePickerModeSave, "test_out.hwk")
	evEnter := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	done, path, canceled := fp.HandleKey(evEnter)
	if !done || canceled || !strings.HasSuffix(path, "test_out.hwk") {
		t.Errorf("expected save path test_out.hwk, got %s (done=%v, canceled=%v)", path, done, canceled)
	}

	// Test Open mode with .hwk file
	tempDir, err := os.MkdirTemp("", "hasucalc_fp_*")
	if err == nil {
		defer os.RemoveAll(tempDir)
		hwkPath := filepath.Join(tempDir, "mysheet.hwk")
		_ = os.WriteFile(hwkPath, []byte("{}"), 0644)
		fp.Open(tui.FilePickerModeOpen, tempDir)
		evDown := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		fp.HandleKey(evDown) // Move from ".." to "mysheet.hwk"
		done, selectedPath, canceled := fp.HandleKey(evEnter)
		if !done || canceled || !strings.HasSuffix(selectedPath, "mysheet.hwk") {
			t.Errorf("expected open path mysheet.hwk, got %s", selectedPath)
		}

		// Test Overwrite Confirmation on Save
		fp.Open(tui.FilePickerModeSave, filepath.Join(tempDir, "mysheet.hwk"))
		done, path, canceled = fp.HandleKey(evEnter)
		if done {
			t.Errorf("expected prompt for overwrite confirmation, but returned done immediately")
		}
		// Press 'n' to cancel overwrite
		evN := tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone)
		done, _, _ = fp.HandleKey(evN)
		if done {
			t.Errorf("expected to stay in filepicker after pressing 'n'")
		}
		// Press Enter again -> prompt overwrite again -> press 'y' to confirm
		fp.HandleKey(evEnter)
		evY := tcell.NewEventKey(tcell.KeyRune, 'y', tcell.ModNone)
		done, path, canceled = fp.HandleKey(evY)
		if !done || canceled || !strings.HasSuffix(path, "mysheet.hwk") {
			t.Errorf("expected confirmed overwrite path mysheet.hwk, got %s (done=%v, canceled=%v)", path, done, canceled)
		}
	}
}

func TestFilenameValidation(t *testing.T) {
	// Valid filenames
	validNames := []string{"DATA.hwk", "my_sheet_2026", "sales-q1.csv", "日本語ファイル.hwk", "report.123.json"}
	for _, name := range validNames {
		if err := tui.ValidateFilename(name); err != nil {
			t.Errorf("expected valid filename %q, got error: %v", name, err)
		}
	}

	// Invalid filenames
	invalidNames := []string{
		"", "   ", ".", "..", "data/test.hwk", "sheet\\1.csv", "file:name.hwk",
		"star*name", "question?mark", "quote\"name", "less<than", "greater>than", "pipe|name",
		"CON", "con.hwk", "PRN", "aux.csv", "NUL", "com1.hwk", "lpt1.csv",
		"trailing_dot.", "trailing_space ",
	}
	for _, name := range invalidNames {
		if err := tui.ValidateFilename(name); err == nil {
			t.Errorf("expected error for invalid filename %q, but got nil", name)
		}
	}

	// Test FilePicker rejecting invalid filename on Enter
	fp := tui.NewFilePicker()
	fp.Open(tui.FilePickerModeSave, "good_name.hwk")
	// Type invalid characters
	for _, r := range ":invalid/name" {
		fp.HandleKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	done, _, _ := fp.HandleKey(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if done {
		t.Errorf("expected FilePicker to reject invalid filename, but returned done=true")
	}
}

func TestRecalculateRenderCacheInvalidation(t *testing.T) {
	sh := sheet.NewSheet()
	fmtF1 := &cell.CellFormat{Type: cell.FmtFixed, Decimals: 1}

	// Set formula at E26 (col 4, row 25) before data is present
	sh.SetCellInput(4, 25, "((+B26+C26)+D26)", fmtF1)
	cE26 := sh.GetCell(4, 25)
	if cE26 == nil {
		t.Fatalf("expected E26 to exist")
	}

	// Render before inputs are filled -> displays 0.0
	renderedBefore := cE26.Render(8, cell.CellFormat{Type: cell.FmtGeneral})
	if strings.TrimSpace(renderedBefore) != "0.0" {
		t.Errorf("expected 0.0 before, got %q", renderedBefore)
	}

	// Now set data B26=1, C26=5, D26=8
	sh.SetCellInput(1, 25, "1", fmtF1)
	sh.SetCellInput(2, 25, "5", fmtF1)
	sh.SetCellInput(3, 25, "8", fmtF1)

	// Recalculate
	sh.Recalculate()

	// Render after recalculation -> MUST be 14.0!
	renderedAfter := cE26.Render(8, cell.CellFormat{Type: cell.FmtGeneral})
	if strings.TrimSpace(renderedAfter) != "14.0" {
		t.Errorf("expected 14.0 after recalculation, got %q (Value=%v)", renderedAfter, cE26.Value)
	}
}

func TestSheetModifiedFlagTracking(t *testing.T) {
	sh := sheet.NewSheet()
	if sh.IsModified() {
		t.Errorf("new sheet should not be modified initially")
	}

	sh.SetCellInput(0, 0, "123", nil)
	if !sh.IsModified() {
		t.Errorf("sheet should be modified after SetCellInput")
	}

	testFile := "test_modified_tracker.123.json"
	defer os.Remove(testFile)

	if err := sh.SaveJSON(testFile); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	if sh.IsModified() {
		t.Errorf("sheet should not be modified after save")
	}

	sh.ClearCell(0, 0)
	if !sh.IsModified() {
		t.Errorf("sheet should be modified after ClearCell")
	}
}

func TestGenerateUnusedFilenameAndHWK(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hasucalc_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Initially, DATA.hwk does not exist
	fn1 := tui.GenerateUnusedFilename(tempDir, "DATA", ".hwk")
	if fn1 != "DATA.hwk" {
		t.Errorf("expected DATA.hwk, got %s", fn1)
	}

	// Create DATA.hwk
	if err := os.WriteFile(filepath.Join(tempDir, "DATA.hwk"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Now DATA.hwk exists, candidate should be DATA1.hwk
	fn2 := tui.GenerateUnusedFilename(tempDir, "DATA", ".hwk")
	if fn2 != "DATA1.hwk" {
		t.Errorf("expected DATA1.hwk, got %s", fn2)
	}

	// Create DATA1.hwk
	if err := os.WriteFile(filepath.Join(tempDir, "DATA1.hwk"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Now DATA.hwk and DATA1.hwk exist, candidate should be DATA2.hwk
	fn3 := tui.GenerateUnusedFilename(tempDir, "DATA", ".hwk")
	if fn3 != "DATA2.hwk" {
		t.Errorf("expected DATA2.hwk, got %s", fn3)
	}
}

func TestGraphSeriesHeaderLabelExtraction(t *testing.T) {
	sh := sheet.NewSheet()
	// Set header and values in column B (col 1)
	sh.SetCellInput(1, 0, "'Japan", nil) // B1
	sh.SetCellInput(1, 1, "10.5", nil)   // B2
	sh.SetCellInput(1, 2, "20.5", nil)   // B3

	// Set header and values in column C (col 2)
	sh.SetCellInput(2, 0, "'London", nil) // C1
	sh.SetCellInput(2, 1, "15.0", nil)    // C2
	sh.SetCellInput(2, 2, "25.0", nil)    // C3

	// RangeX: A1..A3
	sh.SetCellInput(0, 0, "'Time", nil) // A1
	sh.SetCellInput(0, 1, "10:00", nil) // A2
	sh.SetCellInput(0, 2, "11:00", nil) // A3

	rX, _ := coord.ParseRangeRef("A1..A3")
	rA, _ := coord.ParseRangeRef("B1..B3")
	rB, _ := coord.ParseRangeRef("C1..C3")

	sh.Graph().RangeX = &rX
	sh.Graph().Series["A"] = &rA
	sh.Graph().Series["B"] = &rB

	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	// Inject key event so RenderGraphScreen returns immediately
	s.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)

	// Should render without error and extract Japan and London as labels
	tui.RenderGraphScreen(s, sh, tui.InitStyles())

	// Test STACKED bar graph rendering
	sh.Graph().Type = "STACKED"
	s.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	tui.RenderGraphScreen(s, sh, tui.InitStyles())
}

func TestPasteIntoSelectionRange(t *testing.T) {
	sh := sheet.NewSheet()
	// Set cell D1..D5 = 10, 20, 30, 40, 50
	for r := 0; r < 5; r++ {
		sh.SetCellInput(3, r, fmt.Sprintf("%d", (r+1)*10), nil) // Col D
	}

	// Set cell A1 = "+B1*2"
	sh.SetCellInput(0, 0, "+B1*2", nil)

	// Copy A1..A1 into C1..C5
	fromR, _ := coord.ParseRangeRef("A1..A1")
	toR, _ := coord.ParseRangeRef("C1..C5")

	sh.CopyRange(fromR, toR)

	for r := 0; r < 5; r++ {
		c := sh.GetCell(2, r) // Col C = 2
		if c == nil {
			t.Fatalf("cell C%d is nil", r+1)
		}
		expectedVal := float64((r + 1) * 20)
		if v, ok := c.Value.(float64); !ok || v != expectedVal {
			t.Errorf("C%d expected value %v, got %v (formula: %s)", r+1, expectedVal, c.Value, c.RawInput)
		}
	}
}

func TestLabelSpillingAcrossEmptyCells(t *testing.T) {
	sh := sheet.NewSheet()
	// Column A width = 9, B width = 9, C width = 9
	// Set A1 to a 20-character label
	sh.SetCellInput(0, 0, "'Hello Long Title Here", nil)

	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	app := tui.NewApp(s, sh, "DATA.hwk")
	app.RunOnceForTest() // We can test app drawing
}

func TestInputDirectionalNavigation(t *testing.T) {
	sh := sheet.NewSheet()
	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	app := tui.NewApp(s, sh, "DATA.hwk")

	// 1. Type "100" and press Down -> A1=100, cursor on A2
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	cA1 := sh.GetCell(0, 0)
	if cA1 == nil || cA1.RawInput != "100" {
		t.Fatalf("expected A1 = 100, got %v", cA1)
	}
	col, row := app.GetCursorForTest()
	if col != 0 || row != 1 {
		t.Fatalf("expected cursor at A2 (col 0, row 1), got (%d, %d)", col, row)
	}

	// 2. Type "200" and press Tab -> A2=200, cursor on B2
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))

	cA2 := sh.GetCell(0, 1)
	if cA2 == nil || cA2.RawInput != "200" {
		t.Fatalf("expected A2 = 200, got %v", cA2)
	}
	col, row = app.GetCursorForTest()
	if col != 1 || row != 1 {
		t.Fatalf("expected cursor at B2 (col 1, row 1), got (%d, %d)", col, row)
	}

	// 3. Type "300" and press Shift+Tab -> B2=300, cursor back on A2
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '3', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModShift))

	cB2 := sh.GetCell(1, 1)
	if cB2 == nil || cB2.RawInput != "300" {
		t.Fatalf("expected B2 = 300, got %v", cB2)
	}
	col, row = app.GetCursorForTest()
	if col != 0 || row != 1 {
		t.Fatalf("expected cursor at A2 (col 0, row 1), got (%d, %d)", col, row)
	}

	// 4. Type "400" and press Up -> A2=400, cursor on A1
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '4', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '0', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))

	cA2 = sh.GetCell(0, 1)
	if cA2 == nil || cA2.RawInput != "400" {
		t.Fatalf("expected A2 = 400, got %v", cA2)
	}
	col, row = app.GetCursorForTest()
	if col != 0 || row != 0 {
		t.Fatalf("expected cursor at A1 (col 0, row 0), got (%d, %d)", col, row)
	}
}

func TestGraphMenuSetBNavigation(t *testing.T) {
	sh := sheet.NewSheet()
	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	app := tui.NewApp(s, sh, "DATA.hwk")

	// Press '/' to open menu
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	// Press 'C' to open Chart menu
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModNone))
	// Press 'B' to select Series-B
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'B', tcell.ModNone))

	// Should now be in PROMPT mode for Set Series B range
	// Backspace to clear default "A1..A1"
	for i := 0; i < 6; i++ {
		app.HandleEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	// Type range "B1..B10" and press Enter
	for _, r := range "B1..B10" {
		app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Verify sh.Graph().Series["B"] is set to B1..B10
	sB := sh.Graph().Series["B"]
	if sB == nil {
		t.Fatalf("expected Series B to be configured, got nil")
	}
	if sB.String() != "B1:B10" {
		t.Errorf("expected Series B = B1:B10, got %s", sB.String())
	}

	// Test Graph Status Screen
	s.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	tui.RenderGraphStatusScreen(s, sh, tui.InitStyles())

	// Test opening Set-B again -> prompt should pre-populate with "B1..B10"
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'B', tcell.ModNone))
	// Press Enter without modifying
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if sh.Graph().Series["B"].String() != "B1:B10" {
		t.Errorf("expected Series B to remain B1:B10, got %s", sh.Graph().Series["B"].String())
	}
}

func TestMenuRowCleanRendering(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "'Company name: Amazon", nil)

	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	app := tui.NewApp(s, sh, "DATA.hwk")

	// Press '/' to open Menu
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))

	// Inspect row 1 on simulation screen
	contents, _, _ := s.GetContents()
	var row1Chars []rune
	for x := 0; x < 80; x++ {
		r := contents[1*80+x].Runes
		if len(r) > 0 {
			row1Chars = append(row1Chars, r[0])
		} else {
			row1Chars = append(row1Chars, ' ')
		}
	}
	row1Str := string(row1Chars)

	// In row1Str, "File[F]" is followed by space and "Home[H]", NOT stray junk
	if strings.Contains(row1Str, "File[F] n") || strings.Contains(row1Str, "Company") {
		t.Errorf("stray character found on menu row: %q", row1Str)
	}
	if !strings.Contains(row1Str, "Home[H]") {
		t.Errorf("expected Home[H] on menu row, got: %q", row1Str)
	}
	// Classic 80-column terminals (e.g. Raspberry Pi) must still show Help
	if !strings.Contains(row1Str, "Help[?]") {
		t.Errorf("expected Help[?] on menu row (80-col), got: %q", row1Str)
	}
}

func TestDataSortKeyColumnInteractive(t *testing.T) {
	sh := sheet.NewSheet()
	// Row 0: Tanaka, Dept1, 300
	sh.SetCellInput(0, 0, "Tanaka", nil)
	sh.SetCellInput(1, 0, "Sales", nil)
	sh.SetCellInput(2, 0, "300", nil)

	// Row 1: Suzuki, Dept2, 100
	sh.SetCellInput(0, 1, "Suzuki", nil)
	sh.SetCellInput(1, 1, "Dev", nil)
	sh.SetCellInput(2, 1, "100", nil)

	// Row 2: Sato, Dept1, 500
	sh.SetCellInput(0, 2, "Sato", nil)
	sh.SetCellInput(1, 2, "Sales", nil)
	sh.SetCellInput(2, 2, "500", nil)

	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	app := tui.NewApp(s, sh, "DATA.hwk")

	// 1. Select A1..C3 by holding Shift and moving
	// Cursor starts at A1 (0,0)
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift))

	// Cursor is now at C3, range A1..C3 selected

	// 2. Open /DSA (Data -> Sort -> Ascending)
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))

	// Prompt 1: Range -> pre-populated with A1..C3 -> press Enter
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Prompt 2: Key column -> type "C" (for Sales) -> press Enter
	// Clear default col and type 'C'
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Check that rows are sorted by column C ascending:
	// Row 0 should be Suzuki (100)
	if fmt.Sprintf("%v", sh.GetCellValue(0, 0)) != "Suzuki" || fmt.Sprintf("%v", sh.GetCellValue(2, 0)) != "100" {
		t.Errorf("expected Row 0 = Suzuki (100), got %v, %v", sh.GetCellValue(0, 0), sh.GetCellValue(2, 0))
	}
	// Row 1 should be Tanaka (300)
	if fmt.Sprintf("%v", sh.GetCellValue(0, 1)) != "Tanaka" || fmt.Sprintf("%v", sh.GetCellValue(2, 1)) != "300" {
		t.Errorf("expected Row 1 = Tanaka (300), got %v, %v", sh.GetCellValue(0, 1), sh.GetCellValue(2, 1))
	}
	// Row 2 should be Sato (500)
	if fmt.Sprintf("%v", sh.GetCellValue(0, 2)) != "Sato" || fmt.Sprintf("%v", sh.GetCellValue(2, 2)) != "500" {
		t.Errorf("expected Row 2 = Sato (500), got %v, %v", sh.GetCellValue(0, 2), sh.GetCellValue(2, 2))
	}
}

func TestNamedRangesScreenModal(t *testing.T) {
	sh := sheet.NewSheet()
	rRef, _ := coord.ParseRangeRef("B2..B10")
	sh.NamedRanges()["SALES"] = rRef

	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	// Inject key event so RenderNamedRangesScreen returns immediately
	s.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	tui.RenderNamedRangesScreen(s, sh.NamedRanges(), tui.InitStyles())

	app := tui.NewApp(s, sh, "DATA.hwk")

	// Press F3
	s.InjectKey(tcell.KeyEscape, 0, tcell.ModNone)
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
}

func TestHorizontalSorting(t *testing.T) {
	sh := sheet.NewSheet()
	// Row 0: Month labels: March, January, February in cols 0, 1, 2
	sh.SetCellInput(0, 0, "March", nil)
	sh.SetCellInput(1, 0, "January", nil)
	sh.SetCellInput(2, 0, "February", nil)

	// Row 1: Sales numbers: 300, 100, 200 in cols 0, 1, 2
	sh.SetCellInput(0, 1, "300", nil)
	sh.SetCellInput(1, 1, "100", nil)
	sh.SetCellInput(2, 1, "200", nil)

	// Sort A1..C2 horizontally by Row 2 (Sales) ascending
	rRef, _ := coord.ParseRangeRef("A1..C2")
	sh.SortRangeHorizontal(rRef, 1, true) // Row 1 (0-indexed, row 2)

	// Col 0 should now be January (100)
	if fmt.Sprintf("%v", sh.GetCellValue(0, 0)) != "January" || fmt.Sprintf("%v", sh.GetCellValue(0, 1)) != "100" {
		t.Errorf("expected Col 0 = January (100), got %v, %v", sh.GetCellValue(0, 0), sh.GetCellValue(0, 1))
	}
	// Col 1 should be February (200)
	if fmt.Sprintf("%v", sh.GetCellValue(1, 0)) != "February" || fmt.Sprintf("%v", sh.GetCellValue(1, 1)) != "200" {
		t.Errorf("expected Col 1 = February (200), got %v, %v", sh.GetCellValue(1, 0), sh.GetCellValue(1, 1))
	}
	// Col 2 should be March (300)
	if fmt.Sprintf("%v", sh.GetCellValue(2, 0)) != "March" || fmt.Sprintf("%v", sh.GetCellValue(2, 1)) != "300" {
		t.Errorf("expected Col 2 = March (300), got %v, %v", sh.GetCellValue(2, 0), sh.GetCellValue(2, 1))
	}
}

func TestGraphPromptSelectionPriority(t *testing.T) {
	sh := sheet.NewSheet()
	// Pre-set Series A to A4..G4
	oldRange, _ := coord.ParseRangeRef("A4..G4")
	sh.Graph().Series["A"] = &oldRange

	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	app := tui.NewApp(s, sh, "DATA.hwk")

	// 1. Without selection, /CA should pre-populate old range "A4..G4"
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))
	// Confirm without change
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if sh.Graph().Series["A"].String() != "A4:G4" {
		t.Errorf("expected Series A = A4:G4, got %s", sh.Graph().Series["A"].String())
	}

	// 2. Now select B6..F6 with Shift+Arrows
	// Jump cursor to B6 (col 1, row 5)
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	for _, ch := range "B6" {
		app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Select B6..F6 (4 right arrows with Shift)
	for i := 0; i < 4; i++ {
		app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModShift))
	}

	// Open /CA -> should pre-populate "B6..F6" (NOT "A4..G4")!
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if sh.Graph().Series["A"].String() != "B6:F6" {
		t.Errorf("expected Series A to update to B6:F6 from active selection, got %s", sh.Graph().Series["A"].String())
	}
}

func TestModernExcelFunctions(t *testing.T) {
	sh := sheet.NewSheet()

	// 1. Data Setup:
	// A1..A5: Products: Apple, Banana, Apple, Orange, Banana
	sh.SetCellInput(0, 0, "Apple", nil)
	sh.SetCellInput(0, 1, "Banana", nil)
	sh.SetCellInput(0, 2, "Apple", nil)
	sh.SetCellInput(0, 3, "Orange", nil)
	sh.SetCellInput(0, 4, "Banana", nil)

	// B1..B5: Quantities: 10, 20, 30, 40, 50
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(1, 1, "20", nil)
	sh.SetCellInput(1, 2, "30", nil)
	sh.SetCellInput(1, 3, "40", nil)
	sh.SetCellInput(1, 4, "50", nil)

	// SUMIF Apple -> 10 + 30 = 40
	sh.SetCellInput(2, 0, "=@SUMIF(A1..A5, \"Apple\", B1..B5)", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(2, 0)) != "40" {
		t.Errorf("expected SUMIF Apple = 40, got %v", sh.GetCellValue(2, 0))
	}

	// COUNTIF Banana -> 2
	sh.SetCellInput(2, 1, "=@COUNTIF(A1..A5, \"Banana\")", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(2, 1)) != "2" {
		t.Errorf("expected COUNTIF Banana = 2, got %v", sh.GetCellValue(2, 1))
	}

	// AVERAGEIF Apple -> 20
	sh.SetCellInput(2, 2, "=@AVERAGEIF(A1..A5, \"Apple\", B1..B5)", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(2, 2)) != "20" {
		t.Errorf("expected AVERAGEIF Apple = 20, got %v", sh.GetCellValue(2, 2))
	}

	// SUMIF >20 on B1..B5 -> 30 + 40 + 50 = 120
	sh.SetCellInput(2, 3, "=@SUMIF(B1..B5, \">20\")", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(2, 3)) != "120" {
		t.Errorf("expected SUMIF >20 = 120, got %v", sh.GetCellValue(2, 3))
	}

	// MATCH("Orange", A1..A5, 0) -> 4
	sh.SetCellInput(2, 4, "=@MATCH(\"Orange\", A1..A5, 0)", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(2, 4)) != "4" {
		t.Errorf("expected MATCH Orange = 4, got %v", sh.GetCellValue(2, 4))
	}

	// XLOOKUP("Orange", A1..A5, B1..B5) -> 40
	sh.SetCellInput(3, 0, "=@XLOOKUP(\"Orange\", A1..A5, B1..B5)", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(3, 0)) != "40" {
		t.Errorf("expected XLOOKUP Orange = 40, got %v", sh.GetCellValue(3, 0))
	}

	// XLOOKUP Not Found with custom fallback -> "None"
	sh.SetCellInput(3, 1, "=@XLOOKUP(\"Grape\", A1..A5, B1..B5, \"None\")", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(3, 1)) != "None" {
		t.Errorf("expected XLOOKUP fallback = None, got %v", sh.GetCellValue(3, 1))
	}

	// IFERROR(10/0, 999) -> 999
	sh.SetCellInput(3, 2, "=@IFERROR(10/0, 999)", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(3, 2)) != "999" {
		t.Errorf("expected IFERROR 10/0 = 999, got %v", sh.GetCellValue(3, 2))
	}

	// TRIM("  hello   world  ") -> "hello world"
	sh.SetCellInput(3, 3, "=@TRIM(\"  hello   world  \")", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(3, 3)) != "hello world" {
		t.Errorf("expected TRIM = hello world, got %v", sh.GetCellValue(3, 3))
	}

	// SUBSTITUTE("2024-08-25", "-", "/") -> "2024/08/25"
	sh.SetCellInput(3, 4, "=@SUBSTITUTE(\"2024-08-25\", \"-\", \"/\")", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(3, 4)) != "2024/08/25" {
		t.Errorf("expected SUBSTITUTE = 2024/08/25, got %v", sh.GetCellValue(3, 4))
	}

	// UPPER & LOWER
	sh.SetCellInput(4, 0, "=@UPPER(\"hasucalc\")", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(4, 0)) != "HASUCALC" {
		t.Errorf("expected UPPER = HASUCALC, got %v", sh.GetCellValue(4, 0))
	}
	sh.SetCellInput(4, 1, "=@LOWER(\"EXCEL\")", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(4, 1)) != "excel" {
		t.Errorf("expected LOWER = excel, got %v", sh.GetCellValue(4, 1))
	}

	// TEXTJOIN("-", 1, "A", "", "B", "C") -> "A-B-C"
	sh.SetCellInput(4, 2, "=@TEXTJOIN(\"-\", 1, \"A\", \"\", \"B\", \"C\")", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(4, 2)) != "A-B-C" {
		t.Errorf("expected TEXTJOIN = A-B-C, got %v", sh.GetCellValue(4, 2))
	}

	// WEEKDAY(DATE(2026, 8, 25)) -> 3 (Tuesday, return_type 1: Sunday=1, Monday=2, Tuesday=3)
	sh.SetCellInput(4, 3, "=@WEEKDAY(@DATE(2026, 8, 25))", nil)
	if fmt.Sprintf("%v", sh.GetCellValue(4, 3)) != "3" {
		t.Errorf("expected WEEKDAY = 3, got %v", sh.GetCellValue(4, 3))
	}
}

func TestFunctionPickerModal(t *testing.T) {
	sh := sheet.NewSheet()
	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	// Inject key event: Select first function (@SUM) with Enter
	s.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	sn := tui.ShowFunctionPickerModal(s, tui.InitStyles())
	if sn != "@SUM(" {
		t.Errorf("expected @SUM(, got %s", sn)
	}

	app := tui.NewApp(s, sh, "DATA.hwk")

	// Trigger /IF -> opens Function Picker
	s.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'I', tcell.ModNone))
	app.HandleEventForTest(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
}

func TestXLSXImporterWithRealFiles(t *testing.T) {
	// Optional local fixtures only. Sample workbooks are not shipped.
	files := []string{
		"sample.xlsx",
		"sample-5.xlsx",
	}
	found := 0
	for _, name := range files {
		fn := localIOFixture(name)
		if fn == "" {
			t.Logf("Skipping %s (no local fixture)", name)
			continue
		}
		found++
		sheetNames, err := sheet.GetXLSXSheetList(fn)
		if err != nil {
			t.Errorf("%s: %v", fn, err)
			continue
		}
		if len(sheetNames) == 0 {
			t.Errorf("%s returned 0 sheets", fn)
		}
		t.Logf("File %s has %d sheets: %v", fn, len(sheetNames), sheetNames)

		sh, err := sheet.ImportXLSX(fn, 0)
		if err != nil {
			t.Errorf("failed to import sheet 0 from %s: %v", fn, err)
		}
		if sh == nil {
			t.Errorf("imported sheet is nil for %s", fn)
		}
	}
	if found == 0 {
		t.Skip("no optional XLSX fixtures locally")
	}
}

func TestSheetPickerModal(t *testing.T) {
	s := tcell.NewSimulationScreen("")
	s.Init()
	s.SetSize(80, 25)
	defer s.Fini()

	sheets := []string{"Summary", "Sales 2024", "Expenses"}
	s.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	idx := tui.ShowSheetPickerModal(s, "test.xlsx", sheets, tui.InitStyles())
	if idx != 0 {
		t.Errorf("expected selected sheet idx = 0, got %d", idx)
	}
}

func TestComprehensiveFunctions(t *testing.T) {
	sh := sheet.NewSheet()

	// Setup data table in A1:D5
	// A: 10, 20, 30, 40, 50
	// B: "North", "South", "North", "South", "North"
	// C: 100, 200, 300, 400, 500
	// D: "Apple", "Banana", "Apple", "Orange", "Banana"
	for i, v := range []string{"10", "20", "30", "40", "50"} {
		sh.SetCellInput(0, i, v, nil)
	}
	for i, v := range []string{"North", "South", "North", "South", "North"} {
		sh.SetCellInput(1, i, "'"+v, nil)
	}
	for i, v := range []string{"100", "200", "300", "400", "500"} {
		sh.SetCellInput(2, i, v, nil)
	}
	for i, v := range []string{"Apple", "Banana", "Apple", "Orange", "Banana"} {
		sh.SetCellInput(3, i, "'"+v, nil)
	}

	testCases := []struct {
		formula  string
		expected any
		isFloat  bool
		tol      float64
	}{
		// Math & Trigonometry
		{formula: `=PRODUCT(2, 3, 4)`, expected: 24.0, isFloat: true},
		{formula: `=POWER(2, 8)`, expected: 256.0, isFloat: true},
		{formula: `=EXP(0)`, expected: 1.0, isFloat: true},
		{formula: `=LN(1)`, expected: 0.0, isFloat: true},
		{formula: `=LOG10(100)`, expected: 2.0, isFloat: true},
		{formula: `=LOG(8, 2)`, expected: 3.0, isFloat: true},
		{formula: `=QUOTIENT(10, 3)`, expected: 3.0, isFloat: true},
		{formula: `=SIGN(-42)`, expected: -1.0, isFloat: true},
		{formula: `=FACT(5)`, expected: 120.0, isFloat: true},
		{formula: `=GCD(24, 36)`, expected: 12.0, isFloat: true},
		{formula: `=LCM(4, 6)`, expected: 12.0, isFloat: true},
		{formula: `=COMBIN(5, 2)`, expected: 10.0, isFloat: true},
		{formula: `=PERMUT(5, 2)`, expected: 20.0, isFloat: true},
		{formula: `=ROUNDUP(3.1415, 2)`, expected: 3.15, isFloat: true},
		{formula: `=ROUNDDOWN(3.1499, 2)`, expected: 3.14, isFloat: true},
		{formula: `=TRUNC(3.89)`, expected: 3.0, isFloat: true},
		{formula: `=CEILING(4.2, 1)`, expected: 5.0, isFloat: true},
		{formula: `=FLOOR(4.8, 1)`, expected: 4.0, isFloat: true},
		{formula: `=MROUND(10, 3)`, expected: 9.0, isFloat: true},
		{formula: `=DEGREES(PI())`, expected: 180.0, isFloat: true, tol: 1e-6},
		{formula: `=RADIANS(180)`, expected: math.Pi, isFloat: true, tol: 1e-6},
		{formula: `=SIN(0)`, expected: 0.0, isFloat: true},
		{formula: `=COS(0)`, expected: 1.0, isFloat: true},
		{formula: `=TAN(0)`, expected: 0.0, isFloat: true},

		// Multi-criteria Aggregation
		{formula: `=SUMIFS(C1:C5, B1:B5, "North")`, expected: 900.0, isFloat: true},                 // 100+300+500 = 900
		{formula: `=SUMIFS(C1:C5, B1:B5, "North", D1:D5, "Apple")`, expected: 400.0, isFloat: true}, // 100+300 = 400
		{formula: `=COUNTIFS(B1:B5, "North")`, expected: 3.0, isFloat: true},
		{formula: `=COUNTIFS(B1:B5, "North", D1:D5, "Apple")`, expected: 2.0, isFloat: true},
		{formula: `=AVERAGEIFS(C1:C5, B1:B5, "North")`, expected: 300.0, isFloat: true},
		{formula: `=MINIFS(C1:C5, B1:B5, "North")`, expected: 100.0, isFloat: true},
		{formula: `=MAXIFS(C1:C5, B1:B5, "North")`, expected: 500.0, isFloat: true},
		{formula: `=SUMPRODUCT(A1:A3, C1:C3)`, expected: 14000.0, isFloat: true}, // 10*100 + 20*200 + 30*300 = 1000+4000+9000 = 14000
		{formula: `=SUBTOTAL(9, A1:A5)`, expected: 150.0, isFloat: true},

		// Statistical
		{formula: `=COUNTA(A1:A5)`, expected: 5.0, isFloat: true},
		{formula: `=MEDIAN(1, 2, 10, 11, 12)`, expected: 10.0, isFloat: true},
		{formula: `=MODE(1, 2, 2, 3, 4)`, expected: 2.0, isFloat: true},
		{formula: `=LARGE(A1:A5, 1)`, expected: 50.0, isFloat: true},
		{formula: `=SMALL(A1:A5, 1)`, expected: 10.0, isFloat: true},
		{formula: `=PERCENTILE(A1:A5, 0.5)`, expected: 30.0, isFloat: true},
		{formula: `=QUARTILE(A1:A5, 2)`, expected: 30.0, isFloat: true},
		{formula: `=STDEV.P(10, 20, 30)`, expected: math.Sqrt(200.0 / 3.0), isFloat: true, tol: 1e-4},
		{formula: `=VAR.P(10, 20, 30)`, expected: 200.0 / 3.0, isFloat: true, tol: 1e-4},

		// Logic & Info
		{formula: `=IFS(1=2, "NO", 2=2, "YES", 1=1, "FALLBACK")`, expected: "YES"},
		{formula: `=SWITCH(2, 1, "ONE", 2, "TWO", 3, "THREE", "OTHER")`, expected: "TWO"},
		{formula: `=XOR(1, 0, 0)`, expected: 1.0, isFloat: true},
		{formula: `=XOR(1, 1, 0)`, expected: 0.0, isFloat: true},
		{formula: `=ISTEXT("Hello")`, expected: 1.0, isFloat: true},
		{formula: `=ISTEXT(123)`, expected: 0.0, isFloat: true},
		{formula: `=ISNONTEXT(123)`, expected: 1.0, isFloat: true},
		{formula: `=ISEVEN(4)`, expected: 1.0, isFloat: true},
		{formula: `=ISODD(5)`, expected: 1.0, isFloat: true},
		{formula: `=N(TRUE())`, expected: 1.0, isFloat: true},
		{formula: `=T("World")`, expected: "World"},
		{formula: `=TYPE("Text")`, expected: 2.0, isFloat: true},
		{formula: `=TYPE(100)`, expected: 1.0, isFloat: true},

		// Text
		{formula: `=REPLACE("abcdef", 2, 3, "XYZ")`, expected: "aXYZef"},
		{formula: `=REPT("Ha", 3)`, expected: "HaHaHa"},
		{formula: `=EXACT("abc", "abc")`, expected: 1.0, isFloat: true},
		{formula: `=EXACT("abc", "ABC")`, expected: 0.0, isFloat: true},
		{formula: `=CHAR(65)`, expected: "A"},
		{formula: `=CODE("A")`, expected: 65.0, isFloat: true},
		{formula: `=NUMBERVALUE("1,234.56")`, expected: 1234.56, isFloat: true},
		{formula: `=TEXTBEFORE("apple-pie", "-")`, expected: "apple"},
		{formula: `=TEXTAFTER("apple-pie", "-")`, expected: "pie"},

		// Date & Time
		{formula: `=TIME(12, 0, 0)`, expected: 0.5, isFloat: true},
		{formula: `=HOUR(0.5)`, expected: 12.0, isFloat: true},
		{formula: `=MINUTE(TIME(14, 30, 0))`, expected: 30.0, isFloat: true},
		{formula: `=DAYS(DATE(2026, 1, 10), DATE(2026, 1, 1))`, expected: 9.0, isFloat: true},
		{formula: `=DAYS360(DATE(2026, 1, 1), DATE(2026, 2, 1))`, expected: 30.0, isFloat: true},
		{formula: `=NETWORKDAYS(DATE(2026, 1, 5), DATE(2026, 1, 9))`, expected: 5.0, isFloat: true}, // Mon to Fri = 5 days

		// Financial
		{formula: `=ROUND(PMT(0.05/12, 60, -10000), 2)`, expected: 188.71, isFloat: true, tol: 0.1},
		{formula: `=ROUND(SLN(10000, 1000, 5), 2)`, expected: 1800.0, isFloat: true},
		{formula: `=ROUND(SYD(10000, 1000, 5, 1), 2)`, expected: 3000.0, isFloat: true},

		// Lookup & Reference
		{formula: `=LOOKUP(25, A1:A5, B1:B5)`, expected: "South"}, // 25 is between 20 (South) and 30 (North) -> South
		{formula: `=XMATCH("Banana", D1:D5)`, expected: 2.0, isFloat: true},
		{formula: `=OFFSET(A1, 2, 0)`, expected: 30.0, isFloat: true}, // A1 + 2 rows = A3 = 30
	}

	for _, tc := range testCases {
		sh.SetCellInput(5, 0, tc.formula, nil)
		val := sh.GetCellValue(5, 0)
		if tc.isFloat {
			valF, ok := val.(float64)
			expF := tc.expected.(float64)
			tol := tc.tol
			if tol == 0 {
				tol = 1e-6
			}
			if !ok || math.Abs(valF-expF) > tol {
				t.Errorf("Formula %s: expected %v, got %v (%T)", tc.formula, expF, val, val)
			}
		} else {
			if val != tc.expected {
				t.Errorf("Formula %s: expected %v, got %v (%T)", tc.formula, tc.expected, val, val)
			}
		}
	}
}

func TestCrossSheetReferences(t *testing.T) {
	wb := sheet.NewWorkbook("TestWorkbook")
	shYear := wb.Sheets[0]
	shYear.SetName("Year")

	shJan := wb.AddSheet("January")
	shSales := wb.AddSheet("Sales 2024")

	// Set data in Year
	shYear.SetCellInput(1, 3, "42", nil) // B4 = 42
	shYear.SetCellInput(0, 0, "10", nil) // A1 = 10
	shYear.SetCellInput(0, 1, "20", nil) // A2 = 20
	shYear.SetCellInput(0, 2, "30", nil) // A3 = 30

	// Set data in Sales 2024
	shSales.SetCellInput(2, 4, "500", nil) // C5 = 500

	// Test direct cell reference =Year!B4 in January!A1
	shJan.SetCellInput(0, 0, "=Year!B4 * 2", nil)
	valA1 := shJan.GetCellValue(0, 0)
	if valA1 != 84.0 {
		t.Errorf("January!A1 expected 84.0, got %v (%T)", valA1, valA1)
	}

	// Test cross-sheet range reference =SUM(Year!A1:A3) in January!A2
	shJan.SetCellInput(0, 1, "=SUM(Year!A1:A3)", nil)
	valA2 := shJan.GetCellValue(0, 1)
	if valA2 != 60.0 {
		t.Errorf("January!A2 expected 60.0, got %v (%T)", valA2, valA2)
	}

	// Test quoted sheet name ='Sales 2024'!C5 + 10 in January!A3
	shJan.SetCellInput(0, 2, "='Sales 2024'!C5 + 10", nil)
	valA3 := shJan.GetCellValue(0, 2)
	if valA3 != 510.0 {
		t.Errorf("January!A3 expected 510.0, got %v (%T)", valA3, valA3)
	}

	// Test dynamic recalculation across sheets
	shYear.SetCellInput(1, 3, "100", nil) // Update B4 = 100
	wb.RecalculateAll()
	valA1Updated := shJan.GetCellValue(0, 0)
	if valA1Updated != 200.0 {
		t.Errorf("After updating Year!B4, January!A1 expected 200.0, got %v", valA1Updated)
	}
}

func TestSample5CalendarMultiSheet(t *testing.T) {
	sample5Path := localIOFixture("sample-5.xlsx")
	if sample5Path == "" {
		t.Skip("optional fixture sample-5.xlsx not found locally")
	}
	wb, err := sheet.ImportXLSXWorkbook(sample5Path)
	if err != nil {
		t.Fatalf("Failed to import sample-5.xlsx workbook: %v", err)
	}

	t.Logf("sample-5.xlsx sheets (%d): %v", len(wb.Sheets), wb.SheetNames())
	if len(wb.Sheets) < 13 {
		t.Fatalf("Expected at least 13 sheets in sample-5.xlsx, got %d", len(wb.Sheets))
	}

	shJan := wb.GetSheet("January")
	if shJan == nil {
		t.Fatalf("Expected 'January' sheet in sample-5.xlsx")
	}

	// Check B4 cell (formula =Year!B4)
	c := shJan.GetCell(1, 3) // col B (1), row 4 (3)
	if c == nil {
		t.Fatalf("Expected cell B4 to exist in January sheet")
	}

	if c.Value == cell.ErrLotus || c.Value == cell.ErrCirc {
		t.Errorf("January!B4 returned error: %v (input: %s)", c.Value, c.RawInput)
	}

	// Verify all populated cells in January sheet do not have ERR
	errCount := 0
	for pt := range shJan.GetPopulatedCoords() {
		cellPtr := shJan.GetCell(pt.Col, pt.Row)
		if cellPtr != nil && cellPtr.Value == cell.ErrLotus {
			t.Logf("Error in cell %s%d: formula=%s, value=%v", coord.ColToLetter(pt.Col), pt.Row+1, cellPtr.RawInput, cellPtr.Value)
			errCount++
		}
	}
	if errCount > 0 {
		t.Errorf("Found %d ERR cells in January sheet", errCount)
	}
}

func TestSampleDatabaseXLSX(t *testing.T) {
	t0 := time.Now()
	sampleDBPath := localIOFixture("sampleデータベース.xlsx")
	if sampleDBPath == "" {
		t.Skip("optional fixture sampleデータベース.xlsx not found locally")
	}
	sheetNames, err := sheet.GetXLSXSheetList(sampleDBPath)
	if err != nil {
		t.Fatalf("Failed to get sheet list: %v", err)
	}
	t.Logf("sampleデータベース.xlsx sheets (%d): %v (took %v)", len(sheetNames), sheetNames, time.Since(t0))

	t1 := time.Now()
	wb, err := sheet.ImportXLSXWorkbook(sampleDBPath)
	if err != nil {
		t.Fatalf("Failed to import workbook: %v", err)
	}
	t.Logf("ImportXLSXWorkbook took %v", time.Since(t1))
	for _, s := range wb.Sheets {
		t.Logf("Sheet: %s, Populated cells: %d, maxPopRow: %d, maxPopCol: %d", s.Name(), len(s.GetPopulatedCoords()), s.MaxPopulatedRow(), s.MaxPopulatedCol())
	}

	sh0 := wb.Sheets[0]
	cellB3 := sh0.GetCell(1, 2)
	cellC3 := sh0.GetCell(2, 2)
	t.Logf("B3: raw=%s, val=%v", cellB3.RawInput, cellB3.Value)
	t.Logf("C3: raw=%s, val=%v", cellC3.RawInput, cellC3.Value)

	// Save all 3 sheets into sampleデータベース.hwk
	sampleDBHwkPath := filepath.Join(t.TempDir(), "sampleデータベース.hwk")
	if err := wb.SaveJSON(sampleDBHwkPath); err != nil {
		t.Fatalf("Failed to save multi-sheet hwk: %v", err)
	}

	// Re-load and verify all 3 sheets
	loadedWb, err := sheet.LoadWorkbookJSON(sampleDBHwkPath)
	if err != nil {
		t.Fatalf("Failed to load multi-sheet hwk: %v", err)
	}
	if len(loadedWb.Sheets) != 3 {
		t.Fatalf("Expected 3 sheets in loaded hwk, got %d", len(loadedWb.Sheets))
	}

	loadedB3 := loadedWb.Sheets[0].GetCell(1, 2)
	if loadedB3 == nil || loadedB3.Value == cell.ErrLotus {
		t.Errorf("Loaded B3 evaluated to error: %v", loadedB3.Value)
	} else {
		t.Logf("Loaded B3: %v", loadedB3.Value)
	}
}

func TestPerpetualCalendarODS(t *testing.T) {
	odsPath := localIOFixture("Perpetual-Calendar-Version-26-0.ods")
	if odsPath == "" {
		t.Skip("optional fixture Perpetual-Calendar-Version-26-0.ods not found locally")
	}
	wb, err := sheet.ImportODSWorkbook(odsPath)
	if err != nil {
		t.Fatalf("Failed to import ODS: %v", err)
	}
	t.Logf("ODS sheets (%d): %v", len(wb.Sheets), wb.SheetNames())

	sh1 := wb.GetSheet("1")
	if sh1 == nil {
		t.Fatalf("Sheet '1' not found")
	}
	shD := wb.GetSheet("D_Sheet")
	shSettings := wb.GetSheet("Settings")
	if shSettings != nil {
		cellE18 := shSettings.GetCell(4, 17)
		if cellE18 != nil {
			t.Logf("Settings!E18: raw=%q, val=%v", cellE18.RawInput, cellE18.Value)
		} else {
			t.Logf("Settings!E18 is nil")
		}
	}
	if shD != nil {
		evD := formula.NewEvaluator(shD)
		vFirstDay := evD.EvaluateCellAt(3, 5, "=FirstDay")
		t.Logf("At D_Sheet!D6, FirstDay = %v", vFirstDay)
		vYear := evD.EvaluateCellAt(3, 5, "=YEAR(FirstDay)")
		t.Logf("At D_Sheet!D6, YEAR(FirstDay) = %v", vYear)
		vDate := evD.EvaluateCellAt(3, 5, "=DATE(YEAR(FirstDay);MONTH(FirstDay)+C6-1; 1)")
		t.Logf("At D_Sheet!D6, DATE(...) = %v", vDate)
	}
	sh1.Recalculate()
	cellB4 := sh1.GetCell(1, 3)
	if cellB4 == nil || cellB4.Value == cell.ErrLotus || cellB4.Value == cell.ErrCirc {
		t.Errorf("Expected valid B4 calculation, got %v", cellB4.Value)
	} else {
		t.Logf("1!B4 calculated value: %v", cellB4.Value)
	}

	cellB6 := sh1.GetCell(1, 5)
	if cellB6 == nil || cellB6.Value != 1.0 {
		t.Errorf("Expected 1!B6 = 1, got %v", cellB6.Value)
	}

	cellD4 := sh1.GetCell(3, 3)
	if cellD4 == nil || cellD4.Value != "Holy Family School" {
		t.Errorf("Expected 1!D4 = 'Holy Family School', got %v", cellD4.Value)
	}

	shYL := wb.GetSheet("YearLandscape")
	if shYL != nil {
		shYL.Recalculate()
		for r := 3; r <= 18; r++ {
			rowStr := fmt.Sprintf("YearLandscape Row %2d: ", r+1)
			hasNonEmpty := false
			for c := 1; c <= 7; c++ {
				cellObj := shYL.GetCell(c, r)
				if cellObj != nil {
					if cellObj.Value == cell.ErrLotus || cellObj.Value == cell.ErrCirc {
						t.Errorf("Cell in YearLandscape (%s%d) has error: %v (raw: %s)", coord.ColToLetter(c), r+1, cellObj.Value, cellObj.RawInput)
					}
					rowStr += fmt.Sprintf("[%s%d: %v] ", coord.ColToLetter(c), r+1, cellObj.Value)
					hasNonEmpty = true
				}
			}
			if hasNonEmpty {
				t.Logf("%s", rowStr)
			}
		}
	}
}

func TestExcelCardXLSM(t *testing.T) {
	xlsmPath := localIOFixture("ExcelCardDT_flw.xlsm")
	if xlsmPath == "" {
		t.Skip("optional fixture ExcelCardDT_flw.xlsm not found locally")
	}
	wb, err := sheet.ImportXLSXWorkbook(xlsmPath)
	if err != nil {
		t.Fatalf("Failed to import XLSM: %v", err)
	}
	t.Logf("XLSM sheets (%d): %v", len(wb.Sheets), wb.SheetNames())

	shSys := wb.GetSheet("システム設定")
	if shSys == nil {
		t.Fatalf("Sheet 'システム設定' not found")
	}

	cellC10 := shSys.GetCell(2, 9) // Col C (2), Row 10 (9)
	if cellC10 != nil {
		t.Logf("システム設定!C10: raw=%q, type=%v, val=%v", cellC10.RawInput, cellC10.Type, cellC10.Value)
		if cellC10.Value == cell.ErrLotus {
			t.Errorf("Expected valid text in C10, got error: %v", cellC10.Value)
		}
	} else {
		t.Errorf("システム設定!C10 is nil")
	}

	// Also check ExcelCardDT_ownr.xlsm
	ownrPath := localIOFixture("ExcelCardDT_ownr.xlsm")
	if ownrPath != "" {
		wbOwnr, err := sheet.ImportXLSXWorkbook(ownrPath)
		if err == nil {
			t.Logf("ExcelCardDT_ownr sheets (%d): %v", len(wbOwnr.Sheets), wbOwnr.SheetNames())
			shSys2 := wbOwnr.GetSheet("システム設定")
			if shSys2 != nil {
				c10 := shSys2.GetCell(2, 9)
				if c10 != nil {
					t.Logf("ExcelCardDT_ownr システム設定!C10: val=%v", c10.Value)
					if c10.Value == cell.ErrLotus {
						t.Errorf("Expected valid text in ownr C10, got error: %v", c10.Value)
					}
				}
			}
		}
	}
}

func TestGraphPNGExport(t *testing.T) {
	tempDir := t.TempDir()
	sh := sheet.NewSheet()

	// Fill sample data
	products := []string{"Phone", "Tablet", "Monitor", "Printer", "Laptop"}
	sales := []float64{1247, 850, 620, 430, 290}
	profit := []float64{386, 269, 191, 125, 71}

	sh.SetCellInput(0, 0, "'Product", nil)
	sh.SetCellInput(1, 0, "'Sales", nil)
	sh.SetCellInput(2, 0, "'Profit", nil)

	for i := range products {
		sh.SetCellInput(0, i+1, "'"+products[i], nil)
		sh.SetCellInput(1, i+1, fmt.Sprintf("%.0f", sales[i]), nil)
		sh.SetCellInput(2, i+1, fmt.Sprintf("%.0f", profit[i]), nil)
	}

	sh.Graph().RangeX = &coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: len(products)}}
	sh.Graph().Series["A"] = &coord.RangeRef{Start: coord.CellRef{Col: 1, Row: 0}, End: coord.CellRef{Col: 1, Row: len(sales)}}
	sh.Graph().Series["B"] = &coord.RangeRef{Start: coord.CellRef{Col: 2, Row: 0}, End: coord.CellRef{Col: 2, Row: len(profit)}}

	graphTypes := []string{"LINE", "BAR", "STACKED", "PIE"}
	for _, gt := range graphTypes {
		sh.Graph().Type = gt
		sh.Graph().Title = fmt.Sprintf("Document Test - %s Graph", gt)
		outPath := filepath.Join(tempDir, fmt.Sprintf("test_%s.png", gt))

		err := tui.ExportGraphPNG(sh, outPath, 1280, 720)
		if err != nil {
			t.Fatalf("Failed to export %s PNG: %v", gt, err)
		}

		fi, err := os.Stat(outPath)
		if err != nil || fi.Size() < 500 {
			t.Fatalf("Exported PNG file %s is missing or too small (size: %d bytes)", outPath, fi.Size())
		}

		// Verify PNG signature (0x89 'P' 'N' 'G')
		header := make([]byte, 8)
		f, err := os.Open(outPath)
		if err != nil {
			t.Fatalf("Failed to open exported PNG: %v", err)
		}
		_, _ = f.Read(header)
		_ = f.Close()

		if string(header[1:4]) != "PNG" {
			t.Errorf("File %s does not have valid PNG signature, got %q", outPath, header)
		}
		t.Logf("Successfully verified %s graph PNG (%s, size: %d bytes)", gt, filepath.Base(outPath), fi.Size())
	}

	// Test DefaultPNGFilename naming logic
	sh.Graph().Type = "PIE"
	if fn := tui.DefaultPNGFilename(sh, "engraph.hwk"); fn != "engraph_PIE.png" {
		t.Errorf("expected 'engraph_PIE.png', got %q", fn)
	}
	sh.Graph().Type = "BAR"
	if fn := tui.DefaultPNGFilename(sh, "/path/to/monthly_report.xlsx"); fn != "monthly_report_BAR.png" {
		t.Errorf("expected 'monthly_report_BAR.png', got %q", fn)
	}
	sh.Graph().Type = "LINE"
	if fn := tui.DefaultPNGFilename(sh, ""); fn != "Sheet1_LINE.png" {
		t.Errorf("expected 'Sheet1_LINE.png', got %q", fn)
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	t.Logf("Before Reset - HeapAlloc: %d KB, HeapInuse: %d KB, Sys: %d KB", m.HeapAlloc/1024, m.HeapInuse/1024, m.Sys/1024)

	tui.ResetScriptFontCache()
	runtime.ReadMemStats(&m)
	t.Logf("After Reset - HeapAlloc: %d KB, HeapInuse: %d KB, Sys: %d KB", m.HeapAlloc/1024, m.HeapInuse/1024, m.Sys/1024)
}

func TestCompactHWKAndTransparentGzip(t *testing.T) {
	tempDir := t.TempDir()
	wb := sheet.NewWorkbook("TestCompact")
	sh := wb.GetActiveSheet()

	// Fill sample data with numbers, labels, formulas, and formats
	sh.SetCellInput(0, 0, "'Product", nil)
	sh.SetCellInput(1, 0, "'Price", nil)
	sh.SetCellInput(2, 0, "'Qty", nil)
	sh.SetCellInput(3, 0, "'Total", nil)

	fmtCur := cell.ParseCellFormat("(C2)")
	sh.SetCellInput(0, 1, "'Apple", nil)
	sh.SetCellInput(1, 1, "120.5", &fmtCur)
	sh.SetCellInput(2, 1, "10", nil)
	sh.SetCellInput(3, 1, "=B2*C2", &fmtCur)

	sh.SetCellInput(0, 2, "'Orange", nil)
	sh.SetCellInput(1, 2, "85", &fmtCur)
	sh.SetCellInput(2, 2, "20", nil)
	sh.SetCellInput(3, 2, "=B3*C3", &fmtCur)

	sh.SetCellInput(0, 3, "'Grand Total", nil)
	sh.SetCellInput(3, 3, "=SUM(D2:D3)", &fmtCur)

	wb.RecalculateAll()

	// 1. Test uncompressed compact .hwk
	hwkPath := filepath.Join(tempDir, "compact.hwk")
	if err := wb.SaveJSON(hwkPath); err != nil {
		t.Fatalf("Failed to save compact .hwk: %v", err)
	}

	hwkBytes, err := os.ReadFile(hwkPath)
	if err != nil {
		t.Fatalf("Failed to read compact .hwk: %v", err)
	}
	t.Logf("Compact .hwk content:\n%s", string(hwkBytes))

	// Verify uncompressed compact JSON structure
	if strings.Contains(string(hwkBytes), `"type": "LABEL"`) || strings.Contains(string(hwkBytes), `"align": "'"`) {
		t.Errorf("Compact .hwk should not contain redundant type/align fields")
	}

	// 2. Load uncompressed .hwk and verify data
	loadedWb, err := sheet.LoadWorkbookJSON(hwkPath)
	if err != nil {
		t.Fatalf("Failed to load compact .hwk: %v", err)
	}
	loadedSh := loadedWb.GetActiveSheet()
	if loadedSh.GetCellValue(0, 1) != "Apple" {
		t.Errorf("Expected 'Apple', got %v", loadedSh.GetCellValue(0, 1))
	}
	if math.Abs(loadedSh.GetCellValue(3, 3).(float64)-2905.0) > 1e-4 {
		t.Errorf("Expected Grand Total 2905, got %v", loadedSh.GetCellValue(3, 3))
	}

	// 3. Test transparent gzip compressed .hwkz
	hwkzPath := filepath.Join(tempDir, "compact.hwkz")
	if err := wb.SaveJSON(hwkzPath); err != nil {
		t.Fatalf("Failed to save compressed .hwkz: %v", err)
	}

	// Verify .hwkz has gzip magic bytes (0x1f, 0x8b)
	hwkzBytes, err := os.ReadFile(hwkzPath)
	if err != nil || len(hwkzBytes) < 2 || hwkzBytes[0] != 0x1f || hwkzBytes[1] != 0x8b {
		t.Fatalf("Expected .hwkz to have GZIP magic bytes 0x1f 0x8b, got %x", hwkzBytes[:2])
	}
	t.Logf("Compressed .hwkz size: %d bytes (vs uncompressed %d bytes)", len(hwkzBytes), len(hwkBytes))

	// 4. Transparently load compressed .hwkz
	loadedHwkzWb, err := sheet.LoadWorkbookJSON(hwkzPath)
	if err != nil {
		t.Fatalf("Failed to load compressed .hwkz: %v", err)
	}
	loadedHwkzSh := loadedHwkzWb.GetActiveSheet()
	if math.Abs(loadedHwkzSh.GetCellValue(3, 3).(float64)-2905.0) > 1e-4 {
		t.Errorf("Expected Grand Total 2905 in .hwkz, got %v", loadedHwkzSh.GetCellValue(3, 3))
	}
	// 5. Test NormalizeHwkSaveFilename
	testCases := []struct {
		in       string
		expected string
	}{
		{"mega.hwkz", "mega.hwkz"},
		{"mega.hwk.gz", "mega.hwk.gz"},
		{"mega.gz", "mega.hwk.gz"},
		{"mega.hwk", "mega.hwk"},
		{"mega", "mega.hwk"},
		{"mega.xlsx", "mega.hwk"},
		{"mega.ods", "mega.hwk"},
		{"mega.csv", "mega.hwk"},
	}
	for _, tc := range testCases {
		res := tui.NormalizeHwkSaveFilename(tc.in)
		if res != tc.expected {
			t.Errorf("NormalizeHwkSaveFilename(%q) = %q, expected %q", tc.in, res, tc.expected)
		}
	}
}

func TestFilePickerCursorEditing(t *testing.T) {
	fp := tui.NewFilePicker()
	fp.Open(tui.FilePickerModeSave, "DATA.hwk")

	// Helper to send key
	sendKey := func(k tcell.Key, r rune) {
		ev := tcell.NewEventKey(k, r, tcell.ModNone)
		fp.HandleKey(ev)
	}

	// Initial buffer should be DATA.hwk and cursor at end (index 8)
	if string(fp.InputBuffer()) != "DATA.hwk" {
		t.Fatalf("Expected initial buffer DATA.hwk, got %q", string(fp.InputBuffer()))
	}
	if fp.CursorPos() != 8 {
		t.Fatalf("Expected cursor pos 8, got %d", fp.CursorPos())
	}

	// Move Left 4 times (past .hwk)
	for i := 0; i < 4; i++ {
		sendKey(tcell.KeyLeft, 0)
	}
	if fp.CursorPos() != 4 {
		t.Fatalf("Expected cursor pos 4 (before .hwk), got %d", fp.CursorPos())
	}

	// Insert '_2026' in the middle
	for _, r := range "_2026" {
		sendKey(tcell.KeyRune, r)
	}
	if string(fp.InputBuffer()) != "DATA_2026.hwk" {
		t.Errorf("Expected DATA_2026.hwk, got %q", string(fp.InputBuffer()))
	}

	// Jump to Home (Ctrl+A)
	sendKey(tcell.KeyCtrlA, 0)
	if fp.CursorPos() != 0 {
		t.Fatalf("Expected cursor pos 0 after Home, got %d", fp.CursorPos())
	}

	// Delete 4 chars (remove 'DATA')
	for i := 0; i < 4; i++ {
		sendKey(tcell.KeyDelete, 0)
	}
	if string(fp.InputBuffer()) != "_2026.hwk" {
		t.Errorf("Expected _2026.hwk after deleting DATA, got %q", string(fp.InputBuffer()))
	}

	// Insert 'Report' at beginning
	for _, r := range "Report" {
		sendKey(tcell.KeyRune, r)
	}
	if string(fp.InputBuffer()) != "Report_2026.hwk" {
		t.Errorf("Expected Report_2026.hwk, got %q", string(fp.InputBuffer()))
	}

	// Jump to End and backspace 3 chars (remove 'hwk')
	sendKey(tcell.KeyEnd, 0)
	for i := 0; i < 3; i++ {
		sendKey(tcell.KeyBackspace, 0)
	}
	// Insert 'hwkz'
	for _, r := range "hwkz" {
		sendKey(tcell.KeyRune, r)
	}
	if string(fp.InputBuffer()) != "Report_2026.hwkz" {
		t.Errorf("Expected Report_2026.hwkz, got %q", string(fp.InputBuffer()))
	}
}

func TestMultiScriptTypographyAndRendering(t *testing.T) {
	// 1. Test script detection
	if tui.DetectScript('A') != tui.ScriptDefault {
		t.Errorf("expected ScriptDefault for 'A'")
	}
	if tui.DetectScript('漢') != tui.ScriptCJK {
		t.Errorf("expected ScriptCJK for '漢'")
	}
	if tui.DetectScript('م') != tui.ScriptArabic {
		t.Errorf("expected ScriptArabic for 'م'")
	}
	if tui.DetectScript('क') != tui.ScriptDevanagari {
		t.Errorf("expected ScriptDevanagari for 'क'")
	}
	if tui.DetectScript('ก') != tui.ScriptThai {
		t.Errorf("expected ScriptThai for 'ก'")
	}
	if tui.DetectScript('ב') != tui.ScriptHebrew {
		t.Errorf("expected ScriptHebrew for 'ב'")
	}
	if tui.DetectScript('ক') != tui.ScriptBengali {
		t.Errorf("expected ScriptBengali for 'ক'")
	}

	// 2. Test Arabic Reshaping
	sampleArabic := "مرحبا"
	reshaped := tui.ReshapeArabic(sampleArabic)
	if reshaped == sampleArabic {
		t.Errorf("Expected Arabic to be reshaped with contextual glyphs")
	}
	t.Logf("Arabic %s -> Reshaped: %s (runes: %x)", sampleArabic, reshaped, []rune(reshaped))

	// 3. Test Multi-Script PNG Graph Export with Arabic, Hindi, Thai, Hebrew, CJK titles
	tempDir := t.TempDir()
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "'Quarter", nil)
	sh.SetCellInput(0, 1, "'Q1 (مبيعات)", nil) // Arabic "Sales"
	sh.SetCellInput(0, 2, "'Q2 (बिक्री)", nil) // Hindi "Sales"
	sh.SetCellInput(0, 3, "'Q3 (ยอดขาย)", nil) // Thai "Sales"
	sh.SetCellInput(0, 4, "'Q4 (売上/매출)", nil)  // Japanese / Korean

	sh.SetCellInput(1, 0, "Revenue", nil)
	sh.SetCellInput(1, 1, "120", nil)
	sh.SetCellInput(1, 2, "180", nil)
	sh.SetCellInput(1, 3, "240", nil)
	sh.SetCellInput(1, 4, "300", nil)

	rX, _ := coord.ParseRangeRef("A2..A5")
	rA, _ := coord.ParseRangeRef("B2..B5")
	sh.Graph().Type = "BAR"
	sh.Graph().Title = "Global Multi-Script: مبيعات / बिक्री / ยอดขาย / 売上"
	sh.Graph().RangeX = &rX
	sh.Graph().Series["A"] = &rA

	outPath := filepath.Join(tempDir, "global_multilingual_test.png")
	err := tui.ExportGraphPNG(sh, outPath, 1280, 720)
	if err != nil {
		t.Fatalf("Failed to export multi-script graph PNG: %v", err)
	}

	fi, err := os.Stat(outPath)
	if err != nil || fi.Size() < 500 {
		t.Fatalf("Multi-script PNG is missing or too small (size: %d bytes)", fi.Size())
	}
	t.Logf("Successfully exported global multi-script graph PNG (size: %d bytes)", fi.Size())
}

func TestCrossSheetCopyPaste(t *testing.T) {
	wb := sheet.NewWorkbook("test_cross_copy.hwk")
	sh1 := wb.GetActiveSheet()
	sh1.SetName("SourceSheet")

	sh2 := wb.AddSheet("TargetSheet")

	// Set data in SourceSheet
	sh1.SetCellInput(0, 0, "100", nil)          // A1
	sh1.SetCellInput(1, 0, "200", nil)          // B1
	sh1.SetCellInput(2, 0, "=A1+B1", nil)       // C1
	sh1.SetCellInput(3, 0, "'Hello Cross", nil) // D1
	wb.RecalculateAll()

	// Copy A1..D1 from SourceSheet to B5..E5 in TargetSheet
	srcR, _ := coord.ParseRangeRef("A1..D1")
	destR, _ := coord.ParseRangeRef("B5..E5")
	sh2.CopyRangeFrom(sh1, srcR, destR)
	wb.RecalculateAll()

	// Verify TargetSheet cells
	b5 := sh2.GetCell(1, 4) // B5
	if b5 == nil || b5.Value != float64(100) {
		t.Fatalf("Expected B5 in TargetSheet to be 100, got %v", b5)
	}

	c5 := sh2.GetCell(2, 4) // C5
	if c5 == nil || c5.Value != float64(200) {
		t.Fatalf("Expected C5 in TargetSheet to be 200, got %v", c5)
	}

	d5 := sh2.GetCell(3, 4) // D5
	if d5 == nil || (d5.RawInput != "=(B5+C5)" && d5.RawInput != "=B5+C5") || d5.Value != float64(300) {
		t.Fatalf("Expected D5 in TargetSheet to be =(B5+C5) with value 300, got raw=%s, val=%v", d5.RawInput, d5.Value)
	}

	e5 := sh2.GetCell(4, 4) // E5
	if e5 == nil || e5.RawInput != "'Hello Cross" {
		t.Fatalf("Expected E5 in TargetSheet to be 'Hello Cross, got %v", e5)
	}
}

func TestMultilingualWorkbookCrossSheetCopyPaste(t *testing.T) {
	wb, err := sheet.LoadWorkbookJSON("demo_multilingual_all.hwk")
	if err != nil {
		t.Fatalf("Failed to load demo_multilingual_all.hwk: %v", err)
	}

	targetSh := wb.AddSheet("VerificationTarget")

	for i, sh := range wb.Sheets {
		if sh.Name() == "VerificationTarget" {
			continue
		}

		srcR, _ := coord.ParseRangeRef("A3..F8")
		destR, _ := coord.ParseRangeRef(fmt.Sprintf("A%d..F%d", 1+i*8, 6+i*8))

		targetSh.CopyRangeFrom(sh, srcR, destR)
		wb.RecalculateAll()

		// Verify label cell
		labelCell := targetSh.GetCell(0, i*8)
		if labelCell == nil || labelCell.Type != cell.TypeLabel {
			t.Fatalf("Sheet [%s]: Header at A%d is nil or not label", sh.Name(), 1+i*8)
		}

		// Verify formula cell
		fCell := targetSh.GetCell(3, 1+i*8)
		if fCell == nil || fCell.Type != cell.TypeFormula || fCell.Value == nil {
			t.Fatalf("Sheet [%s]: Formula cell at D%d is nil or invalid: %v", sh.Name(), 2+i*8, fCell)
		}
	}
}

func TestDebugArabicToJapaneseCopy(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb, err := sheet.LoadWorkbookJSON("demo_multilingual_all.hwk")
	if err != nil {
		t.Fatal(err)
	}

	sh := wb.GetActiveSheet()
	app := tui.NewApp(simScreen, sh, "demo_multilingual_all.hwk")

	// Switch to Sheet 5 (Arabic: index 4)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModCtrl))

	// Move to A4 (row 3: 'هاتف ذكي)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	// Press Ctrl+C
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))

	// Switch back to Sheet 1 (Japanese: index 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModCtrl))

	// Move to A15 (row 14)
	for i := 0; i < 14; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}

	// Press Ctrl+V
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	// Check screen contents at row 14
	gridY := 3 + 14
	var lineRunes []rune
	for x := 0; x < 80; x++ {
		mainc, _, _, _ := simScreen.GetContent(x, gridY)
		lineRunes = append(lineRunes, mainc)
	}
	t.Logf("Line %d on screen: %q", gridY, string(lineRunes))
}

func TestPasteLinkCrossSheet(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb, err := sheet.LoadWorkbookJSON("demo_multilingual_all.hwk")
	if err != nil {
		t.Fatal(err)
	}

	app := tui.NewApp(simScreen, wb.GetActiveSheet(), "demo_multilingual_all.hwk")

	// 1. Switch to Sheet 5 (Arabic: index 4) using Ctrl+PgDn 4 times
	for i := 0; i < 4; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModCtrl))
	}

	// 2. Move to B4 (col 1, row 3: 2500)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	// 3. Copy (Ctrl+C)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))

	// 4. Switch back to Sheet 1 (Japanese: index 0) using Ctrl+PgUp 4 times
	for i := 0; i < 4; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModCtrl))
	}

	// 5. Move to B15 (col 1, row 14)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	for i := 0; i < 14; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}

	// 6. Paste Link (Ctrl+L)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModCtrl))

	// 7. Verify Japanese Sheet B15 has link formula "='تقرير_المبيعات'!B4" and value 2500
	jpSh := wb.Sheets[0]
	linkedCell := jpSh.GetCell(1, 14)
	if linkedCell == nil {
		t.Fatalf("Expected linked cell at B15 in Japanese sheet, got nil")
	}
	if linkedCell.Type != cell.TypeFormula {
		t.Fatalf("Expected cell type Formula, got %v (raw=%s)", linkedCell.Type, linkedCell.RawInput)
	}
	expectedFormula := "=تقرير_المبيعات!B4"
	if linkedCell.RawInput != expectedFormula {
		t.Fatalf("Expected formula %q, got %q", expectedFormula, linkedCell.RawInput)
	}
	if linkedCell.Value != float64(2500) {
		t.Fatalf("Expected linked cell value 2500, got %v", linkedCell.Value)
	}

	// 8. Modify Arabic sheet B4 to 9999, recalculate workbook and check Japanese B15 updates!
	arSh := wb.Sheets[4]
	arSh.SetCellInput(1, 3, "9999", nil)
	wb.RecalculateAll()

	if linkedCell.Value != float64(9999) {
		t.Fatalf("Expected linked cell value to update to 9999, got %v", linkedCell.Value)
	}
}

func TestPasteLinkViaSlashMenu(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("UTF-8")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb, err := sheet.LoadWorkbookJSON("demo_multilingual_all.hwk")
	if err != nil {
		t.Fatal(err)
	}

	app := tui.NewApp(simScreen, wb.GetActiveSheet(), "demo_multilingual_all.hwk")

	// 1. Copy A4 on Sheet 1 (Japanese: 'たこ)
	for i := 0; i < 3; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))

	// 2. Switch to Sheet 2 (English: QuarterlySales)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyPgDn, 0, tcell.ModCtrl))

	// 3. Move to A20
	for i := 0; i < 19; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}

	// 4. Open Slash Menu, navigate to Home -> Paste-Special -> Link (/HSL)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'H', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'L', tcell.ModNone))

	// 5. Verify linked cell at A20 in Sheet 2
	sh2 := wb.Sheets[1]
	linkedCell := sh2.GetCell(0, 19)
	if linkedCell == nil {
		t.Fatalf("Expected linked cell at A20 in Sheet 2, got nil")
	}
	if linkedCell.Type != cell.TypeFormula {
		t.Fatalf("Expected cell type Formula, got %v", linkedCell.Type)
	}
	expectedFormula := "=商品売上!A4"
	if linkedCell.RawInput != expectedFormula {
		t.Fatalf("Expected formula %q, got %q", expectedFormula, linkedCell.RawInput)
	}
	if linkedCell.Value != "たこ" {
		t.Fatalf("Expected linked cell value 'たこ', got %v", linkedCell.Value)
	}
}

func TestFilePickerTypeAhead(t *testing.T) {
	fp := tui.NewFilePicker()
	fp.Open(tui.FilePickerModeOpen, "")

	// Simulate pressing 'd', then 'e', then 'm', then 'o', then '_'
	// In the workspace directory, we have demo_arabic.hwk, demo_chinese.hwk, demo_multilingual_all.hwk, etc.
	fp.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))
	fp.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	fp.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'm', tcell.ModNone))
	fp.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	fp.HandleKey(tcell.NewEventKey(tcell.KeyRune, '_', tcell.ModNone))
	fp.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'm', tcell.ModNone)) // demo_m... -> demo_multilingual_all.hwk

	selected := fp.GetSelectedEntryForTest()
	if !strings.HasPrefix(selected.Name, "demo_m") {
		t.Fatalf("Expected selection starting with 'demo_m', got %q", selected.Name)
	}
}

func TestExportXLSXAndODS(t *testing.T) {
	wb, err := sheet.LoadWorkbookJSON("demo_multilingual_all.hwk")
	if err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()

	// 1. Export XLSX
	xlsxPath := filepath.Join(tmpDir, "export_test.xlsx")
	if err := wb.ExportXLSX(xlsxPath); err != nil {
		t.Fatalf("ExportXLSX failed: %v", err)
	}
	zr, err := zip.OpenReader(xlsxPath)
	if err != nil {
		t.Fatalf("Failed to open exported XLSX zip: %v", err)
	}
	fileMap := make(map[string]bool)
	for _, f := range zr.File {
		fileMap[f.Name] = true
	}
	zr.Close()

	if !fileMap["[Content_Types].xml"] || !fileMap["xl/workbook.xml"] || !fileMap["xl/worksheets/sheet1.xml"] {
		t.Fatalf("Exported XLSX missing required OpenXML files: %v", fileMap)
	}

	// 2. Export ODS
	odsPath := filepath.Join(tmpDir, "export_test.ods")
	if err := wb.ExportODS(odsPath); err != nil {
		t.Fatalf("ExportODS failed: %v", err)
	}
	zrOds, err := zip.OpenReader(odsPath)
	if err != nil {
		t.Fatalf("Failed to open exported ODS zip: %v", err)
	}
	odsMap := make(map[string]bool)
	for _, f := range zrOds.File {
		odsMap[f.Name] = true
	}
	zrOds.Close()

	if !odsMap["mimetype"] || !odsMap["META-INF/manifest.xml"] || !odsMap["content.xml"] {
		t.Fatalf("Exported ODS missing required OpenDocument files: %v", odsMap)
	}

	// 3. Verify Round-trip Import of XLSX
	importedXLSX, err := sheet.ImportXLSXWorkbook(xlsxPath)
	if err != nil {
		t.Fatalf("Failed to import exported XLSX: %v", err)
	}
	if len(importedXLSX.Sheets) != len(wb.Sheets) {
		t.Fatalf("Expected %d sheets in imported XLSX, got %d", len(wb.Sheets), len(importedXLSX.Sheets))
	}

	// 4. Verify Round-trip Import of ODS
	importedODS, err := sheet.ImportODSWorkbook(odsPath)
	if err != nil {
		t.Fatalf("Failed to import exported ODS: %v", err)
	}
	if len(importedODS.Sheets) != len(wb.Sheets) {
		t.Fatalf("Expected %d sheets in imported ODS, got %d", len(wb.Sheets), len(importedODS.Sheets))
	}
}

func TestExportMarkdownRange(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "'商品名", nil)
	s.SetCellInput(1, 0, "'単価", nil)
	s.SetCellInput(2, 0, "'数量", nil)
	s.SetCellInput(3, 0, "'合計", nil)

	s.SetCellInput(0, 1, "'りんご", nil)
	s.SetCellInput(1, 1, "120", nil)
	s.SetCellInput(2, 1, "5", nil)
	s.SetCellInput(3, 1, "=B2*C2", nil)

	s.SetCellInput(0, 2, "'みかん | 特選", nil)
	s.SetCellInput(1, 2, "80", nil)
	s.SetCellInput(2, 2, "10", nil)
	s.SetCellInput(3, 2, "=B3*C3", nil)

	s.Recalculate()

	// Render Markdown Table
	md := s.RenderMarkdownTableRange(0, 0, 3, 2)
	lines := strings.Split(strings.TrimSpace(md), "\n")
	if len(lines) != 4 {
		t.Fatalf("Expected 4 lines (header, separator, 2 data rows), got %d:\n%s", len(lines), md)
	}

	// Line 1: Header
	if !strings.Contains(lines[0], "商品名") || !strings.Contains(lines[0], "合計") {
		t.Errorf("Header row missing columns: %s", lines[0])
	}
	// Line 2: Separators
	if !strings.Contains(lines[1], "---") || !strings.Contains(lines[1], "|") {
		t.Errorf("Separator row invalid: %s", lines[1])
	}
	// Line 3: Data row 1
	if !strings.Contains(lines[2], "りんご") || !strings.Contains(lines[2], "600") {
		t.Errorf("Data row 1 invalid: %s", lines[2])
	}
	// Line 4: Data row 2 (with escaped pipe)
	if !strings.Contains(lines[3], `みかん \| 特選`) || !strings.Contains(lines[3], "800") {
		t.Errorf("Data row 2 with escaped pipe invalid: %s", lines[3])
	}

	// Test ExportMarkdownRange to file
	tmpPath := filepath.Join(t.TempDir(), "output.md")
	if err := s.ExportMarkdownRange(tmpPath, 0, 0, 3, 2); err != nil {
		t.Fatalf("ExportMarkdownRange failed: %v", err)
	}
	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != md {
		t.Fatalf("File content does not match rendered markdown:\n%s", string(content))
	}
}

func TestExportMarkdownViaSlashMenu(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb, err := sheet.LoadWorkbookJSON("demo_multilingual_all.hwk")
	if err != nil {
		t.Fatal(err)
	}

	app := tui.NewApp(simScreen, wb.GetActiveSheet(), "demo_multilingual_all.hwk")

	// 1. Open Slash Menu -> File -> Export -> Markdown -> Range (/FXMR)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'M', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone))

	// 2. We should be in PROMPT mode for range
	// Press Enter to confirm range
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// 3. We should now be in FILE PICKER mode
	// Press Esc to dismiss
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
}

func TestExportRangeValuesOnlyAllFormats(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "10", nil)
	s.SetCellInput(1, 0, "20", nil)
	s.SetCellInput(2, 0, "=A1+B1", nil)

	s.SetCellInput(0, 1, "100", nil)
	s.SetCellInput(1, 1, "200", nil)
	s.SetCellInput(2, 1, "=A2+B2", nil)

	s.Recalculate()

	tmpDir := t.TempDir()

	// 1. Export CSV Range A1..C2
	csvPath := filepath.Join(tmpDir, "range.csv")
	if err := s.ExportCSVRange(csvPath, 0, 0, 2, 1); err != nil {
		t.Fatalf("ExportCSVRange failed: %v", err)
	}
	csvBytes, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	csvStr := string(csvBytes)
	if !strings.Contains(csvStr, "10,20,30") || !strings.Contains(csvStr, "100,200,300") {
		t.Fatalf("Unexpected CSV range export output:\n%s", csvStr)
	}

	// 2. Export XLSX Range B1..C2 (Values only, no formulas)
	xlsxPath := filepath.Join(tmpDir, "range.xlsx")
	if err := s.ExportXLSXRange(xlsxPath, 1, 0, 2, 1); err != nil {
		t.Fatalf("ExportXLSXRange failed: %v", err)
	}
	zr, err := zip.OpenReader(xlsxPath)
	if err != nil {
		t.Fatalf("Failed to open exported XLSX: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, f.UncompressedSize64)
			rc.Read(buf)
			rc.Close()
			xmlStr := string(buf)
			if strings.Contains(xmlStr, "<f>") {
				t.Fatalf("XLSX Range export must NOT contain <f> formulas, but found formulas:\n%s", xmlStr)
			}
			if !strings.Contains(xmlStr, "<v>30</v>") || !strings.Contains(xmlStr, "<v>300</v>") {
				t.Fatalf("XLSX Range export missing evaluated values:\n%s", xmlStr)
			}
		}
	}
	zr.Close()

	// 3. Export ODS Range B1..C2 (Values only, no formulas)
	odsPath := filepath.Join(tmpDir, "range.ods")
	if err := s.ExportODSRange(odsPath, 1, 0, 2, 1); err != nil {
		t.Fatalf("ExportODSRange failed: %v", err)
	}
	zrOds, err := zip.OpenReader(odsPath)
	if err != nil {
		t.Fatalf("Failed to open exported ODS: %v", err)
	}
	for _, f := range zrOds.File {
		if f.Name == "content.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, f.UncompressedSize64)
			rc.Read(buf)
			rc.Close()
			xmlStr := string(buf)
			if strings.Contains(xmlStr, "table:formula") {
				t.Fatalf("ODS Range export must NOT contain table:formula, but found formulas:\n%s", xmlStr)
			}
			if !strings.Contains(xmlStr, "30") || !strings.Contains(xmlStr, "300") {
				t.Fatalf("ODS Range export missing evaluated values:\n%s", xmlStr)
			}
		}
	}
	zrOds.Close()
}

func TestFullVsRangeExportIsolation(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb, err := sheet.LoadWorkbookJSON("demo_multilingual_all.hwk")
	if err != nil {
		t.Fatal(err)
	}

	app := tui.NewApp(simScreen, wb.GetActiveSheet(), "demo_multilingual_all.hwk")

	// 1. Perform Range Export via /FXER (File -> Export -> Excel -> Range)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'E', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone))

	// Enter range A1..B3
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
	for _, ch := range "A1..B3" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Save to range_test_iso.xlsx
	defer os.Remove("range_test_iso.xlsx")
	defer os.Remove("full_test_iso.xlsx")

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
	for _, ch := range "range_test_iso.xlsx" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// 2. Perform Full Workbook Export via /FXES (File -> Export -> Excel -> Sheet)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'X', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'E', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone))

	// Save directly to full_test_iso.xlsx
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
	for _, ch := range "full_test_iso.xlsx" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// 3. Verify that full_test_iso.xlsx contains ALL 9 sheets!
	importedFull, err := sheet.ImportXLSXWorkbook("full_test_iso.xlsx")
	if err != nil {
		t.Fatalf("Failed to import full export: %v", err)
	}
	if len(importedFull.Sheets) != len(wb.Sheets) {
		t.Fatalf("Expected full export to have all %d sheets, but got %d (range bleed detected)", len(wb.Sheets), len(importedFull.Sheets))
	}
}

func TestExportImportCSVWithUTF8BOM(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "商品名", nil)
	s.SetCellInput(1, 0, "価格", nil)
	s.SetCellInput(2, 0, "備考", nil)

	s.SetCellInput(0, 1, "リンゴ 🍎", nil)
	s.SetCellInput(1, 1, "120", nil)
	s.SetCellInput(2, 1, "青森産 (Grüße / Привет)", nil)

	s.Recalculate()

	tmpPath := filepath.Join(t.TempDir(), "test_bom.csv")
	if err := s.ExportCSV(tmpPath); err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}

	rawBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Verify that file starts with UTF-8 BOM (\xEF\xBB\xBF)
	if len(rawBytes) < 3 || rawBytes[0] != 0xEF || rawBytes[1] != 0xBB || rawBytes[2] != 0xBF {
		t.Fatalf("Expected CSV file to start with UTF-8 BOM (0xEF, 0xBB, 0xBF), but got: %x", rawBytes[:3])
	}

	// 2. Import back into HasuCalc and verify BOM is stripped and values are intact
	imported, err := sheet.ImportSheetCSV(tmpPath)
	if err != nil {
		t.Fatalf("ImportSheetCSV failed: %v", err)
	}

	c00 := imported.GetCell(0, 0)
	if c00 == nil || c00.Value != "商品名" {
		t.Fatalf("Expected A1 to be '商品名' without BOM corruption, got: %v", c00)
	}

	c21 := imported.GetCell(2, 1)
	if c21 == nil || c21.Value != "青森産 (Grüße / Привет)" {
		t.Fatalf("Expected C2 to preserve multilingual text, got: %v", c21)
	}
}

func TestGotoCellRangeSheetName(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_goto.hwk")
	sh1 := wb.GetActiveSheet()
	sh1.SetName("Sheet1")
	sh2 := wb.AddSheet("Sheet2")

	sh1.SetCellInput(0, 0, "100", nil)
	rNamed, _ := coord.ParseCellRef("D15")
	sh1.SetNamedRange("TargetCell", rNamed)

	app := tui.NewApp(simScreen, sh1, "test_goto.hwk")

	// 1. Jump to Cell C10 via F5
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	for _, ch := range "C10" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.CursorCol() != 2 || app.CursorRow() != 9 {
		t.Fatalf("Expected cursor at C10 (col 2, row 9), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}

	// 2. Jump to Range B5..D12 via Ctrl+G
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone))
	for _, ch := range "B5..D12" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.CursorCol() != 1 || app.CursorRow() != 4 {
		t.Fatalf("Expected cursor at B5 (col 1, row 4), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}
	if app.SelectedRange() == nil || app.SelectedRange().String() != "B5:D12" {
		t.Fatalf("Expected selected range B5:D12, got: %v", app.SelectedRange())
	}

	// 3. Jump to Named Range TargetCell
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	for _, ch := range "targetcell" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.CursorCol() != 3 || app.CursorRow() != 14 {
		t.Fatalf("Expected cursor at D15 (col 3, row 14), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}

	// 4. Jump to Cross-Sheet Sheet2!B3
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	for _, ch := range "Sheet2!B3" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.ActiveSheet() != sh2 {
		t.Fatalf("Expected active sheet to be Sheet2, got: %s", app.ActiveSheet().Name())
	}
	if app.CursorCol() != 1 || app.CursorRow() != 2 {
		t.Fatalf("Expected cursor at B3 (col 1, row 2), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}
}

func TestFindAndFindNextPrev(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "Apple", nil)
	s.SetCellInput(1, 2, "Banana", nil)
	s.SetCellInput(2, 5, "Pineapple", nil)
	s.SetCellInput(0, 8, "apple juice", nil)
	s.Recalculate()

	app := tui.NewApp(simScreen, s, "test_find.hwk")

	// 1. Find "apple" with Ctrl+F
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlF, 0, tcell.ModNone))
	for _, ch := range "apple" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// First match is Apple at A1 (col 0, row 0) or Pineapple / apple juice
	if app.CursorCol() != 0 || app.CursorRow() != 0 {
		t.Fatalf("Expected find match at A1 (col 0, row 0), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}

	// 2. Press F3 (Next match) -> Pineapple at C6 (col 2, row 5)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if app.CursorCol() != 2 || app.CursorRow() != 5 {
		t.Fatalf("Expected next match at C6 (col 2, row 5), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}

	// 3. Press F3 (Next match) -> apple juice at A9 (col 0, row 8)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if app.CursorCol() != 0 || app.CursorRow() != 8 {
		t.Fatalf("Expected next match at A9 (col 0, row 8), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}

	// 4. Press F3 (Wrap around) -> Apple at A1 (col 0, row 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	if app.CursorCol() != 0 || app.CursorRow() != 0 {
		t.Fatalf("Expected wrapped match at A1 (col 0, row 0), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}

	// 5. Press Shift+F3 (Prev match) -> apple juice at A9 (col 0, row 8)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModShift))
	if app.CursorCol() != 0 || app.CursorRow() != 8 {
		t.Fatalf("Expected prev match at A9 (col 0, row 8), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}
}

func TestViewportAdjustWithMultipleSheets(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	// Terminal height: 40 lines
	simScreen.SetSize(100, 40)

	wb := sheet.NewWorkbook("test_viewport.hwk")
	sh1 := wb.GetActiveSheet()
	sh1.SetName("Sheet1")
	wb.AddSheet("Sheet2") // Now has multiple sheets! Tab bar is shown at h-2

	// Populate cell at row 345 (A346)
	sh1.SetCellInput(0, 345, "TargetValue", nil)
	sh1.Recalculate()

	app := tui.NewApp(simScreen, sh1, "test_viewport.hwk")

	// Jump to A346 (col 0, row 345)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone))
	for _, ch := range "A346" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if app.CursorRow() != 345 {
		t.Fatalf("Expected cursorRow to be 345, got %d", app.CursorRow())
	}

	// For height 40 with multiple sheets: numGridRows = (40 - 3) - 3 + 1 = 35 rows
	// If cursor is at row 345, topRow must be at least 345 - 35 + 1 = 311
	// So visible rows are 311 to 345, meaning row 345 is visible in grid!
	w, h := simScreen.Size()
	_ = w
	numGridRows := (h - 3) - 3 + 1
	if app.CursorRow() < app.TopRow() || app.CursorRow() >= app.TopRow()+numGridRows {
		t.Fatalf("Cursor row %d is NOT in visible viewport [%d, %d)", app.CursorRow(), app.TopRow(), app.TopRow()+numGridRows)
	}
}

func TestFindAndReplace(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_replace.hwk")
	sh := wb.GetActiveSheet()
	sh.SetCellInput(0, 0, "apple", nil)
	sh.SetCellInput(0, 1, "pineapple", nil)
	sh.SetCellInput(0, 2, "banana", nil)
	sh.SetCellInput(0, 3, "apple juice", nil)
	sh.Recalculate()

	app := tui.NewApp(simScreen, sh, "test_replace.hwk")

	// Trigger Replace (Ctrl+H): find "apple", replace with "orange", scope "C"
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlH, 0, tcell.ModCtrl))
	for _, ch := range "apple" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	for _, ch := range "orange" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Scope: default Current Sheet (Enter)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// In confirmation prompt, clear default 'Y' and press 'A' (Replace All)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if sh.GetCell(0, 0).Value != "orange" {
		t.Fatalf("Expected A1 value to be 'orange', got %v (raw=%s)", sh.GetCell(0, 0).Value, sh.GetCell(0, 0).RawInput)
	}
	if sh.GetCell(0, 1).Value != "pineorange" {
		t.Fatalf("Expected A2 value to be 'pineorange', got %v (raw=%s)", sh.GetCell(0, 1).Value, sh.GetCell(0, 1).RawInput)
	}
	if sh.GetCell(0, 2).Value != "banana" {
		t.Fatalf("Expected A3 value to be 'banana', got %v (raw=%s)", sh.GetCell(0, 2).Value, sh.GetCell(0, 2).RawInput)
	}
	if sh.GetCell(0, 3).Value != "orange juice" {
		t.Fatalf("Expected A4 value to be 'orange juice', got %v (raw=%s)", sh.GetCell(0, 3).Value, sh.GetCell(0, 3).RawInput)
	}

	// Also test replacing value generated by formula
	sh.SetCellInput(1, 0, "=10+14", nil) // B1 = 24
	sh.SetCellInput(1, 1, "24", nil)     // B2 = 24
	sh.Recalculate()

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlH, 0, tcell.ModCtrl))
	for i := 0; i < 10; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "24" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	for i := 0; i < 10; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "444" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Scope: default Current Sheet (Enter)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Replace All
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if sh.GetCell(1, 0).Value != float64(444) && sh.GetCell(1, 0).Value != "444" {
		t.Fatalf("Expected B1 formula value to be replaced to 444, got %v", sh.GetCell(1, 0).Value)
	}
	if sh.GetCell(1, 1).Value != float64(444) && sh.GetCell(1, 1).Value != "444" {
		t.Fatalf("Expected B2 literal value to be replaced to 444, got %v", sh.GetCell(1, 1).Value)
	}
}

func TestWorkbookWideSearch(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_wb_search.hwk")
	sh1 := wb.GetActiveSheet()
	sh1.SetName("Sheet1")
	sh1.SetCellInput(0, 0, "Tokyo", nil)

	sh2 := wb.AddSheet("Sheet2")
	sh2.SetCellInput(2, 5, "Kyoto Unique Key", nil) // C6
	wb.RecalculateAll()

	app := tui.NewApp(simScreen, sh1, "test_wb_search.hwk")

	// Trigger Search-All (/HA = Home -> Find-All)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'H', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))

	for _, ch := range "Kyoto" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Active sheet should now be Sheet2, cursor at C6 (col 2, row 5)
	if app.ActiveSheet().Name() != "Sheet2" {
		t.Fatalf("Expected active sheet to be 'Sheet2', got %q", app.ActiveSheet().Name())
	}
	if app.CursorCol() != 2 || app.CursorRow() != 5 {
		t.Fatalf("Expected cursor at C6 (col 2, row 5), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}
}

func TestFreezePanesTitles(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_freeze.hwk")
	sh := wb.GetActiveSheet()

	app := tui.NewApp(simScreen, sh, "test_freeze.hwk")

	// Move cursor to C4 (col 2, row 3)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	// Freeze Both (/VFB)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'V', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'B', tcell.ModNone))

	if sh.FrozenRows() != 3 {
		t.Fatalf("Expected 3 frozen rows, got %d", sh.FrozenRows())
	}
	if sh.FrozenCols() != 2 {
		t.Fatalf("Expected 2 frozen columns, got %d", sh.FrozenCols())
	}

	// Clear freeze (/VFC)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'V', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModNone))

	if sh.FrozenRows() != 0 || sh.FrozenCols() != 0 {
		t.Fatalf("Expected 0 frozen rows/cols after clear, got rows=%d cols=%d", sh.FrozenRows(), sh.FrozenCols())
	}
}

func TestDataFillAndFillDownRight(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_data_fill.hwk")
	sh := wb.GetActiveSheet()
	app := tui.NewApp(simScreen, sh, "test_data_fill.hwk")

	// 1. Data Fill /DF across A1..A5 with start=10, step=5
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'F', tcell.ModNone))

	for i := 0; i < 10; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "A1..A5" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// start = 10
	for i := 0; i < 10; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "10" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// step = 5
	for i := 0; i < 10; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "5" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// stop = (empty)
	for i := 0; i < 10; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	expected := []float64{10, 15, 20, 25, 30}
	for i, exp := range expected {
		c := sh.GetCell(0, i)
		if c == nil || c.Value != exp {
			t.Fatalf("Expected A%d to be %v, got %v", i+1, exp, c)
		}
	}

	// 2. Put formula =A1*2 in B1, then select B1..B5 and Fill Down (Ctrl+D)
	sh.SetCellInput(1, 0, "=A1*2", nil)
	sh.Recalculate()

	// Select B1..B5
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)) // move to B1
	for i := 0; i < 4; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModShift))
	}

	// Fill Down (Ctrl+D)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModCtrl))

	for i := 0; i < 5; i++ {
		c := sh.GetCell(1, i)
		expVal := expected[i] * 2
		if c == nil || c.Value != expVal {
			t.Fatalf("Expected B%d to be %v, got %v (formula=%s)", i+1, expVal, c, c.RawInput)
		}
	}
}

func TestRangeTranspose(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_transpose.hwk")
	sh := wb.GetActiveSheet()
	// Fill A1:C2 (2 rows x 3 cols)
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(1, 0, "2", nil)
	sh.SetCellInput(2, 0, "3", nil)
	sh.SetCellInput(0, 1, "4", nil)
	sh.SetCellInput(1, 1, "5", nil)
	sh.SetCellInput(2, 1, "6", nil)
	sh.Recalculate()

	app := tui.NewApp(simScreen, sh, "test_transpose.hwk")

	// Transpose A1..C2 to E1 (/DT)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'T', tcell.ModNone))

	for i := 0; i < 10; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "A1..C2" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	for i := 0; i < 10; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "E1" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Target E1:F3 (3 rows x 2 cols)
	// E1=1, F1=4
	// E2=2, F2=5
	// E3=3, F3=6
	if sh.GetCell(4, 0) == nil || sh.GetCell(4, 0).Value != float64(1) || sh.GetCell(5, 0) == nil || sh.GetCell(5, 0).Value != float64(4) {
		t.Fatalf("Transpose row 1 incorrect")
	}
	if sh.GetCell(4, 1) == nil || sh.GetCell(4, 1).Value != float64(2) || sh.GetCell(5, 1) == nil || sh.GetCell(5, 1).Value != float64(5) {
		t.Fatalf("Transpose row 2 incorrect")
	}
	if sh.GetCell(4, 2) == nil || sh.GetCell(4, 2).Value != float64(3) || sh.GetCell(5, 2) == nil || sh.GetCell(5, 2).Value != float64(6) {
		t.Fatalf("Transpose row 3 incorrect")
	}
}

func TestMouseDragAndClick(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_mouse.hwk")
	sh := wb.GetActiveSheet()
	app := tui.NewApp(simScreen, sh, "test_mouse.hwk")
	app.RunOnceForTest()

	// 1. Initial click at cell B3: rowHeaderW=5, col A=9 (x:5..13), col B=9 (x:14..22), row 3=y:5
	app.ProcessEventForTest(tcell.NewEventMouse(15, 5, tcell.Button1, tcell.ModNone))
	if app.CursorCol() != 1 || app.CursorRow() != 2 {
		t.Fatalf("Expected cursor at B3 (col 1, row 2), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}
	if app.SelectedRange() != nil {
		t.Fatalf("Expected no range selection on initial single click, got %v", app.SelectedRange())
	}

	// 2. Drag to cell D5: col D (x:32..40), row 5=y:7
	app.ProcessEventForTest(tcell.NewEventMouse(33, 7, tcell.Button1, tcell.ModNone))
	if app.SelectedRange() == nil {
		t.Fatalf("Expected range selection on mouse drag")
	}
	expectedRange := "B3:D5"
	if app.SelectedRange().String() != expectedRange {
		t.Fatalf("Expected selected range %s, got %s", expectedRange, app.SelectedRange())
	}

	// 3. Release mouse button: selection persists
	app.ProcessEventForTest(tcell.NewEventMouse(33, 7, tcell.ButtonNone, tcell.ModNone))
	if app.SelectedRange() == nil || app.SelectedRange().String() != expectedRange {
		t.Fatalf("Expected selection to persist after release, got %v", app.SelectedRange())
	}

	// 4. Click cell A1: clears selection and moves cursor
	app.ProcessEventForTest(tcell.NewEventMouse(6, 3, tcell.Button1, tcell.ModNone))
	if app.CursorCol() != 0 || app.CursorRow() != 0 {
		t.Fatalf("Expected cursor at A1 (col 0, row 0), got col %d, row %d", app.CursorCol(), app.CursorRow())
	}
	if app.SelectedRange() != nil {
		t.Fatalf("Expected selection to clear on fresh single click, got %v", app.SelectedRange())
	}

	// 5. Mouse WheelDown: scrolls 3 rows down
	curRow := app.CursorRow()
	app.ProcessEventForTest(tcell.NewEventMouse(15, 5, tcell.WheelDown, tcell.ModNone))
	if app.CursorRow() != curRow+3 {
		t.Fatalf("Expected cursorRow to advance by 3, got %d (was %d)", app.CursorRow(), curRow)
	}
}

func TestPasteTransposeAndAutoFill(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_autofill.hwk")
	sh := wb.GetActiveSheet()
	app := tui.NewApp(simScreen, sh, "test_autofill.hwk")
	app.RunOnceForTest()

	// 1. Test AutoFill with 2 seed values: A1=10, A2=20 -> Select A1..A5 -> AutoFill (/DA)
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "20", nil)
	sh.Recalculate()

	// Select A1..A5
	app.ProcessEventForTest(tcell.NewEventMouse(6, 3, tcell.Button1, tcell.ModNone)) // Click A1
	app.ProcessEventForTest(tcell.NewEventMouse(6, 7, tcell.Button1, tcell.ModNone)) // Drag to A5
	app.ProcessEventForTest(tcell.NewEventMouse(6, 7, tcell.ButtonNone, tcell.ModNone))

	// Run AutoFill (/DA)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))

	expected := []float64{10, 20, 30, 40, 50}
	for i, exp := range expected {
		c := sh.GetCell(0, i)
		if c == nil || c.Value != exp {
			t.Fatalf("AutoFill failed at A%d: expected %v, got %v", i+1, exp, c)
		}
	}

	// 2. Test Paste Transpose: Copy A1..A5 (5 rows x 1 col) -> Paste Transpose at C1 (/HST)
	// Select A1..A5
	app.ProcessEventForTest(tcell.NewEventMouse(6, 3, tcell.Button1, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventMouse(6, 7, tcell.Button1, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventMouse(6, 7, tcell.ButtonNone, tcell.ModNone))

	// Copy (Ctrl+C)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl))

	// Move to C1 (col 2, row 0)
	app.ProcessEventForTest(tcell.NewEventMouse(24, 3, tcell.Button1, tcell.ModNone))

	// Paste Transpose (/HST)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'H', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'T', tcell.ModNone))

	// Verify C1..G1 have 10, 20, 30, 40, 50
	for i, exp := range expected {
		col := 2 + i // C=2, D=3, E=4, F=5, G=6
		c := sh.GetCell(col, 0)
		if c == nil || c.Value != exp {
			t.Fatalf("Paste Transpose failed at col %d row 0: expected %v, got %v", col, exp, c)
		}
	}

	// 3. Test Date DataFill and AutoFill: H1 = 2026/08/27, select H1..H5 -> /DA
	sh.SetCellInput(7, 0, "2026/08/27", nil)
	sh.Recalculate()

	app.ProcessEventForTest(tcell.NewEventMouse(69, 3, tcell.Button1, tcell.ModNone)) // Click H1 (x: 68..76)
	app.ProcessEventForTest(tcell.NewEventMouse(69, 7, tcell.Button1, tcell.ModNone)) // Drag to H5
	app.ProcessEventForTest(tcell.NewEventMouse(69, 7, tcell.ButtonNone, tcell.ModNone))

	// Run AutoFill (/DA)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))

	expectedDates := []string{"2026/08/27", "2026/08/28", "2026/08/29", "2026/08/30", "2026/08/31"}
	for i, exp := range expectedDates {
		c := sh.GetCell(7, i)
		if c == nil || c.Value != exp {
			t.Fatalf("Date AutoFill failed at H%d: expected %s, got %v", i+1, exp, c)
		}
	}

	// 4. Test Data -> Transpose (/DT): Transpose H1..H5 (1 col x 5 rows) to J1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'D', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'T', tcell.ModNone))

	for i := 0; i < 15; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "H1..H5" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	for i := 0; i < 15; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	for _, ch := range "J1" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// Verify J1..N1 (cols 9..13) contain the transposed dates
	for i, exp := range expectedDates {
		c := sh.GetCell(9+i, 0)
		if c == nil || c.Value != exp {
			t.Fatalf("Data Transpose (/DT) failed at col %d row 0: expected %s, got %v", 9+i, exp, c)
		}
	}
}

func TestVersionConstant(t *testing.T) {
	if version.Version != "2.0.2" {
		t.Fatalf("Expected version.Version to be '2.0.2', got %q", version.Version)
	}
	if Version != version.Version {
		t.Fatalf("Expected CLI Version (%q) to match version.Version (%q)", Version, version.Version)
	}
	if tui.AppVersion != version.Version {
		t.Fatalf("Expected TUI AppVersion (%q) to match version.Version (%q)", tui.AppVersion, version.Version)
	}
}

func TestAboutDialogViaMenuAndDirect(t *testing.T) {
	if tui.AppVersion != "2.0.2" {
		t.Fatalf("Expected AppVersion to be '2.0.2', got %q", tui.AppVersion)
	}

	simScreen := tcell.NewSimulationScreen("UTF-8")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("test_about.hwk")
	sh := wb.GetActiveSheet()
	app := tui.NewApp(simScreen, sh, "test_about.hwk")

	// 1. Open About Dialog via Slash Menu /?A
	simScreen.PostEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '?', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))

	// 2. Direct RenderAboutScreen verification
	simScreen.PostEvent(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	tui.RenderAboutScreen(simScreen, tui.InitStyles())
}

func TestCircularReferenceDetection(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=B1", nil) // A1
	sh.SetCellInput(1, 0, "=A1", nil) // B1
	sh.Recalculate()
	a1 := sh.GetCell(0, 0)
	b1 := sh.GetCell(1, 0)
	if a1 == nil || a1.Value != cell.ErrCirc {
		t.Fatalf("expected A1 CIRCULAR REF, got %v", a1)
	}
	if b1 == nil || b1.Value != cell.ErrCirc {
		t.Fatalf("expected B1 CIRCULAR REF, got %v", b1)
	}
	if a1.Value.(cell.LotusError).Code != "CIRCULAR REF" {
		t.Fatalf("expected code CIRCULAR REF, got %q", a1.Value.(cell.LotusError).Code)
	}
}

func TestRecalcDependencyOrderNoStale(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(2, 0, "10", nil)    // C1
	sh.SetCellInput(0, 0, "=C1+1", nil) // A1
	sh.SetCellInput(1, 0, "=A1+1", nil) // B1
	sh.Recalculate()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 11 {
		t.Fatalf("expected A1=11, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 12 {
		t.Fatalf("expected B1=12, got %v", sh.GetCellValue(1, 0))
	}
	sh.SetCellInput(2, 0, "100", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 102 {
		t.Fatalf("expected B1=102 after C1 change, got %v", sh.GetCellValue(1, 0))
	}
}

func TestWholeColumnSumBeyond1000(t *testing.T) {
	sh := sheet.NewSheet()
	for r := 0; r < 1500; r++ {
		sh.SetCellInput(0, r, "1", nil)
	}
	sh.SetCellInput(1, 0, "=SUM(A:A)", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 1500 {
		t.Fatalf("expected SUM(A:A)=1500, got %v", sh.GetCellValue(1, 0))
	}
	sh.ClearCell(0, 1499)
	if sh.MaxPopulatedRow() != 1498 {
		t.Fatalf("expected maxPopulatedRow=1498 after clear, got %d", sh.MaxPopulatedRow())
	}
	sh.Recalculate()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 1499 {
		t.Fatalf("expected SUM(A:A)=1499 after clear, got %v", sh.GetCellValue(1, 0))
	}
}

func TestUndoDoesNotCrossSheets(t *testing.T) {
	wb := sheet.NewWorkbook("undo_wb")
	s1 := wb.Sheets[0]
	s1.SetName("Sheet1")
	s2 := wb.AddSheet("Sheet2")
	um := sheet.NewUndoManager(10)

	s1.SetCellInput(0, 0, "111", nil)
	um.Push(s1)
	s1.SetCellInput(0, 0, "222", nil)

	// Switch conceptually to Sheet2 and undo — must restore Sheet1, not corrupt Sheet2.
	s2.SetCellInput(0, 0, "keep", nil)
	resolve := func(name string) *sheet.Sheet {
		return wb.GetSheet(name)
	}
	if !um.Undo(s2, resolve, wb, nil) {
		t.Fatal("undo failed")
	}
	if v, ok := s1.GetCellValue(0, 0).(float64); !ok || v != 111 {
		t.Fatalf("Sheet1 should be restored to 111, got %v", s1.GetCellValue(0, 0))
	}
	if s2.GetCell(0, 0) == nil || s2.GetCell(0, 0).RawInput != "keep" && s2.GetCell(0, 0).RawInput != "'keep" {
		// label may be stored with quote
		raw := ""
		if c := s2.GetCell(0, 0); c != nil {
			raw = c.RawInput
		}
		if !strings.Contains(raw, "keep") {
			t.Fatalf("Sheet2 should keep its value, got %q", raw)
		}
	}
}

func TestShiftPreservesSheetPrefix(t *testing.T) {
	adj := formula.AdjustFormulaReferences("=Sheet2!A1+1", 1, 2)
	if !strings.Contains(adj, "Sheet2!") {
		t.Fatalf("expected Sheet2! prefix preserved, got %q", adj)
	}
}

func TestInsertRowAdjustsFormulaRefs(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 4, "10", nil)    // A5
	sh.SetCellInput(1, 0, "=A5*2", nil) // B1
	sh.Recalculate()
	sh.InsertRow(2, 1) // insert at row 3 (0-based index 2)
	b1 := sh.GetCell(1, 0)
	if b1 == nil || !strings.Contains(b1.RawInput, "A6") {
		t.Fatalf("expected B1 to reference A6 after insert, got %v", b1)
	}
	sh.Recalculate()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("expected B1=20, got %v", sh.GetCellValue(1, 0))
	}
}

func TestNowTodayExcelLike(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=NOW()", nil)
	sh.SetCellInput(1, 0, "=TODAY()", nil)
	sh.Recalculate()
	nowV, ok1 := sh.GetCellValue(0, 0).(float64)
	todayV, ok2 := sh.GetCellValue(1, 0).(float64)
	if !ok1 || !ok2 {
		t.Fatalf("NOW/TODAY should be float64, got %T %T", sh.GetCellValue(0, 0), sh.GetCellValue(1, 0))
	}
	if todayV != float64(int(todayV)) {
		t.Fatalf("TODAY should be integer day serial, got %v", todayV)
	}
	frac := nowV - float64(int(nowV))
	if frac < 0 || frac >= 1 {
		t.Fatalf("NOW fractional day out of range: %v", frac)
	}
	// NOW should not double-count time (frac would be >~0.999 wrongly often, but mainly NOW >= TODAY)
	if nowV < todayV || nowV >= todayV+1 {
		t.Fatalf("NOW=%v should be within TODAY day %v", nowV, todayV)
	}
}

func TestFreezePanesPersistedInHWK(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "freeze.hwk")
	wb := sheet.NewWorkbook("freeze.hwk")
	sh := wb.GetActiveSheet()
	sh.SetCellInput(0, 0, "H", nil)
	sh.SetFrozenRows(2)
	sh.SetFrozenCols(1)
	if err := wb.SaveJSON(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := sheet.LoadWorkbookJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	ls := loaded.GetActiveSheet()
	if ls.FrozenRows() != 2 || ls.FrozenCols() != 1 {
		t.Fatalf("expected freeze 2/1, got %d/%d", ls.FrozenRows(), ls.FrozenCols())
	}
}

func TestScientificFormatEAlias(t *testing.T) {
	f := cell.ParseCellFormat("(E2)")
	if f.Type != cell.FmtScientific || f.Decimals != 2 {
		t.Fatalf("expected scientific E2, got %+v", f)
	}
	f2 := cell.ParseCellFormat("(S3)")
	if f2.Type != cell.FmtScientific || f2.Decimals != 3 {
		t.Fatalf("expected S alias, got %+v", f2)
	}
}

func TestIsErrorVsIsErr(t *testing.T) {
	sh := sheet.NewSheet()
	cNA := cell.NewCell("x", nil)
	cNA.Type = cell.TypeFormula
	cNA.RawInput = "=NA"
	cNA.Value = cell.ErrNA
	sh.SetCell(1, 0, cNA) // B1

	cERR := cell.NewCell("x", nil)
	cERR.Type = cell.TypeFormula
	cERR.RawInput = "=ERR"
	cERR.Value = cell.ErrLotus
	sh.SetCell(0, 0, cERR) // A1

	sh.SetCellInput(2, 0, "=ISERR(B1)", nil)
	sh.SetCellInput(3, 0, "=ISERROR(B1)", nil)
	sh.SetCellInput(4, 0, "=ISERR(A1)", nil)
	sh.SetCellInput(5, 0, "=ISERROR(A1)", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 0 {
		t.Fatalf("ISERR(NA) expected 0, got %v", sh.GetCellValue(2, 0))
	}
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 1 {
		t.Fatalf("ISERROR(NA) expected 1, got %v", sh.GetCellValue(3, 0))
	}
	if v, ok := sh.GetCellValue(4, 0).(float64); !ok || v != 1 {
		t.Fatalf("ISERR(ERR) expected 1, got %v", sh.GetCellValue(4, 0))
	}
	if v, ok := sh.GetCellValue(5, 0).(float64); !ok || v != 1 {
		t.Fatalf("ISERROR(ERR) expected 1, got %v", sh.GetCellValue(5, 0))
	}
}

func TestP0FixesLexerStringEscapeAndCountFunction(t *testing.T) {
	sh := sheet.NewSheet()

	// 1. Verify double quote escaping in formula string literals: ="He said ""Hello"""
	sh.SetCellInput(0, 0, `="He said ""Hello"""`, nil) // A1
	sh.Recalculate()
	valA1 := sh.GetCellValue(0, 0)
	expectedA1 := `He said "Hello"`
	if valA1 != expectedA1 {
		t.Errorf("Expected string with escaped quotes %q, got %v", expectedA1, valA1)
	}

	// 2. Verify COUNT counts only numeric cells, while COUNTA counts all non-empty cells
	// A2: 10, A3: "apple", A4: 20, A5: "orange", A6: empty
	sh.SetCellInput(0, 1, "10", nil)             // A2 (number)
	sh.SetCellInput(0, 2, "'apple", nil)         // A3 (string)
	sh.SetCellInput(0, 3, "20", nil)             // A4 (number)
	sh.SetCellInput(0, 4, "'orange", nil)        // A5 (string)
	sh.SetCellInput(1, 0, "=COUNT(A2:A5)", nil)  // B1
	sh.SetCellInput(1, 1, "=COUNTA(A2:A5)", nil) // B2
	sh.Recalculate()

	countVal := sh.GetCellValue(1, 0)
	if countVal != 2.0 {
		t.Errorf("Expected COUNT(A2:A5) = 2 (numbers only), got %v", countVal)
	}

	countAVal := sh.GetCellValue(1, 1)
	if countAVal != 4.0 {
		t.Errorf("Expected COUNTA(A2:A5) = 4 (all non-empty), got %v", countAVal)
	}
}

func TestP1Fixes(t *testing.T) {
	// 1. Whole column absolute reference $B:$C
	ref, err := coord.ParseRangeRef("$B:$C")
	if err != nil {
		t.Fatalf("ParseRangeRef $B:$C failed: %v", err)
	}
	if ref.MinCol() != 1 || ref.MaxCol() != 2 {
		t.Errorf("Expected MinCol=1, MaxCol=2 for $B:$C, got %d..%d", ref.MinCol(), ref.MaxCol())
	}
	if !ref.Start.ColAbs || !ref.End.ColAbs {
		t.Errorf("Expected ColAbs=true for both endpoints of $B:$C")
	}

	// 2. Whole row range reference in formula: =SUM(1:2)
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "10", nil)        // A1
	sh.SetCellInput(1, 0, "20", nil)        // B1
	sh.SetCellInput(0, 1, "30", nil)        // A2
	sh.SetCellInput(1, 1, "40", nil)        // B2
	sh.SetCellInput(0, 2, "=SUM(1:2)", nil) // A3: 10 + 20 + 30 + 40 = 100
	sh.Recalculate()
	valA3 := sh.GetCellValue(0, 2)
	if valA3 != 100.0 {
		t.Errorf("Expected =SUM(1:2) = 100, got %v", valA3)
	}

	// 3. Leading decimal numbers (.5, -.25)
	cPos := cell.NewCell(".5", nil)
	if cPos.Type != cell.TypeNumber || cPos.Value != 0.5 {
		t.Errorf("Expected .5 to be NUMBER with value 0.5, got type=%v, val=%v", cPos.Type, cPos.Value)
	}
	cNeg := cell.NewCell("-.25", nil)
	if cNeg.Type != cell.TypeNumber || cNeg.Value != -0.25 {
		t.Errorf("Expected -.25 to be NUMBER with value -0.25, got type=%v, val=%v", cNeg.Type, cNeg.Value)
	}

	// 4. Whole column operations without OOM
	wholeColRef, _ := coord.ParseRangeRef("A:A")
	fmtCurr := cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2}
	sh.FormatRange(wholeColRef, fmtCurr)
	// Make sure we didn't populate 1,000,000 cells!
	if len(sh.GetPopulatedCoords()) > 100 {
		t.Errorf("FormatRange on A:A populated too many cells: %d", len(sh.GetPopulatedCoords()))
	}
	sh.ClearRange(wholeColRef)
	// After clearing column A, A1 and A2 should be deleted
	if sh.GetCell(0, 0) != nil {
		t.Errorf("Expected A1 to be cleared")
	}
}

func TestP2Fixes(t *testing.T) {
	// 1. TEXT function with seconds formatting
	sh := sheet.NewSheet()
	// Date serial for 2026-08-26 14:30:45
	sh.SetCellInput(0, 0, "=DATE(2026, 8, 26) + TIME(14, 30, 45)", nil) // A1
	sh.SetCellInput(0, 1, `=TEXT(A1, "yyyy-mm-dd hh:mm:ss")`, nil)      // A2
	sh.Recalculate()
	valA2 := sh.GetCellValue(0, 1)
	expectedA2 := "2026-08-26 14:30:45"
	if valA2 != expectedA2 {
		t.Errorf("Expected TEXT with seconds %q, got %v", expectedA2, valA2)
	}

	// 2. Boolean rendering: TRUE/FALSE uppercase & centered
	cBoolTrue := &cell.Cell{
		Type:      cell.TypeFormula,
		Value:     true,
		Alignment: cell.AlignDefault,
	}
	renderedTrue := cBoolTrue.Render(10, cell.CellFormat{})
	if !strings.Contains(renderedTrue, "TRUE") {
		t.Errorf("Expected bool true to render TRUE, got %q", renderedTrue)
	}
	// Check centering (spaces before and after)
	if !strings.HasPrefix(renderedTrue, " ") || !strings.HasSuffix(renderedTrue, " ") {
		t.Errorf("Expected TRUE to be center aligned, got %q", renderedTrue)
	}

	cBoolFalse := &cell.Cell{
		Type:      cell.TypeFormula,
		Value:     false,
		Alignment: cell.AlignDefault,
	}
	renderedFalse := cBoolFalse.Render(10, cell.CellFormat{})
	if !strings.Contains(renderedFalse, "FALSE") {
		t.Errorf("Expected bool false to render FALSE, got %q", renderedFalse)
	}
	if !strings.HasPrefix(renderedFalse, " ") || !strings.HasSuffix(renderedFalse, " ") {
		t.Errorf("Expected FALSE to be center aligned, got %q", renderedFalse)
	}

	// 3. ExportXLSXWorkbook unification
	wb := sheet.NewWorkbook("TestP2WB")
	sh1 := wb.Sheets[0]
	sh1.SetName("TestP2")
	sh1.SetCellInput(0, 0, "123", nil)
	sh1.SetCellInput(0, 1, "'Hello P2", nil)
	tmpFile := filepath.Join(t.TempDir(), "test_p2_export.xlsx")
	if err := sheet.ExportXLSXWorkbook(wb, tmpFile); err != nil {
		t.Fatalf("ExportXLSXWorkbook failed: %v", err)
	}
	// Verify it can be loaded back
	wbLoaded, err := sheet.ImportXLSXWorkbook(tmpFile)
	if err != nil {
		t.Fatalf("Failed to reload exported XLSX: %v", err)
	}
	if len(wbLoaded.Sheets) != 1 {
		t.Fatalf("Expected 1 sheet, got %d", len(wbLoaded.Sheets))
	}
	loadedSh := wbLoaded.Sheets[0]
	if loadedSh.GetCellValue(0, 0) != 123.0 {
		t.Errorf("Expected 123 in A1, got %v", loadedSh.GetCellValue(0, 0))
	}
	if loadedSh.GetCellValue(0, 1) != "Hello P2" {
		t.Errorf("Expected 'Hello P2' in A2, got %v", loadedSh.GetCellValue(0, 1))
	}
}

func TestP3Fixes(t *testing.T) {
	// 1. AlignRepeat with strings.Repeat optimization
	cRep := cell.NewCell(`\=`, nil) // Repeat "="
	rendered := cRep.Render(10, cell.CellFormat{})
	expected := "=========="
	if rendered != expected {
		t.Errorf("Expected AlignRepeat to produce %q (len %d), got %q (len %d)", expected, len(expected), rendered, len(rendered))
	}

	// 2. AlignRepeat with multi-char repeat pattern
	cMulti := cell.NewCell(`\-+`, nil) // Repeat "-+"
	renderedMulti := cMulti.Render(7, cell.CellFormat{})
	expectedMulti := "-+-+-+-"
	if renderedMulti != expectedMulti {
		t.Errorf("Expected multi-char repeat to produce %q, got %q", expectedMulti, renderedMulti)
	}
}

func TestP4AuditFixes(t *testing.T) {
	sh := sheet.NewSheet()

	// 1. MOD function with negative numbers (Excel compliance)
	sh.SetCellInput(0, 0, "=MOD(-3, 2)", nil)  // A1: expected 1
	sh.SetCellInput(0, 1, "=MOD(3, -2)", nil)  // A2: expected -1
	sh.SetCellInput(0, 2, "=MOD(-3, -2)", nil) // A3: expected -1
	sh.SetCellInput(0, 3, "=MOD(3, 2)", nil)   // A4: expected 1
	sh.Recalculate()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 1.0 {
		t.Errorf("Expected MOD(-3, 2) = 1, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != -1.0 {
		t.Errorf("Expected MOD(3, -2) = -1, got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != -1.0 {
		t.Errorf("Expected MOD(-3, -2) = -1, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 1.0 {
		t.Errorf("Expected MOD(3, 2) = 1, got %v", sh.GetCellValue(0, 3))
	}

	// 2. Mutual recursive named ranges without stack overflow
	wb := sheet.NewWorkbook("TestCircWB")
	shCirc := wb.Sheets[0]
	wb.SetNamedRange("LOOPA", "=LOOPB")
	wb.SetNamedRange("LOOPB", "=LOOPA")
	shCirc.SetCellInput(0, 0, "=LOOPA", nil)
	shCirc.Recalculate()
	if shCirc.GetCellValue(0, 0) != cell.ErrCirc {
		t.Errorf("Expected CIRCULAR REF for mutual named range loop, got %v", shCirc.GetCellValue(0, 0))
	}

	// 3. String concatenation & with empty/nil cell
	shConcat := sheet.NewSheet()
	// B1 is empty
	shConcat.SetCellInput(0, 0, `="X" & B1 & "Y"`, nil) // A1
	shConcat.Recalculate()
	if shConcat.GetCellValue(0, 0) != "XY" {
		t.Errorf("Expected 'XY' for concating empty cell, got %q", shConcat.GetCellValue(0, 0))
	}

	// 4. Formula shift with whole column reference (e.g. =SUM(A:A)) copied down by dRow=5
	shiftedCol := formula.AdjustFormulaReferences("=SUM(A:A)", 0, 5)
	if shiftedCol != "=SUM(A:A)" {
		t.Errorf("Expected whole-column ref to stay =SUM(A:A) when shifted down, got %q", shiftedCol)
	}

	// 5. RangeRef.String() formatting without duplicate sheet prefix
	rng, err := coord.ParseRangeRef("Sheet1!A1..B10")
	if err != nil {
		t.Fatalf("ParseRangeRef failed: %v", err)
	}
	if rng.String() != "Sheet1!A1:B10" {
		t.Errorf("Expected 'Sheet1!A1:B10', got %q", rng.String())
	}

	// 6. Subdirectory creation in Export functions
	subDir := filepath.Join(t.TempDir(), "nested", "subdir")
	odsPath := filepath.Join(subDir, "test.ods")
	csvPath := filepath.Join(subDir, "test.csv")
	mdPath := filepath.Join(subDir, "test.md")

	if err := shConcat.ExportODS(odsPath); err != nil {
		t.Errorf("ExportODS failed with nested path: %v", err)
	}
	if err := shConcat.ExportCSV(csvPath); err != nil {
		t.Errorf("ExportCSV failed with nested path: %v", err)
	}
	if err := shConcat.ExportMarkdown(mdPath); err != nil {
		t.Errorf("ExportMarkdown failed with nested path: %v", err)
	}
}

func TestEdgeCaseFixes(t *testing.T) {
	sh := sheet.NewSheet()

	// 1. EDATE month end clipping (leap year 2024 vs non-leap year 2023)
	// 2024/01/31 is serial 45322
	// 2024/02/29 is serial 45351
	// 2023/01/31 is serial 44957
	// 2023/02/28 is serial 44985
	sh.SetCellInput(0, 0, `=EDATE(DATE(2024, 1, 31), 1)`, nil)  // A1: 2024-02-29
	sh.SetCellInput(0, 1, `=EDATE(DATE(2023, 1, 31), 1)`, nil)  // A2: 2023-02-28
	sh.SetCellInput(0, 2, `=EDATE(DATE(2024, 3, 31), -1)`, nil) // A3: 2024-02-29
	sh.SetCellInput(0, 3, `=EDATE(DATE(2024, 3, 31), 1)`, nil)  // A4: 2024-04-30

	// 2. LEFT and RIGHT with optional 2nd argument omitted
	sh.SetCellInput(1, 0, `=LEFT("Hello")`, nil)  // B1: "H"
	sh.SetCellInput(1, 1, `=RIGHT("World")`, nil) // B2: "d"

	// 3. IRR normal convergence and non-convergence
	// Normal: cash flows -100, 30, 40, 50 -> IRR approx 0.0886
	sh.SetCellInput(2, 0, "-100", nil)
	sh.SetCellInput(2, 1, "30", nil)
	sh.SetCellInput(2, 2, "40", nil)
	sh.SetCellInput(2, 3, "50", nil)
	sh.SetCellInput(3, 0, `=IRR(C1:C4)`, nil) // D1

	// Non-convergence: all positive cash flows (cannot have IRR)
	sh.SetCellInput(2, 4, "100", nil)
	sh.SetCellInput(2, 5, "200", nil)
	sh.SetCellInput(3, 1, `=IRR(C5:C6)`, nil) // D2: expect ErrLotus

	sh.Recalculate()

	// Verify EDATE
	d1 := sh.GetCellValue(0, 0)
	d2 := sh.GetCellValue(0, 1)
	d3 := sh.GetCellValue(0, 2)
	d4 := sh.GetCellValue(0, 3)

	// Format as Date string to inspect exact calendar date
	fmtD := cell.CellFormat{Type: cell.FmtDate, DateFormat: 1}
	if s := cell.FormatNumber(d1.(float64), fmtD); s != "2024/02/29" {
		t.Errorf("Expected EDATE(2024-01-31, 1) to be '2024/02/29', got %q", s)
	}
	if s := cell.FormatNumber(d2.(float64), fmtD); s != "2023/02/28" {
		t.Errorf("Expected EDATE(2023-01-31, 1) to be '2023/02/28', got %q", s)
	}
	if s := cell.FormatNumber(d3.(float64), fmtD); s != "2024/02/29" {
		t.Errorf("Expected EDATE(2024-03-31, -1) to be '2024/02/29', got %q", s)
	}
	if s := cell.FormatNumber(d4.(float64), fmtD); s != "2024/04/30" {
		t.Errorf("Expected EDATE(2024-03-31, 1) to be '2024/04/30', got %q", s)
	}

	// Verify LEFT / RIGHT
	if sh.GetCellValue(1, 0) != "H" {
		t.Errorf("Expected LEFT('Hello') to be 'H', got %v", sh.GetCellValue(1, 0))
	}
	if sh.GetCellValue(1, 1) != "d" {
		t.Errorf("Expected RIGHT('World') to be 'd', got %v", sh.GetCellValue(1, 1))
	}

	// Verify IRR
	irrVal, ok := sh.GetCellValue(3, 0).(float64)
	if !ok || math.Abs(irrVal-0.0886) > 0.001 {
		t.Errorf("Expected IRR(C1:C4) around 0.0886, got %v", sh.GetCellValue(3, 0))
	}
	if sh.GetCellValue(3, 1) != cell.ErrLotus {
		t.Errorf("Expected ERR for non-converging all-positive IRR, got %v", sh.GetCellValue(3, 1))
	}
}

func TestStringAndLookupEdgeCases(t *testing.T) {
	sh := sheet.NewSheet()

	// 1. SEARCH with wildcards ? and *
	sh.SetCellInput(0, 0, `=SEARCH("b?ll", "football")`, nil) // A1: 5
	sh.SetCellInput(0, 1, `=SEARCH("f*t", "football")`, nil)  // A2: 1

	// 2. MID bounds checks
	sh.SetCellInput(1, 0, `=MID("Hello", 0, 2)`, nil)  // B1: ERR (#VALUE!)
	sh.SetCellInput(1, 1, `=MID("Hello", 2, -1)`, nil) // B2: ERR (#VALUE!)
	sh.SetCellInput(1, 2, `=MID("Hello", 2, 0)`, nil)  // B3: ""

	// 3. SUBSTITUTE bounds and empty old_text
	sh.SetCellInput(2, 0, `=SUBSTITUTE("banana", "a", "o", 0)`, nil)  // C1: ERR (#VALUE!)
	sh.SetCellInput(2, 1, `=SUBSTITUTE("banana", "a", "o", -1)`, nil) // C2: ERR (#VALUE!)
	sh.SetCellInput(2, 2, `=SUBSTITUTE("banana", "", "x")`, nil)      // C3: "banana"
	sh.SetCellInput(2, 3, `=SUBSTITUTE("banana", "a", "o", 2)`, nil)  // C4: "bonana"

	// 4. REPLACE bounds
	sh.SetCellInput(3, 0, `=REPLACE("Hello", 0, 2, "X")`, nil)  // D1: ERR (#VALUE!)
	sh.SetCellInput(3, 1, `=REPLACE("Hello", 2, -1, "X")`, nil) // D2: ERR (#VALUE!)

	// 5. XMATCH unsorted best match (matchMode 1 and -1)
	// Array: E1=10, E2=6, E3=2
	sh.SetCellInput(4, 0, "10", nil)
	sh.SetCellInput(4, 1, "6", nil)
	sh.SetCellInput(4, 2, "2", nil)
	sh.SetCellInput(5, 0, `=XMATCH(5, E1:E3, 1)`, nil)  // F1: next larger is 6 (index 2)
	sh.SetCellInput(5, 1, `=XMATCH(5, E1:E3, -1)`, nil) // F2: next smaller is 2 (index 3)

	// 6. TEXTBEFORE and TEXTAFTER with instance_num and if_not_found
	sh.SetCellInput(6, 0, `=TEXTBEFORE("A-B-C-D", "-", 2)`, nil)                     // G1: "A-B"
	sh.SetCellInput(6, 1, `=TEXTBEFORE("A-B-C-D", "-", -1)`, nil)                    // G2: "A-B-C"
	sh.SetCellInput(6, 2, `=TEXTAFTER("A-B-C-D", "-", 2)`, nil)                      // G3: "C-D"
	sh.SetCellInput(6, 3, `=TEXTAFTER("A-B-C-D", "-", -1)`, nil)                     // G4: "D"
	sh.SetCellInput(6, 4, `=TEXTBEFORE("A-B", "X", 1, 0, 0, "CustomNotFound")`, nil) // G5: "CustomNotFound"

	sh.Recalculate()

	// Assertions
	if sh.GetCellValue(0, 0) != 5.0 {
		t.Errorf("Expected SEARCH('b?ll', 'football') = 5, got %v", sh.GetCellValue(0, 0))
	}
	if sh.GetCellValue(0, 1) != 1.0 {
		t.Errorf("Expected SEARCH('f*t', 'football') = 1, got %v", sh.GetCellValue(0, 1))
	}

	if sh.GetCellValue(1, 0) != cell.ErrLotus {
		t.Errorf("Expected ERR for MID(..., 0, 2), got %v", sh.GetCellValue(1, 0))
	}
	if sh.GetCellValue(1, 1) != cell.ErrLotus {
		t.Errorf("Expected ERR for MID(..., 2, -1), got %v", sh.GetCellValue(1, 1))
	}
	if sh.GetCellValue(1, 2) != "" {
		t.Errorf("Expected empty string for MID(..., 2, 0), got %v", sh.GetCellValue(1, 2))
	}

	if sh.GetCellValue(2, 0) != cell.ErrLotus {
		t.Errorf("Expected ERR for SUBSTITUTE with instance_num=0, got %v", sh.GetCellValue(2, 0))
	}
	if sh.GetCellValue(2, 1) != cell.ErrLotus {
		t.Errorf("Expected ERR for SUBSTITUTE with instance_num=-1, got %v", sh.GetCellValue(2, 1))
	}
	if sh.GetCellValue(2, 2) != "banana" {
		t.Errorf("Expected 'banana' for SUBSTITUTE with empty old_text, got %v", sh.GetCellValue(2, 2))
	}
	if sh.GetCellValue(2, 3) != "banona" {
		t.Errorf("Expected 'banona' for SUBSTITUTE, got %v", sh.GetCellValue(2, 3))
	}

	if sh.GetCellValue(3, 0) != cell.ErrLotus {
		t.Errorf("Expected ERR for REPLACE with start_num=0, got %v", sh.GetCellValue(3, 0))
	}
	if sh.GetCellValue(3, 1) != cell.ErrLotus {
		t.Errorf("Expected ERR for REPLACE with num_chars=-1, got %v", sh.GetCellValue(3, 1))
	}

	if sh.GetCellValue(5, 0) != 2.0 {
		t.Errorf("Expected XMATCH(5, {10,6,2}, 1) = 2 (element 6), got %v", sh.GetCellValue(5, 0))
	}
	if sh.GetCellValue(5, 1) != 3.0 {
		t.Errorf("Expected XMATCH(5, {10,6,2}, -1) = 3 (element 2), got %v", sh.GetCellValue(5, 1))
	}

	if sh.GetCellValue(6, 0) != "A-B" {
		t.Errorf("Expected TEXTBEFORE('A-B-C-D', '-', 2) = 'A-B', got %v", sh.GetCellValue(6, 0))
	}
	if sh.GetCellValue(6, 1) != "A-B-C" {
		t.Errorf("Expected TEXTBEFORE('A-B-C-D', '-', -1) = 'A-B-C', got %v", sh.GetCellValue(6, 1))
	}
	if sh.GetCellValue(6, 2) != "C-D" {
		t.Errorf("Expected TEXTAFTER('A-B-C-D', '-', 2) = 'C-D', got %v", sh.GetCellValue(6, 2))
	}
	if sh.GetCellValue(6, 3) != "D" {
		t.Errorf("Expected TEXTAFTER('A-B-C-D', '-', -1) = 'D', got %v", sh.GetCellValue(6, 3))
	}
	if sh.GetCellValue(6, 4) != "CustomNotFound" {
		t.Errorf("Expected 'CustomNotFound' for TEXTBEFORE with fallback, got %v", sh.GetCellValue(6, 4))
	}
}

func TestP5RemainingRisks(t *testing.T) {
	// 1. Whole column preservation on DeleteRow
	sh1 := sheet.NewSheet()
	sh1.SetCellInput(0, 0, "=SUM(B:B)", nil)
	sh1.DeleteRow(4, 1)
	if f := sh1.GetCell(0, 0).RawInput; f != "=SUM(B:B)" {
		t.Errorf("Expected whole column '=SUM(B:B)' after DeleteRow, got %q", f)
	}

	// Whole row preservation on DeleteCol
	sh2 := sheet.NewSheet()
	sh2.SetCellInput(0, 0, "=SUM(1:10)", nil)
	sh2.DeleteCol(2, 1)
	if f := sh2.GetCell(0, 0).RawInput; f != "=SUM(1:10)" {
		t.Errorf("Expected whole row '=SUM(1:10)' after DeleteCol, got %q", f)
	}

	// 2. Cross-sheet reference updates on RenameSheet and DeleteSheet
	wb := sheet.NewWorkbook("TestWb")
	wbSh1 := wb.Sheets[0]
	wbSh2 := wb.AddSheet("Data")
	wbSh2.SetCellInput(0, 0, "42", nil) // Data!A1 = 42

	wbSh1.SetCellInput(0, 0, "=Data!A1 * 2", nil)
	wb.RecalculateAll()
	if v, ok := wbSh1.GetCellValue(0, 0).(float64); !ok || v != 84.0 {
		t.Errorf("Expected Data!A1 * 2 = 84, got %v", wbSh1.GetCellValue(0, 0))
	}

	// Rename Data -> Inventory
	if err := wb.RenameSheet("Data", "Inventory"); err != nil {
		t.Fatalf("RenameSheet failed: %v", err)
	}
	wb.RecalculateAll()
	if v, ok := wbSh1.GetCellValue(0, 0).(float64); !ok || v != 84.0 {
		t.Errorf("Expected Inventory!A1 * 2 = 84, got %v", wbSh1.GetCellValue(0, 0))
	}

	// Delete Inventory sheet (index 1)
	if err := wb.DeleteSheet(1); err != nil {
		t.Fatalf("DeleteSheet failed: %v", err)
	}
	wb.RecalculateAll()
	if v := wbSh1.GetCellValue(0, 0); v != cell.ErrRef {
		t.Errorf("Expected #REF! after sheet delete, got %v", v)
	}

	// 3. Import empty/corrupt XLSX and ODS guard (no panic)
	tmpDir := t.TempDir()
	emptyZip := filepath.Join(tmpDir, "corrupt.xlsx")
	// Write invalid/empty zip
	os.WriteFile(emptyZip, []byte("NOT_A_ZIP"), 0644)
	if _, err := sheet.ImportXLSX(emptyZip, 0); err == nil {
		t.Errorf("Expected error for corrupt xlsx, got nil")
	}

	emptyOds := filepath.Join(tmpDir, "corrupt.ods")
	os.WriteFile(emptyOds, []byte("NOT_A_ZIP"), 0644)
	if _, err := sheet.ImportODS(emptyOds, 0); err == nil {
		t.Errorf("Expected error for corrupt ods, got nil")
	}
}

func TestP6ComprehensiveAuditFixes(t *testing.T) {
	sh := sheet.NewSheet()

	// 1. POWER, power operator right-associativity
	sh.SetCellInput(0, 0, "=POWER(0, -1)", nil)
	sh.SetCellInput(0, 1, "=POWER(-1, 0.5)", nil)
	sh.SetCellInput(0, 2, "=2^3^2", nil) // 2^(3^2) = 2^9 = 512
	sh.Recalculate()
	if sh.GetCellValue(0, 0) != cell.ErrLotus {
		t.Errorf("Expected POWER(0, -1) = ErrLotus, got %v", sh.GetCellValue(0, 0))
	}
	if sh.GetCellValue(0, 1) != cell.ErrLotus {
		t.Errorf("Expected POWER(-1, 0.5) = ErrLotus, got %v", sh.GetCellValue(0, 1))
	}
	if sh.GetCellValue(0, 2) != 512.0 {
		t.Errorf("Expected 2^3^2 = 512 (right-associative), got %v", sh.GetCellValue(0, 2))
	}

	// 2. CEILING / FLOOR with sig == 0
	sh.SetCellInput(1, 0, "=CEILING(10, 0)", nil)
	sh.SetCellInput(1, 1, "=FLOOR(10, 0)", nil)
	sh.Recalculate()
	if sh.GetCellValue(1, 0) != cell.ErrLotus {
		t.Errorf("Expected CEILING(10, 0) = ErrLotus, got %v", sh.GetCellValue(1, 0))
	}
	if sh.GetCellValue(1, 1) != cell.ErrLotus {
		t.Errorf("Expected FLOOR(10, 0) = ErrLotus, got %v", sh.GetCellValue(1, 1))
	}

	// 3. 2-argument IF
	sh.SetCellInput(2, 0, `=IF(1>0, "YES")`, nil)
	sh.SetCellInput(2, 1, `=IF(1<0, "YES")`, nil)
	sh.Recalculate()
	if sh.GetCellValue(2, 0) != "YES" {
		t.Errorf("Expected IF(1>0, 'YES') = 'YES', got %v", sh.GetCellValue(2, 0))
	}
	if sh.GetCellValue(2, 1) != 0.0 {
		t.Errorf("Expected IF(1<0, 'YES') = 0.0, got %v", sh.GetCellValue(2, 1))
	}

	// 4. VLOOKUP / HLOOKUP 1-based index strictness
	sh.SetCellInput(0, 3, "10", nil)                            // A4 = 10
	sh.SetCellInput(1, 3, "Alpha", nil)                         // B4 = Alpha
	sh.SetCellInput(2, 3, "=VLOOKUP(10, A4:B4, 0, FALSE)", nil) // col 0 is invalid
	sh.SetCellInput(3, 3, "=VLOOKUP(10, A4:B4, 1, FALSE)", nil) // col 1 returns 10
	sh.SetCellInput(4, 3, "=VLOOKUP(10, A4:B4, 2, FALSE)", nil) // col 2 returns Alpha
	sh.Recalculate()
	if sh.GetCellValue(2, 3) != cell.ErrLotus {
		t.Errorf("Expected VLOOKUP with col 0 = ErrLotus, got %v", sh.GetCellValue(2, 3))
	}
	if sh.GetCellValue(3, 3) != 10.0 {
		t.Errorf("Expected VLOOKUP with col 1 = 10.0, got %v", sh.GetCellValue(3, 3))
	}
	if sh.GetCellValue(4, 3) != "Alpha" {
		t.Errorf("Expected VLOOKUP with col 2 = Alpha, got %v", sh.GetCellValue(4, 3))
	}

	// 5. CHOOSE 1-based strictness
	sh.SetCellInput(4, 0, `=CHOOSE(0, "A", "B")`, nil)
	sh.SetCellInput(4, 1, `=CHOOSE(1, "A", "B")`, nil)
	sh.Recalculate()
	if sh.GetCellValue(4, 0) != cell.ErrLotus {
		t.Errorf("Expected CHOOSE(0) = ErrLotus, got %v", sh.GetCellValue(4, 0))
	}
	if sh.GetCellValue(4, 1) != "A" {
		t.Errorf("Expected CHOOSE(1) = 'A', got %v", sh.GetCellValue(4, 1))
	}

	// 6. LEFT / RIGHT negative count
	sh.SetCellInput(5, 0, `=LEFT("ABC", -1)`, nil)
	sh.SetCellInput(5, 1, `=RIGHT("ABC", -1)`, nil)
	sh.Recalculate()
	if sh.GetCellValue(5, 0) != cell.ErrLotus {
		t.Errorf("Expected LEFT(..., -1) = ErrLotus, got %v", sh.GetCellValue(5, 0))
	}
	if sh.GetCellValue(5, 1) != cell.ErrLotus {
		t.Errorf("Expected RIGHT(..., -1) = ErrLotus, got %v", sh.GetCellValue(5, 1))
	}

	// 7. NUMBERVALUE percent
	sh.SetCellInput(6, 0, `=NUMBERVALUE("50%")`, nil)
	sh.Recalculate()
	if sh.GetCellValue(6, 0) != 0.5 {
		t.Errorf("Expected NUMBERVALUE('50%%') = 0.5, got %v", sh.GetCellValue(6, 0))
	}

	// 8. TEXT with multibyte currencies
	sh.SetCellInput(7, 0, `=TEXT(1234.56, "¥#,##0")`, nil)
	sh.SetCellInput(7, 1, `=TEXT(1234.56, "€#,##0")`, nil)
	sh.Recalculate()
	if sh.GetCellValue(7, 0) != "¥1,235" {
		t.Errorf("Expected '¥1,235', got %v", sh.GetCellValue(7, 0))
	}
	if sh.GetCellValue(7, 1) != "€1,235" {
		t.Errorf("Expected '€1,235', got %v", sh.GetCellValue(7, 1))
	}

	// 9. Quoted sheet name with whole-row reference lexing
	lex := formula.NewLexer("='My Sheet'!1:10")
	toks, err := lex.Tokenize()
	if err != nil {
		t.Fatalf("Lexing 'My Sheet'!1:10 failed: %v", err)
	}
	foundRange := false
	for _, tok := range toks {
		if tok.Type == formula.TokRangeRef && tok.Value == "'My Sheet'!1..10" {
			foundRange = true
		}
	}
	if !foundRange {
		t.Errorf("Expected TokRangeRef for 'My Sheet'!1..10, got tokens: %+v", toks)
	}

	// 10. FormatRange pointer isolation
	fmtSpec := cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2}
	rRef, _ := coord.ParseRangeRef("A1:B1")
	sh.FormatRange(rRef, fmtSpec)
	c0 := sh.GetCell(0, 0)
	c1 := sh.GetCell(1, 0)
	if c0.FormatSpec == c1.FormatSpec {
		t.Errorf("FormatRange cells share the same pointer!")
	}
	c0.FormatSpec.Decimals = 5
	if c1.FormatSpec.Decimals == 5 {
		t.Errorf("Mutating c0 FormatSpec mutated c1 FormatSpec!")
	}

	// 11. OFFSET boundary checks
	sh.SetCellInput(8, 0, "=OFFSET(A1, -10, 0)", nil)
	sh.SetCellInput(8, 1, "=OFFSET(A1, 0, 0, 0, 1)", nil)
	sh.Recalculate()
	if sh.GetCellValue(8, 0) != cell.ErrLotus {
		t.Errorf("Expected OFFSET with negative row = ErrLotus, got %v", sh.GetCellValue(8, 0))
	}
	if sh.GetCellValue(8, 1) != cell.ErrLotus {
		t.Errorf("Expected OFFSET with height 0 = ErrLotus, got %v", sh.GetCellValue(8, 1))
	}
}

func TestCritical_SumIfWithBlankAlignment(t *testing.T) {
	sh := sheet.NewSheet()
	// Criteria: 10, blank, 30  — Sum range: 100, 200, 300
	// Matching ">0" should sum 100+300=400 (not 100+200 if blanks collapse).
	sh.SetCellInput(0, 0, "10", nil)
	// A2 left blank
	sh.SetCellInput(0, 2, "30", nil)
	sh.SetCellInput(1, 0, "100", nil)
	sh.SetCellInput(1, 1, "200", nil)
	sh.SetCellInput(1, 2, "300", nil)
	sh.SetCellInput(2, 0, "=SUMIF(A1:A3,\">0\",B1:B3)", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 400 {
		t.Errorf("SUMIF with blank in criteria range: want 400, got %v", sh.GetCellValue(2, 0))
	}

	sh.SetCellInput(2, 1, "=COUNTBLANK(A1:A3)", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(2, 1).(float64); !ok || v != 1 {
		t.Errorf("COUNTBLANK(A1:A3) with one empty: want 1, got %v", sh.GetCellValue(2, 1))
	}
}

func TestCritical_ODSLargeColumnRepeatKeepsAlignment(t *testing.T) {
	tmp := t.TempDir()
	odsPath := filepath.Join(tmp, "repeat.ods")

	// Minimal ODS: A1=left, then 1500 empty cols skipped, then cell at column 1501 (=B... with 0-based col 1501)
	content := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
 xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0"
 xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
 <office:body><office:spreadsheet>
  <table:table table:name="Sheet1">
   <table:table-row>
    <table:table-cell office:value-type="string"><text:p>LEFT</text:p></table:table-cell>
    <table:table-cell table:number-columns-repeated="1500"/>
    <table:table-cell office:value-type="float" office:value="42"><text:p>42</text:p></table:table-cell>
   </table:table-row>
  </table:table>
 </office:spreadsheet></office:body>
</office:document-content>`

	f, err := os.Create(odsPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("content.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	// mimetype is optional for our importer (only needs content.xml)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	wb, err := sheet.ImportODSWorkbook(odsPath)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	sh := wb.GetActiveSheet()
	if c := sh.GetCell(0, 0); c == nil || fmt.Sprintf("%v", c.Value) != "LEFT" {
		t.Fatalf("A1 want LEFT, got %#v", c)
	}
	// After LEFT (col 0) + 1500 empty = next data at col 1501
	c := sh.GetCell(1501, 0)
	if c == nil {
		t.Fatalf("expected value at column 1501, cell missing (likely collapsed repeat)")
	}
	if v, ok := c.Value.(float64); !ok || v != 42 {
		t.Errorf("col 1501 want 42, got %v", c.Value)
	}
	// Must not appear at col 1 (the old bug: 1500→1)
	if bad := sh.GetCell(1, 0); bad != nil {
		t.Errorf("column 1 should be empty after large skip, got %v", bad.Value)
	}
}

func TestCritical_CSVImportThenSaveWritesImportedData(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "imported.csv")
	if err := os.WriteFile(csvPath, []byte("Name,Value\nAlpha,99\n"), 0644); err != nil {
		t.Fatal(err)
	}

	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()

	old := sheet.NewSheet()
	old.SetCellInput(0, 0, "OLD_DATA", nil)
	app := tui.NewApp(s, old, "old.hwk")
	app.ImportCSVFileForTest(csvPath)

	savePath := filepath.Join(tmp, "after_import.hwk")
	wb := app.ActiveSheet().Workbook()
	if wb == nil {
		t.Fatal("workbook is nil after CSV import")
	}
	if err := wb.SaveJSON(savePath); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}
	loaded, err := sheet.LoadWorkbookJSON(savePath)
	if err != nil {
		t.Fatalf("LoadWorkbookJSON: %v", err)
	}
	sh := loaded.GetActiveSheet()
	c := sh.GetCell(0, 0)
	if c == nil {
		t.Fatal("A1 missing in saved workbook")
	}
	got := fmt.Sprintf("%v", c.Value)
	if got != "Name" && c.RawInput != "'Name" && !strings.Contains(c.RawInput, "Name") {
		// Label cells store Value as content without quote
		if got != "Name" {
			t.Errorf("saved A1 want Name from CSV, got raw=%q value=%v (OLD_DATA would mean workbook not updated)", c.RawInput, c.Value)
		}
	}
	if v := sh.GetCell(1, 1); v == nil {
		t.Fatal("B2 missing")
	} else if fv, ok := v.Value.(float64); !ok || fv != 99 {
		t.Errorf("B2 want 99, got %v", v.Value)
	}
	if sh.GetCell(0, 0) != nil {
		if strings.Contains(fmt.Sprintf("%v", sh.GetCell(0, 0).Value), "OLD") {
			t.Error("saved file still contains OLD_DATA — CSV was not wired into workbook")
		}
	}
}

func TestP7FinalAuditFixes(t *testing.T) {
	sh := sheet.NewSheet()

	// 1. ISBLANK and ISLOGICAL
	sh.SetCellInput(0, 0, `=ISBLANK("")`, nil)
	sh.SetCellInput(1, 0, `=ISBLANK(Z100)`, nil) // empty cell
	sh.SetCellInput(2, 0, `=ISLOGICAL(1)`, nil)
	sh.SetCellInput(3, 0, `=ISLOGICAL(0)`, nil)
	sh.SetCellInput(4, 0, `=ISLOGICAL(2)`, nil)
	sh.SetCellInput(5, 0, `=ISLOGICAL(1>0)`, nil)
	sh.SetCellInput(6, 0, `=ISLOGICAL(TRUE())`, nil)
	sh.SetCellInput(7, 0, `=ISLOGICAL("TRUE")`, nil)
	sh.Recalculate()
	if sh.GetCellValue(0, 0) != 0.0 {
		t.Errorf("Expected ISBLANK('') = 0, got %v", sh.GetCellValue(0, 0))
	}
	if sh.GetCellValue(1, 0) != 1.0 {
		t.Errorf("Expected ISBLANK(Z100) = 1, got %v", sh.GetCellValue(1, 0))
	}
	if sh.GetCellValue(2, 0) != 0.0 {
		t.Errorf("Expected ISLOGICAL(1) = 0, got %v", sh.GetCellValue(2, 0))
	}
	if sh.GetCellValue(3, 0) != 0.0 {
		t.Errorf("Expected ISLOGICAL(0) = 0, got %v", sh.GetCellValue(3, 0))
	}
	if sh.GetCellValue(4, 0) != 0.0 {
		t.Errorf("Expected ISLOGICAL(2) = 0, got %v", sh.GetCellValue(4, 0))
	}
	if sh.GetCellValue(5, 0) != 1.0 {
		t.Errorf("Expected ISLOGICAL(1>0) = 1, got %v", sh.GetCellValue(5, 0))
	}
	if sh.GetCellValue(6, 0) != 1.0 {
		t.Errorf("Expected ISLOGICAL(TRUE()) = 1, got %v", sh.GetCellValue(6, 0))
	}
	if sh.GetCellValue(7, 0) != 0.0 {
		t.Errorf("Expected ISLOGICAL(\"TRUE\") = 0, got %v", sh.GetCellValue(7, 0))
	}

	// 2. MODE with no duplicates returns ErrNA
	sh.SetCellInput(0, 1, "=MODE(10)", nil)
	sh.SetCellInput(1, 1, "=MODE(1, 2, 3)", nil)
	sh.SetCellInput(2, 1, "=MODE(1, 2, 2, 3)", nil)
	sh.Recalculate()
	if sh.GetCellValue(0, 1) != cell.ErrNA {
		t.Errorf("Expected MODE(10) = ErrNA, got %v", sh.GetCellValue(0, 1))
	}
	if sh.GetCellValue(1, 1) != cell.ErrNA {
		t.Errorf("Expected MODE(1,2,3) = ErrNA, got %v", sh.GetCellValue(1, 1))
	}
	if sh.GetCellValue(2, 1) != 2.0 {
		t.Errorf("Expected MODE(1,2,2,3) = 2.0, got %v", sh.GetCellValue(2, 1))
	}

	// 3. RANDBETWEEN ceiling bounds
	sh.SetCellInput(0, 2, "=RANDBETWEEN(1.1, 1.9)", nil) // no integers in range -> ErrLotus
	sh.SetCellInput(1, 2, "=RANDBETWEEN(1.5, 2.5)", nil) // only 2 is in range
	sh.Recalculate()
	if sh.GetCellValue(0, 2) != cell.ErrLotus {
		t.Errorf("Expected RANDBETWEEN(1.1, 1.9) = ErrLotus, got %v", sh.GetCellValue(0, 2))
	}
	if sh.GetCellValue(1, 2) != 2.0 {
		t.Errorf("Expected RANDBETWEEN(1.5, 2.5) = 2.0, got %v", sh.GetCellValue(1, 2))
	}

	// 4. REPT character length limit
	sh.SetCellInput(0, 3, `=REPT("ABC", 20000)`, nil) // 60,000 chars > 32767 limit -> ErrLotus
	sh.Recalculate()
	if sh.GetCellValue(0, 3) != cell.ErrLotus {
		t.Errorf("Expected REPT exceeding 32767 chars = ErrLotus, got %v", sh.GetCellValue(0, 3))
	}

	// 5. WEEKNUM return_type and ISOWEEKNUM
	sh.SetCellInput(0, 4, `=WEEKNUM(DATE(2026, 1, 4), 1)`, nil) // 2026-01-04 is Sunday -> Week 2 (starts Sunday)
	sh.SetCellInput(1, 4, `=WEEKNUM(DATE(2026, 1, 4), 2)`, nil) // starts Monday -> Week 1
	sh.SetCellInput(2, 4, `=ISOWEEKNUM(DATE(2026, 1, 4))`, nil) // ISO week for 2026-01-04 is 1
	sh.Recalculate()
	if sh.GetCellValue(0, 4) != 2.0 {
		t.Errorf("Expected WEEKNUM(2026-01-04, 1) = 2, got %v", sh.GetCellValue(0, 4))
	}
	if sh.GetCellValue(1, 4) != 1.0 {
		t.Errorf("Expected WEEKNUM(2026-01-04, 2) = 1, got %v", sh.GetCellValue(1, 4))
	}
	if sh.GetCellValue(2, 4) != 1.0 {
		t.Errorf("Expected ISOWEEKNUM(2026-01-04) = 1, got %v", sh.GetCellValue(2, 4))
	}

	// 6. compareEqual: "" should not equal 0.0
	sh.SetCellInput(0, 5, `=IF(""=0, "EQ", "NEQ")`, nil)
	sh.Recalculate()
	if sh.GetCellValue(0, 5) != "NEQ" {
		t.Errorf("Expected IF(''=0, 'EQ', 'NEQ') = 'NEQ', got %v", sh.GetCellValue(0, 5))
	}

	// 7. MoveRange updates formula references
	shMove := sheet.NewSheet()
	shMove.SetCellInput(0, 0, "10", nil)     // A1 = 10
	shMove.SetCellInput(1, 0, "20", nil)     // B1 = 20
	shMove.SetCellInput(0, 1, "=A1+B1", nil) // A2 = A1+B1 = 30
	fromR, _ := coord.ParseRangeRef("A2:A2")
	toR, _ := coord.ParseRangeRef("C5:C5") // deltaCol=+2, deltaRow=+3 -> should become =C4+D4
	shMove.MoveRange(fromR, toR)
	cMoved := shMove.GetCell(2, 4)
	if cMoved == nil || (cMoved.RawInput != "=C4+D4" && cMoved.RawInput != "=(C4+D4)") {
		t.Errorf("Expected moved formula to be =C4+D4, got %q", cMoved.RawInput)
	}

	// 8. SheetSnapshot preserves namedRanges on Undo
	shSnap := sheet.NewSheet()
	rTest, _ := coord.ParseRangeRef("A1:B10")
	shSnap.SetNamedRange("MyData", rTest)
	snap := shSnap.CreateSnapshot()
	shSnap.SetNamedRange("MyData", coord.RangeRef{}) // delete/overwrite
	shSnap.RestoreSnapshot(snap)
	if _, exists := shSnap.GetNamedRange("MyData"); !exists {
		t.Errorf("NamedRange 'MyData' was not restored from snapshot!")
	}

	// 9. Unterminated strings in Lexer return error
	lex1 := formula.NewLexer(`"unterminated string`)
	if _, err := lex1.Tokenize(); err == nil {
		t.Errorf("Expected error for unterminated double quote, got nil")
	}
	lex2 := formula.NewLexer(`'unterminated sheet!A1`)
	if _, err := lex2.Tokenize(); err == nil {
		t.Errorf("Expected error for unterminated single quote, got nil")
	}
}

func TestHighPriority_TrueFalseAndUnaryPow(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=TRUE", nil)
	sh.SetCellInput(0, 1, "=FALSE", nil)
	sh.SetCellInput(0, 2, "=-2^2", nil)
	sh.SetCellInput(0, 3, "=2^-2", nil)
	sh.SetCellInput(0, 4, "=AND(1, 1/0)", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(0, 0).(bool); !ok || !v {
		t.Errorf("TRUE want true, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(bool); !ok || v {
		t.Errorf("FALSE want false, got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 4 {
		t.Errorf("-2^2 want 4, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 0.25 {
		t.Errorf("2^-2 want 0.25, got %v", sh.GetCellValue(0, 3))
	}
	if _, ok := sh.GetCellValue(0, 4).(cell.LotusError); !ok {
		t.Errorf("AND(1,1/0) want error, got %v", sh.GetCellValue(0, 4))
	}
}

func TestHighPriority_CrossSheetRecalcAndRename(t *testing.T) {
	wb := sheet.NewWorkbook("test")
	s1 := wb.Sheets[0]
	s1.SetName("Data")
	s2 := wb.AddSheet("Summary")
	s1.SetCellInput(0, 0, "10", nil)
	s2.SetCellInput(0, 0, "=Data!A1*2", nil)
	wb.RecalculateAll()
	if v, ok := s2.GetCellValue(0, 0).(float64); !ok || v != 20 {
		t.Fatalf("Summary want 20, got %v", s2.GetCellValue(0, 0))
	}
	s1.SetCellInput(0, 0, "15", nil)
	if v, ok := s2.GetCellValue(0, 0).(float64); !ok || v != 30 {
		t.Errorf("after Data change, Summary want 30, got %v", s2.GetCellValue(0, 0))
	}
	if err := wb.RenameSheet("Data", "Src"); err != nil {
		t.Fatal(err)
	}
	raw := s2.GetCell(0, 0).RawInput
	if !strings.Contains(strings.ToUpper(raw), "SRC!") {
		t.Errorf("rename should update formula, got %q", raw)
	}
}

func TestHighPriority_InsertRowSheetScoped(t *testing.T) {
	wb := sheet.NewWorkbook("test")
	s1 := wb.Sheets[0]
	s1.SetName("A")
	s2 := wb.AddSheet("B")
	s1.SetCellInput(0, 1, "100", nil)
	s2.SetCellInput(0, 0, "=A!A2", nil)
	s1.SetCellInput(1, 0, "=B!A1", nil)
	wb.RecalculateAll()
	s1.InsertRow(1, 1)
	if got := s2.GetCell(0, 0).RawInput; !strings.Contains(got, "A3") {
		t.Errorf("B's ref to A!A2 should become A3 after insert, got %q", got)
	}
	if got := s1.GetCell(1, 0).RawInput; strings.Contains(got, "A2") {
		t.Errorf("A's ref to B!A1 should not shift, got %q", got)
	}
}

func TestHighPriority_DeleteSheetActiveIndex(t *testing.T) {
	wb := sheet.NewWorkbook("test")
	wb.AddSheet("B")
	wb.AddSheet("C")
	wb.ActiveSheetIndex = 1
	if err := wb.DeleteSheet(0); err != nil {
		t.Fatal(err)
	}
	if wb.ActiveSheetIndex != 0 || wb.Sheets[0].Name() != "B" {
		t.Errorf("after deleting sheet before active, want active B at 0, got idx=%d name=%s",
			wb.ActiveSheetIndex, wb.Sheets[wb.ActiveSheetIndex].Name())
	}
}

func TestHighPriority_ToExcelExportFormula(t *testing.T) {
	got := sheet.ExcelExportFormulaForTest("=@SUM(A1..B10)")
	if got != "SUM(A1:B10)" {
		t.Errorf("want SUM(A1:B10), got %q", got)
	}
}

func TestUnfinished_CutPasteKeepsFormulaRefs(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "10", nil)    // A1
	sh.SetCellInput(1, 0, "=A1*2", nil) // B1
	sh.Recalculate()

	// Simulate cut: snapshot B1 then clear, paste at D1 without relative adjust.
	src := sh.GetCell(1, 0)
	clipRaw := src.RawInput
	sh.ClearCell(1, 0)
	sh.SuspendRecalc()
	sh.SetCellInput(3, 0, clipRaw, nil) // cut paste: no AdjustFormulaReferences
	sh.EndSuspendRecalc()

	if got := sh.GetCell(3, 0).RawInput; got != "=A1*2" {
		t.Errorf("cut paste should keep =A1*2, got %q", got)
	}
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 20 {
		t.Errorf("D1 want 20, got %v", sh.GetCellValue(3, 0))
	}

	// Copy paste should shift: B1(=A1*2) → D1 becomes =(C1*2) or =C1*2
	adj := formula.AdjustFormulaReferences("=A1*2", 2, 0) // B→D
	if !strings.Contains(adj, "C1") {
		t.Errorf("copy paste shift want C1 ref, got %q", adj)
	}
}

func TestUnfinished_MultiSheetUndoReplace(t *testing.T) {
	wb := sheet.NewWorkbook("wb")
	s1 := wb.Sheets[0]
	s1.SetName("Sheet1")
	s2 := wb.AddSheet("Sheet2")
	s1.SetCellInput(0, 0, "foo", nil)
	s2.SetCellInput(0, 0, "foo", nil)

	um := sheet.NewUndoManager(10)
	um.PushMulti([]*sheet.Sheet{s1, s2})
	s1.SetCellInput(0, 0, "bar", nil)
	s2.SetCellInput(0, 0, "bar", nil)

	resolve := func(name string) *sheet.Sheet { return wb.GetSheet(name) }
	if !um.Undo(s1, resolve, wb, nil) {
		t.Fatal("multi undo failed")
	}
	if !strings.Contains(strings.ToLower(fmt.Sprintf("%v", s1.GetCellValue(0, 0))), "foo") {
		t.Errorf("Sheet1 want foo, got %v", s1.GetCellValue(0, 0))
	}
	if !strings.Contains(strings.ToLower(fmt.Sprintf("%v", s2.GetCellValue(0, 0))), "foo") {
		t.Errorf("Sheet2 want foo, got %v", s2.GetCellValue(0, 0))
	}
}

func TestUnfinished_RelativeNamedFormulaNoFalseCirc(t *testing.T) {
	wb := sheet.NewWorkbook("names")
	sh := wb.Sheets[0]
	sh.SetName("Cal")
	// Named formula used from many cells; later cells OFFSET into earlier =DayOfWeek cells.
	wb.SetNamedRange("DayOfWeek", "=OFFSET(A1, ROW()-4, 0)")
	sh.SetCellInput(0, 0, "1", nil)          // A1
	sh.SetCellInput(0, 3, "=DayOfWeek", nil) // A4 → A1
	sh.SetCellInput(0, 6, "=DayOfWeek", nil) // A7 → A4 (=DayOfWeek) — must not false-circ on name
	wb.RecalculateAll()
	for _, row := range []int{3, 6} {
		v := sh.GetCellValue(0, row)
		if _, isErr := v.(cell.LotusError); isErr {
			t.Fatalf("A%d got error %v (false circular?)", row+1, v)
		}
		if f, ok := v.(float64); !ok || f != 1 {
			t.Errorf("A%d want 1, got %v", row+1, v)
		}
	}
}

func TestUnfinished_WildcardUnicode(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "あいう", nil)
	sh.SetCellInput(0, 1, "あいえ", nil)
	sh.SetCellInput(1, 0, `=COUNTIF(A1:A2,"あ?う")`, nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 1 {
		t.Errorf(`COUNTIF "あ?う" want 1, got %v`, sh.GetCellValue(1, 0))
	}
	sh.SetCellInput(1, 1, `=SEARCH("?う", "あいう")`, nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 2 {
		t.Errorf(`SEARCH "?う" in あいう want 2, got %v`, sh.GetCellValue(1, 1))
	}
}

func TestFollowUp_CopyOutOfBoundsBecomesRef(t *testing.T) {
	// Copy =A1 upward by 1 row → #REF!
	got := formula.AdjustFormulaReferences("=A1", 0, -1)
	if !strings.Contains(got, "#REF!") {
		t.Fatalf("want #REF! in adjusted formula, got %q", got)
	}
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, got, nil)
	sh.Recalculate()
	if _, ok := sh.GetCellValue(0, 0).(cell.LotusError); !ok {
		t.Fatalf("evaluating #REF! formula want LotusError, got %v", sh.GetCellValue(0, 0))
	}
	if code := sh.GetCellValue(0, 0).(cell.LotusError).Code; code != "#REF!" {
		t.Errorf("want code #REF!, got %q", code)
	}
	// Absolute $A$1 must not shift into #REF!
	abs := formula.AdjustFormulaReferences("=$A$1", 0, -1)
	if strings.Contains(abs, "#REF!") {
		t.Errorf("$A$1 should stay absolute, got %q", abs)
	}
}

func TestFollowUp_ODSPreservesAbsoluteRefsAndBaseCell(t *testing.T) {
	got := sheet.ConvertODSFormulaForTest("OFFSET([.$A$1]; ROW()-3; COLUMN()-1)")
	if !strings.Contains(got, "$A$1") {
		t.Fatalf("ODS absolute ref should keep $A$1, got %q", got)
	}
	wb := sheet.NewWorkbook("odsbase")
	sh := wb.Sheets[0]
	// Relative named formula authored at B2 (=A1 relative); used at B3 → A2
	wb.SetNamedRange("RelAbove", formula.NamedExpr{
		Expr: "=A1", HasBase: true, BaseCol: 1, BaseRow: 1, // B2
	})
	sh.SetCellInput(0, 0, "10", nil)        // A1
	sh.SetCellInput(0, 1, "20", nil)        // A2
	sh.SetCellInput(1, 2, "=RelAbove", nil) // B3: shift (0,1) → A2
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(1, 2).(float64); !ok || v != 20 {
		t.Errorf("base-cell relative name at B3 want 20, got %v", sh.GetCellValue(1, 2))
	}
}

func TestFollowUp_MissingSheetIsRef(t *testing.T) {
	wb := sheet.NewWorkbook("ref")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=NoSuchSheet!A1+1", nil)
	wb.RecalculateAll()
	v := sh.GetCellValue(0, 0)
	errVal, ok := v.(cell.LotusError)
	if !ok || errVal.Code != "#REF!" {
		t.Fatalf("missing sheet want #REF!, got %v", v)
	}
}

func TestFollowUp_ExcelDateNumFmtDetection(t *testing.T) {
	if !sheet.IsExcelDateNumFmtForTest(14, "") {
		t.Error("built-in 14 should be date")
	}
	if !sheet.IsExcelDateNumFmtForTest(164, "yyyy-mm-dd") {
		t.Error("custom yyyy-mm-dd should be date")
	}
	if sheet.IsExcelDateNumFmtForTest(0, "General") {
		t.Error("General should not be date")
	}
	if sheet.IsExcelDateNumFmtForTest(164, `#,##0.00`) {
		t.Error("number format should not be date")
	}
}

func TestMedium_CopyKeepsColonAndNoParenGrowth(t *testing.T) {
	adj := formula.AdjustFormulaReferences("=SUM(A1:B10)+C1", 1, 0)
	if strings.Contains(adj, "..") {
		t.Errorf("copy should keep ':' ranges, got %q", adj)
	}
	if strings.Contains(adj, "A2:B11") || strings.Contains(adj, "B1:C10") {
		// shifted by +1 col → B1:C10
	} else if !strings.Contains(adj, "B1:C10") {
		t.Errorf("want B1:C10 in %q", adj)
	}
	// Repeated adjust must not keep wrapping extra parens around the whole expr
	once := formula.AdjustFormulaReferences("=A1+B1", 0, 1)
	twice := formula.AdjustFormulaReferences(once, 0, 1)
	if strings.Count(twice, "(") > strings.Count(once, "(")+1 {
		t.Errorf("paren growth on copy: once=%q twice=%q", once, twice)
	}
}

func TestMedium_ParenLabelNotFormula(t *testing.T) {
	c := cell.NewCell("(n/a)", nil)
	if c.Type == cell.TypeFormula {
		t.Fatalf("(n/a) should be label, got formula")
	}
	c2 := cell.NewCell("(A1+B1)", nil)
	if c2.Type != cell.TypeFormula {
		t.Fatalf("(A1+B1) should be formula, got %s", c2.Type)
	}
}

func TestMedium_SpacedColonRange(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(1, 9, "2", nil)
	sh.SetCellInput(2, 0, "=SUM(A1 : B10)", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 3 {
		t.Fatalf("SUM(A1 : B10) want 3, got %v", sh.GetCellValue(2, 0))
	}
}

func TestMedium_VLookupNAAndRankAvg(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "a", nil)
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(0, 1, "b", nil)
	sh.SetCellInput(1, 1, "20", nil)
	sh.SetCellInput(2, 0, `=VLOOKUP("z",A1:B2,2,FALSE)`, nil)
	sh.SetCellInput(3, 0, "5", nil)
	sh.SetCellInput(3, 1, "5", nil)
	sh.SetCellInput(3, 2, "1", nil)
	sh.SetCellInput(4, 0, "=RANK.AVG(5,D1:D3)", nil)
	sh.SetCellInput(4, 1, "=RANK.EQ(5,D1:D3)", nil)
	sh.Recalculate()
	if errVal, ok := sh.GetCellValue(2, 0).(cell.LotusError); !ok || errVal.Code != "NA" {
		t.Errorf("VLOOKUP miss want NA, got %v", sh.GetCellValue(2, 0))
	}
	// Two 5s and one 1 descending: ranks 1 and 2 for the 5s → avg 1.5
	if v, ok := sh.GetCellValue(4, 0).(float64); !ok || v != 1.5 {
		t.Errorf("RANK.AVG want 1.5, got %v", sh.GetCellValue(4, 0))
	}
	if v, ok := sh.GetCellValue(4, 1).(float64); !ok || v != 1 {
		t.Errorf("RANK.EQ want 1, got %v", sh.GetCellValue(4, 1))
	}
}

func TestMedium_SetCellInputClampsAndNamedRangeCase(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(999999, 0, "x", nil) // beyond maxCols
	if sh.GetCell(999999, 0) != nil {
		t.Fatal("out-of-bounds SetCellInput should be ignored")
	}
	sh.SetNamedRange("Sales", coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 0},
	})
	got, ok := sh.GetNamedRange("sales")
	if !ok {
		t.Fatal("GetNamedRange should be case-insensitive")
	}
	_ = got
}

func TestMedium_WorkbookStructuralUndo(t *testing.T) {
	wb := sheet.NewWorkbook("struct")
	s1 := wb.Sheets[0]
	s1.SetName("Alpha")
	s1.SetCellInput(0, 0, "keep", nil)
	um := sheet.NewUndoManager(10)

	um.PushWorkbook(wb)
	s2 := wb.AddSheet("Beta")
	s2.SetCellInput(0, 0, "new", nil)
	if len(wb.Sheets) != 2 {
		t.Fatalf("want 2 sheets after add, got %d", len(wb.Sheets))
	}
	if !um.Undo(s1, nil, wb, nil) {
		t.Fatal("undo add failed")
	}
	if len(wb.Sheets) != 1 || wb.Sheets[0].Name() != "Alpha" {
		t.Fatalf("undo add want only Alpha, got %v", wb.SheetNames())
	}
	if !strings.Contains(fmt.Sprintf("%v", wb.Sheets[0].GetCellValue(0, 0)), "keep") {
		t.Errorf("Alpha content lost after undo add: %v", wb.Sheets[0].GetCellValue(0, 0))
	}

	um.PushWorkbook(wb)
	_ = wb.AddSheet("Beta")
	wb.Sheets[1].SetCellInput(1, 0, "x", nil)
	um.PushWorkbook(wb)
	if err := wb.DeleteSheet(1); err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 1 {
		t.Fatal("delete failed")
	}
	if !um.Undo(nil, nil, wb, nil) {
		t.Fatal("undo delete failed")
	}
	if len(wb.Sheets) != 2 || wb.GetSheet("Beta") == nil {
		t.Fatalf("undo delete should restore Beta, got %v", wb.SheetNames())
	}

	um.PushWorkbook(wb)
	if err := wb.RenameSheet("Beta", "Gamma"); err != nil {
		t.Fatal(err)
	}
	if !um.Undo(nil, nil, wb, nil) {
		t.Fatal("undo rename failed")
	}
	if wb.GetSheet("Beta") == nil || wb.GetSheet("Gamma") != nil {
		t.Fatalf("undo rename want Beta back, got %v", wb.SheetNames())
	}
}

func TestLowPriority_Serial60AndLogical(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=DATE(1900,2,28)", nil)
	sh.SetCellInput(1, 0, "=DATE(1900,3,1)", nil)
	sh.Recalculate()
	s28, ok28 := sh.GetCellValue(0, 0).(float64)
	sMar1, okMar := sh.GetCellValue(1, 0).(float64)
	if !ok28 || !okMar {
		t.Fatalf("DATE serials: %v %v", sh.GetCellValue(0, 0), sh.GetCellValue(1, 0))
	}
	// Excel leap bug: serial 60 is the fictional 1900-02-29 between 59 and 61.
	if int(s28) != 59 || int(sMar1) != 61 {
		t.Fatalf("want DATE(1900,2,28)=59 and DATE(1900,3,1)=61, got %v and %v", s28, sMar1)
	}

	c := &cell.Cell{Value: float64(60)}
	if got := c.Render(12, cell.CellFormat{Type: cell.FmtDate}); !strings.Contains(got, "1900/02/29") && !strings.Contains(got, "29") {
		t.Errorf("serial 60 date format want 1900-02-29, got %q", got)
	}

	sh.SetCellInput(0, 1, "=YEAR(60)", nil)
	sh.SetCellInput(1, 1, "=MONTH(60)", nil)
	sh.SetCellInput(2, 1, "=DAY(60)", nil)
	sh.SetCellInput(3, 1, "=TRUE()", nil)
	sh.SetCellInput(4, 1, "=ISLOGICAL(TRUE)", nil)
	sh.SetCellInput(5, 1, "=ISLOGICAL(1)", nil)
	sh.Recalculate()
	if sh.GetCellValue(0, 1) != 1900.0 {
		t.Errorf("YEAR(60) want 1900, got %v", sh.GetCellValue(0, 1))
	}
	if sh.GetCellValue(1, 1) != 2.0 {
		t.Errorf("MONTH(60) want 2, got %v", sh.GetCellValue(1, 1))
	}
	if sh.GetCellValue(2, 1) != 29.0 {
		t.Errorf("DAY(60) want 29, got %v", sh.GetCellValue(2, 1))
	}
	if v, ok := sh.GetCellValue(3, 1).(bool); !ok || !v {
		t.Errorf("TRUE() want true, got %v", sh.GetCellValue(3, 1))
	}
	if sh.GetCellValue(4, 1) != 1.0 {
		t.Errorf("ISLOGICAL(TRUE) want 1, got %v", sh.GetCellValue(4, 1))
	}
	if sh.GetCellValue(5, 1) != 0.0 {
		t.Errorf("ISLOGICAL(1) want 0, got %v", sh.GetCellValue(5, 1))
	}
}
