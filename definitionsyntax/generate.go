package definitionsyntax

import (
	"errors"

	"github.com/csstree/csstree/js"
)

// GenerateOptions configure Generate: ForceBraces writes every group in brackets, Compact leaves out optional
// spaces, and Decorate (when set) may rewrite the text generated for each node.
type GenerateOptions struct {
	ForceBraces bool
	Compact     bool
	Decorate    func(result string, node *js.Object) string
}

type generateError struct{ msg string }

func (e *generateError) Error() string { return e.msg }

func generateMultiplier(m *js.Object) string {
	min, _ := m.Num("min")
	max, _ := m.Num("max")
	comma := m.Bool("comma")
	switch {
	case min == 0 && max == 0:
		if comma {
			return "#?"
		}
		return "*"
	case min == 0 && max == 1:
		return "?"
	case min == 1 && max == 0:
		if comma {
			return "#"
		}
		return "+"
	case min == 1 && max == 1:
		return ""
	}
	prefix := ""
	if comma {
		prefix = "#"
	}
	if min == max {
		return prefix + "{" + js.ToString(m.Get("min")) + "}"
	}
	maxText := ""
	if max != 0 {
		maxText = js.ToString(m.Get("max"))
	}
	return prefix + "{" + js.ToString(m.Get("min")) + "," + maxText + "}"
}

func generateTypeOpts(node *js.Object) string {
	if node.Str("type") != "Range" {
		panic(&generateError{"Unknown node type `" + js.ToString(typeOf(node)) + "`"})
	}
	min, max := "-∞", "∞"
	if v := node.Get("min"); v != nil {
		min = js.ToString(v)
	}
	if v := node.Get("max"); v != nil {
		max = js.ToString(v)
	}
	return " [" + min + "," + max + "]"
}

func typeOf(node *js.Object) any {
	if v, ok := node.Lookup("type"); ok {
		return v
	}
	return js.Undefined
}

type generator struct {
	decorate             func(string, *js.Object) string
	forceBraces, compact bool
}

func (g *generator) sequence(node *js.Object) string {
	combinator := node.Str("combinator")
	if combinator != " " && !g.compact {
		combinator = " " + combinator + " "
	}
	result := ""
	terms, _ := node.Get("terms").([]any)
	for i, term := range terms {
		if i > 0 {
			result += combinator
		}
		t, _ := term.(*js.Object)
		result += g.node(t)
	}
	if node.Bool("explicit") || g.forceBraces {
		open, end := "[ ", " ]"
		if g.compact || js.CharCodeAt(result, 0) == ',' {
			open = "["
		}
		if g.compact {
			end = "]"
		}
		return open + result + end
	}
	return result
}

func (g *generator) node(node *js.Object) string {
	var result string
	switch node.Str("type") {
	case "Group":
		result = g.sequence(node)
		if node.Bool("disallowEmpty") {
			result += "!"
		}
	case "Multiplier":
		term, _ := node.Get("term").(*js.Object)
		return g.node(term) + g.decorate(generateMultiplier(node), node)
	case "Boolean":
		term, _ := node.Get("term").(*js.Object)
		result = "<boolean-expr[" + g.node(term) + "]>"
	case "Type":
		result = "<" + js.ToString(node.Get("name"))
		if opts := node.Obj("opts"); opts != nil {
			result += g.decorate(generateTypeOpts(opts), opts)
		}
		result += ">"
	case "Property":
		result = "<'" + js.ToString(node.Get("name")) + "'>"
	case "Keyword":
		result = js.ToString(node.Get("name"))
	case "AtKeyword":
		result = "@" + js.ToString(node.Get("name"))
	case "Function":
		result = js.ToString(node.Get("name")) + "("
	case "String", "Token":
		result = js.ToString(node.Get("value"))
	case "Comma":
		result = ","
	default:
		panic(&generateError{"Unknown node type `" + js.ToString(typeOf(node)) + "`"})
	}
	return g.decorate(result, node)
}

// Generate writes a grammar AST back as text.
func Generate(node *js.Object, opts GenerateOptions) (result string, err error) {
	g := &generator{decorate: opts.Decorate, forceBraces: opts.ForceBraces, compact: opts.Compact}
	if g.decorate == nil {
		g.decorate = func(s string, _ *js.Object) string { return s }
	}
	defer func() {
		if r := recover(); r != nil {
			var ge *generateError
			e, ok := r.(error)
			if !ok || !errors.As(e, &ge) {
				panic(r)
			}
			result, err = "", ge
		}
	}()
	return g.node(node), nil
}

// Walk calls enter before and leave after every node of a grammar AST; at least one of them must be set.
func Walk(node *js.Object, enter, leave func(node *js.Object)) error {
	if enter == nil && leave == nil {
		return errors.New("Neither `enter` nor `leave` walker handler is set or both aren't a function")
	}
	if enter == nil {
		enter = func(*js.Object) {}
	}
	if leave == nil {
		leave = func(*js.Object) {}
	}
	var walk func(n *js.Object) error
	walk = func(n *js.Object) error {
		enter(n)
		switch n.Str("type") {
		case "Group":
			terms, _ := n.Get("terms").([]any)
			for _, t := range terms {
				child, _ := t.(*js.Object)
				if err := walk(child); err != nil {
					return err
				}
			}
		case "Multiplier", "Boolean":
			if err := walk(n.Obj("term")); err != nil {
				return err
			}
		case "Type", "Property", "Keyword", "AtKeyword", "Function", "String", "Token", "Comma":
		default:
			return errors.New("Unknown type: " + js.ToString(typeOf(n)))
		}
		leave(n)
		return nil
	}
	return walk(node)
}
