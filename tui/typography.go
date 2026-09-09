package tui

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/text/unicode/bidi"
)

type ScriptType int

const (
	ScriptDefault ScriptType = iota
	ScriptCJK
	ScriptArabic
	ScriptDevanagari
	ScriptBengali
	ScriptThai
	ScriptHebrew
)

// DetectScript returns the ScriptType for a given rune.
func DetectScript(r rune) ScriptType {
	switch {
	case (r >= 0x0600 && r <= 0x08FF) || (r >= 0xFB50 && r <= 0xFDFF) || (r >= 0xFE70 && r <= 0xFEFF):
		return ScriptArabic
	case r >= 0x0900 && r <= 0x097F:
		return ScriptDevanagari
	case r >= 0x0980 && r <= 0x09FF:
		return ScriptBengali
	case r >= 0x0E00 && r <= 0x0E7F:
		return ScriptThai
	case (r >= 0x0590 && r <= 0x05FF) || (r >= 0xFB1D && r <= 0xFB4F):
		return ScriptHebrew
	case (r >= 0x3000 && r <= 0x9FFF) || (r >= 0xAC00 && r <= 0xD7AF) || (r >= 0xFF00 && r <= 0xFFEF) || (r >= 0x20000 && r <= 0x2A6DF):
		return ScriptCJK
	default:
		return ScriptDefault
	}
}

// IsRTL returns true if the rune belongs to a right-to-left writing system.
func IsRTL(r rune) bool {
	return (r >= 0x0590 && r <= 0x08FF) || (r >= 0xFB1D && r <= 0xFEFC)
}

type glyphForm struct {
	isolated rune
	final    rune
	initial  rune
	medial   rune
	joining  int // 0: none, 1: right-only, 2: dual
}

