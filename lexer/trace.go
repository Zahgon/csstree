package lexer

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/walker"
)

func shouldPutToTrace(syntax *js.Object) bool {
	switch syntax.Str("type") {
	case "Type", "Property", "Keyword":
		return true
	}
	return false
}

// GetTrace returns the syntaxes (types, properties and keywords) a node was matched by, from the outermost one,
// or nil when the node is not part of the match.
func (r *MatchResult) GetTrace(node *js.Object) []*js.Object {
	var trace []*js.Object
	found := false
	var hasMatch func(m *js.Object) bool
	hasMatch = func(m *js.Object) bool {
		if children, ok := m.Get("match").([]any); ok {
			for _, child := range children {
				c, _ := child.(*js.Object)
				if hasMatch(c) {
					if syntax := m.Obj("syntax"); syntax != nil && shouldPutToTrace(syntax) {
						trace = append([]*js.Object{syntax}, trace...)
					}
					return true
				}
			}
		} else if n, _ := m.Get("node").(*js.Object); n == node && n != nil {
			trace = []*js.Object{}
			if syntax := m.Obj("syntax"); syntax != nil && shouldPutToTrace(syntax) {
				trace = append(trace, syntax)
			}
			found = true
			return true
		}
		return false
	}
	if r.Matched != nil {
		hasMatch(r.Matched)
	}
	if !found {
		return nil
	}
	return trace
}

func (r *MatchResult) testNode(node *js.Object, fn func(syntax *js.Object) bool) bool {
	for _, syntax := range r.GetTrace(node) {
		if fn(syntax) {
			return true
		}
	}
	return false
}

// IsType reports whether a node was matched by the named type.
func (r *MatchResult) IsType(node *js.Object, typ string) bool {
	return r.testNode(node, func(s *js.Object) bool { return s.Str("type") == "Type" && s.Str("name") == typ })
}

// IsProperty reports whether a node was matched by the named property.
func (r *MatchResult) IsProperty(node *js.Object, property string) bool {
	return r.testNode(node, func(s *js.Object) bool { return s.Str("type") == "Property" && s.Str("name") == property })
}

// IsKeyword reports whether a node was matched by a keyword.
func (r *MatchResult) IsKeyword(node *js.Object) bool {
	return r.testNode(node, func(s *js.Object) bool { return s.Str("type") == "Keyword" })
}

// Fragment is a run of sibling nodes matched by one syntax, with the list holding them.
type Fragment struct {
	Parent *ast.List
	Nodes  *ast.List
}

func firstMatchNode(m *js.Object) *js.Object {
	if m.Has("node") {
		n, _ := m.Get("node").(*js.Object)
		return n
	}
	children, _ := m.Get("match").([]any)
	if len(children) == 0 {
		return nil
	}
	c, _ := children[0].(*js.Object)
	return firstMatchNode(c)
}

func lastMatchNode(m *js.Object) *js.Object {
	if m.Has("node") {
		n, _ := m.Get("node").(*js.Object)
		return n
	}
	children, _ := m.Get("match").([]any)
	if len(children) == 0 {
		return nil
	}
	c, _ := children[len(children)-1].(*js.Object)
	return lastMatchNode(c)
}

func matchFragments(tree any, match *MatchResult, typ, name string) []Fragment {
	fragments := []Fragment{}
	var find func(m *js.Object)
	find = func(m *js.Object) {
		if syntax := m.Obj("syntax"); syntax != nil && syntax.Str("type") == typ && syntax.Str("name") == name {
			start, end := firstMatchNode(m), lastMatchNode(m)
			walker.Visit(tree, func(_ *walker.Context, node *js.Object, item *ast.ListItem, list *ast.List) walker.Action {
				if node == start && node != nil && item != nil {
					nodes := ast.NewList()
					for it := item; it != nil; it = it.Next {
						nodes.AppendData(it.Data)
						if it.Data == any(end) {
							break
						}
					}
					fragments = append(fragments, Fragment{Parent: list, Nodes: nodes})
				}
				return walker.Continue
			}, nil)
		}
		if children, ok := m.Get("match").([]any); ok {
			for _, child := range children {
				c, _ := child.(*js.Object)
				find(c)
			}
		}
	}
	if match.Matched != nil {
		find(match.Matched)
	}
	return fragments
}

// FindValueFragments matches a value against a property and returns the fragments matched by the syntax of the
// given type ("Type", "Property" or "Keyword") and name.
func (l *Lexer) FindValueFragments(property string, value any, typ, name string) ([]Fragment, error) {
	match, err := l.MatchProperty(property, value)
	if err != nil {
		return nil, err
	}
	return matchFragments(value, match, typ, name), nil
}

// FindDeclarationValueFragments is FindValueFragments for the value of a Declaration node.
func (l *Lexer) FindDeclarationValueFragments(declaration *js.Object, typ, name string) ([]Fragment, error) {
	match, err := l.MatchDeclaration(declaration)
	if err != nil {
		return nil, err
	}
	return matchFragments(declaration.Get("value"), match, typ, name), nil
}

// FindAllFragments returns the fragments of every declaration of an AST.
func (l *Lexer) FindAllFragments(tree any, typ, name string) ([]Fragment, error) {
	result := []Fragment{}
	var walkErr error
	err := walker.Walk(tree, walker.Options{Visit: "Declaration", Enter: func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		fragments, err := l.FindDeclarationValueFragments(node, typ, name)
		if err != nil {
			walkErr = err
			return walker.Break
		}
		result = append(result, fragments...)
		return walker.Continue
	}})
	if err != nil {
		return nil, err
	}
	if walkErr != nil {
		return nil, walkErr
	}
	return result, nil
}
