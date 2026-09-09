package formula

import (
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"hasucalc/cell"
	"hasucalc/coord"
)

type FunctionHandler func(args []any) any

func flatten(args []any) []any {
	var flat []any
	for _, arg := range args {
		if cs, ok := arg.(cellSourced); ok {
			arg = cs.V
		}
		if list, ok := arg.([]any); ok {
			flat = append(flat, flatten(list)...)
		} else if grid, ok := arg.([][]any); ok {
			for _, row := range grid {
				flat = append(flat, flatten(row)...)
			}
		} else {
			// Keep nil so position-aligned ops (SUMIF, SUMPRODUCT, COUNTBLANK)
			// stay aligned with empty cells. Aggregators that ignore blanks
			// (SUM via extractNumbers, COUNTA) already skip nil.
			flat = append(flat, arg)
		}
	}
	return flat
}

// cellSourced marks a value that came from a worksheet cell rather than a formula literal.
type cellSourced struct{ V any }

func unwrapCellSourced(v any) any {
	if cs, ok := v.(cellSourced); ok {
		return cs.V
	}
	return v
}

func firstLotusError(vals ...any) (cell.LotusError, bool) {
	for _, v := range vals {
		v = unwrapCellSourced(v)
		if errVal, ok := v.(cell.LotusError); ok {
			return errVal, true
		}
	}
	return cell.LotusError{}, false
}

func unwrapScalarGrid(v any) any {
	v = unwrapCellSourced(v)
	grid, ok := v.([][]any)
	if !ok || len(grid) == 0 || len(grid[0]) == 0 {
		return v
	}
	// 1x1 range, or implicit intersection of a larger array used as a scalar cell result.
	return unwrapCellSourced(grid[0][0])
}

func extractNumbers(args []any) ([]float64, *cell.LotusError) {
	return collectNumbers(args, true)
}

// collectNumbers coerces literal booleans (SUM(TRUE)=1) but skips booleans
// that come from ranges/grids (Excel: SUM of TRUE cells is 0).
func collectNumbers(args []any, coerceBool bool) ([]float64, *cell.LotusError) {
	var nums []float64
	for _, v := range args {
		if errVal, ok := v.(cell.LotusError); ok {
			return nil, &errVal
		}
		if cs, ok := v.(cellSourced); ok {
			if errVal, ok := cs.V.(cell.LotusError); ok {
				return nil, &errVal
			}
			n, err := collectNumbers([]any{cs.V}, false)
			if err != nil {
				return nil, err
			}
			nums = append(nums, n...)
			continue
		}
		if list, ok := v.([]any); ok {
			n, err := collectNumbers(list, false)
			if err != nil {
				return nil, err
			}
			nums = append(nums, n...)
			continue
		}
		if grid, ok := v.([][]any); ok {
			for _, row := range grid {
				n, err := collectNumbers(row, false)
				if err != nil {
					return nil, err
				}
				nums = append(nums, n...)
			}
			continue
		}
		switch val := v.(type) {
		case float64:
			nums = append(nums, val)
		case int:
			nums = append(nums, float64(val))
		case bool:
			if coerceBool {
				if val {
					nums = append(nums, 1.0)
				} else {
					nums = append(nums, 0.0)
				}
			}
		}
	}
	return nums, nil
}

// Math Functions

func fnSum(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	return sum
}

func fnAvg(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	return sum / float64(len(nums))
}

func fnMin(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return 0.0
	}
	min := nums[0]
	for _, n := range nums[1:] {
		if n < min {
			min = n
		}
	}
	return min
}

func fnMax(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return 0.0
	}
	max := nums[0]
	for _, n := range nums[1:] {
		if n > max {
			max = n
		}
	}
	return max
}

func fnCount(args []any) any {
	flat := flatten(args)
	count := 0
	for _, v := range flat {
		v = unwrapCellSourced(v)
		if v == nil {
			continue
		}
		switch v.(type) {
		case float64, int, int64, int32:
			count++
		}
	}
	return float64(count)
}

func fnRound(args []any) any {
	flat := flatten(args)
	if len(flat) != 2 {
		return cell.ErrLotus
	}
	val, ok1 := toFloat(flat[0])
	digits, ok2 := toFloat(flat[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	d := int(digits)
	shift := math.Pow(10, float64(d))
	res := math.Round(val*shift) / shift
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return cell.ErrLotus
	}
	return res
}

func fnInt(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	val, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	return math.Floor(val)
}

func fnAbs(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	val, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	return math.Abs(val)
}

func fnMod(args []any) any {
	flat := flatten(args)
	if len(flat) != 2 {
		return cell.ErrLotus
	}
	n, ok1 := toFloat(flat[0])
	d, ok2 := toFloat(flat[1])
	if !ok1 || !ok2 || d == 0 {
		return cell.ErrLotus
	}
	// Excel-compliant MOD: n - d * floor(n / d)
	return n - d*math.Floor(n/d)
}

func fnSqrt(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	val, ok := toFloat(flat[0])
	if !ok || val < 0 {
		return cell.ErrLotus
	}
	return math.Sqrt(val)
}

func fnProduct(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	p := 1.0
	for _, n := range nums {
		p *= n
	}
	if math.IsNaN(p) || math.IsInf(p, 0) {
		return cell.ErrLotus
	}
	return p
}

func fnPower(args []any) any {
	flat := flatten(args)
	if len(flat) != 2 {
		return cell.ErrLotus
	}
	b, ok1 := toFloat(flat[0])
	e, ok2 := toFloat(flat[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	if b == 0 && e < 0 {
		return cell.ErrLotus
	}
	if b < 0 && math.Floor(e) != e {
		return cell.ErrLotus
	}
	res := math.Pow(b, e)
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return cell.ErrLotus
	}
	return res
}

func fnExp(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	res := math.Exp(v)
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return cell.ErrLotus
	}
	return res
}

func fnLn(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok || v <= 0 {
		return cell.ErrLotus
	}
	return math.Log(v)
}

func fnLog(args []any) any {
	flat := flatten(args)
	if len(flat) == 1 {
		v, ok := toFloat(flat[0])
		if !ok || v <= 0 {
			return cell.ErrLotus
		}
		return math.Log10(v)
	}
	if len(flat) == 2 {
		v, ok1 := toFloat(flat[0])
		base, ok2 := toFloat(flat[1])
		if !ok1 || !ok2 || v <= 0 || base <= 0 || base == 1 {
			return cell.ErrLotus
		}
		return math.Log(v) / math.Log(base)
	}
	return cell.ErrLotus
}

func fnLog10(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok || v <= 0 {
		return cell.ErrLotus
	}
	return math.Log10(v)
}

func fnQuotient(args []any) any {
	flat := flatten(args)
	if len(flat) != 2 {
		return cell.ErrLotus
	}
	n, ok1 := toFloat(flat[0])
	d, ok2 := toFloat(flat[1])
	if !ok1 || !ok2 || d == 0 {
		return cell.ErrLotus
	}
	return math.Trunc(n / d)
}

func fnSign(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	if v > 0 {
		return 1.0
	} else if v < 0 {
		return -1.0
	}
	return 0.0
}

func fnFact(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok || v < 0 || v > 170 {
		return cell.ErrLotus
	}
	n := int(v)
	res := 1.0
	for i := 2; i <= n; i++ {
		res *= float64(i)
	}
	return res
}

func gcd2(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	if a < 0 {
		return -a
	}
	return a
}

func fnGcd(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	res := int64(math.Abs(nums[0]))
	for _, n := range nums[1:] {
		res = gcd2(res, int64(math.Abs(n)))
	}
	return float64(res)
}

func fnLcm(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	res := int64(math.Abs(nums[0]))
	if res == 0 {
		return 0.0
	}
	for _, n := range nums[1:] {
		cur := int64(math.Abs(n))
		if cur == 0 {
			return 0.0
		}
		res = (res / gcd2(res, cur)) * cur
	}
	return float64(res)
}

func fnCombin(args []any) any {
	flat := flatten(args)
	if len(flat) != 2 {
		return cell.ErrLotus
	}
	nF, ok1 := toFloat(flat[0])
	kF, ok2 := toFloat(flat[1])
	if !ok1 || !ok2 || nF < 0 || kF < 0 || kF > nF {
		return cell.ErrLotus
	}
	n := int(nF)
	k := int(kF)
	if k > n-k {
		k = n - k
	}
	res := 1.0
	for i := 1; i <= k; i++ {
		res = res * float64(n-i+1) / float64(i)
	}
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return cell.ErrLotus
	}
	return math.Round(res)
}

func fnPermut(args []any) any {
	flat := flatten(args)
	if len(flat) != 2 {
		return cell.ErrLotus
	}
	nF, ok1 := toFloat(flat[0])
	kF, ok2 := toFloat(flat[1])
	if !ok1 || !ok2 || nF < 0 || kF < 0 || kF > nF {
		return cell.ErrLotus
	}
	n := int(nF)
	k := int(kF)
	res := 1.0
	for i := 0; i < k; i++ {
		res *= float64(n - i)
	}
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return cell.ErrLotus
	}
	return math.Round(res)
}

func fnRoundUp(args []any) any {
	flat := flatten(args)
	if len(flat) < 1 || len(flat) > 2 {
		return cell.ErrLotus
	}
	val, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	digits := 0
	if len(flat) == 2 {
		if dF, ok := toFloat(flat[1]); ok {
			digits = int(dF)
		}
	}
	shift := math.Pow(10, float64(digits))
	var res float64
	if val >= 0 {
		res = math.Ceil(val*shift) / shift
	} else {
		res = math.Floor(val*shift) / shift
	}
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return cell.ErrLotus
	}
	return res
}

func fnRoundDown(args []any) any {
	flat := flatten(args)
	if len(flat) < 1 || len(flat) > 2 {
		return cell.ErrLotus
	}
	val, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	digits := 0
	if len(flat) == 2 {
		if dF, ok := toFloat(flat[1]); ok {
			digits = int(dF)
		}
	}
	shift := math.Pow(10, float64(digits))
	var res float64
	if val >= 0 {
		res = math.Floor(val*shift) / shift
	} else {
		res = math.Ceil(val*shift) / shift
	}
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return cell.ErrLotus
	}
	return res
}

func fnTrunc(args []any) any {
	return fnRoundDown(args)
}

func fnCeiling(args []any) any {
	flat := flatten(args)
	if len(flat) < 1 || len(flat) > 2 {
		return cell.ErrLotus
	}
	val, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	sig := 1.0
	if len(flat) == 2 {
		sF, ok := toFloat(flat[1])
		if !ok || sF == 0 {
			return cell.ErrLotus
		}
		sig = sF
	}
	if sig < 0 && val > 0 {
		return cell.ErrLotus
	}
	return math.Ceil(val/sig) * sig
}

func fnFloor(args []any) any {
	flat := flatten(args)
	if len(flat) < 1 || len(flat) > 2 {
		return cell.ErrLotus
	}
	val, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	sig := 1.0
	if len(flat) == 2 {
		sF, ok := toFloat(flat[1])
		if !ok || sF == 0 {
			return cell.ErrLotus
		}
		sig = sF
	}
	if sig < 0 && val > 0 {
		return cell.ErrLotus
	}
	return math.Floor(val/sig) * sig
}

func fnMRound(args []any) any {
	flat := flatten(args)
	if len(flat) != 2 {
		return cell.ErrLotus
	}
	val, ok1 := toFloat(flat[0])
	sig, ok2 := toFloat(flat[1])
	if !ok1 || !ok2 || sig == 0 {
		return cell.ErrLotus
	}
	if (val > 0 && sig < 0) || (val < 0 && sig > 0) {
		return cell.ErrLotus
	}
	return math.Round(val/sig) * sig
}

func fnPi(args []any) any {
	return math.Pi
}

func fnDegrees(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	return v * 180.0 / math.Pi
}

func fnRadians(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	return v * math.Pi / 180.0
}

func fnSin(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	return math.Sin(v)
}

func fnCos(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	return math.Cos(v)
}

func fnTan(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	return math.Tan(v)
}

func fnAsin(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok || v < -1 || v > 1 {
		return cell.ErrLotus
	}
	return math.Asin(v)
}

func fnAcos(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok || v < -1 || v > 1 {
		return cell.ErrLotus
	}
	return math.Acos(v)
}

func fnAtan(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	return math.Atan(v)
}

func fnAtan2(args []any) any {
	flat := flatten(args)
	if len(flat) != 2 {
		return cell.ErrLotus
	}
	x, ok1 := toFloat(flat[0])
	y, ok2 := toFloat(flat[1])
	if !ok1 || !ok2 || (x == 0 && y == 0) {
		return cell.ErrLotus
	}
	return math.Atan2(y, x)
}

// Statistical Functions

func fnCountA(args []any) any {
	cnt := 0.0
	for _, v := range flatten(args) {
		v = unwrapCellSourced(v)
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		cnt++
	}
	return cnt
}

func fnCountBlank(args []any) any {
	cnt := 0.0
	for _, v := range flatten(args) {
		v = unwrapCellSourced(v)
		if v == nil {
			cnt++
		} else if s, ok := v.(string); ok && s == "" {
			cnt++
		}
	}
	return cnt
}

func fnMedian(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	sort.Float64s(nums)
	n := len(nums)
	if n%2 == 1 {
		return nums[n/2]
	}
	return (nums[n/2-1] + nums[n/2]) / 2.0
}

func fnMode(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	counts := make(map[float64]int)
	maxCount := 0
	var modeVal float64
	for _, n := range nums {
		counts[n]++
		if counts[n] > maxCount {
			maxCount = counts[n]
			modeVal = n
		}
	}
	if maxCount <= 1 {
		return cell.ErrNA
	}
	return modeVal
}

func fnLarge(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	nums, err := extractNumbers([]any{args[0]})
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	kF, ok := toFloat(args[1])
	if !ok {
		return cell.ErrLotus
	}
	k := int(kF)
	if k < 1 || k > len(nums) {
		return cell.ErrLotus
	}
	sort.Float64s(nums)
	return nums[len(nums)-k]
}

func fnSmall(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	nums, err := extractNumbers([]any{args[0]})
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	kF, ok := toFloat(args[1])
	if !ok {
		return cell.ErrLotus
	}
	k := int(kF)
	if k < 1 || k > len(nums) {
		return cell.ErrLotus
	}
	sort.Float64s(nums)
	return nums[k-1]
}

func fnPercentile(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	nums, err := extractNumbers([]any{args[0]})
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	k, ok := toFloat(args[1])
	if !ok || k < 0 || k > 1 {
		return cell.ErrLotus
	}
	sort.Float64s(nums)
	n := float64(len(nums))
	if k == 0 {
		return nums[0]
	}
	if k == 1 {
		return nums[len(nums)-1]
	}
	idx := k * (n - 1)
	i := int(idx)
	frac := idx - float64(i)
	if i+1 < len(nums) {
		return nums[i] + frac*(nums[i+1]-nums[i])
	}
	return nums[i]
}

func fnQuartile(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	qF, ok := toFloat(args[1])
	if !ok || qF < 0 || qF > 4 {
		return cell.ErrLotus
	}
	q := int(qF)
	switch q {
	case 0:
		return fnMin([]any{args[0]})
	case 1:
		return fnPercentile([]any{args[0], 0.25})
	case 2:
		return fnPercentile([]any{args[0], 0.50})
	case 3:
		return fnPercentile([]any{args[0], 0.75})
	case 4:
		return fnMax([]any{args[0]})
	}
	return cell.ErrLotus
}