var arabicGlyphMap = map[rune]glyphForm{
	0x0621: {0xFE80, 0xFE80, 0xFE80, 0xFE80, 0}, // Hamza
	0x0622: {0xFE81, 0xFE82, 0xFE81, 0xFE82, 1}, // Alef Madda
	0x0623: {0xFE83, 0xFE84, 0xFE83, 0xFE84, 1}, // Alef Hamza Above
	0x0624: {0xFE85, 0xFE86, 0xFE85, 0xFE86, 1}, // Waw Hamza Above
	0x0625: {0xFE87, 0xFE88, 0xFE87, 0xFE88, 1}, // Alef Hamza Below
	0x0626: {0xFE89, 0xFE8A, 0xFE8B, 0xFE8C, 2}, // Yeh Hamza Above
	0x0627: {0xFE8D, 0xFE8E, 0xFE8D, 0xFE8E, 1}, // Alef
	0x0628: {0xFE8F, 0xFE90, 0xFE91, 0xFE92, 2}, // Beh
	0x0629: {0xFE93, 0xFE94, 0xFE93, 0xFE94, 1}, // Teh Marbuta
	0x062A: {0xFE95, 0xFE96, 0xFE97, 0xFE98, 2}, // Teh
	0x062B: {0xFE99, 0xFE9A, 0xFE9B, 0xFE9C, 2}, // Theh
	0x062C: {0xFE9D, 0xFE9E, 0xFE9F, 0xFEA0, 2}, // Jeem
	0x062D: {0xFEA1, 0xFEA2, 0xFEA3, 0xFEA4, 2}, // Hah
	0x062E: {0xFEA5, 0xFEA6, 0xFEA7, 0xFEA8, 2}, // Khah
	0x062F: {0xFEA9, 0xFEAA, 0xFEA9, 0xFEAA, 1}, // Dal
	0x0630: {0xFEAB, 0xFEAC, 0xFEAB, 0xFEAC, 1}, // Thal
	0x0631: {0xFEAD, 0xFEAE, 0xFEAD, 0xFEAE, 1}, // Reh
	0x0632: {0xFEAF, 0xFEB0, 0xFEAF, 0xFEB0, 1}, // Zain
	0x0633: {0xFEB1, 0xFEB2, 0xFEB3, 0xFEB4, 2}, // Seen
	0x0634: {0xFEB5, 0xFEB6, 0xFEB7, 0xFEB8, 2}, // Sheen
	0x0635: {0xFEB9, 0xFEBA, 0xFEBB, 0xFEBC, 2}, // Sad
	0x0636: {0xFEBD, 0xFEBE, 0xFEBF, 0xFEC0, 2}, // Dad
	0x0637: {0xFEC1, 0xFEC2, 0xFEC3, 0xFEC4, 2}, // Tah
	0x0638: {0xFEC5, 0xFEC6, 0xFEC7, 0xFEC8, 2}, // Zah
	0x0639: {0xFEC9, 0xFECA, 0xFECB, 0xFECC, 2}, // Ain
	0x063A: {0xFECD, 0xFECE, 0xFECF, 0xFED0, 2}, // Ghain
	0x0640: {0x0640, 0x0640, 0x0640, 0x0640, 2}, // Tatweel
	0x0641: {0xFED1, 0xFED2, 0xFED3, 0xFED4, 2}, // Feh
	0x0642: {0xFED5, 0xFED6, 0xFED7, 0xFED8, 2}, // Qaf
	0x0643: {0xFED9, 0xFEDA, 0xFEDB, 0xFEDC, 2}, // Kaf
	0x0644: {0xFEDD, 0xFEDE, 0xFEDF, 0xFEE0, 2}, // Lam
	0x0645: {0xFEE1, 0xFEE2, 0xFEE3, 0xFEE4, 2}, // Meem
	0x0646: {0xFEE5, 0xFEE6, 0xFEE7, 0xFEE8, 2}, // Noon
	0x0647: {0xFEE9, 0xFEEA, 0xFEEB, 0xFEEC, 2}, // Heh
	0x0648: {0xFEED, 0xFEEE, 0xFEED, 0xFEEE, 1}, // Waw
	0x0649: {0xFEEF, 0xFEF0, 0xFEEF, 0xFEF0, 1}, // Alef Maksura
	0x064A: {0xFEF1, 0xFEF2, 0xFEF3, 0xFEF4, 2}, // Yeh
	0x067E: {0xFB56, 0xFB57, 0xFB58, 0xFB59, 2}, // Peh (Persian/Urdu)
	0x0686: {0xFB7A, 0xFB7B, 0xFB7C, 0xFB7D, 2}, // Tcheh (Persian/Urdu)
	0x06AF: {0xFB92, 0xFB93, 0xFB94, 0xFB95, 2}, // Gaf (Persian/Urdu)
}

// ReshapeArabic performs contextual glyph shaping for Arabic, Urdu, and Persian text.
func ReshapeArabic(text string) string {
	runes := []rune(text)
	n := len(runes)
	if n == 0 {
		return text
	}

	// 1. Process Lam-Alif ligatures
	var merged []rune
	for i := 0; i < n; i++ {
		r := runes[i]
		if r == 0x0644 && i+1 < n { // Lam
			next := runes[i+1]
			var lig rune
			switch next {
			case 0x0622:
				lig = 0xFEF5 // Lam + Alef Madda
			case 0x0623:
				lig = 0xFEF7 // Lam + Alef Hamza Above
			case 0x0625:
				lig = 0xFEF9 // Lam + Alef Hamza Below
			case 0x0627:
				lig = 0xFEFB // Lam + Alef
			}
			if lig != 0 {
				merged = append(merged, lig)
				i++
				continue
			}
		}
		merged = append(merged, r)
	}
	runes = merged
	n = len(runes)

	var res []rune
	for i := 0; i < n; i++ {
		r := runes[i]
		gf, ok := arabicGlyphMap[r]
		if !ok {
			if r >= 0xFEF5 && r <= 0xFEFB {
				prevConnect := false
				if i > 0 {
					if pg, pok := arabicGlyphMap[runes[i-1]]; pok && pg.joining == 2 {
						prevConnect = true
					}
				}
				if prevConnect {
					res = append(res, r+1) // Final form
				} else {
					res = append(res, r) // Isolated form
				}
				continue
			}
			res = append(res, r)
			continue
		}

		prevConnect := false
		if i > 0 {
			if pg, pok := arabicGlyphMap[runes[i-1]]; pok && pg.joining == 2 {
				prevConnect = true
			}
		}

		nextConnect := false
		if i+1 < n {
			if ng, nok := arabicGlyphMap[runes[i+1]]; nok && ng.joining > 0 {
				nextConnect = true
			} else if runes[i+1] >= 0xFEF5 && runes[i+1] <= 0xFEFB {
				nextConnect = true
			}
		}

		switch {
		case prevConnect && nextConnect && gf.joining == 2:
			res = append(res, gf.medial)
		case prevConnect && (gf.joining == 1 || gf.joining == 2):
			res = append(res, gf.final)
		case nextConnect && gf.joining == 2:
			res = append(res, gf.initial)
		default:
			res = append(res, gf.isolated)
		}
	}
	return string(res)
}

