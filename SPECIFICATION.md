# HasuCalc 2.0 — Specification & Development History

> **Languages:** English (canonical) · [日本語](SPECIFICATION.ja.md)  
> Overview: [README.md](README.md) ([日本語](README.ja.md)) · **User manual:** [USER_MANUAL.md](USER_MANUAL.md) ([日本語](USER_MANUAL.ja.md))  
> If translations disagree, this English document wins.

Official documentation for the modern terminal spreadsheet **HasuCalc 2.0**: architecture, features, UI, menus, and development chronicle.

---

## Contents

1. [Overview & design](#1-overview--design)
   - [1.4 Spec contract (bugs vs intentional differences)](#14-spec-contract-bugs-vs-intentional-differences)
2. [Architecture & packages](#2-architecture--packages)
3. [Feature specification](#3-feature-specification)
   - [3.1 Cells, sheets, workbooks](#31-cells-sheets-workbooks)
   - [3.2 Formula engine](#32-formula-engine)
   - [3.3 UI & keybindings](#33-ui--keybindings)
   - [3.4 Native mouse](#34-native-mouse)
   - [3.5 Slash menus (/)](#35-slash-menus-)
   - [3.6 Find, replace, goto](#36-find-replace-goto)
   - [3.7 Freeze panes](#37-freeze-panes)
   - [3.8 AutoFill & transpose](#38-autofill--transpose)
   - [3.9 Charts & PNG export](#39-charts--png-export)
   - [3.10 File formats & I/O](#310-file-formats--io)
   - [3.11 File picker & save dialog](#311-file-picker--save-dialog)
   - [3.12 Multilingual typography](#312-multilingual-typography)
4. [Development chronicle](#4-development-chronicle)
5. [Testing & QA](#5-testing--qa)

---

## 1. Overview & design

### 1.1 Overview
**HasuCalc** is a fast, lightweight terminal spreadsheet (CLI / TUI) written from scratch in Go. It ships as a fully self-contained, CGO-free binary and starts quickly on macOS, Linux, and Windows terminals.

**Not an Excel-compatible app.** The authoritative model is HasuCalc’s own model and `.hwk` / `.hwkz`. `.xlsx` / `.ods` are **convenience bridges** for exchanging tabular data; reproducing Excel / LibreOffice features or layout is not a goal.

### 1.2 Core design principles
1. **Modern editing + retro TUI**: Keep PC-98 / DOS clarity and speed while adding familiar selection, `Ctrl+C/X/V/Z/Y`, a VS Code-like `Ctrl+K` palette, and native mouse support.
2. **Sparse-matrix scale**: Up to **1,048,576 × 16,384 (`A`–`XFD`)** while allocating only for used cells.
3. **LLM- and Git-friendly storage**: Default `.hwk` is compact uncompressed JSON—readable by `cat`, `grep`, `jq`, Python, and agents; `git diff` can track cell changes line-by-line.
4. **Self-contained image output**: HD PNG (1280×720) via Go graphics and system fonts (TrueType/OpenType)—no ImageMagick / gnuplot.

### 1.3 CLI options
```bash
hasucalc [file]         # Open .hwk, .hwkz, .xlsx, .ods, .csv, .md, .html
hasucalc --demo, -d     # Demo sheet with chart settings
hasucalc --version, -v  # Version (HasuCalc 2.0.2, Go runtime, OS/Arch)
hasucalc --help, -h     # Help
```

### 1.4 Spec contract (bugs vs intentional differences)

HasuCalc is neither a clone nor a compatibility product for Excel. **Only mismatches between this document and the implementation are bugs.** Differences from Excel / LibreOffice that are not written here are intentional and are not fix targets.

Lotus 1-2-3-style and familiar `=` / `A1:B10` notation are **both accepted** (the latter for convenience—not “Excel compatibility”). Evaluation, errors, and I/O scope follow the tables below.

#### Guaranteed (core — breakage here is a bug)

* Sparse grid (max 1,048,576×16,384; store used cells only).
* Cell types NUMBER / LABEL / FORMULA / BOOLEAN / EMPTY and input prefixes (§3.1).
* Formula lexing / parsing / evaluation, circular-ref detection, workbook recalc in AUTO mode.
* Reference retargeting on row/column insert/delete, cut/copy/paste, and sheet rename/delete (broken refs become `#REF!` or quoted sheet names).
* Native `.hwk` / `.hwkz` round-trip preserves values, formulas, names, chart settings, and recalc mode.
* XLSX / ODS round-trip preserves **tabular data** (cells, formulas, names, freeze panes, recalc mode). String literals and `value-type="string"` are not re-interpreted. These bridges do **not** guarantee Excel / LibreOffice features, display, or charts.
* Argument-less `ROW()` / `COLUMN()` return the formula cell’s own row / column.

#### Lotus 1-2-3 family (intentional; may differ from Excel)

| Topic | HasuCalc behavior |
|---|---|
| Formula prefix | `@SUM(A1..A10)`; leading `+` is also a formula |
| Ranges | `A1..B10` (`A1:B10` also OK) |
| Logical ops | `#AND#` / `#OR#` / `#NOT#` |
| Aliases | `AVG`=`AVERAGE`, `PAYMT`=`PMT`, `LENGTH`=`LEN`, `REPEAT`=`REPT`, `MULTIPLY`=`PRODUCT`, `STD`=`STDEV.P`, `STRING` (Lotus stringify) |
| Format | Scientific `(S2)` is an alias of `(E2)` |
| Errors | Generic `ERR`, missing `NA`, circular `CIRCULAR REF` — **not** split into Excel `#VALUE!` / `#NUM!` / `#DIV/0!` |
| `FIND` / `SEARCH` miss | `NA` (Excel: `#VALUE!`). `ISERR` excludes NA |
| `ERR()` / `NA()` | Explicitly return those errors |

#### Excel / OpenFormula-style notation (accepted; not a compatibility product)

Familiar syntax is accepted; implemented functions aim for nearby results. **“Same as Excel” is not a contract.**

| Topic | HasuCalc behavior |
|---|---|
| Prefix | `=` |
| Ranges | `A1:B10`, whole column `A:A`, sheet `Sheet2!A1`, quoted `'Q1-2024'!A1` when needed |
| Unary minus vs `^` | `-2^2` = `(-2)^2` = **4** (use `-(2^2)` for `-4`) |
| Booleans | `TRUE` / `FALSE` / `TRUE()` / `FALSE()`; numeric context 1 / 0 |
| `SUMIF` 3rd arg | Expanded from the top-left to match criteria shape |
| Date serials / major functions | Near Excel for implemented cases (`TIME` 24h wrap, US `DAYS360` end-of-month rules, etc.) |
| Grid | `A`–`XFD`, rows to `1048576` |
| UX | Selection, `Ctrl+C/X/V/Z/Y`, freeze panes, named ranges |

Excel function names are canonical; Lotus aliases are accepted only.

#### HasuCalc-specific (do not match other apps)

* **Native formats** `.hwk` (JSON) and `.hwkz` (gzip); prefer Git/LLM readability.
* **Charts**: one per sheet, series A–F; terminal draw + PNG (1280×720). **No XLSX/ODS chart I/O** (§3.9).
* Label alignment: `'` left, `"` right, `^` center; fill with `\`.
* Currency: **any** per-cell prefix string (no whitelist: `$` / `¥` / `€` / `£` / `USD `, …). Empty → `$`. No suffix currencies or locale grouping like `1.234,56`.
* Whole-column / huge ranges are clipped to the **used range**.
* TUI (tcell), slash menus, `Ctrl+K` palette, Markdown table export.
* **Markdown / HTML import** (convenience bridge): GFM pipe tables and HTML `<table>` become grid cells; other document text becomes column-A labels (§3.10).
* `%` is a percent **only immediately after a numeric literal** (`50%` → 0.5). `=A1%` is not accepted as a formula.
* In MANUAL recalc, insert/delete updates formula text but display values wait for the next recalc.

#### Out of scope (missing features are not bugs)

* VBA / macros, pivots, conditional formatting, merged cells, comments, data validation.
* Dynamic arrays / spill, legacy array formulas `{=...}`.
* Excel / ODS charts, shapes, slicers.
* Full Excel error taxonomy (1:1 `#VALUE!` ↔ `ERR`, etc.).
* Known desktop Excel quirks that contradict documented OpenFormula precedence (e.g. some `-2^2` environments). HasuCalc follows OpenFormula document precedence.
* Unlisted Excel functions, and full arg / locale / date-system coverage of listed ones.
* Full multilingual typography in the grid TUI (PNG path is primary; §5).

If an audit finds “different from Excel,” do **not** change behavior when it falls under Lotus / HasuCalc-specific / out-of-scope above. Fix only when a **core guarantee** breaks.

---

## 2. Architecture & packages

```
hasucalc/
├── main.go               # Entry, CLI parsing
├── version/              # Semantic version constant
├── coord/                # Addresses & ranges (A1, A1:B10, A1..B10, Sheet1!A1)
├── cell/                 # Types, values, display formats (currency, %, date, scientific)
├── formula/              # Lexer / Parser / AST / Evaluator / builtins
├── sheet/                # Sheets, workbook, I/O (hwk/hwkz/xlsx/ods/csv/markdown/html)
│   ├── sheet.go
│   ├── workbook.go
│   ├── xlsx.go / export_xlsx.go / export_ods.go / export_markdown.go
│   ├── import_markup.go  # Markdown / HTML import (tables → grid, prose → labels)
│   └── ods.go
├── tui/                  # Terminal UI, palette, menus, charts, typography
├── engine_test.go        # Integration / regression tests
├── mega_test.go          # Large scenario tests
├── bugfix_regression_test.go
├── SPECIFICATION.md      # This document (English, canonical)
├── SPECIFICATION.ja.md   # Japanese translation
├── USER_MANUAL.md        # End-user manual (UI, modes, function parameters)
└── USER_MANUAL.ja.md     # Japanese user manual
```

---

## 3. Feature specification

### 3.1 Cells, sheets, workbooks

* **Size**: 1,048,576 rows (`1`–`1048576`) × 16,384 columns (`A`–`XFD`).
* **Types**:
  * `NUMBER` (float64)
  * `LABEL` (string: left `'`, right `"`, center `^`)
  * `FORMULA` (`=...`, `@...`, `+...`)
  * `EMPTY`
* **Display formats**:
  * Currency: `(C2)` → `$1,234.56`, `(C2¥)` → `¥1,234.56`. Any prefix string; default `$`. Negatives as `(symbol+amount)`. No suffix currencies or `1.234,56` grouping.
  * Percent: `(P1)` → `12.3%`
  * Fixed: `(F2)` → `12.34`
  * Thousands: `(,)` → `1,234,567`
  * Scientific: `(E2)` → `1.23E+04` (Lotus alias `(S2)`)
  * Date: `(D1)`–`(D5)` → e.g. `2026/08/27`, `27-Aug-06`, …
* **Multi-sheet workbooks**:
  * Multiple worksheets per file.
  * Cross-sheet refs: `=Sheet2!A1*1.1`, `=SUM(Sales!B2:B10)`.
  * Sheet add/delete/rename participate in Undo/Redo of workbook structure.
* **Dependencies & recalc**:
  * AUTO mode recalculates the **whole workbook** in multiple passes (cross-sheet convergence).
  * Circular refs → `CIRCULAR REF`; missing sheets → `#REF!`.
  * Whole-column refs (`A:A`) and huge ranges stop at the **used range**.
  * `TRUE` / `FALSE` identifiers and `TRUE()` / `FALSE()` are booleans; numeric context → 1.0 / 0.0. `ISLOGICAL` is true only for booleans (not numeric 0/1).

---

### 3.2 Formula engine

Both Excel-style and Lotus-style input are accepted (the former is convenience notation, not Excel-product status). Precedence, errors, and aliases follow [§1.4](#14-spec-contract-bugs-vs-intentional-differences). Exact Excel parity is not contracted.

* **Prefixes**: `=` (common), `@` (function), `+` (classic).
* **Ranges**: `:` (e.g. `A1:B10`, `A1 : B10`) and `..` (e.g. `A1..B10`).
* **Power & unary minus**: Unary minus binds tighter than `^` (OpenFormula / Excel documented precedence). `-2^2` = `(-2)^2` = `4`. Write `-(2^2)` for `-4`.
* **Functions (70+)**:
  * **Math / stats**: `SUM`, `AVERAGE`, `COUNT`, `COUNTA`, `COUNTIF`, `SUMIF`, `MAX`, `MIN`, `ROUND`, `ROUNDUP`, `ROUNDDOWN`, `ABS`, `INT`, `MOD`, `SQRT`, `POWER`, `EXP`, `LN`, `LOG`, `LOG10`, `MEDIAN`, `STDEV`, `VAR`
  * **Trig**: `SIN`, `COS`, `TAN`, `ASIN`, `ACOS`, `ATAN`, `PI`, `DEGREES`, `RADIANS`
  * **Logic / lookup**: `IF`, `IFS`, `AND`, `OR`, `NOT`, `TRUE`, `FALSE`, `VLOOKUP`, `HLOOKUP`, `INDEX`, `MATCH`, `CHOOSE`, `ISNUMBER`, `ISTEXT`, `ISBLANK`, `ISERROR`, `IFERROR`
  * **Text**: `CONCAT`, `CONCATENATE`, `LEFT`, `RIGHT`, `MID`, `LEN`, `UPPER`, `LOWER`, `PROPER`, `TRIM`, `REPLACE`, `SUBSTITUTE`, `EXACT`, `FIND`, `SEARCH`, `TEXT`, `VALUE`, `REPT`
  * **Date / time**: `TODAY`, `NOW`, `DATE`, `YEAR`, `MONTH`, `DAY`, `HOUR`, `MINUTE`, `SECOND`, `DAYS`, `DATEDIF`
  * **Finance**: `PMT`, `PV`, `FV`, `NPV`, `IRR`, `RATE`, `NPER`

---

### 3.3 UI & keybindings

* **Move**: arrows
* **Select**: `Shift + arrows` or **mouse drag**
* **Edit**:
  * `F2` / `Ctrl + E`: inline edit (formula bar)
  * Typing starts new input
  * `Del`: clear cell/range
* **Clipboard**:
  * `Ctrl + C` copy
  * `Ctrl + X` cut (no relative shift on paste)
  * `Ctrl + V` paste (relative shift on copy; move-like on cut)
  * `Ctrl + L` paste link (`='Sheet'!A1`)
* **History**:
  * `Ctrl + Z` undo (including sheet structure)
  * `Ctrl + Y` redo
* **Find / goto**:
  * `Ctrl + F` find
  * `Ctrl + H` replace
  * `F3` / `Shift + F3` next / prev
  * `Ctrl + G` / `F5` goto
* **Palette / menus / other**:
  * `Ctrl + K` or `:` command palette
  * `/` slash menu
  * `Alt + =` AutoSum
  * `F9` recalc all sheets
  * `F10` full-screen chart
  * `Ctrl + S` / `Ctrl + O` save / open
  * `Ctrl + T` sheet modal
  * `Ctrl + PgDn` / `Ctrl + PgUp` next / prev sheet
  * `Ctrl + Q` quit (confirm if dirty)
  * `F1` help

---

### 3.4 Native mouse

* **Click**: move cursor; clears selection.
* **Click & drag**: rectangular selection; kept after release until the next plain click. Does not fight terminal text selection.
* **Wheel**: scroll grid by 3 rows.
* **Sheet tabs**: click bottom tabs to switch sheets.

---

### 3.5 Slash menus (/)

Ribbon-inspired hierarchy; also reachable from the `Ctrl+K` palette.

* **`/F` (File)**: `/FN` New, `/FO` Open, `/FS` Save, `/FX` Export (CSV/Excel/ODS/Markdown × Sheet/Range), `/FQ` Quit
* **`/H` (Home)**: Undo/Redo, Cut/Copy/Paste, Paste-Special (Values/Link/Transpose), Clear, Find/Next/Prev/Replace/Find-All, Goto, Number formats, Align, Cells (insert/delete/width)
* **`/I` (Insert)**: Function browser, Rows/Columns, Today/Now, Chart (`F10`)
* **`/O` (Formulas)**: AutoSum, Insert-Function, Average/Count/Max/Min, Recalculate (`F9`)
* **`/D` (Data)**: Sort, AutoFill, Fill, Transpose, Names
* **`/V` (View)**: Freeze-Panes, Select/Next/Prev sheet, Add/Delete/Rename
* **`/C` (Chart)**: View, Type, Title, axes/series, Status, Save-PNG
* **`/?` (Help)**: About, Keybindings (`F1`), Palette

---

### 3.6 Find, replace, goto

* **Goto (`Ctrl+G` / `F5` / `/HG`)**: cell (`B10`), range (`B2..D10`), other sheet (`Sheet2!A1`), named range (`Total`). Viewport scrolls if needed.
* **Find (`Ctrl+F` / `/HF`) & next/prev**: searches `RawInput`, computed `Value`, and formatted display; wraps around.
* **Replace (`Ctrl+H` / `/HE`)**: confirm `Y` / skip `N` / all `A` / cancel `Esc`; can replace cells whose *computed* value matches.
* **Find-All (`/HA`)**: workbook-wide; switches to the sheet that contains the hit.

---

### 3.7 Freeze panes

* **`/VF`**: Both `/VFB`, Horizontal `/VFH`, Vertical `/VFV`, Clear `/VFC`.
* Frozen header rows/columns stay painted at the top/left while scrolling.

---

### 3.8 AutoFill & transpose

* **AutoFill (`/DA`)**: detect pattern from first (and second) cells — numeric series, flexible date sequences.
* **Data Fill (`/DF`)**: interactive start / step (`1d`/`1w`/`1m`/`1y`/number) / stop.
* **Paste-Transpose (`/HST`)**: transpose clipboard with formula retargeting.
* **Range Transpose (`/DT`)**: transpose into a destination (clears leftovers on in-place transpose).

---

### 3.9 Charts & PNG export

#### Chart types
* **LINE**, **BAR**, **STACKED**, **PIE** (with legend / percent labels as applicable).

#### PNG engine (`graph_png.go`)
* **1280×720 (HD)**.
* Auto-detect system CJK fonts (e.g. Hiragino on macOS, Meiryo on Windows, Noto Sans CJK on Linux).
* Naming: `<basename>_<GRAPHTYPE>.png` (e.g. `sample_PIE.png`).
* From `F10` preview, press `S` to save PNG immediately.

#### Persistence
* Series / type / title live in sheet `GraphConfig` and are saved **only in `.hwk` / `.hwkz`**.
* Portable images are **PNG only**.
* **No chart I/O for `.xlsx` / `.ods`.** HasuCalc’s one-chart-per-sheet A–F model does not map 1:1 to Excel/LibreOffice charts; half-broken chart copy would look worse than none. Tabular data only; chart parts are ignored on read and omitted on write.

---

### 3.10 File formats & I/O

#### 1. Compact uncompressed `.hwk` (default)
Slim JSON without gzip:

```json
{
  "version": "HasuCalc/2.0",
  "name": "DATA.hwk",
  "sheets": [
    {
      "name": "Sheet1",
      "cells": {
        "A1": "ID",
        "B1": "Sales",
        "C1": "Profit",
        "A2": 1,
        "B2": 1200,
        "C2": 350,
        "D2": "=B2-C2",
        "B10": { "raw": "100", "fmt": "(C2)" }
      }
    }
  ]
}
```
* Roughly **80%+ smaller** than a naïve dump while staying uncompressed.
* Directly usable with `cat` / `grep` / `jq` / Python / LLM prompts.
* Cell edits show up as clean one-line `git diff` hunks.

#### 2. Transparent GZIP (`.hwkz` / `.hwk.gz`)
* Save compressed by extension; load via magic bytes `0x1f 0x8b` regardless of suffix.

#### 3. External formats

Authoritative storage is `.hwk` / `.hwkz`. The following are **tabular interchange** only—not a claim that HasuCalc replaces those apps.

* **Excel (`.xlsx`)**: multi-sheet cells/formulas/names, etc. **No charts** (§3.9). Not “Excel compatible.”
* **LibreOffice (`.ods`)**: tabular multi-sheet I/O. **No charts** (§3.9).
* **CSV (`.csv`)**: writes **UTF-8 BOM** (`0xEF, 0xBB, 0xBF`) to reduce mojibake in other apps.
* **Markdown (`.md` / `.markdown`)**:
  * **Export**: GFM pipe table from the sheet or a range (`/FX` → Markdown).
  * **Import** (CLI path or Open dialog): GFM `| ... |` tables become multi-column cells (separator rows like `|---|` are skipped). Non-table lines (headings, paragraphs, fenced code lines, etc.) become **labels in column A**. Prose is forced to LABEL so text that looks like `=SUM(...)` is not evaluated. Inline emphasis/links are simplified to plain text. Not a full CommonMark/GFM engine.
* **HTML (`.html` / `.htm`)**:
  * **Import only** (CLI path or Open dialog): each `<table>` becomes a grid block; block text from headings / paragraphs / list items / etc. becomes column-A labels. Nested markup inside cells is flattened to text. Scripts/styles/comments are ignored. Not a browser HTML engine; malformed or exotic markup may be incomplete.

Opening `.md` / `.html` replaces the current workbook with a single imported sheet and suggests a `.hwk` save name (same pattern as CSV import).

---

### 3.11 File picker & save dialog

* Inline editing with yellow block + hardware cursor sync; `←/→`, `Home/End`, `Delete/Backspace`, `Ctrl+U`, `Ctrl+K`.
* Extension normalization and overwrite confirmation.
* **Open dialog** (`Ctrl+O` / `/FO` / palette “Open File”): lists directories plus openable files:
  * Native: `.hwk`, `.hwkz`, `.hwk.gz`, `.json`
  * Office / text bridges: `.xlsx`, `.xlsm`, `.ods`, `.ots`, `.csv`, `.tsv`
  * Markup bridges: **`.md`**, **`.markdown`**, **`.html`**, **`.htm`**
  * Legacy: `.wk3`, `.123` (and `.123.json` where applicable)
* Selecting a `.md` / `.html` / `.htm` file runs the markup importer (§3.10). Selecting CSV/XLSX/ODS/HWK uses the existing importers/loaders.

---

### 3.12 Multilingual typography

* Simple scripts: Latin, Japanese, Chinese (SC/TC), Korean, Cyrillic, Greek.
* RTL / cursive joining: Arabic, Urdu, Persian, Hebrew (BiDi + contextual shaping).
* Complex clusters: Hindi, Bengali, Thai.

---

## 4. Development chronicle

* **Phase 1**: Sparse cells, 70+ function lexer/parser/evaluator, DAG recalc.
* **Phase 2**: tcell grid, selection, Ctrl+C/X/V/Z/Y, Ctrl+K palette, multi-sheet.
* **Phase 3**: F10 terminal charts (Line, Bar, Stacked, Pie).
* **Phase 4**: 1280×720 PNG export with system fonts.
* **Phase 5**: HasuCalc 2.0 branding / menu under title.
* **Phase 6**: Compact JSON serializer (~80% smaller) + transparent gzip.
* **Phase 7**: Stronger save dialog editing.
* **Phase 8**: Global typography (BiDi, Arabic joining, Devanagari/Thai clusters).
* **Phase 9**: Cross-sheet copy/paste with formula shift.
* **Phase 10**: Goto / find-replace / freeze-pane scroll fixes.
* **Phase 11**: AutoFill, Data Fill, Paste-Transpose, Range Transpose.
* **Phase 12**: Menu slimming and Freeze-Panes naming (`/VF`).

---

## 5. Testing & QA

HasuCalc 2.0 pins regressions in `engine_test.go`, `mega_test.go`, and `bugfix_regression_test.go`. Tests enforce the [§1.4](#14-spec-contract-bugs-vs-intentional-differences) contract—not full Excel coverage.

1. Formula & function evaluation (errors, cross-sheet).
2. Recalc & circular refs; used-range limits for whole-column refs.
3. File I/O: compact `.hwk`, gzip `.hwkz`, `.xlsx`, ODS, CSV (UTF-8 BOM), Markdown/HTML import (tables + prose labels).
4. PNG export generation / signature checks.
5. UI: file picker editing (including Open listing of `.md` / `.html`), mouse drag, wheel.
6. Find / replace / goto (including sheet sync and computed values).
7. Freeze / AutoFill / Transpose / multi-sheet Undo isolation.

Multilingual typography (BiDi / ligatures) is applied mainly on the PNG path; full grid-TUI coverage is future work.
