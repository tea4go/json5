# 反引号 Raw String 扩展实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为 JSON5 解码器增加仅可作为值使用、完全 raw 且逐字节保留的反引号多行字符串。

**架构：** 在现有字节级 scanner 中增加反引号值入口和独立扫描状态，由第一个反引号结束字面量；decoder 将反引号归类为字符串，并在 `unquoteBytes` 中直接切除首尾定界符。现有单引号、双引号、对象键和错误路径保持不变，不增加公开 API。

**技术栈：** Go 1.19、手写 scanner 状态机、`testing`、现有 `Unmarshal`/`Decoder`/`RawMessage` 解码路径。

---

## 文件结构

- 修改：`scanner.go` — 在值入口识别反引号，新增完全 raw 的字符串扫描状态；对象键入口保持不变。
- 修改：`decode.go` — 将反引号字面量按字符串解码，并为 `unquoteBytes` 增加逐字节返回路径。
- 创建：`backtick_string_test.go` — 集中覆盖语法、raw 字节语义、解码接口、流式解码和错误边界，避免把非标准扩展混入 ES5 对照测试。
- 修改：`README.md` — 说明本 fork 的非标准反引号 raw string 扩展及限制。

## 任务 1：让 Scanner 识别仅作为值的反引号字符串

**文件：**
- 修改：`scanner.go:215-269,450-480`
- 创建：`backtick_string_test.go`

- [ ] **步骤 1：编写 Scanner 失败测试**

创建 `backtick_string_test.go`，先加入合法位置、非法对象键和未闭合字符串测试：

```go
package json5

import "testing"

func TestBacktickStringScanner(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{name: "top level", input: []byte("`value`")},
		{name: "object value", input: []byte("{value:`text`}")},
		{name: "array value", input: []byte("[`text`]")},
		{name: "multiline", input: []byte("`line 1\r\nline 2\nline 3\rline 4`")},
		{name: "control and invalid UTF-8", input: []byte{'`', 0x00, 0x1f, 0xff, '`'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var scan scanner
			if err := checkValid(tt.input, &scan); err != nil {
				t.Fatalf("checkValid(%q): %v", tt.input, err)
			}
		})
	}
}

func TestBacktickStringScannerErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "object key",
			input:   "{`key`:1}",
			wantErr: "invalid character '`' looking for beginning of object key",
		},
		{
			name:    "unterminated",
			input:   "{value:`text}",
			wantErr: "unexpected end of JSON input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var scan scanner
			err := checkValid([]byte(tt.input), &scan)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("checkValid(%q) error = %v, want %q", tt.input, err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **步骤 2：运行 Scanner 测试并确认失败**

运行：

```bash
go test ./... -run '^TestBacktickStringScanner' -count=1
```

预期：`TestBacktickStringScanner` 的合法用例失败，错误包含 `invalid character '`；错误用例中的对象键测试通过，未闭合测试因尚未进入 raw string 状态而失败为其他语法错误。

- [ ] **步骤 3：实现最小 Scanner 支持**

在 `scanner.go` 的 `stateBeginValue` switch 中，仅在值入口加入：

```go
case '`':
	s.step = stateInStringBacktick
	return scanBeginLiteral
```

在 `stateInStringSingle` 后新增：

```go
// stateInStringBacktick is the state after reading "`".
func stateInStringBacktick(s *scanner, c byte) int {
	if c == '`' {
		s.step = stateEndValue
	}
	return scanContinue
}
```

不要修改 `stateBeginObjectKey`，不要在该状态中处理反斜杠、控制字节、换行或 UTF-8。

- [ ] **步骤 4：运行 Scanner 测试并确认通过**

运行：

```bash
gofmt -w scanner.go backtick_string_test.go
go test ./... -run '^TestBacktickStringScanner' -count=1
```

预期：两个测试函数及其全部子测试均 PASS。

- [ ] **步骤 5：运行现有 Scanner 回归测试**

运行：

```bash
go test ./... -run '^(TestBacktickStringScanner|TestJSON5Decode|TestInvalidNewline)$' -count=1
```

预期：PASS，现有普通字符串换行规则不受影响。

- [ ] **步骤 6：提交 Scanner 变更**

```bash
git add scanner.go backtick_string_test.go
git commit -m "feat(解析器): 识别反引号原始字符串值" -m "在值扫描入口增加反引号字面量状态，保留全部内容字节，并继续拒绝反引号对象键。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

## 任务 2：将反引号字面量解码为逐字节保留的字符串

**文件：**
- 修改：`decode.go:818-940,1121-1150,1183-1311`
- 修改：`backtick_string_test.go`

- [ ] **步骤 1：编写基础解码失败测试**

向 `backtick_string_test.go` 追加：

```go
func TestUnmarshalBacktickString(t *testing.T) {
	input := []byte("{value:`\r\n  echo \"hello\"\n  echo 'world'\n  echo \\n\r`,empty:``}")
	want := "\r\n  echo \"hello\"\n  echo 'world'\n  echo \\n\r"

	var got struct {
		Value string
		Empty string
	}
	if err := Unmarshal(input, &got); err != nil {
		t.Fatal(err)
	}
	if got.Value != want {
		t.Fatalf("Value bytes = %q, want %q", []byte(got.Value), []byte(want))
	}
	if got.Empty != "" {
		t.Fatalf("Empty = %q, want empty string", got.Empty)
	}
}

