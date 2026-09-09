package formula

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"hasucalc/cell"
	"hasucalc/coord"
)

type EvaluationContext interface {
	GetCellValue(col, row int) any
	GetRangeValues(r coord.RangeRef) [][]any
	GetNamedRange(name string) (any, bool)
	GetRawCell(col, row int) *cell.Cell
	GetCrossSheetCell(sheetName string, col, row int) *cell.Cell
	GetCrossSheetCellValue(sheetName string, col, row int) any
	GetCrossSheetRangeValues(sheetName string, r coord.RangeRef) [][]any
	GetCurrentSheetName() string
	GetCurrentSheetIndex() int
	GetSheetName(index int) string
	// SheetContext returns an EvaluationContext for the named sheet (self if empty/same).
	SheetContext(sheetName string) EvaluationContext
	MaxPopulatedRow() int
	MaxPopulatedCol() int
	EvalGeneration() uint64
}

type Evaluator struct {
	ctx        EvaluationContext
	visiting   map[string]bool
	cache      map[string]any
	currentCol int
	currentRow int
}

func NewEvaluator(ctx EvaluationContext) *Evaluator {
	return &Evaluator{
		ctx:      ctx,
		visiting: make(map[string]bool),
		cache:    make(map[string]any),
	}
}

func (e *Evaluator) cellKey(col, row int) string {
	return fmt.Sprintf("%s!%d,%d", e.ctx.GetCurrentSheetName(), col, row)
}

func (e *Evaluator) EvaluateCellAt(col, row int, formulaText string) any {
	prevCol, prevRow := e.currentCol, e.currentRow
	e.currentCol = col
	e.currentRow = row
	defer func() {
		e.currentCol = prevCol
		e.currentRow = prevRow
	}()
	key := e.cellKey(col, row)
	if e.visiting[key] {
		return cell.ErrCirc
	}
	if val, ok := e.cache[key]; ok {
		return val
	}

	e.visiting[key] = true
	defer func() {
		delete(e.visiting, key)
	}()

	ast, err := ParseFormula(formulaText)
	if err != nil {
		e.cache[key] = cell.ErrLotus
		return cell.ErrLotus
	}

	val := e.Evaluate(ast)
	val = unwrapScalarGrid(val)
	e.cache[key] = val
	return val
}

func (e *Evaluator) evaluateFormulaCell(rawCell *cell.Cell, col, row int) any {
	if rawCell == nil {
		return 0.0
	}
	key := e.cellKey(col, row)
	if e.visiting[key] {
		return cell.ErrCirc
	}
	gen := e.ctx.EvalGeneration()
	// Value from this Recalculate generation is fresh (not stale from a prior pass).
	if rawCell.Value != nil && rawCell.EvalGen == gen && gen > 0 {
		return rawCell.Value
	}
	if val, ok := e.cache[key]; ok {
		return val
	}
	val := e.EvaluateCellAt(col, row, rawCell.RawInput)
	rawCell.SetValue(val)
	rawCell.EvalGen = gen
	return val
}

// namedLookup resolves a possibly sheet-qualified name (SALES or Sheet1!SALES)
// on the owning sheet. The returned evaluator is bound to that sheet so
// unqualified ranges in the name evaluate there, not on the caller sheet.
func (e *Evaluator) namedLookup(rawName string) (named any, eval *Evaluator, found bool) {
	sheetQual, ident := splitQualifiedIdent(rawName)
	name := strings.ToUpper(ident)
	ctx := e.ctx
	if sheetQual != "" {
		ctx = e.ctx.SheetContext(sheetQual)
		if ctx == nil {
			return nil, nil, false
		}
	}
	named, found = ctx.GetNamedRange(name)
	if !found {
		return nil, nil, false
	}
	eval = e
	if ctx != e.ctx {
		eval = &Evaluator{
			ctx:        ctx,
			visiting:   e.visiting,
			cache:      e.cache,
			currentCol: e.currentCol,
			currentRow: e.currentRow,
		}
	}
	return named, eval, true
}

func (e *Evaluator) indirectA1Style(args []ASTNode) bool {
	if len(args) < 2 {
		return true
	}
	v := e.Evaluate(args[1])
	if v == nil {
		return true
	}
	return isTruthy(v)
}

func (e *Evaluator) parseIndirectRef(refText string, a1Style bool) (ASTNode, bool) {
	if a1Style {
		if cr, err := coord.ParseCellRef(refText); err == nil {
			return CellRefNode{Ref: cr}, true
		}
		if rr, err := coord.ParseRangeRef(refText); err == nil {
			return RangeRefNode{Ref: rr}, true
		}
		return nil, false
	}
	if cr, err := coord.ParseR1C1CellRef(refText, e.currentCol, e.currentRow); err == nil {
		return CellRefNode{Ref: cr}, true
	}
	if rr, err := coord.ParseR1C1RangeRef(refText, e.currentCol, e.currentRow); err == nil {
		return RangeRefNode{Ref: rr}, true
	}
	return nil, false
}

func (e *Evaluator) evalIndirectText(refText string, a1Style bool) any {
	if named, eval, ok := e.namedLookup(refText); ok {
		return evalNamedTarget(eval, named)
	}
	if node, ok := e.parseIndirectRef(refText, a1Style); ok {
		return e.Evaluate(node)
	}
	return cell.ErrLotus
}

func evalNamedTarget(eval *Evaluator, named any) any {
	switch nr := named.(type) {
	case coord.CellRef:
		return eval.Evaluate(CellRefNode{Ref: nr})
	case coord.RangeRef:
		return eval.Evaluate(RangeRefNode{Ref: nr})
	case NamedExpr:
		return eval.evaluateNamedExpr(nr)
	case string:
		if strings.HasPrefix(nr, "=") {
			return eval.evaluateNamedExpr(NamedExpr{Expr: nr})
		}
		return nr
	default:
		return nr
	}
}

// namedToRefNode converts a named target into a cell/range node when it is a reference
// (including NamedExpr that wraps a single CellRef/RangeRef).
func namedToRefNode(named any, eval *Evaluator) (ASTNode, bool) {
	switch nr := named.(type) {
	case coord.CellRef:
		return CellRefNode{Ref: nr}, true
	case coord.RangeRef:
		return RangeRefNode{Ref: nr}, true
	case NamedExpr:
		expr := nr.Expr
		if !strings.HasPrefix(expr, "=") && !strings.HasPrefix(expr, "@") && !strings.HasPrefix(expr, "+") {
			expr = "=" + expr
		}
		ast, err := ParseFormula(expr)
		if err != nil {
			return nil, false
		}
		if nr.HasBase && eval != nil {
			dCol := eval.currentCol - nr.BaseCol
			dRow := eval.currentRow - nr.BaseRow
			if dCol != 0 || dRow != 0 {
				ast = shiftNode(ast, dCol, dRow)
			}
		}
		switch ast.(type) {
		case CellRefNode, RangeRefNode:
			return ast, true
		default:
			return nil, false
		}
	case string:
		if cr, err := coord.ParseCellRef(nr); err == nil {
			return CellRefNode{Ref: cr}, true
		}
		if rr, err := coord.ParseRangeRef(nr); err == nil {
			return RangeRefNode{Ref: rr}, true
		}
		return nil, false
	default:
		return nil, false
	}
}

func stampSheetOnRef(node ASTNode, sheetName string) ASTNode {
	if sheetName == "" {
		return node
	}
	switch n := node.(type) {
	case CellRefNode:
		if n.Ref.Sheet == "" {
			ref := n.Ref
			ref.Sheet = sheetName
			return CellRefNode{Ref: ref}
		}
	case RangeRefNode:
		if n.Ref.Sheet == "" && n.Ref.Start.Sheet == "" {
			ref := n.Ref
			ref.Sheet = sheetName
			ref.Start.Sheet = sheetName
			ref.End.Sheet = sheetName
			return RangeRefNode{Ref: ref}
		}
	}
	return node
}

