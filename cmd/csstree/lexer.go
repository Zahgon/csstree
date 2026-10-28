package main

import (
	"strings"

	csstree "github.com/csstree/csstree"
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/definitionsyntax"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/lexer"
	"github.com/csstree/csstree/syntax"
	"github.com/csstree/csstree/walker"
)

var forkKeys = map[string]bool{"generic": true, "cssWideKeywords": true, "units": true, "types": true,
	"properties": true, "atrules": true}

func selectLexer(flags flagSet) (*lexer.Lexer, error) {
	base, err := csstree.Default()
	if err != nil {
		return nil, err
	}
	file, ok := flags["fork"]
	if !ok {
		return base.Lexer, nil
	}
	value, err := readJSON(file, true)
	if err != nil {
		return nil, err
	}
	extension, isObject := value.(*js.Object)
	if !isObject {
		return nil, usage("--fork must hold a JSON object")
	}
	for _, key := range extension.Keys() {
		if !forkKeys[key] {
			return nil, usage("--fork cannot extend %s", key)
		}
	}
	forked, err := base.Fork(extension)
	if err != nil {
		return nil, err
	}
	return forked.Lexer, nil
}

func renderMatch(match *js.Object) (any, error) {
	if match == nil {
		return nil, nil
	}
	var syntaxText any
	if s := match.Obj("syntax"); s != nil {
		text, err := definitionsyntax.Generate(s, definitionsyntax.GenerateOptions{})
		if err != nil {
			return nil, err
		}
		syntaxText = text
	}
	if match.Has("token") {
		var nodeTypeValue any
		if n := match.Obj("node"); n != nil {
			nodeTypeValue = nodeType(n)
		}
		return js.NewObject("syntax", syntaxText, "token", match.Get("token"), "node", nodeTypeValue), nil
	}
	children, _ := match.Get("match").([]any)
	rendered := []any{}
	for _, child := range children {
		c, _ := child.(*js.Object)
		r, err := renderMatch(c)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, r)
	}
	return js.NewObject("syntax", syntaxText, "match", rendered), nil
}

func matchOutput(res *lexer.MatchResult, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	matched, err := renderMatch(res.Matched)
	if err != nil {
		return nil, err
	}
	var errValue any
	if res.Error != nil {
		errValue = errorFields(res.Error)
	}
	return js.NewObject("matched", matched, "error", errValue), nil
}

func checked(problem error) any {
	if problem == nil {
		return nil
	}
	return errorFields(problem)
}

func fragmentNodes(fragments []lexer.Fragment, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	out := []any{}
	for _, f := range fragments {
		out = append(out, f.Nodes)
	}
	return out, nil
}

