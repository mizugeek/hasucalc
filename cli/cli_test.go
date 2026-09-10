package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hasucalc/cell"
	"hasucalc/cli"
	"hasucalc/coord"
	"hasucalc/sheet"
)

func createTestWorkbook(t *testing.T) (*sheet.Workbook, string) {
	t.Helper()
	wb := sheet.NewWorkbook("TestWB")
	sh1 := wb.GetActiveSheet()
	sh1.SetName("Sales")

	sh1.SetCellInput(0, 0, "'Region", nil)
	sh1.SetCellInput(1, 0, "'Q1", nil)
	sh1.SetCellInput(2, 0, "'Q2", nil)

	sh1.SetCellInput(0, 1, "'East", nil)
	sh1.SetCellInput(1, 1, "100", nil)
	sh1.SetCellInput(2, 1, "150", nil)

	sh1.SetCellInput(0, 2, "'West", nil)
	sh1.SetCellInput(1, 2, "200", nil)
	sh1.SetCellInput(2, 2, "250", nil)

	sh1.SetCellInput(0, 3, "'Total", nil)
	sh1.SetCellInput(1, 3, "=SUM(B2:B3)", nil)
	sh1.SetCellInput(2, 3, "=SUM(C2:C3)", nil)

	sh1.Graph().Type = "BAR"
	sh1.Graph().Title = "Quarterly Sales"
	rx := coord.RangeRef{Start: coord.CellRef{Col: 0, Row: 1}, End: coord.CellRef{Col: 0, Row: 2}}
	ra := coord.RangeRef{Start: coord.CellRef{Col: 1, Row: 1}, End: coord.CellRef{Col: 1, Row: 2}}
	sh1.Graph().RangeX = &rx
	sh1.Graph().Series["A"] = &ra

	sh2 := wb.AddSheet("Summary")
	sh2.SetCellInput(0, 0, "'GrandTotal", nil)
	sh2.SetCellInput(0, 1, "700", nil)

	wb.RecalculateAll()

	tmpDir := t.TempDir()
	hwkPath := filepath.Join(tmpDir, "test_sales.hwk")
	if err := wb.SaveJSON(hwkPath); err != nil {
		t.Fatalf("failed to save test workbook: %v", err)
	}

	return wb, hwkPath
}

func captureStdout(fn func() int) (int, string) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return code, buf.String()
}

func captureStderr(fn func() int) (int, string) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	code := fn()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return code, buf.String()
}

func captureAll(fn func() int) (int, string, string) {
	oldStdout, oldStderr := os.Stdout, os.Stderr
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout, os.Stderr = wOut, wErr

	code := fn()

	wOut.Close()
	wErr.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr

	var bufOut, bufErr bytes.Buffer
	io.Copy(&bufOut, rOut)
	io.Copy(&bufErr, rErr)
	return code, bufOut.String(), bufErr.String()
}

// ------------------- Convert Tests -------------------

func TestConvert_FileToFile(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	tmpDir := t.TempDir()

	// Convert HWK to CSV
	csvOut := filepath.Join(tmpDir, "out.csv")
	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"convert", hwkPath, "-o", csvOut, "-s", "Sales"})
	})
	if code != 0 {
		t.Fatalf("convert to csv failed: code=%d, stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got: %s", stdout)
	}
	csvContent, err := os.ReadFile(csvOut)
	if err != nil {
		t.Fatalf("failed to read generated csv: %v", err)
	}
	if !strings.Contains(string(csvContent), "Region,Q1,Q2") && !strings.Contains(string(csvContent), "East,100,150") {
		t.Errorf("unexpected csv content: %s", string(csvContent))
	}

	// Convert CSV to Markdown
	mdOut := filepath.Join(tmpDir, "out.md")
	code = cli.Run([]string{"convert", csvOut, "-o", mdOut})
	if code != 0 {
		t.Fatalf("convert to md failed: code=%d", code)
	}
	mdContent, _ := os.ReadFile(mdOut)
	if !strings.Contains(string(mdContent), "| Region") || !strings.Contains(string(mdContent), "Q1") {
		t.Errorf("unexpected md content: %s", string(mdContent))
	}

	// Convert to XLSX
	xlsxOut := filepath.Join(tmpDir, "out.xlsx")
	code = cli.Run([]string{"convert", hwkPath, "-o", xlsxOut})
	if code != 0 {
		t.Fatalf("convert to xlsx failed: code=%d", code)
	}
	if _, err := os.Stat(xlsxOut); err != nil {
		t.Fatalf("xlsx file was not generated: %v", err)
	}

	// Convert to ODS
	odsOut := filepath.Join(tmpDir, "out.ods")
	code = cli.Run([]string{"convert", hwkPath, "-o", odsOut})
	if code != 0 {
		t.Fatalf("convert to ods failed: code=%d", code)
	}
	if _, err := os.Stat(odsOut); err != nil {
		t.Fatalf("ods file was not generated: %v", err)
	}
}

