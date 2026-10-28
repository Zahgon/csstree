// Package csstree is a tool set for CSS: a detailed parser (CSS to AST), walker, generator (AST to CSS, with
// source maps) and lexer (validation and matching against the CSS value definition syntax of properties, types
// and at-rules). The functions of this package use the default syntax; Fork builds a syntax with extended lexer
// definitions.
package csstree

import (
	"sync"

	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/convertor"
	"github.com/csstree/csstree/generator"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/lexer"
	"github.com/csstree/csstree/syntax"
	"github.com/csstree/csstree/tokenizer"
	"github.com/csstree/csstree/walker"
)

// Version is the version of the CSSTree release this library implements.
const Version = "3.2.1"

// Syntax is a parser, generator and walker with a lexer built from a lexer config.
type Syntax struct {
	config *js.Object
	// Lexer validates values against the syntax's definitions.
	Lexer *lexer.Lexer
}

var (
	defaultOnce   sync.Once
	defaultSyntax *Syntax
	defaultErr    error
)

// Default returns the default syntax, built once from the bundled definitions.
func Default() (*Syntax, error) {
	defaultOnce.Do(func() {
		var config *js.Object
		config, defaultErr = lexer.DefaultConfig()
		if defaultErr == nil {
			defaultSyntax, defaultErr = newSyntax(config)
		}
	})
	return defaultSyntax, defaultErr
}

func newSyntax(config *js.Object) (*Syntax, error) {
	l, err := lexer.New(config)
	if err != nil {
		return nil, err
	}
	return &Syntax{config: config, Lexer: l}, nil
}

// Fork returns a syntax whose lexer config is this syntax's config extended with the generic, cssWideKeywords,
// units, types, properties and atrules of extension, merged as lexer.Mix does.
func (s *Syntax) Fork(extension *js.Object) (*Syntax, error) {
	return newSyntax(lexer.Mix(lexer.Mix(js.NewObject(), s.config), extension))
}

// CreateLexer returns a new lexer for a lexer config.
func CreateLexer(config *js.Object) (*lexer.Lexer, error) {
	return lexer.New(config)
}

// Fork extends the default syntax; see Syntax.Fork.
func Fork(extension *js.Object) (*Syntax, error) {
	s, err := Default()
	if err != nil {
		return nil, err
	}
	return s.Fork(extension)
}

// Tokenize splits CSS into tokens, calling onToken with each token's type and UTF-16 start and end offsets.
func Tokenize(source string, onToken func(typ, start, end int)) {
	tokenizer.Tokenize(source, onToken)
}

// Parse parses CSS into an AST.
func Parse(source string, opts syntax.ParseOptions) (*js.Object, error) {
	return syntax.Parse(source, opts)
}

// Generate writes an AST back to CSS.
func Generate(node any, opts generator.Options) (string, error) {
	return generator.Generate(node, opts)
}

// GenerateWithSourceMap writes an AST back to CSS with a Source Map v3.
func GenerateWithSourceMap(node any, opts generator.Options) (string, *js.Object, error) {
	return generator.GenerateWithSourceMap(node, opts)
}

// Walk traverses an AST.
func Walk(root any, opts walker.Options) error {
	return walker.Walk(root, opts)
}

// Find returns the first node for which fn returns true.
func Find(root any, fn func(*walker.Context, *js.Object, *ast.ListItem, *ast.List) bool) *js.Object {
	return walker.Find(root, fn)
}

// FindLast returns the last node for which fn returns true.
func FindLast(root any, fn func(*walker.Context, *js.Object, *ast.ListItem, *ast.List) bool) *js.Object {
	return walker.FindLast(root, fn)
}

// FindAll returns every node for which fn returns true.
func FindAll(root any, fn func(*walker.Context, *js.Object, *ast.ListItem, *ast.List) bool) []*js.Object {
	return walker.FindAll(root, fn)
}

// ToPlainObject turns the children lists of an AST into arrays.
func ToPlainObject(tree any) any {
	return convertor.ToPlainObject(tree)
}

// FromPlainObject turns the children arrays of an AST into lists.
func FromPlainObject(tree any) any {
	return convertor.FromPlainObject(tree)
}

// Clone deep-copies an AST node.
func Clone(node any) (*js.Object, error) {
	return ast.Clone(node)
}
