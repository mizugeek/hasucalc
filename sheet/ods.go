package sheet

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// GetODSSheetList returns the list of table/sheet names in a .ods file.
func GetODSSheetList(filename string) ([]string, error) {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open ods file: %w", err)
	}
	defer zr.Close()

	contentFile := findZipFile(zr, "content.xml")
	if contentFile == nil {
		return nil, fmt.Errorf("content.xml not found in ods archive")
	}

	rc, err := contentFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var sheetNames []string
	decoder := xml.NewDecoder(rc)

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local == "table" {
				for _, attr := range se.Attr {
					if attr.Name.Local == "name" {
						sheetNames = append(sheetNames, attr.Value)
					}
				}
			}
		}
	}

	if len(sheetNames) == 0 {
		sheetNames = append(sheetNames, "Sheet1")
	}
	return sheetNames, nil
}

// ImportODS imports a specific sheet (0-indexed) from a .ods file into a new Sheet.
func ImportODS(filename string, sheetIdx int) (*Sheet, error) {
	return ImportODSWithProgress(filename, sheetIdx, nil)
}

// ImportODSWithProgress imports a sheet from a .ods file with progress updates.
func ImportODSWithProgress(filename string, sheetIdx int, onProgress func(step string)) (*Sheet, error) {
	wb, err := ImportODSWorkbookWithProgress(filename, onProgress)
	if err != nil {
		return nil, err
	}
	if len(wb.Sheets) == 0 {
		return nil, fmt.Errorf("ods file contains no worksheets")
	}
	if sheetIdx < 0 || sheetIdx >= len(wb.Sheets) {
		sheetIdx = 0
	}
	wb.ActiveSheetIndex = sheetIdx
	return wb.Sheets[sheetIdx], nil
}

// ImportODSWorkbook imports all sheets from a .ods file into a Workbook.
func ImportODSWorkbook(filename string) (*Workbook, error) {
	return ImportODSWorkbookWithProgress(filename, nil)
}

// ImportODSWorkbookWithProgress imports all sheets from a .ods file with progress updates.
// ODS charts are ignored; HasuCalc graphs are not reconstructed from them.
func ImportODSWorkbookWithProgress(filename string, onProgress func(step string)) (*Workbook, error) {
	if onProgress != nil {
		onProgress("Opening LibreOffice ODS archive...")
	}
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open ods file: %w", err)
	}
	defer zr.Close()

	if onProgress != nil {
		onProgress("Reading ODS sheet list...")
	}
	sheetNames, err := GetODSSheetList(filename)
	if err != nil || len(sheetNames) == 0 {
		sheetNames = []string{"Sheet1"}
	}

	wb := &Workbook{
		Name:             filepath.Base(filename),
		Sheets:           make([]*Sheet, 0, len(sheetNames)),
		ActiveSheetIndex: 0,
		NamedRanges:      make(map[string]any),
	}

	contentFile := findZipFile(zr, "content.xml")

	for idx, sName := range sheetNames {
		if onProgress != nil {
			onProgress(fmt.Sprintf("Parsing ODS sheet %d/%d: '%s'...", idx+1, len(sheetNames), sName))
		}
		if contentFile == nil {
			return nil, fmt.Errorf("content.xml not found in ods archive")
		}
		rc, err := contentFile.Open()
		if err != nil {
			return nil, err
		}

		sh := NewSheet()
		sh.SetName(sName)
		sh.SetWorkbook(wb)
		sh.SetRecalcMode("MANUAL")

		if err := parseODSContentXML(rc, sh, idx); err != nil {
			rc.Close()
			return nil, fmt.Errorf("failed to parse ods sheet %s: %w", sName, err)
		}
		rc.Close()

		sh.SetRecalcMode("AUTO")
		wb.Sheets = append(wb.Sheets, sh)
	}

	if len(wb.Sheets) == 0 {
		return nil, fmt.Errorf("no sheets could be parsed from ods file")
	}

	if contentFile != nil {
		if rc, err := contentFile.Open(); err == nil {
			applyODSNamedItems(wb, parseODSNamedExpressions(rc))
			rc.Close()
		}
	}
	applyODSViewSettings(zr, wb)

	// Check if any formula cell is missing a cached value
	hasMissingCached := false
	for _, sh := range wb.Sheets {
		for _, c := range sh.cells {
			if c.Type == cell.TypeFormula && c.Value == nil {
				hasMissingCached = true
				break
			}
		}
		if hasMissingCached {
			break
		}
	}

	if hasMissingCached {
		if onProgress != nil {
			onProgress("Recalculating formulas across all worksheets...")
		}
		wb.RecalculateAll()
	}

	for _, sh := range wb.Sheets {
		sh.SetModified(false)
	}

	return wb, nil
}

