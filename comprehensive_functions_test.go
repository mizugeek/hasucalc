package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"hasucalc/cell"
	"hasucalc/formula"
	"hasucalc/sheet"
)

// TestComprehensive_AllFunctionsNoPanic tests that ALL 120 registered functions
// never panic under any combination of adversarial inputs (empty args, nil, errors, extreme values).
func TestComprehensive_AllFunctionsNoPanic(t *testing.T) {
	testInputs := [][]any{
		{},                                  // 0 args
		{nil},                               // 1 nil
		{nil, nil},                          // 2 nils
		{nil, nil, nil},                     // 3 nils
		{nil, nil, nil, nil},                // 4 nils
		{nil, nil, nil, nil, nil},           // 5 nils
		{cell.ErrNA},                        // 1 error
		{cell.ErrRef, cell.ErrNA},           // 2 errors
		{cell.ErrLotus, 10.0, "text"},       // mixed error
		{""},                                // empty string
		{"", ""},                            // empty strings
		{0.0},                               // zero
		{-1.0},                              // negative number
		{math.MaxFloat64},                   // max float
		{-math.MaxFloat64},                  // min float
		{math.Inf(1)},                       // positive infinity
		{math.Inf(-1)},                      // negative infinity
		{math.NaN()},                        // NaN
		{true},                              // boolean true
		{false},                             // boolean false
		{[][]any{}},                         // empty 2D grid
		{[][]any{{nil}}},                    // 2D grid with nil
		{[][]any{{cell.ErrNA}}},             // 2D grid with error
		{[][]any{{1.0, 2.0}, {3.0, 4.0}}},   // valid 2D grid
		{[]any{}},                           // empty 1D slice
		{[]any{nil, 1.0, "a"}},              // mixed 1D slice
		{"non_numeric_string", -999.0, nil}, // arbitrary mix
	}

	for fnName, fn := range formula.Functions {
		for i, inp := range testInputs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC in function %s with input case %d (%#v): %v", fnName, i, inp, r)
					}
				}()
				// Execute the function handler directly
				res := fn(inp)

				// Invariant: Result must not be an unsupported raw type that would break formatting
				switch res.(type) {
				case float64, int, string, bool, nil, cell.LotusError, [][]any, []any:
					// OK valid spreadsheet types
				default:
					t.Errorf("Function %s returned unexpected internal type %T (%#v)", fnName, res, res)
				}
			}()
		}
	}
}

// TestComprehensive_ErrorPropagation tests that computation functions surface NA()
// as a LotusError instead of returning 0/garbage. Inspectors and 0-arg functions are exempt.
func TestComprehensive_ErrorPropagation(t *testing.T) {
	exempt := map[string]bool{
		"@IFERROR":    true,
		"@IFNA":       true,
		"@ISERROR":    true,
		"@ISERR":      true,
		"@ISNA":       true,
		"@TYPE":       true,
		"@ERROR.TYPE": true,
		"@ISBLANK":    true,
		"@ISNUMBER":   true,
		"@ISSTRING":   true,
		"@ISTEXT":     true,
		"@ISNONTEXT":  true,
		"@ISLOGICAL":  true,
		"@ISEVEN":     true,
		"@ISODD":      true,
		"@COUNT":      true, // skips non-numbers
		"@COUNTA":     true, // counts errors as non-blank
		"@COUNTBLANK": true,
		"@RAND":       true,
		"@PI":         true,
		"@TRUE":       true,
		"@FALSE":      true,
		"@TODAY":      true,
		"@NOW":        true,
		"@NA":         true,
		"@ERR":        true,
	}

	names := make([]string, 0, len(formula.Functions))
	for n := range formula.Functions {
		names = append(names, n)
	}
	sort.Strings(names)

	sh := sheet.NewSheet()
	for _, fnName := range names {
		if exempt[fnName] {
			continue
		}
		name := strings.TrimPrefix(fnName, "@")
		forms := []string{
			fmt.Sprintf("=%s(NA())", name),
			fmt.Sprintf("=%s(NA(),1)", name),
			fmt.Sprintf("=%s(NA(),1,1)", name),
			fmt.Sprintf("=%s(NA(),1,1,1)", name),
		}
		propagated := false
		var last any
		for _, f := range forms {
			sh.SetCellInput(0, 0, f, nil)
			sh.Recalculate()
			last = sh.GetCellValue(0, 0)
			if _, ok := last.(cell.LotusError); ok {
				propagated = true
				break
			}
		}
		if !propagated {
			t.Errorf("%s did not propagate NA() as an error (last=%#v)", fnName, last)
		}
	}
}

