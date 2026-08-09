# json-convert 中文参考手册

[English](README.md) | 中文

`json-convert` 在文件之间双向转换严格 JSON 与本项目支持的 JSON5 子集，也可以从 JSON5 推导 Go 对象定义文件。转换器使用有序语法树，因此可以保留对象成员顺序、重复键和数字原文；从 JSON5 转为 JSON 时，还可以把对象成员的关联注释转换为 `<字段名>_hint` 字段。

> [!IMPORTANT]
> 本工具生成和接受的反引号原始字符串是项目扩展，**不是标准 JSON5 语法**。输出给其他 JSON5 实现前，请先确认对方支持反引号字符串；否则应使用 `--out json` 生成严格 JSON。

## 快速开始

在 Windows 的 PowerShell 或 Git Bash 中，从仓库根目录运行以下 4 条命令：

```bash
go build -o json-convert.exe ./cmd/json-convert
./json-convert.exe --out json5 input.json output.json5
./json-convert.exe --out json --indent 4 input.json5 output.json
./json-convert.exe --out golang config.json5 config.go
```

第一条命令构建 Windows 可执行文件；后 3 条命令分别执行 JSON → JSON5、JSON5 → JSON 和 JSON5 → Go 定义转换。Linux/macOS 请将输出文件名改为 `json-convert`，并使用 `./json-convert` 执行。

## 命令语法与参数

```text
json-convert --out json|json5|golang [--indent 0..8] INPUT [OUTPUT]
```

| 参数 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `--out json` 或 `--out=json` | 是（三选一） | 无 | 将 JSON5 输入转换为严格 JSON。 |
| `--out json5` 或 `--out=json5` | 是（三选一） | 无 | 将严格 JSON 输入转换为本项目的 JSON5 格式。 |
| `--out golang` 或 `--out=golang` | 是（三选一） | 无 | 将 JSON5 输入转换为 Go 对象定义文件。 |
| `--indent N` 或 `--indent=N` | 否 | `2` | 每层缩进的空格数。`N` 必须是 `0`～`8` 的整数；`0` 表示紧凑输出。生成 Go 文件时该参数会被忽略。 |
| `INPUT` | 是 | 无 | 输入文件路径。 |
| `OUTPUT` | 否 | 自动推导 | 输出文件路径。省略或显式传空时，会自动生成 `<输入名>_convert` 加目标扩展名的文件。 |

位置参数可以是 1 个或 2 个，即 `INPUT`，以及可选的 `OUTPUT`。当 `OUTPUT` 为空时：

- `--out json` 生成 `<输入名>_convert.json`；
- `--out json5` 生成 `<输入名>_convert.json5`；
- `--out golang` 生成 `<输入名>_convert.go`。

选项必须写在位置参数之前；底层使用 Go `flag` 解析，遇到第一个位置参数后不再解析后续选项。

## 进程行为

| 退出码 | 含义 | 输出位置 |
| --- | --- | --- |
| `0` | 转换成功 | `stdout` 和 `stderr` 均为空。 |
| `1` | 读取、解析、转换或写入失败 | 错误写入 `stderr`。 |
| `2` | 命令行参数错误 | 用法和错误写入 `stderr`。 |

工具不把转换结果写入 `stdout`，只写入指定的 `OUTPUT` 文件。所有成功输出均以**恰好一个 LF 字节**（`0x0A`）结束，包括 Windows；不会改为 CRLF。

## JSON → JSON5

使用 `--out json5` 时，输入按严格 JSON 解析。

### 严格 JSON 输入规则

支持：

- `null`、`true`、`false`；
- 双引号字符串及标准 JSON 转义，包括 Unicode 代理对；
- 数组和对象；对象键必须使用双引号；
- 标准十进制数与指数，例如 `-0`、`1.0`、`1E+2`；
- 空格、Tab、LF 和 CR 空白。

拒绝：

- 注释、未加引号的键、单引号字符串和反引号字符串；
- 数组或对象的尾随逗号；
- `+1`、`.5`、`1.`、十六进制数、`Infinity`、`NaN`；
- 前导零（如 `01`）、非法转义、未转义控制字符、无效 UTF-8 和未配对代理项；
- Form Feed（换页符）空白和根值后的额外内容。

对象成员按输入顺序输出，重复键不会合并。数字不经过浮点解析，原始文本会保留，例如 `1.2300`、`1E+6` 和任意长度整数保持不变。

### 字符串输出策略

