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
	sh.SetCellInput(1, 4, "10", nil)   // B5
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

