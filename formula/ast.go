package formula

import (
	"hasucalc/coord"
)

type ASTNode interface {
	isASTNode()
}

type NumberNode struct {
	Value float64
}

func (NumberNode) isASTNode() {}

type StringNode struct {
	Value string
}

func (StringNode) isASTNode() {}

type CellRefNode struct {
	Ref coord.CellRef
}

func (CellRefNode) isASTNode() {}

type RangeRefNode struct {
	Ref coord.RangeRef
}

func (RangeRefNode) isASTNode() {}

type IdentifierNode struct {
	Name string
}

func (IdentifierNode) isASTNode() {}

type UnaryOpNode struct {
	Op      string // "+", "-", "NOT"
	Operand ASTNode
}

func (UnaryOpNode) isASTNode() {}

type BinaryOpNode struct {
	Op    string // "+", "-", "*", "/", "^", "&", "=", "<>", "<", "<=", ">", ">=", "AND", "OR"
	Left  ASTNode
	Right ASTNode
}

func (BinaryOpNode) isASTNode() {}

type FunctionCallNode struct {
	Name string // e.g. "@SUM", "@IF"
	Args []ASTNode
}

func (FunctionCallNode) isASTNode() {}

// MissingArgNode is an omitted function argument (INDEX(A1:B2,,1)).
type MissingArgNode struct{}

func (MissingArgNode) isASTNode() {}

// ErrorNode is a formula error literal such as #REF!.
type ErrorNode struct {
	Code string // e.g. "#REF!", "ERR", "NA"
}

func (ErrorNode) isASTNode() {}
