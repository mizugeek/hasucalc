package coord

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	MaxCols = 16384
	MaxRows = 1048576
)

// ColToLetter converts 0-based column index to letter (0 -> "A", 25 -> "Z", 26 -> "AA").
func ColToLetter(col int) string {
	if col < 0 {
		return "A"
	}
	result := ""
	col++
	for col > 0 {
		col--
		rem := col % 26
		result = string(rune('A'+rem)) + result
		col /= 26
	}
	return result
}

// LetterToCol converts column letters to 0-based column index ("A" -> 0, "Z" -> 25, "AA" -> 26).
func LetterToCol(letters string) int {
	letters = strings.ToUpper(strings.TrimSpace(letters))
	result := 0
	for _, ch := range letters {
		if ch < 'A' || ch > 'Z' {
			return 0
		}
		result = result*26 + int(ch-'A'+1)
	}
	return result - 1
}

var cellRefRegex = regexp.MustCompile(`^(?:(?:'((?:[^']|'')*)'|([^\s!']+))!)?(\$?)([A-Za-z]+)(\$?)([0-9]+)$`)

// UnquoteSheetName strips Excel-style sheet quotes and unescapes doubled apostrophes.
func UnquoteSheetName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	return s
}

func needsSheetQuotes(name string) bool {
	if name == "" {
		return false
	}
	runes := []rune(name)
	if !isUnquotedSheetStart(runes[0]) {
		return true
	}
	for _, r := range runes[1:] {
		if !isUnquotedSheetCont(r) {
			return true
		}
	}
	// Names that parse as A1 refs (A1, Q1, XFD1048576) must be quoted.
	if _, err := ParseCellRef(name); err == nil {
		return true
	}
	return false
}

func isUnquotedSheetStart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || r == '_' || r == '$'
}

func isUnquotedSheetCont(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r) || r == '_' || r == '$' || r == '.'
}

// QuoteSheetPrefix returns "Sheet!" or "'O”Brien'!" for use in A1 references.
func QuoteSheetPrefix(name string) string {
	if name == "" {
		return ""
	}
	if needsSheetQuotes(name) {
		return "'" + strings.ReplaceAll(name, "'", "''") + "'!"
	}
	return name + "!"
}

// CellRef represents a cell coordinate with optional absolute markers ($A$1) and optional Sheet name.
type CellRef struct {
	Sheet  string // optional sheet name, e.g. "Year" in Year!B4
	Col    int    // 0-based
	Row    int    // 0-based
	ColAbs bool   // $A
	RowAbs bool   // $1
}

func ParseCellRef(s string) (CellRef, error) {
	s = strings.TrimSpace(s)
	matches := cellRefRegex.FindStringSubmatch(s)
	if len(matches) != 7 {
		return CellRef{}, fmt.Errorf("invalid cell reference: %s", s)
	}
	sheetName := strings.ReplaceAll(matches[1], "''", "'")
	if sheetName == "" {
		sheetName = matches[2]
	}
	colAbs := matches[3] == "$"
	colLetters := matches[4]
	rowAbs := matches[5] == "$"
	rowNum, err := strconv.Atoi(matches[6])
	if err != nil || rowNum < 1 {
		return CellRef{}, fmt.Errorf("invalid row number in cell reference: %s", s)
	}

	col := LetterToCol(colLetters)
	row := rowNum - 1
	if col < 0 || col >= MaxCols || row < 0 || row >= MaxRows {
		return CellRef{}, fmt.Errorf("cell reference out of bounds: %s", s)
	}
	return CellRef{
		Sheet:  sheetName,
		Col:    col,
		Row:    row,
		ColAbs: colAbs,
		RowAbs: rowAbs,
	}, nil
}

func (c CellRef) String() string {
	sheetPrefix := QuoteSheetPrefix(c.Sheet)
	cPrefix := ""
	if c.ColAbs {
		cPrefix = "$"
	}
	rPrefix := ""
	if c.RowAbs {
		rPrefix = "$"
	}
	return fmt.Sprintf("%s%s%s%s%d", sheetPrefix, cPrefix, ColToLetter(c.Col), rPrefix, c.Row+1)
}

// RangeRef represents a rectangular range from Start to End (e.g. A1..B10 or Year!A1..B10).
type RangeRef struct {
	Sheet string // optional sheet name
	Start CellRef
	End   CellRef
}

func isAllLetters(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}

