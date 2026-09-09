package sheet

import (
	"fmt"
	"strings"

	"hasucalc/coord"
	"hasucalc/formula"
)

const hasuCalcBaseCommentPrefix = "hasucalc:base="

func hasuCalcBaseComment(ne formula.NamedExpr) string {
	if !ne.HasBase {
		return ""
	}
	return hasuCalcBaseCommentPrefix + coord.CellRef{Col: ne.BaseCol, Row: ne.BaseRow}.String()
}

func parseHasuCalcBaseComment(comment string) (col, row int, ok bool) {
	c := strings.TrimSpace(comment)
	lower := strings.ToLower(c)
	if !strings.HasPrefix(lower, hasuCalcBaseCommentPrefix) {
		return 0, 0, false
	}
	rest := strings.TrimSpace(c[len(hasuCalcBaseCommentPrefix):])
	cr, err := coord.ParseCellRef(rest)
	if err != nil {
		return 0, 0, false
	}
	return cr.Col, cr.Row, true
}

func applyHasuCalcBaseComment(parsed any, comment string) any {
	col, row, ok := parseHasuCalcBaseComment(comment)
	if !ok || parsed == nil {
		return parsed
	}
	switch t := parsed.(type) {
	case formula.NamedExpr:
		t.HasBase = true
		t.BaseCol = col
		t.BaseRow = row
		return t
	case coord.CellRef:
		expr := t.String()
		if t.Sheet == "" {
			expr = coord.CellRef{Col: t.Col, Row: t.Row, ColAbs: t.ColAbs, RowAbs: t.RowAbs}.String()
		}
		return formula.NamedExpr{Expr: "=" + expr, HasBase: true, BaseCol: col, BaseRow: row}
	case coord.RangeRef:
		copyRef := t
		return formula.NamedExpr{Expr: "=" + copyRef.String(), HasBase: true, BaseCol: col, BaseRow: row}
	default:
		return parsed
	}
}

func encodeNamedValue(v any) any {
	switch t := v.(type) {
	case formula.NamedExpr:
		if t.HasBase {
			base := coord.CellRef{Col: t.BaseCol, Row: t.BaseRow}.String()
			return map[string]any{"expr": t.Expr, "base": base}
		}
		return t.Expr
	default:
		return fmt.Sprintf("%v", v)
	}
}

func decodeNamedValue(v any) any {
	switch t := v.(type) {
	case string:
		return decodeNamedString(t)
	case map[string]any:
		expr, _ := t["expr"].(string)
		base, _ := t["base"].(string)
		if expr == "" {
			return nil
		}
		ne := formula.NamedExpr{Expr: expr}
		if base != "" {
			if cr, err := coord.ParseCellRef(base); err == nil {
				ne.HasBase = true
				ne.BaseCol = cr.Col
				ne.BaseRow = cr.Row
			}
		}
		return ne
	default:
		return decodeNamedString(fmt.Sprintf("%v", v))
	}
}

func decodeNamedString(rStr string) any {
	rStr = strings.TrimSpace(rStr)
	if r, err := coord.ParseRangeRef(rStr); err == nil {
		return r
	}
	if cr, err := coord.ParseCellRef(rStr); err == nil {
		return cr
	}
	if strings.HasPrefix(rStr, "=") || strings.HasPrefix(rStr, "@") {
		return formula.NamedExpr{Expr: rStr}
	}
	return formula.NamedExpr{Expr: "=" + rStr}
}

func encodeNamedMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for n, v := range src {
		out[n] = encodeNamedValue(v)
	}
	return out
}

func decodeNamedMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for n, v := range src {
		if decoded := decodeNamedValue(v); decoded != nil {
			out[strings.ToUpper(n)] = decoded
		}
	}
	return out
}

// RetargetFormulasAfterMove rewrites formulas and names that pointed into fromR so they follow a move.
func (s *Sheet) RetargetFormulasAfterMove(fromR coord.RangeRef, dCol, dRow int) {
	s.RetargetFormulasAfterMoveTo(fromR, dCol, dRow, s.Name())
}

