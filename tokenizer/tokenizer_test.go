package tokenizer

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/csstree/csstree/internal/js"
)

const streamCSS = ".test\n{\n  prop: url(foo/bar.jpg) url( a\\(\\33 \\).\\ \\\"\\'test ) calc(1 + 1) \\x \\aa ;\n}<!--<-->\\\n"

type streamToken struct {
	typ     string
	chunk   string
	balance int
}

var streamTokens = []streamToken{
	{"delim-token", ".", 31}, {"ident-token", "test", 31}, {"whitespace-token", "\n", 31}, {"{-token", "{", 25},
	{"whitespace-token", "\n  ", 25}, {"ident-token", "prop", 25}, {"colon-token", ":", 25},
	{"whitespace-token", " ", 25}, {"url-token", "url(foo/bar.jpg)", 25}, {"whitespace-token", " ", 25},
	{"url-token", "url( a\\(\\33 \\).\\ \\\"\\'test )", 25}, {"whitespace-token", " ", 25},
	{"function-token", "calc(", 18}, {"number-token", "1", 18}, {"whitespace-token", " ", 18},
	{"delim-token", "+", 18}, {"whitespace-token", " ", 18}, {"number-token", "1", 18}, {")-token", ")", 12},
	{"whitespace-token", " ", 25}, {"ident-token", "\\x", 25}, {"whitespace-token", " ", 25},
	{"ident-token", "\\aa ", 25}, {"semicolon-token", ";", 25}, {"whitespace-token", "\n", 25}, {"}-token", "}", 3},
	{"CDO-token", "<!--", 31}, {"delim-token", "<", 31}, {"CDC-token", "-->", 31}, {"delim-token", "\\", 31},
	{"whitespace-token", "\n", 31},
}

func TestStreamEdgeCases(t *testing.T) {
	for _, src := range []string{""} {
		s := NewTokenStream(src, Tokenize)
		if !s.EOF || s.TokenType != 0 || s.Source != "" {
			t.Fatalf("empty input: eof=%v type=%d source=%q, want true, 0, \"\"", s.EOF, s.TokenType, s.Source)
		}
	}
	if s := NewTokenStream(streamCSS, nil); s.TokenCount != 0 || !s.EOF {
		t.Fatalf("no tokenizer: tokenCount=%d eof=%v, want 0 and true", s.TokenCount, s.EOF)
	}
}

func TestStreamDump(t *testing.T) {
	s := NewTokenStream(streamCSS, Tokenize)
	want := make([]any, len(streamTokens))
	for i, tok := range streamTokens {
		want[i] = js.NewObject("idx", i, "type", tok.typ, "chunk", tok.chunk, "balance", tok.balance)
	}
	if got, w := js.Stringify(s.Dump()), js.Stringify(want); got != w {
		t.Fatalf("dump()\n got %s\nwant %s", got, w)
	}
}

func TestStreamNext(t *testing.T) {
	s := NewTokenStream(streamCSS, Tokenize)
	var types, starts, ends []string
	for !s.EOF {
		types = append(types, TokenNames[s.TokenType])
		starts = append(starts, fmt.Sprint(s.TokenStart))
		ends = append(ends, fmt.Sprint(s.TokenEnd))
		s.Next()
	}
	var wantTypes, wantStarts, wantEnds []string
	offset := 0
	for _, tok := range streamTokens {
		wantTypes = append(wantTypes, tok.typ)
		wantStarts = append(wantStarts, fmt.Sprint(offset))
		offset += js.Len(tok.chunk)
		wantEnds = append(wantEnds, fmt.Sprint(offset))
	}
	for _, c := range []struct {
		name      string
		got, want []string
	}{
		{"types", types, wantTypes}, {"start", starts, wantStarts}, {"end", ends, wantEnds},
	} {
		if !slices.Equal(c.got, c.want) {
			t.Errorf("next() %s\n got %v\nwant %v", c.name, c.got, c.want)
		}
	}
}

func TestStreamSkip(t *testing.T) {
	s := NewTokenStream(streamCSS, Tokenize)
	var targets []int
	for i, tok := range streamTokens {
		if tok.typ == "ident-token" || tok.typ == "delim-token" {
			targets = append(targets, i)
		}
	}
	var got, want []string
	prev := 0
	for _, idx := range targets {
		s.Skip(idx - prev)
		prev = idx
		got = append(got, TokenNames[s.TokenType])
		want = append(want, streamTokens[idx].typ)
	}
	if len(got) != 8 || !slices.Equal(got, want) {
		t.Fatalf("skip()\n got %v\nwant %v (8 tokens)", got, want)
	}
	s = NewTokenStream(streamCSS, Tokenize)
	s.Skip(len(streamTokens))
	if !s.EOF {
		t.Fatalf("skip() to end: eof=%v, want true", s.EOF)
	}
}

