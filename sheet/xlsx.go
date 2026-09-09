package sheet

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
)

// XLSXSheetInfo represents a sheet found in an XLSX workbook.
type XLSXSheetInfo struct {
	Name    string
	Target  string
	SheetID string
	RelID   string
}

// GetXLSXSheetList returns the list of sheet names in the .xlsx file.
func GetXLSXSheetList(filename string) ([]string, error) {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx file: %w", err)
	}
	defer zr.Close()

	sheets, err := parseWorkbookSheets(zr)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, s := range sheets {
		names = append(names, s.Name)
	}
	return names, nil
}

// ImportXLSX imports a specific sheet (0-indexed) from an .xlsx file into a new Sheet.
func ImportXLSX(filename string, sheetIdx int) (*Sheet, error) {
	return ImportXLSXWithProgress(filename, sheetIdx, nil)
}

// ImportXLSXWithProgress imports a sheet from an .xlsx file with progress updates.
func ImportXLSXWithProgress(filename string, sheetIdx int, onProgress func(step string)) (*Sheet, error) {
	wb, err := ImportXLSXWorkbookWithProgress(filename, onProgress)
	if err != nil {
		return nil, err
	}
	if len(wb.Sheets) == 0 {
		return nil, fmt.Errorf("xlsx file contains no worksheets")
	}
	if sheetIdx < 0 || sheetIdx >= len(wb.Sheets) {
		sheetIdx = 0
	}
	wb.ActiveSheetIndex = sheetIdx
	return wb.Sheets[sheetIdx], nil
}

// ImportXLSXWorkbook imports all sheets in an .xlsx file into a Workbook.
func ImportXLSXWorkbook(filename string) (*Workbook, error) {
	return ImportXLSXWorkbookWithProgress(filename, nil)
}

// ImportXLSXWorkbookWithProgress imports all sheets in the Excel workbook.
// Embedded Excel charts are ignored; HasuCalc graphs are not reconstructed from them.
func ImportXLSXWorkbookWithProgress(filename string, onProgress func(step string)) (*Workbook, error) {
	if onProgress != nil {
		onProgress("Opening Excel ZIP archive...")
	}
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open xlsx file: %w", err)
	}
	defer zr.Close()

	if onProgress != nil {
		onProgress("Reading workbook structure...")
	}
	sheets, err := parseWorkbookSheets(zr)
	if err != nil {
		return nil, err
	}
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in workbook")
	}

	if onProgress != nil {
		onProgress("Reading shared strings table...")
	}
	sharedStrings := parseSharedStrings(zr)
	tables := parseXLSXTables(zr)
	dateStyles := parseXLSXDateStyles(zr)

	wb := &Workbook{
		Name:             filepath.Base(filename),
		Sheets:           make([]*Sheet, 0, len(sheets)),
		ActiveSheetIndex: 0,
		NamedRanges:      make(map[string]any),
	}

	type sheetCached struct {
		sheet        *Sheet
		cachedValues map[coord.CellRef]any
	}
	var loadedSheets []sheetCached

	for i, sInfo := range sheets {
		if onProgress != nil {
			onProgress(fmt.Sprintf("Loading sheet %d/%d: '%s'...", i+1, len(sheets), sInfo.Name))
		}
		sheetFile := findZipFile(zr, sInfo.Target)
		if sheetFile == nil {
			sheetFile = findZipFile(zr, fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1))
			if sheetFile == nil {
				continue
			}
		}
		rc, err := sheetFile.Open()
		if err != nil {
			continue
		}

		sh := NewSheet()
		sh.SetName(sInfo.Name)
		sh.SetWorkbook(wb)
		sh.SetRecalcMode("MANUAL")

		cachedVals, _ := parseWorksheetXML(rc, sh, sharedStrings, tables, dateStyles)
		rc.Close()

		sh.SetRecalcMode("AUTO")
		wb.Sheets = append(wb.Sheets, sh)
		loadedSheets = append(loadedSheets, sheetCached{sheet: sh, cachedValues: cachedVals})
	}

	if len(wb.Sheets) == 0 {
		return nil, fmt.Errorf("no valid sheets could be parsed")
	}

	calcMode := applyXLSXDefinedNames(zr, wb)

	// Check if any formula cell is missing a cached value
	hasMissingCached := false
	for _, sc := range loadedSheets {
		for _, c := range sc.sheet.cells {
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
			onProgress("Calculating formulas across all worksheets...")
		}
		wb.RecalculateAll()

		// Fallback to pre-calculated Excel cached value if formula resulted in an error
		for _, sc := range loadedSheets {
			for pt, cachedVal := range sc.cachedValues {
				cPtr := sc.sheet.GetCell(pt.Col, pt.Row)
				if cPtr != nil && (cPtr.Value == nil || isLotusError(cPtr.Value)) {
					cPtr.Value = cachedVal
				}
			}
		}
	}

	for _, sc := range loadedSheets {
		if strings.EqualFold(calcMode, "manual") {
			sc.sheet.SetRecalcMode("MANUAL")
		}
		sc.sheet.SetModified(false)
	}

	return wb, nil
}