// TestComprehensive_LookupEdgeCases tests edge cases in VLOOKUP, HLOOKUP, XLOOKUP, INDEX, MATCH
func TestComprehensive_LookupEdgeCases(t *testing.T) {
	s := sheet.NewSheet()

	// Setup table data at A1:C5
	// Row 0 (A1:C1): "Alice", 100, "HR"
	// Row 1 (A2:C2): "Bob", 200, "IT"
	// Row 2 (A3:C3): "Charlie", 300, "Finance"
	// Row 3 (A4:C4): "Dave", 400, "Legal"
	// Row 4 (A5:C5): "", nil, "" (empty row)
	s.SetCellInput(0, 0, "Alice", nil)
	s.SetCellInput(1, 0, "100", nil)
	s.SetCellInput(2, 0, "HR", nil)

	s.SetCellInput(0, 1, "Bob", nil)
	s.SetCellInput(1, 1, "200", nil)
	s.SetCellInput(2, 1, "IT", nil)

	s.SetCellInput(0, 2, "Charlie", nil)
	s.SetCellInput(1, 2, "300", nil)
	s.SetCellInput(2, 2, "Finance", nil)

	s.SetCellInput(0, 3, "Dave", nil)
	s.SetCellInput(1, 3, "400", nil)
	s.SetCellInput(2, 3, "Legal", nil)

	// VLOOKUP Exact match not found -> #N/A!
	s.SetCellInput(3, 0, `=VLOOKUP("Unknown", A1:C4, 2, FALSE)`, nil) // D1
	// VLOOKUP Column index 0 -> #VALUE! or #REF!
	s.SetCellInput(3, 1, `=VLOOKUP("Bob", A1:C4, 0, FALSE)`, nil) // D2
	// VLOOKUP Column index greater than cols -> #REF!
	s.SetCellInput(3, 2, `=VLOOKUP("Bob", A1:C4, 5, FALSE)`, nil) // D3
	// VLOOKUP Exact match case-insensitive
	s.SetCellInput(3, 3, `=VLOOKUP("charlie", A1:C4, 2, FALSE)`, nil) // D4 -> 300
	// INDEX out of bounds row -> Error
	s.SetCellInput(3, 4, `=INDEX(A1:C4, 10, 1)`, nil) // D5
	// INDEX out of bounds col -> Error
	s.SetCellInput(3, 5, `=INDEX(A1:C4, 1, 10)`, nil) // D6
	// MATCH exact match not found -> #N/A!
	s.SetCellInput(3, 6, `=MATCH("Unknown", A1:A4, 0)`, nil) // D7
	// MATCH exact match case-insensitive -> 2
	s.SetCellInput(3, 7, `=MATCH("bob", A1:A4, 0)`, nil) // D8 -> 2

	s.Recalculate()

	// Check D1: #N/A!
	d1 := s.GetCellValue(3, 0)
	if _, ok := d1.(cell.LotusError); !ok {
		t.Fatalf("D1 VLOOKUP not found want LotusError, got %#v", d1)
	}

	// Check D2: col 0 want Error
	d2 := s.GetCellValue(3, 1)
	if _, ok := d2.(cell.LotusError); !ok {
		t.Fatalf("D2 VLOOKUP col 0 want LotusError, got %#v", d2)
	}

	// Check D3: col 5 want Error
	d3 := s.GetCellValue(3, 2)
	if _, ok := d3.(cell.LotusError); !ok {
		t.Fatalf("D3 VLOOKUP col > table want LotusError, got %#v", d3)
	}

	// Check D4: 300
	d4 := s.GetCellValue(3, 3)
	if v, ok := d4.(float64); !ok || v != 300 {
		t.Fatalf("D4 VLOOKUP case-insensitive want 300, got %#v", d4)
	}

	// Check D5: INDEX row out of bounds
	d5 := s.GetCellValue(3, 4)
	if _, ok := d5.(cell.LotusError); !ok {
		t.Fatalf("D5 INDEX row out of bounds want LotusError, got %#v", d5)
	}

	// Check D6: INDEX col out of bounds
	d6 := s.GetCellValue(3, 5)
	if _, ok := d6.(cell.LotusError); !ok {
		t.Fatalf("D6 INDEX col out of bounds want LotusError, got %#v", d6)
	}

	// Check D7: MATCH not found
	d7 := s.GetCellValue(3, 6)
	if _, ok := d7.(cell.LotusError); !ok {
		t.Fatalf("D7 MATCH not found want LotusError, got %#v", d7)
	}

	// Check D8: MATCH "bob" -> 2
	d8 := s.GetCellValue(3, 7)
	if v, ok := d8.(float64); !ok || v != 2 {
		t.Fatalf("D8 MATCH \"bob\" want 2, got %#v", d8)
	}
}

