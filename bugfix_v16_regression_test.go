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
)

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
	sh.SetCellInput(0, 0, "10", nil) // A1
	sh.SetCellInput(1, 0, "20", nil) // B1
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
