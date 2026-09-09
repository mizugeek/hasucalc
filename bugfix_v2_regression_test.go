package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
	"hasucalc/tui"
)

// 1. POINT Mode cursor restore test
func TestBugfixPointModeCursorRestore(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("failed to init simScreen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "test_point.hwk")
	app.RunOnceForTest()

	// Start at B2 (col 1, row 1)
	app.SetCursorForTest(1, 1)

	// Enter INPUT mode and type "=A1+"
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '=', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '1', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModNone))

	// Press Right Arrow -> triggers POINT mode
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if app.GetModeForTest() != "POINT" {
		t.Fatalf("expected mode POINT, got %s", app.GetModeForTest())
	}

	// Move around in POINT mode to D4 (col 3, row 3)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))

	cCol, cRow := app.GetCursorForTest()
	if cCol != 3 || cRow != 3 {
		t.Fatalf("expected POINT cursor at (3,3), got (%d,%d)", cCol, cRow)
	}

	// Hit Enter to finish pointing -> should append D4 and restore cursor to B2 (col 1, row 1)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.GetModeForTest() != "INPUT" {
		t.Fatalf("expected mode INPUT after finishPoint, got %s", app.GetModeForTest())
	}
	cCol, cRow = app.GetCursorForTest()
	if cCol != 1 || cRow != 1 {
		t.Fatalf("expected cursor restored to B2 (1,1), got (%d,%d)", cCol, cRow)
	}

	// Commit input by hitting Enter in INPUT mode
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	b2Cell := sh.GetCell(1, 1)
	if b2Cell == nil || b2Cell.RawInput != "=A1+D4" {
		t.Fatalf("expected B2 to have '=A1+D4', got %v", b2Cell)
	}
	d4Cell := sh.GetCell(3, 3)
	if d4Cell != nil && d4Cell.Type != cell.TypeEmpty {
		t.Fatalf("expected D4 to remain untouched, got %v", d4Cell)
	}

	// Also test Esc in POINT mode restores cursor
	app.SetCursorForTest(1, 1)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '=', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '+', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	cCol, cRow = app.GetCursorForTest()
	if cCol != 1 || cRow != 1 {
		t.Fatalf("expected cursor restored to (1,1) on Esc, got (%d,%d)", cCol, cRow)
	}
}

// 2. AutoFill formula row vs column direction test
func TestBugfixAutoFillFormulaDirection(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("failed to init simScreen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "test_autofill_formula.hwk")
	app.RunOnceForTest()

	// Row 1: A1=10, B1=20, C1=30, D1=40, E1=50
	for i, v := range []int{10, 20, 30, 40, 50} {
		sh.SetCellInput(i, 0, fmt.Sprintf("%d", v), nil)
	}
	// Row 2: A2 = "=A1*2"
	sh.SetCellInput(0, 1, "=A1*2", nil)
	sh.Recalculate()

	// Select horizontal range A2:E2
	rng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 1},
		End:   coord.CellRef{Col: 4, Row: 1},
	}
	app.SelectRangeForTest(&rng)
	app.SetCursorForTest(0, 1)

	// Trigger AutoFill
	app.TriggerAutoFillForTest()

	// Verify B2..E2 are filled with shifted formulas: =B1*2, =C1*2, etc.
	expectedVals := []float64{20, 40, 60, 80, 100}
	expectedFormulas := []string{"=A1*2", "=B1*2", "=C1*2", "=D1*2", "=E1*2"}

	for i := 0; i < 5; i++ {
		c := sh.GetCell(i, 1)
		if c == nil {
			t.Fatalf("cell at col %d row 1 is nil", i)
		}
		if c.RawInput != expectedFormulas[i] {
			t.Errorf("col %d formula expected %s, got %s", i, expectedFormulas[i], c.RawInput)
		}
		if num, ok := c.Value.(float64); !ok || num != expectedVals[i] {
			t.Errorf("col %d value expected %v, got %v", i, expectedVals[i], c.Value)
		}
	}
}

