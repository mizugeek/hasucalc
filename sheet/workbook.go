package sheet

import (
	"encoding/json"
	"fmt"
	"strings"

	"hasucalc/cell"
	"hasucalc/formula"
)

// Workbook represents a collection of named worksheets.
type Workbook struct {
	Name             string         `json:"name"`
	Sheets           []*Sheet       `json:"sheets"`
	ActiveSheetIndex int            `json:"active_sheet_index"`
	NamedRanges      map[string]any `json:"named_ranges,omitempty"`
}

// NewWorkbook creates a new empty workbook with one default sheet.
func NewWorkbook(name string) *Workbook {
	if name == "" {
		name = "Workbook"
	}
	wb := &Workbook{
		Name:             name,
		Sheets:           make([]*Sheet, 0),
		ActiveSheetIndex: 0,
		NamedRanges:      make(map[string]any),
	}
	wb.AddSheet("Sheet1")
	return wb
}

// AddSheet creates and appends a new sheet with the given name.
func (wb *Workbook) AddSheet(name string) *Sheet {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("Sheet%d", len(wb.Sheets)+1)
	}
	// Ensure unique sheet name
	uniqueName := name
	counter := 1
	for wb.GetSheet(uniqueName) != nil {
		counter++
		uniqueName = fmt.Sprintf("%s (%d)", name, counter)
	}

	sh := NewSheet()
	sh.SetName(uniqueName)
	sh.SetWorkbook(wb)
	wb.Sheets = append(wb.Sheets, sh)
	return sh
}

// GetSheet retrieves a sheet by case-insensitive name.
func (wb *Workbook) GetSheet(name string) *Sheet {
	trimmed := strings.Trim(strings.TrimSpace(name), "'")
	for _, s := range wb.Sheets {
		if strings.EqualFold(s.Name(), trimmed) {
			return s
		}
	}
	return nil
}

// GetSheetIndex returns the index of the given sheet, or -1 if not found.
func (wb *Workbook) GetSheetIndex(sh *Sheet) int {
	for i, s := range wb.Sheets {
		if s == sh {
			return i
		}
	}
	return -1
}

// GetActiveSheet returns the currently active worksheet.
func (wb *Workbook) GetActiveSheet() *Sheet {
	if len(wb.Sheets) == 0 {
		return wb.AddSheet("Sheet1")
	}
	if wb.ActiveSheetIndex < 0 || wb.ActiveSheetIndex >= len(wb.Sheets) {
		wb.ActiveSheetIndex = 0
	}
	return wb.Sheets[wb.ActiveSheetIndex]
}

// SetActiveSheet sets the active sheet index.
func (wb *Workbook) SetActiveSheet(index int) error {
	if index < 0 || index >= len(wb.Sheets) {
		return fmt.Errorf("sheet index out of range: %d", index)
	}
	wb.ActiveSheetIndex = index
	return nil
}

// SetActiveSheetByName sets the active sheet by case-insensitive name.
func (wb *Workbook) SetActiveSheetByName(name string) error {
	trimmed := strings.Trim(strings.TrimSpace(name), "'")
	for i, s := range wb.Sheets {
		if strings.EqualFold(s.Name(), trimmed) {
			wb.ActiveSheetIndex = i
			return nil
		}
	}
	return fmt.Errorf("sheet not found: %s", name)
}

// RenameSheet renames an existing sheet and updates cross-sheet formula references.
func (wb *Workbook) RenameSheet(oldName, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("sheet name cannot be empty")
	}
	sh := wb.GetSheet(oldName)
	if sh == nil {
		return fmt.Errorf("sheet not found: %s", oldName)
	}
	if existing := wb.GetSheet(newName); existing != nil && existing != sh {
		return fmt.Errorf("sheet with name '%s' already exists", newName)
	}
	realOldName := sh.Name()
	sh.SetName(newName)

	// Update all formula references across all sheets in workbook
	for _, sheetItem := range wb.Sheets {
		for pt, c := range sheetItem.cells {
			if c != nil && c.Type == cell.TypeFormula {
				adj := formula.RenameSheetReferences(c.RawInput, realOldName, newName)
				if adj != c.RawInput {
					fmtCopy := c.FormatSpec
					sheetItem.cells[pt] = cell.NewCell(adj, fmtCopy)
				}
			}
		}
	}
	wb.rewriteNamesForSheetRename(realOldName, newName)
	wb.RecalculateAll()
	return nil
}