func TestUnmarshalBacktickStringInterface(t *testing.T) {
	input := []byte{'[', '`', 0x00, 0x1f, 0xff, '\\', 'n', '`', ']'}

	var got []interface{}
	if err := Unmarshal(input, &got); err != nil {
		t.Fatal(err)
	}
	want := string([]byte{0x00, 0x1f, 0xff, '\\', 'n'})
	if len(got) != 1 || got[0] != want {
		t.Fatalf("got = %#v, want []interface{}{%q}", got, []byte(want))
	}
}
```

这里的 `want` 精确包含 `CRLF`、缩进、单引号、双引号、反斜杠、实际 `LF` 和末尾单独 `CR`。第二个测试确认控制字节和无效 UTF-8 不被替换。

- [ ] **步骤 2：运行基础解码测试并确认失败**

运行：

```bash
go test ./... -run '^TestUnmarshalBacktickString' -count=1
```

预期：FAIL。Scanner 已接受输入，但 decoder 尚未把 `` ` `` 归类为字符串，通常出现内部解码阶段错误。

- [ ] **步骤 3：实现 `unquoteBytes` raw 快速路径**

将 `decode.go` 的 `unquoteBytes` 开头改为：

```go
func unquoteBytes(s []byte) (t []byte, ok bool) {
	if len(s) < 2 {
		return
	}
	if s[0] == '`' {
		if s[len(s)-1] != '`' {
			return
		}
		return s[1 : len(s)-1], true
	}
	if (s[0] != '"' && s[0] != '\'') || (s[len(s)-1] != '"' && s[len(s)-1] != '\'') {
		return
	}
