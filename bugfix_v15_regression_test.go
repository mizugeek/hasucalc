package main

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
)

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
	sh.SetCellInput(0, 0, "10", nil) // A1
	sh.SetCellInput(0, 3, "40", nil) // A4
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
	sh.SetCellInput(0, 0, "Sheet1", nil) // A1 = "Sheet1"
	sh.SetCellInput(1, 0, "=ADDRESS(1, 1, 1, TRUE, A1)", nil) // B1
	sh.SetCellInput(0, 1, "'0.00", nil) // A2 = "0.00" as text
	sh.SetCellInput(1, 1, "=TEXT(12.345, A2)", nil) // B2
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
	sh.SetCellInput(0, 0, "apple", nil) // A1
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
