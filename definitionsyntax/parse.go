package definitionsyntax

import (
	"sort"

	"github.com/csstree/csstree/js"
)

const infinity = 8734 // ∞

var combinatorPrecedence = map[string]int{" ": 1, "&&": 2, "||": 3, "|": 4}

func multiplier(comma bool, min, max float64) *js.Object {
	return js.NewObject("type", "Multiplier", "comma", comma, "min", min, "max", max, "term", nil)
}

func (s *scanner) readMultiplierRange() (float64, float64) {
	s.eat('{')
	s.skipWs()
	min := s.scanNumber()
	max := ""
	s.skipWs()
	if s.charCode() == ',' {
		s.pos++
		s.skipWs()
		if s.charCode() != '}' {
			max = s.scanNumber()
			s.skipWs()
		}
	} else {
		max = min
	}
	s.eat('}')
	maxValue := 0.0
	if max != "" {
		maxValue = js.ToNumber(max)
	}
	return js.ToNumber(min), maxValue
}

func (s *scanner) readMultiplier() *js.Object {
	switch s.charCode() {
	case '*':
		s.pos++
		return multiplier(false, 0, 0)
	case '+':
		s.pos++
		return multiplier(false, 1, 0)
	case '?':
		s.pos++
		return multiplier(false, 0, 1)
	case '#':
		s.pos++
		switch s.charCode() {
		case '{':
			min, max := s.readMultiplierRange()
			return multiplier(true, min, max)
		case '?':
			s.pos++
			return multiplier(true, 0, 0)
		}
		return multiplier(true, 1, 0)
	case '{':
		min, max := s.readMultiplierRange()
		return multiplier(false, min, max)
	}
	return nil
}

func (s *scanner) maybeMultiplied(node *js.Object) *js.Object {
	m := s.readMultiplier()
	if m == nil {
		return node
	}
	m.Set("term", node)
	if s.charCode() == '#' && s.charCodeAt(s.pos-1) == '+' {
		return s.maybeMultiplied(m)
	}
	if s.charCode() == '?' && s.charCodeAt(s.pos-1) == '}' {
		return s.maybeMultiplied(m)
	}
	return m
}

func (s *scanner) maybeToken() *js.Object {
	ch := s.peek()
	if ch == "" {
		return nil
	}
	return s.maybeMultiplied(js.NewObject("type", "Token", "value", ch))
}

func (s *scanner) readProperty() *js.Object {
	s.eat('<')
	s.eat('\'')
	name := s.scanWord()
	s.eat('\'')
	s.eat('>')
	return s.maybeMultiplied(js.NewObject("type", "Property", "name", name))
}

// readRangeBound reads one bound of a type range: a signed number that becomes a string when a unit follows.
func (s *scanner) readRangeBound(sign float64) any {
	var bound any = sign * js.ToNumber(s.scanNumber())
	if isNameChar(s.charCode()) {
		bound = js.NumberToString(bound.(float64)) + s.scanWord()
	}
	return bound
}

func (s *scanner) readTypeRange() *js.Object {
	var min, max any
	sign := 1.0
	s.eat('[')
	if s.charCode() == '-' {
		s.peek()
		sign = -1
	}
	if sign == -1 && s.charCode() == infinity {
		s.peek()
	} else {
		min = s.readRangeBound(sign)
	}
	s.skipWs()
	s.eat(',')
	s.skipWs()
	if s.charCode() == infinity {
		s.peek()
	} else {
		sign = 1
		if s.charCode() == '-' {
			s.peek()
			sign = -1
		}
		max = s.readRangeBound(sign)
	}
	s.eat(']')
	return js.NewObject("type", "Range", "min", min, "max", max)
}

func (s *scanner) readType() *js.Object {
	var opts any
	s.eat('<')
	name := s.scanWord()
	if name == "boolean-expr" {
		s.eat('[')
		group := s.readImplicitGroup(']')
		s.eat(']')
		s.eat('>')
		var term any = group
		if terms := group.Get("terms").([]any); len(terms) == 1 {
			term = terms[0]
		}
		return s.maybeMultiplied(js.NewObject("type", "Boolean", "term", term))
	}
	if s.charCode() == '(' && s.nextCharCode() == ')' {
		s.pos += 2
		name += "()"
	}
	if s.charCodeAt(s.findWsEnd(s.pos)) == '[' {
		s.skipWs()
		opts = s.readTypeRange()
	}
	s.eat('>')
	return s.maybeMultiplied(js.NewObject("type", "Type", "name", name, "opts", opts))
}

func (s *scanner) readKeywordOrFunction() *js.Object {
	name := s.scanWord()
	if s.charCode() == '(' {
		s.pos++
		return js.NewObject("type", "Function", "name", name)
	}
	return s.maybeMultiplied(js.NewObject("type", "Keyword", "name", name))
}

func group(terms []any, combinator string) *js.Object {
	return js.NewObject("type", "Group", "terms", terms, "combinator", combinator, "disallowEmpty", false,
		"explicit", false)
}

func isCombinator(term any, value string) (bool, bool) {
	t, _ := term.(*js.Object)
	if t.Str("type") != "Combinator" {
		return false, false
	}
	return true, t.Str("value") == value
}

