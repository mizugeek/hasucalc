# HasuCalc 2.0 — ヘッドレスCLI & AIエージェント連携仕様書

> **正本（English）:** [HEADLESS_SPEC.md](HEADLESS_SPEC.md)  
> **ステータス:** 承認済みドラフト・Phase 1 実装準備完了  
> **関連ドキュメント:** [SPECIFICATION.ja.md](SPECIFICATION.ja.md) · [USER_MANUAL.ja.md](USER_MANUAL.ja.md) · [README.ja.md](README.ja.md)

本書は、HasuCalc 2.0 における **「ヘッドレスCLIおよびAIエージェント連携サブシステム」** の正式な仕様書です。自律AIエージェント（LLM）、シェルパイプライン、CI/CDなどの自動化ツールから確実・安全に呼び出せるよう、ストリーム規約、終了コード、コマンド構文、JSONスキーマを定義します。

---

## 1. アーキテクチャ原則

### 1.1 デュアルモード起動と後方互換性
HasuCalc は、コマンドライン引数に応じて2つのモードで起動します：
1. **対話型TUIモード**: 引数なし、`--demo` (`-d`)、またはサブコマンドではないファイルパス（例: `hasucalc my_sheet.hwk`）で起動した場合、従来通りフルスクリーンのTUI画面（`tcell`）を初期化して対話型スプレッドシートとして起動します。既存の操作性は100%維持されます。
2. **ヘッドレスCLIモード**: 認識されたサブコマンド（`convert`, `info`, `get`, `eval`, `chart`）で起動した場合、画面初期化を行わず完全に非対話型で実行され、結果を `stdout`、エラーや診断ログを `stderr` に出力して即座に終了します。

### 1.2 ワークブック第一級モデル（マルチシート前提）
HasuCalc は複数シートを持つワークブックをネイティブに扱います。データ取得や計算において、対象シート（`-s` / `--sheet`、省略時はアクティブシート）とともに、ワークブック全体のメタ情報（シート一覧、使用範囲、再計算モード等）を常にコンテキストとしてスコープに含めます。

### 1.3 トークン効率に優れたスパース出力
LLMのコンテキストウィンドウを節約するため、JSON出力は**スパース（値が存在するセルのみ）**を基本とします：
- 空セルは配列から省略されます。
- 各セルは座標（`ref`: "A1"、0始まりの `r` と `c`）、型（`NUMBER`, `LABEL`, `FORMULA`, `BOOLEAN`）、生入力文字列、計算済み値、書式識別子を持ちます。
- 表構造を一目で確認したい場合のために、密なテキスト形式（`--format markdown` および `--format csv`）もサポートします。

### 1.4 厳格なストリームおよび終了コード規約
AIエージェントの自動実行・自己修復ループを支援するため、決定論的な規約を定めます：
- `stdout`: 要求されたデータ（JSON、Markdown、CSV、バイナリ等）**のみ**を出力。
- `stderr`: エラーメッセージ、構文警告、診断ログ**のみ**を出力。
- **Exit 0（正常終了）**: 処理が完了。`=1/0` のゼロ除算（`ERR`）や検索失敗（`NA`）などのスプレッドシート計算エラーは**正常な計算結果**であり、Exit 0 を返します。
- **Exit 1（実行・構文エラー）**: ファイルが存在しない、権限エラー、引数不正、数式の構文エラー（括弧の不整合等）、対象シートが存在しない等の致命的エラー。

---

## 2. 終了コードとストリーム仕様

| コード | 意味 | `stdout` | `stderr` |
|:---|:---|:---|:---|
| **0** | **成功** | 要求されたデータ（JSON, Markdown, CSV 等） | 空（`--verbose` 指定時は診断ログ） |
| **1** | **一般・実行エラー** | 空（`--json` 指定時は構造化エラーJSON） | エラー内容を説明する診断メッセージ |
| **2** | **CLI構文エラー** | コマンド使用法サマリー | 引数解析エラー詳細 |