// evaluateNamedExpr parses a named formula and applies base-cell relative adjustment when present.
func (e *Evaluator) evaluateNamedExpr(nr NamedExpr) any {
	expr := nr.Expr
	if !strings.HasPrefix(expr, "=") && !strings.HasPrefix(expr, "@") && !strings.HasPrefix(expr, "+") {
		expr = "=" + expr
	}
	ast, err := ParseFormula(expr)
	if err != nil {
		return cell.ErrLotus
	}
	if nr.HasBase {
		dCol := e.currentCol - nr.BaseCol
		dRow := e.currentRow - nr.BaseRow
		if dCol != 0 || dRow != 0 {
			ast = shiftNode(ast, dCol, dRow)
		}
	}
	return e.Evaluate(ast)
}

func (e *Evaluator) Evaluate(node ASTNode) any {
	switch n := node.(type) {
	case NumberNode:
		return n.Value

	case StringNode:
		return n.Value

	case MissingArgNode:
		return nil

	case CellRefNode:
		if n.Ref.Sheet != "" {
			rawCell := e.ctx.GetCrossSheetCell(n.Ref.Sheet, n.Ref.Col, n.Ref.Row)
			if rawCell == nil && n.Ref.Sheet != "" {
				// Missing sheet (or deleted) → #REF!, not silent 0
				if e.ctx.SheetContext(n.Ref.Sheet) == nil {
					return cell.ErrRef
				}
			}
			if rawCell != nil && rawCell.Type == cell.TypeFormula {
				targetCtx := e.ctx.SheetContext(n.Ref.Sheet)
				if targetCtx == nil {
					return cell.ErrLotus
				}
				if targetCtx == e.ctx {
					return e.evaluateFormulaCell(rawCell, n.Ref.Col, n.Ref.Row)
				}
				sub := &Evaluator{ctx: targetCtx, visiting: e.visiting, cache: e.cache}
				return sub.evaluateFormulaCell(rawCell, n.Ref.Col, n.Ref.Row)
			}
			val := e.ctx.GetCrossSheetCellValue(n.Ref.Sheet, n.Ref.Col, n.Ref.Row)
			if val == nil {
				return nil
			}
			return val
		}
		key := e.cellKey(n.Ref.Col, n.Ref.Row)
		if e.visiting[key] {
			return cell.ErrCirc
		}
		rawCell := e.ctx.GetRawCell(n.Ref.Col, n.Ref.Row)
		if rawCell != nil && rawCell.Type == cell.TypeFormula {
			return e.evaluateFormulaCell(rawCell, n.Ref.Col, n.Ref.Row)
		}
		val := e.ctx.GetCellValue(n.Ref.Col, n.Ref.Row)
		if val == nil {
			return nil
		}
		return val

	case RangeRefNode:
		sheetName := n.Ref.Sheet
		if sheetName == "" {
			sheetName = n.Ref.Start.Sheet
		}
		if sheetName != "" {
			targetCtx := e.ctx.SheetContext(sheetName)
			if targetCtx == nil {
				return cell.ErrRef
			}
			if targetCtx != e.ctx {
				sub := &Evaluator{ctx: targetCtx, visiting: e.visiting, cache: e.cache}
				return sub.Evaluate(RangeRefNode{Ref: coord.RangeRef{
					Start: n.Ref.Start,
					End:   n.Ref.End,
				}})
			}
		}
		minRow := n.Ref.MinRow()
		maxRow := n.Ref.MaxRow()
		minCol := n.Ref.MinCol()
		maxCol := n.Ref.MaxCol()
		// Whole-column / whole-row: limit to used range (Excel-like sparse semantics).
		if maxRow >= 100000 {
			maxRow = e.ctx.MaxPopulatedRow()
			if maxRow < minRow {
				maxRow = minRow
			}
		}
		if maxCol >= 10000 {
			maxCol = e.ctx.MaxPopulatedCol()
			if maxCol < minCol {
				maxCol = minCol
			}
		}
		var grid [][]any
		for r := minRow; r <= maxRow; r++ {
			var rowVals []any
			for c := minCol; c <= maxCol; c++ {
				key := e.cellKey(c, r)
				if e.visiting[key] {
					rowVals = append(rowVals, cell.ErrCirc)
					continue
				}
				rawCell := e.ctx.GetRawCell(c, r)
				if rawCell != nil && rawCell.Type == cell.TypeFormula {
					rowVals = append(rowVals, e.evaluateFormulaCell(rawCell, c, r))
				} else {
					rowVals = append(rowVals, e.ctx.GetCellValue(c, r))
				}
			}
			grid = append(grid, rowVals)
		}
		return grid

	case IdentifierNode:
		rawName := n.Name
		sheetQual, ident := splitQualifiedIdent(rawName)
		name := strings.ToUpper(ident)
		if name == "ERR" || name == "#ERR#" {
			return cell.ErrLotus
		}
		if name == "NA" || name == "#N/A" || name == "#NA#" {
			return cell.ErrNA
		}
		if name == "#REF!" || name == "REF" || name == "#REF#" {
			return cell.ErrRef
		}
		if name == "TRUE" {
			return true
		}
		if name == "FALSE" {
			return false
		}
		if sheetQual != "" && e.ctx.SheetContext(sheetQual) == nil {
			return cell.ErrRef
		}
		if named, eval, ok := e.namedLookup(rawName); ok {
			// Scope by current cell so relative named formulas (DayOfWeek → OFFSET …)
			// can be used from many cells without a false CIRCULAR REF on the name itself.
			nameKey := fmt.Sprintf("NAME:%s@%s!%d,%d", name, e.ctx.GetCurrentSheetName(), e.currentCol, e.currentRow)
			if e.visiting[nameKey] {
				return cell.ErrCirc
			}
			e.visiting[nameKey] = true
			defer delete(e.visiting, nameKey)
			return evalNamedTarget(eval, named)
		}
		return cell.ErrLotus

	case ErrorNode:
		switch strings.ToUpper(n.Code) {
		case "#REF!", "REF":
			return cell.ErrRef
		case "NA", "#N/A":
			return cell.ErrNA
		default:
			return cell.ErrLotus
		}

	case UnaryOpNode:
		val := e.Evaluate(n.Operand)
		if errVal, ok := firstLotusError(val); ok {
			return errVal
		}
		switch n.Op {
		case "+":
			if num, ok := toArithmeticFloat(val); ok {
				return num
			}
		case "-":
			if num, ok := toArithmeticFloat(val); ok {
				return -num
			}
		case "NOT":
			if isTruthy(val) {
				return false
			}
			return true
		}
		return cell.ErrLotus

	case BinaryOpNode:
		lVal := e.Evaluate(n.Left)
		rVal := e.Evaluate(n.Right)

		if errVal, ok := firstLotusError(lVal); ok {
			return errVal
		}
		if errVal, ok := firstLotusError(rVal); ok {
			return errVal
		}

		if n.Op == "AND" {
			return isTruthy(lVal) && isTruthy(rVal)
		}
		if n.Op == "OR" {
			return isTruthy(lVal) || isTruthy(rVal)
		}
		if n.Op == "&" {
			return stringifyConcat(lVal) + stringifyConcat(rVal)
		}

		// Comparisons — Excel returns boolean TRUE/FALSE
		switch n.Op {
		case "=":
			return compareEqual(lVal, rVal)
		case "<>":
			return !compareEqual(lVal, rVal)
		case "<":
			return compareLess(lVal, rVal)
		case "<=":
			return compareLess(lVal, rVal) || compareEqual(lVal, rVal)
		case ">":
			return !compareLess(lVal, rVal) && !compareEqual(lVal, rVal)
		case ">=":
			return !compareLess(lVal, rVal)
		}

		// Arithmetic
		lNum, ok1 := toArithmeticFloat(lVal)
		rNum, ok2 := toArithmeticFloat(rVal)
		if !ok1 || !ok2 {
			return cell.ErrLotus
		}

		var res float64
		switch n.Op {
		case "+":
			res = lNum + rNum
		case "-":
			res = lNum - rNum
		case "*":
			res = lNum * rNum
		case "/":
			if rNum == 0 {
				return cell.ErrLotus
			}
			res = lNum / rNum
		case "^":
			if lNum < 0 && math.Floor(rNum) != rNum {
				return cell.ErrLotus
			}
			res = math.Pow(lNum, rNum)
		default:
			return cell.ErrLotus
		}
		if math.IsNaN(res) || math.IsInf(res, 0) {
			return cell.ErrLotus
		}
		return res

	case FunctionCallNode:
		fnName := strings.ToUpper(n.Name)
		if fnName == "@ISBLANK" {
			if len(n.Args) != 1 {
				return cell.ErrLotus
			}
			if cr, ok := n.Args[0].(CellRefNode); ok {
				if cr.Ref.Sheet != "" {
					v := e.ctx.GetCrossSheetCellValue(cr.Ref.Sheet, cr.Ref.Col, cr.Ref.Row)
					if v == nil {
						return 1.0
					}
					return 0.0
				}
				rawCell := e.ctx.GetRawCell(cr.Ref.Col, cr.Ref.Row)
				if rawCell == nil || rawCell.Type == cell.TypeEmpty || rawCell.Value == nil {
					return 1.0
				}
				return 0.0
			}
			val := e.Evaluate(n.Args[0])
			if val == nil {
				return 1.0
			}
			return 0.0
		}
		if fnName == "@IF" {
			if len(n.Args) < 2 || len(n.Args) > 3 {
				return cell.ErrLotus
			}
			condVal := e.Evaluate(n.Args[0])
			if errVal, ok := condVal.(cell.LotusError); ok {
				return errVal
			}
			if isTruthy(condVal) {
				return e.Evaluate(n.Args[1])
			}
			if len(n.Args) == 3 {
				return e.Evaluate(n.Args[2])
			}
			return 0.0
		}
		if fnName == "@IFERROR" {
			if len(n.Args) != 2 {
				return cell.ErrLotus
			}
			val := e.Evaluate(n.Args[0])
			if _, ok := val.(cell.LotusError); ok {
				return e.Evaluate(n.Args[1])
			}
			return val
		}
		if fnName == "@IFNA" {
			if len(n.Args) != 2 {
				return cell.ErrLotus
			}
			val := e.Evaluate(n.Args[0])
			if errVal, ok := val.(cell.LotusError); ok && errVal.Code == cell.ErrNA.Code {
				return e.Evaluate(n.Args[1])
			}
			return val
		}
		if fnName == "@IFS" {
			if len(n.Args) < 2 || len(n.Args)%2 != 0 {
				return cell.ErrLotus
			}
			for i := 0; i < len(n.Args); i += 2 {
				cVal := e.Evaluate(n.Args[i])
				if errVal, ok := cVal.(cell.LotusError); ok {
					return errVal
				}
				if isTruthy(cVal) {
					return e.Evaluate(n.Args[i+1])
				}
			}
			return cell.ErrNA
		}
		if fnName == "@SWITCH" {
			if len(n.Args) < 3 {
				return cell.ErrLotus
			}
			targetVal := e.Evaluate(n.Args[0])
			if errVal, ok := targetVal.(cell.LotusError); ok {
				return errVal
			}
			for i := 1; i+1 < len(n.Args); i += 2 {
				matchVal := e.Evaluate(n.Args[i])
				if errVal, ok := matchVal.(cell.LotusError); ok {
					return errVal
				}
				if compareEqual(matchVal, targetVal) {
					return e.Evaluate(n.Args[i+1])
				}
			}
			if (len(n.Args)-1)%2 == 1 {
				return e.Evaluate(n.Args[len(n.Args)-1])
			}
			return cell.ErrNA
		}
		if fnName == "@INDIRECT" {
			if len(n.Args) < 1 || len(n.Args) > 2 {
				return cell.ErrLotus
			}
			refVal := e.Evaluate(n.Args[0])
			if errVal, ok := refVal.(cell.LotusError); ok {
				return errVal
			}
			refText := strings.TrimSpace(fmt.Sprintf("%v", unwrapCellSourced(refVal)))
			if refText == "" {
				return cell.ErrLotus
			}
			return e.evalIndirectText(refText, e.indirectA1Style(n.Args))
		}
		if fnName == "@SHEET" {
			if len(n.Args) == 0 {
				return float64(e.ctx.GetCurrentSheetIndex())
			}
			if cr, ok := n.Args[0].(CellRefNode); ok && cr.Ref.Sheet != "" {
				sName := cr.Ref.Sheet
				for i := 1; i <= 200; i++ {
					if strings.EqualFold(e.ctx.GetSheetName(i), sName) {
						return float64(i)
					}
				}
			}
			return float64(e.ctx.GetCurrentSheetIndex())
		}
		if fnName == "@SHEETNAME" || fnName == "@SHEETNAMES" {
			if len(n.Args) == 0 {
				return e.ctx.GetCurrentSheetName()
			}
			idxVal := e.Evaluate(n.Args[0])
			if idxF, ok := toFloat(idxVal); ok {
				return e.ctx.GetSheetName(int(idxF))
			}
			return e.ctx.GetCurrentSheetName()
		}
		if fnName == "@OFFSET" {
			if len(n.Args) < 3 || len(n.Args) > 5 {
				return cell.ErrLotus
			}
			if _, isErr := n.Args[0].(ErrorNode); isErr {
				return cell.ErrRef
			}
			if id, ok := n.Args[0].(IdentifierNode); ok {
				u := strings.ToUpper(id.Name)
				if u == "#REF!" || u == "REF" || u == "#REF#" {
					return cell.ErrRef
				}
			}
			baseSheet, baseCol, baseRow, isRange, origRange, ok := e.resolveReference(n.Args[0])
			if !ok {
				return cell.ErrLotus
			}
			rowOffVal := e.Evaluate(n.Args[1])
			colOffVal := e.Evaluate(n.Args[2])
			if errVal, ok := firstLotusError(rowOffVal); ok {
				return errVal
			}
			if errVal, ok := firstLotusError(colOffVal); ok {
				return errVal
			}
			rOff, ok1 := toFloat(rowOffVal)
			cOff, ok2 := toFloat(colOffVal)
			if !ok1 || !ok2 {
				return cell.ErrLotus
			}
			targetCol := baseCol + int(cOff)
			targetRow := baseRow + int(rOff)
			height := 1
			width := 1
			if isRange {
				height = origRange.MaxRow() - origRange.MinRow() + 1
				width = origRange.MaxCol() - origRange.MinCol() + 1
			}
			if len(n.Args) >= 4 {
				hEval := e.Evaluate(n.Args[3])
				if errVal, ok := firstLotusError(hEval); ok {
					return errVal
				}
				hVal, ok := toFloat(hEval)
				if !ok || int(hVal) <= 0 {
					return cell.ErrLotus
				}
				height = int(hVal)
			}
			if len(n.Args) >= 5 {
				wEval := e.Evaluate(n.Args[4])
				if errVal, ok := firstLotusError(wEval); ok {
					return errVal
				}
				wVal, ok := toFloat(wEval)
				if !ok || int(wVal) <= 0 {
					return cell.ErrLotus
				}
				width = int(wVal)
			}
			if targetCol < 0 || targetRow < 0 {
				return cell.ErrLotus
			}
			if height == 1 && width == 1 {
				return e.Evaluate(CellRefNode{Ref: coord.CellRef{Sheet: baseSheet, Col: targetCol, Row: targetRow}})
			}
			targetRange := coord.RangeRef{
				Sheet: baseSheet,
				Start: coord.CellRef{Sheet: baseSheet, Col: targetCol, Row: targetRow},
				End:   coord.CellRef{Sheet: baseSheet, Col: targetCol + width - 1, Row: targetRow + height - 1},
			}
			return e.Evaluate(RangeRefNode{Ref: targetRange})
		}
		if fnName == "@ROW" {
			if len(n.Args) == 0 {
				return float64(e.currentRow + 1)
			}
			arg0 := n.Args[0]
			if _, isErr := arg0.(ErrorNode); isErr {
				return cell.ErrRef
			}
			if idNode, ok := arg0.(IdentifierNode); ok {
				u := strings.ToUpper(idNode.Name)
				if u == "#REF!" || u == "REF" || u == "#REF#" {
					return cell.ErrRef
				}
				if named, eval, found := e.namedLookup(idNode.Name); found {
					if refNode, okRef := namedToRefNode(named, eval); okRef {
						arg0 = refNode
					}
				}
			}
			if cr, ok := arg0.(CellRefNode); ok {
				return float64(cr.Ref.Row + 1)
			}
			if rr, ok := arg0.(RangeRefNode); ok {
				return float64(rr.Ref.MinRow() + 1)
			}
			return cell.ErrLotus
		}
		if fnName == "@COLUMN" {
			if len(n.Args) == 0 {
				return float64(e.currentCol + 1)
			}
			arg0 := n.Args[0]
			if _, isErr := arg0.(ErrorNode); isErr {
				return cell.ErrRef
			}
			if idNode, ok := arg0.(IdentifierNode); ok {
				u := strings.ToUpper(idNode.Name)
				if u == "#REF!" || u == "REF" || u == "#REF#" {
					return cell.ErrRef
				}
				if named, eval, found := e.namedLookup(idNode.Name); found {
					if refNode, okRef := namedToRefNode(named, eval); okRef {
						arg0 = refNode
					}
				}
			}
			if cr, ok := arg0.(CellRefNode); ok {
				return float64(cr.Ref.Col + 1)
			}
			if rr, ok := arg0.(RangeRefNode); ok {
				return float64(rr.Ref.MinCol() + 1)
			}
			return cell.ErrLotus
		}

		fn, ok := Functions[fnName]
		if !ok {
			return cell.ErrLotus
		}
		argNodes := n.Args
		if (fnName == "@SUMIF" || fnName == "@AVERAGEIF") && len(argNodes) >= 3 {
			expanded := make([]ASTNode, len(argNodes))
			copy(expanded, argNodes)
			expanded[2] = expandConditionalSumRange(e, expanded[0], expanded[2])
			argNodes = expanded
		}
		var evaluatedArgs []any
		for _, argNode := range argNodes {
			val := e.Evaluate(argNode)
			switch n := argNode.(type) {
			case CellRefNode:
				if _, isGrid := val.([][]any); !isGrid {
					val = cellSourced{V: val}
				}
			case IdentifierNode:
				// TRUE/FALSE/NA literals are not cell-sourced; named ranges are.
				if !isReservedIdent(n.Name) {
					if _, isGrid := val.([][]any); !isGrid {
						val = cellSourced{V: val}
					}
				}
			case FunctionCallNode:
				fnArg := strings.ToUpper(strings.TrimPrefix(argNode.(FunctionCallNode).Name, "@"))
				if fnArg == "INDIRECT" || fnArg == "OFFSET" {
					if _, isGrid := val.([][]any); !isGrid {
						val = cellSourced{V: val}
					}
				}
			}
			evaluatedArgs = append(evaluatedArgs, val)
		}

		if !isErrorTolerantFunction(fnName) {
			for _, v := range evaluatedArgs {
				if errVal, ok := firstLotusError(v); ok {
					return errVal
				}
			}
		}

		return unwrapCellSourced(fn(evaluatedArgs))
	}

	return cell.ErrLotus
}

