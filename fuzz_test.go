package main

import (
	"os"
	"path/filepath"
	"testing"

	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
)

func FuzzFormulaParseAndEval(f *testing.F) {
	seeds := []string{
		"=1+2",
		"=-2^2",
		"=SUM(A1:B10)",
		"=IF(A1>0, 1, -1)",
		"=VLOOKUP(10, A1:B10, 2, FALSE)",
		"=OFFSET(A1, 1, 1, 2, 2)",
		"=A1..B10",
		"@AVG(A1..A5)",
		"+10+20",
		"50%",
		"=\"hello \" & \"world\"",
		"=CONCATENATE(A1, B1)",
		"=1/0",
		"=A1+B1",
		"=INDEX(A1:C3, 2, 2)",
		"=CHOOSE(1, 10, 20)",
		"=TEXT(1234.56, \"#,##0.00\")",
		"=DAYS360(DATE(2024,1,1), DATE(2024,12,31))",
		"=XLOOKUP(\"foo\", A1:A5, B1:B5, \"not found\")",
		"=ROUND(1.234, 2)",
		"=-2^-2",
		"=2^3^2",
		"-(2^2)",
		"=A:A",
		"=1:1",
		"='Sheet-1'!A1",
		"=",
		"@",
		"+",
		"=\"",
		"=((((((((1))))))))",
		"=1//0",
		"=1++2",
		"=@",
		"=-",
		"=.5",
		"=1e308",
		"=1e-308",
		"=1e500",
		"=#REF!",
		"=#VALUE!",
		"=ERR()",
		"=NA()",
		"=ROW()",
		"=COLUMN()",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, expr string) {
		// 1. Formula Parser directly
		ast, err := formula.ParseFormula(expr)
		if err == nil && ast != nil {
			_ = formula.NodeToString(ast)
		}

		// 2. Sheet evaluation (integration)
		wb := sheet.NewWorkbook("fuzz_wb")
		sh := wb.Sheets[0]
		sh.SetCellInput(0, 0, expr, nil)
		wb.RecalculateAll()
		_ = sh.GetCellValue(0, 0)
	})
}

func FuzzCoordParsing(f *testing.F) {
	seeds := []string{
		"A1",
		"XFD1048576",
		"$A$1",
		"Sheet1!A1:B10",
		"'Sheet 1'!$A$1:$B$10",
		"A1..B10",
		"A:A",
		"1:1",
		"",
		"!",
		"'''",
		"XFE1",
		"A1048577",
		"Sheet1!",
		"!A1",
		"'Unterminated!A1",
		"A1:XFD1048576",
		"IV65536",
		"Z$100",
		"$AA99",
		"R1C1",
		"R[-1]C[2]",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = coord.ParseCellRef(input)
		_, _ = coord.ParseRangeRef(input)
		_ = coord.QuoteSheetPrefix(input)
		_ = coord.UnquoteSheetName(input)
		_, _ = coord.ParseR1C1CellRef(input, 5, 5)
		_, _ = coord.ParseR1C1RangeRef(input, 5, 5)
		_ = coord.LetterToCol(input)
	})
}

func FuzzCompactJSONLoader(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"version":"HasuCalc/2.0","name":"test.hwk","sheets":[{"name":"Sheet1","cells":{"A1":10,"B1":"=A1*2"}}]}`),
		[]byte(`{"sheets":[]}`),
		[]byte(`{}`),
		[]byte(`invalid json`),
		[]byte("\x1f\x8b\x08\x00\x00\x00\x00\x00"), // truncated gzip
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		tmpDir := t.TempDir()
		p := filepath.Join(tmpDir, "fuzz.hwk")
		if err := os.WriteFile(p, data, 0644); err != nil {
			return
		}
		_, _ = sheet.LoadSheetJSON(p)
	})
}

func FuzzCSVImport(f *testing.F) {
	seeds := [][]byte{
		[]byte("A,B,C\n1,2,3\n"),
		[]byte("\xEF\xBB\xBFName,Score\nAlice,100\n"),
		[]byte("\"hello, world\",test\n"),
		[]byte("unterminated,\"quote\n"),
		[]byte(""),
		[]byte("\n\n\n"),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		tmpDir := t.TempDir()
		p := filepath.Join(tmpDir, "fuzz.csv")
		if err := os.WriteFile(p, data, 0644); err != nil {
			return
		}
		_, _ = sheet.ImportSheetCSV(p)
	})
}
