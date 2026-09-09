package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/sheet"
	"hasucalc/tui"
	"hasucalc/version"
)

// Version is the current semantic version of HasuCalc, sourced from version.Version.
const Version = version.Version

func createDemoSheet() *sheet.Sheet {
	sh := sheet.NewSheet()

	// Column Widths
	sh.SetColWidth(0, 6)  // A: JST
	sh.SetColWidth(1, 8)  // B: Japan
	sh.SetColWidth(2, 8)  // C: London
	sh.SetColWidth(3, 8)  // D: California
	sh.SetColWidth(4, 8)  // E: Total
	sh.SetColWidth(5, 10) // F: LonTime
	sh.SetColWidth(6, 10) // G: CalTime
	sh.SetColWidth(7, 26) // H: Memo / Japanese text

	// Header Row 1 (Index 0)
	sh.SetCellInput(0, 0, "'JST", nil)
	sh.SetCellInput(1, 0, "'Japan", nil)
	sh.SetCellInput(2, 0, "'London", nil)
	sh.SetCellInput(3, 0, "'Calif", nil)
	sh.SetCellInput(4, 0, "'Total", nil)
	sh.SetCellInput(5, 0, "'LonTime", nil)
	sh.SetCellInput(6, 0, "'CalTime", nil)
	sh.SetCellInput(7, 0, "'調査メモ", nil)

	// Traffic Data (matching Image 1 & 3)
	data := []struct {
		jst   int
		japan float64
		lon   float64
		cal   float64
		memo  string
	}{
		{18, 6.4, 10.7, 4.0, "調査開始"},
		{19, 6.4, 12.0, 3.2, "AM11:00 AM3:00"},
		{19, 6.0, 12.6, 2.8, ""},
		{20, 6.0, 12.4, 2.4, "AM12:00 AM4:00"},
		{20, 6.0, 11.8, 2.1, ""},
		{21, 6.4, 10.9, 2.0, ""},
		{21, 6.4, 10.8, 1.8, ""},
		{22, 6.4, 10.2, 1.8, "14 AM6:00"},
		{22, 5.9, 9.4, 2.1, ""},
		{23, 5.4, 9.3, 2.4, "AM7:00"},
		{23, 5.1, 8.9, 2.9, ""},
		{0, 4.7, 8.7, 3.5, "16"},
		{0, 4.5, 8.5, 4.2, ""},
		{1, 3.7, 8.5, 5.1, "9"},
		{1, 3.1, 8.7, 5.7, ""},
		{2, 3.1, 8.8, 6.2, "18 10"},
		{2, 2.2, 8.9, 6.7, ""},
		{3, 1.8, 8.7, 7.2, "19"},
		{3, 1.5, 8.5, 7.4, ""},
		{4, 1.2, 8.5, 7.8, ""},
		{5, 1.2, 8.7, 8.0, ""},
		{6, 1.5, 9.2, 8.2, ""},
		{7, 2.0, 9.0, 8.5, ""},
		{8, 2.5, 7.8, 9.0, ""},
	}

	fmtF1 := &cell.CellFormat{Type: cell.FmtFixed, Decimals: 1}

	for i, d := range data {
		row := i + 1
		sh.SetCellInput(0, row, fmt.Sprintf("%d", d.jst), nil)
		sh.SetCellInput(1, row, fmt.Sprintf("%.1f", d.japan), fmtF1)
		sh.SetCellInput(2, row, fmt.Sprintf("%.1f", d.lon), fmtF1)
		sh.SetCellInput(3, row, fmt.Sprintf("%.1f", d.cal), fmtF1)
		sh.SetCellInput(4, row, fmt.Sprintf("+B%d+C%d+D%d", row+1, row+1, row+1), fmtF1)
		if d.memo != "" {
			sh.SetCellInput(7, row, "'"+d.memo, nil)
		}
	}

	// Notes
	sh.SetCellInput(7, 25, "'データ提供: http://www.akamai.com", nil)

	// Configure Graph
	sh.Graph().Type = "LINE"
	sh.Graph().Title = ""
	rX := coord.RangeRef{
		Start: coord.CellRef{Col: 0, Row: 1},
		End:   coord.CellRef{Col: 0, Row: len(data)},
	}
	rA := coord.RangeRef{
		Start: coord.CellRef{Col: 1, Row: 1},
		End:   coord.CellRef{Col: 1, Row: len(data)},
	}
	rB := coord.RangeRef{
		Start: coord.CellRef{Col: 2, Row: 1},
		End:   coord.CellRef{Col: 2, Row: len(data)},
	}
	rC := coord.RangeRef{
		Start: coord.CellRef{Col: 3, Row: 1},
		End:   coord.CellRef{Col: 3, Row: len(data)},
	}
	sh.Graph().RangeX = &rX
	sh.Graph().Series["A"] = &rA
	sh.Graph().Series["B"] = &rB
	sh.Graph().Series["C"] = &rC

	sh.Recalculate()
	return sh
}

