# JSON 与扩展 JSON5 双向转换命令实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 新增 `json-convert` 文件转换命令，在标准 JSON 和本项目反引号 JSON5 扩展之间双向转换，同时保留对象顺序、重复键及数字精度，并把对象成员注释转换为 `_hint` 字段。

**架构：** 在 `cmd/json-convert` 内实现独立的字节级递归下降解析器，以有序语法树保存值、成员、数字原文、注释和位置；writer 分别输出扩展 JSON5 与标准 JSON。CLI 负责参数校验、禁止原地转换以及临时文件安全替换，不修改核心 JSON5 decoder。

**技术栈：** Go 1.19、标准库 `flag`/`os`/`path/filepath`/`math/big`/`unicode/utf8`、递归下降解析、Go `testing`。

---

## 文件结构

- 创建：`cmd/json-convert/main.go` — 参数解析、路径校验、转换协调、临时文件安全替换。
- 创建：`cmd/json-convert/parser.go` — token 读取、有序语法树、严格 JSON 与扩展 JSON5 递归下降解析。
- 创建：`cmd/json-convert/comments.go` — 注释正文清理、对象成员注释归属和 `_hint` 成员生成。
- 创建：`cmd/json-convert/writer.go` — JSON/JSON5 字符串、数字、结构和缩进输出。
- 创建：`cmd/json-convert/parser_test.go` — 解析模式、顺序、重复键、字符串、数字、位置及非法输入测试。
- 创建：`cmd/json-convert/comments_test.go` — 前置/行尾注释清理和归属测试。
- 创建：`cmd/json-convert/writer_test.go` — 两种输出格式、数字规范化、UTF-8 和缩进测试。
- 创建：`cmd/json-convert/main_test.go` — CLI 参数、路径冲突、安全覆盖和端到端文件转换测试。
- 修改：`README.md` — 增加转换命令的简明用法。

## 共享内部模型

任务 1 建立后，后续任务统一使用以下内部类型，不另起同义类型：

```go
type parseMode int

const (
	modeJSON parseMode = iota
	modeJSON5
)

type valueKind int

const (
	kindNull valueKind = iota
	kindBool
	kindNumber
	kindString
	kindArray
	kindObject
)

type position struct {
	offset int
	line   int
	column int
}

type comment struct {
	raw       []byte
	start     position
	end       position
	startLine int
	endLine   int
}

type member struct {
	key              string
	value            value
	keyPos           position
	endLine          int
	leadingComments  []comment
	trailingComments []comment
}

type value struct {
	kind   valueKind
	text   []byte
	pos    position
	end    position
	array  []value
	object []member
}
```

`value.text` 的约定：字符串保存解码后的原始字节；数字保存源码 token；布尔值保存 `true`/`false`；null 不需要内容。

---

### 任务 1：建立有序语法树与严格 JSON 解析器

**文件：**
- 创建：`cmd/json-convert/parser.go`
- 创建：`cmd/json-convert/parser_test.go`

- [ ] **步骤 1：编写严格 JSON 解析失败测试**

创建 `parser_test.go`，覆盖成员顺序、重复键、数字原文、标准字符串转义和严格拒绝扩展语法：

```go
package main

import (
	"bytes"
	"testing"
)

func TestParseJSONPreservesOrderDuplicatesAndNumbers(t *testing.T) {
	got, err := parseDocument([]byte(`{"z":1.2300,"a":"line\n\"q\"","z":9007199254740993123456789}`), modeJSON)
	if err != nil {
		t.Fatal(err)
	}
	if got.kind != kindObject || len(got.object) != 3 {
		t.Fatalf("object = %#v", got)
	}
	if got.object[0].key != "z" || string(got.object[0].value.text) != "1.2300" ||
		got.object[1].key != "a" || !bytes.Equal(got.object[1].value.text, []byte("line\n\"q\"")) ||
		got.object[2].key != "z" || string(got.object[2].value.text) != "9007199254740993123456789" {
		t.Fatalf("members = %#v", got.object)
	}
}

func TestParseJSONRejectsExtensions(t *testing.T) {
	for _, input := range []string{
		`{key:1}`,
		`{"key":'value'}`,
		"{\"key\":`value`}",
		`{"key":1,}`,
		`{"key":+1}`,
		`{"key":.5}`,
		"{/* comment */\"key\":1}",
	} {
		if _, err := parseDocument([]byte(input), modeJSON); err == nil {
			t.Errorf("parseDocument(%q) unexpectedly succeeded", input)
		}
	}
}
```

