package sheet

import (
	"bytes"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// ImportMarkupFile imports Markdown (.md/.markdown) or HTML (.html/.htm) into a sheet.
// GFM/HTML tables become multi-column cells; other content is stored as labels in column A.
func ImportMarkupFile(path string) (*Sheet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".html", ".htm":
		return ImportHTML(data)
	case ".md", ".markdown":
		return ImportMarkdown(data)
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
			return ImportHTML(data)
		}
		return ImportMarkdown(data)
	}
}

// ImportMarkdown parses GitHub-Flavored Markdown-ish text into a sheet.
func ImportMarkdown(data []byte) (*Sheet, error) {
	s := NewSheet()
	text := string(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")

	row := 0
	inFence := false
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			if body := strings.TrimSpace(strings.TrimLeft(trimmed, "`~")); body != "" && inFence {
				writeLabel(s, 0, row, body)
				row++
			}
			i++
			continue
		}
		if inFence {
			if trimmed != "" {
				writeLabel(s, 0, row, line)
				row++
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
			wrote := writeMarkdownTable(s, row, tableLines)
			if wrote > 0 {
				row += wrote
				row++ // blank spacer after table
			}
			i = j
			continue
		}

		if trimmed == "" {
			i++
			continue
		}
		writeLabel(s, 0, row, unescapeMarkdownInline(trimmed))
		row++
		i++
	}

	s.Recalculate()
	return s, nil
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

// ImportHTML parses HTML and maps <table> grids to cells; other block text becomes labels.
func ImportHTML(data []byte) (*Sheet, error) {
	s := NewSheet()
	src := string(data)
	src = htmlCommentRe.ReplaceAllString(src, "")
	src = htmlScriptRe.ReplaceAllString(src, "")
	src = htmlStyleRe.ReplaceAllString(src, "")

	row := 0
	last := 0
	matches := htmlTableRe.FindAllStringIndex(src, -1)
	for _, loc := range matches {
		before := src[last:loc[0]]
		row = writeHTMLProse(s, row, before)
		tableHTML := src[loc[0]:loc[1]]
		wrote := writeHTMLTableFromHTML(s, row, tableHTML)
		if wrote > 0 {
			row += wrote
			row++
		}
		last = loc[1]
	}
	row = writeHTMLProse(s, row, src[last:])
	s.Recalculate()
	return s, nil
}

func writeHTMLProse(s *Sheet, row int, fragment string) int {
	if strings.TrimSpace(stripHTMLTags(fragment)) == "" {
		return row
	}
	// Normalize block boundaries into newlines, then label each non-empty line.
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
	// Force LABEL so headings / prose / formula-looking text are not evaluated.
	if !strings.HasPrefix(text, "'") && !strings.HasPrefix(text, `"`) && !strings.HasPrefix(text, "^") {
		text = "'" + text
	}
	s.SetCellInput(col, row, text, nil)
}

func writeTableCell(s *Sheet, col, row int, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	s.SetCellInput(col, row, text, nil)
}
