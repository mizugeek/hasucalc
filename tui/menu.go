package tui

type MenuActionType int

const (
	ActionSubmenu MenuActionType = iota
	ActionPrompt
	ActionExecute
)

type PromptItem struct {
	Key       string
	Prompt    string
	Default   string
	UseCursor bool
}

type MenuItem struct {
	Name          string
	Key           string
	Description   string
	Shortcut      string
	ActionType    MenuActionType
	Children      []*MenuItem
	ActionHandler string
	PromptChain   []PromptItem
}

func exportFormatChildren(fullHandler, rangeHandler string) []*MenuItem {
	return []*MenuItem{
		{
			Name:          "Sheet",
			Key:           "S",
			Description:   "Export entire active sheet / workbook",
			ActionType:    ActionExecute,
			ActionHandler: fullHandler,
		},
		{
			Name:          "Range",
			Key:           "R",
			Description:   "Export specified range only (values)",
			ActionType:    ActionExecute,
			ActionHandler: rangeHandler,
		},
	}
}

func BuildMenuTree() *MenuItem {
	menuFile := &MenuItem{
		Name:        "File",
		Key:         "F",
		Description: "New, Open, Save, Export, Quit",
		Children: []*MenuItem{
			{
				Name:          "New",
				Key:           "N",
				Description:   "Create a new blank worksheet",
				ActionType:    ActionExecute,
				ActionHandler: "doWorksheetErase",
			},
			{
				Name:          "Open",
				Key:           "O",
				Description:   "Open file (.hwk, .xlsx, .ods, .csv, .md, .html, .json)",
				Shortcut:      "Ctrl+O",
				ActionType:    ActionExecute,
				ActionHandler: "doFileOpenDialog",
			},
			{
				Name:          "Save",
				Key:           "S",
				Description:   "Save current workbook to disk",
				Shortcut:      "Ctrl+S",
				ActionType:    ActionExecute,
				ActionHandler: "doFileSaveDialog",
			},
			{
				Name:        "Export",
				Key:         "X",
				Description: "Export to CSV, Excel, ODS, or Markdown",
				Children: []*MenuItem{
					{
						Name:        "CSV",
						Key:         "C",
						Description: "Export as CSV (UTF-8 BOM)",
						Children:    exportFormatChildren("doFileExportCSVFullDialog", "doFileExportCSVRangeDialog"),
					},
					{
						Name:        "Excel",
						Key:         "E",
						Description: "Export as Excel (.xlsx)",
						Children:    exportFormatChildren("doFileExportXLSXFullDialog", "doFileExportXLSXRangeDialog"),
					},
					{
						Name:        "OpenDocument",
						Key:         "O",
						Description: "Export as OpenDocument (.ods)",
						Children:    exportFormatChildren("doFileExportODSFullDialog", "doFileExportODSRangeDialog"),
					},
					{
						Name:        "Markdown",
						Key:         "M",
						Description: "Export as Markdown table (.md)",
						Children:    exportFormatChildren("doFileExportMarkdownDialog", "doFileExportMarkdownRangeDialog"),
					},
				},
			},
			{
				Name:          "Quit",
				Key:           "Q",
				Description:   "Exit application",
				Shortcut:      "Ctrl+Q",
				ActionType:    ActionExecute,
				ActionHandler: "doQuitApp",
			},
		},
	}

	menuEdit := &MenuItem{
		Name:        "Edit",
		Key:         "E",
		Description: "Clipboard, Undo, Find, Goto",
		Children: []*MenuItem{
			{
				Name:        "History",
				Key:         "H",
				Description: "Undo and Redo",
				Children: []*MenuItem{
					{
						Name:          "Undo",
						Key:           "U",
						Description:   "Undo last action",
						Shortcut:      "Ctrl+Z",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteUndo",
					},
					{
						Name:          "Redo",
						Key:           "R",
						Description:   "Redo last undone action",
						Shortcut:      "Ctrl+Y",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteRedo",
					},
				},
			},
			{
				Name:        "Clipboard",
				Key:         "C",
				Description: "Cut, Copy, Paste",
				Children: []*MenuItem{
					{
						Name:          "Cut",
						Key:           "X",
						Description:   "Cut selected cells to clipboard",
						Shortcut:      "Ctrl+X",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteCut",
					},
					{
						Name:          "Copy",
						Key:           "C",
						Description:   "Copy selected cells to clipboard",
						Shortcut:      "Ctrl+C",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteCopy",
					},
					{
						Name:        "Paste",
						Key:         "V",
						Description: "Paste clipboard at cursor",
						Children: []*MenuItem{
							{
								Name:          "All",
								Key:           "A",
								Description:   "Paste clipboard at cursor",
								Shortcut:      "Ctrl+V",
								ActionType:    ActionExecute,
								ActionHandler: "doPalettePaste",
							},
							{
								Name:          "Values",
								Key:           "V",
								Description:   "Paste evaluated values only",
								ActionType:    ActionExecute,
								ActionHandler: "doPalettePasteValues",
							},
							{
								Name:          "Link",
								Key:           "L",
								Description:   "Paste link formulas to source cells",
								Shortcut:      "Ctrl+L",
								ActionType:    ActionExecute,
								ActionHandler: "doPalettePasteLink",
							},
							{
								Name:          "Transpose",
								Key:           "T",
								Description:   "Paste transposed (rows/columns flipped)",
								ActionType:    ActionExecute,
								ActionHandler: "doPasteTranspose",
							},
						},
					},
				},
			},
			{
				Name:          "Clear",
				Key:           "K",
				Description:   "Clear selected cells",
				Shortcut:      "Del",
				ActionType:    ActionExecute,
				ActionHandler: "doPaletteClear",
			},
			{
				Name:        "Find",
				Key:         "F",
				Description: "Find, replace, and search all sheets",
				Children: []*MenuItem{
					{
						Name:          "Find",
						Key:           "F",
						Description:   "Find text, number, or formula",
						Shortcut:      "Ctrl+F",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "query", Prompt: "Find text/number/formula: "}},
						ActionHandler: "doFindPrompt",
					},
					{
						Name:          "Next",
						Key:           "N",
						Description:   "Find next match",
						Shortcut:      "F3",
						ActionType:    ActionExecute,
						ActionHandler: "doFindNext",
					},
					{
						Name:          "Prev",
						Key:           "P",
						Description:   "Find previous match",
						Shortcut:      "Shift+F3",
						ActionType:    ActionExecute,
						ActionHandler: "doFindPrev",
					},
					{
						Name:        "Replace",
						Key:         "E",
						Description: "Find and replace",
						Shortcut:    "Ctrl+H",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "find", Prompt: "Find text/number/formula: "},
							{Key: "replace", Prompt: "Replace with: "},
							{Key: "scope", Prompt: "Scope [C: Current Sheet / A: All Sheets]: ", Default: "C"},
						},
						ActionHandler: "doReplacePrompt",
					},
					{
						Name:          "All",
						Key:           "A",
						Description:   "Search across all worksheets",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "query", Prompt: "Search all sheets for: "}},
						ActionHandler: "doFindAllPrompt",
					},
				},
			},
			{
				Name:          "Goto",
				Key:           "G",
				Description:   "Jump to cell, range, sheet, or name",
				Shortcut:      "F5",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "cell", Prompt: "Enter address or name to go to (e.g. B10, Sheet2!A1, Total): "}},
				ActionHandler: "doGotoCell",
			},
		},
	}

	menuFormat := &MenuItem{
		Name:        "Format",
		Key:         "M",
		Description: "Number formats and cell alignment",
		Children: []*MenuItem{
			{
				Name:        "Number",
				Key:         "N",
				Description: "Number formats (Currency, Percent, Date, ...)",
				Children: []*MenuItem{
					{
						Name:        "Currency",
						Key:         "C",
						Description: "Currency format (symbol + amount; cells may differ)",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "dec", Prompt: "Decimal places (0..15): ", Default: "2"},
							{Key: "symbol", Prompt: "Currency symbol (e.g. $, ¥, €, £): ", Default: "$"},
							{Key: "range", Prompt: "Range to format: ", UseCursor: true},
						},
						ActionHandler: "doRangeFormatCurrency",
					},
					{
						Name:        "Percent",
						Key:         "P",
						Description: "Percent format (12.3%)",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "dec", Prompt: "Decimal places (0..15): ", Default: "2"},
							{Key: "range", Prompt: "Range to format: ", UseCursor: true},
						},
						ActionHandler: "doRangeFormatPercent",
					},
					{
						Name:        "Fixed",
						Key:         "F",
						Description: "Fixed decimal format",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "dec", Prompt: "Decimal places (0..15): ", Default: "2"},
							{Key: "range", Prompt: "Range to format: ", UseCursor: true},
						},
						ActionHandler: "doRangeFormatFixed",
					},
					{
						Name:        "Comma",
						Key:         ",",
						Description: "Thousands separator format",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "dec", Prompt: "Decimal places (0..15): ", Default: "2"},
							{Key: "range", Prompt: "Range to format: ", UseCursor: true},
						},
						ActionHandler: "doRangeFormatComma",
					},
					{
						Name:        "Date",
						Key:         "D",
						Description: "Date format (1..5)",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "type", Prompt: "Date format (1..5): ", Default: "1"},
							{Key: "range", Prompt: "Range to format: ", UseCursor: true},
						},
						ActionHandler: "doRangeFormatDate",
					},
					{
						Name:        "Scientific",
						Key:         "S",
						Description: "Scientific / exponential format (E)",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "dec", Prompt: "Decimal places (0..15): ", Default: "2"},
							{Key: "range", Prompt: "Range to format: ", UseCursor: true},
						},
						ActionHandler: "doRangeFormatScientific",
					},
					{
						Name:          "General",
						Key:           "G",
						Description:   "Reset to general format",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Range to format: ", UseCursor: true}},
						ActionHandler: "doRangeFormatGeneral",
					},
				},
			},
			{
				Name:        "Align",
				Key:         "A",
				Description: "Cell text alignment",
				Children: []*MenuItem{
					{
						Name:          "Left",
						Key:           "L",
						Description:   "Align left (')",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Range to align left: ", UseCursor: true}},
						ActionHandler: "doRangeLabelLeft",
					},
					{
						Name:          "Right",
						Key:           "R",
						Description:   "Align right (\")",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Range to align right: ", UseCursor: true}},
						ActionHandler: "doRangeLabelRight",
					},
					{
						Name:          "Center",
						Key:           "C",
						Description:   "Align center (^)",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Range to center: ", UseCursor: true}},
						ActionHandler: "doRangeLabelCenter",
					},
				},
			},
		},
	}

	menuRow := &MenuItem{
		Name:        "Row",
		Key:         "R",
		Description: "Insert or delete rows",
		Children: []*MenuItem{
			{
				Name:          "Insert",
				Key:           "I",
				Description:   "Insert row(s)",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "range", Prompt: "Insert row range: ", UseCursor: true}},
				ActionHandler: "doInsertRow",
			},
			{
				Name:          "Delete",
				Key:           "D",
				Description:   "Delete row(s)",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "range", Prompt: "Delete row range: ", UseCursor: true}},
				ActionHandler: "doDeleteRow",
			},
		},
	}

	menuColumn := &MenuItem{
		Name:        "Column",
		Key:         "L",
		Description: "Insert/delete columns and set width",
		Children: []*MenuItem{
			{
				Name:          "Insert",
				Key:           "I",
				Description:   "Insert column(s)",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "range", Prompt: "Insert column range: ", UseCursor: true}},
				ActionHandler: "doInsertCol",
			},
			{
				Name:          "Delete",
				Key:           "D",
				Description:   "Delete column(s)",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "range", Prompt: "Delete column range: ", UseCursor: true}},
				ActionHandler: "doDeleteCol",
			},
			{
				Name:        "Width",
				Key:         "W",
				Description: "Set, reset, or global column width",
				Children: []*MenuItem{
					{
						Name:          "Set",
						Key:           "S",
						Description:   "Set current column width",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "width", Prompt: "Enter column width (1..72): ", Default: "9"}},
						ActionHandler: "doColumnSetWidth",
					},
					{
						Name:          "Reset",
						Key:           "R",
						Description:   "Reset column width to default",
						ActionType:    ActionExecute,
						ActionHandler: "doResetColWidth",
					},
					{
						Name:          "Global",
						Key:           "G",
						Description:   "Set default width for all columns",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "width", Prompt: "Enter global column width (1..72): ", Default: "9"}},
						ActionHandler: "doGlobalColWidth",
					},
				},
			},
		},
	}

	menuSheet := &MenuItem{
		Name:        "Sheet",
		Key:         "W",
		Description: "Worksheet tabs, freeze panes",
		Children: []*MenuItem{
			{
				Name:          "Add",
				Key:           "A",
				Description:   "Add a new worksheet",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "name", Prompt: "Enter new worksheet name: "}},
				ActionHandler: "doWorksheetAdd",
			},
			{
				Name:          "Delete",
				Key:           "D",
				Description:   "Delete active worksheet",
				ActionType:    ActionExecute,
				ActionHandler: "doWorksheetDelete",
			},
			{
				Name:          "Rename",
				Key:           "R",
				Description:   "Rename active worksheet",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "name", Prompt: "Enter new worksheet name: "}},
				ActionHandler: "doWorksheetRename",
			},
			{
				Name:        "Go",
				Key:         "G",
				Description: "Select or switch worksheets",
				Children: []*MenuItem{
					{
						Name:          "Select",
						Key:           "S",
						Description:   "Select worksheet from list",
						Shortcut:      "Ctrl+T",
						ActionType:    ActionExecute,
						ActionHandler: "doSheetSwitchModal",
					},
					{
						Name:          "Next",
						Key:           "N",
						Description:   "Activate next worksheet",
						Shortcut:      "Ctrl+PgDn",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteSheetNext",
					},
					{
						Name:          "Prev",
						Key:           "P",
						Description:   "Activate previous worksheet",
						Shortcut:      "Ctrl+PgUp",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteSheetPrev",
					},
				},
			},
			{
				Name:        "Freeze",
				Key:         "F",
				Description: "Freeze rows and/or columns",
				Children: []*MenuItem{
					{
						Name:          "Both",
						Key:           "B",
						Description:   "Freeze rows above and columns left of cursor",
						ActionType:    ActionExecute,
						ActionHandler: "doFreezeBoth",
					},
					{
						Name:          "Horizontal",
						Key:           "H",
						Description:   "Freeze rows above cursor only",
						ActionType:    ActionExecute,
						ActionHandler: "doFreezeHorizontal",
					},
					{
						Name:          "Vertical",
						Key:           "V",
						Description:   "Freeze columns left of cursor only",
						ActionType:    ActionExecute,
						ActionHandler: "doFreezeVertical",
					},
					{
						Name:          "Clear",
						Key:           "C",
						Description:   "Clear all freeze panes",
						ActionType:    ActionExecute,
						ActionHandler: "doFreezeClear",
					},
				},
			},
		},
	}

	menuData := &MenuItem{
		Name:        "Data",
		Key:         "D",
		Description: "Sort, Fill, Transpose, Names",
		Children: []*MenuItem{
			{
				Name:        "Sort",
				Key:         "S",
				Description: "Sort range ascending / descending",
				Children: []*MenuItem{
					{
						Name:        "Ascending",
						Key:         "A",
						Description: "Sort ascending (A-Z, 0-9)",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "range", Prompt: "Enter range to sort: ", UseCursor: true},
							{Key: "col", Prompt: "Sort key column (A-Z) or cell: "},
						},
						ActionHandler: "doDataSortAscPrompt",
					},
					{
						Name:        "Descending",
						Key:         "D",
						Description: "Sort descending (Z-A, 9-0)",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "range", Prompt: "Enter range to sort: ", UseCursor: true},
							{Key: "col", Prompt: "Sort key column (A-Z) or cell: "},
						},
						ActionHandler: "doDataSortDescPrompt",
					},
					{
						Name:        "Horiz-Ascending",
						Key:         "H",
						Description: "Sort columns left-to-right ascending",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "range", Prompt: "Enter range to sort horizontally: ", UseCursor: true},
							{Key: "row", Prompt: "Sort key row number (e.g. 1, 2) or cell: "},
						},
						ActionHandler: "doDataSortHorizAscPrompt",
					},
					{
						Name:        "Horiz-Descending",
						Key:         "Z",
						Description: "Sort columns left-to-right descending",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "range", Prompt: "Enter range to sort horizontally: ", UseCursor: true},
							{Key: "row", Prompt: "Sort key row number (e.g. 1, 2) or cell: "},
						},
						ActionHandler: "doDataSortHorizDescPrompt",
					},
					{
						Name:          "Reset",
						Key:           "R",
						Description:   "Reset sort data range and settings",
						ActionType:    ActionExecute,
						ActionHandler: "doDataSortReset",
					},
				},
			},
			{
				Name:        "Fill",
				Key:         "F",
				Description: "AutoFill, series, fill down/right",
				Children: []*MenuItem{
					{
						Name:          "Auto",
						Key:           "A",
						Description:   "AutoFill sequential series across selection",
						ActionType:    ActionExecute,
						ActionHandler: "doAutoFill",
					},
					{
						Name:        "Series",
						Key:         "S",
						Description: "Fill range with start/step/stop series",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "range", Prompt: "Fill range: ", UseCursor: true},
							{Key: "start", Prompt: "Start value (number or YYYY-MM-DD): ", Default: "1"},
							{Key: "step", Prompt: "Step increment (e.g. 1, 5, 1d, 1m): ", Default: "1"},
							{Key: "stop", Prompt: "Stop value (optional): ", Default: ""},
						},
						ActionHandler: "doDataFill",
					},
					{
						Name:          "Down",
						Key:           "D",
						Description:   "Fill selection downward from top row",
						Shortcut:      "Ctrl+D",
						ActionType:    ActionExecute,
						ActionHandler: "doFillDown",
					},
					{
						Name:          "Right",
						Key:           "R",
						Description:   "Fill selection rightward from left column",
						Shortcut:      "Ctrl+R",
						ActionType:    ActionExecute,
						ActionHandler: "doFillRight",
					},
				},
			},
			{
				Name:        "Transpose",
				Key:         "T",
				Description: "Transpose source range into target cell",
				ActionType:  ActionPrompt,
				PromptChain: []PromptItem{
					{Key: "source", Prompt: "Source range to transpose: ", UseCursor: true},
					{Key: "target", Prompt: "Target top-left cell: ", Default: "A1"},
				},
				ActionHandler: "doRangeTranspose",
			},
			{
				Name:        "Names",
				Key:         "N",
				Description: "Named ranges",
				Children: []*MenuItem{
					{
						Name:        "Create",
						Key:         "C",
						Description: "Create a named range",
						ActionType:  ActionPrompt,
						PromptChain: []PromptItem{
							{Key: "name", Prompt: "Range name: "},
							{Key: "range", Prompt: "Range: ", UseCursor: true},
						},
						ActionHandler: "doRangeNameCreate",
					},
					{
						Name:          "Delete",
						Key:           "D",
						Description:   "Delete a range name",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "name", Prompt: "Range name to delete: "}},
						ActionHandler: "doRangeNameDelete",
					},
					{
						Name:          "List",
						Key:           "L",
						Description:   "List defined range names",
						ActionType:    ActionExecute,
						ActionHandler: "doRangeNameList",
					},
				},
			},
		},
	}

	menuFormula := &MenuItem{
		Name:        "Formula",
		Key:         "O",
		Description: "Aggregates, functions, dates, recalculate",
		Children: []*MenuItem{
			{
				Name:        "Aggregate",
				Key:         "A",
				Description: "Insert SUM/AVERAGE/COUNT/MAX/MIN for nearby numbers",
				Children: []*MenuItem{
					{
						Name:          "Sum",
						Key:           "S",
						Description:   "Insert =SUM() for above/left numbers",
						Shortcut:      "Alt+=",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteAutoSum",
					},
					{
						Name:          "Average",
						Key:           "A",
						Description:   "Insert =AVERAGE()",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteAverage",
					},
					{
						Name:          "Count",
						Key:           "C",
						Description:   "Insert =COUNT()",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteCount",
					},
					{
						Name:          "Max",
						Key:           "M",
						Description:   "Insert =MAX()",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteMax",
					},
					{
						Name:          "Min",
						Key:           "I",
						Description:   "Insert =MIN()",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteMin",
					},
				},
			},
			{
				Name:          "Function",
				Key:           "F",
				Description:   "Browse and insert a worksheet function",
				ActionType:    ActionExecute,
				ActionHandler: "doOpenFunctionPicker",
			},
			{
				Name:        "Date",
				Key:         "D",
				Description: "Insert TODAY or NOW",
				Children: []*MenuItem{
					{
						Name:          "Today",
						Key:           "T",
						Description:   "Insert =TODAY()",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteToday",
					},
					{
						Name:          "Now",
						Key:           "N",
						Description:   "Insert =NOW()",
						ActionType:    ActionExecute,
						ActionHandler: "doPaletteNow",
					},
				},
			},
			{
				Name:          "Recalculate",
				Key:           "9",
				Description:   "Recalculate all formulas in workbook",
				Shortcut:      "F9",
				ActionType:    ActionExecute,
				ActionHandler: "doPaletteRecalc",
			},
		},
	}

	menuChart := &MenuItem{
		Name:        "Chart",
		Key:         "C",
		Description: "Chart view, type, series, PNG export",
		Children: []*MenuItem{
			{
				Name:          "View",
				Key:           "V",
				Description:   "Full-screen chart view",
				Shortcut:      "F10",
				ActionType:    ActionExecute,
				ActionHandler: "doGraphView",
			},
			{
				Name:        "Type",
				Key:         "T",
				Description: "Chart type",
				Children: []*MenuItem{
					{
						Name:          "Line",
						Key:           "L",
						Description:   "Line chart",
						ActionType:    ActionExecute,
						ActionHandler: "doGraphTypeLine",
					},
					{
						Name:          "Bar",
						Key:           "B",
						Description:   "Clustered bar chart",
						ActionType:    ActionExecute,
						ActionHandler: "doGraphTypeBar",
					},
					{
						Name:          "Stacked-Bar",
						Key:           "S",
						Description:   "Stacked bar chart",
						ActionType:    ActionExecute,
						ActionHandler: "doGraphTypeStacked",
					},
					{
						Name:          "Pie",
						Key:           "P",
						Description:   "Pie chart",
						ActionType:    ActionExecute,
						ActionHandler: "doGraphTypePie",
					},
				},
			},
			{
				Name:          "Title",
				Key:           "I",
				Description:   "Set chart title",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "title", Prompt: "Enter graph title (or leave blank to clear): "}},
				ActionHandler: "doGraphSetTitle",
			},
			{
				Name:          "X-Axis",
				Key:           "X",
				Description:   "Set X-axis label range",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "range", Prompt: "Enter X axis range: ", UseCursor: true}},
				ActionHandler: "doGraphSetX",
			},
			{
				Name:        "Series",
				Key:         "E",
				Description: "Set series A–F data ranges",
				Children: []*MenuItem{
					{
						Name:          "A",
						Key:           "A",
						Description:   "Set series A data range",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Enter series A range: ", UseCursor: true}},
						ActionHandler: "doGraphSetA",
					},
					{
						Name:          "B",
						Key:           "B",
						Description:   "Set series B data range",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Enter series B range: ", UseCursor: true}},
						ActionHandler: "doGraphSetB",
					},
					{
						Name:          "C",
						Key:           "C",
						Description:   "Set series C data range",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Enter series C range: ", UseCursor: true}},
						ActionHandler: "doGraphSetC",
					},
					{
						Name:          "D",
						Key:           "D",
						Description:   "Set series D data range",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Enter series D range: ", UseCursor: true}},
						ActionHandler: "doGraphSetD",
					},
					{
						Name:          "E",
						Key:           "E",
						Description:   "Set series E data range",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Enter series E range: ", UseCursor: true}},
						ActionHandler: "doGraphSetE",
					},
					{
						Name:          "F",
						Key:           "F",
						Description:   "Set series F data range",
						ActionType:    ActionPrompt,
						PromptChain:   []PromptItem{{Key: "range", Prompt: "Enter series F range: ", UseCursor: true}},
						ActionHandler: "doGraphSetF",
					},
				},
			},
			{
				Name:          "Status",
				Key:           "S",
				Description:   "Show chart settings",
				ActionType:    ActionExecute,
				ActionHandler: "doGraphStatus",
			},
			{
				Name:          "Save-PNG",
				Key:           "P",
				Description:   "Save chart as 1280x720 PNG",
				ActionType:    ActionPrompt,
				PromptChain:   []PromptItem{{Key: "filename", Prompt: "Save PNG as (leave blank for automatic name): ", Default: ""}},
				ActionHandler: "doGraphSavePNG",
			},
		},
	}

	menuHelp := &MenuItem{
		Name:        "Help",
		Key:         "?",
		Description: "About, keybindings, command palette",
		Children: []*MenuItem{
			{
				Name:          "About",
				Key:           "A",
				Description:   "About HasuCalc, version, license",
				ActionType:    ActionExecute,
				ActionHandler: "doOpenAbout",
			},
			{
				Name:          "Keybindings",
				Key:           "K",
				Description:   "Show keybindings and help",
				Shortcut:      "F1",
				ActionType:    ActionExecute,
				ActionHandler: "doPaletteHelp",
			},
			{
				Name:          "Palette",
				Key:           "P",
				Description:   "Open command palette",
				Shortcut:      "Ctrl+K",
				ActionType:    ActionExecute,
				ActionHandler: "doOpenPalette",
			},
		},
	}

	return &MenuItem{
		Name: "ROOT",
		Children: []*MenuItem{
			menuFile,
			menuEdit,
			menuFormat,
			menuRow,
			menuColumn,
			menuSheet,
			menuData,
			menuFormula,
			menuChart,
			menuHelp,
		},
	}
}