func fnStdev(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) < 2 {
		return cell.ErrLotus
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	mean := sum / float64(len(nums))
	sumSq := 0.0
	for _, n := range nums {
		diff := n - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(nums)-1))
}

func fnStdevP(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	mean := sum / float64(len(nums))
	sumSq := 0.0
	for _, n := range nums {
		diff := n - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(nums)))
}

func fnVar(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) < 2 {
		return cell.ErrLotus
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	mean := sum / float64(len(nums))
	sumSq := 0.0
	for _, n := range nums {
		diff := n - mean
		sumSq += diff * diff
	}
	return sumSq / float64(len(nums)-1)
}

func fnVarP(args []any) any {
	nums, err := extractNumbers(args)
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	sum := 0.0
	for _, n := range nums {
		sum += n
	}
	mean := sum / float64(len(nums))
	sumSq := 0.0
	for _, n := range nums {
		diff := n - mean
		sumSq += diff * diff
	}
	return sumSq / float64(len(nums))
}

func fnSumProduct(args []any) any {
	if len(args) < 1 {
		return cell.ErrLotus
	}
	var arrays [][]float64
	var length int
	for i, a := range args {
		flat := flatten([]any{a})
		var cur []float64
		for _, item := range flat {
			if errVal, ok := firstLotusError(item); ok {
				return errVal
			}
			if num, ok := toFloat(item); ok {
				cur = append(cur, num)
			} else {
				cur = append(cur, 0.0)
			}
		}
		if i == 0 {
			length = len(cur)
		} else if len(cur) != length {
			return cell.ErrLotus
		}
		arrays = append(arrays, cur)
	}
	total := 0.0
	for i := 0; i < length; i++ {
		prod := 1.0
		for j := 0; j < len(arrays); j++ {
			prod *= arrays[j][i]
		}
		total += prod
	}
	return total
}

func fnSubtotal(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	fnNumF, ok := toFloat(args[0])
	if !ok {
		return cell.ErrLotus
	}
	fnNum := int(fnNumF)
	subArgs := args[1:]
	switch fnNum {
	case 1, 101:
		return fnAvg(subArgs)
	case 2, 102:
		return fnCount(subArgs)
	case 3, 103:
		return fnCountA(subArgs)
	case 4, 104:
		return fnMax(subArgs)
	case 5, 105:
		return fnMin(subArgs)
	case 6, 106:
		return fnProduct(subArgs)
	case 7, 107:
		return fnStdev(subArgs)
	case 8, 108:
		return fnStdevP(subArgs)
	case 9, 109:
		return fnSum(subArgs)
	case 10, 110:
		return fnVar(subArgs)
	case 11, 111:
		return fnVarP(subArgs)
	default:
		return cell.ErrLotus
	}
}

// Logic Functions

func fnIf(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	cond := isTruthy(args[0])
	if cond {
		return unwrapCellSourced(args[1])
	}
	if len(args) == 3 {
		return unwrapCellSourced(args[2])
	}
	return 0.0
}

func fnIsNumber(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 0.0
	}
	v := unwrapCellSourced(flat[0])
	if _, isErr := v.(cell.LotusError); isErr {
		return 0.0
	}
	switch v.(type) {
	case float64, int, int64, int32:
		return 1.0
	default:
		return 0.0
	}
}

func fnIsString(args []any) any {
	return fnIsText(args)
}

func fnIsErr(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 0.0
	}
	if e, ok := firstLotusError(flat[0]); ok && e.Code != cell.ErrNA.Code {
		return 1.0
	}
	return 0.0
}

func fnIsError(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 0.0
	}
	if _, ok := firstLotusError(flat[0]); ok {
		return 1.0
	}
	return 0.0
}

func fnIsNA(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 0.0
	}
	if e, ok := firstLotusError(flat[0]); ok && e.Code == cell.ErrNA.Code {
		return 1.0
	}
	return 0.0
}

func fnTrue(args []any) any {
	return true
}

func fnFalse(args []any) any {
	return false
}

func fnNA(args []any) any {
	return cell.ErrNA
}

func fnErr(args []any) any {
	return cell.ErrLotus
}

func fnIfs(args []any) any {
	if len(args) < 2 || len(args)%2 != 0 {
		return cell.ErrLotus
	}
	for i := 0; i < len(args); i += 2 {
		if errVal, ok := firstLotusError(args[i]); ok {
			return errVal
		}
		if isTruthy(args[i]) {
			return unwrapCellSourced(args[i+1])
		}
	}
	return cell.ErrNA
}

func fnSwitch(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	target := unwrapCellSourced(args[0])
	if errVal, ok := firstLotusError(target); ok {
		return errVal
	}
	n := len(args)
	for i := 1; i < n-1; i += 2 {
		if errVal, ok := firstLotusError(args[i]); ok {
			return errVal
		}
		if compareEqual(target, args[i]) {
			return unwrapCellSourced(args[i+1])
		}
	}
	if n%2 == 0 {
		return unwrapCellSourced(args[n-1])
	}
	return cell.ErrLotus
}

func fnXor(args []any) any {
	trueCount := 0
	for _, v := range flatten(args) {
		if errVal, ok := firstLotusError(v); ok {
			return errVal
		}
		if isTruthy(v) {
			trueCount++
		}
	}
	if trueCount%2 == 1 {
		return 1.0
	}
	return 0.0
}

func fnIsBlank(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 1.0
	}
	v := unwrapCellSourced(flat[0])
	if _, isErr := v.(cell.LotusError); isErr {
		return 0.0
	}
	if v == nil {
		return 1.0
	}
	return 0.0
}

func fnIsText(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 0.0
	}
	v := unwrapCellSourced(flat[0])
	if _, isErr := v.(cell.LotusError); isErr {
		return 0.0
	}
	if _, ok := v.(string); ok {
		return 1.0
	}
	return 0.0
}

func fnIsNonText(args []any) any {
	if fnIsText(args) == 1.0 {
		return 0.0
	}
	return 1.0
}

func fnIsLogical(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 0.0
	}
	v := unwrapCellSourced(flat[0])
	if _, isErr := v.(cell.LotusError); isErr {
		return 0.0
	}
	// Excel: only true boolean values — not numbers 0/1 or the strings "TRUE"/"FALSE"
	if _, ok := v.(bool); ok {
		return 1.0
	}
	return 0.0
}

func fnIsEven(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	v, ok := toFloat(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	if int64(math.Floor(math.Abs(v)))%2 == 0 {
		return 1.0
	}
	return 0.0
}

func fnIsOdd(args []any) any {
	if isEv := fnIsEven(args); isEv == 1.0 {
		return 0.0
	} else if isEv == 0.0 {
		return 1.0
	}
	return cell.ErrLotus
}

func fnN(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 0.0
	}
	if errVal, ok := firstLotusError(flat[0]); ok {
		return errVal
	}
	v := unwrapCellSourced(flat[0])
	if num, ok := toFloat(v); ok {
		return num
	}
	if b, ok := v.(bool); ok {
		if b {
			return 1.0
		}
		return 0.0
	}
	return 0.0
}

func fnT(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return ""
	}
	if errVal, ok := firstLotusError(flat[0]); ok {
		return errVal
	}
	v := unwrapCellSourced(flat[0])
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func fnType(args []any) any {
	flat := flatten(args)
	if len(flat) == 0 {
		return 1.0
	}
	v := unwrapCellSourced(flat[0])
	if _, ok := v.(cell.LotusError); ok {
		return 16.0
	}
	if _, ok := v.(bool); ok {
		return 4.0
	}
	if _, ok := v.(string); ok {
		return 2.0
	}
	if _, ok := toFloat(v); ok {
		return 1.0
	}
	return 2.0
}

// Lookup Functions

func fnVLookup(args []any) any {
	if len(args) < 3 || len(args) > 4 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(args...); ok {
		return err
	}
	key := args[0]
	grid, ok := args[1].([][]any)
	if !ok {
		grid = [][]any{{unwrapCellSourced(args[1])}}
	} else if len(grid) == 0 || len(grid[0]) == 0 {
		return cell.ErrLotus
	}
	colOff, ok := toFloat(args[2])
	if !ok {
		return cell.ErrLotus
	}
	cIdx := int(colOff)
	numCols := len(grid[0])
	if cIdx < 1 {
		return cell.ErrLotus
	}
	if cIdx > numCols {
		return cell.ErrRef
	}
	cIdx = cIdx - 1

	exactMatch := false
	if len(args) == 4 {
		exactMatch = !isTruthy(args[3]) // 4th arg FALSE or 0 means exact match
	}

	if exactMatch {
		for r := 0; r < len(grid); r++ {
			firstVal := grid[r][0]
			if firstVal == nil {
				continue
			}
			if compareEqual(firstVal, key) {
				res := grid[r][cIdx]
				if res == nil {
					return 0.0
				}
				return res
			}
		}
		return cell.ErrNA
	}

	// Approximate match: skip errors; do not treat them as end-of-sorted-range.
	var lastMatch any
	matched := false
	for r := 0; r < len(grid); r++ {
		firstVal := grid[r][0]
		if firstVal == nil || isLookupError(firstVal) {
			continue
		}
		if compareEqual(firstVal, key) {
			res := grid[r][cIdx]
			if res == nil {
				return 0.0
			}
			return res
		}
		if compareLess(firstVal, key) {
			lastMatch = grid[r][cIdx]
			matched = true
		} else {
			break
		}
	}
	if matched {
		if lastMatch == nil {
			return 0.0
		}
		return lastMatch
	}
	return cell.ErrNA
}

func fnHLookup(args []any) any {
	if len(args) < 3 || len(args) > 4 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(args...); ok {
		return err
	}
	key := args[0]
	grid, ok := args[1].([][]any)
	if !ok {
		grid = [][]any{{unwrapCellSourced(args[1])}}
	} else if len(grid) == 0 || len(grid[0]) == 0 {
		return cell.ErrLotus
	}
	rowOff, ok := toFloat(args[2])
	if !ok {
		return cell.ErrLotus
	}
	rIdx := int(rowOff)
	numRows := len(grid)
	if rIdx < 1 {
		return cell.ErrLotus
	}
	if rIdx > numRows {
		return cell.ErrRef
	}
	rIdx = rIdx - 1

	exactMatch := false
	if len(args) == 4 {
		exactMatch = !isTruthy(args[3])
	}

	if exactMatch {
		for c := 0; c < len(grid[0]); c++ {
			topVal := grid[0][c]
			if topVal == nil {
				continue
			}
			if compareEqual(topVal, key) {
				res := grid[rIdx][c]
				if res == nil {
					return 0.0
				}
				return res
			}
		}
		return cell.ErrNA
	}

	bestCol := -1
	for c := 0; c < len(grid[0]); c++ {
		topVal := grid[0][c]
		if topVal == nil || isLookupError(topVal) {
			continue
		}
		if keyNum, kOk := toFloat(key); kOk {
			if colNum, cOk := toFloat(topVal); cOk {
				if colNum <= keyNum {
					bestCol = c
				} else {
					break
				}
			}
		} else {
			if compareEqual(topVal, key) || compareLess(topVal, key) {
				bestCol = c
			}
		}
	}

	if bestCol == -1 {
		return cell.ErrNA
	}
	res := grid[rIdx][bestCol]
	if res == nil {
		return 0.0
	}
	return res
}

func fnIndex(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(args...); ok {
		return err
	}
	grid, ok := args[0].([][]any)
	if !ok {
		grid = [][]any{{unwrapCellSourced(args[0])}}
	} else if len(grid) == 0 || len(grid[0]) == 0 {
		return cell.ErrLotus
	}
	idxF, ok1 := toFloat(args[1])
	if isOmitted(args[1]) {
		idxF, ok1 = 0, true
	}
	if !ok1 {
		return cell.ErrLotus
	}
	idx := int(idxF)
	numRows := len(grid)
	numCols := len(grid[0])

	rIdx := 0
	cIdx := 0
	if len(args) == 3 {
		cOff, ok2 := toFloat(args[2])
		if isOmitted(args[2]) {
			cOff, ok2 = 0, true
		}
		if !ok2 {
			return cell.ErrLotus
		}
		rNum := idx
		cNum := int(cOff)
		if rNum < 0 || cNum < 0 {
			return cell.ErrLotus
		}
		if rNum == 0 && cNum == 0 {
			return grid
		}
		if rNum == 0 {
			if cNum < 1 || cNum > numCols {
				return cell.ErrRef
			}
			col := make([][]any, numRows)
			for r := 0; r < numRows; r++ {
				col[r] = []any{grid[r][cNum-1]}
			}
			return col
		}
		if cNum == 0 {
			if rNum < 1 || rNum > numRows {
				return cell.ErrRef
			}
			return [][]any{append([]any(nil), grid[rNum-1]...)}
		}
		if rNum < 1 || rNum > numRows || cNum < 1 || cNum > numCols {
			return cell.ErrRef
		}
		rIdx = rNum - 1
		cIdx = cNum - 1
	} else if numCols == 1 {
		if idx < 0 {
			return cell.ErrLotus
		}
		if idx == 0 {
			return grid
		}
		if idx < 1 || idx > numRows {
			return cell.ErrRef
		}
		rIdx = idx - 1
		cIdx = 0
	} else if numRows == 1 {
		if idx < 0 {
			return cell.ErrLotus
		}
		if idx == 0 {
			return grid
		}
		if idx < 1 || idx > numCols {
			return cell.ErrRef
		}
		rIdx = 0
		cIdx = idx - 1
	} else {
		return cell.ErrRef
	}

	res := grid[rIdx][cIdx]
	if res == nil {
		return 0.0
	}
	return res
}

