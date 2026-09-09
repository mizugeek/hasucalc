package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/sheet"
	"hasucalc/tui"
)

// 1. 検索（Find / Search）の前方・後方検索、折り返し、および全シート横断検索の検証
func TestBugfixV10_FindAndSearchForwardBackwardAndCrossSheet(t *testing.T) {
	wb := sheet.NewWorkbook("find_test.hwk")
	sh1 := wb.Sheets[0]
	sh1.SetName("Sheet1")
	sh2 := wb.AddSheet("Sheet2")

	// Sheet1: A1="Apple", A5="Banana", B10="Pineapple"
	sh1.SetCellInput(0, 0, "Apple", nil)
	sh1.SetCellInput(0, 4, "Banana", nil)
	sh1.SetCellInput(1, 9, "Pineapple", nil)

	// Sheet2: C3="Crabapple"
	sh2.SetCellInput(2, 2, "Crabapple", nil)

	app := newSimApp(t, sh1, "find_test.hwk")

	// 1. Initial Find on current sheet: query "apple"
	app.SetCursorForTest(0, 0)
	app.ExecuteActionHandlerForTest("doFindPrompt", map[string]string{"query": "apple"})
	cCol, cRow := app.GetCursorForTest()
	if cCol != 0 || cRow != 0 {
		t.Fatalf("first find want A1 (0,0), got (%d,%d)", cCol, cRow)
	}

	// 2. F3: findNext -> should move to B10 (1, 9) in Sheet1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	cCol, cRow = app.GetCursorForTest()
	if cCol != 1 || cRow != 9 {
		t.Fatalf("findNext want B10 (1,9), got (%d,%d)", cCol, cRow)
	}

	// 3. F3 again: should wrap around to A1 (0, 0) in Sheet1
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModNone))
	cCol, cRow = app.GetCursorForTest()
	if cCol != 0 || cRow != 0 {
		t.Fatalf("findNext wrap want A1 (0,0), got (%d,%d)", cCol, cRow)
	}

	// 4. Shift+F3: findPrev -> should move backward to B10 (1, 9)
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF3, 0, tcell.ModShift))
	cCol, cRow = app.GetCursorForTest()
	if cCol != 1 || cRow != 9 {
		t.Fatalf("findPrev want B10 (1,9), got (%d,%d)", cCol, cRow)
	}

	// 5. Cross-sheet search: doFindAllPrompt with "Crabapple" -> should switch to Sheet2 C3 (2, 2)
	app.ExecuteActionHandlerForTest("doFindAllPrompt", map[string]string{"query": "Crabapple"})
	cCol, cRow = app.GetCursorForTest()
	if cCol != 2 || cRow != 2 {
		t.Fatalf("cross-sheet find want C3 (2,2) on Sheet2, got (%d,%d)", cCol, cRow)
	}
	if wb.GetActiveSheet().Name() != "Sheet2" {
		t.Fatalf("active sheet want Sheet2, got %s", wb.GetActiveSheet().Name())
	}

	// 6. Not found query: should set status message and keep cursor
	app.ExecuteActionHandlerForTest("doFindPrompt", map[string]string{"query": "NonExistentWordXYZ"})
	if !strings.Contains(app.GetStatusMessageForTest(), "Pattern not found") {
		t.Errorf("status message for not found want 'Pattern not found', got %q", app.GetStatusMessageForTest())
	}
}

