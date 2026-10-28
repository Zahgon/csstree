// Package generator turns a CSSTree AST back into CSS, optionally with a Source Map v3, and lists the tokens
// a tree generates (the lexer's view of an AST value).
package generator

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/tokenizer"
)

// Options are the options of Generate: Mode "spec" separates tokens only where CSS Syntax requires it, any
// other value (the default "safe") also where some browsers misread adjacent tokens.
type Options struct {
	Mode      string
	SourceMap bool
}

// handlerSet are the replaceable steps of generation; decorators wrap them.
type handlerSet struct {
	Node        func(node any)
	TokenBefore TokenBefore
	Token       func(typ int, value string, suppressAutoWhiteSpace bool)
	Emit        func(value string, typ int, auto bool)
	Result      func() any
}

// nodeWriter is what a node type's generate function writes through.
type nodeWriter struct {
	h *handlerSet
}

// Node generates a node.
func (g *nodeWriter) Node(node any) {
	g.h.Node(node)
}

// Token writes one token, preceded by automatic white space where the token-before table requires it.
func (g *nodeWriter) Token(typ int, value string) {
	g.h.Token(typ, value, false)
}

// Tokenize writes raw text token by token; only its first token may get automatic white space before it.
func (g *nodeWriter) Tokenize(raw string) {
	units := js.UTF16(raw)
	tokenizer.TokenizeUnits(units, func(typ, start, end int) {
		g.h.Token(typ, js.FromUTF16(units[start:end]), start != 0)
	})
}

// Children generates the children of a node, calling delimiter (when not nil) between two of them.
func (g *nodeWriter) Children(node *js.Object, delimiter func(prev any)) {
	var prev any
	first := true
	each(node.Get("children"), func(child any) {
		if delimiter != nil {
			if !first {
				delimiter(prev)
			}
			first = false
			prev = child
		}
		g.Node(child)
	})
}

// each visits the items of a List or an array.
func each(children any, fn func(any)) {
	switch c := children.(type) {
	case *ast.List:
		c.ForEach(func(data any, _ *ast.ListItem, _ *ast.List) { fn(data) })
	case []any:
		for _, data := range c {
			fn(data)
		}
	}
}

type generateError struct {
	msg string
}

func (e *generateError) Error() string {
	return e.msg
}

func newHandlers() *handlerSet {
	buffer := []byte{}
	prevCode := 0
	h := &handlerSet{TokenBefore: Safe}
	g := &nodeWriter{h: h}
	h.Node = func(node any) {
		n, _ := node.(*js.Object)
		typ, ok := n.Lookup("type")
		if !ok {
			typ = js.Undefined
		}
		name, isString := typ.(string)
		fn := nodeGenerators[name]
		if n == nil || !isString || fn == nil {
			panic(&generateError{"Unknown node type: " + js.ToString(typ)})
		}
		fn(g, n)
	}
	h.Token = func(typ int, value string, suppressAutoWhiteSpace bool) {
		prevCode = h.TokenBefore(prevCode, typ, value)
		if !suppressAutoWhiteSpace && prevCode&1 != 0 {
			h.Emit(" ", tokenizer.WhiteSpace, true)
		}
		h.Emit(value, typ, false)
		if typ == tokenizer.Delim && js.CharCodeAt(value, 0) == '\\' {
			h.Emit("\n", tokenizer.WhiteSpace, true)
		}
	}
	h.Emit = func(value string, _ int, _ bool) {
		buffer = append(buffer, value...)
	}
	h.Result = func() any {
		return string(buffer)
	}
	return h
}

func run(node any, h *handlerSet, mode string) (result any, err error) {
	switch mode {
	case "spec":
		h.TokenBefore = Spec
	case "safe":
		h.TokenBefore = Safe
	}
	defer func() {
		if r := recover(); r != nil {
			ge, ok := r.(*generateError)
			if !ok {
				panic(r)
			}
			result, err = nil, ge
		}
	}()
	h.Node(node)
	return h.Result(), nil
}

// Generate returns the CSS of a node.
func Generate(node any, opts Options) (string, error) {
	result, err := run(node, newHandlers(), opts.Mode)
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

// Token is one emitted chunk of generated CSS with the node that produced it (nil for automatic white space).
type Token struct {
	Type  int
	Value string
	Node  *js.Object
}

// Tokens returns the chunks a node generates, each with the node it belongs to; it is how the lexer reads a
// tree value.
func Tokens(node any) ([]Token, error) {
	h := newHandlers()
	var tokens []Token
	var curNode *js.Object
	origNode := h.Node
	h.Node = func(n any) {
		tmp := curNode
		curNode, _ = n.(*js.Object)
		origNode(n)
		curNode = tmp
	}
	h.Emit = func(value string, typ int, auto bool) {
		tok := Token{Type: typ, Value: value}
		if !auto {
			tok.Node = curNode
		}
		tokens = append(tokens, tok)
	}
	h.Result = func() any { return tokens }
	if _, err := run(node, h, ""); err != nil {
		return nil, err
	}
	return tokens, nil
}

var nodeGenerators = map[string]func(g *nodeWriter, node *js.Object){}
