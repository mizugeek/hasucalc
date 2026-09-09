package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
	"hasucalc/tui"
)

// Consolidated regression tests formerly split across bugfix_*.go files.

// --- from bugfix_regression_test.go ---

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

// --- from bugfix_currency_symbol_test.go ---

func TestBugfixCurrencySymbolPerCellAndPersist(t *testing.T) {
	wb := sheet.NewWorkbook("currency_sym")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1234.5", nil)
	sh.SetCellInput(1, 0, "1234.5", nil)
	sh.FormatRange(mustParseRange(t, "A1"), cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2, CurrencySymbol: "¥"})
	sh.FormatRange(mustParseRange(t, "B1"), cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2, CurrencySymbol: "€"})

	a1 := sh.GetCell(0, 0).Render(12, sh.GlobalFormat())
	b1 := sh.GetCell(1, 0).Render(12, sh.GlobalFormat())
	if !strings.Contains(strings.TrimSpace(a1), "¥1,234.50") {
		t.Fatalf("A1 want ¥1,234.50, got %q", a1)
	}
	if !strings.Contains(strings.TrimSpace(b1), "€1,234.50") {
		t.Fatalf("B1 want €1,234.50, got %q", b1)
	}

	p := filepath.Join(t.TempDir(), "currency.hwk")
	if err := wb.SaveJSON(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := sheet.LoadWorkbookJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	lsh := loaded.Sheets[0]
	fa := lsh.GetCell(0, 0).FormatSpec
	fb := lsh.GetCell(1, 0).FormatSpec
	if fa == nil || fa.CurrencySymbol != "¥" {
		t.Fatalf("A1 FormatSpec after reload want ¥, got %#v", fa)
	}
	if fb == nil || fb.CurrencySymbol != "€" {
		t.Fatalf("B1 FormatSpec after reload want €, got %#v", fb)
	}
	if got := cell.ParseCellFormat("(C2¥)"); got.Type != cell.FmtCurrency || got.CurrencySymbol != "¥" || got.Decimals != 2 {
		t.Fatalf("ParseCellFormat (C2¥) = %#v", got)
	}
	if got := cell.ParseCellFormat("(C2)"); got.CurrencySymbolOrDefault() != "$" {
		t.Fatalf("default symbol want $, got %#v", got)
	}
	if (cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2, CurrencySymbol: "¥"}).String() != "(C2¥)" {
		t.Fatalf("String() want (C2¥)")
	}
}

func TestBugfixCurrencyNegativeWithSymbol(t *testing.T) {
	got := cell.FormatNumber(-12.3, cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2, CurrencySymbol: "£"})
	if got != "(£12.30)" {
		t.Fatalf("negative currency want (£12.30), got %q", got)
	}
}

// --- from bugfix_v10_regression_test.go ---

// 1. 検索（Find / Search）の前方・後方検索、折り返し、および全シート横断検索の検証
func TestBugfixV10_FindAndSearchForwardBackwardAndCrossSheet(t *testing.T) {
	wb := sheet.NewWorkbook("find_test.hwk")
	sh1 := wb.Sheets[0]
	sh1.SetName("Sheet1")
	sh2 := wb.AddSheet("Sheet2")

	// Sheet1: A1="Apple", A5="Banana", B10="Pineapple"
	sh1.SetCellInput(0, 0, "Apple", nil)
	sh1.SetCellInput(0, 4, "Banana", nil)
	sh1.SetCellInput(1, 9, "Pineapple", nil)

	// Sheet2: C3="Crabapple"
	sh2.SetCellInput(2, 2, "Crabapple", nil)

	app := newSimApp(t, sh1, "find_test.hwk")

	// 1. Initial Find on current sheet: query "apple"
	app.SetCursorForTest(0, 0)
	app.ExecuteActionHandlerForTest("doFindPrompt", map[string]string{"query": "apple"})
	cCol, cRow := app.GetCursorForTest()
	if cCol != 0 || cRow != 0 {
		t.Fatalf("first find want A1 (0,0), got (%d,%d)", cCol, cRow)
	}

	// 2. F3: findNext -> should move to B10 (1, 9) in Sheet1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	cCol, cRow = app.GetCursorForTest()
	if cCol != 1 || cRow != 9 {
		t.Fatalf("findNext want B10 (1,9), got (%d,%d)", cCol, cRow)
	}

	// 3. F3 again: should wrap around to A1 (0, 0) in Sheet1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	cCol, cRow = app.GetCursorForTest()
	if cCol != 0 || cRow != 0 {
		t.Fatalf("findNext wrap want A1 (0,0), got (%d,%d)", cCol, cRow)
	}

	// 4. Shift+F3: findPrev -> should move backward to B10 (1, 9)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModShift))
	cCol, cRow = app.GetCursorForTest()
	if cCol != 1 || cRow != 9 {
		t.Fatalf("findPrev want B10 (1,9), got (%d,%d)", cCol, cRow)
	}

	// 5. Cross-sheet search: doFindAllPrompt with "Crabapple" -> should switch to Sheet2 C3 (2, 2)
	app.ExecuteActionHandlerForTest("doFindAllPrompt", map[string]string{"query": "Crabapple"})
	cCol, cRow = app.GetCursorForTest()
	if cCol != 2 || cRow != 2 {
		t.Fatalf("cross-sheet find want C3 (2,2) on Sheet2, got (%d,%d)", cCol, cRow)
	}
	if wb.GetActiveSheet().Name() != "Sheet2" {
		t.Fatalf("active sheet want Sheet2, got %s", wb.GetActiveSheet().Name())
	}

	// 6. Not found query: should set status message and keep cursor
	app.ExecuteActionHandlerForTest("doFindPrompt", map[string]string{"query": "NonExistentWordXYZ"})
	if !strings.Contains(app.GetStatusMessageForTest(), "Pattern not found") {
		t.Errorf("status message for not found want 'Pattern not found', got %q", app.GetStatusMessageForTest())
	}
}

// 2. TUI インラインセル編集（EDIT モード / handleBufferEditing）の各編集キーの検証
func TestBugfixV10_TUIInlineEditModeKeys(t *testing.T) {
	wb := sheet.NewWorkbook("edit_mode.hwk")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "Hello World", nil)

	app := newSimApp(t, sh, "edit_mode.hwk")
	app.SetCursorForTest(0, 0)

	// 1. Enter EDIT mode via F2
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	if app.GetModeForTest() != "EDIT" {
		t.Fatalf("mode after F2 want EDIT, got %s", app.GetModeForTest())
	}
	if app.GetInputBufferForTest() != "'Hello World" {
		t.Fatalf("initial edit buffer want \"'Hello World\", got %q", app.GetInputBufferForTest())
	}

	// 2. KeyEnd -> KeyBackspace 5 times: "'Hello World" -> "'Hello "
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	for i := 0; i < 5; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	if app.GetInputBufferForTest() != "'Hello " {
		t.Errorf("buffer after backspace want \"'Hello \", got %q", app.GetInputBufferForTest())
	}

	// 3. Type "Go"
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'G', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	if app.GetInputBufferForTest() != "'Hello Go" {
		t.Errorf("buffer after typing want \"'Hello Go\", got %q", app.GetInputBufferForTest())
	}

	// 4. Commit with KeyEnter -> should save to cell and return to READY
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.GetModeForTest() != "READY" {
		t.Errorf("mode after Enter want READY, got %s", app.GetModeForTest())
	}
	if cellRaw(t, sh, 0, 0) != "'Hello Go" {
		t.Errorf("cell value after Enter want \"'Hello Go\", got %q", cellRaw(t, sh, 0, 0))
	}

	// 5. Test Escape cancellation:
	// Start edit, change text, press Escape -> cell should retain "'Hello Go"
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '!', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.GetModeForTest() != "READY" {
		t.Errorf("mode after Escape want READY, got %s", app.GetModeForTest())
	}
	if cellRaw(t, sh, 0, 0) != "'Hello Go" {
		t.Errorf("cell value after Escape should not change, got %q", cellRaw(t, sh, 0, 0))
	}

	// 6. Test KeyTab commits and moves right
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	cCol, cRow := app.GetCursorForTest()
	if cCol != 1 || cRow != 0 {
		t.Errorf("cursor after Tab commit want (1,0), got (%d,%d)", cCol, cRow)
	}

	// 7. Test Undo restores original "'Hello World"
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if cellRaw(t, sh, 0, 0) != "'Hello World" {
		t.Errorf("cell value after Undo want \"'Hello World\", got %q", cellRaw(t, sh, 0, 0))
	}
}

// 3. シート設定・書式の一括操作機能（LabelAlignRange, SetGlobalFormat, ResetColWidth, DeleteNamedRange, IsModified）
func TestBugfixV10_SheetBulkFormatAndLabelAlign(t *testing.T) {
	wb := sheet.NewWorkbook("bulk_format")
	sh := wb.Sheets[0]

	// 1. LabelAlignRange: Left, Right, Center
	sh.SetCellInput(0, 0, "'Alpha", nil)
	sh.SetCellInput(0, 1, "'Beta", nil)
	sh.SetCellInput(0, 2, "'Gamma", nil)
	rng := mustParseRange(t, "A1:A3")

	// Align Right
	sh.LabelAlignRange(rng, cell.AlignRight)
	for r := 0; r < 3; r++ {
		c := sh.GetCell(0, r)
		if c.Alignment != cell.AlignRight || !strings.HasPrefix(c.RawInput, `"`) {
			t.Errorf("row %d after AlignRight want \", got align=%v raw=%q", r, c.Alignment, c.RawInput)
		}
	}

	// Align Center
	sh.LabelAlignRange(rng, cell.AlignCenter)
	for r := 0; r < 3; r++ {
		c := sh.GetCell(0, r)
		if c.Alignment != cell.AlignCenter || !strings.HasPrefix(c.RawInput, "^") {
			t.Errorf("row %d after AlignCenter want ^, got align=%v raw=%q", r, c.Alignment, c.RawInput)
		}
	}

	// 2. SetGlobalFormat: change workbook default format to Currency
	sh.SetCellInput(1, 0, "123.45", nil) // B1 has no individual format
	sh.SetGlobalFormat(cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2})
	cellB1 := sh.GetCell(1, 0)
	formatted := cellB1.FormattedValue(sh.GlobalFormat())
	if !strings.Contains(formatted, "$123.45") {
		t.Errorf("GlobalFormat Currency want $123.45, got %q", formatted)
	}

	// 3. ResetColWidth
	sh.SetColWidth(0, 28)
	if sh.GetColWidth(0) != 28 {
		t.Fatalf("col 0 width want 28, got %d", sh.GetColWidth(0))
	}
	sh.ResetColWidth(0)
	if sh.GetColWidth(0) != sh.DefaultColWidth() {
		t.Errorf("after ResetColWidth col 0 want default %d, got %d", sh.DefaultColWidth(), sh.GetColWidth(0))
	}

	// 4. DeleteNamedRange
	sh.SetNamedRange("TEMPRANGE", rng)
	if _, ok := sh.GetNamedRange("TEMPRANGE"); !ok {
		t.Fatal("TEMPRANGE missing after SetNamedRange")
	}
	sh.DeleteNamedRange("TEMPRANGE")
	if _, ok := sh.GetNamedRange("TEMPRANGE"); ok {
		t.Errorf("TEMPRANGE should be gone after DeleteNamedRange")
	}

	// 5. IsModified state tracking
	if !sh.IsModified() {
		t.Errorf("sheet should be modified after mutations")
	}
	if !wb.IsModified() {
		t.Errorf("workbook should be modified after mutations")
	}
}

// 4. グラフ全タイプ（LINE, BAR, STACKED, PIE）の描画とスタンドアロン PNG 画像出力の検証
func TestBugfixV10_GraphAllTypesAndPNGExport(t *testing.T) {
	sh := sheet.NewSheet()
	// Populate X categories
	sh.SetCellInput(0, 0, "Q1", nil)
	sh.SetCellInput(0, 1, "Q2", nil)
	sh.SetCellInput(0, 2, "Q3", nil)
	sh.SetCellInput(0, 3, "Q4", nil)

	// Series A: standard positive values
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(1, 1, "25", nil)
	sh.SetCellInput(1, 2, "15", nil)
	sh.SetCellInput(1, 3, "40", nil)

	// Series B: identical values (tests zero division protection: minVal == maxVal)
	sh.SetCellInput(2, 0, "50", nil)
	sh.SetCellInput(2, 1, "50", nil)
	sh.SetCellInput(2, 2, "50", nil)
	sh.SetCellInput(2, 3, "50", nil)

	// Series C: all zeros
	sh.SetCellInput(3, 0, "0", nil)
	sh.SetCellInput(3, 1, "0", nil)
	sh.SetCellInput(3, 2, "0", nil)
	sh.SetCellInput(3, 3, "0", nil)

	rx := mustParseRange(t, "A1:A4")
	ra := mustParseRange(t, "B1:B4")
	rb := mustParseRange(t, "C1:C4")
	rc := mustParseRange(t, "D1:D4")

	sh.Graph().RangeX = &rx
	sh.Graph().Series["A"] = &ra
	sh.Graph().Series["B"] = &rb
	sh.Graph().Series["C"] = &rc

	tmpDir := t.TempDir()
	pngMagic := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

	types := []string{"LINE", "BAR", "STACKED", "PIE"}
	for _, gt := range types {
		sh.Graph().Type = gt
		outPath := filepath.Join(tmpDir, "graph_"+gt+".png")
		if err := tui.ExportGraphPNG(sh, outPath, 800, 600); err != nil {
			t.Fatalf("ExportGraphPNG for %s: %v", gt, err)
		}

		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", outPath, err)
		}
		if len(data) < 8 || !bytes.Equal(data[:8], pngMagic) {
			t.Errorf("graph %s output is not valid PNG (magic: %v)", gt, data[:8])
		}
	}
}

// 5. 多言語・タイポグラフィ（DetectScript, IsRTL, ReshapeArabic）の検証
func TestBugfixV10_TypographyScriptDetectionAndRTL(t *testing.T) {
	// 1. DetectScript
	scriptTests := []struct {
		r    rune
		want tui.ScriptType
		desc string
	}{
		{'A', tui.ScriptDefault, "Latin"},
		{'1', tui.ScriptDefault, "Digit"},
		{'あ', tui.ScriptCJK, "Japanese Hiragana"},
		{'漢', tui.ScriptCJK, "Kanji"},
		{0x0645, tui.ScriptArabic, "Arabic Meem"},
		{0x05E9, tui.ScriptHebrew, "Hebrew Shin"},
		{0x0E01, tui.ScriptThai, "Thai Ko Kai"},
		{0x0915, tui.ScriptDevanagari, "Devanagari Ka"},
	}

	for _, tc := range scriptTests {
		got := tui.DetectScript(tc.r)
		if got != tc.want {
			t.Errorf("DetectScript(%q - %s) want %v, got %v", tc.r, tc.desc, tc.want, got)
		}
	}

	// 2. IsRTL
	if tui.IsRTL('A') {
		t.Errorf("IsRTL('A') want false")
	}
	if tui.IsRTL('あ') {
		t.Errorf("IsRTL('あ') want false")
	}
	if !tui.IsRTL(0x0645) { // Arabic Meem
		t.Errorf("IsRTL(Arabic) want true")
	}
	if !tui.IsRTL(0x05E9) { // Hebrew Shin
		t.Errorf("IsRTL(Hebrew) want true")
	}

	// 3. ReshapeArabic: Lam-Alif ligature
	// \u0644 (Lam) + \u0627 (Alef) -> \uFEFB (Lam-Alif Isolated)
	lamAlef := "\u0644\u0627"
	shaped := tui.ReshapeArabic(lamAlef)
	if !strings.ContainsRune(shaped, 0xFEFB) {
		t.Errorf("ReshapeArabic(Lam+Alef) want ligature U+FEFB, got %q (runes: %U)", shaped, []rune(shaped))
	}
}

