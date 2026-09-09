package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
	"hasucalc/tui"
)

// 1. @SWITCH error propagation
func TestBugfixV4SwitchErrorPropagation(t *testing.T) {
	sh := sheet.NewSheet()

	// Error in target expression
	sh.SetCellInput(0, 0, "=SWITCH(NA(), 1, \"A\", \"Def\")", nil)
	// Error in one of the cases
	sh.SetCellInput(0, 1, "=SWITCH(1, NA(), \"A\", 1, \"Match\", \"Def\")", nil)
	// Normal matching
	sh.SetCellInput(0, 2, "=SWITCH(2, 1, \"A\", 2, \"B\", \"Def\")", nil)
	// Default case
	sh.SetCellInput(0, 3, "=SWITCH(9, 1, \"A\", 2, \"B\", \"Def\")", nil)
	sh.Recalculate()

	c0 := sh.GetCell(0, 0)
	if errVal, ok := c0.Value.(cell.LotusError); !ok || errVal.Code != cell.ErrNA.Code {
		t.Fatalf("Expected cell A1 to propagate NA error, got %v", c0.Value)
	}

	c1 := sh.GetCell(0, 1)
	if errVal, ok := c1.Value.(cell.LotusError); !ok || errVal.Code != cell.ErrNA.Code {
		t.Fatalf("Expected cell A2 to propagate NA error from matchVal, got %v", c1.Value)
	}

	c2 := sh.GetCell(0, 2)
	if c2.Value != "B" {
		t.Fatalf("Expected cell A3 to evaluate to 'B', got %v", c2.Value)
	}

	c3 := sh.GetCell(0, 3)
	if c3.Value != "Def" {
		t.Fatalf("Expected cell A4 to evaluate to 'Def', got %v", c3.Value)
	}
}

// 2. String functions empty cell (nil) & error handling
func TestBugfixV4StringFunctionsEmptyAndErrorHandling(t *testing.T) {
	sh := sheet.NewSheet()

	// Empty cell A1
	sh.SetCellInput(0, 0, "", nil) // A1 is empty

	sh.SetCellInput(1, 0, "=LEN(A1)", nil)
	sh.SetCellInput(1, 1, "=LEFT(A1, 2)", nil)
	sh.SetCellInput(1, 2, "=RIGHT(A1, 2)", nil)
	sh.SetCellInput(1, 3, "=MID(A1, 1, 2)", nil)
	sh.SetCellInput(1, 4, "=CONCAT(A1, \"hello\")", nil)
	sh.SetCellInput(1, 5, "=TEXTJOIN(\",\", 1, A1, \"world\")", nil)
	sh.SetCellInput(1, 6, "=UPPER(A1)", nil)
	sh.SetCellInput(1, 7, "=TRIM(A1)", nil)
	sh.SetCellInput(1, 8, "=SUBSTITUTE(A1, \"x\", \"y\")", nil)
	sh.SetCellInput(1, 9, "=REPT(A1, 3)", nil)
	sh.SetCellInput(1, 10, "=CLEAN(A1)", nil)

	// Error propagation
	sh.SetCellInput(2, 0, "=LEN(NA())", nil)
	sh.SetCellInput(2, 1, "=LEFT(ERR(), 1)", nil)
	sh.SetCellInput(2, 2, "=UPPER(NA())", nil)
	sh.SetCellInput(2, 3, "=CONCAT(\"prefix\", NA())", nil)

	sh.Recalculate()

	if v := sh.GetCell(1, 0).Value; v != float64(0) {
		t.Fatalf("Expected LEN(empty) == 0, got %v", v)
	}
	if v := sh.GetCell(1, 1).Value; v != "" {
		t.Fatalf("Expected LEFT(empty) == '', got %v", v)
	}
	if v := sh.GetCell(1, 2).Value; v != "" {
		t.Fatalf("Expected RIGHT(empty) == '', got %v", v)
	}
	if v := sh.GetCell(1, 3).Value; v != "" {
		t.Fatalf("Expected MID(empty) == '', got %v", v)
	}
	if v := sh.GetCell(1, 4).Value; v != "hello" {
		t.Fatalf("Expected CONCAT(empty, 'hello') == 'hello', got %v", v)
	}
	if v := sh.GetCell(1, 5).Value; v != "world" {
		t.Fatalf("Expected TEXTJOIN(',', 1, empty, 'world') == 'world', got %v", v)
	}
	if v := sh.GetCell(1, 6).Value; v != "" {
		t.Fatalf("Expected UPPER(empty) == '', got %v", v)
	}
	if v := sh.GetCell(1, 7).Value; v != "" {
		t.Fatalf("Expected TRIM(empty) == '', got %v", v)
	}
	if v := sh.GetCell(1, 8).Value; v != "" {
		t.Fatalf("Expected SUBSTITUTE(empty) == '', got %v", v)
	}
	if v := sh.GetCell(1, 9).Value; v != "" {
		t.Fatalf("Expected REPT(empty) == '', got %v", v)
	}
	if v := sh.GetCell(1, 10).Value; v != "" {
		t.Fatalf("Expected CLEAN(empty) == '', got %v", v)
	}

	// Verify error propagation
	if errVal, ok := sh.GetCell(2, 0).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrNA.Code {
		t.Fatalf("Expected LEN(NA()) to propagate ErrNA, got %v", sh.GetCell(2, 0).Value)
	}
	if errVal, ok := sh.GetCell(2, 1).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrLotus.Code {
		t.Fatalf("Expected LEFT(ERR(), 1) to propagate ErrLotus, got %v", sh.GetCell(2, 1).Value)
	}
	if errVal, ok := sh.GetCell(2, 2).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrNA.Code {
		t.Fatalf("Expected UPPER(NA()) to propagate ErrNA, got %v", sh.GetCell(2, 2).Value)
	}
	if errVal, ok := sh.GetCell(2, 3).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrNA.Code {
		t.Fatalf("Expected CONCAT(..., NA()) to propagate ErrNA, got %v", sh.GetCell(2, 3).Value)
	}
}

