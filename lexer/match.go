package lexer

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	"github.com/csstree/csstree/js"
	t "github.com/csstree/csstree/tokenizer"
)

// Warnings receives the matcher's warning when a match stops at the iteration limit.
var Warnings io.Writer = os.Stderr

const (
	iterationLimit            = 15000
	exitReasonMatch           = "Match"
	exitReasonMismatch        = "Mismatch"
	exitReasonIterationLimit  = "Maximum iteration number exceeded (please fill an issue on https://github.com/csstree/csstree/issues)"
	stubItem, tokenItem       = 0, 1
	openSyntaxItem, closeItem = 2, 3
)

// token is one token of a value being matched, with the AST node that produced it (nil for text values).
type token struct {
	typ   int
	value string
	node  *js.Object
}

type matchItem struct {
	typ    int
	syntax *js.Object
	token  *token
	prev   *matchItem
}

type syntaxItem struct {
	syntax *js.Object
	opts   *js.Object
	prev   *syntaxItem
}

type thenItem struct {
	nextState   *gnode
	matchStack  *matchItem
	syntaxStack *syntaxItem
	prev        *thenItem
}

type snapshot struct {
	nextState   *gnode
	matchStack  *matchItem
	syntaxStack *syntaxItem
	thenStack   *thenItem
	tokenIndex  int
	prev        *snapshot
}

type matchResult struct {
	tokens       []*token
	reason       string
	iterations   int
	match        *matchItem
	tree         *js.Object
	longestMatch int
}

func caseInsensitiveEqual(test, reference string) bool {
	a, b := js.UTF16(test), js.UTF16(reference)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		code := a[i]
		if code >= 'A' && code <= 'Z' {
			code |= 32
		}
		if code != b[i] {
			return false
		}
	}
	return true
}

// stripIEHack removes a "\0" or "\9" hack and the rest of the text when no line terminator follows it (the
// original's /\\[09].*$/ replacement).
func stripIEHack(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '\\' && (s[i+1] == '0' || s[i+1] == '9') && !strings.ContainsAny(s[i+2:], "\n\r\u2028\u2029") {
			return s[:i]
		}
	}
	return s
}

func isCommaContextStart(tok *token) bool {
	if tok == nil {
		return true
	}
	switch tok.typ {
	case t.Comma, t.Function, t.LeftParenthesis, t.LeftSquareBracket, t.LeftCurlyBracket:
		return true
	case t.Delim:
		return tok.value != "?"
	}
	return false
}

func isCommaContextEnd(tok *token) bool {
	if tok == nil {
		return true
	}
	switch tok.typ {
	case t.RightParenthesis, t.RightSquareBracket, t.RightCurlyBracket:
		return true
	}
	return tok.typ == t.Delim && tok.value == "/"
}

// matchError is an error raised while matching (a broken syntax reference or a broken grammar).
type matchError struct{ err error }

func errorf(format string, args ...any) error { return fmt.Errorf(format, args...) }

func replaceAll(s, old, replacement string) string { return strings.ReplaceAll(s, old, replacement) }

type matcher struct {
	tokens       []*token
	lexer        *Lexer
	state        *gnode
	syntaxStack  *syntaxItem
	thenStack    *thenItem
	elseStack    *snapshot
	stash        *snapshot
	stashOff     bool
	tok          *token
	tokenIndex   int
	longestMatch int
	matchStack   *matchItem
}

func (m *matcher) moveToNextToken() {
	for {
		m.tokenIndex++
		m.tok = nil
		if m.tokenIndex < len(m.tokens) {
			m.tok = m.tokens[m.tokenIndex]
		}
		if m.tok == nil || (m.tok.typ != t.WhiteSpace && m.tok.typ != t.Comment) {
			return
		}
	}
}

func (m *matcher) getNextToken(offset int) *token {
	if next := m.tokenIndex + offset; next >= 0 && next < len(m.tokens) {
		return m.tokens[next]
	}
	return nil
}

func (m *matcher) snapshot(nextState *gnode, prev *snapshot) *snapshot {
	return &snapshot{nextState: nextState, matchStack: m.matchStack, syntaxStack: m.syntaxStack,
		thenStack: m.thenStack, tokenIndex: m.tokenIndex, prev: prev}
}

func (m *matcher) pushThenStack(nextState *gnode) {
	m.thenStack = &thenItem{nextState: nextState, matchStack: m.matchStack, syntaxStack: m.syntaxStack, prev: m.thenStack}
}

func (m *matcher) pushElseStack(nextState *gnode) {
	m.elseStack = m.snapshot(nextState, m.elseStack)
}

func (m *matcher) addTokenToMatch() {
	m.matchStack = &matchItem{typ: tokenItem, syntax: m.state.syntax, token: m.tok, prev: m.matchStack}
	m.moveToNextToken()
	m.stash, m.stashOff = nil, false
	if m.tokenIndex > m.longestMatch {
		m.longestMatch = m.tokenIndex
	}
}

