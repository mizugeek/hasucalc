package tui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"hasucalc/coord"
	"hasucalc/version"
)

type HelpPage struct {
	Title string
	Items [][2]string
}

var HelpPages = []HelpPage{
	{
		Title: "HasuCalc Key Bindings",
		Items: [][2]string{
			{"Arrow Keys", "Move cell cursor (Up, Down, Left, Right)"},
			{"PageUp / PageDn", "Scroll sheet up / down by 20 rows"},
			{"Home", "Jump directly to cell A1"},
			{"End + Arrow", "Jump to the edge of data block"},
			{"/", "Open Slash Menu (File, Edit, Format, Row, Column, Sheet, Data, Formula, Chart, Help)"},
			{"F1", "Display this Help screen"},
			{"F2 / Ctrl+E", "Edit current cell contents on Line 1 (EDIT mode)"},
			{"Ctrl+F", "Find text, number, or formula in sheet"},
			{"Ctrl+H", "Find and Replace text/number/formula (/EFE)"},
			{"F3 / Shift+F3", "Find Next / Find Previous matching cell"},
			{"F5 / Ctrl+G", "GOTO - jump to cell, range, sheet, or name"},
			{"Ctrl+D / Ctrl+R", "Fill Down / Fill Right across selection"},
			{"Ctrl+Z / Ctrl+Y", "Undo / Redo last action"},
			{"Ctrl+C / Ctrl+X / Ctrl+V", "Copy / Cut / Paste cells"},
			{"Ctrl+L", "Paste Link (Reference formula to source cells)"},
			{"Alt+=", "AutoSum (=SUM) for above/left numbers"},
			{"Ctrl+T / Ctrl+PgUp/PgDn", "Switch active worksheet tab"},
			{"F9", "CALC - recalculate all formulas in workbook"},
			{"F10", "GRAPH - display the currently configured graph"},
			{"ESC", "Cancel current prompt/menu / clear selection"},
			{"Enter", "Confirm cell input or menu selection"},
			{"Mouse Click", "Move cursor to clicked cell (clears selection)"},
			{"Mouse Drag", "Select rectangular cell range by dragging"},
			{"Mouse Wheel Up/Down", "Scroll worksheet grid vertically (3 rows)"},
			{"Mouse Click on Tab", "Switch to clicked worksheet tab"},
		},
	},
	{
		Title: "Cell Input & Formula Syntax",
		Items: [][2]string{
			{"Numbers", "123, 45.67, 1e5, -0.05"},
			{"Labels (Left)", "'Text (e.g. 'Japan) - default left alignment"},
			{"Labels (Right)", "\"Text (e.g. \"Total) - right alignment"},
			{"Labels (Center)", "^Text (e.g. ^Title) - center alignment"},
			{"Labels (Repeat)", "\\= or \\- - repeats character across cell width"},
			{"Formulas", "Starts with =, +, @ (e.g. =A1+B1, @SUM(A1..A10), =SUM(A1:A10))"},
			{"Cell References", "Relative: A1, Absolute: $A$1, Mixed: $A1, A$1, Cross-sheet: Sheet2!A1"},
			{"Range Notation", "A1:B10 or A1..B10; whole column A:A uses the used range"},
			{"Operators", "+, -, *, /, ^, &, =, <>, <, <=, >, >="},
			{"Logical Ops", "#AND#, #OR#, #NOT#"},
		},
	},
	{
		Title: "Built-in @Functions",
		Items: [][2]string{
			{"@SUM / @AVG / @COUNT", "Calculates sum, arithmetic mean, non-empty count"},
			{"@SUMIF / @COUNTIF", "Conditional sum and count by criteria (e.g. \">50\")"},
			{"@AVERAGEIF(rng, crit)", "Calculates conditional average"},
			{"@MIN / @MAX / @ROUND", "Minimum / Maximum / Rounding to n places"},
			{"@INT / @ABS / @SQRT / @MOD", "Integer / Absolute / Square root / Modulo"},
			{"@IF(cond, t, f) / @IFERROR", "Conditional branching and error fallback handler"},
			{"@XLOOKUP(key, lk, ret)", "Modern 2-way exact & approximate lookup with fallback"},
			{"@VLOOKUP / @HLOOKUP", "Vertical / Horizontal table lookup"},
			{"@INDEX / @MATCH", "Array intersection and index matching position"},
			{"@CHOOSE / @ISNUMBER", "Value selection by index / Type checking"},
			{"@ISERROR / @ISERR", "Any error / errors excluding #N/A"},
			{"@TRIM / @SUBSTITUTE", "Strip spaces / Replace substring in text"},
			{"@UPPER / @LOWER / @PROPER", "Case conversion: UPPER, lower, Title Case"},
			{"@LEFT / @RIGHT / @MID / @LEN", "Substring extraction & length functions"},
			{"@CONCAT / @TEXTJOIN", "Concatenate strings with delimiter and options"},
			{"@FIND / @SEARCH", "Case-sensitive / wildcard text search position"},
			{"@TEXT(n, fmt) / @VALUE(s)", "Convert number to string / string to number"},
			{"@DATE / @TODAY / @NOW", "Serial date / Current date / Current datetime"},
			{"@YEAR / @MONTH / @DAY / @WEEKDAY", "Extract year, month, day, weekday (1..7)"},
			{"@EDATE / @EOMONTH", "Add months to date / End of month calculation"},
		},
	},
	{
		Title: "Slash Menu Command Hierarchy (/)",
		Items: [][2]string{
			{"/F (File)", "New (/FN), Open (/FO), Save (/FS), Export (/FX → format → Sheet/Range), Quit (/FQ)"},
			{"/E (Edit)", "History (/EH → U/R), Clipboard (/EC → X/C/V→A/V/L/T), Clear (/EK), Find (/EF → F/N/P/E/A), Goto (/EG)"},
			{"/M (Format)", "Number (/MN → C/P/F/,/D/S/G), Align (/MA → L/R/C)"},
			{"/R (Row)", "Insert (/RI), Delete (/RD)"},
			{"/L (Column)", "Insert (/LI), Delete (/LD), Width (/LW → S/R/G)"},
			{"/W (Sheet)", "Add/Delete/Rename (/WA /WD /WR), Go (/WG → S/N/P), Freeze (/WF → B/H/V/C)"},
			{"/D (Data)", "Sort (/DS), Fill (/DF → A/S/D/R), Transpose (/DT), Names (/DN → C/D/L)"},
			{"/O (Formula)", "Aggregate (/OA → S/A/C/M/I), Function (/OF), Date (/OD → T/N), Recalculate (/O9, F9)"},
			{"/C (Chart)", "View (/CV, F10), Type (/CT), Title (/CI), X-Axis (/CX), Series (/CE → A..F), Status (/CS), Save-PNG (/CP)"},
			{"/? (Help)", "About (/?A), Keybindings (/?K, F1), Palette (/?P, Ctrl+K)"},
		},
	},
}