// parseWorkbookSheets extracts sheet metadata and resolves relative target paths.
func parseWorkbookSheets(zr *zip.ReadCloser) ([]XLSXSheetInfo, error) {
	wbFile := findZipFile(zr, "xl/workbook.xml")
	if wbFile == nil {
		return nil, fmt.Errorf("xl/workbook.xml not found")
	}

	rc, err := wbFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	type xmlSheet struct {
		Name    string `xml:"name,attr"`
		SheetID string `xml:"sheetId,attr"`
		RelID   string `xml:"id,attr"`
	}
	type xmlWorkbook struct {
		Sheets []xmlSheet `xml:"sheets>sheet"`
	}

	var wb xmlWorkbook
	if err := xml.NewDecoder(rc).Decode(&wb); err != nil {
		return nil, fmt.Errorf("error decoding xl/workbook.xml: %w", err)
	}

	// Parse relationships (xl/_rels/workbook.xml.rels)
	relMap := make(map[string]string)
	relsFile := findZipFile(zr, "xl/_rels/workbook.xml.rels")
	if relsFile != nil {
		if rrc, err := relsFile.Open(); err == nil {
			type xmlRel struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
			}
			type xmlRels struct {
				Relationships []xmlRel `xml:"Relationship"`
			}
			var rels xmlRels
			if err := xml.NewDecoder(rrc).Decode(&rels); err == nil {
				for _, r := range rels.Relationships {
					relMap[r.ID] = r.Target
				}
			}
			rrc.Close()
		}
	}

	var result []XLSXSheetInfo
	for i, s := range wb.Sheets {
		target := relMap[s.RelID]
		if target == "" {
			target = fmt.Sprintf("worksheets/sheet%d.xml", i+1)
		}
		// Resolve path relative to xl/
		if !strings.HasPrefix(target, "xl/") && !strings.HasPrefix(target, "/") {
			target = "xl/" + strings.TrimPrefix(target, "./")
		} else {
			target = strings.TrimPrefix(target, "/")
		}
		result = append(result, XLSXSheetInfo{
			Name:    s.Name,
			Target:  target,
			SheetID: s.SheetID,
			RelID:   s.RelID,
		})
	}

	return result, nil
}

func applyXLSXDefinedNames(zr *zip.ReadCloser, wb *Workbook) (calcMode string) {
	wbFile := findZipFile(zr, "xl/workbook.xml")
	if wbFile == nil || wb == nil {
		return ""
	}
	rc, err := wbFile.Open()
	if err != nil {
		return ""
	}
	defer rc.Close()

	type xmlDefinedName struct {
		Name         string `xml:"name,attr"`
		LocalSheetID string `xml:"localSheetId,attr"`
		Comment      string `xml:"comment,attr"`
		Text         string `xml:",chardata"`
	}
	type xmlCalcPr struct {
		CalcMode string `xml:"calcMode,attr"`
	}
	type xmlWorkbookNames struct {
		DefinedNames []xmlDefinedName `xml:"definedNames>definedName"`
		CalcPr       xmlCalcPr        `xml:"calcPr"`
	}
	var doc xmlWorkbookNames
	if err := xml.NewDecoder(rc).Decode(&doc); err != nil {
		return ""
	}
	for _, dn := range doc.DefinedNames {
		name := strings.TrimSpace(dn.Name)
		text := strings.TrimSpace(dn.Text)
		if name == "" || text == "" || text == "#REF!" {
			continue
		}
		parsed := applyHasuCalcBaseComment(parseXLSXDefinedNameFormula(text), dn.Comment)
		if parsed == nil {
			continue
		}
		if dn.LocalSheetID != "" {
			if idx, err := strconv.Atoi(dn.LocalSheetID); err == nil && idx >= 0 && idx < len(wb.Sheets) && wb.Sheets[idx] != nil {
				wb.Sheets[idx].SetNamedRange(name, parsed)
				continue
			}
		}
		wb.SetNamedRange(name, parsed)
	}
	return strings.TrimSpace(doc.CalcPr.CalcMode)
}

