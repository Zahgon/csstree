// Package definitionsyntax parses, generates and walks the CSS Value Definition Syntax used by the lexer
// (grammars such as "<length> | auto" or "[ <color> || <length>{2,3} ]#").
package definitionsyntax

import (
	"strings"

	"github.com/csstree/csstree/internal/js"
)

// SyntaxError is a grammar parse error with the input and the offset (in UTF-16 code units) where it occurred.
type SyntaxError struct {
	Message    string
	Input      string
	Offset     int
	RawMessage string
}

func newSyntaxError(message, input string, offset int) *SyntaxError {
	dashes := offset
	if dashes == 0 {
		dashes = js.Len(input)
	}
	return &SyntaxError{
		Message:    message + "\n  " + input + "\n--" + strings.Repeat("-", dashes) + "^",
		Input:      input,
		Offset:     offset,
		RawMessage: message,
	}
}

// Error returns the message with the input and a caret under the offset.
func (e *SyntaxError) Error() string {
	return e.Message
}

// JSObject returns the error as the original reports it: name, message, input, offset, rawMessage.
func (e *SyntaxError) JSObject() *js.Object {
	return js.NewObject("name", "SyntaxError", "message", e.Message, "input", e.Input, "offset", e.Offset,
		"rawMessage", e.RawMessage)
}

type scanner struct {
	input string
	str   []uint16
	pos   int
}

func isNameChar(code int) bool {
	return code >= 0 && code < 128 && (code >= 'a' && code <= 'z' || code >= 'A' && code <= 'Z' ||
		code >= '0' && code <= '9' || code == '-')
}

func (s *scanner) charCodeAt(pos int) int {
	if pos >= 0 && pos < len(s.str) {
		return int(s.str[pos])
	}
	return 0
}

func (s *scanner) charCode() int     { return s.charCodeAt(s.pos) }
func (s *scanner) nextCharCode() int { return s.charCodeAt(s.pos + 1) }

func (s *scanner) findWsEnd(pos int) int {
	for ; pos < len(s.str); pos++ {
		switch s.str[pos] {
		case '\r', '\n', '\f', ' ', '\t':
			continue
		}
		break
	}
	return pos
}

func (s *scanner) skipWs() { s.pos = s.findWsEnd(s.pos) }

func (s *scanner) substringToPos(end int) string {
	start := s.pos
	s.pos = end
	return js.FromUTF16(s.str[min(start, len(s.str)):min(max(end, start), len(s.str))])
}

func (s *scanner) eat(code int) {
	if s.charCode() != code {
		s.error("Expect `" + js.FromCharCode(code) + "`")
	}
	s.pos++
}

func (s *scanner) peek() string {
	if s.pos < len(s.str) {
		s.pos++
		return js.FromUTF16(s.str[s.pos-1 : s.pos])
	}
	return ""
}

func (s *scanner) error(message string) {
	panic(newSyntaxError(message, s.input, s.pos))
}

func (s *scanner) scanSpaces() string {
	return s.substringToPos(s.findWsEnd(s.pos))
}

func (s *scanner) scanWord() string {
	end := s.pos
	for ; end < len(s.str); end++ {
		if !isNameChar(int(s.str[end])) {
			break
		}
	}
	if s.pos == end {
		s.error("Expect a keyword")
	}
	return s.substringToPos(end)
}

func (s *scanner) scanNumber() string {
	end := s.pos
	for ; end < len(s.str); end++ {
		if s.str[end] < '0' || s.str[end] > '9' {
			break
		}
	}
	if s.pos == end {
		s.error("Expect a number")
	}
	return s.substringToPos(end)
}

func (s *scanner) scanString() string {
	end := -1
	for i := s.pos + 1; i < len(s.str); i++ {
		if s.str[i] == '\'' {
			end = i
			break
		}
	}
	if end == -1 {
		s.pos = len(s.str)
		s.error("Expect an apostrophe")
	}
	return s.substringToPos(end + 1)
}
