package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"hasucalc/coord"
	"hasucalc/tui"
)

// RunChart executes the 'chart' subcommand.
func RunChart(args []string) int {
	boolFlags := map[string]bool{}
	normalized := NormalizeArgs(args, boolFlags)

	fs := flag.NewFlagSet("chart", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var output string
	fs.StringVar(&output, "o", "", "Output PNG file path (required)")
	fs.StringVar(&output, "output", "", "Output PNG file path (required)")

	var sheetName string
	fs.StringVar(&sheetName, "s", "", "Target sheet name (default: active sheet)")
	fs.StringVar(&sheetName, "sheet", "", "Target sheet name (default: active sheet)")

	var chartType string
	fs.StringVar(&chartType, "t", "", "Chart type: LINE, BAR, STACKED, PIE")
	fs.StringVar(&chartType, "type", "", "Chart type: LINE, BAR, STACKED, PIE")

	var title string
	fs.StringVar(&title, "title", "", "Chart title string")

	var rangeX string
	fs.StringVar(&rangeX, "x", "", "X-axis category range (e.g. 'A2:A25')")
	fs.StringVar(&rangeX, "range-x", "", "X-axis category range (e.g. 'A2:A25')")

	var seriesA, seriesB, seriesC, seriesD, seriesE, seriesF string
	fs.StringVar(&seriesA, "series-a", "", "Series A data range (e.g. 'B2:B25')")
	fs.StringVar(&seriesB, "series-b", "", "Series B data range")
	fs.StringVar(&seriesC, "series-c", "", "Series C data range")
	fs.StringVar(&seriesD, "series-d", "", "Series D data range")
	fs.StringVar(&seriesE, "series-e", "", "Series E data range")
	fs.StringVar(&seriesF, "series-f", "", "Series F data range")

	var width int
	fs.IntVar(&width, "width", 1280, "Image width in pixels")

	var height int
	fs.IntVar(&height, "height", 720, "Image height in pixels")

	if err := fs.Parse(normalized); err != nil {
		return 2
	}

	posArgs := fs.Args()
	if len(posArgs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: missing input file argument\n")
		return 1
	}
	if output == "" {
		fmt.Fprintf(os.Stderr, "Error: missing required output file (-o <output.png>)\n")
		return 1
	}

	input := posArgs[0]
	wb, err := LoadWorkbookAuto(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load %s: %v\n", input, err)
		return 1
	}

	sh, err := GetTargetSheet(wb, sheetName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	sh.Recalculate()
	g := sh.Graph()
	if g.Series == nil {
		g.Series = make(map[string]*coord.RangeRef)
	}

	if chartType != "" {
		ct := strings.ToUpper(strings.TrimSpace(chartType))
		switch ct {
		case "LINE", "BAR", "STACKED", "PIE":
			g.Type = ct
		default:
			fmt.Fprintf(os.Stderr, "Error: unsupported chart type '%s' (allowed: LINE, BAR, STACKED, PIE)\n", chartType)
			return 1
		}
	} else if g.Type == "" {
		g.Type = "LINE"
	}

	if title != "" {
		g.Title = title
	}

	if rangeX != "" {
		rx, err := coord.ParseRangeRef(rangeX)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid X range '%s': %v\n", rangeX, err)
			return 1
		}
		g.RangeX = &rx
	}

	seriesConfigs := []struct {
		key string
		val string
	}{
		{"A", seriesA},
		{"B", seriesB},
		{"C", seriesC},
		{"D", seriesD},
		{"E", seriesE},
		{"F", seriesF},
	}

	for _, sc := range seriesConfigs {
		if sc.val != "" {
			ref, err := coord.ParseRangeRef(sc.val)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: invalid series %s range '%s': %v\n", sc.key, sc.val, err)
				return 1
			}
			g.Series[sc.key] = &ref
		}
	}

	hasSeries := false
	for _, ref := range g.Series {
		if ref != nil {
			hasSeries = true
			break
		}
	}
	if !hasSeries {
		fmt.Fprintf(os.Stderr, "Error: no data series configured for chart\n")
		return 1
	}

	if err := tui.ExportGraphPNG(sh, output, width, height); err != nil {
		fmt.Fprintf(os.Stderr, "Error: chart export failed: %v\n", err)
		return 1
	}

	return 0
}

// PrintChartHelp displays usage instructions for the chart command.
func PrintChartHelp() {
	msg := `Usage:
  hasucalc chart <input> -o <output.png> [flags]

Flags:
  -o, --output <path>      Output PNG file path (required).
  -s, --sheet <name>       Target sheet name (default: active sheet).
  -t, --type <type>        Chart type: LINE, BAR, STACKED, PIE (overrides saved chart config).
      --title <string>     Chart title string.
  -x, --range-x <range>    X-axis category range (e.g. "A2:A25").
      --series-a <range>   Series A data range (e.g. "B2:B25").
      --series-b <range>   Series B data range (e.g. "C2:C25").
      --series-c <range>   Series C data range (e.g. "D2:D25").
      --series-d <range>   Series D data range.
      --series-e <range>   Series E data range.
      --series-f <range>   Series F data range.
      --width <int>        Image width in pixels (default: 1280).
      --height <int>       Image height in pixels (default: 720).
`
	fmt.Print(msg)
}