// 6. ファイル読み込みの耐障害性・異常系テスト（破損・空ファイル）の検証
func TestBugfixV10_FileParserRobustnessCorruptedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 0-byte files
	emptyFile := filepath.Join(tmpDir, "empty.bin")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := sheet.ImportXLSXWorkbook(emptyFile); err == nil {
		t.Errorf("ImportXLSX on empty file should return error")
	}
	if _, err := sheet.ImportODSWorkbook(emptyFile); err == nil {
		t.Errorf("ImportODS on empty file should return error")
	}
	if _, err := sheet.LoadWorkbookJSON(emptyFile); err == nil {
		t.Errorf("LoadWorkbookJSON on empty file should return error")
	}

	// 2. Corrupted plain text file (not a zip)
	corruptedZip := filepath.Join(tmpDir, "not_a_zip.xlsx")
	if err := os.WriteFile(corruptedZip, []byte("THIS IS PLAIN TEXT NOT A VALID ZIP ARCHIVE"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := sheet.ImportXLSXWorkbook(corruptedZip); err == nil {
		t.Errorf("ImportXLSX on corrupted non-zip should return error")
	}
	if _, err := sheet.ImportODSWorkbook(corruptedZip); err == nil {
		t.Errorf("ImportODS on corrupted non-zip should return error")
	}

	// 3. Corrupted JSON file
	badJSON := filepath.Join(tmpDir, "bad.hwk")
	if err := os.WriteFile(badJSON, []byte("{ name: broken json without quotes or closing brace"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := sheet.LoadWorkbookJSON(badJSON); err == nil {
		t.Errorf("LoadWorkbookJSON on invalid JSON should return error")
	}
}

// --- from bugfix_v11_regression_test.go ---

func TestBugfixV11_SameSheetCutPasteRecalculatesFollowers(t *testing.T) {
	wb := sheet.NewWorkbook("cut_recalc")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "=A1", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 10 {
		t.Fatalf("seed B1 want 10, got %v", sh.GetCellValue(1, 0))
	}

	app := newSimApp(t, sh, "cut_recalc.hwk")
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	if raw := cellRaw(t, sh, 1, 0); raw != "=D1" {
		t.Fatalf("B1 formula after cut want =D1, got %q", raw)
	}
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 10 {
		t.Fatalf("B1 value after cut want 10, got %v", sh.GetCellValue(1, 0))
	}
}

func TestBugfixV11_CrossSheetCutUpdatesQualifiedRefs(t *testing.T) {
	wb := sheet.NewWorkbook("cross_cut")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")
	sh1.SetCellInput(0, 0, "10", nil)
	sh1.SetCellInput(1, 0, "=A1*2", nil)
	sh2.SetCellInput(0, 0, "=Sheet1!A1", nil)
	sh2.SetCellInput(2, 0, "=A1", nil) // local Sheet2 A1, must stay
	wb.RecalculateAll()

	app := newSimApp(t, sh1, "cross_cut.hwk")
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.SwitchSheetForTest(1)
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0) // D1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	rawB1 := cellRaw(t, sh1, 1, 0)
	if !strings.Contains(strings.ToUpper(rawB1), "SHEET2") || !strings.Contains(rawB1, "D1") {
		t.Fatalf("Sheet1 B1 should follow to Sheet2!D1, got %q", rawB1)
	}
	rawA1 := cellRaw(t, sh2, 0, 0)
	if rawA1 != "=D1" && !strings.Contains(rawA1, "D1") {
		t.Fatalf("Sheet2 A1 (was =Sheet1!A1) should become D1, got %q", rawA1)
	}
	if raw := cellRaw(t, sh2, 2, 0); raw != "=A1" {
		t.Fatalf("Sheet2 C1 local =A1 must stay, got %q", raw)
	}
	if v, ok := sh1.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("Sheet1 B1 value want 20, got %v", sh1.GetCellValue(1, 0))
	}
	if v, ok := sh2.GetCellValue(0, 0).(float64); !ok || v != 10 {
		t.Fatalf("Sheet2 A1 value want 10, got %v", sh2.GetCellValue(0, 0))
	}
}

func TestBugfixV11_GraphFollowsOtherSheetInsertAndRename(t *testing.T) {
	wb := sheet.NewWorkbook("graph_other")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Data")
	rx := mustParseRange(t, "Data!A1:A4")
	sh1.Graph().RangeX = &rx
	ra := mustParseRange(t, "Data!B1:B4")
	sh1.Graph().Series["A"] = &ra

	sh2.InsertRow(1, 1)
	if sh1.Graph().RangeX == nil || sh1.Graph().RangeX.MaxRow() != 4 {
		t.Fatalf("RangeX after Data insert want rows 0..4, got %v", sh1.Graph().RangeX)
	}
	if sh1.Graph().Series["A"] == nil || sh1.Graph().Series["A"].MaxRow() != 4 {
		t.Fatalf("Series A after Data insert want rows 0..4, got %v", sh1.Graph().Series["A"])
	}

	if err := wb.RenameSheet("Data", "Inventory"); err != nil {
		t.Fatal(err)
	}
	got := sh1.Graph().RangeX.Sheet
	if got == "" {
		got = sh1.Graph().RangeX.Start.Sheet
	}
	if !strings.EqualFold(got, "Inventory") {
		t.Fatalf("RangeX sheet after rename want Inventory, got %#v", sh1.Graph().RangeX)
	}

	idx := wb.GetSheetIndex(wb.GetSheet("Inventory"))
	if err := wb.DeleteSheet(idx); err != nil {
		t.Fatal(err)
	}
	if sh1.Graph().RangeX != nil {
		t.Fatalf("RangeX should clear after deleting Inventory, got %v", sh1.Graph().RangeX)
	}
}

func TestBugfixV11_IndexChoosePropagateNA(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(2, 0, "1", nil)
	s.SetCellInput(2, 1, "2", nil)
	s.SetCellInput(2, 2, "3", nil)
	s.SetCellInput(0, 0, "=INDEX(C1:C3,NA())", nil)
	s.SetCellInput(0, 1, "=CHOOSE(NA(),1,2)", nil)
	s.Recalculate()
	if err, ok := s.GetCellValue(0, 0).(cell.LotusError); !ok || err.Code != "NA" {
		t.Fatalf("INDEX(...,NA()) want NA, got %#v", s.GetCellValue(0, 0))
	}
	if err, ok := s.GetCellValue(0, 1).(cell.LotusError); !ok || err.Code != "NA" {
		t.Fatalf("CHOOSE(NA(),...) want NA, got %#v", s.GetCellValue(0, 1))
	}
}

// --- from bugfix_v12_regression_test.go ---

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
	sh.SetCellInput(1, 0, "=A1", nil) // B1
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
	sh.SetCellInput(1, 0, "=A1", nil) // B1
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
	s.SetCellInput(0, 0, "42", nil)    // A1
	s.SetCellInput(1, 0, "Hello", nil) // B1
	s.SetCellInput(2, 0, "=NA()", nil) // C1

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

// --- from bugfix_v13_regression_test.go ---

// 1. セル参照（cellSourced）ラップに起因するエラー判定・集計・論理値関数の検証
func TestBugfixV13_CellSourcedErrorAndCriteria(t *testing.T) {
	wb := sheet.NewWorkbook("v13_cellsourced")
	sh := wb.Sheets[0]

	// COUNTIF criteria unwrapping:
	// A1 = 42, B1..B3 = 42, 10, 42
	sh.SetCellInput(0, 0, "42", nil)
	sh.SetCellInput(1, 0, "42", nil)
	sh.SetCellInput(1, 1, "10", nil)
	sh.SetCellInput(1, 2, "42", nil)
	sh.SetCellInput(2, 0, "=COUNTIF(B1:B3, A1)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 2 {
		t.Fatalf("COUNTIF with cell reference criteria want 2, got %v", sh.GetCellValue(2, 0))
	}

	// NOT with #NA cell reference:
	// A2 = @NA, A3 = =NOT(A2)
	sh.SetCellInput(0, 1, "@NA", nil)
	sh.SetCellInput(0, 2, "=NOT(A2)", nil)
	wb.RecalculateAll()

	vNot := sh.GetCellValue(0, 2)
	errNot, okNot := vNot.(cell.LotusError)
	if !okNot || errNot.Code != cell.ErrNA.Code {
		t.Fatalf("NOT(A2) where A2 is #NA want #NA error, got %v (%T)", vNot, vNot)
	}

	// ISERROR, ISNA, ISERR with cell reference:
	// A4 = =ISERROR(A2) -> 1.0
	// A5 = =ISNA(A2) -> 1.0
	// A6 = =ISERR(A2) -> 0.0 (since A2 is #NA)
	sh.SetCellInput(0, 3, "=ISERROR(A2)", nil)
	sh.SetCellInput(0, 4, "=ISNA(A2)", nil)
	sh.SetCellInput(0, 5, "=ISERR(A2)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 1.0 {
		t.Fatalf("ISERROR(A2) want 1.0, got %v", sh.GetCellValue(0, 3))
	}
	if v, ok := sh.GetCellValue(0, 4).(float64); !ok || v != 1.0 {
		t.Fatalf("ISNA(A2) want 1.0, got %v", sh.GetCellValue(0, 4))
	}
	if v, ok := sh.GetCellValue(0, 5).(float64); !ok || v != 0.0 {
		t.Fatalf("ISERR(A2) for #NA want 0.0, got %v", sh.GetCellValue(0, 5))
	}

	// IFERROR and IFNA with cell reference:
	// B4 = =IFERROR(A2, "recovered")
	// B5 = =IFNA(A2, "recovered_na")
	sh.SetCellInput(1, 3, `=IFERROR(A2, "recovered")`, nil)
	sh.SetCellInput(1, 4, `=IFNA(A2, "recovered_na")`, nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(1, 3).(string); !ok || v != "recovered" {
		t.Fatalf("IFERROR(A2, ...) want 'recovered', got %v", sh.GetCellValue(1, 3))
	}
	if v, ok := sh.GetCellValue(1, 4).(string); !ok || v != "recovered_na" {
		t.Fatalf("IFNA(A2, ...) want 'recovered_na', got %v", sh.GetCellValue(1, 4))
	}

	// TYPE, N, T with cell reference error:
	// C4 = =TYPE(A2) -> 16
	// C5 = =N(A2) -> #NA
	sh.SetCellInput(2, 3, "=TYPE(A2)", nil)
	sh.SetCellInput(2, 4, "=N(A2)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(2, 3).(float64); !ok || v != 16.0 {
		t.Fatalf("TYPE(A2) for #NA want 16, got %v", sh.GetCellValue(2, 3))
	}
	if errN, ok := sh.GetCellValue(2, 4).(cell.LotusError); !ok || errN.Code != cell.ErrNA.Code {
		t.Fatalf("N(A2) want #NA error, got %v", sh.GetCellValue(2, 4))
	}

	// Aggregations propagate exact error (#NA instead of converting to #ERR):
	// D1=10, D2=@NA, D3=30
	// D4 = =SUM(D1:D3)
	// D5 = =AVERAGE(D1:D3)
	sh.SetCellInput(3, 0, "10", nil)
	sh.SetCellInput(3, 1, "@NA", nil)
	sh.SetCellInput(3, 2, "30", nil)
	sh.SetCellInput(3, 3, "=SUM(D1:D3)", nil)
	sh.SetCellInput(3, 4, "=AVERAGE(D1:D3)", nil)
	wb.RecalculateAll()

	if errSum, ok := sh.GetCellValue(3, 3).(cell.LotusError); !ok || errSum.Code != cell.ErrNA.Code {
		t.Fatalf("SUM(D1:D3) want #NA, got %v", sh.GetCellValue(3, 3))
	}
	if errAvg, ok := sh.GetCellValue(3, 4).(cell.LotusError); !ok || errAvg.Code != cell.ErrNA.Code {
		t.Fatalf("AVERAGE(D1:D3) want #NA, got %v", sh.GetCellValue(3, 4))
	}

	// Logical AND/OR with error cell reference:
	sh.SetCellInput(4, 0, "=AND(TRUE, A2)", nil)
	sh.SetCellInput(4, 1, "=OR(FALSE, A2)", nil)
	wb.RecalculateAll()

	if errAnd, ok := sh.GetCellValue(4, 0).(cell.LotusError); !ok || errAnd.Code != cell.ErrNA.Code {
		t.Fatalf("AND(TRUE, A2) want #NA, got %v", sh.GetCellValue(4, 0))
	}
	if errOr, ok := sh.GetCellValue(4, 1).(cell.LotusError); !ok || errOr.Code != cell.ErrNA.Code {
		t.Fatalf("OR(FALSE, A2) want #NA, got %v", sh.GetCellValue(4, 1))
	}
}

// 2. ROWS and COLUMNS error propagation and multi-cell count
func TestBugfixV13_RowsAndColumns(t *testing.T) {
	wb := sheet.NewWorkbook("v13_rows_cols")
	sh := wb.Sheets[0]

	sh.SetCellInput(0, 0, "@NA", nil)
	sh.SetCellInput(0, 1, "=ROWS(A1)", nil)
	sh.SetCellInput(0, 2, "=COLUMNS(A1)", nil)
	sh.SetCellInput(1, 0, "=ROWS(A1:C5)", nil)
	sh.SetCellInput(1, 1, "=COLUMNS(A1:C5)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != 1.0 {
		t.Fatalf("ROWS(A1) even when A1 is #NA want 1.0, got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 1.0 {
		t.Fatalf("COLUMNS(A1) even when A1 is #NA want 1.0, got %v", sh.GetCellValue(0, 2))
	}

	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 5.0 {
		t.Fatalf("ROWS(A1:C5) want 5.0, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 3.0 {
		t.Fatalf("COLUMNS(A1:C5) want 3.0, got %v", sh.GetCellValue(1, 1))
	}
}

// 3. @OFFSET, @ROW, @COLUMN error propagation
func TestBugfixV13_OffsetRowColumnErrors(t *testing.T) {
	wb := sheet.NewWorkbook("v13_offset")
	sh := wb.Sheets[0]

	sh.SetCellInput(0, 0, "100", nil)
	sh.SetCellInput(0, 1, "@NA", nil)
	sh.SetCellInput(1, 0, "=OFFSET(A1, A2, 0)", nil)
	sh.SetCellInput(1, 1, "=ROW(#REF!)", nil)
	sh.SetCellInput(1, 2, "=COLUMN(#REF!)", nil)
	wb.RecalculateAll()

	if errOff, ok := sh.GetCellValue(1, 0).(cell.LotusError); !ok || errOff.Code != cell.ErrNA.Code {
		t.Fatalf("OFFSET(A1, A2, 0) want #NA, got %v", sh.GetCellValue(1, 0))
	}
	if errRow, ok := sh.GetCellValue(1, 1).(cell.LotusError); !ok || errRow.Code != cell.ErrRef.Code {
		t.Fatalf("ROW(#REF!) want #REF!, got %v", sh.GetCellValue(1, 1))
	}
	if errCol, ok := sh.GetCellValue(1, 2).(cell.LotusError); !ok || errCol.Code != cell.ErrRef.Code {
		t.Fatalf("COLUMN(#REF!) want #REF!, got %v", sh.GetCellValue(1, 2))
	}
}

// 4. Cross-sheet RangeRef evaluation fallback
func TestBugfixV13_CrossSheetRangeRef(t *testing.T) {
	wb := sheet.NewWorkbook("v13_cross_range")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")

	sh2.SetCellInput(0, 0, "10", nil)
	sh2.SetCellInput(1, 0, "20", nil)
	sh2.SetCellInput(0, 1, "30", nil)
	sh2.SetCellInput(1, 1, "40", nil)

	sh1.SetCellInput(0, 0, "=SUM(Sheet2!A1:B2)", nil)
	wb.RecalculateAll()

	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 100.0 {
		t.Fatalf("SUM(Sheet2!A1:B2) want 100.0, got %v", sh1.GetCellValue(0, 0))
	}
}

// 5. Freeze panes insert/delete tracking
func TestBugfixV13_FrozenPanesTracking(t *testing.T) {
	wb := sheet.NewWorkbook("v13_freeze")
	sh := wb.Sheets[0]

	sh.SetFrozenRows(3)
	sh.SetFrozenCols(2)

	if sh.FrozenRows() != 3 || sh.FrozenCols() != 2 {
		t.Fatalf("Initial freeze want 3 rows, 2 cols, got %d, %d", sh.FrozenRows(), sh.FrozenCols())
	}

	// Insert row above/within frozen rows: row 1 (0-based)
	sh.InsertRow(1, 2)
	if sh.FrozenRows() != 5 {
		t.Fatalf("After InsertRow(1, 2), want 5 frozen rows, got %d", sh.FrozenRows())
	}

	// Insert row below frozen rows: row 10
	sh.InsertRow(10, 1)
	if sh.FrozenRows() != 5 {
		t.Fatalf("After InsertRow(10, 1) outside freeze, want 5 frozen rows, got %d", sh.FrozenRows())
	}

	// Delete row within freeze: row 0, count 2
	sh.DeleteRow(0, 2)
	if sh.FrozenRows() != 3 {
		t.Fatalf("After DeleteRow(0, 2), want 3 frozen rows, got %d", sh.FrozenRows())
	}

	// Insert col within frozen cols: col 0, count 1
	sh.InsertCol(0, 1)
	if sh.FrozenCols() != 3 {
		t.Fatalf("After InsertCol(0, 1), want 3 frozen cols, got %d", sh.FrozenCols())
	}

	// Delete col within freeze: col 1, count 1
	sh.DeleteCol(1, 1)
	if sh.FrozenCols() != 2 {
		t.Fatalf("After DeleteCol(1, 1), want 2 frozen cols, got %d", sh.FrozenCols())
	}

	// Insert at freeze boundary (first unfrozen row/col) must not grow freeze.
	sh.SetFrozenRows(3)
	sh.SetFrozenCols(2)
	sh.InsertRow(3, 1)
	if sh.FrozenRows() != 3 {
		t.Fatalf("InsertRow at freeze boundary want 3, got %d", sh.FrozenRows())
	}
	sh.InsertCol(2, 1)
	if sh.FrozenCols() != 2 {
		t.Fatalf("InsertCol at freeze boundary want 2, got %d", sh.FrozenCols())
	}

	// Deleting all frozen rows/cols clears freeze.
	sh.DeleteRow(0, 3)
	if sh.FrozenRows() != 0 {
		t.Fatalf("DeleteRow of all frozen rows want 0, got %d", sh.FrozenRows())
	}
	sh.DeleteCol(0, 2)
	if sh.FrozenCols() != 0 {
		t.Fatalf("DeleteCol of all frozen cols want 0, got %d", sh.FrozenCols())
	}

	// freeze=0: insert/delete must leave freeze at 0
	sh.InsertRow(0, 2)
	sh.DeleteRow(0, 1)
	sh.InsertCol(0, 1)
	sh.DeleteCol(0, 1)
	if sh.FrozenRows() != 0 || sh.FrozenCols() != 0 {
		t.Fatalf("freeze=0 after insert/delete want 0,0 got %d,%d", sh.FrozenRows(), sh.FrozenCols())
	}
}

// 6. Excel formula alias export
func TestBugfixV13_ExcelFormulaExportAliases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"=REPEAT(\"A\", 3)", "REPT(\"A\", 3)"},
		{"=STRING(123, 2)", "FIXED(123, 2)"},
		{"=REPEAT(\"A..B\", 3)", "REPT(\"A..B\", 3)"},
		{"=PAYMT(0.05, 10, 1000)", "PMT(0.05, 10, 1000)"},
		{"=MULTIPLY(2, 3)", "PRODUCT(2, 3)"},
		{"=AVG(A1:A3)", "AVERAGE(A1:A3)"},
		{"=LENGTH(A1)", "LEN(A1)"},
	}

	for _, tt := range tests {
		got := sheet.ExcelExportFormulaForTest(tt.input)
		if got != tt.expected {
			t.Errorf("ExcelExportFormulaForTest(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// 7. 2D AutoFill protects multi-column formulas
func TestBugfixV13_2DAutoFillMultiColumnFormulas(t *testing.T) {
	wb := sheet.NewWorkbook("v13_autofill")
	sh := wb.Sheets[0]

	// Setup:
	// A1=10, B1==A1*2, C1==A1*3
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "=A1*2", nil)
	sh.SetCellInput(2, 0, "=A1*3", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v13_autofill.hwk")

	// Select A1..C3 and trigger AutoFill
	rng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 2, Row: 2},
	}
	app.SelectRangeForTest(&rng)
	app.SetCursorForTest(0, 0)
	app.TriggerAutoFillForTest()

	// A2 should be 11, A3 should be 12 (number sequence)
	// B2 should be =A2*2, B3 should be =A3*2
	// C2 should be =A2*3, C3 should be =A3*3
	rawB2 := cellRaw(t, sh, 1, 1)
	rawC2 := cellRaw(t, sh, 2, 1)
	rawB3 := cellRaw(t, sh, 1, 2)
	rawC3 := cellRaw(t, sh, 2, 2)

	if !strings.Contains(rawB2, "A2*2") {
		t.Fatalf("B2 formula want referencing A2*2, got %q", rawB2)
	}
	if !strings.Contains(rawC2, "A2*3") {
		t.Fatalf("C2 formula want referencing A2*3, got %q", rawC2)
	}
	if !strings.Contains(rawB3, "A3*2") {
		t.Fatalf("B3 formula want referencing A3*2, got %q", rawB3)
	}
	if !strings.Contains(rawC3, "A3*3") {
		t.Fatalf("C3 formula want referencing A3*3, got %q", rawC3)
	}

	// Values check
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 22.0 {
		t.Fatalf("B2 value want 22.0 (11*2), got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(2, 1).(float64); !ok || v != 33.0 {
		t.Fatalf("C2 value want 33.0 (11*3), got %v", sh.GetCellValue(2, 1))
	}
}

func TestBugfixV13_1RowAutoFillPreservesFormulas(t *testing.T) {
	wb := sheet.NewWorkbook("v13_autofill_row")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "=A1*2", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v13_autofill_row.hwk")
	rng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 2, Row: 0},
	}
	app.SelectRangeForTest(&rng)
	app.SetCursorForTest(0, 0)
	app.TriggerAutoFillForTest()

	if raw := cellRaw(t, sh, 1, 0); raw != "=A1*2" {
		t.Fatalf("B1 formula must stay =A1*2, got %q", raw)
	}
	if !cellHasValue(sh, 2, 0) {
		t.Fatalf("C1 empty cell should be filled from A1, got %v", sh.GetCellValue(2, 0))
	}
}

func TestBugfixV13_2DAutoFillSkipsEmptyColumns(t *testing.T) {
	wb := sheet.NewWorkbook("v13_autofill_emptycol")
	sh := wb.Sheets[0]
	sh.SetCellInput(1, 0, "=A1*2", nil) // B1 formula; A empty
	sh.SetCellInput(2, 0, "=A1*3", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v13_autofill_emptycol.hwk")
	rng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 2, Row: 2},
	}
	app.SelectRangeForTest(&rng)
	app.SetCursorForTest(0, 0)
	app.TriggerAutoFillForTest()

	if sh.GetCell(0, 1) != nil && sh.GetCell(0, 1).RawInput != "" {
		t.Fatalf("A2 should stay empty, got %q", sh.GetCell(0, 1).RawInput)
	}
	if sh.GetCell(0, 2) != nil && sh.GetCell(0, 2).RawInput != "" {
		t.Fatalf("A3 should stay empty, got %q", sh.GetCell(0, 2).RawInput)
	}
}

func cellHasValue(sh *sheet.Sheet, col, row int) bool {
	v := sh.GetCellValue(col, row)
	if v == nil {
		return false
	}
	if s, ok := v.(string); ok && s == "" {
		return false
	}
	return true
}

// --- from bugfix_v14_regression_test.go ---

func TestBugfixV14_AutoFillUpAndLeft(t *testing.T) {
	wb := sheet.NewWorkbook("v14_autofill_dir")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 4, "10", nil) // A5
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v14_autofill_dir.hwk")
	rng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 4},
	}
	app.SelectRangeForTest(&rng)
	app.SetCursorForTest(0, 4)
	app.TriggerAutoFillForTest()

	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 6 {
		t.Fatalf("A1 after fill-up want 6, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 9 {
		t.Fatalf("A4 after fill-up want 9, got %v", sh.GetCellValue(0, 3))
	}

	sh.SetCellInput(4, 0, "10", nil) // E1
	sh.SetCellInput(0, 0, "", nil)
	sh.SetCellInput(0, 1, "", nil)
	sh.SetCellInput(0, 2, "", nil)
	sh.SetCellInput(0, 3, "", nil)
	sh.SetCellInput(0, 4, "", nil)
	wb.RecalculateAll()
	rng2 := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 4, Row: 0},
	}
	app.SelectRangeForTest(&rng2)
	app.SetCursorForTest(4, 0)
	app.TriggerAutoFillForTest()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 6 {
		t.Fatalf("A1 after fill-left want 6, got %v", sh.GetCellValue(0, 0))
	}
}

func TestBugfixV14_AutoFillFormulaUp(t *testing.T) {
	wb := sheet.NewWorkbook("v14_formula_up")
	sh := wb.Sheets[0]
	sh.SetCellInput(1, 4, "10", nil)    // B5
	sh.SetCellInput(0, 4, "=B5*2", nil) // A5
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v14_formula_up.hwk")
	rng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 4},
	}
	app.SelectRangeForTest(&rng)
	app.TriggerAutoFillForTest()
	if raw := cellRaw(t, sh, 0, 0); !strings.Contains(raw, "B1") {
		t.Fatalf("A1 formula after fill-up want B1, got %q", raw)
	}
}

func TestBugfixV14_ReplaceDoesNotTouchCellRefs(t *testing.T) {
	got, ok := formula.ReplaceInFormula("=A1+B1+1", "1", "9")
	if !ok {
		t.Fatal("ReplaceInFormula should replace number 1")
	}
	if strings.Contains(got, "A9") || strings.Contains(got, "B9") {
		t.Fatalf("cell refs must stay A1/B1, got %q", got)
	}
	if !strings.Contains(got, "9") {
		t.Fatalf("literal 1 should become 9, got %q", got)
	}

	got2, ok2 := formula.ReplaceInFormula("=A1+B1", "A1", "C1")
	if !ok2 || !strings.Contains(got2, "C1") || strings.Contains(got2, "A1") {
		t.Fatalf("whole A1 ref replace want C1+B1, got %q ok=%v", got2, ok2)
	}
}

