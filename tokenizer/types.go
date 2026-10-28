// Package tokenizer splits CSS source into tokens as described by CSS Syntax Module Level 3, with every offset
// measured in UTF-16 code units, and provides the token stream and offset-to-location map the parser uses.
package tokenizer

import (
	"slices"

	"github.com/csstree/csstree/internal/js"
)

// Token types (CSS Syntax Module Level 3, https://www.w3.org/TR/css-syntax-3/).
const (
	EOF                = 0  // <EOF-token>
	Ident              = 1  // <ident-token>
	Function           = 2  // <function-token>
	AtKeyword          = 3  // <at-keyword-token>
	Hash               = 4  // <hash-token>
	String             = 5  // <string-token>
	BadString          = 6  // <bad-string-token>
	URL                = 7  // <url-token>
	BadURL             = 8  // <bad-url-token>
	Delim              = 9  // <delim-token>
	Number             = 10 // <number-token>
	Percentage         = 11 // <percentage-token>
	Dimension          = 12 // <dimension-token>
	WhiteSpace         = 13 // <whitespace-token>
	CDO                = 14 // <CDO-token>
	CDC                = 15 // <CDC-token>
	Colon              = 16 // <colon-token> :
	Semicolon          = 17 // <semicolon-token> ;
	Comma              = 18 // <comma-token> ,
	LeftSquareBracket  = 19 // <[-token>
	RightSquareBracket = 20 // <]-token>
	LeftParenthesis    = 21 // <(-token>
	RightParenthesis   = 22 // <)-token>
	LeftCurlyBracket   = 23 // <{-token>
	RightCurlyBracket  = 24 // <}-token>
	Comment            = 25 // a comment, kept as a token
)

// TokenNames maps a token type to its name.
var TokenNames = []string{
	"EOF-token",
	"ident-token",
	"function-token",
	"at-keyword-token",
	"hash-token",
	"string-token",
	"bad-string-token",
	"url-token",
	"bad-url-token",
	"delim-token",
	"number-token",
	"percentage-token",
	"dimension-token",
	"whitespace-token",
	"CDO-token",
	"CDC-token",
	"colon-token",
	"semicolon-token",
	"comma-token",
	"[-token",
	"]-token",
	"(-token",
	")-token",
	"{-token",
	"}-token",
	"comment-token",
}

var tokenTypeNames = []string{
	"EOF", "Ident", "Function", "AtKeyword", "Hash", "String", "BadString", "Url", "BadUrl", "Delim", "Number",
	"Percentage", "Dimension", "WhiteSpace", "CDO", "CDC", "Colon", "Semicolon", "Comma", "LeftSquareBracket",
	"RightSquareBracket", "LeftParenthesis", "RightParenthesis", "LeftCurlyBracket", "RightCurlyBracket", "Comment",
}

// TokenTypes returns the token type numbers by name, in the order of the original's tokenTypes module namespace
// (names sorted by code unit).
func TokenTypes() *js.Object {
	names := slices.Clone(tokenTypeNames)
	slices.Sort(names)
	o := js.NewObject()
	for _, name := range names {
		o.Set(name, slices.Index(tokenTypeNames, name))
	}
	return o
}

// IsDigit reports a code unit in 0..9.
func IsDigit(code int) bool {
	return code >= 0x0030 && code <= 0x0039
}

// IsHexDigit reports a code unit in 0..9, A..F or a..f.
func IsHexDigit(code int) bool {
	return IsDigit(code) || (code >= 0x0041 && code <= 0x0046) || (code >= 0x0061 && code <= 0x0066)
}

// IsUppercaseLetter reports a code unit in A..Z.
func IsUppercaseLetter(code int) bool {
	return code >= 0x0041 && code <= 0x005A
}

// IsLowercaseLetter reports a code unit in a..z.
func IsLowercaseLetter(code int) bool {
	return code >= 0x0061 && code <= 0x007A
}

// IsLetter reports an ASCII letter.
func IsLetter(code int) bool {
	return IsUppercaseLetter(code) || IsLowercaseLetter(code)
}

// IsNonASCII reports a code unit at or above U+0080.
func IsNonASCII(code int) bool {
	return code >= 0x0080
}

// IsNameStart reports a name-start code point: a letter, a non-ASCII code unit or "_".
func IsNameStart(code int) bool {
	return IsLetter(code) || IsNonASCII(code) || code == 0x005F
}

// IsName reports a name code point: a name-start code point, a digit or "-".
func IsName(code int) bool {
	return IsNameStart(code) || IsDigit(code) || code == 0x002D
}

// IsNonPrintable reports U+0000..U+0008, U+000B, U+000E..U+001F and U+007F.
func IsNonPrintable(code int) bool {
	return (code >= 0x0000 && code <= 0x0008) || code == 0x000B || (code >= 0x000E && code <= 0x001F) || code == 0x007F
}

// IsNewline reports LF, CR and FF (the preprocessed input stream would map CR and FF to LF).
func IsNewline(code int) bool {
	return code == 0x000A || code == 0x000D || code == 0x000C
}

// IsWhiteSpace reports a newline, a tab or a space.
func IsWhiteSpace(code int) bool {
	return IsNewline(code) || code == 0x0020 || code == 0x0009
}

// IsValidEscape reports whether two code units start a valid escape.
func IsValidEscape(first, second int) bool {
	if first != 0x005C {
		return false
	}
	return !(IsNewline(second) || second == EOF)
}

// IsIdentifierStart reports whether three code units would start an identifier.
func IsIdentifierStart(first, second, third int) bool {
	if first == 0x002D {
		return IsNameStart(second) || second == 0x002D || IsValidEscape(second, third)
	}
	if IsNameStart(first) {
		return true
	}
	if first == 0x005C {
		return IsValidEscape(first, second)
	}
	return false
}

// IsNumberStart returns how many of three code units start a number (0 when they do not).
func IsNumberStart(first, second, third int) int {
	if first == 0x002B || first == 0x002D {
		if IsDigit(second) {
			return 2
		}
		if second == 0x002E && IsDigit(third) {
			return 3
		}
		return 0
	}
	if first == 0x002E {
		if IsDigit(second) {
			return 2
		}
		return 0
	}
	if IsDigit(first) {
		return 1
	}
	return 0
}

// IsBOM returns 1 for a byte order mark code unit (U+FEFF or U+FFFE), else 0.
func IsBOM(code int) int {
	if code == 0xFEFF || code == 0xFFFE {
		return 1
	}
	return 0
}

// Code unit categories returned by CharCodeCategory for the code units the tokenizer switches on.
const (
	EOFCategory          = 0x80
	WhiteSpaceCategory   = 0x82
	DigitCategory        = 0x83
	NameStartCategory    = 0x84
	NonPrintableCategory = 0x85
)

var category = func() [0x80]int {
	var c [0x80]int
	for i := range c {
		switch {
		case IsWhiteSpace(i):
			c[i] = WhiteSpaceCategory
		case IsDigit(i):
			c[i] = DigitCategory
		case IsNameStart(i):
			c[i] = NameStartCategory
		case IsNonPrintable(i):
			c[i] = NonPrintableCategory
		default:
			c[i] = i
		}
	}
	return c
}()

// CharCodeCategory returns the category of a code unit: one of the category constants, or the code unit itself
// for ASCII punctuation; every non-ASCII code unit is a name-start code point.
func CharCodeCategory(code int) int {
	if code < 0 {
		return -1
	}
	if code < 0x80 {
		return category[code]
	}
	return NameStartCategory
}
