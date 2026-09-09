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
