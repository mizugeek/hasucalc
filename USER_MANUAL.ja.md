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

関数一覧ブラウザは **`/IF`**（Insert → Function）またはパレットから。関数の説明は本節を日本語で記載しています。

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

`@NAME(...)` と、`=` のあとの `NAME(...)` の両方を受け付けます。省略可能な引数は `[角括弧]` です。

### 数学・集計 (Math/Agg)

| 関数 | 構文 | 説明 |
|:---|:---|:---|
| `SUM` | `SUM(range/list)` | 範囲内の数値の合計を求める |
| `SUMIF` | `SUMIF(range, criteria, [sum_range])` | 条件に合うセルの合計（例: ">50", "Apple"） |
| `SUMIFS` | `SUMIFS(sum_rng, crit_rng1, crit1, ...)` | 複数条件を満たすセルの合計 |
| `SUMPRODUCT` | `SUMPRODUCT(array1, [array2]...)` | 対応する要素の積の合計（積和） |
| `PRODUCT` | `PRODUCT(number1, [number2]...)` | 引数の数値をすべて掛け合わせる |
| `SUBTOTAL` | `SUBTOTAL(function_num, ref1, ...)` | リストの小計（9=SUM, 1=AVG など） |
| `ROUND` | `ROUND(val, num_digits)` | 指定桁に四捨五入 |
| `ROUNDUP` | `ROUNDUP(val, num_digits)` | ゼロから遠ざかる方向に切り上げ |
| `ROUNDDOWN` | `ROUNDDOWN(val, num_digits)` | ゼロに近づく方向に切り捨て |
| `TRUNC` | `TRUNC(val, [num_digits])` | 指定桁で切り捨て（端数削除） |
| `INT` | `INT(val)` | 小数点以下を切り捨てて整数にする |
| `ABS` | `ABS(val)` | 絶対値を返す |
| `MOD` | `MOD(number, divisor)` | 除算の余り（剰余） |
| `QUOTIENT` | `QUOTIENT(numerator, denominator)` | 割り算の整数部分 |
| `SIGN` | `SIGN(number)` | 符号（正=1, 負=-1, 0=0） |
| `POWER` | `POWER(number, power)` | 累乗（x^y） |
| `SQRT` | `SQRT(val)` | 正の数の平方根 |
| `EXP` | `EXP(number)` | e の累乗 |
| `LN` | `LN(number)` | 自然対数 |
| `LOG` | `LOG(number, [base])` | 指定底の対数（省略時は10） |
| `LOG10` | `LOG10(number)` | 常用対数（底10） |
| `CEILING` | `CEILING(number, significance)` | 基準値の倍数へ切り上げ |
| `FLOOR` | `FLOOR(number, significance)` | 基準値の倍数へ切り下げ |
| `MROUND` | `MROUND(number, multiple)` | 指定倍数の最も近い値へ丸める |
| `FACT` | `FACT(number)` | 階乗（n!） |
| `GCD` | `GCD(number1, number2, ...)` | 最大公約数 |
| `LCM` | `LCM(number1, number2, ...)` | 最小公倍数 |
| `COMBIN` | `COMBIN(n, k)` | 組み合わせの数（nCr） |
| `PERMUT` | `PERMUT(n, k)` | 順列の数（nPr） |
| `PI` | `PI()` | 円周率 π |
| `DEGREES` | `DEGREES(angle_in_radians)` | ラジアンを度に変換 |
| `RADIANS` | `RADIANS(angle_in_degrees)` | 度をラジアンに変換 |
| `SIN` | `SIN(number)` | 正弦（ラジアン） |
| `COS` | `COS(number)` | 余弦（ラジアン） |
| `TAN` | `TAN(number)` | 正接（ラジアン） |
| `ASIN` | `ASIN(number)` | 逆正弦（ラジアン） |
| `ACOS` | `ACOS(number)` | 逆余弦（ラジアン） |
| `ATAN` | `ATAN(number)` | 逆正接（ラジアン） |
| `ATAN2` | `ATAN2(x_num, y_num)` | 座標から逆正接 |
| `RAND` | `RAND()` | 0以上1未満の乱数 |
| `RANDBETWEEN` | `RANDBETWEEN(min, max)` | min〜max の整数乱数（両端含む） |

### 統計 (Statistical)

