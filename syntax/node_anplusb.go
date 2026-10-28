package syntax

import (
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
)

func (p *parser) checkInteger(offset int, disallowSign bool) {
	pos := p.TokenStart + offset
	code := p.CharCodeAt(pos)
	if code == '+' || code == '-' {
		if disallowSign {
			p.Error("Number sign is not allowed")
		}
		pos++
	}
	for ; pos < p.TokenEnd; pos++ {
		if !t.IsDigit(p.CharCodeAt(pos)) {
			p.ErrorAt("Integer is expected", pos)
		}
	}
}

func (p *parser) expectCharCode(offset, code int) {
	if !p.CmpChar(p.TokenStart+offset, code) {
		message := ""
		switch code {
		case 'n':
			message = "N is expected"
		case '-':
			message = "HyphenMinus is expected"
		}
		p.ErrorAt(message, p.TokenStart+offset)
	}
}

// consumeB reads the b part after "an": an optional sign token, white space and an integer; nil when absent.
func (p *parser) consumeB() any {
	offset := 0
	sign := 0
	typ := p.TokenType
	for typ == t.WhiteSpace || typ == t.Comment {
		offset++
		typ = p.LookupType(offset)
	}
	if typ != t.Number {
		if p.IsDelim('+', offset) || p.IsDelim('-', offset) {
			sign = '-'
			if p.IsDelim('+', offset) {
				sign = '+'
			}
			for {
				offset++
				typ = p.LookupType(offset)
				if typ != t.WhiteSpace && typ != t.Comment {
					break
				}
			}
			if typ != t.Number {
				p.Skip(offset)
				p.checkInteger(0, true)
			}
		} else {
			return nil
		}
	}
	if offset > 0 {
		p.Skip(offset)
	}
	if sign == 0 {
		code := p.CharCodeAt(p.TokenStart)
		if code != '+' && code != '-' {
			p.Error("Number sign is expected")
		}
	}
	p.checkInteger(0, sign != 0)
	if sign == '-' {
		return "-" + p.Consume(t.Number)
	}
	return p.Consume(t.Number)
}

// AnPlusB parses the an+b microsyntax.
func (p *parser) AnPlusB() *js.Object {
	start := p.TokenStart
	var a, b any
	switch {
	case p.TokenType == t.Number:
		p.checkInteger(0, false)
		b = p.Consume(t.Number)
	case p.TokenType == t.Ident && p.CmpChar(p.TokenStart, '-'):
		a = "-1"
		p.expectCharCode(1, 'n')
		switch p.TokenEnd - p.TokenStart {
		case 2:
			p.Next()
			b = p.consumeB()
		case 3:
			p.expectCharCode(2, '-')
			p.Next()
			p.SkipSC()
			p.checkInteger(0, true)
			b = "-" + p.Consume(t.Number)
		default:
			p.expectCharCode(2, '-')
			p.checkInteger(3, true)
			p.Next()
			b = p.SubstrToCursor(start + 2)
		}
	case p.TokenType == t.Ident || (p.IsDelim('+', 0) && p.LookupType(1) == t.Ident):
		sign := 0
		a = "1"
		if p.IsDelim('+', 0) {
			sign = 1
			p.Next()
		}
		p.expectCharCode(0, 'n')
		switch p.TokenEnd - p.TokenStart {
		case 1:
			p.Next()
			b = p.consumeB()
		case 2:
			p.expectCharCode(1, '-')
			p.Next()
			p.SkipSC()
			p.checkInteger(0, true)
			b = "-" + p.Consume(t.Number)
		default:
			p.expectCharCode(1, '-')
			p.checkInteger(2, true)
			p.Next()
			b = p.SubstrToCursor(start + sign + 1)
		}
	case p.TokenType == t.Dimension:
		code := p.CharCodeAt(p.TokenStart)
		sign := 0
		if code == '+' || code == '-' {
			sign = 1
		}
		i := p.TokenStart + sign
		for ; i < p.TokenEnd; i++ {
			if !t.IsDigit(p.CharCodeAt(i)) {
				break
			}
		}
		if i == p.TokenStart+sign {
			p.ErrorAt("Integer is expected", p.TokenStart+sign)
		}
		p.expectCharCode(i-p.TokenStart, 'n')
		a = p.Substring(start, i)
		if i+1 == p.TokenEnd {
			p.Next()
			b = p.consumeB()
		} else {
			p.expectCharCode(i-p.TokenStart+1, '-')
			if i+2 == p.TokenEnd {
				p.Next()
				p.SkipSC()
				p.checkInteger(0, true)
				b = "-" + p.Consume(t.Number)
			} else {
				p.checkInteger(i-p.TokenStart+2, true)
				p.Next()
				b = p.SubstrToCursor(i + 1)
			}
		}
	default:
		p.Error("")
	}
	if s, ok := a.(string); ok && js.CharCodeAt(s, 0) == '+' {
		a = js.Substr(s, 1, js.Len(s))
	}
	if s, ok := b.(string); ok && js.CharCodeAt(s, 0) == '+' {
		b = js.Substr(s, 1, js.Len(s))
	}
	return js.NewObject("type", "AnPlusB", "loc", p.GetLocation(start, p.TokenStart), "a", a, "b", b)
}
