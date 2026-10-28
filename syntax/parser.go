// Package syntax parses CSS into a CSSTree AST: the parser, its parse contexts, the sequence scopes, the at-rule
// and pseudo-class handlers and the parse function of every node type.
package syntax

import (
	"errors"
	"regexp"
	"strings"

	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/tokenizer"
)

// ParseOptions are the options of Parse; a nil pointer is an option that is not given.
type ParseOptions struct {
	// Context names the parse context; "" is "default" (a stylesheet).
	Context string
	// Atrule is the at-rule name for the atrulePrelude context.
	Atrule string
	// Kind is the condition kind for the condition context ("media" when empty).
	Kind string
	// Positions adds loc to every node.
	Positions bool
	// Filename is the loc source; "" is "<unknown>".
	Filename string
	// Offset, Line and Column give the location of the first character (0, 1 and 1 when nil).
	Offset, Line, Column *int
	// ParseAtrulePrelude, ParseRulePrelude and ParseValue turn off the parsing of those parts when false.
	ParseAtrulePrelude, ParseRulePrelude, ParseValue *bool
	// ParseCustomProperty parses custom property values instead of keeping them Raw.
	ParseCustomProperty bool
	// OnParseError receives every recovered error and the node that replaced the unparsed input.
	OnParseError func(err *SyntaxError, fallback *js.Object)
	// OnComment receives the text and location of every comment before parsing starts.
	OnComment func(value string, loc *js.Object)
	// OnToken receives the type, start and end of every token before parsing starts.
	OnToken func(typ, start, end int)
}

type contextFn func(p *parser, opts *ParseOptions) *js.Object

// parser is the parser state: the token stream plus the location map and the current options.
type parser struct {
	tokenizer.TokenStream
	locationMap         *tokenizer.OffsetToLocation
	filename            string
	needPositions       bool
	onParseError        func(*SyntaxError, *js.Object)
	onParseErrorThrow   bool
	contexts            map[string]contextFn
	ParseAtrulePrelude  bool
	ParseRulePrelude    bool
	ParseValue          bool
	ParseCustomProperty bool
}

var defaultContexts = map[string]contextFn{
	"default":    func(p *parser, _ *ParseOptions) *js.Object { return p.StyleSheet() },
	"stylesheet": func(p *parser, _ *ParseOptions) *js.Object { return p.StyleSheet() },
	"atrule":     func(p *parser, _ *ParseOptions) *js.Object { return p.Atrule(false) },
	"atrulePrelude": func(p *parser, o *ParseOptions) *js.Object {
		if o.Atrule != "" {
			return p.AtrulePrelude(o.Atrule, true)
		}
		return p.AtrulePrelude("", false)
	},
	"mediaQueryList": func(p *parser, _ *ParseOptions) *js.Object { return p.MediaQueryList() },
	"mediaQuery":     func(p *parser, _ *ParseOptions) *js.Object { return p.MediaQuery() },
	"condition": func(p *parser, o *ParseOptions) *js.Object {
		if o.Kind == "" {
			return p.Condition("media")
		}
		return p.Condition(o.Kind)
	},
	"rule":            func(p *parser, _ *ParseOptions) *js.Object { return p.Rule() },
	"selectorList":    func(p *parser, _ *ParseOptions) *js.Object { return p.SelectorList() },
	"selector":        func(p *parser, _ *ParseOptions) *js.Object { return p.Selector() },
	"block":           func(p *parser, _ *ParseOptions) *js.Object { return p.Block(true) },
	"declarationList": func(p *parser, _ *ParseOptions) *js.Object { return p.DeclarationList() },
	"declaration":     func(p *parser, _ *ParseOptions) *js.Object { return p.Declaration() },
	"value":           func(p *parser, _ *ParseOptions) *js.Object { return p.Value() },
}

var selectorContexts = map[string]contextFn{
	"default":      func(p *parser, _ *ParseOptions) *js.Object { return p.SelectorList() },
	"selectorList": func(p *parser, _ *ParseOptions) *js.Object { return p.SelectorList() },
	"selector":     func(p *parser, _ *ParseOptions) *js.Object { return p.Selector() },
}

// Parse parses source in the context the options name and returns the AST.
func Parse(source string, opts ParseOptions) (*js.Object, error) {
	return parseWith(defaultContexts, source, opts)
}

// ParseSelector is the selector-only parser: its contexts are default (a selector list), selectorList and
// selector.
func ParseSelector(source string, opts ParseOptions) (*js.Object, error) {
	return parseWith(selectorContexts, source, opts)
}

func optInt(v *int, dflt int) int {
	if v == nil {
		return dflt
	}
	return *v
}

func optBool(v *bool, dflt bool) bool {
	if v == nil {
		return dflt
	}
	return *v
}

