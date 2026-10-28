package lexer

import (
	"errors"
	"strings"
	"testing"

	"github.com/csstree/csstree/js"
)

func defaultLexer(t *testing.T) *Lexer {
	t.Helper()
	config, err := DefaultConfig()
	if err != nil {
		t.Fatal(err)
	}
	l, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestGetAtrulePrelude(t *testing.T) {
	l := defaultLexer(t)
	media := l.GetAtrulePrelude("media", true)
	if media == nil || media.Type() != "AtrulePrelude" || media.Name() != "media" {
		t.Fatalf("GetAtrulePrelude(media, true) = %v, want the AtrulePrelude definition named media", media)
	}
	if got := l.GetAtrulePrelude("-webkit-media", true); got != media {
		t.Errorf("GetAtrulePrelude(-webkit-media, true) = %v, want the media prelude %v", got, media)
	}
	if got := l.GetAtrulePrelude("-webkit-media", false); got != nil {
		t.Errorf("GetAtrulePrelude(-webkit-media, false) = %v, want nil", got)
	}
	if got := l.GetAtrulePrelude("font-face", true); got != nil {
		t.Errorf("GetAtrulePrelude(font-face, true) = %v, want nil (no prelude)", got)
	}
}

func TestGetAtruleDescriptor(t *testing.T) {
	l := defaultLexer(t)
	for _, args := range [][2]string{{"font-face", "src"}, {"page", "size"}, {"unknown", "x"}} {
		got, err := l.GetAtruleDescriptor(args[0], args[1])
		if got != nil || !errors.Is(err, ErrAtruleDescriptor) {
			t.Errorf("GetAtruleDescriptor(%q, %q) = %v, %v, want nil, %v", args[0], args[1], got, err, ErrAtruleDescriptor)
		}
	}
	src := l.GetAtrule("font-face", true).Descriptor("src")
	if src == nil || src.Type() != "AtruleDescriptor" || src.Name() != "src" || src.Parent() != "font-face" {
		t.Errorf("font-face descriptor src = %v, want an AtruleDescriptor named src with parent font-face", src)
	}
}

func TestCreateDescriptor(t *testing.T) {
	l := defaultLexer(t)
	d := l.CreateDescriptor("<number> | auto", "Type", "anonymous", nil)
	if d.Type() != "Type" || d.Name() != "anonymous" || d.Parent() != nil || !d.Serializable() {
		t.Errorf("CreateDescriptor = %s %s %v %v, want Type anonymous <nil> true", d.Type(), d.Name(), d.Parent(),
			d.Serializable())
	}
	syntax, err := d.Syntax()
	if err != nil || syntax.Str("type") != "Group" {
		t.Errorf("Syntax() = %v, %v, want a Group grammar and no error", syntax, err)
	}
	if res, err := l.Match(d, "auto"); err != nil || res.Matched == nil || res.Error != nil {
		t.Errorf("Match(d, auto) = %+v, %v, want a match", res, err)
	}
	res, err := l.Match(d, "red")
	var mismatch *SyntaxMatchError
	if err != nil || res.Matched != nil || !errors.As(res.Error, &mismatch) || !strings.HasPrefix(mismatch.Error(), "Mismatch") {
		t.Errorf("Match(d, red) = %+v, %v, want a SyntaxMatchError starting with Mismatch", res, err)
	}
	if _, err := l.CreateDescriptor("<<", "Type", "bad", nil).Syntax(); err == nil || !strings.HasPrefix(err.Error(), "Expect a keyword") {
		t.Errorf("Syntax() of <<: error %v, want one starting with Expect a keyword", err)
	}
}

func TestDescriptorSerializable(t *testing.T) {
	l := defaultLexer(t)
	number := l.GetType("number")
	if syntax, err := number.Syntax(); number.Serializable() || syntax != nil || err != nil {
		t.Errorf("generic type number: serializable %v, syntax %v, %v, want false, nil, nil", number.Serializable(), syntax, err)
	}
	color := l.GetType("color")
	if syntax, err := color.Syntax(); !color.Serializable() || err != nil || syntax.Str("type") != "Group" {
		t.Errorf("type color: serializable %v, syntax %v, %v, want true and a Group grammar", color.Serializable(), syntax, err)
	}
}

func TestToString(t *testing.T) {
	l := defaultLexer(t)
	got, err := l.ToString()
	if err != nil {
		t.Fatal(err)
	}
	dump, err := l.Dump(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := js.Stringify(dump); got != want {
		t.Errorf("ToString() differs from the compact dump: got %.80s, want %.80s", got, want)
	}
	if want := `{"generic":true,"cssWideKeywords":["initial","inherit","unse`; !strings.HasPrefix(got, want) {
		t.Errorf("ToString() = %.60s, want prefix %s", got, want)
	}
	broken, err := New(js.NewObject("types", js.NewObject("foo", "<<")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := broken.ToString(); err == nil || !strings.HasPrefix(err.Error(), "Expect a keyword") {
		t.Errorf("ToString() with type foo <<: error %v, want one starting with Expect a keyword", err)
	}
}
