package csstree_test

import (
	"testing"

	csstree "github.com/csstree/csstree"
	"github.com/csstree/csstree/js"
	"github.com/csstree/csstree/lexer"
	"github.com/csstree/csstree/syntax"
)

func defaultLexer(t *testing.T) *lexer.Lexer {
	t.Helper()
	s, err := csstree.Default()
	if err != nil {
		t.Fatal(err)
	}
	return s.Lexer
}

func TestLexerChecks(t *testing.T) {
	l := defaultLexer(t)
	cases := []struct {
		name string
		got  error
		want string
	}{
		{"known property", l.CheckPropertyName("color"), ""},
		{"vendor property", l.CheckPropertyName("-webkit-color"), ""},
		{"unknown property", l.CheckPropertyName("colour"), "Unknown property `colour`"},
		{"known at-rule", l.CheckAtruleName("media"), ""},
		{"unknown at-rule", l.CheckAtruleName("foo"), "Unknown at-rule `@foo`"},
		{"known descriptor", l.CheckAtruleDescriptorName("font-face", "src"), ""},
		{"unknown descriptor", l.CheckAtruleDescriptorName("font-face", "foo"), "Unknown at-rule descriptor `foo`"},
		{"no descriptors", l.CheckAtruleDescriptorName("media", "foo"), "At-rule `@media` has no known descriptors"},
	}
	for _, c := range cases {
		got := ""
		if c.got != nil {
			got = c.got.Error()
		}
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestMatchErrors(t *testing.T) {
	l := defaultLexer(t)
	res, err := l.MatchProperty("width", "1İN")
	if err != nil {
		t.Fatal(err)
	}
	mismatch, ok := res.Error.(*lexer.SyntaxMatchError)
	if !ok || mismatch.MismatchOffset != 0 || mismatch.MismatchLength != 3 || mismatch.RawMessage != "Mismatch" {
		t.Fatalf("got %#v, want a mismatch at 0 of length 3", res.Error)
	}
	for _, c := range []struct{ property, value, message string }{
		{"--x", "1px", "Lexer matching doesn't applicable for custom properties"},
		{"width", "var(--x)", "Matching for a tree with var() is not supported"},
		{"widht", "1px", "Unknown property `widht`"},
	} {
		res, err := l.MatchProperty(c.property, c.value)
		if err != nil || res.Error == nil || res.Error.Error() != c.message {
			t.Errorf("%s: %q: got %v (%v), want %q", c.property, c.value, res.Error, err, c.message)
		}
	}
	declaration, err := syntax.Parse("color: red", syntax.ParseOptions{Context: "declaration"})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := l.MatchDeclaration(declaration); err != nil || res.Matched == nil {
		t.Fatalf("declaration color: red should match (%v, %v)", res.Error, err)
	}
}

func TestValidate(t *testing.T) {
	result, err := defaultLexer(t).Validate()
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatalf("the default definitions should be valid, got %s", js.Stringify(result))
	}
}