func main() {
	var sh *sheet.Sheet
	filename := tui.GenerateUnusedFilename(".", "DATA", ".hwk")

	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "--version" || arg == "-v" || arg == "-V" {
			fmt.Printf("HasuCalc version %s (Go runtime: %s, OS/Arch: %s/%s)\n", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			os.Exit(0)
		} else if arg == "--help" || arg == "-h" {
			fmt.Printf("HasuCalc %s - Modern Terminal Spreadsheet (CLI / TUI)\n\n", Version)
			fmt.Println("Usage:")
			fmt.Println("  hasucalc [file]         Open a spreadsheet file (.hwk, .hwkz, .xlsx, .ods, .csv, .md, .html)")
			fmt.Println("  hasucalc --demo, -d     Launch with preloaded sample traffic & graph demo data")
			fmt.Println("  hasucalc --version, -v  Print version information and exit")
			fmt.Println("  hasucalc --help, -h     Show this help message and exit")
			os.Exit(0)
		} else if arg == "--demo" || arg == "-d" {
			sh = createDemoSheet()
		} else if _, err := os.Stat(arg); err == nil {
			ext := strings.ToLower(filepath.Ext(arg))
			var loadErr error
			if ext == ".xlsx" || ext == ".xlsm" {
				baseName := strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
				filename = baseName + ".hwk"
				if loadedWb, err := sheet.ImportXLSXWorkbook(arg); err == nil {
					sh = loadedWb.GetActiveSheet()
				} else {
					loadErr = err
				}
			} else if ext == ".ods" || ext == ".ots" {
				baseName := strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
				filename = baseName + ".hwk"
				if loadedWb, err := sheet.ImportODSWorkbook(arg); err == nil {
					sh = loadedWb.GetActiveSheet()
				} else {
					loadErr = err
				}
			} else if ext == ".csv" || ext == ".tsv" {
				baseName := strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
				filename = baseName + ".hwk"
				if loadedCSV, err := sheet.ImportSheetCSV(arg); err == nil {
					sh = loadedCSV
				} else {
					loadErr = err
				}
			} else if ext == ".md" || ext == ".markdown" || ext == ".html" || ext == ".htm" {
				baseName := strings.TrimSuffix(filepath.Base(arg), filepath.Ext(arg))
				filename = baseName + ".hwk"
				if loadedMarkup, err := sheet.ImportMarkupFile(arg); err == nil {
					sh = loadedMarkup
				} else {
					loadErr = err
				}
			} else {
				filename = filepath.Base(arg)
				if loadedWb, err := sheet.LoadWorkbookJSON(arg); err == nil {
					sh = loadedWb.GetActiveSheet()
				} else if loaded, err := sheet.LoadSheetJSON(arg); err == nil {
					sh = loaded
				} else {
					loadErr = err
				}
			}
			if sh == nil {
				if loadErr != nil {
					fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", arg, loadErr)
					os.Exit(1)
				}
				sh = sheet.NewSheet()
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error: file not found: %s\n", arg)
			os.Exit(1)
		}
	} else {
		sh = sheet.NewSheet()
	}

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating screen: %v\n", err)
		os.Exit(1)
	}

	if err := screen.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing screen: %v\n", err)
		os.Exit(1)
	}
	defer screen.Fini()

	screen.EnablePaste()
	screen.Clear()

	app := tui.NewApp(screen, sh, filename)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "App error: %v\n", err)
	}
}
