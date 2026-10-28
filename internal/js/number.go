package js

import (
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// NumberToString is Number.prototype.toString() in base 10: the shortest round-trip digits, plain notation for
// exponents from -7 to 20 and exponential notation (1e+21, 1.5e-7) outside that range.
func NumberToString(f float64) string {
	switch {
	case f != f:
		return "NaN"
	case f == 0:
		return "0"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	}
	sign := ""
	if f < 0 {
		sign, f = "-", -f
	}
	repr := strconv.FormatFloat(f, 'e', -1, 64)
	mantissa, exponent, _ := strings.Cut(repr, "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	e, _ := strconv.Atoi(exponent)
	n, k := e+1, len(digits)
	switch {
	case k <= n && n <= 21:
		return sign + digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		return sign + digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		return sign + "0." + strings.Repeat("0", -n) + digits
	}
	expSign := "+"
	if n-1 < 0 {
		expSign = "-"
	}
	exp := strconv.Itoa(abs(n - 1))
	if k == 1 {
		return sign + digits + "e" + expSign + exp
	}
	return sign + digits[:1] + "." + digits[1:] + "e" + expSign + exp
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// IsWhiteSpace reports whether a code point is JavaScript white space or a line terminator (what String.prototype.trim
// and Number() strip).
func IsWhiteSpace(r rune) bool {
	switch r {
	case 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x20, 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// Trim is String.prototype.trim.
func Trim(s string) string {
	return strings.TrimFunc(s, IsWhiteSpace)
}

// ToNumber is Number(string): white space is trimmed, an empty string is 0, decimal literals with an optional
// sign, fraction and exponent, Infinity with an optional sign, and unsigned 0x, 0o and 0b integers are accepted;
// everything else is NaN. Out-of-range decimals become Infinity or 0 as in JavaScript.
func ToNumber(s string) float64 {
	t := Trim(s)
	if t == "" {
		return 0
	}
	if len(t) > 2 && t[0] == '0' {
		base := 0
		switch t[1] {
		case 'x', 'X':
			base = 16
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if base != 0 {
			n, ok := new(big.Int).SetString(t[2:], base)
			if !ok || strings.ContainsAny(t[2:], "_+-") {
				return math.NaN()
			}
			f, _ := new(big.Float).SetInt(n).Float64()
			return f
		}
	}
	switch t {
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	if decimalPrefix(t) != len(t) {
		return math.NaN()
	}
	return parseDecimal(t)
}

// ParseFloat is the global parseFloat: leading white space is skipped and the longest decimal literal prefix (or
// Infinity) is converted; a string without one is NaN.
func ParseFloat(s string) float64 {
	t := strings.TrimLeftFunc(s, IsWhiteSpace)
	rest := strings.TrimLeft(t, "+-")
	if len(t)-len(rest) <= 1 && strings.HasPrefix(rest, "Infinity") {
		if strings.HasPrefix(t, "-") {
			return math.Inf(-1)
		}
		return math.Inf(1)
	}
	n := decimalPrefix(t)
	if n == 0 {
		return math.NaN()
	}
	return parseDecimal(t[:n])
}

// ParseInt is the global parseInt with an explicit radix from 2 to 36: leading white space and one sign are
// skipped and the longest run of digits valid in the radix is converted; no digits gives NaN.
func ParseInt(s string, radix int) float64 {
	t := strings.TrimLeftFunc(s, IsWhiteSpace)
	negative := false
	if t != "" && (t[0] == '+' || t[0] == '-') {
		negative = t[0] == '-'
		t = t[1:]
	}
	if radix == 16 && len(t) > 1 && t[0] == '0' && (t[1] == 'x' || t[1] == 'X') {
		t = t[2:]
	}
	end := 0
	for end < len(t) && digitValue(t[end]) < radix {
		end++
	}
	if end == 0 {
		return math.NaN()
	}
	n, _ := new(big.Int).SetString(t[:end], radix)
	f, _ := new(big.Float).SetInt(n).Float64()
	if negative {
		return -f
	}
	return f
}

func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		return int(c-'A') + 10
	}
	return 99
}

// decimalPrefix returns the length of the longest prefix of s that is a StrDecimalLiteral without Infinity:
// [+-]? (digits (. digits?)? | . digits) ([eE] [+-]? digits)?
func decimalPrefix(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	intDigits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		intDigits++
	}
	fracDigits := 0
	if i < len(s) && s[i] == '.' {
		j := i + 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
			fracDigits++
		}
		if intDigits > 0 || fracDigits > 0 {
			i = j
		}
	}
	if intDigits == 0 && fracDigits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		expDigits := 0
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
			expDigits++
		}
		if expDigits > 0 {
			i = j
		}
	}
	return i
}

func parseDecimal(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return math.NaN()
	}
	return f
}

// ToUint32 is the ToUint32 conversion a Uint32Array store applies: the integer part modulo 2^32.
func ToUint32(f float64) uint32 {
	if f != f || math.IsInf(f, 0) {
		return 0
	}
	m := math.Mod(math.Trunc(f), 4294967296)
	if m < 0 {
		m += 4294967296
	}
	return uint32(m)
}

// ToInt32 is the ToInt32 conversion of the bitwise operators.
func ToInt32(f float64) int32 {
	return int32(ToUint32(f))
}