func parseWith(contexts map[string]contextFn, source string, opts ParseOptions) (result *js.Object, err error) {
	p := &parser{locationMap: tokenizer.NewOffsetToLocation(), contexts: contexts}
	p.SetSource(source, tokenizer.Tokenize)
	p.locationMap.SetSource(source, optInt(opts.Offset, 0), optInt(opts.Line, 1), optInt(opts.Column, 1))
	p.filename = opts.Filename
	if p.filename == "" {
		p.filename = "<unknown>"
	}
	p.needPositions = opts.Positions
	p.onParseError = opts.OnParseError
	if p.onParseError == nil {
		p.onParseError = func(*SyntaxError, *js.Object) {}
	}
	p.ParseAtrulePrelude = optBool(opts.ParseAtrulePrelude, true)
	p.ParseRulePrelude = optBool(opts.ParseRulePrelude, true)
	p.ParseValue = optBool(opts.ParseValue, true)
	p.ParseCustomProperty = opts.ParseCustomProperty

	context := opts.Context
	if context == "" {
		context = "default"
	}
	fn, ok := contexts[context]
	if !ok {
		return nil, errors.New("Unknown context `" + context + "`")
	}
	if context == "condition" && opts.Kind != "" && opts.Kind != "media" && opts.Kind != "container" && opts.Kind != "supports" {
		return nil, errors.New("Unsupported condition kind `" + opts.Kind + "`")
	}
	if opts.OnToken != nil {
		p.ForEachToken(func(typ, start, end, _ int) { opts.OnToken(typ, start, end) })
	}
	if opts.OnComment != nil {
		p.ForEachToken(func(typ, start, end, _ int) {
			if typ != tokenizer.Comment {
				return
			}
			loc := p.GetLocation(start, end)
			if tokenizer.CmpStr(p.Src, end-2, end, "*/") {
				opts.OnComment(js.FromUTF16(p.Src[start+2:end-2]), loc)
			} else {
				opts.OnComment(js.FromUTF16(p.Src[min(start+2, end):end]), loc)
			}
		})
	}
	defer func() {
		if r := recover(); r != nil {
			perr, ok := r.(error)
			if !isParseError(perr) || !ok {
				panic(r)
			}
			result, err = nil, perr
		}
	}()
	result = fn(p, &opts)
	if !p.EOF {
		p.Error("")
	}
	return result, nil
}

func isParseError(err error) bool {
	switch err.(type) {
	case *SyntaxError, *RangeError:
		return true
	}
	return false
}

// try runs consumer and returns the parse error it raised, if any.
func (p *parser) try(consumer func() *js.Object) (node *js.Object, err error) {
	defer func() {
		if r := recover(); r != nil {
			perr, ok := r.(error)
			if !ok || !isParseError(perr) {
				panic(r)
			}
			node, err = nil, perr
		}
	}()
	return consumer(), nil
}

// ParseWithFallback runs consumer; on a parse error it rewinds to where consumer started, builds the fallback
// node, reports the error with that node and returns it.
func (p *parser) ParseWithFallback(consumer, fallback func() *js.Object) *js.Object {
	startIndex := p.TokenIndex
	node, err := p.try(consumer)
	if err == nil {
		return node
	}
	if p.onParseErrorThrow {
		panic(err)
	}
	p.Skip(startIndex - p.TokenIndex)
	fallbackNode := fallback()
	p.onParseErrorThrow = true
	if se, ok := err.(*SyntaxError); ok {
		p.onParseError(se, fallbackNode)
	}
	p.onParseErrorThrow = false
	return fallbackNode
}

// CreateList returns an empty children list.
func (p *parser) CreateList() *ast.List {
	return ast.NewList()
}

// CreateSingleNodeList returns a list holding one node.
func (p *parser) CreateSingleNodeList(node *js.Object) *ast.List {
	return ast.NewList().AppendData(node)
}

// LookupNonWSType returns the type of the first token from offset on that is not white space or a comment.
func (p *parser) LookupNonWSType(offset int) int {
	for {
		typ := p.LookupType(offset)
		offset++
		if typ != tokenizer.WhiteSpace && typ != tokenizer.Comment {
			return typ
		}
		if typ == tokenizer.EOF {
			return tokenizer.EOF
		}
	}
}

// CharCodeAt returns the source code unit at offset, or 0 outside the source.
func (p *parser) CharCodeAt(offset int) int {
	if offset >= 0 && offset < len(p.Src) {
		return int(p.Src[offset])
	}
	return 0
}

// CmpChar compares the code unit at offset with a lowercase reference code (ASCII case folding).
func (p *parser) CmpChar(offset, code int) bool {
	return tokenizer.CmpChar(p.Src, offset, code)
}