// expandConditionalSumRange resizes sum_range to the criteria range's shape,
// using the upper-left cell of sum_range as the origin (Excel SUMIF/AVERAGEIF).
func expandConditionalSumRange(e *Evaluator, critNode, sumNode ASTNode) ASTNode {
	crit, ok := nodeRangeShape(e, critNode)
	if !ok {
		return sumNode
	}
	origin, ok := nodeRangeOrigin(e, sumNode)
	if !ok {
		return sumNode
	}
	height := crit.MaxRow() - crit.MinRow() + 1
	width := crit.MaxCol() - crit.MinCol() + 1
	if height < 1 {
		height = 1
	}
	if width < 1 {
		width = 1
	}
	if height >= 100000 {
		maxR := e.ctx.MaxPopulatedRow()
		if origin.Row > maxR {
			height = 1
		} else if n := maxR - origin.Row + 1; n < height {
			height = n
			if height < 1 {
				height = 1
			}
		}
	}
	if width >= 10000 {
		maxC := e.ctx.MaxPopulatedCol()
		if origin.Col > maxC {
			width = 1
		} else if n := maxC - origin.Col + 1; n < width {
			width = n
			if width < 1 {
				width = 1
			}
		}
	}
	endCol := origin.Col + width - 1
	endRow := origin.Row + height - 1
	if endCol >= coord.MaxCols {
		endCol = coord.MaxCols - 1
	}
	if endRow >= coord.MaxRows {
		endRow = coord.MaxRows - 1
	}
	return RangeRefNode{Ref: coord.RangeRef{
		Sheet: origin.Sheet,
		Start: coord.CellRef{Sheet: origin.Sheet, Col: origin.Col, Row: origin.Row},
		End:   coord.CellRef{Sheet: origin.Sheet, Col: endCol, Row: endRow},
	}}
}

