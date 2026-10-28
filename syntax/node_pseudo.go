package syntax

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
)

func (p *parser) attributeName() *js.Object {
	if p.EOF {
		p.Error("Unexpected end of input")
	}
	start := p.TokenStart
	expectIdent := false
	if p.IsDelim('*', 0) {
		expectIdent = true
		p.Next()
	} else if !p.IsDelim('|', 0) {
		p.Eat(t.Ident)
	}
	if p.IsDelim('|', 0) {
		if p.CharCodeAt(p.TokenStart+1) != '=' {
			p.Next()
			p.Eat(t.Ident)
		} else if expectIdent {
			p.ErrorAt("Identifier is expected", p.TokenEnd)
		}
	} else if expectIdent {
		p.Error("Vertical line is expected")
	}
	return js.NewObject("type", "Identifier", "loc", p.GetLocation(start, p.TokenStart), "name", p.SubstrToCursor(start))
}

func (p *parser) attributeOperator() string {
	start := p.TokenStart
	code := p.CharCodeAt(start)
	if code != '=' && code != '~' && code != '^' && code != '$' && code != '*' && code != '|' {
		p.Error("Attribute selector (=, ~=, ^=, $=, *=, |=) is expected")
	}
	p.Next()
	if code != '=' {
		if !p.IsDelim('=', 0) {
			p.Error("Equal sign is expected")
		}
		p.Next()
	}
	return p.SubstrToCursor(start)
}

// AttributeSelector parses "[name]", "[name op value flags]" with namespace prefixes.
func (p *parser) AttributeSelector() *js.Object {
	start := p.TokenStart
	var matcher, value, flags any
	p.Eat(t.LeftSquareBracket)
	p.SkipSC()
	name := p.attributeName()
	p.SkipSC()
	if p.TokenType != t.RightSquareBracket {
		if p.TokenType != t.Ident {
			matcher = p.attributeOperator()
			p.SkipSC()
			if p.TokenType == t.String {
				value = p.String()
			} else {
				value = p.Identifier()
			}
			p.SkipSC()
		}
		if p.TokenType == t.Ident {
			flags = p.Consume(t.Ident)
			p.SkipSC()
		}
	}
	p.Eat(t.RightSquareBracket)
	return js.NewObject("type", "AttributeSelector", "loc", p.GetLocation(start, p.TokenStart), "name", name,
		"matcher", matcher, "value", value, "flags", flags)
}

var pseudoHandlers map[string]func(p *parser) *ast.List

func init() {
	selectorList := func(p *parser) *ast.List { return p.CreateSingleNodeList(p.SelectorList()) }
	selector := func(p *parser) *ast.List { return p.CreateSingleNodeList(p.Selector()) }
	nth := func(p *parser) *ast.List { return p.CreateSingleNodeList(p.Nth()) }
	pseudoHandlers = map[string]func(p *parser) *ast.List{
		"dir":              func(p *parser) *ast.List { return p.CreateSingleNodeList(p.Identifier()) },
		"has":              selectorList,
		"lang":             languageRangeList,
		"matches":          selectorList,
		"is":               selectorList,
		"-moz-any":         selectorList,
		"-webkit-any":      selectorList,
		"where":            selectorList,
		"not":              selectorList,
		"nth-child":        nth,
		"nth-last-child":   nth,
		"nth-last-of-type": nth,
		"nth-of-type":      nth,
		"slotted":          selector,
		"host":             selector,
		"host-context":     selector,
	}
}

func languageRangeList(p *parser) *ast.List {
	children := p.CreateList()
	p.SkipSC()
loop:
	for !p.EOF {
		switch p.TokenType {
		case t.Ident:
			children.Push(p.Identifier())
		case t.String:
			children.Push(p.String())
		case t.Comma:
			children.Push(p.Operator())
		case t.RightParenthesis:
			break loop
		default:
			p.Error("Identifier, string or comma is expected")
		}
		p.SkipSC()
	}
	return children
}

func (p *parser) pseudoSelector(typ string, colons int) *js.Object {
	start := p.TokenStart
	var children any
	var name string
	for i := 0; i < colons; i++ {
		p.Eat(t.Colon)
	}
	if p.TokenType == t.Function {
		name = p.ConsumeFunctionName()
		nameLowerCase := js.ToLowerCase(name)
		if handler, ok := pseudoHandlers[nameLowerCase]; p.LookupNonWSType(0) == t.RightParenthesis {
			children = p.CreateList()
		} else if ok {
			p.SkipSC()
			children = handler(p)
			p.SkipSC()
		} else {
			list := p.CreateList()
			list.Push(p.Raw(nil, false))
			children = list
		}
		p.Eat(t.RightParenthesis)
	} else {
		name = p.Consume(t.Ident)
	}
	return js.NewObject("type", typ, "loc", p.GetLocation(start, p.TokenStart), "name", name, "children", children)
}

// PseudoClassSelector parses ":name" or ":name( ... )".
func (p *parser) PseudoClassSelector() *js.Object {
	return p.pseudoSelector("PseudoClassSelector", 1)
}

// PseudoElementSelector parses "::name" or "::name( ... )".
func (p *parser) PseudoElementSelector() *js.Object {
	return p.pseudoSelector("PseudoElementSelector", 2)
}

// Nth parses the argument of :nth-*(): an+b, odd or even, with an optional "of <selector-list>".
func (p *parser) Nth() *js.Object {
	p.SkipSC()
	start := p.TokenStart
	var selector any
	var nth *js.Object
	if p.LookupValue(0, "odd") || p.LookupValue(0, "even") {
		nth = p.Identifier()
	} else {
		nth = p.AnPlusB()
	}
	end := p.TokenStart
	p.SkipSC()
	if p.LookupValue(0, "of") {
		p.Next()
		selector = p.SelectorList()
		end = p.TokenStart
	}
	return js.NewObject("type", "Nth", "loc", p.GetLocation(start, end), "nth", nth, "selector", selector)
}