// DeleteSheet deletes a sheet by index and invalidates cross-sheet formula references.
func (wb *Workbook) DeleteSheet(index int) error {
	if len(wb.Sheets) <= 1 {
		return fmt.Errorf("cannot delete the only sheet in workbook")
	}
	if index < 0 || index >= len(wb.Sheets) {
		return fmt.Errorf("sheet index out of range: %d", index)
	}
	deletedName := wb.Sheets[index].Name()
	wb.Sheets = append(wb.Sheets[:index], wb.Sheets[index+1:]...)
	if index < wb.ActiveSheetIndex {
		wb.ActiveSheetIndex--
	} else if wb.ActiveSheetIndex >= len(wb.Sheets) {
		wb.ActiveSheetIndex = len(wb.Sheets) - 1
	}

	// Invalidate references to deleted sheet
	for _, sheetItem := range wb.Sheets {
		for pt, c := range sheetItem.cells {
			if c != nil && c.Type == cell.TypeFormula {
				adj := formula.InvalidateSheetReferences(c.RawInput, deletedName)
				if adj != c.RawInput {
					fmtCopy := c.FormatSpec
					sheetItem.cells[pt] = cell.NewCell(adj, fmtCopy)
				}
			}
		}
	}
	wb.rewriteNamesForSheetDelete(deletedName)
	wb.RecalculateAll()
	return nil
}

// SheetNames returns the names of all sheets in order.
func (wb *Workbook) SheetNames() []string {
	var names []string
	for _, s := range wb.Sheets {
		names = append(names, s.Name())
	}
	return names
}

func (wb *Workbook) GetNamedRange(name string) (any, bool) {
	if wb.NamedRanges == nil {
		return nil, false
	}
	v, ok := wb.NamedRanges[strings.ToUpper(name)]
	return v, ok
}

func (wb *Workbook) SetNamedRange(name string, val any) {
	if wb.NamedRanges == nil {
		wb.NamedRanges = make(map[string]any)
	}
	wb.NamedRanges[strings.ToUpper(name)] = val
	if sh := wb.GetActiveSheet(); sh != nil {
		sh.SetModified(true)
	}
}

// IsModified reports whether any sheet in the workbook has unsaved changes.
func (wb *Workbook) IsModified() bool {
	for _, s := range wb.Sheets {
		if s != nil && s.IsModified() {
			return true
		}
	}
	return false
}

// RecalculateAll triggers recalculation across all sheets in the workbook.
// Multiple passes are needed so cross-sheet dependency chains (C→B→A) converge.
func (wb *Workbook) RecalculateAll() {
	n := len(wb.Sheets) + 1
	if n < 2 {
		n = 2
	}
	if n > 8 {
		n = 8
	}
	for pass := 0; pass < n; pass++ {
		for _, s := range wb.Sheets {
			s.Recalculate()
		}
	}
}

type workbookJSONData struct {
	Version          string          `json:"version"`
	Name             string          `json:"name"`
	ActiveSheetIndex int             `json:"active_sheet_index"`
	NamedRanges      map[string]any  `json:"named_ranges,omitempty"`
	Sheets           []sheetJSONData `json:"sheets"`
}

func (wb *Workbook) SaveJSON(filepath string) error {
	data := workbookJSONData{
		Version:          "HasuCalc/2.0",
		Name:             wb.Name,
		ActiveSheetIndex: wb.ActiveSheetIndex,
		Sheets:           make([]sheetJSONData, 0, len(wb.Sheets)),
	}
	if len(wb.NamedRanges) > 0 {
		data.NamedRanges = encodeNamedMap(wb.NamedRanges)
	}
	for _, s := range wb.Sheets {
		sData := s.buildSheetJSONData()
		sData.Name = s.Name()
		data.Sheets = append(data.Sheets, sData)
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	err = WriteMaybeGzipFile(filepath, bytes)
	if err == nil {
		for _, s := range wb.Sheets {
			s.SetModified(false)
		}
	}
	return err
}

func LoadWorkbookJSON(filepath string) (*Workbook, error) {
	bytes, err := ReadMaybeGzipFile(filepath)
	if err != nil {
		return nil, err
	}

	var wbData workbookJSONData
	if err := json.Unmarshal(bytes, &wbData); err == nil && len(wbData.Sheets) > 0 {
		wb := &Workbook{
			Name:             wbData.Name,
			Sheets:           make([]*Sheet, 0, len(wbData.Sheets)),
			ActiveSheetIndex: wbData.ActiveSheetIndex,
			NamedRanges:      make(map[string]any),
		}
		if decoded := decodeNamedMap(wbData.NamedRanges); len(decoded) > 0 {
			wb.NamedRanges = decoded
		}
		for _, sData := range wbData.Sheets {
			sh := loadSheetFromData(sData)
			sh.SetWorkbook(wb)
			wb.Sheets = append(wb.Sheets, sh)
		}
		if wb.ActiveSheetIndex < 0 || wb.ActiveSheetIndex >= len(wb.Sheets) {
			wb.ActiveSheetIndex = 0
		}

		wb.RecalculateAll()
		// Loading/recalculating must not look like an unsaved user edit.
		for _, s := range wb.Sheets {
			if s != nil {
				s.SetModified(false)
			}
		}
		return wb, nil
	}

	// Fallback to single sheet legacy JSON
	sh, err := LoadSheetJSON(filepath)
	if err != nil {
		return nil, err
	}
	wb := NewWorkbook(filepath)
	wb.Sheets[0] = sh
	sh.SetWorkbook(wb)
	return wb, nil
}
