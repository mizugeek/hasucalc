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
	wb, err := ImportMarkdownWorkbook([]byte(md), "Sales")
	if err != nil {
		t.Fatalf("ImportMarkdownWorkbook: %v", err)
	}
	if len(wb.Sheets) != 2 {
		t.Fatalf("want Document + Tables, got %d sheets: %v", len(wb.Sheets), sheetNames(wb))
	}
	doc := wb.GetSheet("Document")
	if doc == nil {
		t.Fatal("Document sheet missing")
	}
	assertLabel(t, doc, 0, 0, "# Sales Report")
	assertLabel(t, doc, 0, 1, "Intro paragraph with bold and a link.")

	tables := wb.GetSheet("Tables")
	if tables == nil {
		t.Fatalf("Tables sheet missing; sheets=%v", sheetNames(wb))
	}
	// Row 0 = banner, row 1 = header
	banner, _ := tables.GetCellValue(0, 0).(string)
	if !strings.Contains(banner, "Sales Report") {
		t.Fatalf("banner = %q", banner)
	}
	if got := tables.GetCellValue(0, 1); got != "Product" {
		t.Fatalf("A2 header = %v", got)
	}
	if got := tables.GetCellValue(1, 1); got != "Qty" {
		t.Fatalf("B2 header = %v", got)
	}
	if got := tables.GetCellValue(0, 2); got != "Apple" {
		t.Fatalf("A3 = %v", got)
	}
	if got, ok := tables.GetCellValue(1, 2).(float64); !ok || got != 10 {
		t.Fatalf("Qty Apple = %v (%T)", tables.GetCellValue(1, 2), tables.GetCellValue(1, 2))
	}

	foundClose := false
	for r := 0; r < 20; r++ {
		if v, _ := doc.GetCellValue(0, r).(string); v == "Closing notes." {
			foundClose = true
			break
		}
	}
	if !foundClose {
		t.Fatalf("closing prose missing on Document:\n%s", dumpSheet(doc))
	}
}