// RetargetFormulasAfterMoveTo rewrites refs that pointed into fromR on this sheet so they follow a move to destSheet.
func (s *Sheet) RetargetFormulasAfterMoveTo(fromR coord.RangeRef, dCol, dRow int, destSheet string) {
	s.retargetAllFormulas(fromR, dCol, dRow, destSheet, true)
}

func (s *Sheet) retargetAllFormulas(fromR coord.RangeRef, dCol, dRow int, destSheet string, moveGraph bool) {
	if destSheet == "" {
		destSheet = s.Name()
	}
	if dCol == 0 && dRow == 0 && strings.EqualFold(destSheet, s.Name()) {
		return
	}
	edited := s.Name()
	onDest := strings.EqualFold(edited, destSheet)
	s.applyFormulaAdjust(func(raw string) string {
		return formula.RetargetFormulaReferencesToSheet(raw, fromR, edited, true, dCol, dRow, destSheet, onDest)
	})
	s.retargetNamedMap(s.namedRanges, fromR, edited, true, dCol, dRow, destSheet, onDest)
	if moveGraph {
		s.retargetGraphAfterMove(fromR, dCol, dRow, edited, destSheet)
	}
	if s.workbook != nil {
		isPrimary := len(s.workbook.Sheets) > 0 && s.workbook.Sheets[0] == s
		s.retargetNamedMap(s.workbook.NamedRanges, fromR, edited, isPrimary, dCol, dRow, destSheet, false)
		for _, other := range s.workbook.Sheets {
			if other == s {
				continue
			}
			otherOnDest := strings.EqualFold(other.Name(), destSheet)
			other.applyFormulaAdjust(func(raw string) string {
				return formula.RetargetFormulaReferencesToSheet(raw, fromR, edited, false, dCol, dRow, destSheet, otherOnDest)
			})
			other.retargetNamedMap(other.namedRanges, fromR, edited, false, dCol, dRow, destSheet, otherOnDest)
			if moveGraph {
				other.retargetGraphAfterMove(fromR, dCol, dRow, edited, destSheet)
			}
			other.modified = true
		}
	}
}

func (s *Sheet) RetargetAfterTransposeMove(fromR coord.RangeRef, destTopC, destTopR int, destSheet string) {
	if destSheet == "" {
		destSheet = s.Name()
	}
	s.retargetGraphAfterTranspose(fromR, destTopC, destTopR, s.Name(), destSheet)
	if s.workbook != nil {
		for _, other := range s.workbook.Sheets {
			if other != nil && other != s {
				other.retargetGraphAfterTranspose(fromR, destTopC, destTopR, s.Name(), destSheet)
			}
		}
	}
	minC, minR := fromR.MinCol(), fromR.MinRow()
	for r := fromR.MinRow(); r <= fromR.MaxRow(); r++ {
		for c := fromR.MinCol(); c <= fromR.MaxCol(); c++ {
			destC := destTopC + (r - minR)
			destR := destTopR + (c - minC)
			cellR := coord.RangeRef{
				Start: coord.CellRef{Col: c, Row: r},
				End:   coord.CellRef{Col: c, Row: r},
			}
			s.retargetAllFormulas(cellR, destC-c, destR-r, destSheet, false)
		}
	}
}

func (wb *Workbook) rewriteNamesForSheetRename(oldName, newName string) {
	if wb == nil {
		return
	}
	rewriteNamedMapRename(wb.NamedRanges, oldName, newName)
	for _, sh := range wb.Sheets {
		if sh != nil {
			rewriteNamedMapRename(sh.namedRanges, oldName, newName)
			rewriteGraphRename(&sh.graph, oldName, newName)
		}
	}
}

func (wb *Workbook) rewriteNamesForSheetDelete(deletedName string) {
	if wb == nil {
		return
	}
	rewriteNamedMapInvalidate(wb.NamedRanges, deletedName)
	for _, sh := range wb.Sheets {
		if sh != nil {
			rewriteNamedMapInvalidate(sh.namedRanges, deletedName)
			rewriteGraphInvalidate(&sh.graph, deletedName)
		}
	}
}