func nodeRangeShape(e *Evaluator, n ASTNode) (coord.RangeRef, bool) {
	switch t := n.(type) {
	case RangeRefNode:
		return t.Ref, true
	case CellRefNode:
		return coord.RangeRef{Sheet: t.Ref.Sheet, Start: t.Ref, End: t.Ref}, true
	case IdentifierNode:
		if named, eval, found := e.namedLookup(t.Name); found {
			if refNode, ok := namedToRefNode(named, eval); ok {
				return nodeRangeShape(e, refNode)
			}
		}
	}
	return coord.RangeRef{}, false
}

func nodeRangeOrigin(e *Evaluator, n ASTNode) (coord.CellRef, bool) {
	rr, ok := nodeRangeShape(e, n)
	if !ok {
		return coord.CellRef{}, false
	}
	return coord.CellRef{Sheet: rr.Sheet, Col: rr.MinCol(), Row: rr.MinRow()}, true
}

func isErrorTolerantFunction(fnName string) bool {
	switch fnName {
	case "@ISERROR", "@ISERR", "@ISNA",
		"@IFERROR", "@IFNA",
		"@TYPE", "@ERROR.TYPE",
		"@ISNUMBER", "@ISSTRING", "@ISTEXT", "@ISNONTEXT", "@ISLOGICAL", "@ISBLANK",
		"@ROWS", "@COLUMNS", "@AREAS",
		"@COUNTIF", "@COUNTIFS", "@SUMIF", "@SUMIFS", "@AVERAGEIF", "@AVERAGEIFS", "@MINIFS", "@MAXIFS",
		"@XLOOKUP", "@XMATCH":
		return true
	}
	return false
}

func compareEqual(a, b any) bool {
	a = unwrapCellSourced(a)
	b = unwrapCellSourced(b)
	if a == nil && b == nil {
		return true
	}
	if (a == nil && b == 0.0) || (b == nil && a == 0.0) {
		return true
	}
	if (a == nil && b == false) || (b == nil && a == false) {
		return true
	}
	if a == nil {
		a = ""
	}
	if b == nil {
		b = ""
	}
	if a == "" && b == "" {
		return true
	}
	if (a == "" && b == 0.0) || (b == "" && a == 0.0) {
		return false
	}
	aNum, aOk := toFloat(a)
	bNum, bOk := toFloat(b)
	if aOk && bOk {
		return aNum == bNum
	}
	return strings.EqualFold(fmt.Sprintf("%v", a), fmt.Sprintf("%v", b))
}

func compareLess(a, b any) bool {
	a = unwrapCellSourced(a)
	b = unwrapCellSourced(b)
	aNum, aOk := toFloat(a)
	bNum, bOk := toFloat(b)
	if a == nil && bOk {
		aNum = 0.0
		aOk = true
	} else if b == nil && aOk {
		bNum = 0.0
		bOk = true
	}
	if aOk && bOk {
		return aNum < bNum
	}
	if aOk && !bOk {
		return true // numbers sort before text
	}
	if !aOk && bOk {
		return false
	}
	if a == nil {
		a = ""
	}
	if b == nil {
		b = ""
	}
	s1 := strings.ToLower(fmt.Sprintf("%v", a))
	s2 := strings.ToLower(fmt.Sprintf("%v", b))
	return s1 < s2
}

// AdjustFormulaReferences shifts relative cell/range references when copied by dCol, dRow.
// References that fall outside the sheet (negative or beyond Excel limits) become #REF!.
func AdjustFormulaReferences(formulaText string, dCol, dRow int) string {
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	shifted := shiftNode(ast, dCol, dRow)
	return reprefixFormula(formulaText, NodeToString(shifted))
}

const (
	maxSheetCols = 16384
	maxSheetRows = 1048576
)