func applyODSViewSettings(zr *zip.ReadCloser, wb *Workbook) {
	if zr == nil || wb == nil {
		return
	}
	sf := findZipFile(zr, "settings.xml")
	if sf == nil {
		return
	}
	rc, err := sf.Open()
	if err != nil {
		return
	}
	defer rc.Close()

	type freezeState struct {
		hMode, vMode int
		hPos, vPos   int
	}
	bySheet := map[string]*freezeState{}
	inTables := false
	curName := ""
	itemName := ""
	autoCalculate := true
	var text strings.Builder

	attrVal := func(se xml.StartElement, local string) string {
		for _, a := range se.Attr {
			if a.Name.Local == local {
				return a.Value
			}
		}
		return ""
	}

	decoder := xml.NewDecoder(rc)
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "config-item-map-named":
				if attrVal(t, "name") == "Tables" {
					inTables = true
				}
			case "config-item-map-entry":
				if inTables {
					curName = attrVal(t, "name")
					if curName != "" && bySheet[curName] == nil {
						bySheet[curName] = &freezeState{}
					}
				}
			case "config-item":
				itemName = attrVal(t, "name")
				text.Reset()
			}
		case xml.CharData:
			if itemName != "" {
				text.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "config-item":
				val := strings.TrimSpace(text.String())
				if itemName == "AutoCalculate" {
					autoCalculate = strings.EqualFold(val, "true") || val == "1"
				}
				if inTables && curName != "" && itemName != "" {
					st := bySheet[curName]
					if st != nil {
						n, _ := strconv.Atoi(val)
						switch itemName {
						case "HorizontalSplitMode":
							st.hMode = n
						case "VerticalSplitMode":
							st.vMode = n
						case "HorizontalSplitPosition":
							st.hPos = n
						case "VerticalSplitPosition":
							st.vPos = n
						}
					}
				}
				itemName = ""
			case "config-item-map-entry":
				if inTables {
					curName = ""
				}
			case "config-item-map-named":
				inTables = false
			}
		}
	}

	mode := "AUTO"
	if !autoCalculate {
		mode = "MANUAL"
	}
	for _, sh := range wb.Sheets {
		if sh == nil {
			continue
		}
		sh.SetRecalcMode(mode)
		st := bySheet[sh.Name()]
		if st == nil {
			continue
		}
		if st.hMode == 2 && st.hPos > 0 {
			sh.SetFrozenCols(st.hPos)
		}
		if st.vMode == 2 && st.vPos > 0 {
			sh.SetFrozenRows(st.vPos)
		}
	}
}

func parseODSNamedExpressions(r io.Reader) []odsNamedItem {
	decoder := xml.NewDecoder(r)
	var items []odsNamedItem
	var tableStack []string
	currentTable := func() string {
		if len(tableStack) == 0 {
			return ""
		}
		return tableStack[len(tableStack)-1]
	}
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch se := tok.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "table":
				name := ""
				for _, attr := range se.Attr {
					if attr.Name.Local == "name" {
						name = attr.Value
					}
				}
				tableStack = append(tableStack, name)
			case "named-range":
				var name, cellRangeAddr string
				for _, attr := range se.Attr {
					if attr.Name.Local == "name" {
						name = attr.Value
					} else if attr.Name.Local == "cell-range-address" {
						cellRangeAddr = attr.Value
					}
				}
				if name != "" && cellRangeAddr != "" {
					if parsed := parseOpenFormulaAddress(cellRangeAddr); parsed != nil {
						items = append(items, odsNamedItem{name: name, sheet: currentTable(), value: parsed})
					}
				}
			case "named-expression":
				var name, expr, baseAddr string
				for _, attr := range se.Attr {
					switch attr.Name.Local {
					case "name":
						name = attr.Value
					case "expression":
						expr = attr.Value
					case "base-cell-address":
						baseAddr = attr.Value
					}
				}
				if name != "" && expr != "" && expr != "#REF!" {
					convertedExpr := convertODSFormula(expr)
					ne := formula.NamedExpr{Expr: convertedExpr}
					if baseAddr != "" {
						if parsed := parseOpenFormulaAddress(baseAddr); parsed != nil {
							if cr, ok := parsed.(coord.CellRef); ok {
								ne.HasBase = true
								ne.BaseCol = cr.Col
								ne.BaseRow = cr.Row
							}
						}
					}
					items = append(items, odsNamedItem{name: name, sheet: currentTable(), value: ne})
				}
			}
		case xml.EndElement:
			if se.Name.Local == "table" && len(tableStack) > 0 {
				tableStack = tableStack[:len(tableStack)-1]
			}
		}
	}
	return items
}

