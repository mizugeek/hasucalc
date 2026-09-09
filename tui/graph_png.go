package tui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"

	"hasucalc/cell"
	"hasucalc/sheet"
)

type pngSeriesData struct {
	name   string
	label  string
	values []float64
	col    color.RGBA
}

func drawStringPNG(img *image.RGBA, x, y int, text string, col color.RGBA, sizePt int) {
	DrawMultiScriptText(img, x, y, text, col, sizePt)
}

func measureStringPNG(text string, sizePt int) int {
	return MeasureMultiScriptText(text, sizePt)
}

// ExportGraphPNG exports the active worksheet's graph configuration to a standalone PNG image file.
// Default dimensions: 1280 x 720 px (HD).
func ExportGraphPNG(sh *sheet.Sheet, filename string, width, height int) (err error) {
	defer func() {
		ResetScriptFontCache()
	}()

	if width <= 0 {
		width = 1280
	}
	if height <= 0 {
		height = 720
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Background
	bgColor := color.RGBA{R: 0x14, G: 0x16, B: 0x22, A: 0xFF} // Dark Slate
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bgColor}, image.Point{}, draw.Src)

	g := sh.Graph()

	seriesColors := []color.RGBA{
		{R: 0xF1, G: 0xC4, B: 0x0F, A: 0xFF}, // Series A: Vibrant Yellow
		{R: 0x00, G: 0xD2, B: 0xD3, A: 0xFF}, // Series B: Cyan
		{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}, // Series C: Emerald Green
		{R: 0xE0, G: 0x56, B: 0xFD, A: 0xFF}, // Series D: Magenta / Pink
		{R: 0xFF, G: 0x52, B: 0x52, A: 0xFF}, // Series E: Coral Red
		{R: 0x48, G: 0xDB, B: 0xFB, A: 0xFF}, // Series F: Sky Blue
	}

	var seriesList []pngSeriesData
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
					leftC := rRef.MinCol() - 1
					leftR := rRef.MinRow()
					leftCell := sh.GetCell(leftC, leftR)
					leftVal := sh.GetCellValue(leftC, leftR)
					if (leftCell != nil && leftCell.Type == cell.TypeLabel) || isNonNumericText(leftVal) {
						label = fmt.Sprintf("%v", leftVal)
					}
				} else if isVert && rRef.MinRow() > 0 {
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
				v := sh.GetCellValue(cells[i].Col, cells[i].Row)
				if num, ok := toFloat(v); ok {
					vals = append(vals, num)
				} else {
					vals = append(vals, 0.0)
				}
			}

			if len(vals) > 0 {
				seriesList = append(seriesList, pngSeriesData{
					name:   sKey,
					label:  label,
					values: vals,
					col:    seriesColors[idx%len(seriesColors)],
				})
			}
		}
	}

	maxSeriesPoints := 0
	for _, si := range seriesList {
		if len(si.values) > maxSeriesPoints {
			maxSeriesPoints = len(si.values)
		}
	}

	var xLabels []string
	if g.RangeX != nil {
		xCells := g.RangeX.Cells()
		startX := 0

		if len(xCells) > 1 && maxSeriesPoints > 0 && len(xCells) == maxSeriesPoints+1 {
			startX = 1
		}

		for i := startX; i < len(xCells); i++ {
			cr := xCells[i]
			v := sh.GetCellValue(cr.Col, cr.Row)
			if v == nil {
				xLabels = append(xLabels, "")
			} else {
				xLabels = append(xLabels, fmt.Sprintf("%v", v))
			}
		}
	}

	// Dimensions & Margins
	marginL, marginR := 40, 40
	marginT, marginB := 56, 62

	boxL := marginL
	boxR := width - marginR
	boxT := marginT
	boxB := height - marginB

	frameCol := color.RGBA{R: 0x4A, G: 0x55, B: 0x68, A: 0xFF}
	gridCol := color.RGBA{R: 0x23, G: 0x29, B: 0x3B, A: 0xFF}
	textCol := color.RGBA{R: 0xE2, G: 0xE8, B: 0xF0, A: 0xFF}
	mutedTextCol := color.RGBA{R: 0x94, G: 0xA3, B: 0xB8, A: 0xFF}

	// Title
	title := g.Title
	if title != "" {
		titleW := measureStringPNG(title, 22)
		drawStringPNG(img, (width-titleW)/2, 16, title, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, 22)
	}

	// Outer Frame
	drawRect(img, boxL, boxT, boxR-boxL, boxB-boxT, frameCol)
	drawRect(img, boxL-1, boxT-1, boxR-boxL+2, boxB-boxT+2, frameCol)

	if g.Type == "PIE" {
		renderPiePNG(img, g, seriesList, xLabels, boxL+10, boxT+10, boxR-10, boxB-10, textCol, mutedTextCol, gridCol)
	} else {
		renderCartesianPNG(img, g, seriesList, xLabels, boxL, boxT, boxR, boxB, textCol, mutedTextCol, gridCol, frameCol)
	}

	// Footer brand
	brand := "HasuCalc 2.0"
	brandW := measureStringPNG(brand, 12)
	drawStringPNG(img, (width-brandW)/2, height-24, brand, mutedTextCol, 12)

	// Save to file
	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return png.Encode(outFile, img)
}