func fnAddress(args []any) any {
	if len(args) < 2 || len(args) > 5 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	rNumF, ok1 := toFloat(args[0])
	cNumF, ok2 := toFloat(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	rowNum := int(rNumF)
	colNum := int(cNumF)
	if rowNum < 1 || colNum < 1 {
		return cell.ErrLotus
	}

	absNum := 1
	if len(args) >= 3 && args[2] != nil {
		if aF, ok := toFloat(args[2]); ok {
			absNum = int(aF)
		}
	}

	colLetter := coord.ColToLetter(colNum - 1)
	var colPart, rowPart string
	switch absNum {
	case 1:
		colPart = "$" + colLetter
		rowPart = fmt.Sprintf("$%d", rowNum)
	case 2:
		colPart = colLetter
		rowPart = fmt.Sprintf("$%d", rowNum)
	case 3:
		colPart = "$" + colLetter
		rowPart = fmt.Sprintf("%d", rowNum)
	case 4:
		colPart = colLetter
		rowPart = fmt.Sprintf("%d", rowNum)
	default:
		colPart = "$" + colLetter
		rowPart = fmt.Sprintf("$%d", rowNum)
	}

	sheetPrefix := ""
	var sName string
	if len(args) == 4 {
		u3 := unwrapCellSourced(args[3])
		if s, ok := u3.(string); ok {
			sUpper := strings.ToUpper(s)
			if sUpper != "TRUE" && sUpper != "FALSE" {
				sName = s
			}
		}
	} else if len(args) >= 5 && args[4] != nil {
		sName = fmt.Sprintf("%v", unwrapCellSourced(args[4]))
	}
	if sName != "" {
		sheetPrefix = coord.QuoteSheetPrefix(sName)
	}

	return fmt.Sprintf("%s%s%s", sheetPrefix, colPart, rowPart)
}

func fnChoose(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(args...); ok {
		return err
	}
	idxF, ok := toFloat(args[0])
	if !ok {
		return cell.ErrLotus
	}
	idx := int(idxF)
	choices := args[1:]
	if idx >= 1 && idx <= len(choices) {
		return unwrapCellSourced(choices[idx-1])
	}
	return cell.ErrLotus
}

func toStringArg(v any) (string, *cell.LotusError) {
	v = unwrapCellSourced(v)
	if v == nil {
		return "", nil
	}
	if errVal, ok := v.(cell.LotusError); ok {
		return "", &errVal
	}
	if b, ok := v.(bool); ok {
		if b {
			return "TRUE", nil
		}
		return "FALSE", nil
	}
	return fmt.Sprintf("%v", v), nil
}

// String Functions

func fnLeft(args []any) any {
	if len(args) < 1 || len(args) > 2 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	n := 1
	if len(args) == 2 {
		if err, ok := firstLotusError(args[1]); ok {
			return err
		}
		nF, ok := toFloat(args[1])
		if !ok {
			return cell.ErrLotus
		}
		n = int(nF)
	}
	if n < 0 {
		return cell.ErrLotus
	}
	if n == 0 {
		return ""
	}
	runes := []rune(str)
	if n > len(runes) {
		n = len(runes)
	}
	return string(runes[:n])
}

func fnRight(args []any) any {
	if len(args) < 1 || len(args) > 2 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	n := 1
	if len(args) == 2 {
		if err, ok := firstLotusError(args[1]); ok {
			return err
		}
		nF, ok := toFloat(args[1])
		if !ok {
			return cell.ErrLotus
		}
		n = int(nF)
	}
	if n < 0 {
		return cell.ErrLotus
	}
	if n == 0 {
		return ""
	}
	runes := []rune(str)
	if n > len(runes) {
		n = len(runes)
	}
	return string(runes[len(runes)-n:])
}

func fnMid(args []any) any {
	if len(args) != 3 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	if err, ok := firstLotusError(args[1], args[2]); ok {
		return err
	}
	startF, ok1 := toFloat(args[1])
	nF, ok2 := toFloat(args[2])
	if !ok1 || !ok2 || startF < 1 || nF < 0 {
		return cell.ErrLotus
	}
	start, n := int(startF)-1, int(nF)
	runes := []rune(str)
	if start >= len(runes) || n == 0 {
		return ""
	}
	end := start + n
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

func fnLength(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	return float64(len([]rune(str)))
}

func fnString(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(args[0], args[1]); ok {
		return err
	}
	val, ok1 := toFloat(args[0])
	dec, ok2 := toFloat(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	d := int(dec)
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%.*f", d, val)
}

func fnValue(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	str = strings.TrimSpace(str)
	if str == "" {
		return 0.0
	}
	str = strings.ReplaceAll(str, "$", "")
	str = strings.ReplaceAll(str, "¥", "")
	str = strings.ReplaceAll(str, ",", "")
	if v, err := strconv.ParseFloat(str, 64); err == nil {
		return v
	}
	return cell.ErrLotus
}

func fnClean(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	var sb strings.Builder
	for _, r := range str {
		if r >= 32 && r != 127 {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func fnReplace(args []any) any {
	if len(args) != 4 {
		return cell.ErrLotus
	}
	oldText, err1 := toStringArg(args[0])
	if err1 != nil {
		return *err1
	}
	if err, ok := firstLotusError(args[1], args[2]); ok {
		return err
	}
	startF, ok1 := toFloat(args[1])
	numF, ok2 := toFloat(args[2])
	newText, err2 := toStringArg(args[3])
	if err2 != nil {
		return *err2
	}
	if !ok1 || !ok2 || startF < 1 || numF < 0 {
		return cell.ErrLotus
	}
	start := int(startF) - 1 // 1-based to 0-based
	num := int(numF)
	runes := []rune(oldText)
	if start > len(runes) {
		start = len(runes)
	}
	end := start + num
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[:start]) + newText + string(runes[end:])
}

func fnRept(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	text, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	if err, ok := firstLotusError(args[1]); ok {
		return err
	}
	timesF, ok := toFloat(args[1])
	if !ok || timesF < 0 {
		return cell.ErrLotus
	}
	times := int(timesF)
	if times < 0 || times > 32767 || times*len(text) > 32767 {
		return cell.ErrLotus
	}
	return strings.Repeat(text, times)
}

func fnExact(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	s1, err1 := toStringArg(args[0])
	if err1 != nil {
		return *err1
	}
	s2, err2 := toStringArg(args[1])
	if err2 != nil {
		return *err2
	}
	if s1 == s2 {
		return 1.0
	}
	return 0.0
}

func fnChar(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(flat[0]); ok {
		return err
	}
	nF, ok := toFloat(flat[0])
	if !ok || nF < 1 || nF > 255 {
		return cell.ErrLotus
	}
	return string(rune(int(nF)))
}

func fnCode(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	s, errVal := toStringArg(flat[0])
	if errVal != nil {
		return *errVal
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return cell.ErrLotus
	}
	return float64(runes[0])
}

func fnUniChar(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(flat[0]); ok {
		return err
	}
	nF, ok := toFloat(flat[0])
	if !ok || nF < 1 || nF > 0x10FFFF {
		return cell.ErrLotus
	}
	return string(rune(int(nF)))
}

func fnUniCode(args []any) any {
	return fnCode(args)
}

func fnNumberValue(args []any) any {
	if len(args) < 1 || len(args) > 3 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	str = strings.TrimSpace(str)
	decSep := "."
	groupSep := ","
	if len(args) >= 2 {
		s, err := toStringArg(args[1])
		if err != nil {
			return *err
		}
		decSep = s
	}
	if len(args) >= 3 {
		s, err := toStringArg(args[2])
		if err != nil {
			return *err
		}
		groupSep = s
	}
	if groupSep != "" {
		str = strings.ReplaceAll(str, groupSep, "")
	}
	if decSep != "." && decSep != "" {
		str = strings.ReplaceAll(str, decSep, ".")
	}
	str = strings.ReplaceAll(str, "$", "")
	str = strings.ReplaceAll(str, "¥", "")
	isPercent := strings.Contains(str, "%")
	str = strings.ReplaceAll(str, "%", "")
	if v, err := strconv.ParseFloat(str, 64); err == nil {
		if isPercent {
			return v / 100.0
		}
		return v
	}
	return cell.ErrLotus
}

func findDelimiterInstances(text, delim string, matchMode int) []int {
	if delim == "" {
		return nil
	}
	var indices []int
	tSearch := text
	dSearch := delim
	if matchMode == 1 {
		tSearch = strings.ToUpper(text)
		dSearch = strings.ToUpper(delim)
	}
	start := 0
	for {
		idx := strings.Index(tSearch[start:], dSearch)
		if idx == -1 {
			break
		}
		actualIdx := start + idx
		indices = append(indices, actualIdx)
		start = actualIdx + len(dSearch)
		if start > len(text) {
			break
		}
	}
	return indices
}

func fnTextBefore(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	text, err1 := toStringArg(args[0])
	if err1 != nil {
		return *err1
	}
	delim, err2 := toStringArg(args[1])
	if err2 != nil {
		return *err2
	}
	if delim == "" {
		return ""
	}
	inst := 1
	if len(args) >= 3 {
		if err, ok := firstLotusError(args[2]); ok {
			return err
		}
		if instF, ok := toFloat(args[2]); ok {
			inst = int(instF)
		}
	}
	if inst == 0 {
		return cell.ErrLotus
	}
	matchMode := 0
	if len(args) >= 4 {
		if err, ok := firstLotusError(args[3]); ok {
			return err
		}
		if mm, ok := toFloat(args[3]); ok {
			matchMode = int(mm)
		}
	}
	var ifNotFound any = cell.ErrNA
	if len(args) >= 6 && args[5] != nil {
		ifNotFound = args[5]
	}

	indices := findDelimiterInstances(text, delim, matchMode)
	var targetIdx int
	if inst > 0 {
		if inst > len(indices) {
			return ifNotFound
		}
		targetIdx = indices[inst-1]
	} else {
		pos := len(indices) + inst
		if pos < 0 || pos >= len(indices) {
			return ifNotFound
		}
		targetIdx = indices[pos]
	}
	return text[:targetIdx]
}

func fnTextAfter(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	text, err1 := toStringArg(args[0])
	if err1 != nil {
		return *err1
	}
	delim, err2 := toStringArg(args[1])
	if err2 != nil {
		return *err2
	}
	if delim == "" {
		return text
	}
	inst := 1
	if len(args) >= 3 {
		if err, ok := firstLotusError(args[2]); ok {
			return err
		}
		if instF, ok := toFloat(args[2]); ok {
			inst = int(instF)
		}
	}
	if inst == 0 {
		return cell.ErrLotus
	}
	matchMode := 0
	if len(args) >= 4 {
		if err, ok := firstLotusError(args[3]); ok {
			return err
		}
		if mm, ok := toFloat(args[3]); ok {
			matchMode = int(mm)
		}
	}
	var ifNotFound any = cell.ErrNA
	if len(args) >= 6 && args[5] != nil {
		ifNotFound = args[5]
	}

	indices := findDelimiterInstances(text, delim, matchMode)
	var targetIdx int
	if inst > 0 {
		if inst > len(indices) {
			return ifNotFound
		}
		targetIdx = indices[inst-1]
	} else {
		pos := len(indices) + inst
		if pos < 0 || pos >= len(indices) {
			return ifNotFound
		}
		targetIdx = indices[pos]
	}
	return text[targetIdx+len(delim):]
}

func fnTextSplit(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	text, err1 := toStringArg(args[0])
	if err1 != nil {
		return *err1
	}
	colDelim, err2 := toStringArg(args[1])
	if err2 != nil {
		return *err2
	}
	parts := strings.Split(text, colDelim)
	var res []any
	for _, p := range parts {
		res = append(res, p)
	}
	return res
}

func fnOr(args []any) any {
	if len(args) == 0 {
		return cell.ErrLotus
	}
	result := false
	hasLogical := false
	for _, a := range flatten(args) {
		if a == nil {
			continue
		}
		if errVal, ok := firstLotusError(a); ok {
			return errVal
		}
		hasLogical = true
		if isTruthy(a) {
			result = true
		}
	}
	if !hasLogical {
		return cell.ErrLotus
	}
	return result
}

func fnAnd(args []any) any {
	if len(args) == 0 {
		return cell.ErrLotus
	}
	result := true
	hasLogical := false
	for _, a := range flatten(args) {
		if a == nil {
			continue
		}
		if errVal, ok := firstLotusError(a); ok {
			return errVal
		}
		hasLogical = true
		if !isTruthy(a) {
			result = false
		}
	}
	if !hasLogical {
		return cell.ErrLotus
	}
	return result
}

func fnNot(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args[0]); ok {
		return errVal
	}
	return !isTruthy(args[0])
}

var monthMap = map[string]time.Month{
	"JANUARY": time.January, "JAN": time.January, "1月": time.January,
	"FEBRUARY": time.February, "FEB": time.February, "2月": time.February,
	"MARCH": time.March, "MAR": time.March, "3月": time.March,
	"APRIL": time.April, "APR": time.April, "4月": time.April,
	"MAY": time.May, "5月": time.May,
	"JUNE": time.June, "JUN": time.June, "6月": time.June,
	"JULY": time.July, "JUL": time.July, "7月": time.July,
	"AUGUST": time.August, "AUG": time.August, "8月": time.August,
	"SEPTEMBER": time.September, "SEP": time.September, "SEPT": time.September, "9月": time.September,
	"OCTOBER": time.October, "OCT": time.October, "10月": time.October,
	"NOVEMBER": time.November, "NOV": time.November, "11月": time.November,
	"DECEMBER": time.December, "DEC": time.December, "12月": time.December,
}

func fnDateValue(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args[0]); ok {
		return errVal
	}
	s, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return cell.ErrLotus
	}

	layouts := []string{
		"2006/01/02", "2006/1/2", "2006-01-02", "2006-1-2",
		"01/02/2006", "1/2/2006", "2006.01.02", "2006年1月2日", "2006年01月02日",
		"January 2, 2006", "Jan 2, 2006", "2-Jan-2006", "02-Jan-2006",
		"2006January2", "2006Jan2",
		"Mon 02 Jan 2006", "Mon 2 Jan 2006", "Mon, 02 Jan 2006", "Mon, 2 Jan 2006",
		"02 Jan 2006", "2 Jan 2006", "02-Jan-06", "2-Jan-06",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return dateToLotusSerial(t)
		}
	}

	u := strings.ToUpper(s)
	for mName, mVal := range monthMap {
		if idx := strings.Index(u, mName); idx != -1 {
			yearPart := strings.TrimSpace(u[:idx])
			dayPart := strings.TrimSpace(u[idx+len(mName):])
			if y, err1 := strconv.Atoi(yearPart); err1 == nil && y >= 1900 && y <= 9999 {
				if d, err2 := strconv.Atoi(dayPart); err2 == nil && d >= 1 && d <= 31 {
					t := time.Date(y, mVal, d, 0, 0, 0, 0, time.UTC)
					return dateToLotusSerial(t)
				}
			}
			if yearPart == "" {
				cleanDayYear := strings.Trim(dayPart, ", /.-")
				var d, y int
				if n, _ := fmt.Sscanf(cleanDayYear, "%d %d", &d, &y); n == 2 {
					t := time.Date(y, mVal, d, 0, 0, 0, 0, time.UTC)
					return dateToLotusSerial(t)
				}
			}
		}
	}

	if num, ok := toFloat(args[0]); ok {
		return num
	}

	return cell.ErrLotus
}

func fnTimeValue(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args[0]); ok {
		return errVal
	}
	s, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	s = strings.TrimSpace(s)
	layouts := []string{
		"15:04:05", "15:04", "3:04:05 PM", "3:04 PM", "3:04:05PM", "3:04PM",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			seconds := t.Hour()*3600 + t.Minute()*60 + t.Second()
			return float64(seconds) / 86400.0
		}
	}
	return cell.ErrLotus
}

func parseDateSerial(v any) (float64, bool) {
	if num, ok := toFloat(v); ok {
		return num, true
	}
	res := fnDateValue([]any{v})
	if num, ok := toFloat(res); ok {
		return num, true
	}
	return 0, false
}

func parseTimeOrDateSerial(v any) (float64, bool) {
	if num, ok := toFloat(v); ok {
		return num, true
	}
	res := fnTimeValue([]any{v})
	if num, ok := toFloat(res); ok {
		return num, true
	}
	res = fnDateValue([]any{v})
	if num, ok := toFloat(res); ok {
		return num, true
	}
	return 0, false
}

// Date Functions

func dateToLotusSerial(t time.Time) float64 {
	base := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	dayT := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	days := int(math.Round(dayT.Sub(base).Hours() / 24))
	if days >= 60 {
		days++
	}
	timeFrac := float64(t.Hour()*3600+t.Minute()*60+t.Second()) / 86400.0
	return float64(days) + timeFrac
}

