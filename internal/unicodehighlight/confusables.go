package unicodehighlight

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	fullwidthFirst  = 0xFF01
	fullwidthLast   = 0xFF5E
	fullwidthOffset = 0xFEE0
	mathAlnumFirst  = 0x1D400
	mathAlnumLast   = 0x1D7FF
)

var confusables = buildConfusables()

func buildConfusables() map[rune]rune {
	out := map[rune]rune{
		'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'у': 'y', 'х': 'x', 'ѕ': 's', 'і': 'i', 'ј': 'j',
		'һ': 'h', 'ԁ': 'd', 'ԛ': 'q', 'ԝ': 'w', 'ӏ': 'l', 'ѵ': 'v', 'ү': 'y', 'б': '6',
		'А': 'A', 'В': 'B', 'Е': 'E', 'К': 'K', 'М': 'M', 'Н': 'H', 'О': 'O', 'Р': 'P', 'С': 'C', 'Т': 'T',
		'Х': 'X', 'Ѕ': 'S', 'І': 'I', 'Ј': 'J', 'Ү': 'Y', 'У': 'Y', 'Ӏ': 'I', 'З': '3',
		'Α': 'A', 'Β': 'B', 'Ε': 'E', 'Ζ': 'Z', 'Η': 'H', 'Ι': 'I', 'Κ': 'K', 'Μ': 'M', 'Ν': 'N', 'Ο': 'O',
		'Ρ': 'P', 'Τ': 'T', 'Υ': 'Y', 'Χ': 'X', 'ο': 'o', 'ν': 'v', 'α': 'a', 'ρ': 'p', 'υ': 'u', 'ι': 'i',
		'γ': 'y', '\u03F2': 'c', 'ϳ': 'j', '\u037E': ';',
		'օ': 'o', 'ս': 'u', 'հ': 'h', 'ո': 'n', 'զ': 'q', 'ց': 'g', 'Ս': 'U', 'Օ': 'O',
		'ı': 'i', 'ɑ': 'a', 'ɡ': 'g', 'ǀ': 'l', 'ɩ': 'i',
		'\u2010': '-', '\u2011': '-', '\u2012': '-', '\u2013': '-', '\u2212': '-', '\u2215': '/', '\u2044': '/',
		'\u2216': '\\', '\u2217': '*', '\u2018': '\'', '\u2019': '\'', '\u201B': '\'', '\u2032': '\'', '\u02BC': '\'',
		'\u201C': '"', '\u201D': '"', '\u2033': '"', '\u201A': ',', '\u2024': '.', '\u0589': ':', '\u05C3': ':',
		'\u2236': ':', '\uA789': ':', '\u02D0': ':', '\u05C0': 'l', '\u2223': 'l', '\u2039': '<', '\u203A': '>',
		'\u02C2': '<', '\u02C3': '>',
	}
	for r := rune(fullwidthFirst); r <= fullwidthLast; r++ {
		out[r] = r - fullwidthOffset
	}
	for r := rune(0x2160); r <= 0x217F; r++ {
		if a, ok := asciiForm(r); ok {
			out[r] = a
		}
	}
	for r := rune(mathAlnumFirst); r <= mathAlnumLast; r++ {
		if a, ok := asciiForm(r); ok {
			out[r] = a
		}
	}
	return out
}

func asciiForm(r rune) (rune, bool) {
	s := norm.NFKC.String(string(r))
	a, n := utf8.DecodeRuneInString(s)
	return a, n == len(s) && a < utf8.RuneSelf && a > ' '
}
