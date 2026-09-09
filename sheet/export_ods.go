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

// ExportODS exports the workbook with all its sheets to an OpenDocument .ods file.
// GraphConfig is not written; graphs stay in .hwk only.
func (wb *Workbook) ExportODS(filepathStr string) error {
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

	// 1. mimetype (must be uncompressed Store method and first entry)
	fh := &zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	}
	mw, err := zw.CreateHeader(fh)
	if err != nil {
		return err
	}
	if _, err := mw.Write([]byte("application/vnd.oasis.opendocument.spreadsheet")); err != nil {
		return err
	}

	// 2. META-INF/manifest.xml
	if err := writeODSManifest(zw); err != nil {
		return err
	}

	// 3. styles.xml
	if err := writeODSStyles(zw); err != nil {
		return err
	}

	// 4. content.xml
	if err := writeODSContent(zw, sheets); err != nil {
		return err
	}

	// 5. settings.xml (freeze panes)
	if err := writeODSSettings(zw, sheets); err != nil {
		return err
	}

	return zw.Close()
}

// ExportODS exports a single sheet to an OpenDocument .ods file.
func (s *Sheet) ExportODS(filepath string) error {
	_, _, maxC, maxR := s.BoundingBox()
	return s.ExportODSRange(filepath, 0, 0, maxC, maxR)
}

// ExportODSRange exports a rectangular range of cells from the sheet as an OpenDocument .ods file (values only, no formulas).
func (s *Sheet) ExportODSRange(filepathStr string, minCol, minRow, maxCol, maxRow int) error {
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

	// 1. mimetype
	fh := &zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	}
	mw, err := zw.CreateHeader(fh)
	if err != nil {
		return err
	}
	if _, err := mw.Write([]byte("application/vnd.oasis.opendocument.spreadsheet")); err != nil {
		return err
	}

	// 2. META-INF/manifest.xml
	if err := writeODSManifest(zw); err != nil {
		return err
	}

	// 3. styles.xml
	if err := writeODSStyles(zw); err != nil {
		return err
	}

	// 4. content.xml (range values only)
	if err := writeODSContentRangeValuesOnly(zw, s, minCol, minRow, maxCol, maxRow); err != nil {
		return err
	}

	if err := writeODSSettings(zw, []*Sheet{s}); err != nil {
		return err
	}

	return zw.Close()
}

func writeODSContentRangeValuesOnly(zw *zip.Writer, sh *Sheet, minCol, minRow, maxCol, maxRow int) error {
	w, err := zw.Create("content.xml")
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"` + "\n")
	sb.WriteString(`  xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"` + "\n")
	sb.WriteString(`  xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"` + "\n")
	sb.WriteString(`  xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0"` + "\n")
	sb.WriteString(`  office:version="1.2">` + "\n")
	sb.WriteString(`  <office:body>` + "\n")
	sb.WriteString(`    <office:spreadsheet>` + "\n")

	name := sh.Name()
	if name == "" {
		name = "Sheet1"
	}
	var nameBuf bytes.Buffer
	xml.EscapeText(&nameBuf, []byte(name))

	sb.WriteString(fmt.Sprintf(`      <table:table table:name="%s">`+"\n", nameBuf.String()))

	for r := minRow; r <= maxRow; r++ {
		sb.WriteString(`        <table:table-row>` + "\n")
		for c := minCol; c <= maxCol; c++ {
			cellData := sh.cells[CellCoord{Col: c, Row: r}]
			if cellData == nil || cellData.Type == cell.TypeEmpty {
				sb.WriteString(`          <table:table-cell/>` + "\n")
				continue
			}

			switch v := cellData.Value.(type) {
			case float64:
				valStr := strconv.FormatFloat(v, 'f', -1, 64)
				sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="float" office:value="%s"><text:p>%s</text:p></table:table-cell>`+"\n", valStr, valStr))
			case int:
				valStr := strconv.Itoa(v)
				sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="float" office:value="%s"><text:p>%s</text:p></table:table-cell>`+"\n", valStr, valStr))
			case string:
				var strBuf bytes.Buffer
				xml.EscapeText(&strBuf, []byte(v))
				sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="string"><text:p>%s</text:p></table:table-cell>`+"\n", strBuf.String()))
			case bool:
				bVal := "false"
				if v {
					bVal = "true"
				}
				sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="boolean" office:boolean-value="%s"><text:p>%s</text:p></table:table-cell>`+"\n", bVal, bVal))
			case cell.LotusError:
				sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="string"><text:p>%s</text:p></table:table-cell>`+"\n", v.Code))
			default:
				textVal := cellData.RawInput
				if strings.HasPrefix(textVal, "'") || strings.HasPrefix(textVal, "\"") || strings.HasPrefix(textVal, "^") || strings.HasPrefix(textVal, "\\") {
					textVal = textVal[1:]
				}
				var strBuf bytes.Buffer
				xml.EscapeText(&strBuf, []byte(textVal))
				sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="string"><text:p>%s</text:p></table:table-cell>`+"\n", strBuf.String()))
			}
		}
		sb.WriteString(`        </table:table-row>` + "\n")
	}

	sb.WriteString(`      </table:table>` + "\n")
	sb.WriteString(`    </office:spreadsheet>` + "\n")
	sb.WriteString(`  </office:body>` + "\n")
	sb.WriteString(`</office:document-content>`)

	_, err = w.Write([]byte(sb.String()))
	return err
}

