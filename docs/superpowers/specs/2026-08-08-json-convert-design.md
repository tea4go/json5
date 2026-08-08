# JSON 与扩展 JSON5 双向转换命令设计

## 背景

本项目已支持非标准的反引号 raw string。长 Shell 脚本、SQL 等字符串可以用反引号表示，从而避免转义单引号、双引号、反斜杠和实际换行。

本次新增独立命令 `json-convert`，用于在标准 JSON 与本项目扩展 JSON5 之间转换。工具必须保留对象成员顺序、重复键和数字文本，并在 JSON5 转 JSON 时把对象成员注释转换为 `<字段名>_hint` 字段。

## 目标

新增以下命令：

```bash
json-convert --out json5 [--indent 0-8] input.json output.json5
json-convert --out json  [--indent 0-8] input.json5 output.json
```

实现目标：

- 标准 JSON 转扩展 JSON5 时，所有不含反引号的字符串值使用反引号；
- 扩展 JSON5 转标准 JSON 时，输出严格合法的 JSON；
- 保留对象成员顺序和重复键；
- 尽量保留标准数字的原始文本，避免精度丢失；
- 将可关联的对象成员注释转换为 `_hint` 字段；
- 提供可配置的紧凑或缩进格式；
- 安全覆盖输出文件，失败时保留原输出文件；
- 不修改现有 JSON5 decoder 的公开 API 和行为。

## 非目标

本次不实现：

- stdin 或 stdout 转换模式；
- 原地转换；
- 自动推断输出格式；
- `--force` 参数；
- 保留 JSON5 原始排版；
- 保留顶层注释或数组元素注释；
- 将 `Infinity`、`-Infinity` 或 `NaN` 有损映射为 `null` 或字符串；
- 在核心 JSON5 decoder 中公开 token API；
- JSON5 编码公共库 API。

## 命令行接口

### 用法

```bash
go run ./cmd/json-convert --out json5 --indent 2 input.json output.json5
go run ./cmd/json-convert --out json --indent 0 input.json5 output.json
```

### 参数

- `--out` 必填，只允许 `json` 或 `json5`；
- `--indent` 可选，默认值为 `2`；
- `--indent 0` 输出紧凑格式；
- `--indent 1` 至 `--indent 8` 指定每层空格数；
- 负数、大于 `8`、小数或非整数均报错；
- 必须提供两个位置参数，依次为输入文件和输出文件；
- 参数缺失、多余或非法时输出用法并以非零状态退出。

成功时不向 stdout 输出转换内容。错误写入 stderr。

## 文件安全

- 规范化输入和输出路径，并检查是否指向同一文件；
- 输入和输出相同路径或同一文件时拒绝转换；
- 输出文件存在时允许覆盖；
- 程序先完整读取、解析、转换并生成结果；
- 在输出目录创建临时文件；
- 临时文件写入并关闭成功后，再替换目标文件；
- 任一步失败时清理临时文件，并保留已有输出文件；
- 输出末尾统一包含一个换行符，`--indent 0` 也不例外。

## 架构

转换工具位于独立命令目录：

```text
cmd/json-convert/
├── main.go
├── parser.go
├── writer.go
├── comments.go
└── json_convert_test.go
```

职责如下：

- `main.go`：解析参数、校验路径、协调读取、转换和安全替换；
- `parser.go`：实现词法分析、递归下降解析和有序语法树；
- `writer.go`：输出 JSON 或扩展 JSON5，处理缩进、字符串和数字；
- `comments.go`：清理注释、判定归属并生成 `_hint` 成员；
- `json_convert_test.go`：覆盖解析、转换、注释、CLI 和文件安全。

这些文件只服务转换命令，不修改现有核心 decoder。

## 有序语法树

转换器使用专用的有序语法树，不把对象解码为 Go `map`。概念结构如下：

```go
type value struct {
    kind   valueKind
    text   string
    object []member
    array  []value
    pos    position
}

type member struct {
    key              string
    value            value
    leadingComments  []comment
    trailingComments []comment
}

type comment struct {
    text      string
    startLine int
    endLine   int
}
```

实际字段可按 Go 习惯调整，但必须满足：