func TestBugfixV14_CountIfErrorCriteria(t *testing.T) {
	wb := sheet.NewWorkbook("v14_countif_err")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "@NA", nil)
	sh.SetCellInput(0, 1, "10", nil)
	sh.SetCellInput(0, 2, "@NA", nil)
	sh.SetCellInput(1, 0, "=COUNTIF(A1:A3, A1)", nil)
	sh.SetCellInput(1, 1, `=COUNTIF(A1:A3, "#N/A")`, nil)
	sh.SetCellInput(1, 2, `=COUNTIF(A1:A3, "#REF!")`, nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 2 {
		t.Fatalf("COUNTIF(..., A1) where A1 is #NA want 2, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 2 {
		t.Fatalf("COUNTIF(..., \"#N/A\") want 2, got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(1, 2).(float64); !ok || v != 0 {
		t.Fatalf("COUNTIF(..., \"#REF!\") want 0, got %v", sh.GetCellValue(1, 2))
	}
}

func TestBugfixV14_CrossSheetCutQualifiesNames(t *testing.T) {
	wb := sheet.NewWorkbook("v14_cut_name")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")
	sh1.SetNamedRange("SALES", mustParseRange(t, "B1:B2"))
	sh1.SetCellInput(1, 0, "10", nil)
	sh1.SetCellInput(1, 1, "20", nil)
	sh1.SetCellInput(0, 0, "=SUM(SALES)", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh1, "v14_cut_name.hwk")
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.SwitchSheetForTest(1)
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	raw := cellRaw(t, sh2, 0, 0)
	if !strings.Contains(strings.ToUpper(raw), "SHEET1") || !strings.Contains(strings.ToUpper(raw), "SALES") {
		t.Fatalf("moved formula want Sheet1-qualified SALES, got %q", raw)
	}
	if v, ok := sh2.GetCellValue(0, 0).(float64); !ok || v != 30 {
		t.Fatalf("Sheet2 A1 SUM(Sheet1!SALES) want 30, got %v", sh2.GetCellValue(0, 0))
	}
}

func TestBugfixV14_TUIReplaceSkipsDigitsInRefs(t *testing.T) {
	wb := sheet.NewWorkbook("v14_replace")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "11", nil)
	sh.SetCellInput(1, 0, "20", nil)
	sh.SetCellInput(2, 0, "=A1+B1", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v14_replace.hwk")
	app.ExecuteActionHandlerForTest("doReplacePrompt", map[string]string{
		"find":    "1",
		"replace": "9",
		"scope":   "C",
	})
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if raw := cellRaw(t, sh, 2, 0); raw != "=A1+B1" {
		t.Fatalf("formula must keep A1+B1 after replacing digit 1, got %q", raw)
	}
}

func TestBugfixV14_NegativeFormatWithCommas(t *testing.T) {
	s1 := cell.FormatNumber(-1234567.89, cell.CellFormat{Type: cell.FmtComma, Decimals: 2})
	if s1 != "(1,234,567.89)" {
		t.Fatalf("want (1,234,567.89), got %q", s1)
	}

	s2 := cell.FormatNumber(-1000.0, cell.CellFormat{Type: cell.FmtComma, Decimals: 0})
	if s2 != "(1,000)" {
		t.Fatalf("want (1,000), got %q", s2)
	}

	s3 := cell.FormatNumber(1234567.89, cell.CellFormat{Type: cell.FmtComma, Decimals: 2})
	if s3 != "1,234,567.89" {
		t.Fatalf("want 1,234,567.89, got %q", s3)
	}
}

func TestBugfixV14_StrictErrorPropagation(t *testing.T) {
	wb := sheet.NewWorkbook("v14_err_prop")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "@NA", nil)
	sh.SetCellInput(0, 1, "=1+A1", nil)
	sh.SetCellInput(0, 2, "=SQRT(A1)", nil)
	sh.SetCellInput(0, 3, "=ROUND(A1, 2)", nil)
	sh.SetCellInput(0, 4, "=SIN(A1)", nil)
	sh.SetCellInput(0, 5, "=1e308 * 1e308", nil)
	wb.RecalculateAll()

	if err, ok := sh.GetCellValue(0, 1).(cell.LotusError); !ok || err.Code != cell.ErrNA.Code {
		t.Fatalf("1+A1 want #NA, got %v", sh.GetCellValue(0, 1))
	}
	if err, ok := sh.GetCellValue(0, 2).(cell.LotusError); !ok || err.Code != cell.ErrNA.Code {
		t.Fatalf("SQRT(A1) want #NA, got %v", sh.GetCellValue(0, 2))
	}
	if err, ok := sh.GetCellValue(0, 3).(cell.LotusError); !ok || err.Code != cell.ErrNA.Code {
		t.Fatalf("ROUND(A1, 2) want #NA, got %v", sh.GetCellValue(0, 3))
	}
	if err, ok := sh.GetCellValue(0, 4).(cell.LotusError); !ok || err.Code != cell.ErrNA.Code {
		t.Fatalf("SIN(A1) want #NA, got %v", sh.GetCellValue(0, 4))
	}
	if err, ok := sh.GetCellValue(0, 5).(cell.LotusError); !ok || err != cell.ErrLotus {
		t.Fatalf("overflow want ErrLotus, got %v", sh.GetCellValue(0, 5))
	}
}

func TestBugfixV14_InfoFunctionsWithCellRefs(t *testing.T) {
	wb := sheet.NewWorkbook("v14_info_refs")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "123", nil)
	sh.SetCellInput(0, 1, "'hello", nil)
	sh.SetCellInput(0, 2, "@NA", nil)
	// A4 is blank

	sh.SetCellInput(1, 0, "=ISNUMBER(A1)", nil)
	sh.SetCellInput(1, 1, "=ISNUMBER(A2)", nil)
	sh.SetCellInput(1, 2, "=ISNUMBER(A3)", nil)
	sh.SetCellInput(1, 3, "=ISNUMBER(A4)", nil)

	sh.SetCellInput(2, 0, "=ISTEXT(A1)", nil)
	sh.SetCellInput(2, 1, "=ISTEXT(A2)", nil)
	sh.SetCellInput(2, 2, "=ISTEXT(A3)", nil)

	sh.SetCellInput(3, 0, "=ISNONTEXT(A1)", nil)
	sh.SetCellInput(3, 1, "=ISNONTEXT(A2)", nil)
	sh.SetCellInput(3, 2, "=ISNONTEXT(A3)", nil)

	sh.SetCellInput(4, 0, "=ISBLANK(A4)", nil)
	sh.SetCellInput(4, 1, "=ISBLANK(A1)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 1.0 {
		t.Fatalf("ISNUMBER(A1) want 1, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 0.0 {
		t.Fatalf("ISNUMBER(A2) want 0, got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(1, 2).(float64); !ok || v != 0.0 {
		t.Fatalf("ISNUMBER(A3) error want 0, got %v", sh.GetCellValue(1, 2))
	}
	if v, ok := sh.GetCellValue(1, 3).(float64); !ok || v != 0.0 {
		t.Fatalf("ISNUMBER(A4) blank want 0, got %v", sh.GetCellValue(1, 3))
	}

	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 0.0 {
		t.Fatalf("ISTEXT(A1) want 0, got %v", sh.GetCellValue(2, 0))
	}
	if v, ok := sh.GetCellValue(2, 1).(float64); !ok || v != 1.0 {
		t.Fatalf("ISTEXT(A2) want 1, got %v", sh.GetCellValue(2, 1))
	}
	if v, ok := sh.GetCellValue(2, 2).(float64); !ok || v != 0.0 {
		t.Fatalf("ISTEXT(A3) error want 0, got %v", sh.GetCellValue(2, 2))
	}

	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 1.0 {
		t.Fatalf("ISNONTEXT(A1) want 1, got %v", sh.GetCellValue(3, 0))
	}
	if v, ok := sh.GetCellValue(3, 1).(float64); !ok || v != 0.0 {
		t.Fatalf("ISNONTEXT(A2) want 0, got %v", sh.GetCellValue(3, 1))
	}
	if v, ok := sh.GetCellValue(3, 2).(float64); !ok || v != 1.0 {
		t.Fatalf("ISNONTEXT(A3) error want 1, got %v", sh.GetCellValue(3, 2))
	}

	if v, ok := sh.GetCellValue(4, 0).(float64); !ok || v != 1.0 {
		t.Fatalf("ISBLANK(A4) blank want 1, got %v", sh.GetCellValue(4, 0))
	}
	if v, ok := sh.GetCellValue(4, 1).(float64); !ok || v != 0.0 {
		t.Fatalf("ISBLANK(A1) want 0, got %v", sh.GetCellValue(4, 1))
	}
}

func TestBugfixV14_CountFunctionsWithCellRefs(t *testing.T) {
	wb := sheet.NewWorkbook("v14_count_refs")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil) // A1
	// A2 blank
	sh.SetCellInput(0, 2, "30", nil) // A3
	sh.SetCellInput(1, 0, "=COUNT(A1, A2, A3)", nil)
	sh.SetCellInput(1, 1, "=COUNTA(A1, A2, A3)", nil)
	sh.SetCellInput(1, 2, "=COUNTBLANK(A1:A3)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 2.0 {
		t.Fatalf("COUNT(A1, A2, A3) want 2, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 2.0 {
		t.Fatalf("COUNTA(A1, A2, A3) want 2, got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(1, 2).(float64); !ok || v != 1.0 {
		t.Fatalf("COUNTBLANK(A1:A3) want 1, got %v", sh.GetCellValue(1, 2))
	}
}

func TestBugfixV14_LookupBoundsReturnRef(t *testing.T) {
	wb := sheet.NewWorkbook("v14_lookup_bounds")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(0, 1, "2", nil)
	sh.SetCellInput(1, 1, "20", nil)

	sh.SetCellInput(2, 0, "=VLOOKUP(1, A1:B2, 3, FALSE)", nil)
	sh.SetCellInput(2, 1, "=HLOOKUP(1, A1:B2, 3, FALSE)", nil)
	sh.SetCellInput(2, 2, "=INDEX(A1:B2, 5, 1)", nil)
	sh.SetCellInput(2, 3, "=INDEX(A1:B2, 1, 5)", nil)
	wb.RecalculateAll()

	if err, ok := sh.GetCellValue(2, 0).(cell.LotusError); !ok || err.Code != cell.ErrRef.Code {
		t.Fatalf("VLOOKUP out-of-bounds col want #REF!, got %v", sh.GetCellValue(2, 0))
	}
	if err, ok := sh.GetCellValue(2, 1).(cell.LotusError); !ok || err.Code != cell.ErrRef.Code {
		t.Fatalf("HLOOKUP out-of-bounds row want #REF!, got %v", sh.GetCellValue(2, 1))
	}
	if err, ok := sh.GetCellValue(2, 2).(cell.LotusError); !ok || err.Code != cell.ErrRef.Code {
		t.Fatalf("INDEX out-of-bounds row want #REF!, got %v", sh.GetCellValue(2, 2))
	}
	if err, ok := sh.GetCellValue(2, 3).(cell.LotusError); !ok || err.Code != cell.ErrRef.Code {
		t.Fatalf("INDEX out-of-bounds col want #REF!, got %v", sh.GetCellValue(2, 3))
	}
}

func TestBugfixV14_IfsDimensionMismatch(t *testing.T) {
	wb := sheet.NewWorkbook("v14_ifs_dim")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(0, 1, "2", nil)
	sh.SetCellInput(0, 2, "3", nil)
	sh.SetCellInput(0, 3, "4", nil)
	sh.SetCellInput(0, 4, "5", nil)

	sh.SetCellInput(1, 0, "1", nil)
	sh.SetCellInput(1, 1, "2", nil)
	sh.SetCellInput(1, 2, "3", nil)

	sh.SetCellInput(2, 0, "=SUMIFS(A1:A5, B1:B3, 1)", nil)
	sh.SetCellInput(2, 1, "=COUNTIFS(A1:A5, 1, B1:B3, 2)", nil)
	sh.SetCellInput(2, 2, "=AVERAGEIFS(A1:A5, B1:B3, 1)", nil)
	sh.SetCellInput(2, 3, "=MINIFS(A1:A5, B1:B3, 1)", nil)
	sh.SetCellInput(2, 4, "=MAXIFS(A1:A5, B1:B3, 1)", nil)
	wb.RecalculateAll()

	for c := 0; c < 5; c++ {
		if err, ok := sh.GetCellValue(2, c).(cell.LotusError); !ok || err != cell.ErrLotus {
			t.Fatalf("IFS dim mismatch col %d want ErrLotus, got %v", c, sh.GetCellValue(2, c))
		}
	}
}

// --- from bugfix_v15_regression_test.go ---

func TestBugfixV15_RenameAndDeleteQualifiedNames(t *testing.T) {
	wb := sheet.NewWorkbook("v15_qual_name")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")
	sh1.SetNamedRange("SALES", mustParseRange(t, "B1:B2"))
	sh1.SetCellInput(1, 0, "10", nil)
	sh1.SetCellInput(1, 1, "20", nil)
	sh2.SetCellInput(0, 0, "=SUM(Sheet1!SALES)", nil)
	wb.RecalculateAll()
	if v, ok := sh2.GetCellValue(0, 0).(float64); !ok || v != 30 {
		t.Fatalf("before rename SUM(Sheet1!SALES) want 30, got %v", sh2.GetCellValue(0, 0))
	}

	if err := wb.RenameSheet("Sheet1", "Data"); err != nil {
		t.Fatal(err)
	}
	raw := cellRaw(t, sh2, 0, 0)
	if !strings.Contains(strings.ToUpper(raw), "DATA") || strings.Contains(strings.ToUpper(raw), "SHEET1") {
		t.Fatalf("after rename want Data!SALES, got %q", raw)
	}
	if v, ok := sh2.GetCellValue(0, 0).(float64); !ok || v != 30 {
		t.Fatalf("after rename SUM(Data!SALES) want 30, got %v", sh2.GetCellValue(0, 0))
	}

	idx := wb.GetSheetIndex(wb.GetSheet("Data"))
	if err := wb.DeleteSheet(idx); err != nil {
		t.Fatal(err)
	}
	rawDel := cellRaw(t, sh2, 0, 0)
	if !strings.Contains(rawDel, "#REF!") {
		t.Fatalf("after delete want #REF!, got %q", rawDel)
	}

	reborn := wb.AddSheet("Data")
	reborn.SetNamedRange("SALES", mustParseRange(t, "A1"))
	reborn.SetCellInput(0, 0, "99", nil)
	wb.RecalculateAll()
	v := sh2.GetCellValue(0, 0)
	if _, isErr := v.(cell.LotusError); !isErr {
		t.Fatalf("recreated Data must not revive deleted name ref, got %v (raw=%q)", v, cellRaw(t, sh2, 0, 0))
	}
}

func TestBugfixV15_MatchVLookupSkipErrors(t *testing.T) {
	wb := sheet.NewWorkbook("v15_lookup_err")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(0, 1, "@NA", nil)
	sh.SetCellInput(0, 2, "3", nil)
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(1, 1, "20", nil)
	sh.SetCellInput(1, 2, "30", nil)
	sh.SetCellInput(2, 0, "=MATCH(3, A1:A3, 1)", nil)
	sh.SetCellInput(2, 1, "=VLOOKUP(3, A1:B3, 2, TRUE)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 3 {
		t.Fatalf("MATCH(3, {1,#N/A,3}, 1) want 3, got %v", sh.GetCellValue(2, 0))
	}
	if v, ok := sh.GetCellValue(2, 1).(float64); !ok || v != 30 {
		t.Fatalf("VLOOKUP approx with error row want 30, got %v", sh.GetCellValue(2, 1))
	}
}

func TestBugfixV15_SortAdjustsFormulas(t *testing.T) {
	wb := sheet.NewWorkbook("v15_sort")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "5", nil)
	sh.SetCellInput(1, 0, "=A1*2", nil)
	sh.SetCellInput(1, 1, "=A2*2", nil)
	wb.RecalculateAll()

	rng := mustParseRange(t, "A1:B2")
	sh.SortRange(rng, 0, true)
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 5 {
		t.Fatalf("A1 after sort want 5, got %v", sh.GetCellValue(0, 0))
	}
	if raw := cellRaw(t, sh, 1, 0); raw != "=A1*2" {
		t.Fatalf("B1 after sort want =A1*2 (moved with row), got %q", raw)
	}
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 10 {
		t.Fatalf("B1 value after sort want 10, got %v", sh.GetCellValue(1, 0))
	}
}

func TestBugfixV15_2DAutoFillDoesNotOverwrite(t *testing.T) {
	wb := sheet.NewWorkbook("v15_autofill_keep")
	sh := wb.Sheets[0]
	sh.SetCellInput(3, 0, "7", nil) // D1
	sh.SetCellInput(0, 0, "=D1", nil)
	sh.SetCellInput(1, 1, "keep", nil) // B2
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v15_autofill_keep.hwk")
	rng := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 2, Row: 2},
	}
	app.SelectRangeForTest(&rng)
	app.TriggerAutoFillForTest()
	if v := sh.GetCellValue(1, 1); v != "keep" {
		t.Fatalf("B2 must not be overwritten by 2D AutoFill, got %v (raw=%q)", v, cellRaw(t, sh, 1, 1))
	}
}

func TestBugfixV15_CutClipboardFollowsRename(t *testing.T) {
	wb := sheet.NewWorkbook("v15_cut_rename")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")
	sh2.SetCellInput(0, 0, "100", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh1, "v15_cut_rename.hwk")
	app.SwitchSheetForTest(1)
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.ExecuteActionHandlerForTest("doWorksheetRename", map[string]string{"name": "Src"})
	app.SwitchSheetForTest(0)
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))
	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 100 {
		t.Fatalf("Sheet1 A1 after cut-from-renamed-sheet want 100, got %v", sh1.GetCellValue(0, 0))
	}
}

func TestBugfixV15_PasteValuesCompletesCut(t *testing.T) {
	wb := sheet.NewWorkbook("v15_paste_values_cut")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "100", nil)
	sh.SetCellInput(1, 0, "=A1", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v15_paste_values_cut.hwk")
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0)
	app.ExecuteActionHandlerForTest("doPalettePasteValues", nil)
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 100 {
		t.Fatalf("D1 paste values want 100, got %v", sh.GetCellValue(3, 0))
	}
	if raw := cellRaw(t, sh, 1, 0); raw != "=D1" {
		t.Fatalf("B1 should retarget to D1 after cut+paste values, got %q", raw)
	}
	app.SetCursorForTest(4, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))
	if cellHasValue(sh, 4, 0) {
		t.Fatalf("E1 should stay empty; cut clipboard must be consumed, got %v", sh.GetCellValue(4, 0))
	}
}

func TestBugfixV15_OffsetChooseAndNamedExpr(t *testing.T) {
	wb := sheet.NewWorkbook("v15_offset_choose")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "30", nil)
	sh.SetNamedRange("ANCHOR", formula.NamedExpr{Expr: "=$A$1"})
	sh.SetCellInput(1, 0, "=OFFSET(CHOOSE(1, A1, B1), 1, 0)", nil)
	sh.SetCellInput(1, 1, "=ROW(ANCHOR)", nil)
	sh.SetCellInput(1, 2, "=OFFSET(ANCHOR, 0, 0)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 30 {
		t.Fatalf("OFFSET(CHOOSE(1,A1,B1),1,0) want 30, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 1 {
		t.Fatalf("ROW(ANCHOR) want 1, got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(1, 2).(float64); !ok || v != 10 {
		t.Fatalf("OFFSET(ANCHOR,0,0) want 10, got %v", sh.GetCellValue(1, 2))
	}
}

func TestBugfixV15_ChoosePropagatesUnusedErrors(t *testing.T) {
	wb := sheet.NewWorkbook("v15_choose_err")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=CHOOSE(1, 10, 1/0)", nil)
	wb.RecalculateAll()
	v := sh.GetCellValue(0, 0)
	if _, ok := v.(cell.LotusError); !ok {
		t.Fatalf("CHOOSE unused 1/0 should propagate error, got %v", v)
	}
}

func TestBugfixV15_NestedOffsetResolve(t *testing.T) {
	wb := sheet.NewWorkbook("v15_nested_offset")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)                              // A1
	sh.SetCellInput(0, 3, "40", nil)                              // A4
	sh.SetCellInput(1, 0, "=OFFSET(OFFSET(A1, 1, 0), 2, 0)", nil) // B1
	wb.RecalculateAll()

	v := sh.GetCellValue(1, 0)
	if num, ok := v.(float64); !ok || num != 40 {
		t.Fatalf("OFFSET(OFFSET(A1, 1, 0), 2, 0) want 40, got %v", v)
	}
}

func TestBugfixV15_TextAndAddressUnwrap(t *testing.T) {
	wb := sheet.NewWorkbook("v15_unwrap")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "Sheet1", nil)                      // A1 = "Sheet1"
	sh.SetCellInput(1, 0, "=ADDRESS(1, 1, 1, TRUE, A1)", nil) // B1
	sh.SetCellInput(0, 1, "'0.00", nil)                       // A2 = "0.00" as text
	sh.SetCellInput(1, 1, "=TEXT(12.345, A2)", nil)           // B2
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(1, 0).(string); !ok || v != "Sheet1!$A$1" {
		t.Fatalf("ADDRESS with cell-ref sheet want 'Sheet1!$A$1', got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(string); !ok || v != "12.35" {
		t.Fatalf("TEXT with cell-ref format want '12.35', got %v", sh.GetCellValue(1, 1))
	}
}

