package sheet

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/mattn/go-runewidth"
	"hasucalc/coord"
)

// ImportMarkupFile imports Markdown (.md/.markdown) or HTML (.html/.htm).
// GFM/HTML tables are collected on a single "Tables" sheet (with separators);
// other content goes on "Document". The returned sheet is the Document
// (or Tables when the file is table-only).
func ImportMarkupFile(path string) (*Sheet, error) {
	wb, err := ImportMarkupWorkbook(path)
	if err != nil {
		return nil, err
	}
	return wb.GetActiveSheet(), nil
}

// ImportMarkupWorkbook imports Markup into a workbook (Document + Tables).
func ImportMarkupWorkbook(path string) (*Workbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if base == "" {
		base = "Import"
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return ImportHTMLWorkbook(data, base)
	case ".md", ".markdown":
		return ImportMarkdownWorkbook(data, base)
	default:
		trim := bytes.TrimSpace(data)
		lower := bytes.ToLower(trim)
		n := len(lower)
		if n > 512 {
			n = 512
		}
		head := lower[:n]
		if bytes.HasPrefix(lower, []byte("<!doctype html")) ||
			bytes.HasPrefix(lower, []byte("<html")) ||
			bytes.Contains(head, []byte("<table")) {
			return ImportHTMLWorkbook(data, base)
		}
		return ImportMarkdownWorkbook(data, base)
	}
}

// ImportMarkdown parses GitHub-Flavored Markdown-ish text into a sheet (Document + Tables).
func ImportMarkdown(data []byte) (*Sheet, error) {
	wb, err := ImportMarkdownWorkbook(data, "Import")
	if err != nil {
		return nil, err
	}
	return wb.GetActiveSheet(), nil
}

// ImportMarkdownWorkbook parses Markdown into a workbook.
// Prose / headings land on "Document"; all GFM pipe tables are stacked on one
// "Tables" sheet, separated by a banner row (heading name) and blank rows.
func ImportMarkdownWorkbook(data []byte, wbName string) (*Workbook, error) {
	wb := NewWorkbook(wbName)
	doc := wb.Sheets[0]
	doc.SetName("Document")

	text := string(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	var tables *Sheet
	tablesRow := 0
	ensureTables := func() *Sheet {
		if tables == nil {
			tables = wb.AddSheet("Tables")
		}
		return tables
	}

	row := 0
	inFence := false
	tableCount := 0
	lastHeading := ""
	hasProse := false
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			if body := strings.TrimSpace(strings.TrimLeft(trimmed, "`~")); body != "" && inFence {
				writeLabel(doc, 0, row, body)
				row++
				hasProse = true
			}
			i++
			continue
		}
		if inFence {
			if trimmed != "" {
				writeLabel(doc, 0, row, line)
				row++
				hasProse = true
			}
			i++
			continue
		}

		if isMarkdownTableRow(trimmed) {
			tableLines := []string{trimmed}
			j := i + 1
			for j < len(lines) {
				t := strings.TrimSpace(lines[j])
				if !isMarkdownTableRow(t) {
					break
				}
				tableLines = append(tableLines, t)
				j++
			}
			title := lastHeading
			tableCount++
			if title == "" {
				title = fmt.Sprintf("Table %d", tableCount)
			}
			ts := ensureTables()
			start := appendTableBlock(ts, &tablesRow, title, tableCount > 1)
			wrote := writeMarkdownTable(ts, start, tableLines)
			if wrote > 0 {
				tablesRow = start + wrote
				addr := coord.CellRef{Col: 0, Row: start}.String()
				writeLabel(doc, 0, row, fmt.Sprintf("→ Tables!%s (%s, %d rows)", addr, title, wrote))
				row += 2
			} else {
				tableCount--
				if tableCount == 0 && tables != nil && len(wb.Sheets) > 1 {
					wb.Sheets = wb.Sheets[:len(wb.Sheets)-1]
					tables = nil
					tablesRow = 0
				}
				writeLabel(doc, 0, row, unescapeMarkdownInline(trimmed))
				row++
				hasProse = true
				i++
				continue
			}
			i = j
			continue
		}

		if trimmed == "" {
			i++
			continue
		}
		if h := markdownHeadingText(trimmed); h != "" {
			lastHeading = h
		}
		writeLabel(doc, 0, row, unescapeMarkdownInline(trimmed))
		row++
		hasProse = true
		i++
	}

	if tables != nil {
		autofitSheetColumns(tables)
	}

	// Pure-table file: drop empty Document and activate Tables.
	if !hasProse && tables != nil {
		wb.Sheets = wb.Sheets[1:]
		for _, sh := range wb.Sheets {
			sh.SetWorkbook(wb)
		}
		wb.ActiveSheetIndex = 0
	}

	wb.RecalculateAll()
	clearWorkbookModified(wb)
	return wb, nil
}

