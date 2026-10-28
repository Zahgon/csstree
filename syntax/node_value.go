package syntax

import (
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
	"github.com/csstree/csstree/utils"
)

// Identifier parses an identifier token.
func (p *Parser) Identifier() *js.Object {
	loc := p.GetLocation(p.TokenStart, p.TokenEnd)
	return js.NewObject("type", "Identifier", "loc", loc, "name", p.Consume(t.Ident))
}

// Number parses a number token.
func (p *Parser) Number() *js.Object {
	loc := p.GetLocation(p.TokenStart, p.TokenEnd)
	return js.NewObject("type", "Number", "loc", loc, "value", p.Consume(t.Number))
}

// Dimension parses a dimension token into its number and unit.
func (p *Parser) Dimension() *js.Object {
	start := p.TokenStart
	value := p.ConsumeNumber(t.Dimension)
	return js.NewObject("type", "Dimension", "loc", p.GetLocation(start, p.TokenStart), "value", value,
		"unit", p.Substring(start+js.Len(value), p.TokenStart))
}

// Percentage parses a percentage token; the value excludes "%".
func (p *Parser) Percentage() *js.Object {
	loc := p.GetLocation(p.TokenStart, p.TokenEnd)
	return js.NewObject("type", "Percentage", "loc", loc, "value", p.ConsumeNumber(t.Percentage))
}

// String parses a string token; the value is decoded.
func (p *Parser) String() *js.Object {
	loc := p.GetLocation(p.TokenStart, p.TokenEnd)
	return js.NewObject("type", "String", "loc", loc, "value", utils.StringDecode(p.Consume(t.String)))
}

// Url parses <url-token> or url( <string> ); the value is decoded.
func (p *Parser) Url() *js.Object {
	start := p.TokenStart
	var value any = js.Undefined
	switch p.TokenType {
	case t.URL:
		value = utils.URLDecode(p.Consume(t.URL))
	case t.Function:
		if !p.CmpStr(p.TokenStart, p.TokenEnd, "url(") {
			p.Error("Function name must be `url`")
		}
		p.Eat(t.Function)
		p.SkipSC()
		value = utils.StringDecode(p.Consume(t.String))
		p.SkipSC()
		if !p.EOF {
			p.Eat(t.RightParenthesis)
		}
	default:
		p.Error("Url or Function is expected")
	}
	return js.NewObject("type", "Url", "loc", p.GetLocation(start, p.TokenStart), "value", value)
}

// Hash parses "#name"; the value excludes "#".
func (p *Parser) Hash() *js.Object {
	start := p.TokenStart
	p.Eat(t.Hash)
	return js.NewObject("type", "Hash", "loc", p.GetLocation(start, p.TokenStart), "value", p.SubstrToCursor(start+1))
}

// Operator parses one token as an operator ("/", "*", ",", ":", "+", "-").
func (p *Parser) Operator() *js.Object {
	start := p.TokenStart
	p.Next()
	return js.NewObject("type", "Operator", "loc", p.GetLocation(start, p.TokenStart), "value", p.SubstrToCursor(start))
}

// Parentheses parses "( sequence )" with the given recognizer.
func (p *Parser) Parentheses(recognizer *Recognizer) *js.Object {
	start := p.TokenStart
	p.Eat(t.LeftParenthesis)
	children := p.ReadSequence(recognizer)
	if !p.EOF {
		p.Eat(t.RightParenthesis)
	}
	return js.NewObject("type", "Parentheses", "loc", p.GetLocation(start, p.TokenStart), "children", children)
}

// Brackets parses "[ sequence ]" with the given recognizer.
func (p *Parser) Brackets(recognizer *Recognizer) *js.Object {
	start := p.TokenStart
	p.Eat(t.LeftSquareBracket)
	children := p.ReadSequence(recognizer)
	if !p.EOF {
		p.Eat(t.RightSquareBracket)
	}
	return js.NewObject("type", "Brackets", "loc", p.GetLocation(start, p.TokenStart), "children", children)
}

// Function parses "name( ... )": special functions of the recognizer parse their own arguments, others are read
// with the recognizer.
func (p *Parser) Function(recognizer *Recognizer) *js.Object {
	start := p.TokenStart
	name := p.ConsumeFunctionName()
	nameLowerCase := js.ToLowerCase(name)
	var children any
	if fn, ok := recognizer.Functions[nameLowerCase]; ok {
		children = fn(p, recognizer)
	} else {
		children = p.ReadSequence(recognizer)
	}
	if !p.EOF {
		p.Eat(t.RightParenthesis)
	}
	return js.NewObject("type", "Function", "loc", p.GetLocation(start, p.TokenStart), "name", name, "children", children)
}