- 对象键始终使用标准 JSON 双引号。
- 字符串值不含反引号时，使用反引号原始字符串。解码后的换行、引号、反斜杠和控制字节直接位于反引号之间。
- 字符串值含反引号时，回退为标准 JSON 双引号字符串，并按 JSON 规则转义。
- 回退字符串必须是有效 UTF-8；严格 JSON 输入已保证这一点。

### 完整示例

`input.json`：

```json
{"z":"line\n\"quoted\"","tick":"has ` mark","z":1.2300,"huge":123456789012345678901234567890}
```

运行：

```bash
go run ./cmd/json-convert --out json5 --indent 2 input.json output.json5
```

`output.json5` 的实际字节内容如下。第一个 `z` 值中包含真实 LF，不是字符序列 `\n`：

```text
{
  "z": `line
"quoted"`,
  "tick": "has ` mark",
  "z": 1.2300,
  "huge": 123456789012345678901234567890
}
```

### 紧凑输出示例

将严格 JSON 转为不带排版空白的 JSON5：

```bash
./json-convert.exe --out json5 --indent 0 input.json compact.json5
```

例如，输入 `{"name":"demo","enabled":true}` 时，`compact.json5` 为：

```text
{"name":`demo`,"enabled":true}
```

## JSON5 → JSON

使用 `--out json` 时，输入按本项目支持的 JSON5 子集解析，输出由严格 JSON writer 生成。

### 支持的 JSON5 输入

- `//` 行注释和 `/* ... */` 块注释；
- 双引号、单引号字符串，以及项目扩展的反引号原始字符串；
- 双引号键、单引号键，或由 ASCII 字母、`_`、`$` 开头且后续可含数字的未引号键；
- 数组和对象尾随逗号；
- JSON5 数字形式：十六进制、前导 `+`、前导小数点、尾随小数点、`Infinity` 和 `NaN`；
- 双引号或单引号字符串中的 LF、CR 或 CRLF 反斜杠续行；
- 空格、Tab、LF、CR 和 Form Feed 空白。

### 明确拒绝的输入

- 以数字开头、含连字符或含非 ASCII 字母的未引号键，例如 `9abc`、`a-b`、`é`；
- 反引号对象键；
- 前导零十进制数，例如 `01`；
- 不完整数字、非法标识符后缀、非法转义、未终止字符串或注释；
- 单引号或双引号字符串中的未转义换行、控制字符或无效 UTF-8；
- 根值后的额外内容。

### 严格 JSON 输出与数字转换

输出对象键和值字符串都使用标准 JSON 双引号及转义。成员顺序和重复键仍然保留。

数字转换不经过 `float64`，因此不会因浮点精度丢失：

| JSON5 输入 | JSON 输出 | 规则 |
| --- | --- | --- |
| `0xFF`、`+0Xff` | `255` | 十六进制使用任意精度整数转换。 |
| `-0xFF` | `-255` | 保留负号。 |
| `+12` | `12` | 删除前导 `+`。 |
| `.5`、`+.5`、`-.5` | `0.5`、`0.5`、`-0.5` | 补齐整数位。 |
| `1.`、`1.e2` | `1.0`、`1.0e2` | 补齐小数位。 |
| `1.2300`、`1E+6` | 原样 | 已是合法 JSON 数字时保留原文。 |
| `Infinity`、`+Infinity`、`-Infinity`、`NaN` | 转换失败 | 非有限数不是合法 JSON。 |

十进制数字和指数原文不会被求值；超长十六进制整数通过任意精度整数转换。`-0x0` 输出为 `-0`。

### 注释与数字完整示例

`input.json5`：

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

运行：

```bash
go run ./cmd/json-convert --out json --indent 2 input.json5 output.json
```

`output.json`：

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

这里输入 raw 值中的 `\n` 是反斜杠和字母 `n` 两个普通字节，所以严格 JSON 输出需把反斜杠写成 `\\`。

## JSON5 → Go 结构体定义

使用 `--out golang` 时，工具按支持的 JSON5 子集解析输入，并生成经过 `gofmt` 格式化的 Go 源文件。此模式会忽略 `--indent`。传入 2 个文件路径时，第 2 个路径是显式输出路径；只传 `INPUT` 时，输出位于输入文件旁，名称为 `<输入名>_convert.go`。

包名取自输出目录名，并转换为合法的小驼峰标识符。根类型名取自**输入文件名**，移除最后一个扩展名后转换为导出的 Go 标识符。因此，`example/config.json5` → `example/generated.go` 会生成 `package example` 和 `type Config`；只改变输出文件名不会改变 `Config`。标识符不含可用字母或数字时，包名回退为 `main`，类型名回退为 `Root`；首字符是数字时分别添加 `x` 或 `X` 前缀。

### 结构、命名、类型与注释

