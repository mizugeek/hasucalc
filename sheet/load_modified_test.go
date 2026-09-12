package sheet

import (
	"path/filepath"
	"testing"
)

func TestLoadWorkbookJSONClearsModified(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "named.hwk")

	wb := NewWorkbook("named")
	wb.Sheets[0].SetName("Sheet1")
	wb.AddSheet("Summary")
	wb.Sheets[0].SetCellInput(0, 0, "Hello", nil)
	wb.Sheets[0].SetCellInput(1, 0, "=A1", nil)
	if err := wb.SaveJSON(path); err != nil {
		t.Fatalf("SaveJSON: %v", err)
	}

	loaded, err := LoadWorkbookJSON(path)
	if err != nil {
		t.Fatalf("LoadWorkbookJSON: %v", err)
	}
	if loaded.IsModified() {
		t.Fatalf("loaded workbook should not be modified")
	}
	for i, s := range loaded.Sheets {
		if s.IsModified() {
			t.Errorf("sheet[%d] %q should not be modified after load", i, s.Name())
		}
	}
}