func shiftCellRef(ref coord.CellRef, dCol, dRow int) (coord.CellRef, bool) {
	newCol := ref.Col
	if !ref.ColAbs {
		newCol = ref.Col + dCol
	}
	newRow := ref.Row
	if !ref.RowAbs {
		newRow = ref.Row + dRow
	}
	if newCol < 0 || newRow < 0 || newCol >= maxSheetCols || newRow >= maxSheetRows {
		return ref, false
	}
	return coord.CellRef{
		Sheet:  ref.Sheet,
		Col:    newCol,
		Row:    newRow,
		ColAbs: ref.ColAbs,
		RowAbs: ref.RowAbs,
	}, true
}

func shiftNode(node ASTNode, dCol, dRow int) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		if ref, ok := shiftCellRef(n.Ref, dCol, dRow); ok {
			return CellRefNode{Ref: ref}
		}
		return ErrorNode{Code: "#REF!"}
	case RangeRefNode:
		cRow := dRow
		if n.Ref.IsWholeColumn() {
			cRow = 0
		}
		cCol := dCol
		if n.Ref.IsWholeRow() {
			cCol = 0
		}
		start, ok1 := shiftCellRef(n.Ref.Start, cCol, cRow)
		end, ok2 := shiftCellRef(n.Ref.End, cCol, cRow)
		if !ok1 || !ok2 {
			return ErrorNode{Code: "#REF!"}
		}
		return RangeRefNode{Ref: coord.RangeRef{
			Sheet: n.Ref.Sheet,
			Start: start,
			End:   end,
		}}
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: shiftNode(n.Operand, dCol, dRow)}
	case BinaryOpNode:
		return BinaryOpNode{
			Op:    n.Op,
			Left:  shiftNode(n.Left, dCol, dRow),
			Right: shiftNode(n.Right, dCol, dRow),
		}
	case FunctionCallNode:
		var newArgs []ASTNode
		for _, arg := range n.Args {
			newArgs = append(newArgs, shiftNode(arg, dCol, dRow))
		}
		return FunctionCallNode{Name: n.Name, Args: newArgs}
	default:
		return node
	}
}

// TransposeFormulaReferences remaps relative refs when a formula cell is pasted transposed
// from (srcCol,srcRow) to (destCol,destRow): relative offsets swap axes.
func TransposeFormulaReferences(formulaText string, srcCol, srcRow, destCol, destRow int) string {
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	shifted := transposeNode(ast, srcCol, srcRow, destCol, destRow)
	return reprefixFormula(formulaText, NodeToString(shifted))
}

func transposeCellRef(ref coord.CellRef, srcCol, srcRow, destCol, destRow int) (coord.CellRef, bool) {
	out := coord.CellRef{
		Sheet:  ref.Sheet,
		Col:    ref.Col,
		Row:    ref.Row,
		ColAbs: ref.ColAbs,
		RowAbs: ref.RowAbs,
	}
	offC := ref.Col - srcCol
	offR := ref.Row - srcRow
	if !ref.ColAbs {
		out.Col = destCol + offR
	} else {
		out.Col = ref.Col
	}
	if !ref.RowAbs {
		out.Row = destRow + offC
	} else {
		out.Row = ref.Row
	}
	if out.Col < 0 || out.Row < 0 || out.Col >= maxSheetCols || out.Row >= maxSheetRows {
		return out, false
	}
	return out, true
}

func transposeNode(node ASTNode, srcCol, srcRow, destCol, destRow int) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		if ref, ok := transposeCellRef(n.Ref, srcCol, srcRow, destCol, destRow); ok {
			return CellRefNode{Ref: ref}
		}
		return ErrorNode{Code: "#REF!"}
	case RangeRefNode:
		start, ok1 := transposeCellRef(n.Ref.Start, srcCol, srcRow, destCol, destRow)
		end, ok2 := transposeCellRef(n.Ref.End, srcCol, srcRow, destCol, destRow)
		if !ok1 || !ok2 {
			return ErrorNode{Code: "#REF!"}
		}
		return RangeRefNode{Ref: coord.RangeRef{
			Sheet: n.Ref.Sheet,
			Start: start,
			End:   end,
		}}
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: transposeNode(n.Operand, srcCol, srcRow, destCol, destRow)}
	case BinaryOpNode:
		return BinaryOpNode{
			Op:    n.Op,
			Left:  transposeNode(n.Left, srcCol, srcRow, destCol, destRow),
			Right: transposeNode(n.Right, srcCol, srcRow, destCol, destRow),
		}
	case FunctionCallNode:
		var newArgs []ASTNode
		for _, arg := range n.Args {
			newArgs = append(newArgs, transposeNode(arg, srcCol, srcRow, destCol, destRow))
		}
		return FunctionCallNode{Name: n.Name, Args: newArgs}
	default:
		return node
	}
}

// AdjustFormulaReferencesForRowChangeOnSheet adjusts row refs that belong to editedSheet.
// onEditedSheet=true: blank or matching sheet name (formulas living on the edited sheet).
// onEditedSheet=false: only explicit editedSheet! refs (formulas on other sheets).
func AdjustFormulaReferencesForRowChangeOnSheet(formulaText, editedSheet string, onEditedSheet bool, atRow, delta int) string {
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	shifted := mapNodeRowsScoped(ast, editedSheet, onEditedSheet, atRow, delta)
	return reprefixFormula(formulaText, NodeToString(shifted))
}

func AdjustFormulaReferencesForColChangeOnSheet(formulaText, editedSheet string, onEditedSheet bool, atCol, delta int) string {
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	shifted := mapNodeColsScoped(ast, editedSheet, onEditedSheet, atCol, delta)
	return reprefixFormula(formulaText, NodeToString(shifted))
}

func reprefixFormula(original, res string) string {
	if strings.HasPrefix(original, "=") {
		res = strings.TrimPrefix(res, "@")
		if !strings.HasPrefix(res, "=") {
			res = "=" + res
		}
	} else if strings.HasPrefix(original, "@") {
		if !strings.HasPrefix(res, "@") {
			res = "@" + res
		}
	} else if strings.HasPrefix(original, "+") {
		if !strings.HasPrefix(res, "+") && !strings.HasPrefix(res, "-") && !strings.HasPrefix(res, "@") && !strings.HasPrefix(res, "(") {
			res = "+" + res
		}
	} else {
		if !strings.HasPrefix(res, "=") && !strings.HasPrefix(res, "@") && !strings.HasPrefix(res, "+") && !strings.HasPrefix(res, "-") {
			res = "=" + res
		}
	}
	return res
}

func refBelongsToEditedSheet(ref coord.CellRef, editedSheet string, onEditedSheet bool) bool {
	if onEditedSheet {
		return ref.Sheet == "" || editedSheet == "" || strings.EqualFold(ref.Sheet, editedSheet)
	}
	return editedSheet != "" && strings.EqualFold(ref.Sheet, editedSheet)
}

func mapCellRowScoped(ref coord.CellRef, editedSheet string, onEditedSheet bool, atRow, delta int) coord.CellRef {
	if !refBelongsToEditedSheet(ref, editedSheet, onEditedSheet) {
		return ref
	}
	if ref.Row >= atRow {
		ref.Row += delta
		if ref.Row < 0 {
			ref.Row = 0
		}
	}
	return ref
}

func mapCellColScoped(ref coord.CellRef, editedSheet string, onEditedSheet bool, atCol, delta int) coord.CellRef {
	if !refBelongsToEditedSheet(ref, editedSheet, onEditedSheet) {
		return ref
	}
	if ref.Col >= atCol {
		ref.Col += delta
		if ref.Col < 0 {
			ref.Col = 0
		}
	}
	return ref
}