| 関数 | 構文 | 説明 |
|:---|:---|:---|
| `AVG` | `AVG(range/list)` | 算術平均 |
| `AVERAGEIF` | `AVERAGEIF(range, criteria, [avg_range])` | 条件に合うセルの平均 |
| `AVERAGEIFS` | `AVERAGEIFS(avg_rng, crit_rng1, crit1, ...)` | 複数条件を満たすセルの平均 |
| `COUNT` | `COUNT(range/list)` | 数値が入ったセルの個数 |
| `COUNTA` | `COUNTA(range/list)` | 空白でないセルの個数 |
| `COUNTBLANK` | `COUNTBLANK(range)` | 空白セルの個数 |
| `COUNTIF` | `COUNTIF(range, criteria)` | 条件に合うセルの個数 |
| `COUNTIFS` | `COUNTIFS(crit_rng1, crit1, ...)` | 複数条件を満たすセルの個数 |
| `MIN` | `MIN(range/list)` | 最小値 |
| `MINIFS` | `MINIFS(min_rng, crit_rng1, crit1, ...)` | 複数条件を満たす中の最小値 |
| `MAX` | `MAX(range/list)` | 最大値 |
| `MAXIFS` | `MAXIFS(max_rng, crit_rng1, crit1, ...)` | 複数条件を満たす中の最大値 |
| `MEDIAN` | `MEDIAN(range/list)` | 中央値（メディアン） |
| `MODE` | `MODE(range/list)` | 最頻値（モード） |
| `LARGE` | `LARGE(array, k)` | k 番目に大きい値 |
| `SMALL` | `SMALL(array, k)` | k 番目に小さい値 |
| `PERCENTILE` | `PERCENTILE(array, k)` | 百分位数（k は 0〜1） |
| `QUARTILE` | `QUARTILE(array, quart)` | 四分位数（0〜4） |
| `STDEV` | `STDEV(range/list)` | 標本標準偏差（n-1） |
| `STDEVP` | `STDEVP(range/list)` | 母標準偏差（n） |
| `VAR` | `VAR(range/list)` | 標本分散（n-1） |
| `VARP` | `VARP(range/list)` | 母分散（n） |
| `RANK` | `RANK(num, range, [order])` | 順位（0=降順, 1=昇順） |

### 検索・参照 (Lookup/Ref)

| 関数 | 構文 | 説明 |
|:---|:---|:---|
| `XLOOKUP` | `XLOOKUP(key, lk_rng, ret_rng, [fallback], [match], [search])` | 縦横対応の検索（正確／近似）と見つからないときの値 |
| `VLOOKUP` | `VLOOKUP(key, table_range, col_offset, [exact])` | 左端列を検索し、指定列の値を返す |
| `HLOOKUP` | `HLOOKUP(key, table_range, row_offset, [exact])` | 最上行を検索し、指定行の値を返す |
| `LOOKUP` | `LOOKUP(val, lookup_vector, [result_vector])` | 1行または1列の範囲で検索 |
| `INDEX` | `INDEX(range, col_offset, row_offset)` | 交差位置のセル値（オフセットは0始まり） |
| `MATCH` | `MATCH(key, lookup_array, [match_type])` | 一致位置（1始まり） |
| `XMATCH` | `XMATCH(key, lookup_array, [match_mode], [search_mode])` | 位置検索（完全一致・ワイルドカード・逆方向） |
| `OFFSET` | `OFFSET(ref, rows, cols, [height], [width])` | 起点からずらした参照を返す |
| `CHOOSE` | `CHOOSE(index, val0, val1, val2...)` | 0始まりの番号でリストから値を選ぶ |
| `ROW` | `ROW([cell])` | 行番号（1始まり） |
| `COLUMN` | `COLUMN([cell])` | 列番号（1始まり） |
| `ROWS` | `ROWS(range)` | 範囲の行数 |
| `COLUMNS` | `COLUMNS(range)` | 範囲の列数 |
| `TRANSPOSE` | `TRANSPOSE(array)` | 行列を入れ替える |

### 論理・エラー (Logic/Error)

