package main

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/convertor"
	"github.com/csstree/csstree/js"
	"github.com/csstree/csstree/walker"
)

// label is the node's name, else its value when that is a string, else null.
func label(node *js.Object) any {
	if name, ok := node.Get("name").(string); ok {
		return name
	}
	if value, ok := node.Get("value").(string); ok {
		return value
	}
	return nil
}

// nodeType is node.type as JSON writes it: null when the node or its type is missing.
func nodeType(node *js.Object) any {
	if v, ok := node.Lookup("type"); ok && v != js.Undefined {
		return v
	}
	return nil
}

var contextKeys = []string{"root", "stylesheet", "atrule", "atrulePrelude", "rule", "selector", "block",
	"declaration", "function"}

func contextValue(ctx *walker.Context, key string) *js.Object {
	switch key {
	case "root":
		return ctx.Root
	case "stylesheet":
		return ctx.Stylesheet
	case "atrule":
		return ctx.Atrule
	case "atrulePrelude":
		return ctx.AtrulePrelude
	case "rule":
		return ctx.Rule
	case "selector":
		return ctx.Selector
	case "block":
		return ctx.Block
	case "declaration":
		return ctx.Declaration
	}
	return ctx.Function
}

func walkCommand(flags flagSet, file string, present bool) (any, error) {
	tree, err := inputAST(flags, file, present, nil)
	if err != nil {
		return nil, err
	}
	records := []any{}
	record := func(phase string, ctx *walker.Context, node *js.Object) {
		entry := []any{phase, nodeType(node), label(node)}
		if flags.has("context-fields") {
			fields := []any{}
			for _, key := range contextKeys {
				if v := contextValue(ctx, key); v != nil {
					fields = append(fields, key+":"+js.ToString(nodeType(v)))
				}
			}
			entry = append(entry, fields)
		}
		records = append(records, entry)
	}
	breakType, hasBreak := flags["break"]
	skipType, hasSkip := flags["skip"]
	opts := walker.Options{Reverse: flags.has("reverse"), Visit: flags["visit"]}
	opts.Enter = func(ctx *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		record("enter", ctx, node)
		typ, _ := node.Get("type").(string)
		if hasBreak && typ == breakType {
			return walker.Break
		}
		if hasSkip && typ == skipType {
			return walker.Skip
		}
		return walker.Continue
	}
	if flags.has("leave") {
		opts.Leave = func(ctx *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
			record("leave", ctx, node)
			return walker.Continue
		}
	}
	if err := walker.Walk(tree, opts); err != nil {
		return nil, err
	}
	return records, nil
}

func findCommand(command string, flags flagSet, positional []string) (any, error) {
	typ, ok := arg(positional, 0)
	if !ok {
		return nil, usage("missing node type")
	}
	file, present := arg(positional, 1)
	tree, err := inputAST(flags, file, present, nil)
	if err != nil {
		return nil, err
	}
	name, hasName := flags["name"]
	test := func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) bool {
		if t, _ := node.Get("type").(string); t != typ {
			return false
		}
		n, isString := node.Get("name").(string)
		return !hasName || (isString && n == name)
	}
	var found any
	switch command {
	case "find":
		found = walker.Find(tree, test)
	case "find-last":
		found = walker.FindLast(tree, test)
	default:
		all := []any{}
		for _, node := range walker.FindAll(tree, test) {
			all = append(all, node)
		}
		return all, nil
	}
	if found.(*js.Object) == nil {
		return nil, nil
	}
	return found, nil
}

func convertCommand(flags flagSet, positional []string) (any, error) {
	mode, _ := arg(positional, 0)
	file, present := arg(positional, 1)
	switch mode {
	case "to-plain":
		tree, err := inputAST(flags, file, present, nil)
		if err != nil {
			return nil, err
		}
		return convertor.ToPlainObject(tree), nil
	case "from-plain":
		value, err := readJSON(file, present)
		if err != nil {
			return nil, err
		}
		return convertor.FromPlainObject(value), nil
	case "clone":
		tree, err := inputAST(flags, file, present, nil)
		if err != nil {
			return nil, err
		}
		return ast.Clone(tree)
	}
	return nil, usage("unknown convert mode %s", mode)
}