func writeODSNamedExpressions(sb *strings.Builder, named map[string]any, sheets []*Sheet) {
	if len(named) == 0 {
		return
	}
	defaultSheet := "Sheet1"
	if len(sheets) > 0 && sheets[0] != nil && sheets[0].Name() != "" {
		defaultSheet = sheets[0].Name()
	}
	sb.WriteString(`      <table:named-expressions>` + "\n")
	keys := make([]string, 0, len(named))
	for name := range named {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		v := named[name]
		switch v.(type) {
		case coord.CellRef, coord.RangeRef, formula.NamedExpr:
			writeODSNamedEntry(sb, name, v, defaultSheet)
		default:
			writeODSNamedEntry(sb, name, decodeNamedValue(v), defaultSheet)
		}
	}
	sb.WriteString(`      </table:named-expressions>` + "\n")
}

func writeODSNamedEntry(sb *strings.Builder, name string, v any, defaultSheet string) {
	if v == nil {
		return
	}
	var nameBuf bytes.Buffer
	xml.EscapeText(&nameBuf, []byte(name))
	switch t := v.(type) {
	case coord.CellRef, coord.RangeRef:
		addr := odsNamedAddress(t, defaultSheet)
		if addr == "" {
			return
		}
		var addrBuf bytes.Buffer
		xml.EscapeText(&addrBuf, []byte(addr))
		sb.WriteString(fmt.Sprintf(`        <table:named-range table:name="%s" table:cell-range-address="%s"/>`+"\n", nameBuf.String(), addrBuf.String()))
	case formula.NamedExpr:
		expr := toOpenFormulaExport(t.Expr)
		var exprBuf bytes.Buffer
		xml.EscapeText(&exprBuf, []byte(expr))
		base := ""
		if t.HasBase {
			base = odsNamedAddress(coord.CellRef{Col: t.BaseCol, Row: t.BaseRow}, defaultSheet)
		}
		if base != "" {
			var baseBuf bytes.Buffer
			xml.EscapeText(&baseBuf, []byte(base))
			sb.WriteString(fmt.Sprintf(`        <table:named-expression table:name="%s" table:expression="%s" table:base-cell-address="%s"/>`+"\n", nameBuf.String(), exprBuf.String(), baseBuf.String()))
		} else {
			sb.WriteString(fmt.Sprintf(`        <table:named-expression table:name="%s" table:expression="%s"/>`+"\n", nameBuf.String(), exprBuf.String()))
		}
	}
}