| 関数 | 構文 | 説明 |
|:---|:---|:---|
| `IF` | `IF(condition, true_val, false_val)` | 条件分岐（真／偽の値） |
| `IFS` | `IFS(cond1, val1, [cond2, val2]...)` | 条件を順に評価する多岐分岐 |
| `SWITCH` | `SWITCH(expr, val1, res1, [val2, res2]..., [default])` | 式を値リストと照合して分岐 |
| `AND` | `AND(logical1, [logical2], ...)` | すべて真なら真 |
| `OR` | `OR(logical1, [logical2], ...)` | どれか真なら真 |
| `NOT` | `NOT(logical)` | 論理値を反転 |
| `XOR` | `XOR(logical1, [logical2]...)` | 排他的論理和（XOR） |
| `IFERROR` | `IFERROR(formula, fallback_val)` | エラー時に代替値を返す |
| `IFNA` | `IFNA(formula, fallback_val)` | NA のとき代替値を返す |
| `ISNUMBER` | `ISNUMBER(val)` | 数値かどうか（1/0） |
| `ISSTRING` | `ISSTRING(val)` | 文字列かどうか（1/0） |
| `ISTEXT` | `ISTEXT(val)` | 文字列かどうか（1/0） |
| `ISNONTEXT` | `ISNONTEXT(val)` | 文字列でないか（1/0） |
| `ISBLANK` | `ISBLANK(val)` | 空白セルかどうか |
| `ISLOGICAL` | `ISLOGICAL(val)` | 真偽値かどうか |
| `ISERR` | `ISERR(val)` | ERR かどうか（NA は含まない、1/0） |
| `ISNA` | `ISNA(val)` | NA かどうか（1/0） |
| `ISEVEN` | `ISEVEN(number)` | 偶数かどうか（1/0） |
| `ISODD` | `ISODD(number)` | 奇数かどうか（1/0） |
| `TRUE` | `TRUE()` | 真を返す |
| `FALSE` | `FALSE()` | 偽を返す |
| `N` | `N(value)` | 数値に変換 |
| `T` | `T(value)` | 文字列ならそのまま、それ以外は空文字 |
| `TYPE` | `TYPE(value)` | 型コード（1=数値, 2=文字 など） |

### 文字列 (Text)

| 関数 | 構文 | 説明 |
|:---|:---|:---|
| `TEXT` | `TEXT(value, format_string)` | 書式文字列で数値・日付を文字列化（例: "yyyy/mm/dd"） |
| `TRIM` | `TRIM(text)` | 前後の空白除去と内部空白の整理 |
| `CLEAN` | `CLEAN(text)` | 印刷不能文字を除去 |
| `SUBSTITUTE` | `SUBSTITUTE(text, old_text, new_text, [instance])` | 部分文字列を置換 |
| `REPLACE` | `REPLACE(old_text, start_pos, num_chars, new_text)` | 位置を指定して文字を置換 |
| `REPT` | `REPT(text, number_times)` | 文字列を指定回数繰り返す |
| `UPPER` | `UPPER(text)` | 英字を大文字に |
| `LOWER` | `LOWER(text)` | 英字を小文字に |
| `PROPER` | `PROPER(text)` | 単語先頭を大文字に（Title Case） |
| `EXACT` | `EXACT(text1, text2)` | 完全一致比較（大文字小文字を区別） |
| `CHAR` | `CHAR(number)` | コード番号に対応する文字 |
| `CODE` | `CODE(text)` | 先頭文字のコード番号 |
| `UNICHAR` | `UNICHAR(number)` | Unicode コードポイントの文字 |
| `UNICODE` | `UNICODE(text)` | 先頭文字の Unicode コードポイント |
| `CONCATENATE` | `CONCATENATE(text1, text2, ...)` | 複数文字列を連結 |
| `CONCAT` | `CONCAT(text1, text2, ...)` | 文字列や範囲を連結 |
| `TEXTJOIN` | `TEXTJOIN(delimiter, ignore_empty, text1, ...)` | 区切り文字付きで連結（空無視オプションあり） |
| `LEFT` | `LEFT(text, num_chars)` | 左から指定文字数を取り出す |
| `RIGHT` | `RIGHT(text, num_chars)` | 右から指定文字数を取り出す |
| `MID` | `MID(text, start_pos, num_chars)` | 途中から指定文字数を取り出す |
| `LEN` | `LEN(text)` | 文字数 |
| `FIND` | `FIND(find_text, within_text, [start])` | 大文字小文字を区別して位置検索（1始まり） |
| `SEARCH` | `SEARCH(find_text, within_text, [start])` | 大文字小文字無視・ワイルドカード対応の位置検索 |
| `STRING` | `STRING(number, decimal_places)` | 小数桁固定で数値を文字列化（Lotus 系） |
| `VALUE` | `VALUE(text)` | 通貨記号・カンマ付き文字列を数値に |
| `NUMBERVALUE` | `NUMBERVALUE(text, [dec_sep], [group_sep])` | 小数点・桁区切りを指定して数値化 |
| `TEXTBEFORE` | `TEXTBEFORE(text, delimiter)` | 区切りより前の文字列 |
| `TEXTAFTER` | `TEXTAFTER(text, delimiter)` | 区切りより後の文字列 |
| `TEXTSPLIT` | `TEXTSPLIT(text, col_delimiter)` | 区切りで分割 |

