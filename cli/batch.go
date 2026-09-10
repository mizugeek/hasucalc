package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
)

// BatchAction defines an individual operation within a batch script.
type BatchAction struct {
	Op      string `json:"op"`
	Sheet   string `json:"sheet,omitempty"`
	Cell    string `json:"cell,omitempty"`
	Range   string `json:"range,omitempty"`
	Value   any    `json:"value,omitempty"`
	Values  any    `json:"values,omitempty"`
	Format  string `json:"format,omitempty"`
	Row     int    `json:"row,omitempty"`
	Col     any    `json:"col,omitempty"` // letter string or 0-indexed int
	Count   int    `json:"count,omitempty"`
	Name    string `json:"name,omitempty"`
	OldName string `json:"old_name,omitempty"`
	NewName string `json:"new_name,omitempty"`
}

type batchScriptEnvelope struct {
	Actions []BatchAction `json:"actions"`
}

// RunBatch executes the 'batch' subcommand.
func RunBatch(args []string) int {
	boolFlags := map[string]bool{
		"recalc":    true,
		"no-recalc": true,
		"dry-run":   true,
		"json":      true,
	}
	normalized := NormalizeArgs(args, boolFlags)

	fs := flag.NewFlagSet("batch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var defaultSheet string
	fs.StringVar(&defaultSheet, "s", "", "Default sheet name for actions omitting sheet")
	fs.StringVar(&defaultSheet, "sheet", "", "Default sheet name for actions omitting sheet")

	var output string
	fs.StringVar(&output, "o", "", "Output destination path (default: in-place overwrite)")
	fs.StringVar(&output, "output", "", "Output destination path (default: in-place overwrite)")

	var scriptPath string
	fs.StringVar(&scriptPath, "f", "", "Path to JSON batch script (reads from stdin if omitted or '-')")
	fs.StringVar(&scriptPath, "file", "", "Path to JSON batch script (reads from stdin if omitted or '-')")

	var recalc bool
	fs.BoolVar(&recalc, "recalc", true, "Recalculate workbook after batch execution")

	var noRecalc bool
	fs.BoolVar(&noRecalc, "no-recalc", false, "Disable automatic recalculation")

	var dryRun bool
	fs.BoolVar(&dryRun, "dry-run", false, "Execute batch in memory without saving to disk")

	var jsonOutput bool
	fs.BoolVar(&jsonOutput, "json", true, "Output execution summary in JSON")

	if err := fs.Parse(normalized); err != nil {
		return 2
	}

	if noRecalc {
		recalc = false
	}

	posArgs := fs.Args()
	if len(posArgs) == 0 {
		return EmitError("batch", "missing input file argument", CodeInvalidArgument, true)
	}

	input := posArgs[0]
	if input == "-" {
		return EmitError("batch", "input workbook must be a file; batch actions can be piped via stdin", CodeInvalidArgument, true)
	}

	// Read batch actions payload
	var rawData []byte
	var err error
	if scriptPath == "" || scriptPath == "-" {
		rawData, err = io.ReadAll(os.Stdin)
		if err != nil {
			return EmitError("batch", fmt.Sprintf("failed to read batch script from stdin: %v", err), CodeIOError, true)
		}
	} else {
		rawData, err = os.ReadFile(scriptPath)
		if err != nil {
			return EmitError("batch", fmt.Sprintf("failed to read batch script '%s': %v", scriptPath, err), CodeIOError, true)
		}
	}

	if len(strings.TrimSpace(string(rawData))) == 0 {
		return EmitError("batch", "empty batch script payload", CodeInvalidArgument, true)
	}

	// Parse actions: accept both array `[ ... ]` and envelope `{ "actions": [ ... ] }`
	var actions []BatchAction
	if err := json.Unmarshal(rawData, &actions); err != nil {
		var env batchScriptEnvelope
		if errEnv := json.Unmarshal(rawData, &env); errEnv != nil {
			return EmitError("batch", fmt.Sprintf("invalid batch JSON syntax: %v", err), CodeSyntaxError, true)
		}
		actions = env.Actions
	}

	if len(actions) == 0 {
		return EmitError("batch", "no actions found in batch script", CodeInvalidArgument, true)
	}

	// Load workbook into memory
	wb, err := LoadWorkbookAuto(input)
	if err != nil {
		if os.IsNotExist(err) {
			return EmitError("batch", fmt.Sprintf("file not found: '%s'", input), CodeFileNotFound, true)
		}
		return EmitError("batch", fmt.Sprintf("failed to load '%s': %v", input, err), CodeIOError, true)
	}

	// Execute actions in memory with transactional atomicity
	for stepIdx, act := range actions {
		if err := executeBatchAction(wb, act, defaultSheet); err != nil {
			fmt.Fprintf(os.Stderr, "Error at batch step %d (%s): %v\n", stepIdx, act.Op, err)
			resp := Response{
				OK:             false,
				Command:        "batch",
				Error:          err.Error(),
				Code:           determineErrorCode(err),
				FailedStep:     stepIdx,
				CompletedSteps: stepIdx,
				TotalSteps:     len(actions),
			}
			_ = WriteJSON(os.Stdout, resp)
			return 1
		}
	}

	if recalc {
		wb.RecalculateAll()
	}

	destPath := input
	if output != "" {
		destPath = output
	}

	saved := false
	if !dryRun {
		if destPath == "-" {
			toFmt := DetectFormat(input)
			if toFmt == "" {
				toFmt = "hwk"
			}
			if err := SaveWorkbookToWriter(wb, defaultSheet, os.Stdout, toFmt, recalc); err != nil {
				return EmitError("batch", fmt.Sprintf("failed to write to stdout: %v", err), CodeIOError, true)
			}
			saved = true
		} else {
			if err := SaveWorkbookAtomic(wb, defaultSheet, destPath, "", recalc); err != nil {
				return EmitError("batch", fmt.Sprintf("failed to save '%s': %v", destPath, err), CodeIOError, true)
			}
			saved = true
		}
	}

	if jsonOutput || dryRun {
		resp := Response{
			OK:      true,
			Command: "batch",
			Meta: MetaInfo{
				File:           filepath.Base(destPath),
				Saved:          saved,
				DryRun:         dryRun,
				ActionsApplied: len(actions),
				RecalcMode:     wb.GetActiveSheet().RecalcMode(),
			},
		}
		if err := WriteJSON(os.Stdout, resp); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if dryRun {
		fmt.Printf("[Dry-Run] Successfully executed %d batch actions in memory (not saved)\n", len(actions))
	} else if destPath != "-" {
		fmt.Printf("Successfully applied %d batch actions (saved to %s)\n", len(actions), destPath)
	}

	return 0
}

func executeBatchAction(wb *sheet.Workbook, act BatchAction, defaultSheet string) error {
	op := strings.ToLower(strings.TrimSpace(act.Op))
	sheetName := act.Sheet
	if sheetName == "" {
		sheetName = defaultSheet
	}

	switch op {
	case "set_cell":
		sh, err := GetTargetSheet(wb, sheetName)
		if err != nil {
			return err
		}
		if act.Cell == "" {
			return fmt.Errorf("missing 'cell' coordinate in set_cell action")
		}
		cRef, err := coord.ParseCellRef(act.Cell)
		if err != nil {
			return fmt.Errorf("invalid cell coordinate '%s': %w", act.Cell, err)
		}
		var fmtSpec *cell.CellFormat
		if act.Format != "" {
			parsed := cell.ParseCellFormat(act.Format)
			fmtSpec = &parsed
		}
		valStr := fmt.Sprintf("%v", act.Value)
		sh.SetCellInput(cRef.Col, cRef.Row, valStr, fmtSpec)
		return nil

	case "set_range":
		sh, err := GetTargetSheet(wb, sheetName)
		if err != nil {
			return err
		}
		if act.Range == "" {
			return fmt.Errorf("missing 'range' in set_range action")
		}
		rng, err := coord.ParseRangeRef(act.Range)
		if err != nil {
			return fmt.Errorf("invalid range '%s': %w", act.Range, err)
		}
		var fmtSpec *cell.CellFormat
		if act.Format != "" {
			parsed := cell.ParseCellFormat(act.Format)
			fmtSpec = &parsed
		}

		// Check if values is 2D slice or 1D slice
		if grid, ok := act.Values.([]any); ok && len(grid) > 0 {
			if _, isRowSlice := grid[0].([]any); isRowSlice {
				// 2D grid
				for rIdx, rowVal := range grid {
					if rowSlice, okRow := rowVal.([]any); okRow {
						for cIdx, val := range rowSlice {
							targetR := rng.MinRow() + rIdx
							targetC := rng.MinCol() + cIdx
							if targetR <= rng.MaxRow() && targetC <= rng.MaxCol() {
								sh.SetCellInput(targetC, targetR, fmt.Sprintf("%v", val), fmtSpec)
							}
						}
					}
				}
				return nil
			}
			// 1D slice: fill sequentially across range
			idx := 0
			for r := rng.MinRow(); r <= rng.MaxRow(); r++ {
				for c := rng.MinCol(); c <= rng.MaxCol(); c++ {
					if idx < len(grid) {
						sh.SetCellInput(c, r, fmt.Sprintf("%v", grid[idx]), fmtSpec)
						idx++
					}
				}
			}
			return nil
		}

		// Scalar fill across range
		fillVal := ""
		if act.Value != nil {
			fillVal = fmt.Sprintf("%v", act.Value)
		} else if act.Values != nil {
			fillVal = fmt.Sprintf("%v", act.Values)
		}
		for r := rng.MinRow(); r <= rng.MaxRow(); r++ {
			for c := rng.MinCol(); c <= rng.MaxCol(); c++ {
				sh.SetCellInput(c, r, fillVal, fmtSpec)
			}
		}
		return nil

	case "clear":
		sh, err := GetTargetSheet(wb, sheetName)
		if err != nil {
			return err
		}
		target := act.Range
		if target == "" {
			target = act.Cell
		}
		if target == "" {
			return fmt.Errorf("missing 'range' or 'cell' in clear action")
		}
		rng, err := coord.ParseRangeRef(target)
		if err != nil {
			return fmt.Errorf("invalid clear target '%s': %w", target, err)
		}
		sh.ClearRange(rng)
		return nil

	case "format":
		sh, err := GetTargetSheet(wb, sheetName)
		if err != nil {
			return err
		}
		if act.Range == "" {
			return fmt.Errorf("missing 'range' in format action")
		}
		rng, err := coord.ParseRangeRef(act.Range)
		if err != nil {
			return fmt.Errorf("invalid range '%s': %w", act.Range, err)
		}
		if act.Format == "" {
			return fmt.Errorf("missing 'format' descriptor in format action")
		}
		parsed := cell.ParseCellFormat(act.Format)
		sh.FormatRange(rng, parsed)
		return nil

	case "insert_row":
		sh, err := GetTargetSheet(wb, sheetName)
		if err != nil {
			return err
		}
		atRow := act.Row
		if atRow >= 1 {
			atRow-- // convert 1-based row number to 0-indexed
		}
		if atRow < 0 {
			atRow = 0
		}
		count := act.Count
		if count <= 0 {
			count = 1
		}
		sh.InsertRow(atRow, count)
		return nil

	case "delete_row":
		sh, err := GetTargetSheet(wb, sheetName)
		if err != nil {
			return err
		}
		atRow := act.Row
		if atRow >= 1 {
			atRow--
		}
		if atRow < 0 {
			atRow = 0
		}
		count := act.Count
		if count <= 0 {
			count = 1
		}
		sh.DeleteRow(atRow, count)
		return nil

	case "insert_col":
		sh, err := GetTargetSheet(wb, sheetName)
		if err != nil {
			return err
		}
		atCol := parseColParam(act.Col)
		count := act.Count
		if count <= 0 {
			count = 1
		}
		sh.InsertCol(atCol, count)
		return nil

	case "delete_col":
		sh, err := GetTargetSheet(wb, sheetName)
		if err != nil {
			return err
		}
		atCol := parseColParam(act.Col)
		count := act.Count
		if count <= 0 {
			count = 1
		}
		sh.DeleteCol(atCol, count)
		return nil

	case "add_sheet":
		if act.Name == "" {
			return fmt.Errorf("missing 'name' in add_sheet action")
		}
		wb.AddSheet(act.Name)
		return nil

	case "rename_sheet":
		if act.OldName == "" || act.NewName == "" {
			return fmt.Errorf("rename_sheet requires both 'old_name' and 'new_name'")
		}
		return wb.RenameSheet(act.OldName, act.NewName)

	case "delete_sheet":
		name := act.Name
		if name == "" {
			name = sheetName
		}
		if name == "" {
			return fmt.Errorf("missing 'name' in delete_sheet action")
		}
		sh := wb.GetSheet(name)
		if sh == nil {
			return fmt.Errorf("sheet not found: '%s'", name)
		}
		idx := wb.GetSheetIndex(sh)
		return wb.DeleteSheet(idx)

	case "recalculate":
		wb.RecalculateAll()
		return nil

	default:
		return fmt.Errorf("unsupported batch action operation: '%s'", act.Op)
	}
}

func parseColParam(c any) int {
	switch v := c.(type) {
	case string:
		v = strings.TrimSpace(v)
		if isAllLetters(v) {
			return coord.LetterToCol(v)
		}
		var num int
		fmt.Sscanf(v, "%d", &num)
		if num >= 1 {
			return num - 1
		}
		return num
	case float64:
		intVal := int(v)
		if intVal >= 1 {
			return intVal - 1
		}
		return intVal
	case int:
		if v >= 1 {
			return v - 1
		}
		return v
	default:
		return 0
	}
}

func isAllLetters(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}

func determineErrorCode(err error) string {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "sheet not found") {
		return CodeSheetNotFound
	}
	if strings.Contains(msg, "coordinate") || strings.Contains(msg, "range") || strings.Contains(msg, "syntax") {
		return CodeSyntaxError
	}
	return CodeInvalidArgument
}

// PrintBatchHelp displays usage instructions for the batch command.
func PrintBatchHelp() {
	msg := `Usage:
  hasucalc batch <input> [flags]
  hasucalc batch <input> -f <script.json> [flags]
  cat script.json | hasucalc batch <input> [flags]

Flags:
  -s, --sheet <name>     Default sheet name for actions omitting sheet.
  -o, --output <path>    Output destination path. Defaults to in-place atomic overwrite.
  -f, --file <path>      Path to JSON batch script file (reads from stdin if omitted or "-").
      --recalc           Recalculate prior to saving (default: true).
      --no-recalc        Disable final recalculation.
      --dry-run          Execute all actions in memory without writing to disk.
`
	fmt.Print(msg)
}