func clearWorkbookModified(wb *Workbook) {
	if wb == nil {
		return
	}
	for _, s := range wb.Sheets {
		if s != nil {
			s.SetModified(false)
		}
	}
}

// appendTableBlock writes a visible separator before a table and returns the
// row where table data should start. When addGap is true (2nd+ table), a blank
// row is inserted first.
func appendTableBlock(s *Sheet, tablesRow *int, title string, addGap bool) int {
	r := *tablesRow
	if addGap {
		r++ // blank row between tables
	}
	writeLabel(s, 0, r, "══ "+title+" ══")
	r++
	*tablesRow = r
	return r
}

func markdownHeadingText(line string) string {
	if !strings.HasPrefix(line, "#") {
		return ""
	}
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return ""
	}
	if i < len(line) && line[i] != ' ' && line[i] != '\t' {
		return ""
	}
	return strings.TrimSpace(unescapeMarkdownInline(line[i:]))
}

func autofitSheetColumns(s *Sheet) {
	maxCol := s.MaxPopulatedCol()
	if maxCol < 0 {
		return
	}
	for c := 0; c <= maxCol; c++ {
		width := 4
		for r := 0; r <= s.MaxPopulatedRow(); r++ {
			cell := s.GetCell(c, r)
			if cell == nil {
				continue
			}
			w := runewidth.StringWidth(fmt.Sprintf("%v", cell.Value))
			if w+2 > width {
				width = w + 2
			}
		}
		if width > 48 {
			width = 48
		}
		if width < 6 {
			width = 6
		}
		s.SetColWidth(c, width)
	}
}

func isMarkdownTableRow(line string) bool {
	return line != "" && strings.HasPrefix(line, "|")
}

func isMarkdownSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		t := strings.TrimSpace(c)
		if t == "" {
			return false
		}
		for _, r := range t {
			if r != '-' && r != ':' && r != ' ' {
				return false
			}
		}
		if !strings.Contains(t, "-") {
			return false
		}
	}
	return true
}

func splitMarkdownRow(line string) []string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "|") {
		line = line[1:]
	}
	if strings.HasSuffix(line, "|") {
		line = line[:len(line)-1]
	}
	var cells []string
	var b strings.Builder
	escaped := false
	for _, r := range line {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '|' {
			cells = append(cells, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	cells = append(cells, strings.TrimSpace(b.String()))
	return cells
}

func writeMarkdownTable(s *Sheet, startRow int, tableLines []string) int {
	if len(tableLines) == 0 {
		return 0
	}
	parsed := make([][]string, 0, len(tableLines))
	for _, ln := range tableLines {
		cells := splitMarkdownRow(ln)
		if isMarkdownSeparatorRow(cells) {
			continue
		}
		parsed = append(parsed, cells)
	}
	if len(parsed) == 0 {
		return 0
	}
	for r, rowCells := range parsed {
		for c, val := range rowCells {
			val = unescapeMarkdownInline(val)
			val = html.UnescapeString(val)
			if strings.TrimSpace(val) == "" {
				continue
			}
			writeTableCell(s, c, startRow+r, val)
		}
	}
	return len(parsed)
}

var mdLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
var mdImageRe = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)

func unescapeMarkdownInline(s string) string {
	s = mdImageRe.ReplaceAllString(s, "$1")
	s = mdLinkRe.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, `\|`, "|")
	return strings.TrimSpace(s)
}