func TestConvert_StdoutPipeline(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	// Stream HWK to Markdown on stdout
	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"convert", hwkPath, "--to", "md", "-s", "Sales"})
	})
	if code != 0 {
		t.Fatalf("convert to stdout failed: code=%d, stderr=%s", code, stderr)
	}
	if !strings.Contains(stdout, "| Region") || !strings.Contains(stdout, "Q1") {
		t.Errorf("expected markdown table in stdout, got:\n%s", stdout)
	}
}

func TestConvert_NonexistentSheet(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "out.csv")

	code, _, stderr := captureAll(func() int {
		return cli.Run([]string{"convert", hwkPath, "-o", outPath, "-s", "NoSuchSheet"})
	})
	if code != 1 {
		t.Errorf("expected exit code 1 for nonexistent sheet, got %d", code)
	}
	if !strings.Contains(stderr, "sheet not found") {
		t.Errorf("expected 'sheet not found' in stderr, got: %s", stderr)
	}
}

// ------------------- Info Tests -------------------

func TestInfo_HumanReadable(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, _ := captureAll(func() int {
		return cli.Run([]string{"info", hwkPath})
	})
	if code != 0 {
		t.Fatalf("info failed: code=%d", code)
	}
	if !strings.Contains(stdout, "Sheets: 2") || !strings.Contains(stdout, "Active: Sales") {
		t.Errorf("unexpected info summary: %s", stdout)
	}
	if !strings.Contains(stdout, "Sheet [0] Sales:") {
		t.Errorf("missing Sales sheet in info: %s", stdout)
	}
}

func TestInfo_JSON(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, _ := captureAll(func() int {
		return cli.Run([]string{"info", hwkPath, "--json"})
	})
	if code != 0 {
		t.Fatalf("info --json failed: code=%d", code)
	}

	var resp cli.Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, stdout)
	}
	if !resp.OK || resp.Command != "info" {
		t.Errorf("unexpected response header: %+v", resp)
	}
}

func TestInfo_SheetNotFound(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"info", hwkPath, "-s", "GhostSheet", "--json"})
	})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "sheet not found") {
		t.Errorf("expected error in stderr, got: %s", stderr)
	}
	var resp cli.Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("expected json error response, got: %v", err)
	}
	if resp.OK || resp.Code != cli.CodeSheetNotFound {
		t.Errorf("unexpected json error: %+v", resp)
	}
}

// ------------------- Get Tests -------------------

