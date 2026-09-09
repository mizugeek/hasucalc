package main

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
	"hasucalc/tui"
)

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