var (
	htmlCommentRe = regexp.MustCompile(`(?is)<!--.*?-->`)
	htmlScriptRe  = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	htmlStyleRe   = regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`)
	htmlTableRe   = regexp.MustCompile(`(?is)<table\b[^>]*>.*?</table>`)
	htmlTagRe     = regexp.MustCompile(`(?is)<[^>]+>`)
	htmlBlockRe   = regexp.MustCompile(`(?is)</?(?:p|div|h[1-6]|li|tr|br|hr|blockquote|pre|section|article|header|footer|ul|ol|dl|dt|dd)\b[^>]*>`)
)

// ImportHTML parses HTML into a sheet (Document + tables).
func ImportHTML(data []byte) (*Sheet, error) {
	wb, err := ImportHTMLWorkbook(data, "Import")
	if err != nil {
		return nil, err
	}
	return wb.GetActiveSheet(), nil
}

// ImportHTMLWorkbook parses HTML into a workbook (Document + one Tables sheet).
func ImportHTMLWorkbook(data []byte, wbName string) (*Workbook, error) {
	wb := NewWorkbook(wbName)
	doc := wb.Sheets[0]
	doc.SetName("Document")

	src := string(data)
	src = htmlCommentRe.ReplaceAllString(src, "")
	src = htmlScriptRe.ReplaceAllString(src, "")
	src = htmlStyleRe.ReplaceAllString(src, "")

	var tables *Sheet
	tablesRow := 0
	ensureTables := func() *Sheet {
		if tables == nil {
			tables = wb.AddSheet("Tables")
		}
		return tables
	}

	row := 0
	last := 0
	tableCount := 0
	hasProse := false
	matches := htmlTableRe.FindAllStringIndex(src, -1)
	for _, loc := range matches {
		before := src[last:loc[0]]
		prevRow := row
		row = writeHTMLProse(doc, row, before)
		if row > prevRow {
			hasProse = true
		}
		tableHTML := src[loc[0]:loc[1]]
		tableCount++
		ts := ensureTables()
		title := fmt.Sprintf("Table %d", tableCount)
		start := appendTableBlock(ts, &tablesRow, title, tableCount > 1)
		wrote := writeHTMLTableFromHTML(ts, start, tableHTML)
		if wrote > 0 {
			tablesRow = start + wrote
			addr := coord.CellRef{Col: 0, Row: start}.String()
			writeLabel(doc, 0, row, fmt.Sprintf("→ Tables!%s (%s, %d rows)", addr, title, wrote))
			row += 2
		} else {
			tableCount--
			if tableCount == 0 && tables != nil && len(wb.Sheets) > 1 {
				wb.Sheets = wb.Sheets[:len(wb.Sheets)-1]
				tables = nil
				tablesRow = 0
			}
		}
		last = loc[1]
	}
	prevRow := row
	row = writeHTMLProse(doc, row, src[last:])
	if row > prevRow {
		hasProse = true
	}

	if tables != nil {
		autofitSheetColumns(tables)
	}

	if !hasProse && tables != nil {
		wb.Sheets = wb.Sheets[1:]
		for _, sh := range wb.Sheets {
			sh.SetWorkbook(wb)
		}
		wb.ActiveSheetIndex = 0
	}

	wb.RecalculateAll()
	clearWorkbookModified(wb)
	return wb, nil
}

func writeHTMLProse(s *Sheet, row int, fragment string) int {
	if strings.TrimSpace(stripHTMLTags(fragment)) == "" {
		return row
	}
	frag := htmlBlockRe.ReplaceAllStringFunc(fragment, func(tag string) string {
		lower := strings.ToLower(tag)
		if strings.HasPrefix(lower, "<br") || strings.HasPrefix(lower, "<hr") || strings.HasPrefix(lower, "</") {
			return "\n"
		}
		return "\n"
	})
	frag = stripHTMLTags(frag)
	frag = html.UnescapeString(frag)
	for _, line := range strings.Split(frag, "\n") {
		line = collapseSpace(line)
		if line == "" {
			continue
		}
		writeLabel(s, 0, row, line)
		row++
	}
	return row
}

func writeHTMLTableFromHTML(s *Sheet, startRow int, tableHTML string) int {
	rowRe := regexp.MustCompile(`(?is)<tr\b[^>]*>(.*?)</tr>`)
	cellRe := regexp.MustCompile(`(?is)<t[hd]\b[^>]*>(.*?)</t[hd]>`)
	rowMatches := rowRe.FindAllStringSubmatch(tableHTML, -1)
	wrote := 0
	for _, rm := range rowMatches {
		cells := cellRe.FindAllStringSubmatch(rm[1], -1)
		if len(cells) == 0 {
			continue
		}
		for c, cm := range cells {
			val := collapseSpace(html.UnescapeString(stripHTMLTags(cm[1])))
			if val == "" {
				continue
			}
			writeTableCell(s, c, startRow+wrote, val)
		}
		wrote++
	}
	return wrote
}

func stripHTMLTags(s string) string {
	return htmlTagRe.ReplaceAllString(s, "")
}

func collapseSpace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		prevSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func writeLabel(s *Sheet, col, row int, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	// Always force a left-aligned LABEL so prose / JSON / formula-looking text
	// is never evaluated or treated as right/center alignment prefixes.
	s.SetCellInput(col, row, "'"+text, nil)
}

func writeTableCell(s *Sheet, col, row int, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.SetCellInput(col, row, text, nil)
}
