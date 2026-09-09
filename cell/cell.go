package cell

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
)

type Alignment string

const (
	AlignLeft    Alignment = "'"
	AlignRight   Alignment = `"`
	AlignCenter  Alignment = "^"
	AlignRepeat  Alignment = `\`
	AlignDefault Alignment = ""
)

type FormatType string

const (
	FmtGeneral    FormatType = "G"
	FmtFixed      FormatType = "F"
	FmtCurrency   FormatType = "C"
	FmtPercent    FormatType = "P"
	FmtComma      FormatType = ","
	FmtDate       FormatType = "D"
	FmtHidden     FormatType = "H"
	FmtScientific FormatType = "E" // Excel-style scientific; (S) accepted as alias
)

type CellFormat struct {
	Type           FormatType `json:"type"`
	Decimals       int        `json:"decimals"`
	DateFormat     int        `json:"date_format"` // 1: YYYY/MM/DD, 2: DD-MMM-YY, 3: DD-MMM, 4: MM/DD/YY, 5: Month D, YYYY
	CurrencySymbol string     `json:"currency_symbol,omitempty"` // display prefix; empty means "$"
}

// CurrencySymbolOrDefault returns the currency prefix for display ($ when unset).
func (f CellFormat) CurrencySymbolOrDefault() string {
	if f.CurrencySymbol == "" {
		return "$"
	}
	return f.CurrencySymbol
}

func (f CellFormat) String() string {
	switch f.Type {
	case FmtFixed:
		return fmt.Sprintf("(F%d)", f.Decimals)
	case FmtCurrency:
		sym := f.CurrencySymbol
		if sym == "" || sym == "$" {
			return fmt.Sprintf("(C%d)", f.Decimals)
		}
		return fmt.Sprintf("(C%d%s)", f.Decimals, sym)
	case FmtPercent:
		return fmt.Sprintf("(P%d)", f.Decimals)
	case FmtComma:
		return fmt.Sprintf("(,%d)", f.Decimals)
	case FmtDate:
		return fmt.Sprintf("(D%d)", f.DateFormat)
	case FmtHidden:
		return "(H)"
	case FmtScientific:
		return fmt.Sprintf("(E%d)", f.Decimals)
	default:
		return "(G)"
	}
}

func ParseCellFormat(s string) CellFormat {
	s = strings.Trim(strings.TrimSpace(s), "()")
	if len(s) == 0 {
		return CellFormat{Type: FmtGeneral}
	}
	code := FormatType(strings.ToUpper(string(s[0])))
	rest := s[1:]
	arg := 2
	symbol := ""
	if rest != "" {
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		if i > 0 {
			if n, err := strconv.Atoi(rest[:i]); err == nil {
				arg = n
			}
		} else if n, err := strconv.Atoi(rest); err == nil {
			arg = n
			i = len(rest)
		}
		if code == FmtCurrency && i < len(rest) {
			symbol = rest[i:]
		}
	}

	switch code {
	case FmtFixed:
		return CellFormat{Type: FmtFixed, Decimals: arg}
	case FmtCurrency:
		return CellFormat{Type: FmtCurrency, Decimals: arg, CurrencySymbol: symbol}
	case FmtPercent:
		return CellFormat{Type: FmtPercent, Decimals: arg}
	case FmtComma:
		return CellFormat{Type: FmtComma, Decimals: arg}
	case FmtDate:
		df := arg
		if df < 1 || df > 5 {
			df = 1
		}
		return CellFormat{Type: FmtDate, DateFormat: df}
	case FmtHidden:
		return CellFormat{Type: FmtHidden}
	case FmtScientific, FormatType("S"): // (E) Excel; (S) Lotus alias
		return CellFormat{Type: FmtScientific, Decimals: arg}
	default:
		return CellFormat{Type: FmtGeneral}
	}
}

type CellType string

const (
	TypeEmpty   CellType = "EMPTY"
	TypeNumber  CellType = "NUMBER"
	TypeLabel   CellType = "LABEL"
	TypeFormula CellType = "FORMULA"
	TypeBoolean CellType = "BOOLEAN"
)

type LotusError struct {
	Code string
}

func (e LotusError) String() string {
	return e.Code
}

var (
	ErrLotus = LotusError{Code: "ERR"}
	ErrNA    = LotusError{Code: "NA"}
	ErrCirc  = LotusError{Code: "CIRCULAR REF"}
	ErrRef   = LotusError{Code: "#REF!"}
)

type Cell struct {
	RawInput   string      `json:"raw"`
	Type       CellType    `json:"type"`
	Alignment  Alignment   `json:"align"`
	FormatSpec *CellFormat `json:"format,omitempty"`
	Value      any         `json:"-"`
	EvalGen    uint64      `json:"-"`

	cachedRender string
	cachedWidth  int
	cachedFmt    CellFormat
}

var numPattern = regexp.MustCompile(`^[+-]?([0-9]+(\.[0-9]*)?|\.[0-9]+)([eE][+-]?[0-9]+)?$`)

func NewCell(raw string, fmtSpec *CellFormat) *Cell {
	rawTrim := strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "'") && !strings.HasPrefix(raw, `"`) && !strings.HasPrefix(raw, "^") && !strings.HasPrefix(raw, `\`) {
		raw = rawTrim
	}

	if raw == "" {
		return &Cell{
			RawInput:   "",
			Type:       TypeEmpty,
			Alignment:  AlignDefault,
			FormatSpec: fmtSpec,
			Value:      nil,
		}
	}

	// Check Label prefix
	prefix := raw[0:1]
	if prefix == "'" || prefix == `"` || prefix == "^" || prefix == `\` {
		content := raw[1:]
		return &Cell{
			RawInput:   raw,
			Type:       TypeLabel,
			Alignment:  Alignment(prefix),
			FormatSpec: fmtSpec,
			Value:      content,
		}
	}

	// Check Formula prefix (=, +, -, @, (, .)
	isFormula := false
	if strings.HasPrefix(raw, "=") || strings.HasPrefix(raw, "@") {
		isFormula = true
	} else if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
		if numPattern.MatchString(raw) {
			if v, err := strconv.ParseFloat(raw, 64); err == nil {
				if fmtSpec != nil && fmtSpec.Type == FmtPercent {
					v = v / 100.0
				}
				return &Cell{
					RawInput:   raw,
					Type:       TypeNumber,
					Alignment:  AlignDefault,
					FormatSpec: fmtSpec,
					Value:      v,
				}
			}
		}
		if len(raw) > 1 {
			isFormula = true
		}
	} else if strings.HasPrefix(raw, "(") {
		if isFormulaCandidate(raw) {
			isFormula = true
		}
	} else if strings.HasPrefix(raw, ".") && len(raw) > 1 && (raw[1] < '0' || raw[1] > '9') {
		if isFormulaCandidate(raw) {
			isFormula = true
		}
	}

	if isFormula {
		return &Cell{
			RawInput:   raw,
			Type:       TypeFormula,
			Alignment:  AlignDefault,
			FormatSpec: fmtSpec,
			Value:      nil,
		}
	}

	// Percent literal: 50% → 0.5 (Excel-like)
	if strings.HasSuffix(raw, "%") {
		core := strings.TrimSpace(strings.TrimSuffix(raw, "%"))
		if numPattern.MatchString(core) {
			if v, err := strconv.ParseFloat(core, 64); err == nil {
				return &Cell{
					RawInput:   raw,
					Type:       TypeNumber,
					Alignment:  AlignDefault,
					FormatSpec: fmtSpec,
					Value:      v / 100.0,
				}
			}
		}
	}

	// Plain number
	if numPattern.MatchString(raw) {
		if v, err := strconv.ParseFloat(raw, 64); err == nil {
			// Excel-like: typing into a percent-formatted cell stores value/100
			if fmtSpec != nil && fmtSpec.Type == FmtPercent {
				v = v / 100.0
			}
			return &Cell{
				RawInput:   raw,
				Type:       TypeNumber,
				Alignment:  AlignDefault,
				FormatSpec: fmtSpec,
				Value:      v,
			}
		}
	}

	// Boolean literals: TRUE / FALSE (Excel-like)
	upper := strings.ToUpper(raw)
	if upper == "TRUE" {
		return NewBooleanCell(true, fmtSpec)
	}
	if upper == "FALSE" {
		return NewBooleanCell(false, fmtSpec)
	}

	// Default label (Left aligned)
	return &Cell{
		RawInput:   "'" + raw,
		Type:       TypeLabel,
		Alignment:  AlignLeft,
		FormatSpec: fmtSpec,
		Value:      raw,
	}
}

// NewBooleanCell creates a boolean cell with centered alignment and boolean type.
func NewBooleanCell(b bool, fmtSpec *CellFormat) *Cell {
	raw := "FALSE"
	if b {
		raw = "TRUE"
	}
	return &Cell{
		RawInput:   raw,
		Type:       TypeBoolean,
		Alignment:  AlignCenter,
		FormatSpec: fmtSpec,
		Value:      b,
	}
}

func isFormulaCandidate(s string) bool {
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		return len(s) > 1
	}
	if strings.HasPrefix(s, "(") {
		// Parenthesized prose like (n/a) or (hello) stays a label.
		if isParenthesizedLabel(s) {
			return false
		}
		return strings.ContainsAny(s, "+-*/^&=<>!:,@")
	}
	if strings.HasPrefix(s, ".") && len(s) > 1 {
		return strings.ContainsAny(s, "+-*/^&=<>!:,@()")
	}
	return false
}

// isParenthesizedLabel reports (word) / (n/a) style annotations without formula structure.
func isParenthesizedLabel(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") || len(s) < 3 {
		return false
	}
	inner := s[1 : len(s)-1]
	if inner == "" {
		return false
	}
	for _, r := range inner {
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
		case r >= '0' && r <= '9':
			return false // numbers → likely formula/quantity
		case r == '/' || r == '-' || r == ' ' || r == '.' || r == '\'' || r == '_':
		default:
			return false
		}
	}
	return true
}

func NewNumberCell(v float64, fmtSpec *CellFormat) *Cell {
	return &Cell{
		RawInput:   fmt.Sprintf("%v", v),
		Type:       TypeNumber,
		Alignment:  AlignDefault,
		FormatSpec: fmtSpec,
		Value:      v,
	}
}

func NewLabelCell(text string, fmtSpec *CellFormat) *Cell {
	raw := text
	align := AlignLeft
	if strings.HasPrefix(raw, "'") || strings.HasPrefix(raw, `"`) || strings.HasPrefix(raw, "^") || strings.HasPrefix(raw, `\`) {
		align = Alignment(raw[0:1])
		content := raw[1:]
		return &Cell{
			RawInput:   raw,
			Type:       TypeLabel,
			Alignment:  align,
			FormatSpec: fmtSpec,
			Value:      content,
		}
	}
	return &Cell{
		RawInput:   "'" + text,
		Type:       TypeLabel,
		Alignment:  AlignLeft,
		FormatSpec: fmtSpec,
		Value:      text,
	}
}