func rewriteNamedMapRename(m map[string]any, oldName, newName string) {
	if m == nil {
		return
	}
	for k, v := range m {
		m[k] = renameNamedValue(v, oldName, newName)
	}
}

func rewriteNamedMapInvalidate(m map[string]any, deletedName string) {
	if m == nil {
		return
	}
	for k, v := range m {
		if nv, keep := invalidateNamedValue(v, deletedName); keep {
			m[k] = nv
		} else {
			delete(m, k)
		}
	}
}

func renameNamedValue(v any, oldName, newName string) any {
	switch t := v.(type) {
	case coord.CellRef:
		if strings.EqualFold(t.Sheet, oldName) {
			t.Sheet = newName
		}
		return t
	case coord.RangeRef:
		if strings.EqualFold(t.Sheet, oldName) {
			t.Sheet = newName
		}
		if strings.EqualFold(t.Start.Sheet, oldName) {
			t.Start.Sheet = newName
		}
		if strings.EqualFold(t.End.Sheet, oldName) {
			t.End.Sheet = newName
		}
		return t
	case formula.NamedExpr:
		t.Expr = formula.RenameSheetReferences(t.Expr, oldName, newName)
		return t
	default:
		return v
	}
}

func invalidateNamedValue(v any, deletedName string) (any, bool) {
	switch t := v.(type) {
	case coord.CellRef:
		if strings.EqualFold(t.Sheet, deletedName) {
			return nil, false
		}
		return t, true
	case coord.RangeRef:
		sheetName := t.Sheet
		if sheetName == "" {
			sheetName = t.Start.Sheet
		}
		if strings.EqualFold(sheetName, deletedName) || strings.EqualFold(t.Start.Sheet, deletedName) || strings.EqualFold(t.End.Sheet, deletedName) {
			return nil, false
		}
		return t, true
	case formula.NamedExpr:
		t.Expr = formula.InvalidateSheetReferences(t.Expr, deletedName)
		return t, true
	default:
		return v, true
	}
}

func rewriteGraphRename(g *GraphConfig, oldName, newName string) {
	if g == nil {
		return
	}
	g.RangeX = renameGraphRange(g.RangeX, oldName, newName)
	for k, v := range g.Series {
		g.Series[k] = renameGraphRange(v, oldName, newName)
	}
}

func rewriteGraphInvalidate(g *GraphConfig, deletedName string) {
	if g == nil {
		return
	}
	g.RangeX = invalidateGraphRange(g.RangeX, deletedName)
	for k, v := range g.Series {
		inv := invalidateGraphRange(v, deletedName)
		if inv == nil {
			delete(g.Series, k)
		} else {
			g.Series[k] = inv
		}
	}
}

func renameGraphRange(r *coord.RangeRef, oldName, newName string) *coord.RangeRef {
	if r == nil {
		return nil
	}
	out := *r
	if strings.EqualFold(out.Sheet, oldName) {
		out.Sheet = newName
	}
	if strings.EqualFold(out.Start.Sheet, oldName) {
		out.Start.Sheet = newName
	}
	if strings.EqualFold(out.End.Sheet, oldName) {
		out.End.Sheet = newName
	}
	return &out
}

func invalidateGraphRange(r *coord.RangeRef, deletedName string) *coord.RangeRef {
	if r == nil {
		return nil
	}
	if strings.EqualFold(r.Sheet, deletedName) || strings.EqualFold(r.Start.Sheet, deletedName) || strings.EqualFold(r.End.Sheet, deletedName) {
		return nil
	}
	return r
}

func graphRangeSheet(r *coord.RangeRef, ownerSheet string) string {
	if r == nil {
		return ownerSheet
	}
	if r.Sheet != "" {
		return r.Sheet
	}
	if r.Start.Sheet != "" {
		return r.Start.Sheet
	}
	return ownerSheet
}

