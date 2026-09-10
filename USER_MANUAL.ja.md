# HasuCalc ユーザーマニュアル

> **正本（English）:** [USER_MANUAL.md](USER_MANUAL.md)  
> 概要: [README.md](README.md)（[日本語](README.ja.md)） · 技術仕様: [SPECIFICATION.md](SPECIFICATION.md)（[日本語](SPECIFICATION.ja.md)）  
> 差分がある場合は英語版ユーザーマニュアルを優先してください。

画面の見方、`[READY]` などの操作モード、キーバインド、対応ファイル形式、および組み込み関数の構文と引数を分かりやすく解説します。標準の保存形式は `.hwk` / `.hwkz` で、`.xlsx` / `.ods` / `.csv` / `.md` / `.html` とのデータ連携にも対応しています。

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
| **`[END]`** | End モード。End を押した後に矢印キーを押すと、連続するデータ領域の端へジャンプ。 |

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

- 標準の `=SUM(A1:B10)` に加え、クラシックな `@SUM(A1..B10)` や `+` から始まる数式記法にも対応。
- 範囲指定はコロン（`A1:B10`）とピリオド（`A1..B10`）の両方に対応。全列参照（`A:A`）はデータ実領域（used range）までに自動制限。
- シート間参照: `Sheet2!A1` や、空白・記号を含む場合の `'Q1-2024'!A1`。
- 絶対参照／複合参照: `$A$1`, `$A1`, `A$1`。
- 真偽値: `TRUE` / `FALSE`（および `TRUE()` / `FALSE()`）。
- 論理演算子: 式中で `#AND#`, `#OR#`, `#NOT#` が利用可能（例: `+A1>10#AND#B1<20`）。
- べき乗と単項マイナス: 単項マイナスが `^` より優先（`-2^2` → `(-2)^2` = `4`。負の累乗は `-(2^2)` = `-4`）。

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
- 設定の永続化は `.hwk` / `.hwkz` に行われます（外部形式 `.xlsx` / `.ods` には表データのみを出力）。

---

## 9. 関数リファレンス

`@NAME(...)` と、`=` のあとの `NAME(...)` の両方を受け付けます。

- `[角括弧]` の引数は**省略可能**です。
- 末尾の `...` は、同じ種類の引数を**繰り返せる**ことを意味します（引数列の `…` を参照）。
- **引数**列に、各パラメータへ何を入れるかを書いています。

### 数学・集計 (Math/Agg)