func TestGet_JSON(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, _ := captureAll(func() int {
		return cli.Run([]string{"get", hwkPath, "-s", "Sales", "-r", "A1:C2"})
	})
	if code != 0 {
		t.Fatalf("get failed: code=%d", code)
	}

	var resp struct {
		OK      bool           `json:"ok"`
		Command string         `json:"command"`
		Meta    cli.MetaInfo   `json:"meta"`
		Cells   []cli.CellItem `json:"cells"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse get JSON: %v\n%s", err, stdout)
	}
	if !resp.OK || resp.Command != "get" {
		t.Errorf("unexpected header: %+v", resp)
	}
	if resp.Meta.Sheet != "Sales" || resp.Meta.QueryRange != "A1:C2" {
		t.Errorf("unexpected meta: %+v", resp.Meta)
	}
	// A1, B1, C1, A2, B2, C2 -> 6 cells
	if len(resp.Cells) != 6 {
		t.Errorf("expected 6 cells, got %d", len(resp.Cells))
	}
}

func TestGet_Markdown(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, _ := captureAll(func() int {
		return cli.Run([]string{"get", hwkPath, "-s", "Sales", "-r", "A1:C3", "-f", "md"})
	})
	if code != 0 {
		t.Fatalf("get md failed: code=%d", code)
	}
	if !strings.Contains(stdout, "| Region") || !strings.Contains(stdout, "Q1") {
		t.Errorf("expected markdown table, got:\n%s", stdout)
	}
}

func TestGet_CSV(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, _ := captureAll(func() int {
		return cli.Run([]string{"get", hwkPath, "-s", "Sales", "-r", "A1:C2", "-f", "csv"})
	})
	if code != 0 {
		t.Fatalf("get csv failed: code=%d", code)
	}
	if !strings.Contains(stdout, "Region,Q1,Q2") || !strings.Contains(stdout, "East,100,150") {
		t.Errorf("unexpected csv output: %s", stdout)
	}
}

func TestGet_Values(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	// Single cell value
	code, stdout, _ := captureAll(func() int {
		return cli.Run([]string{"get", hwkPath, "-s", "Sales", "-r", "B2", "-f", "values"})
	})
	if code != 0 {
		t.Fatalf("get values failed: code=%d", code)
	}
	if strings.TrimSpace(stdout) != "100" {
		t.Errorf("expected '100', got '%s'", strings.TrimSpace(stdout))
	}
}

// ------------------- Eval Tests -------------------

func TestEval_StandaloneSuccess(t *testing.T) {
	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"eval", "=SUM(10, 20, 30) * 1.1"})
	})
	if code != 0 {
		t.Fatalf("eval failed: code=%d, stderr=%s", code, stderr)
	}

	var resp struct {
		OK      bool           `json:"ok"`
		Command string         `json:"command"`
		Formula string         `json:"formula"`
		Result  cli.EvalResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if !resp.OK || resp.Result.Type != "NUMBER" {
		t.Errorf("unexpected eval result: %+v", resp)
	}
	if num, ok := resp.Result.Val.(float64); !ok || num != 66.0 {
		t.Errorf("expected 66.0, got %v", resp.Result.Val)
	}
}

func TestEval_FormulaErrorIsExitZero(t *testing.T) {
	// Division by zero = ERR: Must return exit code 0 as per spec!
	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"eval", "=1/0"})
	})
	if code != 0 {
		t.Fatalf("expected exit code 0 for =1/0, got %d (stderr=%s)", code, stderr)
	}

	var resp struct {
		OK      bool           `json:"ok"`
		Command string         `json:"command"`
		Formula string         `json:"formula"`
		Result  cli.EvalResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if !resp.OK || resp.Result.Type != "ERROR" || resp.Result.Val != "ERR" {
		t.Errorf("expected ERROR 'ERR', got: %+v", resp.Result)
	}
}

func TestEval_SyntaxErrorIsExitOne(t *testing.T) {
	// Formula syntax error = Exit 1
	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"eval", "=SUM((("})
	})
	if code != 1 {
		t.Errorf("expected exit code 1 for syntax error, got %d", code)
	}
	if !strings.Contains(stderr, "syntax error") {
		t.Errorf("expected syntax error in stderr, got: %s", stderr)
	}
	var resp cli.Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("expected json response, got: %v", err)
	}
	if resp.OK || resp.Code != cli.CodeSyntaxError {
		t.Errorf("expected CodeSyntaxError, got: %+v", resp)
	}
}

func TestEval_ContextualWithFile(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"eval", "-f", hwkPath, "-s", "Sales", "=B2+C2"})
	})
	if code != 0 {
		t.Fatalf("contextual eval failed: code=%d, stderr=%s", code, stderr)
	}

	var resp struct {
		OK     bool           `json:"ok"`
		Meta   cli.MetaInfo   `json:"meta"`
		Result cli.EvalResult `json:"result"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	// B2 is 100, C2 is 150 -> 250
	if resp.Result.Val != 250.0 {
		t.Errorf("expected 250.0, got %v", resp.Result.Val)
	}
	if resp.Meta.Sheet != "Sales" {
		t.Errorf("expected sheet 'Sales', got '%s'", resp.Meta.Sheet)
	}
}

func TestEval_RawFormat(t *testing.T) {
	code, stdout, _ := captureAll(func() int {
		return cli.Run([]string{"eval", "=5*12", "--format", "raw"})
	})
	if code != 0 {
		t.Fatalf("eval raw failed: code=%d", code)
	}
	if strings.TrimSpace(stdout) != "60" {
		t.Errorf("expected '60', got '%s'", strings.TrimSpace(stdout))
	}
}

// ------------------- Chart Tests -------------------

func TestChart_GeneratePNG(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	tmpDir := t.TempDir()
	outPNG := filepath.Join(tmpDir, "chart.png")

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{
			"chart", hwkPath,
			"-o", outPNG,
			"-s", "Sales",
			"-t", "LINE",
			"--title", "Sales Trend",
			"-x", "A2:A3",
			"--series-a", "B2:B3",
		})
	})
	if code != 0 {
		t.Fatalf("chart failed: code=%d, stderr=%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("expected empty stdout, got: %s", stdout)
	}

	// Verify PNG file integrity
	f, err := os.Open(outPNG)
	if err != nil {
		t.Fatalf("failed to open generated png: %v", err)
	}
	defer f.Close()

	imgConfig, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatalf("invalid png generated: %v", err)
	}
	if imgConfig.Width != 1280 || imgConfig.Height != 720 {
		t.Errorf("expected 1280x720, got %dx%d", imgConfig.Width, imgConfig.Height)
	}
}