func TestBugfixV15_LookupEmptyCellZero(t *testing.T) {
	wb := sheet.NewWorkbook("v15_lookup_empty")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "apple", nil)  // A1
	sh.SetCellInput(0, 1, "banana", nil) // A2
	// B1 is empty (nil)
	sh.SetCellInput(1, 1, "yellow", nil) // B2
	sh.SetCellInput(2, 0, "=XLOOKUP(\"apple\", A1:A2, B1:B2)", nil)
	sh.SetCellInput(2, 1, "=LOOKUP(\"apple\", A1:A1, B1:B1)", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 0 {
		t.Fatalf("XLOOKUP empty cell want 0, got %v", sh.GetCellValue(2, 0))
	}
	if v, ok := sh.GetCellValue(2, 1).(float64); !ok || v != 0 {
		t.Fatalf("LOOKUP empty cell want 0, got %v", sh.GetCellValue(2, 1))
	}
}

func TestBugfixV15_TimeStringHourMinuteSecond(t *testing.T) {
	wb := sheet.NewWorkbook("v15_time_str")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=HOUR(\"14:30:45\")", nil)
	sh.SetCellInput(0, 1, "=MINUTE(\"14:30:45\")", nil)
	sh.SetCellInput(0, 2, "=SECOND(\"14:30:45\")", nil)
	wb.RecalculateAll()

	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 14 {
		t.Fatalf("HOUR(\"14:30:45\") want 14, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != 30 {
		t.Fatalf("MINUTE(\"14:30:45\") want 30, got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 45 {
		t.Fatalf("SECOND(\"14:30:45\") want 45, got %v", sh.GetCellValue(0, 2))
	}
}

func TestBugfixV15_Atan2ZeroZero(t *testing.T) {
	wb := sheet.NewWorkbook("v15_atan2")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=ATAN2(0, 0)", nil)
	wb.RecalculateAll()

	v := sh.GetCellValue(0, 0)
	if _, ok := v.(cell.LotusError); !ok {
		t.Fatalf("ATAN2(0, 0) want LotusError (#DIV/0!), got %v", v)
	}
}

func TestBugfixV15_MathOverflowAndNaN(t *testing.T) {
	wb := sheet.NewWorkbook("v15_math_overflow")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=ROUND(1.23, 400)", nil)
	sh.SetCellInput(0, 1, "=EXP(1000)", nil)
	sh.SetCellInput(0, 2, "=PRODUCT(1e200, 1e200)", nil)
	wb.RecalculateAll()

	for row := 0; row < 3; row++ {
		v := sh.GetCellValue(0, row)
		if _, ok := v.(cell.LotusError); !ok {
			t.Fatalf("row %d overflow want LotusError, got %v", row, v)
		}
	}
}

func TestBugfixV15_ErrorExportXLSXAndODS(t *testing.T) {
	wb := sheet.NewWorkbook("v15_export_err")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=1/0", nil) // A1 is #DIV/0!
	wb.RecalculateAll()

	xlsxTmp := "/tmp/test_v15_err.xlsx"
	odsTmp := "/tmp/test_v15_err.ods"
	defer os.Remove(xlsxTmp)
	defer os.Remove(odsTmp)

	if err := wb.ExportXLSX(xlsxTmp); err != nil {
		t.Fatalf("ExportXLSX failed with error cell: %v", err)
	}
	if err := wb.ExportODS(odsTmp); err != nil {
		t.Fatalf("ExportODS failed with error cell: %v", err)
	}
}

func TestBugfixV15_ReplaceCaseInsensitiveUTF8(t *testing.T) {
	res, changed := formula.ReplaceInFormula("=\"テスト文字列\"", "テスト", "サンプル")
	if !changed {
		t.Fatalf("ReplaceInFormula should have changed")
	}
	if res != "=\"サンプル文字列\"" {
		t.Fatalf("ReplaceInFormula result want '=\"サンプル文字列\"', got %q", res)
	}
}

func TestBugfixV15_LargeFloatGeneralFormat(t *testing.T) {
	formatted := cell.FormatNumber(math.MaxFloat64, cell.CellFormat{Type: cell.FmtGeneral})
	if formatted == "" {
		t.Fatalf("formatting MaxFloat64 returned empty string")
	}
}

// --- from bugfix_v16_regression_test.go ---

func TestBugfixV16_XLookupHitIgnoresIfNotFoundNA(t *testing.T) {
	wb := sheet.NewWorkbook("v16_xlookup")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(0, 1, "2", nil)
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(1, 1, "20", nil)
	sh.SetCellInput(2, 0, "=XLOOKUP(1, A1:A2, B1:B2, NA())", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 10 {
		t.Fatalf("XLOOKUP hit with NA() if_not_found want 10, got %v", sh.GetCellValue(2, 0))
	}
}

func TestBugfixV16_LookupTwoArgMatrix(t *testing.T) {
	wb := sheet.NewWorkbook("v16_lookup2")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(0, 1, "2", nil)
	sh.SetCellInput(0, 2, "3", nil)
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(1, 1, "20", nil)
	sh.SetCellInput(1, 2, "30", nil)
	sh.SetCellInput(2, 0, "=LOOKUP(2, A1:B3)", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 20 {
		t.Fatalf("LOOKUP(2, A1:B3) vertical want 20, got %v", sh.GetCellValue(2, 0))
	}

	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(1, 0, "2", nil)
	sh.SetCellInput(2, 0, "3", nil)
	sh.SetCellInput(0, 1, "10", nil)
	sh.SetCellInput(1, 1, "20", nil)
	sh.SetCellInput(2, 1, "30", nil)
	sh.SetCellInput(3, 0, "=LOOKUP(2, A1:C2)", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 20 {
		t.Fatalf("LOOKUP(2, A1:C2) horizontal want 20, got %v", sh.GetCellValue(3, 0))
	}
}

func TestBugfixV16_TextMonthAndAMPM(t *testing.T) {
	wb := sheet.NewWorkbook("v16_text")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, `=TEXT(45000, "mmm")`, nil)
	sh.SetCellInput(0, 1, `=TEXT(0.5, "h:mm AM/PM")`, nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(string); !ok || v != "Mar" {
		t.Fatalf("TEXT(45000,\"mmm\") want Mar, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(string); !ok || v != "12:00 PM" {
		t.Fatalf("TEXT(0.5,\"h:mm AM/PM\") want 12:00 PM, got %v", sh.GetCellValue(0, 1))
	}
}

func TestBugfixV16_CutPasteLinkNotSelfRef(t *testing.T) {
	wb := sheet.NewWorkbook("v16_paste_link")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "100", nil) // A1
	sh.SetCellInput(1, 0, "=A1", nil) // B1
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v16_paste_link.hwk")
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(2, 0) // C1
	app.ExecuteActionHandlerForTest("doPalettePasteLink", nil)

	rawC := cellRaw(t, sh, 2, 0)
	if strings.EqualFold(rawC, "=C1") {
		t.Fatalf("C1 after cut+paste link must not be self-ref, got %q", rawC)
	}
	if rawC != "=A1" {
		t.Fatalf("C1 after cut+paste link want =A1, got %q", rawC)
	}
	if raw := cellRaw(t, sh, 1, 0); raw != "=C1" {
		t.Fatalf("B1 should retarget to C1, got %q", raw)
	}
}

func TestBugfixV16_CutPasteTransposeRetarget(t *testing.T) {
	wb := sheet.NewWorkbook("v16_transpose")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)  // A1
	sh.SetCellInput(1, 0, "20", nil)  // B1
	sh.SetCellInput(2, 0, "=B1", nil) // C1
	rx := mustParseRange(t, "A1:B1")
	sh.Graph().RangeX = &rx
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v16_transpose.hwk")
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(0, 2) // A3
	app.ExecuteActionHandlerForTest("doPasteTranspose", nil)

	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 10 {
		t.Fatalf("A3 after transpose want 10, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 20 {
		t.Fatalf("A4 after transpose want 20, got %v", sh.GetCellValue(0, 3))
	}
	if raw := cellRaw(t, sh, 2, 0); raw != "=A4" {
		t.Fatalf("C1 after transpose want =A4, got %q", raw)
	}
	gx := sh.Graph().RangeX
	if gx == nil {
		t.Fatal("RangeX should follow transpose")
	}
	if gx.MinCol() != 0 || gx.MaxCol() != 0 || gx.MinRow() != 2 || gx.MaxRow() != 3 {
		t.Fatalf("RangeX after transpose want A3:A4, got %s", gx.String())
	}
}

func TestBugfixV16_XLSXFreezeRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("v16_freeze")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "x", nil)
	sh.SetFrozenRows(3)
	sh.SetFrozenCols(2)

	p := filepath.Join(t.TempDir(), "freeze.xlsx")
	if err := wb.ExportXLSX(p); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}
	loaded, err := sheet.ImportXLSXWorkbook(p)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	got := loaded.Sheets[0]
	if got.FrozenRows() != 3 || got.FrozenCols() != 2 {
		t.Fatalf("freeze round-trip want rows=3 cols=2, got rows=%d cols=%d", got.FrozenRows(), got.FrozenCols())
	}
	_ = os.Remove(p)
}

func TestBugfixV16_CountIfEscapedWildcard(t *testing.T) {
	wb := sheet.NewWorkbook("v16_countif")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "*", nil)
	sh.SetCellInput(0, 1, "abc", nil)
	sh.SetCellInput(0, 2, "star*", nil)
	sh.SetCellInput(1, 0, `=COUNTIF(A1:A3, "~*")`, nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 1 {
		t.Fatalf("COUNTIF(..., \"~*\") want 1 (literal *), got %v", sh.GetCellValue(1, 0))
	}
}

func TestBugfixV16_RowsNamedAndIndirect(t *testing.T) {
	wb := sheet.NewWorkbook("v16_rows")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "@NA", nil)
	sh.SetNamedRange("DATA", mustParseRange(t, "A1"))
	sh.SetCellInput(1, 0, "=ROWS(DATA)", nil)
	sh.SetCellInput(1, 1, `=ROWS(INDIRECT("A1"))`, nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 1 {
		t.Fatalf("ROWS(DATA) with #N/A cell want 1, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 1 {
		t.Fatalf("ROWS(INDIRECT(\"A1\")) want 1, got %v", sh.GetCellValue(1, 1))
	}
}

func TestBugfixV16_IndexOmittedArg(t *testing.T) {
	wb := sheet.NewWorkbook("v16_index")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(1, 0, "2", nil)
	sh.SetCellInput(0, 1, "3", nil)
	sh.SetCellInput(1, 1, "4", nil)
	sh.SetCellInput(2, 0, "=SUM(INDEX(A1:B2,,1))", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 4 {
		t.Fatalf("SUM(INDEX(A1:B2,,1)) want 4, got %v", sh.GetCellValue(2, 0))
	}
}

func TestBugfixV16_MatchWildcardSkipsNumbers(t *testing.T) {
	wb := sheet.NewWorkbook("v16_match")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "11", nil)
	sh.SetCellInput(0, 2, "12", nil)
	sh.SetCellInput(1, 0, `=MATCH("1*", A1:A3, 0)`, nil)
	wb.RecalculateAll()
	v := sh.GetCellValue(1, 0)
	if _, ok := v.(cell.LotusError); !ok {
		t.Fatalf("MATCH(\"1*\", numeric col, 0) want #N/A, got %v", v)
	}
}

// --- from bugfix_v17_regression_test.go ---

func TestBugfixV17_IndirectR1C1(t *testing.T) {
	wb := sheet.NewWorkbook("v17_r1c1")
	sh := wb.Sheets[0]
	sh.SetCellInput(2, 2, "99", nil) // C3
	sh.SetCellInput(0, 0, `=INDIRECT("R3C3", FALSE)`, nil)
	sh.SetCellInput(1, 1, `=INDIRECT("R[1]C[1]", FALSE)`, nil) // B2 -> C3
	sh.SetCellInput(3, 0, `=SUM(INDIRECT("R3C3:R3C3", FALSE))`, nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 99 {
		t.Fatalf("INDIRECT(\"R3C3\", FALSE) want 99, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 99 {
		t.Fatalf("INDIRECT(\"R[1]C[1]\", FALSE) from B2 want 99, got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 99 {
		t.Fatalf("SUM(INDIRECT R1C1 range) want 99, got %v", sh.GetCellValue(3, 0))
	}
}

func TestBugfixV17_TextScientific(t *testing.T) {
	wb := sheet.NewWorkbook("v17_text_sci")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, `=TEXT(1234.5, "0.00E+00")`, nil)
	sh.SetCellInput(0, 1, `=TEXT(0.5, "0.00E+00")`, nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(string); !ok || v != "1.23E+03" {
		t.Fatalf("TEXT(1234.5,\"0.00E+00\") want 1.23E+03, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(string); !ok || v != "5.00E-01" {
		t.Fatalf("TEXT(0.5,\"0.00E+00\") want 5.00E-01, got %v", sh.GetCellValue(0, 1))
	}
}

func TestBugfixV17_ODSFreezeRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("v17_ods_freeze")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "x", nil)
	sh.SetFrozenRows(3)
	sh.SetFrozenCols(2)
	p := filepath.Join(t.TempDir(), "freeze.ods")
	if err := wb.ExportODS(p); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	loaded, err := sheet.ImportODSWorkbook(p)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	got := loaded.Sheets[0]
	if got.FrozenRows() != 3 || got.FrozenCols() != 2 {
		t.Fatalf("ODS freeze want rows=3 cols=2, got rows=%d cols=%d", got.FrozenRows(), got.FrozenCols())
	}
}

func TestBugfixV17_XLSXManualRecalcRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("v17_xlsx_manual")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "=A1*2", nil)
	sh.SetRecalcMode("MANUAL")
	wb.RecalculateAll()
	p := filepath.Join(t.TempDir(), "manual.xlsx")
	if err := wb.ExportXLSX(p); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}
	loaded, err := sheet.ImportXLSXWorkbook(p)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	lsh := loaded.Sheets[0]
	if lsh.RecalcMode() != "MANUAL" {
		t.Fatalf("XLSX RecalcMode want MANUAL, got %q", lsh.RecalcMode())
	}
	if v, ok := lsh.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("imported B1 want 20, got %v", lsh.GetCellValue(1, 0))
	}
	lsh.SetCellInput(0, 0, "7", nil)
	if v, ok := lsh.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("MANUAL edit of A1 should leave B1 at 20, got %v", lsh.GetCellValue(1, 0))
	}
}

func TestBugfixV17_ParseR1C1Helpers(t *testing.T) {
	cr, err := coord.ParseR1C1CellRef("R1C1", 5, 5)
	if err != nil || cr.Col != 0 || cr.Row != 0 {
		t.Fatalf("R1C1 want A1, got %+v err=%v", cr, err)
	}
	cr, err = coord.ParseR1C1CellRef("R[1]C[2]", 1, 1)
	if err != nil || cr.Col != 3 || cr.Row != 2 {
		t.Fatalf("R[1]C[2] from B2 want D3, got %+v err=%v", cr, err)
	}
	rr, err := coord.ParseR1C1RangeRef("Sheet2!R1C1:R2C2", 0, 0)
	if err != nil || rr.Sheet != "Sheet2" || rr.MinCol() != 0 || rr.MaxRow() != 1 {
		t.Fatalf("range want Sheet2!A1:B2, got %+v err=%v", rr, err)
	}
}

// --- from bugfix_v18_regression_test.go ---

func TestBugfixV18_RowColumnCurrentPosNotClobbered(t *testing.T) {
	wb := sheet.NewWorkbook("v18_row_clobber")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 4, "=2+2", nil)      // A5
	sh.SetCellInput(1, 0, "=A5+ROW()", nil) // B1
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 5 {
		t.Fatalf("B1 =A5+ROW() want 5, got %v", sh.GetCellValue(1, 0))
	}

	sh.SetCellInput(1, 2, "=7", nil) // B3
	sh.SetCellInput(0, 0, "=B3+ROW()", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 8 {
		t.Fatalf("A1 =B3+ROW() want 8, got %v", sh.GetCellValue(0, 0))
	}
}

func TestBugfixV18_RenameSheetQuotesSpecialChars(t *testing.T) {
	wb := sheet.NewWorkbook("v18_rename_dash")
	data := wb.AddSheet("Data")
	data.SetCellInput(0, 0, "42", nil)
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=Data!A1", nil)
	wb.RecalculateAll()
	if err := wb.RenameSheet("Data", "Q1-2024"); err != nil {
		t.Fatalf("RenameSheet: %v", err)
	}
	wb.RecalculateAll()
	raw := sh.GetCell(0, 0).RawInput
	if !strings.Contains(raw, "'Q1-2024'") {
		t.Fatalf("renamed formula want quoted sheet, got %q", raw)
	}
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 42 {
		t.Fatalf("value after rename want 42, got %v", sh.GetCellValue(0, 0))
	}
}

func TestBugfixV18_ODSStringCellsStayLabels(t *testing.T) {
	wb := sheet.NewWorkbook("v18_ods_string")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "'007", nil)
	sh.SetCellInput(0, 1, "'=1+1", nil)
	p := filepath.Join(t.TempDir(), "str.ods")
	if err := wb.ExportODS(p); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	loaded, err := sheet.ImportODSWorkbook(p)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	lsh := loaded.Sheets[0]
	c0 := lsh.GetCell(0, 0)
	if c0 == nil || c0.Type != cell.TypeLabel {
		t.Fatalf("A1 want LABEL, got %+v", c0)
	}
	if c0.Value != "007" {
		t.Fatalf("A1 value want 007, got %v", c0.Value)
	}
	c1 := lsh.GetCell(0, 1)
	if c1 == nil || c1.Type != cell.TypeLabel {
		t.Fatalf("A2 want LABEL, got %+v", c1)
	}
	if c1.Value != "=1+1" {
		t.Fatalf("A2 value want =1+1, got %v", c1.Value)
	}
}

func TestBugfixV18_XLSXStringLiteralColonPreserved(t *testing.T) {
	got := sheet.ConvertExcelFormulaWithTablesForTest(`CONCATENATE("A1:B2","!")`, 0, 0, nil)
	if strings.Contains(got, "A1..B2") {
		t.Fatalf("colon inside string was rewritten: %q", got)
	}
	if !strings.Contains(got, "A1:B2") {
		t.Fatalf("want A1:B2 inside quotes, got %q", got)
	}

	wb := sheet.NewWorkbook("v18_xlsx_colon")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, `="See A1:B2 for details"`, nil)
	wb.RecalculateAll()
	p := filepath.Join(t.TempDir(), "colon.xlsx")
	if err := wb.ExportXLSX(p); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}
	loaded, err := sheet.ImportXLSXWorkbook(p)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	lsh := loaded.Sheets[0]
	lsh.SetCellInput(1, 0, "1", nil) // force a recalc path
	loaded.RecalculateAll()
	if v, ok := lsh.GetCellValue(0, 0).(string); !ok || v != "See A1:B2 for details" {
		t.Fatalf("round-trip string want See A1:B2 for details, got %v (raw %q)", lsh.GetCellValue(0, 0), lsh.GetCell(0, 0).RawInput)
	}
}

func TestBugfixV18_ODSStringLiteralBracketsPreserved(t *testing.T) {
	got := sheet.ConvertODSFormulaForTest(`="see [note]"`)
	if strings.Contains(got, "NOTE") && !strings.Contains(got, "[note]") {
		t.Fatalf("bracket text inside string was rewritten: %q", got)
	}
	if !strings.Contains(got, "[note]") {
		t.Fatalf("want [note] preserved, got %q", got)
	}

	wb := sheet.NewWorkbook("v18_ods_note")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, `="see [note]"`, nil)
	sh.SetCellInput(0, 1, `="range A1:B2"`, nil)
	wb.RecalculateAll()
	p := filepath.Join(t.TempDir(), "note.ods")
	if err := wb.ExportODS(p); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	loaded, err := sheet.ImportODSWorkbook(p)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	lsh := loaded.Sheets[0]
	loaded.RecalculateAll()
	if v, ok := lsh.GetCellValue(0, 0).(string); !ok || v != "see [note]" {
		t.Fatalf("see [note] want preserved, got %v raw=%q", lsh.GetCellValue(0, 0), lsh.GetCell(0, 0).RawInput)
	}
	if v, ok := lsh.GetCellValue(0, 1).(string); !ok || v != "range A1:B2" {
		t.Fatalf("range A1:B2 want preserved, got %v raw=%q", lsh.GetCellValue(0, 1), lsh.GetCell(0, 1).RawInput)
	}
}