| 関数 | 構文 | 引数 | 概要 |
|:---|:---|:---|:---|
| `SUM` | `SUM(range/list)` | `range/list`: セル範囲、または個々の数値 | 範囲内の数値の合計を求める |
| `SUMIF` | `SUMIF(range, criteria, [sum_range])` | `range`: セル範囲<br>`criteria`: 条件（例: ">50", "Apple", セル参照）<br>`sum_range`（省略可）: 条件一致時に合計するセル（省略時は range） | 条件に合うセルの合計（例: ">50", "Apple"） |
| `SUMIFS` | `SUMIFS(sum_rng, crit_rng1, crit1, ...)` | `sum_rng`: 一致行について合計するセル<br>`crit_rng1`: 1つ目の条件範囲（合計／平均などと同形）<br>`crit1`: 1つ目の条件（例: ">10" やセル）<br>`…`（繰り返し可）: 追加の（条件範囲, 条件）の組。形は sum_rng に合わせる | 複数条件を満たすセルの合計 |
| `SUMPRODUCT` | `SUMPRODUCT(array1, [array2]...)` | `array1`: 1つ目の範囲／配列<br>`…`（繰り返し可）: 追加の配列（次元を揃える） | 対応する要素の積の合計（積和） |
| `PRODUCT` | `PRODUCT(number1, [number2]...)` | `number1`: 1つ目の数値<br>`…`（繰り返し可）: 追加の数値 | 引数の数値をすべて掛け合わせる |
| `SUBTOTAL` | `SUBTOTAL(function_num, ref1, ...)` | `function_num`: 集計の種類番号（1=AVG … 9=SUM。下記注記）<br>`ref1`: 集計に含める1つ目の範囲<br>`…`（繰り返し可）: 集計に含める追加の範囲<br>function_num の例: 1 AVG, 2 COUNT, 3 COUNTA, 4 MAX, 5 MIN, 6 PRODUCT, 7 STDEV, 9 SUM, 10 VAR。 | リストの小計（9=SUM, 1=AVG など） |
| `ROUND` | `ROUND(val, num_digits)` | `val`: 入力値<br>`num_digits`: 小数点以下桁数 | 指定桁に四捨五入 |
| `ROUNDUP` | `ROUNDUP(val, num_digits)` | `val`: 入力値<br>`num_digits`: 小数点以下桁数 | ゼロから遠ざかる方向に切り上げ |
| `ROUNDDOWN` | `ROUNDDOWN(val, num_digits)` | `val`: 入力値<br>`num_digits`: 小数点以下桁数 | ゼロに近づく方向に切り捨て |
| `TRUNC` | `TRUNC(val, [num_digits])` | `val`: 入力値<br>`num_digits`（省略可）: 小数点以下桁数 | 指定桁で切り捨て（端数削除） |
| `INT` | `INT(val)` | `val`: 入力値 | 小数点以下を切り捨てて整数にする |
| `ABS` | `ABS(val)` | `val`: 入力値 | 絶対値を返す |
| `MOD` | `MOD(number, divisor)` | `number`: 数値<br>`divisor`: 割る数 | 除算の余り（剰余） |
| `QUOTIENT` | `QUOTIENT(numerator, denominator)` | `numerator`: 割られる数<br>`denominator`: 割る数 | 割り算の整数部分 |
| `SIGN` | `SIGN(number)` | `number`: 数値 | 符号（正=1, 負=-1, 0=0） |
| `POWER` | `POWER(number, power)` | `number`: 数値<br>`power`: 指数 | 累乗（x^y） |
| `SQRT` | `SQRT(val)` | `val`: 入力値 | 正の数の平方根 |
| `EXP` | `EXP(number)` | `number`: 数値 | e の累乗 |
| `LN` | `LN(number)` | `number`: 数値 | 自然対数 |
| `LOG` | `LOG(number, [base])` | `number`: 数値<br>`base`（省略可）: 対数の底（省略時は10） | 指定底の対数（省略時は10） |
| `LOG10` | `LOG10(number)` | `number`: 数値 | 常用対数（底10） |
| `CEILING` | `CEILING(number, significance)` | `number`: 数値<br>`significance`: 丸めの刻み | 基準値の倍数へ切り上げ |
| `FLOOR` | `FLOOR(number, significance)` | `number`: 数値<br>`significance`: 丸めの刻み | 基準値の倍数へ切り下げ |
| `MROUND` | `MROUND(number, multiple)` | `number`: 数値<br>`multiple`: 丸め先の倍数 | 指定倍数の最も近い値へ丸める |
| `FACT` | `FACT(number)` | `number`: 数値 | 階乗（n!） |
| `GCD` | `GCD(number1, number2, ...)` | `number1`: 1つ目の数値<br>`number2`: 追加の数値<br>`…`（繰り返し可）: 追加の整数 | 最大公約数 |
| `LCM` | `LCM(number1, number2, ...)` | `number1`: 1つ目の数値<br>`number2`: 追加の数値<br>`…`（繰り返し可）: 追加の整数 | 最小公倍数 |
| `COMBIN` | `COMBIN(n, k)` | `n`: 総数（nCr / nPr の n）<br>`k`: 順位や百分位など（概要欄を参照） | 組み合わせの数（nCr） |
| `PERMUT` | `PERMUT(n, k)` | `n`: 総数（nCr / nPr の n）<br>`k`: 順位や百分位など（概要欄を参照） | 順列の数（nPr） |
| `PI` | `PI()` | （引数なし） | 円周率 π |
| `DEGREES` | `DEGREES(angle_in_radians)` | `angle_in_radians`: ラジアン単位の角度 | ラジアンを度に変換 |
| `RADIANS` | `RADIANS(angle_in_degrees)` | `angle_in_degrees`: 度単位の角度 | 度をラジアンに変換 |
| `SIN` | `SIN(number)` | `number`: 数値 | 正弦（ラジアン） |
| `COS` | `COS(number)` | `number`: 数値 | 余弦（ラジアン） |
| `TAN` | `TAN(number)` | `number`: 数値 | 正接（ラジアン） |
| `ASIN` | `ASIN(number)` | `number`: 数値 | 逆正弦（ラジアン） |
| `ACOS` | `ACOS(number)` | `number`: 数値 | 逆余弦（ラジアン） |
| `ATAN` | `ATAN(number)` | `number`: 数値 | 逆正接（ラジアン） |
| `ATAN2` | `ATAN2(x_num, y_num)` | `x_num`: X 座標<br>`y_num`: Y 座標 | 座標から逆正接 |
| `RAND` | `RAND()` | （引数なし） | 0以上1未満の乱数 |
| `RANDBETWEEN` | `RANDBETWEEN(min, max)` | `min`: 最小整数（含む）<br>`max`: 最大整数（含む） | min〜max の整数乱数（両端含む） |

### 統計 (Statistical)

