# HasuCalc User Manual

> **Languages:** English (canonical) · [日本語](USER_MANUAL.ja.md)  
> Overview / quick start: [README.md](README.md) · Engineering spec: [SPECIFICATION.md](SPECIFICATION.md) · Headless & MCP: [HEADLESS_SPEC.md](HEADLESS_SPEC.md)

This manual explains the on-screen UI, status indicators such as `[READY]`, editing modes, supported file formats, and the syntax and parameters of every built-in function. HasuCalc uses `.hwk` / `.hwkz` as its authoritative native formats, while offering import/export bridges for `.xlsx` / `.ods` / `.csv` / `.md` / `.html`.

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
11. [Headless CLI and MCP Server](#11-headless-cli-and-mcp-server)

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
| **`[END]`** | End mode: press End, then an arrow key, to jump to the edge of the current data block. |

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

- Supports standard `=SUM(A1:B10)` formulas as well as classic `@SUM(A1..B10)` and `+` expressions.
- Ranges: both colon (`A1:B10`) and double-dot (`A1..B10`) syntax; whole-column references like `A:A` are automatically clipped to the used range.
- Cross-sheet references: `Sheet2!A1`, or `'Q1-2024'!A1` when spaces or symbols are used.
- Absolute and mixed references: `$A$1`, `$A1`, `A$1`.
- Booleans: `TRUE` / `FALSE` (and `TRUE()` / `FALSE()`).
- Logical operators: inline `#AND#`, `#OR#`, `#NOT#` (e.g. `+A1>10#AND#B1<20`).
- Unary minus binds tighter than `^`: `-2^2` → `(-2)^2` = `4` (write `-(2^2)` for `-4`).

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
- Chart settings persist in `.hwk` / `.hwkz` (external `.xlsx` / `.ods` exports contain tabular data only).

---

## 9. Function reference

Functions accept either `@NAME(...)` or `NAME(...)` after `=`.

- Arguments in `[brackets]` are **optional**.
- A trailing `...` means you may **repeat** that kind of argument (see the `…` row in Parameters).
- The **Parameters** column says what to put in each argument.

### Math/Agg

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `SUM` | `SUM(range/list)` | `range/list`: a range and/or individual numbers | Calculates total sum of numbers in range |
| `SUMIF` | `SUMIF(range, criteria, [sum_range])` | `range`: cell range<br>`criteria`: condition such as ">50", "Apple", or a cell reference<br>`sum_range` (optional): cells to add when criteria match (defaults to range) | Sums cells meeting specified criteria (e.g. ">50", "Apple") |
| `SUMIFS` | `SUMIFS(sum_rng, crit_rng1, crit1, ...)` | `sum_rng`: cells to add for matching rows<br>`crit_rng1`: first criteria range (same shape as sum/avg/min/max range)<br>`crit1`: first criteria (e.g. ">10" or a cell)<br>`…` (repeatable): more pairs of (criteria_range, criteria); same shape as sum_rng | Sums cells that meet multiple criteria across ranges |
| `SUMPRODUCT` | `SUMPRODUCT(array1, [array2]...)` | `array1`: first range/array<br>`…` (repeatable): additional arrays (same dimensions) | Calculates sum of products of corresponding items |
| `PRODUCT` | `PRODUCT(number1, [number2]...)` | `number1`: first number<br>`…` (repeatable): additional numbers | Multiplies all numbers given in arguments |
| `SUBTOTAL` | `SUBTOTAL(function_num, ref1, ...)` | `function_num`: which aggregate to run (1=AVG … 9=SUM; see note)<br>`ref1`: first range included in the subtotal<br>`…` (repeatable): additional ranges to include<br>function_num examples: 1 AVG, 2 COUNT, 3 COUNTA, 4 MAX, 5 MIN, 6 PRODUCT, 7 STDEV, 9 SUM, 10 VAR. | Calculates subtotal in list/database (9=SUM, 1=AVG, etc.) |
| `ROUND` | `ROUND(val, num_digits)` | `val`: input value<br>`num_digits`: decimal places | Rounds number to specified decimal places |
| `ROUNDUP` | `ROUNDUP(val, num_digits)` | `val`: input value<br>`num_digits`: decimal places | Rounds number up, away from zero |
| `ROUNDDOWN` | `ROUNDDOWN(val, num_digits)` | `val`: input value<br>`num_digits`: decimal places | Rounds number down, towards zero |
| `TRUNC` | `TRUNC(val, [num_digits])` | `val`: input value<br>`num_digits` (optional): decimal places | Truncates number to specified decimal places |
| `INT` | `INT(val)` | `val`: input value | Rounds number down to nearest integer |
| `ABS` | `ABS(val)` | `val`: input value | Returns absolute value of number |
| `MOD` | `MOD(number, divisor)` | `number`: a number<br>`divisor`: number to divide by | Returns remainder after division (modulo) |
| `QUOTIENT` | `QUOTIENT(numerator, denominator)` | `numerator`: dividend<br>`denominator`: divisor | Returns integer portion of a division |
| `SIGN` | `SIGN(number)` | `number`: a number | Returns sign of number (1=pos, -1=neg, 0=zero) |
| `POWER` | `POWER(number, power)` | `number`: a number<br>`power`: exponent | Calculates number raised to a power (x^y) |
| `SQRT` | `SQRT(val)` | `val`: input value | Calculates square root of positive number |
| `EXP` | `EXP(number)` | `number`: a number | Returns e raised to the power of number |
| `LN` | `LN(number)` | `number`: a number | Returns natural logarithm of number |
| `LOG` | `LOG(number, [base])` | `number`: a number<br>`base` (optional): logarithm base (default 10) | Returns logarithm of number to specified base (default 10) |
| `LOG10` | `LOG10(number)` | `number`: a number | Returns base-10 logarithm of number |
| `CEILING` | `CEILING(number, significance)` | `number`: a number<br>`significance`: rounding step / multiple | Rounds number up to nearest multiple of significance |
| `FLOOR` | `FLOOR(number, significance)` | `number`: a number<br>`significance`: rounding step / multiple | Rounds number down to nearest multiple of significance |
| `MROUND` | `MROUND(number, multiple)` | `number`: a number<br>`multiple`: multiple to round toward | Rounds number to nearest multiple |
| `FACT` | `FACT(number)` | `number`: a number | Calculates factorial of a number (n!) |
| `GCD` | `GCD(number1, number2, ...)` | `number1`: first number<br>`number2`: another number<br>`…` (repeatable): additional integers | Returns greatest common divisor |
| `LCM` | `LCM(number1, number2, ...)` | `number1`: first number<br>`number2`: another number<br>`…` (repeatable): additional integers | Returns least common multiple |
| `COMBIN` | `COMBIN(n, k)` | `n`: n in nCr / nPr (total items)<br>`k`: rank or percentile fraction (see summary) | Returns number of combinations for n items choose k (nCr) |
| `PERMUT` | `PERMUT(n, k)` | `n`: n in nCr / nPr (total items)<br>`k`: rank or percentile fraction (see summary) | Returns number of permutations for n items choose k (nPr) |
| `PI` | `PI()` | (no arguments) | Returns constant value of Pi (3.14159265...) |
| `DEGREES` | `DEGREES(angle_in_radians)` | `angle_in_radians`: angle in radians | Converts radians to degrees |
| `RADIANS` | `RADIANS(angle_in_degrees)` | `angle_in_degrees`: angle in degrees | Converts degrees to radians |
| `SIN` | `SIN(number)` | `number`: a number | Returns sine of an angle in radians |
| `COS` | `COS(number)` | `number`: a number | Returns cosine of an angle in radians |
| `TAN` | `TAN(number)` | `number`: a number | Returns tangent of an angle in radians |
| `ASIN` | `ASIN(number)` | `number`: a number | Returns arcsine (inverse sine) in radians |
| `ACOS` | `ACOS(number)` | `number`: a number | Returns arccosine (inverse cosine) in radians |
| `ATAN` | `ATAN(number)` | `number`: a number | Returns arctangent in radians |
| `ATAN2` | `ATAN2(x_num, y_num)` | `x_num`: X coordinate<br>`y_num`: Y coordinate | Returns arctangent from x and y coordinates |
| `RAND` | `RAND()` | (no arguments) | Returns random real number between 0 and 1 |
| `RANDBETWEEN` | `RANDBETWEEN(min, max)` | `min`: smallest integer allowed (inclusive)<br>`max`: largest integer allowed (inclusive) | Returns random integer between min and max (inclusive) |

### Statistical

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `AVG` | `AVG(range/list)` | `range/list`: a range and/or individual numbers | Calculates arithmetic mean (average) |
| `AVERAGEIF` | `AVERAGEIF(range, criteria, [avg_range])` | `range`: cell range<br>`criteria`: condition such as ">50", "Apple", or a cell reference<br>`avg_range` (optional): cells to average when criteria match (defaults to range) | Calculates average of cells meeting criteria |
| `AVERAGEIFS` | `AVERAGEIFS(avg_rng, crit_rng1, crit1, ...)` | `avg_rng`: cells whose average is computed<br>`crit_rng1`: first criteria range (same shape as sum/avg/min/max range)<br>`crit1`: first criteria (e.g. ">10" or a cell)<br>`…` (repeatable): more pairs of (criteria_range, criteria) | Calculates average of cells meeting multiple criteria |
| `COUNT` | `COUNT(range/list)` | `range/list`: a range and/or individual numbers | Counts number of numeric cells in range |
| `COUNTA` | `COUNTA(range/list)` | `range/list`: a range and/or individual numbers | Counts number of non-empty cells in range |
| `COUNTBLANK` | `COUNTBLANK(range)` | `range`: cell range | Counts number of empty cells in range |
| `COUNTIF` | `COUNTIF(range, criteria)` | `range`: cell range<br>`criteria`: condition such as ">50", "Apple", or a cell reference | Counts number of cells meeting criteria |
| `COUNTIFS` | `COUNTIFS(crit_rng1, crit1, ...)` | `crit_rng1`: first criteria range (same shape as sum/avg/min/max range)<br>`crit1`: first criteria (e.g. ">10" or a cell)<br>`…` (repeatable): more pairs of (criteria_range, criteria) | Counts cells that meet multiple criteria across ranges |
| `MIN` | `MIN(range/list)` | `range/list`: a range and/or individual numbers | Finds minimum value in range/list |
| `MINIFS` | `MINIFS(min_rng, crit_rng1, crit1, ...)` | `min_rng`: cells that supply candidate minimum values<br>`crit_rng1`: first criteria range (same shape as sum/avg/min/max range)<br>`crit1`: first criteria (e.g. ">10" or a cell)<br>`…` (repeatable): more pairs of (criteria_range, criteria) | Finds minimum value among cells meeting multiple criteria |
| `MAX` | `MAX(range/list)` | `range/list`: a range and/or individual numbers | Finds maximum value in range/list |
| `MAXIFS` | `MAXIFS(max_rng, crit_rng1, crit1, ...)` | `max_rng`: cells that supply candidate maximum values<br>`crit_rng1`: first criteria range (same shape as sum/avg/min/max range)<br>`crit1`: first criteria (e.g. ">10" or a cell)<br>`…` (repeatable): more pairs of (criteria_range, criteria) | Finds maximum value among cells meeting multiple criteria |
| `MEDIAN` | `MEDIAN(range/list)` | `range/list`: a range and/or individual numbers | Returns median (middle value) of numbers |
| `MODE` | `MODE(range/list)` | `range/list`: a range and/or individual numbers | Returns most frequently occurring value in data set |
| `LARGE` | `LARGE(array, k)` | `array`: cell range or value list<br>`k`: rank or percentile fraction (see summary)<br>k=1 returns the largest value. | Returns k-th largest value in a data set |
| `SMALL` | `SMALL(array, k)` | `array`: cell range or value list<br>`k`: rank or percentile fraction (see summary)<br>k=1 returns the smallest value. | Returns k-th smallest value in a data set |
| `PERCENTILE` | `PERCENTILE(array, k)` | `array`: cell range or value list<br>`k`: rank or percentile fraction (see summary)<br>k is between 0 and 1 inclusive. | Returns k-th percentile of values in a range (0..1) |
| `QUARTILE` | `QUARTILE(array, quart)` | `array`: cell range or value list<br>`quart`: quartile index 0–4 | Returns quartile of data set (0..4) |
| `STDEV` | `STDEV(range/list)` | `range/list`: a range and/or individual numbers | Estimates sample standard deviation (n-1) |
| `STDEVP` | `STDEVP(range/list)` | `range/list`: a range and/or individual numbers | Calculates population standard deviation (n) |
| `VAR` | `VAR(range/list)` | `range/list`: a range and/or individual numbers | Estimates sample variance (n-1) |
| `VARP` | `VARP(range/list)` | `range/list`: a range and/or individual numbers | Calculates population variance (n) |
| `RANK` | `RANK(num, range, [order])` | `num`: number to rank<br>`range`: cell range<br>`order` (optional): 0 = descending ranks (default), 1 = ascending<br>With order 0 (default), larger numbers get rank 1. | Returns rank of a number in a range (0=desc, 1=asc) |

### Lookup/Ref

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `XLOOKUP` | `XLOOKUP(key, lk_rng, ret_rng, [fallback], [match], [search])` | `key`: value to look up<br>`lk_rng`: range searched for key<br>`ret_rng`: values returned for a successful match<br>`fallback` (optional): value if the key is not found<br>`match` (optional): optional XLOOKUP match mode<br>`search` (optional): optional XLOOKUP search mode | Modern 2-way exact & approximate lookup with fallback value |
| `VLOOKUP` | `VLOOKUP(key, table_range, col_offset, [exact])` | `key`: value to look up<br>`table_range`: full lookup table<br>`col_offset`: column offset inside the range (0-based)<br>`exact` (optional): TRUE/1 = exact match; FALSE/0 = approximate match (data should be sorted) | Searches leftmost column and returns offset column value |
| `HLOOKUP` | `HLOOKUP(key, table_range, row_offset, [exact])` | `key`: value to look up<br>`table_range`: full lookup table<br>`row_offset`: row offset inside the range (0-based)<br>`exact` (optional): TRUE/1 = exact match; FALSE/0 = approximate match (data should be sorted) | Searches topmost row and returns offset row value |
| `LOOKUP` | `LOOKUP(val, lookup_vector, [result_vector])` | `val`: input value<br>`lookup_vector`: lookup row or column<br>`result_vector` (optional): optional parallel range of return values | Looks up value in 1-row or 1-column range |
| `INDEX` | `INDEX(range, col_offset, row_offset)` | `range`: cell range<br>`col_offset`: column offset inside the range (0-based)<br>`row_offset`: row offset inside the range (0-based)<br>Offsets are 0-based (first cell is col_offset=0, row_offset=0). | Returns cell value at intersection coordinate (0-based) |
| `MATCH` | `MATCH(key, lookup_array, [match_type])` | `key`: value to look up<br>`lookup_array`: single row or column to search<br>`match_type` (optional): optional 1 / 0 / -1 (see note)<br>match_type: 1 = largest value ≤ key (ascending data), 0 = exact, -1 = smallest value ≥ key (descending data). | Returns index position of matched item in array (1-based) |
| `XMATCH` | `XMATCH(key, lookup_array, [match_mode], [search_mode])` | `key`: value to look up<br>`lookup_array`: single row or column to search<br>`match_mode` (optional): optional exact / wildcard / approximate mode<br>`search_mode` (optional): optional search direction/mode | Modern position lookup with exact, wildcard, and reverse search |
| `OFFSET` | `OFFSET(ref, rows, cols, [height], [width])` | `ref`: starting cell or range<br>`rows`: rows to move from ref (negative = up)<br>`cols`: columns to move from ref (negative = left)<br>`height` (optional): height in rows of the returned reference<br>`width` (optional): width in columns of the returned reference | Returns reference offset from starting cell/range |
| `CHOOSE` | `CHOOSE(index, val0, val1, val2...)` | `index`: which item to pick (0-based)<br>`val0`: choice for index 0<br>`val1`: a listed value / choice<br>`…` (repeatable): additional choice values<br>index is 0-based: the first value is index 0. | Selects and returns value from list by 0-based index |
| `ROW` | `ROW([cell])` | `cell` (optional): optional cell reference (defaults to this formula’s cell) | Returns row number of current or referenced cell (1-based) |
| `COLUMN` | `COLUMN([cell])` | `cell` (optional): optional cell reference (defaults to this formula’s cell) | Returns column number of current or referenced cell (1-based) |
| `ROWS` | `ROWS(range)` | `range`: cell range | Returns total number of rows in specified range |
| `COLUMNS` | `COLUMNS(range)` | `range`: cell range | Returns total number of columns in specified range |
| `TRANSPOSE` | `TRANSPOSE(array)` | `array`: cell range or value list | Transposes rows and columns of an array |

### Logic/Error

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `IF` | `IF(condition, true_val, false_val)` | `condition`: logical test<br>`true_val`: result when condition is true<br>`false_val`: result when condition is false | Conditional three-way branching |
| `IFS` | `IFS(cond1, val1, [cond2, val2]...)` | `cond1`: first condition<br>`val1`: a listed value / choice<br>`…` (repeatable): more condition/value pairs | Evaluates multiple conditions in sequence |
| `SWITCH` | `SWITCH(expr, val1, res1, [val2, res2]..., [default])` | `expr`: value compared to val1, val2, …<br>`val1`: a listed value / choice<br>`res1`: result when expr equals val1<br>`…` (repeatable): more value/result pairs before default<br>`default` (optional): result if no listed value matched | Evaluates expression against a list of values |
| `AND` | `AND(logical1, [logical2], ...)` | `logical1`: first logical value<br>`logical2` (optional): another logical value<br>`…` (repeatable): additional logical values | Returns true if all arguments are true |
| `OR` | `OR(logical1, [logical2], ...)` | `logical1`: first logical value<br>`logical2` (optional): another logical value<br>`…` (repeatable): additional logical values | Returns true if any argument is true |
| `NOT` | `NOT(logical)` | `logical`: TRUE/FALSE value or expression | Reverses the logical value of argument |
| `XOR` | `XOR(logical1, [logical2]...)` | `logical1`: first logical value<br>`…` (repeatable): additional logical values | Returns exclusive OR of arguments |
| `IFERROR` | `IFERROR(formula, fallback_val)` | `formula`: expression to evaluate<br>`fallback_val`: value if the expression errors / is NA | Returns fallback value if formula results in error |
| `IFNA` | `IFNA(formula, fallback_val)` | `formula`: expression to evaluate<br>`fallback_val`: value if the expression errors / is NA | Returns fallback value if formula results in #N/A |
| `ISNUMBER` | `ISNUMBER(val)` | `val`: input value | Tests if value is a numeric number (returns 1 or 0) |
| `ISSTRING` | `ISSTRING(val)` | `val`: input value | Tests if value is a text string (returns 1 or 0) |
| `ISTEXT` | `ISTEXT(val)` | `val`: input value | Tests if value is text (returns 1 or 0) |
| `ISNONTEXT` | `ISNONTEXT(val)` | `val`: input value | Tests if value is not text (returns 1 or 0) |
| `ISBLANK` | `ISBLANK(val)` | `val`: input value | Tests if referenced cell is blank/empty |
| `ISLOGICAL` | `ISLOGICAL(val)` | `val`: input value | Tests if value is a logical boolean |
| `ISERR` | `ISERR(val)` | `val`: input value | Tests if value is an error #ERR (returns 1 or 0) |
| `ISNA` | `ISNA(val)` | `val`: input value | Tests if value is #N/A (returns 1 or 0) |
| `ISEVEN` | `ISEVEN(number)` | `number`: a number | Tests if number is even (returns 1 or 0) |
| `ISODD` | `ISODD(number)` | `number`: a number | Tests if number is odd (returns 1 or 0) |
| `TRUE` | `TRUE()` | (no arguments) | Returns boolean true |
| `FALSE` | `FALSE()` | (no arguments) | Returns boolean false |
| `N` | `N(value)` | `value`: value to convert or format | Converts value to a numeric number |
| `T` | `T(value)` | `value`: value to convert or format | Returns text string if value is text, empty string otherwise |
| `TYPE` | `TYPE(value)` | `value`: value to convert or format | Returns integer code for value data type (1=num, 2=text, etc.) |

### Text

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `TEXT` | `TEXT(value, format_string)` | `value`: value to convert or format<br>`format_string`: format pattern, e.g. "yyyy/mm/dd" or "#,##0.00" | Formats number or date with custom format string (e.g. "yyyy/mm/dd", "#,##0") |
| `TRIM` | `TRIM(text)` | `text`: text string | Strips leading/trailing spaces and collapses internal spaces |
| `CLEAN` | `CLEAN(text)` | `text`: text string | Removes all non-printable characters from text |
| `SUBSTITUTE` | `SUBSTITUTE(text, old_text, new_text, [instance])` | `text`: text string<br>`old_text`: original text, or the text to find<br>`new_text`: replacement text<br>`instance` (optional): which occurrence to replace (omit = all) | Replaces occurrences of substring in text |
| `REPLACE` | `REPLACE(old_text, start_pos, num_chars, new_text)` | `old_text`: original text, or the text to find<br>`start_pos`: 1-based start character<br>`num_chars`: how many characters to return<br>`new_text`: replacement text | Replaces characters at position within text |
| `REPT` | `REPT(text, number_times)` | `text`: text string<br>`number_times`: repeat count | Repeats text a given number of times |
| `UPPER` | `UPPER(text)` | `text`: text string | Converts all letters in text to UPPERCASE |
| `LOWER` | `LOWER(text)` | `text`: text string | Converts all letters in text to lowercase |
| `PROPER` | `PROPER(text)` | `text`: text string | Converts text to Title Case (capitalizes each word) |
| `EXACT` | `EXACT(text1, text2)` | `text1`: first text value<br>`text2`: another text value | Tests if two text values are exactly identical (case-sensitive) |
| `CHAR` | `CHAR(number)` | `number`: a number | Returns character specified by ASCII/code number |
| `CODE` | `CODE(text)` | `text`: text string | Returns numeric code for the first character in text string |
| `UNICHAR` | `UNICHAR(number)` | `number`: a number | Returns Unicode character specified by numeric value |
| `UNICODE` | `UNICODE(text)` | `text`: text string | Returns numeric Unicode codepoint of first character |
| `CONCATENATE` | `CONCATENATE(text1, text2, ...)` | `text1`: first text value<br>`text2`: another text value<br>`…` (repeatable): additional text values | Joins multiple text strings into a single string |
| `CONCAT` | `CONCAT(text1, text2, ...)` | `text1`: first text value<br>`text2`: another text value<br>`…` (repeatable): additional text values or ranges | Concatenates list or range of text items |
| `TEXTJOIN` | `TEXTJOIN(delimiter, ignore_empty, text1, ...)` | `delimiter`: separator text<br>`ignore_empty`: TRUE/1 = skip empty strings when joining<br>`text1`: first text value<br>`…` (repeatable): additional text values or ranges<br>Pass 1 or TRUE for ignore_empty to skip blanks. | Joins text strings with a custom delimiter and options |
| `LEFT` | `LEFT(text, num_chars)` | `text`: text string<br>`num_chars`: how many characters to return | Extracts leftmost characters from text string |
| `RIGHT` | `RIGHT(text, num_chars)` | `text`: text string<br>`num_chars`: how many characters to return | Extracts rightmost characters from text string |
| `MID` | `MID(text, start_pos, num_chars)` | `text`: text string<br>`start_pos`: 1-based start character<br>`num_chars`: how many characters to return | Extracts substring from middle of text string |
| `LEN` | `LEN(text)` | `text`: text string | Returns total number of characters in text string |
| `FIND` | `FIND(find_text, within_text, [start])` | `find_text`: substring to find<br>`within_text`: text to search within<br>`start` (optional): 1-based character position to start searching | Case-sensitive search for text position (1-based) |
| `SEARCH` | `SEARCH(find_text, within_text, [start])` | `find_text`: substring to find<br>`within_text`: text to search within<br>`start` (optional): 1-based character position to start searching | Case-insensitive & wildcard (*, ?) text position search |
| `STRING` | `STRING(number, decimal_places)` | `number`: a number<br>`decimal_places`: digits after the decimal point | Formats number as string with fixed decimal places |
| `VALUE` | `VALUE(text)` | `text`: text string | Converts numeric text string (with $, ¥, commas) to number |
| `NUMBERVALUE` | `NUMBERVALUE(text, [dec_sep], [group_sep])` | `text`: text string<br>`dec_sep` (optional): decimal separator character<br>`group_sep` (optional): thousands separator character | Parses formatted number text with locale separators |
| `TEXTBEFORE` | `TEXTBEFORE(text, delimiter)` | `text`: text string<br>`delimiter`: separator text | Extracts text occurring before delimiter |
| `TEXTAFTER` | `TEXTAFTER(text, delimiter)` | `text`: text string<br>`delimiter`: separator text | Extracts text occurring after delimiter |
| `TEXTSPLIT` | `TEXTSPLIT(text, col_delimiter)` | `text`: text string<br>`col_delimiter`: text that separates pieces when splitting | Splits text into array by delimiter |

### Date/Time

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `TODAY` | `TODAY()` | (no arguments) | Returns serial number of current date |
| `NOW` | `NOW()` | (no arguments) | Returns serial number of current date and time |
| `DATE` | `DATE(year, month, day)` | `year`: four-digit year<br>`month`: month 1–12<br>`day`: day of month (1–31) | Creates date serial number from year, month, and day |
| `DATEVALUE` | `DATEVALUE(date_text)` | `date_text`: date as text | Converts date text (e.g. "2026/01/01", "2026JANUARY1") to serial number |
| `TIME` | `TIME(hour, minute, second)` | `hour`: hour 0–23<br>`minute`: minute 0–59<br>`second`: second 0–59 | Creates time decimal fraction (0.0..1.0) from hour, min, sec |
| `TIMEVALUE` | `TIMEVALUE(time_text)` | `time_text`: time as text | Converts time text (e.g. "14:30:00") to time fraction (0.0..1.0) |
| `DATEDIF` | `DATEDIF(start_date, end_date, unit)` | `start_date`: start date (serial or date cell)<br>`end_date`: end date (serial or date cell)<br>`unit`: "Y", "M", "D", "YM", "YD", or "MD" | Calculates difference between two dates (unit: "Y", "M", "D", "YM", "YD", "MD") |
| `DAYS` | `DAYS(end_date, start_date)` | `end_date`: end date (serial or date cell)<br>`start_date`: start date (serial or date cell) | Returns number of days between two dates |
| `DAYS360` | `DAYS360(start_date, end_date)` | `start_date`: start date (serial or date cell)<br>`end_date`: end date (serial or date cell) | Calculates difference based on a 360-day year (12 months of 30 days) |
| `NETWORKDAYS` | `NETWORKDAYS(start_date, end_date, [holidays])` | `start_date`: start date (serial or date cell)<br>`end_date`: end date (serial or date cell)<br>`holidays` (optional): optional range of holiday dates to skip | Returns number of working days between two dates |
| `WORKDAY` | `WORKDAY(start_date, days, [holidays])` | `start_date`: start date (serial or date cell)<br>`days`: number of days (or workdays) to shift<br>`holidays` (optional): optional range of holiday dates to skip | Returns date before or after specified number of workdays |
| `YEARFRAC` | `YEARFRAC(start_date, end_date)` | `start_date`: start date (serial or date cell)<br>`end_date`: end date (serial or date cell) | Calculates fraction of year represented by number of whole days |
| `YEAR` | `YEAR(serial_date)` | `serial_date`: date serial (or a cell holding a date) | Extracts 4-digit year from date serial |
| `MONTH` | `MONTH(serial_date)` | `serial_date`: date serial (or a cell holding a date) | Extracts month number (1..12) from date serial |
| `DAY` | `DAY(serial_date)` | `serial_date`: date serial (or a cell holding a date) | Extracts day of month (1..31) from date serial |
| `HOUR` | `HOUR(time_serial)` | `time_serial`: time as fraction of a day, or date-time serial | Extracts hour (0..23) from time serial |
| `MINUTE` | `MINUTE(time_serial)` | `time_serial`: time as fraction of a day, or date-time serial | Extracts minute (0..59) from time serial |
| `SECOND` | `SECOND(time_serial)` | `time_serial`: time as fraction of a day, or date-time serial | Extracts second (0..59) from time serial |
| `WEEKDAY` | `WEEKDAY(serial_date, [type])` | `serial_date`: date serial (or a cell holding a date)<br>`type` (optional): extra mode flag (payment timing or WEEKDAY style; see note)<br>type selects the numbering system (e.g. 1=Sunday…7=Saturday, or Monday-based 11–17). | Returns weekday number (1=Sun..7=Sat, or 1=Mon..7=Sun) |
| `WEEKNUM` | `WEEKNUM(serial_date)` | `serial_date`: date serial (or a cell holding a date) | Returns week number of the year (1..53) |
| `EDATE` | `EDATE(start_date, months)` | `start_date`: start date (serial or date cell)<br>`months`: months to move (may be negative) | Returns date serial n months before or after start date |
| `EOMONTH` | `EOMONTH(start_date, months)` | `start_date`: start date (serial or date cell)<br>`months`: months to move (may be negative) | Returns last day of month n months before or after start date |

### Financial

| Function | Syntax | Parameters | Summary |
|:---|:---|:---|:---|
| `PMT` | `PMT(rate, nper, pv, [fv], [type])` | `rate`: interest or discount rate per period<br>`nper`: number of payment periods<br>`pv`: present value<br>`fv` (optional): future value (default 0)<br>`type` (optional): extra mode flag (payment timing or WEEKDAY style; see note)<br>type: 0 = pay at end of period (default), 1 = pay at beginning. | Calculates periodic loan payment with constant interest rate |
| `PV` | `PV(rate, nper, pmt, [fv], [type])` | `rate`: interest or discount rate per period<br>`nper`: number of payment periods<br>`pmt`: payment per period<br>`fv` (optional): future value (default 0)<br>`type` (optional): extra mode flag (payment timing or WEEKDAY style; see note)<br>type: 0 = end of period (default), 1 = beginning. | Calculates present value of an investment/loan |
| `FV` | `FV(rate, nper, pmt, [pv], [type])` | `rate`: interest or discount rate per period<br>`nper`: number of payment periods<br>`pmt`: payment per period<br>`pv` (optional): present value<br>`type` (optional): extra mode flag (payment timing or WEEKDAY style; see note)<br>type: 0 = end of period (default), 1 = beginning. | Calculates future value of an investment with periodic payments |
| `NPV` | `NPV(rate, val1, [val2]...)` | `rate`: interest or discount rate per period<br>`val1`: a listed value / choice<br>`…` (repeatable): additional cash-flow values | Calculates net present value using discount rate and cash flows |
| `IRR` | `IRR(values, [guess])` | `values`: cash-flow amounts (range)<br>`guess` (optional): optional first guess for the solver | Calculates internal rate of return for a series of cash flows |
| `RATE` | `RATE(nper, pmt, pv, [fv], [type])` | `nper`: number of payment periods<br>`pmt`: payment per period<br>`pv`: present value<br>`fv` (optional): future value (default 0)<br>`type` (optional): extra mode flag (payment timing or WEEKDAY style; see note)<br>type: 0 = end of period (default), 1 = beginning. | Calculates interest rate per period of an annuity |
| `NPER` | `NPER(rate, pmt, pv, [fv], [type])` | `rate`: interest or discount rate per period<br>`pmt`: payment per period<br>`pv`: present value<br>`fv` (optional): future value (default 0)<br>`type` (optional): extra mode flag (payment timing or WEEKDAY style; see note)<br>type: 0 = end of period (default), 1 = beginning. | Returns number of periods for an investment/loan |
| `SLN` | `SLN(cost, salvage, life)` | `cost`: initial cost of the asset<br>`salvage`: value at the end of life<br>`life`: number of periods in the asset’s life | Returns straight-line depreciation of an asset for one period |
| `SYD` | `SYD(cost, salvage, life, per)` | `cost`: initial cost of the asset<br>`salvage`: value at the end of life<br>`life`: number of periods in the asset’s life<br>`per`: period index for this depreciation charge | Returns sum-of-years' digits depreciation for specified period |
| `DDB` | `DDB(cost, salvage, life, period, [factor])` | `cost`: initial cost of the asset<br>`salvage`: value at the end of life<br>`life`: number of periods in the asset’s life<br>`period`: period index<br>`factor` (optional): declining-balance factor (often 2) | Returns double-declining balance depreciation of an asset |

### Aliases (also accepted)

| Alias | Canonical |
|:---|:---|
| `AVG` | `AVERAGE` |
| `LENGTH` | `LEN` |
| `REPEAT` | `REPT` |
| `PAYMT` | `PMT` |
| `MULTIPLY` | `PRODUCT` |
| `STD` | `STDEV.P` / `STDEVP` |
| `STRING` | Number to fixed-decimal text (compatibility) |
| `CONCATENATE` | `CONCAT` (also listed) |

---

## 10. Errors and common messages

| Display | Meaning |
|:---|:---|
| `ERR` | Generic formula or computation error (division by zero, type mismatch, etc.). |
| `NA` | Missing or not found (e.g. failed lookup, `NA()`). |
| `CIRCULAR REF` | Circular reference detected. |
| `#REF!` | Broken reference (e.g. deleted sheet or range). |
| Status “Clipboard is empty!” | Paste with nothing copied. |
| Status “Imported markup from …” | `.md` / `.html` opened successfully. |

For engineering specifications and core behavioral guarantees, see [SPECIFICATION.md](SPECIFICATION.md) §1.4.

---

## 11. Headless CLI and MCP Server

HasuCalc can be used without launching the terminal UI (TUI) as a high-performance command-line utility for shell scripting, data pipelines, automated batch jobs, and AI agents via Model Context Protocol (MCP).

### 11.1 Subcommands Reference

Run `hasucalc --help` or `hasucalc <subcommand> --help` to view built-in options and flags.

#### `convert` — Format Conversion
Converts spreadsheet files between `.hwk`, `.hwkz`, `.xlsx`, `.ods`, `.csv`, `.tsv`, `.md`, and `.html`.
```bash
hasucalc convert input.xlsx output.hwk
hasucalc convert report.hwk output.csv --sheet "Q1 Sales"
hasucalc convert input.hwk - --format markdown  # stream to stdout
```

#### `info` — Workbook & Sheet Metadata
Inspects sheet names, used ranges, cell counts, freeze panes, and chart settings.
```bash
hasucalc info data.hwk
hasucalc info data.hwk --json  # Machine-readable JSON output
```

#### `get` — Data & Cell Extraction
Extracts cell data as sparse JSON, formatted Markdown, CSV, or raw values.
```bash
hasucalc get sales.hwk -r A1:D10 --format markdown
hasucalc get sales.hwk -r B2:B10 --format csv
hasucalc get sales.hwk -r B2:B10 --format values
hasucalc get sales.hwk --json  # Sparse JSON omitting empty cells
```

#### `eval` — Instant Formula Evaluation
Calculates formulas instantly from the command line, either as a standalone calculator or referencing cells in an existing workbook.
```bash
# Standalone calculation
hasucalc eval "=SUM(10, 20, 30) * 1.1"

# Contextual evaluation referencing a workbook
hasucalc eval -f sales.hwk "=XLOOKUP(23, A2:A25, B2:B25)"

# Raw output (value only, suitable for shell piping)
RATE=$(hasucalc eval -f sales.hwk "=B2/B10" --format raw)
```

#### `set` — Cell & Range Mutation
Updates single cells or rectangular ranges with values, strings, formulas, and format descriptors. Automatically recalculates the workbook and performs an atomic overwrite save.
```bash
hasucalc set sales.hwk B2 150
hasucalc set sales.hwk D4 "=SUM(D2:D3)" --fmt "(C2)"
hasucalc set sales.hwk B2:B10 0 --dry-run  # preview without saving
```

#### `batch` — Transactional Multi-Action Execution
Executes an atomic list of actions (`set_cell`, `set_range`, `clear`, `format`, `insert_row`, `delete_row`, `insert_col`, `delete_col`, `add_sheet`, `rename_sheet`, `delete_sheet`, `recalculate`) from a JSON file or standard input. **Guarantees complete rollback if any action fails.**
```bash
cat actions.json | hasucalc batch sales.hwk
hasucalc batch sales.hwk -i actions.json --dry-run
```

#### `chart` — Headless HD PNG Rendering
Renders a 1280×720 HD PNG chart without opening an interactive window.
```bash
hasucalc chart sales.hwk -o chart.png --type BAR --range-x A2:A10 --series-a B2:B10 --title "Q1 Performance"
```

### 11.2 Model Context Protocol (MCP) Server

HasuCalc includes a native, zero-dependency MCP stdio server complying with the JSON-RPC 2.0 specification. It provides 7 tools directly to AI coding assistants and autonomous agents:

1. `read_sheet`: Read cells and tabular data with range and format scoping.
2. `get_info`: Structural metadata inspection.
3. `evaluate_formula`: Standalone or contextual formula evaluation.
4. `edit_cell`: Atomic mutation of cells/ranges.
5. `batch_edit`: Transactional multi-action editing with rollback guarantee.
6. `render_chart`: Headless PNG chart rendering.
7. `convert_file`: 8-way file format conversion.

#### Client Configuration (e.g. Claude Desktop)
Add the following entry to your `claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "hasucalc": {
      "command": "/path/to/hasucalc",
      "args": ["mcp"]
    }
  }
}
```

For detailed protocol specifications, JSON schemas, and error contracts, see the authoritative specification in **[HEADLESS_SPEC.md](HEADLESS_SPEC.md)**.