func lotusSerialToDate(serial float64) time.Time {
	dayInt := int(math.Floor(serial))
	frac := serial - float64(dayInt)
	base := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	if dayInt == 60 {
		// Excel fictional leap day: keep time-of-day on 1900-02-28 as carrier;
		// YEAR/MONTH/DAY special-case serial 60 for the 29th.
		t := time.Date(1900, 2, 28, 0, 0, 0, 0, time.UTC)
		if frac > 0 {
			totalSecs := int(math.Round(frac * 86400.0))
			t = t.Add(time.Duration(totalSecs) * time.Second)
		}
		return t
	}
	if dayInt > 59 {
		dayInt--
	}
	t := base.AddDate(0, 0, dayInt)
	if frac > 0 {
		totalSecs := int(math.Round(frac * 86400.0))
		t = t.Add(time.Duration(totalSecs) * time.Second)
	}
	return t
}

func excelSerialDayParts(serial float64) (year, month, day int, ok bool) {
	dayInt := int(math.Floor(serial))
	if dayInt == 60 {
		return 1900, 2, 29, true
	}
	t := lotusSerialToDate(float64(dayInt))
	return t.Year(), int(t.Month()), t.Day(), true
}

func fnDate(args []any) any {
	if len(args) != 3 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	yF, ok1 := toFloat(args[0])
	mF, ok2 := toFloat(args[1])
	dF, ok3 := toFloat(args[2])
	if !ok1 || !ok2 || !ok3 {
		return cell.ErrLotus
	}
	y, m, d := int(yF), int(mF), int(dF)
	if y < 100 {
		y += 1900
	}
	// Excel's fictional 1900 leap day: DATE(1900,2,29)=60, DATE(1900,2,30)=61.
	if y == 1900 && m == 2 && d >= 29 {
		return float64(59 + (d - 28))
	}
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return dateToLotusSerial(t)
}

func fnNow(args []any) any {
	return dateToLotusSerial(time.Now())
}

func fnToday(args []any) any {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	return dateToLotusSerial(midnight)
}

func fnYear(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	s, ok := parseDateSerial(args[0])
	if !ok {
		return cell.ErrLotus
	}
	y, _, _, ok := excelSerialDayParts(s)
	if !ok {
		return cell.ErrLotus
	}
	return float64(y)
}

func fnMonth(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	s, ok := parseDateSerial(args[0])
	if !ok {
		return cell.ErrLotus
	}
	_, m, _, ok := excelSerialDayParts(s)
	if !ok {
		return cell.ErrLotus
	}
	return float64(m)
}

func fnDay(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	s, ok := parseDateSerial(args[0])
	if !ok {
		return cell.ErrLotus
	}
	_, _, d, ok := excelSerialDayParts(s)
	if !ok {
		return cell.ErrLotus
	}
	return float64(d)
}

func fnWeekday(args []any) any {
	if len(args) < 1 || len(args) > 2 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	serial, ok := parseDateSerial(args[0])
	if !ok {
		return cell.ErrLotus
	}
	t := lotusSerialToDate(serial)
	weekday := int(t.Weekday()) // 0=Sunday, 1=Monday, ..., 6=Saturday
	retType := 1
	if len(args) == 2 {
		if rtF, ok := toFloat(args[1]); ok {
			retType = int(rtF)
		} else {
			return cell.ErrLotus
		}
	}
	switch retType {
	case 1, 17: // 1=Sunday, 7=Saturday
		return float64(weekday + 1)
	case 2, 11: // 1=Monday, 7=Sunday
		if weekday == 0 {
			return 7.0
		}
		return float64(weekday)
	case 3: // 0=Monday, 6=Sunday
		if weekday == 0 {
			return 6.0
		}
		return float64(weekday - 1)
	case 12: // 1=Tuesday, 7=Monday
		return float64((weekday+5)%7 + 1)
	case 13: // 1=Wednesday, 7=Tuesday
		return float64((weekday+4)%7 + 1)
	case 14: // 1=Thursday, 7=Wednesday
		return float64((weekday+3)%7 + 1)
	case 15: // 1=Friday, 7=Thursday
		return float64((weekday+2)%7 + 1)
	case 16: // 1=Saturday, 7=Friday
		return float64((weekday+1)%7 + 1)
	default:
		return cell.ErrLotus
	}
}