| 関数 | 構文 | 引数 | 概要 |
|:---|:---|:---|:---|
| `AVG` | `AVG(range/list)` | `range/list`: セル範囲、または個々の数値 | 算術平均 |
| `AVERAGEIF` | `AVERAGEIF(range, criteria, [avg_range])` | `range`: セル範囲<br>`criteria`: 条件（例: ">50", "Apple", セル参照）<br>`avg_range`（省略可）: 条件一致時に平均するセル（省略時は range） | 条件に合うセルの平均 |
| `AVERAGEIFS` | `AVERAGEIFS(avg_rng, crit_rng1, crit1, ...)` | `avg_rng`: 平均を取るセル範囲<br>`crit_rng1`: 1つ目の条件範囲（合計／平均などと同形）<br>`crit1`: 1つ目の条件（例: ">10" やセル）<br>`…`（繰り返し可）: 追加の（条件範囲, 条件）の組 | 複数条件を満たすセルの平均 |
| `COUNT` | `COUNT(range/list)` | `range/list`: セル範囲、または個々の数値 | 数値が入ったセルの個数 |
| `COUNTA` | `COUNTA(range/list)` | `range/list`: セル範囲、または個々の数値 | 空白でないセルの個数 |
| `COUNTBLANK` | `COUNTBLANK(range)` | `range`: セル範囲 | 空白セルの個数 |
| `COUNTIF` | `COUNTIF(range, criteria)` | `range`: セル範囲<br>`criteria`: 条件（例: ">50", "Apple", セル参照） | 条件に合うセルの個数 |
| `COUNTIFS` | `COUNTIFS(crit_rng1, crit1, ...)` | `crit_rng1`: 1つ目の条件範囲（合計／平均などと同形）<br>`crit1`: 1つ目の条件（例: ">10" やセル）<br>`…`（繰り返し可）: 追加の（条件範囲, 条件）の組 | 複数条件を満たすセルの個数 |
| `MIN` | `MIN(range/list)` | `range/list`: セル範囲、または個々の数値 | 最小値 |
| `MINIFS` | `MINIFS(min_rng, crit_rng1, crit1, ...)` | `min_rng`: 最小値の候補セル<br>`crit_rng1`: 1つ目の条件範囲（合計／平均などと同形）<br>`crit1`: 1つ目の条件（例: ">10" やセル）<br>`…`（繰り返し可）: 追加の（条件範囲, 条件）の組 | 複数条件を満たす中の最小値 |
| `MAX` | `MAX(range/list)` | `range/list`: セル範囲、または個々の数値 | 最大値 |
| `MAXIFS` | `MAXIFS(max_rng, crit_rng1, crit1, ...)` | `max_rng`: 最大値の候補セル<br>`crit_rng1`: 1つ目の条件範囲（合計／平均などと同形）<br>`crit1`: 1つ目の条件（例: ">10" やセル）<br>`…`（繰り返し可）: 追加の（条件範囲, 条件）の組 | 複数条件を満たす中の最大値 |
| `MEDIAN` | `MEDIAN(range/list)` | `range/list`: セル範囲、または個々の数値 | 中央値（メディアン） |
| `MODE` | `MODE(range/list)` | `range/list`: セル範囲、または個々の数値 | 最頻値（モード） |
| `LARGE` | `LARGE(array, k)` | `array`: セル範囲または値の並び<br>`k`: 順位や百分位など（概要欄を参照）<br>k=1 が最大値。 | k 番目に大きい値 |
| `SMALL` | `SMALL(array, k)` | `array`: セル範囲または値の並び<br>`k`: 順位や百分位など（概要欄を参照）<br>k=1 が最小値。 | k 番目に小さい値 |
| `PERCENTILE` | `PERCENTILE(array, k)` | `array`: セル範囲または値の並び<br>`k`: 順位や百分位など（概要欄を参照）<br>k は0以上1以下。 | 百分位数（k は 0〜1） |
| `QUARTILE` | `QUARTILE(array, quart)` | `array`: セル範囲または値の並び<br>`quart`: 四分位番号（0–4） | 四分位数（0〜4） |
| `STDEV` | `STDEV(range/list)` | `range/list`: セル範囲、または個々の数値 | 標本標準偏差（n-1） |
| `STDEVP` | `STDEVP(range/list)` | `range/list`: セル範囲、または個々の数値 | 母標準偏差（n） |
| `VAR` | `VAR(range/list)` | `range/list`: セル範囲、または個々の数値 | 標本分散（n-1） |
| `VARP` | `VARP(range/list)` | `range/list`: セル範囲、または個々の数値 | 母分散（n） |
| `RANK` | `RANK(num, range, [order])` | `num`: 順位を付ける数値<br>`range`: セル範囲<br>`order`（省略可）: 0=降順相当の順位（省略時）、1=昇順<br>order 0（省略時）では大きい数が順位1。 | 順位（0=降順, 1=昇順） |

