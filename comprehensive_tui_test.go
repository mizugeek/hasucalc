package main

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"hasucalc/coord"
	"hasucalc/sheet"
	"hasucalc/tui"
)

// TestComprehensive_TUI_UndoRedo_CellInput tests basic edit undo and redo
func TestComprehensive_TUI_UndoRedo_CellInput(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_undo.hwk")
	sh := wb.Sheets[0]
	app := tui.NewApp(simScreen, sh, "test_undo.hwk")
	app.RunOnceForTest()

	// 1. Enter input in A1: "123"
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '1', 0))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '2', 0))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '3', 0))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, 0))

	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 123 {
		t.Fatalf("A1 want 123, got %#v", sh.GetCellValue(0, 0))
	}

	// 2. Undo (Ctrl+Z)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if sh.GetCellValue(0, 0) != nil {
		t.Fatalf("A1 after undo want nil, got %#v", sh.GetCellValue(0, 0))
	}

	// 3. Redo (Ctrl+Y)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 123 {
		t.Fatalf("A1 after redo want 123, got %#v", sh.GetCellValue(0, 0))
	}
}

// TestComprehensive_TUI_UndoRedo_CutPaste tests cut-paste followed by undo and redo
func TestComprehensive_TUI_UndoRedo_CutPaste(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_cut_undo.hwk")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil) // A1
	sh.SetCellInput(1, 0, "20", nil) // B1
	sh.Recalculate()

	app := tui.NewApp(simScreen, sh, "test_cut_undo.hwk")
	app.RunOnceForTest()

	// Select A1:B1 and cut
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))

	// Paste to D1:E1
	app.SetCursorForTest(3, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))
	sh.Recalculate()

	if sh.GetCellValue(0, 0) != nil || sh.GetCellValue(1, 0) != nil {
		t.Fatal("Source cells A1:B1 should be empty after cut-paste")
	}
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 10 {
		t.Fatalf("D1 want 10, got %#v", sh.GetCellValue(3, 0))
	}

	// Undo cut-paste
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	sh.Recalculate()

	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 10 {
		t.Fatalf("A1 after undo want 10, got %#v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("B1 after undo want 20, got %#v", sh.GetCellValue(1, 0))
	}
	if sh.GetCellValue(3, 0) != nil {
		t.Fatalf("D1 after undo want nil, got %#v", sh.GetCellValue(3, 0))
	}

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	sh.Recalculate()
	if sh.GetCellValue(0, 0) != nil || sh.GetCellValue(1, 0) != nil {
		t.Fatalf("source after redo want empty, got A1=%#v B1=%#v", sh.GetCellValue(0, 0), sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 10 {
		t.Fatalf("D1 after redo want 10, got %#v", sh.GetCellValue(3, 0))
	}
	if v, ok := sh.GetCellValue(4, 0).(float64); !ok || v != 20 {
		t.Fatalf("E1 after redo want 20, got %#v", sh.GetCellValue(4, 0))
	}
}

// TestComprehensive_TUI_UndoRedo_RowColInsertDelete tests undo/redo of row and column mutations
func TestComprehensive_TUI_UndoRedo_RowColInsertDelete(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_mutations.hwk")
	sh := wb.Sheets[0]

	sh.SetCellInput(0, 0, "A1", nil)
	sh.SetCellInput(0, 1, "A2", nil)
	sh.SetCellInput(0, 2, "A3", nil)

	app := tui.NewApp(simScreen, sh, "test_mutations.hwk")
	app.RunOnceForTest()

	// Execute doInsertRow at row 1 (A2)
	app.ExecuteActionHandlerForTest("doInsertRow", map[string]string{"range": "A2"})

	if sh.GetCellValue(0, 0) != "A1" || sh.GetCellValue(0, 2) != "A2" || sh.GetCellValue(0, 3) != "A3" {
		t.Fatalf("Row insert failed: A1=%v, A2=%v, A3=%v", sh.GetCellValue(0, 0), sh.GetCellValue(0, 2), sh.GetCellValue(0, 3))
	}

	// Undo (Ctrl+Z)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if sh.GetCellValue(0, 0) != "A1" || sh.GetCellValue(0, 1) != "A2" || sh.GetCellValue(0, 2) != "A3" {
		t.Fatalf("Row insert undo failed: A1=%v, A2=%v, A3=%v", sh.GetCellValue(0, 0), sh.GetCellValue(0, 1), sh.GetCellValue(0, 2))
	}

	// Redo (Ctrl+Y)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	if sh.GetCellValue(0, 0) != "A1" || sh.GetCellValue(0, 2) != "A2" || sh.GetCellValue(0, 3) != "A3" {
		t.Fatalf("Row insert redo failed: A1=%v, A2=%v, A3=%v", sh.GetCellValue(0, 0), sh.GetCellValue(0, 2), sh.GetCellValue(0, 3))
	}
}