// CmpStr compares source text with a lowercase reference string (ASCII case folding).
func (p *parser) CmpStr(start, end int, str string) bool {
	return tokenizer.CmpStr(p.Src, start, end, str)
}

// Consume checks the current token's type, moves past it and returns its text.
func (p *parser) Consume(tokenType int) string {
	start := p.TokenStart
	p.Eat(tokenType)
	return p.SubstrToCursor(start)
}

// ConsumeFunctionName returns the name of the current function token and moves past it.
func (p *parser) ConsumeFunctionName() string {
	name := p.Substring(p.TokenStart, p.TokenEnd-1)
	p.Eat(tokenizer.Function)
	return name
}

// ConsumeNumber returns the numeric part of the current token and moves past it.
func (p *parser) ConsumeNumber(tokenType int) string {
	number := p.Substring(p.TokenStart, tokenizer.ConsumeNumber(p.Src, p.TokenStart))
	p.Eat(tokenType)
	return number
}

var bracketChars = regexp.MustCompile(`[\[\](){}]`)

// Eat moves past the current token when it has the expected type, else raises "<token> is expected".
func (p *parser) Eat(tokenType int) {
	if p.TokenType != tokenType {
		name := strings.ReplaceAll(strings.TrimSuffix(tokenizer.TokenNames[tokenType], "-token"), "-", " ")
		if name != "" {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		message := name + " is expected"
		if bracketChars.MatchString(name) {
			message = `"` + name + `" is expected`
		}
		offset := p.TokenStart
		switch tokenType {
		case tokenizer.Ident:
			if p.TokenType == tokenizer.Function || p.TokenType == tokenizer.URL {
				offset = p.TokenEnd - 1
				message = "Identifier is expected but function found"
			} else {
				message = "Identifier is expected"
			}
		case tokenizer.Hash:
			if p.IsDelim(0x23, 0) {
				p.Next()
				offset++
				message = "Name is expected"
			}
		case tokenizer.Percentage:
			if p.TokenType == tokenizer.Number {
				offset = p.TokenEnd
				message = "Percent sign is expected"
			}
		}
		p.ErrorAt(message, offset)
	}
	p.Next()
}

// EatIdent moves past an identifier token with the given (lowercase) name, else raises an error.
func (p *parser) EatIdent(name string) {
	if p.TokenType != tokenizer.Ident || !p.LookupValue(0, name) {
		p.Error(`Identifier "` + name + `" is expected`)
	}
	p.Next()
}

// EatDelim moves past the delim code, else raises an error.
func (p *parser) EatDelim(code int) {
	if !p.IsDelim(code, 0) {
		p.Error(`Delim "` + js.FromCharCode(code) + `" is expected`)
	}
	p.Next()
}

// GetLocation returns the loc of a source range, or nil when positions are off.
func (p *parser) GetLocation(start, end int) *js.Object {
	if p.needPositions {
		return p.locationMap.GetLocationRange(start, end, p.filename)
	}
	return nil
}

func locStartOffset(node *js.Object, point string) int {
	v, _ := node.Obj("loc").Obj(point).Num("offset")
	return int(v)
}

// GetLocationFromList returns the loc spanning the nodes of a list, or nil when positions are off.
func (p *parser) GetLocationFromList(list *ast.List) *js.Object {
	if !p.needPositions {
		return nil
	}
	start, end := p.TokenStart, p.TokenStart
	if head, ok := list.First().(*js.Object); ok && head != nil {
		start = locStartOffset(head, "start") - p.locationMap.StartOffset
	}
	if tail, ok := list.Last().(*js.Object); ok && tail != nil {
		end = locStartOffset(tail, "end") - p.locationMap.StartOffset
	}
	return p.locationMap.GetLocationRange(start, end, p.filename)
}

// Error raises a parse error at the current token (or at the end of the input).
func (p *parser) Error(message string) {
	location := p.locationMap.GetLocation(p.TokenStart, "")
	if p.EOF {
		location = p.locationMap.GetLocation(tokenizer.FindWhiteSpaceStart(p.Src, len(p.Src)-1), "")
	}
	p.raise(message, location)
}

// ErrorAt raises a parse error at a source offset (or as Error when the offset is past the end).
func (p *parser) ErrorAt(message string, offset int) {
	if offset < len(p.Src) {
		p.raise(message, p.locationMap.GetLocation(offset, ""))
	}
	p.Error(message)
}

func (p *parser) raise(message string, location *js.Object) {
	if message == "" {
		message = "Unexpected input"
	}
	offset, _ := location.Num("offset")
	line, _ := location.Num("line")
	column, _ := location.Num("column")
	panic(newSyntaxError(message, p.Source, int(offset), int(line), int(column),
		p.locationMap.StartLine, p.locationMap.StartColumn))
}