### 検索・参照 (Lookup/Ref)

| 関数 | 構文 | 引数 | 概要 |
|:---|:---|:---|:---|
| `XLOOKUP` | `XLOOKUP(key, lk_rng, ret_rng, [fallback], [match], [search])` | `key`: 探す値<br>`lk_rng`: キーを探す範囲<br>`ret_rng`: 一致したときに返す値の範囲<br>`fallback`（省略可）: キーが見つからないときに返す値<br>`match`（省略可）: XLOOKUP の一致モード（省略可）<br>`search`（省略可）: XLOOKUP の検索モード（省略可） | 縦横対応の検索（正確／近似）と見つからないときの値 |
| `VLOOKUP` | `VLOOKUP(key, table_range, col_offset, [exact])` | `key`: 探す値<br>`table_range`: 検索テーブル全体<br>`col_offset`: 範囲内の列オフセット（0始まり）<br>`exact`（省略可）: TRUE/1=完全一致、FALSE/0=近似一致（ソート済み想定） | 左端列を検索し、指定列の値を返す |
| `HLOOKUP` | `HLOOKUP(key, table_range, row_offset, [exact])` | `key`: 探す値<br>`table_range`: 検索テーブル全体<br>`row_offset`: 範囲内の行オフセット（0始まり）<br>`exact`（省略可）: TRUE/1=完全一致、FALSE/0=近似一致（ソート済み想定） | 最上行を検索し、指定行の値を返す |
| `LOOKUP` | `LOOKUP(val, lookup_vector, [result_vector])` | `val`: 入力値<br>`lookup_vector`: 検索用の行または列<br>`result_vector`（省略可）: 返す値の並行範囲（省略可） | 1行または1列の範囲で検索 |
| `INDEX` | `INDEX(range, col_offset, row_offset)` | `range`: セル範囲<br>`col_offset`: 範囲内の列オフセット（0始まり）<br>`row_offset`: 範囲内の行オフセット（0始まり）<br>オフセットは0始まり（先頭セルは col_offset=0, row_offset=0）。 | 交差位置のセル値（オフセットは0始まり） |
| `MATCH` | `MATCH(key, lookup_array, [match_type])` | `key`: 探す値<br>`lookup_array`: 検索する1行または1列<br>`match_type`（省略可）: 一致種類 1/0/-1（省略可。注記参照）<br>match_type: 1=key以下の最大（昇順データ）, 0=完全一致, -1=key以上の最小（降順データ）。 | 一致位置（1始まり） |
| `XMATCH` | `XMATCH(key, lookup_array, [match_mode], [search_mode])` | `key`: 探す値<br>`lookup_array`: 検索する1行または1列<br>`match_mode`（省略可）: 一致モード（省略可）<br>`search_mode`（省略可）: 検索方向／モード（省略可） | 位置検索（完全一致・ワイルドカード・逆方向） |
| `OFFSET` | `OFFSET(ref, rows, cols, [height], [width])` | `ref`: 起点のセルまたは範囲<br>`rows`: ref から縦にずらす行数（負で上）<br>`cols`: ref から横にずらす列数（負で左）<br>`height`（省略可）: 返す参照の行数<br>`width`（省略可）: 返す参照の列数 | 起点からずらした参照を返す |
| `CHOOSE` | `CHOOSE(index, val0, val1, val2...)` | `index`: 選ぶ要素の番号（0始まり）<br>`val0`: index=0 の選択肢<br>`val1`: リスト上の値／選択肢<br>`…`（繰り返し可）: 追加の選択肢<br>index は0始まり（先頭の値は index 0）。 | 0始まりの番号でリストから値を選ぶ |
| `ROW` | `ROW([cell])` | `cell`（省略可）: セル参照（省略時はこの数式セル） | 行番号（1始まり） |
| `COLUMN` | `COLUMN([cell])` | `cell`（省略可）: セル参照（省略時はこの数式セル） | 列番号（1始まり） |
| `ROWS` | `ROWS(range)` | `range`: セル範囲 | 範囲の行数 |
| `COLUMNS` | `COLUMNS(range)` | `range`: セル範囲 | 範囲の列数 |
| `TRANSPOSE` | `TRANSPOSE(array)` | `array`: セル範囲または値の並び | 行列を入れ替える |

### 論理・エラー (Logic/Error)

