# AST traversal

```go
err := csstree.Walk(ast, walker.Options{
	Enter: func(ctx *walker.Context, node *js.Object, item *ast.ListItem, list *ast.List) walker.Action {
		fmt.Println(node.Str("type"))
		return walker.Continue
	},
})
```

`walker.Options`:

- `Enter`, `Leave`: called before and after a node's children; at least one is required. They return
  `walker.Continue`, `walker.Skip` (do not enter the node's children; from `Enter` only) or `walker.Break` (stop
  the walk).
- `Reverse`: walk children and fields in reverse order.
- `Visit`: call the visitors for one node type only. For `Atrule`, `Rule` and `Declaration` the walk only enters
  the nodes that can contain them (stylesheets, at-rules, rules, blocks and declaration lists). An unknown type
  fails with the list of known types.

`walker.Context` holds the root and the nearest enclosing stylesheet, at-rule, at-rule prelude, rule, selector
list (`Selector`), block, declaration and function.

`csstree.Find`, `FindLast` and `FindAll` return the first, last or every node for which a function returns true.
