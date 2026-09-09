package main

import (
	"strings"
	"testing"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
)

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
