package tui

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"hasucalc/cell"
	"hasucalc/sheet"
)

type seriesInfo struct {
	name   string
	label  string
	values []float64
	style  tcell.Style
	lineCh rune
	dotCh  rune
}

// DefaultPNGFilename returns a default PNG filename based on the table's filename and graph type (e.g. "engraph_PIE.png").
func DefaultPNGFilename(sh *sheet.Sheet, currentFile string) string {
	base := "hasucalc"
	if currentFile != "" && currentFile != "[Untitled]" {
		base = filepath.Base(currentFile)
		ext := filepath.Ext(base)
		if ext != "" {
			base = strings.TrimSuffix(base, ext)
		}
	} else if sh != nil && sh.Name() != "" {
		base = sh.Name()
	}

	gType := "LINE"
	if sh != nil && sh.Graph().Type != "" {
		gType = strings.ToUpper(sh.Graph().Type)
	}

	return fmt.Sprintf("%s_%s.png", base, gType)
}

func RenderGraphScreen(s tcell.Screen, sh *sheet.Sheet, styles Styles, currentFilename ...string) {
	currFile := ""
	if len(currentFilename) > 0 {
		currFile = currentFilename[0]
	}

	g := sh.Graph()

	// 1. Gather Series data
	seriesColors := []struct {
		style  tcell.Style
		lineCh rune
		dotCh  rune
	}{
		{styles.GraphSeriesA, '─', '●'}, // Yellow
		{styles.GraphSeriesB, '┄', '■'}, // Cyan
		{styles.GraphSeriesC, '┈', '▲'}, // Green
		{styles.GraphSeriesD, '―', '◆'}, // Magenta
		{styles.GraphSeriesE, '─', '★'}, // Red
		{styles.GraphSeriesF, '┄', '✦'}, // Blue
	}

	var seriesList []seriesInfo
	seriesKeys := []string{"A", "B", "C", "D", "E", "F"}

	for idx, sKey := range seriesKeys {
		rRef := g.Series[sKey]
		if rRef != nil {
			cells := rRef.Cells()
			if len(cells) == 0 {
				continue
			}

			label := fmt.Sprintf("Series %s", sKey)
			firstVal := sh.GetCellValue(cells[0].Col, cells[0].Row)
			firstCell := sh.GetCell(cells[0].Col, cells[0].Row)

			var vals []float64
			startIndex := 0

			// If first cell is a text label / non-numeric string, and there are more cells:
			isFirstHeader := false
			if len(cells) > 1 {
				if firstCell != nil && firstCell.Type == cell.TypeLabel {
					isFirstHeader = true
				} else if _, ok := toFloat(firstVal); !ok && firstVal != nil && fmt.Sprintf("%v", firstVal) != "" {
					isFirstHeader = true
				}
			}

			if isFirstHeader {
				label = fmt.Sprintf("%v", firstVal)
				startIndex = 1
			} else {
				isHoriz := (rRef.MinRow() == rRef.MaxRow())
				isVert := (rRef.MinCol() == rRef.MaxCol())

				if isHoriz && rRef.MinCol() > 0 {
					// Check cell to the left for horizontal row series
					leftC := rRef.MinCol() - 1
					leftR := rRef.MinRow()
					leftCell := sh.GetCell(leftC, leftR)
					leftVal := sh.GetCellValue(leftC, leftR)
					if (leftCell != nil && leftCell.Type == cell.TypeLabel) || isNonNumericText(leftVal) {
						label = fmt.Sprintf("%v", leftVal)
					}
				} else if isVert && rRef.MinRow() > 0 {
					// Check cell above for vertical column series
					topC := rRef.MinCol()
					topR := rRef.MinRow() - 1
					topCell := sh.GetCell(topC, topR)
					topVal := sh.GetCellValue(topC, topR)
					if (topCell != nil && topCell.Type == cell.TypeLabel) || isNonNumericText(topVal) {
						label = fmt.Sprintf("%v", topVal)
					}
				}
			}

			for i := startIndex; i < len(cells); i++ {
				cr := cells[i]
				v := sh.GetCellValue(cr.Col, cr.Row)
				if num, ok := toFloat(v); ok {
					vals = append(vals, num)
				} else {
					vals = append(vals, 0.0)
				}
			}

			if len(vals) > 0 {
				colorCfg := seriesColors[idx%len(seriesColors)]
				seriesList = append(seriesList, seriesInfo{
					name:   sKey,
					label:  label,
					values: vals,
					style:  colorCfg.style,
					lineCh: colorCfg.lineCh,
					dotCh:  colorCfg.dotCh,
				})
			}
		}
	}

	// 2. Gather X labels and X-axis subtitle
	var xLabels []string
	xSubTitle := ""

	maxSeriesPoints := 0
	for _, si := range seriesList {
		if len(si.values) > maxSeriesPoints {
			maxSeriesPoints = len(si.values)
		}
	}

	if g.RangeX != nil {
		xCells := g.RangeX.Cells()
		startX := 0

		if len(xCells) > 1 && maxSeriesPoints > 0 && len(xCells) == maxSeriesPoints+1 {
			firstVal := sh.GetCellValue(xCells[0].Col, xCells[0].Row)
			firstCell := sh.GetCell(xCells[0].Col, xCells[0].Row)
			if (firstCell != nil && firstCell.Type == cell.TypeLabel) || firstVal != nil {
				xSubTitle = fmt.Sprintf("%v", firstVal)
				startX = 1
			}
		}

		for i := startX; i < len(xCells); i++ {
			cr := xCells[i]
			val := sh.GetCellValue(cr.Col, cr.Row)
			if val != nil {
				xLabels = append(xLabels, fmt.Sprintf("%v", val))
			} else {
				xLabels = append(xLabels, "")
			}
		}
	}

	s.Clear()
	w, h := s.Size()

	if len(seriesList) == 0 {
		msg := "No graph series configured! Use /CA to set series A, /CX to set X-axis."
		hint := "Press any key to return to Worksheet..."
		drawText(s, (w-runewidth.StringWidth(msg))/2, h/2, msg, styles.Header)
		drawText(s, (w-runewidth.StringWidth(hint))/2, h/2+2, hint, styles.Default)
		s.Show()
		for {
			ev := s.PollEvent()
			if _, ok := ev.(*tcell.EventKey); ok {
				return
			}
		}
	}

	numPoints := 0
	for _, si := range seriesList {
		if len(si.values) > numPoints {
			numPoints = len(si.values)
		}
	}
	for len(xLabels) < numPoints {
		xLabels = append(xLabels, fmt.Sprintf("%d", len(xLabels)+1))
	}

	// Calculate Min & Max values
	minVal, maxVal := 0.0, 1.0
	first := true

	if g.Type == "STACKED" {
		minVal = 0.0
		maxVal = 0.0
		for ptIdx := 0; ptIdx < numPoints; ptIdx++ {
			sumAtPt := 0.0
			for _, si := range seriesList {
				if ptIdx < len(si.values) && si.values[ptIdx] > 0 {
					sumAtPt += si.values[ptIdx]
				}
			}
			if sumAtPt > maxVal {
				maxVal = sumAtPt
			}
		}
		if maxVal == 0 {
			maxVal = 1.0
		}
	} else {
		for _, si := range seriesList {
			for _, v := range si.values {
				if first {
					minVal, maxVal = v, v
					first = false
				} else {
					if v < minVal {
						minVal = v
					}
					if v > maxVal {
						maxVal = v
					}
				}
			}
		}
		if minVal > 0 {
			minVal = 0 // Baseline at 0
		}
		if minVal == maxVal {
			maxVal += 1.0
		}
	}

	// Dimensions
	boxLeft := 2
	boxRight := w - 3
	boxTop := 2
	boxBottom := h - 6
	if boxBottom <= boxTop+5 {
		boxBottom = boxTop + 6
	}

	// Draw Title
	title := g.Title
	if title != "" {
		drawText(s, (w-runewidth.StringWidth(title))/2, 1, title, styles.GraphBorder)
	}

	// Draw Double Border Box (Authentic Image 3 style: ╔═╗║╚═╝)
	for x := boxLeft; x <= boxRight; x++ {
		s.SetContent(x, boxTop, '═', nil, styles.GraphBorder)
		s.SetContent(x, boxBottom, '═', nil, styles.GraphBorder)
	}
	for y := boxTop; y <= boxBottom; y++ {
		s.SetContent(boxLeft, y, '║', nil, styles.GraphBorder)
		s.SetContent(boxRight, y, '║', nil, styles.GraphBorder)
	}
	s.SetContent(boxLeft, boxTop, '╔', nil, styles.GraphBorder)
	s.SetContent(boxRight, boxTop, '╗', nil, styles.GraphBorder)
	s.SetContent(boxLeft, boxBottom, '╚', nil, styles.GraphBorder)
	s.SetContent(boxRight, boxBottom, '╝', nil, styles.GraphBorder)

	if g.Type == "PIE" {
		renderPieGraph(s, sh, g, seriesList, xLabels, styles, w, h, boxLeft, boxRight, boxTop, boxBottom)
		// Bottom Prompt
		bottomPrompt := " [S: Save PNG Image (1280x720) | ESC/Enter: Return] "
		drawText(s, (w-runewidth.StringWidth(bottomPrompt))/2, h-1, bottomPrompt, styles.Status)
		s.Show()
		for {
			ev := s.PollEvent()
			if keyEv, ok := ev.(*tcell.EventKey); ok {
				if keyEv.Key() == tcell.KeyRune && (keyEv.Rune() == 's' || keyEv.Rune() == 'S') {
					fn := DefaultPNGFilename(sh, currFile)
					if err := ExportGraphPNG(sh, fn, 1280, 720); err == nil {
						msg := fmt.Sprintf(" Saved '%s' (1280x720 PNG) ", fn)
						drawText(s, (w-runewidth.StringWidth(msg))/2, h-1, msg, styles.Header)
						s.Show()
					} else {
						msg := fmt.Sprintf(" Error saving PNG: %v ", err)
						drawText(s, (w-runewidth.StringWidth(msg))/2, h-1, msg, styles.Error)
						s.Show()
					}
					continue
				}
				return
			}
			if _, ok := ev.(*tcell.EventResize); ok {
				s.Sync()
				RenderGraphScreen(s, sh, styles, currFile)
				return
			}
		}
	}

	plotRight := boxRight - 2
	plotHeight := boxBottom - boxTop - 1

	if plotHeight < 3 {
		return
	}

	// Y-axis Ticks & Horizontal Dotted Grid Lines
	numTicks := 8
	if plotHeight < 8 {
		numTicks = plotHeight
	}
	tickLabels := make([]string, numTicks+1)
	maxYLblW := 6
	for i := 0; i <= numTicks; i++ {
		frac := float64(i) / float64(numTicks)
		val := minVal + frac*(maxVal-minVal)
		tickLabels[i] = formatAxisTick(val)
		if tw := runewidth.StringWidth(tickLabels[i]); tw > maxYLblW {
			maxYLblW = tw
		}
	}
	yAxisW := maxYLblW
	if yAxisW > 12 {
		yAxisW = 12
	}
	plotLeft := boxLeft + yAxisW + 1
	plotWidth := plotRight - plotLeft + 1

	if plotWidth < 5 {
		return
	}

	for i := 0; i <= numTicks; i++ {
		frac := float64(i) / float64(numTicks)
		y := boxBottom - 1 - int(math.Round(frac*float64(plotHeight-1)))
		lbl := tickLabels[i]
		if runewidth.StringWidth(lbl) > yAxisW {
			lbl = runewidth.Truncate(lbl, yAxisW, "")
		}
		pad := yAxisW - runewidth.StringWidth(lbl)
		if pad > 0 {
			lbl = strings.Repeat(" ", pad) + lbl
		}
		drawText(s, boxLeft+1, y, lbl, styles.Default)

		// Horizontal Dotted Grid Line
		for gx := plotLeft; gx <= plotRight; gx++ {
			if (gx-plotLeft)%2 == 0 {
				s.SetContent(gx, y, '·', nil, styles.GraphGrid)
			}
		}
	}

	// Calculate X step
	stepX := float64(plotWidth) / float64(numPoints)
	if stepX < 1 {
		stepX = 1
	}

	valToY := func(val float64) int {
		frac := (val - minVal) / (maxVal - minVal)
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		return boxBottom - 1 - int(math.Round(frac*float64(plotHeight-1)))
	}

	ptCenterX := make(map[int]int)

	switch g.Type {
	case "BAR":
		numS := len(seriesList)
		if numS > 0 {
			minClusterWidth := numS + 1
			if minClusterWidth < 2 {
				minClusterWidth = 2
			}
			maxClusters := plotWidth / minClusterWidth
			if maxClusters < 1 {
				maxClusters = 1
			}

			barStep := 1
			if numPoints > maxClusters {
				rawStep := float64(numPoints) / float64(maxClusters)
				niceSteps := []int{1, 2, 5, 10, 20, 25, 50, 100, 200, 250, 500, 1000}
				for _, sVal := range niceSteps {
					if float64(sVal) >= rawStep {
						barStep = sVal
						break
					}
				}
				if barStep < int(rawStep) {
					barStep = int(math.Ceil(rawStep))
				}
			}

			var sampledIndices []int
			for ptIdx := 0; ptIdx < numPoints; ptIdx += barStep {
				sampledIndices = append(sampledIndices, ptIdx)
			}
			N := len(sampledIndices)

			for k, ptIdx := range sampledIndices {
				span := float64(plotWidth) / float64(N)
				cLeft := plotLeft + int(float64(k)*span)
				cRight := plotLeft + int(float64(k+1)*span) - 1
				if cRight > plotRight {
					cRight = plotRight
				}
				cWidth := cRight - cLeft + 1

				barsTotalW := cWidth - 1
				if barsTotalW < numS {
					barsTotalW = numS
				}
				subBarW := barsTotalW / numS
				if subBarW < 1 {
					subBarW = 1
				}
				actualW := subBarW * numS
				startX := cLeft + (cWidth-actualW)/2
				if startX < cLeft {
					startX = cLeft
				}

				ptCenterX[ptIdx] = startX + actualW/2

				for sIdx, si := range seriesList {
					if ptIdx >= len(si.values) {
						continue
					}
					v := si.values[ptIdx]
					yTop := valToY(v)
					bx := startX + sIdx*subBarW
					for y := yTop; y <= boxBottom-1; y++ {
						for bw := 0; bw < subBarW; bw++ {
							if bx+bw <= plotRight {
								s.SetContent(bx+bw, y, '█', nil, si.style)
							}
						}
					}
				}
			}
		}

	case "STACKED":
		minClusterWidth := 3
		maxClusters := plotWidth / minClusterWidth
		if maxClusters < 1 {
			maxClusters = 1
		}
		barStep := 1
		if numPoints > maxClusters {
			rawStep := float64(numPoints) / float64(maxClusters)
			niceSteps := []int{1, 2, 5, 10, 20, 25, 50, 100, 200, 250, 500, 1000}
			for _, sVal := range niceSteps {
				if float64(sVal) >= rawStep {
					barStep = sVal
					break
				}
			}
			if barStep < int(rawStep) {
				barStep = int(math.Ceil(rawStep))
			}
		}

		var sampledIndices []int
		for ptIdx := 0; ptIdx < numPoints; ptIdx += barStep {
			sampledIndices = append(sampledIndices, ptIdx)
		}
		N := len(sampledIndices)

		for k, ptIdx := range sampledIndices {
			span := float64(plotWidth) / float64(N)
			cLeft := plotLeft + int(float64(k)*span)
			cRight := plotLeft + int(float64(k+1)*span) - 1
			if cRight > plotRight {
				cRight = plotRight
			}
			cWidth := cRight - cLeft + 1

			barW := cWidth - 1
			if barW < 1 {
				barW = 1
			}
			if barW > 10 {
				barW = 10
			}
			centerX := cLeft + cWidth/2
			bx := centerX - barW/2
			if bx < cLeft {
				bx = cLeft
			}
			ptCenterX[ptIdx] = centerX

			accum := 0.0
			for _, si := range seriesList {
				if ptIdx >= len(si.values) {
					continue
				}
				v := si.values[ptIdx]
				if v <= 0 {
					continue
				}
				prevAccum := accum
				accum += v
				yTop := valToY(accum)
				yBot := valToY(prevAccum)

				for y := yTop; y <= yBot; y++ {
					for bw := 0; bw < barW; bw++ {
						if bx+bw <= plotRight && y >= boxTop+1 && y <= boxBottom-1 {
							s.SetContent(bx+bw, y, '█', nil, si.style)
						}
					}
				}
			}
		}

	default: // "LINE"
		for ptIdx := 0; ptIdx < numPoints; ptIdx++ {
			ptCenterX[ptIdx] = plotLeft + int(math.Round((float64(ptIdx)+0.5)*stepX))
		}
		for _, si := range seriesList {
			var screenPoints [][2]int
			for ptIdx := 0; ptIdx < len(si.values); ptIdx++ {
				px := ptCenterX[ptIdx]
				py := valToY(si.values[ptIdx])
				screenPoints = append(screenPoints, [2]int{px, py})
			}

			for i := 0; i < len(screenPoints)-1; i++ {
				p1 := screenPoints[i]
				p2 := screenPoints[i+1]
				drawLine(s, p1[0], p1[1], p2[0], p2[1], plotLeft, plotRight, boxTop+1, boxBottom-1, si.lineCh, si.style)
			}

			for _, pt := range screenPoints {
				if pt[0] >= plotLeft && pt[0] <= plotRight && pt[1] >= boxTop+1 && pt[1] <= boxBottom-1 {
					s.SetContent(pt[0], pt[1], si.dotCh, nil, si.style)
				}
			}
		}
	}

	valToX := func(ptIdx int) int {
		if x, ok := ptCenterX[ptIdx]; ok {
			return x
		}
		return plotLeft + int(math.Round((float64(ptIdx)+0.5)*stepX))
	}

	// Draw X-axis Ticks and Labels below box
	tickY := boxBottom + 1
	lblY := boxBottom + 2

	// Calculate maximum label width among available X labels
	maxLblW := 1
	for _, l := range xLabels {
		lw := runewidth.StringWidth(l)
		if lw > maxLblW {
			maxLblW = lw
		}
	}
	if maxLblW < 3 {
		maxLblW = 3
	}

	minSlotWidth := maxLblW + 2 // Label width + at least 2 char gap
	maxVisibleLabels := plotWidth / minSlotWidth
	if maxVisibleLabels < 2 {
		maxVisibleLabels = 2
	}

	// Calculate human-friendly thinning step (1, 2, 5, 10, 20, 25, 50, 100, 200, 500, 1000...)
	rawStep := float64(numPoints) / float64(maxVisibleLabels)
	skip := 1
	if rawStep > 1.0 {
		niceSteps := []int{1, 2, 5, 10, 20, 25, 50, 100, 200, 250, 500, 1000, 2000, 5000, 10000}
		for _, sVal := range niceSteps {
			if float64(sVal) >= rawStep {
				skip = sVal
				break
			}
		}
		if skip < int(rawStep) {
			skip = int(math.Ceil(rawStep))
		}
	}

	lastDrawnRight := -1
	for ptIdx := 0; ptIdx < numPoints; ptIdx++ {
		// If BAR or STACKED, only consider points that were actually rendered as bars
		if g.Type == "BAR" || g.Type == "STACKED" {
			if _, ok := ptCenterX[ptIdx]; !ok {
				continue
			}
		}

		// Only draw ticks and labels at step intervals, or candidate last point
		isCandidate := (ptIdx%skip == 0)
		if !isCandidate && ptIdx == numPoints-1 && numPoints > 2 {
			isCandidate = true
		}
		if !isCandidate {
			continue
		}

		px := valToX(ptIdx)
		if px < plotLeft || px > plotRight {
			continue
		}

		var lbl string
		if ptIdx < len(xLabels) {
			lbl = xLabels[ptIdx]
		}
		lw := runewidth.StringWidth(lbl)
		lx := px - lw/2
		if lx < boxLeft {
			lx = boxLeft
		}
		if lx+lw >= w {
			lx = w - 1 - lw
		}

		// Ensure no collision with previously drawn label
		if lastDrawnRight != -1 && lx <= lastDrawnRight+1 {
			continue
		}

		// Draw tick
		s.SetContent(px, tickY, '│', nil, styles.GraphBorder)

		// Draw label
		if lbl != "" {
			drawText(s, lx, lblY, lbl, styles.Default)
			lastDrawnRight = lx + lw
		}
	}

	// Sub-label below X labels (if any)
	if xSubTitle != "" {
		drawText(s, (w-runewidth.StringWidth(xSubTitle))/2, lblY+1, xSubTitle, styles.GraphBorder)
	}

	// Bottom Legends (e.g. ── Japan   ··· London   --- California)
	legendY := h - 2
	var legendParts []string
	var legendStyles []tcell.Style
	for _, si := range seriesList {
		if g.Type == "BAR" || g.Type == "STACKED" {
			legendParts = append(legendParts, fmt.Sprintf("■ %s", si.label))
		} else {
			legendParts = append(legendParts, fmt.Sprintf("%c%c%c %s", si.lineCh, si.dotCh, si.lineCh, si.label))
		}
		legendStyles = append(legendStyles, si.style)
	}

	curX := boxLeft + 4
	for i, part := range legendParts {
		pw := runewidth.StringWidth(part)
		if curX+pw < w-2 {
			drawText(s, curX, legendY, part, legendStyles[i])
			curX += pw + 4
		}
	}

	// Bottom Prompt
	bottomPrompt := " [S: Save PNG Image (1280x720) | ESC/Enter: Return] "
	drawText(s, (w-runewidth.StringWidth(bottomPrompt))/2, h-1, bottomPrompt, styles.Status)

	s.Show()

	for {
		ev := s.PollEvent()
		if keyEv, ok := ev.(*tcell.EventKey); ok {
			if keyEv.Key() == tcell.KeyRune && (keyEv.Rune() == 's' || keyEv.Rune() == 'S') {
				fn := DefaultPNGFilename(sh, currFile)
				if err := ExportGraphPNG(sh, fn, 1280, 720); err == nil {
					msg := fmt.Sprintf(" Saved '%s' (1280x720 PNG) ", fn)
					drawText(s, (w-runewidth.StringWidth(msg))/2, h-1, msg, styles.Header)
					s.Show()
				} else {
					msg := fmt.Sprintf(" Error saving PNG: %v ", err)
					drawText(s, (w-runewidth.StringWidth(msg))/2, h-1, msg, styles.Error)
					s.Show()
				}
				continue
			}
			return
		}
		if _, ok := ev.(*tcell.EventResize); ok {
			s.Sync()
			RenderGraphScreen(s, sh, styles, currFile)
			return
		}
	}
}