- [ ] **步骤 2：运行测试确认红灯**

运行：

```bash
go test ./cmd/json-convert -run '^TestParseJSON' -count=1
```

预期：FAIL，`parseDocument`、`modeJSON` 或内部类型尚未定义。

- [ ] **步骤 3：实现 parser 基础状态与错误位置**

在 `parser.go` 定义共享类型，并实现：

```go
type parser struct {
	data []byte
	mode parseMode
	off  int
	line int
	col  int
}

type parseError struct {
	pos position
	msg string
}

func (e *parseError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.pos.line, e.pos.column, e.msg)
}

func newParser(data []byte, mode parseMode) *parser {
	return &parser{data: data, mode: mode, line: 1, col: 1}
}

func (p *parser) position() position {
	return position{offset: p.off, line: p.line, column: p.col}
}

func (p *parser) next() byte {
	b := p.data[p.off]
	p.off++
	if b == '\n' {
		p.line++
		p.col = 1
	} else {
		p.col++
	}
	return b
}

func (p *parser) fail(pos position, format string, args ...interface{}) error {
	return &parseError{pos: pos, msg: fmt.Sprintf(format, args...)}
}
```

- [ ] **步骤 4：实现严格 JSON 值、对象、数组、字符串和数字解析**

实现入口及递归函数：

```go
func parseDocument(data []byte, mode parseMode) (value, error) {
	p := newParser(data, mode)
	p.skipWhitespace()
	v, err := p.parseValue(nil)
	if err != nil {
		return value{}, err
	}
	p.skipWhitespace()
	if p.off != len(p.data) {
		return value{}, p.fail(p.position(), "unexpected character %q after top-level value", p.data[p.off])
	}
	return v, nil
}
```

`parseValue` 必须按首字节分派 `{`、`[`、`"`、数字、`true`、`false`、`null`。`parseObject` 使用 `[]member` 追加成员；`parseArray` 使用 `[]value`；禁止尾随逗号。`parseJSONString` 解析标准转义和 UTF-16 surrogate pair，非法控制字符报错。`parseJSONNumber` 按标准 JSON 文法读取但不转换数值，直接保存 token。

- [ ] **步骤 5：运行严格 JSON 测试并补位置断言**

追加：

```go
func TestParseErrorReportsLineAndColumn(t *testing.T) {
	_, err := parseDocument([]byte("{\n  \"a\": 1,\n  \"b\": @\n}"), modeJSON)
	if err == nil || !strings.Contains(err.Error(), "line 3, column 8") {
		t.Fatalf("error = %v", err)
	}
}
```

运行：

```bash
gofmt -w cmd/json-convert/parser.go cmd/json-convert/parser_test.go
go test ./cmd/json-convert -run '^(TestParseJSON|TestParseError)' -count=1
```

预期：PASS。

- [ ] **步骤 6：提交严格 JSON parser**