func RenderHelpScreen(s tcell.Screen, styles Styles) {
	pageIdx := 0
	numPages := len(HelpPages)

	for {
		s.Clear()
		_, h := s.Size()

		page := HelpPages[pageIdx]

		// Top Banner
		banner := fmt.Sprintf(" HASUCALC HELP -- Page %d of %d: %s ", pageIdx+1, numPages, page.Title)
		drawText(s, 0, 0, banner, styles.Header)

		// Items
		rowY := 2
		for _, item := range page.Items {
			if rowY >= h-3 {
				break
			}
			keyStr := fmt.Sprintf("  %-24s", item[0])
			drawText(s, 0, rowY, keyStr, styles.Header)
			descStr := fmt.Sprintf(": %s", item[1])
			drawText(s, runewidth.StringWidth(keyStr), rowY, descStr, styles.Default)
			rowY++
		}

		// Bottom Navigation
		navStr := " [Left/Right/PgUp/PgDn/Space: Change Page]  [ESC / Enter / Q: Return to Sheet] "
		drawText(s, 0, h-1, navStr, styles.Header)

		s.Show()

		ev := s.PollEvent()
		switch tev := ev.(type) {
		case *tcell.EventKey:
			switch tev.Key() {
			case tcell.KeyRight, tcell.KeyPgDn, tcell.KeyRune:
				if tev.Key() == tcell.KeyRune && (tev.Rune() == 'q' || tev.Rune() == 'Q') {
					return
				}
				if tev.Key() == tcell.KeyRune && tev.Rune() == ' ' {
					pageIdx = (pageIdx + 1) % numPages
				} else if tev.Key() != tcell.KeyRune {
					pageIdx = (pageIdx + 1) % numPages
				}
			case tcell.KeyLeft, tcell.KeyPgUp:
				pageIdx = (pageIdx - 1 + numPages) % numPages
			case tcell.KeyEscape, tcell.KeyEnter:
				return
			}
		case *tcell.EventResize:
			s.Sync()
		}
	}
}

