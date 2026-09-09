package formula

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"hasucalc/coord"
)

type TokenType int

const (
	TokEOF TokenType = iota
	TokNumber
	TokString
	TokCellRef
	TokRangeRef
	TokFunction   // @SUM, @IF, etc.
	TokIdentifier // range names, etc.
	TokPlus       // +
	TokMinus      // -
	TokMul        // *
	TokDiv        // /
	TokPow        // ^
	TokConcat     // &
	TokEq         // =
	TokNe         // <>
	TokLt         // <
	TokLe         // <=
	TokGt         // >
	TokGe         // >=
	TokAnd        // #AND#
	TokOr         // #OR#
	TokNot        // #NOT#
	TokComma      // ,
	TokSemicolon  // ;
	TokDotDot     // ..
	TokLParen     // (
	TokRParen     // )
)

type Token struct {
	Type  TokenType
	Value string
	Pos   int
}

type Lexer struct {
	input []rune
	pos   int
}

func NewLexer(input string) *Lexer {
	return &Lexer{input: []rune(input), pos: 0}
}

func (l *Lexer) peek() rune {
	if l.pos < len(l.input) {
		return l.input[l.pos]
	}
	return 0
}

func (l *Lexer) peekNext(n int) rune {
	if l.pos+n < len(l.input) {
		return l.input[l.pos+n]
	}
	return 0
}

func (l *Lexer) advance() rune {
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}
}

