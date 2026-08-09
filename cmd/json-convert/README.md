# json-convert Reference Manual

English | [中文](README_zh.md)

`json-convert` performs bidirectional file-to-file conversion between strict JSON and the JSON5 subset supported by this project. It can also derive a Go struct definition file from JSON5. The converter uses an ordered syntax tree, so it preserves object member order, duplicate keys, and original number text. When converting JSON5 to JSON, it can also convert comments associated with object members into `<field-name>_hint` fields.

> [!IMPORTANT]
> The backtick-delimited raw strings generated and accepted by this tool are a project extension and are **not standard JSON5 syntax**. Before sending output to another JSON5 implementation, confirm that it supports backtick strings; otherwise, use `--out json` to generate strict JSON.

## Quick Start

On Windows, run these four commands from the repository root in PowerShell or Git Bash:

```bash
go build -o json-convert.exe ./cmd/json-convert
./json-convert.exe --out json5 input.json output.json5
./json-convert.exe --out json --indent 4 input.json5 output.json
./json-convert.exe --out golang config.json5 config.go
```

The first command builds the Windows executable. The remaining three commands perform JSON → JSON5, JSON5 → JSON, and conversion to Go struct definitions, respectively. On Linux/macOS, change the output filename to `json-convert` and run it as `./json-convert`.

## Command Syntax and Options

```text
json-convert --out json|json5|golang [--indent 0..8] INPUT [OUTPUT]
```

| Option | Required | Default | Description |
| --- | --- | --- | --- |
| `--out json` or `--out=json` | Yes (choose one of three) | None | Converts JSON5 input to strict JSON. |
| `--out json5` or `--out=json5` | Yes (choose one of three) | None | Converts strict JSON input to this project's JSON5 format. |
| `--out golang` or `--out=golang` | Yes (choose one of three) | None | Converts JSON5 input to a Go struct definition file. |
| `--indent N` or `--indent=N` | No | `2` | Number of spaces per indentation level. `N` must be an integer from `0` to `8`; `0` produces compact output. This option is ignored when generating Go files. |
| `INPUT` | Yes | None | Input file path. |
| `OUTPUT` | No | Derived automatically | Output file path. If omitted or explicitly passed as empty, the tool generates a filename consisting of `<input-name>_convert` plus the target extension. |

There may be one or two positional arguments: `INPUT` and the optional `OUTPUT`. When `OUTPUT` is empty:

- `--out json` generates `<input-name>_convert.json`;
- `--out json5` generates `<input-name>_convert.json5`;
- `--out golang` generates `<input-name>_convert.go`.

Options must precede positional arguments. The underlying Go `flag` parser stops parsing options after the first positional argument.

## Process Behavior

| Exit Code | Meaning | Output Location |
| --- | --- | --- |
| `0` | Conversion succeeded | Both `stdout` and `stderr` are empty. |
| `1` | Reading, parsing, conversion, or writing failed | The error is written to `stderr`. |
| `2` | Invalid command-line arguments | Usage information and the error are written to `stderr`. |

The tool never writes conversion results to `stdout`; it writes only to the specified `OUTPUT` file. Every successful output ends with **exactly one LF byte** (`0x0A`), including on Windows; it is not converted to CRLF.

## JSON → JSON5

With `--out json5`, the input is parsed as strict JSON.

### Strict JSON Input Rules

Supported:

- `null`, `true`, and `false`;
- double-quoted strings and standard JSON escapes, including Unicode surrogate pairs;
- arrays and objects; object keys must be double-quoted;
- standard decimal numbers and exponents, such as `-0`, `1.0`, and `1E+2`;
- space, tab, LF, and CR whitespace.

Rejected:

- comments, unquoted keys, single-quoted strings, and backtick strings;
- trailing commas in arrays or objects;
- `+1`, `.5`, `1.`, hexadecimal numbers, `Infinity`, and `NaN`;
- leading zeros (such as `01`), invalid escapes, unescaped control characters, invalid UTF-8, and unpaired surrogates;
- form feed whitespace and extra content after the root value.

Object members are emitted in input order, and duplicate keys are not merged. Numbers are not parsed as floating point, so their original text is preserved; for example, `1.2300`, `1E+6`, and integers of arbitrary length remain unchanged.

