package definitionsyntax

import (
	"testing"

	"github.com/csstree/csstree/internal/fixtureset"
	"github.com/csstree/csstree/js"
)

func grammars(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, dir := range []string{"../fixtures/definition-syntax", "../fixtures/definition-syntax-match"} {
		tests, err := fixtureset.Load(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range tests {
			if s, ok := test.Data.Get("syntax").(string); ok {
				out = append(out, s)
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("no grammars in the fixtures")
	}
	return out
}

func TestParseGenerateRoundTrip(t *testing.T) {
	for _, source := range grammars(t) {
		tree, err := Parse(source)
		if err != nil {
			t.Fatalf("parse %q: %v", source, err)
		}
		for _, opts := range []GenerateOptions{{}, {Compact: true}, {ForceBraces: true}} {
			text, err := Generate(tree, opts)
			if err != nil {
				t.Fatalf("generate %q: %v", source, err)
			}
			again, err := Parse(text)
			if err != nil {
				t.Fatalf("parse generated %q: %v", text, err)
			}
			if !opts.ForceBraces && js.Stringify(again) != js.Stringify(tree) {
				t.Fatalf("%q generated as %q parses differently", source, text)
			}
		}
	}
}

func TestWalkBalanced(t *testing.T) {
	for _, source := range grammars(t) {
		tree, err := Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		depth, entered := 0, 0
		err = Walk(tree, func(*js.Object) { depth++; entered++ }, func(*js.Object) { depth-- })
		if err != nil || depth != 0 || entered == 0 {
			t.Fatalf("walk %q: depth %d after %d nodes (%v)", source, depth, entered, err)
		}
	}
	if err := Walk(js.NewObject("type", "Group", "terms", []any{}), nil, nil); err == nil {
		t.Fatal("a walk without visitors must fail")
	}
}

func TestParseValues(t *testing.T) {
	cases := []struct{ source, want string }{
		{"<x [-0,5]>", `{"type":"Type","name":"x","opts":{"type":"Range","min":0,"max":5}}`},
		{"<x [1e5,5]>", `{"type":"Type","name":"x","opts":{"type":"Range","min":"1e5","max":5}}`},
		{"<x [007px,0010]>", `{"type":"Type","name":"x","opts":{"type":"Range","min":"7px","max":10}}`},
		{"<x [-5px,∞]>", `{"type":"Type","name":"x","opts":{"type":"Range","min":"-5px","max":null}}`},
		{"<x>{0003,0}", `{"type":"Multiplier","comma":false,"min":3,"max":0,"term":{"type":"Type","name":"x","opts":null}}`},
	}
	for _, c := range cases {
		tree, err := Parse(c.source)
		if err != nil {
			t.Fatal(err)
		}
		terms := tree.Get("terms").([]any)
		if got := js.Stringify(terms[0]); got != c.want {
			t.Fatalf("parse %q\n got %s\nwant %s", c.source, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct{ source, message string }{
		{"<x [1,2]", "Expect `>`\n  <x [1,2]\n----------^"},
		{"<😀>", "Expect a keyword\n  <😀>\n---^"},
		{"a | | b", "Unexpected combinator\n  a | | b\n-----^"},
	}
	for _, c := range cases {
		_, err := Parse(c.source)
		if err == nil || err.Error() != c.message {
			t.Fatalf("parse %q: got %v, want %q", c.source, err, c.message)
		}
	}
}