> [!IMPORTANT]
> **数式エラー値は Exit 0**: `hasucalc eval "=1/0"` を実行した場合、終了コードは `0` であり、`{"ok": true, "result": {"val": "ERR", "type": "ERROR", "error": "ERR"}}` が出力されます。非ゼロ終了コードはシステムや構文の致命的失敗を表します。

---

## 3. サブコマンド仕様（Phase 1）

### 3.1 `hasucalc convert`
対応する形式間で表データを相互変換します。ファイル変換およびパイプライン（stdin/stdout）に対応します。

```
使用法:
  hasucalc convert <input> -o <output> [flags]
  hasucalc convert --from <fmt> --to <fmt> [flags] < <stdin> > <stdout>

対応形式:
  hwk, hwkz, xlsx, ods, csv, tsv, md (markdown), html

フラグ:
  -o, --output <path>    出力先ファイルパス。省略時または "-" の場合は stdout に出力。
  -s, --sheet <name>     対象シート名（CSV/MDなどの単一シート形式出力時）。省略時はアクティブシート。
      --from <fmt>       標準入力（stdin）から読み込む際の入力形式（hwk, xlsx, ods, csv, md, html）。
      --to <fmt>         標準出力（stdout）へ書き出す際の出力形式（hwk, xlsx, ods, csv, md, html）。
      --recalc           出力前にワークブック全体の再計算を強制実行。
```

**使用例:**
```bash
# ファイル間変換
hasucalc convert sales.xlsx -o sales.hwk
hasucalc convert data.hwk -o report.md -s "Summary"
hasucalc convert records.csv -o records.xlsx

# パイプラインストリーム
cat raw.csv | hasucalc convert --from csv --to hwk > clean.hwk
cat clean.hwk | hasucalc convert --from hwk --to md
```

---

### 3.2 `hasucalc info`
全セルのダンプを行わずに、ワークブックの構造およびシートのメタデータを取得します。

```
使用法:
  hasucalc info <input> [flags]

フラグ:
  -s, --sheet <name>     特定シートのメタデータを取得。省略時はアクティブシート。
      --json             JSON形式で出力（デフォルトは人間が読みやすいテキストサマリー）。
```

**出力スキーマ (`hasucalc info sales.xlsx --json`):**
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
ワークブックからセルデータを抽出します（スパースJSON、Markdownテーブル、CSV、生値）。

```
使用法:
  hasucalc get <input> [flags]

フラグ:
  -s, --sheet <name>       対象シート名（省略時はアクティブシート）。
  -r, --range <range>       取得範囲（例: "B2", "A1:E10", "A:C"）。省略時は使用済み範囲全体（usedRange）。
  -f, --format <format>     出力形式: "json"（デフォルト）, "markdown" (または "md"), "csv", "values"。
      --recalc              データ抽出前にワークブックを再計算（デフォルト: true）。
```

#### JSON 出力契約 (`hasucalc get data.hwk -r "A1:C3" --format json`)
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

#### Markdown 出力契約 (`hasucalc get data.hwk -r "A1:C3" --format markdown`)
```markdown
| JST | Japan | London |
| :--- | :--- | :--- |
| 18 | 6.4 | 10.7 |
| 19 | 6.4 | 12.0 |
```

---

### 3.4 `hasucalc eval`
数式を即時に計算・評価します。単発の計算（電卓モード）およびファイルコンテキスト上での計算に対応します。

```
使用法:
  hasucalc eval "<formula>" [flags]

フラグ:
  -f, --file <path>        コンテキストとして読み込むワークブックファイルパス（任意）。
  -s, --sheet <name>       コンテキストシート名（省略時はアクティブシート）。
      --format <format>    出力形式: "json"（デフォルト）または "raw"（計算値のみ）。
```

#### 単発計算（電卓）
```bash
hasucalc eval "=SUM(10, 20, 30) * 1.1"
```
**出力:**
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