func TestBugfixV18_SumIfExpandsSumRange(t *testing.T) {
	wb := sheet.NewWorkbook("v18_sumif")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(0, 1, "2", nil)
	sh.SetCellInput(0, 2, "3", nil)
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(1, 1, "20", nil)
	sh.SetCellInput(1, 2, "30", nil)
	sh.SetCellInput(2, 0, `=SUMIF(A1:A3,">1",B1:B1)`, nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(2, 0).(float64); !ok || v != 50 {
		t.Fatalf("SUMIF expand want 50, got %v", sh.GetCellValue(2, 0))
	}
}

func TestBugfixV18_HwkLargeNumberRawInput(t *testing.T) {
	wb := sheet.NewWorkbook("v18_hwk_big")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1e20", nil)
	p := filepath.Join(t.TempDir(), "big.hwk")
	if err := sh.SaveJSON(p); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}
	loaded, err := sheet.LoadSheetJSON(p)
	if err != nil {
		t.Fatalf("LoadSheetJSON: %v", err)
	}
	c := loaded.GetCell(0, 0)
	if c == nil {
		t.Fatal("A1 missing after load")
	}
	if c.RawInput == "9223372036854775807" {
		t.Fatalf("RawInput overflowed int64: %q", c.RawInput)
	}
	v, ok := c.Value.(float64)
	if !ok || v < 1e19 {
		t.Fatalf("value want ~1e20, got %v", c.Value)
	}
}

func TestBugfixV18_Days360USFebruaryEOM(t *testing.T) {
	wb := sheet.NewWorkbook("v18_days360")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=DAYS360(DATE(2024,2,29),DATE(2024,3,31))", nil)
	sh.SetCellInput(0, 1, "=DAYS360(DATE(2023,2,28),DATE(2023,3,31))", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 30 {
		t.Fatalf("leap Feb EOM want 30, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != 30 {
		t.Fatalf("common Feb EOM want 30, got %v", sh.GetCellValue(0, 1))
	}
}

func TestBugfixV18_WeekNumReturnTypes12to16(t *testing.T) {
	wb := sheet.NewWorkbook("v18_weeknum")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=WEEKNUM(DATE(2026,1,3),16)", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 2 {
		t.Fatalf("WEEKNUM(...,16) want 2, got %v", sh.GetCellValue(0, 0))
	}
}

func TestBugfixV18_TimeWrapsAndRejectsNegative(t *testing.T) {
	wb := sheet.NewWorkbook("v18_time")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=TIME(25,0,0)", nil)
	sh.SetCellInput(0, 1, "=TIME(-1,0,0)", nil)
	wb.RecalculateAll()
	v, ok := sh.GetCellValue(0, 0).(float64)
	if !ok || v < 0.04166 || v > 0.04167 {
		t.Fatalf("TIME(25,0,0) want ~0.041666, got %v", sh.GetCellValue(0, 0))
	}
	if _, isErr := sh.GetCellValue(0, 1).(cell.LotusError); !isErr {
		t.Fatalf("TIME(-1,0,0) want ERR, got %v", sh.GetCellValue(0, 1))
	}
}

func TestBugfixV18_AlignAndPadWideTruncation(t *testing.T) {
	got := cell.AlignAndPad("あいうえお", 5, cell.AlignRight)
	if runewidth.StringWidth(got) != 5 {
		t.Fatalf("right-align width want 5, got %d %q", runewidth.StringWidth(got), got)
	}
	c := cell.NewCell(`\ー`, nil)
	rendered := c.Render(9, cell.CellFormat{})
	if runewidth.StringWidth(rendered) != 9 {
		t.Fatalf("repeat width want 9, got %d %q", runewidth.StringWidth(rendered), rendered)
	}
}

func TestBugfixV18_GraphNegativeCommas(t *testing.T) {
	if got := tui.FormatWithCommasForTest(-1234567); got != "-1,234,567" {
		t.Fatalf("formatWithCommas(-1234567) want -1,234,567, got %q", got)
	}
}

func TestBugfixV18_RTLCombiningMarkOrder(t *testing.T) {
	in := "בָ" // bet + qamats
	out := tui.PrepareTextForRendering(in)
	first, _ := utf8.DecodeRuneInString(out)
	if first == '\u05B8' {
		t.Fatalf("combining mark led the reversed run: %q -> %q", in, out)
	}
}

func TestBugfixV18_GraphYAxisLargeLabelsFit(t *testing.T) {
	if got := tui.FormatAxisTickForTest(2000000); got != "2.0M" {
		t.Fatalf("tick 2000000 want 2.0M, got %q", got)
	}
	if got := tui.FormatAxisTickForTest(1000000); got != "1.0M" {
		t.Fatalf("tick 1000000 want 1.0M, got %q", got)
	}
	if w := runewidth.StringWidth("2.0M"); w > 7 {
		t.Fatalf("compact tick still wider than old gutter: %d", w)
	}
}

func TestBugfixV18_CutInsertRowPasteRetarget(t *testing.T) {
	wb := sheet.NewWorkbook("v18_cut_insert")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 4, "99", nil)  // A5
	sh.SetCellInput(3, 0, "=A5", nil) // D1
	wb.RecalculateAll()

	app := newSimApp(t, sh, "v18_cut_insert.hwk")
	app.SetCursorForTest(0, 4)
	app.ExecuteActionHandlerForTest("doPaletteCut", nil)
	app.ExecuteActionHandlerForTest("doInsertRow", map[string]string{"range": "A1"})
	app.SetCursorForTest(1, 9) // B10
	app.ExecuteActionHandlerForTest("doPalettePaste", nil)

	c := sh.GetCell(3, 1) // D2 after insert
	if c == nil {
		t.Fatal("D2 missing after insert")
	}
	if c.RawInput != "=B10" {
		t.Fatalf("D2 raw after cut+insert+paste want =B10, got %q", c.RawInput)
	}
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(3, 1).(float64); !ok || v != 99 {
		t.Fatalf("D2 value want 99, got %v", sh.GetCellValue(3, 1))
	}
}

func TestBugfixV18_DeleteRowDropsNilGraphSeries(t *testing.T) {
	wb := sheet.NewWorkbook("v18_graph_nil")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(0, 1, "2", nil)
	sh.SetCellInput(0, 2, "3", nil)
	rng := mustParseRange(t, "A1:A3")
	sh.Graph().Series["A"] = &rng
	sh.DeleteRow(0, 3)
	if _, ok := sh.Graph().Series["A"]; ok {
		t.Fatalf("series A should be removed, got %#v", sh.Graph().Series["A"])
	}
}

func TestBugfixV18_MinMaxTextOnlyRangeIsZero(t *testing.T) {
	wb := sheet.NewWorkbook("v18_minmax")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "abc", nil)
	sh.SetCellInput(0, 1, "def", nil)
	sh.SetCellInput(1, 0, "=MIN(A1:A2)", nil)
	sh.SetCellInput(1, 1, "=MAX(A1:A2)", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 0 {
		t.Fatalf("MIN(text) want 0, got %v", sh.GetCellValue(1, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 0 {
		t.Fatalf("MAX(text) want 0, got %v", sh.GetCellValue(1, 1))
	}
}

func TestBugfixV18_ManualInsertKeepsFormulaValue(t *testing.T) {
	wb := sheet.NewWorkbook("v18_manual_insert")
	sh := wb.Sheets[0]
	sh.SetRecalcMode("MANUAL")
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "=A1*2", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != 20 {
		t.Fatalf("pre-insert want 20, got %v", sh.GetCellValue(0, 1))
	}
	sh.InsertRow(0, 1)
	c := sh.GetCell(0, 2)
	if c == nil || c.Value == nil {
		t.Fatalf("MANUAL insert should keep cached value, cell=%+v", c)
	}
	if v, ok := c.Value.(float64); !ok || v != 20 {
		t.Fatalf("cached value want 20, got %v", c.Value)
	}
}

func TestBugfixV18_ParseCellRefRejectsOutOfBounds(t *testing.T) {
	if _, err := coord.ParseCellRef("XFE1"); err == nil {
		t.Fatal("XFE1 should be out of bounds")
	}
	if _, err := coord.ParseCellRef("A1048577"); err == nil {
		t.Fatal("A1048577 should be out of bounds")
	}
	if _, err := coord.ParseCellRef("XFD1048576"); err != nil {
		t.Fatalf("XFD1048576 should be valid, got %v", err)
	}
}

func TestBugfixV18_NamedRangeCreateSetsModified(t *testing.T) {
	wb := sheet.NewWorkbook("v18_named")
	sh := wb.Sheets[0]
	sh.SetModified(false)
	app := newSimApp(t, sh, "v18_named.hwk")
	app.ExecuteActionHandlerForTest("doRangeNameCreate", map[string]string{
		"name":  "FOO",
		"range": "A1:A3",
	})
	if !sh.IsModified() {
		t.Fatal("creating a named range should mark the sheet modified")
	}
	if _, ok := sh.NamedRanges()["FOO"]; !ok {
		t.Fatal("FOO missing after create")
	}
}

func TestBugfixV18_UnaryMinusBindsTighterThanPow(t *testing.T) {
	wb := sheet.NewWorkbook("v18_unary_pow")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=-2^2", nil)
	sh.SetCellInput(0, 1, "=-(2^2)", nil)
	sh.SetCellInput(0, 2, "=2^-2", nil)
	sh.SetCellInput(0, 3, "=-2^-2", nil)
	sh.SetCellInput(0, 4, "=2^3^2", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 4 {
		t.Fatalf("-2^2 want 4, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != -4 {
		t.Fatalf("-(2^2) want -4, got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 0.25 {
		t.Fatalf("2^-2 want 0.25, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 0.25 {
		t.Fatalf("-2^-2 want 0.25, got %v", sh.GetCellValue(0, 3))
	}
	if v, ok := sh.GetCellValue(0, 4).(float64); !ok || v != 512 {
		t.Fatalf("2^3^2 want 512, got %v", sh.GetCellValue(0, 4))
	}
}

// --- from bugfix_v19_regression_test.go ---

func TestBugfixV19_ODSManualRecalcRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("v19_ods_manual")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "=A1*2", nil)
	sh.SetRecalcMode("MANUAL")
	wb.RecalculateAll()
	p := filepath.Join(t.TempDir(), "manual.ods")
	if err := wb.ExportODS(p); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	loaded, err := sheet.ImportODSWorkbook(p)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	lsh := loaded.Sheets[0]
	if lsh.RecalcMode() != "MANUAL" {
		t.Fatalf("ODS RecalcMode want MANUAL, got %q", lsh.RecalcMode())
	}
	if v, ok := lsh.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("imported B1 want 20, got %v", lsh.GetCellValue(1, 0))
	}
	lsh.SetCellInput(0, 0, "7", nil)
	if v, ok := lsh.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("MANUAL edit of A1 should leave B1 at 20, got %v", lsh.GetCellValue(1, 0))
	}
}

func TestBugfixV19_XLSXNamedFormulaProtectsStringColon(t *testing.T) {
	wb := sheet.NewWorkbook("v19_xlsx_named_colon")
	sh := wb.Sheets[0]
	wb.SetNamedRange("NOTE", formula.NamedExpr{Expr: `=CONCATENATE("A1:B2","!")`})
	sh.SetCellInput(0, 0, "=NOTE", nil)
	wb.RecalculateAll()
	p := filepath.Join(t.TempDir(), "named.xlsx")
	if err := wb.ExportXLSX(p); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}
	loaded, err := sheet.ImportXLSXWorkbook(p)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	nv, ok := loaded.GetNamedRange("NOTE")
	if !ok {
		t.Fatal("NOTE missing after XLSX roundtrip")
	}
	ne, ok := nv.(formula.NamedExpr)
	if !ok {
		t.Fatalf("NOTE want NamedExpr, got %T %#v", nv, nv)
	}
	if strings.Contains(ne.Expr, "A1..B2") || !strings.Contains(ne.Expr, `"A1:B2"`) {
		t.Fatalf("named formula must keep string colon, got %q", ne.Expr)
	}
	loaded.RecalculateAll()
	if v, ok := loaded.Sheets[0].GetCellValue(0, 0).(string); !ok || v != "A1:B2!" {
		t.Fatalf("A1 want A1:B2!, got %v", loaded.Sheets[0].GetCellValue(0, 0))
	}
}

func TestBugfixV19_ODSHyphenSheetFormulaRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("v19_ods_hyphen")
	src := wb.Sheets[0]
	src.SetName("Q1-2024")
	src.SetCellInput(0, 0, "9", nil)
	src.SetCellInput(0, 1, "2", nil)
	dst := wb.AddSheet("Summary")
	dst.SetCellInput(0, 0, "='Q1-2024'!A1", nil)
	dst.SetCellInput(0, 1, "=SUM('Q1-2024'!A1:A2)", nil)
	wb.RecalculateAll()
	p := filepath.Join(t.TempDir(), "hyphen.ods")
	if err := wb.ExportODS(p); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	loaded, err := sheet.ImportODSWorkbook(p)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	loaded.RecalculateAll()
	var sum *sheet.Sheet
	for _, s := range loaded.Sheets {
		if s.Name() == "Summary" {
			sum = s
			break
		}
	}
	if sum == nil {
		t.Fatal("Summary sheet missing after ODS import")
	}
	a1 := sum.GetCell(0, 0)
	if a1 == nil || !strings.Contains(a1.RawInput, "'Q1-2024'") {
		t.Fatalf("A1 formula want quoted sheet, got %+v", a1)
	}
	if v, ok := sum.GetCellValue(0, 0).(float64); !ok || v != 9 {
		t.Fatalf("A1 want 9, got %v", sum.GetCellValue(0, 0))
	}
	a2 := sum.GetCell(0, 1)
	if a2 == nil || !strings.Contains(a2.RawInput, "'Q1-2024'") {
		t.Fatalf("A2 formula want quoted sheet, got %+v", a2)
	}
	if v, ok := sum.GetCellValue(0, 1).(float64); !ok || v != 11 {
		t.Fatalf("A2 want 11, got %v", sum.GetCellValue(0, 1))
	}
}

// --- from bugfix_v2_regression_test.go ---

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
	sh.SetCellInput(0, 0, "'Hello", nil)                 // TypeLabel
	sh.SetCellInput(1, 0, "123.45", nil)                 // TypeNumber
	sh.SetCellInput(2, 0, "TRUE", nil)                   // TypeBoolean
	sh.SetCellInput(3, 0, "=\"Result: \" & \"OK\"", nil) // TypeFormula (returns string)
	sh.SetCellInput(4, 0, "=1/0", nil)                   // TypeFormula (returns ERR)
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

// --- from bugfix_v3_regression_test.go ---

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

// --- from bugfix_v4_regression_test.go ---

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

// --- from bugfix_v5_regression_test.go ---

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

// --- from bugfix_v6_regression_test.go ---

// TestBugfixV6_CellSourcedLeakage tests that cellSourced is not leaked into cell.Value or formatting
func TestBugfixV6_CellSourcedLeakage(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "10", nil)                    // A1 = 10
	s.SetCellInput(1, 0, "=IF(1=1, A1)", nil)          // B1
	s.SetCellInput(2, 0, "=CHOOSE(1, A1)", nil)        // C1
	s.SetCellInput(3, 0, "=SWITCH(1, 1, A1, 99)", nil) // D1
	s.SetCellInput(4, 0, "=IFS(1=1, A1)", nil)         // E1
	s.Recalculate()

	for col, colName := range []string{"B", "C", "D", "E"} {
		c := s.GetCell(col+1, 0)
		if c == nil {
			t.Fatalf("cell %s1 is nil", colName)
		}
		if v, ok := c.Value.(float64); !ok || v != 10 {
			t.Fatalf("cell %s1 value want 10 (float64), got %#v", colName, c.Value)
		}
		formatted := c.FormattedValue(s.GlobalFormat())
		if formatted != "10" {
			t.Fatalf("cell %s1 formatted want \"10\", got %q", colName, formatted)
		}
	}
}

// TestBugfixV6_EmptyCellComparison tests empty cell comparison semantics vs numbers and text
func TestBugfixV6_EmptyCellComparison(t *testing.T) {
	s := sheet.NewSheet()
	// A1 is empty (nil)
	s.SetCellInput(1, 0, "=A1 < 5", nil)         // B1: 0 < 5 is true
	s.SetCellInput(1, 1, "=A1 > 5", nil)         // B2: 0 > 5 is false
	s.SetCellInput(1, 2, "=A1 = 0", nil)         // B3: empty == 0 is true
	s.SetCellInput(1, 3, "=A1 < -5", nil)        // B4: 0 < -5 is false
	s.SetCellInput(1, 4, "=A1 > -5", nil)        // B5: 0 > -5 is true
	s.SetCellInput(1, 5, "=A1 < \"hello\"", nil) // B6: number < text is true
	s.SetCellInput(1, 6, "=A1 > \"hello\"", nil) // B7: false
	s.SetCellInput(1, 7, "=\"\" = FALSE", nil)   // B8: "" = FALSE is false in Excel/Lotus
	s.SetCellInput(1, 8, "=A1 = FALSE", nil)     // B9: blank = FALSE is true
	s.SetCellInput(1, 9, "=\"\" = 0", nil)       // B10: "" = 0 is false
	s.Recalculate()

	testCases := []struct {
		row  int
		desc string
		want bool
	}{
		{0, "=A1 < 5 (blank < 5)", true},
		{1, "=A1 > 5 (blank > 5)", false},
		{2, "=A1 = 0 (blank = 0)", true},
		{3, "=A1 < -5 (blank < -5)", false},
		{4, "=A1 > -5 (blank > -5)", true},
		{5, "=A1 < \"hello\" (blank < text)", true},
		{6, "=A1 > \"hello\" (blank > text)", false},
		{7, "=\"\" = FALSE", false},
		{8, "=A1 = FALSE (blank = FALSE)", true},
		{9, "=\"\" = 0", false},
	}

	for _, tc := range testCases {
		c := s.GetCell(1, tc.row)
		if c == nil {
			t.Fatalf("row %d (%s) cell is nil", tc.row+1, tc.desc)
		}
		if b, ok := c.Value.(bool); !ok || b != tc.want {
			t.Fatalf("%s want %v, got %#v", tc.desc, tc.want, c.Value)
		}
	}
}

// TestBugfixV6_IsTruthyAndConcatWithCellSourced tests isTruthy and & operator on cell references
func TestBugfixV6_IsTruthyAndConcatWithCellSourced(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "hello", nil)                    // A1 = "hello"
	s.SetCellInput(1, 0, "=IF(A1, \"yes\", \"no\")", nil) // B1: non-empty string cell is truthy
	s.SetCellInput(0, 1, "123", nil)                      // A2 = 123
	s.SetCellInput(1, 1, "=A2 & \"xyz\"", nil)            // B2: "123xyz"
	s.SetCellInput(1, 2, "=\"abc\" & A2", nil)            // B3: "abc123"
	s.Recalculate()

	if v, ok := s.GetCellValue(1, 0).(string); !ok || v != "yes" {
		t.Fatalf("B1 = IF(A1, \"yes\", \"no\") want \"yes\", got %#v", s.GetCellValue(1, 0))
	}
	if v, ok := s.GetCellValue(1, 1).(string); !ok || v != "123xyz" {
		t.Fatalf("B2 = A2 & \"xyz\" want \"123xyz\", got %#v", s.GetCellValue(1, 1))
	}
	if v, ok := s.GetCellValue(1, 2).(string); !ok || v != "abc123" {
		t.Fatalf("B3 = \"abc\" & A2 want \"abc123\", got %#v", s.GetCellValue(1, 2))
	}
}

// TestBugfixV6_MultiSheetNamedRangeShift tests that row/col shifts on Sheet1 do not corrupt Sheet2 named ranges
func TestBugfixV6_MultiSheetNamedRangeShift(t *testing.T) {
	wb := sheet.NewWorkbook("multisheet_names")
	sh1 := wb.Sheets[0] // Sheet1
	sh2 := wb.AddSheet("Sheet2")

	// Sheet1 named range
	sh1Ref, err := coord.ParseRangeRef("Sheet1!A1:A5")
	if err != nil {
		t.Fatal(err)
	}
	wb.SetNamedRange("SALES_SH1", sh1Ref)

	// Sheet2 named range in workbook
	sh2Ref, err := coord.ParseRangeRef("Sheet2!A1:A5")
	if err != nil {
		t.Fatal(err)
	}
	wb.SetNamedRange("SALES_SH2", sh2Ref)

	// Sheet2 local named range
	localRef, err := coord.ParseRangeRef("A1:A5")
	if err != nil {
		t.Fatal(err)
	}
	sh2.SetNamedRange("LOCAL_SH2", localRef)

	// Insert row on Sheet1 at row 2
	sh1.InsertRow(2, 2)

	// SALES_SH1 should have expanded to A1:A7 on Sheet1
	v1, ok := wb.GetNamedRange("SALES_SH1")
	if !ok {
		t.Fatal("SALES_SH1 missing")
	}
	rr1, ok := v1.(coord.RangeRef)
	if !ok || rr1.MinRow() != 0 || rr1.MaxRow() != 6 {
		t.Fatalf("SALES_SH1 after insert on Sheet1 want A1:A7, got %#v", v1)
	}

	// SALES_SH2 on Sheet2 must NOT have changed!
	v2, ok := wb.GetNamedRange("SALES_SH2")
	if !ok {
		t.Fatal("SALES_SH2 missing")
	}
	rr2, ok := v2.(coord.RangeRef)
	if !ok || rr2.MinRow() != 0 || rr2.MaxRow() != 4 {
		t.Fatalf("SALES_SH2 corrupted by Sheet1 insert! Got %#v, want A1:A5", v2)
	}

	// LOCAL_SH2 on Sheet2 must NOT have changed!
	v3, ok := sh2.GetNamedRange("LOCAL_SH2")
	if !ok {
		t.Fatal("LOCAL_SH2 missing")
	}
	rr3, ok := v3.(coord.RangeRef)
	if !ok || rr3.MinRow() != 0 || rr3.MaxRow() != 4 {
		t.Fatalf("LOCAL_SH2 corrupted by Sheet1 insert! Got %#v, want A1:A5", v3)
	}

	// Now delete row 0 on Sheet1
	sh1.DeleteRow(0, 1)

	// SALES_SH2 and LOCAL_SH2 on Sheet2 must STILL be A1:A5!
	v2, _ = wb.GetNamedRange("SALES_SH2")
	rr2 = v2.(coord.RangeRef)
	if rr2.MinRow() != 0 || rr2.MaxRow() != 4 {
		t.Fatalf("SALES_SH2 corrupted by Sheet1 delete! Got %#v, want A1:A5", v2)
	}

	v3, _ = sh2.GetNamedRange("LOCAL_SH2")
	rr3 = v3.(coord.RangeRef)
	if rr3.MinRow() != 0 || rr3.MaxRow() != 4 {
		t.Fatalf("LOCAL_SH2 corrupted by Sheet1 delete! Got %#v, want A1:A5", v3)
	}
}

