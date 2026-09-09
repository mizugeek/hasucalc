# HasuCalc (Modern Terminal Spreadsheet in Go)

> **Languages:** English (canonical) · [日本語](README.ja.md)  
> Full specification: [SPECIFICATION.md](SPECIFICATION.md) ([日本語](SPECIFICATION.ja.md))

A fast terminal spreadsheet that keeps the look, colors, and charting feel of classic DOS / PC-98 sheet apps, while adding modern editing comfort (Shift+arrow selection, Ctrl+C/V/S, Ctrl+K command palette, `=SUM(A1:B10)` formulas).

**This is not an Excel-compatible application.** The native formats (`.hwk` / `.hwkz`) are authoritative. `.xlsx` / `.ods` I/O is only a convenience bridge for exchanging tabular data with other software.

Implemented in Go as a **single, CGO-free binary (`hasucalc`)**.

---

## Getting started

```bash
cd ~/hasucalc

# Start with a blank sheet
./hasucalc

# Demo data and chart settings
./hasucalc --demo
# or
./hasucalc -d

# Open a file
./hasucalc my_sheet.hwk
./hasucalc data.csv
```

---

## Features & keybindings

### 1. Keyboard & selection

| Key | Action | Notes |
|:---|:---|:---|
| **`Ctrl + Z`** | **Undo** | Cell edits, paste, and sheet add/delete/rename |
| **`Ctrl + Y`** | **Redo** | Redo the last undo |
| **`Shift + arrows`** | **Range select** | Blue highlight rectangle |
| **`Ctrl + C`** | **Copy** | Selection or current cell |
| **`Ctrl + V`** | **Paste** | Relative formula shift on copy; cut keeps refs |
| **`Ctrl + L`** | **Paste link** | Paste `=A1` / `=Sheet!A1` style refs |
| **`Ctrl + X`** | **Cut** | No relative shift on paste |
| **`Ctrl + K` (or `:`)** | **Command palette** | Fuzzy search like VS Code |
| **`Ctrl + S`** | **Save** | Directory browser + save dialog |
| **`Ctrl + O`** | **Open** | Directory browser + open dialog |
| **`Ctrl + F`** | **Find** | In-sheet (or workbook) search |
| **`Ctrl + H`** | **Replace** | Find and replace |
| **`F3` / `Shift+F3`** | **Find next/prev** | Repeat last search |
| **`Ctrl + G` (or `F5`)** | **Goto** | Jump to `B20`, `XFD1048576`, etc. |
| **`Ctrl + A`** | **Select all** | Select the used data region |
| **`Ctrl + T`** | **Sheet picker** | Sheet list modal |
| **`Ctrl + PgUp` / `Ctrl + PgDn`** | **Prev/next sheet** | Switch sheets |
| **`Alt + =`** | **AutoSum** | Insert `=SUM()` outside the selection |
| **`Delete` / `Backspace`** | **Clear** | Clear selection or current cell |
| **`Esc`** | **Cancel** | Clear selection / close menus |
| **`F1`** | **Help** | Keys and function list |
| **`F2` / `Ctrl + E`** | **Edit** | Edit in the formula bar (row 2) |
| **`F9`** | **Recalculate** | Recalculate all sheets |
| **`F10`** | **Chart** | Full-screen terminal chart; `S` saves PNG |
| **`Ctrl + Q`** | **Quit** | Confirms if there are unsaved changes |
| **`/`** | **Slash menu** | One-letter paths (e.g. `/FS` Save, `/FO` Open, `/HU` Undo, `/VF` Freeze) or arrows |

---

### 2. Command palette (`Ctrl + K` / `:`)

Incremental search in a centered window, for example:

- `sum` → `AutoSum (=SUM)`
- `average` → `Average (=AVERAGE)`
- `currency` → Currency format (any prefix string per cell; default `$`)
- `percent` → Percent format (`12.3%`)
- `graph` → Chart view (`F10`)
- `sort asc` → Sort ascending
- `csv export` → Export CSV

---

### 3. Formulas & ranges

- Familiar `=` formulas (accepted for convenience, not as “Excel compatibility”):
  - `=SUM(A1:B10)`, `=AVERAGE(A1:A10)`, `=A1+B1*2`, `=IF(A1>50, "OK", "NG")`
  - Identifiers `TRUE` / `FALSE` are booleans (1 / 0 in numeric context). `ISLOGICAL` is true only for booleans
- Classic / Lotus-style:
  - `+A1+B1`, `@SUM(A1..B10)`, `@VLOOKUP(A1, B1..D10, 2)`
- Ranges: both `:` and `..` (spaces like `A1 : B10` OK)
- Cross-sheet: `=Sheet2!A1`; workbook auto-recalc while editing
- Grid size: up to **1,048,576 rows × 16,384 columns (`A`–`XFD`)** (sparse; empty rows cost no memory)

---

## Documentation

Architecture, function coverage, compact `.hwk` format, PNG export, and development history: **[SPECIFICATION.md](SPECIFICATION.md)** (Japanese: [SPECIFICATION.ja.md](SPECIFICATION.ja.md)).