// 3. @NA() and @ERR() function registration
func TestBugfixV4NAAndERRFunctions(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=NA()", nil)
	sh.SetCellInput(0, 1, "=ERR()", nil)
	sh.SetCellInput(0, 2, "@NA", nil)
	sh.SetCellInput(0, 3, "@ERR", nil)
	sh.SetCellInput(0, 4, "=ISNA(NA())", nil)
	sh.SetCellInput(0, 5, "=ISERR(ERR())", nil)
	sh.Recalculate()

	if errVal, ok := sh.GetCell(0, 0).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrNA.Code {
		t.Fatalf("Expected =NA() to return ErrNA, got %v", sh.GetCell(0, 0).Value)
	}
	if errVal, ok := sh.GetCell(0, 1).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrLotus.Code {
		t.Fatalf("Expected =ERR() to return ErrLotus, got %v", sh.GetCell(0, 1).Value)
	}
	if errVal, ok := sh.GetCell(0, 2).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrNA.Code {
		t.Fatalf("Expected @NA to return ErrNA, got %v", sh.GetCell(0, 2).Value)
	}
	if errVal, ok := sh.GetCell(0, 3).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrLotus.Code {
		t.Fatalf("Expected @ERR to return ErrLotus, got %v", sh.GetCell(0, 3).Value)
	}
	if v := sh.GetCell(0, 4).Value; v != float64(1) && v != true {
		t.Fatalf("Expected ISNA(NA()) == 1/true, got %v", v)
	}
	if v := sh.GetCell(0, 5).Value; v != float64(1) && v != true {
		t.Fatalf("Expected ISERR(ERR()) == 1/true, got %v", v)
	}
}

// 4. @ADDRESS sheet name quoting & 4-arg sheet name
func TestBugfixV4AddressSheetQuotingAnd4Arg(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=ADDRESS(1, 1, 1, 1, \"My Sheet\")", nil)
	sh.SetCellInput(0, 1, "=ADDRESS(2, 3, 1, \"Data\")", nil)
	sh.SetCellInput(0, 2, "=ADDRESS(5, 2, 4, 1, \"Simple\")", nil)
	sh.Recalculate()

	if v := sh.GetCell(0, 0).Value; v != "'My Sheet'!$A$1" {
		t.Fatalf("Expected 'My Sheet'!$A$1, got %v", v)
	}
	if v := sh.GetCell(0, 1).Value; v != "Data!$C$2" {
		t.Fatalf("Expected Data!$C$2, got %v", v)
	}
	if v := sh.GetCell(0, 2).Value; v != "Simple!B5" {
		t.Fatalf("Expected Simple!B5, got %v", v)
	}
}