| 関数 | 構文 | 引数 | 概要 |
|:---|:---|:---|:---|
| `IF` | `IF(condition, true_val, false_val)` | `condition`: 論理条件<br>`true_val`: 条件が真のときの結果<br>`false_val`: 条件が偽のときの結果 | 条件分岐（真／偽の値） |
| `IFS` | `IFS(cond1, val1, [cond2, val2]...)` | `cond1`: 1つ目の条件<br>`val1`: リスト上の値／選択肢<br>`…`（繰り返し可）: 追加の条件と結果の組 | 条件を順に評価する多岐分岐 |
| `SWITCH` | `SWITCH(expr, val1, res1, [val2, res2]..., [default])` | `expr`: val1, val2, … と比較する値<br>`val1`: リスト上の値／選択肢<br>`res1`: expr が val1 のときの結果<br>`…`（繰り返し可）: default の前に置く追加の値と結果の組<br>`default`（省略可）: どれにも一致しないときの結果 | 式を値リストと照合して分岐 |
| `AND` | `AND(logical1, [logical2], ...)` | `logical1`: 1つ目の論理値<br>`logical2`（省略可）: 追加の論理値<br>`…`（繰り返し可）: 追加の論理値 | すべて真なら真 |
| `OR` | `OR(logical1, [logical2], ...)` | `logical1`: 1つ目の論理値<br>`logical2`（省略可）: 追加の論理値<br>`…`（繰り返し可）: 追加の論理値 | どれか真なら真 |
| `NOT` | `NOT(logical)` | `logical`: 真偽値または式 | 論理値を反転 |
| `XOR` | `XOR(logical1, [logical2]...)` | `logical1`: 1つ目の論理値<br>`…`（繰り返し可）: 追加の論理値 | 排他的論理和（XOR） |
| `IFERROR` | `IFERROR(formula, fallback_val)` | `formula`: 評価する式<br>`fallback_val`: 式がエラー／NA のときに返す値 | エラー時に代替値を返す |
| `IFNA` | `IFNA(formula, fallback_val)` | `formula`: 評価する式<br>`fallback_val`: 式がエラー／NA のときに返す値 | NA のとき代替値を返す |
| `ISNUMBER` | `ISNUMBER(val)` | `val`: 入力値 | 数値かどうか（1/0） |
| `ISSTRING` | `ISSTRING(val)` | `val`: 入力値 | 文字列かどうか（1/0） |
| `ISTEXT` | `ISTEXT(val)` | `val`: 入力値 | 文字列かどうか（1/0） |
| `ISNONTEXT` | `ISNONTEXT(val)` | `val`: 入力値 | 文字列でないか（1/0） |
| `ISBLANK` | `ISBLANK(val)` | `val`: 入力値 | 空白セルかどうか |
| `ISLOGICAL` | `ISLOGICAL(val)` | `val`: 入力値 | 真偽値かどうか |
| `ISERR` | `ISERR(val)` | `val`: 入力値 | ERR かどうか（NA は含まない、1/0） |
| `ISNA` | `ISNA(val)` | `val`: 入力値 | NA かどうか（1/0） |
| `ISEVEN` | `ISEVEN(number)` | `number`: 数値 | 偶数かどうか（1/0） |
| `ISODD` | `ISODD(number)` | `number`: 数値 | 奇数かどうか（1/0） |
| `TRUE` | `TRUE()` | （引数なし） | 真を返す |
| `FALSE` | `FALSE()` | （引数なし） | 偽を返す |
| `N` | `N(value)` | `value`: 変換・書式化する値 | 数値に変換 |
| `T` | `T(value)` | `value`: 変換・書式化する値 | 文字列ならそのまま、それ以外は空文字 |
| `TYPE` | `TYPE(value)` | `value`: 変換・書式化する値 | 型コード（1=数値, 2=文字 など） |

### 文字列 (Text)