```bash
git add cmd/json-convert/parser.go cmd/json-convert/parser_test.go
git commit -m "feat(转换器): 添加有序 JSON 解析器" -m "保留对象成员顺序、重复键和数字原文，并提供严格 JSON 语法及位置错误检查。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### 任务 2：扩展 parser 支持 JSON5 字符串、键、注释和数字

**文件：**
- 修改：`cmd/json-convert/parser.go`
- 修改：`cmd/json-convert/parser_test.go`

- [ ] **步骤 1：编写 JSON5 解析失败测试**

追加测试：

```go
func TestParseJSON5Extensions(t *testing.T) {
	input := []byte("// top\n{\n  unquoted: `line 1\r\nline 2`,\n  'single-key': 'single\\nvalue',\n  hex: -0xFF, plus: +12, leading: .5, trailing: 1.,\n}")
	got, err := parseDocument(input, modeJSON5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.object) != 6 {
		t.Fatalf("members = %d", len(got.object))
	}
	if got.object[0].key != "unquoted" || !bytes.Equal(got.object[0].value.text, []byte("line 1\r\nline 2")) {
		t.Fatalf("raw value = %q", got.object[0].value.text)
	}
	wantNumbers := []string{"-0xFF", "+12", ".5", "1."}
	for i, want := range wantNumbers {
		if got.object[i+2].value.kind != kindNumber || string(got.object[i+2].value.text) != want {
			t.Fatalf("number %d = %q", i, got.object[i+2].value.text)
		}
	}
}

func TestParseJSON5RejectsBacktickKeyAndUnterminatedRawString(t *testing.T) {
	for _, tt := range []struct {
		input string
		want  string
	}{
		{"{`key`: 1}", "object key"},
		{"{value: `line 1\nline 2}", "unterminated raw string"},
	} {
		_, err := parseDocument([]byte(tt.input), modeJSON5)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("error = %v, want containing %q", err, tt.want)
		}
	}
}
```

- [ ] **步骤 2：运行测试确认红灯**

```bash
go test ./cmd/json-convert -run '^TestParseJSON5' -count=1
```

预期：FAIL，parser 尚不接受 JSON5 扩展。

- [ ] **步骤 3：实现 JSON5 字符串与键**

在 `modeJSON5` 下：

- `parseValue` 接受 `'` 和反引号；
- `parseObject` 接受单/双引号键和由 `[A-Za-z_$][A-Za-z0-9_$]*` 构成的未引号键；
- 反引号键明确报 object key 错误；
- `parseRawString` 从起始反引号后逐字节读到第一个反引号，直接复制中间字节；
- 单引号使用与双引号相同的 JSON5 转义集合，并允许反斜杠续行。

核心 raw 函数：

```go
func (p *parser) parseRawString() (value, error) {
	start := p.position()
	p.next()
	contentStart := p.off
	for p.off < len(p.data) {
		if p.data[p.off] == '`' {
			text := append([]byte(nil), p.data[contentStart:p.off]...)
			p.next()
			return value{kind: kindString, text: text, pos: start, end: p.position()}, nil
		}
		p.next()
	}
	return value{}, p.fail(start, "unterminated raw string")
}
```

- [ ] **步骤 4：实现 JSON5 数字与特殊值 token**

在 `modeJSON5` 下读取：

- 可选 `+`/`-`；
- `0x`/`0X` 十六进制；
- `.5`、`1.`、指数；
- `Infinity`、`NaN`。

所有 token 仍保存原文，不经过 `float64`。`Infinity`/`NaN` 使用 `kindNumber`，writer 在 JSON 模式统一拒绝。

- [ ] **步骤 5：捕获注释及位置而不丢失对象上下文**

把 JSON5 空白扫描改为返回注释：

```go
func (p *parser) skipSpaceAndComments() ([]comment, error)
```

要求：

- JSON 模式只跳过 JSON 空白，遇 `/` 不吞掉；
- JSON5 模式捕获 `//` 到换行前和 `/* ... */`；
- comment 保存原始字节和起止行；
- 顶层与数组调用者可丢弃注释；
- `parseObject` 暂时把成员前 token 之间的注释放到 `leadingComments`；
- 对值结束后、逗号前后收集到的注释保存在候选列表，并依据行号分到当前 `trailingComments` 或下一成员 `leadingComments`；完整归属在任务 4 固化。

- [ ] **步骤 6：运行 parser 测试与全库回归**

```bash
gofmt -w cmd/json-convert/parser.go cmd/json-convert/parser_test.go
go test ./cmd/json-convert -run '^TestParse' -count=1
go test ./...
```

预期：PASS。

- [ ] **步骤 7：提交 JSON5 parser 扩展**

