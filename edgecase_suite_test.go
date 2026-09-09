package main

import (
	"math"
	"testing"

	"hasucalc/cell"
	"hasucalc/sheet"
)

// 1. Zero Division & Zero Values
func TestEdgeCases_ZeroAndDivByZero(t *testing.T) {
	sh := sheet.NewSheet()

	sh.SetCellInput(0, 0, "=10/0", nil)              // A1: #DIV/0!
	sh.SetCellInput(0, 1, "=MOD(10, 0)", nil)        // A2: #DIV/0!
	sh.SetCellInput(0, 2, "=QUOTIENT(10, 0)", nil)   // A3: #DIV/0!
	sh.SetCellInput(0, 3, "=AVERAGE(B1:B5)", nil)    // A4: All empty -> #DIV/0!
	sh.SetCellInput(0, 4, "=SLN(1000, 100, 0)", nil) // A5: Life=0 -> Error
	sh.SetCellInput(0, 5, "=VAR.S(10)", nil)         // A6: N=1 -> Error (denom n-1=0)

	sh.Recalculate()

	assertIsError(t, sh.GetCellValue(0, 0), "10/0")
	assertIsError(t, sh.GetCellValue(0, 1), "MOD(10, 0)")
	assertIsError(t, sh.GetCellValue(0, 2), "QUOTIENT(10, 0)")
	assertIsError(t, sh.GetCellValue(0, 3), "AVERAGE(empty)")
	assertIsError(t, sh.GetCellValue(0, 4), "SLN(..., 0)")
	assertIsError(t, sh.GetCellValue(0, 5), "VAR.S(single)")
}

// 2. Negative & Extreme Numbers / Math Domains
func TestEdgeCases_NegativeAndMathDomain(t *testing.T) {
	sh := sheet.NewSheet()

	sh.SetCellInput(0, 0, "=SQRT(-4)", nil)        // A1: Error
	sh.SetCellInput(0, 1, "=FACT(-1)", nil)        // A2: Error
	sh.SetCellInput(0, 2, "=LOG(0)", nil)          // A3: Error
	sh.SetCellInput(0, 3, "=LOG(-10)", nil)        // A4: Error
	sh.SetCellInput(0, 4, "=LN(0)", nil)           // A5: Error
	sh.SetCellInput(0, 5, "=LN(-5)", nil)          // A6: Error
	sh.SetCellInput(0, 6, "=COMBIN(5, 6)", nil)    // A7: Error (k > n)
	sh.SetCellInput(0, 7, "=COMBIN(5, -1)", nil)   // A8: Error (k < 0)
	sh.SetCellInput(0, 8, "=PERMUT(5, 6)", nil)    // A9: Error (k > n)
	sh.SetCellInput(0, 9, "=POWER(0, 0)", nil)     // A10: 1
	sh.SetCellInput(0, 10, "=POWER(-4, 0.5)", nil) // A11: Error (complex)

	sh.Recalculate()

	assertIsError(t, sh.GetCellValue(0, 0), "SQRT(-4)")
	assertIsError(t, sh.GetCellValue(0, 1), "FACT(-1)")
	assertIsError(t, sh.GetCellValue(0, 2), "LOG(0)")
	assertIsError(t, sh.GetCellValue(0, 3), "LOG(-10)")
	assertIsError(t, sh.GetCellValue(0, 4), "LN(0)")
	assertIsError(t, sh.GetCellValue(0, 5), "LN(-5)")
	assertIsError(t, sh.GetCellValue(0, 6), "COMBIN(5, 6)")
	assertIsError(t, sh.GetCellValue(0, 7), "COMBIN(5, -1)")
	assertIsError(t, sh.GetCellValue(0, 8), "PERMUT(5, 6)")

	if v, ok := sh.GetCellValue(0, 9).(float64); !ok || v != 1.0 {
		t.Errorf("Expected POWER(0, 0) = 1, got %v", sh.GetCellValue(0, 9))
	}
	assertIsError(t, sh.GetCellValue(0, 10), "POWER(-4, 0.5)")
}

