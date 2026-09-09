package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
)

func TestBugfixV12_QualifyFormulaReferences(t *testing.T) {
	q := formula.QualifyFormulaReferences("=A1+B1:C2", "Sheet1")
	if !strings.Contains(q, "Sheet1!A1") || !strings.Contains(q, "Sheet1!B1:C2") {
		t.Fatalf("QualifyFormulaReferences want Sheet1!A1 and Sheet1!B1:C2, got %q", q)
	}
	// Already qualified ref should not be double-qualified
	q2 := formula.QualifyFormulaReferences("=Sheet2!A1+B1", "Sheet1")
	if !strings.Contains(q2, "Sheet2!A1") || !strings.Contains(q2, "Sheet1!B1") {
		t.Fatalf("QualifyFormulaReferences should preserve existing Sheet2!A1, got %q", q2)
	}
	q3 := formula.QualifyFormulaReferences("=SUM(SALES)", "Sheet1")
	u3 := strings.ToUpper(q3)
	if !strings.Contains(u3, "SHEET1") || !strings.Contains(u3, "SALES") {
		t.Fatalf("QualifyFormulaReferences should qualify named range SALES, got %q", q3)
	}
}

// 1. Cross-Sheet Cut & Paste qualifies formulas and correctly follows moved references.
func TestBugfixV12_CrossSheetCutFormulaReferenceQualification(t *testing.T) {
	wb := sheet.NewWorkbook("v12_cross_cut")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")

	// Sheet1: A1=10, B1=20, C1==A1+B1
	sh1.SetCellInput(0, 0, "10", nil)
	sh1.SetCellInput(1, 0, "20", nil)
	sh1.SetCellInput(2, 0, "=A1+B1", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh1, "v12_cross_cut.hwk")

	// Case 1: Cut only C1 (=A1+B1) from Sheet1 to Sheet2!A1.
	// Since A1 and B1 stay on Sheet1, the moved formula must qualify to Sheet1!A1 + Sheet1!B1.
	c1R := coord.RangeRef{Start: coord.CellRef{Col: 2, Row: 0}, End: coord.CellRef{Col: 2, Row: 0}}
	app.SelectRangeForTest(&c1R)
	app.SetCursorForTest(2, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))

	app.SwitchSheetForTest(1) // Sheet2
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(0, 0) // Sheet2!A1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	rawSheet2A1 := cellRaw(t, sh2, 0, 0)
	if !strings.Contains(strings.ToUpper(rawSheet2A1), "SHEET1") {
		t.Fatalf("Sheet2!A1 formula want references qualified with Sheet1, got %q", rawSheet2A1)
	}
	if v, ok := sh2.GetCellValue(0, 0).(float64); !ok || v != 30 {
		t.Fatalf("Sheet2!A1 value want 30, got %v", sh2.GetCellValue(0, 0))
	}

	// Case 2: Cut both A1 (10) and B1 (20) from Sheet1 to Sheet2!D1..E1.
	// Sheet2!A1 (which references Sheet1!A1 and Sheet1!B1) must follow them to D1 and E1 on Sheet2!
	app.SwitchSheetForTest(0) // Sheet1
	abR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	app.SelectRangeForTest(&abR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))

	app.SwitchSheetForTest(1) // Sheet2
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0) // Sheet2!D1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	wb.RecalculateAll()
	rawSheet2A1After := cellRaw(t, sh2, 0, 0)
	if !strings.Contains(rawSheet2A1After, "D1") || !strings.Contains(rawSheet2A1After, "E1") {
		t.Fatalf("Sheet2!A1 formula after moving targets to D1,E1 want refs to D1 and E1, got %q", rawSheet2A1After)
	}
	if v, ok := sh2.GetCellValue(0, 0).(float64); !ok || v != 30 {
		t.Fatalf("Sheet2!A1 value after targets moved want 30, got %v", sh2.GetCellValue(0, 0))
	}
}

