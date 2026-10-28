package utils

import (
	"strconv"

	"github.com/csstree/csstree/js"
	"github.com/csstree/csstree/tokenizer"
)

const (
	reverseSolidus   = 0x005C
	quotationMark    = 0x0022
	apostrophe       = 0x0027
	space            = 0x0020
	leftParenthesis  = 0x0028
	rightParenthesis = 0x0029
)

func unitAt(units []uint16, i int) int {
	if i >= 0 && i < len(units) {
		return int(units[i])
	}
	return -1
}

// decodeRange decodes escapes in units[start..end]; a backslash at end drops the text decoded so far and keeps
// what follows it when the backslash is not the last code unit (the original's behaviour for strings and urls).
func decodeRange(units []uint16, start, end int, keepTail bool) string {
	var decoded []uint16
	for i := start; i <= end; i++ {
		code := unitAt(units, i)
		if code != reverseSolidus {
			decoded = append(decoded, units[i])
			continue
		}
		if i == end {
			if keepTail && i != len(units)-1 {
				decoded = append([]uint16(nil), units[i+1:]...)
			}
			break
		}
		i++
		code = unitAt(units, i)
		if tokenizer.IsValidEscape(reverseSolidus, code) {
			escapeStart := i - 1
			escapeEnd := tokenizer.ConsumeEscaped(units, escapeStart)
			i = escapeEnd - 1
			decoded = append(decoded, js.UTF16(tokenizer.DecodeEscaped(js.FromUTF16(units[escapeStart+1:min(escapeEnd, len(units))])))...)
		} else if code == 0x000D && unitAt(units, i+1) == 0x000A {
			i++
		}
	}
	return js.FromUTF16(decoded)
}

// IdentDecode decodes the escapes of an identifier.
func IdentDecode(str string) string {
	units := js.UTF16(str)
	return decodeRange(units, 0, len(units)-1, false)
}

// IdentEncode escapes an identifier so that it reads back as the same name.
func IdentEncode(str string) string {
	units := js.UTF16(str)
	if len(units) == 1 && units[0] == 0x002D {
		return "\\-"
	}
	var encoded []uint16
	for i, u := range units {
		code := int(u)
		if code == 0x0000 {
			encoded = append(encoded, 0xFFFD)
			continue
		}
		if code <= 0x001F || code == 0x007F || (code >= 0x0030 && code <= 0x0039 && (i == 0 || (i == 1 && units[0] == 0x002D))) {
			encoded = append(encoded, js.UTF16("\\"+strconv.FormatInt(int64(code), 16)+" ")...)
			continue
		}
		if !tokenizer.IsName(code) {
			encoded = append(encoded, reverseSolidus)
		}
		encoded = append(encoded, u)
	}
	return js.FromUTF16(encoded)
}

// StringDecode decodes a CSS string, with or without its quotes.
func StringDecode(str string) string {
	units := js.UTF16(str)
	n := len(units)
	firstChar := unitAt(units, 0)
	start := 0
	if firstChar == quotationMark || firstChar == apostrophe {
		start = 1
	}
	end := n - 1
	if start == 1 && n > 1 && int(units[n-1]) == firstChar {
		end = n - 2
	}
	return decodeRange(units, start, end, true)
}

// encodeQuoted escapes control characters as hex and the characters isSpecial selects with a backslash; a space
// separates a hex escape from a following hex digit (or white space, when wsAfterHex is set).
func encodeQuoted(str string, isSpecial func(code int) bool, wsAfterHex bool) []uint16 {
	var encoded []uint16
	wsBeforeHexIsNeeded := false
	for _, u := range js.UTF16(str) {
		code := int(u)
		switch {
		case code == 0x0000:
			encoded = append(encoded, 0xFFFD)
		case code <= 0x001F || code == 0x007F:
			encoded = append(encoded, js.UTF16("\\"+strconv.FormatInt(int64(code), 16))...)
			wsBeforeHexIsNeeded = true
		case isSpecial(code):
			encoded = append(encoded, reverseSolidus, u)
			wsBeforeHexIsNeeded = false
		default:
			if wsBeforeHexIsNeeded && (tokenizer.IsHexDigit(code) || (wsAfterHex && tokenizer.IsWhiteSpace(code))) {
				encoded = append(encoded, space)
			}
			encoded = append(encoded, u)
			wsBeforeHexIsNeeded = false
		}
	}
	return encoded
}

// StringEncode quotes a string with double quotes, or apostrophes when apostrophe is set.
func StringEncode(str string, apostropheQuote bool) string {
	quote := uint16(quotationMark)
	if apostropheQuote {
		quote = apostrophe
	}
	encoded := encodeQuoted(str, func(code int) bool { return code == int(quote) || code == reverseSolidus }, true)
	return js.FromUTF16(append(append([]uint16{quote}, encoded...), quote))
}

// URLDecode decodes a url(...) token.
func URLDecode(str string) string {
	units := js.UTF16(str)
	n := len(units)
	start := 4
	end := n - 1
	if unitAt(units, n-1) == rightParenthesis {
		end = n - 2
	}
	for start < end && tokenizer.IsWhiteSpace(unitAt(units, start)) {
		start++
	}
	for start < end && tokenizer.IsWhiteSpace(unitAt(units, end)) {
		end--
	}
	return decodeRange(units, start, end, true)
}

// URLEncode writes a string as a url(...) token.
func URLEncode(str string) string {
	encoded := encodeQuoted(str, func(code int) bool {
		switch code {
		case space, reverseSolidus, quotationMark, apostrophe, leftParenthesis, rightParenthesis:
			return true
		}
		return false
	}, false)
	return "url(" + js.FromUTF16(encoded) + ")"
}
