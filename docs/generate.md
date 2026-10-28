# Translate AST to string

```go
css, err := csstree.Generate(ast, generator.Options{})
css, sourceMap, err := csstree.GenerateWithSourceMap(ast, generator.Options{})
```

`generator.Options`:

- `Mode`: `"safe"` (default) or `"spec"`. Both insert white space between tokens where it is needed to keep them
  apart; `safe` also separates pairs some browsers misread.

`GenerateWithSourceMap` also returns a Source Map v3 object (`version`, `sources`, `names`, `mappings`) mapping the
generated position of every at-rule, selector and declaration back to its `loc`; parse with `Positions: true` to
get one. An unknown node type fails with `Unknown node type: <type>`.

`generator.Tokens` returns the chunks a tree generates with the node each chunk belongs to (the lexer matches tree
values this way).