- 顶层对象生成命名结构体，嵌套对象生成额外的命名结构体，数组生成切片。顶层数组或标量生成命名类型，其底层类型为推断出的切片或标量类型。
- 对象键按标点拆分并转换为导出字段名，原始键始终完整保留在 `json:"..."` tag 中。字段名冲突时添加数字后缀，例如 `DisplayName2`；生成的类型名冲突时采用相同规则。
- 布尔值、字符串、整数和非整数分别推断为 `bool`、`string`、`int64` 和 `float64`，十六进制整数也推断为 `int64`。`null`、空数组的元素、不兼容的混合值及其他未知值推断为 `any`。
- 工具会合并数组中全部元素的类型：整数与浮点数合并为 `float64`，对象元素递归合并字段，不兼容类型合并为 `any`。对象元素缺失的字段仍会生成为普通字段；生成器不会添加指针或 `omitempty`。
- 与对象成员关联的前置注释和同行尾随注释经清理后，生成为 Go 字段上方的 `//` 注释。此模式**不会**生成 `_hint` 字段；已有的 `_hint` 结尾键只是普通字段，处理方式与其他键相同。
- 重复对象键合并为一个字段类型。空对象生成空的命名结构体，空数组生成 `[]any`。非对象顶层值（数组、字符串、数字、布尔值或 `null`）生成命名类型声明，而不是顶层结构体。

### 完整示例

`example/config.json5`：

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

使用显式输出路径运行：

```bash
go run ./cmd/json-convert --out golang example/config.json5 example/generated.go
```

生成的 `example/generated.go` 如下：

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

省略第 2 个路径可改为生成 `example/config_convert.go`：

```bash
go run ./cmd/json-convert --out golang example/config.json5
```

对本示例而言，自动与显式命令生成的 Go 源码完全相同，因为包名和根类型名分别根据输出目录与输入文件名推导。

## 注释转换为 `_hint`

只有从 JSON5 转为 JSON 时才处理注释。规则如下：

1. 只为**对象成员**生成提示。关联注释合并为字符串，并在原成员前插入 `<原键>_hint`。
2. 同一成员有多条关联注释时，按出现顺序清理后用 LF 连接。
3. 成员值结束后、同一行上的注释归属该成员；逗号后仍在同一行的注释也归属前一成员。
4. 位于新行、下一成员之前的注释归属下一成员。
5. 多行对象或数组值闭合后，同一行的注释归属该对象成员。
6. 顶层注释、数组元素注释、文档尾注释，以及对象最后一个成员后另起一行的对象尾注释会被删除，不生成提示。
7. 空注释清理后为空，不生成提示。
8. 现有 `_hint` 字段不合并、不覆盖。注释属于 `name_hint` 时会生成 `name_hint_hint`。
9. 重复键分别处理，可能产生重复的 `_hint` 键；顺序保持不变。
10. 处理会递归进入嵌套对象，包括数组中的对象。

注释清理会：

- 删除 `//`、`/*`、`*/` 标记并去除首尾空白；
- 将 CRLF 和 CR 统一为 LF；
- 去掉块注释公共缩进；
- 去掉常见的每行前导 `*` 和其后的一个空格；
- 保留注释内部空行和相对缩进。

完整示例：

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

转换结果：

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

## 反引号 raw 字符串注意事项

反引号 raw 字符串是项目扩展，规则刻意简单：

- 只能作为值，不能作为对象键。
- 从开头反引号后读取到**第一个**反引号即终止；无法在内容中表示反引号。
- 没有任何转义语义。`\n`、`A` 和 `\\` 都只是普通字节序列。
- 实际 LF、CR、CRLF、NUL、其他控制字节乃至无效 UTF-8 都可位于 raw 内容中，并按原字节保留。
- LF、CR 和 CRLF 会影响后续解析错误的行列位置；CRLF 计为一次换行，单独 CR 也计为换行。
- 转为严格 JSON 时，raw 内容必须是有效 UTF-8，否则失败；控制字符和反斜杠会按 JSON 规则转义。

因此，raw 适合承载原样多行文本或字节，但不适合包含反引号的数据。JSON → JSON5 遇到反引号内容时会自动回退到双引号字符串。

## 顺序、重复键与下游风险

本工具保留对象成员顺序和重复键。例如 `{"x":1,"x":2}` 转换后仍有两个 `x`。这对无损文本转换很重要，但许多下游库会把对象解码到 `map`：

- 成员顺序通常丢失；
- 重复键通常只保留最后一个，也可能被拒绝；
- 自动生成的重复 `_hint` 键也面临相同问题。