func odsNamedAddress(v any, defaultSheet string) string {
	sheetName := defaultSheet
	var start, end coord.CellRef
	switch t := v.(type) {
	case coord.CellRef:
		if t.Sheet != "" {
			sheetName = t.Sheet
		}
		start, end = t, t
	case coord.RangeRef:
		if t.Sheet != "" {
			sheetName = t.Sheet
		} else if t.Start.Sheet != "" {
			sheetName = t.Start.Sheet
		}
		start, end = t.Start, t.End
	default:
		return ""
	}
	sheetTok := "$" + sheetName
	if strings.ContainsAny(sheetName, " '!.") {
		sheetTok = "$'" + strings.ReplaceAll(sheetName, "'", "''") + "'"
	}
	a := fmt.Sprintf("%s.$%s$%d", sheetTok, coord.ColToLetter(start.Col), start.Row+1)
	if start.Col != end.Col || start.Row != end.Row {
		a += fmt.Sprintf(":$%s$%d", coord.ColToLetter(end.Col), end.Row+1)
	}
	return a
}

func writeODSManifest(zw *zip.Writer) error {
	w, err := zw.Create("META-INF/manifest.xml")
	if err != nil {
		return err
	}
	content := `<?xml version="1.0" encoding="UTF-8"?>
<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0" manifest:version="1.2">
  <manifest:file-entry manifest:full-path="/" manifest:version="1.2" manifest:media-type="application/vnd.oasis.opendocument.spreadsheet"/>
  <manifest:file-entry manifest:full-path="content.xml" manifest:media-type="text/xml"/>
  <manifest:file-entry manifest:full-path="styles.xml" manifest:media-type="text/xml"/>
  <manifest:file-entry manifest:full-path="settings.xml" manifest:media-type="text/xml"/>
  <manifest:file-entry manifest:full-path="mimetype" manifest:media-type="text/plain"/>
</manifest:manifest>`
	_, err = w.Write([]byte(content))
	return err
}

func writeODSStyles(zw *zip.Writer) error {
	w, err := zw.Create("styles.xml")
	if err != nil {
		return err
	}
	content := `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"
  xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"
  office:version="1.2">
</office:document-styles>`
	_, err = w.Write([]byte(content))
	return err
}

