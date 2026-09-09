package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
)

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