func (m *matcher) openSyntax() {
	opts := m.state.syntax.Obj("opts")
	if opts == nil && m.syntaxStack != nil {
		opts = m.syntaxStack.opts
	}
	m.syntaxStack = &syntaxItem{syntax: m.state.syntax, opts: opts, prev: m.syntaxStack}
	m.matchStack = &matchItem{typ: openSyntaxItem, syntax: m.state.syntax, token: m.matchStack.token, prev: m.matchStack}
}

func (m *matcher) closeSyntax() {
	if m.matchStack.typ == openSyntaxItem {
		m.matchStack = m.matchStack.prev
	} else {
		m.matchStack = &matchItem{typ: closeItem, syntax: m.syntaxStack.syntax, token: m.matchStack.token, prev: m.matchStack}
	}
	m.syntaxStack = m.syntaxStack.prev
}

func (m *matcher) lowPriority(state *gnode) bool {
	return (state.name == "custom-ident" && m.tok.typ == t.Ident) || (state.name == "length" && m.tok.value == "0")
}

// step runs one iteration of the matcher; it returns the exit reason when the match is decided.
func (m *matcher) step() string {
	state := m.state
	switch state.typ {
	case "Match":
		if m.thenStack == nil {
			if m.tok != nil && (m.tokenIndex != len(m.tokens)-1 || (m.tok.value != "\\0" && m.tok.value != "\\9")) {
				m.state = mismatchState
				return ""
			}
			return exitReasonMatch
		}
		m.state = m.thenStack.nextState
		if m.state == disallowEmptyState {
			if m.thenStack.matchStack == m.matchStack {
				m.state = mismatchState
				return ""
			}
			m.state = matchState
		}
		for m.thenStack.syntaxStack != m.syntaxStack {
			m.closeSyntax()
		}
		m.thenStack = m.thenStack.prev
	case "Mismatch":
		if m.stash != nil {
			if m.elseStack == nil || m.tokenIndex > m.elseStack.tokenIndex {
				m.elseStack = m.stash
				m.stash, m.stashOff = nil, true
			}
		} else if m.elseStack == nil {
			return exitReasonMismatch
		}
		e := m.elseStack
		m.state = e.nextState
		m.thenStack = e.thenStack
		m.syntaxStack = e.syntaxStack
		m.matchStack = e.matchStack
		m.tokenIndex = e.tokenIndex
		m.tok = nil
		if m.tokenIndex < len(m.tokens) {
			m.tok = m.tokens[m.tokenIndex]
		}
		m.elseStack = e.prev
	case "MatchGraph":
		m.state = state.match
	case "If":
		if state.els != mismatchState {
			m.pushElseStack(state.els)
		}
		if state.then != matchState {
			m.pushThenStack(state.then)
		}
		m.state = state.match
	case "MatchOnce":
		m.state = &gnode{typ: "MatchOnceBuffer", once: state}
	case "MatchOnceBuffer":
		terms := state.once.terms
		if state.index == len(terms) {
			if state.mask == 0 || state.once.all {
				m.state = mismatchState
			} else {
				m.state = matchState
			}
			return ""
		}
		if state.mask == 1<<len(terms)-1 {
			m.state = matchState
			return ""
		}
		for ; state.index < len(terms); state.index++ {
			flag := 1 << state.index
			if state.mask&flag == 0 {
				m.pushElseStack(state)
				m.pushThenStack(&gnode{typ: "AddMatchOnce", once: state.once, mask: state.mask | flag})
				m.state = terms[state.index]
				state.index++
				break
			}
		}
	case "AddMatchOnce":
		m.state = &gnode{typ: "MatchOnceBuffer", once: state.once, mask: state.mask}
	case "Enum":
		if m.tok != nil {
			name := stripIEHack(js.ToLowerCase(m.tok.value))
			if next, ok := state.enum[name]; ok {
				m.state = next
				return ""
			}
		}
		m.state = mismatchState
	case "Generic":
		var opts *js.Object
		if m.syntaxStack != nil {
			opts = m.syntaxStack.opts
		}
		n := state.fn(m.tok, m.getNextToken, opts)
		lastTokenIndex := m.tokenIndex + int(math.Floor(float64(n)))
		if lastTokenIndex > m.tokenIndex {
			for m.tokenIndex < lastTokenIndex {
				m.addTokenToMatch()
			}
			m.state = matchState
		} else {
			m.state = mismatchState
		}
	case "Type", "Property":
		dict := m.lexer.types
		if state.typ == "Property" {
			dict = m.lexer.properties
		}
		descriptor := dict.lookup(state.name)
		var graph *gnode
		if descriptor != nil {
			graph = descriptor.matchGraph()
		}
		if graph == nil {
			ref := "<" + state.name + ">"
			if state.typ == "Property" {
				ref = "<'" + state.name + "'>"
			}
			panic(&matchError{errorf("Bad syntax reference: %s", ref)})
		}
		if !m.stashOff && m.tok != nil && state.typ == "Type" && m.lowPriority(state) {
			if m.stash == nil {
				m.stash = m.snapshot(state, m.elseStack)
			}
			m.state = mismatchState
			return ""
		}
		m.openSyntax()
		if ref := descriptor.matchRefGraph(); ref != nil {
			m.state = ref
		} else {
			m.state = graph
		}
	case "Keyword":
		if m.tok != nil && caseInsensitiveEqual(stripIEHack(m.tok.value), state.name) {
			m.addTokenToMatch()
			m.state = matchState
			return ""
		}
		m.state = mismatchState
	case "AtKeyword", "Function":
		if m.tok != nil && caseInsensitiveEqual(m.tok.value, state.name) {
			m.addTokenToMatch()
			m.state = matchState
			return ""
		}
		m.state = mismatchState
	case "Token":
		if m.tok != nil && m.tok.value == state.value {
			m.addTokenToMatch()
			m.state = matchState
			return ""
		}
		m.state = mismatchState
	case "Comma":
		if m.tok != nil && m.tok.typ == t.Comma {
			if isCommaContextStart(m.matchStack.token) {
				m.state = mismatchState
			} else {
				m.addTokenToMatch()
				if isCommaContextEnd(m.tok) {
					m.state = mismatchState
				} else {
					m.state = matchState
				}
			}
		} else if isCommaContextStart(m.matchStack.token) || isCommaContextEnd(m.tok) {
			m.state = matchState
		} else {
			m.state = mismatchState
		}
	case "String":
		text := ""
		last := m.tokenIndex
		for ; last < len(m.tokens) && js.Len(text) < js.Len(state.value); last++ {
			text += m.tokens[last].value
		}
		if caseInsensitiveEqual(text, state.value) {
			for m.tokenIndex < last {
				m.addTokenToMatch()
			}
			m.state = matchState
		} else {
			m.state = mismatchState
		}
	default:
		panic(&matchError{errorf("Unknown node type: %s", state.typ)})
	}
	return ""
}