func renderCartesianPNG(img *image.RGBA, g *sheet.GraphConfig, seriesList []pngSeriesData, xLabels []string, boxL, boxT, boxR, boxB int, textCol, mutedCol, gridCol, frameCol color.RGBA) {
	numPoints := 0
	for _, si := range seriesList {
		if len(si.values) > numPoints {
			numPoints = len(si.values)
		}
	}
	if len(xLabels) > numPoints {
		numPoints = len(xLabels)
	}

	if numPoints == 0 || len(seriesList) == 0 {
		drawStringPNG(img, boxL+30, boxT+50, "No graph data defined.", textCol, 14)
		return
	}

	minVal, maxVal := 0.0, 1.0
	if g.Type == "STACKED" {
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
		first := true
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
			minVal = 0
		}
		if minVal == maxVal {
			maxVal += 1.0
		}
	}

	plotL := boxL + 85
	plotR := boxR - 30
	plotT := boxT + 25
	plotB := boxB - 52
	plotW := plotR - plotL
	plotH := plotB - plotT

	valToY := func(val float64) int {
		frac := (val - minVal) / (maxVal - minVal)
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		return plotB - int(math.Round(frac*float64(plotH)))
	}

	numTicks := 5
	for i := 0; i <= numTicks; i++ {
		frac := float64(i) / float64(numTicks)
		y := plotB - int(math.Round(frac*float64(plotH)))
		val := minVal + frac*(maxVal-minVal)

		for gx := plotL; gx <= plotR; gx += 4 {
			img.Set(gx, y, gridCol)
			img.Set(gx+1, y, gridCol)
		}

		lbl := fmt.Sprintf("%.1f", val)
		if math.Abs(val-math.Round(val)) < 1e-6 {
			lbl = formatWithCommas(int64(math.Round(val)))
		}
		lw := measureStringPNG(lbl, 14)
		drawStringPNG(img, plotL-lw-12, y-7, lbl, mutedCol, 14)
	}

	drawLinePNG(img, plotL, plotB, plotR, plotB, frameCol, 1)
	drawLinePNG(img, plotL, plotT, plotL, plotB, frameCol, 1)

	stepX := float64(plotW) / float64(numPoints)
	valToX := func(ptIdx int) int {
		return plotL + int(math.Round((float64(ptIdx)+0.5)*stepX))
	}

	switch g.Type {
	case "BAR":
		numS := len(seriesList)
		groupW := int(stepX * 0.82)
		if groupW < numS {
			groupW = numS
		}
		subW := groupW / numS
		if subW < 1 {
			subW = 1
		}
		actualGroupW := subW * numS

		for ptIdx := 0; ptIdx < numPoints; ptIdx++ {
			cX := valToX(ptIdx)
			startX := cX - actualGroupW/2

			for sIdx, si := range seriesList {
				if ptIdx >= len(si.values) {
					continue
				}
				v := si.values[ptIdx]
				yTop := valToY(v)
				bx := startX + sIdx*subW
				bw := subW - 1
				if bw < 1 {
					bw = 1
				}
				fillRect(img, bx, yTop, bw, plotB-yTop, si.col)
			}
		}

	case "STACKED":
		barW := int(stepX * 0.7)
		if barW < 2 {
			barW = 2
		}
		if barW > 54 {
			barW = 54
		}

		for ptIdx := 0; ptIdx < numPoints; ptIdx++ {
			cX := valToX(ptIdx)
			bx := cX - barW/2
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
				fillRect(img, bx, yTop, barW, yBot-yTop, si.col)
			}
		}

	default: // "LINE"
		for _, si := range seriesList {
			var pts [][2]int
			for ptIdx := 0; ptIdx < len(si.values); ptIdx++ {
				px := valToX(ptIdx)
				py := valToY(si.values[ptIdx])
				pts = append(pts, [2]int{px, py})
			}
			for i := 0; i < len(pts)-1; i++ {
				drawLinePNG(img, pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1], si.col, 3)
			}
			for _, pt := range pts {
				fillCircle(img, pt[0], pt[1], 4, si.col)
				fillCircle(img, pt[0], pt[1], 2, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF})
			}
		}
	}

	// X-axis Ticks & Labels
	maxLblW := 1
	for _, l := range xLabels {
		w := measureStringPNG(l, 14)
		if w > maxLblW {
			maxLblW = w
		}
	}
	minSlotW := maxLblW + 20
	if minSlotW < 36 {
		minSlotW = 36
	}
	maxVisLabels := plotW / minSlotW
	if maxVisLabels < 2 {
		maxVisLabels = 2
	}

	skip := 1
	if numPoints > maxVisLabels {
		raw := float64(numPoints) / float64(maxVisLabels)
		niceSteps := []int{1, 2, 5, 10, 20, 25, 50, 100, 200, 500, 1000}
		for _, s := range niceSteps {
			if float64(s) >= raw {
				skip = s
				break
			}
		}
		if skip < int(raw) {
			skip = int(math.Ceil(raw))
		}
	}

	for ptIdx := 0; ptIdx < numPoints; ptIdx++ {
		if ptIdx%skip != 0 && ptIdx != numPoints-1 {
			continue
		}
		px := valToX(ptIdx)
		drawLinePNG(img, px, plotB, px, plotB+6, frameCol, 1)

		if ptIdx < len(xLabels) && xLabels[ptIdx] != "" {
			lbl := xLabels[ptIdx]
			lw := measureStringPNG(lbl, 14)
			drawStringPNG(img, px-lw/2, plotB+9, lbl, textCol, 14)
		}
	}

	// Legend below plot
	legendY := boxB - 26
	curLX := plotL
	for _, si := range seriesList {
		fillRect(img, curLX, legendY+2, 12, 12, si.col)
		drawStringPNG(img, curLX+18, legendY, si.label, textCol, 14)
		curLX += measureStringPNG(si.label, 14) + 40
	}
}

