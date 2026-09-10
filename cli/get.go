package cli

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
)

// RunGet executes the 'get' subcommand.
func RunGet(args []string) int {
	boolFlags := map[string]bool{
		"recalc":    true,
		"no-recalc": true,
	}
	normalized := NormalizeArgs(args, boolFlags)

	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var sheetName string
	fs.StringVar(&sheetName, "s", "", "Target sheet name (default: active sheet)")
	fs.StringVar(&sheetName, "sheet", "", "Target sheet name (default: active sheet)")

	var rangeStr string
	fs.StringVar(&rangeStr, "r", "", "Target coordinate or range (e.g. 'B2', 'A1:E10')")
	fs.StringVar(&rangeStr, "range", "", "Target coordinate or range (e.g. 'B2', 'A1:E10')")

	var format string
	fs.StringVar(&format, "f", "json", "Output format: json (default), markdown (md), csv, values")
	fs.StringVar(&format, "format", "json", "Output format: json (default), markdown (md), csv, values")

	var recalc bool
	fs.BoolVar(&recalc, "recalc", true, "Recalculate workbook prior to extraction")

	var noRecalc bool
	fs.BoolVar(&noRecalc, "no-recalc", false, "Disable recalculation prior to extraction")

	if err := fs.Parse(normalized); err != nil {
		return 2
	}

	if noRecalc {
		recalc = false
	}

	format = strings.ToLower(strings.TrimSpace(format))
	isJSON := (format == "json" || format == "")

	posArgs := fs.Args()
	if len(posArgs) == 0 {
		return EmitError("get", "missing input file argument", CodeInvalidArgument, isJSON)
	}

	input := posArgs[0]
	var wb *sheet.Workbook
	var err error

	if input == "-" {
		wb, err = LoadWorkbookFromReader(os.Stdin, "hwk")
		if err != nil {
			return EmitError("get", fmt.Sprintf("failed to parse stdin as hwk: %v", err), CodeIOError, isJSON)
		}
	} else {
		wb, err = LoadWorkbookAuto(input)
		if err != nil {
			if os.IsNotExist(err) {
				return EmitError("get", fmt.Sprintf("file not found: '%s'", input), CodeFileNotFound, isJSON)
			}
			return EmitError("get", fmt.Sprintf("failed to load '%s': %v", input, err), CodeIOError, isJSON)
		}
	}

	if recalc {
		wb.RecalculateAll()
	}

	sh, err := GetTargetSheet(wb, sheetName)
	if err != nil {
		return EmitError("get", err.Error(), CodeSheetNotFound, isJSON)
	}

	shMinC, shMinR, shMaxC, shMaxR := sh.BoundingBox()
	cellCount := 0
	for pt := range sh.GetPopulatedCoords() {
		c := sh.GetCell(pt.Col, pt.Row)
		if c != nil && c.Type != cell.TypeEmpty {
			cellCount++
		}
	}

	var usedRange string
	if cellCount > 0 {
		if shMinC == shMaxC && shMinR == shMaxR {
			usedRange = coord.CellRef{Col: shMinC, Row: shMinR}.String()
		} else {
			usedRange = fmt.Sprintf("%s:%s", coord.CellRef{Col: shMinC, Row: shMinR}.String(), coord.CellRef{Col: shMaxC, Row: shMaxR}.String())
		}
	} else {
		usedRange = "A1"
		shMinC, shMinR, shMaxC, shMaxR = 0, 0, 0, 0
	}

	var minC, minR, maxC, maxR int
	var queryRange string

	if rangeStr != "" {
		rng, err := coord.ParseRangeRef(rangeStr)
		if err != nil {
			return EmitError("get", fmt.Sprintf("invalid range: '%s'", rangeStr), CodeSyntaxError, isJSON)
		}
		minC = rng.MinCol()
		maxC = rng.MaxCol()
		minR = rng.MinRow()
		maxR = rng.MaxRow()

		if rng.IsWholeColumn() {
			if cellCount > 0 {
				maxR = shMaxR
			} else {
				maxR = 0
			}
		}
		if rng.IsWholeRow() {
			if cellCount > 0 {
				maxC = shMaxC
			} else {
				maxC = 0
			}
		}

		if minC == maxC && minR == maxR {
			queryRange = coord.CellRef{Col: minC, Row: minR}.String()
		} else {
			queryRange = fmt.Sprintf("%s:%s", coord.CellRef{Col: minC, Row: minR}.String(), coord.CellRef{Col: maxC, Row: maxR}.String())
		}
	} else {
		minC, minR, maxC, maxR = shMinC, shMinR, shMaxC, shMaxR
		queryRange = usedRange
	}

	recalcMode := sh.RecalcMode()
	if recalcMode == "" {
		recalcMode = "AUTO"
	}

	switch format {
	case "json", "":
		var cells []CellItem
		for r := minR; r <= maxR; r++ {
			for c := minC; c <= maxC; c++ {
				cellVal := sh.GetCell(c, r)
				if cellVal == nil || cellVal.Type == cell.TypeEmpty {
					continue
				}
				if cellVal.RawInput == "" && cellVal.Value == nil {
					continue
				}

				ref := coord.CellRef{Col: c, Row: r}.String()
				typeStr := string(cellVal.Type)
				val := cellVal.Value
				if errVal, ok := cellVal.Value.(cell.LotusError); ok {
					typeStr = "ERROR"
					val = errVal.Code
				}

				var fmtStr string
				if cellVal.FormatSpec != nil && cellVal.FormatSpec.Type != cell.FmtGeneral {
					fmtStr = cellVal.FormatSpec.String()
				}

				cells = append(cells, CellItem{
					Ref: ref,
					R:   r,
					C:   c,
					Typ: typeStr,
					Raw: cellVal.RawInput,
					Val: val,
					Fmt: fmtStr,
				})
			}
		}

		resp := Response{
			OK:      true,
			Command: "get",
			Meta: MetaInfo{
				File:       filepath.Base(input),
				Sheet:      sh.Name(),
				Sheets:     wb.SheetNames(),
				UsedRange:  usedRange,
				QueryRange: queryRange,
				RecalcMode: recalcMode,
			},
			Cells: cells,
		}
		if err := WriteJSON(os.Stdout, resp); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
			return 1
		}
		return 0

	case "markdown", "md":
		table := sh.RenderMarkdownTableRange(minC, minR, maxC, maxR)
		fmt.Print(table)
		if !strings.HasSuffix(table, "\n") {
			fmt.Println()
		}
		return 0

	case "csv":
		writer := csv.NewWriter(os.Stdout)
		for r := minR; r <= maxR; r++ {
			var rowVals []string
			for c := minC; c <= maxC; c++ {
				cellVal := sh.GetCell(c, r)
				if cellVal != nil && cellVal.Value != nil && cellVal.Type != cell.TypeEmpty {
					rowVals = append(rowVals, cellVal.FormattedValue(sh.GlobalFormat()))
				} else {
					rowVals = append(rowVals, "")
				}
			}
			if err := writer.Write(rowVals); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
				return 1
			}
		}
		writer.Flush()
		return 0

	case "values":
		for r := minR; r <= maxR; r++ {
			var rowVals []string
			for c := minC; c <= maxC; c++ {
				cellVal := sh.GetCell(c, r)
				if cellVal != nil && cellVal.Value != nil && cellVal.Type != cell.TypeEmpty {
					rowVals = append(rowVals, cellVal.FormattedValue(sh.GlobalFormat()))
				} else {
					rowVals = append(rowVals, "")
				}
			}
			fmt.Println(strings.Join(rowVals, "\t"))
		}
		return 0

	default:
		return EmitError("get", fmt.Sprintf("unsupported format '%s' (allowed: json, markdown, csv, values)", format), CodeInvalidArgument, false)
	}
}

// PrintGetHelp displays usage instructions for the get command.
func PrintGetHelp() {
	msg := `Usage:
  hasucalc get <input> [flags]

Flags:
  -s, --sheet <name>       Target sheet name (default: active sheet).
  -r, --range <range>       Target coordinate or range (e.g. "B2", "A1:E10", "A:C"). Defaults to used range.
  -f, --format <format>     Output format: "json" (default), "markdown" (or "md"), "csv", "values".
      --recalc              Recalculate workbook prior to extraction (default: true).
`
	fmt.Print(msg)
}
