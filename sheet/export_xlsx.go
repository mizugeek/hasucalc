package sheet

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"hasucalc/cell"
	"hasucalc/coord"
	"hasucalc/formula"
)

// SharedStringsTable manages unique strings across all sheets for standard OpenXML compliance.
type sharedStringsTable struct {
	list       []string
	indexMap   map[string]int
	totalCount int
}

func newSharedStringsTable() *sharedStringsTable {
	return &sharedStringsTable{
		indexMap: make(map[string]int),
	}
}

func (sst *sharedStringsTable) add(s string) int {
	sst.totalCount++
	if idx, ok := sst.indexMap[s]; ok {
		return idx
	}
	idx := len(sst.list)
	sst.list = append(sst.list, s)
	sst.indexMap[s] = idx
	return idx
}

// ExportXLSX exports the workbook with all its sheets to a standard Excel .xlsx file.
// GraphConfig is not written (no xl/charts); graphs stay in .hwk only.
func (wb *Workbook) ExportXLSX(filepathStr string) error {
	if dir := filepath.Dir(filepathStr); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	f, err := os.Create(filepathStr)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)

	sheets := wb.Sheets
	if len(sheets) == 0 {
		sheets = []*Sheet{NewSheet()}
	}

	// 1. Build Shared Strings Table
	sst := newSharedStringsTable()
	for _, sh := range sheets {
		for _, cData := range sh.cells {
			if cData == nil {
				continue
			}
			switch cData.Type {
			case cell.TypeLabel:
				strVal := ""
				if s, ok := cData.Value.(string); ok {
					strVal = s
				} else {
					strVal = cData.RawInput
					if strings.HasPrefix(strVal, "'") || strings.HasPrefix(strVal, "\"") || strings.HasPrefix(strVal, "^") || strings.HasPrefix(strVal, "\\") {
						strVal = strVal[1:]
					}
				}
				sst.add(strVal)
			case cell.TypeNumber, cell.TypeFormula, cell.TypeEmpty, cell.TypeBoolean:
				// Numbers, formulas (which emit t="str"), empty, and booleans do not use sst
			default:
				if _, ok := cData.Value.(bool); !ok && strings.TrimSpace(cData.RawInput) != "" {
					sst.add(cData.RawInput)
				}
			}
		}
	}

	// 2. [Content_Types].xml
	if err := writeXLSXContentTypes(zw, len(sheets), len(sst.list) > 0); err != nil {
		return err
	}

	// 3. _rels/.rels
	if err := writeXLSXRels(zw); err != nil {
		return err
	}

	// 4. xl/workbook.xml
	if err := writeXLSXWorkbookXML(zw, sheets); err != nil {
		return err
	}

	// 5. xl/_rels/workbook.xml.rels
	if err := writeXLSXWorkbookRels(zw, len(sheets), len(sst.list) > 0); err != nil {
		return err
	}

	// 6. xl/styles.xml
	if err := writeXLSXStyles(zw); err != nil {
		return err
	}

	// 7. xl/sharedStrings.xml (if strings exist)
	if len(sst.list) > 0 {
		if err := writeXLSXSharedStrings(zw, sst); err != nil {
			return err
		}
	}

	// 8. xl/worksheets/sheetN.xml
	for i, sh := range sheets {
		sheetPath := fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
		if err := writeXLSXWorksheet(zw, sheetPath, sh, sst); err != nil {
			return err
		}
	}

	return zw.Close()
}

// ExportXLSX exports a single sheet to an Excel .xlsx file.
func (s *Sheet) ExportXLSX(filepathStr string) error {
	_, _, maxC, maxR := s.BoundingBox()
	return s.ExportXLSXRange(filepathStr, 0, 0, maxC, maxR)
}

