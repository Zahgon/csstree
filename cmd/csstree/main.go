// Command csstree drives the CSSTree library from the command line: tokenize, parse, generate, walk and find in
// CSS, convert and clone ASTs, match and validate values with the lexer, parse value definition syntax, and
// encode or decode names, identifiers, strings and urls. Results are printed on standard output; an error raised
// by the library is printed on standard error as one JSON object and exits 1; a usage error exits 2.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/csstree/csstree/js"
)

// usageError is a bad command line: unknown command or flag, missing or invalid argument, or unreadable input.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usage(format string, args ...any) error {
	return &usageError{fmt.Sprintf(format, args...)}
}

var valueFlags = map[string]bool{"context": true, "atrule": true, "kind": true, "filename": true, "offset": true,
	"line": true, "column": true, "fragment-lines": true, "mode": true, "visit": true, "skip": true, "break": true,
	"name": true, "fork": true, "section": true, "names": true}

var boolFlags = map[string]bool{"positions": true, "no-parse-atrule-prelude": true, "no-parse-rule-prelude": true,
	"no-parse-value": true, "parse-custom-property": true, "selector-parser": true, "from-json": true,
	"tolerant": true, "comments": true, "tokens": true, "source-map": true, "reverse": true, "leave": true,
	"context-fields": true, "ast": true, "pretty": true, "compact": true, "force-braces": true, "apostrophe": true}

// flagSet holds the flags given on the command line: value flags map to their value, boolean flags to "".
type flagSet map[string]string

func (f flagSet) has(name string) bool { _, ok := f[name]; return ok }

func parseArgs(argv []string) (flagSet, []string, error) {
	flags := flagSet{}
	var positional []string
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if !strings.HasPrefix(arg, "--") {
			positional = append(positional, arg)
			continue
		}
		name := arg[2:]
		switch {
		case valueFlags[name]:
			if i+1 >= len(argv) {
				return nil, nil, usage("--%s needs a value", name)
			}
			i++
			flags[name] = argv[i]
		case boolFlags[name]:
			flags[name] = ""
		default:
			return nil, nil, usage("unknown flag %s", arg)
		}
	}
	return flags, positional, nil
}

var integerPattern = regexp.MustCompile(`^-?[0-9]+$`)

// integer returns the value of an integer flag, or nil when the flag is absent.
func (f flagSet) integer(name string) (*int, error) {
	text, ok := f[name]
	if !ok {
		return nil, nil
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if !integerPattern.MatchString(text) || err != nil || value > 1<<53-1 || value < -(1<<53-1) {
		return nil, usage("--%s needs an integer", name)
	}
	v := int(value)
	return &v, nil
}

// readInput reads a file (or standard input for "-") and decodes it as Node.js decodes UTF-8.
func readInput(file string, present bool) (string, error) {
	if !present {
		return "", usage("missing input file")
	}
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return "", usage("cannot read %s: %v", file, err)
	}
	return js.DecodeUTF8(data), nil
}

func readJSON(file string, present bool) (any, error) {
	text, err := readInput(file, present)
	if err != nil {
		return nil, err
	}
	value, err := js.ParseJSON(text)
	if err != nil {
		return nil, usage("%s is not valid JSON", file)
	}
	return value, nil
}

// arg returns positional[i] and whether it is present.
func arg(positional []string, i int) (string, bool) {
	if i < len(positional) {
		return positional[i], true
	}
	return "", false
}

// jsError is an error that knows its JavaScript properties.
type jsError interface {
	JSObject() *js.Object
}

// errorFields returns an error as the original reports it: name, message and the error's own properties.
func errorFields(err error) *js.Object {
	var withFields jsError
	if errors.As(err, &withFields) {
		return withFields.JSObject()
	}
	return js.NewObject("name", "Error", "message", err.Error())
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(argv []string, stdout, stderr io.Writer) int {
	result, err := dispatch(argv)
	if err != nil {
		var ue *usageError
		if errors.As(err, &ue) {
			return fail(stderr, 2, "csstree: "+ue.msg+"\n")
		}
		return fail(stderr, 1, js.ToUTF8(js.Stringify(errorFields(err)))+"\n")
	}
	text, ok := result.(string)
	if !ok {
		text = js.Stringify(result) + "\n"
	}
	if _, err := io.WriteString(stdout, js.ToUTF8(text)); err != nil {
		return fail(stderr, 1, "csstree: writing the result: "+err.Error()+"\n")
	}
	return 0
}

// fail writes a failure report on standard error and returns the exit status; when standard error cannot be
// written either, the exit status 1 is the only report left.
func fail(stderr io.Writer, code int, text string) int {
	if _, err := io.WriteString(stderr, text); err != nil {
		return 1
	}
	return code
}
