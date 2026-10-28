package lexer

import (
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
)

func isDelimToken(tok *token, code int) bool {
	return tok != nil && tok.typ == t.Delim && js.CharCodeAt(tok.value, 0) == code
}

func cmpTokenChar(tok *token, offset, code int) bool {
	return t.CmpChar(js.UTF16(tok.value), offset, code)
}

func skipSC(tok *token, offset int, next func(int) *token) int {
	for tok != nil && (tok.typ == t.WhiteSpace || tok.typ == t.Comment) {
		offset++
		tok = next(offset)
	}
	return offset
}

func checkInteger(tok *token, valueOffset int, disallowSign bool, offset int) int {
	if tok == nil {
		return 0
	}
	code := js.CharCodeAt(tok.value, valueOffset)
	if code == '+' || code == '-' {
		if disallowSign {
			return 0
		}
		valueOffset++
	}
	for ; valueOffset < js.Len(tok.value); valueOffset++ {
		if !t.IsDigit(js.CharCodeAt(tok.value, valueOffset)) {
			return 0
		}
	}
	return offset + 1
}

func consumeB(tok *token, start int, next func(int) *token) int {
	sign := false
	offset := skipSC(tok, start, next)
	tok = next(offset)
	if tok == nil {
		return start
	}
	if tok.typ != t.Number {
		if !isDelimToken(tok, '+') && !isDelimToken(tok, '-') {
			return start
		}
		sign = true
		offset++
		offset = skipSC(next(offset), offset, next)
		tok = next(offset)
		if tok == nil || tok.typ != t.Number {
			return 0
		}
	}
	if !sign {
		if code := js.CharCodeAt(tok.value, 0); code != '+' && code != '-' {
			return 0
		}
	}
	valueOffset := 1
	if sign {
		valueOffset = 0
	}
	return checkInteger(tok, valueOffset, sign, offset)
}

// anPlusB matches the an+b microsyntax over one or more tokens.
func anPlusB(tok *token, next func(int) *token, _ *js.Object) int {
	offset := 0
	if tok == nil {
		return 0
	}
	n := js.Len(tok.value)
	switch {
	case tok.typ == t.Number:
		return checkInteger(tok, 0, false, offset)
	case tok.typ == t.Ident && js.CharCodeAt(tok.value, 0) == '-':
		if !cmpTokenChar(tok, 1, 'n') {
			return 0
		}
		switch n {
		case 2:
			offset++
			return consumeB(next(offset), offset, next)
		case 3:
			if js.CharCodeAt(tok.value, 2) != '-' {
				return 0
			}
			offset++
			offset = skipSC(next(offset), offset, next)
			return checkInteger(next(offset), 0, true, offset)
		}
		if js.CharCodeAt(tok.value, 2) != '-' {
			return 0
		}
		return checkInteger(tok, 3, true, offset)
	case tok.typ == t.Ident || (isDelimToken(tok, '+') && next(offset+1) != nil && next(offset+1).typ == t.Ident):
		if tok.typ != t.Ident {
			offset++
			tok = next(offset)
		}
		if tok == nil || !cmpTokenChar(tok, 0, 'n') {
			return 0
		}
		switch js.Len(tok.value) {
		case 1:
			offset++
			return consumeB(next(offset), offset, next)
		case 2:
			if js.CharCodeAt(tok.value, 1) != '-' {
				return 0
			}
			offset++
			offset = skipSC(next(offset), offset, next)
			return checkInteger(next(offset), 0, true, offset)
		}
		if js.CharCodeAt(tok.value, 1) != '-' {
			return 0
		}
		return checkInteger(tok, 2, true, offset)
	case tok.typ == t.Dimension:
		code := js.CharCodeAt(tok.value, 0)
		sign := 0
		if code == '+' || code == '-' {
			sign = 1
		}
		i := sign
		for ; i < n; i++ {
			if !t.IsDigit(js.CharCodeAt(tok.value, i)) {
				break
			}
		}
		if i == sign || !cmpTokenChar(tok, i, 'n') {
			return 0
		}
		if i+1 == n {
			offset++
			return consumeB(next(offset), offset, next)
		}
		if js.CharCodeAt(tok.value, i+1) != '-' {
			return 0
		}
		if i+2 == n {
			offset++
			offset = skipSC(next(offset), offset, next)
			return checkInteger(next(offset), 0, true, offset)
		}
		return checkInteger(tok, i+2, true, offset)
	}
	return 0
}

func hexSequence(tok *token, offset int, allowDash bool) int {
	hexlen := 0
	for pos := offset; pos < js.Len(tok.value); pos++ {
		code := js.CharCodeAt(tok.value, pos)
		if code == '-' && allowDash && hexlen != 0 {
			hexSequence(tok, offset+hexlen+1, false)
			return 6
		}
		if !t.IsHexDigit(code) {
			return 0
		}
		hexlen++
		if hexlen > 6 {
			return 0
		}
	}
	return hexlen
}

func withQuestionMarkSequence(consumed, length int, next func(int) *token) int {
	if consumed == 0 {
		return 0
	}
	for isDelimToken(next(length), '?') {
		consumed++
		if consumed > 6 {
			return 0
		}
		length++
	}
	return length
}

// urange matches a unicode-range over one or more tokens.
func urange(tok *token, next func(int) *token, _ *js.Object) int {
	length := 0
	if tok == nil || tok.typ != t.Ident || !cmpTokenChar(tok, 0, 'u') {
		return 0
	}
	length++
	tok = next(length)
	if tok == nil {
		return 0
	}
	if isDelimToken(tok, '+') {
		length++
		tok = next(length)
		if tok == nil {
			return 0
		}
		if tok.typ == t.Ident {
			length++
			return withQuestionMarkSequence(hexSequence(tok, 0, true), length, next)
		}
		if isDelimToken(tok, '?') {
			length++
			return withQuestionMarkSequence(1, length, next)
		}
		return 0
	}
	if tok.typ == t.Number {
		consumed := hexSequence(tok, 1, true)
		if consumed == 0 {
			return 0
		}
		length++
		tok = next(length)
		if tok == nil {
			return length
		}
		if tok.typ == t.Dimension || tok.typ == t.Number {
			if js.CharCodeAt(tok.value, 0) != '-' || hexSequence(tok, 1, false) == 0 {
				return 0
			}
			return length + 1
		}
		return withQuestionMarkSequence(consumed, length, next)
	}
	if tok.typ == t.Dimension {
		length++
		return withQuestionMarkSequence(hexSequence(tok, 1, true), length, next)
	}
	return 0
}