// 2. TUI インラインセル編集（EDIT モード / handleBufferEditing）の各編集キーの検証
func TestBugfixV10_TUIInlineEditModeKeys(t *testing.T) {
	wb := sheet.NewWorkbook("edit_mode.hwk")
	sh := wb.Sheets[0]
	sh.SetCellInput(0, 0, "Hello World", nil)

	app := newSimApp(t, sh, "edit_mode.hwk")
	app.SetCursorForTest(0, 0)

	// 1. Enter EDIT mode via F2
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	if app.GetModeForTest() != "EDIT" {
		t.Fatalf("mode after F2 want EDIT, got %s", app.GetModeForTest())
	}
	if app.GetInputBufferForTest() != "'Hello World" {
		t.Fatalf("initial edit buffer want \"'Hello World\", got %q", app.GetInputBufferForTest())
	}

	// 2. KeyEnd -> KeyBackspace 5 times: "'Hello World" -> "'Hello "
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone))
	for i := 0; i < 5; i++ {
		app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone))
	}
	if app.GetInputBufferForTest() != "'Hello " {
		t.Errorf("buffer after backspace want \"'Hello \", got %q", app.GetInputBufferForTest())
	}

	// 3. Type "Go"
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'G', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, 'o', tcell.ModNone))
	if app.GetInputBufferForTest() != "'Hello Go" {
		t.Errorf("buffer after typing want \"'Hello Go\", got %q", app.GetInputBufferForTest())
	}

	// 4. Commit with KeyEnter -> should save to cell and return to READY
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
	if app.GetModeForTest() != "READY" {
		t.Errorf("mode after Enter want READY, got %s", app.GetModeForTest())
	}
	if cellRaw(t, sh, 0, 0) != "'Hello Go" {
		t.Errorf("cell value after Enter want \"'Hello Go\", got %q", cellRaw(t, sh, 0, 0))
	}

	// 5. Test Escape cancellation:
	// Start edit, change text, press Escape -> cell should retain "'Hello Go"
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyRune, '!', tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if app.GetModeForTest() != "READY" {
		t.Errorf("mode after Escape want READY, got %s", app.GetModeForTest())
	}
	if cellRaw(t, sh, 0, 0) != "'Hello Go" {
		t.Errorf("cell value after Escape should not change, got %q", cellRaw(t, sh, 0, 0))
	}

	// 6. Test KeyTab commits and moves right
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyF2, 0, tcell.ModNone))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	cCol, cRow := app.GetCursorForTest()
	if cCol != 1 || cRow != 0 {
		t.Errorf("cursor after Tab commit want (1,0), got (%d,%d)", cCol, cRow)
	}

	// 7. Test Undo restores original "'Hello World"
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	app.ProcessEventForTest(tcell.NewEventKey(tcell.KeyCtrlZ, 0, tcell.ModCtrl))
	if cellRaw(t, sh, 0, 0) != "'Hello World" {
		t.Errorf("cell value after Undo want \"'Hello World\", got %q", cellRaw(t, sh, 0, 0))
	}
}