// PrepareTextForRendering handles BiDi reordering and Arabic shaping so LTR text drawers render correctly.
func PrepareTextForRendering(text string) string {
	hasRTL := false
	for _, r := range text {
		if IsRTL(r) {
			hasRTL = true
			break
		}
	}
	if !hasRTL {
		return text
	}

	shaped := ReshapeArabic(text)

	p := bidi.Paragraph{}
	p.SetString(shaped)
	order, err := p.Order()
	if err != nil {
		return shaped
	}

	var sb strings.Builder
	numRuns := order.NumRuns()
	for i := 0; i < numRuns; i++ {
		run := order.Run(i)
		runStr := run.String()
		if run.Direction() == bidi.RightToLeft {
			sb.WriteString(reverseRTLRunPreservingMarks(runStr))
		} else {
			sb.WriteString(runStr)
		}
	}
	return sb.String()
}

func isCombiningMark(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) || unicode.Is(unicode.Me, r)
}

// reverseRTLRunPreservingMarks reverses an RTL run while keeping combining
// marks (niqqud, tashkil) attached after their base characters.
func reverseRTLRunPreservingMarks(s string) string {
	runes := []rune(s)
	var clusters [][]rune
	for _, r := range runes {
		if isCombiningMark(r) && len(clusters) > 0 {
			clusters[len(clusters)-1] = append(clusters[len(clusters)-1], r)
			continue
		}
		clusters = append(clusters, []rune{r})
	}
	var out []rune
	for i := len(clusters) - 1; i >= 0; i-- {
		out = append(out, clusters[i]...)
	}
	return string(out)
}

// Multi-script font management
var (
	scriptParsedFontCache = make(map[ScriptType]*opentype.Font)
	scriptFontFaceCache   = make(map[string]font.Face)
	scriptFontMutex       sync.Mutex
)

// ResetScriptFontCache clears all cached font faces and parsed font objects, then forces OS memory release.
func ResetScriptFontCache() {
	scriptFontMutex.Lock()
	defer scriptFontMutex.Unlock()

	for _, face := range scriptFontFaceCache {
		if c, ok := face.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}
	scriptFontFaceCache = make(map[string]font.Face)
	scriptParsedFontCache = make(map[ScriptType]*opentype.Font)

	runtime.GC()
	debug.FreeOSMemory()
}