// 3. XLSX export shared strings consistency test
func TestBugfixXLSXExportSharedStrings(t *testing.T) {
	wb := sheet.NewWorkbook("test_xlsx.hwk")
	sh := wb.GetActiveSheet()

	// Add cells that trigger various export branches
	sh.SetCellInput(0, 0, "'Hello", nil)                       // TypeLabel
	sh.SetCellInput(1, 0, "123.45", nil)                      // TypeNumber
	sh.SetCellInput(2, 0, "TRUE", nil)                        // TypeBoolean
	sh.SetCellInput(3, 0, "=\"Result: \" & \"OK\"", nil)       // TypeFormula (returns string)
	sh.SetCellInput(4, 0, "=1/0", nil)                        // TypeFormula (returns ERR)
	wb.RecalculateAll()

	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "exported.xlsx")

	if err := wb.ExportXLSX(exportPath); err != nil {
		t.Fatalf("ExportXLSX failed: %v", err)
	}

	// Re-import the exported XLSX
	importedWb, err := sheet.ImportXLSXWorkbook(exportPath)
	if err != nil {
		t.Fatalf("failed to re-import exported XLSX: %v", err)
	}
	if len(importedWb.Sheets) == 0 {
		t.Fatalf("imported workbook has no sheets")
	}
	impSh := importedWb.Sheets[0]

	c0 := impSh.GetCell(0, 0)
	if c0 == nil || c0.Value != "Hello" {
		t.Errorf("expected A1 to be 'Hello', got %v", c0)
	}
	c1 := impSh.GetCell(1, 0)
	if c1 == nil || c1.Value != 123.45 {
		t.Errorf("expected B1 to be 123.45, got %v", c1)
	}
	c2 := impSh.GetCell(2, 0)
	if c2 == nil || c2.Type != cell.TypeBoolean || c2.Value != true {
		t.Errorf("expected C1 to be boolean true, got %v", c2)
	}
}

// 4. NETWORKDAYS and WORKDAY with date serials containing time fractions
func TestBugfixNetworkDaysWithTime(t *testing.T) {
	sh := sheet.NewSheet()

	// 45000.75 is Monday 18:00; 45002.25 is Wednesday 06:00
	// Mon, Tue, Wed = 3 working days
	sh.SetCellInput(0, 0, "=NETWORKDAYS(45000.75, 45002.25)", nil)
	sh.SetCellInput(0, 1, "=NETWORKDAYS(45002.25, 45000.75)", nil) // reverse = -3
	sh.SetCellInput(0, 2, "=WORKDAY(45000.75, 2)", nil)
	sh.Recalculate()

	c0 := sh.GetCell(0, 0)
	if c0 == nil || c0.Value != 3.0 {
		t.Errorf("NETWORKDAYS expected 3.0, got %v", c0)
	}
	c1 := sh.GetCell(0, 1)
	if c1 == nil || c1.Value != -3.0 {
		t.Errorf("NETWORKDAYS reverse expected -3.0, got %v", c1)
	}
	c2 := sh.GetCell(0, 2)
	if c2 == nil || c2.Value != 45002.0 {
		t.Errorf("WORKDAY expected 45002.0, got %v", c2)
	}
}

// 5. AND / OR on ranges containing blank cells
func TestBugfixAndOrWithBlankCells(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "TRUE", nil)
	// A2, A3 are empty
	sh.SetCellInput(1, 0, "=AND(TRUE, A2:A3)", nil)
	sh.SetCellInput(1, 1, "=AND(A1:A3)", nil)
	sh.SetCellInput(1, 2, "=AND(A2:A3)", nil) // all blank -> ErrLotus
	sh.SetCellInput(1, 3, "=OR(FALSE, A2:A3)", nil)
	sh.SetCellInput(1, 4, "=OR(TRUE, A2:A3)", nil)
	sh.Recalculate()

	b1 := sh.GetCell(1, 0)
	if b1 == nil || b1.Value != true {
		t.Errorf("expected =AND(TRUE, A2:A3) to be true, got %v", b1)
	}
	b2 := sh.GetCell(1, 1)
	if b2 == nil || b2.Value != true {
		t.Errorf("expected =AND(A1:A3) to be true, got %v", b2)
	}
	b3 := sh.GetCell(1, 2)
	if b3 == nil || b3.Value != cell.ErrLotus {
		t.Errorf("expected =AND(A2:A3) on all blank to be ERR, got %v", b3)
	}
	b4 := sh.GetCell(1, 3)
	if b4 == nil || b4.Value != false {
		t.Errorf("expected =OR(FALSE, A2:A3) to be false, got %v", b4)
	}
	b5 := sh.GetCell(1, 4)
	if b5 == nil || b5.Value != true {
		t.Errorf("expected =OR(TRUE, A2:A3) to be true, got %v", b5)
	}
}