func TestChart_MissingOutputFlag(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, _, stderr := captureAll(func() int {
		return cli.Run([]string{"chart", hwkPath})
	})
	if code != 1 {
		t.Errorf("expected exit code 1 for missing -o, got %d", code)
	}
	if !strings.Contains(stderr, "missing required output file") {
		t.Errorf("expected 'missing required output file' in stderr, got: %s", stderr)
	}
}

// ------------------- Dispatcher & Help Tests -------------------

func TestDispatcher_SubcommandDetection(t *testing.T) {
	valid := []string{"convert", "info", "get", "eval", "chart", "set", "batch", "mcp", "help"}
	for _, v := range valid {
		if !cli.IsSubcommand(v) {
			t.Errorf("expected %s to be recognized as subcommand", v)
		}
	}
	if cli.IsSubcommand("my_file.hwk") {
		t.Errorf("regular files should not be recognized as subcommands")
	}
}

func TestDispatcher_HelpSubcommand(t *testing.T) {
	code, stdout, _ := captureAll(func() int {
		return cli.Run([]string{"help", "eval"})
	})
	if code != 0 {
		t.Errorf("expected exit code 0 for help eval, got %d", code)
	}
	if !strings.Contains(stdout, "hasucalc eval") {
		t.Errorf("expected eval help text, got: %s", stdout)
	}
}

// ------------------- Set Tests (Phase 2) -------------------

func TestSet_SingleCell(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"set", hwkPath, "B2", "350", "-s", "Sales", "--json"})
	})
	if code != 0 {
		t.Fatalf("set failed: code=%d, stderr=%s", code, stderr)
	}

	var resp cli.Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if !resp.OK || resp.Command != "set" {
		t.Errorf("unexpected resp: %+v", resp)
	}

	// Verify persistence
	wbAfter, err := sheet.LoadWorkbookJSON(hwkPath)
	if err != nil {
		t.Fatalf("failed to reload wb: %v", err)
	}
	sh := wbAfter.GetSheet("Sales")
	cellB2 := sh.GetCell(1, 1)
	if cellB2 == nil || cellB2.Value != 350.0 {
		t.Errorf("expected 350.0, got %v", cellB2.Value)
	}
	// Verify sum recalculation at B4 (=SUM(B2:B3), 350 + 200 = 550)
	cellB4 := sh.GetCell(1, 3)
	if cellB4 == nil || cellB4.Value != 550.0 {
		t.Errorf("expected recalculated sum 550.0, got %v", cellB4.Value)
	}
}

func TestSet_RangeWithFormat(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, _, stderr := captureAll(func() int {
		return cli.Run([]string{"set", hwkPath, "D1:D3", "42", "-s", "Sales", "--fmt", "(F2)"})
	})
	if code != 0 {
		t.Fatalf("set range failed: code=%d, stderr=%s", code, stderr)
	}

	wbAfter, _ := sheet.LoadWorkbookJSON(hwkPath)
	sh := wbAfter.GetSheet("Sales")
	for r := 0; r <= 2; r++ {
		c := sh.GetCell(3, r)
		if c == nil || c.Value != 42.0 {
			t.Errorf("row %d: expected 42.0, got %v", r, c.Value)
		}
		if c.FormatSpec == nil || c.FormatSpec.Decimals != 2 {
			t.Errorf("row %d: expected format (F2), got %v", r, c.FormatSpec)
		}
	}
}

