package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Standard error codes for headless CLI
const (
	CodeFileNotFound    = "FILE_NOT_FOUND"
	CodeSheetNotFound   = "SHEET_NOT_FOUND"
	CodeSyntaxError     = "SYNTAX_ERROR"
	CodeInvalidArgument = "INVALID_ARGUMENT"
	CodeIOError         = "IO_ERROR"
	CodeFormatError     = "UNSUPPORTED_FORMAT"
)

// Response represents a standard JSON envelope for all headless CLI commands.
type Response struct {
	OK      bool   `json:"ok"`
	Command string `json:"command,omitempty"`
	Formula string `json:"formula,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Result  any    `json:"result,omitempty"`
	Cells   any    `json:"cells,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

// MetaInfo represents workbook-level and sheet-level metadata.
type MetaInfo struct {
	File        string      `json:"file,omitempty"`
	Format      string      `json:"format,omitempty"`
	Sheet       string      `json:"sheet,omitempty"`
	Sheets      any         `json:"sheets,omitempty"` // []string for get, or []SheetInfo for info
	ActiveSheet string      `json:"activeSheet,omitempty"`
	SheetCount  int         `json:"sheetCount,omitempty"`
	UsedRange   string      `json:"usedRange,omitempty"`
	QueryRange  string      `json:"queryRange,omitempty"`
	RecalcMode  string      `json:"recalcMode,omitempty"`
}

// SheetInfo represents sheet structural info within info meta.
type SheetInfo struct {
	Name       string `json:"name"`
	Index      int    `json:"index"`
	UsedRange  string `json:"usedRange"`
	MaxRow     int    `json:"maxRow"`
	MaxCol     int    `json:"maxCol"`
	CellCount  int    `json:"cellCount"`
	FrozenRows int    `json:"frozenRows"`
	FrozenCols int    `json:"frozenCols"`
	HasGraph   bool   `json:"hasGraph"`
	GraphType  string `json:"graphType"`
}

// CellItem represents a sparse cell output item in get command.
type CellItem struct {
	Ref string `json:"ref"`
	R   int    `json:"r"`
	C   int    `json:"c"`
	Typ string `json:"type"`
	Raw string `json:"raw"`
	Val any    `json:"val"`
	Fmt string `json:"fmt,omitempty"`
}

// EvalResult represents the result payload of an eval command.
type EvalResult struct {
	Val   any    `json:"val"`
	Type  string `json:"type"`
	Error any    `json:"error"`
}

// WriteJSON outputs an indented JSON response to the provided writer.
func WriteJSON(w io.Writer, resp any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(resp)
}

// EmitError outputs the error to stderr and optionally structured JSON to stdout, returning exit code 1.
func EmitError(cmd, errMsg, errCode string, isJSON bool) int {
	fmt.Fprintf(os.Stderr, "Error: %s\n", errMsg)
	if isJSON {
		resp := Response{
			OK:      false,
			Command: cmd,
			Error:   errMsg,
			Code:    errCode,
		}
		_ = WriteJSON(os.Stdout, resp)
	}
	return 1
}