### String Output Strategy

- Object keys always use standard JSON double quotes.
- A string value that contains no backtick uses a backtick-delimited raw string. Decoded newlines, quotes, backslashes, and control bytes appear directly between the backticks.
- A string value containing a backtick falls back to a standard JSON double-quoted string and is escaped according to JSON rules.
- A fallback string must be valid UTF-8; strict JSON input already guarantees this.

### Complete Example

`input.json`:

```json
{"z":"line\n\"quoted\"","tick":"has ` mark","z":1.2300,"huge":123456789012345678901234567890}
```

Run:

```bash
go run ./cmd/json-convert --out json5 --indent 2 input.json output.json5
```

The actual byte content of `output.json5` is shown below. The first `z` value contains an actual LF, not the character sequence `\n`:

```text
{
  "z": `line
"quoted"`,
  "tick": "has ` mark",
  "z": 1.2300,
  "huge": 123456789012345678901234567890
}
```

### Compact Output Example

Convert strict JSON to JSON5 without formatting whitespace:

```bash
./json-convert.exe --out json5 --indent 0 input.json compact.json5
```

For example, when the input is `{"name":"demo","enabled":true}`, `compact.json5` contains:

```text
{"name":`demo`,"enabled":true}
```

## JSON5 → JSON

With `--out json`, the input is parsed as the JSON5 subset supported by this project, and the output is generated by a strict JSON writer.

### Supported JSON5 Input

- `//` line comments and `/* ... */` block comments;
- double-quoted and single-quoted strings, plus the project's backtick-delimited raw string extension;
- double-quoted keys, single-quoted keys, or unquoted keys that begin with an ASCII letter, `_`, or `$` and may be followed by ASCII letters, digits, underscores, or dollar signs;
- trailing commas in arrays and objects;
- JSON5 number forms: hexadecimal, leading `+`, leading decimal point, trailing decimal point, `Infinity`, and `NaN`;
- LF, CR, or CRLF backslash continuations in double-quoted or single-quoted strings;
- space, tab, LF, CR, and form feed whitespace.

### Explicitly Rejected Input

- unquoted keys that begin with a digit, contain a hyphen, or contain non-ASCII letters, such as `9abc`, `a-b`, and `é`;
- backtick-delimited object keys;
- decimal numbers with leading zeros, such as `01`;
- incomplete numbers, invalid identifier suffixes, invalid escapes, and unterminated strings or comments;
- unescaped newlines, control characters, or invalid UTF-8 in single-quoted or double-quoted strings;
- extra content after the root value.

### Strict JSON Output and Number Conversion

Both object keys and string values in the output use standard JSON double quotes and escapes. Member order and duplicate keys are preserved.

Numbers are not converted through `float64`, so floating-point precision is not lost:

| JSON5 Input | JSON Output | Rule |
| --- | --- | --- |
| `0xFF`, `+0Xff` | `255` | Hexadecimal values are converted using arbitrary-precision integer arithmetic. |
| `-0xFF` | `-255` | The minus sign is preserved. |
| `+12` | `12` | The leading `+` is removed. |
| `.5`, `+.5`, `-.5` | `0.5`, `0.5`, `-0.5` | The integer part is added. |
| `1.`, `1.e2` | `1.0`, `1.0e2` | The fractional part is added. |
| `1.2300`, `1E+6` | Unchanged | Original text is preserved when it is already a valid JSON number. |
| `Infinity`, `+Infinity`, `-Infinity`, `NaN` | Conversion fails | Non-finite numbers are not valid JSON. |

Decimal number and exponent text is not evaluated. Arbitrarily long hexadecimal integers are converted using arbitrary-precision integer arithmetic. `-0x0` is emitted as `-0`.

### Complete Comment and Number Example

`input.json5`:

```javascript
{
  // 服务端口
  port: 0x2a,
  /* 比例 */ ratio: .5,
  trailing: 1.,
  plus: +12,
  huge: 0x123456789abcdef0123456789abcdef,
  raw: `line\nliteral`,
}
```

Run:

```bash
go run ./cmd/json-convert --out json --indent 2 input.json5 output.json
```

`output.json`:

