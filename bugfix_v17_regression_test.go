package main

import (
	"path/filepath"
	"testing"

	"hasucalc/coord"
	"hasucalc/sheet"
)

func TestBugfixV17_IndirectR1C1(t *testing.T) {
	wb := sheet.NewWorkbook("v17_r1c1")
	sh := wb.Sheets[0]
	sh.SetCellInput(2, 2, "99", nil) // C3
	sh.SetCellInput(0, 0, `=INDIRECT("R3C3", FALSE)`, nil)
	sh.SetCellInput(1, 1, `=INDIRECT("R[1]C[1]", FALSE)`, nil) // B2 -> C3
	sh.SetCellInput(3, 0, `=SUM(INDIRECT("R3C3:R3C3", FALSE))`, nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 99 {
		t.Fatalf("INDIRECT(\"R3C3\", FALSE) want 99, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(1, 1).(float64); !ok || v != 99 {
		t.Fatalf("INDIRECT(\"R[1]C[1]\", FALSE) from B2 want 99, got %v", sh.GetCellValue(1, 1))
	}
	if v, ok := sh.GetCellValue(3, 0).(float64); !ok || v != 99 {
		t.Fatalf("SUM(INDIRECT R1C1 range) want 99, got %v", sh.GetCellValue(3, 0))
	}
}

func TestBugfixV17_TextScientific(t *testing.T) {
	wb := sheet.NewWorkbook("v17_text_sci")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, `=TEXT(1234.5, "0.00E+00")`, nil)
	sh.SetCellInput(0, 1, `=TEXT(0.5, "0.00E+00")`, nil)
	wb.RecalculateAll()
	if v, ok := sh.GetCellValue(0, 0).(string); !ok || v != "1.23E+03" {
		t.Fatalf("TEXT(1234.5,\"0.00E+00\") want 1.23E+03, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(string); !ok || v != "5.00E-01" {
		t.Fatalf("TEXT(0.5,\"0.00E+00\") want 5.00E-01, got %v", sh.GetCellValue(0, 1))
	}
}

func TestBugfixV17_ODSFreezeRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("v17_ods_freeze")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "x", nil)
	sh.SetFrozenRows(3)
	sh.SetFrozenCols(2)
	p := filepath.Join(t.TempDir(), "freeze.ods")
	if err := wb.ExportODS(p); err != nil {
		t.Fatalf("ExportODS: %v", err)
	}
	loaded, err := sheet.ImportODSWorkbook(p)
	if err != nil {
		t.Fatalf("ImportODSWorkbook: %v", err)
	}
	got := loaded.Sheets[0]
	if got.FrozenRows() != 3 || got.FrozenCols() != 2 {
		t.Fatalf("ODS freeze want rows=3 cols=2, got rows=%d cols=%d", got.FrozenRows(), got.FrozenCols())
	}
}

func TestBugfixV17_XLSXManualRecalcRoundTrip(t *testing.T) {
	wb := sheet.NewWorkbook("v17_xlsx_manual")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "10", nil)
	sh.SetCellInput(1, 0, "=A1*2", nil)
	sh.SetRecalcMode("MANUAL")
	wb.RecalculateAll()
	p := filepath.Join(t.TempDir(), "manual.xlsx")
	if err := wb.ExportXLSX(p); err != nil {
		t.Fatalf("ExportXLSX: %v", err)
	}
	loaded, err := sheet.ImportXLSXWorkbook(p)
	if err != nil {
		t.Fatalf("ImportXLSXWorkbook: %v", err)
	}
	lsh := loaded.Sheets[0]
	if lsh.RecalcMode() != "MANUAL" {
		t.Fatalf("XLSX RecalcMode want MANUAL, got %q", lsh.RecalcMode())
	}
	if v, ok := lsh.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("imported B1 want 20, got %v", lsh.GetCellValue(1, 0))
	}
	lsh.SetCellInput(0, 0, "7", nil)
	if v, ok := lsh.GetCellValue(1, 0).(float64); !ok || v != 20 {
		t.Fatalf("MANUAL edit of A1 should leave B1 at 20, got %v", lsh.GetCellValue(1, 0))
	}
}

func TestBugfixV17_ParseR1C1Helpers(t *testing.T) {
	cr, err := coord.ParseR1C1CellRef("R1C1", 5, 5)
	if err != nil || cr.Col != 0 || cr.Row != 0 {
		t.Fatalf("R1C1 want A1, got %+v err=%v", cr, err)
	}
	cr, err = coord.ParseR1C1CellRef("R[1]C[2]", 1, 1)
	if err != nil || cr.Col != 3 || cr.Row != 2 {
		t.Fatalf("R[1]C[2] from B2 want D3, got %+v err=%v", cr, err)
	}
	rr, err := coord.ParseR1C1RangeRef("Sheet2!R1C1:R2C2", 0, 0)
	if err != nil || rr.Sheet != "Sheet2" || rr.MinCol() != 0 || rr.MaxRow() != 1 {
		t.Fatalf("range want Sheet2!A1:B2, got %+v err=%v", rr, err)
	}
}
