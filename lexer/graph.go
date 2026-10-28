// Package lexer matches CSS values against the CSS Value Definition Syntax of properties, types and at-rules,
// reports mismatches with their positions, and checks AST structure; the default definitions come from package
// data.
package lexer

import (
	"github.com/csstree/csstree/definitionsyntax"
	"github.com/csstree/csstree/internal/js"
)

// gnode is a node of a match graph, and also a matcher state: graph nodes keep their identity (the graph has
// cycles) and the MatchOnceBuffer states are mutated while they are on the matcher's stacks.
type gnode struct {
	typ             string
	match, then     *gnode
	els             *gnode
	enum            map[string]*gnode
	terms           []*gnode
	all             bool
	once            *gnode
	index, mask     int
	name, value     string
	syntax          *js.Object
	fn              genericFn
	source          *js.Object
	hasSyntaxSource bool
}

var (
	matchState         = &gnode{typ: "Match"}
	mismatchState      = &gnode{typ: "Mismatch"}
	disallowEmptyState = &gnode{typ: "DisallowEmpty"}
)

func createCondition(match, then, els *gnode) *gnode {
	if then == matchState && els == mismatchState {
		return match
	}
	if match == matchState && then == matchState && els == matchState {
		return match
	}
	if match.typ == "If" && match.els == mismatchState && then == matchState {
		then = match.then
		match = match.match
	}
	return &gnode{typ: "If", match: match, then: then, els: els}
}

func isFunctionType(name string) bool {
	units := js.UTF16(name)
	return len(units) > 2 && units[len(units)-2] == '(' && units[len(units)-1] == ')'
}

func isEnumCapatible(term *gnode) bool {
	return term.typ == "Keyword" || term.typ == "AtKeyword" || term.typ == "Function" ||
		(term.typ == "Type" && isFunctionType(term.name))
}

func groupNode(terms []any, combinator string, explicit bool) *js.Object {
	return js.NewObject("type", "Group", "terms", terms, "combinator", combinator, "disallowEmpty", false,
		"explicit", explicit)
}

func replaceTypeInGraph(node *gnode, replacements map[string]*gnode, visited map[*gnode]bool) *gnode {
	if !visited[node] {
		visited[node] = true
		switch node.typ {
		case "If":
			node.match = replaceTypeInGraph(node.match, replacements, visited)
			node.then = replaceTypeInGraph(node.then, replacements, visited)
			node.els = replaceTypeInGraph(node.els, replacements, visited)
		case "Type":
			if r, ok := replacements[node.name]; ok {
				return r
			}
		}
	}
	return node
}

func without(terms []*gnode, term *gnode) []*gnode {
	var out []*gnode
	for _, t := range terms {
		if t != term {
			out = append(out, t)
		}
	}
	return out
}

func buildGroupMatchGraph(combinator string, terms []*gnode, atLeastOneTermMatched bool) *gnode {
	switch combinator {
	case " ":
		result := matchState
		for i := len(terms) - 1; i >= 0; i-- {
			result = createCondition(terms[i], result, mismatchState)
		}
		return result
	case "|":
		result := mismatchState
		var enum map[string]*gnode
		for i := len(terms) - 1; i >= 0; i-- {
			term := terms[i]
			if isEnumCapatible(term) {
				if enum == nil && i > 0 && isEnumCapatible(terms[i-1]) {
					enum = map[string]*gnode{}
					result = createCondition(&gnode{typ: "Enum", enum: enum}, matchState, result)
				}
				if enum != nil {
					key := term.name
					if isFunctionType(key) {
						key = js.Slice(key, 0, js.Len(key)-1)
					}
					key = js.ToLowerCase(key)
					if _, exists := enum[key]; !exists {
						enum[key] = term
						continue
					}
				}
			}
			enum = nil
			result = createCondition(term, matchState, result)
		}
		return result
	case "&&", "||":
		if len(terms) > 5 {
			return &gnode{typ: "MatchOnce", terms: terms, all: combinator == "&&"}
		}
		result := mismatchState
		if combinator == "||" && atLeastOneTermMatched {
			result = matchState
		}
		for i := len(terms) - 1; i >= 0; i-- {
			term := terms[i]
			thenClause := matchState
			if len(terms) > 1 {
				thenClause = buildGroupMatchGraph(combinator, without(terms, term), combinator == "||")
			}
			result = createCondition(term, thenClause, result)
		}
		return result
	}
	return nil
}