// ExportXLSXRange exports a rectangular range of cells from the sheet as an Excel .xlsx file (values only, no formulas).
func (s *Sheet) ExportXLSXRange(filepathStr string, minCol, minRow, maxCol, maxRow int) error {
	if minCol > maxCol {
		minCol, maxCol = maxCol, minCol
	}
	if minRow > maxRow {
		minRow, maxRow = maxRow, minRow
	}

	if dir := filepath.Dir(filepathStr); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	f, err := os.Create(filepathStr)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// 1. Build Shared Strings Table for the range (values only) — must include
	// every string the worksheet writer may emit (including errors).
	sst := newSharedStringsTable()
	for r := minRow; r <= maxRow; r++ {
		for c := minCol; c <= maxCol; c++ {
			cData := s.cells[CellCoord{Col: c, Row: r}]
			if cData == nil || cData.Type == cell.TypeEmpty {
				continue
			}
			switch v := cData.Value.(type) {
			case string:
				sst.add(v)
			case cell.LotusError:
				sst.add(v.Code)
			default:
				if cData.Type == cell.TypeLabel {
					strVal := cData.RawInput
					if strings.HasPrefix(strVal, "'") || strings.HasPrefix(strVal, "\"") || strings.HasPrefix(strVal, "^") || strings.HasPrefix(strVal, "\\") {
						strVal = strVal[1:]
					}
					sst.add(strVal)
				} else if cData.Type != cell.TypeNumber && cData.Type != cell.TypeFormula {
					strVal := cData.RawInput
					if strings.HasPrefix(strVal, "'") || strings.HasPrefix(strVal, "\"") || strings.HasPrefix(strVal, "^") || strings.HasPrefix(strVal, "\\") {
						strVal = strVal[1:]
					}
					if strVal != "" {
						sst.add(strVal)
					}
				} else if cData.Value == nil && cData.Type != cell.TypeNumber {
					strVal := cData.RawInput
					if strings.HasPrefix(strVal, "'") || strings.HasPrefix(strVal, "\"") || strings.HasPrefix(strVal, "^") || strings.HasPrefix(strVal, "\\") {
						strVal = strVal[1:]
					}
					if strVal != "" {
						sst.add(strVal)
					}
				}
			}
		}
	}

	// 2. [Content_Types].xml
	if err := writeXLSXContentTypes(zw, 1, len(sst.list) > 0); err != nil {
		return err
	}

	// 3. _rels/.rels
	if err := writeXLSXRels(zw); err != nil {
		return err
	}

	// 4. xl/workbook.xml
	if err := writeXLSXWorkbookXML(zw, []*Sheet{s}); err != nil {
		return err
	}

	// 5. xl/_rels/workbook.xml.rels
	if err := writeXLSXWorkbookRels(zw, 1, len(sst.list) > 0); err != nil {
		return err
	}

	// 6. xl/styles.xml
	if err := writeXLSXStyles(zw); err != nil {
		return err
	}

	// 7. xl/sharedStrings.xml
	if len(sst.list) > 0 {
		if err := writeXLSXSharedStrings(zw, sst); err != nil {
			return err
		}
	}

	// 8. xl/worksheets/sheet1.xml (write range values only)
	return writeXLSXWorksheetRangeValuesOnly(zw, "xl/worksheets/sheet1.xml", s, sst, minCol, minRow, maxCol, maxRow)
}

