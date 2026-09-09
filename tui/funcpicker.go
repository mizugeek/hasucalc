package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

type FuncEntry struct {
	Name        string
	Category    string
	Syntax      string
	Description string
	Snippet     string
}

var AllFunctions = []FuncEntry{
	// Math & Aggregation
	{Name: "@SUM", Category: "Math/Agg", Syntax: "@SUM(range/list)", Description: "Calculates total sum of numbers in range", Snippet: "@SUM("},
	{Name: "@SUMIF", Category: "Math/Agg", Syntax: "@SUMIF(range, criteria, [sum_range])", Description: "Sums cells meeting specified criteria (e.g. \">50\", \"Apple\")", Snippet: "@SUMIF("},
	{Name: "@SUMIFS", Category: "Math/Agg", Syntax: "@SUMIFS(sum_rng, crit_rng1, crit1, ...)", Description: "Sums cells that meet multiple criteria across ranges", Snippet: "@SUMIFS("},
	{Name: "@SUMPRODUCT", Category: "Math/Agg", Syntax: "@SUMPRODUCT(array1, [array2]...)", Description: "Calculates sum of products of corresponding items", Snippet: "@SUMPRODUCT("},
	{Name: "@PRODUCT", Category: "Math/Agg", Syntax: "@PRODUCT(number1, [number2]...)", Description: "Multiplies all numbers given in arguments", Snippet: "@PRODUCT("},
	{Name: "@SUBTOTAL", Category: "Math/Agg", Syntax: "@SUBTOTAL(function_num, ref1, ...)", Description: "Calculates subtotal in list/database (9=SUM, 1=AVG, etc.)", Snippet: "@SUBTOTAL("},
	{Name: "@ROUND", Category: "Math/Agg", Syntax: "@ROUND(val, num_digits)", Description: "Rounds number to specified decimal places", Snippet: "@ROUND("},
	{Name: "@ROUNDUP", Category: "Math/Agg", Syntax: "@ROUNDUP(val, num_digits)", Description: "Rounds number up, away from zero", Snippet: "@ROUNDUP("},
	{Name: "@ROUNDDOWN", Category: "Math/Agg", Syntax: "@ROUNDDOWN(val, num_digits)", Description: "Rounds number down, towards zero", Snippet: "@ROUNDDOWN("},
	{Name: "@TRUNC", Category: "Math/Agg", Syntax: "@TRUNC(val, [num_digits])", Description: "Truncates number to specified decimal places", Snippet: "@TRUNC("},
	{Name: "@INT", Category: "Math/Agg", Syntax: "@INT(val)", Description: "Rounds number down to nearest integer", Snippet: "@INT("},
	{Name: "@ABS", Category: "Math/Agg", Syntax: "@ABS(val)", Description: "Returns absolute value of number", Snippet: "@ABS("},
	{Name: "@MOD", Category: "Math/Agg", Syntax: "@MOD(number, divisor)", Description: "Returns remainder after division (modulo)", Snippet: "@MOD("},
	{Name: "@QUOTIENT", Category: "Math/Agg", Syntax: "@QUOTIENT(numerator, denominator)", Description: "Returns integer portion of a division", Snippet: "@QUOTIENT("},
	{Name: "@SIGN", Category: "Math/Agg", Syntax: "@SIGN(number)", Description: "Returns sign of number (1=pos, -1=neg, 0=zero)", Snippet: "@SIGN("},
	{Name: "@POWER", Category: "Math/Agg", Syntax: "@POWER(number, power)", Description: "Calculates number raised to a power (x^y)", Snippet: "@POWER("},
	{Name: "@SQRT", Category: "Math/Agg", Syntax: "@SQRT(val)", Description: "Calculates square root of positive number", Snippet: "@SQRT("},
	{Name: "@EXP", Category: "Math/Agg", Syntax: "@EXP(number)", Description: "Returns e raised to the power of number", Snippet: "@EXP("},
	{Name: "@LN", Category: "Math/Agg", Syntax: "@LN(number)", Description: "Returns natural logarithm of number", Snippet: "@LN("},
	{Name: "@LOG", Category: "Math/Agg", Syntax: "@LOG(number, [base])", Description: "Returns logarithm of number to specified base (default 10)", Snippet: "@LOG("},
	{Name: "@LOG10", Category: "Math/Agg", Syntax: "@LOG10(number)", Description: "Returns base-10 logarithm of number", Snippet: "@LOG10("},
	{Name: "@CEILING", Category: "Math/Agg", Syntax: "@CEILING(number, significance)", Description: "Rounds number up to nearest multiple of significance", Snippet: "@CEILING("},
	{Name: "@FLOOR", Category: "Math/Agg", Syntax: "@FLOOR(number, significance)", Description: "Rounds number down to nearest multiple of significance", Snippet: "@FLOOR("},
	{Name: "@MROUND", Category: "Math/Agg", Syntax: "@MROUND(number, multiple)", Description: "Rounds number to nearest multiple", Snippet: "@MROUND("},
	{Name: "@FACT", Category: "Math/Agg", Syntax: "@FACT(number)", Description: "Calculates factorial of a number (n!)", Snippet: "@FACT("},
	{Name: "@GCD", Category: "Math/Agg", Syntax: "@GCD(number1, number2, ...)", Description: "Returns greatest common divisor", Snippet: "@GCD("},
	{Name: "@LCM", Category: "Math/Agg", Syntax: "@LCM(number1, number2, ...)", Description: "Returns least common multiple", Snippet: "@LCM("},
	{Name: "@COMBIN", Category: "Math/Agg", Syntax: "@COMBIN(n, k)", Description: "Returns number of combinations for n items choose k (nCr)", Snippet: "@COMBIN("},
	{Name: "@PERMUT", Category: "Math/Agg", Syntax: "@PERMUT(n, k)", Description: "Returns number of permutations for n items choose k (nPr)", Snippet: "@PERMUT("},
	{Name: "@PI", Category: "Math/Agg", Syntax: "@PI()", Description: "Returns constant value of Pi (3.14159265...)", Snippet: "@PI()"},
	{Name: "@DEGREES", Category: "Math/Agg", Syntax: "@DEGREES(angle_in_radians)", Description: "Converts radians to degrees", Snippet: "@DEGREES("},
	{Name: "@RADIANS", Category: "Math/Agg", Syntax: "@RADIANS(angle_in_degrees)", Description: "Converts degrees to radians", Snippet: "@RADIANS("},
	{Name: "@SIN", Category: "Math/Agg", Syntax: "@SIN(number)", Description: "Returns sine of an angle in radians", Snippet: "@SIN("},
	{Name: "@COS", Category: "Math/Agg", Syntax: "@COS(number)", Description: "Returns cosine of an angle in radians", Snippet: "@COS("},
	{Name: "@TAN", Category: "Math/Agg", Syntax: "@TAN(number)", Description: "Returns tangent of an angle in radians", Snippet: "@TAN("},
	{Name: "@ASIN", Category: "Math/Agg", Syntax: "@ASIN(number)", Description: "Returns arcsine (inverse sine) in radians", Snippet: "@ASIN("},
	{Name: "@ACOS", Category: "Math/Agg", Syntax: "@ACOS(number)", Description: "Returns arccosine (inverse cosine) in radians", Snippet: "@ACOS("},
	{Name: "@ATAN", Category: "Math/Agg", Syntax: "@ATAN(number)", Description: "Returns arctangent in radians", Snippet: "@ATAN("},
	{Name: "@ATAN2", Category: "Math/Agg", Syntax: "@ATAN2(x_num, y_num)", Description: "Returns arctangent from x and y coordinates", Snippet: "@ATAN2("},
	{Name: "@RAND", Category: "Math/Agg", Syntax: "@RAND()", Description: "Returns random real number between 0 and 1", Snippet: "@RAND()"},
	{Name: "@RANDBETWEEN", Category: "Math/Agg", Syntax: "@RANDBETWEEN(min, max)", Description: "Returns random integer between min and max (inclusive)", Snippet: "@RANDBETWEEN("},

	// Statistical
	{Name: "@AVG", Category: "Statistical", Syntax: "@AVG(range/list)", Description: "Calculates arithmetic mean (average)", Snippet: "@AVG("},
	{Name: "@AVERAGEIF", Category: "Statistical", Syntax: "@AVERAGEIF(range, criteria, [avg_range])", Description: "Calculates average of cells meeting criteria", Snippet: "@AVERAGEIF("},
	{Name: "@AVERAGEIFS", Category: "Statistical", Syntax: "@AVERAGEIFS(avg_rng, crit_rng1, crit1, ...)", Description: "Calculates average of cells meeting multiple criteria", Snippet: "@AVERAGEIFS("},
	{Name: "@COUNT", Category: "Statistical", Syntax: "@COUNT(range/list)", Description: "Counts number of numeric cells in range", Snippet: "@COUNT("},
	{Name: "@COUNTA", Category: "Statistical", Syntax: "@COUNTA(range/list)", Description: "Counts number of non-empty cells in range", Snippet: "@COUNTA("},
	{Name: "@COUNTBLANK", Category: "Statistical", Syntax: "@COUNTBLANK(range)", Description: "Counts number of empty cells in range", Snippet: "@COUNTBLANK("},
	{Name: "@COUNTIF", Category: "Statistical", Syntax: "@COUNTIF(range, criteria)", Description: "Counts number of cells meeting criteria", Snippet: "@COUNTIF("},
	{Name: "@COUNTIFS", Category: "Statistical", Syntax: "@COUNTIFS(crit_rng1, crit1, ...)", Description: "Counts cells that meet multiple criteria across ranges", Snippet: "@COUNTIFS("},
	{Name: "@MIN", Category: "Statistical", Syntax: "@MIN(range/list)", Description: "Finds minimum value in range/list", Snippet: "@MIN("},
	{Name: "@MINIFS", Category: "Statistical", Syntax: "@MINIFS(min_rng, crit_rng1, crit1, ...)", Description: "Finds minimum value among cells meeting multiple criteria", Snippet: "@MINIFS("},
	{Name: "@MAX", Category: "Statistical", Syntax: "@MAX(range/list)", Description: "Finds maximum value in range/list", Snippet: "@MAX("},
	{Name: "@MAXIFS", Category: "Statistical", Syntax: "@MAXIFS(max_rng, crit_rng1, crit1, ...)", Description: "Finds maximum value among cells meeting multiple criteria", Snippet: "@MAXIFS("},
	{Name: "@MEDIAN", Category: "Statistical", Syntax: "@MEDIAN(range/list)", Description: "Returns median (middle value) of numbers", Snippet: "@MEDIAN("},
	{Name: "@MODE", Category: "Statistical", Syntax: "@MODE(range/list)", Description: "Returns most frequently occurring value in data set", Snippet: "@MODE("},
	{Name: "@LARGE", Category: "Statistical", Syntax: "@LARGE(array, k)", Description: "Returns k-th largest value in a data set", Snippet: "@LARGE("},
	{Name: "@SMALL", Category: "Statistical", Syntax: "@SMALL(array, k)", Description: "Returns k-th smallest value in a data set", Snippet: "@SMALL("},
	{Name: "@PERCENTILE", Category: "Statistical", Syntax: "@PERCENTILE(array, k)", Description: "Returns k-th percentile of values in a range (0..1)", Snippet: "@PERCENTILE("},
	{Name: "@QUARTILE", Category: "Statistical", Syntax: "@QUARTILE(array, quart)", Description: "Returns quartile of data set (0..4)", Snippet: "@QUARTILE("},
	{Name: "@STDEV", Category: "Statistical", Syntax: "@STDEV(range/list)", Description: "Estimates sample standard deviation (n-1)", Snippet: "@STDEV("},
	{Name: "@STDEVP", Category: "Statistical", Syntax: "@STDEVP(range/list)", Description: "Calculates population standard deviation (n)", Snippet: "@STDEVP("},
	{Name: "@VAR", Category: "Statistical", Syntax: "@VAR(range/list)", Description: "Estimates sample variance (n-1)", Snippet: "@VAR("},
	{Name: "@VARP", Category: "Statistical", Syntax: "@VARP(range/list)", Description: "Calculates population variance (n)", Snippet: "@VARP("},
	{Name: "@RANK", Category: "Statistical", Syntax: "@RANK(num, range, [order])", Description: "Returns rank of a number in a range (0=desc, 1=asc)", Snippet: "@RANK("},

	// Lookup & Reference
	{Name: "@XLOOKUP", Category: "Lookup/Ref", Syntax: "@XLOOKUP(key, lk_rng, ret_rng, [fallback], [match], [search])", Description: "Modern 2-way exact & approximate lookup with fallback value", Snippet: "@XLOOKUP("},
	{Name: "@VLOOKUP", Category: "Lookup/Ref", Syntax: "@VLOOKUP(key, table_range, col_offset, [exact])", Description: "Searches leftmost column and returns offset column value", Snippet: "@VLOOKUP("},
	{Name: "@HLOOKUP", Category: "Lookup/Ref", Syntax: "@HLOOKUP(key, table_range, row_offset, [exact])", Description: "Searches topmost row and returns offset row value", Snippet: "@HLOOKUP("},
	{Name: "@LOOKUP", Category: "Lookup/Ref", Syntax: "@LOOKUP(val, lookup_vector, [result_vector])", Description: "Looks up value in 1-row or 1-column range", Snippet: "@LOOKUP("},
	{Name: "@INDEX", Category: "Lookup/Ref", Syntax: "@INDEX(range, col_offset, row_offset)", Description: "Returns cell value at intersection coordinate (0-based)", Snippet: "@INDEX("},
	{Name: "@MATCH", Category: "Lookup/Ref", Syntax: "@MATCH(key, lookup_array, [match_type])", Description: "Returns index position of matched item in array (1-based)", Snippet: "@MATCH("},
	{Name: "@XMATCH", Category: "Lookup/Ref", Syntax: "@XMATCH(key, lookup_array, [match_mode], [search_mode])", Description: "Modern position lookup with exact, wildcard, and reverse search", Snippet: "@XMATCH("},
	{Name: "@OFFSET", Category: "Lookup/Ref", Syntax: "@OFFSET(ref, rows, cols, [height], [width])", Description: "Returns reference offset from starting cell/range", Snippet: "@OFFSET("},
	{Name: "@CHOOSE", Category: "Lookup/Ref", Syntax: "@CHOOSE(index, val0, val1, val2...)", Description: "Selects and returns value from list by 0-based index", Snippet: "@CHOOSE("},
	{Name: "@ROW", Category: "Lookup/Ref", Syntax: "@ROW([cell])", Description: "Returns row number of current or referenced cell (1-based)", Snippet: "@ROW("},
	{Name: "@COLUMN", Category: "Lookup/Ref", Syntax: "@COLUMN([cell])", Description: "Returns column number of current or referenced cell (1-based)", Snippet: "@COLUMN("},
	{Name: "@ROWS", Category: "Lookup/Ref", Syntax: "@ROWS(range)", Description: "Returns total number of rows in specified range", Snippet: "@ROWS("},
	{Name: "@COLUMNS", Category: "Lookup/Ref", Syntax: "@COLUMNS(range)", Description: "Returns total number of columns in specified range", Snippet: "@COLUMNS("},
	{Name: "@TRANSPOSE", Category: "Lookup/Ref", Syntax: "@TRANSPOSE(array)", Description: "Transposes rows and columns of an array", Snippet: "@TRANSPOSE("},

	// Logic & Error Handling
	{Name: "@IF", Category: "Logic/Error", Syntax: "@IF(condition, true_val, false_val)", Description: "Conditional three-way branching", Snippet: "@IF("},
	{Name: "@IFS", Category: "Logic/Error", Syntax: "@IFS(cond1, val1, [cond2, val2]...)", Description: "Evaluates multiple conditions in sequence", Snippet: "@IFS("},
	{Name: "@SWITCH", Category: "Logic/Error", Syntax: "@SWITCH(expr, val1, res1, [val2, res2]..., [default])", Description: "Evaluates expression against a list of values", Snippet: "@SWITCH("},
	{Name: "@AND", Category: "Logic/Error", Syntax: "@AND(logical1, [logical2], ...)", Description: "Returns true if all arguments are true", Snippet: "@AND("},
	{Name: "@OR", Category: "Logic/Error", Syntax: "@OR(logical1, [logical2], ...)", Description: "Returns true if any argument is true", Snippet: "@OR("},
	{Name: "@NOT", Category: "Logic/Error", Syntax: "@NOT(logical)", Description: "Reverses the logical value of argument", Snippet: "@NOT("},
	{Name: "@XOR", Category: "Logic/Error", Syntax: "@XOR(logical1, [logical2]...)", Description: "Returns exclusive OR of arguments", Snippet: "@XOR("},
	{Name: "@IFERROR", Category: "Logic/Error", Syntax: "@IFERROR(formula, fallback_val)", Description: "Returns fallback value if formula results in error", Snippet: "@IFERROR("},
	{Name: "@IFNA", Category: "Logic/Error", Syntax: "@IFNA(formula, fallback_val)", Description: "Returns fallback value if formula results in #N/A", Snippet: "@IFNA("},
	{Name: "@ISNUMBER", Category: "Logic/Error", Syntax: "@ISNUMBER(val)", Description: "Tests if value is a numeric number (returns 1 or 0)", Snippet: "@ISNUMBER("},
	{Name: "@ISSTRING", Category: "Logic/Error", Syntax: "@ISSTRING(val)", Description: "Tests if value is a text string (returns 1 or 0)", Snippet: "@ISSTRING("},
	{Name: "@ISTEXT", Category: "Logic/Error", Syntax: "@ISTEXT(val)", Description: "Tests if value is text (returns 1 or 0)", Snippet: "@ISTEXT("},
	{Name: "@ISNONTEXT", Category: "Logic/Error", Syntax: "@ISNONTEXT(val)", Description: "Tests if value is not text (returns 1 or 0)", Snippet: "@ISNONTEXT("},
	{Name: "@ISBLANK", Category: "Logic/Error", Syntax: "@ISBLANK(val)", Description: "Tests if referenced cell is blank/empty", Snippet: "@ISBLANK("},
	{Name: "@ISLOGICAL", Category: "Logic/Error", Syntax: "@ISLOGICAL(val)", Description: "Tests if value is a logical boolean", Snippet: "@ISLOGICAL("},
	{Name: "@ISERR", Category: "Logic/Error", Syntax: "@ISERR(val)", Description: "Tests if value is an error #ERR (returns 1 or 0)", Snippet: "@ISERR("},
	{Name: "@ISNA", Category: "Logic/Error", Syntax: "@ISNA(val)", Description: "Tests if value is #N/A (returns 1 or 0)", Snippet: "@ISNA("},
	{Name: "@ISEVEN", Category: "Logic/Error", Syntax: "@ISEVEN(number)", Description: "Tests if number is even (returns 1 or 0)", Snippet: "@ISEVEN("},
	{Name: "@ISODD", Category: "Logic/Error", Syntax: "@ISODD(number)", Description: "Tests if number is odd (returns 1 or 0)", Snippet: "@ISODD("},
	{Name: "@TRUE", Category: "Logic/Error", Syntax: "@TRUE()", Description: "Returns boolean true", Snippet: "@TRUE()"},
	{Name: "@FALSE", Category: "Logic/Error", Syntax: "@FALSE()", Description: "Returns boolean false", Snippet: "@FALSE()"},
	{Name: "@N", Category: "Logic/Error", Syntax: "@N(value)", Description: "Converts value to a numeric number", Snippet: "@N("},
	{Name: "@T", Category: "Logic/Error", Syntax: "@T(value)", Description: "Returns text string if value is text, empty string otherwise", Snippet: "@T("},
	{Name: "@TYPE", Category: "Logic/Error", Syntax: "@TYPE(value)", Description: "Returns integer code for value data type (1=num, 2=text, etc.)", Snippet: "@TYPE("},

	// Text & String
	{Name: "@TEXT", Category: "Text", Syntax: "@TEXT(value, format_string)", Description: "Formats number or date with custom format string (e.g. \"yyyy/mm/dd\", \"#,##0\")", Snippet: "@TEXT("},
	{Name: "@TRIM", Category: "Text", Syntax: "@TRIM(text)", Description: "Strips leading/trailing spaces and collapses internal spaces", Snippet: "@TRIM("},
	{Name: "@CLEAN", Category: "Text", Syntax: "@CLEAN(text)", Description: "Removes all non-printable characters from text", Snippet: "@CLEAN("},
	{Name: "@SUBSTITUTE", Category: "Text", Syntax: "@SUBSTITUTE(text, old_text, new_text, [instance])", Description: "Replaces occurrences of substring in text", Snippet: "@SUBSTITUTE("},
	{Name: "@REPLACE", Category: "Text", Syntax: "@REPLACE(old_text, start_pos, num_chars, new_text)", Description: "Replaces characters at position within text", Snippet: "@REPLACE("},
	{Name: "@REPT", Category: "Text", Syntax: "@REPT(text, number_times)", Description: "Repeats text a given number of times", Snippet: "@REPT("},
	{Name: "@UPPER", Category: "Text", Syntax: "@UPPER(text)", Description: "Converts all letters in text to UPPERCASE", Snippet: "@UPPER("},
	{Name: "@LOWER", Category: "Text", Syntax: "@LOWER(text)", Description: "Converts all letters in text to lowercase", Snippet: "@LOWER("},
	{Name: "@PROPER", Category: "Text", Syntax: "@PROPER(text)", Description: "Converts text to Title Case (capitalizes each word)", Snippet: "@PROPER("},
	{Name: "@EXACT", Category: "Text", Syntax: "@EXACT(text1, text2)", Description: "Tests if two text values are exactly identical (case-sensitive)", Snippet: "@EXACT("},
	{Name: "@CHAR", Category: "Text", Syntax: "@CHAR(number)", Description: "Returns character specified by ASCII/code number", Snippet: "@CHAR("},
	{Name: "@CODE", Category: "Text", Syntax: "@CODE(text)", Description: "Returns numeric code for the first character in text string", Snippet: "@CODE("},
	{Name: "@UNICHAR", Category: "Text", Syntax: "@UNICHAR(number)", Description: "Returns Unicode character specified by numeric value", Snippet: "@UNICHAR("},
	{Name: "@UNICODE", Category: "Text", Syntax: "@UNICODE(text)", Description: "Returns numeric Unicode codepoint of first character", Snippet: "@UNICODE("},
	{Name: "@CONCATENATE", Category: "Text", Syntax: "@CONCATENATE(text1, text2, ...)", Description: "Joins multiple text strings into a single string", Snippet: "@CONCATENATE("},
	{Name: "@CONCAT", Category: "Text", Syntax: "@CONCAT(text1, text2, ...)", Description: "Concatenates list or range of text items", Snippet: "@CONCAT("},
	{Name: "@TEXTJOIN", Category: "Text", Syntax: "@TEXTJOIN(delimiter, ignore_empty, text1, ...)", Description: "Joins text strings with a custom delimiter and options", Snippet: "@TEXTJOIN("},
	{Name: "@LEFT", Category: "Text", Syntax: "@LEFT(text, num_chars)", Description: "Extracts leftmost characters from text string", Snippet: "@LEFT("},
	{Name: "@RIGHT", Category: "Text", Syntax: "@RIGHT(text, num_chars)", Description: "Extracts rightmost characters from text string", Snippet: "@RIGHT("},
	{Name: "@MID", Category: "Text", Syntax: "@MID(text, start_pos, num_chars)", Description: "Extracts substring from middle of text string", Snippet: "@MID("},
	{Name: "@LEN", Category: "Text", Syntax: "@LEN(text)", Description: "Returns total number of characters in text string", Snippet: "@LEN("},
	{Name: "@FIND", Category: "Text", Syntax: "@FIND(find_text, within_text, [start])", Description: "Case-sensitive search for text position (1-based)", Snippet: "@FIND("},
	{Name: "@SEARCH", Category: "Text", Syntax: "@SEARCH(find_text, within_text, [start])", Description: "Case-insensitive & wildcard (*, ?) text position search", Snippet: "@SEARCH("},
	{Name: "@STRING", Category: "Text", Syntax: "@STRING(number, decimal_places)", Description: "Formats number as string with fixed decimal places", Snippet: "@STRING("},
	{Name: "@VALUE", Category: "Text", Syntax: "@VALUE(text)", Description: "Converts numeric text string (with $, ¥, commas) to number", Snippet: "@VALUE("},
	{Name: "@NUMBERVALUE", Category: "Text", Syntax: "@NUMBERVALUE(text, [dec_sep], [group_sep])", Description: "Parses formatted number text with locale separators", Snippet: "@NUMBERVALUE("},
	{Name: "@TEXTBEFORE", Category: "Text", Syntax: "@TEXTBEFORE(text, delimiter)", Description: "Extracts text occurring before delimiter", Snippet: "@TEXTBEFORE("},
	{Name: "@TEXTAFTER", Category: "Text", Syntax: "@TEXTAFTER(text, delimiter)", Description: "Extracts text occurring after delimiter", Snippet: "@TEXTAFTER("},
	{Name: "@TEXTSPLIT", Category: "Text", Syntax: "@TEXTSPLIT(text, col_delimiter)", Description: "Splits text into array by delimiter", Snippet: "@TEXTSPLIT("},

	// Date & Time
	{Name: "@TODAY", Category: "Date/Time", Syntax: "@TODAY()", Description: "Returns serial number of current date", Snippet: "@TODAY()"},
	{Name: "@NOW", Category: "Date/Time", Syntax: "@NOW()", Description: "Returns serial number of current date and time", Snippet: "@NOW()"},
	{Name: "@DATE", Category: "Date/Time", Syntax: "@DATE(year, month, day)", Description: "Creates date serial number from year, month, and day", Snippet: "@DATE("},
	{Name: "@DATEVALUE", Category: "Date/Time", Syntax: "@DATEVALUE(date_text)", Description: "Converts date text (e.g. \"2026/01/01\", \"2026JANUARY1\") to serial number", Snippet: "@DATEVALUE("},
	{Name: "@TIME", Category: "Date/Time", Syntax: "@TIME(hour, minute, second)", Description: "Creates time decimal fraction (0.0..1.0) from hour, min, sec", Snippet: "@TIME("},
	{Name: "@TIMEVALUE", Category: "Date/Time", Syntax: "@TIMEVALUE(time_text)", Description: "Converts time text (e.g. \"14:30:00\") to time fraction (0.0..1.0)", Snippet: "@TIMEVALUE("},
	{Name: "@DATEDIF", Category: "Date/Time", Syntax: "@DATEDIF(start_date, end_date, unit)", Description: "Calculates difference between two dates (unit: \"Y\", \"M\", \"D\", \"YM\", \"YD\", \"MD\")", Snippet: "@DATEDIF("},
	{Name: "@DAYS", Category: "Date/Time", Syntax: "@DAYS(end_date, start_date)", Description: "Returns number of days between two dates", Snippet: "@DAYS("},
	{Name: "@DAYS360", Category: "Date/Time", Syntax: "@DAYS360(start_date, end_date)", Description: "Calculates difference based on a 360-day year (12 months of 30 days)", Snippet: "@DAYS360("},
	{Name: "@NETWORKDAYS", Category: "Date/Time", Syntax: "@NETWORKDAYS(start_date, end_date, [holidays])", Description: "Returns number of working days between two dates", Snippet: "@NETWORKDAYS("},
	{Name: "@WORKDAY", Category: "Date/Time", Syntax: "@WORKDAY(start_date, days, [holidays])", Description: "Returns date before or after specified number of workdays", Snippet: "@WORKDAY("},
	{Name: "@YEARFRAC", Category: "Date/Time", Syntax: "@YEARFRAC(start_date, end_date)", Description: "Calculates fraction of year represented by number of whole days", Snippet: "@YEARFRAC("},
	{Name: "@YEAR", Category: "Date/Time", Syntax: "@YEAR(serial_date)", Description: "Extracts 4-digit year from date serial", Snippet: "@YEAR("},
	{Name: "@MONTH", Category: "Date/Time", Syntax: "@MONTH(serial_date)", Description: "Extracts month number (1..12) from date serial", Snippet: "@MONTH("},
	{Name: "@DAY", Category: "Date/Time", Syntax: "@DAY(serial_date)", Description: "Extracts day of month (1..31) from date serial", Snippet: "@DAY("},
	{Name: "@HOUR", Category: "Date/Time", Syntax: "@HOUR(time_serial)", Description: "Extracts hour (0..23) from time serial", Snippet: "@HOUR("},
	{Name: "@MINUTE", Category: "Date/Time", Syntax: "@MINUTE(time_serial)", Description: "Extracts minute (0..59) from time serial", Snippet: "@MINUTE("},
	{Name: "@SECOND", Category: "Date/Time", Syntax: "@SECOND(time_serial)", Description: "Extracts second (0..59) from time serial", Snippet: "@SECOND("},
	{Name: "@WEEKDAY", Category: "Date/Time", Syntax: "@WEEKDAY(serial_date, [type])", Description: "Returns weekday number (1=Sun..7=Sat, or 1=Mon..7=Sun)", Snippet: "@WEEKDAY("},
	{Name: "@WEEKNUM", Category: "Date/Time", Syntax: "@WEEKNUM(serial_date)", Description: "Returns week number of the year (1..53)", Snippet: "@WEEKNUM("},
	{Name: "@EDATE", Category: "Date/Time", Syntax: "@EDATE(start_date, months)", Description: "Returns date serial n months before or after start date", Snippet: "@EDATE("},
	{Name: "@EOMONTH", Category: "Date/Time", Syntax: "@EOMONTH(start_date, months)", Description: "Returns last day of month n months before or after start date", Snippet: "@EOMONTH("},

	// Financial
	{Name: "@PMT", Category: "Financial", Syntax: "@PMT(rate, nper, pv, [fv], [type])", Description: "Calculates periodic loan payment with constant interest rate", Snippet: "@PMT("},
	{Name: "@PV", Category: "Financial", Syntax: "@PV(rate, nper, pmt, [fv], [type])", Description: "Calculates present value of an investment/loan", Snippet: "@PV("},
	{Name: "@FV", Category: "Financial", Syntax: "@FV(rate, nper, pmt, [pv], [type])", Description: "Calculates future value of an investment with periodic payments", Snippet: "@FV("},
	{Name: "@NPV", Category: "Financial", Syntax: "@NPV(rate, val1, [val2]...)", Description: "Calculates net present value using discount rate and cash flows", Snippet: "@NPV("},
	{Name: "@IRR", Category: "Financial", Syntax: "@IRR(values, [guess])", Description: "Calculates internal rate of return for a series of cash flows", Snippet: "@IRR("},
	{Name: "@RATE", Category: "Financial", Syntax: "@RATE(nper, pmt, pv, [fv], [type])", Description: "Calculates interest rate per period of an annuity", Snippet: "@RATE("},
	{Name: "@NPER", Category: "Financial", Syntax: "@NPER(rate, pmt, pv, [fv], [type])", Description: "Returns number of periods for an investment/loan", Snippet: "@NPER("},
	{Name: "@SLN", Category: "Financial", Syntax: "@SLN(cost, salvage, life)", Description: "Returns straight-line depreciation of an asset for one period", Snippet: "@SLN("},
	{Name: "@SYD", Category: "Financial", Syntax: "@SYD(cost, salvage, life, per)", Description: "Returns sum-of-years' digits depreciation for specified period", Snippet: "@SYD("},
	{Name: "@DDB", Category: "Financial", Syntax: "@DDB(cost, salvage, life, period, [factor])", Description: "Returns double-declining balance depreciation of an asset", Snippet: "@DDB("},
}