func parseXLSXDefinedNameFormula(text string) any {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "=")
	if text == "" || text == "#REF!" {
		return nil
	}
	addr := mapFormulaNonStrings(text, func(part string) string {
		return strings.ReplaceAll(part, ":", "..")
	})
	if r, err := coord.ParseRangeRef(addr); err == nil {
		if r.Start.Col != r.End.Col || r.Start.Row != r.End.Row || r.IsWholeColumn() || r.IsWholeRow() || strings.Contains(text, ":") {
			return r
		}
		return r.Start
	}
	if cr, err := coord.ParseCellRef(addr); err == nil {
		return cr
	}
	expr := text
	if !strings.HasPrefix(expr, "=") && !strings.HasPrefix(expr, "@") {
		expr = "=" + expr
	}
	expr = mapFormulaNonStrings(expr, func(part string) string {
		return strings.ReplaceAll(part, ":", "..")
	})
	return formula.NamedExpr{Expr: expr}
}

// parseSharedStrings parses xl/sharedStrings.xml into a slice of strings.
func parseSharedStrings(zr *zip.ReadCloser) []string {
	ssFile := findZipFile(zr, "xl/sharedStrings.xml")
	if ssFile == nil {
		return nil
	}
	rc, err := ssFile.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()

	decoder := xml.NewDecoder(rc)
	var stringsList []string
	var inT bool
	var curText strings.Builder

	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "si" {
				curText.Reset()
			} else if t.Name.Local == "t" {
				inT = true
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inT = false
			} else if t.Name.Local == "si" {
				stringsList = append(stringsList, curText.String())
			}
		case xml.CharData:
			if inT {
				curText.Write(t)
			}
		}
	}

	return stringsList
}