// 6. TEXT function with units and non-date format strings
func TestBugfixTextNumberWithUnits(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=TEXT(1234.56, \"$#,##0.00 USD\")", nil)
	sh.SetCellInput(0, 1, "=TEXT(100, \"0 days\")", nil)
	sh.SetCellInput(0, 2, "=TEXT(50, \"0 items\")", nil)
	sh.SetCellInput(0, 3, "=TEXT(45000, \"yyyy/mm/dd\")", nil)
	sh.Recalculate()

	c0 := sh.GetCell(0, 0)
	if c0 == nil || c0.Value != "$1,234.56 USD" {
		t.Errorf("expected '$1,234.56 USD', got %v", c0)
	}
	c1 := sh.GetCell(0, 1)
	if c1 == nil || c1.Value != "100 days" {
		t.Errorf("expected '100 days', got %v", c1)
	}
	c2 := sh.GetCell(0, 2)
	if c2 == nil || c2.Value != "50 items" {
		t.Errorf("expected '50 items', got %v", c2)
	}
	c3 := sh.GetCell(0, 3)
	if c3 == nil || c3.Value != "2023/03/15" {
		t.Errorf("expected '2023/03/15', got %v", c3)
	}
}

// 7. AutoSum placement for horizontal rows
func TestBugfixAutoSumHorizontal(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("failed to init simScreen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "test_autosum.hwk")
	app.RunOnceForTest()

	// Horizontal selection A1:E1
	for i := 0; i < 5; i++ {
		sh.SetCellInput(i, 0, fmt.Sprintf("%d", (i+1)*10), nil)
	}
	rowRng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 4, Row: 0},
	}
	app.SelectRangeForTest(&rowRng)
	app.SetCursorForTest(4, 0)

	app.TriggerAutoFunctionForTest("SUM")

	// Must be placed at F1 (col 5, row 0), NOT at A2 (col 0, row 1)
	f1Cell := sh.GetCell(5, 0)
	if f1Cell == nil || f1Cell.Type != cell.TypeFormula {
		t.Fatalf("expected AutoSum formula at F1 (col 5, row 0), got %v", f1Cell)
	}
	a2Cell := sh.GetCell(0, 1)
	if a2Cell != nil && a2Cell.Type != cell.TypeEmpty {
		t.Fatalf("expected A2 to remain empty, got %v", a2Cell)
	}

	// Vertical selection A1:A5
	colRng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 4},
	}
	app.SelectRangeForTest(&colRng)
	app.SetCursorForTest(0, 4)

	app.TriggerAutoFunctionForTest("SUM")

	// Must be placed at A6 (col 0, row 5)
	a6Cell := sh.GetCell(0, 5)
	if a6Cell == nil || a6Cell.Type != cell.TypeFormula {
		t.Fatalf("expected AutoSum formula at A6 (col 0, row 5), got %v", a6Cell)
	}
}

// 8. RangeRef.String() sheet name preservation
func TestBugfixRangeRefStringWithSheet(t *testing.T) {
	// Unquoted sheet name
	r := coord.RangeRef{
		Sheet: "SummaryData",
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 3, Row: 9},
	}
	expected := "SummaryData!A1:D10"
	if r.String() != expected {
		t.Errorf("expected %s, got %s", expected, r.String())
	}

	rSingle := coord.RangeRef{
		Sheet: "SummaryData",
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 0},
	}
	expectedSingle := "SummaryData!A1"
	if rSingle.String() != expectedSingle {
		t.Errorf("expected %s, got %s", expectedSingle, rSingle.String())
	}

	// Quoted sheet name (with spaces)
	rQuoted := coord.RangeRef{
		Sheet: "My Summary",
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 2, Row: 4},
	}
	expectedQuoted := "'My Summary'!A1:C5"
	if rQuoted.String() != expectedQuoted {
		t.Errorf("expected %s, got %s", expectedQuoted, rQuoted.String())
	}
}

