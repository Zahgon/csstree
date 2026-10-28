package syntax

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
)

// atruleHandler parses the prelude and the block of a known at-rule; a nil function uses the generic parser.
type atruleHandler struct {
	prelude func(p *Parser) *ast.List
	block   func(p *Parser, isDeclaration bool) *js.Object
}

func nestedBlock(p *Parser, nested bool) *js.Object { return p.Block(nested) }
func styleBlock(p *Parser, _ bool) *js.Object       { return p.Block(true) }

var nonContainerNameKeywords = map[string]bool{"none": true, "and": true, "not": true, "or": true}

var atruleHandlers map[string]atruleHandler

func init() {
	atruleHandlers = map[string]atruleHandler{
		"container": {prelude: func(p *Parser) *ast.List {
			children := p.CreateList()
			if p.TokenType == t.Ident {
				name := p.Substring(p.TokenStart, p.TokenEnd)
				if !nonContainerNameKeywords[js.ToLowerCase(name)] {
					children.Push(p.Identifier())
				}
			}
			children.Push(p.Condition("container"))
			return children
		}, block: nestedBlock},
		"font-face": {block: styleBlock},
		"import":    {prelude: importPrelude},
		"layer": {prelude: func(p *Parser) *ast.List { return p.CreateSingleNodeList(p.LayerList()) },
			block: func(p *Parser, _ bool) *js.Object { return p.Block(false) }},
		"media": {prelude: func(p *Parser) *ast.List { return p.CreateSingleNodeList(p.MediaQueryList()) },
			block: nestedBlock},
		"nest": {prelude: func(p *Parser) *ast.List { return p.CreateSingleNodeList(p.SelectorList()) },
			block: styleBlock},
		"page": {prelude: func(p *Parser) *ast.List { return p.CreateSingleNodeList(p.SelectorList()) },
			block: styleBlock},
		"scope": {prelude: func(p *Parser) *ast.List { return p.CreateSingleNodeList(p.Scope()) },
			block: nestedBlock},
		"starting-style": {block: nestedBlock},
		"supports": {prelude: func(p *Parser) *ast.List { return p.CreateSingleNodeList(p.Condition("supports")) },
			block: nestedBlock},
	}
}

// importFallback wraps an @import function argument parser: after it runs, successfully or not, white space is
// skipped and anything but ")" is an error (which replaces the parser's own error, like a JavaScript finally).
func (p *Parser) importFallback(parse func() *js.Object, fallback func() *js.Object) *js.Object {
	if fallback == nil {
		fallback = func() *js.Object { return p.Raw(nil, true) }
	}
	return p.ParseWithFallback(func() (node *js.Object) {
		defer func() {
			r := recover()
			p.SkipSC()
			if p.LookupNonWSType(0) != t.RightParenthesis {
				p.Error("")
			}
			if r != nil {
				panic(r)
			}
		}()
		return parse()
	}, fallback)
}

func pushUnlessEmptyRaw(children *ast.List, node *js.Object) {
	if node.Str("type") != "Raw" || node.Str("value") != "" {
		children.Push(node)
	}
}

var importFunctions = &Recognizer{Functions: map[string]func(p *Parser, r *Recognizer) *ast.List{
	"layer": func(p *Parser, _ *Recognizer) *ast.List {
		p.SkipSC()
		children := p.CreateList()
		pushUnlessEmptyRaw(children, p.importFallback(p.Layer, nil))
		return children
	},
	"supports": func(p *Parser, _ *Recognizer) *ast.List {
		p.SkipSC()
		children := p.CreateList()
		node := p.importFallback(p.Declaration, func() *js.Object {
			return p.importFallback(func() *js.Object { return p.Condition("supports") }, nil)
		})
		pushUnlessEmptyRaw(children, node)
		return children
	},
}}

func importPrelude(p *Parser) *ast.List {
	children := p.CreateList()
	switch p.TokenType {
	case t.String:
		children.Push(p.String())
	case t.URL, t.Function:
		children.Push(p.Url())
	default:
		p.Error("String or url() is expected")
	}
	p.SkipSC()
	if p.TokenType == t.Ident && p.CmpStr(p.TokenStart, p.TokenEnd, "layer") {
		children.Push(p.Identifier())
	} else if p.TokenType == t.Function && p.CmpStr(p.TokenStart, p.TokenEnd, "layer(") {
		children.Push(p.Function(importFunctions))
	}
	p.SkipSC()
	if p.TokenType == t.Function && p.CmpStr(p.TokenStart, p.TokenEnd, "supports(") {
		children.Push(p.Function(importFunctions))
	}
	if p.LookupNonWSType(0) == t.Ident || p.LookupNonWSType(0) == t.LeftParenthesis {
		children.Push(p.MediaQueryList())
	}
	return children
}

func (p *Parser) isDeclarationBlockAtrule() bool {
	for offset := 1; ; offset++ {
		typ := p.LookupType(offset)
		if typ == t.EOF {
			return false
		}
		if typ == t.RightCurlyBracket {
			return true
		}
		if typ == t.LeftCurlyBracket || typ == t.AtKeyword {
			return false
		}
	}
}

// Atrule parses an at-rule; isDeclaration is set inside declaration blocks.
func (p *Parser) Atrule(isDeclaration bool) *js.Object {
	start := p.TokenStart
	var prelude, block *js.Object
	consumeRaw := func() *js.Object { return p.Raw(ConsumeUntilLeftCurlyBracketOrSemicolon, true) }
	p.Eat(t.AtKeyword)
	name := p.SubstrToCursor(start + 1)
	nameLowerCase := js.ToLowerCase(name)
	p.SkipSC()
	if !p.EOF && p.TokenType != t.LeftCurlyBracket && p.TokenType != t.Semicolon {
		if p.ParseAtrulePrelude {
			prelude = p.ParseWithFallback(func() *js.Object { return p.AtrulePrelude(name, true) }, consumeRaw)
		} else {
			prelude = consumeRaw()
		}
		p.SkipSC()
	}
	switch p.TokenType {
	case t.Semicolon:
		p.Next()
	case t.LeftCurlyBracket:
		if h, ok := atruleHandlers[nameLowerCase]; ok && h.block != nil {
			block = h.block(p, isDeclaration)
		} else {
			block = p.Block(p.isDeclarationBlockAtrule())
		}
	}
	return js.NewObject("type", "Atrule", "loc", p.GetLocation(start, p.TokenStart), "name", name,
		"prelude", prelude, "block", block)
}

// AtrulePrelude parses the prelude of the named at-rule (hasName false: no name, the generic prelude).
func (p *Parser) AtrulePrelude(name string, hasName bool) *js.Object {
	var children *ast.List
	if hasName {
		name = js.ToLowerCase(name)
	}
	p.SkipSC()
	if h, ok := atruleHandlers[name]; ok && hasName && h.prelude != nil {
		children = h.prelude(p)
	} else {
		children = p.ReadSequence(AtrulePreludeScope)
	}
	p.SkipSC()
	if !p.EOF && p.TokenType != t.LeftCurlyBracket && p.TokenType != t.Semicolon {
		p.Error("Semicolon or block is expected")
	}
	return js.NewObject("type", "AtrulePrelude", "loc", p.GetLocationFromList(children), "children", children)
}