// 5. @WEEKDAY return types 11-17 & error handling
func TestBugfixV4WeekdayReturnTypes(t *testing.T) {
	sh := sheet.NewSheet()
	// 2026-09-07 is Monday
	sh.SetCellInput(0, 0, "=WEEKDAY(DATE(2026, 9, 7), 1)", nil)  // Sun=1..Sat=7 -> Mon is 2
	sh.SetCellInput(0, 1, "=WEEKDAY(DATE(2026, 9, 7), 2)", nil)  // Mon=1..Sun=7 -> Mon is 1
	sh.SetCellInput(0, 2, "=WEEKDAY(DATE(2026, 9, 7), 3)", nil)  // Mon=0..Sun=6 -> Mon is 0
	sh.SetCellInput(0, 3, "=WEEKDAY(DATE(2026, 9, 7), 11)", nil) // Mon=1..Sun=7 -> Mon is 1
	sh.SetCellInput(0, 4, "=WEEKDAY(DATE(2026, 9, 7), 12)", nil) // Tue=1..Mon=7 -> Mon is 7
	sh.SetCellInput(0, 5, "=WEEKDAY(DATE(2026, 9, 7), 99)", nil) // Invalid return_type -> ErrLotus
	sh.Recalculate()

	if v := sh.GetCell(0, 0).Value; v != float64(2) {
		t.Fatalf("Expected WEEKDAY(..., 1) == 2, got %v", v)
	}
	if v := sh.GetCell(0, 1).Value; v != float64(1) {
		t.Fatalf("Expected WEEKDAY(..., 2) == 1, got %v", v)
	}
	if v := sh.GetCell(0, 2).Value; v != float64(0) {
		t.Fatalf("Expected WEEKDAY(..., 3) == 0, got %v", v)
	}
	if v := sh.GetCell(0, 3).Value; v != float64(1) {
		t.Fatalf("Expected WEEKDAY(..., 11) == 1, got %v", v)
	}
	if v := sh.GetCell(0, 4).Value; v != float64(7) {
		t.Fatalf("Expected WEEKDAY(..., 12) == 7, got %v", v)
	}
	if errVal, ok := sh.GetCell(0, 5).Value.(cell.LotusError); !ok || errVal.Code != cell.ErrLotus.Code {
		t.Fatalf("Expected invalid return_type to return ErrLotus, got %v", sh.GetCell(0, 5).Value)
	}
}

// 6. @DATEDIF time normalization
func TestBugfixV4DateDifTimeNormalization(t *testing.T) {
	sh := sheet.NewSheet()
	// Start at 2026-01-01 23:00, End at 2026-01-02 01:00 (only 2 hours later, but next calendar day)
	sh.SetCellInput(0, 0, "=DATEDIF(DATE(2026, 1, 1) + TIME(23, 0, 0), DATE(2026, 1, 2) + TIME(1, 0, 0), \"D\")", nil)
	// Full years
	sh.SetCellInput(0, 1, "=DATEDIF(DATE(2020, 3, 1), DATE(2024, 3, 1), \"Y\")", nil)
	// Full months
	sh.SetCellInput(0, 2, "=DATEDIF(DATE(2026, 1, 15), DATE(2026, 5, 15), \"M\")", nil)
	sh.Recalculate()

	if v := sh.GetCell(0, 0).Value; v != float64(1) {
		t.Fatalf("Expected DATEDIF calendar day difference == 1, got %v", v)
	}
	if v := sh.GetCell(0, 1).Value; v != float64(4) {
		t.Fatalf("Expected DATEDIF year difference == 4, got %v", v)
	}
	if v := sh.GetCell(0, 2).Value; v != float64(4) {
		t.Fatalf("Expected DATEDIF month difference == 4, got %v", v)
	}
}

// 7. @MATCH descending string lookup
func TestBugfixV4MatchDescendingString(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "Zebra", nil) // A1
	sh.SetCellInput(0, 1, "Lion", nil)  // A2
	sh.SetCellInput(0, 2, "Cat", nil)   // A3
	sh.SetCellInput(0, 3, "Ant", nil)   // A4

	// Exact match with matchType = -1
	sh.SetCellInput(1, 0, "=MATCH(\"Lion\", A1:A4, -1)", nil)
	// Smallest value >= "Dog": in descending [Zebra, Lion, Cat, Ant], "Lion" > "Dog" >= "Cat".
	sh.SetCellInput(1, 1, "=MATCH(\"Dog\", A1:A4, -1)", nil)
	sh.Recalculate()

	if v := sh.GetCell(1, 0).Value; v != float64(2) {
		t.Fatalf("Expected MATCH('Lion', A1:A4, -1) == 2, got %v", v)
	}
	if v := sh.GetCell(1, 1).Value; v != float64(2) {
		t.Fatalf("Expected MATCH('Dog', A1:A4, -1) == 2 ('Lion'), got %v", v)
	}
}