func writeXLSXWorksheetRangeValuesOnly(zw *zip.Writer, entryPath string, sh *Sheet, sst *sharedStringsTable, minCol, minRow, maxCol, maxRow int) error {
	w, err := zw.Create(entryPath)
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + "\n")

	// 1. Dimension
	dimRef := fmt.Sprintf("%s:%s", coord.CellRef{Col: minCol, Row: minRow}.String(), coord.CellRef{Col: maxCol, Row: maxRow}.String())
	sb.WriteString(fmt.Sprintf(`  <dimension ref="%s"/>`+"\n", dimRef))

	// 2. SheetViews
	writeXLSXSheetViews(&sb, sh.FrozenRows(), sh.FrozenCols())

	// 3. SheetFormatPr
	sb.WriteString(`  <sheetFormatPr defaultRowHeight="15"/>` + "\n")

	// 4. Column Widths
	if len(sh.colWidths) > 0 {
		var cols []int
		for c := minCol; c <= maxCol; c++ {
			if _, ok := sh.colWidths[c]; ok {
				cols = append(cols, c)
			}
		}
		sort.Ints(cols)
		if len(cols) > 0 {
			sb.WriteString(`  <cols>` + "\n")
			for _, c := range cols {
				cw := sh.colWidths[c]
				sb.WriteString(fmt.Sprintf(`    <col min="%d" max="%d" width="%d" customWidth="1"/>`+"\n", c+1, c+1, cw))
			}
			sb.WriteString(`  </cols>` + "\n")
		}
	}

	// 5. SheetData
	sb.WriteString(`  <sheetData>` + "\n")
	for r := minRow; r <= maxRow; r++ {
		sb.WriteString(fmt.Sprintf(`    <row r="%d">`+"\n", r+1))
		for c := minCol; c <= maxCol; c++ {
			cellData := sh.cells[CellCoord{Col: c, Row: r}]
			if cellData == nil || cellData.Type == cell.TypeEmpty {
				continue
			}
			cellRef := coord.CellRef{Col: c, Row: r}.String()

			switch v := cellData.Value.(type) {
			case float64:
				sb.WriteString(fmt.Sprintf(`      <c r="%s"><v>%s</v></c>`+"\n", cellRef, strconv.FormatFloat(v, 'f', -1, 64)))
			case int:
				sb.WriteString(fmt.Sprintf(`      <c r="%s"><v>%d</v></c>`+"\n", cellRef, v))
			case string:
				idx := sst.add(v)
				sb.WriteString(fmt.Sprintf(`      <c r="%s" t="s"><v>%d</v></c>`+"\n", cellRef, idx))
			case bool:
				bVal := 0
				if v {
					bVal = 1
				}
				sb.WriteString(fmt.Sprintf(`      <c r="%s" t="b"><v>%d</v></c>`+"\n", cellRef, bVal))
			case cell.LotusError:
				idx := sst.add(v.Code)
				sb.WriteString(fmt.Sprintf(`      <c r="%s" t="s"><v>%d</v></c>`+"\n", cellRef, idx))
			default:
				if cellData.Type == cell.TypeNumber {
					valStr := "0"
					if cellData.RawInput != "" {
						valStr = cellData.RawInput
					}
					sb.WriteString(fmt.Sprintf(`      <c r="%s"><v>%s</v></c>`+"\n", cellRef, valStr))
				} else {
					strVal := cellData.RawInput
					if strings.HasPrefix(strVal, "'") || strings.HasPrefix(strVal, "\"") || strings.HasPrefix(strVal, "^") || strings.HasPrefix(strVal, "\\") {
						strVal = strVal[1:]
					}
					idx := sst.add(strVal)
					sb.WriteString(fmt.Sprintf(`      <c r="%s" t="s"><v>%d</v></c>`+"\n", cellRef, idx))
				}
			}
		}
		sb.WriteString(`    </row>` + "\n")
	}
	sb.WriteString(`  </sheetData>` + "\n")
	sb.WriteString(`  <pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>` + "\n")
	sb.WriteString(`</worksheet>`)

	_, err = w.Write([]byte(sb.String()))
	return err
}

func writeXLSXSheetViews(sb *strings.Builder, frozenRows, frozenCols int) {
	sb.WriteString(`  <sheetViews>` + "\n")
	if frozenRows <= 0 && frozenCols <= 0 {
		sb.WriteString(`    <sheetView workbookViewId="0"/>` + "\n")
		sb.WriteString(`  </sheetViews>` + "\n")
		return
	}
	topLeft := coord.CellRef{Col: frozenCols, Row: frozenRows}.String()
	sb.WriteString(`    <sheetView workbookViewId="0">` + "\n")
	sb.WriteString(fmt.Sprintf(`      <pane xSplit="%d" ySplit="%d" topLeftCell="%s" activePane="bottomRight" state="frozen"/>`+"\n", frozenCols, frozenRows, topLeft))
	sb.WriteString(`      <selection pane="bottomRight"/>` + "\n")
	sb.WriteString(`    </sheetView>` + "\n")
	sb.WriteString(`  </sheetViews>` + "\n")
}

