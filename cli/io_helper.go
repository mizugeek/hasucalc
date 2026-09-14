package cli

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"hasucalc/cell"
	"hasucalc/sheet"
)

// DetectFormat attempts to identify the spreadsheet format from file path extension.
func DetectFormat(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".xlsx", ".xlsm":
		return "xlsx"
	case ".ods", ".ots":
		return "ods"
	case ".csv":
		return "csv"
	case ".tsv":
		return "tsv"
	case ".md", ".markdown":
		return "md"
	case ".html", ".htm":
		return "html"
	case ".hwkz":
		return "hwkz"
	case ".hwk", ".json":
		return "hwk"
	default:
		return ""
	}
}

// LoadWorkbookAuto loads a workbook from a filesystem path, auto-detecting format.
func LoadWorkbookAuto(path string) (*sheet.Workbook, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}

	fmtType := DetectFormat(path)
	switch fmtType {
	case "xlsx":
		return sheet.ImportXLSXWorkbook(path)
	case "ods":
		return sheet.ImportODSWorkbook(path)
	case "csv", "tsv":
		sh, err := sheet.ImportSheetCSV(path)
		if err != nil {
			return nil, err
		}
		baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		wb := sheet.NewWorkbook(baseName)
		wb.Sheets[0] = sh
		sh.SetWorkbook(wb)
		return wb, nil
	case "md", "html":
		return sheet.ImportMarkupWorkbook(path)
	default:
		// Attempt .hwk / .hwkz / json workbook or fallback single sheet
		return sheet.LoadWorkbookJSON(path)
	}
}

// LoadWorkbookFromReader loads a workbook from an io.Reader according to the specified format.
func LoadWorkbookFromReader(r io.Reader, format string) (*sheet.Workbook, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read input stream: %w", err)
	}

	format = strings.ToLower(strings.TrimPrefix(format, "."))
	switch format {
	case "csv", "tsv":
		// Strip UTF-8 BOM if present
		if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
			data = data[3:]
		}
		csvR := csv.NewReader(bytes.NewReader(data))
		csvR.LazyQuotes = true
		csvR.FieldsPerRecord = -1
		if format == "tsv" {
			csvR.Comma = '\t'
		}
		records, err := csvR.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("CSV parsing failed: %w", err)
		}
		sh := sheet.NewSheet()
		for rowIdx, row := range records {
			for colIdx, val := range row {
				if strings.TrimSpace(val) != "" {
					sh.SetCellInput(colIdx, rowIdx, val, nil)
				}
			}
		}
		sh.Recalculate()
		wb := sheet.NewWorkbook("Stream")
		wb.Sheets[0] = sh
		sh.SetWorkbook(wb)
		return wb, nil

	case "md", "markdown":
		wb, err := sheet.ImportMarkdownWorkbook(data, "Stream")
		if err != nil {
			return nil, fmt.Errorf("markdown parsing failed: %w", err)
		}
		return wb, nil

	case "html", "htm":
		wb, err := sheet.ImportHTMLWorkbook(data, "Stream")
		if err != nil {
			return nil, fmt.Errorf("HTML parsing failed: %w", err)
		}
		return wb, nil

	case "hwk", "hwkz", "json", "":
		// Write to a temporary file because LoadWorkbookJSON handles gzip / zip detection
		tmpFile, err := os.CreateTemp("", "hasucalc-stream-*.hwk")
		if err != nil {
			return nil, err
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		if _, err := tmpFile.Write(data); err != nil {
			tmpFile.Close()
			return nil, err
		}
		tmpFile.Close()
		return sheet.LoadWorkbookJSON(tmpPath)

	case "xlsx", "ods":
		// Binary formats require temp file to parse zip archive
		tmpFile, err := os.CreateTemp("", "hasucalc-stream-*."+format)
		if err != nil {
			return nil, err
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		if _, err := tmpFile.Write(data); err != nil {
			tmpFile.Close()
			return nil, err
		}
		tmpFile.Close()

		if format == "xlsx" {
			return sheet.ImportXLSXWorkbook(tmpPath)
		}
		return sheet.ImportODSWorkbook(tmpPath)

	default:
		return nil, fmt.Errorf("unsupported input format: %s", format)
	}
}

// GetTargetSheet retrieves the requested sheet or the active sheet if sheetName is empty.
func GetTargetSheet(wb *sheet.Workbook, sheetName string) (*sheet.Sheet, error) {
	if wb == nil || len(wb.Sheets) == 0 {
		return nil, fmt.Errorf("workbook contains no sheets")
	}
	if strings.TrimSpace(sheetName) == "" {
		return wb.GetActiveSheet(), nil
	}
	sh := wb.GetSheet(sheetName)
	if sh == nil {
		return nil, fmt.Errorf("sheet not found: '%s' (available: %s)", sheetName, strings.Join(wb.SheetNames(), ", "))
	}
	return sh, nil
}

