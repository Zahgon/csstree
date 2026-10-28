package utils

import (
	"os"
	"testing"

	"github.com/csstree/csstree/internal/js"
)

// TestDecodeEncode runs the original's decode/encode tables (testdata/decode-encode.json holds
// [function, input, expected] rows taken from its test suite).
func TestDecodeEncode(t *testing.T) {
	data, err := os.ReadFile("testdata/decode-encode.json")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := js.ParseJSON(js.DecodeUTF8(data))
	if err != nil {
		t.Fatal(err)
	}
	funcs := map[string]func(string) string{
		"ident > decode":  IdentDecode,
		"ident > encode":  IdentEncode,
		"string > decode": StringDecode,
		"string > encode": func(s string) string { return StringEncode(s, false) },
		"url > decode":    URLDecode,
		"url > encode":    URLEncode,
	}
	for _, r := range rows.([]any) {
		row := r.([]any)
		name, input, want := row[0].(string), row[1].(string), row[2].(string)
		fn, ok := funcs[name]
		if !ok {
			t.Fatalf("no function for %q", name)
		}
		if got := fn(input); got != want {
			t.Errorf("%s(%q) = %q, want %q", name, input, got, want)
		}
	}
}

func TestKeyword(t *testing.T) {
	cases := map[string]string{
		"test":      `{"basename":"test","name":"test","prefix":"","vendor":"","custom":false}`,
		"TesT":      `{"basename":"test","name":"test","prefix":"","vendor":"","custom":false}`,
		"-moz-test": `{"basename":"test","name":"-moz-test","prefix":"-moz-","vendor":"-moz-","custom":false}`,
	}
	for input, want := range cases {
		if got := js.Stringify(Keyword(input).JSObject()); got != want {
			t.Errorf("keyword(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestProperty(t *testing.T) {
	cases := map[string]string{
		"*zoom":            `{"basename":"zoom","name":"zoom","hack":"*","vendor":"","prefix":"*","custom":false}`,
		"//-WEBKIT-margin": `{"basename":"margin","name":"-webkit-margin","hack":"//","vendor":"-webkit-","prefix":"//-webkit-","custom":false}`,
		"--Custom":         `{"basename":"--Custom","name":"--Custom","hack":"","vendor":"","prefix":"","custom":true}`,
	}
	for input, want := range cases {
		if got := js.Stringify(Property(input).JSObject()); got != want {
			t.Errorf("property(%q) = %s, want %s", input, got, want)
		}
	}
	if !IsCustomProperty("--x", 0) || IsCustomProperty("-x", 0) || VendorPrefix("-o-x", 0) != "-o-" {
		t.Error("isCustomProperty or vendorPrefix")
	}
}
