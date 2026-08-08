# json5 [![GoDoc](https://godoc.org/github.com/titanous/json5?status.svg)](https://godoc.org/github.com/titanous/json5) [![Build Status](https://github.com/titanous/json5/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/titanous/json5/actions/workflows/ci.yml)

This is a Go package that implements decoding of
[JSON5](https://github.com/json5/json5). See [the
documentation](https://godoc.org/github.com/titanous/json5) for usage information.

## Backtick raw strings

This fork supports backtick-delimited raw strings as a non-standard JSON5
extension. They are accepted only as values, not as object keys. Every byte
between the delimiters is preserved exactly: quotes, backslashes, escape-like
sequences, control bytes, and newlines are neither interpreted nor normalized.

The first backtick after the opening delimiter ends the string, so the content
cannot contain a backtick. For example, a shell variable can hold a multiline
JSON5 document without escaping its raw string value:

```sh
payload='{
  command: `printf "line one\n"
C:\tmp\line-two`
}'
```

Other JSON5 implementations may reject this extension.

## JSON conversion command

Convert JSON5 to standard JSON:

```sh
go run ./cmd/json-convert --out json input.json5 output.json
```

Convert standard JSON to JSON5:

```sh
go run ./cmd/json-convert --out json5 input.json output.json5
```

Use `--indent 0` for compact output. Conversion preserves object member order
and duplicate keys. When converting JSON5 to JSON, comments attached to an
object member become a preceding `<name>_hint` member.
