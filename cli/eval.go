package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"hasucalc/sheet"
)

// RunEval executes the 'eval' subcommand.
func RunEval(args []string) int {
	boolFlags := map[string]bool{}
	normalized := NormalizeArgs(args, boolFlags)

	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var filePath string
	fs.StringVar(&filePath, "f", "", "Context workbook file to load (optional)")
	fs.StringVar(&filePath, "file", "", "Context workbook file to load (optional)")

	var sheetName string
	fs.StringVar(&sheetName, "s", "", "Context sheet name (defaults to active sheet)")
	fs.StringVar(&sheetName, "sheet", "", "Context sheet name (defaults to active sheet)")

	var format string
	fs.StringVar(&format, "format", "json", "Output format: 'json' (default) or 'raw'")

	if err := fs.Parse(normalized); err != nil {
		return 2
	}

	format = strings.ToLower(strings.TrimSpace(format))
	isJSON := (format != "raw")

	posArgs := fs.Args()
	if len(posArgs) == 0 {
		return EmitError("eval", "missing formula argument", CodeInvalidArgument, isJSON)
	}

	formulaText := posArgs[0]

	var wb *sheet.Workbook
	var sh *sheet.Sheet
	var err error

	if filePath != "" {
		wb, err = LoadWorkbookAuto(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return EmitError("eval", fmt.Sprintf("file not found: '%s'", filePath), CodeFileNotFound, isJSON)
			}
			return EmitError("eval", fmt.Sprintf("failed to load '%s': %v", filePath, err), CodeIOError, isJSON)
		}
		sh, err = GetTargetSheet(wb, sheetName)
		if err != nil {
			return EmitError("eval", err.Error(), CodeSheetNotFound, isJSON)
		}
	} else {
		sh = sheet.NewSheet()
		wb = sheet.NewWorkbook("Eval")
		wb.Sheets[0] = sh
		sh.SetWorkbook(wb)
	}

	// Parse formula syntax
	ast, err := formula.ParseFormula(formulaText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: formula syntax error: %v\n", err)
		if isJSON {
			resp := Response{
				OK:      false,
				Command: "eval",
				Formula: formulaText,
				Error:   err.Error(),
				Code:    CodeSyntaxError,
			}
			_ = WriteJSON(os.Stdout, resp)
		}
		return 1
	}

	// Evaluate AST in sheet context
	evaluator := formula.NewEvaluator(sh)
	val := evaluator.Evaluate(ast)
	val = unwrapGrid(val)

	var typeStr string
	var valPayload any
	var errPayload any

	if errVal, ok := val.(cell.LotusError); ok {
		typeStr = "ERROR"
		valPayload = errVal.Code
		errPayload = errVal.Code
	} else {
		switch v := val.(type) {
		case float64:
			typeStr = "NUMBER"
			valPayload = v
		case int:
			typeStr = "NUMBER"
			valPayload = v
		case bool:
			typeStr = "BOOLEAN"
			valPayload = v
		case string:
			typeStr = "LABEL"
			valPayload = v
		case nil:
			typeStr = "EMPTY"
			valPayload = nil
		default:
			typeStr = "LABEL"
			valPayload = fmt.Sprintf("%v", v)
		}
	}

	if !isJSON {
		// Raw format
		if valPayload == nil {
			fmt.Println("")
		} else {
			fmt.Println(valPayload)
		}
		return 0
	}

	// JSON format
	resp := Response{
		OK:      true,
		Command: "eval",
		Formula: formulaText,
		Result: EvalResult{
			Val:   valPayload,
			Type:  typeStr,
			Error: errPayload,
		},
	}

	if filePath != "" {
		minC, minR, maxC, maxR := sh.BoundingBox()
		var usedRange string
		if len(sh.GetPopulatedCoords()) > 0 {
			if minC == maxC && minR == maxR {
				usedRange = coord.CellRef{Col: minC, Row: minR}.String()
			} else {
				usedRange = fmt.Sprintf("%s:%s", coord.CellRef{Col: minC, Row: minR}.String(), coord.CellRef{Col: maxC, Row: maxR}.String())
			}
		} else {
			usedRange = "A1"
		}

		resp.Meta = MetaInfo{
			File:      filepath.Base(filePath),
			Sheet:     sh.Name(),
			UsedRange: usedRange,
		}
	}

	if err := WriteJSON(os.Stdout, resp); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing JSON: %v\n", err)
		return 1
	}

	return 0
}

func unwrapGrid(v any) any {
	if grid, ok := v.([][]any); ok {
		if len(grid) == 1 && len(grid[0]) == 1 {
			return grid[0][0]
		}
	}
	return v
}

// PrintEvalHelp displays usage instructions for the eval command.
func PrintEvalHelp() {
	msg := `Usage:
  hasucalc eval "<formula>" [flags]

Flags:
  -f, --file <path>        Context workbook file to load (optional).
  -s, --sheet <name>       Context sheet name (defaults to active sheet).
      --format <format>    Output format: "json" (default) or "raw" (raw value only).
`
	fmt.Print(msg)
}
