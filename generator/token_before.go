package generator

import (
	"github.com/csstree/csstree/js"
	t "github.com/csstree/csstree/tokenizer"
)

// TokenBefore decides whether white space must separate a token from the previous one. It receives the code
// returned for the previous token and returns the code of this token, with bit 0 set when white space is needed.
type TokenBefore func(prevCode, typ int, value string) int

// pairType is a token type or, for a delim, its character.
type pairType struct {
	typ   int
	delim rune
}

func tp(typ int) pairType    { return pairType{typ: typ} }
func dl(delim rune) pairType { return pairType{typ: t.Delim, delim: delim} }
func (p pairType) code() int { return pairCode(p.typ, string(p.delim)) }

// pairCode is the original's code(): a delim is replaced by its first character (non-ASCII as 0x80, NaN as 0)
// shifted left by 6; the result is shifted left by 1.
func pairCode(typ int, value string) int {
	if typ == t.Delim {
		c := js.CharCodeAt(value, 0)
		if c < 0 {
			return 0
		}
		return min(c, 0x80) << 6 << 1
	}
	return typ << 1
}

var specPairs = [][2]pairType{
	{tp(t.Ident), tp(t.Ident)}, {tp(t.Ident), tp(t.Function)}, {tp(t.Ident), tp(t.URL)}, {tp(t.Ident), tp(t.BadURL)},
	{tp(t.Ident), dl('-')}, {tp(t.Ident), tp(t.Number)}, {tp(t.Ident), tp(t.Percentage)}, {tp(t.Ident), tp(t.Dimension)},
	{tp(t.Ident), tp(t.CDC)}, {tp(t.Ident), tp(t.LeftParenthesis)},
	{tp(t.AtKeyword), tp(t.Ident)}, {tp(t.AtKeyword), tp(t.Function)}, {tp(t.AtKeyword), tp(t.URL)},
	{tp(t.AtKeyword), tp(t.BadURL)}, {tp(t.AtKeyword), dl('-')}, {tp(t.AtKeyword), tp(t.Number)},
	{tp(t.AtKeyword), tp(t.Percentage)}, {tp(t.AtKeyword), tp(t.Dimension)}, {tp(t.AtKeyword), tp(t.CDC)},
	{tp(t.Hash), tp(t.Ident)}, {tp(t.Hash), tp(t.Function)}, {tp(t.Hash), tp(t.URL)}, {tp(t.Hash), tp(t.BadURL)},
	{tp(t.Hash), dl('-')}, {tp(t.Hash), tp(t.Number)}, {tp(t.Hash), tp(t.Percentage)}, {tp(t.Hash), tp(t.Dimension)},
	{tp(t.Hash), tp(t.CDC)},
	{tp(t.Dimension), tp(t.Ident)}, {tp(t.Dimension), tp(t.Function)}, {tp(t.Dimension), tp(t.URL)},
	{tp(t.Dimension), tp(t.BadURL)}, {tp(t.Dimension), dl('-')}, {tp(t.Dimension), tp(t.Number)},
	{tp(t.Dimension), tp(t.Percentage)}, {tp(t.Dimension), tp(t.Dimension)}, {tp(t.Dimension), tp(t.CDC)},
	{dl('#'), tp(t.Ident)}, {dl('#'), tp(t.Function)}, {dl('#'), tp(t.URL)}, {dl('#'), tp(t.BadURL)}, {dl('#'), dl('-')},
	{dl('#'), tp(t.Number)}, {dl('#'), tp(t.Percentage)}, {dl('#'), tp(t.Dimension)}, {dl('#'), tp(t.CDC)},
	{dl('-'), tp(t.Ident)}, {dl('-'), tp(t.Function)}, {dl('-'), tp(t.URL)}, {dl('-'), tp(t.BadURL)}, {dl('-'), dl('-')},
	{dl('-'), tp(t.Number)}, {dl('-'), tp(t.Percentage)}, {dl('-'), tp(t.Dimension)}, {dl('-'), tp(t.CDC)},
	{tp(t.Number), tp(t.Ident)}, {tp(t.Number), tp(t.Function)}, {tp(t.Number), tp(t.URL)}, {tp(t.Number), tp(t.BadURL)},
	{tp(t.Number), tp(t.Number)}, {tp(t.Number), tp(t.Percentage)}, {tp(t.Number), tp(t.Dimension)},
	{tp(t.Number), dl('%')}, {tp(t.Number), tp(t.CDC)},
	{dl('@'), tp(t.Ident)}, {dl('@'), tp(t.Function)}, {dl('@'), tp(t.URL)}, {dl('@'), tp(t.BadURL)}, {dl('@'), dl('-')},
	{dl('@'), tp(t.CDC)},
	{dl('.'), tp(t.Number)}, {dl('.'), tp(t.Percentage)}, {dl('.'), tp(t.Dimension)},
	{dl('+'), tp(t.Number)}, {dl('+'), tp(t.Percentage)}, {dl('+'), tp(t.Dimension)},
	{dl('/'), dl('*')},
}

var safeExtraPairs = [][2]pairType{
	{tp(t.Ident), tp(t.Hash)}, {tp(t.Dimension), tp(t.Hash)}, {tp(t.Hash), tp(t.Hash)},
	{tp(t.AtKeyword), tp(t.LeftParenthesis)}, {tp(t.AtKeyword), tp(t.String)}, {tp(t.AtKeyword), tp(t.Colon)},
	{tp(t.Percentage), tp(t.Percentage)}, {tp(t.Percentage), tp(t.Dimension)}, {tp(t.Percentage), tp(t.Function)},
	{tp(t.Percentage), dl('-')},
	{tp(t.RightParenthesis), tp(t.Ident)}, {tp(t.RightParenthesis), tp(t.Function)},
	{tp(t.RightParenthesis), tp(t.Percentage)}, {tp(t.RightParenthesis), tp(t.Dimension)},
	{tp(t.RightParenthesis), tp(t.Hash)}, {tp(t.RightParenthesis), dl('-')},
}

func createMap(pairs [][2]pairType) TokenBefore {
	required := map[int]bool{}
	for _, pair := range pairs {
		required[pair[0].code()<<16|pair[1].code()] = true
	}
	return func(prevCode, typ int, value string) int {
		nextCode := pairCode(typ, value)
		nextCharCode := js.CharCodeAt(value, 0)
		var emitWs bool
		if (nextCharCode == '-' && typ != t.Ident && typ != t.Function && typ != t.CDC) || nextCharCode == '+' {
			emitWs = required[(prevCode&0xFFFE)<<16|nextCharCode<<7]
		} else {
			emitWs = required[(prevCode&0xFFFE)<<16|nextCode]
		}
		if emitWs {
			return nextCode | 1
		}
		return nextCode
	}
}

// Spec separates tokens only where CSS Syntax requires it; Safe also separates pairs some browsers misread.
var (
	Spec = createMap(specPairs)
	Safe = createMap(append(append([][2]pairType{}, specPairs...), safeExtraPairs...))
)
