#!/usr/bin/env python3
"""Regenerate §9 of USER_MANUAL.md / USER_MANUAL.ja.md with parameter docs."""
from __future__ import annotations

import re
import subprocess
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def parse_go_string(s: str, start: int) -> tuple[str, int]:
    assert s[start] == '"'
    i = start + 1
    out: list[str] = []
    while i < len(s):
        c = s[i]
        if c == "\\":
            out.append(s[i : i + 2])
            i += 2
            continue
        if c == '"':
            raw = "".join(out)
            return raw.replace(r"\"", '"').replace(r"\\", "\\"), i + 1
        out.append(c)
        i += 1
    raise ValueError("unterminated string")


def parse_entries(text: str) -> list[tuple[str, str, str, str]]:
    rows: list[tuple[str, str, str, str]] = []
    for m in re.finditer(r"\{Name:\s*", text):
        i = m.end()
        name, i = parse_go_string(text, i)
        i += re.match(r",\s*Category:\s*", text[i:]).end()
        cat, i = parse_go_string(text, i)
        i += re.match(r",\s*Syntax:\s*", text[i:]).end()
        syn, i = parse_go_string(text, i)
        i += re.match(r",\s*Description:\s*", text[i:]).end()
        desc, i = parse_go_string(text, i)
        bare = name[1:] if name.startswith("@") else name
        syn2 = syn[1:] if syn.startswith("@") else syn
        rows.append((bare, cat, syn2, desc))
    return rows


def split_args(argstr: str) -> list[tuple[str, bool, bool]]:
    """Split a function argument list into (name, optional, variadic) tuples."""
    if not argstr.strip():
        return []

    # Normalize awkward catalog forms that nest brackets around pairs.
    replacements = {
        "cond1, val1, [cond2, val2]...": "cond1, val1, ...",
        "expr, val1, res1, [val2, res2]..., [default]": "expr, val1, res1, ..., [default]",
        "logical1, [logical2], ...": "logical1, [logical2], ...",
        "logical1, [logical2]...": "logical1, ...",
        "number1, [number2]...": "number1, ...",
        "text1, text2, ...": "text1, text2, ...",
        "text1, [text2]...": "text1, ...",
        "val1, [val2]...": "val1, ...",
        "array1, [array2]...": "array1, ...",
        "ref1, ...": "ref1, ...",
        "sum_rng, crit_rng1, crit1, ...": "sum_rng, crit_rng1, crit1, ...",
        "delimiter, ignore_empty, text1, ...": "delimiter, ignore_empty, text1, ...",
        "rate, val1, [val2]...": "rate, val1, ...",
        "index, val0, val1, val2...": "index, val0, val1, ...",
        "avg_rng, crit_rng1, crit1, ...": "avg_rng, crit_rng1, crit1, ...",
        "crit_rng1, crit1, ...": "crit_rng1, crit1, ...",
        "min_rng, crit_rng1, crit1, ...": "min_rng, crit_rng1, crit1, ...",
        "max_rng, crit_rng1, crit1, ...": "max_rng, crit_rng1, crit1, ...",
        "function_num, ref1, ...": "function_num, ref1, ...",
    }
    s = argstr.strip()
    for a, b in replacements.items():
        if s == a:
            s = b
            break

    out: list[tuple[str, bool, bool]] = []
    for part in s.split(","):
        p = part.strip()
        if not p:
            continue
        optional = False
        variadic = False
        if p.endswith("..."):
            variadic = True
            p = p[:-3].strip()
        if p.startswith("[") and p.endswith("]"):
            optional = True
            p = p[1:-1].strip()
        # Bare "..." becomes empty + variadic
        out.append((p, optional, variadic))
    return out


