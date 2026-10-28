package lexer

import (
	"math"

	"github.com/csstree/csstree/internal/js"
	t "github.com/csstree/csstree/tokenizer"
)

// genericFn matches a generic type at a token and returns how many tokens it consumes (0: no match).
type genericFn func(tok *token, next func(int) *token, opts *js.Object) int

var (
	calcFunctionNames       = []string{"calc(", "-moz-calc(", "-webkit-calc("}
	comparisonFunctionNames = []string{"min(", "max(", "clamp("}
	steppedFunctionNames    = []string{"round(", "mod(", "rem("}
	trigNumberFunctionNames = []string{"sin(", "cos(", "tan("}
	trigAngleFunctionNames  = []string{"asin(", "acos(", "atan(", "atan2("}
	otherNumberFunctions    = []string{"pow(", "sqrt(", "log(", "exp(", "sign("}
	hypotFunctionNames      = []string{"hypot("}
	signFunctionNames       = []string{"abs("}
	numberFunctionNames     = concat(calcFunctionNames, comparisonFunctionNames, steppedFunctionNames,
		trigNumberFunctionNames, otherNumberFunctions, hypotFunctionNames, signFunctionNames)
	percentageFunctionNames = concat(calcFunctionNames, comparisonFunctionNames, steppedFunctionNames,
		hypotFunctionNames, signFunctionNames)
	dimensionFunctionNames = concat(calcFunctionNames, comparisonFunctionNames, steppedFunctionNames,
		trigAngleFunctionNames, hypotFunctionNames, signFunctionNames)
)