```json
{
  "port_hint": "服务端口",
  "port": 42,
  "ratio_hint": "比例",
  "ratio": 0.5,
  "trailing": 1.0,
  "plus": 12,
  "huge": 1512366075204170929049582354406559215,
  "raw": "line\\nliteral"
}
```

Here, `\n` in the input `raw` value consists of two ordinary bytes—a backslash and the letter `n`—so the strict JSON output must write the backslash as `\\`.

## JSON5 → Go Struct Definitions

Use `--out golang` to parse the input as the supported JSON5 subset and generate a `gofmt`-formatted Go source file. `--indent` has no effect in this mode. With two file paths, the second path is the explicit output; with only `INPUT`, the output is written beside the input as `<input-name>_convert.go`.

The package name comes from the output directory name, converted to a valid lower camel-case identifier. The root type name comes from the **input filename**, without its final extension, converted to an exported Go identifier. Therefore, `example/config.json5` → `example/generated.go` produces `package example` and `type Config`; changing only the output filename does not change `Config`. If an identifier has no usable letters or digits, the fallbacks are `main` for the package and `Root` for a type. A leading digit is prefixed with `x` or `X`.

### Structure, Names, Types, and Comments

- A root object becomes a named struct. Nested objects become additional named structs, and arrays become slices. A root array or scalar becomes a named type whose underlying type is the inferred slice or scalar type.
- Object keys are split at punctuation and converted to exported field names. The exact original key is always retained in a `json:"..."` tag. Field-name collisions receive numeric suffixes such as `DisplayName2`; colliding generated type names are resolved the same way.
- Booleans, strings, integers, and non-integer numbers map to `bool`, `string`, `int64`, and `float64`. Hexadecimal integer tokens are also inferred as `int64`. `null`, empty-array elements, incompatible mixed values, and otherwise unknown values map to `any`.
- Array element schemas are merged across all elements. Integer and floating-point values merge to `float64`; object elements merge their fields recursively; incompatible kinds merge to `any`. Missing object fields are still emitted as ordinary fields—generation does not add pointers or `omitempty`.
- Leading and same-line trailing comments associated with an object member are cleaned and emitted as `//` comments above the Go field. This mode does **not** create `_hint` fields. Existing keys ending in `_hint` are ordinary fields and are treated like any other key.
- Duplicate object keys are merged into one field schema. An empty object produces an empty named struct, and an empty array produces `[]any`. A non-object root (array, string, number, boolean, or `null`) produces a named type declaration rather than a struct root.

### Complete Example

`example/config.json5`:

```javascript
{
  // 服务配置
  service: {
    // 监听地址
    host_name: 'localhost',
    ports: [80, 443],
  },
  users: [
    {user_id: 1, enabled: true, score: 1},
    {user_id: 2, enabled: false, score: 2.5},
  ],
  'display-name': 'demo',
  display_name: 'fallback',
  optional: null,
  empty: {},
  flags: [],
}
```

Run with an explicit output path:

```bash
go run ./cmd/json-convert --out golang example/config.json5 example/generated.go
```

The generated `example/generated.go` is:

```go
// Code generated by json-convert. DO NOT EDIT.

package example

type Config struct {
	// 服务配置
	Service      ConfigService     `json:"service"`
	Users        []ConfigUsersItem `json:"users"`
	DisplayName  string            `json:"display-name"`
	DisplayName2 string            `json:"display_name"`
	Optional     any               `json:"optional"`
	Empty        ConfigEmpty       `json:"empty"`
	Flags        []any             `json:"flags"`
}

type ConfigService struct {
	// 监听地址
	HostName string  `json:"host_name"`
	Ports    []int64 `json:"ports"`
}

type ConfigUsersItem struct {
	UserId  int64   `json:"user_id"`
	Enabled bool    `json:"enabled"`
	Score   float64 `json:"score"`
}

type ConfigEmpty struct {
}
```

Omit the second path to generate `example/config_convert.go` instead:

```bash
go run ./cmd/json-convert --out golang example/config.json5
```

The automatic and explicit commands generate identical Go source for this example because package and root type inference use the output directory and input filename, respectively.

## Converting Comments to `_hint`

Comments are processed only when converting JSON5 to JSON. The rules are:

1. Hints are generated only for **object members**. Associated comments are combined into a string, and a `<original-key>_hint` member is inserted before the original member.
2. When multiple comments are associated with the same member, they are cleaned in order of appearance and joined with LF.
3. A comment on the same line after a member value belongs to that member. A comment after the comma on the same line also belongs to the preceding member.
4. A comment on a new line before the next member belongs to the next member.
5. A comment on the same line after the closing delimiter of a multiline object or array value belongs to that object member.
6. Top-level comments, array-element comments, trailing document comments, and object-tail comments on a new line after the object's last member are removed without generating hints.
7. An empty comment that remains empty after cleaning does not generate a hint.
8. Existing `_hint` fields are neither merged nor overwritten. A comment belonging to `name_hint` generates `name_hint_hint`.
9. Duplicate keys are processed separately and may produce duplicate `_hint` keys; order remains unchanged.
10. Processing recurses into nested objects, including objects inside arrays.

Comment cleaning:

- removes the `//`, `/*`, and `*/` markers and trims surrounding whitespace;
- normalizes CRLF and CR to LF;
- removes common indentation from block comments;
- removes the customary leading `*` and one following space from each line;
- preserves blank lines and relative indentation within comments.

Complete example:

```javascript
{
  // 第一条
  /*
   * 第二条
   */
  name: 'demo', // 行尾
  name_hint: '已有值',
  // 重复成员
  name: 'again',
  nested: {/* 子项 */ enabled: true},
  items: [
    {/* 端口 */ port: 8080},
    // 数组元素注释会删除
    2,
  ],
  /* 描述提示 */ tagged_hint: 'old',
}
```

Conversion result:

```json
{
  "name_hint": "第一条\n第二条\n行尾",
  "name": "demo",
  "name_hint": "已有值",
  "name_hint": "重复成员",
  "name": "again",
  "nested": {
    "enabled_hint": "子项",
    "enabled": true
  },
  "items": [
    {
      "port_hint": "端口",
      "port": 8080
    },
    2
  ],
  "tagged_hint_hint": "描述提示",
  "tagged_hint": "old"
}
```

## Backtick Raw String Notes

Backtick-delimited raw strings are a project extension with deliberately simple rules:

- They can be used only as values, not as object keys.
- Content is read from the opening backtick through the **first** subsequent backtick; a backtick cannot be represented in the content.
- There are no escape semantics. `\n`, `A`, and `\\` are all ordinary byte sequences.
- Actual LF, CR, CRLF, NUL and other control bytes, and even invalid UTF-8 may appear in raw content and are preserved byte for byte.
- LF, CR, and CRLF affect line and column positions in subsequent parse errors. CRLF counts as one newline, and a lone CR also counts as a newline.
- When converting to strict JSON, raw content must be valid UTF-8 or conversion fails. Control characters and backslashes are escaped according to JSON rules.

Raw strings are therefore suitable for verbatim multiline text or bytes, but not for data containing backticks. When JSON → JSON5 conversion encounters content with a backtick, it automatically falls back to a double-quoted string.

## Order, Duplicate Keys, and Downstream Risks

This tool preserves object member order and duplicate keys. For example, `{"x":1,"x":2}` still contains two `x` members after conversion. This is important for lossless text conversion, but many downstream libraries decode objects into a `map`:

- member order is usually lost;
- only the last duplicate key is usually retained, or duplicates may be rejected;
- automatically generated duplicate `_hint` keys face the same issue.

When this information must be preserved, use a parser that supports ordered members and duplicate keys instead of decoding directly into an ordinary `map`.

## File Safety

- `INPUT` and `OUTPUT` cannot refer to the same path. File identity checks also reject hard links and symbolic links that point to the same file.
- An existing `OUTPUT` must be a regular file. Directories, symbolic links, and other non-regular files are rejected without modifying their targets.
- The parent directory for a new output must already exist.
- The tool first creates a `.json-convert-*` temporary file in the output directory, writes it completely, syncs it to disk, and then atomically renames it, preventing exposure of a partially written output file.
- If conversion fails during reading, parsing, or generation, an existing output is not modified.
- On platforms that cannot directly replace an existing file, the tool creates a `.json-convert-backup-*` backup in the same output directory before replacing the output. If replacement fails, it attempts to restore the old file.
- If replacement and restoration both fail, the error reports the retained backup path. Restore manually from that `.json-convert-backup-*` file; do not delete it before confirming the data is safe.