// FormatNumber formats float64 value according to CellFormat.
func FormatNumber(val float64, fmtSpec CellFormat) string {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return "ERR"
	}

	dec := fmtSpec.Decimals
	if dec < 0 {
		dec = 0
	}
	if dec > 15 {
		dec = 15
	}

	switch fmtSpec.Type {
	case FmtFixed:
		return fmt.Sprintf("%.*f", dec, val)

	case FmtCurrency:
		absVal := math.Abs(val)
		str := formatWithCommas(absVal, dec)
		sym := fmtSpec.CurrencySymbolOrDefault()
		if val < 0 {
			return fmt.Sprintf("(%s%s)", sym, str)
		}
		return fmt.Sprintf("%s%s", sym, str)

	case FmtPercent:
		pct := val * 100
		return fmt.Sprintf("%.*f%%", dec, pct)

	case FmtComma:
		absVal := math.Abs(val)
		str := formatWithCommas(absVal, dec)
		if val < 0 {
			return fmt.Sprintf("(%s)", str)
		}
		return str

	case FmtScientific:
		return fmt.Sprintf("%.*e", dec, val)

	case FmtDate:
		dayInt := int(val)
		// Excel's fictional 1900-02-29 (serial 60)
		if dayInt == 60 {
			switch fmtSpec.DateFormat {
			case 2:
				return "29-Feb-00"
			case 3:
				return "29-Feb"
			case 4:
				return "02/29/00"
			case 5:
				return "February 29, 1900"
			default:
				return "1900/02/29"
			}
		}
		base := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
		// Excel / Lotus 1900 leap-year compatibility
		if dayInt > 59 {
			dayInt--
		}
		target := base.AddDate(0, 0, dayInt)
		switch fmtSpec.DateFormat {
		case 1:
			return target.Format("2006/01/02")
		case 2:
			return target.Format("02-Jan-06")
		case 3:
			return target.Format("02-Jan")
		case 4:
			return target.Format("01/02/06")
		case 5:
			return target.Format("January 2, 2006")
		default:
			return target.Format("2006/01/02")
		}

	case FmtHidden:
		return ""

	default: // General
		if val >= math.MinInt64 && val <= math.MaxInt64 && val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'g', 10, 64)
	}
}

