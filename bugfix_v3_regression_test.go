package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
	"hasucalc/tui"
)

func TestBugfixAutoFillSingleFormulaCellDoesNotOverwrite(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("failed to init simScreen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "test_autofill_single.hwk")
	app.RunOnceForTest()

	sh.SetCellInput(2, 5, "UP", nil)   // C6
	sh.SetCellInput(2, 6, "=1+2", nil) // C7
	sh.Recalculate()

	rng := coord.RangeRef{
		Start: coord.CellRef{Col: 2, Row: 6},
		End:   coord.CellRef{Col: 2, Row: 6},
	}
	app.SelectRangeForTest(&rng)
	app.SetCursorForTest(2, 6)
	app.TriggerAutoFillForTest()

	c7 := sh.GetCell(2, 6)
	if c7 == nil || c7.RawInput != "=1+2" {
		raw := ""
		if c7 != nil {
			raw = c7.RawInput
		}
		t.Fatalf("single-cell AutoFill overwrote C7: raw=%q, want =1+2", raw)
	}
}

func TestBugfixTextNumberFormatCommaAndZeroPad(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, `=TEXT(1234.56,"0.00")`, nil)
	sh.SetCellInput(0, 1, `=TEXT(1000,"#")`, nil)
	sh.SetCellInput(0, 2, `=TEXT(1.2,"00.00")`, nil)
	sh.SetCellInput(0, 3, `=TEXT(1234.56,"#,##0.00")`, nil)
	sh.SetCellInput(0, 4, `=TEXT(1234.56,"$#,##0.00 USD")`, nil)
	sh.Recalculate()

	cases := []struct {
		row  int
		want string
	}{
		{0, "1234.56"},
		{1, "1000"},
		{2, "01.20"},
		{3, "1,234.56"},
		{4, "$1,234.56 USD"},
	}
	for _, tc := range cases {
		got := sh.GetCellValue(0, tc.row)
		if got != tc.want {
			t.Errorf("row %d: got %v, want %q", tc.row, got, tc.want)
		}
	}
}