G_EN = {
    "angle_in_degrees": "angle in degrees",
    "angle_in_radians": "angle in radians",
    "array": "cell range or value list",
    "array1": "first range/array",
    "array2": "additional range/array (same shape as array1)",
    "avg_range": "cells to average when criteria match (defaults to range)",
    "avg_rng": "cells whose average is computed",
    "base": "logarithm base (default 10)",
    "cell": "optional cell reference (defaults to this formula’s cell)",
    "col_delimiter": "text that separates pieces when splitting",
    "col_offset": "column offset inside the range (0-based)",
    "cols": "columns to move from ref (negative = left)",
    "cond1": "first condition",
    "cond2": "next condition",
    "condition": "logical test",
    "cost": "initial cost of the asset",
    "crit1": "first criteria (e.g. \">10\" or a cell)",
    "crit_rng1": "first criteria range (same shape as sum/avg/min/max range)",
    "criteria": "condition such as \">50\", \"Apple\", or a cell reference",
    "date_text": "date as text",
    "day": "day of month (1–31)",
    "days": "number of days (or workdays) to shift",
    "dec_sep": "decimal separator character",
    "decimal_places": "digits after the decimal point",
    "default": "result if no listed value matched",
    "delimiter": "separator text",
    "denominator": "divisor",
    "divisor": "number to divide by",
    "end_date": "end date (serial or date cell)",
    "exact": "TRUE/1 = exact match; FALSE/0 = approximate match (data should be sorted)",
    "expr": "value compared to val1, val2, …",
    "factor": "declining-balance factor (often 2)",
    "fallback": "value if the key is not found",
    "fallback_val": "value if the expression errors / is NA",
    "false_val": "result when condition is false",
    "find_text": "substring to find",
    "format_string": "format pattern, e.g. \"yyyy/mm/dd\" or \"#,##0.00\"",
    "formula": "expression to evaluate",
    "function_num": "which aggregate to run (1=AVG … 9=SUM; see note)",
    "fv": "future value (default 0)",
    "group_sep": "thousands separator character",
    "guess": "optional first guess for the solver",
    "height": "height in rows of the returned reference",
    "holidays": "optional range of holiday dates to skip",
    "hour": "hour 0–23",
    "ignore_empty": "TRUE/1 = skip empty strings when joining",
    "index": "which item to pick (0-based)",
    "instance": "which occurrence to replace (omit = all)",
    "k": "rank or percentile fraction (see summary)",
    "key": "value to look up",
    "life": "number of periods in the asset’s life",
    "lk_rng": "range searched for key",
    "logical": "TRUE/FALSE value or expression",
    "logical1": "first logical value",
    "logical2": "another logical value",
    "lookup_array": "single row or column to search",
    "lookup_vector": "lookup row or column",
    "match": "optional XLOOKUP match mode",
    "match_mode": "optional exact / wildcard / approximate mode",
    "match_type": "optional 1 / 0 / -1 (see note)",
    "max": "largest integer allowed (inclusive)",
    "max_rng": "cells that supply candidate maximum values",
    "min": "smallest integer allowed (inclusive)",
    "min_rng": "cells that supply candidate minimum values",
    "minute": "minute 0–59",
    "month": "month 1–12",
    "months": "months to move (may be negative)",
    "multiple": "multiple to round toward",
    "n": "n in nCr / nPr (total items)",
    "new_text": "replacement text",
    "nper": "number of payment periods",
    "num": "number to rank",
    "num_chars": "how many characters to return",
    "num_digits": "decimal places",
    "number": "a number",
    "number1": "first number",
    "number2": "another number",
    "number_times": "repeat count",
    "numerator": "dividend",
    "old_text": "original text, or the text to find",
    "order": "0 = descending ranks (default), 1 = ascending",
    "per": "period index for this depreciation charge",
    "period": "period index",
    "pmt": "payment per period",
    "power": "exponent",
    "pv": "present value",
    "quart": "quartile index 0–4",
    "range": "cell range",
    "range/list": "a range and/or individual numbers",
    "rate": "interest or discount rate per period",
    "ref": "starting cell or range",
    "ref1": "first range included in the subtotal",
    "res1": "result when expr equals val1",
    "res2": "result when expr equals val2",
    "result_vector": "optional parallel range of return values",
    "ret_rng": "values returned for a successful match",
    "row_offset": "row offset inside the range (0-based)",
    "rows": "rows to move from ref (negative = up)",
    "salvage": "value at the end of life",
    "search": "optional XLOOKUP search mode",
    "search_mode": "optional search direction/mode",
    "second": "second 0–59",
    "serial_date": "date serial (or a cell holding a date)",
    "significance": "rounding step / multiple",
    "start": "1-based character position to start searching",
    "start_date": "start date (serial or date cell)",
    "start_pos": "1-based start character",
    "sum_range": "cells to add when criteria match (defaults to range)",
    "sum_rng": "cells to add for matching rows",
    "table_range": "full lookup table",
    "text": "text string",
    "text1": "first text value",
    "text2": "another text value",
    "time_serial": "time as fraction of a day, or date-time serial",
    "time_text": "time as text",
    "true_val": "result when condition is true",
    "type": "extra mode flag (payment timing or WEEKDAY style; see note)",
    "unit": "\"Y\", \"M\", \"D\", \"YM\", \"YD\", or \"MD\"",
    "val": "input value",
    "val0": "choice for index 0",
    "val1": "a listed value / choice",
    "val2": "another listed value / choice",
    "value": "value to convert or format",
    "values": "cash-flow amounts (range)",
    "width": "width in columns of the returned reference",
    "within_text": "text to search within",
    "x_num": "X coordinate",
    "y_num": "Y coordinate",
    "year": "four-digit year",
}

