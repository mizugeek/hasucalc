package formula

import (
	"regexp"
	"strconv"
	"strings"

	"hasucalc/coord"
)

// ReplaceInFormula replaces find with rep in formula AST nodes (numbers, strings,
// whole identifiers, whole cell/range refs, function names). Cell refs such as A1
// are not altered when find is a digit that merely appears inside them.
func ReplaceInFormula(formulaText, find, rep string) (string, bool) {
	if find == "" {
		return formulaText, false
	}
	ast, err := ParseFormula(formulaText)
	if err != nil {
		return formulaText, false
	}
	updated, changed := replaceNode(ast, find, rep)
	if !changed {
		return formulaText, false
	}
	return reprefixFormula(formulaText, NodeToString(updated)), true
}

func replaceNode(node ASTNode, find, rep string) (ASTNode, bool) {
	switch n := node.(type) {
	case NumberNode:
		if fv, err := strconv.ParseFloat(strings.TrimSpace(find), 64); err == nil && fv == n.Value {
			if rv, err2 := strconv.ParseFloat(strings.TrimSpace(rep), 64); err2 == nil {
				return NumberNode{Value: rv}, true
			}
			return StringNode{Value: rep}, true
		}
		return n, false
	case StringNode:
		next := replaceCaseInsensitive(n.Value, find, rep)
		if next == n.Value {
			return n, false
		}
		return StringNode{Value: next}, true
	case CellRefNode:
		if strings.EqualFold(n.Ref.String(), find) {
			if cr, err := coord.ParseCellRef(rep); err == nil {
				return CellRefNode{Ref: cr}, true
			}
		}
		return n, false
	case RangeRefNode:
		if strings.EqualFold(n.Ref.String(), find) {
			if rr, err := coord.ParseRangeRef(rep); err == nil {
				return RangeRefNode{Ref: rr}, true
			}
		}
		return n, false
	case IdentifierNode:
		if strings.EqualFold(n.Name, find) {
			return IdentifierNode{Name: rep}, true
		}
		return n, false
	case ErrorNode:
		if strings.EqualFold(n.Code, find) {
			return ErrorNode{Code: strings.ToUpper(rep)}, true
		}
		return n, false
	case UnaryOpNode:
		inner, ch := replaceNode(n.Operand, find, rep)
		if !ch {
			return n, false
		}
		return UnaryOpNode{Op: n.Op, Operand: inner}, true
	case BinaryOpNode:
		left, lch := replaceNode(n.Left, find, rep)
		right, rch := replaceNode(n.Right, find, rep)
		if !lch && !rch {
			return n, false
		}
		return BinaryOpNode{Op: n.Op, Left: left, Right: right}, true
	case FunctionCallNode:
		changed := false
		name := n.Name
		bare := strings.TrimPrefix(name, "@")
		if strings.EqualFold(bare, find) || strings.EqualFold(name, find) {
			repName := strings.TrimSpace(rep)
			if !strings.HasPrefix(repName, "@") {
				repName = "@" + repName
			}
			name = strings.ToUpper(repName)
			changed = true
		}
		args := make([]ASTNode, len(n.Args))
		for i, a := range n.Args {
			na, ch := replaceNode(a, find, rep)
			args[i] = na
			if ch {
				changed = true
			}
		}
		if !changed {
			return n, false
		}
		return FunctionCallNode{Name: name, Args: args}, true
	default:
		return node, false
	}
}

func replaceCaseInsensitive(src, find, rep string) string {
	if find == "" {
		return src
	}
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(find))
	if err != nil {
		return src
	}
	return re.ReplaceAllLiteralString(src, rep)
}

func isReservedIdent(name string) bool {
	u := strings.ToUpper(strings.TrimSpace(name))
	switch u {
	case "TRUE", "FALSE", "ERR", "NA", "REF",
		"#ERR#", "#N/A", "#NA#", "#NA", "#REF!", "#REF#", "CIRCULAR REF":
		return true
	}
	return false
}

func splitQualifiedIdent(name string) (sheetName, ident string) {
	idx := strings.LastIndex(name, "!")
	if idx < 0 {
		return "", name
	}
	return coord.UnquoteSheetName(name[:idx]), name[idx+1:]
}
