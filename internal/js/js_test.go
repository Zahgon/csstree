package js

import (
	"math"
	"testing"
)

// The expectations below are what Node.js 22 prints for the same operations.

func TestNumberToString(t *testing.T) {
	cases := map[float64]string{0: "0", -1: "-1", 0.1: "0.1", 1e21: "1e+21", 1e20: "100000000000000000000",
		123456789012345678901: "123456789012345680000", 1.5e-7: "1.5e-7", 1e-6: "0.000001", 5e-324: "5e-324",
		1.7976931348623157e308: "1.7976931348623157e+308", 123e-20: "1.23e-18", math.Copysign(0, -1): "0"}
	for f, want := range cases {
		if got := NumberToString(f); got != want {
			t.Errorf("NumberToString(%v) = %q, want %q", f, got, want)
		}
	}
}

func TestToNumber(t *testing.T) {
	cases := map[string]string{"": "0", " 12 ": "12", "1e400": "Infinity", "-1e-400": "0", "0x1F": "31", "1.": "1",
		".5": "0.5", "+.5e2": "50", "1e": "NaN", "Infinity": "Infinity", "0b101": "5", "1_000": "NaN", "\u00a01\u2028": "1"}
	for s, want := range cases {
		if got := NumberToString(ToNumber(s)); got != want {
			t.Errorf("Number(%q) = %s, want %s", s, got, want)
		}
	}
}

func TestToLowerCase(t *testing.T) {
	cases := map[string]string{"İMPORT": "i̇mport", "-WEBKIT-İΑΣ": "-webkit-i̇ας", "*-MOZ-ΑΣ-Σ": "*-moz-ας-σ",
		"Σ": "σ", "ΑΣ.": "ας.", "ΑΣ'Α": "ασ'α"}
	for s, want := range cases {
		if got := ToLowerCase(s); got != want {
			t.Errorf("toLowerCase(%q) = %q, want %q", s, got, want)
		}
	}
}

func TestStrings(t *testing.T) {
	s := "a😀b"
	if Len(s) != 4 || Stringify(Substring(s, 0, 2)) != `"a\ud83d"` || Stringify(Slice(s, 2, 10)) != `"\ude00b"` ||
		CharCodeAt(s, 1) != 55357 || ToUTF8(Substring(s, 0, 2)) != "a\uFFFD" {
		t.Fatal("UTF-16 string operations")
	}
	if got := DecodeUTF8([]byte("\xe2\x82A\xedA\xf0\x9f\x98A")); got != "\uFFFDA\uFFFDA\uFFFDA" {
		t.Fatalf("DecodeUTF8 = %q", got)
	}
}

func TestJSON(t *testing.T) {
	o, err := ParseObject(`{"type":"X","10":1,"2":2,"b":"\ud800x\u2028<>&\u007f","a":[1,-0,1e400,null,true],"type":"Y"}`)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"2\":2,\"10\":1,\"type\":\"Y\",\"b\":\"\\ud800x\u2028<>&\u007f\",\"a\":[1,0,null,null,true]}"
	if got := Stringify(o); got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}
