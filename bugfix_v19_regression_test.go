package main

import (
	"path/filepath"
	"strings"
	"testing"

	"hasucalc/formula"
	"hasucalc/sheet"
)

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