G_JA = {
    "angle_in_degrees": "度単位の角度",
    "angle_in_radians": "ラジアン単位の角度",
    "array": "セル範囲または値の並び",
    "array1": "1つ目の範囲／配列",
    "array2": "追加の範囲／配列（array1 と同じ形）",
    "avg_range": "条件一致時に平均するセル（省略時は range）",
    "avg_rng": "平均を取るセル範囲",
    "base": "対数の底（省略時は10）",
    "cell": "セル参照（省略時はこの数式セル）",
    "col_delimiter": "分割に使う区切り文字",
    "col_offset": "範囲内の列オフセット（0始まり）",
    "cols": "ref から横にずらす列数（負で左）",
    "cond1": "1つ目の条件",
    "cond2": "次の条件",
    "condition": "論理条件",
    "cost": "資産の取得原価",
    "crit1": "1つ目の条件（例: \">10\" やセル）",
    "crit_rng1": "1つ目の条件範囲（合計／平均などと同形）",
    "criteria": "条件（例: \">50\", \"Apple\", セル参照）",
    "date_text": "日付を表す文字列",
    "day": "日（1–31）",
    "days": "加減する日数（または営業日数）",
    "dec_sep": "小数点に使う文字",
    "decimal_places": "小数点以下の桁数",
    "default": "どれにも一致しないときの結果",
    "delimiter": "区切り文字列",
    "denominator": "割る数",
    "divisor": "割る数",
    "end_date": "終了日（シリアルまたは日付セル）",
    "exact": "TRUE/1=完全一致、FALSE/0=近似一致（ソート済み想定）",
    "expr": "val1, val2, … と比較する値",
    "factor": "定率法の係数（多くの場合2）",
    "fallback": "キーが見つからないときに返す値",
    "fallback_val": "式がエラー／NA のときに返す値",
    "false_val": "条件が偽のときの結果",
    "find_text": "探す部分文字列",
    "format_string": "書式（例: \"yyyy/mm/dd\", \"#,##0.00\"）",
    "formula": "評価する式",
    "function_num": "集計の種類番号（1=AVG … 9=SUM。下記注記）",
    "fv": "将来価値（省略時0）",
    "group_sep": "桁区切りに使う文字",
    "guess": "計算の初期推定値（省略可）",
    "height": "返す参照の行数",
    "holidays": "除く休日の日付範囲（省略可）",
    "hour": "時（0–23）",
    "ignore_empty": "TRUE/1 なら空文字を連結しない",
    "index": "選ぶ要素の番号（0始まり）",
    "instance": "何番目の出現を置換するか（省略時は全部）",
    "k": "順位や百分位など（概要欄を参照）",
    "key": "探す値",
    "life": "耐用の期数",
    "lk_rng": "キーを探す範囲",
    "logical": "真偽値または式",
    "logical1": "1つ目の論理値",
    "logical2": "追加の論理値",
    "lookup_array": "検索する1行または1列",
    "lookup_vector": "検索用の行または列",
    "match": "XLOOKUP の一致モード（省略可）",
    "match_mode": "一致モード（省略可）",
    "match_type": "一致種類 1/0/-1（省略可。注記参照）",
    "max": "最大整数（含む）",
    "max_rng": "最大値の候補セル",
    "min": "最小整数（含む）",
    "min_rng": "最小値の候補セル",
    "minute": "分（0–59）",
    "month": "月（1–12）",
    "months": "ずらす月数（負も可）",
    "multiple": "丸め先の倍数",
    "n": "総数（nCr / nPr の n）",
    "new_text": "置き換え後の文字列",
    "nper": "支払回数（期数）",
    "num": "順位を付ける数値",
    "num_chars": "取り出す文字数",
    "num_digits": "小数点以下桁数",
    "number": "数値",
    "number1": "1つ目の数値",
    "number2": "追加の数値",
    "number_times": "繰り返し回数",
    "numerator": "割られる数",
    "old_text": "元の文字列、または探す文字列",
    "order": "0=降順相当の順位（省略時）、1=昇順",
    "per": "この減価償却額を求める期",
    "period": "期番号",
    "pmt": "各期の支払額",
    "power": "指数",
    "pv": "現在価値",
    "quart": "四分位番号（0–4）",
    "range": "セル範囲",
    "range/list": "セル範囲、または個々の数値",
    "rate": "各期の利率／割引率",
    "ref": "起点のセルまたは範囲",
    "ref1": "集計に含める1つ目の範囲",
    "res1": "expr が val1 のときの結果",
    "res2": "expr が val2 のときの結果",
    "result_vector": "返す値の並行範囲（省略可）",
    "ret_rng": "一致したときに返す値の範囲",
    "row_offset": "範囲内の行オフセット（0始まり）",
    "rows": "ref から縦にずらす行数（負で上）",
    "salvage": "残存価額",
    "search": "XLOOKUP の検索モード（省略可）",
    "search_mode": "検索方向／モード（省略可）",
    "second": "秒（0–59）",
    "serial_date": "日付シリアル（または日付セル）",
    "significance": "丸めの刻み",
    "start": "検索開始位置（1始まり）",
    "start_date": "開始日（シリアルまたは日付セル）",
    "start_pos": "開始文字位置（1始まり）",
    "sum_range": "条件一致時に合計するセル（省略時は range）",
    "sum_rng": "一致行について合計するセル",
    "table_range": "検索テーブル全体",
    "text": "文字列",
    "text1": "1つ目の文字列",
    "text2": "追加の文字列",
    "time_serial": "時刻（1日の小数）または日時シリアル",
    "time_text": "時刻を表す文字列",
    "true_val": "条件が真のときの結果",
    "type": "追加のモード（支払時期や WEEKDAY 形式。注記参照）",
    "unit": "\"Y\", \"M\", \"D\", \"YM\", \"YD\", \"MD\" のいずれか",
    "val": "入力値",
    "val0": "index=0 の選択肢",
    "val1": "リスト上の値／選択肢",
    "val2": "追加の値／選択肢",
    "value": "変換・書式化する値",
    "values": "キャッシュフロー金額の範囲",
    "width": "返す参照の列数",
    "within_text": "検索対象の文字列",
    "x_num": "X 座標",
    "y_num": "Y 座標",
    "year": "西暦年（4桁）",
}