```bash
git add cmd/json-convert/parser.go cmd/json-convert/parser_test.go
git commit -m "feat(转换器): 解析扩展 JSON5 语法" -m "支持单引号、反引号、未引号键、注释、尾随逗号和 JSON5 数字，同时保留原始位置。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### 任务 3：实现 JSON5 writer 与字符串反引号策略

**文件：**
- 创建：`cmd/json-convert/writer.go`
- 创建：`cmd/json-convert/writer_test.go`

- [ ] **步骤 1：编写 JSON5 writer 失败测试**

```go
package main

import (
	"bytes"
	"testing"
)

func TestWriteJSON5UsesBackticksAndPreservesStructure(t *testing.T) {
	input := []byte(`{"z":"echo \"hello\"\necho 'world'","a":"echo ` + "`" + `date` + "`" + `","z":1.2300}`)
	v, err := parseDocument(input, modeJSON)
	if err != nil {
		t.Fatal(err)
	}
	got, err := writeDocument(v, outputJSON5, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("{\n  \"z\": `echo \\\"hello\\\"\necho 'world'`,\n  \"a\": \"echo `date`\",\n  \"z\": 1.2300\n}\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteJSON5Compact(t *testing.T) {
	v, err := parseDocument([]byte(`{"a":["x","y"],"empty":{}}`), modeJSON)
	if err != nil {
		t.Fatal(err)
	}
	got, err := writeDocument(v, outputJSON5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\"a\":[`x`,`y`],\"empty\":{}}\n" {
		t.Fatalf("got = %q", got)
	}
}
```

- [ ] **步骤 2：运行 writer 测试确认红灯**

```bash
go test ./cmd/json-convert -run '^TestWriteJSON5' -count=1
```

预期：FAIL，writer 类型与函数尚未定义。

- [ ] **步骤 3：实现 writer 框架与缩进**

定义：

```go
type outputMode int

const (
	outputJSON outputMode = iota
	outputJSON5
)

type writer struct {
	buf    bytes.Buffer
	mode   outputMode
	indent int
	depth  int
}

func writeDocument(v value, mode outputMode, indent int) ([]byte, error) {
	w := &writer{mode: mode, indent: indent}
	if err := w.writeValue(v); err != nil {
		return nil, err
	}
	w.buf.WriteByte('\n')
	return w.buf.Bytes(), nil
}
```

实现对象、数组、布尔、null、数字递归输出。对象遍历 `[]member`，键始终调用标准 JSON string quoting。`indent == 0` 不写结构空白；正数写换行和 `depth*indent` 空格。

- [ ] **步骤 4：实现 JSON5 字符串策略**

```go
func (w *writer) writeJSON5String(text []byte) {
	if !bytes.ContainsRune(text, '`') {
		w.buf.WriteByte('`')
		w.buf.Write(text)
		w.buf.WriteByte('`')
		return
	}
	writeJSONString(&w.buf, text)
}
```

`writeJSONString` 必须标准转义 `"`、`\\`、控制字符、换行和回车；有效非 ASCII UTF-8 可直接写出。对象键永远使用 `writeJSONString`。

- [ ] **步骤 5：运行 writer 测试并验证 JSON5 可回读**

追加 round-trip：用项目包 `github.com/titanous/json5` 的 `Unmarshal` 解码 writer 输出，确认字符串语义正确。运行：

```bash
gofmt -w cmd/json-convert/writer.go cmd/json-convert/writer_test.go
go test ./cmd/json-convert -run '^TestWriteJSON5' -count=1
```

预期：PASS。

- [ ] **步骤 6：提交 JSON5 writer**

```bash
git add cmd/json-convert/writer.go cmd/json-convert/writer_test.go
git commit -m "feat(转换器): 输出反引号 JSON5" -m "按字符串内容选择反引号或标准双引号，并保留成员顺序、重复键、数字原文和可配置缩进。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### 任务 4：实现注释清理、归属与 `_hint` 成员

**文件：**
- 创建：`cmd/json-convert/comments.go`
- 创建：`cmd/json-convert/comments_test.go`
- 修改：`cmd/json-convert/parser.go`

- [ ] **步骤 1：编写注释清理失败测试**

```go
package main

import "testing"

func TestCleanComment(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"// 服务名称", "服务名称"},
		{"/*\n   * 服务名称\n   * 用于页面展示\n   */", "服务名称\n用于页面展示"},
		{"/* one line */", "one line"},
	}
	for _, tt := range tests {
		if got := cleanComment([]byte(tt.raw)); got != tt.want {
			t.Errorf("cleanComment(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}
```

- [ ] **步骤 2：编写注释归属和 hint 失败测试**

```go
func TestCommentHints(t *testing.T) {
	input := []byte(`
// removed top-level
{
  // first

  /* second */
  name: ` + "`demo`" + `, // trailing
  name_hint: ` + "`existing`" + `,
  // hint comment
  name_hint: ` + "`value`" + `,
  port: 8080, /* belongs to port */ host: ` + "`localhost`" + `,
  script: ` + "`line 1\nline 2`" + `, // script end
  items: [// removed array comment
    ` + "`a`" + `,
  ],
}`)
	v, err := parseDocument(input, modeJSON5)
	if err != nil {
		t.Fatal(err)
	}
	got := addHintMembers(v)
	keys := make([]string, len(got.object))
	for i := range got.object {
		keys[i] = got.object[i].key
	}
	wantKeys := []string{"name_hint", "name", "name_hint", "name_hint_hint", "name_hint", "port_hint", "port", "host", "script_hint", "script", "items"}
	if !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("keys = %#v, want %#v", keys, wantKeys)
	}
	if string(got.object[0].value.text) != "first\nsecond\ntrailing" {
		t.Fatalf("name hint = %q", got.object[0].value.text)
	}
	if string(got.object[5].value.text) != "belongs to port" {
		t.Fatalf("port hint = %q", got.object[5].value.text)
	}
}
```

- [ ] **步骤 3：运行注释测试确认红灯**

```bash
go test ./cmd/json-convert -run '^(TestCleanComment|TestCommentHints)$' -count=1
```

预期：FAIL，`cleanComment` 和 `addHintMembers` 尚未定义或归属不完整。

- [ ] **步骤 4：实现注释清理**

在 `comments.go` 实现：

```go
func cleanComment(raw []byte) string
```

算法：去定界符；按 `\r\n`、`\n`、`\r` 统一拆行；移除首尾空白行；计算非空行公共前导空白；去公共缩进；每行去可选 `*` 和其后一个空格；最终只去正文整体首尾空白，保留内部换行。

- [ ] **步骤 5：固化注释归属规则**

调整 `parseObject`：

- 成员前收集的注释先作为 pending；
- 当前成员值结束后，记录 `member.endLine`；
- 逗号前和逗号后收集注释；
- 注释 `startLine == member.endLine` 时归当前成员 trailing；
- 否则保留给下一成员 leading；
- 同一行注释一旦归前一成员，不再进入下一成员；
- 空白行不清空 pending；
- 数组和顶层调用只丢弃注释。

- [ ] **步骤 6：实现 hint 成员生成**

```go
func addHintMembers(v value) value
```

递归处理对象和数组。对象遍历原成员：先递归处理 value；清理并按顺序连接 leading，再连接 trailing；非空时先 append 一个 `key + "_hint"` 的字符串成员，再 append 原成员。不做去重，因此允许重复键和 `_hint_hint`。

- [ ] **步骤 7：补逗号前、逗号后和复合值测试并运行**

分别测试：

```json5
{name: `demo` // before comma
, port: 1}
```

```json5
{name: `demo`, // after comma
port: 1}
```

以及对象、数组、raw string 在结束行后的注释。运行：

```bash
gofmt -w cmd/json-convert/parser.go cmd/json-convert/comments.go cmd/json-convert/comments_test.go
go test ./cmd/json-convert -run 'Comment|Hint' -count=1
go test ./...
```

预期：PASS。

- [ ] **步骤 8：提交注释转换**

```bash
git add cmd/json-convert/parser.go cmd/json-convert/comments.go cmd/json-convert/comments_test.go
git commit -m "feat(转换器): 将成员注释转换为提示字段" -m "清理前置与行尾注释并按源码位置归属，生成保序且允许重复的 _hint 成员。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### 任务 5：实现标准 JSON writer、数字规范化和 UTF-8 校验

**文件：**
- 修改：`cmd/json-convert/writer.go`
- 修改：`cmd/json-convert/writer_test.go`

- [ ] **步骤 1：编写标准 JSON 输出失败测试**

```go
func TestWriteJSONNormalizesJSON5Numbers(t *testing.T) {
	v, err := parseDocument([]byte(`{hex:0xFF,negative:-0xFF,plus:+12,leading:.5,negativeLeading:-.5,trailing:1.,plusTrailing:+1.}`), modeJSON5)
	if err != nil {
		t.Fatal(err)
	}
	got, err := writeDocument(addHintMembers(v), outputJSON, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"hex\":255,\"negative\":-255,\"plus\":12,\"leading\":0.5,\"negativeLeading\":-0.5,\"trailing\":1.0,\"plusTrailing\":1.0}\n"
	if string(got) != want {
		t.Fatalf("got = %q, want %q", got, want)
	}
}

func TestWriteJSONRejectsNonFiniteAndInvalidUTF8(t *testing.T) {
	for _, input := range []string{`{v:Infinity}`, `{v:-Infinity}`, `{v:NaN}`} {
		v, err := parseDocument([]byte(input), modeJSON5)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writeDocument(v, outputJSON, 2); err == nil {
			t.Errorf("writeDocument(%s) unexpectedly succeeded", input)
		}
	}
	v := value{kind: kindString, text: []byte{0xff}, pos: position{line: 3, column: 4}}
	if _, err := writeDocument(v, outputJSON, 2); err == nil || !strings.Contains(err.Error(), "line 3, column 4") {
		t.Fatalf("error = %v", err)
	}
}
```

- [ ] **步骤 2：运行 JSON writer 测试确认红灯**

```bash
go test ./cmd/json-convert -run '^TestWriteJSON' -count=1
```

预期：数字规范化或非有限值/UTF-8 断言失败。

- [ ] **步骤 3：实现任意精度十六进制转换**

实现：

```go
func normalizeNumber(text []byte) ([]byte, error)
```

- 分离符号；
- `0x`/`0X` 使用 `big.Int.SetString(hex, 16)`；
- 恢复负号；
- 前导 `+` 删除；
- `.5`、`-.5` 补 `0`；
- `1.` 补 `0`；
- `Infinity`、`-Infinity`、`+Infinity`、`NaN` 返回带 value position 的转换错误；
- 已符合 JSON 数字语法的文本原样返回。

- [ ] **步骤 4：实现标准 JSON 字符串和 UTF-8 校验**

在 outputJSON 模式写字符串前调用 `utf8.Valid(text)`；无效时返回包含 `value.pos` 的错误。有效字符串使用标准 JSON 转义。hint 值同样是普通字符串节点，因此走相同验证路径。

- [ ] **步骤 5：验证输出可被 `encoding/json` 接受**

追加测试：writer 输出后用 `encoding/json.Valid(got[:len(got)-1])` 验证；同时比较重复键输出文本，不能解码到 map 后比较。

运行：

```bash
gofmt -w cmd/json-convert/writer.go cmd/json-convert/writer_test.go
go test ./cmd/json-convert -run '^TestWriteJSON' -count=1
go test ./...
```

预期：PASS。

- [ ] **步骤 6：提交标准 JSON writer**

```bash
git add cmd/json-convert/writer.go cmd/json-convert/writer_test.go
git commit -m "feat(转换器): 输出标准 JSON" -m "规范化 JSON5 数字，拒绝非有限数值与无效 UTF-8，并生成严格合法的标准 JSON。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### 任务 6：实现 CLI 参数与安全文件替换

**文件：**
- 创建：`cmd/json-convert/main.go`
- 创建：`cmd/json-convert/main_test.go`

- [ ] **步骤 1：编写参数解析失败测试**

让 `main` 只负责 `os.Exit(run(...))`，测试 `run(args, stderr)`：

```go
func TestParseOptions(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantOut outputMode
		wantInd int
		wantErr bool
	}{
		{"json5 default indent", []string{"--out", "json5", "in.json", "out.json5"}, outputJSON5, 2, false},
		{"json compact", []string{"--out", "json", "--indent", "0", "in.json5", "out.json"}, outputJSON, 0, false},
		{"missing out", []string{"in.json", "out.json5"}, 0, 0, true},
		{"invalid out", []string{"--out", "yaml", "in", "out"}, 0, 0, true},
		{"negative indent", []string{"--out", "json", "--indent", "-1", "in", "out"}, 0, 0, true},
		{"large indent", []string{"--out", "json", "--indent", "9", "in", "out"}, 0, 0, true},
		{"extra argument", []string{"--out", "json", "in", "out", "extra"}, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := parseOptions(tt.args, io.Discard)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v", err)
			}
			if err == nil && (opts.out != tt.wantOut || opts.indent != tt.wantInd) {
				t.Fatalf("options = %#v", opts)
			}
		})
	}
}
```

- [ ] **步骤 2：编写文件安全与端到端失败测试**

使用 `t.TempDir()`：

- JSON 输入转 JSON5，确认字符串反引号和末尾换行；
- JSON5 输入转 JSON，确认 hint、数字和严格 JSON；
- 输入输出同路径时报错；
- 使用 hard link 指向同一文件时报错（Windows 不支持时按创建 hard link 的实际错误 `t.Skip`）；
- 已有输出内容为 `keep`，输入解析失败后仍为 `keep`；
- 成功时覆盖已有输出。

核心断言示例：

```go
func TestConvertFileFailureKeepsExistingOutput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "bad.json")
	out := filepath.Join(dir, "out.json5")
	os.WriteFile(in, []byte(`{"broken":}`), 0o600)
	os.WriteFile(out, []byte("keep"), 0o600)
	if err := convertFile(options{out: outputJSON5, indent: 2, input: in, output: out}); err == nil {
		t.Fatal("convertFile unexpectedly succeeded")
	}
	got, _ := os.ReadFile(out)
	if string(got) != "keep" {
		t.Fatalf("output = %q", got)
	}
}
```

- [ ] **步骤 3：运行 CLI 测试确认红灯**

```bash
go test ./cmd/json-convert -run '^(TestParseOptions|TestConvertFile)' -count=1
```

预期：FAIL，CLI 函数尚未定义。

- [ ] **步骤 4：实现参数解析和转换协调**

定义：

```go
type options struct {
	out    outputMode
	indent int
	input  string
	output string
}

