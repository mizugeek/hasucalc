package sheet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattn/go-runewidth"
	"hasucalc/cell"
)

// RenderMarkdownTableRange renders a rectangular range of cells as an aligned GitHub Flavored Markdown table.
func (s *Sheet) RenderMarkdownTableRange(minCol, minRow, maxCol, maxRow int) string {
	if minCol > maxCol {
		minCol, maxCol = maxCol, minCol
	}
	if minRow > maxRow {
		minRow, maxRow = maxRow, minRow
	}

	numCols := maxCol - minCol + 1
	numRows := maxRow - minRow + 1
	if numCols <= 0 || numRows <= 0 {
		return ""
	}

	// 1. Collect cell formatted strings and determine alignment
	grid := make([][]string, numRows)
	colAligns := make([]cell.Alignment, numCols)
	colWidths := make([]int, numCols)

	for rIdx := 0; rIdx < numRows; rIdx++ {
		r := minRow + rIdx
		grid[rIdx] = make([]string, numCols)
		for cIdx := 0; cIdx < numCols; cIdx++ {
			c := minCol + cIdx
			cData := s.cells[CellCoord{Col: c, Row: r}]
			valStr := ""
			if cData != nil && cData.Value != nil && cData.Type != cell.TypeEmpty {
				valStr = formatMarkdownCellValue(cData, s.globalFormat)
				if colAligns[cIdx] == "" && cData.Alignment != "" {
					colAligns[cIdx] = cData.Alignment
				} else if colAligns[cIdx] == "" && cData.Type == cell.TypeNumber {
					colAligns[cIdx] = cell.AlignRight
				}
			}

			// Clean for Markdown
			valStr = cleanMarkdownText(valStr)
			grid[rIdx][cIdx] = valStr

			w := runewidth.StringWidth(valStr)
			if w < 3 {
				w = 3 // Minimum width for markdown separator dashes
			}
			if w > colWidths[cIdx] {
				colWidths[cIdx] = w
			}
		}
	}

	var sb strings.Builder

	// 2. Render Header Row (Row 0 of the range)
	sb.WriteString("|")
	for cIdx := 0; cIdx < numCols; cIdx++ {
		cellText := grid[0][cIdx]
		align := colAligns[cIdx]
		if align == "" {
			align = cell.AlignLeft
		}
		padded := padMarkdownCell(cellText, colWidths[cIdx], align)
		sb.WriteString(" " + padded + " |")
	}
	sb.WriteString("\n")

	// 3. Render Separator Row
	sb.WriteString("|")
	for cIdx := 0; cIdx < numCols; cIdx++ {
		align := colAligns[cIdx]
		w := colWidths[cIdx]
		var sep string
		switch align {
		case cell.AlignRight:
			sep = strings.Repeat("-", w-1) + ":"
		case cell.AlignCenter:
			sep = ":" + strings.Repeat("-", w-2) + ":"
		default: // AlignLeft or default
			sep = ":" + strings.Repeat("-", w-1)
		}
		sb.WriteString(" " + sep + " |")
	}
	sb.WriteString("\n")

	// 4. Render Data Rows (Subsequent rows)
	for rIdx := 1; rIdx < numRows; rIdx++ {
		sb.WriteString("|")
		for cIdx := 0; cIdx < numCols; cIdx++ {
			cellText := grid[rIdx][cIdx]
			align := colAligns[cIdx]
			if align == "" {
				align = cell.AlignLeft
			}
			padded := padMarkdownCell(cellText, colWidths[cIdx], align)
			sb.WriteString(" " + padded + " |")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// ExportMarkdownRange exports a rectangular range of cells from the sheet as a Markdown table.
func (s *Sheet) ExportMarkdownRange(filepathStr string, minCol, minRow, maxCol, maxRow int) error {
	if dir := filepath.Dir(filepathStr); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	mdContent := s.RenderMarkdownTableRange(minCol, minRow, maxCol, maxRow)
	return os.WriteFile(filepathStr, []byte(mdContent), 0644)
}

// ExportMarkdown exports the populated bounding box of the sheet as a Markdown table.
func (s *Sheet) ExportMarkdown(filepathStr string) error {
	if dir := filepath.Dir(filepathStr); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	if len(s.cells) == 0 {
		return os.WriteFile(filepathStr, []byte(""), 0644)
	}

	minC, minR, maxC, maxR := 0, 0, 0, 0
	hasCells := false
	for pt, c := range s.cells {
		if c == nil || c.Type == cell.TypeEmpty {
			continue
		}
		if !hasCells {
			minC, maxC = pt.Col, pt.Col
			minR, maxR = pt.Row, pt.Row
			hasCells = true
		} else {
			if pt.Col < minC {
				minC = pt.Col
			}
			if pt.Col > maxC {
				maxC = pt.Col
			}
			if pt.Row < minR {
				minR = pt.Row
			}
			if pt.Row > maxR {
				maxR = pt.Row
			}
		}
	}
	if !hasCells {
		return os.WriteFile(filepathStr, []byte(""), 0644)
	}

	return s.ExportMarkdownRange(filepathStr, minC, minR, maxC, maxR)
}

func formatMarkdownCellValue(c *cell.Cell, defaultFmt cell.CellFormat) string {
	fmtSpec := defaultFmt
	if c.FormatSpec != nil {
		fmtSpec = *c.FormatSpec
	}

	switch v := c.Value.(type) {
	case float64:
		return cell.FormatNumber(v, fmtSpec)
	case int:
		return cell.FormatNumber(float64(v), fmtSpec)
	case string:
		return v
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	case cell.LotusError:
		return v.Code
	default:
		if c.RawInput != "" {
			raw := c.RawInput
			if strings.HasPrefix(raw, "'") || strings.HasPrefix(raw, "\"") || strings.HasPrefix(raw, "^") || strings.HasPrefix(raw, "\\") {
				return raw[1:]
			}
			return raw
		}
		return fmt.Sprintf("%v", v)
	}
}

func cleanMarkdownText(s string) string {
	// Escape pipes and replace newlines
	s = strings.ReplaceAll(s, "|", `\|`)
	s = strings.ReplaceAll(s, "\r\n", "<br>")
	s = strings.ReplaceAll(s, "\n", "<br>")
	return strings.TrimSpace(s)
}

func padMarkdownCell(text string, width int, align cell.Alignment) string {
	tw := runewidth.StringWidth(text)
	if tw >= width {
		return text
	}
	diff := width - tw
	switch align {
	case cell.AlignRight:
		return strings.Repeat(" ", diff) + text
	case cell.AlignCenter:
		left := diff / 2
		right := diff - left
		return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
	default: // Left
		return text + strings.Repeat(" ", diff)
	}
}