VARIADIC_EN = {
    "SUMIFS": "more pairs of (criteria_range, criteria); same shape as sum_rng",
    "AVERAGEIFS": "more pairs of (criteria_range, criteria)",
    "COUNTIFS": "more pairs of (criteria_range, criteria)",
    "MINIFS": "more pairs of (criteria_range, criteria)",
    "MAXIFS": "more pairs of (criteria_range, criteria)",
    "SUBTOTAL": "additional ranges to include",
    "GCD": "additional integers",
    "LCM": "additional integers",
    "AND": "additional logical values",
    "OR": "additional logical values",
    "XOR": "additional logical values",
    "CONCAT": "additional text values or ranges",
    "CONCATENATE": "additional text values",
    "TEXTJOIN": "additional text values or ranges",
    "SUMPRODUCT": "additional arrays (same dimensions)",
    "NPV": "additional cash-flow values",
    "IFS": "more condition/value pairs",
    "SWITCH": "more value/result pairs before default",
    "PRODUCT": "additional numbers",
    "CHOOSE": "additional choice values",
}

VARIADIC_JA = {
    "SUMIFS": "追加の（条件範囲, 条件）の組。形は sum_rng に合わせる",
    "AVERAGEIFS": "追加の（条件範囲, 条件）の組",
    "COUNTIFS": "追加の（条件範囲, 条件）の組",
    "MINIFS": "追加の（条件範囲, 条件）の組",
    "MAXIFS": "追加の（条件範囲, 条件）の組",
    "SUBTOTAL": "集計に含める追加の範囲",
    "GCD": "追加の整数",
    "LCM": "追加の整数",
    "AND": "追加の論理値",
    "OR": "追加の論理値",
    "XOR": "追加の論理値",
    "CONCAT": "追加の文字列または範囲",
    "CONCATENATE": "追加の文字列",
    "TEXTJOIN": "追加の文字列または範囲",
    "SUMPRODUCT": "追加の配列（次元を揃える）",
    "NPV": "追加のキャッシュフロー値",
    "IFS": "追加の条件と結果の組",
    "SWITCH": "default の前に置く追加の値と結果の組",
    "PRODUCT": "追加の数値",
    "CHOOSE": "追加の選択肢",
}