func writeODSSettings(zw *zip.Writer, sheets []*Sheet) error {
	w, err := zw.Create("settings.xml")
	if err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<office:document-settings xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"` + "\n")
	sb.WriteString(`  xmlns:config="urn:oasis:names:tc:opendocument:xmlns:config:1.0"` + "\n")
	sb.WriteString(`  office:version="1.2">` + "\n")
	sb.WriteString(`  <office:settings>` + "\n")
	sb.WriteString(`    <config:config-item-set config:name="ooo:view-settings">` + "\n")
	sb.WriteString(`      <config:config-item-map-indexed config:name="Views">` + "\n")
	sb.WriteString(`        <config:config-item-map-entry>` + "\n")
	sb.WriteString(`          <config:config-item config:name="ViewId" config:type="string">view1</config:config-item>` + "\n")
	sb.WriteString(`          <config:config-item-map-named config:name="Tables">` + "\n")
	for _, sh := range sheets {
		if sh == nil {
			continue
		}
		name := sh.Name()
		if name == "" {
			name = "Sheet1"
		}
		fr, fc := sh.FrozenRows(), sh.FrozenCols()
		hMode, vMode := 0, 0
		if fc > 0 {
			hMode = 2
		}
		if fr > 0 {
			vMode = 2
		}
		var nameBuf bytes.Buffer
		xml.EscapeText(&nameBuf, []byte(name))
		sb.WriteString(fmt.Sprintf(`            <config:config-item-map-entry config:name="%s">`+"\n", nameBuf.String()))
		sb.WriteString(fmt.Sprintf(`              <config:config-item config:name="HorizontalSplitMode" config:type="short">%d</config:config-item>`+"\n", hMode))
		sb.WriteString(fmt.Sprintf(`              <config:config-item config:name="VerticalSplitMode" config:type="short">%d</config:config-item>`+"\n", vMode))
		sb.WriteString(fmt.Sprintf(`              <config:config-item config:name="HorizontalSplitPosition" config:type="int">%d</config:config-item>`+"\n", fc))
		sb.WriteString(fmt.Sprintf(`              <config:config-item config:name="VerticalSplitPosition" config:type="int">%d</config:config-item>`+"\n", fr))
		sb.WriteString(`              <config:config-item config:name="PositionLeft" config:type="int">0</config:config-item>` + "\n")
		sb.WriteString(fmt.Sprintf(`              <config:config-item config:name="PositionRight" config:type="int">%d</config:config-item>`+"\n", fc))
		sb.WriteString(`              <config:config-item config:name="PositionTop" config:type="int">0</config:config-item>` + "\n")
		sb.WriteString(fmt.Sprintf(`              <config:config-item config:name="PositionBottom" config:type="int">%d</config:config-item>`+"\n", fr))
		sb.WriteString(`            </config:config-item-map-entry>` + "\n")
	}
	sb.WriteString(`          </config:config-item-map-named>` + "\n")
	sb.WriteString(`        </config:config-item-map-entry>` + "\n")
	sb.WriteString(`      </config:config-item-map-indexed>` + "\n")
	sb.WriteString(`    </config:config-item-set>` + "\n")
	autoCalc := "true"
	for _, sh := range sheets {
		if sh != nil && strings.EqualFold(sh.RecalcMode(), "MANUAL") {
			autoCalc = "false"
			break
		}
	}
	sb.WriteString(`    <config:config-item-set config:name="ooo:configuration-settings">` + "\n")
	sb.WriteString(fmt.Sprintf(`      <config:config-item config:name="AutoCalculate" config:type="boolean">%s</config:config-item>`+"\n", autoCalc))
	sb.WriteString(`    </config:config-item-set>` + "\n")
	sb.WriteString(`  </office:settings>` + "\n")
	sb.WriteString(`</office:document-settings>`)
	_, err = w.Write([]byte(sb.String()))
	return err
}