type odsNamedItem struct {
	name  string
	sheet string
	value any
}

func applyODSNamedItems(wb *Workbook, items []odsNamedItem) {
	if wb == nil {
		return
	}
	for _, it := range items {
		if it.name == "" || it.value == nil {
			continue
		}
		if it.sheet != "" {
			if sh := wb.GetSheet(it.sheet); sh != nil {
				sh.SetNamedRange(it.name, it.value)
				continue
			}
		}
		wb.SetNamedRange(it.name, it.value)
	}
}

func parseOpenFormulaAddress(addr string) any {
	addr = strings.ReplaceAll(addr, "&apos;", "'")
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	parts := strings.Split(addr, ":")
	p0 := strings.TrimSpace(parts[0])

	sheetName := ""
	cell0 := ""
	if dotIdx := strings.LastIndex(p0, "."); dotIdx != -1 {
		sheetPart := p0[:dotIdx]
		cell0 = p0[dotIdx+1:]
		sheetPart = strings.TrimPrefix(sheetPart, "$")
		sheetName = coord.UnquoteSheetName(sheetPart)
	} else {
		cell0 = strings.TrimPrefix(p0, "$")
	}
	cell0 = strings.ReplaceAll(cell0, "$", "")

	if len(parts) == 1 {
		fullCell := cell0
		if sheetName != "" {
			fullCell = coord.QuoteSheetPrefix(sheetName) + cell0
		}
		if cr, err := coord.ParseCellRef(fullCell); err == nil {
			return cr
		}
		return nil
	}

	p1 := strings.TrimSpace(parts[1])
	cell1 := ""
	if dotIdx := strings.LastIndex(p1, "."); dotIdx != -1 {
		cell1 = p1[dotIdx+1:]
	} else {
		cell1 = strings.TrimPrefix(p1, "$")
	}
	cell1 = strings.ReplaceAll(cell1, "$", "")

	fullRange := fmt.Sprintf("%s..%s", cell0, cell1)
	if sheetName != "" {
		fullRange = coord.QuoteSheetPrefix(sheetName) + cell0 + ".." + cell1
	}
	if rr, err := coord.ParseRangeRef(fullRange); err == nil {
		return rr
	}
	return nil
}

