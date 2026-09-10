package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
)

// RunInfo executes the 'info' subcommand.
func RunInfo(args []string) int {
	boolFlags := map[string]bool{
		"json": true,
	}
	normalized := NormalizeArgs(args, boolFlags)

	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var sheetName string
	fs.StringVar(&sheetName, "s", "", "Inspect specific sheet (defaults to active sheet)")
	fs.StringVar(&sheetName, "sheet", "", "Inspect specific sheet (defaults to active sheet)")

	var jsonOutput bool
	fs.BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	if err := fs.Parse(normalized); err != nil {
		return 2
	}

	posArgs := fs.Args()
	if len(posArgs) == 0 {
		return EmitError("info", "missing input file argument", CodeInvalidArgument, jsonOutput)
	}

	input := posArgs[0]
	var wb *sheet.Workbook
	var err error

	if input == "-" {
		wb, err = LoadWorkbookFromReader(os.Stdin, "hwk")
		if err != nil {
			return EmitError("info", fmt.Sprintf("failed to parse stdin as hwk: %v", err), CodeIOError, jsonOutput)
		}
	} else {
		wb, err = LoadWorkbookAuto(input)
		if err != nil {
			if os.IsNotExist(err) {
				return EmitError("info", fmt.Sprintf("file not found: '%s'", input), CodeFileNotFound, jsonOutput)
			}
			return EmitError("info", fmt.Sprintf("failed to load '%s': %v", input, err), CodeIOError, jsonOutput)
		}
	}

	if sheetName != "" {
		if _, err := GetTargetSheet(wb, sheetName); err != nil {
			return EmitError("info", err.Error(), CodeSheetNotFound, jsonOutput)
		}
	}

	activeSheetName := wb.GetActiveSheet().Name()
	if sheetName != "" {
		activeSheetName = sheetName
	}

	format := DetectFormat(input)
	if format == "" {
		format = "hwk"
	}

	var sheetInfos []SheetInfo
	for idx, s := range wb.Sheets {
		if sheetName != "" && !strings.EqualFold(s.Name(), sheetName) {
			continue
		}
		sheetInfos = append(sheetInfos, extractSheetInfo(s, idx))
	}

	globalRecalc := wb.GetActiveSheet().RecalcMode()
	if globalRecalc == "" {
		globalRecalc = "AUTO"
	}

	if jsonOutput {
		resp := Response{
			OK:      true,
			Command: "info",
			Meta: MetaInfo{
				File:        filepath.Base(input),
				Format:      format,
				ActiveSheet: activeSheetName,
				SheetCount:  len(wb.Sheets),
				Sheets:      sheetInfos,
				RecalcMode:  globalRecalc,
			},
		}
		if err := WriteJSON(os.Stdout, resp); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON output: %v\n", err)
			return 1
		}
		return 0
	}

	// Human-readable summary output
	fmt.Printf("File: %s (%s)\n", filepath.Base(input), format)
	fmt.Printf("Sheets: %d (Active: %s)\n", len(wb.Sheets), activeSheetName)
	fmt.Printf("Recalc: %s\n\n", globalRecalc)

	for _, si := range sheetInfos {
		fmt.Printf("Sheet [%d] %s:\n", si.Index, si.Name)
		fmt.Printf("  Used Range:  %s\n", si.UsedRange)
		fmt.Printf("  Max Bounds:  %d rows x %d cols\n", si.MaxRow, si.MaxCol)
		fmt.Printf("  Cells:       %d populated\n", si.CellCount)
		fmt.Printf("  Frozen:      %d rows, %d cols\n", si.FrozenRows, si.FrozenCols)
		graphStr := "none"
		if si.HasGraph {
			graphStr = si.GraphType
		}
		fmt.Printf("  Graph:       %s\n\n", graphStr)
	}

	return 0
}

func extractSheetInfo(sh *sheet.Sheet, index int) SheetInfo {
	minC, minR, maxC, maxR := sh.BoundingBox()
	cellCount := 0
	for pt := range sh.GetPopulatedCoords() {
		c := sh.GetCell(pt.Col, pt.Row)
		if c != nil && c.Type != cell.TypeEmpty {
			cellCount++
		}
	}

	var usedRange string
	var maxRow, maxCol int
	if cellCount > 0 {
		if minC == maxC && minR == maxR {
			usedRange = coord.CellRef{Col: minC, Row: minR}.String()
		} else {
			usedRange = fmt.Sprintf("%s:%s", coord.CellRef{Col: minC, Row: minR}.String(), coord.CellRef{Col: maxC, Row: maxR}.String())
		}
		maxRow = maxR + 1
		maxCol = maxC + 1
	} else {
		usedRange = "A1"
		maxRow = 0
		maxCol = 0
	}

	hasGraph := sh.Graph() != nil && sh.Graph().Type != ""
	graphType := ""
	if hasGraph {
		graphType = sh.Graph().Type
	}

	return SheetInfo{
		Name:       sh.Name(),
		Index:      index,
		UsedRange:  usedRange,
		MaxRow:     maxRow,
		MaxCol:     maxCol,
		CellCount:  cellCount,
		FrozenRows: sh.FrozenRows(),
		FrozenCols: sh.FrozenCols(),
		HasGraph:   hasGraph,
		GraphType:  graphType,
	}
}

// PrintInfoHelp displays usage instructions for the info command.
func PrintInfoHelp() {
	msg := `Usage:
  hasucalc info <input> [flags]

Flags:
  -s, --sheet <name>     Inspect specific sheet. Defaults to active sheet.
      --json             Output in JSON format (default is human-readable summary).
`
	fmt.Print(msg)
}
