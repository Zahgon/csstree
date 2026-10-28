package js

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const hexDigits = "0123456789abcdef"

// Stringify is JSON.stringify(value) without indentation.
func Stringify(value any) string {
	return StringifyIndent(value, "")
}

// StringifyIndent is JSON.stringify(value, null, indent). A top-level undefined value gives "".
func StringifyIndent(value any, indent string) string {
	var b strings.Builder
	writeValue(&b, value, indent, "")
	return b.String()
}

// Quote writes a string as JSON.stringify does: the escapes \" \\ \b \f \n \r \t, \u00XX for other control
// characters, \uXXXX (lowercase hex) for lone surrogates, and every other character as itself.
func Quote(s string) string {
	var b strings.Builder
	quoteTo(&b, s)
	return b.String()
}

func quoteTo(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c < 0x80 && c != '"' && c != '\\' {
			b.WriteByte(c)
			i++
			continue
		}
		if c < 0x80 {
			switch c {
			case '"':
				b.WriteString(`\"`)
			case '\\':
				b.WriteString(`\\`)
			case '\b':
				b.WriteString(`\b`)
			case '\f':
				b.WriteString(`\f`)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				b.WriteString(`\u00`)
				b.WriteByte(hexDigits[c>>4])
				b.WriteByte(hexDigits[c&0xF])
			}
			i++
			continue
		}
		r, size := decodeRune(s, i)
		if r >= 0xD800 && r <= 0xDBFF && i+size < len(s) {
			if next, nsize := decodeRune(s, i+size); next >= 0xDC00 && next <= 0xDFFF {
				b.WriteString(string(rune(0x10000 + (r-0xD800)<<10 + next - 0xDC00)))
				i += size + nsize
				continue
			}
		}
		if r >= 0xD800 && r <= 0xDFFF {
			b.WriteString(`\u`)
			for shift := 12; shift >= 0; shift -= 4 {
				b.WriteByte(hexDigits[(r>>uint(shift))&0xF])
			}
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
}

func numberJSON(f float64) string {
	if f != f || math.IsInf(f, 0) {
		return "null"
	}
	return NumberToString(f)
}

// skipped reports whether JSON.stringify leaves a value out of an object (undefined and functions).
func skipped(v any) bool {
	if v == Undefined {
		return true
	}
	_, isFunc := v.(func())
	return isFunc
}

func writeValue(b *strings.Builder, v any, indent, prefix string) {
	if j, ok := v.(JSONer); ok {
		v = j.ToJSON()
	}
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case undefinedValue:
		if prefix == "" && b.Len() == 0 {
			return
		}
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case float64:
		b.WriteString(numberJSON(x))
	case int:
		b.WriteString(strconv.Itoa(x))
	case int64:
		b.WriteString(numberJSON(float64(x)))
	case uint32:
		b.WriteString(strconv.FormatUint(uint64(x), 10))
	case string:
		quoteTo(b, x)
	case *Object:
		writeObject(b, x, indent, prefix)
	case []any:
		writeArray(b, len(x), func(i int) any { return x[i] }, indent, prefix)
	case []*Object:
		writeArray(b, len(x), func(i int) any { return x[i] }, indent, prefix)
	case []string:
		writeArray(b, len(x), func(i int) any { return x[i] }, indent, prefix)
	case []float64:
		writeArray(b, len(x), func(i int) any { return x[i] }, indent, prefix)
	default:
		panic(fmt.Sprintf("js: cannot stringify %T", v))
	}
}

func writeObject(b *strings.Builder, o *Object, indent, prefix string) {
	if o == nil {
		b.WriteString("null")
		return
	}
	inner := prefix + indent
	first := true
	b.WriteByte('{')
	for _, k := range o.Keys() {
		v := o.vals[k]
		if j, ok := v.(JSONer); ok {
			v = j.ToJSON()
		}
		if skipped(v) {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		first = false
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(inner)
		}
		quoteTo(b, k)
		b.WriteByte(':')
		if indent != "" {
			b.WriteByte(' ')
		}
		writeValue(b, v, indent, inner)
	}
	if !first && indent != "" {
		b.WriteByte('\n')
		b.WriteString(prefix)
	}
	b.WriteByte('}')
}

func writeArray(b *strings.Builder, n int, at func(int) any, indent, prefix string) {
	inner := prefix + indent
	b.WriteByte('[')
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(inner)
		}
		v := at(i)
		if j, ok := v.(JSONer); ok {
			v = j.ToJSON()
		}
		if skipped(v) {
			b.WriteString("null")
			continue
		}
		writeValue(b, v, indent, inner)
	}
	if n > 0 && indent != "" {
		b.WriteByte('\n')
		b.WriteString(prefix)
	}
	b.WriteByte(']')
}