func parseODSContentXML(r io.Reader, sh *Sheet, targetSheetIdx int) error {
	decoder := xml.NewDecoder(r)

	currentTableIdx := -1
	inTargetTable := false
	curRow := 0
	curCol := 0

	var inTextP bool
	var curText strings.Builder
	var curFormula string
	var curValue string
	var curValueType string
	var cellColSpan int
	var rowRepeat int

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "table":
				currentTableIdx++
				if currentTableIdx == targetSheetIdx {
					inTargetTable = true
					curRow = 0
				} else {
					inTargetTable = false
				}

			case "table-row":
				if inTargetTable {
					curCol = 0
					rowRepeat = 1
					for _, attr := range t.Attr {
						if attr.Name.Local == "number-rows-repeated" {
							if n, err := strconv.Atoi(attr.Value); err == nil && n > 1 {
								// LibreOffice uses large repeats for empty-row skips;
								// never collapse to 1 (that shifts all later rows).
								rowRepeat = n
								if rowRepeat > 1048576 {
									rowRepeat = 1048576
								}
							}
						}
					}
				}

			case "table-cell":
				curText.Reset()
				curFormula = ""
				curValue = ""
				curValueType = ""
				cellColSpan = 1

				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "formula":
						curFormula = attr.Value
					case "value":
						curValue = attr.Value
					case "date-value":
						curValue = attr.Value
					case "string-value":
						curValue = attr.Value
					case "boolean-value":
						curValue = attr.Value
					case "value-type":
						curValueType = attr.Value
					case "number-columns-repeated":
						if n, err := strconv.Atoi(attr.Value); err == nil && n > 1 {
							// LibreOffice uses large repeats for empty-column skips;
							// never collapse to 1 (that shifts all later columns).
							cellColSpan = n
							if cellColSpan > 16384 {
								cellColSpan = 16384
							}
						}
					}
				}

			case "p":
				if inTargetTable {
					inTextP = true
				}

			case "covered-table-cell":
				span := 1
				for _, attr := range t.Attr {
					if attr.Name.Local == "number-columns-repeated" {
						if n, err := strconv.Atoi(attr.Value); err == nil && n > 1 {
							span = n
							if span > 16384 {
								span = 16384
							}
						}
					}
				}
				if inTargetTable {
					curCol += span
				}
			}

		case xml.EndElement:
			switch t.Name.Local {
			case "p":
				inTextP = false

			case "table-cell":
				if !inTargetTable {
					continue
				}
				textVal := strings.TrimSpace(curText.String())
				hasContent := (textVal != "" || curValue != "" || curFormula != "")

				if hasContent {
					for rep := 0; rep < cellColSpan; rep++ {
						c := curCol + rep
						if curFormula != "" {
							converted := convertODSFormula(curFormula)
							setDirectCell(sh, c, curRow, converted)
							if curValue != "" || textVal != "" {
								cellObj := sh.cells[CellCoord{Col: c, Row: curRow}]
								if cellObj != nil {
									if curValueType == "boolean" {
										cellObj.Value = strings.EqualFold(curValue, "true") || strings.EqualFold(textVal, "true")
									} else if num, err := strconv.ParseFloat(curValue, 64); err == nil {
										cellObj.Value = num
									} else if curValue != "" {
										cellObj.Value = curValue
									} else if textVal != "" {
										cellObj.Value = textVal
									}
								}
							}
						} else if curValueType == "boolean" {
							bVal := strings.EqualFold(curValue, "true") || (curValue == "" && strings.EqualFold(textVal, "true"))
							setDirectBooleanCell(sh, c, curRow, bVal)
						} else if curValueType == "float" || curValueType == "currency" || curValueType == "percentage" {
							if curValue != "" {
								setDirectCell(sh, c, curRow, curValue)
							} else if textVal != "" {
								setDirectCell(sh, c, curRow, textVal)
							}
						} else if curValueType == "string" {
							if textVal != "" {
								setDirectLabelCell(sh, c, curRow, textVal)
							} else if curValue != "" {
								setDirectLabelCell(sh, c, curRow, curValue)
							}
						} else if textVal != "" {
							setDirectCell(sh, c, curRow, textVal)
						} else if curValue != "" {
							setDirectCell(sh, c, curRow, curValue)
						}
					}
				}
				curCol += cellColSpan

			case "table-row":
				if !inTargetTable {
					continue
				}
				if rowRepeat > 1 {
					srcRow := curRow
					for rOff := 1; rOff < rowRepeat; rOff++ {
						destRow := srcRow + rOff
						if destRow >= 1048576 {
							break
						}
						for c := 0; c <= sh.maxPopulatedCol; c++ {
							src := sh.cells[CellCoord{Col: c, Row: srcRow}]
							if src == nil {
								continue
							}
							sh.cells[CellCoord{Col: c, Row: destRow}] = cloneImportedCell(src)
							if destRow > sh.maxPopulatedRow {
								sh.maxPopulatedRow = destRow
							}
						}
					}
				}
				curRow += rowRepeat

			case "table":
				if inTargetTable {
					return nil // finished parsing target sheet
				}
			}

		case xml.CharData:
			if inTextP {
				curText.Write(t)
			}
		}
	}

	return nil
}

var odsCellBracketRegex = regexp.MustCompile(`\[\s*(.*?)\]`)

// convertODSFormula converts OpenFormula of:=SUM([.A1:.B10]) syntax into HasuCalc format.
// Absolute markers ($) on columns/rows are preserved.
func convertODSFormula(f string) string {
	f = strings.TrimSpace(f)
	if strings.HasPrefix(f, "of:=") {
		f = "=" + strings.TrimPrefix(f, "of:=")
	} else if !strings.HasPrefix(f, "=") {
		f = "=" + f
	}

	return mapFormulaNonStrings(f, func(part string) string {
		part = odsCellBracketRegex.ReplaceAllStringFunc(part, func(m string) string {
			inner := strings.TrimSpace(m[1 : len(m)-1])
			if converted := convertODSBracketRef(inner); converted != "" {
				return converted
			}
			return m
		})
		return excelRangeColonRegex.ReplaceAllString(part, "$1..$2")
	})
}