func TestSet_DryRun(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	beforeContent, _ := os.ReadFile(hwkPath)

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"set", hwkPath, "B2", "9999", "-s", "Sales", "--dry-run"})
	})
	if code != 0 {
		t.Fatalf("set dry-run failed: code=%d, stderr=%s", code, stderr)
	}

	var resp cli.Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if !resp.OK {
		t.Errorf("expected ok: true, got %+v", resp)
	}

	// Verify disk file was NOT modified
	afterContent, _ := os.ReadFile(hwkPath)
	if !bytes.Equal(beforeContent, afterContent) {
		t.Errorf("dry-run should not modify file on disk!")
	}
}

func TestSet_SheetNotFound(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"set", hwkPath, "B2", "10", "-s", "NonExistentSheet", "--json"})
	})
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "sheet not found") {
		t.Errorf("expected 'sheet not found' in stderr, got: %s", stderr)
	}
	var resp cli.Response
	json.Unmarshal([]byte(stdout), &resp)
	if resp.Code != cli.CodeSheetNotFound {
		t.Errorf("expected SHEET_NOT_FOUND code, got: %s", resp.Code)
	}
}

// ------------------- Batch Tests (Phase 2) -------------------

func TestBatch_MultiOperations(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	tmpDir := t.TempDir()

	script := `{
		"actions": [
			{ "op": "set_cell", "sheet": "Sales", "cell": "B2", "value": "800", "format": "(C2)" },
			{ "op": "clear", "sheet": "Sales", "range": "C2:C3" },
			{ "op": "add_sheet", "name": "Q3_Target" },
			{ "op": "set_cell", "sheet": "Q3_Target", "cell": "A1", "value": "Projected" },
			{ "op": "recalculate" }
		]
	}`
	scriptPath := filepath.Join(tmpDir, "batch.json")
	os.WriteFile(scriptPath, []byte(script), 0644)

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"batch", hwkPath, "-f", scriptPath})
	})
	if code != 0 {
		t.Fatalf("batch failed: code=%d, stderr=%s", code, stderr)
	}

	var resp cli.Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if !resp.OK || resp.Command != "batch" {
		t.Errorf("unexpected response: %+v", resp)
	}

	// Verify changes persisted
	wbAfter, _ := sheet.LoadWorkbookJSON(hwkPath)
	shSales := wbAfter.GetSheet("Sales")
	if shSales.GetCell(1, 1).Value != 800.0 {
		t.Errorf("expected 800.0, got %v", shSales.GetCell(1, 1).Value)
	}
	// Cleared C2 and C3
	if shSales.GetCell(2, 1) != nil && shSales.GetCell(2, 1).Value != nil && shSales.GetCell(2, 1).Type != cell.TypeEmpty {
		t.Errorf("expected C2 to be empty, got %v", shSales.GetCell(2, 1).Value)
	}
	// Added Q3_Target sheet
	shQ3 := wbAfter.GetSheet("Q3_Target")
	if shQ3 == nil {
		t.Fatalf("expected Q3_Target sheet to exist")
	}
	if shQ3.GetCell(0, 0).Value != "Projected" {
		t.Errorf("expected 'Projected', got %v", shQ3.GetCell(0, 0).Value)
	}
}

func TestBatch_RollbackOnFailure(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	beforeContent, _ := os.ReadFile(hwkPath)
	tmpDir := t.TempDir()

	// Script where step 0 succeeds, but step 1 fails
	script := `[
		{ "op": "set_cell", "sheet": "Sales", "cell": "B2", "value": "99999" },
		{ "op": "set_cell", "sheet": "NoSuchSheetExist", "cell": "A1", "value": "Fail" }
	]`
	scriptPath := filepath.Join(tmpDir, "fail_batch.json")
	os.WriteFile(scriptPath, []byte(script), 0644)

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"batch", hwkPath, "-f", scriptPath})
	})
	if code != 1 {
		t.Errorf("expected exit code 1 for failing batch, got %d", code)
	}
	if !strings.Contains(stderr, "batch step 1") {
		t.Errorf("expected error details in stderr, got: %s", stderr)
	}

	var resp cli.Response
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, stdout)
	}
	if resp.OK || resp.FailedStep != 1 || resp.CompletedSteps != 1 || resp.TotalSteps != 2 {
		t.Errorf("unexpected error structure: %+v", resp)
	}

	// Verify rollback: original file on disk MUST NOT have changed!
	afterContent, _ := os.ReadFile(hwkPath)
	if !bytes.Equal(beforeContent, afterContent) {
		t.Errorf("transaction rollback failed: original file on disk was modified!")
	}
}

