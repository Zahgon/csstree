package syntax

import (
	"regexp"
	"strings"
	"testing"

	"github.com/csstree/csstree/internal/fixtureset"
	"github.com/csstree/csstree/internal/js"
)

func loadAstFixtures(t *testing.T) []fixtureset.Test {
	t.Helper()
	tests, err := fixtureset.Load("../fixtures/ast")
	if err != nil || len(tests) == 0 {
		t.Fatalf("reading fixtures/ast: %v (%d tests)", err, len(tests))
	}
	return tests
}

func withoutLoc(v any) string { return fixtureset.WithoutLoc(v) }

var wrappers = map[string]func(*js.Object) *js.Object{
	"stylesheet": func(a *js.Object) *js.Object { return js.NewObject("type", "StyleSheet", "children", []any{a}) },
	"mediaQuery": func(a *js.Object) *js.Object {
		return js.NewObject("type", "MediaQuery", "modifier", nil, "mediaType", nil, "condition", a)
	},
	"rule": func(a *js.Object) *js.Object {
		return js.NewObject("type", "Rule", "prelude", js.NewObject("type", "SelectorList", "children", []any{}), "block", a)
	},
	"selector": func(a *js.Object) *js.Object { return js.NewObject("type", "Selector", "children", []any{a}) },
	"value":    func(a *js.Object) *js.Object { return js.NewObject("type", "Value", "children", []any{a}) },
}

func fixtureOptions(scope string, test *js.Object) ParseOptions {
	opts := ParseOptions{Context: scope}
	o := test.Obj("options")
	opts.Atrule = o.Str("atrule")
	opts.ParseCustomProperty = o.Bool("parseCustomProperty")
	for key, dest := range map[string]**bool{"parseAtrulePrelude": &opts.ParseAtrulePrelude,
		"parseRulePrelude": &opts.ParseRulePrelude, "parseValue": &opts.ParseValue} {
		if v, ok := o.Get(key).(bool); ok {
			*dest = &v
		}
	}
	return opts
}

var lineBreaks = regexp.MustCompile(`\r|\r\n|\n|\f`)

func TestParseFixtures(t *testing.T) {
	for _, at := range loadAstFixtures(t) {
		test := at.Data
		if test.Bool("skip") {
			continue
		}
		t.Run(at.Name, func(t *testing.T) {
			opts := fixtureOptions(at.Scope, test)
			source := test.Str("source")
			if !test.Has("error") {
				ast, err := Parse(source, opts)
				if err != nil {
					t.Fatalf("parse %q: %v", source, err)
				}
				expected := test.Obj("ast")
				if wrap, ok := wrappers[at.Scope]; ok && !strings.EqualFold(expected.Str("type"), at.Scope) {
					expected = wrap(expected)
				}
				if got, want := withoutLoc(ast), withoutLoc(expected); got != want {
					t.Fatalf("parse %q\n got %s\nwant %s", source, got, want)
				}
				return
			}
			var first *SyntaxError
			opts.OnParseError = func(err *SyntaxError, _ *js.Object) {
				if first == nil {
					first = err
				}
			}
			_, err := Parse(source, opts)
			if first == nil {
				first, _ = err.(*SyntaxError)
			}
			if first == nil {
				t.Fatalf("parse %q: expected the error %q, got %v", source, test.Str("error"), err)
			}
			offset, line, column := expectedPosition(test)
			if first.Message != test.Str("error") || first.Offset != offset || first.Line != line || first.Column != column {
				t.Fatalf("parse %q: got %q at %d:%d:%d, want %q at %d:%d:%d", source, first.Message, first.Offset,
					first.Line, first.Column, test.Str("error"), offset, line, column)
			}
		})
	}
}

// expectedPosition reads an error position given as a caret string under the source, or as an object.
func expectedPosition(test *js.Object) (int, int, int) {
	if caret, ok := test.Get("offset").(string); ok {
		offset := js.IndexOf(caret, "^", 0)
		lines := lineBreaks.Split(js.Substr(test.Str("source"), 0, offset), -1)
		return offset, len(lines), js.Len(lines[len(lines)-1]) + 1
	}
	p := test.Obj("position")
	o, _ := p.Num("offset")
	l, _ := p.Num("line")
	c, _ := p.Num("column")
	return int(o), int(l), int(c)
}

func TestParseSelectorFixtures(t *testing.T) {
	for _, at := range loadAstFixtures(t) {
		if (at.Scope != "selector" && at.Scope != "selectorList") || at.Data.Has("error") || at.Data.Bool("skip") {
			continue
		}
		t.Run(at.Name, func(t *testing.T) {
			full, err := Parse(at.Data.Str("source"), fixtureOptions(at.Scope, at.Data))
			if err != nil {
				t.Fatal(err)
			}
			only, err := ParseSelector(at.Data.Str("source"), fixtureOptions(at.Scope, at.Data))
			if err != nil {
				t.Fatal(err)
			}
			if a, b := withoutLoc(full), withoutLoc(only); a != b {
				t.Fatalf("selector parser\n got %s\nwant %s", b, a)
			}
		})
	}
}

func TestUnknownContext(t *testing.T) {
	_, err := Parse("a", ParseOptions{Context: "unknown"})
	if err == nil || err.Error() != "Unknown context `unknown`" {
		t.Fatalf("got %v, want Unknown context `unknown`", err)
	}
}

func TestPositions(t *testing.T) {
	ast, err := Parse("a{b:c}", ParseOptions{Positions: true, Filename: "test.css", Line: intPtr(3), Column: intPtr(5)})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"source":"test.css","start":{"offset":0,"line":3,"column":5},"end":{"offset":6,"line":3,"column":11}}`
	if got := js.Stringify(ast.Get("loc")); got != want {
		t.Fatalf("stylesheet loc\n got %s\nwant %s", got, want)
	}
}

func intPtr(v int) *int { return &v }

func TestOnComment(t *testing.T) {
	var comments []string
	_, err := Parse("/*a*/b{/*c*/}/*d", ParseOptions{OnComment: func(value string, _ *js.Object) {
		comments = append(comments, value)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(comments, "|"); got != "a|c|d" {
		t.Fatalf("comments %q, want %q", got, "a|c|d")
	}
}