func drawLine(s tcell.Screen, x0, y0, x1, y1, minX, maxX, minY, maxY int, ch rune, style tcell.Style) {
	dx := int(math.Abs(float64(x1 - x0)))
	dy := int(math.Abs(float64(y1 - y0)))
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy

	for {
		if x0 >= minX && x0 <= maxX && y0 >= minY && y0 <= maxY {
			s.SetContent(x0, y0, ch, nil, style)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func drawText(s tcell.Screen, x, y int, text string, style tcell.Style) {
	w, h := s.Size()
	if y < 0 || y >= h {
		return
	}
	curX := x
	for _, r := range text {
		rw := runewidth.RuneWidth(r)
		if curX < 0 {
			curX += rw
			continue
		}
		if curX+rw > w {
			break
		}
		s.SetContent(curX, y, r, nil, style)
		curX += rw
	}
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	}
	return 0, false
}

func isNonNumericText(v any) bool {
	if v == nil {
		return false
	}
	s := strings.TrimSpace(fmt.Sprintf("%v", v))
	if s == "" {
		return false
	}
	_, ok := toFloat(v)
	return !ok
}

func RenderGraphStatusScreen(s tcell.Screen, sh *sheet.Sheet, styles Styles) {
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
	keyStyle := styles.Header

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

	title := " GRAPH SETTINGS & PARAMETERS "
	drawTextFast(s, modalX+(modalW-len(title))/2, modalY, title, boxStyle, w)

	g := sh.Graph()
	gType := g.Type
	if gType == "" {
		gType = "LINE"
	}
	gTitle := g.Title
	if gTitle == "" {
		gTitle = "(none)"
	}

	lineY := modalY + 2
	drawTextFast(s, modalX+4, lineY, fmt.Sprintf("Graph Type   : %-10s  (/CTL, /CTB, /CTS, /CTP)", gType), itemStyle, modalX+modalW-2)
	lineY++
	drawTextFast(s, modalX+4, lineY, fmt.Sprintf("Graph Title  : %s", gTitle), itemStyle, modalX+modalW-2)
	lineY += 2

	xRangeStr := "(none)"
	if g.RangeX != nil {
		xRangeStr = g.RangeX.String()
	}
	drawTextFast(s, modalX+4, lineY, fmt.Sprintf("X-Axis Range : %-15s (/CX)", xRangeStr), keyStyle, modalX+modalW-2)
	lineY++

	seriesKeys := []string{"A", "B", "C", "D", "E", "F"}
	seriesColors := []string{"Yellow", "Cyan", "Green", "Magenta", "Red", "Blue"}

	for idx, sKey := range seriesKeys {
		rRef := g.Series[sKey]
		rStr := "(none)"
		legendStr := ""
		if rRef != nil {
			rStr = rRef.String()
			cells := rRef.Cells()
			if len(cells) > 0 {
				firstVal := sh.GetCellValue(cells[0].Col, cells[0].Row)
				firstCell := sh.GetCell(cells[0].Col, cells[0].Row)
				if (firstCell != nil && firstCell.Type == cell.TypeLabel) || isNonNumericText(firstVal) {
					legendStr = fmt.Sprintf(" [Label: %v]", firstVal)
				} else if rRef.MinRow() == rRef.MaxRow() && rRef.MinCol() > 0 {
					// Check cell to the left
					leftVal := sh.GetCellValue(rRef.MinCol()-1, rRef.MinRow())
					leftCell := sh.GetCell(rRef.MinCol()-1, rRef.MinRow())
					if (leftCell != nil && leftCell.Type == cell.TypeLabel) || isNonNumericText(leftVal) {
						legendStr = fmt.Sprintf(" [Label: %v]", leftVal)
					}
				} else if rRef.MinCol() == rRef.MaxCol() && rRef.MinRow() > 0 {
					// Check cell above
					topVal := sh.GetCellValue(rRef.MinCol(), rRef.MinRow()-1)
					topCell := sh.GetCell(rRef.MinCol(), rRef.MinRow()-1)
					if (topCell != nil && topCell.Type == cell.TypeLabel) || isNonNumericText(topVal) {
						legendStr = fmt.Sprintf(" [Label: %v]", topVal)
					}
				}
			}
		}
		colorName := seriesColors[idx]
		drawTextFast(s, modalX+4, lineY, fmt.Sprintf("Series %s (%-7s): %-15s (/C%s)%s", sKey, colorName, rStr, sKey, legendStr), itemStyle, modalX+modalW-2)
		lineY++
	}

	footer := " [Press any key or ESC to return to Worksheet] "
	drawTextFast(s, modalX+(modalW-runewidth.StringWidth(footer))/2, modalY+modalH-1, footer, styles.Status, w)

	s.Show()
	for {
		ev := s.PollEvent()
		if _, ok := ev.(*tcell.EventKey); ok {
			return
		}
		if _, ok := ev.(*tcell.EventResize); ok {
			s.Sync()
			RenderGraphStatusScreen(s, sh, styles)
			return
		}
	}
}

func renderPieGraph(s tcell.Screen, sh *sheet.Sheet, g *sheet.GraphConfig, seriesList []seriesInfo, xLabels []string, styles Styles, w, h, boxLeft, boxRight, boxTop, boxBottom int) {
	plotLeft := boxLeft + 2
	plotRight := boxRight - 2
	plotTop := boxTop + 1
	plotBottom := boxBottom - 1
	plotWidth := plotRight - plotLeft + 1
	plotHeight := plotBottom - plotTop + 1

	if plotWidth < 10 || plotHeight < 5 {
		return
	}

	// Series A provides the pie slice values
	if len(seriesList) == 0 || len(seriesList[0].values) == 0 {
		drawText(s, plotLeft+2, plotTop+2, "No data in Series A for Pie Chart (Use /CA to set Series A)", styles.Error)
		return
	}

	vals := seriesList[0].values
	type sliceItem struct {
		label      string
		val        float64
		pct        float64
		startAngle float64
		endAngle   float64
		style      tcell.Style
		char       rune
	}

	palette := []struct {
		style tcell.Style
		char  rune
	}{
		{styles.GraphSeriesA, '█'}, // Yellow solid
		{styles.GraphSeriesB, '█'}, // Cyan solid
		{styles.GraphSeriesC, '█'}, // Green solid
		{styles.GraphSeriesD, '█'}, // Magenta solid
		{styles.GraphSeriesE, '█'}, // Red solid
		{styles.GraphSeriesF, '█'}, // Blue solid
		{styles.GraphSeriesA, '▓'}, // Yellow dark shade
		{styles.GraphSeriesB, '▓'}, // Cyan dark shade
		{styles.GraphSeriesC, '▓'}, // Green dark shade
		{styles.GraphSeriesD, '▓'}, // Magenta dark shade
		{styles.Default, '▒'},      // White/Gray medium shade
		{styles.Status, '░'},       // Light shade
	}

	total := 0.0
	for _, v := range vals {
		if v > 0 {
			total += v
		}
	}

	if total <= 0 {
		drawText(s, plotLeft+2, plotTop+2, "All Series A values are <= 0 for Pie Chart", styles.Error)
		return
	}

	var slices []sliceItem
	curAngle := -math.Pi / 2.0 // Start at 12 o'clock

	for i, v := range vals {
		if v <= 0 {
			continue
		}
		lbl := fmt.Sprintf("#%d", i+1)
		if i < len(xLabels) && strings.TrimSpace(xLabels[i]) != "" {
			lbl = strings.TrimSpace(xLabels[i])
		}
		pct := (v / total) * 100.0
		span := (v / total) * 2.0 * math.Pi

		palIdx := len(slices) % len(palette)
		slices = append(slices, sliceItem{
			label:      lbl,
			val:        v,
			pct:        pct,
			startAngle: curAngle,
			endAngle:   curAngle + span,
			style:      palette[palIdx].style,
			char:       palette[palIdx].char,
		})
		curAngle += span
	}

	if len(slices) == 0 {
		return
	}

	// Terminal aspect ratio: row height / col width ~ 2.05
	aspect := 2.05

	ry := float64(plotHeight-2) / 2.0
	if ry > 12.0 {
		ry = 12.0
	}
	if ry < 3.0 {
		ry = 3.0
	}
	rx := ry * aspect

	// Check if legend fits on the right
	legendWidth := 34
	for _, sl := range slices {
		lw := runewidth.StringWidth(sl.label) + 20
		if lw > legendWidth {
			legendWidth = lw
		}
	}
	if legendWidth > 45 {
		legendWidth = 45
	}

	var cx, cy int
	var legendLeft int
	if plotWidth >= int(rx*2)+legendWidth+6 {
		// Pie on Left, Legend on Right
		cx = plotLeft + int(rx) + 2
		cy = plotTop + plotHeight/2
		legendLeft = cx + int(rx) + 4
	} else {
		// Centered Pie
		cx = plotLeft + plotWidth/2
		cy = plotTop + plotHeight/2
		legendLeft = plotRight + 10 // hide or overflow
	}

	// Draw the Pie Slices
	minX := cx - int(rx) - 1
	maxX := cx + int(rx) + 1
	minY := cy - int(ry) - 1
	maxY := cy + int(ry) + 1

	if minX < plotLeft {
		minX = plotLeft
	}
	if maxX > plotRight {
		maxX = plotRight
	}
	if minY < plotTop {
		minY = plotTop
	}
	if maxY > plotBottom {
		maxY = plotBottom
	}

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float64(x-cx) / rx
			dy := float64(y-cy) / ry
			dist := dx*dx + dy*dy
			if dist <= 1.0 {
				ang := math.Atan2(float64(y-cy), float64(x-cx)*(ry/rx))
				for ang < -math.Pi/2.0 {
					ang += 2.0 * math.Pi
				}
				for ang >= 3.0*math.Pi/2.0 {
					ang -= 2.0 * math.Pi
				}

				// Find corresponding slice
				var chosen sliceItem
				found := false
				for _, sl := range slices {
					if ang >= sl.startAngle && ang < sl.endAngle {
						chosen = sl
						found = true
						break
					}
				}
				if !found && len(slices) > 0 {
					chosen = slices[len(slices)-1]
				}

				s.SetContent(x, y, chosen.char, nil, chosen.style)
			}
		}
	}

	// Draw Legend on the Right
	if legendLeft <= plotRight-10 {
		maxItems := plotHeight - 3
		legendTop := cy - len(slices)/2
		if legendTop < plotTop+1 {
			legendTop = plotTop + 1
		}
		if legendTop+len(slices)+1 > plotBottom {
			legendTop = plotBottom - len(slices) - 1
			if legendTop < plotTop+1 {
				legendTop = plotTop + 1
			}
		}

		// Legend Header
		header := fmt.Sprintf("TOTAL: %s (100.0%%)", formatNumberAuto(total))
		drawText(s, legendLeft, legendTop-1, header, styles.GraphBorder)

		for i, sl := range slices {
			if i >= maxItems {
				rem := len(slices) - i
				drawText(s, legendLeft, legendTop+i, fmt.Sprintf("... (%d more items)", rem), styles.Default)
				break
			}
			rowY := legendTop + i
			// Draw badge: [█] in slice style
			s.SetContent(legendLeft, rowY, '[', nil, styles.Default)
			s.SetContent(legendLeft+1, rowY, sl.char, nil, sl.style)
			s.SetContent(legendLeft+2, rowY, ']', nil, styles.Default)

			// Label & Value & Percentage
			lblText := sl.label
			if runewidth.StringWidth(lblText) > 16 {
				lblText = runewidth.Truncate(lblText, 14, "..")
			}
			line := fmt.Sprintf(" %-16s: %10s (%5.1f%%)", lblText, formatNumberAuto(sl.val), sl.pct)
			drawText(s, legendLeft+3, rowY, line, styles.Default)
		}
	}
}

