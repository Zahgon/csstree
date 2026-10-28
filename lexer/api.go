package lexer

import (
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/utils"
)

// MatchResult is the outcome of a match: the match tree (nil when the value does not match) or the error.
type MatchResult struct {
	Matched    *js.Object
	Iterations int
	Error      error
}

func result(matched *js.Object, err error, iterations int) *MatchResult {
	return &MatchResult{Matched: matched, Error: err, Iterations: iterations}
}

func valueHasVar(tokens []*token) bool {
	for _, tok := range tokens {
		if js.ToLowerCase(tok.value) == "var(" {
			return true
		}
	}
	return false
}

func (l *Lexer) matchSyntax(syntax *descriptor, value any, useCSSWideKeywords bool) *MatchResult {
	tokens, err := prepareTokens(value)
	if err != nil {
		panic(&matchError{err})
	}
	if valueHasVar(tokens) {
		return result(nil, &Error{"Error", "Matching for a tree with var() is not supported"}, 0)
	}
	var res *matchResult
	if useCSSWideKeywords {
		res = matchAsTree(tokens, l.cssWideSyntax, l)
	}
	if !useCSSWideKeywords || res.tree == nil {
		res = matchAsTree(tokens, syntax.matchGraph(), l)
		if res.tree == nil {
			return result(nil, newMatchError(res.reason, syntax.grammar(), value, res), res.iterations)
		}
	}
	return result(res.tree, nil, res.iterations)
}

// guard runs fn and returns the error the original would throw from it.
func guard(fn func() *MatchResult) (res *MatchResult, err error) {
	defer recoverError(&err)
	return fn(), nil
}

// GetAtrule returns an at-rule definition by name; a vendor-prefixed name falls back to the unprefixed one.
func (l *Lexer) GetAtrule(name string, fallbackBasename bool) *atrule {
	d := utils.Keyword(name)
	entry, _ := l.atrules.Get(d.Name).(*atrule)
	if entry == nil && d.Vendor != "" && fallbackBasename {
		entry, _ = l.atrules.Get(d.Basename).(*atrule)
	}
	return entry
}

// GetProperty returns a property definition by name; a vendor-prefixed name falls back to the unprefixed one.
func (l *Lexer) GetProperty(name string, fallbackBasename bool) *descriptor {
	d := utils.Property(name)
	entry := l.properties.lookup(d.Name)
	if entry == nil && d.Vendor != "" && fallbackBasename {
		entry = l.properties.lookup(d.Basename)
	}
	return entry
}

// GetType returns a type definition by name.
func (l *Lexer) GetType(name string) *descriptor {
	return l.types.lookup(name)
}

// CheckAtruleName returns an error for an unknown at-rule.
func (l *Lexer) CheckAtruleName(name string) error {
	if l.GetAtrule(name, true) == nil {
		return newReferenceError("Unknown at-rule", "@"+name, true)
	}
	return nil
}

// CheckAtrulePrelude returns the problem with an at-rule prelude (prelude is CSS text or an AST node; "" or nil
// is no prelude): an unknown at-rule, a prelude that is not allowed, or a missing prelude that is required. err
// is set when the check itself fails, as when the at-rule's grammar does not parse.
func (l *Lexer) CheckAtrulePrelude(name string, prelude any) (problem error, err error) {
	defer recoverError(&err)
	return l.checkAtrulePrelude(name, prelude), nil
}

func (l *Lexer) checkAtrulePrelude(name string, prelude any) error {
	if err := l.CheckAtruleName(name); err != nil {
		return err
	}
	entry := l.GetAtrule(name, true)
	present := js.Truthy(prelude)
	if entry.prelude == nil && present {
		return &Error{"SyntaxError", "At-rule `@" + name + "` should not contain a prelude"}
	}
	if entry.prelude != nil && !present && l.matchSyntax(entry.prelude, "", false).Matched == nil {
		return &Error{"SyntaxError", "At-rule `@" + name + "` should contain a prelude"}
	}
	return nil
}

