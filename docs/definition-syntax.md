# Value Definition Syntax and the lexer

Package `definitionsyntax` reads the [CSS Value Definition Syntax](https://www.w3.org/TR/css-values-4/#value-defs):

- `Parse(source)` returns the grammar AST (`Group`, `Multiplier`, `Type`, `Property`, `Keyword`, `Function`,
  `String`, `Token`, `Comma`, `AtKeyword`, `Boolean`, with `Range` options) or a `*SyntaxError` with `Input`,
  `Offset` and `RawMessage`.
- `Generate(node, GenerateOptions{Compact, ForceBraces, Decorate})` writes it back.
- `Walk(node, enter, leave)` visits it.

## Lexer

`csstree.Default()` returns the default syntax, whose `Lexer` knows the types, properties and at-rules of
[data](../data). `Fork(extension)` extends it with `generic`, `cssWideKeywords`, `units`, `types`, `properties`
and `atrules` (a syntax starting with `|` extends the existing one).

| method | |
|---|---|
| `MatchProperty(name, value)`, `MatchType(name, value)`, `Match(syntax, value)` | match CSS text or an AST value |
| `MatchDeclaration(node)`, `MatchAtrulePrelude(name, prelude)`, `MatchAtruleDescriptor(atrule, name, value)` | |
| `CheckAtruleName`, `CheckAtrulePrelude`, `CheckAtruleDescriptorName`, `CheckPropertyName` | the problem, or nil |
| `CheckStructure(ast)` | structure warnings of an AST |
| `FindValueFragments`, `FindDeclarationValueFragments`, `FindAllFragments` | nodes matched by a type or keyword |
| `Validate()`, `Dump(asAST, pretty)` | broken definitions; all definitions |

A `MatchResult` holds the match tree (`Matched`) or the `Error` (`*lexer.SyntaxMatchError` with the mismatch
offset and length, `*lexer.SyntaxReferenceError`, or `*lexer.Error`), and answers `GetTrace`, `IsType`,
`IsProperty` and `IsKeyword` for the nodes of the value. Matching stops after 15000 iterations with a warning on
`lexer.Warnings`.