func formatNumberAuto(v float64) string {
	if math.Abs(v-math.Round(v)) < 1e-6 {
		return formatWithCommas(int64(math.Round(v)))
	}
	return fmt.Sprintf("%.2f", v)
}

func formatAxisTick(val float64) string {
	abs := math.Abs(val)
	switch {
	case abs >= 1e9:
		return strings.TrimSpace(fmt.Sprintf("%.1fB", val/1e9))
	case abs >= 1e6:
		return strings.TrimSpace(fmt.Sprintf("%.1fM", val/1e6))
	case abs >= 10000:
		return strings.TrimSpace(fmt.Sprintf("%.1fK", val/1e3))
	default:
		return strings.TrimSpace(fmt.Sprintf("%.1f", val))
	}
}

// FormatAxisTickForTest exposes formatAxisTick for regression tests.
func FormatAxisTickForTest(val float64) string {
	return formatAxisTick(val)
}

func formatWithCommas(n int64) string {
	in := fmt.Sprintf("%d", n)
	sign := ""
	if in[0] == '-' {
		sign = "-"
		in = in[1:]
	}
	if len(in) <= 3 {
		return sign + in
	}
	var res []rune
	for i, r := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			res = append(res, ',')
		}
		res = append(res, r)
	}
	return sign + string(res)
}

// FormatWithCommasForTest exposes formatWithCommas for regression tests.
func FormatWithCommasForTest(n int64) string {
	return formatWithCommas(n)
}
