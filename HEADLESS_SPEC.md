# HasuCalc 2.0 — Headless CLI & AI Agent Specification

> **Status:** Approved Draft · Phase 1 Implementation Ready  
> **Languages:** English (canonical) · [日本語](HEADLESS_SPEC.ja.md)  
> **Related documents:** [SPECIFICATION.md](SPECIFICATION.md) · [USER_MANUAL.md](USER_MANUAL.md) · [README.md](README.md)

This document defines the formal specification for the **Headless CLI & AI Agent Integration** subsystem of HasuCalc 2.0. It establishes deterministic stream contracts, exit code semantics, command syntax, and JSON schemas designed for autonomous AI agents, shell pipelines, and automated workflows.

---

## 1. Architectural Principles

### 1.1 Dual-Mode Dispatch & Backward Compatibility
HasuCalc operates in two distinct modes based on invocation arguments:
1. **Interactive TUI Mode**: When launched with no arguments, `--demo` (`-d`), or directly with a file path that is not a subcommand (`hasucalc my_sheet.hwk`), HasuCalc initializes the full terminal screen (`tcell`) and runs interactively. Existing user workflows remain 100% backward compatible.
2. **Headless CLI Mode**: When launched with a recognized headless subcommand (`convert`, `info`, `get`, `eval`, `chart`), HasuCalc executes strictly non-interactively without initializing `tcell`, outputs data to `stdout`, diagnostic logs to `stderr`, and exits immediately.

### 1.2 Workbook-First Data Model
HasuCalc treats multi-sheet workbooks as first-class citizens. When inspecting or extracting data, the active sheet or explicitly targeted sheet (`-s` / `--sheet`) is always scoped alongside workbook-level metadata (`sheets`, `usedRange`, `recalcMode`).

### 1.3 Token-Efficient Sparse Output
To optimize for Large Language Model (LLM) context windows and minimize bandwidth, JSON outputs are **sparse**:
- Empty cells are omitted from cell collections.
- Cells include coordinates (`ref`, 0-indexed `r` and `c`), semantic type (`NUMBER`, `LABEL`, `FORMULA`, `BOOLEAN`), raw input, formatted/computed value, and optional formatting tags.
- For high-level tabular inspection, dense text modes (`--format markdown` and `--format csv`) are available.

### 1.4 Strict Stream & Exit Code Contracts
AI agents depend on deterministic exit codes and clean output streams:
- `stdout`: Contains **only** requested output data (JSON, Markdown, CSV, or converted file stream).
- `stderr`: Contains **only** error diagnostics, syntax warnings, and human-readable logging.
- **Exit 0 (Success)**: Operation completed normally. Spreadsheet-level computational error states (such as `=1/0` yielding `ERR` or lookup misses yielding `NA`) are **valid spreadsheet results** and return Exit 0.
- **Exit 1 (Execution / Syntax Failure)**: Unrecoverable failures: file not found, permission denied, invalid arguments, formula syntax parse error, or non-existent target sheet.

---

## 2. Exit Code & Stream Specification

| Code | Meaning | `stdout` | `stderr` |
|:---|:---|:---|:---|
| **0** | **Success** | Requested data payload (JSON, Markdown, CSV, etc.) | Empty (or verbose diagnostic info if `--verbose` requested) |
| **1** | **General / User Error** | Empty (or structured error JSON if `--json` was requested) | Diagnostic error message describing cause |
| **2** | **CLI Syntax Error** | Command-line usage summary | Argument parsing error details |

> [!IMPORTANT]
> **Formula Error Values are Exit 0**: When evaluating `=1/0` via `hasucalc eval`, the exit code is `0` and the output is `{"ok": true, "result": {"val": "ERR", "type": "ERROR", "error": "ERR"}}`. A non-zero exit code indicates an engine/system failure, not a domain arithmetic error.

---

## 3. Subcommand Specifications (Phase 1)

### 3.1 `hasucalc convert`
Converts tabular data between supported formats, supporting both file-to-file and streaming Unix pipelines.

```
Usage:
  hasucalc convert <input> -o <output> [flags]
  hasucalc convert --from <fmt> --to <fmt> [flags] < <stdin> > <stdout>

Supported Formats:
  hwk, hwkz, xlsx, ods, csv, tsv, md (markdown), html

Flags:
  -o, --output <path>    Output destination path. If omitted or "-", writes to stdout.
  -s, --sheet <name>     Target sheet to export (for single-sheet formats like CSV/MD). Defaults to active sheet.
      --from <fmt>       Input format when reading from stdin (hwk, xlsx, ods, csv, md, html).
      --to <fmt>         Output format when writing to stdout (hwk, xlsx, ods, csv, md, html).
      --recalc           Force full workbook recalculation before exporting.
```