func (s *Sheet) retargetGraphAfterMove(fromR coord.RangeRef, dCol, dRow int, srcSheet, destSheet string) {
	s.graph.RangeX = retargetGraphRange(s.graph.RangeX, fromR, dCol, dRow, srcSheet, destSheet, s.Name())
	for k, v := range s.graph.Series {
		s.graph.Series[k] = retargetGraphRange(v, fromR, dCol, dRow, srcSheet, destSheet, s.Name())
	}
}

func retargetGraphRange(r *coord.RangeRef, fromR coord.RangeRef, dCol, dRow int, srcSheet, destSheet, ownerSheet string) *coord.RangeRef {
	if r == nil {
		return nil
	}
	if !strings.EqualFold(graphRangeSheet(r, ownerSheet), srcSheet) {
		return r
	}
	if !fromR.Contains(r.Start.Col, r.Start.Row) || !fromR.Contains(r.End.Col, r.End.Row) {
		return r
	}
	out := *r
	out.Start.Col += dCol
	out.Start.Row += dRow
	out.End.Col += dCol
	out.End.Row += dRow
	if !strings.EqualFold(srcSheet, destSheet) {
		if strings.EqualFold(ownerSheet, destSheet) {
			out.Sheet = ""
			out.Start.Sheet = ""
			out.End.Sheet = ""
		} else {
			out.Sheet = destSheet
			out.Start.Sheet = destSheet
			out.End.Sheet = destSheet
		}
	}
	return &out
}

func (s *Sheet) retargetGraphAfterTranspose(fromR coord.RangeRef, destTopC, destTopR int, srcSheet, destSheet string) {
	s.graph.RangeX = retargetGraphRangeTranspose(s.graph.RangeX, fromR, destTopC, destTopR, srcSheet, destSheet, s.Name())
	for k, v := range s.graph.Series {
		s.graph.Series[k] = retargetGraphRangeTranspose(v, fromR, destTopC, destTopR, srcSheet, destSheet, s.Name())
	}
}

func retargetGraphRangeTranspose(r *coord.RangeRef, fromR coord.RangeRef, destTopC, destTopR int, srcSheet, destSheet, ownerSheet string) *coord.RangeRef {
	if r == nil {
		return nil
	}
	if !strings.EqualFold(graphRangeSheet(r, ownerSheet), srcSheet) {
		return r
	}
	if !fromR.Contains(r.Start.Col, r.Start.Row) || !fromR.Contains(r.End.Col, r.End.Row) {
		return r
	}
	minC, minR := fromR.MinCol(), fromR.MinRow()
	sc, sr := destTopC+(r.Start.Row-minR), destTopR+(r.Start.Col-minC)
	ec, er := destTopC+(r.End.Row-minR), destTopR+(r.End.Col-minC)
	out := *r
	out.Start.Col, out.Start.Row = sc, sr
	out.End.Col, out.End.Row = ec, er
	if !strings.EqualFold(srcSheet, destSheet) {
		if strings.EqualFold(ownerSheet, destSheet) {
			out.Sheet = ""
			out.Start.Sheet = ""
			out.End.Sheet = ""
		} else {
			out.Sheet = destSheet
			out.Start.Sheet = destSheet
			out.End.Sheet = destSheet
		}
	}
	return &out
}

func refBelongsToSheet(refSheet string, onEditedSheet bool, editedSheet string) bool {
	if refSheet != "" {
		return strings.EqualFold(refSheet, editedSheet)
	}
	return onEditedSheet
}

func (s *Sheet) retargetNamedMap(m map[string]any, fromR coord.RangeRef, editedSheet string, onEditedSheet bool, dCol, dRow int, destSheet string, onDestSheet bool) {
	if m == nil {
		return
	}
	for k, v := range m {
		m[k] = retargetNamedValue(v, fromR, editedSheet, onEditedSheet, dCol, dRow, destSheet, onDestSheet)
	}
}