func mapNodeRowsScoped(node ASTNode, editedSheet string, onEditedSheet bool, atRow, delta int) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		return CellRefNode{Ref: mapCellRowScoped(n.Ref, editedSheet, onEditedSheet, atRow, delta)}
	case RangeRefNode:
		if n.Ref.IsWholeColumn() {
			return n
		}
		return RangeRefNode{Ref: coord.RangeRef{
			Sheet: n.Ref.Sheet,
			Start: mapCellRowScoped(n.Ref.Start, editedSheet, onEditedSheet, atRow, delta),
			End:   mapCellRowScoped(n.Ref.End, editedSheet, onEditedSheet, atRow, delta),
		}}
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: mapNodeRowsScoped(n.Operand, editedSheet, onEditedSheet, atRow, delta)}
	case BinaryOpNode:
		return BinaryOpNode{Op: n.Op, Left: mapNodeRowsScoped(n.Left, editedSheet, onEditedSheet, atRow, delta), Right: mapNodeRowsScoped(n.Right, editedSheet, onEditedSheet, atRow, delta)}
	case FunctionCallNode:
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			args[i] = mapNodeRowsScoped(a, editedSheet, onEditedSheet, atRow, delta)
		}
		return FunctionCallNode{Name: n.Name, Args: args}
	default:
		return node
	}
}

func mapNodeColsScoped(node ASTNode, editedSheet string, onEditedSheet bool, atCol, delta int) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		return CellRefNode{Ref: mapCellColScoped(n.Ref, editedSheet, onEditedSheet, atCol, delta)}
	case RangeRefNode:
		if n.Ref.IsWholeRow() {
			return n
		}
		return RangeRefNode{Ref: coord.RangeRef{
			Sheet: n.Ref.Sheet,
			Start: mapCellColScoped(n.Ref.Start, editedSheet, onEditedSheet, atCol, delta),
			End:   mapCellColScoped(n.Ref.End, editedSheet, onEditedSheet, atCol, delta),
		}}
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: mapNodeColsScoped(n.Operand, editedSheet, onEditedSheet, atCol, delta)}
	case BinaryOpNode:
		return BinaryOpNode{Op: n.Op, Left: mapNodeColsScoped(n.Left, editedSheet, onEditedSheet, atCol, delta), Right: mapNodeColsScoped(n.Right, editedSheet, onEditedSheet, atCol, delta)}
	case FunctionCallNode:
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			args[i] = mapNodeColsScoped(a, editedSheet, onEditedSheet, atCol, delta)
		}
		return FunctionCallNode{Name: n.Name, Args: args}
	default:
		return node
	}
}

func AdjustFormulaReferencesForRowDeleteOnSheet(formulaText, editedSheet string, onEditedSheet bool, atRow, count int) string {
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	shifted := mapNodeRowsDeleteScoped(ast, editedSheet, onEditedSheet, atRow, count)
	return reprefixFormula(formulaText, NodeToString(shifted))
}

func AdjustFormulaReferencesForColDeleteOnSheet(formulaText, editedSheet string, onEditedSheet bool, atCol, count int) string {
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	shifted := mapNodeColsDeleteScoped(ast, editedSheet, onEditedSheet, atCol, count)
	return reprefixFormula(formulaText, NodeToString(shifted))
}

// RenameSheetReferences updates references from oldName to newName in formulaText.
func RenameSheetReferences(formulaText, oldName, newName string) string {
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	updated := mapNodeSheetRename(ast, oldName, newName)
	return reprefixFormula(formulaText, NodeToString(updated))
}

// InvalidateSheetReferences replaces references to deletedSheet with ERR in formulaText.
func InvalidateSheetReferences(formulaText, deletedSheet string) string {
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	updated := mapNodeSheetInvalidate(ast, deletedSheet)
	return reprefixFormula(formulaText, NodeToString(updated))
}

func mapNodeSheetRename(node ASTNode, oldName, newName string) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		if strings.EqualFold(n.Ref.Sheet, oldName) {
			ref := n.Ref
			ref.Sheet = newName
			return CellRefNode{Ref: ref}
		}
		return n
	case RangeRefNode:
		targetSheet := n.Ref.Sheet
		if targetSheet == "" {
			targetSheet = n.Ref.Start.Sheet
		}
		if strings.EqualFold(targetSheet, oldName) {
			ref := n.Ref
			ref.Sheet = newName
			ref.Start.Sheet = newName
			ref.End.Sheet = newName
			return RangeRefNode{Ref: ref}
		}
		return n
	case IdentifierNode:
		sheetQual, ident := splitQualifiedIdent(n.Name)
		if sheetQual != "" && strings.EqualFold(sheetQual, oldName) {
			return IdentifierNode{Name: coord.QuoteSheetPrefix(newName) + ident}
		}
		return n
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: mapNodeSheetRename(n.Operand, oldName, newName)}
	case BinaryOpNode:
		return BinaryOpNode{Op: n.Op, Left: mapNodeSheetRename(n.Left, oldName, newName), Right: mapNodeSheetRename(n.Right, oldName, newName)}
	case FunctionCallNode:
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			args[i] = mapNodeSheetRename(a, oldName, newName)
		}
		return FunctionCallNode{Name: n.Name, Args: args}
	default:
		return node
	}
}

func mapNodeSheetInvalidate(node ASTNode, deletedSheet string) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		if strings.EqualFold(n.Ref.Sheet, deletedSheet) {
			return ErrorNode{Code: "#REF!"}
		}
		return n
	case RangeRefNode:
		targetSheet := n.Ref.Sheet
		if targetSheet == "" {
			targetSheet = n.Ref.Start.Sheet
		}
		if strings.EqualFold(targetSheet, deletedSheet) {
			return ErrorNode{Code: "#REF!"}
		}
		return n
	case IdentifierNode:
		sheetQual, _ := splitQualifiedIdent(n.Name)
		if sheetQual != "" && strings.EqualFold(sheetQual, deletedSheet) {
			return ErrorNode{Code: "#REF!"}
		}
		return n
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: mapNodeSheetInvalidate(n.Operand, deletedSheet)}
	case BinaryOpNode:
		return BinaryOpNode{Op: n.Op, Left: mapNodeSheetInvalidate(n.Left, deletedSheet), Right: mapNodeSheetInvalidate(n.Right, deletedSheet)}
	case FunctionCallNode:
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			args[i] = mapNodeSheetInvalidate(a, deletedSheet)
		}
		return FunctionCallNode{Name: n.Name, Args: args}
	default:
		return node
	}
}

func mapCellRowDeleteScoped(ref coord.CellRef, editedSheet string, onEditedSheet bool, atRow, count int) ASTNode {
	if !refBelongsToEditedSheet(ref, editedSheet, onEditedSheet) {
		return CellRefNode{Ref: ref}
	}
	if ref.Row >= atRow && ref.Row < atRow+count {
		return ErrorNode{Code: "#REF!"}
	}
	if ref.Row >= atRow+count {
		ref.Row -= count
	}
	return CellRefNode{Ref: ref}
}

func mapCellColDeleteScoped(ref coord.CellRef, editedSheet string, onEditedSheet bool, atCol, count int) ASTNode {
	if !refBelongsToEditedSheet(ref, editedSheet, onEditedSheet) {
		return CellRefNode{Ref: ref}
	}
	if ref.Col >= atCol && ref.Col < atCol+count {
		return ErrorNode{Code: "#REF!"}
	}
	if ref.Col >= atCol+count {
		ref.Col -= count
	}
	return CellRefNode{Ref: ref}
}

func mapNodeRowsDeleteScoped(node ASTNode, editedSheet string, onEditedSheet bool, atRow, count int) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		return mapCellRowDeleteScoped(n.Ref, editedSheet, onEditedSheet, atRow, count)
	case RangeRefNode:
		if n.Ref.IsWholeColumn() {
			return n
		}
		if !refBelongsToEditedSheet(n.Ref.Start, editedSheet, onEditedSheet) &&
			!refBelongsToEditedSheet(n.Ref.End, editedSheet, onEditedSheet) {
			return n
		}
		if shrunk, ok := ShrinkRangeForRowDelete(n.Ref, atRow, count); ok {
			return RangeRefNode{Ref: shrunk}
		}
		return ErrorNode{Code: "#REF!"}
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: mapNodeRowsDeleteScoped(n.Operand, editedSheet, onEditedSheet, atRow, count)}
	case BinaryOpNode:
		return BinaryOpNode{Op: n.Op, Left: mapNodeRowsDeleteScoped(n.Left, editedSheet, onEditedSheet, atRow, count), Right: mapNodeRowsDeleteScoped(n.Right, editedSheet, onEditedSheet, atRow, count)}
	case FunctionCallNode:
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			args[i] = mapNodeRowsDeleteScoped(a, editedSheet, onEditedSheet, atRow, count)
		}
		return FunctionCallNode{Name: n.Name, Args: args}
	default:
		return node
	}
}