func parseOptions(args []string, stderr io.Writer) (options, error)
func convertFile(opts options) error
func run(args []string, stderr io.Writer) int
```

`convertFile` 根据 out 选择输入 parseMode；JSON 输出前调用 `addHintMembers`；writer 使用目标 outputMode。

- [ ] **步骤 5：实现同文件检查和安全替换**

```go
func sameFile(input, output string) (bool, error)
func writeFileAtomic(path string, data []byte) error
```

规则：先比较 `filepath.Abs` + `filepath.Clean`（Windows 大小写使用 `strings.EqualFold`）；两者都存在时用 `os.SameFile`；原地则报错。安全写入使用 `os.CreateTemp(filepath.Dir(path), ".json-convert-*")`，写、`Sync`、Close 后替换目标。Windows 上目标存在时，先用同目录备份重命名策略保证替换失败可恢复原文件；所有失败路径清理临时文件和备份。

- [ ] **步骤 6：运行 CLI 和完整测试**

```bash
gofmt -w cmd/json-convert/main.go cmd/json-convert/main_test.go
go test ./cmd/json-convert -run '^(TestParseOptions|TestConvertFile)' -count=1
go test ./...
```

预期：PASS。

- [ ] **步骤 7：命令级手工烟测**

在临时目录创建实际文件并运行：

```bash
go run ./cmd/json-convert --out json5 --indent 2 input.json output.json5
go run ./cmd/json-convert --out json --indent 0 output.json5 roundtrip.json
```

用 Python 或 Go 比较标准 JSON 的语义，确认 roundtrip 字符串和数字含义一致；检查输出尾部恰好一个 `\n`。

- [ ] **步骤 8：提交 CLI**

```bash
git add cmd/json-convert/main.go cmd/json-convert/main_test.go
git commit -m "feat(转换器): 添加双向文件转换命令" -m "提供输出格式和缩进参数，禁止原地转换，并通过临时文件安全替换目标文件。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### 任务 7：补充边界矩阵、README 与完整验证