func RenderNamedRangesScreen(s tcell.Screen, namedMap map[string]any, styles Styles) {
	s.Clear()
	w, h := s.Size()
	modalW := 68
	if modalW > w-4 {
		modalW = w - 4
	}
	modalH := 18
	if modalH > h-2 {
		modalH = h - 2
	}
	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2
	if modalY < 1 {
		modalY = 1
	}

	boxStyle := styles.GraphBorder
	itemStyle := styles.Default
	headerStyle := styles.Header

	// Background & Borders
	for y := modalY; y < modalY+modalH; y++ {
		for x := modalX; x < modalX+modalW; x++ {
			s.SetContent(x, y, ' ', nil, itemStyle)
		}
	}
	for x := modalX; x < modalX+modalW; x++ {
		s.SetContent(x, modalY, '═', nil, boxStyle)
		s.SetContent(x, modalY+modalH-1, '═', nil, boxStyle)
	}
	for y := modalY; y < modalY+modalH; y++ {
		s.SetContent(modalX, y, '║', nil, boxStyle)
		s.SetContent(modalX+modalW-1, y, '║', nil, boxStyle)
	}
	s.SetContent(modalX, modalY, '╔', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY, '╗', nil, boxStyle)
	s.SetContent(modalX, modalY+modalH-1, '╚', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY+modalH-1, '╝', nil, boxStyle)

	title := " NAMED RANGES LIST (F3) "
	drawTextFast(s, modalX+(modalW-len(title))/2, modalY, title, boxStyle, w)

	// Column headers
	colHdr := fmt.Sprintf("  %-20s %-20s %-16s", "RANGE NAME", "TARGET RANGE", "TYPE")
	drawTextFast(s, modalX+2, modalY+2, colHdr, headerStyle, modalX+modalW-2)
	divLine := ""
	for i := 0; i < modalW-6; i++ {
		divLine += "─"
	}
	drawTextFast(s, modalX+2, modalY+3, divLine, boxStyle, modalX+modalW-2)

	lineY := modalY + 4
	if len(namedMap) == 0 {
		drawTextFast(s, modalX+4, lineY, "No named ranges defined yet.", itemStyle, modalX+modalW-2)
		drawTextFast(s, modalX+4, lineY+1, "Use /DN (Data -> Name-Create) to define a range name.", itemStyle, modalX+modalW-2)
	} else {
		var names []string
		for n := range namedMap {
			names = append(names, n)
		}
		// Alphabetical sort
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				if names[i] > names[j] {
					names[i], names[j] = names[j], names[i]
				}
			}
		}

		for _, name := range names {
			if lineY >= modalY+modalH-3 {
				drawTextFast(s, modalX+4, lineY, fmt.Sprintf("... and %d more", len(names)-(lineY-(modalY+4))), itemStyle, modalX+modalW-2)
				break
			}
			targetVal := namedMap[name]
			rangeStr := fmt.Sprintf("%v", targetVal)
			typeStr := "Range"
			if rRef, ok := targetVal.(coord.RangeRef); ok {
				numCells := (rRef.MaxCol() - rRef.MinCol() + 1) * (rRef.MaxRow() - rRef.MinRow() + 1)
				if numCells == 1 {
					typeStr = "Single Cell"
				} else {
					typeStr = fmt.Sprintf("%d cells", numCells)
				}
			} else if _, ok := targetVal.(coord.CellRef); ok {
				typeStr = "Single Cell"
			}
			rowStr := fmt.Sprintf("  %-20s %-20s %-16s", name, rangeStr, typeStr)
			drawTextFast(s, modalX+2, lineY, rowStr, itemStyle, modalX+modalW-2)
			lineY++
		}
	}

	footer := " [Press any key or ESC to return] "
	drawTextFast(s, modalX+(modalW-runewidth.StringWidth(footer))/2, modalY+modalH-1, footer, styles.Status, w)

	s.Show()
	for {
		ev := s.PollEvent()
		if _, ok := ev.(*tcell.EventKey); ok {
			return
		}
		if _, ok := ev.(*tcell.EventResize); ok {
			s.Sync()
			RenderNamedRangesScreen(s, namedMap, styles)
			return
		}
	}
}