func mapNodeColsDeleteScoped(node ASTNode, editedSheet string, onEditedSheet bool, atCol, count int) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		return mapCellColDeleteScoped(n.Ref, editedSheet, onEditedSheet, atCol, count)
	case RangeRefNode:
		if n.Ref.IsWholeRow() {
			return n
		}
		if !refBelongsToEditedSheet(n.Ref.Start, editedSheet, onEditedSheet) &&
			!refBelongsToEditedSheet(n.Ref.End, editedSheet, onEditedSheet) {
			return n
		}
		if shrunk, ok := ShrinkRangeForColDelete(n.Ref, atCol, count); ok {
			return RangeRefNode{Ref: shrunk}
		}
		return ErrorNode{Code: "#REF!"}
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: mapNodeColsDeleteScoped(n.Operand, editedSheet, onEditedSheet, atCol, count)}
	case BinaryOpNode:
		return BinaryOpNode{Op: n.Op, Left: mapNodeColsDeleteScoped(n.Left, editedSheet, onEditedSheet, atCol, count), Right: mapNodeColsDeleteScoped(n.Right, editedSheet, onEditedSheet, atCol, count)}
	case FunctionCallNode:
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			args[i] = mapNodeColsDeleteScoped(a, editedSheet, onEditedSheet, atCol, count)
		}
		return FunctionCallNode{Name: n.Name, Args: args}
	default:
		return node
	}
}

func ShrinkRangeForRowDelete(r coord.RangeRef, atRow, count int) (coord.RangeRef, bool) {
	delLo, delHi := atRow, atRow+count-1
	start, end := r.Start, r.End
	sRow, eRow := start.Row, end.Row
	lo, hi := sRow, eRow
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo >= delLo && hi <= delHi {
		return r, false
	}
	clamp := func(row int) int {
		if row >= delLo && row <= delHi {
			if row == lo {
				return delHi + 1
			}
			return delLo - 1
		}
		return row
	}
	start.Row = clamp(start.Row)
	end.Row = clamp(end.Row)
	if start.Row >= atRow+count {
		start.Row -= count
	}
	if end.Row >= atRow+count {
		end.Row -= count
	}
	if start.Row < 0 || end.Row < 0 {
		return r, false
	}
	return coord.RangeRef{Sheet: r.Sheet, Start: start, End: end}, true
}

func ShrinkRangeForColDelete(r coord.RangeRef, atCol, count int) (coord.RangeRef, bool) {
	delLo, delHi := atCol, atCol+count-1
	start, end := r.Start, r.End
	sCol, eCol := start.Col, end.Col
	lo, hi := sCol, eCol
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo >= delLo && hi <= delHi {
		return r, false
	}
	clamp := func(col int) int {
		if col >= delLo && col <= delHi {
			if col == lo {
				return delHi + 1
			}
			return delLo - 1
		}
		return col
	}
	start.Col = clamp(start.Col)
	end.Col = clamp(end.Col)
	if start.Col >= atCol+count {
		start.Col -= count
	}
	if end.Col >= atCol+count {
		end.Col -= count
	}
	if start.Col < 0 || end.Col < 0 {
		return r, false
	}
	return coord.RangeRef{Sheet: r.Sheet, Start: start, End: end}, true
}

func RetargetFormulaReferencesOnSheet(formulaText string, fromR coord.RangeRef, editedSheet string, onEditedSheet bool, dCol, dRow int) string {
	return RetargetFormulaReferencesToSheet(formulaText, fromR, editedSheet, onEditedSheet, dCol, dRow, editedSheet, onEditedSheet)
}

// RetargetFormulaReferencesToSheet is like OnSheet but also rewrites the sheet of moved refs when destSheet differs.
func RetargetFormulaReferencesToSheet(formulaText string, fromR coord.RangeRef, srcSheet string, onSrcSheet bool, dCol, dRow int, destSheet string, onDestSheet bool) string {
	if dCol == 0 && dRow == 0 && strings.EqualFold(srcSheet, destSheet) {
		return formulaText
	}
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	updated := retargetNode(ast, fromR, srcSheet, onSrcSheet, dCol, dRow, destSheet, onDestSheet)
	return reprefixFormula(formulaText, NodeToString(updated))
}

// QualifyFormulaReferences qualifies unqualified CellRef, RangeRef, and named
// identifiers in formulaText with sheetName.
func QualifyFormulaReferences(formulaText, sheetName string) string {
	if sheetName == "" {
		return formulaText
	}
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText
	}
	updated := qualifyNode(ast, sheetName)
	return reprefixFormula(formulaText, NodeToString(updated))
}

func qualifyNode(node ASTNode, sheetName string) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		if n.Ref.Sheet == "" {
			ref := n.Ref
			ref.Sheet = sheetName
			return CellRefNode{Ref: ref}
		}
		return n
	case RangeRefNode:
		ref := n.Ref
		if ref.Sheet == "" && ref.Start.Sheet == "" {
			ref.Sheet = sheetName
			ref.Start.Sheet = sheetName
			ref.End.Sheet = sheetName
			return RangeRefNode{Ref: ref}
		}
		return n
	case IdentifierNode:
		if isReservedIdent(n.Name) {
			return n
		}
		if sheet, _ := splitQualifiedIdent(n.Name); sheet != "" {
			return n
		}
		return IdentifierNode{Name: coord.QuoteSheetPrefix(sheetName) + n.Name}
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: qualifyNode(n.Operand, sheetName)}
	case BinaryOpNode:
		return BinaryOpNode{Op: n.Op, Left: qualifyNode(n.Left, sheetName), Right: qualifyNode(n.Right, sheetName)}
	case FunctionCallNode:
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			args[i] = qualifyNode(a, sheetName)
		}
		return FunctionCallNode{Name: n.Name, Args: args}
	default:
		return node
	}
}

func retargetNode(node ASTNode, fromR coord.RangeRef, srcSheet string, onSrcSheet bool, dCol, dRow int, destSheet string, onDestSheet bool) ASTNode {
	switch n := node.(type) {
	case CellRefNode:
		if ref, ok := retargetCell(n.Ref, fromR, srcSheet, onSrcSheet, dCol, dRow, destSheet, onDestSheet); ok {
			return CellRefNode{Ref: ref}
		}
		return n
	case RangeRefNode:
		start, sOK := retargetCell(n.Ref.Start, fromR, srcSheet, onSrcSheet, dCol, dRow, destSheet, onDestSheet)
		end, eOK := retargetCell(n.Ref.End, fromR, srcSheet, onSrcSheet, dCol, dRow, destSheet, onDestSheet)
		if sOK && eOK {
			rr := coord.RangeRef{Sheet: n.Ref.Sheet, Start: start, End: end}
			if !strings.EqualFold(srcSheet, destSheet) {
				if onDestSheet {
					rr.Sheet = ""
					start.Sheet = ""
					end.Sheet = ""
				} else {
					rr.Sheet = destSheet
					start.Sheet = destSheet
					end.Sheet = destSheet
				}
				rr.Start, rr.End = start, end
			}
			return RangeRefNode{Ref: rr}
		}
		return n
	case UnaryOpNode:
		return UnaryOpNode{Op: n.Op, Operand: retargetNode(n.Operand, fromR, srcSheet, onSrcSheet, dCol, dRow, destSheet, onDestSheet)}
	case BinaryOpNode:
		return BinaryOpNode{Op: n.Op, Left: retargetNode(n.Left, fromR, srcSheet, onSrcSheet, dCol, dRow, destSheet, onDestSheet), Right: retargetNode(n.Right, fromR, srcSheet, onSrcSheet, dCol, dRow, destSheet, onDestSheet)}
	case FunctionCallNode:
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			args[i] = retargetNode(a, fromR, srcSheet, onSrcSheet, dCol, dRow, destSheet, onDestSheet)
		}
		return FunctionCallNode{Name: n.Name, Args: args}
	default:
		return node
	}
}