**Examples:**
```bash
# File-to-file conversion
hasucalc convert sales.xlsx -o sales.hwk
hasucalc convert data.hwk -o report.md -s "Summary"
hasucalc convert records.csv -o records.xlsx

# Pipeline stream
cat raw.csv | hasucalc convert --from csv --to hwk > clean.hwk
cat clean.hwk | hasucalc convert --from hwk --to md
```

---

### 3.2 `hasucalc info`
Extracts structural and sheet metadata from a workbook without dumping full cell arrays.

```
Usage:
  hasucalc info <input> [flags]

Flags:
  -s, --sheet <name>     Inspect specific sheet. Defaults to active sheet.
      --json             Output in JSON format (default is human-readable summary).
```

**Output Schema (`hasucalc info sales.xlsx --json`):**
```json
{
  "ok": true,
  "command": "info",
  "meta": {
    "file": "sales.xlsx",
    "format": "xlsx",
    "activeSheet": "Q1",
    "sheetCount": 3,
    "sheets": [
      {
        "name": "Q1",
        "index": 0,
        "usedRange": "A1:E26",
        "maxRow": 26,
        "maxCol": 5,
        "cellCount": 104,
        "frozenRows": 1,
        "frozenCols": 0,
        "hasGraph": true,
        "graphType": "LINE"
      },
      {
        "name": "Q2",
        "index": 1,
        "usedRange": "A1:E20",
        "maxRow": 20,
        "maxCol": 5,
        "cellCount": 80,
        "frozenRows": 1,
        "frozenCols": 0,
        "hasGraph": false,
        "graphType": ""
      }
    ],
    "recalcMode": "AUTO"
  }
}
```

---

### 3.3 `hasucalc get`
Extracts cell data from a workbook, supporting sparse JSON, Markdown tables, CSV, or raw values.

```
Usage:
  hasucalc get <input> [flags]

Flags:
  -s, --sheet <name>       Target sheet name (default: active sheet).
  -r, --range <range>       Target coordinate or range (e.g. "B2", "A1:E10", "A:C"). Defaults to used range.
  -f, --format <format>     Output format: "json" (default), "markdown" (or "md"), "csv", "values".
      --recalc              Recalculate workbook prior to extraction (default: true).
```

#### JSON Output Contract (`hasucalc get data.hwk -r "A1:C3" --format json`)
```json
{
  "ok": true,
  "command": "get",
  "meta": {
    "file": "data.hwk",
    "sheet": "Sheet1",
    "sheets": ["Sheet1", "Sheet2"],
    "usedRange": "A1:H26",
    "queryRange": "A1:C3",
    "recalcMode": "AUTO"
  },
  "cells": [
    { "ref": "A1", "r": 0, "c": 0, "type": "LABEL", "raw": "'JST", "val": "JST" },
    { "ref": "B1", "r": 0, "c": 1, "type": "LABEL", "raw": "'Japan", "val": "Japan" },
    { "ref": "C1", "r": 0, "c": 2, "type": "LABEL", "raw": "'London", "val": "London" },
    { "ref": "A2", "r": 1, "c": 0, "type": "NUMBER", "raw": "18", "val": 18 },
    { "ref": "B2", "r": 1, "c": 1, "type": "NUMBER", "raw": "6.4", "val": 6.4, "fmt": "(F1)" },
    { "ref": "C2", "r": 1, "c": 2, "type": "NUMBER", "raw": "10.7", "val": 10.7, "fmt": "(F1)" }
  ]
}
```

#### Markdown Output Contract (`hasucalc get data.hwk -r "A1:C3" --format markdown`)
```markdown
| JST | Japan | London |
| :--- | :--- | :--- |
| 18 | 6.4 | 10.7 |
| 19 | 6.4 | 12.0 |
```

---

### 3.4 `hasucalc eval`
Evaluates a formula immediately. Supports both standalone one-shot calculation and evaluation within the context of a loaded workbook.

```
Usage:
  hasucalc eval "<formula>" [flags]

Flags:
  -f, --file <path>        Context workbook file to load (optional).
  -s, --sheet <name>       Context sheet name (defaults to active sheet).
      --format <format>    Output format: "json" (default) or "raw" (raw value only).
```

#### Standalone Evaluation
```bash
hasucalc eval "=SUM(10, 20, 30) * 1.1"
```
**Output:**
```json
{
  "ok": true,
  "command": "eval",
  "formula": "=SUM(10, 20, 30) * 1.1",
  "result": {
    "val": 66,
    "type": "NUMBER",
    "error": null
  }
}
```

