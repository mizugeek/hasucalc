package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
	"hasucalc/tui"
)

func TestBugfixV5NamedExprHasBasePersistsHWK(t *testing.T) {
	wb := sheet.NewWorkbook("named_base.hwk")
	sh := wb.Sheets[0]
	wb.SetNamedRange("RelAbove", formula.NamedExpr{
		Expr: "=A1", HasBase: true, BaseCol: 1, BaseRow: 1,
	})
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "20", nil)
	sh.SetCellInput(1, 2, "=RelAbove", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(1, 2).(float64); !ok || v != 20 {
		t.Fatalf("before save RelAbove at B3 want 20, got %v", sh.GetCellValue(1, 2))
	}

	p := filepath.Join(t.TempDir(), "named_base.hwk")
	if err := wb.SaveJSON(p); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}
	loaded, err := sheet.LoadWorkbookJSON(p)
	if err != nil {
		t.Fatalf("LoadWorkbookJSON: %v", err)
	}
	got, ok := loaded.GetNamedRange("RELABOVE")
	if !ok {
		t.Fatal("RELABOVE missing after reload")
	}
	ne, ok := got.(formula.NamedExpr)
	if !ok {
		t.Fatalf("RELABOVE type %T, want NamedExpr", got)
	}
	if !ne.HasBase || ne.BaseCol != 1 || ne.BaseRow != 1 {
		t.Fatalf("HasBase lost: %+v", ne)
	}
	loaded.RecalculateAll()
	if v, ok := loaded.Sheets[0].GetCellValue(1, 2).(float64); !ok || v != 20 {
		t.Fatalf("after reload RelAbove at B3 want 20, got %v", loaded.Sheets[0].GetCellValue(1, 2))
	}
}

func TestBugfixV5ODSXLSXNamedRangesRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("named_io")
	sh := wb.GetActiveSheet()
	wb.SetNamedRange("Sales", coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 4},
	})
	wb.SetNamedRange("RelAbove", formula.NamedExpr{
		Expr: "=A1", HasBase: true, BaseCol: 1, BaseRow: 1,
	})
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "20", nil)
	sh.SetCellInput(1, 2, "=RelAbove", nil)
	wb.RecalculateAll()

	odsPath := filepath.Join(t.TempDir(), "named.ods")
	if err := wb.ExportODS(odsPath); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	odsWB, err := sheet.ImportODSWorkbook(odsPath)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	sales, ok := odsWB.GetNamedRange("SALES")
	if !ok {
		t.Fatal("ODS missing SALES")
	}
	if rr, ok := sales.(coord.RangeRef); !ok || rr.MinRow() != 0 || rr.MaxRow() != 4 {
		t.Fatalf("ODS SALES = %#v", sales)
	}
	rel, ok := odsWB.GetNamedRange("RELABOVE")
	if !ok {
		t.Fatal("ODS missing RELABOVE")
	}
	if ne, ok := rel.(formula.NamedExpr); !ok || !ne.HasBase || ne.BaseCol != 1 || ne.BaseRow != 1 {
		t.Fatalf("ODS RelAbove HasBase lost: %#v", rel)
	}

	xlsxPath := filepath.Join(t.TempDir(), "named.xlsx")
	if err := wb.ExportXLSX(xlsxPath); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}
	xlsxWB, err := sheet.ImportXLSXWorkbook(xlsxPath)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	sales, ok = xlsxWB.GetNamedRange("SALES")
	if !ok {
		t.Fatal("XLSX missing SALES")
	}
	if rr, ok := sales.(coord.RangeRef); !ok || rr.MinRow() != 0 || rr.MaxRow() != 4 {
		t.Fatalf("XLSX SALES = %#v", sales)
	}
	relX, ok := xlsxWB.GetNamedRange("RELABOVE")
	if !ok {
		t.Fatal("XLSX missing RELABOVE")
	}
	if ne, ok := relX.(formula.NamedExpr); !ok || !ne.HasBase || ne.BaseCol != 1 || ne.BaseRow != 1 {
		t.Fatalf("XLSX RelAbove HasBase lost: %#v", relX)
	}
	xlsxWB.RecalculateAll()
	if v, ok := xlsxWB.Sheets[0].GetCellValue(1, 2).(float64); !ok || v != 20 {
		t.Fatalf("XLSX RelAbove at B3 want 20, got %v", xlsxWB.Sheets[0].GetCellValue(1, 2))
	}
}