func retargetCell(ref coord.CellRef, fromR coord.RangeRef, srcSheet string, onSrcSheet bool, dCol, dRow int, destSheet string, onDestSheet bool) (coord.CellRef, bool) {
	if !refBelongsToEditedSheet(ref, srcSheet, onSrcSheet) {
		return ref, false
	}
	if !fromR.Contains(ref.Col, ref.Row) {
		return ref, false
	}
	ref.Col += dCol
	ref.Row += dRow
	if ref.Col < 0 || ref.Row < 0 || ref.Col >= maxSheetCols || ref.Row >= maxSheetRows {
		return ref, false
	}
	if !strings.EqualFold(srcSheet, destSheet) {
		if onDestSheet {
			ref.Sheet = ""
		} else {
			ref.Sheet = destSheet
		}
	}
	return ref, true
}

func NodeToString(node ASTNode) string {
	return nodeToStringPrec(node, -999, false)
}

func binOpPrec(op string) int {
	switch op {
	case "^":
		return 5
	case "*", "/":
		return 4
	case "+", "-":
		return 3
	case "&":
		return 2
	case "=", "<>", "<", "<=", ">", ">=":
		return 1
	case "AND", "OR":
		return 0
	default:
		return 0
	}
}

func nodeToStringPrec(node ASTNode, parentPrec int, isRight bool) string {
	switch n := node.(type) {
	case NumberNode:
		return fmt.Sprintf("%g", n.Value)
	case StringNode:
		escaped := strings.ReplaceAll(n.Value, `"`, `""`)
		return fmt.Sprintf(`"%s"`, escaped)
	case CellRefNode:
		return n.Ref.String()
	case RangeRefNode:
		return n.Ref.String()
	case IdentifierNode:
		return n.Name
	case MissingArgNode:
		return ""
	case ErrorNode:
		if n.Code == "" {
			return "#REF!"
		}
		return n.Code
	case UnaryOpNode:
		if n.Op == "NOT" {
			return fmt.Sprintf("#NOT#%s", nodeToStringPrec(n.Operand, -999, false))
		}
		inner := nodeToStringPrec(n.Operand, 6, true)
		if _, ok := n.Operand.(BinaryOpNode); ok {
			return fmt.Sprintf("%s(%s)", n.Op, inner)
		}
		return fmt.Sprintf("%s%s", n.Op, inner)
	case BinaryOpNode:
		prec := binOpPrec(n.Op)
		l := nodeToStringPrec(n.Left, prec, false)
		r := nodeToStringPrec(n.Right, prec, true)
		var body string
		if n.Op == "AND" || n.Op == "OR" {
			body = fmt.Sprintf("%s#%s#%s", l, n.Op, r)
		} else {
			body = fmt.Sprintf("%s%s%s", l, n.Op, r)
		}
		needParen := prec < parentPrec
		if !needParen && isRight && prec == parentPrec {
			// Keep (a-(b-c)) and (a/(b/c)) and (a^(b^c)) unambiguous
			if parentPrec == binOpPrec("-") || parentPrec == binOpPrec("/") || parentPrec == binOpPrec("^") {
				needParen = true
			}
		}
		if needParen || n.Op == "AND" || n.Op == "OR" {
			return "(" + body + ")"
		}
		return body
	case FunctionCallNode:
		var args []string
		for _, arg := range n.Args {
			args = append(args, nodeToStringPrec(arg, -999, false))
		}
		name := n.Name
		if strings.HasPrefix(name, "@") {
			name = name[1:]
		}
		return fmt.Sprintf("%s(%s)", name, strings.Join(args, ","))
	}
	return ""
}

func (e *Evaluator) resolveReference(node ASTNode) (sheet string, col int, row int, isRange bool, rangeRef coord.RangeRef, ok bool) {
	switch n := node.(type) {
	case CellRefNode:
		return n.Ref.Sheet, n.Ref.Col, n.Ref.Row, false, coord.RangeRef{}, true
	case RangeRefNode:
		sheetName := n.Ref.Sheet
		if sheetName == "" {
			sheetName = n.Ref.Start.Sheet
		}
		return sheetName, n.Ref.MinCol(), n.Ref.MinRow(), true, n.Ref, true
	case IdentifierNode:
		if named, eval, exists := e.namedLookup(n.Name); exists {
			if refNode, okRef := namedToRefNode(named, eval); okRef {
				refNode = stampSheetOnRef(refNode, eval.ctx.GetCurrentSheetName())
				return eval.resolveReference(refNode)
			}
		}
	case FunctionCallNode:
		uname := strings.ToUpper(strings.TrimPrefix(n.Name, "@"))
		if uname == "CHOOSE" {
			if len(n.Args) >= 2 {
				idxVal := e.Evaluate(n.Args[0])
				if _, isErr := idxVal.(cell.LotusError); !isErr {
					if idxF, okIdx := toFloat(idxVal); okIdx {
						idx := int(idxF)
						if idx >= 1 && idx < len(n.Args) {
							return e.resolveReference(n.Args[idx])
						}
					}
				}
			}
		}
		if uname == "INDIRECT" {
			if len(n.Args) >= 1 {
				refVal := e.Evaluate(n.Args[0])
				refText := strings.TrimSpace(fmt.Sprintf("%v", unwrapCellSourced(refVal)))
				a1 := e.indirectA1Style(n.Args)
				if node, ok := e.parseIndirectRef(refText, a1); ok {
					return e.resolveReference(node)
				}
				if named, eval, exists := e.namedLookup(refText); exists {
					if refNode, okRef := namedToRefNode(named, eval); okRef {
						refNode = stampSheetOnRef(refNode, eval.ctx.GetCurrentSheetName())
						return eval.resolveReference(refNode)
					}
				}
			}
		}
		if uname == "OFFSET" {
			if len(n.Args) >= 3 && len(n.Args) <= 5 {
				bSheet, bCol, bRow, isRng, oRng, okRef := e.resolveReference(n.Args[0])
				if okRef {
					rOffVal := e.Evaluate(n.Args[1])
					cOffVal := e.Evaluate(n.Args[2])
					rOff, okR := toFloat(rOffVal)
					cOff, okC := toFloat(cOffVal)
					if okR && okC {
						tCol := bCol + int(cOff)
						tRow := bRow + int(rOff)
						h := 1
						w := 1
						if isRng {
							h = oRng.MaxRow() - oRng.MinRow() + 1
							w = oRng.MaxCol() - oRng.MinCol() + 1
						}
						if len(n.Args) >= 4 {
							if hF, okH := toFloat(e.Evaluate(n.Args[3])); okH && int(hF) > 0 {
								h = int(hF)
							}
						}
						if len(n.Args) >= 5 {
							if wF, okW := toFloat(e.Evaluate(n.Args[4])); okW && int(wF) > 0 {
								w = int(wF)
							}
						}
						if tCol >= 0 && tRow >= 0 {
							if h == 1 && w == 1 {
								return bSheet, tCol, tRow, false, coord.RangeRef{}, true
							}
							rng := coord.RangeRef{
								Sheet: bSheet,
								Start: coord.CellRef{Sheet: bSheet, Col: tCol, Row: tRow},
								End:   coord.CellRef{Sheet: bSheet, Col: tCol + w - 1, Row: tRow + h - 1},
							}
							return bSheet, tCol, tRow, true, rng, true
						}
					}
				}
			}
		}
	}

	val := e.Evaluate(node)
	if str, ok := val.(string); ok {
		if cr, err := coord.ParseCellRef(str); err == nil {
			return cr.Sheet, cr.Col, cr.Row, false, coord.RangeRef{}, true
		}
		if rr, err := coord.ParseRangeRef(str); err == nil {
			return rr.Sheet, rr.MinCol(), rr.MinRow(), true, rr, true
		}
	}
	return "", 0, 0, false, coord.RangeRef{}, false
}

func stringifyConcat(v any) string {
	v = unwrapCellSourced(v)
	if v == nil {
		return ""
	}
	if err, ok := v.(cell.LotusError); ok {
		return err.Code
	}
	if f, ok := v.(float64); ok {
		if f == float64(int64(f)) {
			return strconv.FormatInt(int64(f), 10)
		}
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	if b, ok := v.(bool); ok {
		if b {
			return "TRUE"
		}
		return "FALSE"
	}
	return fmt.Sprintf("%v", v)
}

func toArithmeticFloat(v any) (float64, bool) {
	if v == nil {
		return 0.0, true
	}
	return toFloat(v)
}
