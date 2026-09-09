package tui

import (
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// ShowSheetPickerModal presents an interactive modal for the user to select which sheet to open.
// Returns the 0-based selected sheet index, or -1 if canceled.
func ShowSheetPickerModal(s tcell.Screen, fullPath string, sheetNames []string, styles Styles) int {
	if len(sheetNames) == 0 {
		return 0
	}
	if len(sheetNames) == 1 {
		return 0
	}

	selectedIdx := 0
	scrollOffset := 0
	baseName := filepath.Base(fullPath)

	for {
		s.Clear()
		w, h := s.Size()
		modalW := 64
		if modalW > w-4 {
			modalW = w - 4
		}
		modalH := 16
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
		selStyle := styles.CellCursor
		hdrStyle := styles.Header

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

		title := " SELECT SHEET TO IMPORT "
		drawTextFast(s, modalX+(modalW-len(title))/2, modalY, title, boxStyle, w)

		// File info
		fileStr := fmt.Sprintf(" File: %s (%d sheets)", baseName, len(sheetNames))
		drawTextFast(s, modalX+2, modalY+1, fileStr, hdrStyle, modalX+modalW-2)

		promptStr := " Select which sheet tab to open:"
		drawTextFast(s, modalX+2, modalY+2, promptStr, itemStyle, modalX+modalW-2)

		divLine := ""
		for i := 0; i < modalW-4; i++ {
			divLine += "─"
		}
		drawTextFast(s, modalX+2, modalY+3, divLine, boxStyle, modalX+modalW-2)

		listH := modalH - 6
		if listH < 3 {
			listH = 3
		}

		if selectedIdx < scrollOffset {
			scrollOffset = selectedIdx
		}
		if selectedIdx >= scrollOffset+listH {
			scrollOffset = selectedIdx - listH + 1
		}

		for i := 0; i < listH; i++ {
			itemIdx := scrollOffset + i
			rowY := modalY + 4 + i
			if itemIdx < len(sheetNames) {
				shName := sheetNames[itemIdx]
				st := itemStyle
				prefix := "   "
				if itemIdx == selectedIdx {
					st = selStyle
					prefix = " > "
				}
				rowText := fmt.Sprintf("%s%d. %s", prefix, itemIdx+1, shName)
				if runewidth.StringWidth(rowText) > modalW-4 {
					rowText = runewidth.Truncate(rowText, modalW-4, "...")
				}
				for x := modalX + 2; x < modalX+modalW-2; x++ {
					s.SetContent(x, rowY, ' ', nil, st)
				}
				drawTextFast(s, modalX+2, rowY, rowText, st, modalX+modalW-2)
			}
		}

		footer := " [↑/↓: Select Sheet]  [Enter: Open]  [ESC: Cancel] "
		drawTextFast(s, modalX+(modalW-runewidth.StringWidth(footer))/2, modalY+modalH-1, footer, styles.Status, w)

		s.Show()

		ev := s.PollEvent()
		switch tev := ev.(type) {
		case *tcell.EventKey:
			switch tev.Key() {
			case tcell.KeyEscape:
				return -1
			case tcell.KeyEnter:
				return selectedIdx
			case tcell.KeyUp:
				if selectedIdx > 0 {
					selectedIdx--
				}
			case tcell.KeyDown:
				if selectedIdx < len(sheetNames)-1 {
					selectedIdx++
				}
			case tcell.KeyHome:
				selectedIdx = 0
			case tcell.KeyEnd:
				selectedIdx = len(sheetNames) - 1
			case tcell.KeyRune:
				// Number selection (1..9)
				if tev.Rune() >= '1' && tev.Rune() <= '9' {
					num := int(tev.Rune() - '1')
					if num < len(sheetNames) {
						return num
					}
				}
			}
		case *tcell.EventResize:
			s.Sync()
		}
	}
}

// RenderLoadingModal renders a sleek centered loading / processing modal indicator on screen.
func RenderLoadingModal(s tcell.Screen, filename string, message string, styles Styles) {
	w, h := s.Size()
	modalW := 58
	if modalW > w-4 {
		modalW = w - 4
	}
	modalH := 7
	modalX := (w - modalW) / 2
	modalY := (h - modalH) / 2
	if modalY < 1 {
		modalY = 1
	}

	boxStyle := styles.GraphBorder
	itemStyle := styles.Default
	hdrStyle := styles.Header
	waitStyle := styles.ModeBox

	// Box background
	for y := modalY; y < modalY+modalH; y++ {
		for x := modalX; x < modalX+modalW; x++ {
			s.SetContent(x, y, ' ', nil, itemStyle)
		}
	}
	// Borders
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

	title := " [WAIT] PROCESSING WORKSHEET "
	drawTextFast(s, modalX+(modalW-len(title))/2, modalY, title, waitStyle, w)

	fileLine := fmt.Sprintf(" Target: %s", filepath.Base(filename))
	if runewidth.StringWidth(fileLine) > modalW-4 {
		fileLine = runewidth.Truncate(fileLine, modalW-4, "...")
	}
	drawTextFast(s, modalX+2, modalY+2, fileLine, hdrStyle, modalX+modalW-2)

	msgLine := fmt.Sprintf(" Status: %s", message)
	if runewidth.StringWidth(msgLine) > modalW-4 {
		msgLine = runewidth.Truncate(msgLine, modalW-4, "...")
	}
	drawTextFast(s, modalX+2, modalY+4, msgLine, itemStyle, modalX+modalW-2)

	s.Show()
}