// 8. SortRange & SortRangeHorizontal empty cells and case-insensitivity
func TestBugfixV4SortRangeEmptyCells(t *testing.T) {
	sh := sheet.NewSheet()
	// A1: "Banana", A2: nil (empty), A3: "apple", A4: 10
	sh.SetCellInput(0, 0, "Banana", nil)
	// A2 left nil
	sh.SetCellInput(0, 2, "apple", nil)
	sh.SetCellInput(0, 3, "10", nil)

	rng, err := coord.ParseRangeRef("A1..A4")
	if err != nil {
		t.Fatalf("Failed to parse range: %v", err)
	}

	// Ascending sort
	sh.SortRange(rng, 0, true)
	// Order should be: 10 (number), apple, Banana (case-insensitive string), nil (empty at bottom)
	if v := sh.GetCell(0, 0).Value; v != float64(10) {
		t.Fatalf("Expected row 0 to be 10, got %v", v)
	}
	if v := sh.GetCell(0, 1).Value; v != "apple" {
		t.Fatalf("Expected row 1 to be 'apple', got %v", v)
	}
	if v := sh.GetCell(0, 2).Value; v != "Banana" {
		t.Fatalf("Expected row 2 to be 'Banana', got %v", v)
	}
	if c := sh.GetCell(0, 3); c != nil && c.Type != cell.TypeEmpty && c.Value != nil {
		t.Fatalf("Expected row 3 to be nil/empty, got %v", c)
	}

	// Descending sort: Banana, apple, 10, nil (empty MUST STILL BE AT BOTTOM)
	sh.SortRange(rng, 0, false)
	if v := sh.GetCell(0, 0).Value; v != "Banana" {
		t.Fatalf("Expected descending row 0 to be 'Banana', got %v", v)
	}
	if v := sh.GetCell(0, 1).Value; v != "apple" {
		t.Fatalf("Expected descending row 1 to be 'apple', got %v", v)
	}
	if v := sh.GetCell(0, 2).Value; v != float64(10) {
		t.Fatalf("Expected descending row 2 to be 10, got %v", v)
	}
	if c := sh.GetCell(0, 3); c != nil && c.Type != cell.TypeEmpty && c.Value != nil {
		t.Fatalf("Expected descending row 3 to remain nil/empty at bottom, got %v", c)
	}

	// Test horizontal sort
	shH := sheet.NewSheet()
	// Row 1 (index 0): A1="Banana", B1=nil, C1="apple", D1=10
	shH.SetCellInput(0, 0, "Banana", nil)
	shH.SetCellInput(2, 0, "apple", nil)
	shH.SetCellInput(3, 0, "10", nil)

	rngH, _ := coord.ParseRangeRef("A1..D1")
	// Horizontal descending sort
	shH.SortRangeHorizontal(rngH, 0, false)
	// Col A: Banana, Col B: apple, Col C: 10, Col D: nil (at far right!)
	if v := shH.GetCell(0, 0).Value; v != "Banana" {
		t.Fatalf("Expected horiz descending col 0 to be 'Banana', got %v", v)
	}
	if v := shH.GetCell(1, 0).Value; v != "apple" {
		t.Fatalf("Expected horiz descending col 1 to be 'apple', got %v", v)
	}
	if v := shH.GetCell(2, 0).Value; v != float64(10) {
		t.Fatalf("Expected horiz descending col 2 to be 10, got %v", v)
	}
	if c := shH.GetCell(3, 0); c != nil && c.Type != cell.TypeEmpty && c.Value != nil {
		t.Fatalf("Expected horiz descending col 3 to remain nil at far right, got %v", c)
	}
}

// 9. Workbook undo on insert/delete row/col with cross-sheet formulas
func TestBugfixV4CrossSheetUndoOnInsertDelete(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("failed to init simScreen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_cross_undo.hwk")
	sh1 := wb.GetActiveSheet()
	sh1.SetName("Sheet1")
	sh1.SetCellInput(0, 1, "target", nil) // A2 = "target"

	sh2 := wb.AddSheet("Sheet2")
	sh2.SetCellInput(0, 0, "=Sheet1!A2", nil) // A1 references Sheet1!A2
	wb.RecalculateAll()

	app := tui.NewApp(simScreen, sh1, "test_cross_undo.hwk")
	app.RunOnceForTest()

	// Switch cursor to Sheet1 row 1 (A1) and insert a row via /IR
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'I', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'R', tcell.ModNone))
	// Enter range (default row)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	// In Sheet2, formula should have adjusted to =Sheet1!A3
	sh2Cell := wb.GetSheet("Sheet2").GetCell(0, 0)
	if sh2Cell == nil || sh2Cell.RawInput != "=Sheet1!A3" {
		t.Fatalf("Expected Sheet2 formula to adjust to =Sheet1!A3 after row insert, got %v", sh2Cell)
	}

	// Undo the row insertion
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))

	// Verify Sheet2 formula has been restored to =Sheet1!A2!
	sh2CellRestored := wb.GetSheet("Sheet2").GetCell(0, 0)
	if sh2CellRestored == nil || sh2CellRestored.RawInput != "=Sheet1!A2" {
		t.Fatalf("Expected Sheet2 formula to restore to =Sheet1!A2 after Undo, got %v", sh2CellRestored)
	}
}