// TestComprehensive_FinancialEdgeCases tests IRR, PMT, NPV, SLN, DDB with invalid and exact values.
func TestComprehensive_FinancialEdgeCases(t *testing.T) {
	s := sheet.NewSheet()

	s.SetCellInput(0, 0, "100", nil)
	s.SetCellInput(0, 1, "200", nil)
	s.SetCellInput(0, 2, "300", nil)
	s.SetCellInput(1, 0, "=IRR(A1:A3)", nil)           // B1 no sign change
	s.SetCellInput(1, 1, "=PMT(0.05, 0, 1000)", nil)   // B2 nper=0
	s.SetCellInput(1, 2, "=SLN(1000, 100, 0)", nil)    // B3 life=0
	s.SetCellInput(1, 3, "=DDB(1000, 100, 0, 1)", nil) // B4 life=0
	s.SetCellInput(1, 4, "=DDB(1000, 100, 5, 0)", nil) // B5 period=0
	s.SetCellInput(1, 5, "=SLN(1000, 100, 5)", nil)    // B6 180
	s.SetCellInput(1, 6, "=PMT(0.1, 2, 1000)", nil)    // B7 -(0.1*1000*1.21)/(1.21-1)
	s.SetCellInput(1, 7, "=NPV(0.1, 100, 200)", nil)   // B8 100/1.1 + 200/1.21
	s.Recalculate()

	for row, desc := range []string{"IRR(no sign change)", "PMT(nper=0)", "SLN(life=0)", "DDB(life=0)", "DDB(period=0)"} {
		val := s.GetCellValue(1, row)
		if _, ok := val.(cell.LotusError); !ok {
			t.Errorf("Financial edge case %s want LotusError, got %#v", desc, val)
		}
	}

	if v, ok := s.GetCellValue(1, 5).(float64); !ok || v != 180 {
		t.Errorf("SLN(1000,100,5) want 180, got %#v", s.GetCellValue(1, 5))
	}
	wantPMT := -(0.1 * (1000 * math.Pow(1.1, 2))) / (math.Pow(1.1, 2) - 1)
	if v, ok := s.GetCellValue(1, 6).(float64); !ok || math.Abs(v-wantPMT) > 1e-9 {
		t.Errorf("PMT(0.1,2,1000) want %v, got %#v", wantPMT, s.GetCellValue(1, 6))
	}
	wantNPV := 100/1.1 + 200/1.21
	if v, ok := s.GetCellValue(1, 7).(float64); !ok || math.Abs(v-wantNPV) > 1e-9 {
		t.Errorf("NPV(0.1,100,200) want %v, got %#v", wantNPV, s.GetCellValue(1, 7))
	}
}

