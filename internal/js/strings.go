package js

import (
	"strings"
	"unicode/utf8"
)

const replacementChar = 0xFFFD

// decodeRune decodes one code point of a WTF-8 string: a lone surrogate encoded as three bytes is returned as
// itself, and a byte that starts no valid sequence is returned as U+FFFD with size 1.
func decodeRune(s string, i int) (rune, int) {
	c := s[i]
	if c < utf8.RuneSelf {
		return rune(c), 1
	}
	r, size := utf8.DecodeRuneInString(s[i:])
	if r == utf8.RuneError && size == 1 && c == 0xED && i+2 < len(s) &&
		s[i+1] >= 0xA0 && s[i+1] <= 0xBF && s[i+2] >= 0x80 && s[i+2] <= 0xBF {
		return rune(0xD000) | rune(s[i+1]&0x3F)<<6 | rune(s[i+2]&0x3F), 3
	}
	return r, size
}

// appendRune appends a code point in WTF-8; surrogate code points keep their three-byte form.
func appendRune(b []byte, r rune) []byte {
	if r >= 0xD800 && r <= 0xDFFF {
		return append(b, byte(0xE0|r>>12), byte(0x80|(r>>6)&0x3F), byte(0x80|r&0x3F))
	}
	return utf8.AppendRune(b, r)
}

// UTF16 returns the UTF-16 code units of a string.
func UTF16(s string) []uint16 {
	units := make([]uint16, 0, len(s))
	for i := 0; i < len(s); {
		r, size := decodeRune(s, i)
		i += size
		if r >= 0x10000 {
			r -= 0x10000
			units = append(units, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		} else {
			units = append(units, uint16(r))
		}
	}
	return units
}

// FromUTF16 returns the string made of UTF-16 code units; surrogate pairs become one code point and lone
// surrogates are kept.
func FromUTF16(units []uint16) string {
	b := make([]byte, 0, len(units))
	for i := 0; i < len(units); i++ {
		c := rune(units[i])
		if c >= 0xD800 && c <= 0xDBFF && i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF {
			c = 0x10000 + (c-0xD800)<<10 + rune(units[i+1]) - 0xDC00
			i++
		}
		b = appendRune(b, c)
	}
	return string(b)
}

// Len returns the length of a string in UTF-16 code units.
func Len(s string) int {
	n := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c < 0x80:
			i++
			n++
		default:
			r, size := decodeRune(s, i)
			i += size
			if r >= 0x10000 {
				n += 2
			} else {
				n++
			}
		}
	}
	return n
}

// IsASCII reports whether every byte of s is ASCII, in which case UTF-16 indexes equal byte indexes.
func IsASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// CharCodeAt returns the UTF-16 code unit at index i, or -1 when i is out of range (JavaScript NaN).
func CharCodeAt(s string, i int) int {
	if i < 0 {
		return -1
	}
	if IsASCII(s) {
		if i < len(s) {
			return int(s[i])
		}
		return -1
	}
	units := UTF16(s)
	if i < len(units) {
		return int(units[i])
	}
	return -1
}

func clamp(i, lo, hi int) int {
	if i < lo {
		return lo
	}
	if i > hi {
		return hi
	}
	return i
}

// Substring is String.prototype.substring: indexes are clamped to the string and swapped when start > end.
func Substring(s string, start, end int) string {
	if IsASCII(s) {
		start, end = clamp(start, 0, len(s)), clamp(end, 0, len(s))
		if start > end {
			start, end = end, start
		}
		return s[start:end]
	}
	units := UTF16(s)
	start, end = clamp(start, 0, len(units)), clamp(end, 0, len(units))
	if start > end {
		start, end = end, start
	}
	return FromUTF16(units[start:end])
}

// Slice is String.prototype.slice: negative indexes count from the end and an empty string is returned when
// start >= end.
func Slice(s string, start, end int) string {
	n := Len(s)
	if start < 0 {
		start = max(n+start, 0)
	}
	if end < 0 {
		end = max(n+end, 0)
	}
	start, end = min(start, n), min(end, n)
	if start >= end {
		return ""
	}
	return Substring(s, start, end)
}