| 関数 | 構文 | 引数 | 概要 |
|:---|:---|:---|:---|
| `TEXT` | `TEXT(value, format_string)` | `value`: 変換・書式化する値<br>`format_string`: 書式（例: "yyyy/mm/dd", "#,##0.00"） | 書式文字列で数値・日付を文字列化（例: "yyyy/mm/dd"） |
| `TRIM` | `TRIM(text)` | `text`: 文字列 | 前後の空白除去と内部空白の整理 |
| `CLEAN` | `CLEAN(text)` | `text`: 文字列 | 印刷不能文字を除去 |
| `SUBSTITUTE` | `SUBSTITUTE(text, old_text, new_text, [instance])` | `text`: 文字列<br>`old_text`: 元の文字列、または探す文字列<br>`new_text`: 置き換え後の文字列<br>`instance`（省略可）: 何番目の出現を置換するか（省略時は全部） | 部分文字列を置換 |
| `REPLACE` | `REPLACE(old_text, start_pos, num_chars, new_text)` | `old_text`: 元の文字列、または探す文字列<br>`start_pos`: 開始文字位置（1始まり）<br>`num_chars`: 取り出す文字数<br>`new_text`: 置き換え後の文字列 | 位置を指定して文字を置換 |
| `REPT` | `REPT(text, number_times)` | `text`: 文字列<br>`number_times`: 繰り返し回数 | 文字列を指定回数繰り返す |
| `UPPER` | `UPPER(text)` | `text`: 文字列 | 英字を大文字に |
| `LOWER` | `LOWER(text)` | `text`: 文字列 | 英字を小文字に |
| `PROPER` | `PROPER(text)` | `text`: 文字列 | 単語先頭を大文字に（Title Case） |
| `EXACT` | `EXACT(text1, text2)` | `text1`: 1つ目の文字列<br>`text2`: 追加の文字列 | 完全一致比較（大文字小文字を区別） |
| `CHAR` | `CHAR(number)` | `number`: 数値 | コード番号に対応する文字 |
| `CODE` | `CODE(text)` | `text`: 文字列 | 先頭文字のコード番号 |
| `UNICHAR` | `UNICHAR(number)` | `number`: 数値 | Unicode コードポイントの文字 |
| `UNICODE` | `UNICODE(text)` | `text`: 文字列 | 先頭文字の Unicode コードポイント |
| `CONCATENATE` | `CONCATENATE(text1, text2, ...)` | `text1`: 1つ目の文字列<br>`text2`: 追加の文字列<br>`…`（繰り返し可）: 追加の文字列 | 複数文字列を連結 |
| `CONCAT` | `CONCAT(text1, text2, ...)` | `text1`: 1つ目の文字列<br>`text2`: 追加の文字列<br>`…`（繰り返し可）: 追加の文字列または範囲 | 文字列や範囲を連結 |
| `TEXTJOIN` | `TEXTJOIN(delimiter, ignore_empty, text1, ...)` | `delimiter`: 区切り文字列<br>`ignore_empty`: TRUE/1 なら空文字を連結しない<br>`text1`: 1つ目の文字列<br>`…`（繰り返し可）: 追加の文字列または範囲<br>ignore_empty に 1 または TRUE で空をスキップ。 | 区切り文字付きで連結（空無視オプションあり） |
| `LEFT` | `LEFT(text, num_chars)` | `text`: 文字列<br>`num_chars`: 取り出す文字数 | 左から指定文字数を取り出す |
| `RIGHT` | `RIGHT(text, num_chars)` | `text`: 文字列<br>`num_chars`: 取り出す文字数 | 右から指定文字数を取り出す |
| `MID` | `MID(text, start_pos, num_chars)` | `text`: 文字列<br>`start_pos`: 開始文字位置（1始まり）<br>`num_chars`: 取り出す文字数 | 途中から指定文字数を取り出す |
| `LEN` | `LEN(text)` | `text`: 文字列 | 文字数 |
| `FIND` | `FIND(find_text, within_text, [start])` | `find_text`: 探す部分文字列<br>`within_text`: 検索対象の文字列<br>`start`（省略可）: 検索開始位置（1始まり） | 大文字小文字を区別して位置検索（1始まり） |
| `SEARCH` | `SEARCH(find_text, within_text, [start])` | `find_text`: 探す部分文字列<br>`within_text`: 検索対象の文字列<br>`start`（省略可）: 検索開始位置（1始まり） | 大文字小文字無視・ワイルドカード対応の位置検索 |
| `STRING` | `STRING(number, decimal_places)` | `number`: 数値<br>`decimal_places`: 小数点以下の桁数 | 小数桁数を固定して数値を文字列に変換 |
| `VALUE` | `VALUE(text)` | `text`: 文字列 | 通貨記号・カンマ付き文字列を数値に |
| `NUMBERVALUE` | `NUMBERVALUE(text, [dec_sep], [group_sep])` | `text`: 文字列<br>`dec_sep`（省略可）: 小数点に使う文字<br>`group_sep`（省略可）: 桁区切りに使う文字 | 小数点・桁区切りを指定して数値化 |
| `TEXTBEFORE` | `TEXTBEFORE(text, delimiter)` | `text`: 文字列<br>`delimiter`: 区切り文字列 | 区切りより前の文字列 |
| `TEXTAFTER` | `TEXTAFTER(text, delimiter)` | `text`: 文字列<br>`delimiter`: 区切り文字列 | 区切りより後の文字列 |
| `TEXTSPLIT` | `TEXTSPLIT(text, col_delimiter)` | `text`: 文字列<br>`col_delimiter`: 分割に使う区切り文字 | 区切りで分割 |