// 3. シート設定・書式の一括操作機能（LabelAlignRange, SetGlobalFormat, ResetColWidth, DeleteNamedRange, IsModified）
func TestBugfixV10_SheetBulkFormatAndLabelAlign(t *testing.T) {
	wb := sheet.NewWorkbook("bulk_format")
	sh := wb.Sheets[0]

	// 1. LabelAlignRange: Left, Right, Center
	sh.SetCellInput(0, 0, "'Alpha", nil)
	sh.SetCellInput(0, 1, "'Beta", nil)
	sh.SetCellInput(0, 2, "'Gamma", nil)
	rng := mustParseRange(t, "A1:A3")

	// Align Right
	sh.LabelAlignRange(rng, cell.AlignRight)
	for r := 0; r < 3; r++ {
		c := sh.GetCell(0, r)
		if c.Alignment != cell.AlignRight || !strings.HasPrefix(c.RawInput, `"`) {
			t.Errorf("row %d after AlignRight want \", got align=%v raw=%q", r, c.Alignment, c.RawInput)
		}
	}

	// Align Center
	sh.LabelAlignRange(rng, cell.AlignCenter)
	for r := 0; r < 3; r++ {
		c := sh.GetCell(0, r)
		if c.Alignment != cell.AlignCenter || !strings.HasPrefix(c.RawInput, "^") {
			t.Errorf("row %d after AlignCenter want ^, got align=%v raw=%q", r, c.Alignment, c.RawInput)
		}
	}

	// 2. SetGlobalFormat: change workbook default format to Currency
	sh.SetCellInput(1, 0, "123.45", nil) // B1 has no individual format
	sh.SetGlobalFormat(cell.CellFormat{Type: cell.FmtCurrency, Decimals: 2})
	cellB1 := sh.GetCell(1, 0)
	formatted := cellB1.FormattedValue(sh.GlobalFormat())
	if !strings.Contains(formatted, "$123.45") {
		t.Errorf("GlobalFormat Currency want $123.45, got %q", formatted)
	}

	// 3. ResetColWidth
	sh.SetColWidth(0, 28)
	if sh.GetColWidth(0) != 28 {
		t.Fatalf("col 0 width want 28, got %d", sh.GetColWidth(0))
	}
	sh.ResetColWidth(0)
	if sh.GetColWidth(0) != sh.DefaultColWidth() {
		t.Errorf("after ResetColWidth col 0 want default %d, got %d", sh.DefaultColWidth(), sh.GetColWidth(0))
	}

	// 4. DeleteNamedRange
	sh.SetNamedRange("TEMPRANGE", rng)
	if _, ok := sh.GetNamedRange("TEMPRANGE"); !ok {
		t.Fatal("TEMPRANGE missing after SetNamedRange")
	}
	sh.DeleteNamedRange("TEMPRANGE")
	if _, ok := sh.GetNamedRange("TEMPRANGE"); ok {
		t.Errorf("TEMPRANGE should be gone after DeleteNamedRange")
	}

	// 5. IsModified state tracking
	if !sh.IsModified() {
		t.Errorf("sheet should be modified after mutations")
	}
	if !wb.IsModified() {
		t.Errorf("workbook should be modified after mutations")
	}
}

// 4. グラフ全タイプ（LINE, BAR, STACKED, PIE）の描画とスタンドアロン PNG 画像出力の検証
func TestBugfixV10_GraphAllTypesAndPNGExport(t *testing.T) {
	sh := sheet.NewSheet()
	// Populate X categories
	sh.SetCellInput(0, 0, "Q1", nil)
	sh.SetCellInput(0, 1, "Q2", nil)
	sh.SetCellInput(0, 2, "Q3", nil)
	sh.SetCellInput(0, 3, "Q4", nil)

	// Series A: standard positive values
	sh.SetCellInput(1, 0, "10", nil)
	sh.SetCellInput(1, 1, "25", nil)
	sh.SetCellInput(1, 2, "15", nil)
	sh.SetCellInput(1, 3, "40", nil)

	// Series B: identical values (tests zero division protection: minVal == maxVal)
	sh.SetCellInput(2, 0, "50", nil)
	sh.SetCellInput(2, 1, "50", nil)
	sh.SetCellInput(2, 2, "50", nil)
	sh.SetCellInput(2, 3, "50", nil)

	// Series C: all zeros
	sh.SetCellInput(3, 0, "0", nil)
	sh.SetCellInput(3, 1, "0", nil)
	sh.SetCellInput(3, 2, "0", nil)
	sh.SetCellInput(3, 3, "0", nil)

	rx := mustParseRange(t, "A1:A4")
	ra := mustParseRange(t, "B1:B4")
	rb := mustParseRange(t, "C1:C4")
	rc := mustParseRange(t, "D1:D4")

	sh.Graph().RangeX = &rx
	sh.Graph().Series["A"] = &ra
	sh.Graph().Series["B"] = &rb
	sh.Graph().Series["C"] = &rc

	tmpDir := t.TempDir()
	pngMagic := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

	types := []string{"LINE", "BAR", "STACKED", "PIE"}
	for _, gt := range types {
		sh.Graph().Type = gt
		outPath := filepath.Join(tmpDir, "graph_"+gt+".png")
		if err := tui.ExportGraphPNG(sh, outPath, 800, 600); err != nil {
			t.Fatalf("ExportGraphPNG for %s: %v", gt, err)
		}

		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", outPath, err)
		}
		if len(data) < 8 || !bytes.Equal(data[:8], pngMagic) {
			t.Errorf("graph %s output is not valid PNG (magic: %v)", gt, data[:8])
		}
	}
}