func TestGetBlockTokenPairIndex(t *testing.T) {
	cases := []struct {
		name, source string
		want         []int
	}{
		{"all blocks closed", "start fn({[()]}) end", []int{-1, -1, 9, 8, 7, 6, 5, 4, 3, 2, -1, -1}},
		{"non-closed blocks", "start fn(]}()[{)", []int{-1, -1, -1, -1, -1, 6, 5, -1, -1, -1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewTokenStream(c.source, Tokenize)
			var got []int
			s.ForEachToken(func(_, _, _, index int) { got = append(got, s.GetBlockTokenPairIndex(index)) })
			if !slices.Equal(got, c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestSkipUntilBalanced(t *testing.T) {
	leftCurlyBracket := func(code int) int {
		if code == '{' {
			return 1
		}
		return 0
	}
	semicolonIncluded := func(code int) int {
		if code == ';' {
			return 2
		}
		return 0
	}
	cases := []struct {
		source, start, skip string
		mode                func(int) int
		expected            string
	}{
		{"? { }", "^", "^", leftCurlyBracket, "? "},
		{"div { }", "^", "^", leftCurlyBracket, "div "},
		{"foo(bar(1)(2)(3[{}])(4{}){}(5))", "             ^", "             ^", leftCurlyBracket, "(3[{}])(4{})"},
		{"foo(bar(1) (2) (3[{}]) (4{}) {} (5))", "               ^", "                ^", leftCurlyBracket, "(3[{}]) (4{}) "},
		{"func(a func(;))", "     ^", "       ^", semicolonIncluded, "a func(;)"},
		{"func(a func(;))", "     ^", "            ^", semicolonIncluded, "a func(;)"},
		{"func(a func(;); b)", "     ^", "       ^", semicolonIncluded, "a func(;);"},
		{"func()", "     ^", "     ^", nil, ""},
		{"func([{}])", "      ^", "       ^", nil, "{}"},
		{"func([{})", "     ^", "      ^", nil, "[{})"},
		{"func(1, 2, 3) {}", "^", "      ^", nil, "func(1, 2, 3) {}"},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("testcase#%d", i), func(t *testing.T) {
			s := NewTokenStream(c.source, Tokenize)
			startOffset, skipTo := strings.Index(c.start, "^"), strings.Index(c.skip, "^")
			startToken := s.TokenIndex
			for s.TokenStart < startOffset {
				s.Next()
				startToken = s.TokenIndex
			}
			for s.TokenStart < skipTo {
				s.Next()
			}
			mode := c.mode
			if mode == nil {
				mode = func(int) int { return 0 }
			}
			s.SkipUntilBalanced(startToken, mode)
			if got := js.Substring(c.source, startOffset, s.TokenStart); got != c.expected {
				t.Fatalf("got %q, want %q", got, c.expected)
			}
		})
	}
}

func TestDynamicBuffer(t *testing.T) {
	bufferSize := len(NewTokenStream(streamCSS, Tokenize).OffsetAndType) + 10
	s := NewTokenStream(strings.Repeat(".", bufferSize), Tokenize)
	count := 0
	for !s.EOF {
		count++
		s.Next()
	}
	if count != bufferSize || len(s.OffsetAndType) < bufferSize {
		t.Fatalf("count=%d buffer=%d, want %d and at least %d", count, len(s.OffsetAndType), bufferSize, bufferSize)
	}
}

func fixtureTokens(kind string, value any) (string, string) {
	if s, ok := value.(string); ok {
		return s, js.Stringify([]any{js.NewObject("type", kind, "chunk", s)})
	}
	o := value.(*js.Object)
	return o.Str("value"), js.Stringify(o.Get("tokens"))
}

func TestTokenizeFixtures(t *testing.T) {
	files, err := filepath.Glob("../fixtures/tokenize/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no tokenize fixtures: %v", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		tests, err := js.ParseObject(js.DecodeUTF8(data))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		kind := strings.TrimSuffix(filepath.Base(file), ".json")
		for _, testType := range []string{"valid", "invalid"} {
			list, _ := tests.Get(testType).([]any)
			for idx, value := range list {
				source, want := fixtureTokens(kind, value)
				t.Run(fmt.Sprintf("%s/%s#%d", kind, testType, idx), func(t *testing.T) {
					var actual []any
					Tokenize(source, func(typ, start, end int) {
						actual = append(actual, js.NewObject("type", TokenNames[typ], "chunk", js.Substring(source, start, end)))
					})
					got := js.Stringify(actual)
					if (got == want) != (testType == "valid") {
						t.Fatalf("%q: tokens %s, expected %s to be %s", source, got, want, map[bool]string{true: "equal", false: "different"}[testType == "valid"])
					}
				})
			}
		}
	}
}
