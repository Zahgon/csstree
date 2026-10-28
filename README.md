<img align="right" width="111" height="111" alt="CSSTree logo" src="assets/csstree-logo-rounded.svg" />

# CSSTree

[![Build Status](https://github.com/csstree/csstree/actions/workflows/build.yml/badge.svg)](https://github.com/csstree/csstree/actions/workflows/build.yml)

CSSTree is a tool set for CSS, written in Go: a detailed parser (CSS → AST), walker (AST traversal), generator (AST → CSS, with source maps) and lexer (validation and matching) based on specs and browser implementations. The main goal is to be W3C spec compliant, with focus on CSS analyzing and source-to-source transforming tasks.

## Features

- **Detailed parsing with an adjustable level of detail**

  By default CSSTree parses CSS as detailed as possible, i.e. each single logical part is represented by its own AST node (see [AST format](docs/ast.md) for all possible node types). The level of detail can be changed through [parse options](docs/parsing.md), for example, you can disable parsing of selectors or declaration values for component parts.

- **Tolerant to errors by design**

  The parser behaves as the [spec says](https://www.w3.org/TR/css-syntax-3/#error-handling): "When errors occur in CSS, the parser attempts to recover gracefully, throwing away only the minimum amount of content before returning to parsing as normal". The only thing the parser departs from the specification is that it doesn't throw away bad content, but wraps it in a special node type (`Raw`) that allows processing it later.

- **Syntax validation**

  The built-in lexer can test CSS against syntaxes defined by W3C. CSSTree uses [mdn/data](https://github.com/mdn/data/) (copied into [data/mdn](data/mdn)) as a basis for the lexer's dictionaries and extends it with vendor specific and legacy syntaxes ([data/patch.json](data/patch.json)).

## Install

```
go get github.com/csstree/csstree
go install github.com/csstree/csstree/cmd/csstree@latest
```

The module uses the Go standard library only.

## Documentation

- [AST format](docs/ast.md)
- [Parsing CSS → AST](docs/parsing.md)
- [Serialization AST → CSS](docs/generate.md)
- [AST traversal](docs/traversal.md)
- [Util functions](docs/utils.md) and the [List](docs/list.md) type
- [Value Definition Syntax](docs/definition-syntax.md) and the lexer
- [Command line](docs/command-line.md)

## Packages

| package | contents |
|---|---|
| `github.com/csstree/csstree` | the default syntax: `Parse`, `Generate`, `Walk`, `Find*`, `Clone`, conversions, `Default()` lexer, `Fork` |
| `.../tokenizer` | `Tokenize`, `TokenStream`, `OffsetToLocation`, token types and names |
| `.../syntax` | the parser (`Parse`, `ParseSelector`, `ParseOptions`, `SyntaxError`) |
| `.../generator` | `Generate`, `GenerateWithSourceMap`, `Tokens` |
| `.../walker` | `Walk`, `Find`, `FindLast`, `FindAll`, node structures |
| `.../convertor` | `ToPlainObject`, `FromPlainObject` |
| `.../lexer` | `Lexer` with matching, checks, `Validate`, `Dump`; `Mix` and `DefaultConfig` |
| `.../definitionsyntax` | `Parse`, `Generate`, `Walk` of value definition grammars |
| `.../ast` | `List`, `Clone` |
| `.../utils` | `Keyword`, `Property`, `VendorPrefix`, `IsCustomProperty`, ident/string/url encoding |
| `.../data` | the definitions the default lexer uses |

AST nodes are `*js.Object` values (an ordered string-keyed object from the module's `internal/js` package) holding
the node's properties in the same order and with the same values CSSTree produces, so ASTs serialise to the same
JSON and can be read back from it.

## Usage

```go
package main

import (
	"fmt"

	csstree "github.com/csstree/csstree"
	"github.com/csstree/csstree/generator"
	"github.com/csstree/csstree/syntax"
)

func main() {
	ast, err := csstree.Parse(".example { world: \"!\" }", syntax.ParseOptions{})
	if err != nil {
		panic(err)
	}
	css, err := csstree.Generate(ast, generator.Options{})
	if err != nil {
		panic(err)
	}
	fmt.Println(css) // .example{world:"!"}
}
```

Syntax matching:

```go
s, err := csstree.Default()
if err != nil {
	panic(err)
}
result, err := s.Lexer.MatchProperty("border", "1px solid red")
// result.Matched is the match tree, or nil with result.Error describing the mismatch
```

## Top level API

![API map](docs/api-map.svg)

## Development

```
go build ./...
go vet ./...
go test ./...
```

The tests read the shared test data in [fixtures](fixtures).

## License

MIT
