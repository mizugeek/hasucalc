package sheet

// ExportXLSXWorkbook writes a Workbook to a standard XLSX (zip) file.
// Delegates to wb.ExportXLSX to maintain a single unified, compliant OpenXML export implementation.
func ExportXLSXWorkbook(wb *Workbook, filename string) error {
	return wb.ExportXLSX(filename)
}