// SaveWorkbookAuto saves a workbook to path, formatting according to target extension.
func SaveWorkbookAuto(wb *sheet.Workbook, targetSheetName, path, format string, recalc bool) error {
	if recalc {
		wb.RecalculateAll()
	}

	if format == "" {
		format = DetectFormat(path)
	}
	format = strings.ToLower(strings.TrimPrefix(format, "."))

	switch format {
	case "xlsx":
		return wb.ExportXLSX(path)
	case "ods":
		return wb.ExportODS(path)
	case "csv", "tsv":
		sh, err := GetTargetSheet(wb, targetSheetName)
		if err != nil {
			return err
		}
		return sh.ExportCSV(path)
	case "md", "markdown":
		sh, err := GetTargetSheet(wb, targetSheetName)
		if err != nil {
			return err
		}
		return sh.ExportMarkdown(path)
	case "hwkz", "hwk", "json", "":
		return wb.SaveJSON(path)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// SaveWorkbookAtomic saves a workbook safely using a sibling temporary file and atomic rename.
func SaveWorkbookAtomic(wb *sheet.Workbook, targetSheetName, path, format string, recalc bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(dir, ".hasucalc-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if format == "" {
		format = DetectFormat(path)
	}

	if err := SaveWorkbookAuto(wb, targetSheetName, tmpPath, format, recalc); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// SaveWorkbookToWriter serializes a workbook to an io.Writer (e.g. stdout).
func SaveWorkbookToWriter(wb *sheet.Workbook, targetSheetName string, w io.Writer, format string, recalc bool) error {
	if recalc {
		wb.RecalculateAll()
	}

	format = strings.ToLower(strings.TrimPrefix(format, "."))
	switch format {
	case "csv", "tsv":
		sh, err := GetTargetSheet(wb, targetSheetName)
		if err != nil {
			return err
		}
		minC, minR, maxC, maxR := sh.BoundingBox()
		if len(sh.GetPopulatedCoords()) == 0 {
			minC, minR, maxC, maxR = 0, 0, 0, 0
		}
		// Write UTF-8 BOM
		if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			return err
		}
		writer := csv.NewWriter(w)
		if format == "tsv" {
			writer.Comma = '\t'
		}
		for r := minR; r <= maxR; r++ {
			var rowVals []string
			for c := minC; c <= maxC; c++ {
				cellVal := sh.GetCell(c, r)
				if cellVal != nil && cellVal.Value != nil && cellVal.Type != cell.TypeEmpty {
					valStr := fmt.Sprintf("%v", cellVal.Value)
					rowVals = append(rowVals, valStr)
				} else {
					rowVals = append(rowVals, "")
				}
			}
			if err := writer.Write(rowVals); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()

	case "md", "markdown":
		sh, err := GetTargetSheet(wb, targetSheetName)
		if err != nil {
			return err
		}
		minC, minR, maxC, maxR := sh.BoundingBox()
		if len(sh.GetPopulatedCoords()) == 0 {
			minC, minR, maxC, maxR = 0, 0, 0, 0
		}
		md := sh.RenderMarkdownTableRange(minC, minR, maxC, maxR)
		_, err = io.WriteString(w, md)
		return err

	case "hwk", "json", "":
		// Serialize JSON to temp file and stream
		tmpFile, err := os.CreateTemp("", "hasucalc-out-*.hwk")
		if err != nil {
			return err
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)
		tmpFile.Close()

		if err := wb.SaveJSON(tmpPath); err != nil {
			return err
		}
		bytes, err := os.ReadFile(tmpPath)
		if err != nil {
			return err
		}
		_, err = w.Write(bytes)
		return err

	case "xlsx", "ods":
		// Binary multi-file format to writer via temp file
		tmpFile, err := os.CreateTemp("", "hasucalc-out-*."+format)
		if err != nil {
			return err
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)
		tmpFile.Close()

		if format == "xlsx" {
			if err := wb.ExportXLSX(tmpPath); err != nil {
				return err
			}
		} else {
			if err := wb.ExportODS(tmpPath); err != nil {
				return err
			}
		}
		bytes, err := os.ReadFile(tmpPath)
		if err != nil {
			return err
		}
		_, err = w.Write(bytes)
		return err

	default:
		return fmt.Errorf("unsupported output format for stream: %s", format)
	}
}