// TestComprehensive_DateTimeEdgeCases tests DATE, DATEDIF, WORKDAY, NETWORKDAYS serials.
func TestComprehensive_DateTimeEdgeCases(t *testing.T) {
	s := sheet.NewSheet()

	s.SetCellInput(0, 0, `=DATEDIF(DATE(2023, 12, 1), DATE(2023, 1, 1), "D")`, nil)   // A1 error
	s.SetCellInput(0, 1, `=DATEDIF(DATE(2023, 1, 1), DATE(2023, 12, 1), "XYZ")`, nil) // A2 error
	s.SetCellInput(0, 2, `=DATE(2023, 13, 1)`, nil)                                   // A3 = 2024-01-01
	s.SetCellInput(0, 3, `=DATE(2024, 1, 1)`, nil)                                    // A4
	s.SetCellInput(0, 4, `=WORKDAY(DATE(2023, 1, 2), 0)`, nil)                        // A5 same Monday
	s.SetCellInput(0, 5, `=DATE(2023, 1, 2)`, nil)                                    // A6
	s.SetCellInput(0, 6, `=YEAR(DATE(2023, 13, 1))`, nil)                             // A7 2024
	s.SetCellInput(0, 7, `=MONTH(DATE(2023, 13, 1))`, nil)                            // A8 1
	s.SetCellInput(0, 8, `=DAY(DATE(2023, 13, 1))`, nil)                              // A9 1
	s.SetCellInput(0, 9, `=NETWORKDAYS(DATE(2023, 1, 2), DATE(2023, 1, 6))`, nil)     // A10 Mon-Fri = 5
	s.SetCellInput(0, 10, `=DATEDIF(DATE(2023, 1, 1), DATE(2023, 12, 1), "M")`, nil)  // A11 11
	s.SetCellInput(0, 11, `=DATEDIF(DATE(2023, 1, 1), DATE(2024, 1, 1), "Y")`, nil)   // A12 1
	s.SetCellInput(0, 12, `=DATEDIF(DATE(2023, 1, 1), DATE(2023, 1, 11), "D")`, nil)  // A13 10

	s.Recalculate()

	if _, ok := s.GetCellValue(0, 0).(cell.LotusError); !ok {
		t.Errorf("DATEDIF start > end want LotusError, got %#v", s.GetCellValue(0, 0))
	}
	if _, ok := s.GetCellValue(0, 1).(cell.LotusError); !ok {
		t.Errorf("DATEDIF invalid unit want LotusError, got %#v", s.GetCellValue(0, 1))
	}

	rolled, ok1 := s.GetCellValue(0, 2).(float64)
	canonical, ok2 := s.GetCellValue(0, 3).(float64)
	if !ok1 || !ok2 || rolled != canonical {
		t.Errorf("DATE(2023,13,1)=%#v want DATE(2024,1,1)=%#v", s.GetCellValue(0, 2), s.GetCellValue(0, 3))
	}
	if ok2 && canonical != 45292 {
		t.Errorf("DATE(2024,1,1) serial want 45292, got %v", canonical)
	}

	wd, ok3 := s.GetCellValue(0, 4).(float64)
	mon, ok4 := s.GetCellValue(0, 5).(float64)
	if !ok3 || !ok4 || wd != mon {
		t.Errorf("WORKDAY(DATE(2023,1,2),0)=%#v want DATE(2023,1,2)=%#v", s.GetCellValue(0, 4), s.GetCellValue(0, 5))
	}

	if v, ok := s.GetCellValue(0, 6).(float64); !ok || v != 2024 {
		t.Errorf("YEAR(DATE(2023,13,1)) want 2024, got %#v", s.GetCellValue(0, 6))
	}
	if v, ok := s.GetCellValue(0, 7).(float64); !ok || v != 1 {
		t.Errorf("MONTH(DATE(2023,13,1)) want 1, got %#v", s.GetCellValue(0, 7))
	}
	if v, ok := s.GetCellValue(0, 8).(float64); !ok || v != 1 {
		t.Errorf("DAY(DATE(2023,13,1)) want 1, got %#v", s.GetCellValue(0, 8))
	}
	if v, ok := s.GetCellValue(0, 9).(float64); !ok || v != 5 {
		t.Errorf("NETWORKDAYS Mon-Fri want 5, got %#v", s.GetCellValue(0, 9))
	}
	if v, ok := s.GetCellValue(0, 10).(float64); !ok || v != 11 {
		t.Errorf("DATEDIF M Jan-Dec want 11, got %#v", s.GetCellValue(0, 10))
	}
	if v, ok := s.GetCellValue(0, 11).(float64); !ok || v != 1 {
		t.Errorf("DATEDIF Y 2023-2024 want 1, got %#v", s.GetCellValue(0, 11))
	}
	if v, ok := s.GetCellValue(0, 12).(float64); !ok || v != 10 {
		t.Errorf("DATEDIF D 10 days want 10, got %#v", s.GetCellValue(0, 12))
	}
}

