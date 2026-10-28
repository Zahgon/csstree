package main

import (
	"github.com/csstree/csstree/convertor"
	"github.com/csstree/csstree/generator"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/syntax"
	"github.com/csstree/csstree/tokenizer"
)

func dispatch(argv []string) (any, error) {
	if len(argv) == 0 {
		return nil, usage("missing command")
	}
	command := argv[0]
	flags, positional, err := parseArgs(argv[1:])
	if err != nil {
		return nil, err
	}
	file, present := arg(positional, 0)
	switch command {
	case "tokenize":
		source, err := readInput(file, present)
		if err != nil {
			return nil, err
		}
		return tokenizer.NewTokenStream(source, tokenizer.Tokenize).Dump(), nil
	case "parse":
		return parseCommand(flags, file, present)
	case "generate":
		return generateCommand(flags, file, present)
	case "walk":
		return walkCommand(flags, file, present)
	case "find", "find-last", "find-all":
		return findCommand(command, flags, positional)
	case "convert":
		return convertCommand(flags, positional)
	case "lexer":
		return lexerCommand(flags, positional)
	case "syntax":
		return syntaxCommand(flags, positional)
	case "names", "ident", "string", "url":
		return codecCommand(command, flags, positional)
	}
	return nil, usage("unknown command %s", command)
}

func parseOptions(flags flagSet) (syntax.ParseOptions, error) {
	var opts syntax.ParseOptions
	if kind, ok := flags["kind"]; ok && kind != "media" && kind != "container" && kind != "supports" {
		return opts, usage("--kind must be media, container or supports")
	}
	for name, dest := range map[string]*string{"context": &opts.Context, "atrule": &opts.Atrule, "kind": &opts.Kind,
		"filename": &opts.Filename} {
		if value, ok := flags[name]; ok {
			if value == "" {
				return opts, usage("--%s needs a non-empty value", name)
			}
			*dest = value
		}
	}
	var err error
	if opts.Offset, err = flags.integer("offset"); err != nil {
		return opts, err
	}
	if opts.Line, err = flags.integer("line"); err != nil {
		return opts, err
	}
	if opts.Column, err = flags.integer("column"); err != nil {
		return opts, err
	}
	opts.Positions = flags.has("positions")
	off := false
	if flags.has("no-parse-atrule-prelude") {
		opts.ParseAtrulePrelude = &off
	}
	if flags.has("no-parse-rule-prelude") {
		opts.ParseRulePrelude = &off
	}
	if flags.has("no-parse-value") {
		opts.ParseValue = &off
	}
	opts.ParseCustomProperty = flags.has("parse-custom-property")
	return opts, nil
}

// inputAST returns the AST of the input: parsed CSS, or a JSON AST converted with FromPlainObject.
func inputAST(flags flagSet, file string, present bool, extra func(*syntax.ParseOptions)) (any, error) {
	if flags.has("from-json") {
		value, err := readJSON(file, present)
		if err != nil {
			return nil, err
		}
		return convertor.FromPlainObject(value), nil
	}
	opts, err := parseOptions(flags)
	if err != nil {
		return nil, err
	}
	source, err := readInput(file, present)
	if err != nil {
		return nil, err
	}
	if extra != nil {
		extra(&opts)
	}
	if flags.has("selector-parser") {
		return syntax.ParseSelector(source, opts)
	}
	return syntax.Parse(source, opts)
}

func parseCommand(flags flagSet, file string, present bool) (any, error) {
	fragmentLines, err := flags.integer("fragment-lines")
	if err != nil {
		return nil, err
	}
	out := js.NewObject()
	var comments, tokens, errorList []any
	extra := func(opts *syntax.ParseOptions) {
		if flags.has("comments") {
			opts.OnComment = func(value string, loc *js.Object) {
				comments = append(comments, js.NewObject("value", value, "loc", loc))
			}
		}
		if flags.has("tokens") {
			opts.OnToken = func(typ, start, end int) {
				tokens = append(tokens, js.NewObject("type", tokenizer.TokenNames[typ], "start", start, "end", end))
			}
		}
		if flags.has("tolerant") {
			opts.OnParseError = func(perr *syntax.SyntaxError, fallback *js.Object) {
				record := perr.JSObject()
				if fragmentLines != nil {
					record.Set("sourceFragment", perr.SourceFragment(*fragmentLines))
				}
				var fallbackType any
				if t, ok := fallback.Get("type").(string); ok && t != "" {
					fallbackType = t
				}
				record.Set("fallback", fallbackType)
				errorList = append(errorList, record)
			}
		}
	}
	ast, err := inputAST(flags, file, present, extra)
	if err != nil {
		return nil, err
	}
	if !flags.has("tolerant") && !flags.has("comments") && !flags.has("tokens") {
		return ast, nil
	}
	if flags.has("comments") {
		out.Set("comments", nonNil(comments))
	}
	if flags.has("tokens") {
		out.Set("tokens", nonNil(tokens))
	}
	out.Set("ast", ast)
	if flags.has("tolerant") {
		out.Set("errors", nonNil(errorList))
	}
	return out, nil
}

func nonNil(list []any) []any {
	if list == nil {
		return []any{}
	}
	return list
}

func generateCommand(flags flagSet, file string, present bool) (any, error) {
	ast, err := inputAST(flags, file, present, nil)
	if err != nil {
		return nil, err
	}
	opts := generator.Options{Mode: flags["mode"]}
	if flags.has("source-map") {
		css, sourceMap, err := generator.GenerateWithSourceMap(ast, opts)
		if err != nil {
			return nil, err
		}
		return js.NewObject("css", css, "map", sourceMap), nil
	}
	css, err := generator.Generate(ast, opts)
	if err != nil {
		return nil, err
	}
	return css + "\n", nil
}