func fnEDate(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	serial, ok1 := parseDateSerial(args[0])
	months, ok2 := toFloat(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	t := lotusSerialToDate(serial)
	// Go's AddDate can overflow month (e.g. Jan 31 + 1 month -> Mar 2).
	// Excel specification clips to the last valid day of the target month.
	firstOfTarget := time.Date(t.Year(), t.Month()+time.Month(int(months)), 1, 0, 0, 0, 0, time.UTC)
	firstOfNext := time.Date(firstOfTarget.Year(), firstOfTarget.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	daysInTarget := firstOfNext.AddDate(0, 0, -1).Day()

	d := t.Day()
	if d > daysInTarget {
		d = daysInTarget
	}
	newT := time.Date(firstOfTarget.Year(), firstOfTarget.Month(), d, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
	return dateToLotusSerial(newT)
}

func fnEOMonth(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	serial, ok1 := parseDateSerial(args[0])
	months, ok2 := toFloat(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	t := lotusSerialToDate(serial)
	firstOfNext := time.Date(t.Year(), t.Month()+time.Month(int(months)+1), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstOfNext.AddDate(0, 0, -1)
	return dateToLotusSerial(lastDay)
}

func fnTime(args []any) any {
	if len(args) != 3 {
		return cell.ErrLotus
	}
	hF, ok1 := toFloat(args[0])
	mF, ok2 := toFloat(args[1])
	sF, ok3 := toFloat(args[2])
	if !ok1 || !ok2 || !ok3 {
		return cell.ErrLotus
	}
	if hF < 0 || mF < 0 || sF < 0 || hF > 32767 || mF > 32767 || sF > 32767 {
		return cell.ErrLotus
	}
	sec := hF*3600 + mF*60 + sF
	dayFrac := math.Mod(sec/86400.0, 1.0)
	if dayFrac < 0 {
		dayFrac += 1.0
	}
	return dayFrac
}

func fnHour(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	s, ok := parseTimeOrDateSerial(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	t := lotusSerialToDate(s)
	return float64(t.Hour())
}

func fnMinute(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	s, ok := parseTimeOrDateSerial(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	t := lotusSerialToDate(s)
	return float64(t.Minute())
}

func fnSecond(args []any) any {
	flat := flatten(args)
	if len(flat) != 1 {
		return cell.ErrLotus
	}
	s, ok := parseTimeOrDateSerial(flat[0])
	if !ok {
		return cell.ErrLotus
	}
	t := lotusSerialToDate(s)
	return float64(t.Second())
}

func fnWeekNum(args []any) any {
	if len(args) < 1 || len(args) > 2 {
		return cell.ErrLotus
	}
	serial, ok := parseDateSerial(args[0])
	if !ok {
		return cell.ErrLotus
	}
	retType := 1
	if len(args) == 2 {
		if rF, ok := toFloat(args[1]); ok {
			retType = int(rF)
		} else {
			return cell.ErrLotus
		}
	}
	t := lotusSerialToDate(serial)
	if retType == 21 {
		_, isoW := t.ISOWeek()
		return float64(isoW)
	}

	startWeekday := time.Sunday
	switch retType {
	case 1, 17:
		startWeekday = time.Sunday
	case 2, 11:
		startWeekday = time.Monday
	case 12:
		startWeekday = time.Tuesday
	case 13:
		startWeekday = time.Wednesday
	case 14:
		startWeekday = time.Thursday
	case 15:
		startWeekday = time.Friday
	case 16:
		startWeekday = time.Saturday
	default:
		return cell.ErrLotus
	}

	jan1 := time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	dayOfYear := t.YearDay() - 1
	jan1Offset := (int(jan1.Weekday()) - int(startWeekday) + 7) % 7
	weekNum := (dayOfYear+jan1Offset)/7 + 1
	return float64(weekNum)
}

func fnIsoWeekNum(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	serial, ok := parseDateSerial(args[0])
	if !ok {
		return cell.ErrLotus
	}
	t := lotusSerialToDate(serial)
	_, isoW := t.ISOWeek()
	return float64(isoW)
}

func fnDays(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	d1, ok1 := parseDateSerial(args[0])
	d2, ok2 := parseDateSerial(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	return d1 - d2
}

func fnDays360(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	d1, ok1 := parseDateSerial(args[0])
	d2, ok2 := parseDateSerial(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	european := false
	if len(args) >= 3 {
		european = isTruthy(args[2])
	}
	t1 := lotusSerialToDate(d1)
	t2 := lotusSerialToDate(d2)
	y1, m1, day1 := t1.Year(), t1.Month(), t1.Day()
	y2, m2, day2 := t2.Year(), t2.Month(), t2.Day()
	if european {
		if day1 == 31 {
			day1 = 30
		}
		if day2 == 31 {
			day2 = 30
		}
	} else {
		// US / NASD (Excel DAYS360): last day of month → 30, with the
		// February / end-of-month start-day rule.
		if isLastDayOfMonth(t1) {
			day1 = 30
		}
		if isLastDayOfMonth(t2) {
			if day1 < 30 {
				next := time.Date(t2.Year(), t2.Month()+1, 1, 0, 0, 0, 0, time.UTC)
				y2, m2, day2 = next.Year(), next.Month(), 1
			} else {
				day2 = 30
			}
		}
	}
	return float64((y2-y1)*360 + int(m2-m1)*30 + (day2 - day1))
}

func isLastDayOfMonth(t time.Time) bool {
	return t.AddDate(0, 0, 1).Day() == 1
}

func fnNetworkDays(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	d1, ok1 := parseDateSerial(args[0])
	d2, ok2 := parseDateSerial(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	t1 := lotusSerialToDate(d1)
	t2 := lotusSerialToDate(d2)
	t1 = time.Date(t1.Year(), t1.Month(), t1.Day(), 0, 0, 0, 0, time.UTC)
	t2 = time.Date(t2.Year(), t2.Month(), t2.Day(), 0, 0, 0, 0, time.UTC)
	isRev := false
	if t1.After(t2) {
		t1, t2 = t2, t1
		isRev = true
	}
	holidays := make(map[string]bool)
	if len(args) >= 3 {
		for _, h := range flatten(args[2:]) {
			if hs, hOk := parseDateSerial(h); hOk {
				ht := lotusSerialToDate(hs)
				holidays[ht.Format("2006-01-02")] = true
			}
		}
	}
	count := 0
	for cur := t1; !cur.After(t2); cur = cur.AddDate(0, 0, 1) {
		wd := cur.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			if !holidays[cur.Format("2006-01-02")] {
				count++
			}
		}
	}
	if isRev {
		return float64(-count)
	}
	return float64(count)
}

func fnWorkDay(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	d1, ok1 := parseDateSerial(args[0])
	daysF, ok2 := toFloat(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	t := lotusSerialToDate(d1)
	t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	days := int(daysF)
	step := 1
	if days < 0 {
		step = -1
		days = -days
	}
	holidays := make(map[string]bool)
	if len(args) >= 3 {
		for _, h := range flatten(args[2:]) {
			if hs, hOk := parseDateSerial(h); hOk {
				ht := lotusSerialToDate(hs)
				holidays[ht.Format("2006-01-02")] = true
			}
		}
	}
	added := 0
	cur := t
	for added < days {
		cur = cur.AddDate(0, 0, step)
		wd := cur.Weekday()
		if wd != time.Saturday && wd != time.Sunday && !holidays[cur.Format("2006-01-02")] {
			added++
		}
	}
	return dateToLotusSerial(cur)
}

func fnYearFrac(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	d1, ok1 := parseDateSerial(args[0])
	d2, ok2 := parseDateSerial(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	basis := 0
	if len(args) == 3 {
		if bF, ok := toFloat(args[2]); ok {
			basis = int(bF)
		} else {
			return cell.ErrLotus
		}
	}
	if basis < 0 || basis > 4 {
		return cell.ErrLotus
	}

	diff := math.Abs(d2 - d1)
	switch basis {
	case 1: // Actual/actual
		t1 := lotusSerialToDate(math.Min(d1, d2))
		t2 := lotusSerialToDate(math.Max(d1, d2))
		daysInYear := 365.0
		for y := t1.Year(); y <= t2.Year(); y++ {
			if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
				daysInYear = 366.0
				break
			}
		}
		return diff / daysInYear
	case 2: // Actual/360
		return diff / 360.0
	case 3: // Actual/365
		return diff / 365.0
	default: // 0: US 30/360, 4: European 30/360
		days360Res := fnDays360([]any{math.Min(d1, d2), math.Max(d1, d2), basis == 4})
		if d360, ok := toFloat(days360Res); ok {
			return math.Abs(d360) / 360.0
		}
		return diff / 360.0
	}
}

// Financial Functions

func fnPmt(args []any) any {
	if len(args) < 3 || len(args) > 5 {
		return cell.ErrLotus
	}
	rate, ok1 := toFloat(args[0])
	nper, ok2 := toFloat(args[1])
	pv, ok3 := toFloat(args[2])
	if !ok1 || !ok2 || !ok3 || nper == 0 {
		return cell.ErrLotus
	}
	fv := 0.0
	typ := 0.0
	if len(args) >= 4 {
		if v, ok := toFloat(args[3]); ok {
			fv = v
		}
	}
	if len(args) >= 5 {
		if v, ok := toFloat(args[4]); ok && v != 0 {
			typ = 1.0
		}
	}
	if rate == 0 {
		return -(pv + fv) / nper
	}
	pvif := math.Pow(1+rate, nper)
	return -(rate * (pv*pvif + fv)) / ((1 + rate*typ) * (pvif - 1))
}

func fnPv(args []any) any {
	if len(args) < 3 || len(args) > 5 {
		return cell.ErrLotus
	}
	rate, ok1 := toFloat(args[0])
	nper, ok2 := toFloat(args[1])
	pmt, ok3 := toFloat(args[2])
	if !ok1 || !ok2 || !ok3 {
		return cell.ErrLotus
	}
	fv := 0.0
	typ := 0.0
	if len(args) >= 4 {
		if v, ok := toFloat(args[3]); ok {
			fv = v
		}
	}
	if len(args) >= 5 {
		if v, ok := toFloat(args[4]); ok && v != 0 {
			typ = 1.0
		}
	}
	if rate == 0 {
		return -(fv + pmt*nper)
	}
	pvif := math.Pow(1+rate, nper)
	return ((-fv - pmt*(1+rate*typ)*(pvif-1)/rate) / pvif)
}

func fnFv(args []any) any {
	if len(args) < 3 || len(args) > 5 {
		return cell.ErrLotus
	}
	rate, ok1 := toFloat(args[0])
	nper, ok2 := toFloat(args[1])
	pmt, ok3 := toFloat(args[2])
	if !ok1 || !ok2 || !ok3 {
		return cell.ErrLotus
	}
	pv := 0.0
	typ := 0.0
	if len(args) >= 4 {
		if v, ok := toFloat(args[3]); ok {
			pv = v
		}
	}
	if len(args) >= 5 {
		if v, ok := toFloat(args[4]); ok && v != 0 {
			typ = 1.0
		}
	}
	if rate == 0 {
		return -(pv + pmt*nper)
	}
	pvif := math.Pow(1+rate, nper)
	return -(pv*pvif + pmt*(1+rate*typ)*(pvif-1)/rate)
}

func fnNpv(args []any) any {
	if len(args) < 2 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	rate, ok := toFloat(args[0])
	if !ok || rate <= -1 {
		return cell.ErrLotus
	}
	nums, err := extractNumbers(args[1:])
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrLotus
	}
	total := 0.0
	for i, v := range nums {
		total += v / math.Pow(1+rate, float64(i+1))
	}
	return total
}

func fnIrr(args []any) any {
	if len(args) < 1 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	nums, err := extractNumbers([]any{args[0]})
	if err != nil {
		return *err
	}
	if len(nums) < 2 {
		return cell.ErrLotus
	}
	guess := 0.1
	if len(args) >= 2 {
		if g, ok := toFloat(args[1]); ok {
			guess = g
		}
	}
	rate := guess
	for iter := 0; iter < 100; iter++ {
		if rate <= -1 {
			rate = -0.99
		}
		npv := 0.0
		dnpv := 0.0
		for t, cf := range nums {
			denom := math.Pow(1+rate, float64(t))
			npv += cf / denom
			if t > 0 {
				dnpv -= float64(t) * cf / (denom * (1 + rate))
			}
		}
		if math.Abs(npv) < 1e-7 {
			return rate
		}
		if dnpv == 0 || math.IsNaN(dnpv) || math.IsInf(dnpv, 0) {
			break
		}
		newRate := rate - npv/dnpv
		if math.IsNaN(newRate) || math.IsInf(newRate, 0) {
			break
		}
		if math.Abs(newRate-rate) < 1e-7 {
			return newRate
		}
		rate = newRate
	}
	return cell.ErrLotus
}

func fnRate(args []any) any {
	if len(args) < 3 {
		return cell.ErrLotus
	}
	nper, ok1 := toFloat(args[0])
	pmt, ok2 := toFloat(args[1])
	pv, ok3 := toFloat(args[2])
	if !ok1 || !ok2 || !ok3 || nper <= 0 {
		return cell.ErrLotus
	}
	fv := 0.0
	typ := 0.0
	guess := 0.1
	if len(args) >= 4 {
		if v, ok := toFloat(args[3]); ok {
			fv = v
		}
	}
	if len(args) >= 5 {
		if v, ok := toFloat(args[4]); ok && v != 0 {
			typ = 1.0
		}
	}
	if len(args) >= 6 {
		if v, ok := toFloat(args[5]); ok {
			guess = v
		}
	}
	rate := guess
	for iter := 0; iter < 100; iter++ {
		if rate <= -1 {
			rate = -0.99
		}
		if rate == 0 {
			rate = 0.0001
		}
		pvif := math.Pow(1+rate, nper)
		f := pv*pvif + pmt*(1+rate*typ)*(pvif-1)/rate + fv
		df := nper*pv*math.Pow(1+rate, nper-1) + pmt*typ*(pvif-1)/rate + pmt*(1+rate*typ)*(nper*math.Pow(1+rate, nper-1)*rate-(pvif-1))/(rate*rate)
		if math.Abs(f) < 1e-7 {
			return rate
		}
		if df == 0 || math.IsNaN(df) || math.IsInf(df, 0) {
			break
		}
		newRate := rate - f/df
		if math.IsNaN(newRate) || math.IsInf(newRate, 0) {
			break
		}
		if math.Abs(newRate-rate) < 1e-7 {
			return newRate
		}
		rate = newRate
	}
	return cell.ErrLotus
}

func fnNper(args []any) any {
	if len(args) < 3 || len(args) > 5 {
		return cell.ErrLotus
	}
	rate, ok1 := toFloat(args[0])
	pmt, ok2 := toFloat(args[1])
	pv, ok3 := toFloat(args[2])
	if !ok1 || !ok2 || !ok3 {
		return cell.ErrLotus
	}
	fv := 0.0
	typ := 0.0
	if len(args) >= 4 {
		if v, ok := toFloat(args[3]); ok {
			fv = v
		}
	}
	if len(args) >= 5 {
		if v, ok := toFloat(args[4]); ok && v != 0 {
			typ = 1.0
		}
	}
	if rate == 0 {
		if pmt == 0 {
			return cell.ErrLotus
		}
		return -(pv + fv) / pmt
	}
	num := (pmt*(1+rate*typ) - fv*rate) / (pmt*(1+rate*typ) + pv*rate)
	if num <= 0 {
		return cell.ErrLotus
	}
	return math.Log(num) / math.Log(1+rate)
}

func fnSln(args []any) any {
	if len(args) != 3 {
		return cell.ErrLotus
	}
	cost, ok1 := toFloat(args[0])
	salvage, ok2 := toFloat(args[1])
	life, ok3 := toFloat(args[2])
	if !ok1 || !ok2 || !ok3 || life <= 0 {
		return cell.ErrLotus
	}
	return (cost - salvage) / life
}

func fnSyd(args []any) any {
	if len(args) != 4 {
		return cell.ErrLotus
	}
	cost, ok1 := toFloat(args[0])
	salvage, ok2 := toFloat(args[1])
	life, ok3 := toFloat(args[2])
	per, ok4 := toFloat(args[3])
	if !ok1 || !ok2 || !ok3 || !ok4 || life <= 0 || per < 1 || per > life {
		return cell.ErrLotus
	}
	return (cost - salvage) * (life - per + 1) * 2 / (life * (life + 1))
}

func fnDdb(args []any) any {
	if len(args) < 4 || len(args) > 5 {
		return cell.ErrLotus
	}
	cost, ok1 := toFloat(args[0])
	salvage, ok2 := toFloat(args[1])
	life, ok3 := toFloat(args[2])
	period, ok4 := toFloat(args[3])
	if !ok1 || !ok2 || !ok3 || !ok4 || life <= 0 || period < 1 || period > life || cost < salvage {
		return cell.ErrLotus
	}
	factor := 2.0
	if len(args) == 5 {
		if f, ok := toFloat(args[4]); ok && f > 0 {
			factor = f
		}
	}
	totalDepr := 0.0
	for p := 1.0; p <= period; p++ {
		curBook := cost - totalDepr
		depr := curBook * factor / life
		if curBook-depr < salvage {
			depr = curBook - salvage
		}
		if depr < 0 {
			depr = 0
		}
		if p == period {
			return depr
		}
		totalDepr += depr
	}
	return 0.0
}

func fnLookup(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(args...); ok {
		return err
	}
	lookupVal := args[0]
	var lookupVec, resultVec []any
	if len(args) == 2 {
		lookupVec, resultVec = lookupVectorPair(args[1])
	} else {
		lookupVec = flatten(args[1:2])
		resultVec = flatten(args[2:3])
	}
	bestIdx := -1
	lNum, lIsNum := toFloat(lookupVal)
	for i, item := range lookupVec {
		iNum, iIsNum := toFloat(item)
		if lIsNum && iIsNum {
			if iNum <= lNum {
				bestIdx = i
			} else {
				break
			}
		} else {
			if compareEqual(item, lookupVal) || compareLess(item, lookupVal) {
				bestIdx = i
			}
		}
	}
	if bestIdx != -1 && bestIdx < len(resultVec) {
		res := resultVec[bestIdx]
		if res == nil {
			return 0.0
		}
		return res
	}
	return cell.ErrNA
}

// lookupVectorPair implements 2-arg LOOKUP on a matrix: search the first
// row or column (whichever is longer) and return from the last row/column.
func lookupVectorPair(arg any) (lookupVec, resultVec []any) {
	v := unwrapCellSourced(arg)
	grid, ok := v.([][]any)
	if !ok || len(grid) == 0 || len(grid[0]) == 0 {
		flat := flatten([]any{arg})
		return flat, flat
	}
	nR, nC := len(grid), len(grid[0])
	if nR >= nC {
		for r := 0; r < nR; r++ {
			lookupVec = append(lookupVec, grid[r][0])
			resultVec = append(resultVec, grid[r][nC-1])
		}
		return lookupVec, resultVec
	}
	for c := 0; c < nC; c++ {
		lookupVec = append(lookupVec, grid[0][c])
		resultVec = append(resultVec, grid[nR-1][c])
	}
	return lookupVec, resultVec
}

func fnXMatch(args []any) any {
	if len(args) < 2 || len(args) > 4 {
		return cell.ErrLotus
	}
	if err, ok := firstLotusError(args...); ok {
		return err
	}
	lookupVal := unwrapCellSourced(args[0])
	lookupArray := flatten(args[1:2])
	matchMode := 0
	if len(args) >= 3 {
		if mm, ok := toFloat(args[2]); ok {
			matchMode = int(mm)
		}
	}
	searchMode := 1
	if len(args) >= 4 {
		if sm, ok := toFloat(args[3]); ok {
			searchMode = int(sm)
		}
	}
	var indices []int
	if searchMode == -1 {
		for i := len(lookupArray) - 1; i >= 0; i-- {
			indices = append(indices, i)
		}
	} else {
		for i := 0; i < len(lookupArray); i++ {
			indices = append(indices, i)
		}
	}
	switch matchMode {
	case 0: // Exact match
		for _, i := range indices {
			if compareEqual(lookupArray[i], lookupVal) {
				return float64(i + 1)
			}
		}
	case 1: // Exact match or next larger
		for _, i := range indices {
			if compareEqual(lookupArray[i], lookupVal) {
				return float64(i + 1)
			}
		}
		lNum, lIsNum := toFloat(lookupVal)
		bestIdx := -1
		var bestDiff float64
		for _, i := range indices {
			item := lookupArray[i]
			if lIsNum {
				if iNum, iIsNum := toFloat(item); iIsNum && iNum > lNum {
					diff := iNum - lNum
					if bestIdx == -1 || diff < bestDiff {
						bestDiff = diff
						bestIdx = i
					}
				}
			} else {
				if compareLess(lookupVal, item) {
					if bestIdx == -1 || compareLess(item, lookupArray[bestIdx]) {
						bestIdx = i
					}
				}
			}
		}
		if bestIdx != -1 {
			return float64(bestIdx + 1)
		}
	case -1: // Exact match or next smaller
		for _, i := range indices {
			if compareEqual(lookupArray[i], lookupVal) {
				return float64(i + 1)
			}
		}
		lNum, lIsNum := toFloat(lookupVal)
		bestIdx := -1
		var bestDiff float64
		for _, i := range indices {
			item := lookupArray[i]
			if lIsNum {
				if iNum, iIsNum := toFloat(item); iIsNum && iNum < lNum {
					diff := lNum - iNum
					if bestIdx == -1 || diff < bestDiff {
						bestDiff = diff
						bestIdx = i
					}
				}
			} else {
				if compareLess(item, lookupVal) {
					if bestIdx == -1 || compareLess(lookupArray[bestIdx], item) {
						bestIdx = i
					}
				}
			}
		}
		if bestIdx != -1 {
			return float64(bestIdx + 1)
		}
	case 2: // Wildcard match
		for _, i := range indices {
			if matchCriteria(lookupArray[i], lookupVal) {
				return float64(i + 1)
			}
		}
	}
	return cell.ErrNA
}

func fnTranspose(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args[0]); ok {
		return errVal
	}
	grid, ok := args[0].([][]any)
	if !ok || len(grid) == 0 {
		return unwrapCellSourced(args[0])
	}
	numRows := len(grid)
	numCols := len(grid[0])
	trans := make([][]any, numCols)
	for c := 0; c < numCols; c++ {
		trans[c] = make([]any, numRows)
		for r := 0; r < numRows; r++ {
			trans[c][r] = grid[r][c]
		}
	}
	return trans
}

// Criteria Matching for SUMIF, COUNTIF, AVERAGEIF
func matchCriteria(val any, crit any) bool {
	val = unwrapCellSourced(val)
	crit = unwrapCellSourced(crit)
	if crit == nil {
		return val == nil
	}

	if critErr, critIsErr := asLotusError(crit); critIsErr {
		valErr, valIsErr := asLotusError(val)
		return valIsErr && normalizeErrCode(valErr.Code) == normalizeErrCode(critErr.Code)
	}
	if valErr, valIsErr := asLotusError(val); valIsErr {
		critStr := strings.TrimSpace(fmt.Sprintf("%v", crit))
		op, targetStr := splitCriteriaOp(critStr)
		if code, ok := parseErrorLiteral(targetStr); ok {
			match := normalizeErrCode(valErr.Code) == code
			if op == "<>" {
				return !match
			}
			return match
		}
		if op == "<>" {
			return true
		}
		return false
	}
	critStr := strings.TrimSpace(fmt.Sprintf("%v", crit))
	if critStr == "" {
		return val == nil || fmt.Sprintf("%v", val) == ""
	}

	op := ""
	targetStr := critStr
	if strings.HasPrefix(critStr, ">=") {
		op = ">="
		targetStr = strings.TrimSpace(critStr[2:])
	} else if strings.HasPrefix(critStr, "<=") {
		op = "<="
		targetStr = strings.TrimSpace(critStr[2:])
	} else if strings.HasPrefix(critStr, "<>") {
		op = "<>"
		targetStr = strings.TrimSpace(critStr[2:])
	} else if strings.HasPrefix(critStr, ">") {
		op = ">"
		targetStr = strings.TrimSpace(critStr[1:])
	} else if strings.HasPrefix(critStr, "<") {
		op = "<"
		targetStr = strings.TrimSpace(critStr[1:])
	} else if strings.HasPrefix(critStr, "=") {
		op = "="
		targetStr = strings.TrimSpace(critStr[1:])
	}

	valNum, valIsNum := toFloat(val)
	targetNum, targetIsNum := toFloat(targetStr)

	if op != "" {
		isBlank := val == nil
		if !isBlank {
			if s, ok := val.(string); ok && s == "" {
				isBlank = true
			}
		}
		if isBlank {
			// Excel: blanks match only "=" (empty) / "<>" (non-empty); never ">", "<", etc.
			switch op {
			case "=":
				return targetStr == ""
			case "<>":
				return targetStr != ""
			default:
				return false
			}
		}
		if valIsNum && targetIsNum {
			switch op {
			case ">=":
				return valNum >= targetNum
			case "<=":
				return valNum <= targetNum
			case "<>":
				return valNum != targetNum
			case ">":
				return valNum > targetNum
			case "<":
				return valNum < targetNum
			case "=":
				return valNum == targetNum
			}
		}
		valStr := fmt.Sprintf("%v", val)
		switch op {
		case "=":
			return strings.EqualFold(valStr, targetStr)
		case "<>":
			return !strings.EqualFold(valStr, targetStr)
		case ">":
			return strings.ToUpper(valStr) > strings.ToUpper(targetStr)
		case "<":
			return strings.ToUpper(valStr) < strings.ToUpper(targetStr)
		case ">=":
			return strings.ToUpper(valStr) >= strings.ToUpper(targetStr)
		case "<=":
			return strings.ToUpper(valStr) <= strings.ToUpper(targetStr)
		}
	}

	if val == nil {
		return false
	}
	if valIsNum && targetIsNum {
		return valNum == targetNum
	}
	valStr := fmt.Sprintf("%v", val)
	return matchWildcard(strings.ToUpper(valStr), strings.ToUpper(critStr))
}

func asLotusError(v any) (cell.LotusError, bool) {
	errVal, ok := v.(cell.LotusError)
	return errVal, ok
}

func isLookupError(v any) bool {
	_, ok := asLotusError(unwrapCellSourced(v))
	return ok
}

func splitCriteriaOp(critStr string) (op, target string) {
	critStr = strings.TrimSpace(critStr)
	switch {
	case strings.HasPrefix(critStr, ">="):
		return ">=", strings.TrimSpace(critStr[2:])
	case strings.HasPrefix(critStr, "<="):
		return "<=", strings.TrimSpace(critStr[2:])
	case strings.HasPrefix(critStr, "<>"):
		return "<>", strings.TrimSpace(critStr[2:])
	case strings.HasPrefix(critStr, ">"):
		return ">", strings.TrimSpace(critStr[1:])
	case strings.HasPrefix(critStr, "<"):
		return "<", strings.TrimSpace(critStr[1:])
	case strings.HasPrefix(critStr, "="):
		return "=", strings.TrimSpace(critStr[1:])
	default:
		return "", critStr
	}
}

func parseErrorLiteral(s string) (string, bool) {
	code := normalizeErrCode(s)
	switch code {
	case "NA", "REF", "ERR", "CIRCULAR REF":
		return code, true
	}
	return "", false
}

func normalizeErrCode(code string) string {
	u := strings.ToUpper(strings.TrimSpace(code))
	u = strings.TrimPrefix(u, "#")
	u = strings.TrimSuffix(u, "#")
	u = strings.TrimSuffix(u, "!")
	switch u {
	case "N/A", "NA":
		return "NA"
	case "REF":
		return "REF"
	case "ERR", "ERROR", "VALUE", "DIV/0", "DIV/0!":
		if u == "DIV/0" || u == "DIV/0!" {
			return "ERR"
		}
		if u == "VALUE" {
			return "ERR"
		}
		return "ERR"
	case "CIRCULAR REF", "CIRC":
		return "CIRCULAR REF"
	default:
		return strings.ToUpper(strings.TrimSpace(code))
	}
}

func matchWildcard(s, pattern string) bool {
	pat, lit := unescapeWildcardPattern(pattern)
	hasWild := false
	for i, ch := range pat {
		if !lit[i] && (ch == '*' || ch == '?') {
			hasWild = true
			break
		}
	}
	if !hasWild {
		return s == string(pat)
	}
	sRunes := []rune(s)
	pRunes := pat
	pLen := len(pRunes)
	sLen := len(sRunes)
	pIdx, sIdx := 0, 0
	pStar, sStar := -1, -1

	for sIdx < sLen {
		if pIdx < pLen && !lit[pIdx] && pRunes[pIdx] == '*' {
			pStar = pIdx
			sStar = sIdx
			pIdx++
		} else if pIdx < pLen && (pRunes[pIdx] == sRunes[sIdx] || (!lit[pIdx] && pRunes[pIdx] == '?')) {
			pIdx++
			sIdx++
		} else if pStar != -1 {
			pIdx = pStar + 1
			sStar++
			sIdx = sStar
		} else {
			return false
		}
	}
	for pIdx < pLen && !lit[pIdx] && pRunes[pIdx] == '*' {
		pIdx++
	}
	return pIdx == pLen
}

func unescapeWildcardPattern(pattern string) (pat []rune, lit []bool) {
	pr := []rune(pattern)
	for i := 0; i < len(pr); i++ {
		if pr[i] == '~' && i+1 < len(pr) {
			pat = append(pat, pr[i+1])
			lit = append(lit, true)
			i++
			continue
		}
		pat = append(pat, pr[i])
		lit = append(lit, false)
	}
	return pat, lit
}

func fnSumIf(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	evalCells := flatten(args[0:1])
	criteria := args[1]
	sumCells := evalCells
	if len(args) == 3 {
		sumCells = flatten(args[2:3])
	}

	sum := 0.0
	for i, c := range evalCells {
		if matchCriteria(c, criteria) {
			if i < len(sumCells) {
				if num, ok := toFloat(sumCells[i]); ok {
					sum += num
				}
			}
		}
	}
	return sum
}

func fnCountIf(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	evalCells := flatten(args[0:1])
	criteria := args[1]
	count := 0
	for _, c := range evalCells {
		if matchCriteria(c, criteria) {
			count++
		}
	}
	return float64(count)
}

func fnAverageIf(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	evalCells := flatten(args[0:1])
	criteria := args[1]
	avgCells := evalCells
	if len(args) == 3 {
		avgCells = flatten(args[2:3])
	}

	sum := 0.0
	count := 0
	for i, c := range evalCells {
		if matchCriteria(c, criteria) {
			if i < len(avgCells) {
				if num, ok := toFloat(avgCells[i]); ok {
					sum += num
					count++
				}
			}
		}
	}
	if count == 0 {
		return cell.ErrLotus
	}
	return sum / float64(count)
}

func fnSumIfs(args []any) any {
	if len(args) < 3 || (len(args)-1)%2 != 0 {
		return cell.ErrLotus
	}
	sumCells := flatten([]any{args[0]})
	var critRanges [][]any
	var crits []any
	for i := 1; i < len(args); i += 2 {
		cr := flatten([]any{args[i]})
		if len(cr) != len(sumCells) {
			return cell.ErrLotus
		}
		critRanges = append(critRanges, cr)
		crits = append(crits, args[i+1])
	}
	total := 0.0
	for i, sVal := range sumCells {
		matchAll := true
		for j, cr := range critRanges {
			if !matchCriteria(cr[i], crits[j]) {
				matchAll = false
				break
			}
		}
		if matchAll {
			if num, ok := toFloat(sVal); ok {
				total += num
			}
		}
	}
	return total
}

func fnCountIfs(args []any) any {
	if len(args) < 2 || len(args)%2 != 0 {
		return cell.ErrLotus
	}
	var critRanges [][]any
	var crits []any
	for i := 0; i < len(args); i += 2 {
		critRanges = append(critRanges, flatten([]any{args[i]}))
		crits = append(crits, args[i+1])
	}
	length := len(critRanges[0])
	for _, cr := range critRanges[1:] {
		if len(cr) != length {
			return cell.ErrLotus
		}
	}
	count := 0
	for i := 0; i < length; i++ {
		matchAll := true
		for j, cr := range critRanges {
			if !matchCriteria(cr[i], crits[j]) {
				matchAll = false
				break
			}
		}
		if matchAll {
			count++
		}
	}
	return float64(count)
}

func fnAverageIfs(args []any) any {
	if len(args) < 3 || (len(args)-1)%2 != 0 {
		return cell.ErrLotus
	}
	avgCells := flatten([]any{args[0]})
	var critRanges [][]any
	var crits []any
	for i := 1; i < len(args); i += 2 {
		cr := flatten([]any{args[i]})
		if len(cr) != len(avgCells) {
			return cell.ErrLotus
		}
		critRanges = append(critRanges, cr)
		crits = append(crits, args[i+1])
	}
	total := 0.0
	count := 0
	for i, sVal := range avgCells {
		matchAll := true
		for j, cr := range critRanges {
			if !matchCriteria(cr[i], crits[j]) {
				matchAll = false
				break
			}
		}
		if matchAll {
			if num, ok := toFloat(sVal); ok {
				total += num
				count++
			}
		}
	}
	if count == 0 {
		return cell.ErrLotus
	}
	return total / float64(count)
}

func fnMinIfs(args []any) any {
	if len(args) < 3 || (len(args)-1)%2 != 0 {
		return cell.ErrLotus
	}
	minCells := flatten([]any{args[0]})
	var critRanges [][]any
	var crits []any
	for i := 1; i < len(args); i += 2 {
		cr := flatten([]any{args[i]})
		if len(cr) != len(minCells) {
			return cell.ErrLotus
		}
		critRanges = append(critRanges, cr)
		crits = append(crits, args[i+1])
	}
	var minVal *float64
	for i, sVal := range minCells {
		matchAll := true
		for j, cr := range critRanges {
			if !matchCriteria(cr[i], crits[j]) {
				matchAll = false
				break
			}
		}
		if matchAll {
			if num, ok := toFloat(sVal); ok {
				if minVal == nil || num < *minVal {
					minVal = &num
				}
			}
		}
	}
	if minVal == nil {
		return 0.0
	}
	return *minVal
}

func fnMaxIfs(args []any) any {
	if len(args) < 3 || (len(args)-1)%2 != 0 {
		return cell.ErrLotus
	}
	maxCells := flatten([]any{args[0]})
	var critRanges [][]any
	var crits []any
	for i := 1; i < len(args); i += 2 {
		cr := flatten([]any{args[i]})
		if len(cr) != len(maxCells) {
			return cell.ErrLotus
		}
		critRanges = append(critRanges, cr)
		crits = append(crits, args[i+1])
	}
	var maxVal *float64
	for i, sVal := range maxCells {
		matchAll := true
		for j, cr := range critRanges {
			if !matchCriteria(cr[i], crits[j]) {
				matchAll = false
				break
			}
		}
		if matchAll {
			if num, ok := toFloat(sVal); ok {
				if maxVal == nil || num > *maxVal {
					maxVal = &num
				}
			}
		}
	}
	if maxVal == nil {
		return 0.0
	}
	return *maxVal
}

func fnMatch(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	lookupVal := unwrapCellSourced(args[0])
	lookupArray := flatten(args[1:2])
	matchType := 1
	if len(args) == 3 {
		if mt, ok := toFloat(args[2]); ok {
			matchType = int(mt)
		}
	}

	if matchType == 0 {
		crit := unwrapCellSourced(lookupVal)
		critStr := strings.TrimSpace(fmt.Sprintf("%v", crit))
		useWild := false
		if _, critNum := toFloat(crit); !critNum {
			if s, ok := crit.(string); ok {
				useWild = strings.ContainsAny(s, "*?")
			} else {
				useWild = strings.ContainsAny(critStr, "*?")
			}
		}
		for i, item := range lookupArray {
			if useWild {
				if _, itemNum := toFloat(unwrapCellSourced(item)); itemNum {
					continue
				}
				if matchCriteria(item, crit) {
					return float64(i + 1)
				}
				continue
			}
			if compareEqual(item, lookupVal) {
				return float64(i + 1)
			}
		}
		return cell.ErrNA
	} else if matchType == 1 {
		bestIdx := -1
		lNum, lIsNum := toFloat(lookupVal)
		for i, item := range lookupArray {
			if isLookupError(item) {
				continue
			}
			iNum, iIsNum := toFloat(item)
			if lIsNum && iIsNum {
				if iNum <= lNum {
					bestIdx = i + 1
				} else {
					break
				}
			} else {
				if compareEqual(item, lookupVal) || compareLess(item, lookupVal) {
					bestIdx = i + 1
				} else {
					break
				}
			}
		}
		if bestIdx != -1 {
			return float64(bestIdx)
		}
		return cell.ErrNA
	} else if matchType == -1 {
		bestIdx := -1
		lNum, lIsNum := toFloat(lookupVal)
		for i, item := range lookupArray {
			if isLookupError(item) {
				continue
			}
			iNum, iIsNum := toFloat(item)
			if lIsNum && iIsNum {
				if iNum >= lNum {
					bestIdx = i + 1
				} else {
					break
				}
			} else {
				if compareEqual(item, lookupVal) || compareLess(lookupVal, item) {
					bestIdx = i + 1
				} else {
					break
				}
			}
		}
		if bestIdx != -1 {
			return float64(bestIdx)
		}
		return cell.ErrNA
	}
	return cell.ErrLotus
}

func fnXLookup(args []any) any {
	if len(args) < 3 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args[0]); ok {
		return errVal
	}
	lookupVal := unwrapCellSourced(args[0])
	lookupArray := flatten(args[1:2])

	var ifNotFound any = cell.ErrNA
	if len(args) >= 4 && !isOmitted(args[3]) && args[3] != nil {
		ifNotFound = args[3]
	}

	matchMode := 0
	if len(args) >= 5 {
		if mm, ok := toFloat(args[4]); ok {
			matchMode = int(mm)
		}
	}

	searchMode := 1
	if len(args) >= 6 {
		if sm, ok := toFloat(args[5]); ok {
			searchMode = int(sm)
		}
	}

	n := len(lookupArray)
	indices := make([]int, n)
	if searchMode == -1 {
		for i := 0; i < n; i++ {
			indices[i] = n - 1 - i
		}
	} else {
		for i := 0; i < n; i++ {
			indices[i] = i
		}
	}

	for _, idx := range indices {
		item := lookupArray[idx]
		hit := false
		if matchMode == 2 {
			hit = matchCriteria(item, lookupVal)
		} else {
			hit = compareEqual(item, lookupVal)
		}
		if hit {
			return xlookupPickReturn(args[2], idx, n)
		}
	}

	if matchMode == -1 || matchMode == 1 {
		lNum, lIsNum := toFloat(lookupVal)
		bestIdx := -1
		var bestDiff float64

		for _, idx := range indices {
			item := lookupArray[idx]
			if isLookupError(item) {
				continue
			}
			if lIsNum {
				if iNum, iIsNum := toFloat(item); iIsNum {
					if matchMode == -1 && iNum < lNum {
						diff := lNum - iNum
						if bestIdx == -1 || diff < bestDiff {
							bestDiff = diff
							bestIdx = idx
						}
					} else if matchMode == 1 && iNum > lNum {
						diff := iNum - lNum
						if bestIdx == -1 || diff < bestDiff {
							bestDiff = diff
							bestIdx = idx
						}
					}
				}
			} else if matchMode == -1 {
				if compareLess(item, lookupVal) {
					if bestIdx == -1 || compareLess(lookupArray[bestIdx], item) {
						bestIdx = idx
					}
				}
			} else if matchMode == 1 {
				if compareLess(lookupVal, item) {
					if bestIdx == -1 || compareLess(item, lookupArray[bestIdx]) {
						bestIdx = idx
					}
				}
			}
		}
		if bestIdx != -1 {
			return xlookupPickReturn(args[2], bestIdx, n)
		}
	}

	return ifNotFound
}

func isOmitted(v any) bool {
	return v == nil
}

func xlookupPickReturn(returnArg any, idx, lookupLen int) any {
	v := unwrapCellSourced(returnArg)
	if grid, ok := v.([][]any); ok && len(grid) > 0 && len(grid[0]) > 0 {
		nR, nC := len(grid), len(grid[0])
		if nR == lookupLen && idx >= 0 && idx < nR {
			row := grid[idx]
			if nC == 1 {
				if row[0] == nil {
					return 0.0
				}
				return row[0]
			}
			out := make([]any, nC)
			copy(out, row)
			return [][]any{out}
		}
		if nC == lookupLen && idx >= 0 && idx < nC {
			if nR == 1 {
				if grid[0][idx] == nil {
					return 0.0
				}
				return grid[0][idx]
			}
			col := make([][]any, nR)
			for r := 0; r < nR; r++ {
				col[r] = []any{grid[r][idx]}
			}
			return col
		}
	}
	flat := flatten([]any{returnArg})
	if idx >= 0 && idx < len(flat) {
		if flat[idx] == nil {
			return 0.0
		}
		return flat[idx]
	}
	return 0.0
}

func fnIfError(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	if _, ok := firstLotusError(args[0]); ok {
		return unwrapCellSourced(args[1])
	}
	return unwrapCellSourced(args[0])
}

func fnIfNA(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args[0]); ok && errVal.Code == cell.ErrNA.Code {
		return unwrapCellSourced(args[1])
	}
	return unwrapCellSourced(args[0])
}

func fnTrim(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	words := strings.Fields(strings.TrimSpace(str))
	return strings.Join(words, " ")
}

func fnSubstitute(args []any) any {
	if len(args) < 3 || len(args) > 4 {
		return cell.ErrLotus
	}
	text, err1 := toStringArg(args[0])
	if err1 != nil {
		return *err1
	}
	oldText, err2 := toStringArg(args[1])
	if err2 != nil {
		return *err2
	}
	newText, err3 := toStringArg(args[2])
	if err3 != nil {
		return *err3
	}
	if oldText == "" {
		return text
	}
	if len(args) == 4 {
		if err, ok := firstLotusError(args[3]); ok {
			return err
		}
		instF, ok := toFloat(args[3])
		if !ok || instF <= 0 {
			return cell.ErrLotus
		}
		inst := int(instF)
		parts := strings.Split(text, oldText)
		if inst >= len(parts) {
			return text
		}
		var sb strings.Builder
		for i := 0; i < len(parts); i++ {
			sb.WriteString(parts[i])
			if i < len(parts)-1 {
				if i+1 == inst {
					sb.WriteString(newText)
				} else {
					sb.WriteString(oldText)
				}
			}
		}
		return sb.String()
	}
	return strings.ReplaceAll(text, oldText, newText)
}

func fnUpper(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	return strings.ToUpper(str)
}

func fnLower(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	return strings.ToLower(str)
}

func fnProper(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	str, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	return strings.Title(strings.ToLower(str))
}

func fnConcat(args []any) any {
	flat := flatten(args)
	var sb strings.Builder
	for _, item := range flat {
		s, errVal := toStringArg(item)
		if errVal != nil {
			return *errVal
		}
		sb.WriteString(s)
	}
	return sb.String()
}

func fnTextJoin(args []any) any {
	if len(args) < 3 {
		return cell.ErrLotus
	}
	delim, errVal := toStringArg(args[0])
	if errVal != nil {
		return *errVal
	}
	if err, ok := firstLotusError(args[1]); ok {
		return err
	}
	ignoreEmpty := isTruthy(args[1])
	flat := flatten(args[2:])
	var parts []string
	for _, item := range flat {
		s, errVal := toStringArg(item)
		if errVal != nil {
			return *errVal
		}
		if ignoreEmpty && (item == nil || s == "") {
			continue
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, delim)
}

func fnFind(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	findStr, err1 := toStringArg(args[0])
	if err1 != nil {
		return *err1
	}
	withinStr, err2 := toStringArg(args[1])
	if err2 != nil {
		return *err2
	}
	startPos := 1
	if len(args) == 3 {
		if err, ok := firstLotusError(args[2]); ok {
			return err
		}
		if sF, ok := toFloat(args[2]); ok {
			startPos = int(sF)
		}
	}
	runes := []rune(withinStr)
	if startPos < 1 || startPos > len(runes)+1 {
		return cell.ErrLotus
	}
	sub := string(runes[startPos-1:])
	idx := strings.Index(sub, findStr)
	if idx == -1 {
		return cell.ErrNA
	}
	prefixRunes := []rune(sub[:idx])
	return float64(startPos + len(prefixRunes))
}

func fnSearch(args []any) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	fStr, err1 := toStringArg(args[0])
	if err1 != nil {
		return *err1
	}
	wStr, err2 := toStringArg(args[1])
	if err2 != nil {
		return *err2
	}
	findStr := strings.ToUpper(fStr)
	withinStr := strings.ToUpper(wStr)
	startPos := 1
	if len(args) == 3 {
		if err, ok := firstLotusError(args[2]); ok {
			return err
		}
		if sF, ok := toFloat(args[2]); ok {
			startPos = int(sF)
		}
	}
	runes := []rune(withinStr)
	if startPos < 1 || startPos > len(runes)+1 {
		return cell.ErrLotus
	}
	sub := string(runes[startPos-1:])

	if strings.ContainsAny(findStr, "*?~") {
		var sb strings.Builder
		findRunes := []rune(findStr)
		for i := 0; i < len(findRunes); i++ {
			ch := findRunes[i]
			switch ch {
			case '*':
				sb.WriteString(".*")
			case '?':
				sb.WriteString(".")
			case '~':
				if i+1 < len(findRunes) && (findRunes[i+1] == '*' || findRunes[i+1] == '?' || findRunes[i+1] == '~') {
					i++
					sb.WriteString(regexp.QuoteMeta(string(findRunes[i])))
				} else {
					sb.WriteString(regexp.QuoteMeta("~"))
				}
			default:
				sb.WriteString(regexp.QuoteMeta(string(ch)))
			}
		}
		if re, err := regexp.Compile(sb.String()); err == nil {
			loc := re.FindStringIndex(sub)
			if loc == nil {
				return cell.ErrNA
			}
			prefixRunes := []rune(sub[:loc[0]])
			return float64(startPos + len(prefixRunes))
		}
	}

	idx := strings.Index(sub, findStr)
	if idx == -1 {
		return cell.ErrNA
	}
	prefixRunes := []rune(sub[:idx])
	return float64(startPos + len(prefixRunes))
}

func toFloat(v any) (float64, bool) {
	v = unwrapCellSourced(v)
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case bool:
		if val {
			return 1.0, true
		}
		return 0.0, true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func isTruthy(v any) bool {
	v = unwrapCellSourced(v)
	if f, ok := toFloat(v); ok {
		return f != 0
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if s, ok := v.(string); ok {
		return s != ""
	}
	return false
}

func fnRank(args []any) any {
	return rankImpl(args, false)
}

func fnRankAvg(args []any) any {
	return rankImpl(args, true)
}

func rankImpl(args []any, averageTies bool) any {
	if len(args) < 2 || len(args) > 3 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	targetVal, ok := toFloat(args[0])
	if !ok {
		return cell.ErrLotus
	}
	order := 0 // 0 = descending (largest is 1), 1 = ascending
	if len(args) == 3 {
		if ordNum, ok := toFloat(args[2]); ok && ordNum != 0 {
			order = 1
		}
	}

	nums, err := extractNumbers([]any{args[1]})
	if err != nil {
		return *err
	}
	if len(nums) == 0 {
		return cell.ErrNA
	}

	targetFound := false
	tieCount := 0
	for _, num := range nums {
		if math.Abs(num-targetVal) < 1e-9 {
			targetFound = true
			tieCount++
		}
	}
	if !targetFound {
		return cell.ErrNA
	}

	rank := 1
	for _, num := range nums {
		if order == 0 {
			if num > targetVal+1e-9 {
				rank++
			}
		} else {
			if num < targetVal-1e-9 {
				rank++
			}
		}
	}
	if averageTies && tieCount > 1 {
		return float64(rank) + float64(tieCount-1)/2.0
	}
	return float64(rank)
}

func fnRows(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if _, fromCell := args[0].(cellSourced); fromCell {
		v := unwrapCellSourced(args[0])
		if grid, ok := v.([][]any); ok {
			return float64(len(grid))
		}
		return 1.0
	}
	v := unwrapCellSourced(args[0])
	if errVal, ok := v.(cell.LotusError); ok {
		return errVal
	}
	if grid, ok := v.([][]any); ok {
		return float64(len(grid))
	}
	return 1.0
}

func fnColumns(args []any) any {
	if len(args) != 1 {
		return cell.ErrLotus
	}
	if _, fromCell := args[0].(cellSourced); fromCell {
		v := unwrapCellSourced(args[0])
		if grid, ok := v.([][]any); ok && len(grid) > 0 {
			return float64(len(grid[0]))
		}
		return 1.0
	}
	v := unwrapCellSourced(args[0])
	if errVal, ok := v.(cell.LotusError); ok {
		return errVal
	}
	if grid, ok := v.([][]any); ok && len(grid) > 0 {
		return float64(len(grid[0]))
	}
	return 1.0
}

func fnRowFallback(args []any) any {
	if len(args) == 0 {
		return 1.0
	}
	return 1.0
}

func fnColFallback(args []any) any {
	if len(args) == 0 {
		return 1.0
	}
	return 1.0
}

func fnRand(args []any) any {
	return rand.Float64()
}

func fnRandBetween(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	minVal, ok1 := toFloat(args[0])
	maxVal, ok2 := toFloat(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	minInt := int64(math.Ceil(minVal))
	maxInt := int64(math.Floor(maxVal))
	if minInt > maxInt {
		return cell.ErrLotus
	}
	diff := maxInt - minInt + 1
	if diff <= 0 {
		return cell.ErrLotus
	}
	n := rand.Int63n(diff) + minInt
	return float64(n)
}

func fnDateDif(args []any) any {
	if len(args) != 3 {
		return cell.ErrLotus
	}
	if errVal, ok := firstLotusError(args...); ok {
		return errVal
	}
	d1, ok1 := parseDateSerial(args[0])
	d2, ok2 := parseDateSerial(args[1])
	if !ok1 || !ok2 {
		return cell.ErrLotus
	}
	rawT1 := lotusSerialToDate(d1)
	rawT2 := lotusSerialToDate(d2)
	t1 := time.Date(rawT1.Year(), rawT1.Month(), rawT1.Day(), 0, 0, 0, 0, time.UTC)
	t2 := time.Date(rawT2.Year(), rawT2.Month(), rawT2.Day(), 0, 0, 0, 0, time.UTC)
	if t1.After(t2) {
		return cell.ErrLotus
	}

	unit, errUnit := toStringArg(args[2])
	if errUnit != nil {
		return *errUnit
	}
	unit = strings.ToUpper(strings.TrimSpace(unit))
	switch unit {
	case "Y":
		years := t2.Year() - t1.Year()
		if t2.Month() < t1.Month() || (t2.Month() == t1.Month() && t2.Day() < t1.Day()) {
			years--
		}
		return float64(years)
	case "M":
		months := (t2.Year()-t1.Year())*12 + int(t2.Month()-t1.Month())
		if t2.Day() < t1.Day() {
			months--
		}
		return float64(months)
	case "D":
		days := int(math.Round(t2.Sub(t1).Hours() / 24))
		return float64(days)
	case "YM":
		m1 := int(t1.Month())
		m2 := int(t2.Month())
		months := m2 - m1
		if t2.Day() < t1.Day() {
			months--
		}
		if months < 0 {
			months += 12
		}
		return float64(months)
	case "YD":
		t1ThisYear := time.Date(t2.Year(), t1.Month(), t1.Day(), 0, 0, 0, 0, time.UTC)
		if t1ThisYear.After(t2) {
			t1ThisYear = time.Date(t2.Year()-1, t1.Month(), t1.Day(), 0, 0, 0, 0, time.UTC)
		}
		days := int(math.Round(t2.Sub(t1ThisYear).Hours() / 24))
		return float64(days)
	case "MD":
		d1Day := t1.Day()
		d2Day := t2.Day()
		if d2Day >= d1Day {
			return float64(d2Day - d1Day)
		}
		prevMonth := t2.AddDate(0, -1, 0)
		daysInPrevMonth := time.Date(prevMonth.Year(), prevMonth.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		adjStart := d1Day
		if adjStart > daysInPrevMonth {
			adjStart = daysInPrevMonth
		}
		return float64(daysInPrevMonth - adjStart + d2Day)
	default:
		return cell.ErrLotus
	}
}

var (
	textReYYYY        = regexp.MustCompile(`(?i)yyyy`)
	textReYY          = regexp.MustCompile(`(?i)yy`)
	textReMMMM        = regexp.MustCompile(`(?i)mmmm`)
	textReMMM         = regexp.MustCompile(`(?i)mmm`)
	textReDDDD        = regexp.MustCompile(`(?i)dddd`)
	textReDDD         = regexp.MustCompile(`(?i)ddd`)
	textReAMPM        = regexp.MustCompile(`(?i)am/pm`)
	textReAP          = regexp.MustCompile(`(?i)a/p`)
	textReMinAfterH   = regexp.MustCompile(`(?i)(h{1,2}\s*:\s*)mm`)
	textReMinAfterH1  = regexp.MustCompile(`(?i)(h{1,2}\s*:\s*)m([^m]|$)`)
	textReMinBeforeS  = regexp.MustCompile(`(?i)mm(\s*:\s*s)`)
	textReMinBeforeS1 = regexp.MustCompile(`(?i)(^|[^m])m(\s*:\s*s)`)
	textReMM          = regexp.MustCompile(`(?i)mm`)
	textReM           = regexp.MustCompile(`(?i)m`)
	textReDD          = regexp.MustCompile(`(?i)dd`)
	textReD           = regexp.MustCompile(`(?i)d`)
	textReHH          = regexp.MustCompile(`(?i)hh`)
	textReH           = regexp.MustCompile(`(?i)h`)
	textReSS          = regexp.MustCompile(`(?i)ss`)
	textReS           = regexp.MustCompile(`(?i)s`)
)

func formatTextDate(t time.Time, fmtPattern string) string {
	lower := strings.ToLower(fmtPattern)
	use12 := strings.Contains(lower, "am/pm") || strings.Contains(lower, "a/p")
	hour := t.Hour()
	hDisp := hour
	ampm, ap := "AM", "A"
	if hour >= 12 {
		ampm, ap = "PM", "P"
	}
	if use12 {
		hDisp = hour % 12
		if hDisp == 0 {
			hDisp = 12
		}
	}

	// Word tokens (Mar, Monday, AM) contain letters that later m/d/a
	// replacements would eat; stash them until numeric tokens are done.
	var slots []string
	put := func(val string) string {
		id := fmt.Sprintf("\x00%d\x00", len(slots))
		slots = append(slots, val)
		return id
	}

	res := fmtPattern
	res = textReAMPM.ReplaceAllStringFunc(res, func(string) string { return put(ampm) })
	res = textReAP.ReplaceAllStringFunc(res, func(string) string { return put(ap) })
	res = textReMMMM.ReplaceAllStringFunc(res, func(string) string { return put(t.Month().String()) })
	res = textReMMM.ReplaceAllStringFunc(res, func(string) string { return put(t.Format("Jan")) })
	res = textReDDDD.ReplaceAllStringFunc(res, func(string) string { return put(t.Weekday().String()) })
	res = textReDDD.ReplaceAllStringFunc(res, func(string) string { return put(t.Format("Mon")) })
	res = textReYYYY.ReplaceAllString(res, fmt.Sprintf("%04d", t.Year()))
	res = textReYY.ReplaceAllString(res, fmt.Sprintf("%02d", t.Year()%100))
	res = textReMinAfterH.ReplaceAllString(res, "${1}"+fmt.Sprintf("%02d", t.Minute()))
	res = textReMinAfterH1.ReplaceAllString(res, "${1}"+fmt.Sprintf("%d", t.Minute())+"${2}")
	res = textReMinBeforeS.ReplaceAllString(res, fmt.Sprintf("%02d", t.Minute())+"${1}")
	res = textReMinBeforeS1.ReplaceAllString(res, "${1}"+fmt.Sprintf("%d", t.Minute())+"${2}")
	res = textReMM.ReplaceAllString(res, fmt.Sprintf("%02d", int(t.Month())))
	res = textReM.ReplaceAllString(res, fmt.Sprintf("%d", int(t.Month())))
	res = textReDD.ReplaceAllString(res, fmt.Sprintf("%02d", t.Day()))
	res = textReD.ReplaceAllString(res, fmt.Sprintf("%d", t.Day()))
	res = textReHH.ReplaceAllString(res, fmt.Sprintf("%02d", hDisp))
	res = textReH.ReplaceAllString(res, fmt.Sprintf("%d", hDisp))
	res = textReSS.ReplaceAllString(res, fmt.Sprintf("%02d", t.Second()))
	res = textReS.ReplaceAllString(res, fmt.Sprintf("%d", t.Second()))
	for i := len(slots) - 1; i >= 0; i-- {
		res = strings.ReplaceAll(res, fmt.Sprintf("\x00%d\x00", i), slots[i])
	}
	return res
}

func isDateFormatPattern(pat string) bool {
	lower := strings.ToLower(pat)
	hasDigitPlaceholders := strings.ContainsAny(pat, "0#")
	hasDateTokens := strings.Contains(lower, "yy") || strings.Contains(lower, "yyyy") ||
		strings.Contains(lower, "dd") || strings.Contains(lower, "hh") ||
		strings.Contains(lower, "ss") || strings.Contains(lower, "am/pm") ||
		strings.Contains(lower, "a/p") || strings.Contains(lower, "mmm") ||
		(strings.Contains(lower, "m/") || strings.Contains(lower, "/d") || strings.Contains(lower, "d/m") || strings.Contains(lower, "d-") || strings.Contains(lower, "-m"))
	if hasDigitPlaceholders && !hasDateTokens {
		return false
	}
	return strings.ContainsAny(lower, "ymdhs")
}

func fnText(args []any) any {
	if len(args) != 2 {
		return cell.ErrLotus
	}
	val := unwrapCellSourced(args[0])
	fmtPattern := strings.TrimSpace(fmt.Sprintf("%v", unwrapCellSourced(args[1])))

	if isDateFormatPattern(fmtPattern) {
		serial, ok := parseDateSerial(val)
		if !ok {
			return fmt.Sprintf("%v", val)
		}
		t := lotusSerialToDate(serial)
		res := formatTextDate(t, fmtPattern)
		return res
	}

	num, ok := toFloat(val)
	if !ok {
		return fmt.Sprintf("%v", val)
	}

	firstDigit := strings.IndexAny(fmtPattern, "0#")
	lastDigit := strings.LastIndexAny(fmtPattern, "0#")

	if firstDigit == -1 {
		return fmt.Sprintf("%v", num)
	}

	prefix := fmtPattern[:firstDigit]
	mask := fmtPattern[firstDigit : lastDigit+1]
	suffix := fmtPattern[lastDigit+1:]

	prefix = strings.ReplaceAll(prefix, `"`, "")
	suffix = strings.ReplaceAll(suffix, `"`, "")

	isPercent := strings.Contains(mask, "%") || strings.Contains(suffix, "%")
	if isPercent {
		num = num * 100
		mask = strings.ReplaceAll(mask, "%", "")
	}

	var formattedNum string
	if _, _, sci := scientificMaskParts(mask); sci {
		formattedNum = formatScientificWithMask(num, mask)
	} else if strings.ContainsAny(mask, "0#,") {
		formattedNum = formatNumberWithMask(num, mask)
	} else {
		formattedNum = fmt.Sprintf("%v", num)
	}

	return prefix + formattedNum + suffix
}

func scientificMaskParts(mask string) (sigMask, expMask string, ok bool) {
	u := strings.ToUpper(mask)
	idx := strings.Index(u, "E+")
	if idx < 0 {
		idx = strings.Index(u, "E-")
	}
	if idx < 0 {
		return "", "", false
	}
	return mask[:idx], mask[idx+2:], true
}

func countDigitPlaceholders(s string) int {
	n := 0
	for _, ch := range s {
		if ch == '0' || ch == '#' {
			n++
		}
	}
	return n
}

func formatScientificWithMask(num float64, mask string) string {
	sigMask, expMask, ok := scientificMaskParts(mask)
	if !ok {
		return formatNumberWithMask(num, mask)
	}
	expDigits := countDigitPlaceholders(expMask)
	if expDigits < 1 {
		expDigits = 1
	}
	intWidth := 0
	decWidth := 0
	seenDot := false
	for _, ch := range sigMask {
		switch ch {
		case '.':
			seenDot = true
		case '0', '#':
			if seenDot {
				decWidth++
			} else {
				intWidth++
			}
		}
	}
	if intWidth < 1 {
		intWidth = 1
	}

	sign := ""
	if num < 0 {
		sign = "-"
		num = -num
	}
	var exp int
	var mantissa float64
	if num == 0 {
		exp, mantissa = 0, 0
	} else {
		exp = int(math.Floor(math.Log10(num) + 1e-12))
		mantissa = num / math.Pow(10, float64(exp))
	}
	if intWidth > 1 {
		rem := exp % intWidth
		if rem < 0 {
			rem += intWidth
		}
		mantissa *= math.Pow(10, float64(rem))
		exp -= rem
	}
	pow := math.Pow(10, float64(decWidth))
	mantissa = math.Round(mantissa*pow) / pow
	limit := math.Pow(10, float64(intWidth))
	for mantissa >= limit && mantissa > 0 {
		mantissa /= 10
		exp++
		if intWidth > 1 {
			for exp%intWidth != 0 {
				mantissa *= 10
				exp--
				if mantissa >= limit {
					break
				}
			}
		}
	}
	sigClean := strings.ReplaceAll(sigMask, ",", "")
	mantStr := formatNumberWithMask(mantissa, sigClean)
	expSign := "+"
	expAbs := exp
	if exp < 0 {
		expSign = "-"
		expAbs = -exp
	}
	return sign + mantStr + "E" + expSign + fmt.Sprintf("%0*d", expDigits, expAbs)
}

func formatNumberWithMask(num float64, mask string) string {
	useComma := strings.Contains(mask, ",")
	decimals := 0
	intMask := mask
	if idx := strings.Index(mask, "."); idx != -1 {
		for _, ch := range mask[idx+1:] {
			if ch == '0' || ch == '#' {
				decimals++
			}
		}
		intMask = mask[:idx]
	}
	intWidth := 0
	for _, ch := range intMask {
		if ch == '0' {
			intWidth++
		}
	}
	return formatNumberComma(num, decimals, intWidth, useComma)
}

func formatNumberComma(num float64, decimals, intWidth int, useComma bool) string {
	isNeg := num < 0
	if isNeg {
		num = -num
	}
	formatStr := fmt.Sprintf("%%.%df", decimals)
	formatted := fmt.Sprintf(formatStr, num)
	parts := strings.Split(formatted, ".")
	intPart := parts[0]
	if intWidth > len(intPart) {
		intPart = strings.Repeat("0", intWidth-len(intPart)) + intPart
	}

	resStr := intPart
	if useComma {
		var result []byte
		n := len(intPart)
		for i := 0; i < n; i++ {
			if i > 0 && (n-i)%3 == 0 {
				result = append(result, ',')
			}
			result = append(result, intPart[i])
		}
		resStr = string(result)
	}
	if len(parts) > 1 {
		resStr += "." + parts[1]
	}
	if isNeg {
		resStr = "-" + resStr
	}
	return resStr
}

var Functions = map[string]FunctionHandler{
	// Math & Basic Arithmetic
	"@SUM":         fnSum,
	"@AVG":         fnAvg,
	"@AVERAGE":     fnAvg,
	"@MIN":         fnMin,
	"@MAX":         fnMax,
	"@COUNT":       fnCount,
	"@COUNTA":      fnCountA,
	"@COUNTBLANK":  fnCountBlank,
	"@ROUND":       fnRound,
	"@ROUNDUP":     fnRoundUp,
	"@ROUNDDOWN":   fnRoundDown,
	"@TRUNC":       fnTrunc,
	"@INT":         fnInt,
	"@ABS":         fnAbs,
	"@MOD":         fnMod,
	"@SQRT":        fnSqrt,
	"@PRODUCT":     fnProduct,
	"@MULTIPLY":    fnProduct,
	"@POWER":       fnPower,
	"@EXP":         fnExp,
	"@LN":          fnLn,
	"@LOG":         fnLog,
	"@LOG10":       fnLog10,
	"@QUOTIENT":    fnQuotient,
	"@SIGN":        fnSign,
	"@FACT":        fnFact,
	"@GCD":         fnGcd,
	"@LCM":         fnLcm,
	"@COMBIN":      fnCombin,
	"@PERMUT":      fnPermut,
	"@CEILING":     fnCeiling,
	"@FLOOR":       fnFloor,
	"@MROUND":      fnMRound,
	"@PI":          fnPi,
	"@DEGREES":     fnDegrees,
	"@RADIANS":     fnRadians,
	"@SIN":         fnSin,
	"@COS":         fnCos,
	"@TAN":         fnTan,
	"@ASIN":        fnAsin,
	"@ACOS":        fnAcos,
	"@ATAN":        fnAtan,
	"@ATAN2":       fnAtan2,
	"@SUMPRODUCT":  fnSumProduct,
	"@SUBTOTAL":    fnSubtotal,
	"@RAND":        fnRand,
	"@RANDBETWEEN": fnRandBetween,

	// Conditional Aggregations
	"@SUMIF":      fnSumIf,
	"@SUMIFS":     fnSumIfs,
	"@COUNTIF":    fnCountIf,
	"@COUNTIFS":   fnCountIfs,
	"@AVERAGEIF":  fnAverageIf,
	"@AVERAGEIFS": fnAverageIfs,
	"@MINIFS":     fnMinIfs,
	"@MAXIFS":     fnMaxIfs,

	// Statistical
	"@MEDIAN":     fnMedian,
	"@MODE":       fnMode,
	"@MODE.SNGL":  fnMode,
	"@LARGE":      fnLarge,
	"@SMALL":      fnSmall,
	"@PERCENTILE": fnPercentile,
	"@QUARTILE":   fnQuartile,
	"@STDEV":      fnStdev,
	"@STDEV.S":    fnStdev,
	"@STDEVP":     fnStdevP,
	"@STDEV.P":    fnStdevP,
	"@STD":        fnStdevP,
	"@VAR":        fnVar,
	"@VAR.S":      fnVar,
	"@VARP":       fnVarP,
	"@VAR.P":      fnVarP,
	"@RANK":       fnRank,
	"@RANK.EQ":    fnRank,
	"@RANK.AVG":   fnRankAvg,

	// Logical & Error Handling
	"@IF":        fnIf,
	"@IFS":       fnIfs,
	"@SWITCH":    fnSwitch,
	"@CHOOSE":    fnChoose,
	"@AND":       fnAnd,
	"@OR":        fnOr,
	"@NOT":       fnNot,
	"@XOR":       fnXor,
	"@IFERROR":   fnIfError,
	"@IFNA":      fnIfNA,
	"@ISNUMBER":  fnIsNumber,
	"@ISSTRING":  fnIsString,
	"@ISTEXT":    fnIsText,
	"@ISNONTEXT": fnIsNonText,
	"@ISBLANK":   fnIsBlank,
	"@ISLOGICAL": fnIsLogical,
	"@ISERR":     fnIsErr,
	"@ISERROR":   fnIsError,
	"@ISNA":      fnIsNA,
	"@ISEVEN":    fnIsEven,
	"@ISODD":     fnIsOdd,
	"@TRUE":      fnTrue,
	"@FALSE":     fnFalse,
	"@NA":        fnNA,
	"@ERR":       fnErr,
	"@N":         fnN,
	"@T":         fnT,
	"@TYPE":      fnType,

	// Text & String
	"@TEXT":        fnText,
	"@STRING":      fnString,
	"@VALUE":       fnValue,
	"@NUMBERVALUE": fnNumberValue,
	"@TRIM":        fnTrim,
	"@CLEAN":       fnClean,
	"@SUBSTITUTE":  fnSubstitute,
	"@REPLACE":     fnReplace,
	"@REPT":        fnRept,
	"@REPEAT":      fnRept,
	"@UPPER":       fnUpper,
	"@LOWER":       fnLower,
	"@PROPER":      fnProper,
	"@EXACT":       fnExact,
	"@CHAR":        fnChar,
	"@CODE":        fnCode,
	"@UNICHAR":     fnUniChar,
	"@UNICODE":     fnUniCode,
	"@CONCATENATE": fnConcat,
	"@CONCAT":      fnConcat,
	"@TEXTJOIN":    fnTextJoin,
	"@LEFT":        fnLeft,
	"@RIGHT":       fnRight,
	"@MID":         fnMid,
	"@LENGTH":      fnLength,
	"@LEN":         fnLength,
	"@FIND":        fnFind,
	"@SEARCH":      fnSearch,
	"@TEXTBEFORE":  fnTextBefore,
	"@TEXTAFTER":   fnTextAfter,
	"@TEXTSPLIT":   fnTextSplit,

	// Date & Time
	"@TODAY":       fnToday,
	"@NOW":         fnNow,
	"@DATE":        fnDate,
	"@DATEVALUE":   fnDateValue,
	"@TIME":        fnTime,
	"@TIMEVALUE":   fnTimeValue,
	"@YEAR":        fnYear,
	"@MONTH":       fnMonth,
	"@DAY":         fnDay,
	"@HOUR":        fnHour,
	"@MINUTE":      fnMinute,
	"@SECOND":      fnSecond,
	"@WEEKDAY":     fnWeekday,
	"@WEEKNUM":     fnWeekNum,
	"@ISOWEEKNUM":  fnIsoWeekNum,
	"@EDATE":       fnEDate,
	"@EOMONTH":     fnEOMonth,
	"@DATEDIF":     fnDateDif,
	"@DAYS":        fnDays,
	"@DAYS360":     fnDays360,
	"@NETWORKDAYS": fnNetworkDays,
	"@WORKDAY":     fnWorkDay,
	"@YEARFRAC":    fnYearFrac,

	// Financial
	"@PMT":   fnPmt,
	"@PAYMT": fnPmt,
	"@PV":    fnPv,
	"@FV":    fnFv,
	"@NPV":   fnNpv,
	"@IRR":   fnIrr,
	"@RATE":  fnRate,
	"@NPER":  fnNper,
	"@SLN":   fnSln,
	"@SYD":   fnSyd,
	"@DDB":   fnDdb,

	// Lookup & Reference
	"@XLOOKUP":   fnXLookup,
	"@VLOOKUP":   fnVLookup,
	"@HLOOKUP":   fnHLookup,
	"@LOOKUP":    fnLookup,
	"@INDEX":     fnIndex,
	"@ADDRESS":   fnAddress,
	"@MATCH":     fnMatch,
	"@XMATCH":    fnXMatch,
	"@ROW":       fnRowFallback,
	"@COLUMN":    fnColFallback,
	"@ROWS":      fnRows,
	"@COLUMNS":   fnColumns,
	"@TRANSPOSE": fnTranspose,
}