// 2. Undoing a cut selection clears the cut clipboard so subsequent paste doesn't retarget.
func TestBugfixV12_CutUndoClearsCutClipboard(t *testing.T) {
	wb := sheet.NewWorkbook("v12_cut_undo")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "100", nil) // A1
	sh.SetCellInput(1, 0, "=A1", nil)  // B1
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v12_cut_undo.hwk")

	// Cut A1
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))

	// Undo the cut
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))

	// Verify A1 restored
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 100 {
		t.Fatalf("A1 value after Undo want 100, got %v", sh.GetCellValue(0, 0))
	}

	// Move cursor to D1 and paste: clipboard should be empty, B1 should NOT be retargeted to D1!
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0) // D1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	if raw := cellRaw(t, sh, 1, 0); raw != "=A1" {
		t.Fatalf("B1 formula must remain =A1 after cancelled cut paste, got %q", raw)
	}
	if sh.GetCellValue(3, 0) != nil {
		t.Fatalf("D1 must remain empty, got %v", sh.GetCellValue(3, 0))
	}
}

func TestBugfixV12_UnrelatedUndoKeepsCutClipboard(t *testing.T) {
	wb := sheet.NewWorkbook("v12_cut_keep")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "100", nil) // A1
	sh.SetCellInput(1, 0, "=A1", nil)  // B1
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v12_cut_keep.hwk")
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))

	app.SelectRangeForTest(nil)
	app.SetCursorForTest(2, 0) // C1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '9', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))

	app.SetCursorForTest(3, 0) // D1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 100 {
		t.Fatalf("D1 after paste want 100, got %v", sh.GetCellValue(3, 0))
	}
	if raw := cellRaw(t, sh, 1, 0); raw != "=D1" {
		t.Fatalf("B1 should follow cut paste to D1, got %q", raw)
	}
}

func TestBugfixV12_CutUndoRedoRestoresClipboard(t *testing.T) {
	wb := sheet.NewWorkbook("v12_cut_redo")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "100", nil)
	sh.SetCellInput(1, 0, "=A1", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v12_cut_redo.hwk")
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 100 {
		t.Fatalf("A1 after Undo want 100, got %v", sh.GetCellValue(0, 0))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 100 {
		t.Fatalf("D1 after Redo+Paste want 100, got %v", sh.GetCellValue(3, 0))
	}
	if raw := cellRaw(t, sh, 1, 0); raw != "=D1" {
		t.Fatalf("B1 after Redo+Paste want =D1, got %q", raw)
	}
}

// 3. INDEX, VLOOKUP, HLOOKUP with single cell references, scalars, and error propagation.
func TestBugfixV12_IndexVLookupHLookupSingleCellAndScalars(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "42", nil)      // A1
	s.SetCellInput(1, 0, "Hello", nil)   // B1
	s.SetCellInput(2, 0, "=NA()", nil)   // C1

	// INDEX with single cell reference
	s.SetCellInput(0, 1, "=INDEX(A1, 1, 1)", nil)
	// INDEX with scalar literal
	s.SetCellInput(1, 1, "=INDEX(123, 1, 1)", nil)
	// VLOOKUP with single cell reference
	s.SetCellInput(0, 2, "=VLOOKUP(42, A1, 1, FALSE)", nil)
	// HLOOKUP with single cell reference
	s.SetCellInput(1, 2, "=HLOOKUP(\"Hello\", B1, 1, FALSE)", nil)
	// Error propagation in VLOOKUP, HLOOKUP, LOOKUP
	s.SetCellInput(0, 3, "=VLOOKUP(\"x\", C1, 1, FALSE)", nil)
	s.SetCellInput(1, 3, "=HLOOKUP(\"x\", C1, 1, FALSE)", nil)
	s.SetCellInput(2, 3, "=LOOKUP(C1, A1:B1)", nil)

	s.Recalculate()

	if v, ok := s.GetCellValue(0, 1).(float64); !ok || v != 42 {
		t.Fatalf("INDEX(A1, 1, 1) want 42, got %v", s.GetCellValue(0, 1))
	}
	if v, ok := s.GetCellValue(1, 1).(float64); !ok || v != 123 {
		t.Fatalf("INDEX(123, 1, 1) want 123, got %v", s.GetCellValue(1, 1))
	}
	if v, ok := s.GetCellValue(0, 2).(float64); !ok || v != 42 {
		t.Fatalf("VLOOKUP(42, A1, 1, FALSE) want 42, got %v", s.GetCellValue(0, 2))
	}
	if v, ok := s.GetCellValue(1, 2).(string); !ok || v != "Hello" {
		t.Fatalf("HLOOKUP(\"Hello\", B1, 1, FALSE) want Hello, got %v", s.GetCellValue(1, 2))
	}
	if errVal, ok := s.GetCellValue(0, 3).(cell.LotusError); !ok || errVal.Code != "NA" {
		t.Fatalf("VLOOKUP with error arg want NA error, got %v", s.GetCellValue(0, 3))
	}
	if errVal, ok := s.GetCellValue(1, 3).(cell.LotusError); !ok || errVal.Code != "NA" {
		t.Fatalf("HLOOKUP with error arg want NA error, got %v", s.GetCellValue(1, 3))
	}
	if errVal, ok := s.GetCellValue(2, 3).(cell.LotusError); !ok || errVal.Code != "NA" {
		t.Fatalf("LOOKUP with error arg want NA error, got %v", s.GetCellValue(2, 3))
	}
}