func buildMultiplierMatchGraph(node *js.Object) *gnode {
	result := matchState
	matchTerm := buildMatchGraphInternal(node.Get("term"))
	min, _ := node.Num("min")
	max, _ := node.Num("max")
	comma := node.Bool("comma")
	commaNode := func() *gnode { return &gnode{typ: "Comma", syntax: node} }
	if max == 0 {
		matchTerm = createCondition(matchTerm, disallowEmptyState, mismatchState)
		result = createCondition(matchTerm, nil, mismatchState)
		result.then = createCondition(matchState, matchState, result)
		if comma {
			result.then.els = createCondition(commaNode(), result, mismatchState)
		}
	} else {
		start := min
		if start == 0 {
			start = 1
		}
		for i := start; i <= max; i++ {
			if comma && result != matchState {
				result = createCondition(commaNode(), result, mismatchState)
			}
			result = createCondition(matchTerm, createCondition(matchState, matchState, result), mismatchState)
		}
	}
	if min == 0 {
		result = createCondition(matchState, matchState, result)
	} else {
		for i := 0.0; i < min-1; i++ {
			if comma && result != matchState {
				result = createCondition(commaNode(), result, mismatchState)
			}
			result = createCondition(matchTerm, result, mismatchState)
		}
	}
	return result
}

func keyword(name string) *js.Object { return js.NewObject("type", "Keyword", "name", name) }
func typeRef(name string) *js.Object { return js.NewObject("type", "Type", "name", name) }

func buildBooleanMatchGraph(node *js.Object) *gnode {
	term := buildMatchGraphInternal(node.Get("term"))
	repeat := func(word string) *js.Object {
		return js.NewObject("type", "Multiplier", "comma", false, "min", 0.0, "max", 0.0,
			"term", groupNode([]any{keyword(word), typeRef("!boolean-group")}, " ", false))
	}
	matchNode := buildMatchGraphInternal(groupNode([]any{
		groupNode([]any{keyword("not"), typeRef("!boolean-group")}, " ", false),
		groupNode([]any{typeRef("!boolean-group"), groupNode([]any{repeat("and"), repeat("or")}, "|", false)}, " ", false),
	}, "|", false))
	booleanGroup := buildMatchGraphInternal(groupNode([]any{
		typeRef("!term"),
		groupNode([]any{js.NewObject("type", "Token", "value", "("), typeRef("!self"),
			js.NewObject("type", "Token", "value", ")")}, " ", false),
		typeRef("general-enclosed"),
	}, "|", false))
	replaceTypeInGraph(booleanGroup, map[string]*gnode{"!term": term, "!self": matchNode}, map[*gnode]bool{})
	replaceTypeInGraph(matchNode, map[string]*gnode{"!boolean-group": booleanGroup}, map[*gnode]bool{})
	return matchNode
}

// graphError is an error raised while a match graph is built.
type graphError struct{ err error }

func buildMatchGraphInternal(value any) *gnode {
	if fn, ok := value.(genericFn); ok {
		return &gnode{typ: "Generic", fn: fn}
	}
	node, _ := value.(*js.Object)
	switch node.Str("type") {
	case "Group":
		raw, _ := node.Get("terms").([]any)
		terms := make([]*gnode, len(raw))
		for i, t := range raw {
			terms[i] = buildMatchGraphInternal(t)
		}
		result := buildGroupMatchGraph(node.Str("combinator"), terms, false)
		if node.Bool("disallowEmpty") {
			result = createCondition(result, disallowEmptyState, mismatchState)
		}
		return result
	case "Multiplier":
		return buildMultiplierMatchGraph(node)
	case "Boolean":
		return buildBooleanMatchGraph(node)
	case "Type", "Property":
		return &gnode{typ: node.Str("type"), name: node.Str("name"), syntax: node}
	case "Keyword":
		return &gnode{typ: "Keyword", name: js.ToLowerCase(node.Str("name")), syntax: node}
	case "AtKeyword":
		return &gnode{typ: "AtKeyword", name: "@" + js.ToLowerCase(node.Str("name")), syntax: node}
	case "Function":
		return &gnode{typ: "Function", name: js.ToLowerCase(node.Str("name")) + "(", syntax: node}
	case "String":
		value := node.Str("value")
		if js.Len(value) == 3 {
			return &gnode{typ: "Token", value: js.Substr(value, 1, 1), syntax: node}
		}
		inner := js.Substr(value, 1, js.Len(value)-2)
		return &gnode{typ: "String", value: replaceAll(inner, `\'`, "'"), syntax: node}
	case "Token":
		return &gnode{typ: "Token", value: node.Str("value"), syntax: node}
	case "Comma":
		return &gnode{typ: "Comma", syntax: node}
	}
	panic(&graphError{errorf("Unknown node type:")})
}

// buildMatchGraph builds the match graph of a grammar (a string is parsed first) or of a generic type.
func buildMatchGraph(syntaxTree any, ref *js.Object) *gnode {
	if s, ok := syntaxTree.(string); ok {
		tree, err := definitionsyntax.Parse(s)
		if err != nil {
			panic(&graphError{err})
		}
		syntaxTree = tree
	}
	root := &gnode{typ: "MatchGraph", match: buildMatchGraphInternal(syntaxTree), syntax: ref}
	if tree, ok := syntaxTree.(*js.Object); ok {
		root.source, root.hasSyntaxSource = tree, true
	}
	return root
}