NOTES_EN = {
    "INDEX": "Offsets are 0-based (first cell is col_offset=0, row_offset=0).",
    "CHOOSE": "index is 0-based: the first value is index 0.",
    "SUBTOTAL": "function_num examples: 1 AVG, 2 COUNT, 3 COUNTA, 4 MAX, 5 MIN, 6 PRODUCT, 7 STDEV, 9 SUM, 10 VAR.",
    "MATCH": "match_type: 1 = largest value ≤ key (ascending data), 0 = exact, -1 = smallest value ≥ key (descending data).",
    "WEEKDAY": "type selects the numbering system (e.g. 1=Sunday…7=Saturday, or Monday-based 11–17).",
    "PMT": "type: 0 = pay at end of period (default), 1 = pay at beginning.",
    "PV": "type: 0 = end of period (default), 1 = beginning.",
    "FV": "type: 0 = end of period (default), 1 = beginning.",
    "RATE": "type: 0 = end of period (default), 1 = beginning.",
    "NPER": "type: 0 = end of period (default), 1 = beginning.",
    "LARGE": "k=1 returns the largest value.",
    "SMALL": "k=1 returns the smallest value.",
    "PERCENTILE": "k is between 0 and 1 inclusive.",
    "RANK": "With order 0 (default), larger numbers get rank 1.",
    "TEXTJOIN": "Pass 1 or TRUE for ignore_empty to skip blanks.",
}

