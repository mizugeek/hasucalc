package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"hasucalc/coord"
	"hasucalc/sheet"
	"hasucalc/tui"
)

// TestBugfixV6_CellSourcedLeakage tests that cellSourced is not leaked into cell.Value or formatting
func TestBugfixV6_CellSourcedLeakage(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "10", nil) // A1 = 10
	s.SetCellInput(1, 0, "=IF(1=1, A1)", nil) // B1
	s.SetCellInput(2, 0, "=CHOOSE(1, A1)", nil) // C1
	s.SetCellInput(3, 0, "=SWITCH(1, 1, A1, 99)", nil) // D1
	s.SetCellInput(4, 0, "=IFS(1=1, A1)", nil) // E1
	s.Recalculate()

	for col, colName := range []string{"B", "C", "D", "E"} {
		c := s.GetCell(col+1, 0)
		if c == nil {
			t.Fatalf("cell %s1 is nil", colName)
		}
		if v, ok := c.Value.(float64); !ok || v != 10 {
			t.Fatalf("cell %s1 value want 10 (float64), got %#v", colName, c.Value)
		}
		formatted := c.FormattedValue(s.GlobalFormat())
		if formatted != "10" {
			t.Fatalf("cell %s1 formatted want \"10\", got %q", colName, formatted)
		}
	}
}

// TestBugfixV6_EmptyCellComparison tests empty cell comparison semantics vs numbers and text
func TestBugfixV6_EmptyCellComparison(t *testing.T) {
	s := sheet.NewSheet()
	// A1 is empty (nil)
	s.SetCellInput(1, 0, "=A1 < 5", nil) // B1: 0 < 5 is true
	s.SetCellInput(1, 1, "=A1 > 5", nil) // B2: 0 > 5 is false
	s.SetCellInput(1, 2, "=A1 = 0", nil) // B3: empty == 0 is true
	s.SetCellInput(1, 3, "=A1 < -5", nil) // B4: 0 < -5 is false
	s.SetCellInput(1, 4, "=A1 > -5", nil) // B5: 0 > -5 is true
	s.SetCellInput(1, 5, "=A1 < \"hello\"", nil) // B6: number < text is true
	s.SetCellInput(1, 6, "=A1 > \"hello\"", nil) // B7: false
	s.SetCellInput(1, 7, "=\"\" = FALSE", nil) // B8: "" = FALSE is false in Excel/Lotus
	s.SetCellInput(1, 8, "=A1 = FALSE", nil) // B9: blank = FALSE is true
	s.SetCellInput(1, 9, "=\"\" = 0", nil) // B10: "" = 0 is false
	s.Recalculate()

	testCases := []struct {
		row  int
		desc string
		want bool
	}{
		{0, "=A1 < 5 (blank < 5)", true},
		{1, "=A1 > 5 (blank > 5)", false},
		{2, "=A1 = 0 (blank = 0)", true},
		{3, "=A1 < -5 (blank < -5)", false},
		{4, "=A1 > -5 (blank > -5)", true},
		{5, "=A1 < \"hello\" (blank < text)", true},
		{6, "=A1 > \"hello\" (blank > text)", false},
		{7, "=\"\" = FALSE", false},
		{8, "=A1 = FALSE (blank = FALSE)", true},
		{9, "=\"\" = 0", false},
	}

	for _, tc := range testCases {
		c := s.GetCell(1, tc.row)
		if c == nil {
			t.Fatalf("row %d (%s) cell is nil", tc.row+1, tc.desc)
		}
		if b, ok := c.Value.(bool); !ok || b != tc.want {
			t.Fatalf("%s want %v, got %#v", tc.desc, tc.want, c.Value)
		}
	}
}

// TestBugfixV6_IsTruthyAndConcatWithCellSourced tests isTruthy and & operator on cell references
func TestBugfixV6_IsTruthyAndConcatWithCellSourced(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(0, 0, "hello", nil) // A1 = "hello"
	s.SetCellInput(1, 0, "=IF(A1, \"yes\", \"no\")", nil) // B1: non-empty string cell is truthy
	s.SetCellInput(0, 1, "123", nil) // A2 = 123
	s.SetCellInput(1, 1, "=A2 & \"xyz\"", nil) // B2: "123xyz"
	s.SetCellInput(1, 2, "=\"abc\" & A2", nil) // B3: "abc123"
	s.Recalculate()

	if v, ok := s.GetCellValue(1, 0).(string); !ok || v != "yes" {
		t.Fatalf("B1 = IF(A1, \"yes\", \"no\") want \"yes\", got %#v", s.GetCellValue(1, 0))
	}
	if v, ok := s.GetCellValue(1, 1).(string); !ok || v != "123xyz" {
		t.Fatalf("B2 = A2 & \"xyz\" want \"123xyz\", got %#v", s.GetCellValue(1, 1))
	}
	if v, ok := s.GetCellValue(1, 2).(string); !ok || v != "abc123" {
		t.Fatalf("B3 = \"abc\" & A2 want \"abc123\", got %#v", s.GetCellValue(1, 2))
	}
}