// TestComprehensive_StringEdgeCases tests string functions with out-of-range bounds
func TestComprehensive_StringEdgeCases(t *testing.T) {
	s := sheet.NewSheet()

	// MID with start_num <= 0 -> Error
	s.SetCellInput(0, 0, `=MID("hello", 0, 2)`, nil) // A1
	// MID with length < 0 -> Error
	s.SetCellInput(0, 1, `=MID("hello", 1, -1)`, nil) // A2
	// LEFT with length < 0 -> Error
	s.SetCellInput(0, 2, `=LEFT("hello", -1)`, nil) // A3
	// RIGHT with length < 0 -> Error
	s.SetCellInput(0, 3, `=RIGHT("hello", -1)`, nil) // A4
	// FIND string not found -> Error
	s.SetCellInput(0, 4, `=FIND("xyz", "hello")`, nil) // A5
	// SEARCH string not found -> Error
	s.SetCellInput(0, 5, `=SEARCH("xyz", "hello")`, nil) // A6
	// SUBSTITUTE instance_num <= 0 -> Error
	s.SetCellInput(0, 6, `=SUBSTITUTE("abab", "a", "x", 0)`, nil) // A7

	s.Recalculate()

	testCases := []string{
		"MID(start=0)",
		"MID(len=-1)",
		"LEFT(len=-1)",
		"RIGHT(len=-1)",
		"FIND(not found)",
		"SEARCH(not found)",
		"SUBSTITUTE(instance=0)",
	}

	for row, desc := range testCases {
		v := s.GetCellValue(0, row)
		if _, ok := v.(cell.LotusError); !ok {
			t.Errorf("String edge case %s want LotusError, got %#v", desc, v)
		}
	}
}

// TestComprehensive_CrossSheetCircularRefDetection tests that cross-sheet circular references
// cleanly return ErrCirc without crashing, panicking, or hanging in an infinite loop.
func TestComprehensive_CrossSheetCircularRefDetection(t *testing.T) {
	wb := sheet.NewWorkbook("circ_test.hwk")
	sh1 := wb.Sheets[0]
	sh1.SetName("Sheet1")
	sh2 := wb.AddSheet("Sheet2")

	sh1.SetCellInput(0, 0, "=Sheet2!A1 + 1", nil)
	sh2.SetCellInput(0, 0, "=Sheet1!A1 + 1", nil)

	wb.RecalculateAll()

	val1 := sh1.GetCellValue(0, 0)
	val2 := sh2.GetCellValue(0, 0)

	err1, ok1 := val1.(cell.LotusError)
	err2, ok2 := val2.(cell.LotusError)

	if !ok1 || err1.Code != "CIRCULAR REF" {
		t.Fatalf("Sheet1!A1 want CIRCULAR REF, got %#v", val1)
	}
	if !ok2 || err2.Code != "CIRCULAR REF" {
		t.Fatalf("Sheet2!A1 want CIRCULAR REF, got %#v", val2)
	}
}

// TestComprehensive_XLookupEdgeCases tests XLOOKUP with not_found defaults and search modes
func TestComprehensive_XLookupEdgeCases(t *testing.T) {
	s := sheet.NewSheet()

	// Table: A1:B3 -> [10, "First"], [20, "Second"], [10, "Third"]
	s.SetCellInput(0, 0, "10", nil)
	s.SetCellInput(1, 0, "First", nil)
	s.SetCellInput(0, 1, "20", nil)
	s.SetCellInput(1, 1, "Second", nil)
	s.SetCellInput(0, 2, "10", nil)
	s.SetCellInput(1, 2, "Third", nil)

	// XLOOKUP with match not found and custom default
	s.SetCellInput(2, 0, `=XLOOKUP(99, A1:A3, B1:B3, "NotFound")`, nil) // C1

	// XLOOKUP default first-to-last match for 10 -> "First"
	s.SetCellInput(2, 1, `=XLOOKUP(10, A1:A3, B1:B3)`, nil) // C2

	// XLOOKUP last-to-first match (search_mode = -1) for 10 -> "Third"
	s.SetCellInput(2, 2, `=XLOOKUP(10, A1:A3, B1:B3, "NotFound", 0, -1)`, nil) // C3

	s.Recalculate()

	c1 := s.GetCellValue(2, 0)
	if c1 != "NotFound" {
		t.Fatalf("XLOOKUP custom not found want \"NotFound\", got %#v", c1)
	}

	c2 := s.GetCellValue(2, 1)
	if c2 != "First" {
		t.Fatalf("XLOOKUP first match want \"First\", got %#v", c2)
	}

	c3 := s.GetCellValue(2, 2)
	if c3 != "Third" {
		t.Fatalf("XLOOKUP last-to-first want \"Third\", got %#v", c3)
	}
}
