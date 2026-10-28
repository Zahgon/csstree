package syntax

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
	"github.com/csstree/csstree/utils"
)

// StyleSheet parses a stylesheet: rules, at-rules, CDO, CDC and top-level /*! comments.
func (p *parser) StyleSheet() *js.Object {
	start := p.TokenStart
	children := p.CreateList()
	consumeRaw := func() *js.Object { return p.Raw(nil, false) }
	for !p.EOF {
		var child *js.Object
		switch p.TokenType {
		case t.WhiteSpace:
			p.Next()
			continue
		case t.Comment:
			if p.CharCodeAt(p.TokenStart+2) != '!' {
				p.Next()
				continue
			}
			child = p.Comment()
		case t.CDO:
			child = p.CDO()
		case t.CDC:
			child = p.CDC()
		case t.AtKeyword:
			child = p.ParseWithFallback(func() *js.Object { return p.Atrule(false) }, consumeRaw)
		default:
			child = p.ParseWithFallback(p.Rule, consumeRaw)
		}
		children.Push(child)
	}
	return js.NewObject("type", "StyleSheet", "loc", p.GetLocation(start, p.TokenStart), "children", children)
}

// Rule parses a qualified rule: a selector list prelude and a declaration block.
func (p *parser) Rule() *js.Object {
	startOffset := p.TokenStart
	consumeRaw := func() *js.Object { return p.Raw(consumeUntilLeftCurlyBracket, true) }
	var prelude *js.Object
	if p.ParseRulePrelude {
		prelude = p.ParseWithFallback(func() *js.Object {
			prelude := p.SelectorList()
			if prelude.Str("type") != "Raw" && !p.EOF && p.TokenType != t.LeftCurlyBracket {
				p.Error("")
			}
			return prelude
		}, consumeRaw)
	} else {
		prelude = consumeRaw()
	}
	block := p.Block(true)
	return js.NewObject("type", "Rule", "loc", p.GetLocation(startOffset, p.TokenStart), "prelude", prelude, "block", block)
}

// Block parses a { } block: declarations (and nested rules starting with &) for a style block, rules otherwise.
func (p *parser) Block(isStyleBlock bool) *js.Object {
	consumeRaw := func() *js.Object { return p.Raw(nil, true) }
	consumeRule := func() *js.Object { return p.ParseWithFallback(p.Rule, consumeRaw) }
	consumeRawDeclaration := func() *js.Object { return p.Raw(consumeUntilSemicolonIncluded, true) }
	consumeDeclaration := func() *js.Object {
		if p.TokenType == t.Semicolon {
			return consumeRawDeclaration()
		}
		node := p.ParseWithFallback(p.Declaration, consumeRawDeclaration)
		if p.TokenType == t.Semicolon {
			p.Next()
		}
		return node
	}
	consumer := consumeRule
	if isStyleBlock {
		consumer = consumeDeclaration
	}
	start := p.TokenStart
	children := p.CreateList()
	p.Eat(t.LeftCurlyBracket)
scan:
	for !p.EOF {
		switch p.TokenType {
		case t.RightCurlyBracket:
			break scan
		case t.WhiteSpace, t.Comment:
			p.Next()
		case t.AtKeyword:
			children.Push(p.ParseWithFallback(func() *js.Object { return p.Atrule(isStyleBlock) }, consumeRaw))
		default:
			if isStyleBlock && p.IsDelim('&', 0) {
				children.Push(consumeRule())
			} else {
				children.Push(consumer())
			}
		}
	}
	if !p.EOF {
		p.Eat(t.RightCurlyBracket)
	}
	return js.NewObject("type", "Block", "loc", p.GetLocation(start, p.TokenStart), "children", children)
}

// DeclarationList parses declarations, at-rules and nested rules without surrounding braces.
func (p *parser) DeclarationList() *js.Object {
	children := p.CreateList()
	consumeRaw := func() *js.Object { return p.Raw(consumeUntilSemicolonIncluded, true) }
	for !p.EOF {
		switch p.TokenType {
		case t.WhiteSpace, t.Comment, t.Semicolon:
			p.Next()
		case t.AtKeyword:
			children.Push(p.ParseWithFallback(func() *js.Object { return p.Atrule(true) }, consumeRaw))
		default:
			if p.IsDelim('&', 0) {
				children.Push(p.ParseWithFallback(p.Rule, consumeRaw))
			} else {
				children.Push(p.ParseWithFallback(p.Declaration, consumeRaw))
			}
		}
	}
	return js.NewObject("type", "DeclarationList", "loc", p.GetLocationFromList(children), "children", children)
}

// Raw consumes tokens until consumeUntil stops (nil: the end of the enclosing block) and returns them as text;
// excludeWhiteSpace leaves trailing white space out.
func (p *parser) Raw(consumeUntil func(int) int, excludeWhiteSpace bool) *js.Object {
	startOffset := p.GetTokenStart(p.TokenIndex)
	if consumeUntil == nil {
		consumeUntil = consumeUntilBalanceEnd
	}
	p.SkipUntilBalanced(p.TokenIndex, consumeUntil)
	endOffset := p.TokenStart
	if excludeWhiteSpace && p.TokenStart > startOffset {
		endOffset = p.offsetExcludeWS()
	}
	return js.NewObject("type", "Raw", "loc", p.GetLocation(startOffset, endOffset),
		"value", p.Substring(startOffset, endOffset))
}