func internalMatch(tokens []*token, state *gnode, lexer *Lexer) *matchResult {
	m := &matcher{tokens: tokens, lexer: lexer, state: state, tokenIndex: -1,
		matchStack: &matchItem{typ: stubItem}}
	m.moveToNextToken()
	reason := ""
	iterations := 0
	for reason == "" {
		iterations++
		if iterations >= iterationLimit {
			break
		}
		reason = m.step()
	}
	switch reason {
	case "":
		if _, err := fmt.Fprintf(Warnings, "[csstree-match] BREAK after %d iterations\n", iterationLimit); err != nil {
			panic(&matchError{fmt.Errorf("writing the iteration limit warning: %w", err)})
		}
		reason = exitReasonIterationLimit
		m.matchStack = nil
	case exitReasonMatch:
		for m.syntaxStack != nil {
			m.closeSyntax()
		}
	default:
		m.matchStack = nil
	}
	return &matchResult{tokens: tokens, reason: reason, iterations: iterations, match: m.matchStack,
		longestMatch: m.longestMatch}
}

// matchAsTree matches tokens and turns the match into a tree of {syntax, match: [...]} and
// {syntax, token, node} objects rooted at the graph's syntax reference.
func matchAsTree(tokens []*token, graph *gnode, lexer *Lexer) *matchResult {
	result := internalMatch(tokens, graph, lexer)
	if result.match == nil {
		return result
	}
	var reversed *matchItem
	for item := result.match; item != nil; {
		next := item.prev
		item.prev = reversed
		reversed = item
		item = next
	}
	var rootSyntax any
	if graph.syntax != nil {
		rootSyntax = graph.syntax
	}
	host := js.NewObject("syntax", rootSyntax, "match", []any{})
	result.tree = host
	stack := []*js.Object{host}
	for item := reversed.prev; item != nil; item = item.prev {
		switch item.typ {
		case openSyntaxItem:
			child := js.NewObject("syntax", item.syntax, "match", []any{})
			host.Set("match", append(host.Get("match").([]any), child))
			host = child
			stack = append(stack, host)
		case closeItem:
			stack = stack[:len(stack)-1]
			host = stack[len(stack)-1]
		default:
			var syntax any
			if item.syntax != nil {
				syntax = item.syntax
			}
			var node any
			if item.token.node != nil {
				node = item.token.node
			}
			host.Set("match", append(host.Get("match").([]any), js.NewObject("syntax", syntax,
				"token", item.token.value, "node", node)))
		}
	}
	return result
}