// regroupTerms nests the terms joined by lower-precedence combinators into groups and returns the combinator of
// the remaining top level ("" when there was none).
func regroupTerms(terms *[]any, combinators []string) string {
	sort.SliceStable(combinators, func(i, j int) bool {
		return combinatorPrecedence[combinators[i]] < combinatorPrecedence[combinators[j]]
	})
	combinator := ""
	for len(combinators) > 0 {
		combinator, combinators = combinators[0], combinators[1:]
		i, subgroupStart := 0, 0
		for ; i < len(*terms); i++ {
			isComb, same := isCombinator((*terms)[i], combinator)
			if !isComb {
				continue
			}
			if same {
				if subgroupStart == -1 {
					subgroupStart = i - 1
				}
				*terms = append((*terms)[:i:i], (*terms)[i+1:]...)
				i--
				continue
			}
			if subgroupStart != -1 && i-subgroupStart > 1 {
				sub := append([]any(nil), (*terms)[subgroupStart:i]...)
				rest := append([]any{group(sub, combinator)}, (*terms)[i:]...)
				*terms = append((*terms)[:subgroupStart:subgroupStart], rest...)
				i = subgroupStart + 1
			}
			subgroupStart = -1
		}
		if subgroupStart != -1 && len(combinators) > 0 {
			sub := append([]any(nil), (*terms)[subgroupStart:i]...)
			rest := append([]any{group(sub, combinator)}, (*terms)[i:]...)
			*terms = append((*terms)[:subgroupStart:subgroupStart], rest...)
		}
	}
	return combinator
}

func (s *scanner) readImplicitGroup(stopCharCode int) *js.Object {
	var combinators []string
	seen := map[string]bool{}
	addCombinator := func(c string) {
		if !seen[c] {
			seen[c] = true
			combinators = append(combinators, c)
		}
	}
	terms := []any{}
	var prevToken *js.Object
	prevTokenPos := s.pos
	prevTokenIsFunction := false
	for s.charCode() != stopCharCode {
		var token *js.Object
		if prevTokenIsFunction {
			token = s.readImplicitGroup(')')
		} else {
			token = s.peekToken()
		}
		if token == nil {
			break
		}
		if token.Str("type") == "Spaces" {
			continue
		}
		if prevTokenIsFunction {
			inner := token.Get("terms").([]any)
			if len(inner) == 0 {
				prevTokenIsFunction = false
				continue
			}
			if token.Str("combinator") == " " {
				for len(inner) > 1 {
					addCombinator(" ")
					terms = append(terms, js.NewObject("type", "Combinator", "value", " "), inner[0])
					inner = inner[1:]
				}
				token = inner[0].(*js.Object)
			}
		}
		if token.Str("type") == "Combinator" {
			if prevToken == nil || prevToken.Str("type") == "Combinator" {
				s.pos = prevTokenPos
				s.error("Unexpected combinator")
			}
			addCombinator(token.Str("value"))
		} else if prevToken != nil && prevToken.Str("type") != "Combinator" {
			addCombinator(" ")
			terms = append(terms, js.NewObject("type", "Combinator", "value", " "))
		}
		terms = append(terms, token)
		prevToken = token
		prevTokenPos = s.pos
		prevTokenIsFunction = token.Str("type") == "Function"
	}
	if prevToken != nil && prevToken.Str("type") == "Combinator" {
		s.pos -= prevTokenPos
		s.error("Unexpected combinator")
	}
	combinator := regroupTerms(&terms, combinators)
	if combinator == "" {
		combinator = " "
	}
	return group(terms, combinator)
}

func (s *scanner) readGroup() *js.Object {
	s.eat('[')
	result := s.readImplicitGroup(']')
	s.eat(']')
	result.Set("explicit", true)
	if s.charCode() == '!' {
		s.pos++
		result.Set("disallowEmpty", true)
	}
	return result
}

// peekToken reads the next term, combinator, comma or white space; nil stops the group.
func (s *scanner) peekToken() *js.Object {
	code := s.charCode()
	switch code {
	case ']', '*', '+', '?', '#', '!':
		return nil
	case '[':
		return s.maybeMultiplied(s.readGroup())
	case '<':
		if s.nextCharCode() == '\'' {
			return s.readProperty()
		}
		return s.readType()
	case '|':
		n := 1
		if s.nextCharCode() == '|' {
			n = 2
		}
		return js.NewObject("type", "Combinator", "value", s.substringToPos(s.pos+n))
	case '&':
		s.pos++
		s.eat('&')
		return js.NewObject("type", "Combinator", "value", "&&")
	case ',':
		s.pos++
		return js.NewObject("type", "Comma")
	case '\'':
		return s.maybeMultiplied(js.NewObject("type", "String", "value", s.scanString()))
	case ' ', '\t', '\n', '\r', '\f':
		return js.NewObject("type", "Spaces", "value", s.scanSpaces())
	case '@':
		if isNameChar(s.nextCharCode()) {
			s.pos++
			return js.NewObject("type", "AtKeyword", "name", s.scanWord())
		}
		return s.maybeToken()
	case '{':
		if next := s.nextCharCode(); next < '0' || next > '9' {
			return s.maybeToken()
		}
		return nil
	}
	if isNameChar(code) {
		return s.readKeywordOrFunction()
	}
	return s.maybeToken()
}

// Parse parses a value definition grammar into its AST (a Group unless the grammar is one explicit group).
func Parse(source string) (result *js.Object, err error) {
	s := &scanner{input: source, str: js.UTF16(source)}
	defer func() {
		if r := recover(); r != nil {
			se, ok := r.(*SyntaxError)
			if !ok {
				panic(r)
			}
			result, err = nil, se
		}
	}()
	result = s.readImplicitGroup(-1)
	if s.pos != len(s.str) {
		s.error("Unexpected input")
	}
	if terms := result.Get("terms").([]any); len(terms) == 1 {
		if t := terms[0].(*js.Object); t.Str("type") == "Group" {
			return t, nil
		}
	}
	return result, nil
}