### 日付・時刻 (Date/Time)

| 関数 | 構文 | 引数 | 概要 |
|:---|:---|:---|:---|
| `TODAY` | `TODAY()` | （引数なし） | 今日の日付シリアル |
| `NOW` | `NOW()` | （引数なし） | 現在日時のシリアル |
| `DATE` | `DATE(year, month, day)` | `year`: 西暦年（4桁）<br>`month`: 月（1–12）<br>`day`: 日（1–31） | 年・月・日から日付シリアルを作成 |
| `DATEVALUE` | `DATEVALUE(date_text)` | `date_text`: 日付を表す文字列 | 日付文字列をシリアルに変換 |
| `TIME` | `TIME(hour, minute, second)` | `hour`: 時（0–23）<br>`minute`: 分（0–59）<br>`second`: 秒（0–59） | 時・分・秒から時刻小数（0〜1）を作成 |
| `TIMEVALUE` | `TIMEVALUE(time_text)` | `time_text`: 時刻を表す文字列 | 時刻文字列を時刻小数に変換 |
| `DATEDIF` | `DATEDIF(start_date, end_date, unit)` | `start_date`: 開始日（シリアルまたは日付セル）<br>`end_date`: 終了日（シリアルまたは日付セル）<br>`unit`: "Y", "M", "D", "YM", "YD", "MD" のいずれか | 日付差（単位: "Y","M","D","YM","YD","MD"） |
| `DAYS` | `DAYS(end_date, start_date)` | `end_date`: 終了日（シリアルまたは日付セル）<br>`start_date`: 開始日（シリアルまたは日付セル） | 2つの日付の日数差 |
| `DAYS360` | `DAYS360(start_date, end_date)` | `start_date`: 開始日（シリアルまたは日付セル）<br>`end_date`: 終了日（シリアルまたは日付セル） | 360日年（各月30日）での日数差 |
| `NETWORKDAYS` | `NETWORKDAYS(start_date, end_date, [holidays])` | `start_date`: 開始日（シリアルまたは日付セル）<br>`end_date`: 終了日（シリアルまたは日付セル）<br>`holidays`（省略可）: 除く休日の日付範囲（省略可） | 営業日数（休日オプション可） |
| `WORKDAY` | `WORKDAY(start_date, days, [holidays])` | `start_date`: 開始日（シリアルまたは日付セル）<br>`days`: 加減する日数（または営業日数）<br>`holidays`（省略可）: 除く休日の日付範囲（省略可） | 営業日数だけ前後した日付 |
| `YEARFRAC` | `YEARFRAC(start_date, end_date)` | `start_date`: 開始日（シリアルまたは日付セル）<br>`end_date`: 終了日（シリアルまたは日付セル） | 年間に対する日数の割合 |
| `YEAR` | `YEAR(serial_date)` | `serial_date`: 日付シリアル（または日付セル） | 西暦年（4桁） |
| `MONTH` | `MONTH(serial_date)` | `serial_date`: 日付シリアル（または日付セル） | 月（1〜12） |
| `DAY` | `DAY(serial_date)` | `serial_date`: 日付シリアル（または日付セル） | 日（1〜31） |
| `HOUR` | `HOUR(time_serial)` | `time_serial`: 時刻（1日の小数）または日時シリアル | 時（0〜23） |
| `MINUTE` | `MINUTE(time_serial)` | `time_serial`: 時刻（1日の小数）または日時シリアル | 分（0〜59） |
| `SECOND` | `SECOND(time_serial)` | `time_serial`: 時刻（1日の小数）または日時シリアル | 秒（0〜59） |
| `WEEKDAY` | `WEEKDAY(serial_date, [type])` | `serial_date`: 日付シリアル（または日付セル）<br>`type`（省略可）: 追加のモード（支払時期や WEEKDAY 形式。注記参照）<br>type で番号体系を選ぶ（例: 1=日…7=土、月曜始まりの11–17）。 | 曜日番号（形式により日始まり／月始まり） |
| `WEEKNUM` | `WEEKNUM(serial_date)` | `serial_date`: 日付シリアル（または日付セル） | 週番号（1〜53） |
| `EDATE` | `EDATE(start_date, months)` | `start_date`: 開始日（シリアルまたは日付セル）<br>`months`: ずらす月数（負も可） | n か月前後の日付 |
| `EOMONTH` | `EOMONTH(start_date, months)` | `start_date`: 開始日（シリアルまたは日付セル）<br>`months`: ずらす月数（負も可） | n か月前後の月末日 |

### 財務 (Financial)