func TestBatch_DryRun(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	beforeContent, _ := os.ReadFile(hwkPath)
	tmpDir := t.TempDir()

	script := `[
		{ "op": "set_cell", "sheet": "Sales", "cell": "B2", "value": "777" }
	]`
	scriptPath := filepath.Join(tmpDir, "dry_batch.json")
	os.WriteFile(scriptPath, []byte(script), 0644)

	code, stdout, stderr := captureAll(func() int {
		return cli.Run([]string{"batch", hwkPath, "-f", scriptPath, "--dry-run"})
	})
	if code != 0 {
		t.Fatalf("batch dry-run failed: code=%d, stderr=%s", code, stderr)
	}

	var resp cli.Response
	json.Unmarshal([]byte(stdout), &resp)
	if !resp.OK {
		t.Errorf("expected ok: true, got %+v", resp)
	}

	// Disk must be unmodified
	afterContent, _ := os.ReadFile(hwkPath)
	if !bytes.Equal(beforeContent, afterContent) {
		t.Errorf("batch dry-run modified disk file!")
	}
}

// ------------------- MCP Server Tests (Phase 3) -------------------

func sendMCPRequest(t *testing.T, reqStr string) map[string]any {
	t.Helper()
	var inBuf bytes.Buffer
	inBuf.WriteString(reqStr)
	if !strings.HasSuffix(reqStr, "\n") {
		inBuf.WriteString("\n")
	}
	var outBuf bytes.Buffer

	if err := cli.ServeMCP(&inBuf, &outBuf); err != nil && err != io.EOF {
		t.Fatalf("ServeMCP error: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(outBuf.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json-rpc response: %v\nOutput: %s", err, outBuf.String())
	}
	return resp
}

func TestMCP_Initialize(t *testing.T) {
	resp := sendMCPRequest(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %v", resp["result"])
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("unexpected protocolVersion: %v", result["protocolVersion"])
	}
	serverInfo, ok := result["serverInfo"].(map[string]any)
	if !ok || serverInfo["name"] != "hasucalc" {
		t.Errorf("unexpected serverInfo: %v", serverInfo)
	}
}

func TestMCP_Ping(t *testing.T) {
	resp := sendMCPRequest(t, `{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if resp["id"].(float64) != 2 {
		t.Errorf("expected id 2, got %v", resp["id"])
	}
	if resp["result"] == nil {
		t.Errorf("expected non-nil result object")
	}
}

func TestMCP_ToolsList(t *testing.T) {
	resp := sendMCPRequest(t, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result object, got %v", resp["result"])
	}
	tools, ok := result["tools"].([]any)
	if !ok || len(tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(tools))
	}
	toolNames := make(map[string]bool)
	for _, ti := range tools {
		tObj := ti.(map[string]any)
		toolNames[tObj["name"].(string)] = true
	}
	expectedTools := []string{
		"read_sheet", "get_info", "evaluate_formula",
		"edit_cell", "batch_edit", "render_chart", "convert_file",
	}
	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("missing tool definition: %s", expected)
		}
	}
}

func TestMCP_CallReadSheet(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	// Test read_sheet JSON
	reqJSON := fmt.Sprintf(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_sheet","arguments":{"file":"%s","sheet":"Sales","range":"A1:B2"}}}`, hwkPath)
	resp := sendMCPRequest(t, reqJSON)
	result := resp["result"].(map[string]any)
	if result["isError"] == true {
		t.Fatalf("unexpected tool call error: %v", result)
	}
	contentList := result["content"].([]any)
	text := contentList[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "Region") || !strings.Contains(text, "Sales") {
		t.Errorf("expected Region and Sales in response, got: %s", text)
	}

	// Test read_sheet Markdown
	reqMD := fmt.Sprintf(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"read_sheet","arguments":{"file":"%s","sheet":"Sales","format":"markdown"}}}`, hwkPath)
	respMD := sendMCPRequest(t, reqMD)
	resultMD := respMD["result"].(map[string]any)
	textMD := resultMD["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(textMD, "| Region") {
		t.Errorf("expected markdown table, got:\n%s", textMD)
	}
}

func TestMCP_CallEvaluateFormula(t *testing.T) {
	// Standalone formula
	resp := sendMCPRequest(t, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"evaluate_formula","arguments":{"formula":"=SUM(10,20,30)*1.1"}}}`)
	result := resp["result"].(map[string]any)
	text := result["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "66") {
		t.Errorf("expected 66 in eval text, got: %s", text)
	}

	// Division by zero formula error contract: isError is false, val is "ERR"
	respErr := sendMCPRequest(t, `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"evaluate_formula","arguments":{"formula":"=1/0"}}}`)
	resultErr := respErr["result"].(map[string]any)
	if resultErr["isError"] == true {
		t.Errorf("formula computational errors must return isError: false per spec")
	}
	textErr := resultErr["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(textErr, "ERR") {
		t.Errorf("expected ERR in text, got: %s", textErr)
	}
}

