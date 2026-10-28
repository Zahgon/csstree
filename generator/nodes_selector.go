package generator

import (
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
)

func commaSeparated(g *nodeWriter, n *js.Object) {
	g.Children(n, func(any) { g.Token(t.Comma, ",") })
}

func generatePseudo(colons int) func(g *nodeWriter, n *js.Object) {
	return func(g *nodeWriter, n *js.Object) {
		for i := 0; i < colons; i++ {
			g.Token(t.Colon, ":")
		}
		if v, ok := n.Lookup("children"); ok && v == nil {
			g.Token(t.Ident, field(n, "name"))
			return
		}
		g.Token(t.Function, field(n, "name")+"(")
		g.Children(n, nil)
		g.Token(t.RightParenthesis, ")")
	}
}

func init() {
	for name, fn := range map[string]func(g *nodeWriter, n *js.Object){
		"UnicodeRange":          func(g *nodeWriter, n *js.Object) { g.Tokenize(field(n, "value")) },
		"SelectorList":          commaSeparated,
		"MediaQueryList":        commaSeparated,
		"LayerList":             commaSeparated,
		"Selector":              children,
		"TypeSelector":          func(g *nodeWriter, n *js.Object) { g.Tokenize(field(n, "name")) },
		"IdSelector":            func(g *nodeWriter, n *js.Object) { g.Token(t.Delim, "#"+field(n, "name")) },
		"ClassSelector":         func(g *nodeWriter, n *js.Object) { g.Token(t.Delim, "."); g.Token(t.Ident, field(n, "name")) },
		"Combinator":            func(g *nodeWriter, n *js.Object) { g.Tokenize(field(n, "name")) },
		"NestingSelector":       func(g *nodeWriter, _ *js.Object) { g.Token(t.Delim, "&") },
		"Layer":                 func(g *nodeWriter, n *js.Object) { g.Tokenize(field(n, "name")) },
		"AttributeSelector":     generateAttributeSelector,
		"PseudoClassSelector":   generatePseudo(1),
		"PseudoElementSelector": generatePseudo(2),
		"Nth":                   generateNth,
		"AnPlusB":               generateAnPlusB,
		"MediaQuery":            generateMediaQuery,
		"Condition":             generateCondition,
		"Feature":               generateFeature,
		"FeatureRange":          generateFeatureRange,
		"FeatureFunction": func(g *nodeWriter, n *js.Object) {
			g.Token(t.Function, field(n, "feature")+"(")
			g.Node(n.Get("value"))
			g.Token(t.RightParenthesis, ")")
		},
		"GeneralEnclosed":     generateGeneralEnclosed,
		"SupportsDeclaration": func(g *nodeWriter, n *js.Object) { parenthesized(g, n.Get("declaration")) },
		"Ratio":               generateRatio,
		"Scope":               generateScope,
	} {
		nodeGenerators[name] = fn
	}
}

func parenthesized(g *nodeWriter, node any) {
	g.Token(t.LeftParenthesis, "(")
	g.Node(node)
	g.Token(t.RightParenthesis, ")")
}

func generateAttributeSelector(g *nodeWriter, n *js.Object) {
	g.Token(t.Delim, "[")
	g.Node(n.Get("name"))
	if v, ok := n.Lookup("matcher"); !ok || v != nil {
		g.Tokenize(field(n, "matcher"))
		g.Node(n.Get("value"))
	}
	if v, ok := n.Lookup("flags"); !ok || v != nil {
		g.Token(t.Ident, field(n, "flags"))
	}
	g.Token(t.Delim, "]")
}

func generateNth(g *nodeWriter, n *js.Object) {
	g.Node(n.Get("nth"))
	if v, ok := n.Lookup("selector"); !ok || v != nil {
		g.Token(t.Ident, "of")
		g.Node(v)
	}
}

func generateAnPlusB(g *nodeWriter, n *js.Object) {
	if !js.Truthy(n.Get("a")) {
		g.Tokenize(field(n, "b"))
		return
	}
	var a string
	switch av := n.Get("a"); av {
	case "+1", "1":
		a = "n"
	case "-1":
		a = "-n"
	default:
		a = js.ToString(av) + "n"
	}
	if !js.Truthy(n.Get("b")) {
		g.Tokenize(a)
		return
	}
	b := field(n, "b")
	if first := js.CharCodeAt(b, 0); first != '-' && first != '+' {
		b = "+" + b
	}
	g.Tokenize(a + b)
}

func generateMediaQuery(g *nodeWriter, n *js.Object) {
	switch {
	case js.Truthy(n.Get("mediaType")):
		if js.Truthy(n.Get("modifier")) {
			g.Token(t.Ident, field(n, "modifier"))
		}
		g.Token(t.Ident, field(n, "mediaType"))
		if js.Truthy(n.Get("condition")) {
			g.Token(t.Ident, "and")
			g.Node(n.Get("condition"))
		}
	case js.Truthy(n.Get("condition")):
		g.Node(n.Get("condition"))
	}
}

func generateCondition(g *nodeWriter, n *js.Object) {
	each(n.Get("children"), func(child any) {
		if c, _ := child.(*js.Object); c != nil && c.Str("type") == "Condition" {
			parenthesized(g, c)
		} else {
			g.Node(child)
		}
	})
}

func generateFeature(g *nodeWriter, n *js.Object) {
	g.Token(t.LeftParenthesis, "(")
	g.Token(t.Ident, field(n, "name"))
	if v, ok := n.Lookup("value"); !ok || v != nil {
		g.Token(t.Colon, ":")
		g.Node(v)
	}
	g.Token(t.RightParenthesis, ")")
}

func generateFeatureRange(g *nodeWriter, n *js.Object) {
	g.Token(t.LeftParenthesis, "(")
	g.Node(n.Get("left"))
	g.Tokenize(field(n, "leftComparison"))
	g.Node(n.Get("middle"))
	if js.Truthy(n.Get("right")) {
		g.Tokenize(field(n, "rightComparison"))
		g.Node(n.Get("right"))
	}
	g.Token(t.RightParenthesis, ")")
}

func generateGeneralEnclosed(g *nodeWriter, n *js.Object) {
	if js.Truthy(n.Get("function")) {
		g.Token(t.Function, field(n, "function")+"(")
	} else {
		g.Token(t.LeftParenthesis, "(")
	}
	g.Children(n, nil)
	g.Token(t.RightParenthesis, ")")
}

func generateRatio(g *nodeWriter, n *js.Object) {
	g.Node(n.Get("left"))
	g.Token(t.Delim, "/")
	if js.Truthy(n.Get("right")) {
		g.Node(n.Get("right"))
	} else {
		// The original passes a token type here instead of a node, which fails as an unknown node type.
		g.Node(float64(t.Number))
	}
}

func generateScope(g *nodeWriter, n *js.Object) {
	if js.Truthy(n.Get("root")) {
		parenthesized(g, n.Get("root"))
	}
	if js.Truthy(n.Get("limit")) {
		g.Token(t.Ident, "to")
		parenthesized(g, n.Get("limit"))
	}
}
