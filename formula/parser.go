package formula

import (
	"fmt"
	"strconv"
	"strings"

	"hasucalc/coord"
)

type Parser struct {
	tokens []Token
	pos    int
}

func ParseFormula(expr string) (ASTNode, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "=") {
		expr = expr[1:]
	}
	lexer := NewLexer(expr)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}
	parser := &Parser{tokens: tokens, pos: 0}
	return parser.Parse()
}

func (p *Parser) peek() Token {
	if p.pos < len(p.tokens) {
		return p.tokens[p.pos]
	}
	return Token{Type: TokEOF}
}

func (p *Parser) advance() Token {
	tok := p.peek()
	p.pos++
	return tok
}

func (p *Parser) match(types ...TokenType) bool {
	cur := p.peek().Type
	for _, t := range types {
		if cur == t {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) expect(t TokenType, errMsg string) (Token, error) {
	tok := p.peek()
	if tok.Type != t {
		return tok, fmt.Errorf("%s (found %v '%s' at pos %d)", errMsg, tok.Type, tok.Value, tok.Pos)
	}
	return p.advance(), nil
}

func (p *Parser) Parse() (ASTNode, error) {
	node, err := p.exprOr()
	if err != nil {
		return nil, err
	}
	if p.peek().Type != TokEOF {
		return nil, fmt.Errorf("unexpected token '%s' at position %d", p.peek().Value, p.peek().Pos)
	}
	return node, nil
}

// 1. OR
func (p *Parser) exprOr() (ASTNode, error) {
	node, err := p.exprAnd()
	if err != nil {
		return nil, err
	}
	for p.match(TokOr) {
		right, err := p.exprAnd()
		if err != nil {
			return nil, err
		}
		node = BinaryOpNode{Op: "OR", Left: node, Right: right}
	}
	return node, nil
}

// 2. AND
func (p *Parser) exprAnd() (ASTNode, error) {
	node, err := p.exprComparison()
	if err != nil {
		return nil, err
	}
	for p.match(TokAnd) {
		right, err := p.exprComparison()
		if err != nil {
			return nil, err
		}
		node = BinaryOpNode{Op: "AND", Left: node, Right: right}
	}
	return node, nil
}

// 3. Comparison
func (p *Parser) exprComparison() (ASTNode, error) {
	node, err := p.exprConcat()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		if tok.Type == TokEq || tok.Type == TokNe || tok.Type == TokLt || tok.Type == TokLe || tok.Type == TokGt || tok.Type == TokGe {
			p.advance()
			right, err := p.exprConcat()
			if err != nil {
				return nil, err
			}
			node = BinaryOpNode{Op: tok.Value, Left: node, Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// 4. String concat: &
func (p *Parser) exprConcat() (ASTNode, error) {
	node, err := p.exprAddSub()
	if err != nil {
		return nil, err
	}
	for p.match(TokConcat) {
		right, err := p.exprAddSub()
		if err != nil {
			return nil, err
		}
		node = BinaryOpNode{Op: "&", Left: node, Right: right}
	}
	return node, nil
}

// 5. Addition / Subtraction: +, -
func (p *Parser) exprAddSub() (ASTNode, error) {
	node, err := p.exprMulDiv()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		if tok.Type == TokPlus || tok.Type == TokMinus {
			p.advance()
			right, err := p.exprMulDiv()
			if err != nil {
				return nil, err
			}
			node = BinaryOpNode{Op: tok.Value, Left: node, Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// 6. Multiplication / Division: *, /
func (p *Parser) exprMulDiv() (ASTNode, error) {
	node, err := p.exprPow()
	if err != nil {
		return nil, err
	}
	for {
		tok := p.peek()
		if tok.Type == TokMul || tok.Type == TokDiv {
			p.advance()
			right, err := p.exprPow()
			if err != nil {
				return nil, err
			}
			node = BinaryOpNode{Op: tok.Value, Left: node, Right: right}
		} else {
			break
		}
	}
	return node, nil
}

// 7. Power: ^ (right-associative). OpenFormula / Excel documented precedence:
// unary minus binds tighter than ^, so -2^2 = (-2)^2 = 4. Use -(2^2) for -4.
func (p *Parser) exprPow() (ASTNode, error) {
	node, err := p.exprUnary()
	if err != nil {
		return nil, err
	}
	if p.match(TokPow) {
		right, err := p.exprPow()
		if err != nil {
			return nil, err
		}
		return BinaryOpNode{Op: "^", Left: node, Right: right}, nil
	}
	return node, nil
}

// 8. Unary: +, -, NOT — tighter than power, so -2^2 is (-2)^2.
func (p *Parser) exprUnary() (ASTNode, error) {
	if p.match(TokPlus) {
		operand, err := p.exprUnary()
		if err != nil {
			return nil, err
		}
		return UnaryOpNode{Op: "+", Operand: operand}, nil
	}
	if p.match(TokMinus) {
		operand, err := p.exprUnary()
		if err != nil {
			return nil, err
		}
		return UnaryOpNode{Op: "-", Operand: operand}, nil
	}
	if p.match(TokNot) {
		operand, err := p.exprUnary()
		if err != nil {
			return nil, err
		}
		return UnaryOpNode{Op: "NOT", Operand: operand}, nil
	}
	return p.exprPrimary()
}

// 9. Primary
func (p *Parser) exprPrimary() (ASTNode, error) {
	tok := p.peek()

	// Number
	if p.match(TokNumber) {
		val, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number '%s'", tok.Value)
		}
		return NumberNode{Value: val}, nil
	}

	// String
	if p.match(TokString) {
		return StringNode{Value: tok.Value}, nil
	}

	// Function: @SUM(arg1, arg2...)
	if p.match(TokFunction) {
		fnName := tok.Value
		var args []ASTNode
		if p.match(TokLParen) {
			if p.peek().Type != TokRParen {
				for {
					if p.peek().Type == TokComma || p.peek().Type == TokSemicolon {
						args = append(args, MissingArgNode{})
					} else if p.peek().Type == TokRParen {
						break
					} else {
						arg, err := p.exprOr()
						if err != nil {
							return nil, err
						}
						args = append(args, arg)
					}
					if !p.match(TokComma, TokSemicolon) {
						break
					}
				}
			}
			if _, err := p.expect(TokRParen, "Expected ')' after function arguments"); err != nil {
				return nil, err
			}
		}
		return FunctionCallNode{Name: fnName, Args: args}, nil
	}

	// Range reference token: A1..B10
	if p.match(TokRangeRef) {
		ref, err := coord.ParseRangeRef(tok.Value)
		if err != nil {
			return nil, err
		}
		return RangeRefNode{Ref: ref}, nil
	}

	// Cell reference token: A1
	if p.match(TokCellRef) {
		cellRef, err := coord.ParseCellRef(tok.Value)
		if err != nil {
			return nil, err
		}
		if p.match(TokDotDot) {
			endTok := p.peek()
			if p.match(TokCellRef, TokIdentifier) {
				endRef, err := coord.ParseCellRef(endTok.Value)
				if err != nil {
					return nil, err
				}
				return RangeRefNode{Ref: rangeRefFromCells(cellRef, endRef)}, nil
			}
			return nil, fmt.Errorf("expected cell reference after '..'")
		}
		return CellRefNode{Ref: cellRef}, nil
	}

	// Identifier
	if p.match(TokIdentifier) {
		name := tok.Value
		if p.match(TokDotDot) {
			endTok := p.peek()
			if p.match(TokCellRef, TokIdentifier) {
				c1, err1 := coord.ParseCellRef(name)
				c2, err2 := coord.ParseCellRef(endTok.Value)
				if err1 == nil && err2 == nil {
					return RangeRefNode{Ref: rangeRefFromCells(c1, c2)}, nil
				}
			}
		}
		return IdentifierNode{Name: name}, nil
	}

	// Parenthesized: (expr)
	if p.match(TokLParen) {
		node, err := p.exprOr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokRParen, "Expected ')'"); err != nil {
			return nil, err
		}
		return node, nil
	}

	return nil, fmt.Errorf("unexpected token '%s' (%v) at position %d", tok.Value, tok.Type, tok.Pos)
}

func rangeRefFromCells(start, end coord.CellRef) coord.RangeRef {
	sheet := start.Sheet
	if sheet == "" {
		sheet = end.Sheet
	}
	start.Sheet = sheet
	end.Sheet = sheet
	return coord.RangeRef{Sheet: sheet, Start: start, End: end}
}
