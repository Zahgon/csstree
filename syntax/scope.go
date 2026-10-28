package syntax

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/tokenizer"
)

// recognizer drives ReadSequence: GetNode returns the next node or nil to stop, OnWhiteSpace runs when white
// space separated two nodes (next is nil at the end), and Functions parse the arguments of special functions.
type recognizer struct {
	GetNode      func(p *parser, ctx *sequenceContext) *js.Object
	OnWhiteSpace func(p *parser, next *js.Object, children *ast.List, ctx *sequenceContext)
	Functions    map[string]func(p *parser, r *recognizer) *ast.List
}

// sequenceContext is the state ReadSequence passes to its recognizer.
type sequenceContext struct {
	recognizer *recognizer
}

// Stop conditions for SkipUntilBalanced and Raw: 0 continue, 1 stop before, 2 stop after the token.

// consumeUntilBalanceEnd never stops before the end of the enclosing block.
func consumeUntilBalanceEnd(int) int { return 0 }

// consumeUntilLeftCurlyBracket stops before "{".
func consumeUntilLeftCurlyBracket(code int) int { return stopIf(code == '{', 1) }

// consumeUntilLeftCurlyBracketOrSemicolon stops before "{" or ";".
func consumeUntilLeftCurlyBracketOrSemicolon(code int) int {
	return stopIf(code == '{' || code == ';', 1)
}

// consumeUntilExclamationMarkOrSemicolon stops before "!" or ";".
func consumeUntilExclamationMarkOrSemicolon(code int) int {
	return stopIf(code == '!' || code == ';', 1)
}

// consumeUntilSemicolonIncluded stops after ";".
func consumeUntilSemicolonIncluded(code int) int { return stopIf(code == ';', 2) }

func stopIf(cond bool, v int) int {
	if cond {
		return v
	}
	return 0
}

// ReadSequence reads nodes with a recognizer until it returns nil, skipping comments and reporting white space.
func (p *parser) ReadSequence(recognizer *recognizer) *ast.List {
	children := p.CreateList()
	space := false
	ctx := &sequenceContext{recognizer: recognizer}
	for !p.EOF {
		switch p.TokenType {
		case tokenizer.Comment:
			p.Next()
			continue
		case tokenizer.WhiteSpace:
			space = true
			p.Next()
			continue
		}
		child := recognizer.GetNode(p, ctx)
		if child == nil {
			break
		}
		if space {
			if recognizer.OnWhiteSpace != nil {
				recognizer.OnWhiteSpace(p, child, children, ctx)
			}
			space = false
		}
		children.Push(child)
	}
	if space && recognizer.OnWhiteSpace != nil {
		recognizer.OnWhiteSpace(p, nil, children, ctx)
	}
	return children
}

func defaultGetNode(p *parser, ctx *sequenceContext) *js.Object {
	switch p.TokenType {
	case tokenizer.Hash:
		return p.Hash()
	case tokenizer.Comma:
		return p.Operator()
	case tokenizer.LeftParenthesis:
		return p.Parentheses(ctx.recognizer)
	case tokenizer.LeftSquareBracket:
		return p.Brackets(ctx.recognizer)
	case tokenizer.String:
		return p.String()
	case tokenizer.Dimension:
		return p.Dimension()
	case tokenizer.Percentage:
		return p.Percentage()
	case tokenizer.Number:
		return p.Number()
	case tokenizer.Function:
		if p.CmpStr(p.TokenStart, p.TokenEnd, "url(") {
			return p.URL()
		}
		return p.Function(ctx.recognizer)
	case tokenizer.URL:
		return p.URL()
	case tokenizer.Ident:
		if p.CmpChar(p.TokenStart, 'u') && p.CmpChar(p.TokenStart+1, '+') {
			return p.UnicodeRange()
		}
		return p.Identifier()
	case tokenizer.Delim:
		code := p.CharCodeAt(p.TokenStart)
		if code == '/' || code == '*' || code == '+' || code == '-' {
			return p.Operator()
		}
		if code == '#' {
			p.ErrorAt("Hex or identifier is expected", p.TokenStart+1)
		}
	}
	return nil
}