NOTES_JA = {
    "INDEX": "オフセットは0始まり（先頭セルは col_offset=0, row_offset=0）。",
    "CHOOSE": "index は0始まり（先頭の値は index 0）。",
    "SUBTOTAL": "function_num の例: 1 AVG, 2 COUNT, 3 COUNTA, 4 MAX, 5 MIN, 6 PRODUCT, 7 STDEV, 9 SUM, 10 VAR。",
    "MATCH": "match_type: 1=key以下の最大（昇順データ）, 0=完全一致, -1=key以上の最小（降順データ）。",
    "WEEKDAY": "type で番号体系を選ぶ（例: 1=日…7=土、月曜始まりの11–17）。",
    "PMT": "type: 0=期末払い（省略時）, 1=期首払い。",
    "PV": "type: 0=期末（省略時）, 1=期首。",
    "FV": "type: 0=期末（省略時）, 1=期首。",
    "RATE": "type: 0=期末（省略時）, 1=期首。",
    "NPER": "type: 0=期末（省略時）, 1=期首。",
    "LARGE": "k=1 が最大値。",
    "SMALL": "k=1 が最小値。",
    "PERCENTILE": "k は0以上1以下。",
    "RANK": "order 0（省略時）では大きい数が順位1。",
    "TEXTJOIN": "ignore_empty に 1 または TRUE で空をスキップ。",
}

ORDER = ["Math/Agg", "Statistical", "Lookup/Ref", "Logic/Error", "Text", "Date/Time", "Financial"]
CAT_JA = {
    "Math/Agg": "数学・集計 (Math/Agg)",
    "Statistical": "統計 (Statistical)",
    "Lookup/Ref": "検索・参照 (Lookup/Ref)",
    "Logic/Error": "論理・エラー (Logic/Error)",
    "Text": "文字列 (Text)",
    "Date/Time": "日付・時刻 (Date/Time)",
    "Financial": "財務 (Financial)",
}


def esc(s: str) -> str:
    return s.replace("|", "\\|")


def format_params(
    name: str,
    syn: str,
    gloss: dict[str, str],
    notes: dict[str, str],
    variadic_map: dict[str, str],
    lang: str,
) -> str:
    m = re.search(r"\((.*)\)$", syn)
    none = "（引数なし）" if lang == "ja" else "(no arguments)"
    if not m:
        return none
    args = split_args(m.group(1))
    if not args:
        return none
    lines: list[str] = []
    for p, optional, variadic in args:
        if p == "" and variadic:
            meaning = variadic_map.get(
                name, "additional arguments" if lang == "en" else "追加の引数"
            )
            if lang == "en":
                lines.append(f"`…` (repeatable): {meaning}")
            else:
                lines.append(f"`…`（繰り返し可）: {meaning}")
            continue
        if not p:
            continue
        if p not in gloss:
            raise KeyError(f"{name}: missing gloss for {p!r} in {syn!r}")
        meaning = gloss[p]
        if lang == "en":
            flag = ""
            if optional:
                flag += " (optional)"
            if variadic:
                flag += " (repeatable)"
            lines.append(f"`{p}`{flag}: {meaning}")
        else:
            flag = ""
            if optional:
                flag += "（省略可）"
            if variadic:
                flag += "（繰り返し可）"
            lines.append(f"`{p}`{flag}: {meaning}")
    if name in notes:
        lines.append(notes[name])
    return "<br>".join(lines)


def load_desc_ja() -> dict[str, str]:
    old = subprocess.check_output(
        ["git", "show", "4652115:USER_MANUAL.ja.md"],
        cwd=ROOT,
        text=True,
    )
    out: dict[str, str] = {}
    for line in old.splitlines():
        if not line.startswith("| `"):
            continue
        cols = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cols) == 3 and cols[0].startswith("`"):
            out[cols[0].strip("`")] = cols[2]
    return out


