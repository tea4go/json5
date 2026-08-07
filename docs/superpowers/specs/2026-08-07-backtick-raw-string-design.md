# 反引号 Raw String 扩展设计

## 背景

本项目实现标准 JSON5 解码。标准 JSON5 仅支持单引号和双引号字符串，长 Shell 脚本、SQL 等内容同时包含单引号、双引号、反斜杠和实际换行时，需要大量转义，配置可读性较差。

本次为该 fork 增加反引号 raw string 扩展。该语法参考 Go raw string 的主要使用体验，但换行处理采用逐字节保留，不执行 Go 的回车移除规则。

## 目标

允许反引号字符串作为 JSON5 值：

~~~json5
{
  shell: {
    script: `#!/bin/bash
echo "hello"
echo 'world'
echo "\n remains raw"
`,
  },
}
~~~

实现必须满足：

- 单引号和双引号无需转义；
- 反斜杠及 `\n`、`\t`、`\uXXXX` 等序列不进行解释；
- 实际换行、空行、缩进及首尾空白全部保留；
- `LF`、`CRLF` 和单独的 `CR` 逐字节保留；
- 控制字节和无效 UTF-8 字节逐字节保留；
- 不改变现有 JSON 和 JSON5 行为；
- 不增加依赖或公开 API。

## 非目标

本次不实现：

- 反引号对象键；
- 反引号转义或在 raw string 内容中嵌入反引号；
- JavaScript 模板插值；
- 自动去除首尾换行；
- 自动删除公共缩进；
- 严格 JSON5 模式与扩展模式开关；
- 编码或 Marshal 反引号字符串。

## 语法与语义

### 使用位置

反引号字符串只允许作为值，可用于顶层值、对象值和数组元素：

~~~json5
`top-level`

{value: `object value`}

[`array value`]
~~~

反引号对象键非法：

~~~json5
{`key`: "value"}
~~~

解析器沿用现有对象键错误路径，不新增专用错误。

### 终止规则

起始反引号后的第一个反引号结束字符串。反引号没有转义形式，因此 raw string 内容无法包含反引号。

### Raw 规则

除结束反引号外，所有字节均作为普通内容：

| 输入内容 | 解码结果 |
|---|---|
| `'` | 单引号字节 |
| `"` | 双引号字节 |
| `\n` | 反斜杠和字母 `n` 两个字节 |
| 反斜杠后跟 `u4e2d` | 原始 6 个 ASCII 字节 |
| 实际 `LF` | 保留 `LF` |
| 实际 `CRLF` | 保留 `CRLF` |
| 单独的 `CR` | 保留 `CR` |
| 无效 UTF-8 | 原始字节不变 |

解析器不执行转义、UTF-8 校验、UTF-8 修复、换行标准化、trim 或公共缩进删除。

### 未闭合字符串

输入结束前未遇到结束反引号时，沿用 `scanner.eof()` 的现有错误：

```text
unexpected end of JSON input
```

不增加专用错误类型或错误消息。

## 实现设计

### Scanner

在 `scanner.go` 的 `stateBeginValue` 中识别反引号，并进入独立状态 `stateInStringBacktick`：

```go
case '`':
    s.step = stateInStringBacktick
    return scanBeginLiteral
```

新状态只识别结束反引号：

```go
func stateInStringBacktick(s *scanner, c byte) int {
    if c == '`' {
        s.step = stateEndValue
    }
    return scanContinue
}
```

该状态不检查控制字符和 UTF-8，也不把反斜杠视为转义起始符。

`stateBeginObjectKey` 保持不变，从语法入口保证反引号不能作为对象键。

### Decoder

将反引号加入所有字符串值分支：

```go
case '"', '\'', '`':
```

受影响的解码路径包括普通反射解码和 `interface{}` 快速路径。

`unquoteBytes` 增加 raw string 快速路径：

1. 检查长度至少为 2；
2. 如果首字符为反引号，则要求尾字符也是反引号；
3. 直接返回 `s[1:len(s)-1]`；
4. 不复制、不转义、不校验或修复 UTF-8；
5. 单引号和双引号继续使用现有逻辑。

### 自定义解码接口

行为沿用当前 JSON5 字符串约定：

- `json.Unmarshaler.UnmarshalJSON` 收到包含首尾反引号的原始字面量；
- `encoding.TextUnmarshaler.UnmarshalText` 收到移除首尾反引号后的 raw 内容。

例如输入：

~~~json5
`a
"b"
'c'
\n`
~~~

`UnmarshalJSON` 收到包含首尾反引号和全部中间原始字节的数据。`UnmarshalText` 只收到中间原始字节。

### 其他解析入口

`Unmarshal` 和 `Decoder.Decode` 共用 scanner，因此应自然识别反引号值。内部 `checkValid` 也必须接受合法的反引号值，并拒绝反引号对象键及未闭合 raw string。

本仓库当前没有 `Valid`、`Compact` 或 `Indent` 公共函数。本次不为测试该扩展而新增这些 API。

`RawMessage` 应保留包含首尾反引号的完整原始字面量。

## 兼容性

现有合法 JSON 和 JSON5 行为保持不变：

- 单引号和双引号继续执行原有转义和 UTF-8 处理；
- 注释、数字、对象、数组和未加引号的键不变；
- 现有错误消息和字节偏移不变；
- 原来在值起始位置非法的反引号现在成为合法 raw string 起始符；
- 原来在对象键起始位置非法的反引号仍然非法。

README 必须明确说明反引号字符串是该 fork 默认启用的非标准 JSON5 扩展，其他 JSON5 实现不一定支持。

## 测试设计

### 正常解码

覆盖以下场景：

1. 空 raw string；
2. 基本单行字符串；
3. 单引号和双引号无需转义；
4. `\n`、`\t`、`\\`、反斜杠后跟 `u4e2d` 均原样保留；
5. 实际多行、空行、缩进、首尾换行全部保留；
6. `LF`、`CRLF` 和单独的 `CR` 分别逐字节保留；
7. `0x00` 等控制字节原样保留；
8. 无效 UTF-8 字节原样保留；
9. 顶层值、对象值和数组元素均可解析。

### 解码路径

覆盖：

- 解码到 `string`；
- 解码到 `interface{}`；
- `Decoder.Decode`；
- `encoding.TextUnmarshaler`；
- `json.Unmarshaler`；
- `RawMessage`；
- 内部 `checkValid`。

### 错误与限制

覆盖：

- 反引号对象键被拒绝，并沿用现有对象键错误；
- 未闭合 raw string 返回 `unexpected end of JSON input`；
- 第一个反引号结束字符串，后续无分隔内容按现有语法报错。

## 验收标准

运行：

```bash
go test ./...
```

必须满足：

- 新增测试全部通过；
- 现有测试全部通过；
- 命令退出码为 0；
- 改动仅包含实现、相关测试和 README；
- 不增加依赖或公开 API；
- README 明确标注该语法不是标准 JSON5。
