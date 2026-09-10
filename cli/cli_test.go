package cli_test

import (
	"bytes"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	valid := []string{"convert", "info", "get", "eval", "chart", "help"}
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