- 对象以 `[]member` 保存，保留成员顺序和重复键；
- 数组以 `[]value` 保存，保留元素顺序；
- 字符串保存解码后的精确字节；
- 数字保存原始 token 文本；
- 注释保存原始位置和清理后的正文；
- 对象键解码为字符串，输出时统一使用 JSON 双引号。

## 输入解析模式

### 输出为 JSON5 时

`--out json5` 的输入严格按标准 JSON 解析：

- 对象键必须使用双引号；
- 字符串只允许标准 JSON 转义；
- 不允许注释、单引号、反引号和尾随逗号；
- 不允许 JSON5 扩展数字；
- 保存数字原始文本；
- 保留重复键和成员顺序；
- 输入必须只包含一个完整顶层值，之后只能有 JSON 空白。

### 输出为 JSON 时

`--out json` 接受项目支持的 JSON5 语法及反引号扩展：

- 双引号、单引号和反引号字符串值；
- 双引号、单引号和合法未加引号对象键；
- 行注释和块注释；
- 尾随逗号；
- 十六进制、前导 `+`、前导或尾随小数点；
- `Infinity`、`-Infinity` 和 `NaN` 可以被解析，但输出阶段报错；
- 反引号只允许作为值，不允许作为对象键；
- raw string 内容逐字节保留；
- 解析错误包含输入文件、行号和列号。

## JSON 转 JSON5

执行：

```bash
json-convert --out json5 input.json output.json5
```

规则：

- 对象键始终输出为标准 JSON 双引号字符串；
- 不含反引号的字符串值使用反引号；
- 含反引号的字符串值回退为标准 JSON 双引号字符串；
- 反引号字符串中的单引号、双引号、反斜杠、换行和控制字符输出为解码后的原始内容；
- 数字保留输入原文，包括大整数、`1.2300` 和 `1e6`；
- 布尔值和 `null` 使用标准形式；
- 对象顺序和重复键保持不变；
- JSON 输入没有注释，因此不生成 `_hint`。

示例输入：

```json
{
  "name": "demo",
  "command": "echo \"hello\"\necho 'world'",
  "inline": "echo `date`",
  "number": 1.2300
}
```

示例输出：

~~~json5
{
  "name": `demo`,
  "command": `echo "hello"
echo 'world'`,
  "inline": "echo `date`",
  "number": 1.2300
}
~~~

## JSON5 转 JSON

执行：

```bash
json-convert --out json input.json5 output.json
```

规则：

- 所有对象键输出为标准 JSON 双引号字符串；
- 所有字符串值输出为标准 JSON 双引号字符串；
- 引号、反斜杠、换行和控制字符使用标准 JSON 转义；
- 输出必须是严格合法的标准 JSON；
- 对象顺序和重复键保持不变；
- 尾随逗号被移除；
- 顶层注释和数组元素注释被删除；
- 输出前校验所有字符串及生成的 hint 文本是有效 UTF-8；
- 遇到无效 UTF-8 时报告位置并停止，不覆盖输出文件。

## 数字转换

标准 JSON 数字保持原始文本。JSON5 扩展数字按最小规则规范化：

| JSON5 输入 | JSON 输出 |
|---|---|
| `0xFF` | `255` |
| `-0xFF` | `-255` |
| `+12` | `12` |
| `.5` | `0.5` |
| `-.5` | `-0.5` |
| `1.` | `1.0` |
| `+1.` | `1.0` |
| `Infinity` | 报错 |
| `-Infinity` | 报错 |
| `NaN` | 报错 |

十六进制使用任意精度整数转换，避免溢出。十进制和指数形式不经过 `float64`，防止精度丢失。

## 注释转换为 `_hint`

该规则只在 `--out json` 时生效。

### 可关联注释

以下注释生成 `<字段名>_hint`：

1. 对象成员前的一个或多个前置注释；
2. 与成员结束位置同一行的行尾注释；
3. 行尾注释可以位于成员逗号之前或之后；
4. 多行字符串、数组或对象按值结束定界符或成员逗号所在行判断。

顶层注释和数组元素注释删除。

前置注释与字段之间允许任意数量的空白行，只要中间没有其他语法 token，仍归属于该字段。

### 归属优先级