func lastUnit(s string) int {
	units := js.UTF16(s)
	if len(units) == 0 {
		return -1
	}
	return int(units[len(units)-1])
}

func isPlusMinusOperator(node *js.Object) bool {
	if node == nil || node.Str("type") != "Operator" {
		return false
	}
	last := lastUnit(node.Str("value"))
	return last == '-' || last == '+'
}

// valueScope reads declaration values: white space around + and - operators is kept in the operator, and
// var() and expression() arguments are parsed specially.
var valueScope = &recognizer{
	GetNode: defaultGetNode,
	OnWhiteSpace: func(_ *parser, next *js.Object, children *ast.List, _ *sequenceContext) {
		if isPlusMinusOperator(next) {
			next.Set("value", " "+next.Str("value"))
		}
		if last, _ := children.Last().(*js.Object); isPlusMinusOperator(last) {
			last.Set("value", last.Str("value")+" ")
		}
	},
}

// atrulePreludeScope reads generic at-rule preludes.
var atrulePreludeScope = &recognizer{GetNode: defaultGetNode}

// selectorScope reads compound and complex selectors; white space between two non-combinators becomes a
// descendant combinator.
var selectorScope = &recognizer{
	GetNode: selectorGetNode,
	OnWhiteSpace: func(_ *parser, next *js.Object, children *ast.List, _ *sequenceContext) {
		last, _ := children.Last().(*js.Object)
		if last != nil && last.Str("type") != "Combinator" && next != nil && next.Str("type") != "Combinator" {
			children.Push(js.NewObject("type", "Combinator", "loc", nil, "name", " "))
		}
	},
}

func init() {
	valueScope.Functions = map[string]func(p *parser, r *recognizer) *ast.List{
		"expression": expressionFunction,
		"var":        varFunction,
	}
}

func selectorGetNode(p *parser, _ *sequenceContext) *js.Object {
	switch p.TokenType {
	case tokenizer.LeftSquareBracket:
		return p.AttributeSelector()
	case tokenizer.Hash:
		return p.IdSelector()
	case tokenizer.Colon:
		if p.LookupType(1) == tokenizer.Colon {
			return p.PseudoElementSelector()
		}
		return p.PseudoClassSelector()
	case tokenizer.Ident:
		return p.TypeSelector()
	case tokenizer.Number, tokenizer.Percentage:
		return p.Percentage()
	case tokenizer.Dimension:
		if p.CharCodeAt(p.TokenStart) == '.' {
			p.ErrorAt("Identifier is expected", p.TokenStart+1)
		}
	case tokenizer.Delim:
		switch p.CharCodeAt(p.TokenStart) {
		case '+', '>', '~', '/':
			return p.Combinator()
		case '.':
			return p.ClassSelector()
		case '*', '|':
			return p.TypeSelector()
		case '#':
			return p.IdSelector()
		case '&':
			return p.NestingSelector()
		}
	}
	return nil
}

// expressionFunction parses the legacy IE expression( <any-value> ) as one Raw node.
func expressionFunction(p *parser, _ *recognizer) *ast.List {
	return p.CreateSingleNodeList(p.Raw(nil, false))
}

// varFunction parses var( <ident> , <value>? ).
func varFunction(p *parser, _ *recognizer) *ast.List {
	children := p.CreateList()
	p.SkipSC()
	children.Push(p.Identifier())
	p.SkipSC()
	if p.TokenType == tokenizer.Comma {
		children.Push(p.Operator())
		startIndex := p.TokenIndex
		var value *js.Object
		if p.ParseCustomProperty {
			value = p.Value()
		} else {
			value = p.Raw(consumeUntilExclamationMarkOrSemicolon, false)
		}
		if list, ok := value.Get("children").(*ast.List); ok && value.Str("type") == "Value" && list.IsEmpty() {
			for offset := startIndex - p.TokenIndex; offset <= 0; offset++ {
				if p.LookupType(offset) == tokenizer.WhiteSpace {
					list.AppendData(js.NewObject("type", "WhiteSpace", "loc", nil, "value", " "))
					break
				}
			}
		}
		children.Push(value)
	}
	return children
}
