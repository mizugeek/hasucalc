# HasuCalc

> **正本（English）:** [README.md](README.md)  
> **ユーザーマニュアル（詳細）:** [USER_MANUAL.md](USER_MANUAL.md)（[日本語](USER_MANUAL.ja.md)）  
> 技術仕様: [SPECIFICATION.md](SPECIFICATION.md)（[日本語](SPECIFICATION.ja.md)）

DOS / PC-98 風の見た目に、現代的な編集操作（`Shift`+矢印、`Ctrl+C/V/S`、`Ctrl+K`、`=SUM(A1:B10)`）を足したターミナル表計算です。

**Excel 互換アプリではありません。** 正本は `.hwk` / `.hwkz`。`.xlsx` / `.ods` / `.csv` / `.md` / `.html` はデータ受け渡し用の橋渡しです。

CGO 不要の単一バイナリ: `hasucalc`。

## クイックスタート

```bash
./hasucalc              # 空白シート
./hasucalc --demo       # デモ＋グラフ
./hasucalc my_sheet.hwk
./hasucalc notes.md     # 表→グリッド、本文→ラベル
```

画面構成、`[READY]` / `[CALC]` の意味、関数の引数、メニュー、ファイル形式の詳細は **[ユーザーマニュアル](USER_MANUAL.ja.md)** を参照してください。

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