// 5. 多言語・タイポグラフィ（DetectScript, IsRTL, ReshapeArabic）の検証
func TestBugfixV10_TypographyScriptDetectionAndRTL(t *testing.T) {
	// 1. DetectScript
	scriptTests := []struct {
		r    rune
		want tui.ScriptType
		desc string
	}{
		{'A', tui.ScriptDefault, "Latin"},
		{'1', tui.ScriptDefault, "Digit"},
		{'あ', tui.ScriptCJK, "Japanese Hiragana"},
		{'漢', tui.ScriptCJK, "Kanji"},
		{0x0645, tui.ScriptArabic, "Arabic Meem"},
		{0x05E9, tui.ScriptHebrew, "Hebrew Shin"},
		{0x0E01, tui.ScriptThai, "Thai Ko Kai"},
		{0x0915, tui.ScriptDevanagari, "Devanagari Ka"},
	}

	for _, tc := range scriptTests {
		got := tui.DetectScript(tc.r)
		if got != tc.want {
			t.Errorf("DetectScript(%q - %s) want %v, got %v", tc.r, tc.desc, tc.want, got)
		}
	}

	// 2. IsRTL
	if tui.IsRTL('A') {
		t.Errorf("IsRTL('A') want false")
	}
	if tui.IsRTL('あ') {
		t.Errorf("IsRTL('あ') want false")
	}
	if !tui.IsRTL(0x0645) { // Arabic Meem
		t.Errorf("IsRTL(Arabic) want true")
	}
	if !tui.IsRTL(0x05E9) { // Hebrew Shin
		t.Errorf("IsRTL(Hebrew) want true")
	}

	// 3. ReshapeArabic: Lam-Alif ligature
	// \u0644 (Lam) + \u0627 (Alef) -> \uFEFB (Lam-Alif Isolated)
	lamAlef := "\u0644\u0627"
	shaped := tui.ReshapeArabic(lamAlef)
	if !strings.ContainsRune(shaped, 0xFEFB) {
		t.Errorf("ReshapeArabic(Lam+Alef) want ligature U+FEFB, got %q (runes: %U)", shaped, []rune(shaped))
	}
}

// 6. ファイル読み込みの耐障害性・異常系テスト（破損・空ファイル）の検証
func TestBugfixV10_FileParserRobustnessCorruptedFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. 0-byte files
	emptyFile := filepath.Join(tmpDir, "empty.bin")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := sheet.ImportXLSXWorkbook(emptyFile); err == nil {
		t.Errorf("ImportXLSX on empty file should return error")
	}
	if _, err := sheet.ImportODSWorkbook(emptyFile); err == nil {
		t.Errorf("ImportODS on empty file should return error")
	}
	if _, err := sheet.LoadWorkbookJSON(emptyFile); err == nil {
		t.Errorf("LoadWorkbookJSON on empty file should return error")
	}

	// 2. Corrupted plain text file (not a zip)
	corruptedZip := filepath.Join(tmpDir, "not_a_zip.xlsx")
	if err := os.WriteFile(corruptedZip, []byte("THIS IS PLAIN TEXT NOT A VALID ZIP ARCHIVE"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := sheet.ImportXLSXWorkbook(corruptedZip); err == nil {
		t.Errorf("ImportXLSX on corrupted non-zip should return error")
	}
	if _, err := sheet.ImportODSWorkbook(corruptedZip); err == nil {
		t.Errorf("ImportODS on corrupted non-zip should return error")
	}

	// 3. Corrupted JSON file
	badJSON := filepath.Join(tmpDir, "bad.hwk")
	if err := os.WriteFile(badJSON, []byte("{ name: broken json without quotes or closing brace"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := sheet.LoadWorkbookJSON(badJSON); err == nil {
		t.Errorf("LoadWorkbookJSON on invalid JSON should return error")
	}
}