// AppVersion is the version identifier for HasuCalc TUI, sourced from version.Version.
const AppVersion = version.Version

// RenderAboutScreen displays a simple, clean About dialog modal with app name, version, and MIT license.
func RenderAboutScreen(s tcell.Screen, styles Styles) {
	s.Clear()
	w, h := s.Size()
	modalW := 48
	if modalW > w-4 {
		modalW = w - 4
	}
	modalH := 9
	if modalH > h-2 {
		modalH = h - 2
	}
	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2
	if modalY < 1 {
		modalY = 1
	}

	boxStyle := styles.GraphBorder
	itemStyle := styles.Default
	headerStyle := styles.Header
	accentStyle := styles.Status

	// Background & Borders
	for y := modalY; y < modalY+modalH; y++ {
		for x := modalX; x < modalX+modalW; x++ {
			s.SetContent(x, y, ' ', nil, itemStyle)
		}
	}
	for x := modalX; x < modalX+modalW; x++ {
		s.SetContent(x, modalY, '═', nil, boxStyle)
		s.SetContent(x, modalY+modalH-1, '═', nil, boxStyle)
	}
	for y := modalY; y < modalY+modalH; y++ {
		s.SetContent(modalX, y, '║', nil, boxStyle)
		s.SetContent(modalX+modalW-1, y, '║', nil, boxStyle)
	}
	s.SetContent(modalX, modalY, '╔', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY, '╗', nil, boxStyle)
	s.SetContent(modalX, modalY+modalH-1, '╚', nil, boxStyle)
	s.SetContent(modalX+modalW-1, modalY+modalH-1, '╝', nil, boxStyle)

	// App Name & Version
	appName := fmt.Sprintf("HasuCalc  Version %s", AppVersion)
	drawTextFast(s, modalX+(modalW-runewidth.StringWidth(appName))/2, modalY+2, appName, headerStyle, modalX+modalW-2)

	// License & Copyright
	licStr := "MIT License"
	drawTextFast(s, modalX+(modalW-runewidth.StringWidth(licStr))/2, modalY+4, licStr, accentStyle, modalX+modalW-2)

	cpyStr := "Copyright (c) 2026 HasuCalc Contributors"
	drawTextFast(s, modalX+(modalW-runewidth.StringWidth(cpyStr))/2, modalY+5, cpyStr, itemStyle, modalX+modalW-2)

	// Footer close prompt on bottom border
	footer := " [ Press any key to close ] "
	drawTextFast(s, modalX+(modalW-runewidth.StringWidth(footer))/2, modalY+modalH-1, footer, styles.Status, w)

	s.Show()
	for {
		ev := s.PollEvent()
		if _, ok := ev.(*tcell.EventKey); ok {
			return
		}
		if _, ok := ev.(*tcell.EventResize); ok {
			s.Sync()
			RenderAboutScreen(s, styles)
			return
		}
	}
}