func retargetNamedValue(v any, fromR coord.RangeRef, editedSheet string, onEditedSheet bool, dCol, dRow int, destSheet string, onDestSheet bool) any {
	switch t := v.(type) {
	case coord.CellRef:
		if refBelongsToSheet(t.Sheet, onEditedSheet, editedSheet) && fromR.Contains(t.Col, t.Row) {
			t.Col += dCol
			t.Row += dRow
			if !strings.EqualFold(editedSheet, destSheet) {
				if onDestSheet {
					t.Sheet = ""
				} else {
					t.Sheet = destSheet
				}
			}
		}
		return t
	case coord.RangeRef:
		refSheet := t.Sheet
		if refSheet == "" {
			refSheet = t.Start.Sheet
		}
		if refBelongsToSheet(refSheet, onEditedSheet, editedSheet) &&
			fromR.Contains(t.Start.Col, t.Start.Row) && fromR.Contains(t.End.Col, t.End.Row) {
			t.Start.Col += dCol
			t.Start.Row += dRow
			t.End.Col += dCol
			t.End.Row += dRow
			if !strings.EqualFold(editedSheet, destSheet) {
				if onDestSheet {
					t.Sheet = ""
					t.Start.Sheet = ""
					t.End.Sheet = ""
				} else {
					t.Sheet = destSheet
					t.Start.Sheet = destSheet
					t.End.Sheet = destSheet
				}
			}
		}
		return t
	case formula.NamedExpr:
		t.Expr = formula.RetargetFormulaReferencesToSheet(t.Expr, fromR, editedSheet, onEditedSheet, dCol, dRow, destSheet, onDestSheet)
		if onEditedSheet && t.HasBase && fromR.Contains(t.BaseCol, t.BaseRow) {
			t.BaseCol += dCol
			t.BaseRow += dRow
		}
		return t
	default:
		return v
	}
}

func (s *Sheet) adjustNamedForRowChange(atRow, delta int) {
	editedSheet := s.Name()
	isPrimary := s.workbook == nil || (len(s.workbook.Sheets) > 0 && s.workbook.Sheets[0] == s)
	adjustNamedMapRow(s.namedRanges, editedSheet, true, atRow, delta, false)
	if s.workbook != nil {
		adjustNamedMapRow(s.workbook.NamedRanges, editedSheet, isPrimary, atRow, delta, false)
		for _, other := range s.workbook.Sheets {
			if other != s {
				adjustNamedMapRow(other.namedRanges, editedSheet, false, atRow, delta, false)
			}
		}
	}
}

func (s *Sheet) adjustNamedForColChange(atCol, delta int) {
	editedSheet := s.Name()
	isPrimary := s.workbook == nil || (len(s.workbook.Sheets) > 0 && s.workbook.Sheets[0] == s)
	adjustNamedMapCol(s.namedRanges, editedSheet, true, atCol, delta, false)
	if s.workbook != nil {
		adjustNamedMapCol(s.workbook.NamedRanges, editedSheet, isPrimary, atCol, delta, false)
		for _, other := range s.workbook.Sheets {
			if other != s {
				adjustNamedMapCol(other.namedRanges, editedSheet, false, atCol, delta, false)
			}
		}
	}
}

func (s *Sheet) adjustNamedForRowDelete(atRow, count int) {
	editedSheet := s.Name()
	isPrimary := s.workbook == nil || (len(s.workbook.Sheets) > 0 && s.workbook.Sheets[0] == s)
	adjustNamedMapRow(s.namedRanges, editedSheet, true, atRow, count, true)
	if s.workbook != nil {
		adjustNamedMapRow(s.workbook.NamedRanges, editedSheet, isPrimary, atRow, count, true)
		for _, other := range s.workbook.Sheets {
			if other != s {
				adjustNamedMapRow(other.namedRanges, editedSheet, false, atRow, count, true)
			}
		}
	}
}

func (s *Sheet) adjustNamedForColDelete(atCol, count int) {
	editedSheet := s.Name()
	isPrimary := s.workbook == nil || (len(s.workbook.Sheets) > 0 && s.workbook.Sheets[0] == s)
	adjustNamedMapCol(s.namedRanges, editedSheet, true, atCol, count, true)
	if s.workbook != nil {
		adjustNamedMapCol(s.workbook.NamedRanges, editedSheet, isPrimary, atCol, count, true)
		for _, other := range s.workbook.Sheets {
			if other != s {
				adjustNamedMapCol(other.namedRanges, editedSheet, false, atCol, count, true)
			}
		}
	}
}