// TestComprehensive_TUI_PointModeCancel tests entering POINT mode and canceling with Escape
func TestComprehensive_TUI_PointModeCancel(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_point.hwk")
	sh := wb.Sheets[0]
	app := tui.NewApp(simScreen, sh, "test_point.hwk")
	app.RunOnceForTest()

	// Type "=" at A1
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '=', 0))

	// Press Right arrow to enter POINT mode
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, 0))
	if app.GetModeForTest() != "POINT" {
		t.Fatalf("Expected mode POINT, got %s", app.GetModeForTest())
	}

	// Press Escape to cancel POINT mode
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if app.GetModeForTest() != "INPUT" {
		t.Fatalf("Expected mode INPUT after Esc in POINT, got %s", app.GetModeForTest())
	}

	// Press Escape again to cancel EDIT mode
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, 0))
	if app.GetModeForTest() != "READY" {
		t.Fatalf("Expected mode READY after second Esc, got %s", app.GetModeForTest())
	}

	// Cell A1 should be empty
	if sh.GetCellValue(0, 0) != nil {
		t.Fatalf("A1 should remain empty after canceling point/edit, got %#v", sh.GetCellValue(0, 0))
	}
}

// TestComprehensive_TUI_UndoRedo_SheetAddDelete tests adding a sheet, undoing it, and redoing it
func TestComprehensive_TUI_UndoRedo_SheetAddDelete(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_sheets_undo.hwk")
	sh1 := wb.Sheets[0]
	sh1.SetName("Sheet1")
	sh1.SetCellInput(0, 0, "keep", nil)

	app := tui.NewApp(simScreen, sh1, "test_sheets_undo.hwk")
	app.RunOnceForTest()

	app.ExecuteActionHandlerForTest("doWorksheetAdd", map[string]string{"name": "Sheet2"})
	if len(wb.Sheets) != 2 || wb.GetSheet("Sheet2") == nil {
		t.Fatalf("Expected 2 sheets with Sheet2, got %d sheets", len(wb.Sheets))
	}

	app.SetCursorForTest(0, 0)
	for _, r := range "Sheet2Data" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	sh2 := wb.GetSheet("Sheet2")
	if sh2 == nil || sh2.GetCellValue(0, 0) != "Sheet2Data" {
		got := any(nil)
		if sh2 != nil {
			got = sh2.GetCellValue(0, 0)
		}
		t.Fatalf("Sheet2!A1 want Sheet2Data, got %#v", got)
	}

	app.ExecuteActionHandlerForTest("doWorksheetDelete", nil)
	if len(wb.Sheets) != 1 || wb.GetSheet("Sheet2") != nil {
		t.Fatalf("Expected Sheet2 deleted, got %d sheets %v", len(wb.Sheets), wb.SheetNames())
	}
	if wb.Sheets[0].GetCellValue(0, 0) != "keep" {
		t.Fatalf("Sheet1!A1 lost after delete: %#v", wb.Sheets[0].GetCellValue(0, 0))
	}

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	restored := wb.GetSheet("Sheet2")
	if len(wb.Sheets) != 2 || restored == nil {
		t.Fatalf("Expected Sheet2 restored after undo delete, got %d sheets %v", len(wb.Sheets), wb.SheetNames())
	}
	if restored.GetCellValue(0, 0) != "Sheet2Data" {
		t.Fatalf("Sheet2!A1 after undo delete want Sheet2Data, got %#v", restored.GetCellValue(0, 0))
	}
	if wb.GetSheet("Sheet1").GetCellValue(0, 0) != "keep" {
		t.Fatalf("Sheet1!A1 after undo delete want keep, got %#v", wb.GetSheet("Sheet1").GetCellValue(0, 0))
	}

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	if len(wb.Sheets) != 1 || wb.GetSheet("Sheet2") != nil {
		t.Fatalf("Expected Sheet2 gone after redo delete, got %d sheets %v", len(wb.Sheets), wb.SheetNames())
	}
	if wb.Sheets[0].GetCellValue(0, 0) != "keep" {
		t.Fatalf("Sheet1!A1 after redo delete want keep, got %#v", wb.Sheets[0].GetCellValue(0, 0))
	}

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	restored = wb.GetSheet("Sheet2")
	if restored == nil || restored.GetCellValue(0, 0) != "Sheet2Data" {
		got := any(nil)
		if restored != nil {
			got = restored.GetCellValue(0, 0)
		}
		t.Fatalf("Sheet2!A1 after second undo want Sheet2Data, got %#v", got)
	}
}

