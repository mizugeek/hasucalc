# HasuCalc

> **正本（English）:** [README.md](README.md)  
> **ユーザーマニュアル（詳細）:** [USER_MANUAL.md](USER_MANUAL.md)（[日本語](USER_MANUAL.ja.md)）  
> 技術仕様: [SPECIFICATION.md](SPECIFICATION.md)（[日本語](SPECIFICATION.ja.md)）  
> ヘッドレスCLI & AIエージェント仕様: [HEADLESS_SPEC.md](HEADLESS_SPEC.md)（[日本語](HEADLESS_SPEC.ja.md)）

DOS / PC-98 風の視認性に優れたTUIと、現代的な操作感（`Shift`+矢印での範囲選択、`Ctrl+C/V/S`、`Ctrl+K` コマンドパレット、`=SUM(A1:B10)` などの数式）を融合した高速・軽量なターミナル表計算ソフトウェアです。

標準フォーマットには Git や AI（LLM）と親和性の高い `.hwk`（コンパクトJSON）/ `.hwkz`（gzip）を採用。外部ファイル（`.xlsx`, `.ods`, `.csv`, `.md`, `.html`）とのインポート・エクスポートによるデータ連携にも対応しています。

外部依存（CGO）なしの単一バイナリとして動作します。

## クイックスタート

```bash
./hasucalc              # 空白シート
./hasucalc --demo       # デモ＋グラフ
./hasucalc my_sheet.hwk
./hasucalc notes.md     # 表→グリッド、本文→ラベル
```

画面構成、`[READY]` / `[CALC]` の意味、関数の引数、メニュー、ファイル形式の詳細は **[ユーザーマニュアル](USER_MANUAL.ja.md)** を参照してください。

## ヘッドレスCLI & MCPサーバー（AIエージェント・自動化連携）

HasuCalc は画面（TUI）を開かずにコマンドラインやパイプラインから直接操作できるヘッドレスCLI、および LLM エージェントと直接通信可能な **Model Context Protocol (MCP)** stdio サーバーを外部依存ゼロで内蔵しています。

### CLI 使用例

```bash
# ファイル形式の相互変換 (.xlsx, .ods, .csv, .tsv, .md, .html, .hwk, .hwkz)
hasucalc convert input.xlsx output.hwk

# セル・表データを Markdown またはスパースJSONで抽出
hasucalc get sales.hwk -r A1:E10 --format markdown
hasucalc get sales.hwk -r B2:D5 --json

# 数式の即時計算（単発電卓、またはワークブックの値を参照した計算）
hasucalc eval "=SUM(10, 20, 30) * 1.1"
hasucalc eval -f sales.hwk "=XLOOKUP(23, A2:A25, B2:B25)"

# セルの原子的更新と自動再計算
hasucalc set sales.hwk B2 150
hasucalc set sales.hwk D4 "=SUM(D2:D3)" --fmt "(C2)"

# トランザクション一括アクション実行（失敗時は自動で完全ロールバック）
cat actions.json | hasucalc batch sales.hwk

# ヘッドレス 1280x720 HD PNG グラフ画像の生成
hasucalc chart sales.hwk -o chart.png --type BAR --range-x A2:A10 --series-a B2:B10
```

### AIエージェント（MCPサーバー）設定

Claude Desktop、Cursor、その他の MCP クライアントの設定ファイル（`claude_desktop_config.json` 等）に以下を追加するだけで連携できます：

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

提供される 7 つの MCP ツール: `read_sheet`, `get_info`, `evaluate_formula`, `edit_cell`, `batch_edit`, `render_chart`, `convert_file`  
詳細なスキーマやオプション仕様は **[HEADLESS_SPEC.ja.md](HEADLESS_SPEC.ja.md)**（英語正本: [HEADLESS_SPEC.md](HEADLESS_SPEC.md)）を参照してください。

## キーバインド（要約）