func concat(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

var genericBalancePair = map[int]int{t.Function: t.RightParenthesis, t.LeftParenthesis: t.RightParenthesis,
	t.LeftSquareBracket: t.RightSquareBracket, t.LeftCurlyBracket: t.RightCurlyBracket}

// undefinedType stands for the undefined a JavaScript pop() of an empty stack returns.
const undefinedType = -1

func codeAt(s string, i int) int {
	if c := js.CharCodeAt(s, i); c >= 0 {
		return c
	}
	return 0
}

func eqStr(actual, expected string) bool {
	units := js.UTF16(actual)
	return t.CmpStr(units, 0, len(units), expected)
}

func eqStrAny(actual string, expected []string) bool {
	for _, e := range expected {
		if eqStr(actual, e) {
			return true
		}
	}
	return false
}

func isPostfixIeHack(s string, offset int) bool {
	return offset == js.Len(s)-2 && codeAt(s, offset) == '\\' && t.IsDigit(codeAt(s, offset+1))
}

// outOfRange checks the numeric part of a value against a Range: Number() of the text, and only bounds that
// are numbers (a bound with a unit is a string and is ignored).
func outOfRange(opts *js.Object, value string, numEnd int) bool {
	if opts.Str("type") != "Range" {
		return false
	}
	text := value
	if numEnd >= 0 && numEnd != js.Len(value) {
		text = js.Substr(value, 0, numEnd)
	}
	num := js.ToNumber(text)
	if math.IsNaN(num) {
		return true
	}
	if min, ok := opts.Get("min").(float64); ok && num < min {
		return true
	}
	if max, ok := opts.Get("max").(float64); ok && num > max {
		return true
	}
	return false
}

// balancedLength counts the tokens of a balanced run; stopAt tells which tokens end it outside nested blocks;
// closing the outermost block ends a function argument list when closeEnds is set.
func balancedLength(tok *token, next func(int) *token, stopAt func(tok *token, closeType int) bool, closeEnds bool) int {
	closeType := 0
	var stash []int
	length := 0
	for tok != nil {
		if stopAt != nil && stopAt(tok, closeType) {
			break
		}
		switch tok.typ {
		case t.RightCurlyBracket, t.RightParenthesis, t.RightSquareBracket:
			if tok.typ != closeType {
				return length
			}
			closeType = undefinedType
			if n := len(stash); n > 0 {
				closeType, stash = stash[n-1], stash[:n-1]
			}
			if closeEnds && len(stash) == 0 {
				return length + 1
			}
		case t.Function, t.LeftParenthesis, t.LeftSquareBracket, t.LeftCurlyBracket:
			stash = append(stash, closeType)
			closeType = genericBalancePair[tok.typ]
		}
		length++
		tok = next(length)
	}
	return length
}

func consumeFunction(tok *token, next func(int) *token) int {
	return balancedLength(tok, next, nil, true)
}

func math2(fn genericFn, names []string) genericFn {
	return func(tok *token, next func(int) *token, opts *js.Object) int {
		if tok == nil {
			return 0
		}
		if tok.typ == t.Function && eqStrAny(tok.value, names) {
			return consumeFunction(tok, next)
		}
		return fn(tok, next, opts)
	}
}

func tokenType(expected int) genericFn {
	return func(tok *token, _ func(int) *token, _ *js.Object) int {
		if tok == nil || tok.typ != expected {
			return 0
		}
		return 1
	}
}

func customIdent(tok *token, _ func(int) *token, _ *js.Object) int {
	if tok == nil || tok.typ != t.Ident {
		return 0
	}
	name := js.ToLowerCase(tok.value)
	if eqStrAny(name, cssWideKeywords) || eqStr(name, "default") {
		return 0
	}
	return 1
}

func dashedIdent(tok *token, _ func(int) *token, _ *js.Object) int {
	if tok == nil || tok.typ != t.Ident || codeAt(tok.value, 0) != '-' || codeAt(tok.value, 1) != '-' {
		return 0
	}
	return 1
}

func customPropertyName(tok *token, next func(int) *token, opts *js.Object) int {
	if dashedIdent(tok, next, opts) == 0 || tok.value == "--" {
		return 0
	}
	return 1
}

func hexColor(tok *token, _ func(int) *token, _ *js.Object) int {
	if tok == nil || tok.typ != t.Hash {
		return 0
	}
	n := js.Len(tok.value)
	if n != 4 && n != 5 && n != 7 && n != 9 {
		return 0
	}
	for i := 1; i < n; i++ {
		if !t.IsHexDigit(codeAt(tok.value, i)) {
			return 0
		}
	}
	return 1
}

func idSelector(tok *token, _ func(int) *token, _ *js.Object) int {
	if tok == nil || tok.typ != t.Hash {
		return 0
	}
	if !t.IsIdentifierStart(codeAt(tok.value, 1), codeAt(tok.value, 2), codeAt(tok.value, 3)) {
		return 0
	}
	return 1
}

func declarationValue(tok *token, next func(int) *token, _ *js.Object) int {
	return balancedLength(tok, next, func(tok *token, closeType int) bool {
		switch tok.typ {
		case t.BadString, t.BadURL:
			return true
		case t.Semicolon:
			return closeType == 0
		case t.Delim:
			return closeType == 0 && tok.value == "!"
		}
		return false
	}, false)
}

func anyValue(tok *token, next func(int) *token, _ *js.Object) int {
	return balancedLength(tok, next, func(tok *token, _ int) bool {
		return tok.typ == t.BadString || tok.typ == t.BadURL
	}, false)
}

// dimension matches a dimension token whose unit (after js.ToLowerCase) is in units; nil units accept any unit.
func dimension(units []string, anyUnit bool) genericFn {
	set := map[string]bool{}
	for _, u := range units {
		set[u] = true
	}
	return func(tok *token, _ func(int) *token, opts *js.Object) int {
		if tok == nil || tok.typ != t.Dimension {
			return 0
		}
		numberEnd := t.ConsumeNumber(js.UTF16(tok.value), 0)
		if !anyUnit {
			solidus := js.IndexOf(tok.value, "\\", numberEnd)
			var unit string
			if solidus == -1 || !isPostfixIeHack(tok.value, solidus) {
				unit = js.Substr(tok.value, numberEnd, js.Len(tok.value))
			} else {
				unit = js.Substring(tok.value, numberEnd, solidus)
			}
			if !set[js.ToLowerCase(unit)] {
				return 0
			}
		}
		if outOfRange(opts, tok.value, numberEnd) {
			return 0
		}
		return 1
	}
}

func percentage(tok *token, _ func(int) *token, opts *js.Object) int {
	if tok == nil || tok.typ != t.Percentage || outOfRange(opts, tok.value, js.Len(tok.value)-1) {
		return 0
	}
	return 1
}

func zero(next genericFn) genericFn {
	if next == nil {
		next = func(*token, func(int) *token, *js.Object) int { return 0 }
	}
	return func(tok *token, getNext func(int) *token, opts *js.Object) int {
		if tok != nil && tok.typ == t.Number && js.ToNumber(tok.value) == 0 {
			return 1
		}
		return next(tok, getNext, opts)
	}
}

func number(tok *token, _ func(int) *token, opts *js.Object) int {
	if tok == nil {
		return 0
	}
	numberEnd := t.ConsumeNumber(js.UTF16(tok.value), 0)
	if numberEnd != js.Len(tok.value) && !isPostfixIeHack(tok.value, numberEnd) {
		return 0
	}
	if outOfRange(opts, tok.value, numberEnd) {
		return 0
	}
	return 1
}

func integer(tok *token, _ func(int) *token, opts *js.Object) int {
	if tok == nil || tok.typ != t.Number {
		return 0
	}
	i := 0
	if c := codeAt(tok.value, 0); c == '+' || c == '-' {
		i = 1
	}
	for ; i < js.Len(tok.value); i++ {
		if !t.IsDigit(codeAt(tok.value, i)) {
			return 0
		}
	}
	if outOfRange(opts, tok.value, i) {
		return 0
	}
	return 1
}

var unitGroups = []string{"length", "angle", "time", "frequency", "resolution", "flex", "decibel", "semitones"}

func attrUnit(units *js.Object) genericFn {
	set := map[string]bool{}
	for _, group := range unitGroups {
		if list, ok := units.Get(group).([]any); ok {
			for _, u := range list {
				set[js.ToLowerCase(js.ToString(u))] = true
			}
		}
	}
	return func(tok *token, _ func(int) *token, _ *js.Object) int {
		if tok == nil {
			return 0
		}
		if (tok.typ == t.Delim && tok.value == "%") || (tok.typ == t.Ident && set[js.ToLowerCase(tok.value)]) {
			return 1
		}
		return 0
	}
}

func unitList(units *js.Object, group string) []string {
	var out []string
	list, _ := units.Get(group).([]any)
	for _, u := range list {
		out = append(out, js.ToString(u))
	}
	return out
}

type genericEntry struct {
	name string
	fn   genericFn
}

// createGenericTypes returns the generic types in the original's order: token types, productions, dimensions
// and attr-unit.
func createGenericTypes(units *js.Object) []genericEntry {
	var types []genericEntry
	for i, name := range t.TokenNames[1:] {
		if name == "comment-token" {
			continue
		}
		types = append(types, genericEntry{name, tokenType(i + 1)})
	}
	dim := func(group string) genericFn {
		return math2(dimension(unitList(units, group), false), dimensionFunctionNames)
	}
	return append(types,
		genericEntry{"string", tokenType(t.String)},
		genericEntry{"ident", tokenType(t.Ident)},
		genericEntry{"percentage", math2(percentage, percentageFunctionNames)},
		genericEntry{"zero", zero(nil)},
		genericEntry{"number", math2(number, numberFunctionNames)},
		genericEntry{"integer", math2(integer, numberFunctionNames)},
		genericEntry{"custom-ident", customIdent},
		genericEntry{"dashed-ident", dashedIdent},
		genericEntry{"custom-property-name", customPropertyName},
		genericEntry{"hex-color", hexColor},
		genericEntry{"id-selector", idSelector},
		genericEntry{"an-plus-b", anPlusB},
		genericEntry{"urange", urange},
		genericEntry{"declaration-value", declarationValue},
		genericEntry{"any-value", anyValue},
		genericEntry{"dimension", math2(dimension(nil, true), dimensionFunctionNames)},
		genericEntry{"angle", dim("angle")},
		genericEntry{"decibel", dim("decibel")},
		genericEntry{"frequency", dim("frequency")},
		genericEntry{"flex", dim("flex")},
		genericEntry{"length", math2(zero(dimension(unitList(units, "length"), false)), dimensionFunctionNames)},
		genericEntry{"resolution", dim("resolution")},
		genericEntry{"semitones", dim("semitones")},
		genericEntry{"time", dim("time")},
		genericEntry{"attr-unit", attrUnit(units)},
	)
}
