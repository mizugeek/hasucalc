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

Functions accept either `@NAME(...)` or `NAME(...)` after `=`. Optional arguments are shown in `[brackets]`.

### Math/Agg

| Function | Syntax | Description |
|:---|:---|:---|
| `SUM` | `SUM(range/list)` | Calculates total sum of numbers in range |
| `SUMIF` | `SUMIF(range, criteria, [sum_range])` | Sums cells meeting specified criteria (e.g. \ |
| `SUMIFS` | `SUMIFS(sum_rng, crit_rng1, crit1, ...)` | Sums cells that meet multiple criteria across ranges |
| `SUMPRODUCT` | `SUMPRODUCT(array1, [array2]...)` | Calculates sum of products of corresponding items |
| `PRODUCT` | `PRODUCT(number1, [number2]...)` | Multiplies all numbers given in arguments |
| `SUBTOTAL` | `SUBTOTAL(function_num, ref1, ...)` | Calculates subtotal in list/database (9=SUM, 1=AVG, etc.) |
| `ROUND` | `ROUND(val, num_digits)` | Rounds number to specified decimal places |
| `ROUNDUP` | `ROUNDUP(val, num_digits)` | Rounds number up, away from zero |
| `ROUNDDOWN` | `ROUNDDOWN(val, num_digits)` | Rounds number down, towards zero |
| `TRUNC` | `TRUNC(val, [num_digits])` | Truncates number to specified decimal places |
| `INT` | `INT(val)` | Rounds number down to nearest integer |
| `ABS` | `ABS(val)` | Returns absolute value of number |
| `MOD` | `MOD(number, divisor)` | Returns remainder after division (modulo) |
| `QUOTIENT` | `QUOTIENT(numerator, denominator)` | Returns integer portion of a division |
| `SIGN` | `SIGN(number)` | Returns sign of number (1=pos, -1=neg, 0=zero) |
| `POWER` | `POWER(number, power)` | Calculates number raised to a power (x^y) |
| `SQRT` | `SQRT(val)` | Calculates square root of positive number |
| `EXP` | `EXP(number)` | Returns e raised to the power of number |
| `LN` | `LN(number)` | Returns natural logarithm of number |
| `LOG` | `LOG(number, [base])` | Returns logarithm of number to specified base (default 10) |
| `LOG10` | `LOG10(number)` | Returns base-10 logarithm of number |
| `CEILING` | `CEILING(number, significance)` | Rounds number up to nearest multiple of significance |
| `FLOOR` | `FLOOR(number, significance)` | Rounds number down to nearest multiple of significance |
| `MROUND` | `MROUND(number, multiple)` | Rounds number to nearest multiple |
| `FACT` | `FACT(number)` | Calculates factorial of a number (n!) |
| `GCD` | `GCD(number1, number2, ...)` | Returns greatest common divisor |
| `LCM` | `LCM(number1, number2, ...)` | Returns least common multiple |
| `COMBIN` | `COMBIN(n, k)` | Returns number of combinations for n items choose k (nCr) |
| `PERMUT` | `PERMUT(n, k)` | Returns number of permutations for n items choose k (nPr) |
| `PI` | `PI()` | Returns constant value of Pi (3.14159265...) |
| `DEGREES` | `DEGREES(angle_in_radians)` | Converts radians to degrees |
| `RADIANS` | `RADIANS(angle_in_degrees)` | Converts degrees to radians |
| `SIN` | `SIN(number)` | Returns sine of an angle in radians |
| `COS` | `COS(number)` | Returns cosine of an angle in radians |
| `TAN` | `TAN(number)` | Returns tangent of an angle in radians |
| `ASIN` | `ASIN(number)` | Returns arcsine (inverse sine) in radians |
| `ACOS` | `ACOS(number)` | Returns arccosine (inverse cosine) in radians |
| `ATAN` | `ATAN(number)` | Returns arctangent in radians |
| `ATAN2` | `ATAN2(x_num, y_num)` | Returns arctangent from x and y coordinates |
| `RAND` | `RAND()` | Returns random real number between 0 and 1 |
| `RANDBETWEEN` | `RANDBETWEEN(min, max)` | Returns random integer between min and max (inclusive) |

### Statistical

| Function | Syntax | Description |
|:---|:---|:---|
| `AVG` | `AVG(range/list)` | Calculates arithmetic mean (average) |
| `AVERAGEIF` | `AVERAGEIF(range, criteria, [avg_range])` | Calculates average of cells meeting criteria |
| `AVERAGEIFS` | `AVERAGEIFS(avg_rng, crit_rng1, crit1, ...)` | Calculates average of cells meeting multiple criteria |
| `COUNT` | `COUNT(range/list)` | Counts number of numeric cells in range |
| `COUNTA` | `COUNTA(range/list)` | Counts number of non-empty cells in range |
| `COUNTBLANK` | `COUNTBLANK(range)` | Counts number of empty cells in range |
| `COUNTIF` | `COUNTIF(range, criteria)` | Counts number of cells meeting criteria |
| `COUNTIFS` | `COUNTIFS(crit_rng1, crit1, ...)` | Counts cells that meet multiple criteria across ranges |
| `MIN` | `MIN(range/list)` | Finds minimum value in range/list |
| `MINIFS` | `MINIFS(min_rng, crit_rng1, crit1, ...)` | Finds minimum value among cells meeting multiple criteria |
| `MAX` | `MAX(range/list)` | Finds maximum value in range/list |
| `MAXIFS` | `MAXIFS(max_rng, crit_rng1, crit1, ...)` | Finds maximum value among cells meeting multiple criteria |
| `MEDIAN` | `MEDIAN(range/list)` | Returns median (middle value) of numbers |
| `MODE` | `MODE(range/list)` | Returns most frequently occurring value in data set |
| `LARGE` | `LARGE(array, k)` | Returns k-th largest value in a data set |
| `SMALL` | `SMALL(array, k)` | Returns k-th smallest value in a data set |
| `PERCENTILE` | `PERCENTILE(array, k)` | Returns k-th percentile of values in a range (0..1) |
| `QUARTILE` | `QUARTILE(array, quart)` | Returns quartile of data set (0..4) |
| `STDEV` | `STDEV(range/list)` | Estimates sample standard deviation (n-1) |
| `STDEVP` | `STDEVP(range/list)` | Calculates population standard deviation (n) |
| `VAR` | `VAR(range/list)` | Estimates sample variance (n-1) |
| `VARP` | `VARP(range/list)` | Calculates population variance (n) |
| `RANK` | `RANK(num, range, [order])` | Returns rank of a number in a range (0=desc, 1=asc) |

### Lookup/Ref

| Function | Syntax | Description |
|:---|:---|:---|
| `XLOOKUP` | `XLOOKUP(key, lk_rng, ret_rng, [fallback], [match], [search])` | Modern 2-way exact & approximate lookup with fallback value |
| `VLOOKUP` | `VLOOKUP(key, table_range, col_offset, [exact])` | Searches leftmost column and returns offset column value |
| `HLOOKUP` | `HLOOKUP(key, table_range, row_offset, [exact])` | Searches topmost row and returns offset row value |
| `LOOKUP` | `LOOKUP(val, lookup_vector, [result_vector])` | Looks up value in 1-row or 1-column range |
| `INDEX` | `INDEX(range, col_offset, row_offset)` | Returns cell value at intersection coordinate (0-based) |
| `MATCH` | `MATCH(key, lookup_array, [match_type])` | Returns index position of matched item in array (1-based) |
| `XMATCH` | `XMATCH(key, lookup_array, [match_mode], [search_mode])` | Modern position lookup with exact, wildcard, and reverse search |
| `OFFSET` | `OFFSET(ref, rows, cols, [height], [width])` | Returns reference offset from starting cell/range |
| `CHOOSE` | `CHOOSE(index, val0, val1, val2...)` | Selects and returns value from list by 0-based index |
| `ROW` | `ROW([cell])` | Returns row number of current or referenced cell (1-based) |
| `COLUMN` | `COLUMN([cell])` | Returns column number of current or referenced cell (1-based) |
| `ROWS` | `ROWS(range)` | Returns total number of rows in specified range |
| `COLUMNS` | `COLUMNS(range)` | Returns total number of columns in specified range |
| `TRANSPOSE` | `TRANSPOSE(array)` | Transposes rows and columns of an array |

### Logic/Error

| Function | Syntax | Description |
|:---|:---|:---|
| `IF` | `IF(condition, true_val, false_val)` | Conditional three-way branching |
| `IFS` | `IFS(cond1, val1, [cond2, val2]...)` | Evaluates multiple conditions in sequence |
| `SWITCH` | `SWITCH(expr, val1, res1, [val2, res2]..., [default])` | Evaluates expression against a list of values |
| `AND` | `AND(logical1, [logical2], ...)` | Returns true if all arguments are true |
| `OR` | `OR(logical1, [logical2], ...)` | Returns true if any argument is true |
| `NOT` | `NOT(logical)` | Reverses the logical value of argument |
| `XOR` | `XOR(logical1, [logical2]...)` | Returns exclusive OR of arguments |
| `IFERROR` | `IFERROR(formula, fallback_val)` | Returns fallback value if formula results in error |
| `IFNA` | `IFNA(formula, fallback_val)` | Returns fallback value if formula results in #N/A |
| `ISNUMBER` | `ISNUMBER(val)` | Tests if value is a numeric number (returns 1 or 0) |
| `ISSTRING` | `ISSTRING(val)` | Tests if value is a text string (returns 1 or 0) |
| `ISTEXT` | `ISTEXT(val)` | Tests if value is text (returns 1 or 0) |
| `ISNONTEXT` | `ISNONTEXT(val)` | Tests if value is not text (returns 1 or 0) |
| `ISBLANK` | `ISBLANK(val)` | Tests if referenced cell is blank/empty |
| `ISLOGICAL` | `ISLOGICAL(val)` | Tests if value is a logical boolean |
| `ISERR` | `ISERR(val)` | Tests if value is an error #ERR (returns 1 or 0) |
| `ISNA` | `ISNA(val)` | Tests if value is #N/A (returns 1 or 0) |
| `ISEVEN` | `ISEVEN(number)` | Tests if number is even (returns 1 or 0) |
| `ISODD` | `ISODD(number)` | Tests if number is odd (returns 1 or 0) |
| `TRUE` | `TRUE()` | Returns boolean true |
| `FALSE` | `FALSE()` | Returns boolean false |
| `N` | `N(value)` | Converts value to a numeric number |
| `T` | `T(value)` | Returns text string if value is text, empty string otherwise |
| `TYPE` | `TYPE(value)` | Returns integer code for value data type (1=num, 2=text, etc.) |

### Text

| Function | Syntax | Description |
|:---|:---|:---|
| `TEXT` | `TEXT(value, format_string)` | Formats number or date with custom format string (e.g. \ |
| `TRIM` | `TRIM(text)` | Strips leading/trailing spaces and collapses internal spaces |
| `CLEAN` | `CLEAN(text)` | Removes all non-printable characters from text |
| `SUBSTITUTE` | `SUBSTITUTE(text, old_text, new_text, [instance])` | Replaces occurrences of substring in text |
| `REPLACE` | `REPLACE(old_text, start_pos, num_chars, new_text)` | Replaces characters at position within text |
| `REPT` | `REPT(text, number_times)` | Repeats text a given number of times |
| `UPPER` | `UPPER(text)` | Converts all letters in text to UPPERCASE |
| `LOWER` | `LOWER(text)` | Converts all letters in text to lowercase |
| `PROPER` | `PROPER(text)` | Converts text to Title Case (capitalizes each word) |
| `EXACT` | `EXACT(text1, text2)` | Tests if two text values are exactly identical (case-sensitive) |
| `CHAR` | `CHAR(number)` | Returns character specified by ASCII/code number |
| `CODE` | `CODE(text)` | Returns numeric code for the first character in text string |
| `UNICHAR` | `UNICHAR(number)` | Returns Unicode character specified by numeric value |
| `UNICODE` | `UNICODE(text)` | Returns numeric Unicode codepoint of first character |
| `CONCATENATE` | `CONCATENATE(text1, text2, ...)` | Joins multiple text strings into a single string |
| `CONCAT` | `CONCAT(text1, text2, ...)` | Concatenates list or range of text items |
| `TEXTJOIN` | `TEXTJOIN(delimiter, ignore_empty, text1, ...)` | Joins text strings with a custom delimiter and options |
| `LEFT` | `LEFT(text, num_chars)` | Extracts leftmost characters from text string |
| `RIGHT` | `RIGHT(text, num_chars)` | Extracts rightmost characters from text string |
| `MID` | `MID(text, start_pos, num_chars)` | Extracts substring from middle of text string |
| `LEN` | `LEN(text)` | Returns total number of characters in text string |
| `FIND` | `FIND(find_text, within_text, [start])` | Case-sensitive search for text position (1-based) |
| `SEARCH` | `SEARCH(find_text, within_text, [start])` | Case-insensitive & wildcard (*, ?) text position search |
| `STRING` | `STRING(number, decimal_places)` | Formats number as string with fixed decimal places |
| `VALUE` | `VALUE(text)` | Converts numeric text string (with $, ¥, commas) to number |
| `NUMBERVALUE` | `NUMBERVALUE(text, [dec_sep], [group_sep])` | Parses formatted number text with locale separators |
| `TEXTBEFORE` | `TEXTBEFORE(text, delimiter)` | Extracts text occurring before delimiter |
| `TEXTAFTER` | `TEXTAFTER(text, delimiter)` | Extracts text occurring after delimiter |
| `TEXTSPLIT` | `TEXTSPLIT(text, col_delimiter)` | Splits text into array by delimiter |

### Date/Time

| Function | Syntax | Description |
|:---|:---|:---|
| `TODAY` | `TODAY()` | Returns serial number of current date |
| `NOW` | `NOW()` | Returns serial number of current date and time |
| `DATE` | `DATE(year, month, day)` | Creates date serial number from year, month, and day |
| `DATEVALUE` | `DATEVALUE(date_text)` | Converts date text (e.g. \ |
| `TIME` | `TIME(hour, minute, second)` | Creates time decimal fraction (0.0..1.0) from hour, min, sec |
| `TIMEVALUE` | `TIMEVALUE(time_text)` | Converts time text (e.g. \ |
| `DATEDIF` | `DATEDIF(start_date, end_date, unit)` | Calculates difference between two dates (unit: \ |
| `DAYS` | `DAYS(end_date, start_date)` | Returns number of days between two dates |
| `DAYS360` | `DAYS360(start_date, end_date)` | Calculates difference based on a 360-day year (12 months of 30 days) |
| `NETWORKDAYS` | `NETWORKDAYS(start_date, end_date, [holidays])` | Returns number of working days between two dates |
| `WORKDAY` | `WORKDAY(start_date, days, [holidays])` | Returns date before or after specified number of workdays |
| `YEARFRAC` | `YEARFRAC(start_date, end_date)` | Calculates fraction of year represented by number of whole days |
| `YEAR` | `YEAR(serial_date)` | Extracts 4-digit year from date serial |
| `MONTH` | `MONTH(serial_date)` | Extracts month number (1..12) from date serial |
| `DAY` | `DAY(serial_date)` | Extracts day of month (1..31) from date serial |
| `HOUR` | `HOUR(time_serial)` | Extracts hour (0..23) from time serial |
| `MINUTE` | `MINUTE(time_serial)` | Extracts minute (0..59) from time serial |
| `SECOND` | `SECOND(time_serial)` | Extracts second (0..59) from time serial |
| `WEEKDAY` | `WEEKDAY(serial_date, [type])` | Returns weekday number (1=Sun..7=Sat, or 1=Mon..7=Sun) |
| `WEEKNUM` | `WEEKNUM(serial_date)` | Returns week number of the year (1..53) |
| `EDATE` | `EDATE(start_date, months)` | Returns date serial n months before or after start date |
| `EOMONTH` | `EOMONTH(start_date, months)` | Returns last day of month n months before or after start date |

### Financial

| Function | Syntax | Description |
|:---|:---|:---|
| `PMT` | `PMT(rate, nper, pv, [fv], [type])` | Calculates periodic loan payment with constant interest rate |
| `PV` | `PV(rate, nper, pmt, [fv], [type])` | Calculates present value of an investment/loan |
| `FV` | `FV(rate, nper, pmt, [pv], [type])` | Calculates future value of an investment with periodic payments |
| `NPV` | `NPV(rate, val1, [val2]...)` | Calculates net present value using discount rate and cash flows |
| `IRR` | `IRR(values, [guess])` | Calculates internal rate of return for a series of cash flows |
| `RATE` | `RATE(nper, pmt, pv, [fv], [type])` | Calculates interest rate per period of an annuity |
| `NPER` | `NPER(rate, pmt, pv, [fv], [type])` | Returns number of periods for an investment/loan |
| `SLN` | `SLN(cost, salvage, life)` | Returns straight-line depreciation of an asset for one period |
| `SYD` | `SYD(cost, salvage, life, per)` | Returns sum-of-years' digits depreciation for specified period |
| `DDB` | `DDB(cost, salvage, life, period, [factor])` | Returns double-declining balance depreciation of an asset |

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