// parseWorksheetXML reads rows, cells, formulas, values, and column widths into Sheet.
func parseWorksheetXML(r io.Reader, sh *Sheet, sharedStrings []string, tables map[string]*XLSXTable, dateStyles []bool) (map[coord.CellRef]any, error) {
	decoder := xml.NewDecoder(r)
	cachedValues := make(map[coord.CellRef]any)

	type xmlCol struct {
		Min   int     `xml:"min,attr"`
		Max   int     `xml:"max,attr"`
		Width float64 `xml:"width,attr"`
	}
	type sharedMaster struct {
		formula string
		col     int
		row     int
	}
	sharedFormulas := make(map[int]*sharedMaster)

	var curCellRef string
	var curCellType string
	var curStyleIdx = -1
	var curFormula strings.Builder
	var curValue strings.Builder
	var inF, inV, inInlineT bool
	var curFType string
	var curFSI = -1

	for {
		tok, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "col":
				var col xmlCol
				if err := decoder.DecodeElement(&col, &t); err == nil {
					for c := col.Min - 1; c <= col.Max-1; c++ {
						if c >= 0 && col.Width > 0 {
							w := int(col.Width)
							if w < 3 {
								w = 3
							}
							if w > 72 {
								w = 72
							}
							sh.SetColWidth(c, w)
						}
					}
				}

			case "pane":
				var xSplit, ySplit float64
				state := ""
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "xSplit":
						xSplit, _ = strconv.ParseFloat(attr.Value, 64)
					case "ySplit":
						ySplit, _ = strconv.ParseFloat(attr.Value, 64)
					case "state":
						state = strings.ToLower(attr.Value)
					}
				}
				if state == "frozen" || state == "frozensplit" {
					if xSplit > 0 {
						sh.SetFrozenCols(int(xSplit))
					}
					if ySplit > 0 {
						sh.SetFrozenRows(int(ySplit))
					}
				}

			case "c":
				curCellRef = ""
				curCellType = ""
				curStyleIdx = -1
				curFormula.Reset()
				curValue.Reset()
				curFType = ""
				curFSI = -1
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "r":
						curCellRef = attr.Value
					case "t":
						curCellType = attr.Value
					case "s":
						if n, err := strconv.Atoi(attr.Value); err == nil {
							curStyleIdx = n
						}
					}
				}

			case "f":
				inF = true
				curFormula.Reset()
				curFType = ""
				curFSI = -1
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "t":
						curFType = attr.Value
					case "si":
						if n, err := strconv.Atoi(attr.Value); err == nil {
							curFSI = n
						}
					}
				}

			case "v":
				inV = true
				curValue.Reset()

			case "t":
				inInlineT = true
			}

		case xml.EndElement:
			switch t.Name.Local {
			case "f":
				inF = false
			case "v":
				inV = false
			case "t":
				inInlineT = false

			case "c":
				if curCellRef == "" {
					continue
				}
				cr, err := coord.ParseCellRef(curCellRef)
				if err != nil {
					continue
				}
				rawVal := strings.TrimSpace(curValue.String())
				rawFormula := strings.TrimSpace(curFormula.String())

				finalForm := ""
				if curFType == "shared" && curFSI >= 0 {
					if rawFormula != "" {
						finalForm = convertExcelFormulaWithTables(rawFormula, cr.Col, cr.Row, tables)
						sharedFormulas[curFSI] = &sharedMaster{formula: finalForm, col: cr.Col, row: cr.Row}
					} else if master := sharedFormulas[curFSI]; master != nil {
						finalForm = formula.AdjustFormulaReferences(master.formula, cr.Col-master.col, cr.Row-master.row)
					}
				} else if rawFormula != "" {
					finalForm = convertExcelFormulaWithTables(rawFormula, cr.Col, cr.Row, tables)
				}

				if finalForm != "" {
					setDirectCell(sh, cr.Col, cr.Row, finalForm)
					if rawVal != "" {
						c := sh.cells[CellCoord{Col: cr.Col, Row: cr.Row}]
						if curCellType == "s" {
							if idx, err := strconv.Atoi(rawVal); err == nil && idx >= 0 && idx < len(sharedStrings) {
								cachedValues[cr] = sharedStrings[idx]
								if c != nil {
									c.Value = sharedStrings[idx]
								}
							}
						} else if curCellType == "b" {
							bVal := rawVal == "1" || strings.EqualFold(rawVal, "true")
							cachedValues[cr] = bVal
							if c != nil {
								c.Value = bVal
							}
						} else if num, err := strconv.ParseFloat(rawVal, 64); err == nil {
							cachedValues[cr] = num
							if c != nil {
								c.Value = num
							}
						} else {
							cachedValues[cr] = rawVal
							if c != nil {
								c.Value = rawVal
							}
						}
					}
				} else if curCellType == "s" {
					if idx, err := strconv.Atoi(rawVal); err == nil && idx >= 0 && idx < len(sharedStrings) {
						setDirectLabelCell(sh, cr.Col, cr.Row, sharedStrings[idx])
					}
				} else if curCellType == "inlineStr" || curCellType == "str" {
					setDirectLabelCell(sh, cr.Col, cr.Row, rawVal)
				} else if curCellType == "b" {
					bVal := (rawVal == "1" || strings.EqualFold(rawVal, "true"))
					setDirectBooleanCell(sh, cr.Col, cr.Row, bVal)
				} else if rawVal != "" {
					var fmtSpec *cell.CellFormat
					if curStyleIdx >= 0 && curStyleIdx < len(dateStyles) && dateStyles[curStyleIdx] {
						if _, err := strconv.ParseFloat(rawVal, 64); err == nil {
							fmtSpec = &cell.CellFormat{Type: cell.FmtDate, DateFormat: 1}
						}
					}
					setDirectCellWithFormat(sh, cr.Col, cr.Row, rawVal, fmtSpec)
				}
			}

		case xml.CharData:
			if inF {
				curFormula.Write(t)
			} else if inV {
				curValue.Write(t)
			} else if inInlineT {
				curValue.Write(t)
			}
		}
	}

	return cachedValues, nil
}

