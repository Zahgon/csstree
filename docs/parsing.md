# Parsing CSS into AST

```go
ast, err := csstree.Parse(source, syntax.ParseOptions{})
```

`csstree.Parse` (and `syntax.Parse`) returns the AST of the source, a `*js.Object` (see [AST format](ast.md)), or
an error. A `*syntax.SyntaxError` carries `Message`, `Source`, `Offset`, `Line` and `Column`, `FormattedMessage()`
(the message with two lines of source context) and `SourceFragment(extraLines)`.

## Options

`syntax.ParseOptions` fields (a nil pointer is an option that is not given):

| field | default | meaning |
|---|---|---|
| `Context` | `"default"` | what the source is: `default` or `stylesheet`, `atrule`, `atrulePrelude`, `mediaQueryList`, `mediaQuery`, `condition`, `rule`, `selectorList`, `selector`, `block`, `declarationList`, `declaration`, `value`; any other name fails with ``Unknown context `name` `` |
| `Atrule` | | the at-rule name for the `atrulePrelude` context |
| `Kind` | `"media"` | the condition kind for the `condition` context: `media`, `container` or `supports` |
| `Positions` | `false` | add `loc` (source, start and end offset, line and column) to every node |
| `Filename` | `"<unknown>"` | the `source` of every `loc` |
| `Offset`, `Line`, `Column` | 0, 1, 1 | the location of the first character |
| `ParseAtrulePrelude`, `ParseRulePrelude`, `ParseValue` | true | parse those parts into nodes; when false they are kept as `Raw` |
| `ParseCustomProperty` | false | parse custom property values instead of keeping them as `Raw` |
| `OnParseError` | | called with every recovered error and the node that replaced the unparsed input |
| `OnComment` | | called with the text and location of every comment |
| `OnToken` | | called with the type, start and end of every token |

Positions are counted in UTF-16 code units. Errors in a stylesheet are recovered: the unparsable part becomes a
`Raw` node and `OnParseError` is called; an error the parser cannot recover from is returned.

`syntax.ParseSelector` is the selector-only parser (contexts `default`, `selectorList`, `selector`).
