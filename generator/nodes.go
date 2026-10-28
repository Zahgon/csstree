package generator

import (
	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
	"github.com/csstree/csstree/utils"
)

// field returns a node property as JavaScript string concatenation sees it ("undefined" when absent).
func field(n *js.Object, key string) string {
	v, ok := n.Lookup(key)
	if !ok {
		return "undefined"
	}
	return js.ToString(v)
}

func children(g *Generator, n *js.Object) { g.Children(n, nil) }

func semicolonAfterDeclaration(g *Generator) func(prev any) {
	return func(prev any) {
		if p, _ := prev.(*js.Object); p != nil && p.Str("type") == "Declaration" {
			g.Token(t.Semicolon, ";")
		}
	}
}

func init() {
	for name, fn := range map[string]func(g *Generator, n *js.Object){
		"StyleSheet":      children,
		"Value":           children,
		"AtrulePrelude":   children,
		"Rule":            func(g *Generator, n *js.Object) { g.Node(n.Get("prelude")); g.Node(n.Get("block")) },
		"Block":           generateBlock,
		"DeclarationList": func(g *Generator, n *js.Object) { g.Children(n, semicolonAfterDeclaration(g)) },
		"Raw":             func(g *Generator, n *js.Object) { g.Tokenize(field(n, "value")) },
		"Comment":         func(g *Generator, n *js.Object) { g.Token(t.Comment, "/*"+field(n, "value")+"*/") },
		"WhiteSpace":      func(g *Generator, n *js.Object) { g.Token(t.WhiteSpace, field(n, "value")) },
		"CDO":             func(g *Generator, _ *js.Object) { g.Token(t.CDO, "<!--") },
		"CDC":             func(g *Generator, _ *js.Object) { g.Token(t.CDC, "-->") },
		"Declaration":     generateDeclaration,
		"Atrule":          generateAtrule,
		"Identifier":      func(g *Generator, n *js.Object) { g.Token(t.Ident, field(n, "name")) },
		"Number":          func(g *Generator, n *js.Object) { g.Token(t.Number, field(n, "value")) },
		"Dimension":       func(g *Generator, n *js.Object) { g.Token(t.Dimension, field(n, "value")+field(n, "unit")) },
		"Percentage":      func(g *Generator, n *js.Object) { g.Token(t.Percentage, field(n, "value")+"%") },
		"String":          func(g *Generator, n *js.Object) { g.Token(t.String, utils.StringEncode(field(n, "value"), false)) },
		"Url":             func(g *Generator, n *js.Object) { g.Token(t.URL, utils.URLEncode(field(n, "value"))) },
		"Hash":            func(g *Generator, n *js.Object) { g.Token(t.Hash, "#"+field(n, "value")) },
		"Operator":        func(g *Generator, n *js.Object) { g.Tokenize(field(n, "value")) },
		"Parentheses": func(g *Generator, n *js.Object) {
			g.Token(t.LeftParenthesis, "(")
			g.Children(n, nil)
			g.Token(t.RightParenthesis, ")")
		},
		"Brackets": func(g *Generator, n *js.Object) {
			g.Token(t.Delim, "[")
			g.Children(n, nil)
			g.Token(t.Delim, "]")
		},
		"Function": func(g *Generator, n *js.Object) {
			g.Token(t.Function, field(n, "name")+"(")
			g.Children(n, nil)
			g.Token(t.RightParenthesis, ")")
		},
	} {
		nodeGenerators[name] = fn
	}
}

func generateBlock(g *Generator, n *js.Object) {
	g.Token(t.LeftCurlyBracket, "{")
	g.Children(n, semicolonAfterDeclaration(g))
	g.Token(t.RightCurlyBracket, "}")
}

func generateDeclaration(g *Generator, n *js.Object) {
	g.Token(t.Ident, field(n, "property"))
	g.Token(t.Colon, ":")
	g.Node(n.Get("value"))
	if important := n.Get("important"); js.Truthy(important) {
		g.Token(t.Delim, "!")
		if important == true {
			g.Token(t.Ident, "important")
		} else {
			g.Token(t.Ident, js.ToString(important))
		}
	}
}

func generateAtrule(g *Generator, n *js.Object) {
	g.Token(t.AtKeyword, "@"+field(n, "name"))
	if prelude, ok := n.Lookup("prelude"); !ok || prelude != nil {
		g.Node(prelude)
	}
	if block := n.Get("block"); js.Truthy(block) {
		g.Node(block)
	} else {
		g.Token(t.Semicolon, ";")
	}
}