func writeXLSXContentTypes(zw *zip.Writer, numSheets int, hasSharedStrings bool) error {
	w, err := zw.Create("[Content_Types].xml")
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` + "\n")
	sb.WriteString(`  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` + "\n")
	sb.WriteString(`  <Default Extension="xml" ContentType="application/xml"/>` + "\n")
	sb.WriteString(`  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` + "\n")
	sb.WriteString(`  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>` + "\n")
	if hasSharedStrings {
		sb.WriteString(`  <Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>` + "\n")
	}
	for i := 1; i <= numSheets; i++ {
		sb.WriteString(fmt.Sprintf(`  <Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`+"\n", i))
	}
	sb.WriteString(`</Types>`)
	_, err = w.Write([]byte(sb.String()))
	return err
}

func writeXLSXRels(zw *zip.Writer) error {
	w, err := zw.Create("_rels/.rels")
	if err != nil {
		return err
	}
	content := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
	_, err = w.Write([]byte(content))
	return err
}

func writeXLSXWorkbookXML(zw *zip.Writer, sheets []*Sheet) error {
	w, err := zw.Create("xl/workbook.xml")
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + "\n")
	sb.WriteString(`  <workbookPr date1904="false"/>` + "\n")
	sb.WriteString(`  <bookViews>` + "\n")
	sb.WriteString(`    <workbookView xWindow="0" yWindow="0" windowWidth="24000" windowHeight="12000"/>` + "\n")
	sb.WriteString(`  </bookViews>` + "\n")
	sb.WriteString(`  <sheets>` + "\n")
	for i, sh := range sheets {
		name := sh.Name()
		if name == "" {
			name = fmt.Sprintf("Sheet%d", i+1)
		}
		var nameBuf bytes.Buffer
		xml.EscapeText(&nameBuf, []byte(name))
		sb.WriteString(fmt.Sprintf(`    <sheet name="%s" sheetId="%d" r:id="rId%d"/>`+"\n", nameBuf.String(), i+1, i+1))
	}
	sb.WriteString(`  </sheets>` + "\n")
	writeXLSXDefinedNames(&sb, sheets)
	calcMode := "auto"
	for _, sh := range sheets {
		if sh != nil && strings.EqualFold(sh.RecalcMode(), "MANUAL") {
			calcMode = "manual"
			break
		}
	}
	if calcMode == "manual" {
		sb.WriteString(`  <calcPr calcId="124512" calcMode="manual" fullCalcOnLoad="1"/>` + "\n")
	} else {
		sb.WriteString(`  <calcPr calcId="124512"/>` + "\n")
	}
	sb.WriteString(`</workbook>`)
	_, err = w.Write([]byte(sb.String()))
	return err
}

func writeXLSXDefinedNames(sb *strings.Builder, sheets []*Sheet) {
	type item struct {
		name    string
		localID int
		formula string
		comment string
	}
	var items []item
	fallback := "Sheet1"
	if len(sheets) > 0 && sheets[0] != nil && sheets[0].Name() != "" {
		fallback = sheets[0].Name()
	}
	if len(sheets) > 0 && sheets[0] != nil && sheets[0].workbook != nil {
		for name, v := range sheets[0].workbook.NamedRanges {
			f, cmt := xlsxDefinedNameParts(v, fallback)
			items = append(items, item{name: name, localID: -1, formula: f, comment: cmt})
		}
	}
	for i, sh := range sheets {
		if sh == nil {
			continue
		}
		fb := sh.Name()
		if fb == "" {
			fb = fallback
		}
		for name, v := range sh.namedRanges {
			f, cmt := xlsxDefinedNameParts(v, fb)
			items = append(items, item{name: name, localID: i, formula: f, comment: cmt})
		}
	}
	if len(items) == 0 {
		return
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].localID != items[j].localID {
			return items[i].localID < items[j].localID
		}
		return items[i].name < items[j].name
	})
	sb.WriteString(`  <definedNames>` + "\n")
	for _, it := range items {
		var nameBuf bytes.Buffer
		xml.EscapeText(&nameBuf, []byte(it.name))
		var fBuf bytes.Buffer
		xml.EscapeText(&fBuf, []byte(it.formula))
		commentAttr := ""
		if it.comment != "" {
			var cBuf bytes.Buffer
			xml.EscapeText(&cBuf, []byte(it.comment))
			commentAttr = fmt.Sprintf(` comment="%s"`, cBuf.String())
		}
		if it.localID >= 0 {
			sb.WriteString(fmt.Sprintf(`    <definedName name="%s"%s localSheetId="%d">%s</definedName>`+"\n", nameBuf.String(), commentAttr, it.localID, fBuf.String()))
		} else {
			sb.WriteString(fmt.Sprintf(`    <definedName name="%s"%s>%s</definedName>`+"\n", nameBuf.String(), commentAttr, fBuf.String()))
		}
	}
	sb.WriteString(`  </definedNames>` + "\n")
}

func xlsxDefinedNameParts(v any, fallbackSheet string) (formulaText, comment string) {
	switch t := v.(type) {
	case coord.CellRef:
		return xlsxAbsCell(t, fallbackSheet), ""
	case coord.RangeRef:
		return xlsxAbsRange(t, fallbackSheet), ""
	case formula.NamedExpr:
		e := strings.TrimPrefix(strings.TrimSpace(t.Expr), "=")
		return mapFormulaNonStrings(e, func(part string) string {
			return strings.ReplaceAll(part, "..", ":")
		}), hasuCalcBaseComment(t)
	default:
		s := strings.TrimPrefix(fmt.Sprintf("%v", v), "=")
		return mapFormulaNonStrings(s, func(part string) string {
			return strings.ReplaceAll(part, "..", ":")
		}), ""
	}
}

func xlsxQuoteSheetPrefix(name string) string {
	return coord.QuoteSheetPrefix(name)
}

func xlsxAbsCell(c coord.CellRef, fallback string) string {
	sh := c.Sheet
	if sh == "" {
		sh = fallback
	}
	return fmt.Sprintf("%s$%s$%d", xlsxQuoteSheetPrefix(sh), coord.ColToLetter(c.Col), c.Row+1)
}

func xlsxAbsRange(r coord.RangeRef, fallback string) string {
	sh := r.Sheet
	if sh == "" {
		sh = r.Start.Sheet
	}
	if sh == "" {
		sh = fallback
	}
	prefix := xlsxQuoteSheetPrefix(sh)
	if r.Start.Col == r.End.Col && r.Start.Row == r.End.Row {
		return fmt.Sprintf("%s$%s$%d", prefix, coord.ColToLetter(r.Start.Col), r.Start.Row+1)
	}
	return fmt.Sprintf("%s$%s$%d:$%s$%d", prefix, coord.ColToLetter(r.Start.Col), r.Start.Row+1, coord.ColToLetter(r.End.Col), r.End.Row+1)
}

func writeXLSXWorkbookRels(zw *zip.Writer, numSheets int, hasSharedStrings bool) error {
	w, err := zw.Create("xl/_rels/workbook.xml.rels")
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + "\n")
	for i := 1; i <= numSheets; i++ {
		sb.WriteString(fmt.Sprintf(`  <Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`+"\n", i, i))
	}
	stylesId := numSheets + 1
	sb.WriteString(fmt.Sprintf(`  <Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`+"\n", stylesId))
	if hasSharedStrings {
		sstId := numSheets + 2
		sb.WriteString(fmt.Sprintf(`  <Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>`+"\n", sstId))
	}
	sb.WriteString(`</Relationships>`)
	_, err = w.Write([]byte(sb.String()))
	return err
}

func writeXLSXStyles(zw *zip.Writer) error {
	w, err := zw.Create("xl/styles.xml")
	if err != nil {
		return err
	}
	content := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1">
    <font>
      <sz val="11"/>
      <color theme="1"/>
      <name val="Calibri"/>
      <family val="2"/>
      <scheme val="minor"/>
    </font>
  </fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1">
    <border><left/><right/><top/><bottom/><diagonal/></border>
  </borders>
  <cellStyleXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>
  </cellStyleXfs>
  <cellXfs count="1">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
  </cellXfs>
  <cellStyles count="1">
    <cellStyle name="Normal" xfId="0" builtinId="0"/>
  </cellStyles>
  <dxfs count="0"/>
  <tableStyles count="0" defaultTableStyle="TableStyleMedium9" defaultPivotStyle="PivotStyleLight16"/>
</styleSheet>`
	_, err = w.Write([]byte(content))
	return err
}

