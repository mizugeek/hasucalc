package main

import (
	"path/filepath"
	"strings"
	"testing"

	"hasucalc/cell"
	"hasucalc/sheet"
)

func TestBugfixCurrencySymbolPerCellAndPersist(t *testing.T) {
	wb := sheet.NewWorkbook("currency_sym")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1234.5", nil)
	sh.SetCellInput(1, 0, "1234.5", nil)
	sh.FormatRange(mustParseRange(t, "A1"), cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2, CurrencySymbol: "¥"})
	sh.FormatRange(mustParseRange(t, "B1"), cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2, CurrencySymbol: "€"})

	a1 := sh.GetCell(0, 0).Render(12, sh.GlobalFormat())
	b1 := sh.GetCell(1, 0).Render(12, sh.GlobalFormat())
	if !strings.Contains(strings.TrimSpace(a1), "¥1,234.50") {
		t.Fatalf("A1 want ¥1,234.50, got %q", a1)
	}
	if !strings.Contains(strings.TrimSpace(b1), "€1,234.50") {
		t.Fatalf("B1 want €1,234.50, got %q", b1)
	}

	p := filepath.Join(t.TempDir(), "currency.hwk")
	if err := wb.SaveJSON(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := sheet.LoadWorkbookJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	lsh := loaded.Sheets[0]
	fa := lsh.GetCell(0, 0).FormatSpec
	fb := lsh.GetCell(1, 0).FormatSpec
	if fa == nil || fa.CurrencySymbol != "¥" {
		t.Fatalf("A1 FormatSpec after reload want ¥, got %#v", fa)
	}
	if fb == nil || fb.CurrencySymbol != "€" {
		t.Fatalf("B1 FormatSpec after reload want €, got %#v", fb)
	}
	if got := cell.ParseCellFormat("(C2¥)"); got.Type != cell.FmtCurrency || got.CurrencySymbol != "¥" || got.Decimals != 2 {
		t.Fatalf("ParseCellFormat (C2¥) = %#v", got)
	}
	if got := cell.ParseCellFormat("(C2)"); got.CurrencySymbolOrDefault() != "$" {
		t.Fatalf("default symbol want $, got %#v", got)
	}
	if (cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2, CurrencySymbol: "¥"}).String() != "(C2¥)" {
		t.Fatalf("String() want (C2¥)")
	}
}

func TestBugfixCurrencyNegativeWithSymbol(t *testing.T) {
	got := cell.FormatNumber(-12.3, cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2, CurrencySymbol: "£"})
	if got != "(£12.30)" {
		t.Fatalf("negative currency want (£12.30), got %q", got)
	}
}