func (l *Lexer) Tokenize() ([]Token, error) {
	var tokens []Token

	for l.pos < len(l.input) {
		l.skipWhitespace()
		if l.pos >= len(l.input) {
			break
		}

		startPos := l.pos
		ch := l.peek()

		// Number: digits
		if unicode.IsDigit(ch) || (ch == '.' && unicode.IsDigit(l.peekNext(1))) {
			var num []rune
			hasDot := false
			for l.pos < len(l.input) {
				c := l.peek()
				if unicode.IsDigit(c) {
					num = append(num, l.advance())
				} else if c == '.' && !hasDot && l.peekNext(1) != '.' {
					hasDot = true
					num = append(num, l.advance())
				} else if (c == 'e' || c == 'E') && (l.peekNext(1) == '+' || l.peekNext(1) == '-' || unicode.IsDigit(l.peekNext(1))) {
					num = append(num, l.advance())
					if l.peek() == '+' || l.peek() == '-' {
						num = append(num, l.advance())
					}
				} else {
					break
				}
			}

			// Percent literal: 50% → 0.5
			if l.peek() == '%' {
				l.advance()
				if v, err := strconv.ParseFloat(string(num), 64); err == nil {
					tokens = append(tokens, Token{Type: TokNumber, Value: strconv.FormatFloat(v/100.0, 'g', -1, 64), Pos: startPos})
					continue
				}
			}

			// Check if integer is followed by : or .. to form a whole-row RangeRef (e.g. 1:10 or 1..10)
			if !hasDot && ((l.peek() == ':' && (unicode.IsDigit(l.peekNext(1)) || l.peekNext(1) == '$' || unicode.IsSpace(l.peekNext(1)))) ||
				(l.peek() == '.' && l.peekNext(1) == '.' && (unicode.IsDigit(l.peekNext(2)) || l.peekNext(2) == '$' || unicode.IsSpace(l.peekNext(2))))) {
				sep := ".."
				if l.peek() == ':' {
					l.advance()
				} else {
					l.advance()
					l.advance()
				}
				l.skipWhitespace()
				var endNum []rune
				if l.peek() == '$' {
					endNum = append(endNum, l.advance())
				}
				for l.pos < len(l.input) && unicode.IsDigit(l.peek()) {
					endNum = append(endNum, l.advance())
				}
				rangeStr := string(num) + sep + string(endNum)
				tokens = append(tokens, Token{Type: TokRangeRef, Value: rangeStr, Pos: startPos})
				continue
			}

			tokens = append(tokens, Token{Type: TokNumber, Value: string(num), Pos: startPos})
			continue
		}

		// Two dots: ..
		if ch == '.' && l.peekNext(1) == '.' {
			l.advance()
			l.advance()
			tokens = append(tokens, Token{Type: TokDotDot, Value: "..", Pos: startPos})
			continue
		}

		// Function: @SUM or @RANK.EQ
		if ch == '@' {
			l.advance()
			var fn []rune
			for l.pos < len(l.input) && (unicode.IsLetter(l.input[l.pos]) || unicode.IsDigit(l.input[l.pos]) || l.input[l.pos] == '_' || (l.input[l.pos] == '.' && l.peekNext(1) != '.')) {
				fn = append(fn, l.advance())
			}
			tokens = append(tokens, Token{Type: TokFunction, Value: normalizeFunctionName(string(fn)), Pos: startPos})
			continue
		}

		// Logical words #AND#, #OR#, #NOT# and Excel-style #REF!
		if ch == '#' {
			// #REF! (and similar #N/A) before #WORD# Lotus operators
			rest := string(l.input[l.pos:])
			upper := strings.ToUpper(rest)
			excelErrs := []string{"#REF!", "#N/A", "#VALUE!", "#DIV/0!", "#NAME?", "#NULL!", "#NUM!"}
			matched := false
			for _, ee := range excelErrs {
				if strings.HasPrefix(upper, ee) {
					for i := 0; i < len([]rune(ee)); i++ {
						l.advance()
					}
					tokVal := ee
					if ee == "#N/A" {
						tokVal = "NA"
					} else if ee == "#REF!" {
						tokVal = "#REF!"
					} else {
						tokVal = "ERR"
					}
					tokens = append(tokens, Token{Type: TokIdentifier, Value: tokVal, Pos: startPos})
					matched = true
					break
				}
			}
			if matched {
				continue
			}
			l.advance()
			var word []rune
			for l.pos < len(l.input) && l.peek() != '#' {
				word = append(word, l.advance())
			}
			if l.pos < len(l.input) && l.peek() == '#' {
				l.advance()
			}
			op := strings.ToUpper(string(word))
			switch op {
			case "AND":
				tokens = append(tokens, Token{Type: TokAnd, Value: "#AND#", Pos: startPos})
			case "OR":
				tokens = append(tokens, Token{Type: TokOr, Value: "#OR#", Pos: startPos})
			case "NOT":
				tokens = append(tokens, Token{Type: TokNot, Value: "#NOT#", Pos: startPos})
			case "LE":
				tokens = append(tokens, Token{Type: TokLe, Value: "<=", Pos: startPos})
			case "GE":
				tokens = append(tokens, Token{Type: TokGe, Value: ">=", Pos: startPos})
			case "NE":
				tokens = append(tokens, Token{Type: TokNe, Value: "<>", Pos: startPos})
			case "EQ":
				tokens = append(tokens, Token{Type: TokEq, Value: "=", Pos: startPos})
			case "REF!":
				tokens = append(tokens, Token{Type: TokIdentifier, Value: "#REF!", Pos: startPos})
			default:
				tokens = append(tokens, Token{Type: TokIdentifier, Value: "#" + op + "#", Pos: startPos})
			}
			continue
		}

		// String literal: "..."
		if ch == '"' {
			l.advance()
			var str []rune
			terminated := false
			for l.pos < len(l.input) {
				c := l.peek()
				if c == '"' {
					if l.peekNext(1) == '"' {
						l.advance()
						str = append(str, l.advance())
					} else {
						l.advance()
						terminated = true
						break
					}
				} else {
					str = append(str, l.advance())
				}
			}
			if !terminated {
				return nil, fmt.Errorf("unterminated string literal at position %d", startPos)
			}
			tokens = append(tokens, Token{Type: TokString, Value: string(str), Pos: startPos})
			continue
		}

		// Single-quoted sheet name: 'Sheet Name'!A1
		if ch == '\'' {
			l.advance()
			var str []rune
			terminated := false
			for l.pos < len(l.input) {
				if l.peek() == '\'' {
					if l.peekNext(1) == '\'' {
						l.advance()
						str = append(str, l.advance())
					} else {
						l.advance()
						terminated = true
						break
					}
				} else {
					str = append(str, l.advance())
				}
			}
			if !terminated {
				return nil, fmt.Errorf("unterminated single-quoted name at position %d", startPos)
			}
			if l.peek() == '!' {
				l.advance()
				sheetPrefix := coord.QuoteSheetPrefix(string(str))
				var cellPart []rune
				for l.pos < len(l.input) {
					c := l.peek()
					if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '$' || c == '_' {
						cellPart = append(cellPart, l.advance())
					} else {
						break
					}
				}
				cellPartStr := strings.ToUpper(string(cellPart))
				if (l.peek() == '.' && l.peekNext(1) == '.') || (l.peek() == ':' && isCellRefOrCol(cellPartStr)) {
					sep := ".."
					if l.peek() == ':' {
						l.advance()
					} else {
						l.advance()
						l.advance()
					}
					l.skipWhitespace()
					var endIdent []rune
					for l.pos < len(l.input) {
						c := l.peek()
						if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '$' || c == '_' {
							endIdent = append(endIdent, l.advance())
						} else {
							break
						}
					}
					rangeStr := sheetPrefix + cellPartStr + sep + strings.ToUpper(string(endIdent))
					tokens = append(tokens, Token{Type: TokRangeRef, Value: rangeStr, Pos: startPos})
				} else if isCellRef(cellPartStr) {
					tokens = append(tokens, Token{Type: TokCellRef, Value: sheetPrefix + cellPartStr, Pos: startPos})
				} else {
					tokens = append(tokens, Token{Type: TokIdentifier, Value: sheetPrefix + cellPartStr, Pos: startPos})
				}
				continue
			}
			tokens = append(tokens, Token{Type: TokString, Value: string(str), Pos: startPos})
			continue
		}

		// Operators & punctuation
		switch ch {
		case '+':
			l.advance()
			tokens = append(tokens, Token{Type: TokPlus, Value: "+", Pos: startPos})
		case '-':
			l.advance()
			tokens = append(tokens, Token{Type: TokMinus, Value: "-", Pos: startPos})
		case '*':
			l.advance()
			tokens = append(tokens, Token{Type: TokMul, Value: "*", Pos: startPos})
		case '/':
			l.advance()
			tokens = append(tokens, Token{Type: TokDiv, Value: "/", Pos: startPos})
		case '^':
			l.advance()
			tokens = append(tokens, Token{Type: TokPow, Value: "^", Pos: startPos})
		case '&':
			l.advance()
			tokens = append(tokens, Token{Type: TokConcat, Value: "&", Pos: startPos})
		case '(':
			l.advance()
			tokens = append(tokens, Token{Type: TokLParen, Value: "(", Pos: startPos})
		case ')':
			l.advance()
			tokens = append(tokens, Token{Type: TokRParen, Value: ")", Pos: startPos})
		case ',':
			l.advance()
			tokens = append(tokens, Token{Type: TokComma, Value: ",", Pos: startPos})
		case ';':
			l.advance()
			tokens = append(tokens, Token{Type: TokSemicolon, Value: ";", Pos: startPos})
		case ':':
			// Excel-style range separator (also allows "A1 : B10" with spaces)
			l.advance()
			tokens = append(tokens, Token{Type: TokDotDot, Value: ":", Pos: startPos})
		case '=':
			l.advance()
			tokens = append(tokens, Token{Type: TokEq, Value: "=", Pos: startPos})
		case '<':
			l.advance()
			if l.peek() == '>' {
				l.advance()
				tokens = append(tokens, Token{Type: TokNe, Value: "<>", Pos: startPos})
			} else if l.peek() == '=' {
				l.advance()
				tokens = append(tokens, Token{Type: TokLe, Value: "<=", Pos: startPos})
			} else {
				tokens = append(tokens, Token{Type: TokLt, Value: "<", Pos: startPos})
			}
		case '>':
			l.advance()
			if l.peek() == '=' {
				l.advance()
				tokens = append(tokens, Token{Type: TokGe, Value: ">=", Pos: startPos})
			} else {
				tokens = append(tokens, Token{Type: TokGt, Value: ">", Pos: startPos})
			}
		default:
			// Identifiers, Functions, or Cell references ($A$1, A1, A1..B10, Year!B4, ยอดขาย!A1)
			if isIdentStartRune(ch) {
				var ident []rune
				for l.pos < len(l.input) {
					c := l.peek()
					if isIdentRune(c) || (c == '.' && l.peekNext(1) != '.') {
						ident = append(ident, l.advance())
					} else {
						break
					}
				}
				rawIdent := string(ident)
				identStr := strings.ToUpper(rawIdent)

				// Check for Sheet! prefix
				if l.peek() == '!' {
					l.advance()
					sheetPrefix := rawIdent + "!"
					var cellPart []rune
					for l.pos < len(l.input) {
						c := l.peek()
						if isIdentRune(c) {
							cellPart = append(cellPart, l.advance())
						} else {
							break
						}
					}
					cellPartStr := strings.ToUpper(string(cellPart))
					if (l.peek() == '.' && l.peekNext(1) == '.') || (l.peek() == ':' && isCellRefOrCol(cellPartStr)) {
						sep := ".."
						if l.peek() == ':' {
							l.advance()
						} else {
							l.advance()
							l.advance()
						}
						l.skipWhitespace()
						var endIdent []rune
						for l.pos < len(l.input) {
							c := l.peek()
							if isIdentRune(c) {
								endIdent = append(endIdent, l.advance())
							} else {
								break
							}
						}
						rangeStr := sheetPrefix + cellPartStr + sep + strings.ToUpper(string(endIdent))
						tokens = append(tokens, Token{Type: TokRangeRef, Value: rangeStr, Pos: startPos})
					} else if isCellRef(cellPartStr) {
						tokens = append(tokens, Token{Type: TokCellRef, Value: sheetPrefix + cellPartStr, Pos: startPos})
					} else {
						tokens = append(tokens, Token{Type: TokIdentifier, Value: sheetPrefix + cellPartStr, Pos: startPos})
					}
					continue
				}

				// Check if followed by .. or : to form RangeRef token directly
				if (l.peek() == '.' && l.peekNext(1) == '.') || (l.peek() == ':' && isCellRefOrCol(identStr)) {
					sep := ".."
					if l.peek() == ':' {
						l.advance()
					} else {
						l.advance()
						l.advance()
					}
					l.skipWhitespace()
					var endIdent []rune
					for l.pos < len(l.input) {
						c := l.peek()
						if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '$' || c == '_' {
							endIdent = append(endIdent, l.advance())
						} else {
							break
						}
					}
					rangeStr := identStr + sep + strings.ToUpper(string(endIdent))
					tokens = append(tokens, Token{Type: TokRangeRef, Value: rangeStr, Pos: startPos})
				} else if l.peek() == '(' {
					// Function call (e.g. SUM, AVERAGE, IF, LOG10, RANK.EQ)
					fnName := normalizeFunctionName(identStr)
					tokens = append(tokens, Token{Type: TokFunction, Value: fnName, Pos: startPos})
				} else if isCellRef(identStr) {
					tokens = append(tokens, Token{Type: TokCellRef, Value: identStr, Pos: startPos})
				} else {
					tokens = append(tokens, Token{Type: TokIdentifier, Value: identStr, Pos: startPos})
				}
			} else {
				return nil, fmt.Errorf("unexpected character '%c' at position %d", ch, l.pos)
			}
		}
	}

	tokens = append(tokens, Token{Type: TokEOF, Value: "", Pos: l.pos})
	return tokens, nil
}

