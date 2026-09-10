package tui

import (
	"github.com/gdamore/tcell/v2"
)

type Styles struct {
	Default       tcell.Style
	Header        tcell.Style
	CellCursor    tcell.Style
	MenuSel       tcell.Style
	MenuText      tcell.Style
	MenuDesc      tcell.Style
	Status        tcell.Style
	ModeBox       tcell.Style
	RangeSel      tcell.Style
	Error         tcell.Style
	GridNumber    tcell.Style
	GridLabel     tcell.Style
	ActiveTab     tcell.Style
	InactiveTab   tcell.Style
	TabArrow      tcell.Style
	GraphBorder   tcell.Style
	GraphGrid     tcell.Style
	GraphSeriesA  tcell.Style
	GraphSeriesB  tcell.Style
	GraphSeriesC  tcell.Style
	GraphSeriesD  tcell.Style
	GraphSeriesE  tcell.Style
	GraphSeriesF  tcell.Style
}

func InitStyles() Styles {
	// Authentic PC-98 / DOS Lotus 1-2-3 color palette
	cyanBg := tcell.ColorDarkCyan
	blackBg := tcell.ColorBlack
	yellowFg := tcell.ColorYellow
	cyanFg := tcell.ColorAqua
	greenFg := tcell.ColorGreen
	whiteFg := tcell.ColorWhite
	blackFg := tcell.ColorBlack

	return Styles{
		Default:       tcell.StyleDefault.Background(blackBg).Foreground(whiteFg),
		Header:        tcell.StyleDefault.Background(cyanBg).Foreground(whiteFg).Bold(true),
		CellCursor:    tcell.StyleDefault.Background(cyanFg).Foreground(blackFg).Bold(true),
		MenuSel:       tcell.StyleDefault.Background(cyanBg).Foreground(whiteFg).Bold(true),
		MenuText:      tcell.StyleDefault.Background(blackBg).Foreground(whiteFg),
		MenuDesc:      tcell.StyleDefault.Background(blackBg).Foreground(tcell.ColorLightGray),
		Status:        tcell.StyleDefault.Background(blackBg).Foreground(tcell.ColorLightCyan),
		ModeBox:       tcell.StyleDefault.Background(cyanBg).Foreground(whiteFg).Bold(true),
		ActiveTab:     tcell.StyleDefault.Background(cyanFg).Foreground(blackFg).Bold(true),
		InactiveTab:   tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(tcell.ColorLightGray),
		TabArrow:      tcell.StyleDefault.Background(blackBg).Foreground(yellowFg).Bold(true),
		RangeSel:      tcell.StyleDefault.Background(tcell.ColorNavy).Foreground(yellowFg).Bold(true),
		Error:         tcell.StyleDefault.Background(tcell.ColorRed).Foreground(whiteFg).Bold(true),
		GridNumber:    tcell.StyleDefault.Background(blackBg).Foreground(greenFg),
		GridLabel:     tcell.StyleDefault.Background(blackBg).Foreground(whiteFg),
		GraphBorder:   tcell.StyleDefault.Background(blackBg).Foreground(whiteFg).Bold(true),
		GraphGrid:     tcell.StyleDefault.Background(blackBg).Foreground(tcell.ColorGray),
		GraphSeriesA:  tcell.StyleDefault.Background(blackBg).Foreground(yellowFg).Bold(true),
		GraphSeriesB:  tcell.StyleDefault.Background(blackBg).Foreground(cyanFg).Bold(true),
		GraphSeriesC:  tcell.StyleDefault.Background(blackBg).Foreground(greenFg).Bold(true),
		GraphSeriesD:  tcell.StyleDefault.Background(blackBg).Foreground(tcell.ColorFuchsia).Bold(true),
		GraphSeriesE:  tcell.StyleDefault.Background(blackBg).Foreground(tcell.ColorRed).Bold(true),
		GraphSeriesF:  tcell.StyleDefault.Background(blackBg).Foreground(tcell.ColorDodgerBlue).Bold(true),
	}
}