### 日付・時刻 (Date/Time)

| 関数 | 構文 | 説明 |
|:---|:---|:---|
| `TODAY` | `TODAY()` | 今日の日付シリアル |
| `NOW` | `NOW()` | 現在日時のシリアル |
| `DATE` | `DATE(year, month, day)` | 年・月・日から日付シリアルを作成 |
| `DATEVALUE` | `DATEVALUE(date_text)` | 日付文字列をシリアルに変換 |
| `TIME` | `TIME(hour, minute, second)` | 時・分・秒から時刻小数（0〜1）を作成 |
| `TIMEVALUE` | `TIMEVALUE(time_text)` | 時刻文字列を時刻小数に変換 |
| `DATEDIF` | `DATEDIF(start_date, end_date, unit)` | 日付差（単位: "Y","M","D","YM","YD","MD"） |
| `DAYS` | `DAYS(end_date, start_date)` | 2つの日付の日数差 |
| `DAYS360` | `DAYS360(start_date, end_date)` | 360日年（各月30日）での日数差 |
| `NETWORKDAYS` | `NETWORKDAYS(start_date, end_date, [holidays])` | 営業日数（休日オプション可） |
| `WORKDAY` | `WORKDAY(start_date, days, [holidays])` | 営業日数だけ前後した日付 |
| `YEARFRAC` | `YEARFRAC(start_date, end_date)` | 年間に対する日数の割合 |
| `YEAR` | `YEAR(serial_date)` | 西暦年（4桁） |
| `MONTH` | `MONTH(serial_date)` | 月（1〜12） |
| `DAY` | `DAY(serial_date)` | 日（1〜31） |
| `HOUR` | `HOUR(time_serial)` | 時（0〜23） |
| `MINUTE` | `MINUTE(time_serial)` | 分（0〜59） |
| `SECOND` | `SECOND(time_serial)` | 秒（0〜59） |
| `WEEKDAY` | `WEEKDAY(serial_date, [type])` | 曜日番号（形式により日始まり／月始まり） |
| `WEEKNUM` | `WEEKNUM(serial_date)` | 週番号（1〜53） |
| `EDATE` | `EDATE(start_date, months)` | n か月前後の日付 |
| `EOMONTH` | `EOMONTH(start_date, months)` | n か月前後の月末日 |

### 財務 (Financial)

| 関数 | 構文 | 説明 |
|:---|:---|:---|
| `PMT` | `PMT(rate, nper, pv, [fv], [type])` | ローン等の定期支払額 |
| `PV` | `PV(rate, nper, pmt, [fv], [type])` | 現在価値 |
| `FV` | `FV(rate, nper, pmt, [pv], [type])` | 将来価値 |
| `NPV` | `NPV(rate, val1, [val2]...)` | 正味現在価値（NPV） |
| `IRR` | `IRR(values, [guess])` | 内部収益率（IRR） |
| `RATE` | `RATE(nper, pmt, pv, [fv], [type])` | 期間あたり利率 |
| `NPER` | `NPER(rate, pmt, pv, [fv], [type])` | 期間数 |
| `SLN` | `SLN(cost, salvage, life)` | 定額法の減価償却費 |
| `SYD` | `SYD(cost, salvage, life, per)` | 級数法の減価償却費 |
| `DDB` | `DDB(cost, salvage, life, period, [factor])` | 定率法（倍額定率など）の減価償却費 |

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