func formatWithCommas(val float64, dec int) string {
	sign := ""
	if val < 0 {
		sign = "-"
		val = -val
	}
	parts := strings.Split(fmt.Sprintf("%.*f", dec, val), ".")
	intPart := parts[0]
	var res []rune
	l := len(intPart)
	for i, ch := range intPart {
		if i > 0 && (l-i)%3 == 0 {
			res = append(res, ',')
		}
		res = append(res, ch)
	}
	formattedInt := sign + string(res)
	if len(parts) > 1 && dec > 0 {
		return formattedInt + "." + parts[1]
	}
	return formattedInt
}

func AlignAndPad(text string, width int, align Alignment) string {
	if width <= 0 {
		return ""
	}
	tw := runewidth.StringWidth(text)
	if tw == width {
		return text
	}
	if tw > width {
		text = runewidth.Truncate(text, width, "")
		tw = runewidth.StringWidth(text)
		if tw == width {
			return text
		}
	}

	diff := width - tw
	if diff <= 0 {
		return text
	}
	switch align {
	case AlignRight:
		return strings.Repeat(" ", diff) + text
	case AlignCenter:
		left := diff / 2
		right := diff - left
		return strings.Repeat(" ", left) + text + strings.Repeat(" ", right)
	default: // Left
		return text + strings.Repeat(" ", diff)
	}
}

