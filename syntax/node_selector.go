package syntax

import (
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
)

// UnicodeRange parses u+0-7F, u+1??, u+0025-00FF and the other unicode-range forms.
func (p *Parser) UnicodeRange() *js.Object {
	start := p.TokenStart
	p.EatIdent("u")
	p.scanUnicodeRange()
	return js.NewObject("type", "UnicodeRange", "loc", p.GetLocation(start, p.TokenStart), "value", p.SubstrToCursor(start))
}

func (p *Parser) eatHexSequence(offset int, allowDash bool) int {
	length := 0
	for pos := p.TokenStart + offset; pos < p.TokenEnd; pos++ {
		code := p.CharCodeAt(pos)
		if code == '-' && allowDash && length != 0 {
			p.eatHexSequence(offset+length+1, false)
			return -1
		}
		if !t.IsHexDigit(code) {
			var message string
			switch {
			case allowDash && length != 0 && length < 6:
				message = "Hyphen minus or hex digit is expected"
			case allowDash && length != 0:
				message = "Hyphen minus is expected"
			case length < 6:
				message = "Hex digit is expected"
			default:
				message = "Unexpected input"
			}
			p.ErrorAt(message, pos)
		}
		length++
		if length > 6 {
			p.ErrorAt("Too many hex digits", pos)
		}
	}
	p.Next()
	return length
}

func (p *Parser) eatQuestionMarkSequence(maxCount int) {
	count := 0
	for p.IsDelim('?', 0) {
		count++
		if count > maxCount {
			p.Error("Too many question marks")
		}
		p.Next()
	}
}

func (p *Parser) unicodeRangeStartsWith(code int) {
	if p.CharCodeAt(p.TokenStart) != code {
		if code == '+' {
			p.Error("Plus sign is expected")
		}
		p.Error("Hyphen minus is expected")
	}
}

func (p *Parser) scanUnicodeRange() {
	switch p.TokenType {
	case t.Number:
		hexLength := p.eatHexSequence(1, true)
		if p.IsDelim('?', 0) {
			p.eatQuestionMarkSequence(6 - hexLength)
			return
		}
		if p.TokenType == t.Dimension || p.TokenType == t.Number {
			p.unicodeRangeStartsWith('-')
			p.eatHexSequence(1, false)
		}
	case t.Dimension:
		if hexLength := p.eatHexSequence(1, true); hexLength > 0 {
			p.eatQuestionMarkSequence(6 - hexLength)
		}
	default:
		p.EatDelim('+')
		if p.TokenType == t.Ident {
			if hexLength := p.eatHexSequence(0, true); hexLength > 0 {
				p.eatQuestionMarkSequence(6 - hexLength)
			}
			return
		}
		if p.IsDelim('?', 0) {
			p.Next()
			p.eatQuestionMarkSequence(5)
			return
		}
		p.Error("Hex digit or question mark is expected")
	}
}

// SelectorList parses comma-separated selectors.
func (p *Parser) SelectorList() *js.Object {
	children := p.CreateList()
	for !p.EOF {
		children.Push(p.Selector())
		if p.TokenType == t.Comma {
			p.Next()
			continue
		}
		break
	}
	return js.NewObject("type", "SelectorList", "loc", p.GetLocationFromList(children), "children", children)
}

// Selector parses one complex selector.
func (p *Parser) Selector() *js.Object {
	children := p.ReadSequence(SelectorScope)
	if children.First() == nil {
		p.Error("Selector is expected")
	}
	return js.NewObject("type", "Selector", "loc", p.GetLocationFromList(children), "children", children)
}

func (p *Parser) eatIdentifierOrAsterisk() {
	if p.TokenType != t.Ident && !p.IsDelim('*', 0) {
		p.Error("Identifier or asterisk is expected")
	}
	p.Next()
}

// TypeSelector parses a type or universal selector with an optional namespace prefix.
func (p *Parser) TypeSelector() *js.Object {
	start := p.TokenStart
	if p.IsDelim('|', 0) {
		p.Next()
		p.eatIdentifierOrAsterisk()
	} else {
		p.eatIdentifierOrAsterisk()
		if p.IsDelim('|', 0) {
			p.Next()
			p.eatIdentifierOrAsterisk()
		}
	}
	return js.NewObject("type", "TypeSelector", "loc", p.GetLocation(start, p.TokenStart), "name", p.SubstrToCursor(start))
}

// IdSelector parses "#name".
func (p *Parser) IdSelector() *js.Object {
	start := p.TokenStart
	p.Eat(t.Hash)
	return js.NewObject("type", "IdSelector", "loc", p.GetLocation(start, p.TokenStart), "name", p.SubstrToCursor(start+1))
}

// ClassSelector parses ".name".
func (p *Parser) ClassSelector() *js.Object {
	p.EatDelim('.')
	loc := p.GetLocation(p.TokenStart-1, p.TokenEnd)
	return js.NewObject("type", "ClassSelector", "loc", loc, "name", p.Consume(t.Ident))
}

// Combinator parses ">", "+", "~" or "/deep/".
func (p *Parser) Combinator() *js.Object {
	start := p.TokenStart
	var name any = js.Undefined
	switch p.TokenType {
	case t.WhiteSpace:
		name = " "
	case t.Delim:
		switch p.CharCodeAt(p.TokenStart) {
		case '>', '+', '~':
			p.Next()
		case '/':
			p.Next()
			p.EatIdent("deep")
			p.EatDelim('/')
		default:
			p.Error("Combinator is expected")
		}
		name = p.SubstrToCursor(start)
	}
	return js.NewObject("type", "Combinator", "loc", p.GetLocation(start, p.TokenStart), "name", name)
}

// NestingSelector parses "&".
func (p *Parser) NestingSelector() *js.Object {
	start := p.TokenStart
	p.EatDelim('&')
	return js.NewObject("type", "NestingSelector", "loc", p.GetLocation(start, p.TokenStart))
}