// TestBugfixV6_MoveRangeRetargetingFracture tests that moving only part of a range does NOT stretch range references
func TestBugfixV6_MoveRangeRetargetingFracture(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(2, 0, "=SUM(A1:A10)", nil) // C1

	// Move only A1 to Z100
	fromR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	s.RetargetFormulasAfterMove(fromR, 25, 99)

	// C1 must NOT be stretched to Z100:A10!
	raw := s.GetCell(2, 0).RawInput
	if raw != "=SUM(A1:A10)" {
		t.Fatalf("Formula was fractured by partial range move! Got %q, want =SUM(A1:A10)", raw)
	}

	// Move ENTIRE range A1:A10 to B1:B10
	entireR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 9}}
	s.RetargetFormulasAfterMove(entireR, 1, 0)

	raw = s.GetCell(2, 0).RawInput
	if raw != "=SUM(B1:B10)" {
		t.Fatalf("Entire range move did not retarget! Got %q, want =SUM(B1:B10)", raw)
	}
}

// TestBugfixV6_ReverseRangeShrinkOnDelete tests that reverse ranges (A5:A1) do not become #REF! when deleting an end
func TestBugfixV6_ReverseRangeShrinkOnDelete(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(2, 9, "=SUM(A5:A1)", nil) // C10
	s.DeleteRow(0, 1)                        // delete row 0 (A1)

	cellC9 := s.GetCell(2, 8)
	if cellC9 == nil {
		t.Fatal("C9 is nil after deleting row 0")
	}
	raw := cellC9.RawInput
	if strings.Contains(raw, "#REF!") {
		t.Fatalf("Reverse range A5:A1 became #REF! after deleting row 1: %q", raw)
	}
	if raw != "=SUM(A4:A1)" && raw != "=SUM(A1:A4)" {
		t.Fatalf("Reverse range after deleting row 1 want A4:A1, got %q", raw)
	}
}

// TestBugfixV6_CrossSheetCutPasteDoesNotCorruptLocalRefs tests that cutting on Sheet1 and pasting on Sheet2 doesn't shift Sheet2 formulas
func TestBugfixV6_CrossSheetCutPasteDoesNotCorruptLocalRefs(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_cross_cut")
	sh1 := wb.Sheets[0] // Sheet1
	sh2 := wb.AddSheet("Sheet2")

	app := tui.NewApp(simScreen, sh1, "test.hwk")
	app.RunOnceForTest()

	// Sheet1 has data at A1:B1
	sh1.SetCellInput(0, 0, "10", nil)
	sh1.SetCellInput(1, 0, "=A1*2", nil)

	// Sheet2 has formula at C1 pointing to A1
	sh2.SetCellInput(2, 0, "=A1", nil)
	wb.RecalculateAll()

	// Switch to Sheet1, select A1:B1 and cut
	app.SwitchSheetForTest(0)
	cutRange := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	app.SelectRangeForTest(&cutRange)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))

	// Switch to Sheet2, move cursor to D1 and paste
	app.SwitchSheetForTest(1)
	app.SetCursorForTest(3, 0) // D1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	// Verify that Sheet2 C1 was NOT shifted by Sheet1's move
	sh2C1 := sh2.GetCell(2, 0)
	if sh2C1 == nil || sh2C1.RawInput != "=A1" {
		t.Fatalf("Sheet2 C1 formula was corrupted by cross-sheet paste! Got %q, want =A1", sh2C1.RawInput)
	}
}

// TestBugfixV6_DeterministicODSNamedExpressions verifies that ODS named expressions are written in sorted deterministic order
func TestBugfixV6_DeterministicODSNamedExpressions(t *testing.T) {
	wb := sheet.NewWorkbook("deterministic_ods")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)

	// Add multiple named ranges
	names := []string{"ZEBRA", "ALPHA", "MANGO", "BETA", "CHARLIE", "DELTA"}
	for i, name := range names {
		wb.SetNamedRange(name, coord.CellRef{Col: i, Row: 0})
	}

	tmpDir, err := os.MkdirTemp("", "ods_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "out1.ods")
	file2 := filepath.Join(tmpDir, "out2.ods")

	if err := wb.ExportODS(file1); err != nil {
		t.Fatal(err)
	}
	if err := wb.ExportODS(file2); err != nil {
		t.Fatal(err)
	}

	readContentXML := func(path string) string {
		r, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		for _, f := range r.File {
			if f.Name == "content.xml" {
				rc, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				defer rc.Close()
				var buf bytes.Buffer
				io.Copy(&buf, rc)
				return buf.String()
			}
		}
		t.Fatal("content.xml not found")
		return ""
	}

	c1 := readContentXML(file1)
	c2 := readContentXML(file2)

	if c1 != c2 {
		t.Fatal("ODS content.xml is non-deterministic between exports!")
	}

	// Verify order in content.xml has ALPHA before ZEBRA
	alphaIdx := strings.Index(c1, `name="ALPHA"`)
	zebraIdx := strings.Index(c1, `name="ZEBRA"`)
	if alphaIdx == -1 || zebraIdx == -1 {
		t.Fatal("ALPHA or ZEBRA missing in content.xml")
	}
	if alphaIdx > zebraIdx {
		t.Fatalf("Named expressions in ODS are not sorted: ALPHA is at %d, ZEBRA is at %d", alphaIdx, zebraIdx)
	}
}

// --- from bugfix_v7_regression_test.go ---

func newSimApp(t *testing.T, sh *sheet.Sheet, name string) *tui.App {
	t.Helper()
	sim := tcell.NewSimulationScreen("")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	sim.SetSize(100, 30)
	app := tui.NewApp(sim, sh, name)
	app.RunOnceForTest()
	return app
}

func mustParseRange(t *testing.T, s string) coord.RangeRef {
	t.Helper()
	r, err := coord.ParseRangeRef(s)
	if err != nil {
		t.Fatalf("ParseRangeRef(%q): %v", s, err)
	}
	return r
}

func namedAsRange(t *testing.T, v any) coord.RangeRef {
	t.Helper()
	switch x := v.(type) {
	case coord.RangeRef:
		return x
	case coord.CellRef:
		return coord.RangeRef{Sheet: x.Sheet, Start: x, End: x}
	default:
		t.Fatalf("named value type %T, want range/cell", v)
		return coord.RangeRef{}
	}
}

func assertRangeCoords(t *testing.T, got coord.RangeRef, minCol, minRow, maxCol, maxRow int, desc string) {
	t.Helper()
	if got.MinCol() != minCol || got.MinRow() != minRow || got.MaxCol() != maxCol || got.MaxRow() != maxRow {
		t.Fatalf("%s: want cols %d..%d rows %d..%d, got %s (%#v)",
			desc, minCol, maxCol, minRow, maxRow, got.String(), got)
	}
}

func cellRaw(t *testing.T, sh *sheet.Sheet, col, row int) string {
	t.Helper()
	c := sh.GetCell(col, row)
	if c == nil {
		t.Fatalf("cell %s%d is nil", coord.ColToLetter(col), row+1)
	}
	return c.RawInput
}

func TestBugfixV7_XLSXSheetLocalNamedRangesRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("local_names")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")
	sh1.SetNamedRange("LOCAL", mustParseRange(t, "A1:A3"))
	sh2.SetNamedRange("LOCAL", mustParseRange(t, "B1:B3"))
	wb.SetNamedRange("GLOBAL", mustParseRange(t, "Sheet1!C1:C3"))

	p := filepath.Join(t.TempDir(), "local.xlsx")
	if err := wb.ExportXLSX(p); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}
	loaded, err := sheet.ImportXLSXWorkbook(p)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	if len(loaded.Sheets) < 2 {
		t.Fatalf("want 2 sheets, got %d", len(loaded.Sheets))
	}
	l1, l2 := loaded.Sheets[0], loaded.Sheets[1]

	if _, ok := loaded.GetNamedRange("LOCAL"); ok {
		t.Fatal("LOCAL leaked to workbook-level names")
	}
	if _, ok := l1.NamedRanges()["LOCAL"]; !ok {
		t.Fatal("Sheet1 missing sheet-local LOCAL")
	}
	if _, ok := l2.NamedRanges()["LOCAL"]; !ok {
		t.Fatal("Sheet2 missing sheet-local LOCAL")
	}

	v1, ok := l1.GetNamedRange("LOCAL")
	if !ok {
		t.Fatal("Sheet1 GetNamedRange LOCAL missing")
	}
	assertRangeCoords(t, namedAsRange(t, v1), 0, 0, 0, 2, "Sheet1 LOCAL")

	v2, ok := l2.GetNamedRange("LOCAL")
	if !ok {
		t.Fatal("Sheet2 GetNamedRange LOCAL missing")
	}
	assertRangeCoords(t, namedAsRange(t, v2), 1, 0, 1, 2, "Sheet2 LOCAL")

	g, ok := loaded.GetNamedRange("GLOBAL")
	if !ok {
		t.Fatal("GLOBAL missing after XLSX reload")
	}
	gr := namedAsRange(t, g)
	assertRangeCoords(t, gr, 2, 0, 2, 2, "GLOBAL")
	if gr.Sheet != "" && !strings.EqualFold(gr.Sheet, "Sheet1") {
		t.Fatalf("GLOBAL sheet %q, want Sheet1", gr.Sheet)
	}
}

func TestBugfixV7_HasBaseRelativeNameSurvivesRowInsertAndHWK(t *testing.T) {
	wb := sheet.NewWorkbook("rel_insert")
	sh := wb.Sheets[0]
	wb.SetNamedRange("RelAbove", formula.NamedExpr{
		Expr: "=A1", HasBase: true, BaseCol: 1, BaseRow: 1,
	})
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "20", nil)
	sh.SetCellInput(1, 2, "=RelAbove", nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(1, 2).(float64); !ok || v != 20 {
		t.Fatalf("before insert RelAbove at B3 want 20, got %v", sh.GetCellValue(1, 2))
	}

	sh.InsertRow(0, 1)
	wb.RecalculateAll()
	got, ok := wb.GetNamedRange("RELABOVE")
	if !ok {
		t.Fatal("RELABOVE missing after insert")
	}
	ne, ok := got.(formula.NamedExpr)
	if !ok {
		t.Fatalf("RELABOVE type %T", got)
	}
	if !ne.HasBase || ne.BaseCol != 1 || ne.BaseRow != 2 {
		t.Fatalf("after insert HasBase/base want B3, got %+v", ne)
	}
	if v, ok := sh.GetCellValue(1, 3).(float64); !ok || v != 20 {
		t.Fatalf("after insert RelAbove at B4 want 20, got %v", sh.GetCellValue(1, 3))
	}

	p := filepath.Join(t.TempDir(), "rel_insert.hwk")
	if err := wb.SaveJSON(p); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}
	loaded, err := sheet.LoadWorkbookJSON(p)
	if err != nil {
		t.Fatalf("LoadWorkbookJSON: %v", err)
	}
	got, ok = loaded.GetNamedRange("RELABOVE")
	if !ok {
		t.Fatal("RELABOVE missing after HWK reload")
	}
	ne, ok = got.(formula.NamedExpr)
	if !ok {
		t.Fatalf("after reload type %T", got)
	}
	if !ne.HasBase || ne.BaseCol != 1 || ne.BaseRow != 2 {
		t.Fatalf("HasBase lost after reload: %+v", ne)
	}
	if v, ok := loaded.Sheets[0].GetCellValue(1, 3).(float64); !ok || v != 20 {
		t.Fatalf("after reload RelAbove at B4 want 20, got %v", loaded.Sheets[0].GetCellValue(1, 3))
	}
}

func TestBugfixV7_InsertDeleteColUndoRestoresFormulaNameWidth(t *testing.T) {
	wb := sheet.NewWorkbook("col_undo")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "20", nil)
	sh.SetCellInput(2, 0, "=A1+B1", nil)
	sh.SetColWidth(1, 20)
	wb.SetNamedRange("SALES", mustParseRange(t, "A1:C1"))
	sh.SetNamedRange("LOCALSALES", mustParseRange(t, "A1:C1"))
	wb.RecalculateAll()

	app := newSimApp(t, sh, "col_undo.hwk")
	app.ExecuteActionHandlerForTest("doInsertCol", map[string]string{"range": "B1"})

	if sh.GetCell(1, 0) != nil && sh.GetCell(1, 0).RawInput != "" {
		t.Fatalf("after insert B1 should be empty, got %q", sh.GetCell(1, 0).RawInput)
	}
	if raw := cellRaw(t, sh, 3, 0); raw != "=A1+C1" {
		t.Fatalf("after insert D1 formula want =A1+C1, got %q", raw)
	}
	if sh.GetColWidth(2) != 20 {
		t.Fatalf("after insert col C width want 20, got %d", sh.GetColWidth(2))
	}
	v, ok := wb.GetNamedRange("SALES")
	if !ok {
		t.Fatal("SALES missing after insert")
	}
	assertRangeCoords(t, namedAsRange(t, v), 0, 0, 3, 0, "SALES after insert")
	lv, ok := sh.GetNamedRange("LOCALSALES")
	if !ok {
		t.Fatal("LOCALSALES missing after insert")
	}
	assertRangeCoords(t, namedAsRange(t, lv), 0, 0, 3, 0, "LOCALSALES after insert")

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	sh = wb.GetActiveSheet()
	if raw := cellRaw(t, sh, 2, 0); raw != "=A1+B1" {
		t.Fatalf("undo D1/C1 formula want =A1+B1, got %q", raw)
	}
	if sh.GetColWidth(1) != 20 {
		t.Fatalf("undo col B width want 20, got %d", sh.GetColWidth(1))
	}
	v, _ = wb.GetNamedRange("SALES")
	assertRangeCoords(t, namedAsRange(t, v), 0, 0, 2, 0, "SALES after undo")
	lv, _ = sh.GetNamedRange("LOCALSALES")
	assertRangeCoords(t, namedAsRange(t, lv), 0, 0, 2, 0, "LOCALSALES after undo")

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModCtrl))
	sh = wb.GetActiveSheet()
	if raw := cellRaw(t, sh, 3, 0); raw != "=A1+C1" {
		t.Fatalf("redo D1 formula want =A1+C1, got %q", raw)
	}

	app.ExecuteActionHandlerForTest("doDeleteCol", map[string]string{"range": "B1"})
	if raw := cellRaw(t, sh, 2, 0); raw != "=A1+B1" {
		t.Fatalf("after delete C1 formula want =A1+B1, got %q", raw)
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	sh = wb.GetActiveSheet()
	if raw := cellRaw(t, sh, 3, 0); raw != "=A1+C1" {
		t.Fatalf("delete-col undo want =A1+C1 at D1, got %q", raw)
	}
}

func TestBugfixV7_PasteValuesAndPasteLinkUndo(t *testing.T) {
	wb := sheet.NewWorkbook("paste_undo")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "=10+10", nil)
	sh.SetCellInput(3, 0, "keep-values", nil)
	sh.SetCellInput(4, 0, "keep-link", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh, "paste_undo.hwk")
	rng := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	app.SelectRangeForTest(&rng)
	app.SetCursorForTest(0, 0)
	app.ExecuteActionHandlerForTest("doPaletteCopy", nil)

	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0)
	app.ExecuteActionHandlerForTest("doPalettePasteValues", nil)
	c := sh.GetCell(3, 0)
	if c == nil {
		t.Fatal("D1 nil after paste values")
	}
	if c.Type == cell.TypeFormula {
		t.Fatalf("paste values left a formula: %q", c.RawInput)
	}
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 20 {
		t.Fatalf("paste values D1 want 20, got %v", sh.GetCellValue(3, 0))
	}

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if sh.GetCellValue(3, 0) != "keep-values" {
		t.Fatalf("paste values undo want keep-values, got %v (raw=%q)", sh.GetCellValue(3, 0), cellRaw(t, sh, 3, 0))
	}

	app.SelectRangeForTest(nil)
	app.SetCursorForTest(4, 0)
	app.ExecuteActionHandlerForTest("doPalettePasteLink", nil)
	if raw := cellRaw(t, sh, 4, 0); raw != "=A1" {
		t.Fatalf("paste link E1 want =A1, got %q", raw)
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if sh.GetCellValue(4, 0) != "keep-link" {
		t.Fatalf("paste link undo want keep-link, got %v (raw=%q)", sh.GetCellValue(4, 0), cellRaw(t, sh, 4, 0))
	}
}

func TestBugfixV7_RenameSheetINDIRECTStaysLiteral(t *testing.T) {
	wb := sheet.NewWorkbook("rename_indirect")
	sh1 := wb.Sheets[0]
	data := wb.AddSheet("Data")
	data.SetCellInput(0, 0, "42", nil)
	sh1.SetCellInput(0, 0, "=Data!A1", nil)
	sh1.SetCellInput(0, 1, `=INDIRECT("Data!A1")`, nil)
	wb.RecalculateAll()
	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 42 {
		t.Fatalf("direct ref want 42, got %v", sh1.GetCellValue(0, 0))
	}
	if v, ok := sh1.GetCellValue(0, 1).(float64); !ok || v != 42 {
		t.Fatalf("INDIRECT before rename want 42, got %v", sh1.GetCellValue(0, 1))
	}

	if err := wb.RenameSheet("Data", "Inventory"); err != nil {
		t.Fatalf("RenameSheet: %v", err)
	}
	direct := cellRaw(t, sh1, 0, 0)
	if !strings.Contains(strings.ToUpper(direct), "INVENTORY") {
		t.Fatalf("direct formula should follow rename, got %q", direct)
	}
	if strings.Contains(strings.ToUpper(direct), "DATA!") {
		t.Fatalf("direct formula still points at Data: %q", direct)
	}
	indirect := cellRaw(t, sh1, 0, 1)
	if !strings.Contains(indirect, "Data!A1") {
		t.Fatalf("INDIRECT literal should stay Data!A1, got %q", indirect)
	}
	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 42 {
		t.Fatalf("direct ref after rename want 42, got %v", sh1.GetCellValue(0, 0))
	}
	if _, ok := sh1.GetCellValue(0, 1).(cell.LotusError); !ok {
		t.Fatalf("INDIRECT after rename want #REF!/error, got %v (%T)", sh1.GetCellValue(0, 1), sh1.GetCellValue(0, 1))
	}
}

func TestBugfixV7_CopyKeepsAbsoluteCutFollowsMove(t *testing.T) {
	wb := sheet.NewWorkbook("abs_copy_cut")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "=$A$1", nil)
	wb.RecalculateAll()

	app := newSimApp(t, sh, "abs_copy_cut.hwk")
	copyR := coord.RangeRef{Start: coord.CellRef{Col: 1, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	app.SelectRangeForTest(&copyR)
	app.SetCursorForTest(1, 0)
	app.ExecuteActionHandlerForTest("doPaletteCopy", nil)
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0)
	app.ExecuteActionHandlerForTest("doPalettePaste", nil)
	if raw := cellRaw(t, sh, 3, 0); raw != "=$A$1" {
		t.Fatalf("copy of absolute ref should stay =$A$1, got %q", raw)
	}

	sh.SetCellInput(3, 0, "", nil)
	cutR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	app.SelectRangeForTest(&cutR)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(3, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))
	if raw := cellRaw(t, sh, 4, 0); raw != "=$D$1" {
		t.Fatalf("cut of absolute ref inside move should become =$D$1, got %q", raw)
	}
}