**文件：**
- 修改：`cmd/json-convert/parser_test.go`
- 修改：`cmd/json-convert/comments_test.go`
- 修改：`cmd/json-convert/writer_test.go`
- 修改：`cmd/json-convert/main_test.go`
- 修改：`README.md`

- [ ] **步骤 1：补齐规格边界测试矩阵**

逐项补齐并确保每项有独立断言：

- JSON：`\u0000`、surrogate pair、非法 surrogate、前导零、尾随内容；
- JSON5：单双引号转义、反斜杠续行、CR/LF/CRLF raw、控制字节、未闭合注释和字符串；
- 数字：大十六进制、指数原文、`+Infinity`、负零；
- 注释：多个前置、空白行、逗号前/后、多行 raw/对象/数组结束行、数组/顶层删除、同行归前一成员、`_hint_hint`、同名重复；
- writer：缩进 `0` 至 `8`、空结构、反引号回退、raw 内容不被缩进改变；
- CLI：所有参数错误、同路径、hard link、覆盖、失败保留、末尾一个换行。

- [ ] **步骤 2：运行聚焦测试并修复仅由新增测试暴露的问题**

```bash
go test ./cmd/json-convert -count=1
go test ./cmd/json-convert -count=10
```

预期：连续 PASS。任何失败按 TDD 最小修复，不重构核心 decoder。