func renderPiePNG(img *image.RGBA, g *sheet.GraphConfig, seriesList []pngSeriesData, xLabels []string, boxL, boxT, boxR, boxB int, textCol, mutedCol, gridCol color.RGBA) {
	if len(seriesList) == 0 || len(seriesList[0].values) == 0 {
		drawStringPNG(img, boxL+30, boxT+50, "No Series A data for Pie Chart.", textCol, 14)
		return
	}

	vals := seriesList[0].values
	total := 0.0
	for _, v := range vals {
		if v > 0 {
			total += v
		}
	}
	if total <= 0 {
		drawStringPNG(img, boxL+30, boxT+50, "All Series A values are <= 0 for Pie Chart.", textCol, 14)
		return
	}

	palette := []color.RGBA{
		{R: 0xF1, G: 0xC4, B: 0x0F, A: 0xFF}, // Yellow
		{R: 0x00, G: 0xD2, B: 0xD3, A: 0xFF}, // Cyan
		{R: 0x2E, G: 0xCC, B: 0x71, A: 0xFF}, // Emerald
		{R: 0xE0, G: 0x56, B: 0xFD, A: 0xFF}, // Magenta
		{R: 0xFF, G: 0x52, B: 0x52, A: 0xFF}, // Coral Red
		{R: 0x48, G: 0xDB, B: 0xFB, A: 0xFF}, // Sky Blue
		{R: 0xFA, G: 0x82, B: 0x31, A: 0xFF}, // Orange
		{R: 0xA5, G: 0x5E, B: 0xEA, A: 0xFF}, // Purple
		{R: 0x20, G: 0xBF, B: 0x6B, A: 0xFF}, // Mint
		{R: 0xFD, G: 0x96, B: 0x44, A: 0xFF}, // Peach
	}

	type sliceData struct {
		label      string
		val        float64
		pct        float64
		startAngle float64
		endAngle   float64
		col        color.RGBA
	}

	var slices []sliceData
	curAngle := -math.Pi / 2.0

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

		slices = append(slices, sliceData{
			label:      lbl,
			val:        v,
			pct:        pct,
			startAngle: curAngle,
			endAngle:   curAngle + span,
			col:        palette[len(slices)%len(palette)],
		})
		curAngle += span
	}

	plotH := boxB - boxT
	radius := float64(plotH)/2.0 - 25.0
	if radius > 230 {
		radius = 230
	}
	cx := float64(boxL) + radius + 60.0
	cy := float64(boxT) + float64(plotH)/2.0

	// Draw Slices
	minX := int(cx - radius - 1)
	maxX := int(cx + radius + 1)
	minY := int(cy - radius - 1)
	maxY := int(cy + radius + 1)

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist <= radius {
				ang := math.Atan2(dy, dx)
				for ang < -math.Pi/2.0 {
					ang += 2.0 * math.Pi
				}
				for ang >= 3.0*math.Pi/2.0 {
					ang -= 2.0 * math.Pi
				}

				for _, sl := range slices {
					if ang >= sl.startAngle && ang < sl.endAngle {
						img.Set(x, y, sl.col)
						break
					}
				}
			}
		}
	}

	// Draw Legend on Right
	legendL := int(cx + radius + 60.0)
	legendT := int(cy) - (len(slices)*32)/2
	if legendT < boxT+20 {
		legendT = boxT + 20
	}

	// Total Header
	totalHdr := fmt.Sprintf("TOTAL: %s (100.0%%)", formatNumberAuto(total))
	drawStringPNG(img, legendL, legendT-28, totalHdr, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, 16)

	for i, sl := range slices {
		rowY := legendT + i*32
		fillRect(img, legendL, rowY+2, 14, 14, sl.col)

		line := fmt.Sprintf("%-10s : %8s ( %4.1f%% )", sl.label, formatNumberAuto(sl.val), sl.pct)
		drawStringPNG(img, legendL+24, rowY, line, textCol, 14)
	}
}