## Common Errors

| Symptom or Error | Cause | Resolution |
| --- | --- | --- |
| `--out must be json, json5, or golang` | `--out` was omitted or has an unsupported value. | Specify `--out json`, `--out json5`, or `--out golang`. |
| `--indent must be an integer from 0 to 8` | Indentation is outside the allowed range. | Use `0` through `8`. Non-integers are reported directly by the option parser. |
| `expected one or two file paths` | No input path was supplied, or more than two positional arguments were supplied. | Pass one path for automatic output naming or two paths for an explicit output. |
| `input and output are the same file` | The paths are identical or refer to the same file through a hard link or symbolic link. | Use a different output file. |
| `line N, column M: ...` | The input has a syntax error. | Inspect the input at the reported line and column; note that CR/LF in raw strings affects line numbers. |
| `unterminated raw string` | A raw string has no closing backtick. | Add a backtick at the end of the raw content; the content itself cannot contain backticks. |
| `parse input ...` (`--out json5`) | Input to `--out json5` must be strict JSON, but it contains JSON5 syntax such as comments, single-quoted strings, unquoted keys, or trailing commas. | Convert the input to strict JSON. If the input is JSON5 and should become JSON, use `--out json` instead. |
| `non-finite number ... is not valid JSON` | An attempt was made to write `Infinity` or `NaN` as strict JSON. | Replace it with a finite number or preserve it as a string. |
| `invalid UTF-8 in string` | A raw string contains invalid UTF-8 and cannot be written as strict JSON. | Convert the content to valid UTF-8 first. |
| `refuse to replace non-regular output` | The output is a directory, symbolic link, or other special file. | Use a nonexistent path or a regular file. |
| `read input` / `write output` | The input is unreadable, or the output directory does not exist or is not writable. | Check the paths, parent directory, and file permissions. |
| An option placed after `INPUT` has no effect | Go `flag` stops parsing at the first positional argument. | Move all options before `INPUT OUTPUT`. |

## PowerShell and Git Bash Examples

### PowerShell

```powershell
# JSON → JSON5；路径含空格时使用引号
./json-convert.exe --out json5 --indent 2 ".\data\input file.json" ".\data\output file.json5"

# JSON5 → 紧凑 JSON
./json-convert.exe --out json --indent 0 .\data\input.json5 .\data\output.json

# 获取原生可执行文件的退出码
./json-convert.exe --out json .\input.json5 .\output.json
$LASTEXITCODE
```

### Git Bash

```bash
# JSON → JSON5
./json-convert.exe --out json5 --indent 2 './data/input file.json' './data/output file.json5'

# JSON5 → 紧凑 JSON
./json-convert.exe --out json --indent 0 ./data/input.json5 ./data/output.json

# 获取退出码
./json-convert.exe --out json ./input.json5 ./output.json
printf 'exit=%s\n' "$?"
```

## Capability Summary

| Capability | Status |
| --- | --- |
| File-to-file conversion | Supported. |
| Generating Go struct definitions from JSON5 | Supported with `--out golang`; package and root type names are inferred from the output directory and input filename. |
| Reading from `stdin` or writing data to `stdout` | Not supported; `-` is treated only as an ordinary filename. |
| In-place input replacement | Not supported, including hard links and symbolic links to the same file. |
| Preserving object member order and duplicate keys | Supported. |
| Preserving original number text | Supported for JSON → JSON5; for JSON5 → JSON, valid JSON number text is preserved and other forms are normalized according to the documented rules. |
| Preserving original indentation, spaces, newline style, and quote style | Not supported; output is reformatted according to `--indent`. |
| Preserving comments verbatim | Not supported; JSON5 → JSON converts only comments associated with object members into `_hint` fields and removes the rest. |
| Full standard JSON5 syntax compatibility | Not supported; unquoted keys are limited to an ASCII subset, while backtick strings are a nonstandard extension. |
| Writing `Infinity` / `NaN` to strict JSON | Not supported. |
| Escaping backticks in raw strings | Not supported; the first backtick always terminates the string. |
| Creating output directories automatically | Not supported. |
| JSON Schema validation, key deduplication, or semantic merging | Not supported. |