// convertODSBracketRef converts OpenFormula [.A1], [.$A$1], [.Sheet.$B$2:.C3], etc.
func convertODSBracketRef(inner string) string {
	inner = strings.TrimSpace(inner)
	if inner == "" || strings.EqualFold(inner, "#REF!") {
		return "#REF!"
	}

	parts := strings.SplitN(inner, ":", 2)
	left, errL := parseODSAddressPart(strings.TrimSpace(parts[0]))
	if errL != nil {
		return ""
	}
	if len(parts) == 1 {
		return left
	}
	right, errR := parseODSAddressPart(strings.TrimSpace(parts[1]))
	if errR != nil {
		return ""
	}
	// Drop sheet from right if same prefix style; Range uses sheet on left only
	if i := strings.LastIndex(right, "!"); i >= 0 {
		right = right[i+1:]
	}
	return left + ".." + right
}

func parseODSAddressPart(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty")
	}
	// Optional leading '.' (current sheet relative marker)
	p = strings.TrimPrefix(p, ".")

	sheet := ""
	cellPart := p
	if strings.HasPrefix(p, "$'") {
		// $'Sheet name'.A1
		rest := p[2:]
		end := strings.Index(rest, "'")
		if end < 0 {
			return "", fmt.Errorf("bad quoted sheet")
		}
		sheet = rest[:end]
		rest = rest[end+1:]
		rest = strings.TrimPrefix(rest, ".")
		cellPart = rest
	} else if idx := strings.LastIndex(p, "."); idx >= 0 {
		sheetPart := p[:idx]
		cellPart = p[idx+1:]
		sheetPart = strings.TrimPrefix(sheetPart, "$")
		sheetPart = strings.Trim(sheetPart, "'")
		sheet = sheetPart
	}

	cellPart = strings.TrimSpace(cellPart)
	if cellPart == "" {
		return "", fmt.Errorf("no cell")
	}

	var b strings.Builder
	if sheet != "" {
		b.WriteString(coord.QuoteSheetPrefix(sheet))
	}

	colAbs := strings.HasPrefix(cellPart, "$")
	if colAbs {
		cellPart = cellPart[1:]
	}
	// Check whole row (e.g. 1 or $10)
	if isAllDigitsStr(cellPart) {
		if colAbs {
			b.WriteByte('$')
		}
		b.WriteString(cellPart)
		return b.String(), nil
	}

	// Split letters / optional $ / digits
	i := 0
	for i < len(cellPart) && ((cellPart[i] >= 'A' && cellPart[i] <= 'Z') || (cellPart[i] >= 'a' && cellPart[i] <= 'z')) {
		i++
	}
	if i == 0 {
		return "", fmt.Errorf("no col")
	}
	colLetters := cellPart[:i]
	rest := cellPart[i:]
	if rest == "" {
		if colAbs {
			b.WriteByte('$')
		}
		b.WriteString(strings.ToUpper(colLetters))
		return b.String(), nil
	}

	rowAbs := strings.HasPrefix(rest, "$")
	if rowAbs {
		rest = rest[1:]
	}
	if rest == "" {
		return "", fmt.Errorf("no row")
	}
	for _, ch := range rest {
		if ch < '0' || ch > '9' {
			return "", fmt.Errorf("bad row")
		}
	}

	if colAbs {
		b.WriteByte('$')
	}
	b.WriteString(strings.ToUpper(colLetters))
	if rowAbs {
		b.WriteByte('$')
	}
	b.WriteString(rest)
	return b.String(), nil
}

