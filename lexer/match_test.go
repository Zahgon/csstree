package lexer

import (
	"testing"

	"github.com/csstree/csstree/data"
	"github.com/csstree/csstree/internal/fixtureset"
	"github.com/csstree/csstree/js"
)

// fixtureLexer builds the registry the original's definition-syntax-match tests use: the default types and
// properties when the fixture asks for them, then the generic types, then the fixture's own types and
// properties.
func fixtureLexer(t *testing.T, test *js.Object) *Lexer {
	t.Helper()
	l := &Lexer{types: newDict(), properties: newDict(), units: js.NewObject(), atrules: js.NewObject()}
	for _, u := range defaultUnits {
		l.units.Set(u.group, toAnySlice(u.units))
	}
	if js.Truthy(test.Get("defaultSyntaxes")) {
		d, err := data.Load()
		if err != nil {
			t.Fatal(err)
		}
		for _, kind := range []string{"types", "properties"} {
			defs := d.Obj(kind)
			for _, name := range defs.Keys() {
				if kind == "types" {
					l.addType(name, defs.Get(name))
				} else {
					l.addProperty(name, defs.Get(name))
				}
			}
		}
	}
	for _, g := range createGenericTypes(l.units) {
		l.addType(g.name, g.fn)
	}
	for _, kind := range []string{"types", "properties"} {
		defs := test.Obj(kind)
		for _, name := range defs.Keys() {
			if kind == "types" {
				l.addType(name, defs.Get(name))
			} else {
				l.addProperty(name, defs.Get(name))
			}
		}
	}
	return l
}

func TestDefinitionSyntaxMatch(t *testing.T) {
	tests, err := fixtureset.Load("../fixtures/definition-syntax-match")
	if err != nil || len(tests) == 0 {
		t.Fatalf("fixtures: %v", err)
	}
	for _, test := range tests {
		data := test.Data
		if js.Truthy(data.Get("skip")) {
			continue
		}
		t.Run(test.Name, func(t *testing.T) {
			l := fixtureLexer(t, data)
			grammar := test.Key
			if s, ok := data.Get("syntax").(string); ok {
				grammar = s
			}
			for _, kind := range []string{"valid", "invalid"} {
				values, _ := data.Get(kind).([]any)
				for _, v := range values {
					value, _ := v.(string)
					tokens, err := prepareTokens(value)
					if err != nil {
						t.Fatal(err)
					}
					var res *matchResult
					func() {
						defer func() {
							if r := recover(); r != nil {
								t.Fatalf("%s %q: %v", grammar, value, r)
							}
						}()
						res = internalMatch(tokens, buildMatchGraph(grammar, nil), l)
					}()
					if (res.match != nil) != (kind == "valid") {
						t.Errorf("%s: %q should be %s (%s)", grammar, value, kind, res.reason)
					}
				}
			}
		})
	}
}