| 関数 | 構文 | 引数 | 概要 |
|:---|:---|:---|:---|
| `PMT` | `PMT(rate, nper, pv, [fv], [type])` | `rate`: 各期の利率／割引率<br>`nper`: 支払回数（期数）<br>`pv`: 現在価値<br>`fv`（省略可）: 将来価値（省略時0）<br>`type`（省略可）: 追加のモード（支払時期や WEEKDAY 形式。注記参照）<br>type: 0=期末払い（省略時）, 1=期首払い。 | ローン等の定期支払額 |
| `PV` | `PV(rate, nper, pmt, [fv], [type])` | `rate`: 各期の利率／割引率<br>`nper`: 支払回数（期数）<br>`pmt`: 各期の支払額<br>`fv`（省略可）: 将来価値（省略時0）<br>`type`（省略可）: 追加のモード（支払時期や WEEKDAY 形式。注記参照）<br>type: 0=期末（省略時）, 1=期首。 | 現在価値 |
| `FV` | `FV(rate, nper, pmt, [pv], [type])` | `rate`: 各期の利率／割引率<br>`nper`: 支払回数（期数）<br>`pmt`: 各期の支払額<br>`pv`（省略可）: 現在価値<br>`type`（省略可）: 追加のモード（支払時期や WEEKDAY 形式。注記参照）<br>type: 0=期末（省略時）, 1=期首。 | 将来価値 |
| `NPV` | `NPV(rate, val1, [val2]...)` | `rate`: 各期の利率／割引率<br>`val1`: リスト上の値／選択肢<br>`…`（繰り返し可）: 追加のキャッシュフロー値 | 正味現在価値（NPV） |
| `IRR` | `IRR(values, [guess])` | `values`: キャッシュフロー金額の範囲<br>`guess`（省略可）: 計算の初期推定値（省略可） | 内部収益率（IRR） |
| `RATE` | `RATE(nper, pmt, pv, [fv], [type])` | `nper`: 支払回数（期数）<br>`pmt`: 各期の支払額<br>`pv`: 現在価値<br>`fv`（省略可）: 将来価値（省略時0）<br>`type`（省略可）: 追加のモード（支払時期や WEEKDAY 形式。注記参照）<br>type: 0=期末（省略時）, 1=期首。 | 期間あたり利率 |
| `NPER` | `NPER(rate, pmt, pv, [fv], [type])` | `rate`: 各期の利率／割引率<br>`pmt`: 各期の支払額<br>`pv`: 現在価値<br>`fv`（省略可）: 将来価値（省略時0）<br>`type`（省略可）: 追加のモード（支払時期や WEEKDAY 形式。注記参照）<br>type: 0=期末（省略時）, 1=期首。 | 期間数 |
| `SLN` | `SLN(cost, salvage, life)` | `cost`: 資産の取得原価<br>`salvage`: 残存価額<br>`life`: 耐用の期数 | 定額法の減価償却費 |
| `SYD` | `SYD(cost, salvage, life, per)` | `cost`: 資産の取得原価<br>`salvage`: 残存価額<br>`life`: 耐用の期数<br>`per`: この減価償却額を求める期 | 級数法の減価償却費 |
| `DDB` | `DDB(cost, salvage, life, period, [factor])` | `cost`: 資産の取得原価<br>`salvage`: 残存価額<br>`life`: 耐用の期数<br>`period`: 期番号<br>`factor`（省略可）: 定率法の係数（多くの場合2） | 定率法（倍額定率など）の減価償却費 |

### 別名（受理するだけ）

| 別名 | 正本 |
|:---|:---|
| `AVG` | `AVERAGE` |
| `LENGTH` | `LEN` |
| `REPEAT` | `REPT` |
| `PAYMT` | `PMT` |
| `MULTIPLY` | `PRODUCT` |
| `STD` | `STDEV.P` / `STDEVP` |
| `STRING` | 数値を固定小数点表記の文字列に変換（互換用） |
| `CONCATENATE` | `CONCAT`（一覧にもあり） |

---

## 10. エラーとよくあるメッセージ

| 表示 | 意味 |
|:---|:---|
| `ERR` | 一般的な数式・計算エラー（ゼロ除算や不正な引数など）。 |
| `NA` | 未発見・欠落（lookup 失敗、`NA()` など）。 |
| `CIRCULAR REF` | 循環参照。 |
| `#REF!` | 壊れた参照（削除されたシートなど）。 |
| “Clipboard is empty!” | コピーなしで貼り付け。 |
| “Imported markup from …” | `.md` / `.html` の取り込み成功。 |

仕様方針と動作保証範囲の詳細は [SPECIFICATION.md](SPECIFICATION.md) §1.4（[日本語](SPECIFICATION.ja.md)）を参照。
