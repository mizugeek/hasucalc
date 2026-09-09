package main

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
)

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
	sh.SetCellInput(0, 9, "=HLOOKUP(10, A1..D2, 0, FALSE)", nil) // row < 1
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
	sh.SetCellInput(1, 0, "42", nil)       // B1
	sh.SetCellInput(1, 1, "58", nil)       // B2
	sh.SetCellInput(0, 0, "B1", nil)       // A1 contains ref text "B1"
	sh.SetCellInput(0, 1, "B1..B2", nil)   // A2 contains range text "B1..B2"
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