#### Contextual Evaluation
```bash
hasucalc eval -f data.hwk "=XLOOKUP(23, A2:A25, B2:B25)"
```
**Output:**
```json
{
  "ok": true,
  "command": "eval",
  "formula": "=XLOOKUP(23, A2:A25, B2:B25)",
  "meta": {
    "file": "data.hwk",
    "sheet": "Sheet1",
    "usedRange": "A1:H26"
  },
  "result": {
    "val": 5.4,
    "type": "NUMBER",
    "error": null
  }
}
```

#### Error Evaluation Handling (Exit 0)
```bash
hasucalc eval "=1/0"
```
**Output:**
```json
{
  "ok": true,
  "command": "eval",
  "formula": "=1/0",
  "result": {
    "val": "ERR",
    "type": "ERROR",
    "error": "ERR"
  }
}
```

---

### 3.5 `hasucalc chart`
Renders high-definition (1280×720 HD) PNG charts from sheet data without opening a terminal window.

```
Usage:
  hasucalc chart <input> -o <output.png> [flags]

Flags:
  -o, --output <path>      Output PNG file path (required).
  -s, --sheet <name>       Target sheet name (default: active sheet).
  -t, --type <type>        Chart type: LINE, BAR, STACKED, PIE (overrides saved chart config).
      --title <string>     Chart title string.
  -x, --range-x <range>    X-axis category range (e.g. "A2:A25").
      --series-a <range>   Series A data range (e.g. "B2:B25").
      --series-b <range>   Series B data range (e.g. "C2:C25").
      --series-c <range>   Series C data range (e.g. "D2:D25").
      --series-d <range>   Series D data range.
      --series-e <range>   Series E data range.
      --series-f <range>   Series F data range.
      --width <int>        Image width in pixels (default: 1280).
      --height <int>       Image height in pixels (default: 720).
```

**Examples:**
```bash
# Render chart using existing configuration saved in .hwk
hasucalc chart data.hwk -o traffic.png

# Render on-the-fly chart from an Excel file
hasucalc chart sales.xlsx -s "2026" --type BAR --title "Revenue by Region" -x "A2:A10" --series-a "B2:B10" -o revenue.png
```

---

## 4. Shared JSON Schema Specifications

### 4.1 Meta Object Schema
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "file": { "type": "string" },
    "format": { "type": "string" },
    "sheet": { "type": "string" },
    "sheets": { "type": "array", "items": { "type": "string" } },
    "usedRange": { "type": "string" },
    "queryRange": { "type": "string" },
    "recalcMode": { "type": "string", "enum": ["AUTO", "MANUAL"] }
  },
  "required": ["sheet", "usedRange"]
}
```

### 4.2 Cell Object Schema
```json
{
  "type": "object",
  "properties": {
    "ref": { "type": "string", "description": "A1 coordinate format" },
    "r": { "type": "integer", "description": "0-indexed row number" },
    "c": { "type": "integer", "description": "0-indexed column number" },
    "type": { "type": "string", "enum": ["NUMBER", "LABEL", "FORMULA", "BOOLEAN", "EMPTY", "ERROR"] },
    "raw": { "type": "string", "description": "Raw cell input or formula string" },
    "val": { "description": "Calculated value (float64, int, string, bool, or null)" },
    "fmt": { "type": "string", "description": "Display format descriptor e.g. (F1), (C2), $" }
  },
  "required": ["ref", "r", "c", "type", "raw"]
}
```

### 4.3 Error Response Schema (Exit 1)
```json
{
  "ok": false,
  "command": "get",
  "error": "sheet not found: 'Sheet9'",
  "code": "SHEET_NOT_FOUND"
}
```

---

## 5. Phase 2 & 3 Roadmap Alignment

| Phase | Deliverables | Interface |
|:---|:---|:---|
| **Phase 1** (Current) | `convert`, `info`, `get`, `eval`, `chart` + stdin/stdout pipes | CLI binary (`hasucalc <subcommand>`) |
| **Phase 2** | `set` (single/multi-cell update), `batch` (JSON transactional actions) | CLI binary (`hasucalc set`, `hasucalc batch`) |
| **Phase 3** | Built-in MCP Server (Model Context Protocol) wrapping Phase 1 & 2 1:1 | Stdio protocol (`hasucalc mcp`) |

By establishing this rigorous specification in Phase 1, HasuCalc guarantees that Phase 3 MCP tools (`read_sheet`, `evaluate_formula`, `convert_file`, `render_chart`) will map directly to the proven Go headless functions with zero structural divergence.