func (c *Cell) SetValue(v any) {
	c.Value = v
	c.cachedRender = ""
}

func (c *Cell) InvalidateCache() {
	c.cachedRender = ""
}

// Render formats the cell text for grid display given column width.
func (c *Cell) Render(colWidth int, globalFmt CellFormat) string {
	if c == nil || c.Type == TypeEmpty || c.Value == nil {
		return strings.Repeat(" ", colWidth)
	}

	fmtSpec := globalFmt
	if c.FormatSpec != nil {
		fmtSpec = *c.FormatSpec
	}

	if fmtSpec.Type == FmtHidden {
		return strings.Repeat(" ", colWidth)
	}

	if c.cachedRender != "" && c.cachedWidth == colWidth && c.cachedFmt == fmtSpec {
		return c.cachedRender
	}

	res := c.computeRender(colWidth, fmtSpec)
	c.cachedRender = res
	c.cachedWidth = colWidth
	c.cachedFmt = fmtSpec
	return res
}

func (c *Cell) computeRender(colWidth int, fmtSpec CellFormat) string {
	if errVal, ok := c.Value.(LotusError); ok {
		return AlignAndPad(errVal.Code, colWidth, AlignRight)
	}

	if c.Alignment == AlignRepeat {
		strVal := fmt.Sprintf("%v", c.Value)
		if strVal == "" {
			return strings.Repeat(" ", colWidth)
		}
		sw := runewidth.StringWidth(strVal)
		if sw == 0 {
			return strings.Repeat(" ", colWidth)
		}
		repeatCount := (colWidth / sw) + 1
		res := strings.Repeat(strVal, repeatCount)
		return AlignAndPad(runewidth.Truncate(res, colWidth, ""), colWidth, AlignLeft)
	}

	switch v := c.Value.(type) {
	case float64:
		formatted := FormatNumber(v, fmtSpec)
		if runewidth.StringWidth(formatted) > colWidth {
			return strings.Repeat("*", colWidth) // Asterisks on overflow
		}
		align := c.Alignment
		if align == AlignDefault {
			align = AlignRight
		}
		return AlignAndPad(formatted, colWidth, align)

	case int:
		formatted := FormatNumber(float64(v), fmtSpec)
		if runewidth.StringWidth(formatted) > colWidth {
			return strings.Repeat("*", colWidth)
		}
		align := c.Alignment
		if align == AlignDefault {
			align = AlignRight
		}
		return AlignAndPad(formatted, colWidth, align)

	case string:
		align := c.Alignment
		if align == AlignDefault {
			align = AlignLeft
		}
		return AlignAndPad(v, colWidth, align)

	case bool:
		str := "FALSE"
		if v {
			str = "TRUE"
		}
		align := c.Alignment
		if align == AlignDefault {
			align = AlignCenter
		}
		return AlignAndPad(str, colWidth, align)

	default:
		align := c.Alignment
		if align == AlignDefault {
			align = AlignLeft
		}
		return AlignAndPad(fmt.Sprintf("%v", v), colWidth, align)
	}
}

// FormattedValue returns the formatted string value of the cell without column padding.
func (c *Cell) FormattedValue(globalFmt CellFormat) string {
	if c == nil || c.Type == TypeEmpty || c.Value == nil {
		return ""
	}
	fmtSpec := globalFmt
	if c.FormatSpec != nil {
		fmtSpec = *c.FormatSpec
	}
	if errVal, ok := c.Value.(LotusError); ok {
		return errVal.Code
	}
	switch v := c.Value.(type) {
	case float64:
		return FormatNumber(v, fmtSpec)
	case int:
		return FormatNumber(float64(v), fmtSpec)
	case string:
		return v
	case bool:
		if v {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprintf("%v", v)
	}
}