// TestComprehensive_TUI_UndoRedo_SortRange tests sorting range and undo/redo
func TestComprehensive_TUI_UndoRedo_SortRange(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_sort_undo.hwk")
	sh := wb.Sheets[0]

	// Unsorted rows in A1:B3
	sh.SetCellInput(0, 0, "30", nil)
	sh.SetCellInput(1, 0, "C", nil)
	sh.SetCellInput(0, 1, "10", nil)
	sh.SetCellInput(1, 1, "A", nil)
	sh.SetCellInput(0, 2, "20", nil)
	sh.SetCellInput(1, 2, "B", nil)

	app := tui.NewApp(simScreen, sh, "test_sort_undo.hwk")
	app.RunOnceForTest()

	// Sort A1:B3 ascending by Col A
	app.ExecuteActionHandlerForTest("doDataSortAscPrompt", map[string]string{
		"range": "A1:B3",
		"col":   "A",
	})

	if v, _ := sh.GetCellValue(0, 0).(float64); v != 10 {
		t.Fatalf("Sorted row 0 want 10, got %v", v)
	}
	if v, _ := sh.GetCellValue(0, 1).(float64); v != 20 {
		t.Fatalf("Sorted row 1 want 20, got %v", v)
	}
	if v, _ := sh.GetCellValue(0, 2).(float64); v != 30 {
		t.Fatalf("Sorted row 2 want 30, got %v", v)
	}

	// Undo (Ctrl+Z) -> should restore original order (30, 10, 20)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))

	if v, _ := sh.GetCellValue(0, 0).(float64); v != 30 {
		t.Fatalf("Undo sort row 0 want 30, got %v", v)
	}
	if v, _ := sh.GetCellValue(0, 1).(float64); v != 10 {
		t.Fatalf("Undo sort row 1 want 10, got %v", v)
	}
	if v, _ := sh.GetCellValue(0, 2).(float64); v != 20 {
		t.Fatalf("Undo sort row 2 want 20, got %v", v)
	}

	// Redo (Ctrl+Y) -> should restore sorted order (10, 20, 30)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))

	if v, _ := sh.GetCellValue(0, 0).(float64); v != 10 {
		t.Fatalf("Redo sort row 0 want 10, got %v", v)
	}
	if v, _ := sh.GetCellValue(0, 1).(float64); v != 20 {
		t.Fatalf("Redo sort row 1 want 20, got %v", v)
	}
	if v, _ := sh.GetCellValue(0, 2).(float64); v != 30 {
		t.Fatalf("Redo sort row 2 want 30, got %v", v)
	}
	if sh.GetCellValue(1, 0) != "A" || sh.GetCellValue(1, 1) != "B" || sh.GetCellValue(1, 2) != "C" {
		t.Fatalf("Redo sort labels want A,B,C got %v,%v,%v", sh.GetCellValue(1, 0), sh.GetCellValue(1, 1), sh.GetCellValue(1, 2))
	}
}