// TestBugfixV6_MultiSheetNamedRangeShift tests that row/col shifts on Sheet1 do not corrupt Sheet2 named ranges
func TestBugfixV6_MultiSheetNamedRangeShift(t *testing.T) {
	wb := sheet.NewWorkbook("multisheet_names")
	sh1 := wb.Sheets[0] // Sheet1
	sh2 := wb.AddSheet("Sheet2")

	// Sheet1 named range
	sh1Ref, err := coord.ParseRangeRef("Sheet1!A1:A5")
	if err != nil {
		t.Fatal(err)
	}
	wb.SetNamedRange("SALES_SH1", sh1Ref)

	// Sheet2 named range in workbook
	sh2Ref, err := coord.ParseRangeRef("Sheet2!A1:A5")
	if err != nil {
		t.Fatal(err)
	}
	wb.SetNamedRange("SALES_SH2", sh2Ref)

	// Sheet2 local named range
	localRef, err := coord.ParseRangeRef("A1:A5")
	if err != nil {
		t.Fatal(err)
	}
	sh2.SetNamedRange("LOCAL_SH2", localRef)

	// Insert row on Sheet1 at row 2
	sh1.InsertRow(2, 2)

	// SALES_SH1 should have expanded to A1:A7 on Sheet1
	v1, ok := wb.GetNamedRange("SALES_SH1")
	if !ok {
		t.Fatal("SALES_SH1 missing")
	}
	rr1, ok := v1.(coord.RangeRef)
	if !ok || rr1.MinRow() != 0 || rr1.MaxRow() != 6 {
		t.Fatalf("SALES_SH1 after insert on Sheet1 want A1:A7, got %#v", v1)
	}

	// SALES_SH2 on Sheet2 must NOT have changed!
	v2, ok := wb.GetNamedRange("SALES_SH2")
	if !ok {
		t.Fatal("SALES_SH2 missing")
	}
	rr2, ok := v2.(coord.RangeRef)
	if !ok || rr2.MinRow() != 0 || rr2.MaxRow() != 4 {
		t.Fatalf("SALES_SH2 corrupted by Sheet1 insert! Got %#v, want A1:A5", v2)
	}

	// LOCAL_SH2 on Sheet2 must NOT have changed!
	v3, ok := sh2.GetNamedRange("LOCAL_SH2")
	if !ok {
		t.Fatal("LOCAL_SH2 missing")
	}
	rr3, ok := v3.(coord.RangeRef)
	if !ok || rr3.MinRow() != 0 || rr3.MaxRow() != 4 {
		t.Fatalf("LOCAL_SH2 corrupted by Sheet1 insert! Got %#v, want A1:A5", v3)
	}

	// Now delete row 0 on Sheet1
	sh1.DeleteRow(0, 1)

	// SALES_SH2 and LOCAL_SH2 on Sheet2 must STILL be A1:A5!
	v2, _ = wb.GetNamedRange("SALES_SH2")
	rr2 = v2.(coord.RangeRef)
	if rr2.MinRow() != 0 || rr2.MaxRow() != 4 {
		t.Fatalf("SALES_SH2 corrupted by Sheet1 delete! Got %#v, want A1:A5", v2)
	}

	v3, _ = sh2.GetNamedRange("LOCAL_SH2")
	rr3 = v3.(coord.RangeRef)
	if rr3.MinRow() != 0 || rr3.MaxRow() != 4 {
		t.Fatalf("LOCAL_SH2 corrupted by Sheet1 delete! Got %#v, want A1:A5", v3)
	}
}

// TestBugfixV6_MoveRangeRetargetingFracture tests that moving only part of a range does NOT stretch range references
func TestBugfixV6_MoveRangeRetargetingFracture(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(2, 0, "=SUM(A1:A10)", nil) // C1

	// Move only A1 to Z100
	fromR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 0}}
	s.RetargetFormulasAfterMove(fromR, 25, 99)

	// C1 must NOT be stretched to Z100:A10!
	raw := s.GetCell(2, 0).RawInput
	if raw != "=SUM(A1:A10)" {
		t.Fatalf("Formula was fractured by partial range move! Got %q, want =SUM(A1:A10)", raw)
	}

	// Move ENTIRE range A1:A10 to B1:B10
	entireR := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 0, Row: 9}}
	s.RetargetFormulasAfterMove(entireR, 1, 0)

	raw = s.GetCell(2, 0).RawInput
	if raw != "=SUM(B1:B10)" {
		t.Fatalf("Entire range move did not retarget! Got %q, want =SUM(B1:B10)", raw)
	}
}

