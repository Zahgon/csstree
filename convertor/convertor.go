// Package convertor switches the children of every node of an AST between lists and plain arrays.
package convertor

import (
	"github.com/csstree/csstree/ast"
	"github.com/csstree/csstree/internal/js"
	"github.com/csstree/csstree/walker"
)

// FromPlainObject turns the children arrays of every node reached by a walk into lists, in place, and returns
// the AST; children given as a string become a list of its characters.
func FromPlainObject(tree any) any {
	walker.Visit(tree, func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		switch children := node.Get("children").(type) {
		case []any:
			node.Set("children", ast.NewList().FromArray(children))
		case string:
			var chars []any
			for _, r := range children {
				chars = append(chars, string(r))
			}
			if children != "" {
				node.Set("children", ast.NewList().FromArray(chars))
			}
		}
		return walker.Continue
	}, nil)
	return tree
}

// ToPlainObject turns the children lists of every node reached by a walk into arrays, in place, and returns the
// AST.
func ToPlainObject(tree any) any {
	walker.Visit(tree, nil, func(_ *walker.Context, node *js.Object, _ *ast.ListItem, _ *ast.List) walker.Action {
		if children, ok := node.Get("children").(*ast.List); ok {
			node.Set("children", children.ToArray())
		}
		return walker.Continue
	})
	return tree
}
