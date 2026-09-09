# HasuCalc

> **Languages:** English (canonical) · [日本語](README.ja.md)  
> **User manual (detailed):** [USER_MANUAL.md](USER_MANUAL.md) ([日本語](USER_MANUAL.ja.md))  
> Engineering specification: [SPECIFICATION.md](SPECIFICATION.md) ([日本語](SPECIFICATION.ja.md))

Terminal spreadsheet with a classic DOS / PC-98 feel and modern editing (`Shift`+arrows, `Ctrl+C/V/S`, `Ctrl+K`, `=SUM(A1:B10)`).

**Not an Excel-compatible app.** Native formats are `.hwk` / `.hwkz`. `.xlsx` / `.ods` / `.csv` / `.md` / `.html` are convenience bridges only.

Single CGO-free Go binary: `hasucalc`.

## Quick start

```bash
./hasucalc              # blank sheet
./hasucalc --demo       # demo data + chart
./hasucalc my_sheet.hwk
./hasucalc notes.md     # tables → grid; other text → labels
```

For screen layout, `[READY]` / `[CALC]` meanings, parameters of every function, menus, and file formats, see the **[User Manual](USER_MANUAL.md)**.

## Keybindings (summary)

| Key | Action |
|:---|:---|
| Arrows / Shift+Arrows | Move / select |
| F2 / Ctrl+E | Edit cell |
| Ctrl+C / X / V / L | Copy / Cut / Paste / Paste link |
| Ctrl+Z / Y | Undo / Redo |
| Ctrl+S / O | Save / Open |
| Ctrl+K or `:` | Command palette |
| Ctrl+F / H | Find / Replace |
| F3 / Shift+F3 | Find next / prev |
| F5 / Ctrl+G | Goto |
| F9 | Recalculate |
| F10 | Chart (`S` = PNG) |
| Alt+= | AutoSum |
| `/` | Slash menu |
| F1 | In-app help |
| Ctrl+Q | Quit |

Full list and mode explanations: [USER_MANUAL.md §5](USER_MANUAL.md#5-keybindings).

## Function names

Use `=NAME(...)` or `@NAME(...)`. Optional arguments are documented in the [function reference](USER_MANUAL.md#9-function-reference).

**Math/Agg:** `SUM`, `SUMIF`, `SUMIFS`, `SUMPRODUCT`, `PRODUCT`, `SUBTOTAL`, `ROUND`, `ROUNDUP`, `ROUNDDOWN`, `TRUNC`, `INT`, `ABS`, `MOD`, `QUOTIENT`, `SIGN`, `POWER`, `SQRT`, `EXP`, `LN`, `LOG`, `LOG10`, `CEILING`, `FLOOR`, `MROUND`, `FACT`, `GCD`, `LCM`, `COMBIN`, `PERMUT`, `PI`, `DEGREES`, `RADIANS`, `SIN`, `COS`, `TAN`, `ASIN`, `ACOS`, `ATAN`, `ATAN2`, `RAND`, `RANDBETWEEN`

**Statistical:** `AVG`, `AVERAGEIF`, `AVERAGEIFS`, `COUNT`, `COUNTA`, `COUNTBLANK`, `COUNTIF`, `COUNTIFS`, `MIN`, `MINIFS`, `MAX`, `MAXIFS`, `MEDIAN`, `MODE`, `LARGE`, `SMALL`, `PERCENTILE`, `QUARTILE`, `STDEV`, `STDEVP`, `VAR`, `VARP`, `RANK`

**Lookup/Ref:** `XLOOKUP`, `VLOOKUP`, `HLOOKUP`, `LOOKUP`, `INDEX`, `MATCH`, `XMATCH`, `OFFSET`, `CHOOSE`, `ROW`, `COLUMN`, `ROWS`, `COLUMNS`, `TRANSPOSE`

**Logic/Error:** `IF`, `IFS`, `SWITCH`, `AND`, `OR`, `NOT`, `XOR`, `IFERROR`, `IFNA`, `ISNUMBER`, `ISSTRING`, `ISTEXT`, `ISNONTEXT`, `ISBLANK`, `ISLOGICAL`, `ISERR`, `ISNA`, `ISEVEN`, `ISODD`, `TRUE`, `FALSE`, `N`, `T`, `TYPE`

**Text:** `TEXT`, `TRIM`, `CLEAN`, `SUBSTITUTE`, `REPLACE`, `REPT`, `UPPER`, `LOWER`, `PROPER`, `EXACT`, `CHAR`, `CODE`, `UNICHAR`, `UNICODE`, `CONCATENATE`, `CONCAT`, `TEXTJOIN`, `LEFT`, `RIGHT`, `MID`, `LEN`, `FIND`, `SEARCH`, `STRING`, `VALUE`, `NUMBERVALUE`, `TEXTBEFORE`, `TEXTAFTER`, `TEXTSPLIT`

**Date/Time:** `TODAY`, `NOW`, `DATE`, `DATEVALUE`, `TIME`, `TIMEVALUE`, `DATEDIF`, `DAYS`, `DAYS360`, `NETWORKDAYS`, `WORKDAY`, `YEARFRAC`, `YEAR`, `MONTH`, `DAY`, `HOUR`, `MINUTE`, `SECOND`, `WEEKDAY`, `WEEKNUM`, `EDATE`, `EOMONTH`

**Financial:** `PMT`, `PV`, `FV`, `NPV`, `IRR`, `RATE`, `NPER`, `SLN`, `SYD`, `DDB`

Aliases such as `AVERAGE`←`AVG`, `LEN`←`LENGTH`, `PMT`←`PAYMT` are accepted; see the manual.
