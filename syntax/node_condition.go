package syntax

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/js"
	t "github.com/csstree/csstree/tokenizer"
)

func (p *parser) rangeTerm() *js.Object {
	p.SkipSC()
	switch p.TokenType {
	case t.Number:
		if p.IsDelim('/', p.LookupOffsetNonSC(1)) {
			return p.Ratio()
		}
		return p.Number()
	case t.Dimension:
		return p.Dimension()
	case t.Ident:
		return p.Identifier()
	case t.Function:
		return p.ParseWithFallback(func() *js.Object {
			res := p.Function(valueScope)
			p.SkipSC()
			if p.IsDelim('/', 0) {
				p.Error("")
			}
			return res
		}, p.Ratio)
	}
	p.Error("Number, dimension, ratio or identifier is expected")
	return nil
}

func (p *parser) rangeComparison(expectColon bool) string {
	p.SkipSC()
	if p.IsDelim('<', 0) || p.IsDelim('>', 0) {
		value := js.FromUTF16(p.Src[p.TokenStart : p.TokenStart+1])
		p.Next()
		if p.IsDelim('=', 0) {
			p.Next()
			return value + "="
		}
		return value
	}
	if p.IsDelim('=', 0) {
		return "="
	}
	if expectColon {
		p.Error(`Expected ":", "<", ">", "=" or ")"`)
	}
	p.Error(`Expected "<", ">", "=" or ")"`)
	return ""
}

// FeatureRange parses a range feature such as (width >= 600px) or (400px < width < 700px).
func (p *parser) FeatureRange(kind string) *js.Object {
	start := p.TokenStart
	p.SkipSC()
	p.Eat(t.LeftParenthesis)
	left := p.rangeTerm()
	leftComparison := p.rangeComparison(left.Str("type") == "Identifier")
	middle := p.rangeTerm()
	var rightComparison, right any
	if p.LookupNonWSType(0) != t.RightParenthesis {
		rightComparison = p.rangeComparison(false)
		right = p.rangeTerm()
	}
	p.SkipSC()
	p.Eat(t.RightParenthesis)
	return js.NewObject("type", "FeatureRange", "loc", p.GetLocation(start, p.TokenStart), "kind", kind,
		"left", left, "leftComparison", leftComparison, "middle", middle, "rightComparison", rightComparison,
		"right", right)
}

// featureParser returns the parser of a feature function of a condition kind: selector() for supports and
// style() for container; "constructor" resolves to the Object constructor of the original's plain object.
func (p *parser) featureParser(kind, name string) func() *js.Object {
	switch {
	case kind == "supports" && name == "selector":
		return p.Selector
	case kind == "container" && name == "style":
		return p.Declaration
	case name == "constructor":
		return func() *js.Object { return js.NewObject() }
	}
	p.Error("Unknown feature " + name + "()")
	return nil
}

// FeatureFunction parses a feature function such as selector( ... ) in @supports or style( ... ) in @container.
func (p *parser) FeatureFunction(kind string) *js.Object {
	start := p.TokenStart
	functionName := p.ConsumeFunctionName()
	valueParser := p.featureParser(kind, js.ToLowerCase(functionName))
	p.SkipSC()
	value := p.ParseWithFallback(func() *js.Object {
		startValueToken := p.TokenIndex
		value := valueParser()
		if !p.EOF && !p.IsBalanceEdge(startValueToken) {
			p.Error("")
		}
		return value
	}, func() *js.Object { return p.Raw(nil, false) })
	if !p.EOF {
		p.Eat(t.RightParenthesis)
	}
	return js.NewObject("type", "FeatureFunction", "loc", p.GetLocation(start, p.TokenStart), "kind", kind,
		"feature", functionName, "value", value)
}

