package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
	"hasucalc/tui"
)

func TestBugfix_CopyRangeUpdatesUsedRange(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "5", nil)
	from, _ := coord.ParseRangeRef("A1")
	to, _ := coord.ParseRangeRef("A100")
	sh.CopyRange(from, to)
	sh.SetCellInput(1, 0, "=SUM(A:A)", nil)
	sh.Recalculate()
	if sh.MaxPopulatedRow() < 99 {
		t.Fatalf("maxPopulatedRow after copy to A100 = %d, want >= 99", sh.MaxPopulatedRow())
	}
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 10 {
		t.Fatalf("SUM(A:A) after copy = %v, want 10", sh.GetCellValue(1, 0))
	}
}

func TestBugfix_CrossSheetRangeWhitespace(t *testing.T) {
	wb := sheet.NewWorkbook("t")
	s1 := wb.Sheets[0]
	s1.SetName("Sheet1")
	s2 := wb.AddSheet("Data")
	s2.SetCellInput(0, 0, "100", nil)
	s2.SetCellInput(1, 0, "200", nil)
	s2.SetCellInput(0, 1, "300", nil)
	s2.SetCellInput(1, 1, "400", nil)
	s1.SetCellInput(0, 0, "1", nil)
	s1.SetCellInput(1, 0, "2", nil)
	s1.SetCellInput(0, 1, "3", nil)
	s1.SetCellInput(1, 1, "4", nil)
	s1.SetCellInput(5, 0, "=SUM(Data!A1:B2)", nil)
	s1.SetCellInput(5, 1, "=SUM(Data!A1 : B2)", nil)
	s1.SetCellInput(5, 2, "=SUM(Data!A1: B2)", nil)
	wb.RecalculateAll()
	for i, label := range []string{"Data!A1:B2", "Data!A1 : B2", "Data!A1: B2"} {
		v, ok := s1.GetCellValue(5, i).(float64)
		if !ok || v != 1000 {
			t.Errorf("SUM(%s) = %v, want 1000", label, s1.GetCellValue(5, i))
		}
	}
}

