package cli

import (
	"flag"
	"fmt"
	"os"

	"hasucalc/sheet"
)

// RunConvert executes the 'convert' subcommand.
func RunConvert(args []string) int {
	boolFlags := map[string]bool{
		"recalc": true,
	}
	normalized := NormalizeArgs(args, boolFlags)

	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var output string
	fs.StringVar(&output, "o", "", "Output destination path (default: stdout)")
	fs.StringVar(&output, "output", "", "Output destination path (default: stdout)")

	var sheetName string
	fs.StringVar(&sheetName, "s", "", "Target sheet name")
	fs.StringVar(&sheetName, "sheet", "", "Target sheet name")

	var fromFmt string
	fs.StringVar(&fromFmt, "from", "", "Input format when reading from stdin")

	var toFmt string
	fs.StringVar(&toFmt, "to", "", "Output format when writing to stdout")

	var recalc bool
	fs.BoolVar(&recalc, "recalc", false, "Force full workbook recalculation before exporting")

	if err := fs.Parse(normalized); err != nil {
		return 2
	}

	posArgs := fs.Args()
	var input string
	if len(posArgs) > 0 {
		input = posArgs[0]
	} else {
		input = "-"
	}

	var wb *sheet.Workbook
	var err error

	if input == "-" {
		if fromFmt == "" {
			fromFmt = "hwk"
		}
		wb, err = LoadWorkbookFromReader(os.Stdin, fromFmt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to parse stdin as %s: %v\n", fromFmt, err)
			return 1
		}
	} else {
		wb, err = LoadWorkbookAuto(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to load %s: %v\n", input, err)
			return 1
		}
	}

	if recalc {
		wb.RecalculateAll()
	}

	// Verify target sheet if specified
	if sheetName != "" {
		if _, err := GetTargetSheet(wb, sheetName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
	}

	if output == "" || output == "-" {
		// Stream to stdout
		if toFmt == "" {
			toFmt = "hwk"
		}
		if err := SaveWorkbookToWriter(wb, sheetName, os.Stdout, toFmt, recalc); err != nil {
			fmt.Fprintf(os.Stderr, "Error: output conversion failed: %v\n", err)
			return 1
		}
	} else {
		// File-to-file
		if toFmt == "" {
			toFmt = DetectFormat(output)
		}
		if toFmt == "" {
			fmt.Fprintf(os.Stderr, "Error: cannot determine output format from filename '%s', specify --to\n", output)
			return 1
		}
		if err := SaveWorkbookAuto(wb, sheetName, output, toFmt, recalc); err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to save to %s: %v\n", output, err)
			return 1
		}
	}

	return 0
}

// PrintConvertHelp displays usage instructions for the convert command.
func PrintConvertHelp() {
	msg := `Usage:
  hasucalc convert <input> -o <output> [flags]
  hasucalc convert --from <fmt> --to <fmt> [flags] < <stdin> > <stdout>

Supported Formats:
  hwk, hwkz, xlsx, ods, csv, tsv, md (markdown), html

Flags:
  -o, --output <path>    Output destination path. If omitted or "-", writes to stdout.
  -s, --sheet <name>     Target sheet to export (for single-sheet formats like CSV/MD). Defaults to active sheet.
      --from <fmt>       Input format when reading from stdin (hwk, xlsx, ods, csv, md, html).
      --to <fmt>         Output format when writing to stdout (hwk, xlsx, ods, csv, md, html).
      --recalc           Force full workbook recalculation before exporting.
`
	fmt.Print(msg)
}
