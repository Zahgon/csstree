package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The expected outputs are what the original's reference command line prints for the same arguments.
func TestCommands(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	css := write("a.css", "a{color:red}")
	value := write("v.txt", "1px solid")
	grammar := write("g.txt", "a|b\n")
	cases := []struct {
		args           []string
		code           int
		stdout, stderr string
	}{
		{[]string{"generate", css}, 0, "a{color:red}\n", ""},
		{[]string{"parse", "--context", "value", value}, 0,
			`{"type":"Value","loc":null,"children":[{"type":"Dimension","loc":null,"value":"1","unit":"px"},{"type":"Identifier","loc":null,"name":"solid"}]}` + "\n", ""},
		{[]string{"lexer", "match-property", "border", value}, 0,
			`{"matched":{"syntax":"<'border'>","match":[{"syntax":"<line-width>","match":[{"syntax":"<length>","match":[{"syntax":null,"token":"1px","node":null}]}]},{"syntax":"<line-style>","match":[{"syntax":"solid","token":"solid","node":null}]}]},"error":null}` + "\n", ""},
		{[]string{"syntax", "generate", grammar}, 0, "a | b\n", ""},
		{[]string{"parse", "--context", "nope", css}, 1, "", `{"name":"Error","message":"Unknown context ` + "`nope`" + `"}` + "\n"},
		{[]string{"parse", "--bogus", css}, 2, "", "csstree: unknown flag --bogus\n"},
	}
	for _, c := range cases {
		var stdout, stderr bytes.Buffer
		code := run(c.args, &stdout, &stderr)
		if code != c.code || stdout.String() != c.stdout || stderr.String() != c.stderr {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want %d, %q, %q", c.args, code, stdout.String(),
				stderr.String(), c.code, c.stdout, c.stderr)
		}
	}
}