func isAllDigitsStr(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func cloneImportedCell(c *cell.Cell) *cell.Cell {
	if c == nil {
		return nil
	}
	var fmtCopy *cell.CellFormat
	if c.FormatSpec != nil {
		f := *c.FormatSpec
		fmtCopy = &f
	}
	return &cell.Cell{
		RawInput:   c.RawInput,
		Type:       c.Type,
		Alignment:  c.Alignment,
		FormatSpec: fmtCopy,
		Value:      c.Value,
	}
}

// OpenFormula cell/range refs: Sheet!A1, 'My Sheet'!A1:B2, A1, A:A, 1:10.
var openFormulaRefRe = regexp.MustCompile(`(?:'(?:[^']|'')+'|[^\s'!()]+)!(?:\$?[A-Za-z]+\$?[0-9]+(?::\$?[A-Za-z]+\$?[0-9]+)?|\$?[A-Za-z]+:\$?[A-Za-z]+|\$?[0-9]+:\$?[0-9]+)|\$?[A-Za-z]+\$?[0-9]+(?::\$?[A-Za-z]+\$?[0-9]+)?|\$?[A-Za-z]+:\$?[A-Za-z]+|\$?[0-9]+:\$?[0-9]+`)

// toOpenFormulaExport converts HasuCalc formula text to OpenFormula (without of:= prefix).
func toOpenFormulaExport(raw string) string {
	f := toExcelExportFormula(raw)
	if f == "" {
		return ""
	}
	return rewriteFormulaOutsideStrings(f, wrapOpenFormulaRefsInChunk)
}

// OpenFormulaExportFormulaForTest exposes toOpenFormulaExport for unit tests.
func OpenFormulaExportFormulaForTest(raw string) string {
	return toOpenFormulaExport(raw)
}

func rewriteFormulaOutsideStrings(f string, rewrite func(string) string) string {
	var b strings.Builder
	i := 0
	for i < len(f) {
		if f[i] == '"' {
			start := i
			i++
			for i < len(f) {
				if f[i] == '"' {
					i++
					if i < len(f) && f[i] == '"' {
						i++
						continue
					}
					break
				}
				i++
			}
			b.WriteString(f[start:i])
			continue
		}
		j := i
		for j < len(f) && f[j] != '"' {
			j++
		}
		b.WriteString(rewrite(f[i:j]))
		i = j
	}
	return b.String()
}

func wrapOpenFormulaRefsInChunk(chunk string) string {
	matches := openFormulaRefRe.FindAllStringIndex(chunk, -1)
	if len(matches) == 0 {
		return chunk
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if end < len(chunk) && chunk[end] == '(' {
			continue
		}
		if start > 0 {
			prev := chunk[start-1]
			if (prev >= 'A' && prev <= 'Z') || (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') || prev == '_' || prev == '$' {
				continue
			}
		}
		b.WriteString(chunk[last:start])
		b.WriteString(wrapOpenFormulaRef(chunk[start:end]))
		last = end
	}
	b.WriteString(chunk[last:])
	return b.String()
}

func wrapOpenFormulaRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if strings.Contains(ref, ":") || strings.Contains(ref, "..") {
		r, err := coord.ParseRangeRef(ref)
		if err != nil {
			r, err = coord.ParseRangeRef(strings.ReplaceAll(ref, ":", ".."))
		}
		if err == nil {
			return openFormulaFromRange(r)
		}
	}
	cr, err := coord.ParseCellRef(ref)
	if err != nil {
		return "[." + ref + "]"
	}
	return "[" + openFormulaSheetDot(cr.Sheet) + openFormulaCellAddr(cr) + "]"
}

func openFormulaSheetDot(name string) string {
	if name == "" {
		return "."
	}
	return strings.TrimSuffix(coord.QuoteSheetPrefix(name), "!") + "."
}

func openFormulaCellAddr(c coord.CellRef) string {
	col := coord.ColToLetter(c.Col)
	if c.ColAbs {
		col = "$" + col
	}
	row := strconv.Itoa(c.Row + 1)
	if c.RowAbs {
		row = "$" + row
	}
	return col + row
}

func openFormulaColAddr(c coord.CellRef) string {
	col := coord.ColToLetter(c.Col)
	if c.ColAbs {
		return "$" + col
	}
	return col
}

func openFormulaRowAddr(c coord.CellRef) string {
	row := strconv.Itoa(c.Row + 1)
	if c.RowAbs {
		return "$" + row
	}
	return row
}

func openFormulaFromRange(r coord.RangeRef) string {
	sheet := r.Sheet
	if sheet == "" {
		sheet = r.Start.Sheet
	}
	prefix := openFormulaSheetDot(sheet)
	if r.IsWholeColumn() {
		return "[" + prefix + openFormulaColAddr(r.Start) + ":." + openFormulaColAddr(r.End) + "]"
	}
	if r.IsWholeRow() {
		return "[" + prefix + openFormulaRowAddr(r.Start) + ":." + openFormulaRowAddr(r.End) + "]"
	}
	return "[" + prefix + openFormulaCellAddr(r.Start) + ":." + openFormulaCellAddr(r.End) + "]"
}