func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func ParseRangeRef(s string) (RangeRef, error) {
	s = strings.TrimSpace(s)
	sheetPrefix := ""
	cellPart := s
	if idx := strings.LastIndex(s, "!"); idx != -1 {
		sheetPart := strings.TrimSpace(s[:idx])
		sheetPrefix = UnquoteSheetName(sheetPart)
		cellPart = strings.TrimSpace(s[idx+1:])
	}

	if strings.Contains(cellPart, "..") || strings.Contains(cellPart, ":") {
		sep := ".."
		if strings.Contains(cellPart, ":") {
			sep = ":"
		}
		parts := strings.Split(cellPart, sep)
		if len(parts) != 2 {
			return RangeRef{}, fmt.Errorf("invalid range reference: %s", s)
		}
		p0 := strings.TrimSpace(parts[0])
		p1 := strings.TrimSpace(parts[1])

		// Check whole column reference (e.g. A:B or A:A or $B:$C)
		if isAllLetters(strings.TrimPrefix(p0, "$")) && isAllLetters(strings.TrimPrefix(p1, "$")) {
			col1 := LetterToCol(strings.TrimPrefix(p0, "$"))
			col2 := LetterToCol(strings.TrimPrefix(p1, "$"))
			c1 := CellRef{Sheet: sheetPrefix, Col: col1, Row: 0, ColAbs: strings.HasPrefix(p0, "$")}
			c2 := CellRef{Sheet: sheetPrefix, Col: col2, Row: 1048575, ColAbs: strings.HasPrefix(p1, "$")}
			return RangeRef{Sheet: sheetPrefix, Start: c1, End: c2}, nil
		}

		// Check whole row reference (e.g. 1:10)
		if isAllDigits(strings.TrimPrefix(p0, "$")) && isAllDigits(strings.TrimPrefix(p1, "$")) {
			r1, _ := strconv.Atoi(strings.TrimPrefix(p0, "$"))
			r2, _ := strconv.Atoi(strings.TrimPrefix(p1, "$"))
			if r1 < 1 {
				r1 = 1
			}
			if r2 < 1 {
				r2 = 1
			}
			c1 := CellRef{Sheet: sheetPrefix, Col: 0, Row: r1 - 1, RowAbs: strings.HasPrefix(p0, "$")}
			c2 := CellRef{Sheet: sheetPrefix, Col: 16383, Row: r2 - 1, RowAbs: strings.HasPrefix(p1, "$")}
			return RangeRef{Sheet: sheetPrefix, Start: c1, End: c2}, nil
		}

		c1, err1 := ParseCellRef(p0)
		c2, err2 := ParseCellRef(p1)
		if err1 != nil || err2 != nil {
			return RangeRef{}, fmt.Errorf("invalid range coordinates: %s", s)
		}
		c1.Sheet = sheetPrefix
		c2.Sheet = sheetPrefix
		return RangeRef{Sheet: sheetPrefix, Start: c1, End: c2}, nil
	}

	c, err := ParseCellRef(s)
	if err != nil {
		return RangeRef{}, err
	}
	return RangeRef{Sheet: c.Sheet, Start: c, End: c}, nil
}

func (r RangeRef) MinCol() int {
	if r.Start.Col < r.End.Col {
		return r.Start.Col
	}
	return r.End.Col
}

func (r RangeRef) MaxCol() int {
	if r.Start.Col > r.End.Col {
		return r.Start.Col
	}
	return r.End.Col
}

func (r RangeRef) MinRow() int {
	if r.Start.Row < r.End.Row {
		return r.Start.Row
	}
	return r.End.Row
}

func (r RangeRef) MaxRow() int {
	if r.Start.Row > r.End.Row {
		return r.Start.Row
	}
	return r.End.Row
}

func (r RangeRef) Contains(col, row int) bool {
	return col >= r.MinCol() && col <= r.MaxCol() && row >= r.MinRow() && row <= r.MaxRow()
}

func (r RangeRef) Cells() []CellRef {
	minR := r.MinRow()
	maxR := r.MaxRow()
	minC := r.MinCol()
	maxC := r.MaxCol()
	// Avoid allocating huge grids for whole-column/row refs (A:A, 1:1).
	const maxExpand = 200000
	rows := maxR - minR + 1
	cols := maxC - minC + 1
	if rows > 0 && cols > 0 && rows*cols > maxExpand {
		if r.IsWholeColumn() {
			maxR = minR + maxExpand/cols - 1
		} else if r.IsWholeRow() {
			maxC = minC + maxExpand/rows - 1
		} else {
			maxR = minR + 10000
			if maxR > r.MaxRow() {
				maxR = r.MaxRow()
			}
			maxC = minC + 1000
			if maxC > r.MaxCol() {
				maxC = r.MaxCol()
			}
		}
	}
	var list []CellRef
	for row := minR; row <= maxR; row++ {
		for col := minC; col <= maxC; col++ {
			list = append(list, CellRef{Sheet: r.Sheet, Col: col, Row: row})
		}
	}
	return list
}

func (r RangeRef) IsWholeColumn() bool {
	return r.MinRow() == 0 && r.MaxRow() >= 1048575
}

func (r RangeRef) IsWholeRow() bool {
	return r.MinCol() == 0 && r.MaxCol() >= 16383
}