func TestBugfixV5MoveRangeUpdatesExternalRefs(t *testing.T) {
	wb := sheet.NewWorkbook("move")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)    // A1
	sh.SetCellInput(1, 0, "=A1*2", nil) // B1
	sh.SetCellInput(2, 0, "=A1", nil)   // C1 external
	wb.SetNamedRange("Anchor", coord.CellRef{Col: 0, Row: 0})
	fromR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	toR := coord.RangeRef{Start: coord.CellRef{Col: 3, Row: 0}, End: coord.CellRef{Col: 4, Row: 0}}
	sh.MoveRange(fromR, toR)
	wb.RecalculateAll()

	if raw := sh.GetCell(4, 0).RawInput; raw != "=D1*2" && raw != "=D1*2.0" {
		t.Fatalf("moved B1 formula = %q, want =D1*2", raw)
	}
	if raw := sh.GetCell(2, 0).RawInput; !strings.Contains(raw, "D1") {
		t.Fatalf("external C1 not retargeted, raw=%q", raw)
	}
	anchor, ok := wb.GetNamedRange("ANCHOR")
	if !ok {
		t.Fatal("ANCHOR missing")
	}
	if cr, ok := anchor.(coord.CellRef); !ok || cr.Col != 3 || cr.Row != 0 {
		t.Fatalf("ANCHOR after move = %#v, want D1", anchor)
	}
}

func TestBugfixV5CutPasteRetargetsExternalRefs(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "test_cut.hwk")
	app.RunOnceForTest()

	sh.SetCellInput(0, 0, "10", nil)    // A1
	sh.SetCellInput(1, 0, "=A1*2", nil) // B1
	sh.SetCellInput(2, 0, "=A1", nil)   // C1 external
	sh.Recalculate()

	rng := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	app.SelectRangeForTest(&rng)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.SetCursorForTest(3, 0) // D1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))
	sh.Recalculate()

	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 10 {
		t.Fatalf("D1 after cut-paste want 10, got %v", sh.GetCellValue(3, 0))
	}
	if raw := sh.GetCell(4, 0).RawInput; !strings.Contains(raw, "D1") {
		t.Fatalf("cut formula E1 = %q, want ref to D1", raw)
	}
	if raw := sh.GetCell(2, 0).RawInput; !strings.Contains(raw, "D1") {
		t.Fatalf("external C1 = %q, want =D1", raw)
	}
}

func TestBugfixV5InsertDeleteShiftsNamedRanges(t *testing.T) {
	wb := sheet.NewWorkbook("named_shift")
	sh := wb.Sheets[0]
	wb.SetNamedRange("Sales", coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 4},
	})
	sh.InsertRow(2, 1)
	v, ok := wb.GetNamedRange("SALES")
	if !ok {
		t.Fatal("SALES missing after insert")
	}
	rr, ok := v.(coord.RangeRef)
	if !ok || rr.MinRow() != 0 || rr.MaxRow() != 5 {
		t.Fatalf("insert inside Sales want A1:A6, got %#v", v)
	}

	sh.DeleteRow(0, 1)
	v, ok = wb.GetNamedRange("SALES")
	if !ok {
		t.Fatal("SALES missing after delete")
	}
	rr, ok = v.(coord.RangeRef)
	if !ok || rr.MinRow() != 0 || rr.MaxRow() != 4 {
		t.Fatalf("delete first row of Sales want A1:A5, got %#v", v)
	}
}

func TestBugfixV5EmptyEqualsFalse(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(1, 0, "=A1=FALSE", nil)
	sh.SetCellInput(1, 1, "=A1=0", nil)
	sh.SetCellInput(1, 2, `=A1=""`, nil)
	sh.SetCellInput(1, 3, `=""=0`, nil)
	sh.Recalculate()

	if v, ok := sh.GetCellValue(1, 0).(bool); !ok || !v {
		t.Fatalf("empty=FALSE want true, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(bool); !ok || !v {
		t.Fatalf("empty=0 want true, got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(1, 2).(bool); !ok || !v {
		t.Fatalf(`empty="" want true, got %v`, sh.GetCellValue(1, 2))
	}
	if v, ok := sh.GetCellValue(1, 3).(bool); !ok || v {
		t.Fatalf(`""=0 want false, got %v`, sh.GetCellValue(1, 3))
	}
}

func TestBugfixV5DeleteFirstRowOfRangeShrinksNotREF(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(0, 1, "2", nil)
	sh.SetCellInput(0, 2, "3", nil)
	sh.SetCellInput(1, 4, "=SUM(A1:A3)", nil)
	sh.DeleteRow(0, 1)
	c := sh.GetCell(1, 3)
	if c == nil {
		t.Fatal("formula cell vanished")
	}
	if strings.Contains(c.RawInput, "#REF!") || strings.Contains(c.RawInput, "ERR") {
		t.Fatalf("deleting first row of range became %q", c.RawInput)
	}
	if !strings.Contains(c.RawInput, "A1") || !strings.Contains(c.RawInput, "A2") {
		t.Fatalf("want SUM(A1:A2), got %q", c.RawInput)
	}
	sh.Recalculate()
	if v, ok := sh.GetCellValue(1, 3).(float64); !ok || v != 5 {
		t.Fatalf("SUM after delete first row want 5, got %v", sh.GetCellValue(1, 3))
	}
}

func TestBugfixV5DeletedSheetRefIsREF(t *testing.T) {
	wb := sheet.NewWorkbook("refdel")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Data")
	sh2.SetCellInput(0, 0, "42", nil)
	sh1.SetCellInput(0, 0, "=Data!A1", nil)
	wb.RecalculateAll()
	if err := wb.DeleteSheet(1); err != nil {
		t.Fatal(err)
	}
	wb.RecalculateAll()
	v := sh1.GetCellValue(0, 0)
	errVal, ok := v.(cell.LotusError)
	if !ok || errVal.Code != "#REF!" {
		t.Fatalf("deleted sheet want #REF!, got %v", v)
	}
	if raw := sh1.GetCell(0, 0).RawInput; !strings.Contains(raw, "#REF!") {
		t.Fatalf("formula text want #REF!, got %q", raw)
	}
}

func TestBugfixV5AutoFillFormula2D(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "test_af2d.hwk")
	app.RunOnceForTest()
	sh.SetCellInput(0, 0, "=B1", nil)
	rng := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 2}}
	app.SelectRangeForTest(&rng)
	app.TriggerAutoFillForTest()

	if raw := sh.GetCell(1, 0).RawInput; raw != "=C1" {
		t.Fatalf("B1 want =C1, got %q", raw)
	}
	if raw := sh.GetCell(0, 1).RawInput; raw != "=B2" {
		t.Fatalf("A2 want =B2, got %q", raw)
	}
	if raw := sh.GetCell(1, 2).RawInput; raw != "=C3" {
		t.Fatalf("B3 want =C3, got %q", raw)
	}
}