// GeneralEnclosed parses "( any )" or "name( any )" that is not a known condition form.
func (p *parser) GeneralEnclosed(kind string) *js.Object {
	start := p.TokenStart
	var functionName any
	if p.TokenType == t.Function {
		functionName = p.ConsumeFunctionName()
	} else {
		p.Eat(t.LeftParenthesis)
	}
	var children *ast.List
	p.ParseWithFallback(func() *js.Object {
		startValueToken := p.TokenIndex
		children = p.ReadSequence(valueScope)
		if !p.EOF && !p.IsBalanceEdge(startValueToken) {
			p.Error("")
		}
		return nil
	}, func() *js.Object {
		children = p.CreateSingleNodeList(p.Raw(nil, false))
		return nil
	})
	if !p.EOF {
		p.Eat(t.RightParenthesis)
	}
	return js.NewObject("type", "GeneralEnclosed", "loc", p.GetLocation(start, p.TokenStart), "kind", kind,
		"function", functionName, "children", children)
}

// SupportsDeclaration parses "( declaration )".
func (p *parser) SupportsDeclaration() *js.Object {
	start := p.TokenStart
	p.Eat(t.LeftParenthesis)
	p.SkipSC()
	declaration := p.Declaration()
	if !p.EOF {
		p.Eat(t.RightParenthesis)
	}
	return js.NewObject("type", "SupportsDeclaration", "loc", p.GetLocation(start, p.TokenStart), "declaration", declaration)
}

func (p *parser) ratioTerm() *js.Object {
	p.SkipSC()
	switch p.TokenType {
	case t.Number:
		return p.Number()
	case t.Function:
		return p.Function(valueScope)
	}
	p.Error("Number of function is expected")
	return nil
}

// Ratio parses "number [/ number]" where either side may be a function.
func (p *parser) Ratio() *js.Object {
	start := p.TokenStart
	left := p.ratioTerm()
	var right any
	p.SkipSC()
	if p.IsDelim('/', 0) {
		p.EatDelim('/')
		right = p.ratioTerm()
	}
	return js.NewObject("type", "Ratio", "loc", p.GetLocation(start, p.TokenStart), "left", left, "right", right)
}

// Layer parses a dotted layer name.
func (p *parser) Layer() *js.Object {
	tokenStart := p.TokenStart
	name := p.Consume(t.Ident)
	for p.IsDelim('.', 0) {
		p.Eat(t.Delim)
		name += "." + p.Consume(t.Ident)
	}
	return js.NewObject("type", "Layer", "loc", p.GetLocation(tokenStart, p.TokenStart), "name", name)
}

// LayerList parses comma-separated layer names.
func (p *parser) LayerList() *js.Object {
	children := p.CreateList()
	p.SkipSC()
	for !p.EOF {
		children.Push(p.Layer())
		if p.LookupTypeNonSC(0) != t.Comma {
			break
		}
		p.SkipSC()
		p.Next()
		p.SkipSC()
	}
	return js.NewObject("type", "LayerList", "loc", p.GetLocationFromList(children), "children", children)
}

// Scope parses the @scope prelude "(root) to (limit)".
func (p *parser) Scope() *js.Object {
	var root, limit any
	p.SkipSC()
	startOffset := p.TokenStart
	raw := func() *js.Object { return p.Raw(nil, true) }
	if p.TokenType == t.LeftParenthesis {
		p.Next()
		p.SkipSC()
		root = p.ParseWithFallback(p.SelectorList, raw)
		p.SkipSC()
		p.Eat(t.RightParenthesis)
	}
	if p.LookupNonWSType(0) == t.Ident {
		p.SkipSC()
		p.EatIdent("to")
		p.SkipSC()
		p.Eat(t.LeftParenthesis)
		p.SkipSC()
		limit = p.ParseWithFallback(p.SelectorList, raw)
		p.SkipSC()
		p.Eat(t.RightParenthesis)
	}
	return js.NewObject("type", "Scope", "loc", p.GetLocation(startOffset, p.TokenStart), "root", root, "limit", limit)
}
