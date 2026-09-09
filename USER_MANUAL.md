# HasuCalc User Manual

> **Languages:** English (canonical) · [日本語](USER_MANUAL.ja.md)  
> Overview / quick start: [README.md](README.md) · Engineering spec: [SPECIFICATION.md](SPECIFICATION.md)

This manual explains the on-screen UI, status indicators such as `[READY]`, editing modes, files, and every built-in function with parameters. HasuCalc is **not** an Excel-compatible product; `.hwk` / `.hwkz` are authoritative. `=` formulas and `.xlsx` / `.ods` / `.md` / `.html` bridges exist for convenience.

---

## Contents

1. [Screen layout](#1-screen-layout)
2. [Mode indicators (`[READY]`, …)](#2-mode-indicators-ready-)
3. [Status bar](#3-status-bar)
4. [Entering values and formulas](#4-entering-values-and-formulas)
5. [Keybindings](#5-keybindings)
6. [Slash menu and command palette](#6-slash-menu-and-command-palette)
7. [Files and formats](#7-files-and-formats)
8. [Charts](#8-charts)
9. [Function reference](#9-function-reference)
10. [Errors and common messages](#10-errors-and-common-messages)

---

## 1. Screen layout

Typical lines from top to bottom:

| Line | Contents |
|:---|:---|
| **Line 0 (header)** | Active cell address (e.g. `B12:` or `Sheet2!B12:`), optional format `[…]` and column width `[W…]`, then the cell’s raw input. Far right: mode box such as `[READY]`. |
| **Line 1 (edit / menu)** | In READY: preview of the current cell. In INPUT/EDIT/POINT: the text you are typing. In MENU: slash-menu bar. In PROMPT: prompt + answer. |
| **Line 2** | Column letters (`A`, `B`, …). Frozen panes stay visible while scrolling. |
| **Grid** | Row numbers on the left; cells in the middle. Blue highlight = selection. |
| **Sheet tabs** (multi-sheet) | Click or `Ctrl+T` / `Ctrl+PgUp` / `Ctrl+PgDn` to switch. |
| **Bottom status bar** | Filename (and sheet index), clock, optional status message, and right-side hints / `[CALC]`. |

Press **F1** for the in-app help screens.

---

## 2. Mode indicators (`[READY]`, …)

The top-right box shows the current interaction mode:

| Indicator | Meaning |
|:---|:---|
| **`[READY]`** | Idle. Arrow keys move the cursor; typing starts INPUT; `/` opens the menu; most shortcuts work. |
| **`[INPUT]`** | Entering a **new** value into the current cell (started by typing). Confirm with Enter; cancel with Esc. |
| **`[EDIT]`** | Editing the existing cell on the formula line (`F2` / `Ctrl+E`). Cursor edits the buffer. |
| **`[POINT]`** | While writing a formula, you pointed at cells/ranges with the cursor (or mouse) to insert references. |
| **`[MENU]`** | Slash menu (`/`) is open. |
| **`[PROMPT]`** | Waiting for typed input to a prompt (find, goto, format symbol, …). |
| **`[END]`** | Lotus-style End mode: press End, then an arrow, to jump to the edge of the current data block. |

---

## 3. Status bar

| Element | Meaning |
|:---|:---|
| Filename | Current suggested/saved name (often `DATA.hwk`). With multiple sheets: `file.hwk [SheetName] (i/n)`. |
| Clock | Local date/time. |
| Center message | Temporary feedback (saved, error, find result, …). |
| **`[CALC]`** | Workbook needs recalculation (e.g. MANUAL mode or pending work). Press **F9** to recalculate. When idle, the right side shows shortcut hints instead. |

---

## 4. Entering values and formulas

### Value types

| Prefix / form | Result |
|:---|:---|
| Number | `123`, `45.67`, `1e5`, `-0.05`, `50%` (literal percent → 0.5) |
| `'text` | Label, left-aligned |
| `"text` | Label, right-aligned |
| `^text` | Label, centered |
| `\=` or `\-` | Repeat fill across the cell width |
| Formula | Starts with `=`, `@`, or `+` |

### Formula tips

- Both `=SUM(A1:B10)` and `@SUM(A1..B10)` work; Excel-style `=` is for convenience, not compatibility.
- Ranges: `A1:B10` or `A1..B10`; whole column `A:A` is limited to the used range.
- Cross-sheet: `Sheet2!A1` or `'Q1-2024'!A1`.
- Absolute / mixed: `$A$1`, `$A1`, `A$1`.
- Booleans: `TRUE` / `FALSE` (and `TRUE()` / `FALSE()`).
- Lotus logicals: `#AND#`, `#OR#`, `#NOT#`.
- Unary minus binds tighter than `^`: `-2^2` → `4`.

Open the function browser with **`/IF`** (Insert → Function) or the palette.

---

## 5. Keybindings

| Key | Action |
|:---|:---|
| Arrows | Move cursor |
| Shift+Arrows | Extend selection |
| PageUp / PageDn | Scroll ~20 rows |
| Home | Jump to A1 |
| End, then Arrow | Jump to data-block edge (`[END]`) |
| Enter | Confirm input / menu |
| Esc | Cancel / clear selection / close UI |
| F1 | Help |
| F2 / Ctrl+E | EDIT current cell |
| F3 / Shift+F3 | Find next / previous |
| F5 / Ctrl+G | Goto cell, range, sheet, or name |
| F9 | Recalculate workbook |
| F10 | Chart view (`S` saves PNG) |
| Ctrl+C / X / V | Copy / Cut / Paste |
| Ctrl+L | Paste link (`=Sheet!A1` style) |
| Ctrl+Z / Y | Undo / Redo |
| Ctrl+F / H | Find / Replace |
| Ctrl+S / O | Save / Open |
| Ctrl+K or `:` | Command palette |
| Ctrl+A | Select used region |
| Ctrl+T | Sheet picker |
| Ctrl+PgUp / PgDn | Prev / next sheet |
| Ctrl+D / R | Fill down / right |
| Alt+= | AutoSum |
| Delete / Backspace | Clear cell or selection |
| Ctrl+Q | Quit (confirms if dirty) |
| `/` | Slash menu |
| Mouse click / drag / wheel | Move, select, scroll; click sheet tabs |

---

## 6. Slash menu and command palette

### Slash menu (`/`)

Type `/` then letter keys (or use arrows). Common paths:

| Path | Purpose |
|:---|:---|
| `/FN` `/FO` `/FS` `/FQ` | New / Open / Save / Quit |
| `/FX` | Export → CSV / Excel / ODS / Markdown → Sheet or Range |
| `/HU` `/HR` `/HX` `/HC` `/HV` | Undo / Redo / Cut / Copy / Paste |
| `/HS` | Paste special (values / link / transpose) |
| `/HF` `/HE` `/HG` | Find / Replace / Goto |
| `/HM` | Number formats (currency, %, …) |
| `/IF` | Function browser |
| `/OS` / `/O9` | AutoSum / Recalculate |
| `/DS` `/DA` `/DF` `/DT` | Sort / AutoFill / Fill / Transpose |
| `/VF` | Freeze panes |
| `/CV` `/CT` `/CP` | Chart view / type / save PNG |
| `/?K` | Keybindings help |

### Command palette (`Ctrl+K` / `:`)

Fuzzy search for actions (e.g. `sum`, `currency`, `graph`, `csv export`).

---

## 7. Files and formats

| Format | Role |
|:---|:---|
| **`.hwk`** | Default native workbook (compact JSON). Preferred for Git / LLM. |
| **`.hwkz` / `.hwk.gz`** | Same JSON, gzip-compressed. |
| **`.xlsx` / `.xlsm` / `.ods`** | Tabular import/export bridge (no charts). |
| **`.csv` / `.tsv`** | Delimited text; CSV export adds UTF-8 BOM. |
| **`.md` / `.html`** | **Import**: tables → grid; other text → column-A labels. Markdown can also be **exported** as a GFM table. |

Open via CLI (`hasucalc file.md`) or **Ctrl+O** (lists `.md` / `.html` among other openable types).

---

## 8. Charts

- One chart configuration per sheet (series A–F): Line, Bar, Stacked, Pie.
- **F10** full-screen terminal preview; **S** writes a 1280×720 PNG.
- Chart settings persist in `.hwk` / `.hwkz` only — not in Excel/ODS.

---

## 9. Function reference

Functions accept either `@NAME(...)` or `NAME(...)` after `=`. Arguments in `[brackets]` are optional. Trailing `...` means you may repeat that argument.

The **Parameters** column explains what to put in each slot.

### Math/Agg

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `SUM` | `SUM(range/list)` | `range/list`: cell range and/or individual numbers | Calculates total sum of numbers in range |
| `SUMIF` | `SUMIF(range, criteria, [sum_range])` | `range`: cell range<br>`criteria`: condition such as ">50", "Apple", or a cell<br>`sum_range` (optional): optional cells to sum (defaults to range) | Sums cells meeting specified criteria (e.g. ">50", "Apple") |
| `SUMIFS` | `SUMIFS(sum_rng, crit_rng1, crit1, ...)` | `sum_rng`: cells to sum when criteria match<br>`crit_rng1`: first criteria range (same shape as related ranges)<br>`crit1`: first criteria value/expression<br>`array2` (repeatable): additional range/array (same shape) | Calculates sum of products of corresponding items |
| `PRODUCT` | `PRODUCT(number1, [number2]...)` | `number1`: first number<br>`number2` (repeatable): additional number | Multiplies all numbers given in arguments |
| `SUBTOTAL` | `SUBTOTAL(function_num, ref1, ...)` | `function_num`: SUBTOTAL function code (1=AVG, 2=COUNT, 9=SUM, …)<br>`ref1`: first reference for SUBTOTAL<br>Common function_num: 1 AVG, 2 COUNT, 3 COUNTA, 4 MAX, 5 MIN, 6 PRODUCT, 7 STDEV, 9 SUM, 10 VAR. | Calculates subtotal in list/database (9=SUM, 1=AVG, etc.) |
| `ROUND` | `ROUND(val, num_digits)` | `val`: value to test or convert<br>`num_digits`: digits after the decimal point | Rounds number to specified decimal places |
| `ROUNDUP` | `ROUNDUP(val, num_digits)` | `val`: value to test or convert<br>`num_digits`: digits after the decimal point | Rounds number up, away from zero |
| `ROUNDDOWN` | `ROUNDDOWN(val, num_digits)` | `val`: value to test or convert<br>`num_digits`: digits after the decimal point | Rounds number down, towards zero |
| `TRUNC` | `TRUNC(val, [num_digits])` | `val`: value to test or convert<br>`num_digits` (optional): digits after the decimal point | Truncates number to specified decimal places |
| `INT` | `INT(val)` | `val`: value to test or convert | Rounds number down to nearest integer |
| `ABS` | `ABS(val)` | `val`: value to test or convert | Returns absolute value of number |
| `MOD` | `MOD(number, divisor)` | `number`: numeric value<br>`divisor`: number to divide by | Returns remainder after division (modulo) |
| `QUOTIENT` | `QUOTIENT(numerator, denominator)` | `numerator`: dividend<br>`denominator`: divisor | Returns integer portion of a division |
| `SIGN` | `SIGN(number)` | `number`: numeric value | Returns sign of number (1=pos, -1=neg, 0=zero) |
| `POWER` | `POWER(number, power)` | `number`: numeric value<br>`power`: exponent | Calculates number raised to a power (x^y) |
| `SQRT` | `SQRT(val)` | `val`: value to test or convert | Calculates square root of positive number |
| `EXP` | `EXP(number)` | `number`: numeric value | Returns e raised to the power of number |
| `LN` | `LN(number)` | `number`: numeric value | Returns natural logarithm of number |
| `LOG` | `LOG(number, [base])` | `number`: numeric value<br>`base` (optional): logarithm base (default 10) | Returns logarithm of number to specified base (default 10) |
| `LOG10` | `LOG10(number)` | `number`: numeric value | Returns base-10 logarithm of number |
| `CEILING` | `CEILING(number, significance)` | `number`: numeric value<br>`significance`: multiple / significance step | Rounds number up to nearest multiple of significance |
| `FLOOR` | `FLOOR(number, significance)` | `number`: numeric value<br>`significance`: multiple / significance step | Rounds number down to nearest multiple of significance |
| `MROUND` | `MROUND(number, multiple)` | `number`: numeric value<br>`multiple`: multiple to round to | Rounds number to nearest multiple |
| `FACT` | `FACT(number)` | `number`: numeric value | Calculates factorial of a number (n!) |
| `GCD` | `GCD(number1, number2, ...)` | `number1`: first number<br>`number2`: additional number<br>`number2`: additional number<br>`k`: rank or fraction depending on the function (see description) | Returns number of combinations for n items choose k (nCr) |
| `PERMUT` | `PERMUT(n, k)` | `n`: total items / periods depending on function<br>`k`: rank or fraction depending on the function (see description) | Returns number of permutations for n items choose k (nPr) |
| `PI` | `PI()` | —(no arguments)— | Returns constant value of Pi (3.14159265...) |
| `DEGREES` | `DEGREES(angle_in_radians)` | `angle_in_radians`: angle in radians | Converts radians to degrees |
| `RADIANS` | `RADIANS(angle_in_degrees)` | `angle_in_degrees`: angle in degrees | Converts degrees to radians |
| `SIN` | `SIN(number)` | `number`: numeric value | Returns sine of an angle in radians |
| `COS` | `COS(number)` | `number`: numeric value | Returns cosine of an angle in radians |
| `TAN` | `TAN(number)` | `number`: numeric value | Returns tangent of an angle in radians |
| `ASIN` | `ASIN(number)` | `number`: numeric value | Returns arcsine (inverse sine) in radians |
| `ACOS` | `ACOS(number)` | `number`: numeric value | Returns arccosine (inverse cosine) in radians |
| `ATAN` | `ATAN(number)` | `number`: numeric value | Returns arctangent in radians |
| `ATAN2` | `ATAN2(x_num, y_num)` | `x_num`: X coordinate<br>`y_num`: Y coordinate | Returns arctangent from x and y coordinates |
| `RAND` | `RAND()` | —(no arguments)— | Returns random real number between 0 and 1 |
| `RANDBETWEEN` | `RANDBETWEEN(min, max)` | `min`: minimum integer (inclusive)<br>`max`: maximum integer (inclusive) | Returns random integer between min and max (inclusive) |

### Statistical

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `AVG` | `AVG(range/list)` | `range/list`: cell range and/or individual numbers | Calculates arithmetic mean (average) |
| `AVERAGEIF` | `AVERAGEIF(range, criteria, [avg_range])` | `range`: cell range<br>`criteria`: condition such as ">50", "Apple", or a cell<br>`avg_range` (optional): optional cells to average (defaults to range) | Calculates average of cells meeting criteria |
| `AVERAGEIFS` | `AVERAGEIFS(avg_rng, crit_rng1, crit1, ...)` | `avg_rng`: cells whose average is computed<br>`crit_rng1`: first criteria range (same shape as related ranges)<br>`crit1`: first criteria value/expression<br>`criteria`: condition such as ">50", "Apple", or a cell | Counts number of cells meeting criteria |
| `COUNTIFS` | `COUNTIFS(crit_rng1, crit1, ...)` | `crit_rng1`: first criteria range (same shape as related ranges)<br>`crit1`: first criteria value/expression<br>`crit_rng1`: first criteria range (same shape as related ranges)<br>`crit1`: first criteria value/expression<br>`crit_rng1`: first criteria range (same shape as related ranges)<br>`crit1`: first criteria value/expression<br>`k`: rank or fraction depending on the function (see description)<br>k=1 is the largest value. | Returns k-th largest value in a data set |
| `SMALL` | `SMALL(array, k)` | `array`: cell range or value list<br>`k`: rank or fraction depending on the function (see description)<br>k=1 is the smallest value. | Returns k-th smallest value in a data set |
| `PERCENTILE` | `PERCENTILE(array, k)` | `array`: cell range or value list<br>`k`: rank or fraction depending on the function (see description)<br>k is from 0 through 1 (0%=min, 100%=max). | Returns k-th percentile of values in a range (0..1) |
| `QUARTILE` | `QUARTILE(array, quart)` | `array`: cell range or value list<br>`quart`: quartile number 0–4 | Returns quartile of data set (0..4) |
| `STDEV` | `STDEV(range/list)` | `range/list`: cell range and/or individual numbers | Estimates sample standard deviation (n-1) |
| `STDEVP` | `STDEVP(range/list)` | `range/list`: cell range and/or individual numbers | Calculates population standard deviation (n) |
| `VAR` | `VAR(range/list)` | `range/list`: cell range and/or individual numbers | Estimates sample variance (n-1) |
| `VARP` | `VARP(range/list)` | `range/list`: cell range and/or individual numbers | Calculates population variance (n) |
| `RANK` | `RANK(num, range, [order])` | `num`: number whose rank is returned<br>`range`: cell range<br>`order` (optional): 0 = descending rank, 1 = ascending (optional)<br>order omitted or 0 = rank as if sorted descending. | Returns rank of a number in a range (0=desc, 1=asc) |

### Lookup/Ref

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `XLOOKUP` | `XLOOKUP(key, lk_rng, ret_rng, [fallback], [match], [search])` | `key`: lookup key to find<br>`lk_rng`: range where the key is searched<br>`ret_rng`: range of values to return for a match<br>`fallback` (optional): value if not found<br>`match` (optional): match mode for XLOOKUP (optional)<br>`search` (optional): search direction/mode for XLOOKUP (optional) | Modern 2-way exact & approximate lookup with fallback value |
| `VLOOKUP` | `VLOOKUP(key, table_range, col_offset, [exact])` | `key`: lookup key to find<br>`table_range`: lookup table range<br>`col_offset`: column offset within the range (0-based for INDEX)<br>`exact` (optional): TRUE/1 = exact match; FALSE/0 = approximate (sorted) | Searches leftmost column and returns offset column value |
| `HLOOKUP` | `HLOOKUP(key, table_range, row_offset, [exact])` | `key`: lookup key to find<br>`table_range`: lookup table range<br>`row_offset`: row offset within the range (0-based for INDEX)<br>`exact` (optional): TRUE/1 = exact match; FALSE/0 = approximate (sorted) | Searches topmost row and returns offset row value |
| `LOOKUP` | `LOOKUP(val, lookup_vector, [result_vector])` | `val`: value to test or convert<br>`lookup_vector`: lookup row/column<br>`result_vector` (optional): optional parallel range of return values | Looks up value in 1-row or 1-column range |
| `INDEX` | `INDEX(range, col_offset, row_offset)` | `range`: cell range<br>`col_offset`: column offset within the range (0-based for INDEX)<br>`row_offset`: row offset within the range (0-based for INDEX)<br>Note: col_offset and row_offset are 0-based in HasuCalc. | Returns cell value at intersection coordinate (0-based) |
| `MATCH` | `MATCH(key, lookup_array, [match_type])` | `key`: lookup key to find<br>`lookup_array`: one-row or one-column range to search<br>`match_type` (optional): 1 / 0 / -1 match behavior (optional; see MATCH)<br>match_type: 1 largest≤key (asc), 0 exact, -1 smallest≥key (desc). | Returns index position of matched item in array (1-based) |
| `XMATCH` | `XMATCH(key, lookup_array, [match_mode], [search_mode])` | `key`: lookup key to find<br>`lookup_array`: one-row or one-column range to search<br>`match_mode` (optional): exact / wildcard / approx mode (optional)<br>`search_mode` (optional): search direction/mode (optional) | Modern position lookup with exact, wildcard, and reverse search |
| `OFFSET` | `OFFSET(ref, rows, cols, [height], [width])` | `ref`: starting cell or range<br>`rows`: rows to shift from the start reference<br>`cols`: columns to shift from the start reference<br>`height` (optional): optional height of the returned range (rows)<br>`width` (optional): optional width of the returned range (columns) | Returns reference offset from starting cell/range |
| `CHOOSE` | `CHOOSE(index, val0, val1, val2...)` | `index`: 0-based index into the value list (CHOOSE)<br>`val0`: first choice value (index 0)<br>`val1`: value / choice in a list<br>`val2` (repeatable): additional value in a list<br>Note: index is 0-based (first value is index 0). | Selects and returns value from list by 0-based index |
| `ROW` | `ROW([cell])` | `cell` (optional): optional cell reference (defaults to the formula cell) | Returns row number of current or referenced cell (1-based) |
| `COLUMN` | `COLUMN([cell])` | `cell` (optional): optional cell reference (defaults to the formula cell) | Returns column number of current or referenced cell (1-based) |
| `ROWS` | `ROWS(range)` | `range`: cell range | Returns total number of rows in specified range |
| `COLUMNS` | `COLUMNS(range)` | `range`: cell range | Returns total number of columns in specified range |
| `TRANSPOSE` | `TRANSPOSE(array)` | `array`: cell range or value list | Transposes rows and columns of an array |

### Logic/Error

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `IF` | `IF(condition, true_val, false_val)` | `condition`: logical test (true/false)<br>`true_val`: result when the condition is true<br>`false_val`: result when the condition is false | Conditional three-way branching |
| `IFS` | `IFS(cond1, val1, [cond2, val2]...)` | `cond1`: first condition (true/false)<br>`val1`: value / choice in a list<br>`cond2`: next condition<br>`val2` (repeatable): additional value in a list | Evaluates multiple conditions in sequence |
| `SWITCH` | `SWITCH(expr, val1, res1, [val2, res2]..., [default])` | `expr`: value to compare against the list<br>`val1`: value / choice in a list<br>`res1`: result when expr equals val1<br>`val2`: additional value in a list<br>`res2` (repeatable): result when expr equals val2<br>`default` (optional): value if nothing else matched | Evaluates expression against a list of values |
| `AND` | `AND(logical1, [logical2], ...)` | `logical1`: first logical value<br>`logical2` (optional): additional logical value<br>`logical2` (optional): additional logical value<br>`logical2` (repeatable): additional logical value | Returns exclusive OR of arguments |
| `IFERROR` | `IFERROR(formula, fallback_val)` | `formula`: expression or reference to evaluate<br>`fallback_val`: value returned when the primary result is an error/NA | Returns fallback value if formula results in error |
| `IFNA` | `IFNA(formula, fallback_val)` | `formula`: expression or reference to evaluate<br>`fallback_val`: value returned when the primary result is an error/NA | Returns fallback value if formula results in #N/A |
| `ISNUMBER` | `ISNUMBER(val)` | `val`: value to test or convert | Tests if value is a numeric number (returns 1 or 0) |
| `ISSTRING` | `ISSTRING(val)` | `val`: value to test or convert | Tests if value is a text string (returns 1 or 0) |
| `ISTEXT` | `ISTEXT(val)` | `val`: value to test or convert | Tests if value is text (returns 1 or 0) |
| `ISNONTEXT` | `ISNONTEXT(val)` | `val`: value to test or convert | Tests if value is not text (returns 1 or 0) |
| `ISBLANK` | `ISBLANK(val)` | `val`: value to test or convert | Tests if referenced cell is blank/empty |
| `ISLOGICAL` | `ISLOGICAL(val)` | `val`: value to test or convert | Tests if value is a logical boolean |
| `ISERR` | `ISERR(val)` | `val`: value to test or convert | Tests if value is an error #ERR (returns 1 or 0) |
| `ISNA` | `ISNA(val)` | `val`: value to test or convert | Tests if value is #N/A (returns 1 or 0) |
| `ISEVEN` | `ISEVEN(number)` | `number`: numeric value | Tests if number is even (returns 1 or 0) |
| `ISODD` | `ISODD(number)` | `number`: numeric value | Tests if number is odd (returns 1 or 0) |
| `TRUE` | `TRUE()` | —(no arguments)— | Returns boolean true |
| `FALSE` | `FALSE()` | —(no arguments)— | Returns boolean false |
| `N` | `N(value)` | `value`: value to convert or format | Converts value to a numeric number |
| `T` | `T(value)` | `value`: value to convert or format | Returns text string if value is text, empty string otherwise |
| `TYPE` | `TYPE(value)` | `value`: value to convert or format | Returns integer code for value data type (1=num, 2=text, etc.) |

### Text

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `TEXT` | `TEXT(value, format_string)` | `value`: value to convert or format<br>`format_string`: display format, e.g. "yyyy/mm/dd" or "#,##0.00" | Formats number or date with custom format string (e.g. "yyyy/mm/dd", "#,##0") |
| `TRIM` | `TRIM(text)` | `text`: text string | Strips leading/trailing spaces and collapses internal spaces |
| `CLEAN` | `CLEAN(text)` | `text`: text string | Removes all non-printable characters from text |
| `SUBSTITUTE` | `SUBSTITUTE(text, old_text, new_text, [instance])` | `text`: text string<br>`old_text`: original text / substring to find<br>`new_text`: replacement text<br>`instance` (optional): which occurrence to replace (optional; all if omitted) | Replaces occurrences of substring in text |
| `REPLACE` | `REPLACE(old_text, start_pos, num_chars, new_text)` | `old_text`: original text / substring to find<br>`start_pos`: 1-based start character position<br>`num_chars`: how many characters to take<br>`new_text`: replacement text | Replaces characters at position within text |
| `REPT` | `REPT(text, number_times)` | `text`: text string<br>`number_times`: how many times to repeat | Repeats text a given number of times |
| `UPPER` | `UPPER(text)` | `text`: text string | Converts all letters in text to UPPERCASE |
| `LOWER` | `LOWER(text)` | `text`: text string | Converts all letters in text to lowercase |
| `PROPER` | `PROPER(text)` | `text`: text string | Converts text to Title Case (capitalizes each word) |
| `EXACT` | `EXACT(text1, text2)` | `text1`: first text value<br>`text2`: additional text value | Tests if two text values are exactly identical (case-sensitive) |
| `CHAR` | `CHAR(number)` | `number`: numeric value | Returns character specified by ASCII/code number |
| `CODE` | `CODE(text)` | `text`: text string | Returns numeric code for the first character in text string |
| `UNICHAR` | `UNICHAR(number)` | `number`: numeric value | Returns Unicode character specified by numeric value |
| `UNICODE` | `UNICODE(text)` | `text`: text string | Returns numeric Unicode codepoint of first character |
| `CONCATENATE` | `CONCATENATE(text1, text2, ...)` | `text1`: first text value<br>`text2`: additional text value<br>`text2`: additional text value<br>`ignore_empty`: TRUE to skip empty pieces when joining<br>`text1`: first text value<br>ignore_empty: use 1/TRUE to skip blanks. | Joins text strings with a custom delimiter and options |
| `LEFT` | `LEFT(text, num_chars)` | `text`: text string<br>`num_chars`: how many characters to take | Extracts leftmost characters from text string |
| `RIGHT` | `RIGHT(text, num_chars)` | `text`: text string<br>`num_chars`: how many characters to take | Extracts rightmost characters from text string |
| `MID` | `MID(text, start_pos, num_chars)` | `text`: text string<br>`start_pos`: 1-based start character position<br>`num_chars`: how many characters to take | Extracts substring from middle of text string |
| `LEN` | `LEN(text)` | `text`: text string | Returns total number of characters in text string |
| `FIND` | `FIND(find_text, within_text, [start])` | `find_text`: text to search for<br>`within_text`: text to search inside<br>`start` (optional): 1-based start position (optional) | Case-sensitive search for text position (1-based) |
| `SEARCH` | `SEARCH(find_text, within_text, [start])` | `find_text`: text to search for<br>`within_text`: text to search inside<br>`start` (optional): 1-based start position (optional) | Case-insensitive & wildcard (*, ?) text position search |
| `STRING` | `STRING(number, decimal_places)` | `number`: numeric value<br>`decimal_places`: number of digits after the decimal point | Formats number as string with fixed decimal places |
| `VALUE` | `VALUE(text)` | `text`: text string | Converts numeric text string (with $, ¥, commas) to number |
| `NUMBERVALUE` | `NUMBERVALUE(text, [dec_sep], [group_sep])` | `text`: text string<br>`dec_sep` (optional): decimal separator character (optional)<br>`group_sep` (optional): thousands grouping separator (optional) | Parses formatted number text with locale separators |
| `TEXTBEFORE` | `TEXTBEFORE(text, delimiter)` | `text`: text string<br>`delimiter`: separator text | Extracts text occurring before delimiter |
| `TEXTAFTER` | `TEXTAFTER(text, delimiter)` | `text`: text string<br>`delimiter`: separator text | Extracts text occurring after delimiter |
| `TEXTSPLIT` | `TEXTSPLIT(text, col_delimiter)` | `text`: text string<br>`col_delimiter`: text that separates columns when splitting | Splits text into array by delimiter |

### Date/Time

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `TODAY` | `TODAY()` | —(no arguments)— | Returns serial number of current date |
| `NOW` | `NOW()` | —(no arguments)— | Returns serial number of current date and time |
| `DATE` | `DATE(year, month, day)` | `year`: four-digit year<br>`month`: month number (1–12)<br>`day`: day of month (1–31) | Creates date serial number from year, month, and day |
| `DATEVALUE` | `DATEVALUE(date_text)` | `date_text`: date written as text | Converts date text (e.g. "2026/01/01", "2026JANUARY1") to serial number |
| `TIME` | `TIME(hour, minute, second)` | `hour`: hour (0–23)<br>`minute`: minute (0–59)<br>`second`: second (0–59) | Creates time decimal fraction (0.0..1.0) from hour, min, sec |
| `TIMEVALUE` | `TIMEVALUE(time_text)` | `time_text`: time written as text | Converts time text (e.g. "14:30:00") to time fraction (0.0..1.0) |
| `DATEDIF` | `DATEDIF(start_date, end_date, unit)` | `start_date`: starting date (serial or date value)<br>`end_date`: ending date (serial or date value)<br>`unit`: difference unit: "Y","M","D","YM","YD","MD" | Calculates difference between two dates (unit: "Y", "M", "D", "YM", "YD", "MD") |
| `DAYS` | `DAYS(end_date, start_date)` | `end_date`: ending date (serial or date value)<br>`start_date`: starting date (serial or date value) | Returns number of days between two dates |
| `DAYS360` | `DAYS360(start_date, end_date)` | `start_date`: starting date (serial or date value)<br>`end_date`: ending date (serial or date value) | Calculates difference based on a 360-day year (12 months of 30 days) |
| `NETWORKDAYS` | `NETWORKDAYS(start_date, end_date, [holidays])` | `start_date`: starting date (serial or date value)<br>`end_date`: ending date (serial or date value)<br>`holidays` (optional): optional range of holiday dates to exclude | Returns number of working days between two dates |
| `WORKDAY` | `WORKDAY(start_date, days, [holidays])` | `start_date`: starting date (serial or date value)<br>`days`: number of (work)days to add/subtract<br>`holidays` (optional): optional range of holiday dates to exclude | Returns date before or after specified number of workdays |
| `YEARFRAC` | `YEARFRAC(start_date, end_date)` | `start_date`: starting date (serial or date value)<br>`end_date`: ending date (serial or date value) | Calculates fraction of year represented by number of whole days |
| `YEAR` | `YEAR(serial_date)` | `serial_date`: date serial number (or date-valued cell) | Extracts 4-digit year from date serial |
| `MONTH` | `MONTH(serial_date)` | `serial_date`: date serial number (or date-valued cell) | Extracts month number (1..12) from date serial |
| `DAY` | `DAY(serial_date)` | `serial_date`: date serial number (or date-valued cell) | Extracts day of month (1..31) from date serial |
| `HOUR` | `HOUR(time_serial)` | `time_serial`: time serial (fraction of a day) or datetime | Extracts hour (0..23) from time serial |
| `MINUTE` | `MINUTE(time_serial)` | `time_serial`: time serial (fraction of a day) or datetime | Extracts minute (0..59) from time serial |
| `SECOND` | `SECOND(time_serial)` | `time_serial`: time serial (fraction of a day) or datetime | Extracts second (0..59) from time serial |
| `WEEKDAY` | `WEEKDAY(serial_date, [type])` | `serial_date`: date serial number (or date-valued cell)<br>`type` (optional): payment timing (0=end, 1=beginning) or WEEKDAY return style<br>type selects numbering (e.g. 1=Sun…7=Sat, or Mon-based variants 11–17). | Returns weekday number (1=Sun..7=Sat, or 1=Mon..7=Sun) |
| `WEEKNUM` | `WEEKNUM(serial_date)` | `serial_date`: date serial number (or date-valued cell) | Returns week number of the year (1..53) |
| `EDATE` | `EDATE(start_date, months)` | `start_date`: starting date (serial or date value)<br>`months`: months to shift (can be negative) | Returns date serial n months before or after start date |
| `EOMONTH` | `EOMONTH(start_date, months)` | `start_date`: starting date (serial or date value)<br>`months`: months to shift (can be negative) | Returns last day of month n months before or after start date |

### Financial

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `PMT` | `PMT(rate, nper, pv, [fv], [type])` | `rate`: interest / discount rate per period<br>`nper`: number of payment periods<br>`pv`: present value<br>`fv` (optional): future value (optional; default 0)<br>`type` (optional): payment timing (0=end, 1=beginning) or WEEKDAY return style<br>type: 0=payment at period end (default), 1=at beginning. | Calculates periodic loan payment with constant interest rate |
| `PV` | `PV(rate, nper, pmt, [fv], [type])` | `rate`: interest / discount rate per period<br>`nper`: number of payment periods<br>`pmt`: payment amount per period<br>`fv` (optional): future value (optional; default 0)<br>`type` (optional): payment timing (0=end, 1=beginning) or WEEKDAY return style<br>type: 0=end of period (default), 1=beginning. | Calculates present value of an investment/loan |
| `FV` | `FV(rate, nper, pmt, [pv], [type])` | `rate`: interest / discount rate per period<br>`nper`: number of payment periods<br>`pmt`: payment amount per period<br>`pv` (optional): present value<br>`type` (optional): payment timing (0=end, 1=beginning) or WEEKDAY return style<br>type: 0=end of period (default), 1=beginning. | Calculates future value of an investment with periodic payments |
| `NPV` | `NPV(rate, val1, [val2]...)` | `rate`: interest / discount rate per period<br>`val1`: value / choice in a list<br>`val2` (repeatable): additional value in a list | Calculates net present value using discount rate and cash flows |
| `IRR` | `IRR(values, [guess])` | `values`: range of cash-flow values<br>`guess` (optional): optional starting guess for IRR | Calculates internal rate of return for a series of cash flows |
| `RATE` | `RATE(nper, pmt, pv, [fv], [type])` | `nper`: number of payment periods<br>`pmt`: payment amount per period<br>`pv`: present value<br>`fv` (optional): future value (optional; default 0)<br>`type` (optional): payment timing (0=end, 1=beginning) or WEEKDAY return style<br>type: 0=end of period (default), 1=beginning. | Calculates interest rate per period of an annuity |
| `NPER` | `NPER(rate, pmt, pv, [fv], [type])` | `rate`: interest / discount rate per period<br>`pmt`: payment amount per period<br>`pv`: present value<br>`fv` (optional): future value (optional; default 0)<br>`type` (optional): payment timing (0=end, 1=beginning) or WEEKDAY return style<br>type: 0=end of period (default), 1=beginning. | Returns number of periods for an investment/loan |
| `SLN` | `SLN(cost, salvage, life)` | `cost`: initial asset cost<br>`salvage`: value at end of life<br>`life`: useful life in periods | Returns straight-line depreciation of an asset for one period |
| `SYD` | `SYD(cost, salvage, life, per)` | `cost`: initial asset cost<br>`salvage`: value at end of life<br>`life`: useful life in periods<br>`per`: period number for which depreciation is calculated | Returns sum-of-years' digits depreciation for specified period |
| `DDB` | `DDB(cost, salvage, life, period, [factor])` | `cost`: initial asset cost<br>`salvage`: value at end of life<br>`life`: useful life in periods<br>`period`: period number<br>`factor` (optional): declining-balance factor (optional; often 2) | Returns double-declining balance depreciation of an asset |

### Aliases (also accepted)

| Alias | Canonical |
|:---|:---|
| `AVG` | `AVERAGE` |
| `LENGTH` | `LEN` |
| `REPEAT` | `REPT` |
| `PAYMT` | `PMT` |
| `MULTIPLY` | `PRODUCT` |
| `STD` | `STDEV.P` / `STDEVP` |
| `STRING` | Lotus-style number→text |
| `CONCATENATE` | `CONCAT` (also listed) |

---

## 10. Errors and common messages

| Display | Meaning |
|:---|:---|
| `ERR` | Generic formula / value error (HasuCalc does not split Excel `#VALUE!` / `#DIV/0!` / …). |
| `NA` | Missing / not found (e.g. failed lookup, `NA()`). |
| `CIRCULAR REF` | Circular reference detected. |
| `#REF!` | Broken reference (deleted sheet/range). |
| Status “Clipboard is empty!” | Paste with nothing copied. |
| Status “Imported markup from …” | `.md` / `.html` opened successfully. |

For engineering contracts (what is a bug vs intentional Excel difference), see [SPECIFICATION.md](SPECIFICATION.md) §1.4.