def build_section(lang: str, rows: list[tuple[str, str, str, str]], desc_ja: dict[str, str]) -> str:
    by: dict[str, list[tuple[str, str, str]]] = defaultdict(list)
    for name, cat, syn, desc in rows:
        by[cat].append((name, syn, desc))

    lines: list[str] = []
    if lang == "en":
        lines.append(
            """## 9. Function reference

Functions accept either `@NAME(...)` or `NAME(...)` after `=`.

- Arguments in `[brackets]` are **optional**.
- A trailing `...` means you may **repeat** that kind of argument (see the `…` row in Parameters).
- The **Parameters** column says what to put in each argument.

"""
        )
        hdr = "| Function | Syntax | Parameters | Summary |\n|:---|:---|:---|:---|\n"
        gloss, notes, vmap = G_EN, NOTES_EN, VARIADIC_EN
        cats = {c: c for c in ORDER}

        def summary(name: str, desc: str) -> str:
            return esc(desc)

        alias = """### Aliases (also accepted)

| Alias | Canonical |
|:---|:---|
| `AVG` | `AVERAGE` |
| `LENGTH` | `LEN` |
| `REPEAT` | `REPT` |
| `PAYMT` | `PMT` |
| `MULTIPLY` | `PRODUCT` |
| `STD` | `STDEV.P` / `STDEVP` |
| `STRING` | Number to fixed-decimal text (compatibility) |
| `CONCATENATE` | `CONCAT` (also listed) |

---

"""
    else:
        lines.append(
            """## 9. 関数リファレンス

`@NAME(...)` と、`=` のあとの `NAME(...)` の両方を受け付けます。

- `[角括弧]` の引数は**省略可能**です。
- 末尾の `...` は、同じ種類の引数を**繰り返せる**ことを意味します（引数列の `…` を参照）。
- **引数**列に、各パラメータへ何を入れるかを書いています。

"""
        )
        hdr = "| 関数 | 構文 | 引数 | 概要 |\n|:---|:---|:---|:---|\n"
        gloss, notes, vmap = G_JA, NOTES_JA, VARIADIC_JA
        cats = CAT_JA

        def summary(name: str, desc: str) -> str:
            return esc(desc_ja.get(name, desc))

        alias = """### 別名（受理するだけ）

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

"""

    for cat in ORDER:
        lines.append(f"### {cats[cat]}\n\n")
        lines.append(hdr)
        for name, syn, desc in by[cat]:
            params = format_params(name, syn, gloss, notes, vmap, lang)
            lines.append(
                f"| `{name}` | `{esc(syn)}` | {params} | {summary(name, desc)} |\n"
            )
        lines.append("\n")
    lines.append(alias)
    return "".join(lines)


def replace_section(path: Path, section: str) -> None:
    text = path.read_text()
    if path.name == "USER_MANUAL.md":
        m9, m10 = "## 9. Function reference", "## 10. Errors and common messages"
    else:
        m9, m10 = "## 9. 関数リファレンス", "## 10. エラーとよくあるメッセージ"
    i9, i10 = text.find(m9), text.find(m10)
    if i9 < 0 or i10 < 0:
        raise SystemExit(f"markers not found in {path}")
    path.write_text(text[:i9] + section + text[i10:])


def main() -> None:
    for k in G_EN:
        G_JA.setdefault(k, G_EN[k])
    rows = parse_entries((ROOT / "tui" / "funcpicker.go").read_text())
    desc_ja = load_desc_ja()
    replace_section(ROOT / "USER_MANUAL.md", build_section("en", rows, desc_ja))
    replace_section(ROOT / "USER_MANUAL.ja.md", build_section("ja", rows, desc_ja))
    ja = (ROOT / "USER_MANUAL.ja.md").read_text()
    assert "| `SUMPRODUCT`" in ja
    assert "| `SUMIFS`" in ja
    print("regenerated USER_MANUAL.md and USER_MANUAL.ja.md")


if __name__ == "__main__":
    main()