如果注释起点与前一成员结束位置在同一行，则优先归属前一成员，不再作为后一成员的前置注释：

```json5
{
  port: 8080, /* 服务名称 */ name: `demo`,
}
```

该注释生成 `port_hint`，不生成 `name_hint`。每条注释最多归属一个成员。

### 注释清理

行注释删除 `//`。块注释按以下步骤清理：

- 删除 `/*` 和 `*/`；
- 删除正文首尾空白行；
- 删除每行公共缩进；
- 删除每行开头可选的 `*` 及其后一个空格；
- 保留正文内部换行；
- 多条注释按源码顺序用换行连接。

例如：

```json5
{
  /*
   * 服务名称
   * 用于页面展示
   */
  name: `demo`,
}
```

转换后的 hint 值为 `服务名称\n用于页面展示`。

### 生成字段

输入：

```json5
{
  // 服务名称
  name: `demo`, // 用于页面展示
}
```

输出：

```json
{
  "name_hint": "服务名称\n用于页面展示",
  "name": "demo"
}
```

规则：

- hint 字段紧邻对应成员并位于其前面；
- 字段名直接追加 `_hint`；
- `name_hint` 的注释生成 `name_hint_hint`；
- 已存在同名 hint 时允许重复键，不合并、不覆盖；
- 前置注释在前，行尾注释在后，以换行连接；
- 无关联注释时不生成 hint。

## 排版

- `--indent 0` 紧凑输出对象和数组；
- `--indent N` 每层使用 `N` 个空格；
- 格式化只影响结构性空白；
- 反引号 raw string 内部字节不因缩进改变；
- 空对象和空数组输出 `{}` 和 `[]`；
- 文件末尾统一写入一个换行符。

## 错误处理

错误包含阶段、输入文件和尽可能准确的位置：

```text
parse input.json5: line 4, column 12: unterminated raw string
convert input.json5: line 8, column 5: Infinity cannot be represented in JSON
convert input.json5: line 9, column 11: string contains invalid UTF-8
write output.json: permission denied
```

失败规则：

- 参数错误不创建输出；
- 解析或转换失败不修改已有输出；
- 临时文件写入或替换失败时清理临时文件；
- 成功退出码为 `0`，失败为非零。

## 测试设计

### JSON 转 JSON5

覆盖：

- 所有字符串值使用反引号；
- 含反引号的值回退到双引号；
- 多行、单双引号和反斜杠无需转义；
- 数字原文、成员顺序和重复键保持不变；
- `--indent 0` 及 `1–8`。

### JSON5 转 JSON

覆盖：

- 单引号、双引号和反引号字符串；
- 未加引号键、注释和尾随逗号；
- 十六进制、前导 `+`、前导或尾随小数点；
- `Infinity`、`-Infinity` 和 `NaN` 报错；
- 无效 UTF-8 报错；
- 成员顺序和重复键保持不变。

### 注释转换

覆盖：

- 行注释和块注释清理；
- 多个前置注释；
- 空白行不打断关联；
- 逗号前、逗号后行尾注释；
- 多行复合值的结束行；
- 前置与行尾注释换行合并；
- `_hint_hint`；
- 原有同名 hint 产生重复键；
- 顶层和数组注释删除；
- 同行歧义优先归属前一成员。

### CLI 与文件安全

覆盖：

- 缺失参数、非法 `--out` 和非法 `--indent`；
- 输入输出相同路径或同一文件；
- 输出文件覆盖；
- 失败时已有输出不变；
- 成功输出末尾恰好一个换行符。

### 回归

运行：

```bash
go test ./...
```

必须满足：

- 转换命令新增测试全部通过；
- 现有 JSON5 decoder 测试全部通过；
- 不增加第三方依赖；
- 不改变现有核心 decoder 的公开 API 或行为。

## 验收标准

- 两种 `--out` 模式均能通过文件参数完成转换；
- 输出格式满足标准 JSON 或本项目反引号 JSON5 扩展；
- 顺序、重复键和数字精度要求得到满足；
- 注释按本规格生成 `_hint`；
- 参数、解析、转换和文件安全测试完整；
- `go test ./...` 退出码为 `0`；
- 改动仅涉及转换命令、相关测试和必要文档。