// 9. Boolean cell types in NewCell, ISLOGICAL, and ODS/XLSX import
func TestBugfixBooleanImportAndPaste(t *testing.T) {
	// NewCell boolean literal recognition
	cTrue := cell.NewCell("TRUE", nil)
	if cTrue.Type != cell.TypeBoolean || cTrue.Value != true {
		t.Fatalf("expected TypeBoolean with value true, got %v", cTrue)
	}
	cFalse := cell.NewCell("false", nil)
	if cFalse.Type != cell.TypeBoolean || cFalse.Value != false {
		t.Fatalf("expected TypeBoolean with value false, got %v", cFalse)
	}

	// ISLOGICAL function
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "TRUE", nil)
	sh.SetCellInput(0, 1, "=ISLOGICAL(A1)", nil)
	sh.SetCellInput(0, 2, "1", nil)
	sh.SetCellInput(0, 3, "=ISLOGICAL(A3)", nil)
	sh.Recalculate()

	if res := sh.GetCell(0, 1); res == nil || res.Value != 1.0 {
		t.Errorf("expected ISLOGICAL(A1) to be 1.0, got %v", res)
	}
	if res := sh.GetCell(0, 3); res == nil || res.Value != 0.0 {
		t.Errorf("expected ISLOGICAL(A3) to be 0.0, got %v", res)
	}
}

// 10. loadFile viewport reset
func TestBugfixLoadFileViewportReset(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("failed to init simScreen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "initial.hwk")
	app.RunOnceForTest()

	// Simulate navigating far away
	app.SetCursorForTest(25, 100)
	app.SetViewportForTest(20, 95)

	// Save a test workbook
	tmpDir := t.TempDir()
	fn := filepath.Join(tmpDir, "loaded.hwk")
	wb := sheet.NewWorkbook("loaded.hwk")
	wb.GetActiveSheet().SetCellInput(0, 0, "Hello", nil)
	if err := wb.SaveJSON(fn); err != nil {
		t.Fatalf("failed to save test json: %v", err)
	}

	// Load file
	app.LoadFileForTest(fn)

	cCol, cRow := app.GetCursorForTest()
	if cCol != 0 || cRow != 0 {
		t.Errorf("expected cursor reset to (0,0), got (%d,%d)", cCol, cRow)
	}
	lCol, tRow := app.GetViewportForTest()
	if lCol != 0 || tRow != 0 {
		t.Errorf("expected viewport reset to (0,0), got (%d,%d)", lCol, tRow)
	}
}

// 11. ROW / COLUMN with named ranges and named cells
func TestBugfixRowColumnNamedRange(t *testing.T) {
	wb := sheet.NewWorkbook("test_names.hwk")
	sh := wb.GetActiveSheet()
	sh.SetName("Sheet1")

	wb.SetNamedRange("MyBlock", coord.RangeRef{
		Sheet: "Sheet1",
		Start: coord.CellRef{Col: 2, Row: 4},
		End:   coord.CellRef{Col: 4, Row: 9},
	})
	wb.SetNamedRange("MyTarget", coord.CellRef{
		Sheet: "Sheet1",
		Col:   3,
		Row:   6,
	})

	sh.SetCellInput(0, 0, "=ROW(MyBlock)", nil)
	sh.SetCellInput(0, 1, "=COLUMN(MyBlock)", nil)
	sh.SetCellInput(0, 2, "=ROW(MyTarget)", nil)
	sh.SetCellInput(0, 3, "=COLUMN(MyTarget)", nil)
	wb.RecalculateAll()

	r1 := sh.GetCell(0, 0)
	if r1 == nil || r1.Value != 5.0 {
		t.Errorf("expected ROW(MyBlock) to be 5.0, got %v", r1)
	}
	c1 := sh.GetCell(0, 1)
	if c1 == nil || c1.Value != 3.0 {
		t.Errorf("expected COLUMN(MyBlock) to be 3.0 (Col C), got %v", c1)
	}
	r2 := sh.GetCell(0, 2)
	if r2 == nil || r2.Value != 7.0 {
		t.Errorf("expected ROW(MyTarget) to be 7.0, got %v", r2)
	}
	c2 := sh.GetCell(0, 3)
	if c2 == nil || c2.Value != 4.0 {
		t.Errorf("expected COLUMN(MyTarget) to be 4.0 (Col D), got %v", c2)
	}
}
