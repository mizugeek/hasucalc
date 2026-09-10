package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
)

// RunSet executes the 'set' subcommand.
func RunSet(args []string) int {
	boolFlags := map[string]bool{
		"recalc":    true,
		"no-recalc": true,
		"dry-run":   true,
		"json":      true,
	}
	normalized := NormalizeArgs(args, boolFlags)

	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var sheetName string
	fs.StringVar(&sheetName, "s", "", "Target sheet name (default: active sheet)")
	fs.StringVar(&sheetName, "sheet", "", "Target sheet name (default: active sheet)")

	var output string
	fs.StringVar(&output, "o", "", "Output destination path (default: in-place overwrite)")
	fs.StringVar(&output, "output", "", "Output destination path (default: in-place overwrite)")

	var fmtStr string
	fs.StringVar(&fmtStr, "fmt", "", "Cell format descriptor e.g. '(F1)', '(C2)', '(P0)'")

	var recalc bool
	fs.BoolVar(&recalc, "recalc", true, "Recalculate workbook after mutation")

	var noRecalc bool
	fs.BoolVar(&noRecalc, "no-recalc", false, "Disable automatic recalculation")

	var dryRun bool
	fs.BoolVar(&dryRun, "dry-run", false, "Preview mutation in memory without saving to disk")

	var jsonOutput bool
	fs.BoolVar(&jsonOutput, "json", false, "Output result metadata in JSON")

	if err := fs.Parse(normalized); err != nil {
		return 2
	}

	if noRecalc {
		recalc = false
	}

	isJSON := jsonOutput || dryRun

	posArgs := fs.Args()
	if len(posArgs) < 3 {
		return EmitError("set", "missing required arguments: <input> <target> <value>", CodeInvalidArgument, isJSON)
	}

	input := posArgs[0]
	target := posArgs[1]
	value := posArgs[2]

	var wb *sheet.Workbook
	var err error

	if input == "-" {
		return EmitError("set", "cannot mutate stdin in place; use 'batch' or 'convert' for stream pipelines", CodeInvalidArgument, isJSON)
	}

	wb, err = LoadWorkbookAuto(input)
	if err != nil {
		if os.IsNotExist(err) {
			return EmitError("set", fmt.Sprintf("file not found: '%s'", input), CodeFileNotFound, isJSON)
		}
		return EmitError("set", fmt.Sprintf("failed to load '%s': %v", input, err), CodeIOError, isJSON)
	}

	sh, err := GetTargetSheet(wb, sheetName)
	if err != nil {
		return EmitError("set", err.Error(), CodeSheetNotFound, isJSON)
	}

	// Parse target range or cell ref
	rng, err := coord.ParseRangeRef(target)
	if err != nil {
		return EmitError("set", fmt.Sprintf("invalid coordinate/range '%s': %v", target, err), CodeSyntaxError, isJSON)
	}

	var fmtSpec *cell.CellFormat
	if fmtStr != "" {
		parsedFmt := cell.ParseCellFormat(fmtStr)
		fmtSpec = &parsedFmt
	}

	minC, maxC := rng.MinCol(), rng.MaxCol()
	minR, maxR := rng.MinRow(), rng.MaxRow()

	for r := minR; r <= maxR; r++ {
		for c := minC; c <= maxC; c++ {
			sh.SetCellInput(c, r, value, fmtSpec)
		}
	}

	if recalc {
		wb.RecalculateAll()
	}

	var affected []CellItem
	for r := minR; r <= maxR; r++ {
		for c := minC; c <= maxC; c++ {
			cellVal := sh.GetCell(c, r)
			if cellVal == nil {
				continue
			}
			ref := coord.CellRef{Col: c, Row: r}.String()
			typeStr := string(cellVal.Type)
			val := cellVal.Value
			if errVal, ok := cellVal.Value.(cell.LotusError); ok {
				typeStr = "ERROR"
				val = errVal.Code
			}

			var appliedFmt string
			if cellVal.FormatSpec != nil && cellVal.FormatSpec.Type != cell.FmtGeneral {
				appliedFmt = cellVal.FormatSpec.String()
			}

			affected = append(affected, CellItem{
				Ref: ref,
				R:   r,
				C:   c,
				Typ: typeStr,
				Raw: cellVal.RawInput,
				Val: val,
				Fmt: appliedFmt,
			})
		}
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
			if err := SaveWorkbookToWriter(wb, sh.Name(), os.Stdout, toFmt, recalc); err != nil {
				return EmitError("set", fmt.Sprintf("failed to write to stdout: %v", err), CodeIOError, isJSON)
			}
			saved = true
		} else {
			if err := SaveWorkbookAtomic(wb, sh.Name(), destPath, "", recalc); err != nil {
				return EmitError("set", fmt.Sprintf("failed to save '%s': %v", destPath, err), CodeIOError, isJSON)
			}
			saved = true
		}
	}

	if isJSON {
		resp := Response{
			OK:      true,
			Command: "set",
			Meta: MetaInfo{
				File:       filepath.Base(destPath),
				Sheet:      sh.Name(),
				Target:     target,
				Saved:      saved,
				DryRun:     dryRun,
				RecalcMode: sh.RecalcMode(),
			},
			Affected: affected,
		}
		if err := WriteJSON(os.Stdout, resp); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
			return 1
		}
		return 0
	}

	if dryRun {
		fmt.Printf("[Dry-Run] Simulated set %s to '%s' in sheet '%s' (%d cells affected, not saved)\n", target, value, sh.Name(), len(affected))
	} else if destPath != "-" {
		fmt.Printf("Set %s to '%s' in sheet '%s' (saved to %s)\n", target, value, sh.Name(), destPath)
	}

	return 0
}

// PrintSetHelp displays usage instructions for the set command.
func PrintSetHelp() {
	msg := `Usage:
  hasucalc set <input> <target> <value> [flags]

Flags:
  -s, --sheet <name>     Target sheet name (default: active sheet).
  -o, --output <path>    Output destination path. Defaults to overwriting <input> in-place atomically. If "-", writes to stdout.
      --fmt <format>     Cell display format descriptor e.g. "(F1)", "(C2)", "(P0)".
      --recalc           Recalculate workbook after mutation (default: true).
      --no-recalc        Disable automatic recalculation.
      --dry-run          Simulate mutation in-memory and return JSON preview without writing to disk.
      --json             Output mutation result metadata in structured JSON.
`
	fmt.Print(msg)
}