| キー | 動作 |
|:---|:---|
| 矢印 / Shift+矢印 | 移動 / 選択 |
| F2 / Ctrl+E | セル編集 |
| Ctrl+C / X / V / L | コピー / 切り取り / 貼り付け / リンク貼付 |
| Ctrl+Z / Y | Undo / Redo |
| Ctrl+S / O | 保存 / 開く |
| Ctrl+K または `:` | コマンドパレット |
| Ctrl+F / H | 検索 / 置換 |
| F3 / Shift+F3 | 次／前を検索 |
| F5 / Ctrl+G | ジャンプ |
| F9 | 再計算 |
| F10 | グラフ（`S` = PNG） |
| Alt+= | AutoSum |
| `/` | スラッシュメニュー |
| F1 | アプリ内ヘルプ |
| Ctrl+Q | 終了 |

一覧とモード説明: [USER_MANUAL.ja.md §5](USER_MANUAL.ja.md#5-キーバインド)。

## 関数名一覧

`=NAME(...)` または `@NAME(...)`。引数の詳細は [関数リファレンス](USER_MANUAL.ja.md#9-関数リファレンス) を参照。

**Math/Agg:** `SUM`, `SUMIF`, `SUMIFS`, `SUMPRODUCT`, `PRODUCT`, `SUBTOTAL`, `ROUND`, `ROUNDUP`, `ROUNDDOWN`, `TRUNC`, `INT`, `ABS`, `MOD`, `QUOTIENT`, `SIGN`, `POWER`, `SQRT`, `EXP`, `LN`, `LOG`, `LOG10`, `CEILING`, `FLOOR`, `MROUND`, `FACT`, `GCD`, `LCM`, `COMBIN`, `PERMUT`, `PI`, `DEGREES`, `RADIANS`, `SIN`, `COS`, `TAN`, `ASIN`, `ACOS`, `ATAN`, `ATAN2`, `RAND`, `RANDBETWEEN`

**Statistical:** `AVG`, `AVERAGEIF`, `AVERAGEIFS`, `COUNT`, `COUNTA`, `COUNTBLANK`, `COUNTIF`, `COUNTIFS`, `MIN`, `MINIFS`, `MAX`, `MAXIFS`, `MEDIAN`, `MODE`, `LARGE`, `SMALL`, `PERCENTILE`, `QUARTILE`, `STDEV`, `STDEVP`, `VAR`, `VARP`, `RANK`

**Lookup/Ref:** `XLOOKUP`, `VLOOKUP`, `HLOOKUP`, `LOOKUP`, `INDEX`, `MATCH`, `XMATCH`, `OFFSET`, `CHOOSE`, `ROW`, `COLUMN`, `ROWS`, `COLUMNS`, `TRANSPOSE`

**Logic/Error:** `IF`, `IFS`, `SWITCH`, `AND`, `OR`, `NOT`, `XOR`, `IFERROR`, `IFNA`, `ISNUMBER`, `ISSTRING`, `ISTEXT`, `ISNONTEXT`, `ISBLANK`, `ISLOGICAL`, `ISERR`, `ISNA`, `ISEVEN`, `ISODD`, `TRUE`, `FALSE`, `N`, `T`, `TYPE`

**Text:** `TEXT`, `TRIM`, `CLEAN`, `SUBSTITUTE`, `REPLACE`, `REPT`, `UPPER`, `LOWER`, `PROPER`, `EXACT`, `CHAR`, `CODE`, `UNICHAR`, `UNICODE`, `CONCATENATE`, `CONCAT`, `TEXTJOIN`, `LEFT`, `RIGHT`, `MID`, `LEN`, `FIND`, `SEARCH`, `STRING`, `VALUE`, `NUMBERVALUE`, `TEXTBEFORE`, `TEXTAFTER`, `TEXTSPLIT`

**Date/Time:** `TODAY`, `NOW`, `DATE`, `DATEVALUE`, `TIME`, `TIMEVALUE`, `DATEDIF`, `DAYS`, `DAYS360`, `NETWORKDAYS`, `WORKDAY`, `YEARFRAC`, `YEAR`, `MONTH`, `DAY`, `HOUR`, `MINUTE`, `SECOND`, `WEEKDAY`, `WEEKNUM`, `EDATE`, `EOMONTH`

**Financial:** `PMT`, `PV`, `FV`, `NPV`, `IRR`, `RATE`, `NPER`, `SLN`, `SYD`, `DDB`

`AVG`→`AVERAGE` などの別名はマニュアル参照。