func TestBugfixV5PointFinishCompareConcatAndEqual(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "test_point_cmp.hwk")
	app.RunOnceForTest()
	app.SetCursorForTest(1, 1)

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '=', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if app.GetModeForTest() != "POINT" {
		t.Fatalf("expected POINT, got %s", app.GetModeForTest())
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '<', tcell.ModNone))
	if app.GetModeForTest() != "INPUT" {
		t.Fatalf("after '<' expected INPUT, got %s", app.GetModeForTest())
	}
	if buf := app.GetInputBufferForTest(); buf != "=C2<" {
		t.Fatalf("buffer after '<' = %q, want =C2<", buf)
	}

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	app.SetCursorForTest(1, 1)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '=', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '&', tcell.ModNone))
	if buf := app.GetInputBufferForTest(); buf != "=B3&" {
		t.Fatalf("buffer after '&' = %q, want =B3&", buf)
	}
}

func TestBugfixV5SumBoolCellVsLiteralAndTrueString(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(1, 0, "=TRUE()", nil) // B1
	sh.SetCellInput(0, 0, "=SUM(B1)", nil)
	sh.SetCellInput(0, 1, "=SUM(B1:B1)", nil)
	sh.SetCellInput(0, 2, "=SUM(TRUE)", nil)
	sh.SetCellInput(0, 3, "=LEFT(TRUE(),2)", nil)
	sh.SetCellInput(0, 4, "=LEFT(FALSE(),2)", nil)
	sh.Recalculate()

	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 0 {
		t.Fatalf("SUM(TRUE cell) want 0, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != 0 {
		t.Fatalf("SUM(B1:B1) want 0, got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 1 {
		t.Fatalf("SUM(TRUE) want 1, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(string); !ok || v != "TR" {
		t.Fatalf("LEFT(TRUE(),2) want TR, got %v", sh.GetCellValue(0, 3))
	}
	if v, ok := sh.GetCellValue(0, 4).(string); !ok || v != "FA" {
		t.Fatalf("LEFT(FALSE(),2) want FA, got %v", sh.GetCellValue(0, 4))
	}
}

func TestBugfixV5IndexZeroReturnsArray(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "20", nil)
	sh.SetCellInput(0, 2, "30", nil)
	sh.SetCellInput(1, 0, "1", nil)
	sh.SetCellInput(1, 1, "2", nil)
	sh.SetCellInput(1, 2, "3", nil)
	sh.SetCellInput(2, 0, "=SUM(INDEX(A1:B3,0,1))", nil)
	sh.SetCellInput(2, 1, "=SUM(INDEX(A1:A3,0))", nil)
	sh.SetCellInput(2, 2, "=INDEX(A1:A3,0,1)", nil)
	sh.Recalculate()

	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 60 {
		t.Fatalf("SUM(INDEX(A1:B3,0,1)) want 60, got %v", sh.GetCellValue(2, 0))
	}
	if v, ok := sh.GetCellValue(2, 1).(float64); !ok || v != 60 {
		t.Fatalf("SUM(INDEX(A1:A3,0)) want 60, got %v", sh.GetCellValue(2, 1))
	}
	if v, ok := sh.GetCellValue(2, 2).(float64); !ok || v != 10 {
		t.Fatalf("INDEX column in cell (implicit intersection) want 10, got %v", sh.GetCellValue(2, 2))
	}
}