需要保留这些信息时，请使用支持有序成员和重复键的解析器，不要直接解码到普通 `map`。

## 文件安全

- `INPUT` 与 `OUTPUT` 不能是同一路径。工具也会通过文件身份检测拒绝指向同一文件的硬链接和符号链接。
- 已存在的 `OUTPUT` 必须是普通文件。目录、符号链接及其他非普通文件会被拒绝，且不会修改其目标。
- 新输出的父目录必须已经存在。
- 工具先在输出目录创建 `.json-convert-*` 临时文件，完整写入并同步后再原子重命名，避免把半写文件暴露为输出。
- 转换在读取、解析或生成阶段失败时，不会改动现有输出。
- 某些平台不能直接覆盖现有文件时，工具会在同一输出目录使用 `.json-convert-backup-*` 备份，再替换输出；替换失败会尝试恢复旧文件。
- 如果替换和恢复同时失败，错误信息会报告保留的备份路径。请从该 `.json-convert-backup-*` 文件人工恢复；不要在确认数据前删除它。

## 常见错误

| 现象或错误 | 原因 | 处理方法 |
| --- | --- | --- |
| `--out must be json, json5, or golang` | 未传 `--out` 或值不受支持。 | 指定 `--out json`、`--out json5` 或 `--out golang`。 |
| `--indent must be an integer from 0 to 8` | 缩进超出范围。 | 使用 `0`～`8`。非整数会由参数解析器直接报错。 |
| `expected one or two file paths` | 未提供输入路径，或位置参数超过 2 个。 | 传 1 个路径以自动命名输出，或传 2 个路径以显式指定输出。 |
| `input and output are the same file` | 路径相同，或路径通过硬链接/符号链接指向同一文件。 | 使用不同的输出文件。 |
| `line N, column M: ...` | 输入语法错误。 | 按所示行列检查输入；注意 raw 中的 CR/LF 会影响行号。 |
| `unterminated raw string` | raw 字符串缺少结束反引号。 | 在 raw 内容末尾添加反引号；内容本身不能包含反引号。 |
| `parse input ...`（`--out json5`） | `--out json5` 的输入必须是严格 JSON，但输入含注释、单引号字符串、未加引号的键、尾随逗号等 JSON5 语法。 | 将输入改为严格 JSON；如果输入本来是 JSON5 并要转成 JSON，请改用 `--out json`。 |
| `non-finite number ... is not valid JSON` | 尝试把 `Infinity` 或 `NaN` 写成严格 JSON。 | 改为有限数，或保留为字符串。 |
| `invalid UTF-8 in string` | raw 字符串包含无效 UTF-8，无法写为严格 JSON。 | 先将内容转成有效 UTF-8。 |
| `refuse to replace non-regular output` | 输出是目录、符号链接或其他特殊文件。 | 改用不存在的路径或普通文件。 |
| `read input` / `write output` | 输入不可读，或输出目录不存在、无权限。 | 检查路径、父目录和文件权限。 |
| 参数放在 `INPUT` 后未生效 | Go `flag` 在第一个位置参数处停止解析。 | 把全部选项移到 `INPUT OUTPUT` 之前。 |

## PowerShell 与 Git Bash 示例

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

## 能力限制速查

| 能力 | 状态 |
| --- | --- |
| 文件到文件转换 | 支持。 |
| 从 JSON5 生成 Go 结构体定义 | 支持；使用 `--out golang`，包名和根类型名分别根据输出目录与输入文件名推导。 |
| 从 `stdin` 读取或向 `stdout` 输出数据 | 不支持；`-` 也只会被当作普通文件名。 |
| 原地覆盖输入 | 不支持，包括相同文件的硬链接和符号链接。 |
| 保留对象成员顺序、重复键 | 支持。 |
| 保留数字原文 | JSON → JSON5 支持；JSON5 → JSON 对合法 JSON 数字保留原文，其他形式按规则规范化。 |
| 保留原始缩进、空格、换行风格、引号样式 | 不支持；输出会按 `--indent` 重新排版。 |
| 原样保留注释 | 不支持；JSON5 → JSON 仅将可关联对象成员的注释转换为 `_hint`，其余删除。 |
| 标准 JSON5 全语法兼容 | 不支持；未引号键仅限 ASCII 子集，反引号字符串则是非标准扩展。 |
| 输出 `Infinity` / `NaN` 到严格 JSON | 不支持。 |
| 在 raw 字符串中转义反引号 | 不支持；第一个反引号总是终止字符串。 |
| 自动创建输出目录 | 不支持。 |
| JSON Schema 校验、键去重、语义合并 | 不支持。 |
