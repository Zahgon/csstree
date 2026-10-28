package syntax

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/csstree/csstree/internal/js"
)

const (
	maxLineLength    = 100
	offsetCorrection = 60
	tabReplacement   = "    "
)

// SyntaxError is a parse error with its position and the source it was raised for.
type SyntaxError struct {
	Message    string
	Source     string
	Offset     int
	Line       int
	Column     int
	baseLine   int
	baseColumn int
	formatted  string
}

// maxStringLength is the longest string the original's JavaScript engine can build (V8 on 64-bit).
const maxStringLength = 1<<29 - 24

// RangeError is the error the original raises instead of a SyntaxError when the excerpt of its formatted
// message would be longer than the longest JavaScript string (a start line or column near 2^32).
type RangeError struct{}

// Error returns the engine's message.
func (*RangeError) Error() string { return "Invalid string length" }

// JSObject returns {name: "RangeError", message}.
func (e *RangeError) JSObject() *js.Object {
	return js.NewObject("name", "RangeError", "message", e.Error())
}

func newSyntaxError(message, source string, offset, line, column, baseLine, baseColumn int) error {
	if max(baseLine-1, 0)+max(baseColumn-1, 0)+js.Len(source) > maxStringLength {
		return &RangeError{}
	}
	e := &SyntaxError{Message: message, Source: source, Offset: offset, Line: line, Column: column,
		baseLine: baseLine, baseColumn: baseColumn}
	e.formatted = "Parse error: " + message + "\n" + e.SourceFragment(2)
	return e
}

// Error returns the message.
func (e *SyntaxError) Error() string {
	return e.Message
}

// FormattedMessage returns "Parse error: <message>" followed by the source excerpt with two lines of context.
func (e *SyntaxError) FormattedMessage() string {
	return e.formatted
}

// JSObject returns the error's properties as the original's error object has them: name, message, source,
// offset, line, column, formattedMessage.
func (e *SyntaxError) JSObject() *js.Object {
	return js.NewObject("name", "SyntaxError", "message", e.Message, "source", e.Source, "offset", e.Offset,
		"line", e.Line, "column", e.Column, "formattedMessage", e.formatted)
}

var (
	lineBreak    = regexp.MustCompile(`\r\n?|\n|\f`)
	jsSpace      = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`
	leadingEmpty = regexp.MustCompile(`^(` + jsSpace + `+[0-9]+` + jsSpace + `+\|\n)+`)
	trailEmpty   = regexp.MustCompile(`\n(` + jsSpace + `+[0-9]+` + jsSpace + `+\|)+$`)
)

// SourceFragment returns the lines around the error with line numbers and a caret under the error column,
// extraLines lines before and after; lines longer than 100 columns are cut around the column.
func (e *SyntaxError) SourceFragment(extraLines int) string {
	line, column := e.Line, e.Column
	prelines := strings.Repeat("\n", max(e.baseLine-1, 0))
	precolumns := strings.Repeat(" ", max(e.baseColumn-1, 0))
	lines := lineBreak.Split(prelines+precolumns+e.Source, -1)
	startLine := max(1, line-extraLines) - 1
	endLine := min(line+extraLines, len(lines)+1)
	maxNumLength := max(4, len(strconv.Itoa(endLine))) + 1
	cutLeft := 0

	if line-1 >= 0 && line-1 < len(lines) {
		column += (len(tabReplacement) - 1) * strings.Count(js.Substr(lines[line-1], 0, column-1), "\t")
	}
	if column > maxLineLength {
		cutLeft = column - offsetCorrection + 3
		column = offsetCorrection - 2
	}
	for i := startLine; i <= endLine; i++ {
		if i >= 0 && i < len(lines) {
			l := strings.ReplaceAll(lines[i], "\t", tabReplacement)
			n := js.Len(l)
			prefix, suffix := "", ""
			if cutLeft > 0 && n > cutLeft {
				prefix = "\u2026"
			}
			if n > cutLeft+maxLineLength-1 {
				suffix = "\u2026"
			}
			lines[i] = prefix + js.Substr(l, cutLeft, maxLineLength-2) + suffix
		}
	}
	processLines := func(start, end int) string {
		var out []string
		for idx := start; idx < end && idx < len(lines); idx++ {
			if idx < 0 {
				continue
			}
			num := strconv.Itoa(idx + 1)
			out = append(out, strings.Repeat(" ", max(maxNumLength-len(num), 0))+num+" |"+lines[idx])
		}
		return strings.Join(out, "\n")
	}
	var parts []string
	for _, part := range []string{
		processLines(startLine, line),
		strings.Repeat("-", max(column+maxNumLength+1, 0)) + "^",
		processLines(line, endLine),
	} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	result := strings.Join(parts, "\n")
	result = leadingEmpty.ReplaceAllString(result, "")
	return trailEmpty.ReplaceAllString(result, "")
}
