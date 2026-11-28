package unicodehighlight

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Kind uint8

const (
	Control Kind = iota + 1
	Invisible
	Ambiguous
	NonBasicASCII
)

const (
	controlPicturesBase = 0x2400
	deleteChar          = 0x7F
	deletePicture       = 0x2421
	hexDigits           = "0123456789ABCDEF"
	codePointDigits     = 4
	codePointShift      = 4
)

type Options struct {
	Control       bool
	Invisible     bool
	Ambiguous     bool
	NonBasicASCII bool
	Allowed       map[rune]bool
	Locales       Locales
}

type Mark struct {
	Start, End int
	Kind       Kind
	Rune       rune
	Confusable rune
}

func (o *Options) Any() bool {
	return o.Control || o.Invisible || o.Ambiguous || o.NonBasicASCII
}

func Scan(line string, base int, o *Options, dst []Mark) []Mark {
	if !o.Any() || plain(line) {
		return dst
	}
	for i, r := range line {
		if r == '\t' || r >= ' ' && r < deleteChar {
			continue
		}
		size := utf8.RuneLen(r)
		if size < 0 {
			size = 1
		}
		m := Mark{Start: base + i, End: base + i + size, Rune: r}
		switch {
		case o.Control && IsControl(r):
			m.Kind = Control
		case o.Allowed[r]:
			continue
		case o.Invisible && IsInvisible(r) && !joinsEmoji(line, i, r):
			m.Kind = Invisible
		case r < utf8.RuneSelf:
			continue
		case o.Ambiguous && confusableHere(line, i, r, o):
			m.Kind, m.Confusable = Ambiguous, confusables[r]
		case o.NonBasicASCII && !IsInvisible(r) && highlightsInWord(line, i):
			m.Kind = NonBasicASCII
		default:
			continue
		}
		dst = append(dst, m)
	}
	return dst
}

func plain(line string) bool {
	for i := 0; i < len(line); i++ {
		if b := line[i]; b >= utf8.RuneSelf || b < ' ' && b != '\t' || b == deleteChar {
			return false
		}
	}
	return true
}

func IsControl(r rune) bool {
	switch {
	case r < ' ':
		return r != '\t' && r != '\n' && r != '\r'
	case r == deleteChar, r == 0x061C, r == 0x200E, r == 0x200F:
		return true
	case r >= 0x202A && r <= 0x202E, r >= 0x2066 && r <= 0x2069:
		return true
	}
	return false
}

func ControlText(r rune) string {
	switch {
	case r < ' ':
		return string(controlPicturesBase + r)
	case r == deleteChar:
		return string(rune(deletePicture))
	}
	return "[" + CodePoint(r) + "]"
}

func CodePoint(r rune) string {
	digits := make([]byte, 0, codePointDigits+2)
	for v := uint32(r); v > 0 || len(digits) < codePointDigits; v >>= codePointShift {
		digits = append(digits, hexDigits[v&0xF])
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return "U+" + string(digits)
}

func Describe(m Mark) string {
	char := CodePoint(m.Rune)
	switch m.Kind {
	case Control:
		return "The character " + char + " is a control character that can change how the text around it shows"
	case Invisible:
		return "The character " + char + " is invisible"
	case Ambiguous:
		return "The character " + char + " \"" + string(m.Rune) + "\" could be confused with the ASCII character " +
			CodePoint(m.Confusable) + " \"" + string(m.Confusable) + "\", which is more common in source code"
	}
	return "The character " + char + " \"" + string(m.Rune) + "\" is not a basic ASCII character"
}

func IsInvisible(r rune) bool {
	switch {
	case r == 0x00A0, r == 0x00AD, r == 0x034F, r == 0x061C, r == 0x115F, r == 0x1160, r == 0x17B4, r == 0x17B5:
		return true
	case r >= 0x180B && r <= 0x180E, r >= 0x2000 && r <= 0x200F, r >= 0x202A && r <= 0x202F:
		return true
	case r >= 0x205F && r <= 0x206F, r == 0x2800, r == 0x3000, r == 0x3164, r >= 0xFE00 && r <= 0xFE0F:
		return true
	case r == 0xFEFF, r == 0xFFA0, r >= 0xFFF0 && r <= 0xFFF8, r >= 0x1BCA0 && r <= 0x1BCA3:
		return true
	case r >= 0x1D173 && r <= 0x1D17A, r >= 0xE0000 && r <= 0xE007F, r >= 0xE0100 && r <= 0xE01EF:
		return true
	}
	return false
}

func joinsEmoji(line string, i int, r rune) bool {
	if r != 0x200D && !(r >= 0xFE00 && r <= 0xFE0F) && !(r >= 0xE0020 && r <= 0xE007F) {
		return false
	}
	prev, _ := utf8.DecodeLastRuneInString(line[:i])
	return pictographic(prev) || prev == 0xFE0F || prev >= 0xE0020 && prev <= 0xE007F
}

func pictographic(r rune) bool {
	return r >= 0x1F000 || unicode.Is(unicode.So, r) || unicode.Is(unicode.Sk, r) && r >= 0x1F3FB
}

func confusableHere(line string, i int, r rune, o *Options) bool {
	if _, ok := confusables[r]; !ok || o.Locales.allows(r) {
		return false
	}
	return highlightsInWord(line, i)
}

func highlightsInWord(line string, i int) bool {
	start, end := i, i
	for start > 0 {
		r, n := utf8.DecodeLastRuneInString(line[:start])
		if !wordRune(r) {
			break
		}
		start -= n
	}
	for end < len(line) {
		r, n := utf8.DecodeRuneInString(line[end:])
		if !wordRune(r) && end > i {
			break
		}
		end += n
	}
	basic, distinct := false, false
	for _, r := range line[start:end] {
		if r < utf8.RuneSelf {
			basic = true
			continue
		}
		if _, ok := confusables[r]; !ok && !IsInvisible(r) {
			distinct = true
		}
	}
	return basic || !distinct
}

func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

type Locales struct {
	scripts []*unicode.RangeTable
	wide    bool
}

var localeScripts = map[string]*unicode.RangeTable{
	"ru": unicode.Cyrillic, "uk": unicode.Cyrillic, "be": unicode.Cyrillic, "bg": unicode.Cyrillic, "sr": unicode.Cyrillic,
	"mk": unicode.Cyrillic, "kk": unicode.Cyrillic, "ky": unicode.Cyrillic, "mn": unicode.Cyrillic, "tg": unicode.Cyrillic,
	"tt": unicode.Cyrillic, "ba": unicode.Cyrillic, "cv": unicode.Cyrillic, "el": unicode.Greek, "hy": unicode.Armenian,
}

var wideLocales = map[string]bool{"ja": true, "zh": true, "ko": true}

func ForLanguages(tags []string) Locales {
	var l Locales
	for _, tag := range tags {
		lang := strings.ToLower(tag)
		if i := strings.IndexAny(lang, "-_"); i >= 0 {
			lang = lang[:i]
		}
		if s, ok := localeScripts[lang]; ok {
			l.scripts = append(l.scripts, s)
		}
		l.wide = l.wide || wideLocales[lang]
	}
	return l
}

func (l Locales) allows(r rune) bool {
	if l.wide && r >= fullwidthFirst && r <= fullwidthLast {
		return true
	}
	for _, s := range l.scripts {
		if unicode.Is(s, r) {
			return true
		}
	}
	return false
}