func (p *parser) offsetExcludeWS() int {
	if p.TokenIndex > 0 && p.LookupType(-1) == t.WhiteSpace {
		if p.TokenIndex > 1 {
			return p.GetTokenStart(p.TokenIndex - 1)
		}
		return p.FirstCharOffset
	}
	return p.TokenStart
}

// Comment parses a comment token.
func (p *parser) Comment() *js.Object {
	start, end := p.TokenStart, p.TokenEnd
	p.Eat(t.Comment)
	if end-start+2 >= 2 && p.CharCodeAt(end-2) == '*' && p.CharCodeAt(end-1) == '/' {
		end -= 2
	}
	return js.NewObject("type", "Comment", "loc", p.GetLocation(start, p.TokenStart), "value", p.Substring(start+2, end))
}

// spaceNode is the single WhiteSpace node every white space parses to, shared like the original's frozen object.
var spaceNode = js.NewObject("type", "WhiteSpace", "loc", nil, "value", " ")

// WhiteSpace parses white space as the shared single-space node.
func (p *parser) WhiteSpace() *js.Object {
	p.Eat(t.WhiteSpace)
	return spaceNode
}

// CDO parses "<!--".
func (p *parser) CDO() *js.Object {
	start := p.TokenStart
	p.Eat(t.CDO)
	return js.NewObject("type", "CDO", "loc", p.GetLocation(start, p.TokenStart))
}

// CDC parses "-->".
func (p *parser) CDC() *js.Object {
	start := p.TokenStart
	p.Eat(t.CDC)
	return js.NewObject("type", "CDC", "loc", p.GetLocation(start, p.TokenStart))
}

// Value parses a declaration value with the value scope.
func (p *parser) Value() *js.Object {
	start := p.TokenStart
	children := p.ReadSequence(valueScope)
	return js.NewObject("type", "Value", "loc", p.GetLocation(start, p.TokenStart), "children", children)
}

func appendSpaceIfEmpty(p *parser, value *js.Object, fromToken int) {
	list, ok := value.Get("children").(*ast.List)
	if value.Str("type") != "Value" || !ok || !list.IsEmpty() {
		return
	}
	for offset := fromToken - p.TokenIndex; offset <= 0; offset++ {
		if p.LookupType(offset) == t.WhiteSpace {
			list.AppendData(js.NewObject("type", "WhiteSpace", "loc", nil, "value", " "))
			return
		}
	}
}

// Declaration parses "property: value !important", with property hacks and custom properties.
func (p *parser) Declaration() *js.Object {
	start := p.TokenStart
	startToken := p.TokenIndex
	property := p.readProperty()
	customProperty := utils.IsCustomProperty(property, 0)
	parseValue := p.ParseValue
	consumeRaw := func() *js.Object { return p.Raw(consumeUntilExclamationMarkOrSemicolon, true) }
	if customProperty {
		parseValue = p.ParseCustomProperty
		consumeRaw = func() *js.Object { return p.Raw(consumeUntilExclamationMarkOrSemicolon, false) }
	}
	var important any = false
	p.SkipSC()
	p.Eat(t.Colon)
	valueStart := p.TokenIndex
	if !customProperty {
		p.SkipSC()
	}
	var value *js.Object
	if parseValue {
		value = p.ParseWithFallback(func() *js.Object {
			startValueToken := p.TokenIndex
			value := p.Value()
			if value.Str("type") != "Raw" && !p.EOF && p.TokenType != t.Semicolon && !p.IsDelim('!', 0) &&
				!p.IsBalanceEdge(startValueToken) {
				p.Error("")
			}
			return value
		}, consumeRaw)
	} else {
		value = consumeRaw()
	}
	if customProperty {
		appendSpaceIfEmpty(p, value, valueStart)
	}
	if p.IsDelim('!', 0) {
		important = p.important()
		p.SkipSC()
	}
	if !p.EOF && p.TokenType != t.Semicolon && !p.IsBalanceEdge(startToken) {
		p.Error("")
	}
	return js.NewObject("type", "Declaration", "loc", p.GetLocation(start, p.TokenStart), "important", important,
		"property", property, "value", value)
}

func (p *parser) readProperty() string {
	start := p.TokenStart
	if p.TokenType == t.Delim {
		switch p.CharCodeAt(p.TokenStart) {
		case '*', '$', '+', '#', '&':
			p.Next()
		case '/':
			p.Next()
			if p.IsDelim('/', 0) {
				p.Next()
			}
		}
	}
	if p.TokenType == t.Hash {
		p.Eat(t.Hash)
	} else {
		p.Eat(t.Ident)
	}
	return p.SubstrToCursor(start)
}

// important parses "! ws* ident" and returns true for "important", else the identifier's text.
func (p *parser) important() any {
	p.Eat(t.Delim)
	p.SkipSC()
	important := p.Consume(t.Ident)
	if important == "important" {
		return true
	}
	return important
}