func adjustNamedMapRow(m map[string]any, editedSheet string, onEditedSheet bool, at, n int, del bool) {
	if m == nil {
		return
	}
	for k, v := range m {
		if nv, ok := adjustNamedValueRow(v, editedSheet, onEditedSheet, at, n, del); ok {
			m[k] = nv
		} else {
			delete(m, k)
		}
	}
}

func adjustNamedMapCol(m map[string]any, editedSheet string, onEditedSheet bool, at, n int, del bool) {
	if m == nil {
		return
	}
	for k, v := range m {
		if nv, ok := adjustNamedValueCol(v, editedSheet, onEditedSheet, at, n, del); ok {
			m[k] = nv
		} else {
			delete(m, k)
		}
	}
}

func adjustNamedValueRow(v any, editedSheet string, onEditedSheet bool, atRow, n int, del bool) (any, bool) {
	switch t := v.(type) {
	case coord.CellRef:
		if !refBelongsToSheet(t.Sheet, onEditedSheet, editedSheet) {
			return t, true
		}
		if del {
			if t.Row >= atRow && t.Row < atRow+n {
				return nil, false
			}
			if t.Row >= atRow+n {
				t.Row -= n
			}
			return t, true
		}
		if t.Row >= atRow {
			t.Row += n
		}
		return t, true
	case coord.RangeRef:
		refSheet := t.Sheet
		if refSheet == "" {
			refSheet = t.Start.Sheet
		}
		if !refBelongsToSheet(refSheet, onEditedSheet, editedSheet) {
			return t, true
		}
		if del {
			rr, ok := formula.ShrinkRangeForRowDelete(t, atRow, n)
			return rr, ok
		}
		return expandRangeForRowInsert(t, atRow, n), true
	case formula.NamedExpr:
		if del {
			t.Expr = formula.AdjustFormulaReferencesForRowDeleteOnSheet(t.Expr, editedSheet, onEditedSheet, atRow, n)
			if onEditedSheet && t.HasBase {
				if t.BaseRow >= atRow && t.BaseRow < atRow+n {
					t.HasBase = false
				} else if t.BaseRow >= atRow+n {
					t.BaseRow -= n
				}
			}
			return t, true
		}
		t.Expr = formula.AdjustFormulaReferencesForRowChangeOnSheet(t.Expr, editedSheet, onEditedSheet, atRow, n)
		if onEditedSheet && t.HasBase && t.BaseRow >= atRow {
			t.BaseRow += n
		}
		return t, true
	default:
		return v, true
	}
}

func adjustNamedValueCol(v any, editedSheet string, onEditedSheet bool, atCol, n int, del bool) (any, bool) {
	switch t := v.(type) {
	case coord.CellRef:
		if !refBelongsToSheet(t.Sheet, onEditedSheet, editedSheet) {
			return t, true
		}
		if del {
			if t.Col >= atCol && t.Col < atCol+n {
				return nil, false
			}
			if t.Col >= atCol+n {
				t.Col -= n
			}
			return t, true
		}
		if t.Col >= atCol {
			t.Col += n
		}
		return t, true
	case coord.RangeRef:
		refSheet := t.Sheet
		if refSheet == "" {
			refSheet = t.Start.Sheet
		}
		if !refBelongsToSheet(refSheet, onEditedSheet, editedSheet) {
			return t, true
		}
		if del {
			rr, ok := formula.ShrinkRangeForColDelete(t, atCol, n)
			return rr, ok
		}
		return expandRangeForColInsert(t, atCol, n), true
	case formula.NamedExpr:
		if del {
			t.Expr = formula.AdjustFormulaReferencesForColDeleteOnSheet(t.Expr, editedSheet, onEditedSheet, atCol, n)
			if onEditedSheet && t.HasBase {
				if t.BaseCol >= atCol && t.BaseCol < atCol+n {
					t.HasBase = false
				} else if t.BaseCol >= atCol+n {
					t.BaseCol -= n
				}
			}
			return t, true
		}
		t.Expr = formula.AdjustFormulaReferencesForColChangeOnSheet(t.Expr, editedSheet, onEditedSheet, atCol, n)
		if onEditedSheet && t.HasBase && t.BaseCol >= atCol {
			t.BaseCol += n
		}
		return t, true
	default:
		return v, true
	}
}

