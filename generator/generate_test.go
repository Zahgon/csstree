package generator_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/csstree/csstree/convertor"
	"github.com/csstree/csstree/generator"
	"github.com/csstree/csstree/internal/fixtureset"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/syntax"
	"github.com/csstree/csstree/tokenizer"
)

func parseOptions(test fixtureset.Test) syntax.ParseOptions {
	opts := syntax.ParseOptions{Context: test.Scope}
	o := test.Data.Obj("options")
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

func generateTests(t *testing.T) []fixtureset.Test {
	t.Helper()
	all, err := fixtureset.Load("../fixtures/ast")
	if err != nil {
		t.Fatal(err)
	}
	var tests []fixtureset.Test
	for _, test := range all {
		if !test.Data.Has("error") && !test.Data.Bool("skip") {
			tests = append(tests, test)
		}
	}
	return tests
}

func expectedCSS(test fixtureset.Test) string {
	if test.Data.Has("generate") {
		return test.Data.Str("generate")
	}
	return test.Data.Str("source")
}

func TestGenerateFixtures(t *testing.T) {
	for _, test := range generateTests(t) {
		t.Run(test.Name, func(t *testing.T) {
			opts := parseOptions(test)
			ast, err := syntax.Parse(test.Data.Str("source"), opts)
			if err != nil {
				t.Fatal(err)
			}
			want := expectedCSS(test)
			if got, err := generator.Generate(ast, generator.Options{}); err != nil || got != want {
				t.Fatalf("generate: got %q (%v), want %q", got, err, want)
			}
			if got, err := generator.Generate(convertor.ToPlainObject(ast), generator.Options{}); err != nil || got != want {
				t.Fatalf("generate (plain object): got %q (%v), want %q", got, err, want)
			}
			if !js.Truthy(test.Data.Get("skipRoundtrip")) {
				css, _ := generator.Generate(ast, generator.Options{})
				again, err := syntax.Parse(css, opts)
				if err != nil {
					t.Fatalf("round-trip parse of %q: %v", css, err)
				}
				if a, b := js.Stringify(again), js.Stringify(ast); a != b {
					t.Fatalf("round-trip\n got %s\nwant %s", a, b)
				}
			}
			opts.Positions = true
			positioned, err := syntax.Parse(test.Data.Str("source"), opts)
			if err != nil {
				t.Fatal(err)
			}
			if got, _, err := generator.GenerateWithSourceMap(positioned, generator.Options{}); err != nil || got != want {
				t.Fatalf("generate with source map: got %q (%v), want %q", got, err, want)
			}
		})
	}
}

func TestUnknownNodeType(t *testing.T) {
	_, err := generator.Generate(js.NewObject("type", "xxx"), generator.Options{})
	if err == nil || err.Error() != "Unknown node type: xxx" {
		t.Fatalf("got %v, want Unknown node type: xxx", err)
	}
}

func TestSourceMap(t *testing.T) {
	cases := []struct{ source, css, mappings string }{
		{".a {\n  color: red;\n}\n", ".a{color:red}", "AAAA,E,CACE,S"},
		{".a { color: #ff0000; } .b { display: block; float: left; } @media foo { .c { color: red } }",
			".a{color:#ff0000}.b{display:block;float:left}@media foo{.c{color:red}}",
			"AAAA,E,CAAK,a,CAAkB,E,CAAK,a,CAAgB,U,CAAe,WAAa,E,CAAK,W"},
	}
	for _, c := range cases {
		ast, err := syntax.Parse(c.source, syntax.ParseOptions{Filename: "test.css", Positions: true})
		if err != nil {
			t.Fatal(err)
		}
		css, sourceMap, err := generator.GenerateWithSourceMap(ast, generator.Options{})
		want := `{"version":3,"sources":["test.css"],"names":[],"mappings":"` + c.mappings + `"}`
		if err != nil || css != c.css || js.Stringify(sourceMap) != want {
			t.Fatalf("got %q %s (%v), want %q %s", css, js.Stringify(sourceMap), err, c.css, want)
		}
	}
}

func tokens(s string) string {
	var out []string
	tokenizer.Tokenize(s, func(typ, start, end int) {
		out = append(out, fmt.Sprintf("%d:%s", typ, js.Substring(s, start, end)))
	})
	return strings.Join(out, " ")
}

// TestSpecModeSeparation checks every pair of sample tokens: spec mode may write them without white space only
// when the result still tokenizes as the two tokens.
func TestSpecModeSeparation(t *testing.T) {
	samples := []string{"foo", "--foo", "--", "fn(", "--fn(", "@test", "#f00", `"foo"`, "url(#)", "@", "#", "-", "+",
		".", "/", "*", "%", "-.5", "-1", "-1e-2", "1e2", "1.5", "0", ".5", "1%", "-1e2%", "1px", "-.5px", "<!--",
		"-->", ":", ";", ",", "[", "]", "(", ")", "{", "}"}
	for _, left := range samples {
		for _, right := range samples {
			value := js.NewObject("type", "Value", "children", []any{
				js.NewObject("type", "Raw", "value", left), js.NewObject("type", "Raw", "value", right)})
			got, err := generator.Generate(convertor.FromPlainObject(value), generator.Options{Mode: "spec"})
			if err != nil {
				t.Fatal(err)
			}
			if got == left+right && tokens(got) != strings.TrimSpace(tokens(left)+" "+tokens(right)) {
				t.Errorf("%q + %q generated without white space but tokenizes differently", left, right)
			}
		}
	}
}

func TestAutoWhiteSpace(t *testing.T) {
	off := false
	cases := []struct {
		opts syntax.ParseOptions
		want string
	}{
		{syntax.ParseOptions{}, "span#foo{border:1% var(--a) #ff0000}"},
		{syntax.ParseOptions{ParseRulePrelude: &off, ParseValue: &off}, "span#foo{border:1%var(--a)#ff0000}"},
	}
	for _, c := range cases {
		ast, err := syntax.Parse("span#foo { border: 1%var(--a)#ff0000; }", c.opts)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := generator.Generate(ast, generator.Options{}); err != nil || got != c.want {
			t.Fatalf("got %q (%v), want %q", got, err, c.want)
		}
	}
}
