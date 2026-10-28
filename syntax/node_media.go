package syntax

import (
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
)

// MediaQueryList parses comma-separated media queries.
func (p *Parser) MediaQueryList() *js.Object {
	children := p.CreateList()
	p.SkipSC()
	for !p.EOF {
		children.Push(p.MediaQuery())
		if p.TokenType != t.Comma {
			break
		}
		p.Next()
	}
	return js.NewObject("type", "MediaQueryList", "loc", p.GetLocationFromList(children), "children", children)
}

// MediaQuery parses "[not|only] type [and condition]" or a condition.
func (p *Parser) MediaQuery() *js.Object {
	start := p.TokenStart
	var modifier, mediaType, condition any
	p.SkipSC()
	if p.TokenType == t.Ident && p.LookupTypeNonSC(1) != t.LeftParenthesis {
		ident := p.Consume(t.Ident)
		identLowerCase := js.ToLowerCase(ident)
		if identLowerCase == "not" || identLowerCase == "only" {
			p.SkipSC()
			modifier = identLowerCase
			mediaType = p.Consume(t.Ident)
		} else {
			mediaType = ident
		}
		switch p.LookupTypeNonSC(0) {
		case t.Ident:
			p.SkipSC()
			p.EatIdent("and")
			condition = p.Condition("media")
		case t.LeftCurlyBracket, t.Semicolon, t.Comma, t.EOF:
		default:
			p.Error("Identifier or parenthesis is expected")
		}
	} else {
		switch p.TokenType {
		case t.Ident, t.LeftParenthesis, t.Function:
			condition = p.Condition("media")
		case t.LeftCurlyBracket, t.Semicolon, t.EOF:
		default:
			p.Error("Identifier or parenthesis is expected")
		}
	}
	return js.NewObject("type", "MediaQuery", "loc", p.GetLocation(start, p.TokenStart), "modifier", modifier,
		"mediaType", mediaType, "condition", condition)
}

func (p *Parser) featureOrRange(kind string) *js.Object {
	if p.LookupTypeNonSC(1) == t.Ident {
		switch p.LookupTypeNonSC(2) {
		case t.Colon, t.RightParenthesis, t.EOF:
			return p.Feature(kind)
		}
	}
	return p.FeatureRange(kind)
}

func (p *Parser) conditionParentheses(kind string) *js.Object {
	if kind == "supports" {
		return p.SupportsDeclaration()
	}
	return p.featureOrRange(kind)
}

// Condition parses a media, container or supports condition of the given kind.
func (p *Parser) Condition(kind string) *js.Object {
	children := p.CreateList()
	nothing := func() *js.Object { return nil }
scan:
	for !p.EOF {
		switch p.TokenType {
		case t.Comment, t.WhiteSpace:
			p.Next()
			continue
		case t.Ident:
			children.Push(p.Identifier())
		case t.LeftParenthesis:
			term := p.ParseWithFallback(func() *js.Object { return p.conditionParentheses(kind) }, nothing)
			if term == nil {
				term = p.ParseWithFallback(func() *js.Object {
					p.Eat(t.LeftParenthesis)
					res := p.Condition(kind)
					p.Eat(t.RightParenthesis)
					return res
				}, func() *js.Object { return p.GeneralEnclosed(kind) })
			}
			children.Push(term)
		case t.Function:
			term := p.ParseWithFallback(func() *js.Object { return p.FeatureFunction(kind) }, nothing)
			if term == nil {
				term = p.GeneralEnclosed(kind)
			}
			children.Push(term)
		default:
			break scan
		}
	}
	if children.IsEmpty() {
		p.Error("Condition is expected")
	}
	return js.NewObject("type", "Condition", "loc", p.GetLocationFromList(children), "kind", kind, "children", children)
}

// Feature parses "( name [: value] )".
func (p *Parser) Feature(kind string) *js.Object {
	start := p.TokenStart
	var value any
	p.Eat(t.LeftParenthesis)
	p.SkipSC()
	name := p.Consume(t.Ident)
	p.SkipSC()
	if p.TokenType != t.RightParenthesis {
		p.Eat(t.Colon)
		p.SkipSC()
		switch p.TokenType {
		case t.Number:
			if p.LookupNonWSType(1) == t.Delim {
				value = p.Ratio()
			} else {
				value = p.Number()
			}
		case t.Dimension:
			value = p.Dimension()
		case t.Ident:
			value = p.Identifier()
		case t.Function:
			value = p.ParseWithFallback(func() *js.Object {
				res := p.Function(ValueScope)
				p.SkipSC()
				if p.IsDelim('/', 0) {
					p.Error("")
				}
				return res
			}, p.Ratio)
		default:
			p.Error("Number, dimension, ratio or identifier is expected")
		}
		p.SkipSC()
	}
	if !p.EOF {
		p.Eat(t.RightParenthesis)
	}
	return js.NewObject("type", "Feature", "loc", p.GetLocation(start, p.TokenStart), "kind", kind, "name", name,
		"value", value)
}
