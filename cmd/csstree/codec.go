package main

import (
	"github.com/csstree/csstree/definitionsyntax"
	"github.com/csstree/csstree/js"
	"github.com/csstree/csstree/utils"
)

func syntaxCommand(flags flagSet, positional []string) (any, error) {
	command, _ := arg(positional, 0)
	file, present := arg(positional, 1)
	if command != "parse" && command != "generate" && command != "walk" {
		return nil, usage("unknown syntax command %s", command)
	}
	source, err := readInput(file, present)
	if err != nil {
		return nil, err
	}
	tree, err := definitionsyntax.Parse(source)
	if err != nil {
		return nil, err
	}
	switch command {
	case "generate":
		text, err := definitionsyntax.Generate(tree, definitionsyntax.GenerateOptions{
			Compact: flags.has("compact"), ForceBraces: flags.has("force-braces")})
		if err != nil {
			return nil, err
		}
		return text + "\n", nil
	case "walk":
		records := []any{}
		visit := func(phase string) func(*js.Object) {
			return func(node *js.Object) { records = append(records, []any{phase, nodeType(node), label(node)}) }
		}
		if err := definitionsyntax.Walk(tree, visit("enter"), visit("leave")); err != nil {
			return nil, err
		}
		return records, nil
	}
	return tree, nil
}

var codecs = map[string]map[string]func(string, flagSet) any{
	"names": {
		"keyword":            func(s string, _ flagSet) any { return utils.Keyword(s).JSObject() },
		"property":           func(s string, _ flagSet) any { return utils.Property(s).JSObject() },
		"vendor-prefix":      func(s string, _ flagSet) any { return utils.VendorPrefix(s, 0) },
		"is-custom-property": func(s string, _ flagSet) any { return utils.IsCustomProperty(s, 0) },
	},
	"ident": {
		"encode": func(s string, _ flagSet) any { return utils.IdentEncode(s) },
		"decode": func(s string, _ flagSet) any { return utils.IdentDecode(s) },
	},
	"string": {
		"encode": func(s string, f flagSet) any { return utils.StringEncode(s, f.has("apostrophe")) },
		"decode": func(s string, _ flagSet) any { return utils.StringDecode(s) },
	},
	"url": {
		"encode": func(s string, _ flagSet) any { return utils.URLEncode(s) },
		"decode": func(s string, _ flagSet) any { return utils.URLDecode(s) },
	},
}

func codecCommand(command string, flags flagSet, positional []string) (any, error) {
	mode, _ := arg(positional, 0)
	codec, ok := codecs[command][mode]
	if !ok {
		return nil, usage("unknown %s mode %s", command, mode)
	}
	file, present := arg(positional, 1)
	text, err := readInput(file, present)
	if err != nil {
		return nil, err
	}
	return codec(text, flags), nil
}