// parseXLSXDateStyles returns whether each cellXfs style index is a date/time number format.
func parseXLSXDateStyles(zr *zip.ReadCloser) []bool {
	stylesFile := findZipFile(zr, "xl/styles.xml")
	if stylesFile == nil {
		return nil
	}
	rc, err := stylesFile.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()

	customFmts := make(map[int]string)
	var numFmtIDs []int
	decoder := xml.NewDecoder(rc)
	var inCellXfs bool
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			if ee, ok := tok.(xml.EndElement); ok && ee.Name.Local == "cellXfs" {
				inCellXfs = false
			}
			continue
		}
		switch se.Name.Local {
		case "numFmt":
			var id int
			var code string
			for _, a := range se.Attr {
				if a.Name.Local == "numFmtId" {
					id, _ = strconv.Atoi(a.Value)
				} else if a.Name.Local == "formatCode" {
					code = a.Value
				}
			}
			if code != "" {
				customFmts[id] = code
			}
		case "cellXfs":
			inCellXfs = true
		case "xf":
			if !inCellXfs {
				continue
			}
			id := 0
			for _, a := range se.Attr {
				if a.Name.Local == "numFmtId" {
					id, _ = strconv.Atoi(a.Value)
					break
				}
			}
			numFmtIDs = append(numFmtIDs, id)
		}
	}

	out := make([]bool, len(numFmtIDs))
	for i, id := range numFmtIDs {
		out[i] = isExcelDateNumFmt(id, customFmts[id])
	}
	return out
}

func isExcelDateNumFmt(id int, code string) bool {
	switch id {
	case 14, 15, 16, 17, 18, 19, 20, 21, 22, 27, 30, 36, 45, 46, 47, 50, 57:
		return true
	}
	if code == "" {
		return false
	}
	// Date-like custom formats contain y/m/d/h/s outside brackets/quotes.
	lower := strings.ToLower(code)
	stripped := stripExcelFmtNoise(lower)
	hasDate := strings.ContainsAny(stripped, "ymd")
	hasTime := strings.ContainsAny(stripped, "hs")
	return hasDate || hasTime
}

