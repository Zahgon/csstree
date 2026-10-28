# Command line

`go install github.com/csstree/csstree/cmd/csstree@latest` installs `csstree`. Every command reads its input from
a file, or standard input for `-`, and prints its result as JSON (CSS and grammar text for `generate` and
`syntax generate`). An error from the library is printed on standard error as a JSON object with its `name`,
`message` and fields, with exit status 1; a usage error exits 2.

| command | |
|---|---|
| `tokenize FILE` | the token stream |
| `parse [options] FILE` | the AST; `--tolerant`, `--comments`, `--tokens` add recovered errors, comments and tokens |
| `generate [--mode spec] [--source-map] [options] FILE` | CSS, or `{css, map}` |
| `walk [--visit T] [--reverse] [--leave] [--context-fields] [--skip T] [--break T] FILE` | the visits |
| `find`, `find-last`, `find-all TYPE [--name N] FILE` | nodes |
| `convert to-plain\|clone FILE`, `convert from-plain FILE` | |
| `lexer SUBCOMMAND ... [--fork FILE]` | `match-property`, `match-type`, `match`, `match-atrule-prelude`, `match-atrule-descriptor`, `match-declaration`, the `check-*` commands, `check-structure`, `fragments`, `all-fragments`, `trace`, `validate`, `dump` |
| `syntax parse\|generate\|walk FILE` | value definition syntax |
| `names keyword\|property\|vendor-prefix\|is-custom-property FILE`, `ident\|string\|url encode\|decode FILE` | |

Parse options: `--context`, `--atrule`, `--kind`, `--positions`, `--filename`, `--offset`, `--line`, `--column`,
`--no-parse-atrule-prelude`, `--no-parse-rule-prelude`, `--no-parse-value`, `--parse-custom-property`,
`--selector-parser`, `--from-json` (the input is an AST in JSON).