func normalizeFunctionName(name string) string {
	u := strings.ToUpper(strings.TrimPrefix(name, "@"))
	switch u {
	case "AVERAGE", "AVG":
		return "@AVG"
	case "LEN", "LENGTH":
		return "@LENGTH"
	case "TEXT":
		return "@TEXT"
	case "STRING":
		return "@STRING"
	case "ISERROR":
		return "@ISERROR"
	case "ISERR":
		return "@ISERR"
	case "REPEAT", "REPT":
		return "@REPT"
	case "STD", "STDEV.P", "STDEVP":
		return "@STDEVP"
	case "VARP", "VAR.P":
		return "@VARP"
	case "PAYMT", "PMT":
		return "@PMT"
	default:
		return "@" + u
	}
}

func isCellRefOrCol(s string) bool {
	return isCellRef(s) || isColLetter(s) || isRowNumber(s)
}

func isRowNumber(s string) bool {
	s = strings.TrimPrefix(s, "$")
	if idx := strings.LastIndex(s, "!"); idx != -1 {
		s = s[idx+1:]
		s = strings.TrimPrefix(s, "$")
	}
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isColLetter(s string) bool {
	s = strings.TrimPrefix(s, "$")
	if idx := strings.LastIndex(s, "!"); idx != -1 {
		s = s[idx+1:]
		s = strings.TrimPrefix(s, "$")
	}
	if len(s) == 0 || len(s) > 3 {
		return false
	}
	for _, r := range s {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return false
		}
	}
	return true
}

func isCellRef(s string) bool {
	// e.g. A1, $A$1, $A1, A$1, Year!B4, 'Sheet 1'!A1
	if idx := strings.LastIndex(s, "!"); idx != -1 {
		s = s[idx+1:]
	}
	s = strings.TrimPrefix(s, "$")
	var letters []rune
	var digits []rune
	for _, r := range s {
		if r == '$' {
			continue
		}
		if unicode.IsLetter(r) && len(digits) == 0 {
			letters = append(letters, r)
		} else if unicode.IsDigit(r) {
			digits = append(digits, r)
		} else {
			return false
		}
	}
	return len(letters) > 0 && len(digits) > 0
}

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r) || r == '_' || r == '$'
}

func isIdentStartRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsMark(r) || r == '_' || r == '$'
}
