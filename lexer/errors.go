package lexer

import (
	"regexp"
	"strings"

	"github.com/csstree/csstree/definitionsyntax"
	"github.com/csstree/csstree/internal/js"
)

// Error is a plain JavaScript error the lexer returns: its name is "Error" or "SyntaxError".
type Error struct {
	Name    string
	Message string
}

// Error returns the message.
func (e *Error) Error() string { return e.Message }

// JSObject returns {name, message}.
func (e *Error) JSObject() *js.Object { return js.NewObject("name", e.Name, "message", e.Message) }

// SyntaxReferenceError reports an unknown property, type, at-rule or descriptor, or a bad syntax.
type SyntaxReferenceError struct {
	Message   string
	Reference string
	hasRef    bool
}

func newReferenceError(kind string, reference string, hasRef bool) *SyntaxReferenceError {
	message := kind
	if hasRef && reference != "" {
		message += " `" + reference + "`"
	}
	return &SyntaxReferenceError{Message: message, Reference: reference, hasRef: hasRef}
}

// Error returns the message.
func (e *SyntaxReferenceError) Error() string { return e.Message }

// JSObject returns {name, message, reference}.
func (e *SyntaxReferenceError) JSObject() *js.Object {
	o := js.NewObject("name", "SyntaxReferenceError", "message", e.Message)
	if e.hasRef {
		o.Set("reference", e.Reference)
	}
	return o
}

// SyntaxMatchError reports a value that does not match a syntax, with the position of the mismatch.
type SyntaxMatchError struct {
	Message        string
	RawMessage     string
	Syntax         string
	CSS            string
	MismatchOffset int
	MismatchLength int
	start, end     *js.Object
	source         any
}

// Error returns the message with the syntax, the value and a caret under the mismatch.
func (e *SyntaxMatchError) Error() string { return e.Message }

// JSObject returns the error's properties in the original's order: name, message, rawMessage, syntax, css,
// mismatchOffset, mismatchLength, the start location's offset, line and column, and loc.
func (e *SyntaxMatchError) JSObject() *js.Object {
	o := js.NewObject("name", "SyntaxMatchError", "message", e.Message, "rawMessage", e.RawMessage,
		"syntax", e.Syntax, "css", e.CSS, "mismatchOffset", e.MismatchOffset, "mismatchLength", e.MismatchLength)
	for _, k := range e.start.Keys() {
		v, _ := e.start.Lookup(k)
		o.Set(k, v)
	}
	o.Set("loc", js.NewObject("source", e.source, "start", e.start, "end", e.end))
	return o
}

var lineSplit = regexp.MustCompile(`\n|\r\n?|\f`)

// buildLoc copies a location and moves it past extra.
func buildLoc(loc *js.Object, extra string) *js.Object {
	offset, _ := loc.Num("offset")
	line, _ := loc.Num("line")
	column, _ := loc.Num("column")
	if extra != "" {
		lines := lineSplit.Split(extra, -1)
		offset += float64(js.Len(extra))
		line += float64(len(lines) - 1)
		if len(lines) == 1 {
			column += float64(js.Len(extra))
		} else {
			column = float64(js.Len(lines[len(lines)-1]) + 1)
		}
	}
	return js.NewObject("offset", jsNumber(offset), "line", jsNumber(line), "column", jsNumber(column))
}

// jsNumber keeps integral values as int so they print the way JavaScript numbers do.
func jsNumber(f float64) any {
	if f == float64(int(f)) {
		return int(f)
	}
	return f
}

func fromLoc(node any, point string) *js.Object {
	n, _ := node.(*js.Object)
	value := n.Obj("loc").Obj(point)
	if value == nil {
		return nil
	}
	if value.Has("line") {
		return buildLoc(value, "")
	}
	return value
}

func defaultLoc() *js.Object { return js.NewObject("offset", 0, "line", 1, "column", 1) }

func newMatchError(message string, syntax *js.Object, node any, result *matchResult) *SyntaxMatchError {
	tokens := result.tokens
	longestMatch := result.longestMatch
	var mismatchNode *js.Object
	if longestMatch < len(tokens) {
		mismatchNode = tokens[longestMatch].node
	}
	var badNode *js.Object
	if n, isNode := node.(*js.Object); mismatchNode != nil && (!isNode || mismatchNode != n) {
		badNode = mismatchNode
	}
	mismatchOffset, mismatchLength, entries := 0, 0, 0
	var css strings.Builder
	cssLen := 0
	for i, tok := range tokens {
		if i == longestMatch {
			mismatchLength = js.Len(tok.value)
			mismatchOffset = cssLen
		}
		if badNode != nil && tok.node == badNode {
			if i <= longestMatch {
				entries++
			} else {
				entries = 0
			}
		}
		css.WriteString(tok.value)
		cssLen += js.Len(tok.value)
	}
	text := css.String()
	var start, end *js.Object
	if longestMatch == len(tokens) || entries > 1 {
		var target any = node
		if badNode != nil {
			target = badNode
		}
		if start = fromLoc(target, "end"); start == nil {
			start = buildLoc(defaultLoc(), text)
		}
		end = buildLoc(start, "")
	} else {
		if start = fromLoc(badNode, "start"); start == nil {
			base := fromLoc(node, "start")
			if base == nil {
				base = defaultLoc()
			}
			start = buildLoc(base, js.Slice(text, 0, mismatchOffset))
		}
		if end = fromLoc(badNode, "end"); end == nil {
			end = buildLoc(start, js.Substr(text, mismatchOffset, mismatchLength))
		}
	}
	syntaxText := "<generic>"
	if syntax != nil {
		if generated, err := definitionsyntax.Generate(syntax, definitionsyntax.GenerateOptions{}); err == nil {
			syntaxText = generated
		}
	}
	shown := text
	if shown == "" {
		shown = "<empty string>"
	}
	var source any = "<unknown>"
	if n, ok := node.(*js.Object); ok {
		if s := n.Obj("loc").Get("source"); js.Truthy(s) {
			source = s
		}
	}
	return &SyntaxMatchError{
		Message: message + "\n  syntax: " + syntaxText + "\n   value: " + shown + "\n  --------" +
			strings.Repeat("-", mismatchOffset) + "^",
		RawMessage:     message,
		Syntax:         syntaxText,
		CSS:            text,
		MismatchOffset: mismatchOffset,
		MismatchLength: mismatchLength,
		start:          start,
		end:            end,
		source:         source,
	}
}