func lexerCommand(flags flagSet, positional []string) (any, error) {
	command, _ := arg(positional, 0)
	args := positional[min(1, len(positional)):]
	l, err := selectLexer(flags)
	if err != nil {
		return nil, err
	}
	need := func(n int) error {
		if len(args) != n {
			return usage("lexer %s needs %d arguments", command, n)
		}
		return nil
	}
	value := func(file string, opts syntax.ParseOptions) (any, error) {
		source, err := readInput(file, true)
		if err != nil || !flags.has("ast") {
			return source, err
		}
		opts.Positions = flags.has("positions")
		return syntax.Parse(source, opts)
	}
	parsed := func(file string, opts syntax.ParseOptions) (*js.Object, error) {
		source, err := readInput(file, true)
		if err != nil {
			return nil, err
		}
		opts.Positions = flags.has("positions")
		return syntax.Parse(source, opts)
	}
	switch command {
	case "match-property", "match-type", "match", "match-atrule-prelude":
		if err := need(2); err != nil {
			return nil, err
		}
		opts := syntax.ParseOptions{Context: "value"}
		if command == "match-atrule-prelude" {
			opts = syntax.ParseOptions{Context: "atrulePrelude", Atrule: args[0]}
		}
		v, err := value(args[1], opts)
		if err != nil {
			return nil, err
		}
		switch command {
		case "match-property":
			return matchOutput(l.MatchProperty(args[0], v))
		case "match-type":
			return matchOutput(l.MatchType(args[0], v))
		case "match":
			return matchOutput(l.Match(args[0], v))
		}
		return matchOutput(l.MatchAtrulePrelude(args[0], v))
	case "match-atrule-descriptor":
		if err := need(3); err != nil {
			return nil, err
		}
		v, err := value(args[2], syntax.ParseOptions{Context: "value"})
		if err != nil {
			return nil, err
		}
		return matchOutput(l.MatchAtruleDescriptor(args[0], args[1], v))
	case "match-declaration":
		if err := need(1); err != nil {
			return nil, err
		}
		declaration, err := parsed(args[0], syntax.ParseOptions{Context: "declaration"})
		if err != nil {
			return nil, err
		}
		return matchOutput(l.MatchDeclaration(declaration))
	case "check-atrule-name":
		if err := need(1); err != nil {
			return nil, err
		}
		return checked(l.CheckAtruleName(args[0])), nil
	case "check-atrule-prelude":
		if err := need(2); err != nil {
			return nil, err
		}
		v, err := value(args[1], syntax.ParseOptions{Context: "atrulePrelude", Atrule: args[0]})
		if err != nil {
			return nil, err
		}
		problem, err := l.CheckAtrulePrelude(args[0], v)
		if err != nil {
			return nil, err
		}
		return checked(problem), nil
	case "check-atrule-descriptor-name":
		if err := need(2); err != nil {
			return nil, err
		}
		return checked(l.CheckAtruleDescriptorName(args[0], args[1])), nil
	case "check-property-name":
		if err := need(1); err != nil {
			return nil, err
		}
		return checked(l.CheckPropertyName(args[0])), nil
	case "check-structure":
		if err := need(1); err != nil {
			return nil, err
		}
		tree, err := inputAST(flags, args[0], true, nil)
		if err != nil {
			return nil, err
		}
		warnings := l.CheckStructure(tree)
		if warnings == nil {
			return false, nil
		}
		out := []any{}
		for _, w := range warnings {
			var typ any
			if n, ok := w.Node.(*js.Object); ok && js.Truthy(n.Get("type")) {
				typ = n.Get("type")
			}
			out = append(out, js.NewObject("node", typ, "message", w.Message))
		}
		return out, nil
	case "fragments", "all-fragments":
		n := 4
		if command == "all-fragments" {
			n = 3
		}
		if err := need(n); err != nil {
			return nil, err
		}
		if command == "fragments" {
			tree, err := parsed(args[3], syntax.ParseOptions{Context: "value"})
			if err != nil {
				return nil, err
			}
			return fragmentNodes(l.FindValueFragments(args[0], tree, args[1], args[2]))
		}
		tree, err := parsed(args[2], syntax.ParseOptions{})
		if err != nil {
			return nil, err
		}
		return fragmentNodes(l.FindAllFragments(tree, args[0], args[1]))
	case "trace":
		if err := need(2); err != nil {
			return nil, err
		}
		source, err := readInput(args[1], true)
		if err != nil {
			return nil, err
		}
		tree, err := syntax.Parse(source, syntax.ParseOptions{Context: "value"})
		if err != nil {
			return nil, err
		}
		return traceOutput(l, args[0], tree)
	case "validate":
		if err := need(0); err != nil {
			return nil, err
		}
		result, err := l.Validate()
		if err != nil || result == nil {
			return nil, err
		}
		return result, nil
	case "dump":
		if err := need(0); err != nil {
			return nil, err
		}
		return dumpOutput(l, flags)
	}
	return nil, usage("unknown lexer command %s", command)
}

func traceOutput(l *lexer.Lexer, property string, tree *js.Object) (any, error) {
	match, err := l.MatchProperty(property, tree)
	if err != nil {
		return nil, err
	}
	nodes := []any{}
	walker.Visit(tree, func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		var labels any
		if trace := match.GetTrace(node); trace != nil {
			list := []any{}
			for _, s := range trace {
				list = append(list, js.ToString(s.Get("type"))+":"+js.ToString(s.Get("name")))
			}
			labels = list
		}
		nodes = append(nodes, []any{nodeType(node), labels, match.IsKeyword(node), match.IsProperty(node, property)})
		return walker.Continue
	}, nil)
	var errValue any
	if match.Error != nil {
		errValue = errorFields(match.Error)
	}
	return js.NewObject("error", errValue, "nodes", nodes), nil
}

func dumpOutput(l *lexer.Lexer, flags flagSet) (any, error) {
	dump, err := l.Dump(flags.has("ast"), flags.has("pretty"))
	if err != nil {
		return nil, err
	}
	section, ok := flags["section"]
	if !ok {
		return dump, nil
	}
	var selected *js.Object
	switch section {
	case "meta":
		selected = js.NewObject("generic", dump.Get("generic"), "cssWideKeywords", dump.Get("cssWideKeywords"),
			"units", dump.Get("units"))
	case "types", "properties", "atrules":
		selected = dump.Obj(section)
	default:
		return nil, usage("unknown --section %s", section)
	}
	names, ok := flags["names"]
	if !ok {
		return selected, nil
	}
	out := js.NewObject()
	for _, name := range strings.Split(names, ",") {
		if v, has := selected.Lookup(name); has {
			out.Set(name, v)
		} else {
			out.Set(name, nil)
		}
	}
	return out, nil
}