#### コンテキスト付き計算
```bash
hasucalc eval -f data.hwk "=XLOOKUP(23, A2:A25, B2:B25)"
```
**出力:**
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

#### 計算エラー時の出力例（Exit 0）
```bash
hasucalc eval "=1/0"
```
**出力:**
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
端末画面を開くことなく、シートデータから高解像度（1280×720 HD）のPNG画像を生成します。

```
使用法:
  hasucalc chart <input> -o <output.png> [flags]

フラグ:
  -o, --output <path>      出力PNGファイルパス（必須）。
  -s, --sheet <name>       対象シート名（省略時はアクティブシート）。
  -t, --type <type>        グラフ種類: LINE, BAR, STACKED, PIE（シート保存設定を上書き）。
      --title <string>     グラフタイトル文字列。
  -x, --range-x <range>    X軸カテゴリ範囲（例: "A2:A25"）。
      --series-a <range>   系列Aデータ範囲（例: "B2:B25"）。
      --series-b <range>   系列Bデータ範囲（例: "C2:C25"）。
      --series-c <range>   系列Cデータ範囲（例: "D2:D25"）。
      --series-d <range>   系列Dデータ範囲。
      --series-e <range>   系列Eデータ範囲。
      --series-f <range>   系列Fデータ範囲。
      --width <int>        画像幅（ピクセル、デフォルト: 1280）。
      --height <int>       画像高さ（ピクセル、デフォルト: 720）。
```

**使用例:**
```bash
# .hwk 内に保存済みのグラフ設定でそのまま出力
hasucalc chart data.hwk -o traffic.png

# Excelファイルからその場で棒グラフを生成
hasucalc chart sales.xlsx -s "2026" --type BAR --title "地域別売上" -x "A2:A10" --series-a "B2:B10" -o revenue.png
```

---

## 4. 共通 JSON スキーマ仕様

### 4.1 Meta オブジェクト
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

### 4.2 Cell オブジェクト
```json
{
  "type": "object",
  "properties": {
    "ref": { "type": "string", "description": "A1形式のセル座標" },
    "r": { "type": "integer", "description": "0始まりの行番号" },
    "c": { "type": "integer", "description": "0始まりの列番号" },
    "type": { "type": "string", "enum": ["NUMBER", "LABEL", "FORMULA", "BOOLEAN", "EMPTY", "ERROR"] },
    "raw": { "type": "string", "description": "セルの生入力文字列または数式文字列" },
    "val": { "description": "計算結果の値 (float64, int, string, bool, または null)" },
    "fmt": { "type": "string", "description": "表示書式識別子 例: (F1), (C2), $" }
  },
  "required": ["ref", "r", "c", "type", "raw"]
}
```

### 4.3 エラー応答スキーマ（Exit 1）
```json
{
  "ok": false,
  "command": "get",
  "error": "sheet not found: 'Sheet9'",
  "code": "SHEET_NOT_FOUND"
}
```

---

## 5. Phase 2 および Phase 3 への展開

| フェーズ | 提供機能 | インターフェース |
|:---|:---|:---|
| **Phase 1**（現在） | `convert`, `info`, `get`, `eval`, `chart` ＋ stdin/stdout パイプ | CLI コマンド (`hasucalc <subcommand>`) |
| **Phase 2** | `set` (単一/複数セルの更新), `batch` (JSON一括アクション実行) | CLI コマンド (`hasucalc set`, `hasucalc batch`) |
| **Phase 3** | 内蔵 MCP サーバー (Model Context Protocol) 1:1 ラッパー | Stdio プロトコル (`hasucalc mcp`) |

Phase 1 でこの厳格な仕様と出力契約を確立することで、Phase 3 の MCP ツール（`read_sheet`, `evaluate_formula`, `convert_file`, `render_chart`）は、CLI と全く同一の Go ヘッドレスコア関数を呼ぶだけの薄いアダプタとして安全・迅速に実装されます。
