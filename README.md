# HasuCalc

> **Languages:** English (canonical) · [日本語](README.ja.md)  
> **User manual (detailed):** [USER_MANUAL.md](USER_MANUAL.md) ([日本語](USER_MANUAL.ja.md))  
> Engineering specification: [SPECIFICATION.md](SPECIFICATION.md) ([日本語](SPECIFICATION.ja.md))  
> Headless CLI & AI agents: [HEADLESS_SPEC.md](HEADLESS_SPEC.md) ([日本語](HEADLESS_SPEC.ja.md))

Fast and lightweight terminal spreadsheet combining the clean clarity of a classic DOS interface with modern editing (`Shift`+arrows, `Ctrl+C/V/S`, `Ctrl+K` palette, `=SUM(A1:B10)` formulas).

Uses compact `.hwk` (JSON) and `.hwkz` (gzip) as native, Git- and LLM-friendly file formats, with import/export support for tabular formats (`.xlsx`, `.ods`, `.csv`, `.md`, `.html`).

Ships as a single, self-contained, CGO-free Go binary: `hasucalc`.

## Quick start

```bash
./hasucalc              # blank sheet
./hasucalc --demo       # demo data + chart
./hasucalc my_sheet.hwk
./hasucalc notes.md     # tables → grid; other text → labels
```

For screen layout, `[READY]` / `[CALC]` meanings, parameters of every function, menus, and file formats, see the **[User Manual](USER_MANUAL.md)**.

## Install (Releases)

Pre-built archives are on [GitHub Releases](https://github.com/mizugeek/hasucalc/releases/latest).

1. Download the archive for your OS and CPU (Linux: `.tar.gz`; macOS and Windows: `.zip`).
2. Extract it. On macOS you can double-click the `.zip` in Finder. Put `hasucalc` (or `hasucalc.exe` on Windows) on your `PATH`, or run it by full path. Each archive includes `LICENSE`.
3. Confirm with `hasucalc --version`.
4. (Optional, for development) `CGO_ENABLED=0 go build -o hasucalc .`

### macOS notes (Gatekeeper)

macOS Release binaries are **not Apple-signed or notarized**. The first launch may show a dialog such as *“hasucalc” cannot be opened* (the binary is not corrupted). If you trust the Release, allow it with one of:

1. **Finder:** **Control-click** `hasucalc` → **Open**, then **Open** again if prompted.
2. **Terminal** (clear the download quarantine attribute):
   ```bash
   xattr -d com.apple.quarantine ./hasucalc
   ./hasucalc --version
   ```
3. **System Settings → Privacy & Security:** if a block notice appears, choose **Open Anyway**.

Unsigned Windows binaries may trigger SmartScreen; allow the binary if you trust the Release.

## Headless CLI & MCP Server (AI Agents & Automation)

HasuCalc includes a native, zero-dependency headless CLI and a built-in **Model Context Protocol (MCP)** server over `stdio` for LLM agents, CI/CD scripts, and terminal pipelines.

### CLI Examples

```bash
# Convert between formats (.xlsx, .ods, .csv, .tsv, .md, .html, .hwk, .hwkz)
hasucalc convert input.xlsx output.hwk

# Extract table data as Markdown or sparse JSON
hasucalc get sales.hwk -r A1:E10 --format markdown
hasucalc get sales.hwk -r B2:D5 --json

# Instant formula evaluation (standalone or with workbook context)
hasucalc eval "=SUM(10, 20, 30) * 1.1"
hasucalc eval -f sales.hwk "=XLOOKUP(23, A2:A25, B2:B25)"

# Atomic cell update with automatic recalculation
hasucalc set sales.hwk B2 150
hasucalc set sales.hwk D4 "=SUM(D2:D3)" --fmt "(C2)"

# Transactional batch execution (with complete rollback on failure)
cat actions.json | hasucalc batch sales.hwk

# Headless 1280x720 HD PNG chart generation
hasucalc chart sales.hwk -o chart.png --type BAR --range-x A2:A10 --series-a B2:B10
```

### AI Agent / MCP Server Setup

Connect HasuCalc to Claude Desktop, Cursor, or any MCP client by adding to `claude_desktop_config.json`:

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

Exposes 7 MCP tools: `read_sheet`, `get_info`, `evaluate_formula`, `edit_cell`, `batch_edit`, `render_chart`, and `convert_file`.  
For complete schemas and CLI options, see **[HEADLESS_SPEC.md](HEADLESS_SPEC.md)**.

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

## License

MIT License. See [LICENSE](LICENSE).