func writeXLSXSharedStrings(zw *zip.Writer, sst *sharedStringsTable) error {
	w, err := zw.Create("xl/sharedStrings.xml")
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString(fmt.Sprintf(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">`+"\n", sst.totalCount, len(sst.list)))
	for _, str := range sst.list {
		var buf bytes.Buffer
		xml.EscapeText(&buf, []byte(str))
		escaped := buf.String()
		if strings.HasPrefix(str, " ") || strings.HasSuffix(str, " ") || strings.Contains(str, "\n") {
			sb.WriteString(fmt.Sprintf(`  <si><t xml:space="preserve">%s</t></si>`+"\n", escaped))
		} else {
			sb.WriteString(fmt.Sprintf(`  <si><t>%s</t></si>`+"\n", escaped))
		}
	}
	sb.WriteString(`</sst>`)
	_, err = w.Write([]byte(sb.String()))
	return err
}

func writeXLSXWorksheet(zw *zip.Writer, entryPath string, sh *Sheet, sst *sharedStringsTable) error {
	w, err := zw.Create(entryPath)
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` + "\n")

	// 1. Dimension
	minC, minR, maxC, maxR := 0, 0, 0, 0
	hasCells := false
	for pt := range sh.cells {
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
	dimRef := "A1"
	if hasCells {
		dimRef = fmt.Sprintf("%s:%s", coord.CellRef{Col: minC, Row: minR}.String(), coord.CellRef{Col: maxC, Row: maxR}.String())
	}
	sb.WriteString(fmt.Sprintf(`  <dimension ref="%s"/>`+"\n", dimRef))

	// 2. SheetViews
	writeXLSXSheetViews(&sb, sh.FrozenRows(), sh.FrozenCols())

	// 3. SheetFormatPr
	sb.WriteString(`  <sheetFormatPr defaultRowHeight="15"/>` + "\n")

	// 4. Column Widths
	if len(sh.colWidths) > 0 {
		var cols []int
		for c := range sh.colWidths {
			cols = append(cols, c)
		}
		sort.Ints(cols)
		sb.WriteString(`  <cols>` + "\n")
		for _, c := range cols {
			cw := sh.colWidths[c]
			sb.WriteString(fmt.Sprintf(`    <col min="%d" max="%d" width="%d" customWidth="1"/>`+"\n", c+1, c+1, cw))
		}
		sb.WriteString(`  </cols>` + "\n")
	}

	// 5. SheetData
	rowMap := make(map[int][]int)
	for pt := range sh.cells {
		rowMap[pt.Row] = append(rowMap[pt.Row], pt.Col)
	}

	var sortedRows []int
	for r := range rowMap {
		sortedRows = append(sortedRows, r)
	}
	sort.Ints(sortedRows)

	sb.WriteString(`  <sheetData>` + "\n")
	for _, r := range sortedRows {
		cols := rowMap[r]
		sort.Ints(cols)
		sb.WriteString(fmt.Sprintf(`    <row r="%d">`+"\n", r+1))
		for _, c := range cols {
			cellData := sh.cells[CellCoord{Col: c, Row: r}]
			if cellData == nil {
				continue
			}
			cellRef := coord.CellRef{Col: c, Row: r}.String()

			switch cellData.Type {
			case cell.TypeFormula:
				formulaText := toExcelExportFormula(cellData.RawInput)
				var fBuf bytes.Buffer
				xml.EscapeText(&fBuf, []byte(formulaText))

				if cellData.Value != nil {
					switch v := cellData.Value.(type) {
					case float64:
						sb.WriteString(fmt.Sprintf(`      <c r="%s"><f>%s</f><v>%s</v></c>`+"\n", cellRef, fBuf.String(), strconv.FormatFloat(v, 'f', -1, 64)))
					case int:
						sb.WriteString(fmt.Sprintf(`      <c r="%s"><f>%s</f><v>%d</v></c>`+"\n", cellRef, fBuf.String(), v))
					case string:
						var valBuf bytes.Buffer
						xml.EscapeText(&valBuf, []byte(v))
						sb.WriteString(fmt.Sprintf(`      <c r="%s" t="str"><f>%s</f><v>%s</v></c>`+"\n", cellRef, fBuf.String(), valBuf.String()))
					case bool:
						bVal := 0
						if v {
							bVal = 1
						}
						sb.WriteString(fmt.Sprintf(`      <c r="%s" t="b"><f>%s</f><v>%d</v></c>`+"\n", cellRef, fBuf.String(), bVal))
					case cell.LotusError:
						var errBuf bytes.Buffer
						xml.EscapeText(&errBuf, []byte(v.Code))
						sb.WriteString(fmt.Sprintf(`      <c r="%s" t="e"><f>%s</f><v>%s</v></c>`+"\n", cellRef, fBuf.String(), errBuf.String()))
					default:
						sb.WriteString(fmt.Sprintf(`      <c r="%s"><f>%s</f></c>`+"\n", cellRef, fBuf.String()))
					}
				} else {
					sb.WriteString(fmt.Sprintf(`      <c r="%s"><f>%s</f></c>`+"\n", cellRef, fBuf.String()))
				}

			case cell.TypeNumber:
				valStr := "0"
				if num, ok := cellData.Value.(float64); ok {
					valStr = strconv.FormatFloat(num, 'f', -1, 64)
				} else if cellData.RawInput != "" {
					valStr = cellData.RawInput
				}
				sb.WriteString(fmt.Sprintf(`      <c r="%s"><v>%s</v></c>`+"\n", cellRef, valStr))

			case cell.TypeLabel:
				strVal := ""
				if s, ok := cellData.Value.(string); ok {
					strVal = s
				} else {
					strVal = cellData.RawInput
					if strings.HasPrefix(strVal, "'") || strings.HasPrefix(strVal, "\"") || strings.HasPrefix(strVal, "^") || strings.HasPrefix(strVal, "\\") {
						strVal = strVal[1:]
					}
				}
				idx := sst.add(strVal)
				sb.WriteString(fmt.Sprintf(`      <c r="%s" t="s"><v>%d</v></c>`+"\n", cellRef, idx))

			case cell.TypeEmpty:
				continue

			case cell.TypeBoolean:
				bVal := 0
				if b, ok := cellData.Value.(bool); ok && b {
					bVal = 1
				} else if cellData.RawInput == "TRUE" {
					bVal = 1
				}
				sb.WriteString(fmt.Sprintf(`      <c r="%s" t="b"><v>%d</v></c>`+"\n", cellRef, bVal))

			default:
				if errVal, ok := cellData.Value.(cell.LotusError); ok {
					var errBuf bytes.Buffer
					xml.EscapeText(&errBuf, []byte(errVal.Code))
					sb.WriteString(fmt.Sprintf(`      <c r="%s" t="e"><v>%s</v></c>`+"\n", cellRef, errBuf.String()))
				} else if b, ok := cellData.Value.(bool); ok {
					bVal := 0
					if b {
						bVal = 1
					}
					sb.WriteString(fmt.Sprintf(`      <c r="%s" t="b"><v>%d</v></c>`+"\n", cellRef, bVal))
				} else if strings.TrimSpace(cellData.RawInput) == "" {
					continue
				} else {
					strVal := cellData.RawInput
					idx := sst.add(strVal)
					sb.WriteString(fmt.Sprintf(`      <c r="%s" t="s"><v>%d</v></c>`+"\n", cellRef, idx))
				}
			}
		}
		sb.WriteString(`    </row>` + "\n")
	}
	sb.WriteString(`  </sheetData>` + "\n")
	sb.WriteString(`  <pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/>` + "\n")
	sb.WriteString(`</worksheet>`)

	_, err = w.Write([]byte(sb.String()))
	return err
}
