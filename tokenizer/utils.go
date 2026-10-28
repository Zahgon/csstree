package tokenizer

import (
	"math"

	"github.com/csstree/csstree/js"
)

// at is charCodeAt on UTF-16 code units: -1 (NaN) outside the source.
func at(src []uint16, offset int) int {
	if offset >= 0 && offset < len(src) {
		return int(src[offset])
	}
	return -1
}

// charCode returns the code unit at offset, or 0 (EOF) past the end of the source.
func charCode(src []uint16, offset int) int {
	if offset < len(src) {
		return at(src, offset)
	}
	return 0
}

// GetNewlineLength returns 2 for CR LF at offset (code is the code unit there), else 1.
func GetNewlineLength(src []uint16, offset, code int) int {
	if code == 13 && charCode(src, offset+1) == 10 {
		return 2
	}
	return 1
}

// CmpChar compares the code unit at offset with a lowercase reference code, folding ASCII uppercase only.
func CmpChar(src []uint16, offset, referenceCode int) bool {
	code := at(src, offset)
	if IsUppercaseLetter(code) {
		code |= 32
	}
	return code == referenceCode
}

// CmpStr compares src[start:end] with a lowercase reference string, folding ASCII uppercase only.
func CmpStr(src []uint16, start, end int, referenceStr string) bool {
	ref := js.UTF16(referenceStr)
	if end-start != len(ref) {
		return false
	}
	if start < 0 || end > len(src) {
		return false
	}
	for i := start; i < end; i++ {
		testCode := int(src[i])
		if IsUppercaseLetter(testCode) {
			testCode |= 32
		}
		if testCode != int(ref[i-start]) {
			return false
		}
	}
	return true
}

// FindWhiteSpaceStart walks back from offset over white space and returns the offset after the last
// non-white-space code unit.
func FindWhiteSpaceStart(src []uint16, offset int) int {
	for ; offset >= 0; offset-- {
		if !IsWhiteSpace(at(src, offset)) {
			break
		}
	}
	return offset + 1
}

// FindWhiteSpaceEnd returns the offset of the first non-white-space code unit at or after offset.
func FindWhiteSpaceEnd(src []uint16, offset int) int {
	for ; offset < len(src); offset++ {
		if !IsWhiteSpace(at(src, offset)) {
			break
		}
	}
	return offset
}

// FindDecimalNumberEnd returns the offset after the run of digits starting at offset.
func FindDecimalNumberEnd(src []uint16, offset int) int {
	for ; offset < len(src); offset++ {
		if !IsDigit(at(src, offset)) {
			break
		}
	}
	return offset
}

// ConsumeEscaped consumes an escape whose backslash is at offset and returns the offset after it
// (§ 4.3.7. Consume an escaped code point).
func ConsumeEscaped(src []uint16, offset int) int {
	offset += 2
	if IsHexDigit(charCode(src, offset-1)) {
		for maxOffset := min(len(src), offset+5); offset < maxOffset; offset++ {
			if !IsHexDigit(charCode(src, offset)) {
				break
			}
		}
		code := charCode(src, offset)
		if IsWhiteSpace(code) {
			offset += GetNewlineLength(src, offset, code)
		}
	}
	return offset
}

// ConsumeName consumes name code points and escapes from offset and returns the offset after them
// (§ 4.3.11. Consume a name).
func ConsumeName(src []uint16, offset int) int {
	for ; offset < len(src); offset++ {
		code := at(src, offset)
		if IsName(code) {
			continue
		}
		if IsValidEscape(code, charCode(src, offset+1)) {
			offset = ConsumeEscaped(src, offset) - 1
			continue
		}
		break
	}
	return offset
}

// ConsumeNumber consumes a number from offset and returns the offset after it (§ 4.3.12. Consume a number).
func ConsumeNumber(src []uint16, offset int) int {
	code := at(src, offset)
	if code == 0x002B || code == 0x002D {
		offset++
		code = at(src, offset)
	}
	if IsDigit(code) {
		offset = FindDecimalNumberEnd(src, offset+1)
		code = at(src, offset)
	}
	if code == 0x002E && IsDigit(at(src, offset+1)) {
		offset += 2
		offset = FindDecimalNumberEnd(src, offset)
	}
	if CmpChar(src, offset, 101) {
		sign := 0
		code = at(src, offset+1)
		if code == 0x002D || code == 0x002B {
			sign = 1
			code = at(src, offset+2)
		}
		if IsDigit(code) {
			offset = FindDecimalNumberEnd(src, offset+1+sign+1)
		}
	}
	return offset
}

// ConsumeBadURLRemnants consumes the rest of a bad url up to and including ")" and returns the offset after it
// (§ 4.3.14. Consume the remnants of a bad url).
func ConsumeBadURLRemnants(src []uint16, offset int) int {
	for ; offset < len(src); offset++ {
		code := at(src, offset)
		if code == 0x0029 {
			offset++
			break
		}
		if IsValidEscape(code, charCode(src, offset+1)) {
			offset = ConsumeEscaped(src, offset)
		}
	}
	return offset
}

// DecodeEscaped returns the code point an escape body (the text after the backslash) stands for: a single
// non-hex code unit stands for itself, hex digits for their code point, with zero, surrogates and values over
// U+10FFFF replaced by U+FFFD.
func DecodeEscaped(escaped string) string {
	units := js.UTF16(escaped)
	if len(units) == 1 && !IsHexDigit(int(units[0])) {
		return js.FromUTF16(units[:1])
	}
	code := js.ParseInt(escaped, 16)
	if math.IsNaN(code) {
		return "\uFFFD"
	}
	if code == 0 || (code >= 0xD800 && code <= 0xDFFF) || code > 0x10FFFF {
		code = 0xFFFD
	}
	return js.FromCodePoint(int(code))
}