var FuncCategories = []string{"All", "Math/Agg", "Statistical", "Lookup/Ref", "Logic/Error", "Text", "Date/Time", "Financial"}

// ShowFunctionPickerModal displays an interactive function browser dialog
// and returns the function snippet to insert into the cell (e.g. "@XLOOKUP("), or empty string if canceled.
func ShowFunctionPickerModal(s tcell.Screen, styles Styles) string {
	catIdx := 0
	selectedIdx := 0
	scrollOffset := 0
	searchQuery := ""

	for {
		// Filter functions by category and search query
		var filtered []FuncEntry
		currentCat := FuncCategories[catIdx]
		q := strings.ToUpper(strings.TrimSpace(searchQuery))

		for _, fn := range AllFunctions {
			if currentCat != "All" && fn.Category != currentCat {
				continue
			}
			if q != "" {
				nameMatch := strings.Contains(strings.ToUpper(fn.Name), q)
				descMatch := strings.Contains(strings.ToUpper(fn.Description), q)
				if !nameMatch && !descMatch {
					continue
				}
			}
			filtered = append(filtered, fn)
		}

		if selectedIdx >= len(filtered) {
			selectedIdx = len(filtered) - 1
		}
		if selectedIdx < 0 {
			selectedIdx = 0
		}

		// Calculate visible items
		s.Clear()
		w, h := s.Size()
		modalW := 76
		if modalW > w-4 {
			modalW = w - 4
		}
		modalH := 21
		if modalH > h-2 {
			modalH = h - 2
		}
		modalX := (w - modalW) / 2
		modalY := (h - modalH) / 2
		if modalY < 1 {
			modalY = 1
		}

		boxStyle := styles.GraphBorder
		itemStyle := styles.Default
		selStyle := styles.CellCursor
		hdrStyle := styles.Header
		tabActiveStyle := styles.CellCursor
		tabInactiveStyle := styles.Header

		// Draw Modal Box
		for y := modalY; y < modalY+modalH; y++ {
			for x := modalX; x < modalX+modalW; x++ {
				s.SetContent(x, y, ' ', nil, itemStyle)
			}
		}
		for x := modalX; x < modalX+modalW; x++ {
			s.SetContent(x, modalY, '═', nil, boxStyle)
			s.SetContent(x, modalY+modalH-1, '═', nil, boxStyle)
		}
		for y := modalY; y < modalY+modalH; y++ {
			s.SetContent(modalX, y, '║', nil, boxStyle)
			s.SetContent(modalX+modalW-1, y, '║', nil, boxStyle)
		}
		s.SetContent(modalX, modalY, '╔', nil, boxStyle)
		s.SetContent(modalX+modalW-1, modalY, '╗', nil, boxStyle)
		s.SetContent(modalX, modalY+modalH-1, '╚', nil, boxStyle)
		s.SetContent(modalX+modalW-1, modalY+modalH-1, '╝', nil, boxStyle)

		title := " INSERT @FUNCTION "
		drawTextFast(s, modalX+(modalW-len(title))/2, modalY, title, boxStyle, w)

		// Search / Filter row
		searchPrompt := " Search: " + searchQuery + "█"
		drawTextFast(s, modalX+2, modalY+1, searchPrompt, hdrStyle, modalX+modalW-2)

		// Category Tabs row
		tabX := modalX + 2
		for i, catName := range FuncCategories {
			tabLabel := fmt.Sprintf(" %s ", catName)
			st := tabInactiveStyle
			if i == catIdx {
				st = tabActiveStyle
			}
			drawTextFast(s, tabX, modalY+2, tabLabel, st, modalX+modalW-2)
			tabX += runewidth.StringWidth(tabLabel) + 1
		}

		divLine := ""
		for i := 0; i < modalW-4; i++ {
			divLine += "─"
		}
		drawTextFast(s, modalX+2, modalY+3, divLine, boxStyle, modalX+modalW-2)

		// List Area
		listH := modalH - 9
		if listH < 4 {
			listH = 4
		}

		if selectedIdx < scrollOffset {
			scrollOffset = selectedIdx
		}
		if selectedIdx >= scrollOffset+listH {
			scrollOffset = selectedIdx - listH + 1
		}

		for i := 0; i < listH; i++ {
			itemIdx := scrollOffset + i
			rowY := modalY + 4 + i
			if itemIdx < len(filtered) {
				fn := filtered[itemIdx]
				st := itemStyle
				prefix := "  "
				if itemIdx == selectedIdx {
					st = selStyle
					prefix = "> "
				}
				rowText := fmt.Sprintf("%s%-14s %-20s", prefix, fn.Name, fn.Description)
				if runewidth.StringWidth(rowText) > modalW-4 {
					rowText = runewidth.Truncate(rowText, modalW-4, "...")
				}
				// Fill background for row
				for x := modalX + 2; x < modalX+modalW-2; x++ {
					s.SetContent(x, rowY, ' ', nil, st)
				}
				drawTextFast(s, modalX+2, rowY, rowText, st, modalX+modalW-2)
			}
		}

		// Divider above preview
		previewDivY := modalY + 4 + listH
		drawTextFast(s, modalX+2, previewDivY, divLine, boxStyle, modalX+modalW-2)

		// Detail Preview
		if len(filtered) > 0 && selectedIdx < len(filtered) {
			curFn := filtered[selectedIdx]
			syntaxLine := fmt.Sprintf(" Syntax: %s", curFn.Syntax)
			drawTextFast(s, modalX+2, previewDivY+1, syntaxLine, hdrStyle, modalX+modalW-2)
			descLine := fmt.Sprintf(" %s", curFn.Description)
			drawTextFast(s, modalX+2, previewDivY+2, descLine, itemStyle, modalX+modalW-2)
		} else {
			drawTextFast(s, modalX+2, previewDivY+1, " (No matching functions found)", itemStyle, modalX+modalW-2)
		}

		// Footer controls
		footer := " [↑/↓: Select] [Tab/←/→: Category] [Enter: Insert] [ESC: Cancel] "
		drawTextFast(s, modalX+(modalW-runewidth.StringWidth(footer))/2, modalY+modalH-1, footer, styles.Status, w)

		s.Show()

		// Event handling
		ev := s.PollEvent()
		switch tev := ev.(type) {
		case *tcell.EventKey:
			switch tev.Key() {
			case tcell.KeyEscape:
				return ""

			case tcell.KeyEnter:
				if len(filtered) > 0 && selectedIdx < len(filtered) {
					return filtered[selectedIdx].Snippet
				}
				return ""

			case tcell.KeyUp:
				if selectedIdx > 0 {
					selectedIdx--
				}
			case tcell.KeyDown:
				if selectedIdx < len(filtered)-1 {
					selectedIdx++
				}

			case tcell.KeyLeft:
				catIdx = (catIdx - 1 + len(FuncCategories)) % len(FuncCategories)
				selectedIdx = 0
				scrollOffset = 0
			case tcell.KeyRight, tcell.KeyTab:
				catIdx = (catIdx + 1) % len(FuncCategories)
				selectedIdx = 0
				scrollOffset = 0

			case tcell.KeyBackspace, tcell.KeyBackspace2:
				if len(searchQuery) > 0 {
					searchQuery = searchQuery[:len(searchQuery)-1]
					selectedIdx = 0
					scrollOffset = 0
				}

			case tcell.KeyRune:
				searchQuery += string(tev.Rune())
				selectedIdx = 0
				scrollOffset = 0
			}

		case *tcell.EventResize:
			s.Sync()
		}
	}
}