// 3. Empty Cells, Nil, and Type Coercion
func TestEdgeCases_EmptyAndTypeCoercion(t *testing.T) {
	sh := sheet.NewSheet()

	// B1 is empty
	sh.SetCellInput(0, 0, "=B1 + 10", nil)                // A1: 10
	sh.SetCellInput(0, 1, "=10 - B1", nil)                // A2: 10
	sh.SetCellInput(0, 2, "=B1 * 5", nil)                 // A3: 0
	sh.SetCellInput(0, 3, "=\"10\" + 20", nil)            // A4: 30 (string number coercion)
	sh.SetCellInput(0, 4, "=\"1.5\" * 2", nil)            // A5: 3.0
	sh.SetCellInput(0, 5, "=\"hello\" + 10", nil)         // A6: Error
	sh.SetCellInput(0, 6, "=SUM(10, \"hello\", 20)", nil) // A7: 30 (strings ignored in SUM)

	sh.Recalculate()

	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 10.0 {
		t.Errorf("Expected B1 + 10 = 10, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != 10.0 {
		t.Errorf("Expected 10 - B1 = 10, got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != 0.0 {
		t.Errorf("Expected B1 * 5 = 0, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 30.0 {
		t.Errorf("Expected \"10\" + 20 = 30, got %v", sh.GetCellValue(0, 3))
	}
	if v, ok := sh.GetCellValue(0, 4).(float64); !ok || v != 3.0 {
		t.Errorf("Expected \"1.5\" * 2 = 3, got %v", sh.GetCellValue(0, 4))
	}
	assertIsError(t, sh.GetCellValue(0, 5), "\"hello\" + 10")
	if v, ok := sh.GetCellValue(0, 6).(float64); !ok || v != 30.0 {
		t.Errorf("Expected SUM(10, \"hello\", 20) = 30, got %v", sh.GetCellValue(0, 6))
	}
}

// 4. Date & Time Boundaries (Month/Day Overflows)
func TestEdgeCases_DateAndTimeBoundaries(t *testing.T) {
	sh := sheet.NewSheet()

	// Month overflows
	sh.SetCellInput(0, 0, "=DATE(2024, 13, 1)", nil) // A1: 2025-01-01
	sh.SetCellInput(0, 1, "=DATE(2024, 0, 1)", nil)  // A2: 2023-12-01
	sh.SetCellInput(0, 2, "=DATE(2024, -1, 1)", nil) // A3: 2023-11-01

	// Day overflows
	sh.SetCellInput(0, 3, "=DATE(2024, 1, 32)", nil) // A4: 2024-02-01
	sh.SetCellInput(0, 4, "=DATE(2024, 1, 0)", nil)  // A5: 2023-12-31

	// DATEDIF start > end
	sh.SetCellInput(0, 5, "=DATEDIF(DATE(2024, 2, 1), DATE(2024, 1, 1), \"D\")", nil) // A6: Error

	sh.Recalculate()

	fmtD := cell.CellFormat{Type: cell.FmtDate, DateFormat: 1}
	if s := cell.FormatNumber(sh.GetCellValue(0, 0).(float64), fmtD); s != "2025/01/01" {
		t.Errorf("Expected DATE(2024, 13, 1) = 2025/01/01, got %q", s)
	}
	if s := cell.FormatNumber(sh.GetCellValue(0, 1).(float64), fmtD); s != "2023/12/01" {
		t.Errorf("Expected DATE(2024, 0, 1) = 2023/12/01, got %q", s)
	}
	if s := cell.FormatNumber(sh.GetCellValue(0, 2).(float64), fmtD); s != "2023/11/01" {
		t.Errorf("Expected DATE(2024, -1, 1) = 2023/11/01, got %q", s)
	}
	if s := cell.FormatNumber(sh.GetCellValue(0, 3).(float64), fmtD); s != "2024/02/01" {
		t.Errorf("Expected DATE(2024, 1, 32) = 2024/02/01, got %q", s)
	}
	if s := cell.FormatNumber(sh.GetCellValue(0, 4).(float64), fmtD); s != "2023/12/31" {
		t.Errorf("Expected DATE(2024, 1, 0) = 2023/12/31, got %q", s)
	}
	assertIsError(t, sh.GetCellValue(0, 5), "DATEDIF(start > end)")
}

// 5. Lookup & Ranking Boundaries
func TestEdgeCases_LookupAndRankingBoundaries(t *testing.T) {
	sh := sheet.NewSheet()

	sh.SetCellInput(1, 0, "10", nil) // B1
	sh.SetCellInput(1, 1, "20", nil) // B2
	sh.SetCellInput(1, 2, "30", nil) // B3

	sh.SetCellInput(0, 0, "=INDEX(B1:B3, 0, 1)", nil)      // A1: whole column, implicit intersection → B1
	sh.SetCellInput(0, 1, "=INDEX(B1:B3, 4, 1)", nil)      // A2: Error (row > 3)
	sh.SetCellInput(0, 2, "=LARGE(B1:B3, 0)", nil)         // A3: Error (k < 1)
	sh.SetCellInput(0, 3, "=LARGE(B1:B3, 4)", nil)         // A4: Error (k > 3)
	sh.SetCellInput(0, 4, "=SMALL(B1:B3, 0)", nil)         // A5: Error (k < 1)
	sh.SetCellInput(0, 5, "=PERCENTILE(B1:B3, -0.1)", nil) // A6: Error (k < 0)
	sh.SetCellInput(0, 6, "=PERCENTILE(B1:B3, 1.1)", nil)  // A7: Error (k > 1)
	sh.SetCellInput(0, 7, "=MODE(B1:B3)", nil)             // A8: #N/A (no duplicate)

	sh.Recalculate()

	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 10 {
		t.Errorf("INDEX(..., 0, 1) = %v, want 10 (first of returned column)", sh.GetCellValue(0, 0))
	}
	assertIsError(t, sh.GetCellValue(0, 1), "INDEX(..., 4, 1)")
	assertIsError(t, sh.GetCellValue(0, 2), "LARGE(..., 0)")
	assertIsError(t, sh.GetCellValue(0, 3), "LARGE(..., 4)")
	assertIsError(t, sh.GetCellValue(0, 4), "SMALL(..., 0)")
	assertIsError(t, sh.GetCellValue(0, 5), "PERCENTILE(..., -0.1)")
	assertIsError(t, sh.GetCellValue(0, 6), "PERCENTILE(..., 1.1)")
	assertIsError(t, sh.GetCellValue(0, 7), "MODE(no duplicates)")
}

// 6. Formula Parsing Extremes (Consecutive Unary Operators & Deep Nesting)
func TestEdgeCases_FormulaParsingExtremes(t *testing.T) {
	sh := sheet.NewSheet()

	sh.SetCellInput(0, 0, "=--5", nil)               // A1: 5
	sh.SetCellInput(0, 1, "=-+5", nil)               // A2: -5
	sh.SetCellInput(0, 2, "=---5", nil)              // A3: -5
	sh.SetCellInput(0, 3, "=((((1+2)*3)+4)*5)", nil) // A4: ((3*3)+4)*5 = 13*5 = 65
	sh.SetCellInput(0, 4, "=1+2*3^2", nil)           // A5: 1 + 2*9 = 19 (precedence)
	sh.SetCellInput(0, 5, "=(1+2)*3", nil)           // A6: 9

	sh.Recalculate()

	if v, ok := sh.GetCellValue(0, 0).(float64); !ok || v != 5.0 {
		t.Errorf("Expected =--5 = 5, got %v", sh.GetCellValue(0, 0))
	}
	if v, ok := sh.GetCellValue(0, 1).(float64); !ok || v != -5.0 {
		t.Errorf("Expected =-+5 = -5, got %v", sh.GetCellValue(0, 1))
	}
	if v, ok := sh.GetCellValue(0, 2).(float64); !ok || v != -5.0 {
		t.Errorf("Expected =---5 = -5, got %v", sh.GetCellValue(0, 2))
	}
	if v, ok := sh.GetCellValue(0, 3).(float64); !ok || v != 65.0 {
		t.Errorf("Expected =((((1+2)*3)+4)*5) = 65, got %v", sh.GetCellValue(0, 3))
	}
	if v, ok := sh.GetCellValue(0, 4).(float64); !ok || v != 19.0 {
		t.Errorf("Expected =1+2*3^2 = 19, got %v", sh.GetCellValue(0, 4))
	}
	if v, ok := sh.GetCellValue(0, 5).(float64); !ok || v != 9.0 {
		t.Errorf("Expected =(1+2)*3 = 9, got %v", sh.GetCellValue(0, 5))
	}
}

func assertIsError(t *testing.T, val any, label string) {
	t.Helper()
	if val == nil {
		t.Errorf("[%s] Expected error, got nil", label)
		return
	}
	if errVal, ok := val.(cell.LotusError); ok {
		_ = errVal
		return
	}
	if f, ok := val.(float64); ok && (math.IsNaN(f) || math.IsInf(f, 0)) {
		return
	}
	t.Errorf("[%s] Expected error (LotusError), got %T: %v", label, val, val)
}
