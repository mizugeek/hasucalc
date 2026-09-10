# HasuCalc 2.0 — Specification & Development History

> **Languages:** English (canonical) · [日本語](SPECIFICATION.ja.md)  
> Overview: [README.md](README.md) ([日本語](README.ja.md)) · **User manual:** [USER_MANUAL.md](USER_MANUAL.md) ([日本語](USER_MANUAL.ja.md))  
> If translations disagree, this English document wins.

Official documentation for the modern terminal spreadsheet **HasuCalc 2.0**: architecture, features, UI, menus, and development chronicle.

---

## Contents

1. [Overview & design](#1-overview--design)
   - [1.4 Specification policy and core guarantees](#14-specification-policy-and-core-guarantees)
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

**Design independence and data interoperability**: HasuCalc is an independent terminal spreadsheet application whose authoritative model and native formats are `.hwk` / `.hwkz`. Support for external formats like `.xlsx` and `.ods` serves as a bridge for exchanging tabular data, not as a clone or replica of other software.

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

### 1.4 Specification policy and core guarantees

The architecture and behavior documented here represent the authoritative specification for HasuCalc. The application accepts both standard spreadsheet notation and classic syntax while maintaining a streamlined model optimized for terminal environments.

#### 1. Core behavioral guarantees

The following features and behaviors are guaranteed as HasuCalc's core specification:
* **Sparse-matrix grid**: Supports up to 1,048,576 rows × 16,384 columns (`A`–`XFD`), allocating memory only for cells in use.
* **Cell types and prefixes**: Full preservation of `NUMBER`, `LABEL` (`'` left, `"` right, `^` center), `FORMULA`, `BOOLEAN`, and `EMPTY` (§3.1).
* **Formula engine**: Lexing, parsing, and evaluation; circular reference detection (`CIRCULAR REF`); automatic recalculation in AUTO mode.
* **Reference retargeting**: Cell references update automatically across row/column insert/delete, cut/copy/paste, and sheet add/delete/rename (invalid references become `#REF!`).
* **Native format integrity**: Complete round-trip preservation of cell values, formulas, names, chart settings, and recalculation modes in `.hwk` / `.hwkz`.
* **Tabular data interoperability**: Accurate round-trip preservation of tabular data (cells, formulas, named ranges, freeze panes, recalculation mode) in `.xlsx` and `.ods` (charts and presentation objects are out of scope; §3.9).
* **Cell coordinate functions**: Argument-less `ROW()` / `COLUMN()` return the formula cell’s own row / column index.

#### 2. Formula and syntax support

HasuCalc supports both standard formula syntax and classic syntax conventions:

| Element | Standard syntax | Classic syntax | Notes |
|---|---|---|---|
| Formula prefix | `=` (e.g. `=SUM(A1:B10)`) | `@` (e.g. `@SUM(A1..B10)`), `+` (e.g. `+A1+B1`) | All are parsed and evaluated as formulas |
| Range separator | `:` (e.g. `A1:B10`) | `..` (e.g. `A1..B10`) | Whole-column references like `A:A` stop at the used range |
| Logical operators | Functions `AND()`, `OR()`, `NOT()` | Inline `#AND#`, `#OR#`, `#NOT#` | Can be used directly within expressions |
| Function aliases | Canonical (`AVERAGE`, `LEN`, `REPT`, `PMT`, `PRODUCT`, `STDEV.P`) | Aliases (`AVG`, `LENGTH`, `REPEAT`, `PAYMT`, `MULTIPLY`, `STD`, `STRING`) | Evaluated identically to canonical names |
| Scientific format | `(E2)` (e.g. `1.23E+04`) | `(S2)` | Supported as alias |
| Operator precedence | Unary minus binds tighter than `^` | — | `-2^2` evaluates to `(-2)^2` = `4` (use `-(2^2)` for `-4`) |

#### 3. Error handling taxonomy

HasuCalc consolidates error values into four clear indicators:

| Indicator | Meaning and conditions |
|---|---|
| `ERR` | General computation error (division by zero, invalid argument types, domain errors). Can be explicitly generated with `ERR()`. |
| `NA` | Missing or not found value (lookup misses in `VLOOKUP`, `MATCH`, etc., or the `NA()` function). |
| `CIRCULAR REF` | Circular dependency detected among cells. |
| `#REF!` | Invalid reference resulting from row/column or worksheet deletion. |

*Note*: `FIND` and `SEARCH` return `NA` when substrings are not found, and `ISERR` excludes `NA`.

#### 4. HasuCalc-specific characteristics

* **Storage formats**: Compact uncompressed JSON (`.hwk`) and transparent gzip (`.hwkz`), optimized for `git diff` and LLM/agent processing.
* **Chart subsystem**: One configurable chart per sheet (series A–F), featuring instant terminal preview and 1280×720 HD PNG export (§3.9).
* **Flexible currency symbols**: Arbitrary prefix strings (e.g. `$`, `¥`, `€`, `£`, `USD `) configurable per cell (§3.1).
* **Percent syntax**: `%` is interpreted as a percentage only directly after a numeric literal (e.g. `50%` → 0.5); `=A1%` is not accepted.
* **Markup ingestion**: Convenience bridges to extract tables into cells and plain text into column-A labels from Markdown (`.md`) and HTML (`.html`) files (§3.10).

#### 5. Deliberately out of scope

To preserve speed, simplicity, and terminal focus, the following features are not part of HasuCalc:
* Script/macro execution (e.g. VBA)
* Pivot tables, slicers, conditional formatting, merged cells, cell comments, data validation
* Dynamic array formulas and spill ranges
* External format charts, shapes, or non-tabular objects in `.xlsx` / `.ods`

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
  * Scientific: `(E2)` → `1.23E+04` (alias `(S2)`)
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

Supports standard `=` formulas as well as classic `@` function syntax and `+` expressions. Operator precedence, error reporting, and aliases follow [§1.4](#14-specification-policy-and-core-guarantees).

* **Prefixes**: `=` (standard formula), `@` (function syntax), `+` (classic formula).
* **Ranges**: `:` (e.g. `A1:B10`) and `..` (e.g. `A1..B10`).
* **Power & unary minus**: Unary minus binds tighter than `^` (`-2^2` = `(-2)^2` = `4`; write `-(2^2)` for `-4`).
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

Logically organized hierarchical menus; also accessible via the `Ctrl+K` command palette.

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
* **External format (`.xlsx` / `.ods`) policy**: HasuCalc's charting system uses a specialized model (one chart per sheet, series A–F, terminal view + HD PNG export). When reading or writing external spreadsheet files, only tabular data (cells, formulas, names) is exchanged; chart components are omitted on write and skipped on read. For sharing or exporting visualizations, use PNG export (§3.9.2).

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

The authoritative formats are `.hwk` / `.hwkz`. The following external formats are supported for **tabular data interoperability**:

* **Excel workbooks (`.xlsx`)**: multi-sheet cells, formulas, named ranges, and table structure (charts are excluded; see §3.9).
* **LibreOffice Calc (`.ods`)**: multi-sheet tabular data (charts are excluded; see §3.9).
* **CSV (`.csv`)**: includes **UTF-8 BOM** (`0xEF, 0xBB, 0xBF`) to ensure clean character rendering across spreadsheet applications.
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

HasuCalc 2.0 ensures quality and prevents regressions through test suites in `engine_test.go`, `mega_test.go`, and `bugfix_regression_test.go`. Tests verify compliance with the [§1.4](#14-specification-policy-and-core-guarantees) specification.

1. Formula & function evaluation (errors, cross-sheet).
2. Recalc & circular refs; used-range limits for whole-column refs.
3. File I/O: compact `.hwk`, gzip `.hwkz`, `.xlsx`, ODS, CSV (UTF-8 BOM), Markdown/HTML import (tables + prose labels).
4. PNG export generation / signature checks.
5. UI: file picker editing (including Open listing of `.md` / `.html`), mouse drag, wheel.
6. Find / replace / goto (including sheet sync and computed values).
7. Freeze / AutoFill / Transpose / multi-sheet Undo isolation.

Multilingual typography (BiDi / ligatures) is applied mainly on the PNG path; full grid-TUI coverage is future work.