```

保留该函数后续单双引号逻辑不变。raw 路径必须直接返回原切片的中间部分，不调用 `utf8.DecodeRune`。

- [ ] **步骤 4：将反引号加入字符串值分支**

在 `literalStore` 和 `literalInterface` 中，把现有分支：

```go
case '"', '\'': // string
```

改为：

```go
case '"', '\'', '`': // string
```

同时在 `literalStore` 的 `encoding.TextUnmarshaler` 类型检查中，把：

```go
if item[0] != '"' && item[0] != '\'' {
```

改为：

```go
if item[0] != '"' && item[0] != '\'' && item[0] != '`' {
```

不要修改对象键解码分支，因为 scanner 不允许反引号键。

- [ ] **步骤 5：格式化并运行基础解码测试**

运行：

```bash
gofmt -w decode.go backtick_string_test.go
go test ./... -run '^TestUnmarshalBacktickString' -count=1
```

预期：PASS；测试比较原始字节，因此 `CRLF`、单独 `CR` 和 `0xff` 必须完全保留。

- [ ] **步骤 6：运行 decoder 回归测试**

运行：

```bash
go test ./... -run '^(TestUnmarshal|TestDecodeSingleQuoteStringInterface|TestQuotedQuote|TestInvalidNewline)$' -count=1
```

预期：PASS，单双引号的转义、UTF-8 和换行行为保持不变。

- [ ] **步骤 7：提交 decoder 变更**

```bash
git add decode.go backtick_string_test.go
git commit -m "feat(解码器): 解码反引号原始字符串" -m "直接移除反引号定界符并逐字节保留内容，覆盖普通 string 和 interface 解码路径。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

## 任务 3：覆盖自定义解码接口、RawMessage 和流式 Decoder

**文件：**
- 修改：`backtick_string_test.go`

- [ ] **步骤 1：定义测试辅助类型并编写失败测试**

向 `backtick_string_test.go` 追加以下类型和测试：

```go
type backtickJSONReceiver []byte

func (r *backtickJSONReceiver) UnmarshalJSON(data []byte) error {
	*r = append((*r)[:0], data...)
	return nil
}

type backtickTextReceiver []byte

func (r *backtickTextReceiver) UnmarshalText(data []byte) error {
	*r = append((*r)[:0], data...)
	return nil
}

func TestBacktickStringUnmarshalInterfaces(t *testing.T) {
	input := []byte("`line 1\r\n\\n'\\\"line 2`")
	content := input[1 : len(input)-1]

	var jsonValue backtickJSONReceiver
	if err := Unmarshal(input, &jsonValue); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(jsonValue, input) {
		t.Fatalf("UnmarshalJSON received %q, want %q", []byte(jsonValue), input)
	}

	var textValue backtickTextReceiver
	if err := Unmarshal(input, &textValue); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(textValue, content) {
		t.Fatalf("UnmarshalText received %q, want %q", []byte(textValue), content)
	}
}

func TestBacktickStringRawMessage(t *testing.T) {
	input := []byte("{value:`line 1\r\nline 2`}")
	want := []byte("`line 1\r\nline 2`")
	var got struct {
		Value RawMessage
	}
	if err := Unmarshal(input, &got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Value, want) {
		t.Fatalf("RawMessage = %q, want %q", []byte(got.Value), want)
	}
}

func TestDecoderBacktickString(t *testing.T) {
	dec := NewDecoder(strings.NewReader("`line 1\nline 2` true"))
	var first string
	if err := dec.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if first != "line 1\nline 2" {
		t.Fatalf("first = %q", first)
	}
	var second bool
	if err := dec.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if !second {
		t.Fatal("second = false, want true")
	}
}
```

并在 import 中加入：

```go
import (
	"bytes"
	"strings"
	"testing"
)
```

- [ ] **步骤 2：运行接口和流式测试**

运行：

```bash
gofmt -w backtick_string_test.go
go test ./... -run '^(TestBacktickStringUnmarshalInterfaces|TestBacktickStringRawMessage|TestDecoderBacktickString)$' -count=1
```

预期：PASS。若 `UnmarshalText` 失败并报告无法解码字符串，检查任务 2 是否已在 TextUnmarshaler 类型检查中接受 `` ` ``；若 `UnmarshalJSON` 或 `RawMessage` 内容不含首尾反引号，检查是否错误地提前调用了 `unquoteBytes`。

- [ ] **步骤 3：补充结束反引号和相邻语法测试**

向 `TestBacktickStringScannerErrors` 表格追加：

```go
{
	name:    "first backtick terminates string",
	input:   "{value:`a`b`}",
	wantErr: "invalid character 'b' after object key:value pair",
},
```

该用例证明第一个反引号结束字符串，不存在反引号转义。

- [ ] **步骤 4：运行全部扩展测试**

运行：

```bash
gofmt -w backtick_string_test.go
go test ./... -run 'Backtick' -count=1
```

预期：PASS，所有名称包含 `Backtick` 的测试通过。

- [ ] **步骤 5：提交接口和流式测试**

```bash
git add backtick_string_test.go
git commit -m "test(解码器): 覆盖反引号字符串解码入口" -m "验证 UnmarshalJSON、UnmarshalText、RawMessage、流式 Decoder 及首个反引号终止规则。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

## 任务 4：记录非标准扩展并执行完整验证

**文件：**
- 修改：`README.md:1-5`

- [ ] **步骤 1：在 README 增加扩展说明**

在现有简介后追加：

````markdown

## Backtick raw strings

This fork supports backtick-delimited raw strings as a non-standard JSON5
extension. Backtick strings are allowed as values, but not as object keys.
Their contents are preserved byte-for-byte: quotes, backslashes, escape-like
sequences, control bytes, and line endings are not interpreted or normalized.
The first backtick terminates the string, so a backtick cannot appear in its
contents.

```json5
{
  shell: {
    script: `#!/bin/bash
echo "double quotes need no escaping"
echo 'single quotes need no escaping'
`,
  },
}
```

Other JSON5 implementations may reject this syntax.
````

README 保持现有英文文档风格，不扩写无关使用说明。

- [ ] **步骤 2：运行格式与差异检查**

运行：

```bash
gofmt -w scanner.go decode.go backtick_string_test.go
git diff --check
git status --short
```

预期：`git diff --check` 无输出且退出码为 0；状态只包含本任务尚未提交的 `README.md`，以及计划执行过程中确实尚未提交的相关文件。

- [ ] **步骤 3：运行完整测试套件**

运行：

```bash
go test ./...
```

预期：包 `github.com/titanous/json5` 显示 `ok`，退出码为 0。

- [ ] **步骤 4：重复运行扩展测试排除状态污染**

运行：

```bash
go test ./... -run 'Backtick' -count=10
```

预期：连续 10 次 PASS。

- [ ] **步骤 5：提交 README**

```bash
git add README.md
git commit -m "docs(解析器): 说明反引号字符串扩展" -m "记录反引号字符串仅可作为值、逐字节保留及不能包含反引号等兼容边界。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

- [ ] **步骤 6：确认最终仓库状态并推送**

运行：

```bash
git status --short --branch
git log -5 --oneline
git push origin main
```

预期：工作树干净；最近提交包含本计划产生的 scanner、decoder、测试和 README 提交；普通 push 成功。