// ParseJSON is JSON.parse: objects become *Object (a repeated key keeps its first position and its last value),
// arrays []any, numbers float64, and \u escapes may produce lone surrogates.
func ParseJSON(text string) (any, error) {
	p := &jsonParser{s: text}
	p.space()
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.i < len(p.s) {
		return nil, p.fail("unexpected data after the JSON value")
	}
	return v, nil
}

type jsonParser struct {
	s string
	i int
}

func (p *jsonParser) fail(msg string) error {
	return fmt.Errorf("invalid JSON at position %d: %s", p.i, msg)
}

func (p *jsonParser) space() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t' || p.s[p.i] == '\n' || p.s[p.i] == '\r') {
		p.i++
	}
}

func (p *jsonParser) value() (any, error) {
	if p.i >= len(p.s) {
		return nil, p.fail("unexpected end of input")
	}
	switch c := p.s[p.i]; {
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '"':
		return p.str()
	case c == '-' || (c >= '0' && c <= '9'):
		return p.number()
	case strings.HasPrefix(p.s[p.i:], "true"):
		p.i += 4
		return true, nil
	case strings.HasPrefix(p.s[p.i:], "false"):
		p.i += 5
		return false, nil
	case strings.HasPrefix(p.s[p.i:], "null"):
		p.i += 4
		return nil, nil
	}
	return nil, p.fail("unexpected token")
}

func (p *jsonParser) object() (any, error) {
	o := NewObject()
	p.i++
	p.space()
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		return o, nil
	}
	for {
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != '"' {
			return nil, p.fail("expected a property name")
		}
		k, err := p.str()
		if err != nil {
			return nil, err
		}
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			return nil, p.fail("expected ':'")
		}
		p.i++
		p.space()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		o.Set(k, v)
		p.space()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.i < len(p.s) && p.s[p.i] == '}' {
			p.i++
			return o, nil
		}
		return nil, p.fail("expected ',' or '}'")
	}
}

func (p *jsonParser) array() (any, error) {
	a := []any{}
	p.i++
	p.space()
	if p.i < len(p.s) && p.s[p.i] == ']' {
		p.i++
		return a, nil
	}
	for {
		p.space()
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		a = append(a, v)
		p.space()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
			continue
		}
		if p.i < len(p.s) && p.s[p.i] == ']' {
			p.i++
			return a, nil
		}
		return nil, p.fail("expected ',' or ']'")
	}
}

func (p *jsonParser) number() (any, error) {
	start := p.i
	if p.s[p.i] == '-' {
		p.i++
	}
	digits := func() int {
		n := 0
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
			n++
		}
		return n
	}
	if p.i < len(p.s) && p.s[p.i] == '0' {
		p.i++
	} else if digits() == 0 {
		return nil, p.fail("invalid number")
	}
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		if digits() == 0 {
			return nil, p.fail("invalid number")
		}
	}
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		p.i++
		if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
			p.i++
		}
		if digits() == 0 {
			return nil, p.fail("invalid number")
		}
	}
	return parseDecimal(p.s[start:p.i]), nil
}

func (p *jsonParser) str() (string, error) {
	p.i++
	var units []uint16
	var out []byte
	flush := func() {
		if units != nil {
			out = append(out, FromUTF16(units)...)
			units = nil
		}
	}
	for {
		if p.i >= len(p.s) {
			return "", p.fail("unterminated string")
		}
		c := p.s[p.i]
		switch {
		case c == '"':
			p.i++
			flush()
			return string(out), nil
		case c < 0x20:
			return "", p.fail("control character in string")
		case c == '\\':
			if p.i+1 >= len(p.s) {
				return "", p.fail("unterminated string")
			}
			e := p.s[p.i+1]
			p.i += 2
			if e == 'u' {
				if p.i+4 > len(p.s) {
					return "", p.fail("bad unicode escape")
				}
				n, err := strconv.ParseUint(p.s[p.i:p.i+4], 16, 16)
				if err != nil {
					return "", p.fail("bad unicode escape")
				}
				p.i += 4
				units = append(units, uint16(n))
				continue
			}
			flush()
			switch e {
			case '"', '\\', '/':
				out = append(out, e)
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			default:
				return "", p.fail("bad escape")
			}
		default:
			flush()
			r, size := decodeRune(p.s, p.i)
			out = appendRune(out, r)
			p.i += size
		}
	}
}

// ErrNotObject is returned by ParseObject when the JSON text is valid but not an object.
var ErrNotObject = errors.New("JSON value is not an object")

// ParseObject parses JSON text whose top-level value must be an object.
func ParseObject(text string) (*Object, error) {
	v, err := ParseJSON(text)
	if err != nil {
		return nil, err
	}
	o, ok := v.(*Object)
	if !ok {
		return nil, ErrNotObject
	}
	return o, nil
}
