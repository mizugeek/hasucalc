package formula

// NamedExpr is a workbook/sheet named formula, optionally authored relative to a base cell
// (ODF table:base-cell-address). Relative refs are shifted by (evalCell - baseCell) on use.
type NamedExpr struct {
	Expr    string
	BaseCol int
	BaseRow int
	HasBase bool
}

func (n NamedExpr) String() string {
	return n.Expr
}