func TestImportMarkdownMultipleTablesOneSheet(t *testing.T) {
	md := `# First

| A | B |
|---|---|
| 1 | 2 |

## Second

| X | Y |
|---|---|
| 9 | 8 |
`
	wb, err := ImportMarkdownWorkbook([]byte(md), "Multi")
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 2 {
		t.Fatalf("want Document+Tables only, got %v", sheetNames(wb))
	}
	tables := wb.GetSheet("Tables")
	if tables == nil {
		t.Fatal("Tables missing")
	}
	banners := 0
	for r := 0; r <= tables.MaxPopulatedRow(); r++ {
		v, _ := tables.GetCellValue(0, r).(string)
		if strings.HasPrefix(v, "══ ") {
			banners++
		}
	}
	if banners != 2 {
		t.Fatalf("want 2 table banners, got %d:\n%s", banners, dumpSheet(tables))
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
	wb, err := ImportHTMLWorkbook([]byte(htmlDoc), "Budget")
	if err != nil {
		t.Fatalf("ImportHTMLWorkbook: %v", err)
	}
	doc := wb.GetSheet("Document")
	if doc == nil {
		t.Fatal("Document missing")
	}
	assertLabel(t, doc, 0, 0, "Budget")
	assertLabel(t, doc, 0, 1, "Notes & more")

	tables := wb.GetSheet("Tables")
	if tables == nil {
		t.Fatalf("Tables missing; sheets=%v", sheetNames(wb))
	}
	if got := tables.GetCellValue(0, 1); got != "Item" {
		t.Fatalf("header = %v dump=\n%s", got, dumpSheet(tables))
	}
	if got := tables.GetCellValue(0, 2); got != "Pen" {
		t.Fatalf("Pen = %v", got)
	}
	if got, ok := tables.GetCellValue(1, 2).(float64); !ok || got != 120 {
		t.Fatalf("Cost = %v", tables.GetCellValue(1, 2))
	}
}

func TestImportMarkupFileByExt(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "a.md")
	if err := os.WriteFile(mdPath, []byte("| A | B |\n|---|---|\n| 1 | 2 |\n"), 0644); err != nil {
		t.Fatal(err)
	}
	wb, err := ImportMarkupWorkbook(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	// Pure table file: Document dropped → Tables is active
	sh := wb.GetActiveSheet()
	if sh.Name() != "Tables" {
		t.Fatalf("active=%q sheets=%v", sh.Name(), sheetNames(wb))
	}
	// banner then header
	if got := sh.GetCellValue(0, 1); got != "A" {
		t.Fatalf("A2=%v dump=\n%s", got, dumpSheet(sh))
	}
	if got, ok := sh.GetCellValue(1, 2).(float64); !ok || got != 2 {
		t.Fatalf("B3=%v", sh.GetCellValue(1, 2))
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

func TestImportMarkdownPreservesQuotesAndCarets(t *testing.T) {
	md := "\"Quoted Line\"\n'Single Quoted'\n^Caret Line\n"
	sh, err := ImportMarkdown([]byte(md))
	if err != nil {
		t.Fatal(err)
	}
	if got := sh.GetCellValue(0, 0); got != "\"Quoted Line\"" {
		t.Errorf("row 0 = %q, want %q", got, "\"Quoted Line\"")
	}
	if got := sh.GetCellValue(0, 1); got != "'Single Quoted'" {
		t.Errorf("row 1 = %q, want %q", got, "'Single Quoted'")
	}
	if got := sh.GetCellValue(0, 2); got != "^Caret Line" {
		t.Errorf("row 2 = %q, want %q", got, "^Caret Line")
	}
}

func TestImportMarkdownTableFirstThenProse(t *testing.T) {
	md := "| A | B |\n|---|---|\n| 1 | 2 |\n\nSome notes after table.\n"
	wb, err := ImportMarkdownWorkbook([]byte(md), "Test")
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 2 {
		t.Fatalf("want 2 sheets, got %d", len(wb.Sheets))
	}
	doc := wb.GetSheet("Document")
	if doc == nil {
		t.Fatal("Document missing")
	}
	ptr, _ := doc.GetCellValue(0, 0).(string)
	if !strings.HasPrefix(ptr, "→ Tables!") {
		t.Errorf("expected pointer to table at Document row 0, got %q", ptr)
	}
	notes, _ := doc.GetCellValue(0, 2).(string)
	if notes != "Some notes after table." {
		t.Errorf("expected notes at Document row 2, got %q", notes)
	}
}

func TestImportHTMLTableFirstThenProse(t *testing.T) {
	htmlDoc := "<table><tr><td>Item</td><td>Price</td></tr><tr><td>A</td><td>10</td></tr></table><p>After table</p>"
	wb, err := ImportHTMLWorkbook([]byte(htmlDoc), "TestHTML")
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 2 {
		t.Fatalf("want 2 sheets, got %d", len(wb.Sheets))
	}
	doc := wb.GetSheet("Document")
	if doc == nil {
		t.Fatal("Document missing")
	}
	ptr, _ := doc.GetCellValue(0, 0).(string)
	if !strings.HasPrefix(ptr, "→ Tables!") {
		t.Errorf("expected pointer to table at Document row 0, got %q", ptr)
	}
}

func TestImportMarkdownClearsModified(t *testing.T) {

	md := "# T\n\n| A | B |\n|---|---|\n| 1 | 2 |\n"
	wb, err := ImportMarkdownWorkbook([]byte(md), "T")
	if err != nil {
		t.Fatal(err)
	}
	if wb.IsModified() {
		t.Fatal("imported markdown workbook should not be modified")
	}
	for i, s := range wb.Sheets {
		if s.IsModified() {
			t.Fatalf("sheet[%d] %q should not be modified", i, s.Name())
		}
	}
}

func TestImportREADMEJATablesSheet(t *testing.T) {
	wb, err := ImportMarkupWorkbook("../README.ja.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(wb.Sheets) != 2 {
		t.Fatalf("expected Document + Tables, got %v", sheetNames(wb))
	}
	tables := wb.GetSheet("Tables")
	if tables == nil {
		t.Fatal("Tables missing")
	}
	found := false
	for r := 0; r <= tables.MaxPopulatedRow(); r++ {
		if tables.GetCellValue(0, r) == "キー" && tables.GetCellValue(1, r) == "動作" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("keybindings header not found:\n%s", dumpSheet(tables))
	}
	if tables.GetColWidth(0) <= 9 {
		t.Fatalf("expected autofit wider than default 9, got %d", tables.GetColWidth(0))
	}
}

func sheetNames(wb *Workbook) []string {
	names := make([]string, len(wb.Sheets))
	for i, sh := range wb.Sheets {
		names[i] = sh.Name()
	}
	return names
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
	for r := 0; r < 40; r++ {
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
			fmt.Fprintf(&b, "R%d: %s\n", r, line.String())
		}
	}
	return b.String()
}