func fillRect(img *image.RGBA, x, y, w, h int, col color.RGBA) {
	for rY := y; rY < y+h; rY++ {
		for rX := x; rX < x+w; rX++ {
			if rX >= 0 && rX < img.Bounds().Dx() && rY >= 0 && rY < img.Bounds().Dy() {
				img.Set(rX, rY, col)
			}
		}
	}
}

func drawRect(img *image.RGBA, x, y, w, h int, col color.RGBA) {
	for rX := x; rX < x+w; rX++ {
		img.Set(rX, y, col)
		img.Set(rX, y+h-1, col)
	}
	for rY := y; rY < y+h; rY++ {
		img.Set(x, rY, col)
		img.Set(x+w-1, rY, col)
	}
}

func fillCircle(img *image.RGBA, cx, cy, r int, col color.RGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= r*r {
				if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
					img.Set(x, y, col)
				}
			}
		}
	}
}

func drawLinePNG(img *image.RGBA, x0, y0, x1, y1 int, col color.RGBA, thickness int) {
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
		if thickness <= 1 {
			if x0 >= 0 && x0 < img.Bounds().Dx() && y0 >= 0 && y0 < img.Bounds().Dy() {
				img.Set(x0, y0, col)
			}
		} else {
			fillCircle(img, x0, y0, thickness/2, col)
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