// 4. ODS Import parses whole-column and whole-row OpenFormula references.
func TestBugfixV12_ODSImportWholeColumnAndRow(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"of:=SUM([.A:.B])", "=SUM(A..B)"},
		{"of:=SUM([.$A:.$B])", "=SUM($A..$B)"},
		{"of:=SUM([.1:.10])", "=SUM(1..10)"},
		{"of:=SUM([.$1:.$10])", "=SUM($1..$10)"},
		{"of:=SUM(['Sheet2'.$A:'Sheet2'.$B])", "=SUM(Sheet2!$A..$B)"},
		{"of:=SUM(['My Sheet'.$A:'My Sheet'.$B])", "=SUM('My Sheet'!$A..$B)"},
	}
	for _, tc := range tests {
		got := sheet.ConvertODSFormulaForTest(tc.input)
		if got != tc.want {
			t.Errorf("ConvertODSFormulaForTest(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// 5. XLSX Export converts whole-column and whole-row dots and Excel function names.
func TestBugfixV12_XLSXExportWholeColumnRowAndFunctions(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"=SUM(A..B)", "SUM(A:B)"},
		{"=SUM($A..$B)", "SUM($A:$B)"},
		{"=SUM(1..10)", "SUM(1:10)"},
		{"=SUM($1..$10)", "SUM($1:$10)"},
		{"=@AVG(A1..A10)", "AVERAGE(A1:A10)"},
		{"=@LENGTH(A1)", "LEN(A1)"},
		{"=IF(A1, @AVG(A..B), @LENGTH(C1))", "IF(A1, AVERAGE(A:B), LEN(C1))"},
	}
	for _, tc := range tests {
		got := sheet.ExcelExportFormulaForTest(tc.input)
		if got != tc.want {
			t.Errorf("ExcelExportFormulaForTest(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// 6. Workbook.NamedRanges correctly retargets when moved cross-sheet to primary sheet.
func TestBugfixV12_WorkbookNamedRangesRetargetCrossSheet(t *testing.T) {
	wb := sheet.NewWorkbook("v12_named_ranges")
	sh1 := wb.Sheets[0] // Primary sheet
	sh2 := wb.AddSheet("Sheet2")

	// Set workbook-level named range DATA pointing to Sheet2!A1:B2
	r := mustParseRange(t, "Sheet2!A1:B2")
	wb.NamedRanges["DATA"] = r

	// Move A1:B2 from Sheet2 to Sheet1!C1:D2
	fromR := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 1, Row: 1},
	}
	sh2.RetargetFormulasAfterMoveTo(fromR, 2, 0, sh1.Name())

	updated, ok := wb.NamedRanges["DATA"].(coord.RangeRef)
	if !ok {
		t.Fatalf("DATA in NamedRanges want RangeRef, got %#v", wb.NamedRanges["DATA"])
	}
	// Moved onto Sheet1: keep workbook-visible qualification so other sheets still resolve DATA.
	if !strings.EqualFold(updated.Sheet, sh1.Name()) {
		t.Fatalf("DATA moved to Sheet1 should keep Sheet=%q, got %q", sh1.Name(), updated.Sheet)
	}
	if updated.Start.Col != 2 || updated.Start.Row != 0 || updated.End.Col != 3 || updated.End.Row != 1 {
		t.Fatalf("DATA coords want C1:D2 (cols 2..3, rows 0..1), got %v", updated)
	}

	sh1.SetCellInput(2, 0, "1", nil)
	sh1.SetCellInput(3, 0, "2", nil)
	sh1.SetCellInput(2, 1, "3", nil)
	sh1.SetCellInput(3, 1, "4", nil)
	sh2.SetCellInput(0, 2, "=SUM(DATA)", nil)
	wb.RecalculateAll()
	if v, ok := sh2.GetCellValue(0, 2).(float64); !ok || v != 10 {
		t.Fatalf("Sheet2 SUM(DATA) after move to Sheet1 want 10, got %v", sh2.GetCellValue(0, 2))
	}
}

// 7. Graph Series invalidate removes nil map entries upon sheet deletion.
func TestBugfixV12_GraphInvalidateDeletesSeriesKey(t *testing.T) {
	wb := sheet.NewWorkbook("v12_graph_invalidate")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("TempData")

	ra := mustParseRange(t, "TempData!A1:A5")
	sh1.Graph().Series["A"] = &ra

	if len(sh1.Graph().Series) != 1 {
		t.Fatalf("Series len want 1 before delete, got %d", len(sh1.Graph().Series))
	}

	idx := wb.GetSheetIndex(sh2)
	if err := wb.DeleteSheet(idx); err != nil {
		t.Fatal(err)
	}

	if _, exists := sh1.Graph().Series["A"]; exists {
		t.Fatalf("Series A must be deleted from map after TempData deleted, but key still exists")
	}
	if len(sh1.Graph().Series) != 0 {
		t.Fatalf("Series len want 0 after delete, got %d", len(sh1.Graph().Series))
	}
}

// 8. Sheet rename and invalidate correctly update RangeRefNode when only Start.Sheet is set.
func TestBugfixV12_ASTRenameInvalidateStartSheetFallback(t *testing.T) {
	wb := sheet.NewWorkbook("v12_ast_fallback")
	sh1 := wb.Sheets[0]
	wb.AddSheet("OldData")

	// Set cell with formula referencing OldData
	sh1.SetCellInput(0, 0, "=SUM(OldData!A1..B5)", nil)

	// Rename OldData -> NewData
	if err := wb.RenameSheet("OldData", "NewData"); err != nil {
		t.Fatal(err)
	}

	raw := cellRaw(t, sh1, 0, 0)
	if !strings.Contains(raw, "NewData") || strings.Contains(raw, "OldData") {
		t.Fatalf("Formula after rename want NewData, got %q", raw)
	}

	// Delete NewData
	idx := wb.GetSheetIndex(wb.GetSheet("NewData"))
	if err := wb.DeleteSheet(idx); err != nil {
		t.Fatal(err)
	}

	rawAfterDel := cellRaw(t, sh1, 0, 0)
	if !strings.Contains(rawAfterDel, "#REF!") {
		t.Fatalf("Formula after sheet deletion want #REF!, got %q", rawAfterDel)
	}
}