func TestMCP_CallEditCell(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)

	// edit_cell with dry_run
	reqDry := fmt.Sprintf(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"edit_cell","arguments":{"file":"%s","sheet":"Sales","target":"B2","value":"7777","dry_run":true}}}`, hwkPath)
	respDry := sendMCPRequest(t, reqDry)
	resDry := respDry["result"].(map[string]any)
	textDry := resDry["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(textDry, "7777") || !strings.Contains(textDry, `"dryRun": true`) {
		t.Errorf("unexpected dry_run text: %s", textDry)
	}

	// Disk must be unmodified
	wbAfter, _ := sheet.LoadWorkbookJSON(hwkPath)
	if wbAfter.GetSheet("Sales").GetCell(1, 1).Value == 7777.0 {
		t.Errorf("dry_run should not modify disk file")
	}

	// edit_cell persist
	reqPersist := fmt.Sprintf(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"edit_cell","arguments":{"file":"%s","sheet":"Sales","target":"B2","value":"654"}}}`, hwkPath)
	respPersist := sendMCPRequest(t, reqPersist)
	resPersist := respPersist["result"].(map[string]any)
	if resPersist["isError"] == true {
		t.Fatalf("edit_cell persist error: %v", resPersist)
	}

	wbPersisted, _ := sheet.LoadWorkbookJSON(hwkPath)
	if wbPersisted.GetSheet("Sales").GetCell(1, 1).Value != 654.0 {
		t.Errorf("expected 654.0 on disk, got %v", wbPersisted.GetSheet("Sales").GetCell(1, 1).Value)
	}
}

func TestMCP_CallBatchEdit(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	beforeBytes, _ := os.ReadFile(hwkPath)

	// Batch edit with failure rollback
	reqFail := fmt.Sprintf(`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"batch_edit","arguments":{"file":"%s","actions":[{"op":"set_cell","sheet":"Sales","cell":"B2","value":"99999"},{"op":"set_cell","sheet":"NoSuchSheet","cell":"A1","value":"fail"}]}}}`, hwkPath)
	respFail := sendMCPRequest(t, reqFail)
	resFail := respFail["result"].(map[string]any)
	if resFail["isError"] != true {
		t.Errorf("expected isError: true on failing batch")
	}
	textFail := resFail["content"].([]any)[0].(map[string]any)["text"].(string)
	if !strings.Contains(textFail, "SHEET_NOT_FOUND") || !strings.Contains(textFail, `"failed_step": 1`) {
		t.Errorf("unexpected batch failure text: %s", textFail)
	}

	// Disk must be unmodified
	afterBytes, _ := os.ReadFile(hwkPath)
	if !bytes.Equal(beforeBytes, afterBytes) {
		t.Errorf("batch rollback failed in MCP: disk file was modified!")
	}
}

func TestMCP_CallRenderChart(t *testing.T) {
	_, hwkPath := createTestWorkbook(t)
	tmpDir := t.TempDir()
	outPNG := filepath.Join(tmpDir, "mcp_chart.png")

	reqChart := fmt.Sprintf(`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"render_chart","arguments":{"file":"%s","output":"%s","sheet":"Sales","type":"BAR"}}}`, hwkPath, outPNG)
	resp := sendMCPRequest(t, reqChart)
	res := resp["result"].(map[string]any)
	if res["isError"] == true {
		t.Fatalf("render_chart failed: %v", res)
	}

	if _, err := os.Stat(outPNG); err != nil {
		t.Errorf("PNG chart was not generated on disk: %v", err)
	}
}

func TestMCP_UnknownMethod(t *testing.T) {
	resp := sendMCPRequest(t, `{"jsonrpc":"2.0","id":99,"method":"random_method"}`)
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %v", resp)
	}
	if errObj["code"].(float64) != -32601 {
		t.Errorf("expected code -32601, got %v", errObj["code"])
	}
}