// SliceFrom is String.prototype.slice with only a start index.
func SliceFrom(s string, start int) string {
	return Slice(s, start, Len(s))
}

// Substr is String.prototype.substr: a start index (negative counts from the end) and a length.
func Substr(s string, start, length int) string {
	n := Len(s)
	if start < 0 {
		start = max(n+start, 0)
	}
	start = min(start, n)
	end := min(start+max(length, 0), n)
	if start >= end {
		return ""
	}
	return Substring(s, start, end)
}

// IndexOf is String.prototype.indexOf with a start position; it returns a UTF-16 index or -1.
func IndexOf(s, search string, from int) int {
	if IsASCII(s) && IsASCII(search) {
		from = clamp(from, 0, len(s))
		i := strings.Index(s[from:], search)
		if i < 0 {
			return -1
		}
		return i + from
	}
	units, sub := UTF16(s), UTF16(search)
	from = clamp(from, 0, len(units))
	for i := from; i+len(sub) <= len(units); i++ {
		match := true
		for j := range sub {
			if units[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// FromCharCode is String.fromCharCode: each value is truncated to a 16-bit code unit.
func FromCharCode(codes ...int) string {
	units := make([]uint16, len(codes))
	for i, c := range codes {
		units[i] = uint16(c)
	}
	return FromUTF16(units)
}

// FromCodePoint is String.fromCodePoint for a valid code point (0 to 0x10FFFF).
func FromCodePoint(cp int) string {
	return string(appendRune(nil, rune(cp)))
}

// DecodeUTF8 decodes bytes the way Node.js decodes a UTF-8 buffer into a string: every maximal subpart of an
// ill-formed sequence becomes one U+FFFD (the WHATWG Encoding Standard decoder) and a byte order mark is kept.
func DecodeUTF8(b []byte) string {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out = append(out, c)
			i++
			continue
		}
		need, lower, upper := 0, byte(0x80), byte(0xBF)
		var cp rune
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need, cp = 1, rune(c&0x1F)
		case c == 0xE0:
			need, lower, cp = 2, 0xA0, rune(c&0x0F)
		case c == 0xED:
			need, upper, cp = 2, 0x9F, rune(c&0x0F)
		case c >= 0xE1 && c <= 0xEF:
			need, cp = 2, rune(c&0x0F)
		case c == 0xF0:
			need, lower, cp = 3, 0x90, rune(c&0x07)
		case c >= 0xF1 && c <= 0xF3:
			need, cp = 3, rune(c&0x07)
		case c == 0xF4:
			need, upper, cp = 3, 0x8F, rune(c&0x07)
		default:
			out = utf8.AppendRune(out, replacementChar)
			i++
			continue
		}
		j := i + 1
		complete := true
		for k := 0; k < need; k++ {
			if j >= len(b) || b[j] < lower || b[j] > upper {
				complete = false
				break
			}
			cp = cp<<6 | rune(b[j]&0x3F)
			lower, upper = 0x80, 0xBF
			j++
		}
		if complete {
			out = utf8.AppendRune(out, cp)
		} else {
			out = utf8.AppendRune(out, replacementChar)
		}
		i = j
	}
	return string(out)
}

// ToUTF8 returns s as well-formed UTF-8, the bytes Node.js writes for a string: surrogate pairs are joined and
// lone surrogates become U+FFFD.
func ToUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	units := UTF16(s)
	b := make([]byte, 0, len(s))
	for i := 0; i < len(units); i++ {
		c := rune(units[i])
		switch {
		case c >= 0xD800 && c <= 0xDBFF && i+1 < len(units) && units[i+1] >= 0xDC00 && units[i+1] <= 0xDFFF:
			c = 0x10000 + (c-0xD800)<<10 + rune(units[i+1]) - 0xDC00
			i++
		case c >= 0xD800 && c <= 0xDFFF:
			c = replacementChar
		}
		b = utf8.AppendRune(b, c)
	}
	return string(b)
}