func getScriptFontPaths(st ScriptType) []string {
	switch runtime.GOOS {
	case "darwin":
		switch st {
		case ScriptArabic:
			return []string{
				"/System/Library/Fonts/GeezaPro.ttc",
				"/System/Library/Fonts/Supplemental/DecoTypeNaskh.ttc",
				"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
				"/Library/Fonts/Arial Unicode.ttf",
			}
		case ScriptDevanagari:
			return []string{
				"/System/Library/Fonts/Supplemental/DevanagariMT.ttc",
				"/System/Library/Fonts/Supplemental/DevanagariSangamMN.ttc",
				"/System/Library/Fonts/Supplemental/Kohinoor.ttc",
				"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			}
		case ScriptBengali:
			return []string{
				"/System/Library/Fonts/Supplemental/Bangla Sangam MN.ttc",
				"/System/Library/Fonts/Supplemental/Bangla MN.ttc",
				"/System/Library/Fonts/Supplemental/KohinoorBangla.ttc",
				"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			}
		case ScriptThai:
			return []string{
				"/System/Library/Fonts/Supplemental/Thonburi.ttc",
				"/System/Library/Fonts/Supplemental/Ayuthaya.ttf",
				"/System/Library/Fonts/Supplemental/Krub.ttc",
				"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			}
		case ScriptHebrew:
			return []string{
				"/System/Library/Fonts/SFHebrew.ttf",
				"/System/Library/Fonts/Supplemental/Raanana.ttc",
				"/System/Library/Fonts/Supplemental/Arial Hebrew.ttc",
				"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			}
		case ScriptCJK:
			return []string{
				"/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc",
				"/System/Library/Fonts/ヒラギノ角ゴシック W6.ttc",
				"/System/Library/Fonts/Hiragino Sans.ttc",
				"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
				"/System/Library/Fonts/PingFang.ttc",
				"/System/Library/Fonts/Hiragino Sans GB.ttc",
			}
		default:
			return []string{
				"/System/Library/Fonts/ヒラギノ角ゴシック W3.ttc",
				"/System/Library/Fonts/Hiragino Sans.ttc",
				"/System/Library/Fonts/Helvetica.ttc",
				"/System/Library/Fonts/Supplemental/Arial.ttf",
				"/System/Library/Fonts/Hiragino Sans GB.ttc",
			}
		}

	case "windows":
		windir := os.Getenv("WINDIR")
		if windir == "" {
			windir = `C:\Windows`
		}
		switch st {
		case ScriptArabic:
			return []string{windir + `\Fonts\seguihis.ttf`, windir + `\Fonts\arial.ttf`, windir + `\Fonts\tahoma.ttf`}
		case ScriptDevanagari:
			return []string{windir + `\Fonts\mangal.ttf`, windir + `\Fonts\aparaj.ttf`, windir + `\Fonts\nirmala.ttf`}
		case ScriptBengali:
			return []string{windir + `\Fonts\vrinda.ttf`, windir + `\Fonts\shonar.ttf`, windir + `\Fonts\nirmala.ttf`}
		case ScriptThai:
			return []string{windir + `\Fonts\leelawad.ttf`, windir + `\Fonts\leelawdb.ttf`, windir + `\Fonts\tahoma.ttf`}
		case ScriptHebrew:
			return []string{windir + `\Fonts\david.ttf`, windir + `\Fonts\frank.ttf`, windir + `\Fonts\gisha.ttf`, windir + `\Fonts\arial.ttf`}
		case ScriptCJK:
			return []string{windir + `\Fonts\meiryo.ttc`, windir + `\Fonts\msyh.ttc`, windir + `\Fonts\YuGothM.ttc`, windir + `\Fonts\malgun.ttf`}
		default:
			return []string{windir + `\Fonts\segoeui.ttf`, windir + `\Fonts\arial.ttf`, windir + `\Fonts\calibri.ttf`}
		}

	default: // Linux
		switch st {
		case ScriptArabic:
			return []string{
				"/usr/share/fonts/truetype/noto/NotoSansArabic-Regular.ttf",
				"/usr/share/fonts/opentype/noto/NotoSansArabic-Regular.otf",
				"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			}
		case ScriptDevanagari:
			return []string{
				"/usr/share/fonts/truetype/noto/NotoSansDevanagari-Regular.ttf",
				"/usr/share/fonts/truetype/lohit-devanagari/Lohit-Devanagari.ttf",
				"/usr/share/fonts/truetype/gargi/gargi.ttf",
			}
		case ScriptBengali:
			return []string{
				"/usr/share/fonts/truetype/noto/NotoSansBengali-Regular.ttf",
				"/usr/share/fonts/truetype/lohit-bengali/Lohit-Bengali.ttf",
			}
		case ScriptThai:
			return []string{
				"/usr/share/fonts/truetype/noto/NotoSansThai-Regular.ttf",
				"/usr/share/fonts/truetype/tlwg/Loma.ttf",
				"/usr/share/fonts/truetype/tlwg/Garuda.ttf",
			}
		case ScriptHebrew:
			return []string{
				"/usr/share/fonts/truetype/noto/NotoSansHebrew-Regular.ttf",
			}
		case ScriptCJK:
			return []string{
				"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
				"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
				"/usr/share/fonts/truetype/fonts-japanese-gothic.ttf",
			}
		default:
			return []string{
				"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
				"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
				"/usr/share/fonts/truetype/ubuntu/Ubuntu-R.ttf",
			}
		}
	}
}

func getParsedScriptFont(st ScriptType) *opentype.Font {
	if f, ok := scriptParsedFontCache[st]; ok {
		return f
	}

	paths := getScriptFontPaths(st)
	for _, fp := range paths {
		data, err := os.ReadFile(fp)
		if err != nil || len(data) == 0 {
			continue
		}

		if col, err := opentype.ParseCollection(data); err == nil {
			if f, err := col.Font(0); err == nil {
				scriptParsedFontCache[st] = f
				return f
			}
		}

		if f, err := opentype.Parse(data); err == nil {
			scriptParsedFontCache[st] = f
			return f
		}
	}

	// Fallback to CJK or embedded Go font
	if st != ScriptDefault && st != ScriptCJK {
		if cjk := getParsedScriptFont(ScriptCJK); cjk != nil {
			scriptParsedFontCache[st] = cjk
			return cjk
		}
	}

	// Embedded Go Regular fallback
	if f, err := opentype.Parse(goregular.TTF); err == nil {
		scriptParsedFontCache[st] = f
		return f
	}

	return nil
}

// GetScriptFontFace returns a high-quality font.Face for a specific script and size.
func GetScriptFontFace(st ScriptType, sizePt int) font.Face {
	scriptFontMutex.Lock()
	defer scriptFontMutex.Unlock()

	cacheKey := fmt.Sprintf("%d_%d", st, sizePt)
	if face, ok := scriptFontFaceCache[cacheKey]; ok {
		return face
	}

	f := getParsedScriptFont(st)
	if f != nil {
		if face, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size: float64(sizePt), DPI: 72, Hinting: font.HintingFull,
		}); err == nil {
			scriptFontFaceCache[cacheKey] = face
			return face
		}
	}

	face := basicfont.Face7x13
	scriptFontFaceCache[cacheKey] = face
	return face
}

// DrawMultiScriptText draws text on an image handling BiDi, Arabic shaping, and multi-script font fallbacks.
func DrawMultiScriptText(img *image.RGBA, x, y int, text string, col color.RGBA, sizePt int) {
	prepared := PrepareTextForRendering(text)
	curX := x
	baselineY := y + sizePt

	runes := []rune(prepared)
	for _, r := range runes {
		st := DetectScript(r)
		face := GetScriptFontFace(st, sizePt)

		isCombining := unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r)

		d := &font.Drawer{
			Dst:  img,
			Src:  image.NewUniform(col),
			Face: face,
			Dot:  fixed.P(curX, baselineY),
		}

		// Draw single character
		d.DrawString(string(r))

		if !isCombining {
			adv := d.MeasureString(string(r))
			w := adv.Ceil()
			if w <= 0 {
				w = sizePt * 3 / 5
			}
			curX += w
		}
	}
}

// MeasureMultiScriptText measures pixel width of multi-script text.
func MeasureMultiScriptText(text string, sizePt int) int {
	prepared := PrepareTextForRendering(text)
	totalW := 0
	runes := []rune(prepared)
	for _, r := range runes {
		st := DetectScript(r)
		face := GetScriptFontFace(st, sizePt)

		isCombining := unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r)
		if isCombining {
			continue
		}

		d := &font.Drawer{Face: face}
		adv := d.MeasureString(string(r))
		w := adv.Ceil()
		if w <= 0 {
			w = sizePt * 3 / 5
		}
		totalW += w
	}
	return totalW
}