func stripExcelFmtNoise(code string) string {
	var b strings.Builder
	inBracket := false
	inQuote := false
	for _, r := range code {
		switch {
		case r == '[' && !inQuote:
			inBracket = true
		case r == ']' && !inQuote:
			inBracket = false
		case r == '"':
			inQuote = !inQuote
		case inBracket || inQuote:
			// skip
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

type XLSXTableColumn struct {
	ID   int
	Name string
}

type XLSXTable struct {
	Name           string
	DisplayName    string
	Ref            string
	StartCol       int
	StartRow       int
	EndCol         int
	EndRow         int
	HeaderRowCount int
	TotalsRowCount int
	Columns        []XLSXTableColumn
}

func parseXLSXTables(zr *zip.ReadCloser) map[string]*XLSXTable {
	tables := make(map[string]*XLSXTable)
	for _, f := range zr.File {
		nameSlash := filepath.ToSlash(f.Name)
		if strings.HasPrefix(nameSlash, "xl/tables/") && strings.HasSuffix(nameSlash, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			dec := xml.NewDecoder(rc)
			var curTbl *XLSXTable
			for {
				tok, err := dec.Token()
				if err != nil {
					break
				}
				switch t := tok.(type) {
				case xml.StartElement:
					switch t.Name.Local {
					case "table":
						curTbl = &XLSXTable{
							HeaderRowCount: 1,
							TotalsRowCount: 0,
						}
						for _, a := range t.Attr {
							switch a.Name.Local {
							case "name":
								curTbl.Name = a.Value
							case "displayName":
								curTbl.DisplayName = a.Value
							case "ref":
								curTbl.Ref = a.Value
								if rng, err := coord.ParseRangeRef(a.Value); err == nil {
									curTbl.StartCol = rng.MinCol()
									curTbl.StartRow = rng.MinRow()
									curTbl.EndCol = rng.MaxCol()
									curTbl.EndRow = rng.MaxRow()
								}
							case "headerRowCount":
								if v, err := strconv.Atoi(a.Value); err == nil {
									curTbl.HeaderRowCount = v
								}
							case "totalsRowCount":
								if v, err := strconv.Atoi(a.Value); err == nil {
									curTbl.TotalsRowCount = v
								}
							case "totalsRowShown":
								if a.Value == "1" && curTbl.TotalsRowCount == 0 {
									curTbl.TotalsRowCount = 1
								}
							}
						}
					case "tableColumn":
						if curTbl != nil {
							col := XLSXTableColumn{}
							for _, a := range t.Attr {
								switch a.Name.Local {
								case "id":
									col.ID, _ = strconv.Atoi(a.Value)
								case "name":
									col.Name = a.Value
								}
							}
							curTbl.Columns = append(curTbl.Columns, col)
						}
					}
				}
			}
			rc.Close()
			if curTbl != nil && curTbl.Ref != "" {
				if curTbl.Name != "" {
					tables[strings.ToUpper(curTbl.Name)] = curTbl
				}
				if curTbl.DisplayName != "" {
					tables[strings.ToUpper(curTbl.DisplayName)] = curTbl
				}
			}
		}
	}
	return tables
}

var (
	excelRangeColonRegex    = regexp.MustCompile(`(\$?[A-Za-z]+\$?[0-9]+):(\$?[A-Za-z]+\$?[0-9]+)`)
	hasuRangeDotDotRegex    = regexp.MustCompile(`(\$?[A-Za-z]+\$?[0-9]+)\.\.(\$?[A-Za-z]+\$?[0-9]+)`)
	hasuColRangeDotDotRegex = regexp.MustCompile(`(\$?[A-Za-z]{1,3})\.\.(\$?[A-Za-z]{1,3}\b)`)
	hasuRowRangeDotDotRegex = regexp.MustCompile(`(\$?[0-9]+)\.\.(\$?[0-9]+\b)`)
	fnAtPrefixRegex         = regexp.MustCompile(`@([A-Za-z_][A-Za-z0-9_.]*\()`)
	fnAvgRegex              = regexp.MustCompile(`(?i)\bAVG\(`)
	fnLengthRegex           = regexp.MustCompile(`(?i)\bLENGTH\(`)
	fnRepeatRegex           = regexp.MustCompile(`(?i)\bREPEAT\(`)
	fnStringRegex           = regexp.MustCompile(`(?i)\bSTRING\(`)
	fnPaymtRegex            = regexp.MustCompile(`(?i)\bPAYMT\(`)
	fnMultiplyRegex         = regexp.MustCompile(`(?i)\bMULTIPLY\(`)
	structRefRegex          = regexp.MustCompile(`([A-Za-z0-9_]+)?(\[\[?[^\]]+\]?(?:,\s*\[\[?[^\]]+\]?)*\])`)
	bracketItemRegex        = regexp.MustCompile(`\[([^\[\]]+)\]`)
)

// toExcelExportFormula converts HasuCalc formula text to Excel-compatible syntax.
func toExcelExportFormula(raw string) string {
	f := strings.TrimSpace(raw)
	f = strings.TrimPrefix(f, "=")
	f = strings.TrimPrefix(f, "@")
	return mapFormulaNonStrings(f, func(part string) string {
		part = fnAtPrefixRegex.ReplaceAllString(part, "$1")
		part = hasuRangeDotDotRegex.ReplaceAllString(part, "$1:$2")
		part = hasuColRangeDotDotRegex.ReplaceAllString(part, "$1:$2")
		part = hasuRowRangeDotDotRegex.ReplaceAllString(part, "$1:$2")
		part = fnAvgRegex.ReplaceAllString(part, "AVERAGE(")
		part = fnLengthRegex.ReplaceAllString(part, "LEN(")
		part = fnRepeatRegex.ReplaceAllString(part, "REPT(")
		part = fnStringRegex.ReplaceAllString(part, "FIXED(")
		part = fnPaymtRegex.ReplaceAllString(part, "PMT(")
		part = fnMultiplyRegex.ReplaceAllString(part, "PRODUCT(")
		return part
	})
}

// mapFormulaNonStrings applies fn to formula text outside double-quoted string literals.
func mapFormulaNonStrings(f string, fn func(string) string) string {
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
		start := i
		for i < len(f) && f[i] != '"' {
			i++
		}
		b.WriteString(fn(f[start:i]))
	}
	return b.String()
}

// ExcelExportFormulaForTest exposes toExcelExportFormula for unit tests.
func ExcelExportFormulaForTest(raw string) string {
	return toExcelExportFormula(raw)
}

// ConvertExcelFormulaWithTablesForTest exposes convertExcelFormulaWithTables for testing.
func ConvertExcelFormulaWithTablesForTest(f string, cellCol, cellRow int, tables map[string]*XLSXTable) string {
	return convertExcelFormulaWithTables(f, cellCol, cellRow, tables)
}

// NewXLSXTableForTest creates a new XLSXTable for testing structured references.
func NewXLSXTableForTest(name string, startCol, startRow, endCol, endRow int, colNames []string) *XLSXTable {
	tbl := &XLSXTable{
		Name:           name,
		DisplayName:    name,
		StartCol:       startCol,
		StartRow:       startRow,
		EndCol:         endCol,
		EndRow:         endRow,
		HeaderRowCount: 1,
		TotalsRowCount: 0,
	}
	for i, c := range colNames {
		tbl.Columns = append(tbl.Columns, XLSXTableColumn{ID: i + 1, Name: c})
	}
	return tbl
}

func resolveStructuredReference(tblName, spec string, cellCol, cellRow int, tables map[string]*XLSXTable) (string, bool) {
	var tbl *XLSXTable
	if tblName != "" {
		tbl = tables[strings.ToUpper(tblName)]
	} else {
		for _, t := range tables {
			if cellCol >= t.StartCol && cellCol <= t.EndCol && cellRow >= t.StartRow && cellRow <= t.EndRow {
				tbl = t
				break
			}
		}
	}
	if tbl == nil {
		return "", false
	}

	items := bracketItemRegex.FindAllStringSubmatch(spec, -1)
	var tokens []string
	for _, it := range items {
		tokens = append(tokens, strings.TrimSpace(it[1]))
	}
	if len(tokens) == 0 {
		clean := strings.Trim(spec, "[]")
		clean = strings.TrimPrefix(clean, "@")
		if clean != "" {
			tokens = append(tokens, "@", clean)
		}
	}

	hasThisRow := false
	hasHeaders := false
	hasTotals := false
	hasAll := false
	hasData := false
	var colName string

	for _, tok := range tokens {
		u := strings.ToUpper(tok)
		if u == "#THIS ROW" || u == "@" || strings.HasPrefix(u, "@") {
			hasThisRow = true
			if len(tok) > 1 && strings.HasPrefix(tok, "@") {
				colName = tok[1:]
			}
		} else if u == "#HEADERS" {
			hasHeaders = true
		} else if u == "#TOTALS" {
			hasTotals = true
		} else if u == "#ALL" {
			hasAll = true
		} else if u == "#DATA" {
			hasData = true
		} else {
			colName = tok
		}
	}

	headerRow := tbl.StartRow
	dataStartRow := tbl.StartRow + tbl.HeaderRowCount
	dataEndRow := tbl.EndRow - tbl.TotalsRowCount
	totalsRow := tbl.EndRow

	if colName != "" {
		colIdx := -1
		for i, c := range tbl.Columns {
			if strings.EqualFold(c.Name, colName) {
				colIdx = tbl.StartCol + i
				break
			}
		}
		if colIdx == -1 {
			return "", false
		}
		colLet := coord.ColToLetter(colIdx)

		if hasThisRow {
			return fmt.Sprintf("%s%d", colLet, cellRow+1), true
		}
		if hasHeaders {
			return fmt.Sprintf("%s%d", colLet, headerRow+1), true
		}
		if hasTotals {
			return fmt.Sprintf("%s%d", colLet, totalsRow+1), true
		}
		if hasAll {
			return fmt.Sprintf("%s%d..%s%d", colLet, tbl.StartRow+1, colLet, tbl.EndRow+1), true
		}
		if hasData || true {
			if dataStartRow == dataEndRow {
				return fmt.Sprintf("%s%d", colLet, dataStartRow+1), true
			}
			return fmt.Sprintf("%s%d..%s%d", colLet, dataStartRow+1, colLet, dataEndRow+1), true
		}
	}

	startLet := coord.ColToLetter(tbl.StartCol)
	endLet := coord.ColToLetter(tbl.EndCol)
	if hasHeaders {
		return fmt.Sprintf("%s%d..%s%d", startLet, headerRow+1, endLet, headerRow+1), true
	}
	if hasTotals {
		return fmt.Sprintf("%s%d..%s%d", startLet, totalsRow+1, endLet, totalsRow+1), true
	}
	if hasAll {
		return fmt.Sprintf("%s%d..%s%d", startLet, tbl.StartRow+1, endLet, tbl.EndRow+1), true
	}
	return fmt.Sprintf("%s%d..%s%d", startLet, dataStartRow+1, endLet, dataEndRow+1), true
}

func convertExcelFormulaWithTables(f string, cellCol, cellRow int, tables map[string]*XLSXTable) string {
	f = strings.TrimSpace(f)
	if !strings.HasPrefix(f, "=") {
		f = "=" + f
	}

	return mapFormulaNonStrings(f, func(part string) string {
		if len(tables) > 0 {
			part = structRefRegex.ReplaceAllStringFunc(part, func(m string) string {
				matches := structRefRegex.FindStringSubmatch(m)
				if len(matches) < 3 {
					return m
				}
				tblName := matches[1]
				spec := matches[2]
				if rep, ok := resolveStructuredReference(tblName, spec, cellCol, cellRow, tables); ok {
					return rep
				}
				return m
			})
		}
		return excelRangeColonRegex.ReplaceAllString(part, "$1..$2")
	})
}

func isLotusError(v any) bool {
	if _, ok := v.(cell.LotusError); ok {
		return true
	}
	return false
}

func findZipFile(zr *zip.ReadCloser, name string) *zip.File {
	name = filepath.ToSlash(name)
	for _, f := range zr.File {
		if filepath.ToSlash(f.Name) == name {
			return f
		}
	}
	return nil
}

func setDirectCell(sh *Sheet, col, row int, rawText string) {
	setDirectCellWithFormat(sh, col, row, rawText, nil)
}

func setDirectCellWithFormat(sh *Sheet, col, row int, rawText string, fmtSpec *cell.CellFormat) {
	if strings.TrimSpace(rawText) == "" {
		return
	}
	c := cell.NewCell(rawText, fmtSpec)
	sh.cells[CellCoord{Col: col, Row: row}] = c
	if col > sh.maxPopulatedCol {
		sh.maxPopulatedCol = col
	}
	if row > sh.maxPopulatedRow {
		sh.maxPopulatedRow = row
	}
}

func setDirectLabelCell(sh *Sheet, col, row int, text string) {
	if text == "" {
		return
	}
	c := cell.NewLabelCell(text, nil)
	sh.cells[CellCoord{Col: col, Row: row}] = c
	if col > sh.maxPopulatedCol {
		sh.maxPopulatedCol = col
	}
	if row > sh.maxPopulatedRow {
		sh.maxPopulatedRow = row
	}
}

func setDirectBooleanCell(sh *Sheet, col, row int, b bool) {
	c := cell.NewBooleanCell(b, nil)
	sh.cells[CellCoord{Col: col, Row: row}] = c
	if col > sh.maxPopulatedCol {
		sh.maxPopulatedCol = col
	}
	if row > sh.maxPopulatedRow {
		sh.maxPopulatedRow = row
	}
}

// ConvertODSFormulaForTest exposes convertODSFormula for regression tests.
func ConvertODSFormulaForTest(f string) string {
	return convertODSFormula(f)
}

// IsExcelDateNumFmtForTest exposes date-format detection for tests.
func IsExcelDateNumFmtForTest(id int, code string) bool {
	return isExcelDateNumFmt(id, code)
}
