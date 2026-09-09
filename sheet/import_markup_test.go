package sheet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hasucalc/cell"
)

func TestImportMarkdownTablesAndProse(t *testing.T) {
	md := `# Sales Report

Intro paragraph with **bold** and a [link](https://example.com).

| Product | Qty | Price |
|:--------|----:|------:|
| Apple   |  10 |  1.5  |
| Orange  |   3 |  2.0  |

Closing notes.
`
	sh, err := ImportMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("ImportMarkdown: %v", err)
	}

	assertLabel(t, sh, 0, 0, "# Sales Report")
	assertLabel(t, sh, 0, 1, "Intro paragraph with bold and a link.")

	headerRow := -1
	for r := 0; r < 20; r++ {
		if v, _ := sh.GetCellValue(0, r).(string); v == "Product" {
			headerRow = r
			break
		}
	}
	if headerRow < 0 {
		t.Fatalf("table header not found; sheet dump:\n%s", dumpSheet(sh))
	}
	if got := sh.GetCellValue(1, headerRow); got != "Qty" {
		t.Fatalf("B header = %v", got)
	}
	if got := sh.GetCellValue(0, headerRow+1); got != "Apple" {
		t.Fatalf("A data = %v", got)
	}
	if got, ok := sh.GetCellValue(1, headerRow+1).(float64); !ok || got != 10 {
		t.Fatalf("Qty Apple = %v (%T)", sh.GetCellValue(1, headerRow+1), sh.GetCellValue(1, headerRow+1))
	}
	if got, ok := sh.GetCellValue(2, headerRow+1).(float64); !ok || got != 1.5 {
		t.Fatalf("Price Apple = %v", sh.GetCellValue(2, headerRow+1))
	}

	foundClose := false
	for r := headerRow + 2; r < headerRow+10; r++ {
		if v, _ := sh.GetCellValue(0, r).(string); v == "Closing notes." {
			foundClose = true
			break
		}
	}
	if !foundClose {
		t.Fatalf("closing prose missing:\n%s", dumpSheet(sh))
	}
}

func TestImportHTMLTablesAndProse(t *testing.T) {
	htmlDoc := `<!DOCTYPE html><html><body>
<h1>Budget</h1>
<p>Notes &amp; more</p>
<table>
  <tr><th>Item</th><th>Cost</th></tr>
  <tr><td>Pen</td><td>120</td></tr>
  <tr><td>Paper</td><td>50</td></tr>
</table>
<p>End</p>
</body></html>`
	sh, err := ImportHTML([]byte(htmlDoc))
	if err != nil {
		t.Fatalf("ImportHTML: %v", err)
	}
	assertLabel(t, sh, 0, 0, "Budget")
	assertLabel(t, sh, 0, 1, "Notes & more")

	headerRow := -1
	for r := 0; r < 20; r++ {
		if v, _ := sh.GetCellValue(0, r).(string); v == "Item" {
			headerRow = r
			break
		}
	}
	if headerRow < 0 {
		t.Fatalf("html table missing:\n%s", dumpSheet(sh))
	}
	if got := sh.GetCellValue(0, headerRow+1); got != "Pen" {
		t.Fatalf("Pen = %v", got)
	}
	if got, ok := sh.GetCellValue(1, headerRow+1).(float64); !ok || got != 120 {
		t.Fatalf("Cost = %v", sh.GetCellValue(1, headerRow+1))
	}
}

func TestImportMarkupFileByExt(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "a.md")
	if err := os.WriteFile(mdPath, []byte("| A | B |\n|---|---|\n| 1 | 2 |\n"), 0644); err != nil {
		t.Fatal(err)
	}
	sh, err := ImportMarkupFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if sh.GetCellValue(0, 0) != "A" {
		t.Fatalf("A1=%v", sh.GetCellValue(0, 0))
	}
	if got, ok := sh.GetCellValue(1, 1).(float64); !ok || got != 2 {
		t.Fatalf("B2=%v", sh.GetCellValue(1, 1))
	}
}

func TestImportMarkdownForcesProseLabels(t *testing.T) {
	sh, err := ImportMarkdown([]byte("=SUM(A1:B2)\n"))
	if err != nil {
		t.Fatal(err)
	}
	c := sh.GetCell(0, 0)
	if c == nil || c.Type != cell.TypeLabel {
		t.Fatalf("prose looking like formula should be LABEL, got %+v", c)
	}
}

func assertLabel(t *testing.T, sh *Sheet, col, row int, want string) {
	t.Helper()
	got := sh.GetCellValue(col, row)
	if got != want {
		t.Fatalf("cell (%d,%d) = %v (%T), want %q", col, row, got, got, want)
	}
}

func dumpSheet(sh *Sheet) string {
	var b strings.Builder
	for r := 0; r < 30; r++ {
		empty := true
		var line strings.Builder
		for c := 0; c < 8; c++ {
			v := sh.GetCellValue(c, r)
			if v == nil {
				line.WriteByte('\t')
				continue
			}
			empty = false
			fmt.Fprintf(&line, "%v\t", v)
		}
		if !empty {
			b.WriteString(line.String())
			b.WriteByte('\n')
		}
	}
	return b.String()
}
