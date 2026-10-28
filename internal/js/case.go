package js

import "unicode"

// ToLowerCase is String.prototype.toLowerCase: the full Unicode lowercase mapping, including the unconditional
// SpecialCasing entry (U+0130 becomes "i" followed by U+0307) and the Final_Sigma rule for U+03A3. Lone
// surrogates are kept.
func ToLowerCase(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
		if s[i] >= 'A' && s[i] <= 'Z' {
			ascii = false
		}
	}
	if ascii {
		return s
	}
	runes := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		r, size := decodeRune(s, i)
		runes = append(runes, r)
		i += size
	}
	out := make([]byte, 0, len(s)+4)
	for i, r := range runes {
		switch {
		case r < 0x80:
			if r >= 'A' && r <= 'Z' {
				r += 'a' - 'A'
			}
			out = append(out, byte(r))
		case r == 0x0130:
			out = append(out, 'i', 0xCC, 0x87)
		case r == 0x03A3:
			if isFinalSigma(runes, i) {
				out = appendRune(out, 0x03C2)
			} else {
				out = appendRune(out, 0x03C3)
			}
		case r >= 0xD800 && r <= 0xDFFF:
			out = appendRune(out, r)
		default:
			out = appendRune(out, unicode.ToLower(r))
		}
	}
	return string(out)
}

// isFinalSigma applies the Final_Sigma condition: a cased letter (skipping case-ignorable characters) before the
// sigma and none after it.
func isFinalSigma(runes []rune, at int) bool {
	before := false
	for i := at - 1; i >= 0; i-- {
		if isCaseIgnorable(runes[i]) {
			continue
		}
		before = isCased(runes[i])
		break
	}
	if !before {
		return false
	}
	for i := at + 1; i < len(runes); i++ {
		if isCaseIgnorable(runes[i]) {
			continue
		}
		return !isCased(runes[i])
	}
	return true
}

func isCased(r rune) bool {
	return unicode.IsLower(r) || unicode.IsUpper(r) || unicode.IsTitle(r) ||
		unicode.Is(unicode.Other_Lowercase, r) || unicode.Is(unicode.Other_Uppercase, r)
}

func isCaseIgnorable(r rune) bool {
	switch r {
	case 0x0027, 0x002E, 0x003A, 0x00B7, 0x0387, 0x055F, 0x05F4, 0x2018, 0x2019, 0x2024, 0x2027, 0xFE13, 0xFE52,
		0xFE55, 0xFF07, 0xFF0E, 0xFF1A:
		return true
	}
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Lm, unicode.Sk)
}