func TestBugfixV7_GraphSeriesFollowsRowAndColInsert(t *testing.T) {
	sh := sheet.NewSheet()
	rx := mustParseRange(t, "A1:A4")
	ra := mustParseRange(t, "B1:B4")
	sh.Graph().RangeX = &rx
	sh.Graph().Series["A"] = &ra

	sh.InsertRow(1, 1)
	if sh.Graph().RangeX == nil || sh.Graph().RangeX.String() != "A1:A5" {
		t.Fatalf("RangeX after row insert want A1:A5, got %v", sh.Graph().RangeX)
	}
	if sh.Graph().Series["A"] == nil || sh.Graph().Series["A"].String() != "B1:B5" {
		t.Fatalf("Series A after row insert want B1:B5, got %v", sh.Graph().Series["A"])
	}

	sh.InsertCol(1, 1)
	if sh.Graph().RangeX == nil || sh.Graph().RangeX.String() != "A1:A5" {
		t.Fatalf("RangeX after col insert should stay A1:A5, got %v", sh.Graph().RangeX)
	}
	if sh.Graph().Series["A"] == nil || sh.Graph().Series["A"].String() != "C1:C5" {
		t.Fatalf("Series A after col insert want C1:C5, got %v", sh.Graph().Series["A"])
	}

	sh.DeleteRow(0, 1)
	if sh.Graph().Series["A"] == nil || sh.Graph().Series["A"].String() != "C1:C4" {
		t.Fatalf("Series A after row delete want C1:C4, got %v", sh.Graph().Series["A"])
	}
}

func TestBugfixV7_ReplaceAllSheetsUndo(t *testing.T) {
	wb := sheet.NewWorkbook("replace_all")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")
	sh1.SetCellInput(0, 0, "foo", nil)
	sh2.SetCellInput(0, 0, "foo", nil)

	app := newSimApp(t, sh1, "replace_all.hwk")
	app.ExecuteActionHandlerForTest("doReplacePrompt", map[string]string{
		"find":    "foo",
		"replace": "bar",
		"scope":   "A",
	})
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))

	if sh1.GetCellValue(0, 0) != "bar" {
		t.Fatalf("Sheet1 A1 after replace-all want bar, got %v (raw=%q)", sh1.GetCellValue(0, 0), cellRaw(t, sh1, 0, 0))
	}
	if sh2.GetCellValue(0, 0) != "bar" {
		t.Fatalf("Sheet2 A1 after replace-all want bar, got %v (raw=%q)", sh2.GetCellValue(0, 0), cellRaw(t, sh2, 0, 0))
	}

	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	sh1 = wb.GetSheet("Sheet1")
	sh2 = wb.GetSheet("Sheet2")
	if sh1.GetCellValue(0, 0) != "foo" {
		t.Fatalf("Sheet1 undo want foo, got %v (raw=%q)", sh1.GetCellValue(0, 0), cellRaw(t, sh1, 0, 0))
	}
	if sh2.GetCellValue(0, 0) != "foo" {
		t.Fatalf("Sheet2 undo want foo, got %v (raw=%q)", sh2.GetCellValue(0, 0), cellRaw(t, sh2, 0, 0))
	}
}

func TestBugfixV7_QuotedSheetNamesNamedRangeRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("quoted_names")
	sh := wb.Sheets[0]
	sh.SetName("My Sheet")
	ob := wb.AddSheet("O'Brien")
	ob.SetCellInput(0, 0, "7", nil)
	wb.SetNamedRange("Sales", coord.RangeRef{
		Sheet: "My Sheet",
		Start: coord.CellRef{Sheet: "My Sheet", Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 4},
	})
	wb.SetNamedRange("ObrienCell", coord.CellRef{Sheet: "O'Brien", Col: 0, Row: 0})

	dir := t.TempDir()
	odsPath := filepath.Join(dir, "quoted.ods")
	xlsxPath := filepath.Join(dir, "quoted.xlsx")
	if err := wb.ExportODS(odsPath); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	if err := wb.ExportXLSX(xlsxPath); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}

	check := func(t *testing.T, loaded *sheet.Workbook, via string) {
		t.Helper()
		if loaded.GetSheet("My Sheet") == nil {
			t.Fatalf("%s missing sheet My Sheet, have %v", via, loaded.SheetNames())
		}
		if loaded.GetSheet("O'Brien") == nil {
			t.Fatalf("%s missing sheet O'Brien, have %v", via, loaded.SheetNames())
		}
		sales, ok := loaded.GetNamedRange("SALES")
		if !ok {
			t.Fatalf("%s missing SALES", via)
		}
		sr := namedAsRange(t, sales)
		assertRangeCoords(t, sr, 0, 0, 0, 4, via+" SALES")
		if !strings.EqualFold(sr.Sheet, "My Sheet") && !strings.EqualFold(sr.Start.Sheet, "My Sheet") {
			t.Fatalf("%s SALES sheet want My Sheet, got %#v", via, sr)
		}
		obn, ok := loaded.GetNamedRange("OBRIENCELL")
		if !ok {
			t.Fatalf("%s missing OBRIENCELL", via)
		}
		or := namedAsRange(t, obn)
		if or.MinCol() != 0 || or.MinRow() != 0 {
			t.Fatalf("%s OBRIENCELL want A1, got %#v", via, or)
		}
		sheetName := or.Sheet
		if sheetName == "" {
			sheetName = or.Start.Sheet
		}
		if sheetName != "O'Brien" {
			t.Fatalf("%s OBRIENCELL sheet want O'Brien, got %q", via, sheetName)
		}
	}

	odsWB, err := sheet.ImportODSWorkbook(odsPath)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	check(t, odsWB, "ODS")

	xlsxWB, err := sheet.ImportXLSXWorkbook(xlsxPath)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	check(t, xlsxWB, "XLSX")
}

func TestBugfixV7_ManualRecalcPersistsAcrossHWKOpen(t *testing.T) {
	wb := sheet.NewWorkbook("manual")
	sh := wb.Sheets[0]
	sh.SetRecalcMode("MANUAL")
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "=A1", nil)
	sh.Recalculate()
	if v, ok := sh.GetCellValue(1, 0).(float64); !ok || v != 10 {
		t.Fatalf("seed B1 want 10, got %v", sh.GetCellValue(1, 0))
	}

	p := filepath.Join(t.TempDir(), "manual.hwk")
	if err := wb.SaveJSON(p); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}
	loaded, err := sheet.LoadWorkbookJSON(p)
	if err != nil {
		t.Fatalf("LoadWorkbookJSON: %v", err)
	}
	lsh := loaded.Sheets[0]
	if lsh.RecalcMode() != "MANUAL" {
		t.Fatalf("RecalcMode after open want MANUAL, got %q", lsh.RecalcMode())
	}
	lsh.SetCellInput(0, 0, "99", nil)
	if v, ok := lsh.GetCellValue(1, 0).(float64); !ok || v != 10 {
		t.Fatalf("MANUAL edit of A1 should leave B1 at 10, got %v", lsh.GetCellValue(1, 0))
	}
	lsh.Recalculate()
	if v, ok := lsh.GetCellValue(1, 0).(float64); !ok || v != 99 {
		t.Fatalf("after Recalculate B1 want 99, got %v", lsh.GetCellValue(1, 0))
	}
}

// --- from bugfix_v8_regression_test.go ---

func TestBugfixV8_RenameSheetUpdatesNamedRanges(t *testing.T) {
	wb := sheet.NewWorkbook("rename_names")
	sh1 := wb.Sheets[0]
	data := wb.AddSheet("Data")
	data.SetCellInput(0, 0, "42", nil)
	data.SetCellInput(0, 1, "8", nil)
	wb.SetNamedRange("Sales", mustParseRange(t, "Data!A1:A2"))
	wb.SetNamedRange("Anchor", formula.NamedExpr{Expr: "=Data!A1"})
	data.SetNamedRange("LOCAL", mustParseRange(t, "A1:A2"))
	sh1.SetCellInput(0, 0, "=SUM(Sales)", nil)
	sh1.SetCellInput(0, 1, "=Anchor", nil)
	wb.RecalculateAll()
	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 50 {
		t.Fatalf("before rename SUM(Sales) want 50, got %v", sh1.GetCellValue(0, 0))
	}

	if err := wb.RenameSheet("Data", "Inventory"); err != nil {
		t.Fatalf("RenameSheet: %v", err)
	}

	sales, ok := wb.GetNamedRange("SALES")
	if !ok {
		t.Fatal("SALES missing after rename")
	}
	sr := namedAsRange(t, sales)
	sheetName := sr.Sheet
	if sheetName == "" {
		sheetName = sr.Start.Sheet
	}
	if !strings.EqualFold(sheetName, "Inventory") {
		t.Fatalf("SALES sheet want Inventory, got %#v", sr)
	}
	anchor, ok := wb.GetNamedRange("ANCHOR")
	if !ok {
		t.Fatal("ANCHOR missing after rename")
	}
	ne, ok := anchor.(formula.NamedExpr)
	if !ok {
		t.Fatalf("ANCHOR type %T", anchor)
	}
	if !strings.Contains(strings.ToUpper(ne.Expr), "INVENTORY") {
		t.Fatalf("ANCHOR expr should follow rename, got %q", ne.Expr)
	}
	if strings.Contains(strings.ToUpper(ne.Expr), "DATA!") {
		t.Fatalf("ANCHOR still points at Data: %q", ne.Expr)
	}

	inv := wb.GetSheet("Inventory")
	if inv == nil {
		t.Fatal("Inventory sheet missing")
	}
	if _, ok := inv.NamedRanges()["LOCAL"]; !ok {
		t.Fatal("sheet-local LOCAL did not stay on renamed sheet")
	}

	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 50 {
		t.Fatalf("after rename SUM(Sales) want 50, got %v", sh1.GetCellValue(0, 0))
	}
	if v, ok := sh1.GetCellValue(0, 1).(float64); !ok || v != 42 {
		t.Fatalf("after rename Anchor want 42, got %v", sh1.GetCellValue(0, 1))
	}
}

func TestBugfixV8_DeleteSheetDropsNamesPointingAtIt(t *testing.T) {
	wb := sheet.NewWorkbook("delete_names")
	sh1 := wb.Sheets[0]
	data := wb.AddSheet("Data")
	data.SetCellInput(0, 0, "42", nil)
	wb.SetNamedRange("Sales", mustParseRange(t, "Data!A1"))
	wb.SetNamedRange("Keep", mustParseRange(t, "Sheet1!A1"))
	wb.SetNamedRange("FromData", formula.NamedExpr{Expr: "=Data!A1"})
	sh1.SetCellInput(0, 0, "9", nil)
	idx := wb.GetSheetIndex(data)
	if err := wb.DeleteSheet(idx); err != nil {
		t.Fatalf("DeleteSheet: %v", err)
	}
	if _, ok := wb.GetNamedRange("SALES"); ok {
		t.Fatal("SALES should be removed with deleted sheet")
	}
	keep, ok := wb.GetNamedRange("KEEP")
	if !ok {
		t.Fatal("KEEP should survive delete of Data")
	}
	kr := namedAsRange(t, keep)
	assertRangeCoords(t, kr, 0, 0, 0, 0, "KEEP")
	from, ok := wb.GetNamedRange("FROMDATA")
	if !ok {
		t.Fatal("FROMDATA named formula should remain")
	}
	ne, ok := from.(formula.NamedExpr)
	if !ok {
		t.Fatalf("FROMDATA type %T", from)
	}
	if !strings.Contains(ne.Expr, "#REF!") {
		t.Fatalf("FROMDATA should become #REF!, got %q", ne.Expr)
	}
}

func TestBugfixV8_ODSSheetLocalNamedRangesRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("ods_local")
	sh1 := wb.Sheets[0]
	sh2 := wb.AddSheet("Sheet2")
	sh1.SetNamedRange("LOCAL", mustParseRange(t, "A1:A3"))
	sh2.SetNamedRange("LOCAL", mustParseRange(t, "B1:B3"))
	wb.SetNamedRange("GLOBAL", mustParseRange(t, "Sheet1!C1:C3"))

	p := filepath.Join(t.TempDir(), "local.ods")
	if err := wb.ExportODS(p); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	loaded, err := sheet.ImportODSWorkbook(p)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	if len(loaded.Sheets) < 2 {
		t.Fatalf("want 2 sheets, got %d", len(loaded.Sheets))
	}
	l1, l2 := loaded.Sheets[0], loaded.Sheets[1]
	if _, ok := loaded.GetNamedRange("LOCAL"); ok {
		t.Fatal("LOCAL leaked to workbook-level names")
	}
	if _, ok := l1.NamedRanges()["LOCAL"]; !ok {
		t.Fatal("Sheet1 missing sheet-local LOCAL")
	}
	if _, ok := l2.NamedRanges()["LOCAL"]; !ok {
		t.Fatal("Sheet2 missing sheet-local LOCAL")
	}
	v1, ok := l1.GetNamedRange("LOCAL")
	if !ok {
		t.Fatal("Sheet1 GetNamedRange LOCAL missing")
	}
	assertRangeCoords(t, namedAsRange(t, v1), 0, 0, 0, 2, "Sheet1 LOCAL")
	v2, ok := l2.GetNamedRange("LOCAL")
	if !ok {
		t.Fatal("Sheet2 GetNamedRange LOCAL missing")
	}
	assertRangeCoords(t, namedAsRange(t, v2), 1, 0, 1, 2, "Sheet2 LOCAL")
	g, ok := loaded.GetNamedRange("GLOBAL")
	if !ok {
		t.Fatal("GLOBAL missing after ODS reload")
	}
	assertRangeCoords(t, namedAsRange(t, g), 2, 0, 2, 2, "GLOBAL")
}

func TestBugfixV8_MoveRangeUpdatesGraphSeries(t *testing.T) {
	sh := sheet.NewSheet()
	rx := mustParseRange(t, "A1:A3")
	ra := mustParseRange(t, "B1:B3")
	sh.Graph().RangeX = &rx
	sh.Graph().Series["A"] = &ra

	fromR := mustParseRange(t, "A1:B3")
	toR := mustParseRange(t, "A10:B12")
	sh.MoveRange(fromR, toR)

	if sh.Graph().RangeX == nil || sh.Graph().RangeX.String() != "A10:A12" {
		t.Fatalf("RangeX after move want A10:A12, got %v", sh.Graph().RangeX)
	}
	if sh.Graph().Series["A"] == nil || sh.Graph().Series["A"].String() != "B10:B12" {
		t.Fatalf("Series A after move want B10:B12, got %v", sh.Graph().Series["A"])
	}

	// Partial move must not stretch the remaining graph range.
	sh2 := sheet.NewSheet()
	rx2 := mustParseRange(t, "A1:A10")
	sh2.Graph().RangeX = &rx2
	sh2.MoveRange(mustParseRange(t, "A1:A3"), mustParseRange(t, "C1:C3"))
	if sh2.Graph().RangeX == nil || sh2.Graph().RangeX.String() != "A1:A10" {
		t.Fatalf("partial move stretched RangeX, got %v", sh2.Graph().RangeX)
	}
}

func TestBugfixV8_CutPasteUpdatesGraphSeries(t *testing.T) {
	wb := sheet.NewWorkbook("cut_graph")
	sh := wb.Sheets[0]
	rx := mustParseRange(t, "A1:A2")
	ra := mustParseRange(t, "B1:B2")
	sh.Graph().RangeX = &rx
	sh.Graph().Series["A"] = &ra
	sh.SetCellInput(0, 0, "1", nil)
	sh.SetCellInput(0, 1, "2", nil)
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(1, 1, "20", nil)

	fromR := mustParseRange(t, "A1:B2")
	sh.MoveRange(fromR, mustParseRange(t, "D5:E6"))
	if sh.Graph().RangeX == nil || sh.Graph().RangeX.String() != "D5:D6" {
		t.Fatalf("cut/move RangeX want D5:D6, got %v", sh.Graph().RangeX)
	}
	if sh.Graph().Series["A"] == nil || sh.Graph().Series["A"].String() != "E5:E6" {
		t.Fatalf("cut/move Series A want E5:E6, got %v", sh.Graph().Series["A"])
	}
}

// --- from bugfix_v9_regression_test.go ---

// 1. シート削除（DeleteSheet）時に他シートの通常セル数式が安全に #REF! へ無効化されエラー評価されるか
func TestBugfixV9_DeleteSheetInvalidatesCellFormulasToRef(t *testing.T) {
	wb := sheet.NewWorkbook("delete_cell_refs")
	sh1 := wb.Sheets[0]
	data := wb.AddSheet("Data")

	data.SetCellInput(0, 0, "10", nil) // A1
	data.SetCellInput(0, 1, "20", nil) // A2
	data.SetCellInput(1, 0, "30", nil) // B1
	data.SetCellInput(1, 1, "40", nil) // B2

	sh1.SetCellInput(0, 0, "=Data!A1+5", nil)
	sh1.SetCellInput(0, 1, "=SUM(Data!A1..B2)", nil)
	sh1.SetCellInput(0, 2, "=Data!$A$1*2", nil)
	wb.RecalculateAll()

	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 15 {
		t.Fatalf("before delete Sheet1!A1 want 15, got %v", sh1.GetCellValue(0, 0))
	}
	if v, ok := sh1.GetCellValue(0, 1).(float64); !ok || v != 100 {
		t.Fatalf("before delete Sheet1!A2 want 100, got %v", sh1.GetCellValue(0, 1))
	}
	if v, ok := sh1.GetCellValue(0, 2).(float64); !ok || v != 20 {
		t.Fatalf("before delete Sheet1!A3 want 20, got %v", sh1.GetCellValue(0, 2))
	}

	dataIdx := wb.GetSheetIndex(data)
	if err := wb.DeleteSheet(dataIdx); err != nil {
		t.Fatalf("DeleteSheet: %v", err)
	}

	raw1 := cellRaw(t, sh1, 0, 0)
	raw2 := cellRaw(t, sh1, 0, 1)
	raw3 := cellRaw(t, sh1, 0, 2)

	if !strings.Contains(raw1, "#REF!") {
		t.Errorf("after delete Sheet1!A1 want formula containing #REF!, got %q", raw1)
	}
	if !strings.Contains(raw2, "#REF!") {
		t.Errorf("after delete Sheet1!A2 want formula containing #REF!, got %q", raw2)
	}
	if !strings.Contains(raw3, "#REF!") {
		t.Errorf("after delete Sheet1!A3 want formula containing #REF!, got %q", raw3)
	}

	wb.RecalculateAll()
	if _, ok := sh1.GetCellValue(0, 0).(cell.LotusError); !ok {
		t.Errorf("Sheet1!A1 after delete want LotusError, got %v (%T)", sh1.GetCellValue(0, 0), sh1.GetCellValue(0, 0))
	}
	if _, ok := sh1.GetCellValue(0, 1).(cell.LotusError); !ok {
		t.Errorf("Sheet1!A2 after delete want LotusError, got %v (%T)", sh1.GetCellValue(0, 1), sh1.GetCellValue(0, 1))
	}
	if _, ok := sh1.GetCellValue(0, 2).(cell.LotusError); !ok {
		t.Errorf("Sheet1!A3 after delete want LotusError, got %v (%T)", sh1.GetCellValue(0, 2), sh1.GetCellValue(0, 2))
	}
}

// 2. 空白や記号を含むシート名への改名（RenameSheet）でセル数式が正しくクォートされて追従するか
func TestBugfixV9_RenameSheetWithSpecialCharsUpdatesCellFormulas(t *testing.T) {
	wb := sheet.NewWorkbook("rename_special")
	sh1 := wb.Sheets[0]
	data := wb.AddSheet("Data")
	data.SetCellInput(0, 0, "42", nil)
	data.SetCellInput(0, 1, "8", nil)

	sh1.SetCellInput(0, 0, "=Data!A1+1", nil)
	sh1.SetCellInput(0, 1, "=SUM(Data!A1..A2)", nil)
	wb.RecalculateAll()

	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 43 {
		t.Fatalf("before rename A1 want 43, got %v", sh1.GetCellValue(0, 0))
	}

	// Rename with space
	if err := wb.RenameSheet("Data", "2024 Sales"); err != nil {
		t.Fatalf("RenameSheet: %v", err)
	}
	rawA1 := cellRaw(t, sh1, 0, 0)
	if !strings.Contains(rawA1, "'2024 Sales'") {
		t.Errorf("formula after rename want '2024 Sales', got %q", rawA1)
	}
	wb.RecalculateAll()
	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 43 {
		t.Errorf("after rename to 2024 Sales A1 want 43, got %v", sh1.GetCellValue(0, 0))
	}
	if v, ok := sh1.GetCellValue(0, 1).(float64); !ok || v != 50 {
		t.Errorf("after rename to 2024 Sales A2 want 50, got %v", sh1.GetCellValue(0, 1))
	}

	// Rename with single quote in name
	if err := wb.RenameSheet("2024 Sales", "O'Brien"); err != nil {
		t.Fatalf("RenameSheet O'Brien: %v", err)
	}
	rawA1 = cellRaw(t, sh1, 0, 0)
	if !strings.Contains(rawA1, "'O''Brien'") {
		t.Errorf("formula after rename want 'O''Brien', got %q", rawA1)
	}
	wb.RecalculateAll()
	if v, ok := sh1.GetCellValue(0, 0).(float64); !ok || v != 43 {
		t.Errorf("after rename to O'Brien A1 want 43, got %v", sh1.GetCellValue(0, 0))
	}
}