func (r RangeRef) String() string {
	sheet := r.Sheet
	if sheet == "" {
		sheet = r.Start.Sheet
	}
	sheetPrefix := QuoteSheetPrefix(sheet)

	if r.IsWholeColumn() {
		c1Prefix := ""
		if r.Start.ColAbs {
			c1Prefix = "$"
		}
		c2Prefix := ""
		if r.End.ColAbs {
			c2Prefix = "$"
		}
		return fmt.Sprintf("%s%s%s:%s%s", sheetPrefix, c1Prefix, ColToLetter(r.Start.Col), c2Prefix, ColToLetter(r.End.Col))
	}

	if r.IsWholeRow() {
		r1Prefix := ""
		if r.Start.RowAbs {
			r1Prefix = "$"
		}
		r2Prefix := ""
		if r.End.RowAbs {
			r2Prefix = "$"
		}
		return fmt.Sprintf("%s%s%d:%s%d", sheetPrefix, r1Prefix, r.Start.Row+1, r2Prefix, r.End.Row+1)
	}

	startCopy := r.Start
	startCopy.Sheet = ""
	if r.Start == r.End {
		return fmt.Sprintf("%s%s", sheetPrefix, startCopy.String())
	}
	endCopy := r.End
	endCopy.Sheet = ""
	return fmt.Sprintf("%s%s:%s", sheetPrefix, startCopy.String(), endCopy.String())
}

var r1c1BodyRe = regexp.MustCompile(`(?i)^R(?:\[(-?\d+)\]|(\d+))?C(?:\[(-?\d+)\]|(\d+))?$`)

func splitSheetPrefix(s string) (sheet, rest string) {
	s = strings.TrimSpace(s)
	bang := strings.LastIndex(s, "!")
	if bang == -1 {
		return "", s
	}
	return UnquoteSheetName(s[:bang]), strings.TrimSpace(s[bang+1:])
}

func parseR1C1Part(bracket, abs string, origin int, isCol bool) (idx int, absRef bool, err error) {
	if bracket != "" {
		n, convErr := strconv.Atoi(bracket)
		if convErr != nil {
			return 0, false, convErr
		}
		return origin + n, false, nil
	}
	if abs != "" {
		n, convErr := strconv.Atoi(abs)
		if convErr != nil {
			return 0, false, convErr
		}
		if n < 1 {
			if isCol {
				return 0, false, fmt.Errorf("invalid R1C1 column")
			}
			return 0, false, fmt.Errorf("invalid R1C1 row")
		}
		return n - 1, true, nil
	}
	return origin, false, nil
}

func parseR1C1Body(body, sheet string, originCol, originRow int) (CellRef, error) {
	body = strings.TrimSpace(body)
	m := r1c1BodyRe.FindStringSubmatch(body)
	if len(m) != 5 {
		return CellRef{}, fmt.Errorf("invalid R1C1 reference: %s", body)
	}
	row, rowAbs, err := parseR1C1Part(m[1], m[2], originRow, false)
	if err != nil {
		return CellRef{}, err
	}
	col, colAbs, err := parseR1C1Part(m[3], m[4], originCol, true)
	if err != nil {
		return CellRef{}, err
	}
	if col < 0 || col >= MaxCols || row < 0 || row >= MaxRows {
		return CellRef{}, fmt.Errorf("R1C1 out of bounds: %s", body)
	}
	return CellRef{Sheet: sheet, Col: col, Row: row, ColAbs: colAbs, RowAbs: rowAbs}, nil
}

// ParseR1C1CellRef parses an Excel R1C1 cell address relative to originCol/originRow (0-based).
func ParseR1C1CellRef(s string, originCol, originRow int) (CellRef, error) {
	sheet, body := splitSheetPrefix(s)
	if strings.Contains(body, ":") {
		return CellRef{}, fmt.Errorf("invalid R1C1 cell reference: %s", s)
	}
	return parseR1C1Body(body, sheet, originCol, originRow)
}

// ParseR1C1RangeRef parses an Excel R1C1 cell or range (R1C1:R5C2) relative to origin.
func ParseR1C1RangeRef(s string, originCol, originRow int) (RangeRef, error) {
	sheet, body := splitSheetPrefix(s)
	parts := strings.Split(body, ":")
	if len(parts) == 1 {
		cr, err := parseR1C1Body(parts[0], sheet, originCol, originRow)
		if err != nil {
			return RangeRef{}, err
		}
		return RangeRef{Sheet: sheet, Start: cr, End: cr}, nil
	}
	if len(parts) != 2 {
		return RangeRef{}, fmt.Errorf("invalid R1C1 range: %s", s)
	}
	c1, err1 := parseR1C1Body(parts[0], sheet, originCol, originRow)
	if err1 != nil {
		return RangeRef{}, err1
	}
	c2, err2 := parseR1C1Body(parts[1], sheet, originCol, originRow)
	if err2 != nil {
		return RangeRef{}, err2
	}
	c1.Sheet, c2.Sheet = sheet, sheet
	return RangeRef{Sheet: sheet, Start: c1, End: c2}, nil
}