// CheckAtruleDescriptorName returns an error for an unknown at-rule or descriptor.
func (l *Lexer) CheckAtruleDescriptorName(atruleName, descriptorName string) error {
	if err := l.CheckAtruleName(atruleName); err != nil {
		return err
	}
	entry := l.GetAtrule(atruleName, true)
	d := utils.Keyword(descriptorName)
	if entry.descriptors == nil {
		return &Error{"SyntaxError", "At-rule `@" + atruleName + "` has no known descriptors"}
	}
	if entry.descriptors.lookup(d.Name) == nil && entry.descriptors.lookup(d.Basename) == nil {
		return newReferenceError("Unknown at-rule descriptor", descriptorName, true)
	}
	return nil
}

// CheckPropertyName returns an error for an unknown property.
func (l *Lexer) CheckPropertyName(name string) error {
	if l.GetProperty(name, true) == nil {
		return newReferenceError("Unknown property", name, true)
	}
	return nil
}

// MatchAtrulePrelude matches the prelude of an at-rule.
func (l *Lexer) MatchAtrulePrelude(name string, prelude any) (*MatchResult, error) {
	return guard(func() *MatchResult {
		if err := l.checkAtrulePrelude(name, prelude); err != nil {
			return result(nil, err, 0)
		}
		entry := l.GetAtrule(name, true)
		if entry.prelude == nil {
			return result(nil, nil, 0)
		}
		if !js.Truthy(prelude) {
			prelude = ""
		}
		return l.matchSyntax(entry.prelude, prelude, false)
	})
}

// MatchAtruleDescriptor matches the value of an at-rule descriptor.
func (l *Lexer) MatchAtruleDescriptor(atruleName, descriptorName string, value any) (*MatchResult, error) {
	if err := l.CheckAtruleDescriptorName(atruleName, descriptorName); err != nil {
		return result(nil, err, 0), nil
	}
	entry := l.GetAtrule(atruleName, true)
	d := utils.Keyword(descriptorName)
	syntax := entry.descriptors.lookup(d.Name)
	if syntax == nil {
		syntax = entry.descriptors.lookup(d.Basename)
	}
	return guard(func() *MatchResult { return l.matchSyntax(syntax, value, false) })
}

// MatchDeclaration matches the value of a Declaration node against its property.
func (l *Lexer) MatchDeclaration(node *js.Object) (*MatchResult, error) {
	if node.Str("type") != "Declaration" {
		return result(nil, &Error{"Error", "Not a Declaration node"}, 0), nil
	}
	return l.MatchProperty(node.Str("property"), node.Get("value"))
}

// MatchProperty matches a value against a property's syntax, CSS-wide keywords first.
func (l *Lexer) MatchProperty(name string, value any) (*MatchResult, error) {
	if utils.Property(name).Custom {
		return result(nil, &Error{"Error", "Lexer matching doesn't applicable for custom properties"}, 0), nil
	}
	if err := l.CheckPropertyName(name); err != nil {
		return result(nil, err, 0), nil
	}
	property := l.GetProperty(name, true)
	return guard(func() *MatchResult { return l.matchSyntax(property, value, true) })
}

// MatchType matches a value against a type's syntax.
func (l *Lexer) MatchType(name string, value any) (*MatchResult, error) {
	syntax := l.GetType(name)
	if syntax == nil {
		return result(nil, newReferenceError("Unknown type", name, true), 0), nil
	}
	return guard(func() *MatchResult { return l.matchSyntax(syntax, value, false) })
}

// Match matches a value against a grammar given as text or as a parsed grammar.
func (l *Lexer) Match(syntax any, value any) (*MatchResult, error) {
	switch s := syntax.(type) {
	case string:
	case *js.Object:
		if !js.Truthy(s.Get("type")) {
			return result(nil, newReferenceError("Bad syntax", "", false), 0), nil
		}
	default:
		return result(nil, newReferenceError("Bad syntax", "", false), 0), nil
	}
	d := newDescriptor(syntax, "Type", "anonymous", nil)
	return guard(func() *MatchResult { return l.matchSyntax(d, value, false) })
}