func TestBugfixPointModeAfterCaretConcatCompareAndRange(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("failed to init simScreen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	sh := sheet.NewSheet()
	app := tui.NewApp(simScreen, sh, "test_point_ops.hwk")
	app.RunOnceForTest()

	// =3^ then Left -> POINT
	app.SetCursorForTest(1, 1)
	for _, r := range "=3^" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone))
	if app.GetModeForTest() != "POINT" {
		t.Fatalf("after '^'+Left expected POINT, got %s", app.GetModeForTest())
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	// =A1& then Down -> POINT
	app.SetCursorForTest(1, 1)
	for _, r := range "=A1&" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.GetModeForTest() != "POINT" {
		t.Fatalf("after '&'+Down expected POINT, got %s", app.GetModeForTest())
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	// =A1< then Right -> POINT
	app.SetCursorForTest(1, 1)
	for _, r := range "=A1<" {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if app.GetModeForTest() != "POINT" {
		t.Fatalf("after '<'+Right expected POINT, got %s", app.GetModeForTest())
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))

	// =SUM(A1.. then Down -> POINT (must not commit the unfinished formula)
	app.SetCursorForTest(1, 1)
	for _, r := range "=SUM(A1.." {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.GetModeForTest() != "POINT" {
		t.Fatalf("after '..'+Down expected POINT, got %s", app.GetModeForTest())
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	b2 := sh.GetCell(1, 1)
	if b2 != nil && b2.RawInput == "=SUM(A1.." {
		t.Fatalf("Down after '..' committed unfinished formula to B2")
	}
}

func TestBugfixXLSXFormulaBooleanCachedAsBool(t *testing.T) {
	wb := sheet.NewWorkbook("bool_formula.xlsx")
	sh := wb.GetActiveSheet()
	sh.SetCellInput(0, 0, "=TRUE()", nil)
	sh.SetCellInput(0, 1, "=FALSE()", nil)
	wb.RecalculateAll()

	tmp := filepath.Join(t.TempDir(), "bool_formula.xlsx")
	if err := wb.ExportXLSX(tmp); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}
	imp, err := sheet.ImportXLSXWorkbook(tmp)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	a1 := imp.Sheets[0].GetCell(0, 0)
	if a1 == nil || a1.Type != cell.TypeFormula {
		t.Fatalf("A1 want FORMULA, got %+v", a1)
	}
	if v, ok := a1.Value.(bool); !ok || !v {
		t.Errorf("A1 cached value want bool true, got %T %v", a1.Value, a1.Value)
	}
	a2 := imp.Sheets[0].GetCell(0, 1)
	if a2 == nil {
		t.Fatal("A2 is nil")
	}
	if v, ok := a2.Value.(bool); !ok || v {
		t.Errorf("A2 cached value want bool false, got %T %v", a2.Value, a2.Value)
	}
}

func TestBugfixODSFormulaBooleanAndOpenFormulaRefs(t *testing.T) {
	got := sheet.OpenFormulaExportFormulaForTest("=SUM(A1..B10)")
	if got != "SUM([.A1:.B10])" {
		t.Errorf("OpenFormula SUM range: got %q", got)
	}
	got = sheet.OpenFormulaExportFormulaForTest("=Data!A1")
	if got != "[Data.A1]" {
		t.Errorf("OpenFormula sheet cell: got %q", got)
	}
	got = sheet.OpenFormulaExportFormulaForTest("='My Sheet'!A1:B2")
	if got != "['My Sheet'.A1:.B2]" {
		t.Errorf("OpenFormula quoted sheet range: got %q", got)
	}
	got = sheet.OpenFormulaExportFormulaForTest(`="A1"`)
	if got != `"A1"` {
		t.Errorf("string that looks like a ref should stay a string, got %q", got)
	}
	round := sheet.ConvertODSFormulaForTest("of:=" + sheet.OpenFormulaExportFormulaForTest("=SUM(A1..B10)"))
	if round != "=SUM(A1..B10)" {
		t.Errorf("OpenFormula roundtrip got %q", round)
	}

	wb := sheet.NewWorkbook("bool_formula.ods")
	sh := wb.GetActiveSheet()
	sh.SetCellInput(0, 0, "=TRUE()", nil)
	sh.SetCellInput(0, 1, "=SUM(A3:B3)", nil)
	sh.SetCellInput(0, 2, "1", nil)
	sh.SetCellInput(1, 2, "2", nil)
	wb.RecalculateAll()

	tmp := filepath.Join(t.TempDir(), "bool_formula.ods")
	if err := wb.ExportODS(tmp); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	imp, err := sheet.ImportODSWorkbook(tmp)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	a1 := imp.Sheets[0].GetCell(0, 0)
	if a1 == nil || a1.Type != cell.TypeFormula {
		t.Fatalf("A1 want FORMULA, got %+v", a1)
	}
	if v, ok := a1.Value.(bool); !ok || !v {
		t.Errorf("ODS A1 cached value want bool true, got %T %v", a1.Value, a1.Value)
	}
	a2 := imp.Sheets[0].GetCell(0, 1)
	if a2 == nil || !strings.Contains(a2.RawInput, "A3") {
		t.Errorf("ODS formula refs not restored, raw=%v", a2)
	}
	if v, ok := a2.Value.(float64); !ok || v != 3 {
		t.Errorf("ODS SUM want 3, got %v", a2.Value)
	}
}

func TestBugfixSnapshotDefaultWidthAndRecalcMode(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetDefaultColWidth(20)
	sh.SetRecalcMode("MANUAL")
	snap := sh.CreateSnapshot()

	sh.SetDefaultColWidth(9)
	sh.SetRecalcMode("AUTO")
	sh.RestoreSnapshot(snap)

	if sh.DefaultColWidth() != 20 {
		t.Errorf("DefaultColWidth restored %d, want 20", sh.DefaultColWidth())
	}
	if sh.RecalcMode() != "MANUAL" {
		t.Errorf("RecalcMode restored %q, want MANUAL", sh.RecalcMode())
	}
}

func TestBugfixExportMarkdownSkipsEmptyFormattedCells(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "x", nil)
	r, err := coord.ParseRangeRef("A1:Z10")
	if err != nil {
		t.Fatal(err)
	}
	sh.FormatRange(r, cell.CellFormat{Type: cell.FmtFixed, Decimals: 2})

	path := filepath.Join(t.TempDir(), "t.md")
	if err := sh.ExportMarkdown(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 1 {
		t.Fatalf("empty markdown: %q", data)
	}
	nPipes := strings.Count(lines[0], "|")
	if nPipes != 2 {
		t.Errorf("expected 1 data column (2 pipes), got %d pipes in %q", nPipes, lines[0])
	}
}