- [ ] **步骤 3：在 README 增加命令用法**

在 Backtick raw strings 章节后增加简洁英文说明：

```markdown
## JSON conversion command

Convert standard JSON to this fork's JSON5 extension:

```sh
go run ./cmd/json-convert --out json5 --indent 2 input.json output.json5
```

Convert JSON5 back to standard JSON:

```sh
go run ./cmd/json-convert --out json --indent 2 input.json5 output.json
```

Use `--indent 0` for compact output. The converter preserves object member
order and duplicate keys. When converting to JSON, comments associated with an
object member are emitted as `<name>_hint` fields.
```

- [ ] **步骤 4：执行最终格式和测试验证**

```bash
gofmt -w cmd/json-convert/*.go
git diff --check
go test ./...
go test ./cmd/json-convert -count=10
git status --short
```

预期：所有命令退出码为 `0`；状态仅包含本任务测试和 README 改动。

- [ ] **步骤 5：审查范围与依赖**

```bash
git diff --stat main...HEAD
git diff main...HEAD -- go.mod go.sum
git status --short --branch
```

预期：`go.mod`、`go.sum` 无变化；核心 `decode.go`、`scanner.go` 无变化；只新增转换命令和相关文档。

- [ ] **步骤 6：提交最终测试与文档**

```bash
git add cmd/json-convert/*_test.go README.md
git commit -m "test(转换器): 完善双向转换边界覆盖" -m "覆盖解析、数字、注释归属、缩进和文件安全边界，并补充命令使用文档。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

- [ ] **步骤 7：最终审查后推送**

在规格合规审查和代码质量审查均通过后运行：

```bash
git push origin <当前功能分支>
```

若按本地合并工作流，则先合并回 `main`，在合并结果上重新运行 `go test ./...`，再推送 `main`。