// TestBugfixV6_ReverseRangeShrinkOnDelete tests that reverse ranges (A5:A1) do not become #REF! when deleting an end
func TestBugfixV6_ReverseRangeShrinkOnDelete(t *testing.T) {
	s := sheet.NewSheet()
	s.SetCellInput(2, 9, "=SUM(A5:A1)", nil) // C10
	s.DeleteRow(0, 1) // delete row 0 (A1)

	cellC9 := s.GetCell(2, 8)
	if cellC9 == nil {
		t.Fatal("C9 is nil after deleting row 0")
	}
	raw := cellC9.RawInput
	if strings.Contains(raw, "#REF!") {
		t.Fatalf("Reverse range A5:A1 became #REF! after deleting row 1: %q", raw)
	}
	if raw != "=SUM(A4:A1)" && raw != "=SUM(A1:A4)" {
		t.Fatalf("Reverse range after deleting row 1 want A4:A1, got %q", raw)
	}
}

// TestBugfixV6_CrossSheetCutPasteDoesNotCorruptLocalRefs tests that cutting on Sheet1 and pasting on Sheet2 doesn't shift Sheet2 formulas
func TestBugfixV6_CrossSheetCutPasteDoesNotCorruptLocalRefs(t *testing.T) {
	simScreen := tcell.NewSimulationScreen("")
	if err := simScreen.Init(); err != nil {
		t.Fatalf("init screen: %v", err)
	}
	defer simScreen.Fini()
	simScreen.SetSize(80, 24)

	wb := sheet.NewWorkbook("test_cross_cut")
	sh1 := wb.Sheets[0] // Sheet1
	sh2 := wb.AddSheet("Sheet2")

	app := tui.NewApp(simScreen, sh1, "test.hwk")
	app.RunOnceForTest()

	// Sheet1 has data at A1:B1
	sh1.SetCellInput(0, 0, "10", nil)
	sh1.SetCellInput(1, 0, "=A1*2", nil)

	// Sheet2 has formula at C1 pointing to A1
	sh2.SetCellInput(2, 0, "=A1", nil)
	wb.RecalculateAll()

	// Switch to Sheet1, select A1:B1 and cut
	app.SwitchSheetForTest(0)
	cutRange := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 0}, End: coord.CellRef{Col: 1, Row: 0}}
	app.SelectRangeForTest(&cutRange)
	app.SetCursorForTest(0, 0)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModCtrl))

	// Switch to Sheet2, move cursor to D1 and paste
	app.SwitchSheetForTest(1)
	app.SetCursorForTest(3, 0) // D1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlV, 0, tcell.ModCtrl))

	// Verify that Sheet2 C1 was NOT shifted by Sheet1's move
	sh2C1 := sh2.GetCell(2, 0)
	if sh2C1 == nil || sh2C1.RawInput != "=A1" {
		t.Fatalf("Sheet2 C1 formula was corrupted by cross-sheet paste! Got %q, want =A1", sh2C1.RawInput)
	}
}

// TestBugfixV6_DeterministicODSNamedExpressions verifies that ODS named expressions are written in sorted deterministic order
func TestBugfixV6_DeterministicODSNamedExpressions(t *testing.T) {
	wb := sheet.NewWorkbook("deterministic_ods")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "1", nil)

	// Add multiple named ranges
	names := []string{"ZEBRA", "ALPHA", "MANGO", "BETA", "CHARLIE", "DELTA"}
	for i, name := range names {
		wb.SetNamedRange(name, coord.CellRef{Col: i, Row: 0})
	}

	tmpDir, err := os.MkdirTemp("", "ods_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "out1.ods")
	file2 := filepath.Join(tmpDir, "out2.ods")

	if err := wb.ExportODS(file1); err != nil {
		t.Fatal(err)
	}
	if err := wb.ExportODS(file2); err != nil {
		t.Fatal(err)
	}

	readContentXML := func(path string) string {
		r, err := zip.OpenReader(path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		for _, f := range r.File {
			if f.Name == "content.xml" {
				rc, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				defer rc.Close()
				var buf bytes.Buffer
				io.Copy(&buf, rc)
				return buf.String()
			}
		}
		t.Fatal("content.xml not found")
		return ""
	}

	c1 := readContentXML(file1)
	c2 := readContentXML(file2)

	if c1 != c2 {
		t.Fatal("ODS content.xml is non-deterministic between exports!")
	}

	// Verify order in content.xml has ALPHA before ZEBRA
	alphaIdx := strings.Index(c1, `name="ALPHA"`)
	zebraIdx := strings.Index(c1, `name="ZEBRA"`)
	if alphaIdx == -1 || zebraIdx == -1 {
		t.Fatal("ALPHA or ZEBRA missing in content.xml")
	}
	if alphaIdx > zebraIdx {
		t.Fatalf("Named expressions in ODS are not sorted: ALPHA is at %d, ZEBRA is at %d", alphaIdx, zebraIdx)
	}
}
