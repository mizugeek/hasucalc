# HasuCalc ユーザーマニュアル

> **正本（English）:** [USER_MANUAL.md](USER_MANUAL.md)  
> 概要: [README.md](README.md)（[日本語](README.ja.md)） · 技術仕様: [SPECIFICATION.md](SPECIFICATION.md)（[日本語](SPECIFICATION.ja.md)）  
> 差分がある場合は英語版ユーザーマニュアルを優先してください。

画面の見方、`[READY]` などのモード表示、キー操作、ファイル形式、および組み込み関数の引数を説明します。HasuCalc は **Excel 互換アプリではありません**。正本は `.hwk` / `.hwkz` です。`=` 数式や `.xlsx` / `.ods` / `.md` / `.html` は利便のための橋渡しです。

---

## 目次

1. [画面構成](#1-画面構成)
2. [モード表示（`[READY]` など）](#2-モード表示ready-など)
3. [ステータスバー](#3-ステータスバー)
4. [入力と数式](#4-入力と数式)
5. [キーバインド](#5-キーバインド)
6. [スラッシュメニューとコマンドパレット](#6-スラッシュメニューとコマンドパレット)
7. [ファイル形式](#7-ファイル形式)
8. [グラフ](#8-グラフ)
9. [関数リファレンス](#9-関数リファレンス)
10. [エラーとよくあるメッセージ](#10-エラーとよくあるメッセージ)

---

## 1. 画面構成

上から下へ、おおよそ次の行構成です。

| 行 | 内容 |
|:---|:---|
| **0 行目（ヘッダ）** | カレントセル番地（例: `B12:` や `Sheet2!B12:`）、書式 `[…]` や列幅 `[W…]`、セルの生入力。右端に `[READY]` などのモード表示。 |
| **1 行目（編集／メニュー）** | READY 時はセル内容のプレビュー。INPUT/EDIT/POINT 時は入力中の文字列。MENU 時はスラッシュメニューバー。PROMPT 時はプロンプト＋入力。 |
| **2 行目** | 列見出し（`A`, `B`, …）。枠固定時はスクロールしても残る。 |
| **グリッド** | 左に行番号、中央にセル。青ハイライトは選択範囲。 |
| **シートタブ**（複数シート時） | クリック、または `Ctrl+T` / `Ctrl+PgUp` / `Ctrl+PgDn`。 |
| **最下行ステータス** | ファイル名（とシート番号）、時刻、一時メッセージ、右側のヒント / `[CALC]`。 |

アプリ内ヘルプは **F1** です。

---

## 2. モード表示（`[READY]` など）

右上の枠は、いまの操作モードを示します。

| 表示 | 意味 |
|:---|:---|
| **`[READY]`** | 待機中。矢印で移動、文字入力で INPUT 開始、`/` でメニュー、各種ショートカットが使える。 |
| **`[INPUT]`** | カレントセルへ**新規入力**中（キー入力で開始）。Enter で確定、Esc で取消。 |
| **`[EDIT]`** | 既存セルを数式バーで編集中（`F2` / `Ctrl+E`）。 |
| **`[POINT]`** | 数式入力中に、カーソルやマウスでセル／範囲を指して参照を挿入している状態。 |
| **`[MENU]`** | スラッシュメニュー（`/`）が開いている。 |
| **`[PROMPT]`** | 検索・ジャンプ・書式記号など、対話入力待ち。 |
| **`[END]`** | Lotus 風 End モード。End のあと矢印で、データ塊の端へジャンプ。 |

---

## 3. ステータスバー

| 要素 | 意味 |
|:---|:---|
| ファイル名 | 保存名・候補名（多くは `DATA.hwk`）。複数シート時は `file.hwk [Sheet] (i/n)`。 |
| 時刻 | ローカル日時。 |
| 中央メッセージ | 保存完了・エラー・検索結果などの一時表示。 |
| **`[CALC]`** | 再計算が必要（MANUAL モードや未反映の変更など）。**F9** でブック再計算。不要なときは右側にショートカット案内のみ。 |

---

## 4. 入力と数式

### 値の種類

| 形 | 結果 |
|:---|:---|
| 数値 | `123`, `45.67`, `1e5`, `-0.05`, `50%`（リテラル百分率 → 0.5） |
| `'文字列` | ラベル（左寄せ） |
| `"文字列` | ラベル（右寄せ） |
| `^文字列` | ラベル（中央） |
| `\=` や `\-` | セル幅いっぱいの繰り返し塗り |
| 数式 | `=` / `@` / `+` で開始 |

### 数式の要点

- `=SUM(A1:B10)` と `@SUM(A1..B10)` の両方可（`=` は利便であり Excel 互換を意味しない）。
- 範囲は `A1:B10` と `A1..B10`。全列 `A:A` は used range までに制限。
- 他シート: `Sheet2!A1` や `'Q1-2024'!A1`。
- 絶対／複合参照: `$A$1`, `$A1`, `A$1`。
- 真偽: `TRUE` / `FALSE`（および `TRUE()` / `FALSE()`）。
- Lotus 論理: `#AND#`, `#OR#`, `#NOT#`。
- 単項マイナスは `^` より強い: `-2^2` → `4`。

関数一覧ブラウザは **`/IF`**（Insert → Function）またはパレットから。

---

## 5. キーバインド

| キー | 動作 |
|:---|:---|
| 矢印 | カーソル移動 |
| Shift+矢印 | 選択拡張 |
| PageUp / PageDn | 約20行スクロール |
| Home | A1 へ |
| End → 矢印 | データ塊の端へ（`[END]`） |
| Enter | 入力／メニュー確定 |
| Esc | 取消・選択解除・UI を閉じる |
| F1 | ヘルプ |
| F2 / Ctrl+E | セル編集（EDIT） |
| F3 / Shift+F3 | 次／前を検索 |
| F5 / Ctrl+G | ジャンプ（セル・範囲・シート・名前） |
| F9 | ブック再計算 |
| F10 | グラフ（`S` で PNG 保存） |
| Ctrl+C / X / V | コピー / 切り取り / 貼り付け |
| Ctrl+L | リンク貼り付け |
| Ctrl+Z / Y | Undo / Redo |
| Ctrl+F / H | 検索 / 置換 |
| Ctrl+S / O | 保存 / 開く |
| Ctrl+K または `:` | コマンドパレット |
| Ctrl+A | 使用領域を全選択 |
| Ctrl+T | シート選択 |
| Ctrl+PgUp / PgDn | 前後のシート |
| Ctrl+D / R | 下方向／右方向へフィル |
| Alt+= | AutoSum |
| Delete / Backspace | 消去 |
| Ctrl+Q | 終了（未保存時は確認） |
| `/` | スラッシュメニュー |
| マウスクリック／ドラッグ／ホイール | 移動・選択・スクロール・タブ切替 |

---

## 6. スラッシュメニューとコマンドパレット

### スラッシュメニュー（`/`）

`/` のあと文字キー（または矢印）。主な経路:

| 経路 | 用途 |
|:---|:---|
| `/FN` `/FO` `/FS` `/FQ` | 新規 / 開く / 保存 / 終了 |
| `/FX` | 書き出し → CSV / Excel / ODS / Markdown → Sheet または Range |
| `/HU` `/HR` `/HX` `/HC` `/HV` | Undo / Redo / Cut / Copy / Paste |
| `/HS` | 形式を選択して貼り付け |
| `/HF` `/HE` `/HG` | 検索 / 置換 / ジャンプ |
| `/HM` | 数値書式（通貨・% など） |
| `/IF` | 関数ブラウザ |
| `/OS` / `/O9` | AutoSum / 再計算 |
| `/DS` `/DA` `/DF` `/DT` | ソート / AutoFill / Fill / 転置 |
| `/VF` | 枠の固定 |
| `/CV` `/CT` `/CP` | グラフ表示 / 種類 / PNG 保存 |
| `/?K` | キーバインドヘルプ |

### コマンドパレット（`Ctrl+K` / `:`）

`sum`・`currency`・`graph`・`csv export` などをファジー検索。

---

## 7. ファイル形式

| 形式 | 役割 |
|:---|:---|
| **`.hwk`** | 標準のネイティブブック（コンパクト JSON）。Git / LLM 向け。 |
| **`.hwkz` / `.hwk.gz`** | 同上の gzip 圧縮。 |
| **`.xlsx` / `.xlsm` / `.ods`** | 表データの入出力ブリッジ（グラフなし）。 |
| **`.csv` / `.tsv`** | 区切りテキスト。CSV 書き出しは UTF-8 BOM 付き。 |
| **`.md` / `.html`** | **読み込み**: 表→グリッド、本文→A列ラベル。Markdown は表として**書き出し**も可。 |

CLI（`hasucalc file.md`）または **Ctrl+O**（`.md` / `.html` も一覧に出る）。

---

## 8. グラフ

- シートあたり1つの設定（系列 A〜F）: Line / Bar / Stacked / Pie。
- **F10** で端末プレビュー、**S** で 1280×720 PNG。
- 設定の永続化は `.hwk` / `.hwkz` のみ（Excel/ODS には書かない）。

---

## 9. 関数リファレンス

`@NAME(...)` と、`=` のあとの `NAME(...)` の両方を受け付けます。省略可能な引数は `[角括弧]` です。構文・説明の詳細は英語正本と同じカタログです。

### 数学・集計 (Math/Agg)

| 関数 | 構文 | 説明 |
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

### 統計 (Statistical)

| 関数 | 構文 | 説明 |
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

### 検索・参照 (Lookup/Ref)

| 関数 | 構文 | 説明 |
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

### 論理・エラー (Logic/Error)

| 関数 | 構文 | 説明 |
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

### 文字列 (Text)

| 関数 | 構文 | 説明 |
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

### 日付・時刻 (Date/Time)

| 関数 | 構文 | 説明 |
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

### 財務 (Financial)

| 関数 | 構文 | 説明 |
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

### 別名（受理するだけ）

| 別名 | 正本 |
|:---|:---|
| `AVG` | `AVERAGE` |
| `LENGTH` | `LEN` |
| `REPEAT` | `REPT` |
| `PAYMT` | `PMT` |
| `MULTIPLY` | `PRODUCT` |
| `STD` | `STDEV.P` / `STDEVP` |
| `STRING` | Lotus 系の数値→文字列 |
| `CONCATENATE` | `CONCAT`（一覧にもあり） |

---

## 10. エラーとよくあるメッセージ

| 表示 | 意味 |
|:---|:---|
| `ERR` | 一般的な数式／値エラー（Excel の `#VALUE!` 等には細分しない）。 |
| `NA` | 未発見・欠落（lookup 失敗、`NA()` など）。 |
| `CIRCULAR REF` | 循環参照。 |
| `#REF!` | 壊れた参照（削除されたシートなど）。 |
| “Clipboard is empty!” | コピーなしで貼り付け。 |
| “Imported markup from …” | `.md` / `.html` の取り込み成功。 |

不具合と仕様差の契約は [SPECIFICATION.md](SPECIFICATION.md) §1.4（[日本語](SPECIFICATION.ja.md)）を参照。