// 3. HLOOKUP の近似一致・完全一致・境界値
func TestBugfixV9_HLookupEdgeCases(t *testing.T) {
	sh := sheet.NewSheet()
	// Row 1 (keys): 10, 20, 30, 40
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "20", nil)
	sh.SetCellInput(2, 0, "30", nil)
	sh.SetCellInput(3, 0, "40", nil)
	// Row 2 (values): "Alpha", "Beta", "Gamma", "Delta"
	sh.SetCellInput(0, 1, "Alpha", nil)
	sh.SetCellInput(1, 1, "Beta", nil)
	sh.SetCellInput(2, 1, "Gamma", nil)
	sh.SetCellInput(3, 1, "Delta", nil)

	// Test approximate match (default or TRUE)
	sh.SetCellInput(0, 2, "=HLOOKUP(25, A1..D2, 2, TRUE)", nil)
	sh.SetCellInput(0, 3, "=HLOOKUP(5, A1..D2, 2, TRUE)", nil)  // less than smallest key
	sh.SetCellInput(0, 4, "=HLOOKUP(40, A1..D2, 2, TRUE)", nil) // exact largest
	sh.SetCellInput(0, 5, "=HLOOKUP(99, A1..D2, 2, TRUE)", nil) // greater than largest
	sh.SetCellInput(0, 6, "=HLOOKUP(20, A1..D2, 2)", nil)       // omitted 4th arg (approximate)

	// Test exact match (FALSE)
	sh.SetCellInput(0, 7, "=HLOOKUP(20, A1..D2, 2, FALSE)", nil)
	sh.SetCellInput(0, 8, "=HLOOKUP(25, A1..D2, 2, FALSE)", nil) // not found exact

	// Test invalid row index
	sh.SetCellInput(0, 9, "=HLOOKUP(10, A1..D2, 0, FALSE)", nil)  // row < 1
	sh.SetCellInput(0, 10, "=HLOOKUP(10, A1..D2, 3, FALSE)", nil) // row > 2

	sh.Recalculate()

	if v := sh.GetCellValue(0, 2); v != "Beta" {
		t.Errorf("HLOOKUP(25, approx) want Beta, got %v", v)
	}
	if _, ok := sh.GetCellValue(0, 3).(cell.LotusError); !ok {
		t.Errorf("HLOOKUP(5, approx) want #N/A/LotusError, got %v", sh.GetCellValue(0, 3))
	}
	if v := sh.GetCellValue(0, 4); v != "Delta" {
		t.Errorf("HLOOKUP(40, approx) want Delta, got %v", v)
	}
	if v := sh.GetCellValue(0, 5); v != "Delta" {
		t.Errorf("HLOOKUP(99, approx) want Delta, got %v", v)
	}
	if v := sh.GetCellValue(0, 6); v != "Beta" {
		t.Errorf("HLOOKUP(20, default) want Beta, got %v", v)
	}
	if v := sh.GetCellValue(0, 7); v != "Beta" {
		t.Errorf("HLOOKUP(20, exact) want Beta, got %v", v)
	}
	if _, ok := sh.GetCellValue(0, 8).(cell.LotusError); !ok {
		t.Errorf("HLOOKUP(25, exact) want #N/A/LotusError, got %v", sh.GetCellValue(0, 8))
	}
	if _, ok := sh.GetCellValue(0, 9).(cell.LotusError); !ok {
		t.Errorf("HLOOKUP row 0 want LotusError, got %v", sh.GetCellValue(0, 9))
	}
	if _, ok := sh.GetCellValue(0, 10).(cell.LotusError); !ok {
		t.Errorf("HLOOKUP row 3 want LotusError, got %v", sh.GetCellValue(0, 10))
	}
}

// 4. SUBTOTAL の各関数コード（1..11, 109）の検証
func TestBugfixV9_SubtotalAllCodes(t *testing.T) {
	sh := sheet.NewSheet()
	// Values: 10, 20, 30, 40, 50
	for i, n := range []string{"10", "20", "30", "40", "50"} {
		sh.SetCellInput(0, i, n, nil)
	}

	tests := []struct {
		code    int
		formula string
		wantVal float64
		desc    string
	}{
		{1, "=SUBTOTAL(1, A1..A5)", 30.0, "AVG"},
		{2, "=SUBTOTAL(2, A1..A5)", 5.0, "COUNT"},
		{3, "=SUBTOTAL(3, A1..A5)", 5.0, "COUNTA"},
		{4, "=SUBTOTAL(4, A1..A5)", 50.0, "MAX"},
		{5, "=SUBTOTAL(5, A1..A5)", 10.0, "MIN"},
		{6, "=SUBTOTAL(6, A1..A5)", 12000000.0, "PRODUCT"},
		{9, "=SUBTOTAL(9, A1..A5)", 150.0, "SUM"},
		{10, "=SUBTOTAL(10, A1..A5)", 250.0, "VAR"},
		{11, "=SUBTOTAL(11, A1..A5)", 200.0, "VARP"},
		{109, "=SUBTOTAL(109, A1..A5)", 150.0, "SUM 109"},
	}

	for i, tc := range tests {
		row := 6 + i
		sh.SetCellInput(0, row, tc.formula, nil)
	}

	// Invalid codes
	sh.SetCellInput(0, 20, "=SUBTOTAL(0, A1..A5)", nil)
	sh.SetCellInput(0, 21, "=SUBTOTAL(999, A1..A5)", nil)

	sh.Recalculate()

	for i, tc := range tests {
		row := 6 + i
		v, ok := sh.GetCellValue(0, row).(float64)
		if !ok || math.Abs(v-tc.wantVal) > 1e-4 {
			t.Errorf("SUBTOTAL(%d, %s) want %v, got %v (cell A%d)", tc.code, tc.desc, tc.wantVal, sh.GetCellValue(0, row), row+1)
		}
	}

	if _, ok := sh.GetCellValue(0, 20).(cell.LotusError); !ok {
		t.Errorf("SUBTOTAL(0) want LotusError, got %v", sh.GetCellValue(0, 20))
	}
	if _, ok := sh.GetCellValue(0, 21).(cell.LotusError); !ok {
		t.Errorf("SUBTOTAL(999) want LotusError, got %v", sh.GetCellValue(0, 21))
	}
}

// 5. YEARFRAC の各 basis 引数（0..4）および閏年（うるう年）跨ぎの検証
func TestBugfixV9_YearFracAllBasesAndLeapYear(t *testing.T) {
	sh := sheet.NewSheet()
	// 2024 is a leap year (Feb has 29 days). 2024-01-01 to 2024-07-01 is exactly 182 days.
	sh.SetCellInput(0, 0, `=YEARFRAC(DATE(2024, 1, 1), DATE(2024, 7, 1), 0)`, nil) // US 30/360: 180/360 = 0.5
	sh.SetCellInput(0, 1, `=YEARFRAC(DATE(2024, 1, 1), DATE(2024, 7, 1), 1)`, nil) // Act/Act: 182/366 ≈ 0.497268
	sh.SetCellInput(0, 2, `=YEARFRAC(DATE(2024, 1, 1), DATE(2024, 7, 1), 2)`, nil) // Act/360: 182/360 ≈ 0.505556
	sh.SetCellInput(0, 3, `=YEARFRAC(DATE(2024, 1, 1), DATE(2024, 7, 1), 3)`, nil) // Act/365: 182/365 ≈ 0.498630
	sh.SetCellInput(0, 4, `=YEARFRAC(DATE(2024, 1, 1), DATE(2024, 7, 1), 4)`, nil) // Euro 30/360: 180/360 = 0.5

	// Invalid bases
	sh.SetCellInput(0, 5, `=YEARFRAC(DATE(2024, 1, 1), DATE(2024, 7, 1), -1)`, nil)
	sh.SetCellInput(0, 6, `=YEARFRAC(DATE(2024, 1, 1), DATE(2024, 7, 1), 5)`, nil)

	sh.Recalculate()

	checkFloat := func(row int, want float64, basis int) {
		t.Helper()
		v, ok := sh.GetCellValue(0, row).(float64)
		if !ok || math.Abs(v-want) > 1e-4 {
			t.Errorf("YEARFRAC basis %d want %v, got %v", basis, want, sh.GetCellValue(0, row))
		}
	}

	checkFloat(0, 0.5, 0)
	checkFloat(1, 182.0/366.0, 1)
	checkFloat(2, 182.0/360.0, 2)
	checkFloat(3, 182.0/365.0, 3)
	checkFloat(4, 0.5, 4)

	if _, ok := sh.GetCellValue(0, 5).(cell.LotusError); !ok {
		t.Errorf("YEARFRAC basis -1 want LotusError, got %v", sh.GetCellValue(0, 5))
	}
	if _, ok := sh.GetCellValue(0, 6).(cell.LotusError); !ok {
		t.Errorf("YEARFRAC basis 5 want LotusError, got %v", sh.GetCellValue(0, 6))
	}
}

// 6. INDIRECT による動的セル参照および動的範囲集計の検証
func TestBugfixV9_IndirectDynamicReferences(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(1, 0, "42", nil)     // B1
	sh.SetCellInput(1, 1, "58", nil)     // B2
	sh.SetCellInput(0, 0, "B1", nil)     // A1 contains ref text "B1"
	sh.SetCellInput(0, 1, "B1..B2", nil) // A2 contains range text "B1..B2"
	sh.SetCellInput(0, 2, "=INDIRECT(A1)", nil)
	sh.SetCellInput(0, 3, "=SUM(INDIRECT(A2))", nil)
	sh.SetCellInput(0, 4, `=INDIRECT("INVALID!")`, nil)

	sh.Recalculate()

	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 42 {
		t.Errorf("INDIRECT(A1) want 42, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 100 {
		t.Errorf("SUM(INDIRECT(A2)) want 100, got %v", sh.GetCellValue(0, 3))
	}
	if _, ok := sh.GetCellValue(0, 4).(cell.LotusError); !ok {
		t.Errorf("INDIRECT(invalid) want LotusError, got %v", sh.GetCellValue(0, 4))
	}
}

// 7. Markdown エクスポート時の特殊文字（パイプ、改行）のエスケープ検証
func TestBugfixV9_ExportMarkdownSpecialCharsEscape(t *testing.T) {
	wb := sheet.NewWorkbook("md_escape")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "Item | Name", nil)
	sh.SetCellInput(1, 0, "Line1\nLine2", nil)
	sh.SetCellInput(0, 1, "Alpha", nil)
	sh.SetCellInput(1, 1, "100", nil)

	p := filepath.Join(t.TempDir(), "table.md")
	if err := sh.ExportMarkdown(p); err != nil {
		t.Fatalf("ExportMarkdown: %v", err)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, `Item \| Name`) {
		t.Errorf("Markdown export should escape pipe character, got:\n%s", text)
	}
	if !strings.Contains(text, `Line1<br>Line2`) {
		t.Errorf("Markdown export should convert newline to <br>, got:\n%s", text)
	}

	// Verify that each row line has the expected number of pipes (consistent columns)
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) < 3 {
		t.Fatalf("want at least 3 lines, got %d", len(lines))
	}
}

// 8. CSV エクスポート時の RFC 4180 準拠（カンマ、クォート、改行のエスケープ）検証
func TestBugfixV9_ExportCSVRFC4180(t *testing.T) {
	wb := sheet.NewWorkbook("csv_rfc")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "Hello, World", nil)
	sh.SetCellInput(1, 0, `He said "Hi"`, nil)
	sh.SetCellInput(0, 1, "Line1\nLine2", nil)
	sh.SetCellInput(1, 1, "42", nil)

	p := filepath.Join(t.TempDir(), "test.csv")
	if err := sh.ExportCSV(p); err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}

	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, `"Hello, World"`) {
		t.Errorf("CSV should quote comma: got:\n%s", text)
	}
	if !strings.Contains(text, `"He said ""Hi"""`) {
		t.Errorf("CSV should double quote inside quotes: got:\n%s", text)
	}
	if !strings.Contains(text, "\"Line1\nLine2\"") {
		t.Errorf("CSV should quote newline: got:\n%s", text)
	}
}

// 9. DataFill の日付ステップ単位（d, w, m, y）および stop 境界値の検証
func TestBugfixV9_DataFillDateUnitsAndBounds(t *testing.T) {
	sh := sheet.NewSheet()
	targetMonth := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 0},
		End:   coord.CellRef{Col: 0, Row: 5},
	}
	// Step 1 month, stop at 2023-03-01
	if err := sh.DataFill(targetMonth, "2023-01-01", "1m", "2023-03-01"); err != nil {
		t.Fatalf("DataFill month: %v", err)
	}
	if cellRaw(t, sh, 0, 0) != "'2023-01-01" {
		t.Errorf("Row 0 want '2023-01-01, got %q", cellRaw(t, sh, 0, 0))
	}
	if cellRaw(t, sh, 0, 1) != "'2023-02-01" {
		t.Errorf("Row 1 want '2023-02-01, got %q", cellRaw(t, sh, 0, 1))
	}
	if cellRaw(t, sh, 0, 2) != "'2023-03-01" {
		t.Errorf("Row 2 want '2023-03-01, got %q", cellRaw(t, sh, 0, 2))
	}
	// Row 3 and 4 should be empty due to stop date
	if c := sh.GetCell(0, 3); c != nil && c.RawInput != "" {
		t.Errorf("Row 3 should be empty, got %q", c.RawInput)
	}

	// Step 1 week (7 days)
	targetWeek := coord.RangeRef{
		Start: coord.CellRef{Col: 1, Row: 0},
		End:   coord.CellRef{Col: 1, Row: 2},
	}
	if err := sh.DataFill(targetWeek, "2023-01-01", "1w", ""); err != nil {
		t.Fatalf("DataFill week: %v", err)
	}
	if cellRaw(t, sh, 1, 1) != "'2023-01-08" {
		t.Errorf("Week Row 1 want '2023-01-08, got %q", cellRaw(t, sh, 1, 1))
	}

	// Step 1 year
	targetYear := coord.RangeRef{
		Start: coord.CellRef{Col: 2, Row: 0},
		End:   coord.CellRef{Col: 2, Row: 2},
	}
	if err := sh.DataFill(targetYear, "2020-01-01", "1y", ""); err != nil {
		t.Fatalf("DataFill year: %v", err)
	}
	if cellRaw(t, sh, 2, 1) != "'2021-01-01" {
		t.Errorf("Year Row 1 want '2021-01-01, got %q", cellRaw(t, sh, 2, 1))
	}
}

// 10. TUI オート関数（AutoSum/Average/Max）およびセル境界ジャンプ・日付挿入の検証
func TestBugfixV9_TUIAutoFunctionsAndBoundaryJump(t *testing.T) {
	wb := sheet.NewWorkbook("tui_auto")
	sh := wb.Sheets[0]
	// Column numbers at A1..A3
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(0, 1, "20", nil)
	sh.SetCellInput(0, 2, "30", nil)

	app := newSimApp(t, sh, "tui_auto.hwk")

	// 1. AutoSum below vertical numbers (at A4)
	app.SetCursorForTest(0, 3)
	app.ExecuteActionHandlerForTest("doPaletteAutoSum", nil)
	if raw := cellRaw(t, sh, 0, 3); raw != "=SUM(A1..A3)" {
		t.Errorf("AutoSum at A4 want =SUM(A1..A3), got %q", raw)
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if c := sh.GetCell(0, 3); c != nil && c.RawInput != "" {
		t.Errorf("AutoSum undo want empty, got %q", c.RawInput)
	}

	// 2. Horizontal selection AutoAverage
	sh.SetCellInput(0, 5, "5", nil)
	sh.SetCellInput(1, 5, "15", nil)
	sh.SetCellInput(2, 5, "25", nil)
	rngH := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 5},
		End:   coord.CellRef{Col: 2, Row: 5},
	}
	app.SelectRangeForTest(&rngH)
	app.SetCursorForTest(0, 5)
	app.ExecuteActionHandlerForTest("doPaletteAverage", nil)
	if raw := cellRaw(t, sh, 3, 5); !strings.Contains(raw, "=AVERAGE(A6") || !strings.Contains(raw, "C6)") {
		t.Errorf("AutoAverage want =AVERAGE(A6:C6) at D6, got %q", raw)
	}

	// 3. doPaletteToday & doPaletteNow
	app.SelectRangeForTest(nil)
	app.SetCursorForTest(4, 0)
	app.ExecuteActionHandlerForTest("doPaletteToday", nil)
	if raw := cellRaw(t, sh, 4, 0); raw != "=TODAY()" {
		t.Errorf("doPaletteToday want =TODAY(), got %q", raw)
	}
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if c := sh.GetCell(4, 0); c != nil && c.RawInput != "" {
		t.Errorf("doPaletteToday undo want empty, got %q", c.RawInput)
	}

	// 4. jumpBoundary test via Lotus End + Down / Up
	// Block 1: A1..A3 (rows 0..2)
	// Gap: A4..A5 (rows 3..4)
	// Block 2: A6 (row 5)
	app.SetCursorForTest(0, 0)
	// Jump 1: from row 0 (in-block) -> lands on row 2 (end of Block 1)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.CursorRow() != 2 {
		t.Errorf("End+Down from A1 want row 2 (A3, end of block), got %d", app.CursorRow())
	}
	// Jump 2: from row 2 across gap -> lands on row 5 (A6, start of Block 2)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if app.CursorRow() != 5 {
		t.Errorf("End+Down across gap want row 5 (A6), got %d", app.CursorRow())
	}
	// Jump 3: from row 5 up across gap -> lands on row 2 (A3, bottom of Block 1)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if app.CursorRow() != 2 {
		t.Errorf("End+Up across gap want row 2 (A3), got %d", app.CursorRow())
	}
	// Jump 4: from row 2 up within Block 1 -> lands on row 0 (A1, top of Block 1)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone))
	if app.CursorRow() != 0 {
		t.Errorf("End+Up within block want row 0 (A1), got %d", app.CursorRow())
	}
}

// 11. EraseAll によるシート全消去・全状態リセットの検証
func TestBugfixV9_SheetEraseAll(t *testing.T) {
	sh := sheet.NewSheet()
	sh.SetCellInput(0, 0, "42", nil)
	sh.SetColWidth(0, 25)
	sh.SetNamedRange("MYRANGE", mustParseRange(t, "A1:A5"))
	sh.SetFrozenRows(2)
	sh.SetFrozenCols(2)
	r := mustParseRange(t, "A1:A5")
	sh.Graph().RangeX = &r

	sh.EraseAll()

	if c := sh.GetCell(0, 0); c != nil && c.RawInput != "" {
		t.Errorf("after EraseAll A1 should be empty, got %q", c.RawInput)
	}
	if len(sh.NamedRanges()) != 0 {
		t.Errorf("after EraseAll NamedRanges should be empty, got %v", sh.NamedRanges())
	}
	if sh.GetColWidth(0) != sh.DefaultColWidth() {
		t.Errorf("after EraseAll col width want %d, got %d", sh.DefaultColWidth(), sh.GetColWidth(0))
	}
	if sh.FrozenRows() != 0 || sh.FrozenCols() != 0 {
		t.Errorf("after EraseAll frozen want 0,0, got %d,%d", sh.FrozenRows(), sh.FrozenCols())
	}
	if sh.Graph().RangeX != nil {
		t.Errorf("after EraseAll Graph.RangeX want nil, got %v", sh.Graph().RangeX)
	}
}

// 12. XLSX テーブル構造化参照（Structured References）の解決検証
func TestBugfixV9_XLSXStructuredReferences(t *testing.T) {
	// Table "Sales" spanning columns A..C (0..2), rows 1..5 (0..4).
	// Header is at row 0 (row 1 in Excel), Data rows 1..4 (rows 2..5 in Excel).
	cols := []string{"Region", "Q1", "Q2"}
	tbl := sheet.NewXLSXTableForTest("Sales", 0, 0, 2, 4, cols)
	tables := map[string]*sheet.XLSXTable{
		"SALES": tbl,
	}

	// 1. Column reference: Sales[Q1] -> B2..B5
	f1 := sheet.ConvertExcelFormulaWithTablesForTest("=SUM(Sales[Q1])", 0, 0, tables)
	if f1 != "=SUM(B2..B5)" {
		t.Errorf("Sales[Q1] want =SUM(B2..B5), got %q", f1)
	}

	// 2. This row reference: Sales[@Q1] + Sales[@Q2] at row 2 (0-indexed) -> B3 + C3
	f2 := sheet.ConvertExcelFormulaWithTablesForTest("=Sales[@Q1]+Sales[@Q2]", 0, 2, tables)
	if f2 != "=B3+C3" {
		t.Errorf("Sales[@Q1]+Sales[@Q2] want =B3+C3, got %q", f2)
	}

	// 3. Headers reference: Sales[[#Headers],[Region]] -> A1
	f3 := sheet.ConvertExcelFormulaWithTablesForTest("=Sales[[#Headers],[Region]]", 0, 0, tables)
	if f3 != "=A1" {
		t.Errorf("Sales[[#Headers],[Region]] want =A1, got %q", f3)
	}

	// 4. All reference: Sales[[#All],[Q1]] -> B1..B5
	f4 := sheet.ConvertExcelFormulaWithTablesForTest("=Sales[[#All],[Q1]]", 0, 0, tables)
	if f4 != "=B1..B5" {
		t.Errorf("Sales[[#All],[Q1]] want =B1..B5, got %q", f4)
	}
}

// Suppress unused import warning for time
var _ = time.Now