func writeODSContent(zw *zip.Writer, sheets []*Sheet) error {
	w, err := zw.Create("content.xml")
	if err != nil {
		return err
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	sb.WriteString(`<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0"` + "\n")
	sb.WriteString(`  xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0"` + "\n")
	sb.WriteString(`  xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0"` + "\n")
	sb.WriteString(`  xmlns:table="urn:oasis:names:tc:opendocument:xmlns:table:1.0"` + "\n")
	sb.WriteString(`  office:version="1.2">` + "\n")
	sb.WriteString(`  <office:body>` + "\n")
	sb.WriteString(`    <office:spreadsheet>` + "\n")

	for i, sh := range sheets {
		name := sh.Name()
		if name == "" {
			name = fmt.Sprintf("Sheet%d", i+1)
		}
		var nameBuf bytes.Buffer
		xml.EscapeText(&nameBuf, []byte(name))

		sb.WriteString(fmt.Sprintf(`      <table:table table:name="%s">`+"\n", nameBuf.String()))

		// Determine max populated row and col in this sheet
		maxC, maxR := -1, -1
		for pt := range sh.cells {
			if pt.Col > maxC {
				maxC = pt.Col
			}
			if pt.Row > maxR {
				maxR = pt.Row
			}
		}

		if maxR >= 0 && maxC >= 0 {
			for r := 0; r <= maxR; r++ {
				sb.WriteString(`        <table:table-row>` + "\n")
				for c := 0; c <= maxC; c++ {
					cellData := sh.cells[CellCoord{Col: c, Row: r}]
					if cellData == nil {
						sb.WriteString(`          <table:table-cell/>` + "\n")
						continue
					}

					switch cellData.Type {
					case cell.TypeFormula:
						formulaText := toOpenFormulaExport(cellData.RawInput)
						var fBuf bytes.Buffer
						xml.EscapeText(&fBuf, []byte(formulaText))

						valStr := ""
						valType := "string"
						if cellData.Value != nil {
							switch v := cellData.Value.(type) {
							case float64:
								valType = "float"
								valStr = strconv.FormatFloat(v, 'f', -1, 64)
							case int:
								valType = "float"
								valStr = strconv.Itoa(v)
							case bool:
								valType = "boolean"
								if v {
									valStr = "true"
								} else {
									valStr = "false"
								}
							case string:
								valType = "string"
								valStr = v
							case cell.LotusError:
								valType = "string"
								valStr = v.Code
							}
						}
						var vBuf bytes.Buffer
						xml.EscapeText(&vBuf, []byte(valStr))

						if valType == "float" {
							sb.WriteString(fmt.Sprintf(`          <table:table-cell table:formula="of:=%s" office:value-type="float" office:value="%s"><text:p>%s</text:p></table:table-cell>`+"\n", fBuf.String(), valStr, vBuf.String()))
						} else if valType == "boolean" {
							sb.WriteString(fmt.Sprintf(`          <table:table-cell table:formula="of:=%s" office:value-type="boolean" office:boolean-value="%s"><text:p>%s</text:p></table:table-cell>`+"\n", fBuf.String(), valStr, vBuf.String()))
						} else {
							sb.WriteString(fmt.Sprintf(`          <table:table-cell table:formula="of:=%s" office:value-type="string"><text:p>%s</text:p></table:table-cell>`+"\n", fBuf.String(), vBuf.String()))
						}

					case cell.TypeNumber:
						valStr := "0"
						if num, ok := cellData.Value.(float64); ok {
							valStr = strconv.FormatFloat(num, 'f', -1, 64)
						} else if cellData.RawInput != "" {
							valStr = cellData.RawInput
						}
						sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="float" office:value="%s"><text:p>%s</text:p></table:table-cell>`+"\n", valStr, valStr))

					case cell.TypeLabel:
						textVal := ""
						if sVal, ok := cellData.Value.(string); ok {
							textVal = sVal
						} else {
							textVal = cellData.RawInput
							if strings.HasPrefix(textVal, "'") || strings.HasPrefix(textVal, "\"") || strings.HasPrefix(textVal, "^") || strings.HasPrefix(textVal, "\\") {
								textVal = textVal[1:]
							}
						}
						var strBuf bytes.Buffer
						xml.EscapeText(&strBuf, []byte(textVal))
						sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="string"><text:p>%s</text:p></table:table-cell>`+"\n", strBuf.String()))

					default:
						if errVal, ok := cellData.Value.(cell.LotusError); ok {
							var strBuf bytes.Buffer
							xml.EscapeText(&strBuf, []byte(errVal.Code))
							sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="string"><text:p>%s</text:p></table:table-cell>`+"\n", strBuf.String()))
						} else if b, ok := cellData.Value.(bool); ok {
							bVal := "false"
							if b {
								bVal = "true"
							}
							sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="boolean" office:boolean-value="%s"><text:p>%s</text:p></table:table-cell>`+"\n", bVal, bVal))
						} else {
							var strBuf bytes.Buffer
							xml.EscapeText(&strBuf, []byte(cellData.RawInput))
							sb.WriteString(fmt.Sprintf(`          <table:table-cell office:value-type="string"><text:p>%s</text:p></table:table-cell>`+"\n", strBuf.String()))
						}
					}
				}
				sb.WriteString(`        </table:table-row>` + "\n")
			}
		}

		writeODSNamedExpressions(&sb, sh.namedRanges, []*Sheet{sh})
		sb.WriteString(`      </table:table>` + "\n")
	}

	var wbNames map[string]any
	if len(sheets) > 0 && sheets[0] != nil && sheets[0].workbook != nil {
		wbNames = sheets[0].workbook.NamedRanges
	}
	writeODSNamedExpressions(&sb, wbNames, sheets)

	sb.WriteString(`    </office:spreadsheet>` + "\n")
	sb.WriteString(`  </office:body>` + "\n")
	sb.WriteString(`</office:document-content>`)

	_, err = w.Write([]byte(sb.String()))
	return err
}