func TestBugfix_PowerOperatorNaN(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=(-4)^0.5", nil)
	sh.SetCellInput(0, 1, "=POWER(-4,0.5)", nil)
	sh.SetCellInput(0, 2, "=IFERROR(A1,0)", nil)
	sh.SetCellInput(0, 3, "=ISERROR(A1)", nil)
	sh.Recalculate()
	if _, isErr := sh.GetCellValue(0, 0).(cell.LotusError); !isErr {
		t.Errorf("^ NaN path = %v, want ERR", sh.GetCellValue(0, 0))
	}
	if _, isErr := sh.GetCellValue(0, 1).(cell.LotusError); !isErr {
		t.Errorf("POWER = %v, want ERR", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 0 {
		t.Errorf("IFERROR = %v, want 0", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 1 {
		t.Errorf("ISERROR = %v, want 1", sh.GetCellValue(0, 3))
	}
	if f, ok := sh.GetCellValue(0, 0).(float64); ok && math.IsNaN(f) {
		t.Fatal("operator ^ still returns NaN")
	}
}

func TestBugfix_AndOrXorPropagateErrors(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=AND(FALSE,1/0)", nil)
	sh.SetCellInput(0, 1, "=OR(TRUE,1/0)", nil)
	sh.SetCellInput(0, 2, "=XOR(1/0)", nil)
	sh.Recalculate()
	assertIsError(t, sh.GetCellValue(0, 0), "AND(FALSE,1/0)")
	assertIsError(t, sh.GetCellValue(0, 1), "OR(TRUE,1/0)")
	assertIsError(t, sh.GetCellValue(0, 2), "XOR(1/0)")
}

func TestBugfix_DateDays360DatedifSumIndexPercent(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "=DATE(1900,2,29)", nil)
	sh.SetCellInput(0, 1, "=DATE(1900,3,1)", nil)
	sh.SetCellInput(0, 2, "=DAYS360(DATE(2024,1,1),DATE(2024,3,31),FALSE)", nil)
	sh.SetCellInput(0, 3, "=DAYS360(DATE(2024,1,1),DATE(2024,3,31),TRUE)", nil)
	sh.SetCellInput(0, 4, "=DATEDIF(DATE(2024,1,31),DATE(2024,3,1),\"MD\")", nil)
	sh.SetCellInput(0, 5, "=TRUE()", nil)
	sh.SetCellInput(1, 5, "5", nil)
	sh.SetCellInput(0, 6, "=SUM(A6:B6)", nil)
	sh.SetCellInput(0, 7, "=SUM(TRUE,5)", nil)
	sh.SetCellInput(10, 0, "10", nil)
	sh.SetCellInput(11, 0, "20", nil)
	sh.SetCellInput(10, 1, "30", nil)
	sh.SetCellInput(11, 1, "40", nil)
	sh.SetCellInput(0, 8, "=INDEX(K1:L2,2)", nil)
	sh.SetCellInput(0, 9, "50%", nil)
	sh.SetCellInput(0, 10, "=50%", nil)
	sh.Recalculate()

	if sh.GetCellValue(0, 0) != 60.0 {
		t.Errorf("DATE(1900,2,29) = %v, want 60", sh.GetCellValue(0, 0))
	}
	if sh.GetCellValue(0, 1) != 61.0 {
		t.Errorf("DATE(1900,3,1) = %v, want 61", sh.GetCellValue(0, 1))
	}
	if sh.GetCellValue(0, 2) != 90.0 {
		t.Errorf("DAYS360 US = %v, want 90", sh.GetCellValue(0, 2))
	}
	if sh.GetCellValue(0, 3) != 89.0 {
		t.Errorf("DAYS360 EU = %v, want 89", sh.GetCellValue(0, 3))
	}
	if sh.GetCellValue(0, 4) != 1.0 {
		t.Errorf("DATEDIF MD = %v, want 1", sh.GetCellValue(0, 4))
	}
	if sh.GetCellValue(0, 6) != 5.0 {
		t.Errorf("SUM(TRUE cell, 5) = %v, want 5", sh.GetCellValue(0, 6))
	}
	if sh.GetCellValue(0, 7) != 6.0 {
		t.Errorf("SUM(TRUE,5) literal = %v, want 6", sh.GetCellValue(0, 7))
	}
	if _, isErr := sh.GetCellValue(0, 8).(cell.LotusError); !isErr {
		t.Errorf("INDEX(multi-col, row) = %v, want ERR", sh.GetCellValue(0, 8))
	}
	if sh.GetCellValue(0, 9) != 0.5 {
		t.Errorf("50%% cell = %v, want 0.5", sh.GetCellValue(0, 9))
	}
	if sh.GetCellValue(0, 10) != 0.5 {
		t.Errorf("=50%% = %v, want 0.5", sh.GetCellValue(0, 10))
	}
}

func TestBugfix_SheetNameApostrophe(t *testing.T) {
	wb := sheet.NewWorkbook("t")
	wb.Sheets[0].SetName("Sheet1")
	s := wb.AddSheet("O'Brien")
	s.SetCellInput(0, 0, "99", nil)
	wb.Sheets[0].SetCellInput(0, 0, `=INDIRECT("'O''Brien'!A1")`, nil)
	wb.RecalculateAll()
	if v, ok := wb.Sheets[0].GetCellValue(0, 0).(float64); !ok || v != 99 {
		t.Fatalf("INDIRECT escaped apostrophe = %v, want 99", wb.Sheets[0].GetCellValue(0, 0))
	}
	got := coord.CellRef{Sheet: "O'Brien", Col: 0, Row: 0}.String()
	if got != "'O''Brien'!A1" {
		t.Errorf("CellRef.String() = %q, want 'O''Brien'!A1", got)
	}
	adj := formula.AdjustFormulaReferences("='O''Brien'!A1", 0, 1)
	if adj != "='O''Brien'!A2" {
		t.Errorf("AdjustFormulaReferences = %q, want ='O''Brien'!A2", adj)
	}
}

func TestBugfix_TransposeFormulas(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "=A1*2", nil)
	src, _ := coord.ParseRangeRef("A1..A2")
	if err := sh.TransposeRange(src, coord.CellRef{Col: 2, Row: 0}); err != nil {
		t.Fatal(err)
	}
	c := sh.GetCell(3, 0)
	if c == nil || c.RawInput != "=C1*2" {
		raw := ""
		if c != nil {
			raw = c.RawInput
		}
		t.Fatalf("transposed formula raw = %q, want =C1*2", raw)
	}
}

func TestBugfix_InsertRowDropsOverflow(t *testing.T) {
	sh := sheet.NewSheet()
	last := sh.MaxRows() - 1
	sh.SetCellInput(0, last, "edge", nil)
	sh.InsertRow(0, 1)
	if sh.GetCell(0, last+1) != nil {
		t.Fatal("overflow cell should be dropped")
	}
	if sh.MaxPopulatedRow() >= sh.MaxRows() {
		t.Fatalf("maxPopulatedRow=%d should be < maxRows=%d", sh.MaxPopulatedRow(), sh.MaxRows())
	}
	if sh.GetCell(0, last) != nil {
		t.Fatal("cell pushed past last row should be dropped, not kept on last row")
	}
}

func TestBugfix_TSVImport(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.tsv")
	if err := os.WriteFile(p, []byte("A\tB\tC\n1\t2\t3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sh, err := sheet.ImportSheetCSV(p)
	if err != nil {
		t.Fatal(err)
	}
	if sh.GetCellValue(0, 0) != "A" || sh.GetCellValue(1, 0) != "B" || sh.GetCellValue(2, 0) != "C" {
		t.Fatalf("TSV header = %v %v %v", sh.GetCellValue(0, 0), sh.GetCellValue(1, 0), sh.GetCellValue(2, 0))
	}
}

func TestBugfix_SheetExportODSKeepsOrigin(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(2, 5, "X", nil)
	p := filepath.Join(t.TempDir(), "out.ods")
	if err := sh.ExportODS(p); err != nil {
		t.Fatal(err)
	}
	wb, err := sheet.ImportODSWorkbook(p)
	if err != nil {
		t.Fatal(err)
	}
	ls := wb.GetActiveSheet()
	if ls.GetCellValue(2, 5) != "X" {
		t.Fatalf("C6 after ODS roundtrip = %v, A1 = %v", ls.GetCellValue(2, 5), ls.GetCellValue(0, 0))
	}
}

func TestBugfix_JSONBoolCell(t *testing.T) {
	p := filepath.Join(t.TempDir(), "t.hwk")
	data := `{"version":"HasuCalc/2.0","sheets":[{"name":"S","cells":{"A1":true}}]}`
	if err := os.WriteFile(p, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	wb, err := sheet.LoadWorkbookJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	wb.RecalculateAll()
	if wb.Sheets[0].GetCell(0, 0) == nil {
		t.Fatal("boolean JSON cell was dropped")
	}
	if v, ok := wb.Sheets[0].GetCellValue(0, 0).(bool); !ok || !v {
		t.Fatalf("boolean JSON value = %v", wb.Sheets[0].GetCellValue(0, 0))
	}
}

func TestBugfix_SortRangeRecalcCrossSheet(t *testing.T) {
	wb := sheet.NewWorkbook("t")
	s1 := wb.Sheets[0]
	s1.SetName("Sheet1")
	s1.SetCellInput(0, 0, "2", nil)
	s1.SetCellInput(0, 1, "1", nil)
	s2 := wb.AddSheet("Other")
	s2.SetCellInput(0, 0, "=Sheet1!A1", nil)
	wb.RecalculateAll()
	r, _ := coord.ParseRangeRef("A1..A2")
	s1.SortRange(r, 0, true)
	if s1.GetCellValue(0, 0) != 1.0 {
		t.Fatalf("Sheet1!A1 after sort = %v, want 1", s1.GetCellValue(0, 0))
	}
	if s2.GetCellValue(0, 0) != 1.0 {
		t.Fatalf("Other!A1 after sort = %v, want 1 (workbook recalc)", s2.GetCellValue(0, 0))
	}
}

func TestBugfix_XLSXFormatRangeRoundtrip(t *testing.T) {
	wb := sheet.NewWorkbook("t")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "Hi", nil)
	r, _ := coord.ParseRangeRef("A1..B2")
	sh.FormatRange(r, cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2})
	p := filepath.Join(t.TempDir(), "out.xlsx")
	if err := wb.ExportXLSX(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := sheet.ImportXLSXWorkbook(p)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GetActiveSheet().GetCellValue(0, 0) != "Hi" {
		t.Fatalf("A1 after xlsx roundtrip = %v", loaded.GetActiveSheet().GetCellValue(0, 0))
	}
}

func TestBugfix_MouseCommitDuringInput(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("mouse.hwk")
	sh := wb.GetActiveSheet()
	app := tui.NewApp(simScreen, sh, "mouse.hwk")
	app.RunOnceForTest()

	for _, ch := range "hello" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, ch, tcell.ModNone))
	}
	// Click B3 (same coords as existing mouse test: x=15, y=5)
	app.ProcessEventForTest(tcell.NewEventMouse(15, 5, tcell.Button1, tcell.ModNone))
	if got := sh.GetCellValue(0, 0); got != "hello" {
		t.Fatalf("A1 after click-away = %v, want hello", got)
	}
	if sh.GetCell(1, 2) != nil {
		t.Fatalf("B3 should be empty, got %v", sh.GetCellValue(1, 2))
	}
	if app.CursorCol() != 1 || app.CursorRow() != 2 {
		t.Fatalf("cursor after click = %d,%d want B3", app.CursorCol(), app.CursorRow())
	}
}

func TestBugfix_OpenUndoRestoresWorkbook(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatal(err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(100, 30)

	wb := sheet.NewWorkbook("orig.hwk")
	s1 := wb.GetActiveSheet()
	s1.SetName("Sheet1")
	s2 := wb.AddSheet("Sheet2")
	s2.SetCellInput(0, 0, "keep-me", nil)
	app := tui.NewApp(simScreen, s1, "orig.hwk")

	other := filepath.Join(t.TempDir(), "other.hwk")
	owb := sheet.NewWorkbook("other.hwk")
	owb.Sheets[0].SetCellInput(0, 0, "new", nil)
	if err := owb.SaveJSON(other); err != nil {
		t.Fatal(err)
	}
	app.LoadFileForTest(other)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModNone))

	restored := app.ActiveSheet().Workbook()
	if restored == nil {
		t.Fatal("workbook missing after undo")
	}
	sh2 := restored.GetSheet("Sheet2")
	if sh2 == nil || sh2.GetCellValue(0, 0) != "keep-me" {
		t.Fatalf("Sheet2 not restored after open+undo, sheet=%v", sh2)
	}
}
