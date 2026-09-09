package main

import (
	"path/filepath"
	"strings"
	"testing"

	"hasucalc/formula"
	"hasucalc/sheet"
)

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