func (s *Sheet) adjustGraphForRowChange(atRow, n int, del bool) {
	s.adjustGraphsForRowChange(s.Name(), atRow, n, del)
	if s.workbook != nil {
		for _, other := range s.workbook.Sheets {
			if other != nil && other != s {
				other.adjustGraphsForRowChange(s.Name(), atRow, n, del)
			}
		}
	}
}

func (s *Sheet) adjustGraphsForRowChange(editedSheet string, atRow, n int, del bool) {
	if strings.EqualFold(graphRangeSheet(s.graph.RangeX, s.Name()), editedSheet) {
		s.graph.RangeX = adjustGraphRangePtr(s.graph.RangeX, atRow, n, del, true)
	}
	for k, v := range s.graph.Series {
		if strings.EqualFold(graphRangeSheet(v, s.Name()), editedSheet) {
			if adj := adjustGraphRangePtr(v, atRow, n, del, true); adj == nil {
				delete(s.graph.Series, k)
			} else {
				s.graph.Series[k] = adj
			}
		}
	}
}

func (s *Sheet) adjustGraphForColChange(atCol, n int, del bool) {
	s.adjustGraphsForColChange(s.Name(), atCol, n, del)
	if s.workbook != nil {
		for _, other := range s.workbook.Sheets {
			if other != nil && other != s {
				other.adjustGraphsForColChange(s.Name(), atCol, n, del)
			}
		}
	}
}

func (s *Sheet) adjustGraphsForColChange(editedSheet string, atCol, n int, del bool) {
	if strings.EqualFold(graphRangeSheet(s.graph.RangeX, s.Name()), editedSheet) {
		s.graph.RangeX = adjustGraphRangePtr(s.graph.RangeX, atCol, n, del, false)
	}
	for k, v := range s.graph.Series {
		if strings.EqualFold(graphRangeSheet(v, s.Name()), editedSheet) {
			if adj := adjustGraphRangePtr(v, atCol, n, del, false); adj == nil {
				delete(s.graph.Series, k)
			} else {
				s.graph.Series[k] = adj
			}
		}
	}
}

func adjustGraphRangePtr(r *coord.RangeRef, at, n int, del, row bool) *coord.RangeRef {
	if r == nil {
		return nil
	}
	rr := *r
	if row {
		if del {
			out, ok := formula.ShrinkRangeForRowDelete(rr, at, n)
			if !ok {
				return nil
			}
			return &out
		}
		out := expandRangeForRowInsert(rr, at, n)
		return &out
	}
	if del {
		out, ok := formula.ShrinkRangeForColDelete(rr, at, n)
		if !ok {
			return nil
		}
		return &out
	}
	out := expandRangeForColInsert(rr, at, n)
	return &out
}

func expandRangeForRowInsert(r coord.RangeRef, atRow, count int) coord.RangeRef {
	minR, maxR := r.MinRow(), r.MaxRow()
	if maxR < atRow {
		return r
	}
	if minR >= atRow {
		r.Start.Row += count
		r.End.Row += count
		return r
	}
	if r.Start.Row <= r.End.Row {
		r.End.Row += count
	} else {
		r.Start.Row += count
	}
	return r
}

func expandRangeForColInsert(r coord.RangeRef, atCol, count int) coord.RangeRef {
	minC, maxC := r.MinCol(), r.MaxCol()
	if maxC < atCol {
		return r
	}
	if minC >= atCol {
		r.Start.Col += count
		r.End.Col += count
		return r
	}
	if r.Start.Col <= r.End.Col {
		r.End.Col += count
	} else {
		r.Start.Col += count
	}
	return r
}
