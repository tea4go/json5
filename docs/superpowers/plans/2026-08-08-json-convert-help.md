# json-convert 中文帮助文档实施计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 在 `cmd/json-convert/README.md` 编写中文参考手册，准确说明命令参数、双向转换规则、能力限制、注释提示、文件安全、常见错误和操作示例。

**架构：** 只新增一份与命令同目录的 Markdown 文档。文档事实以 `main.go`、`parser.go`、`writer.go`、`comments.go` 和现有测试为准；通过实际运行文档命令和比对输出验证示例，不修改命令实现、测试、根 README 或 `--help`。

**技术栈：** Markdown、Go 1.19、`go run`、Go 测试、UTF-8。

---

## 文件结构

- 创建：`cmd/json-convert/README.md` — `json-convert` 中文完整参考手册。

不修改以下文件：

- `cmd/json-convert/*.go`
- 根目录 `README.md`
- `go.mod`
- `go.sum`

---

### 任务 1：编写并验证 json-convert 中文参考手册

**文件：**
- 创建：`cmd/json-convert/README.md`
- 参考：`cmd/json-convert/main.go`
- 参考：`cmd/json-convert/parser.go`
- 参考：`cmd/json-convert/writer.go`
- 参考：`cmd/json-convert/comments.go`
- 参考：`cmd/json-convert/*_test.go`

- [ ] **步骤 1：建立文档标题、简介和快速开始**

创建 `cmd/json-convert/README.md`，使用以下开头：

````markdown
# json-convert 使用手册

`json-convert` 是标准 JSON 与本项目扩展 JSON5 之间的双向文件转换工具。

- `--out json5`：将严格标准 JSON 转换为本项目扩展 JSON5；
- `--out json`：将本项目支持的 JSON5 转换为严格标准 JSON。

> [!IMPORTANT]
> 反引号 raw string 是本项目的非标准 JSON5 扩展。其他 JSON5 实现可能无法解析 `json-convert` 生成的反引号字符串。

## 快速开始

```bash
# 标准 JSON → 扩展 JSON5
go run ./cmd/json-convert --out json5 --indent 2 input.json output.json5

# 扩展 JSON5 → 标准 JSON
go run ./cmd/json-convert --out json --indent 2 input.json5 output.json

# 紧凑输出
go run ./cmd/json-convert --out json5 --indent 0 input.json output.json5
```
````

- [ ] **步骤 2：编写命令语法、参数和退出行为**

追加：

````markdown
## 命令语法

```text
json-convert --out json|json5 [--indent 0..8] INPUT OUTPUT
```

| 参数 | 必填 | 默认值 | 说明 |
|---|---:|---:|---|
| `--out` | 是 | 无 | `json5`：JSON → JSON5；`json`：JSON5 → JSON |
| `--indent` | 否 | `2` | `0` 为紧凑输出，`1–8` 为每层缩进空格数 |
| `INPUT` | 是 | 无 | 输入文件路径 |
| `OUTPUT` | 是 | 无 | 输出文件路径 |

必须恰好提供输入和输出两个位置参数。`--out` 只接受 `json`、`json5`；`--indent` 只接受整数 `0–8`。

### 退出码和输出

| 退出码 | 含义 |
|---:|---|
| `0` | 转换成功 |
| `1` | 读取、解析、转换或写入失败 |
| `2` | 命令行参数错误 |

成功时不向 stdout 输出转换内容；错误写入 stderr。输出文件末尾始终包含一个 LF，`--indent 0` 也不例外。
````

- [ ] **步骤 3：编写 JSON → JSON5 能力、限制和完整示例**

创建「JSON → JSON5」章节，必须明确：

- 输入按严格标准 JSON 解析；
- 支持对象、数组、字符串、数字、布尔值、`null`、重复键、嵌套结构、合法 Unicode；
- 保留对象成员顺序、重复键和数字 token 原文；
- 对象键保持标准双引号；
- 不含反引号的字符串值使用反引号；
- 含反引号的字符串回退为标准双引号；
- 结构缩进不改变 raw string 内容；
- 注释、单引号/反引号输入、未引号键、尾随逗号、JSON5 数字、无效 UTF-8 和尾随内容会报错。

使用以下输入示例：

```json
{
  "name": "demo",
  "script": "echo \"hello\"\necho 'world'",
  "inline": "echo `date`",
  "number": 1.2300
}
```

文档中的预期输出必须与命令实际输出一致：

~~~json5
{
  "name": `demo`,
  "script": `echo "hello"
echo 'world'`,
  "inline": "echo `date`",
  "number": 1.2300
}
~~~

追加「严格 JSON 输入中不支持」列表：注释、单引号、反引号、未引号键、尾随逗号、十六进制、前导 `+`、`.5`、`1.`、`Infinity`、`NaN`、未转义控制字符、无效 UTF-8 和额外顶层内容。

- [ ] **步骤 4：编写 JSON5 → JSON 能力、数字规则和限制**

创建「JSON5 → JSON」章节，必须说明支持：

- 单引号、双引号和反引号字符串值；
- 单/双引号及合法未引号对象键；
- 行注释、块注释和尾随逗号；
- 十六进制、前导 `+`、前导/尾随小数点；
- 顺序、重复键和嵌套结构。

说明输出行为：所有键和值字符串使用标准 JSON 双引号和转义，移除尾随逗号，输出可由标准 JSON parser 读取。

加入精确数字表：

| JSON5 输入 | JSON 输出 |
|---|---|
| `0xFF` | `255` |
| `-0xFF` | `-255` |
| `+12` | `12` |
| `.5` | `0.5` |
| `-.5` | `-0.5` |
| `1.` | `1.0` |
| `+1.` | `1.0` |
| 标准十进制或指数 | 保留原文 |
| `Infinity` / `±Infinity` | 报错 |
| `NaN` / `±NaN` | 报错 |

说明十六进制使用任意精度整数。明确无法转换：非有限数、无效 UTF-8、未闭合字符串/注释、反引号对象键、非法数字 token 和顶层尾随内容。

- [ ] **步骤 5：编写注释转 `_hint` 章节**

使用以下示例：

~~~json5
{
  // 服务名称
  name: `demo`, // 用于页面展示
}
~~~

输出：

```json
{
  "name_hint": "服务名称\n用于页面展示",
  "name": "demo"
}
```

正文必须覆盖：

- 该规则只在 JSON5 → JSON 生效；
- 对象成员前置注释及成员结束行同行注释可关联；
- 逗号前、逗号后、多行 raw/object/array 结束行同行注释均支持；
- 顶层注释、数组元素注释和对象尾部独立注释删除；
- 空白行不打断前置关联；
- 前置在前、行尾在后，以换行连接；
- 同行歧义优先归前一成员；
- hint 在原字段之前，字段名直接追加 `_hint`；
- 支持 `_hint_hint` 和重复 hint 键，不合并、不覆盖；
- 块注释会清理定界符、公共缩进和可选 `*` 前缀。

- [ ] **步骤 6：编写 raw string、重复键和文件安全注意事项**

分别增加章节并明确：

**反引号 raw string：**

- 非标准 JSON5；
- 仅可作为值；
- 第一个反引号终止；
- 不能嵌入反引号，没有 `\`` 转义；
- `\n`、`\t`、`\uXXXX` 不解释；
- LF、CRLF、单独 CR 逐字节保留；
- JSON → JSON5 遇反引号内容时自动回退双引号。

**顺序和重复键：**

- 有序语法树保留顺序和重复键；
- `_hint` 可能产生重复键；
- 许多下游 parser 解码到 map 时只保留最后一个，使用者需自行确认兼容性。

**文件安全：**

- 禁止原地转换；
- hard link 或 symlink 指向同一输入也拒绝；
- 输出只能是不存在路径或普通文件；
- 目录、symlink 和其他非普通文件拒绝覆盖；
- 完整转换成功后才替换输出；
- 失败时保留旧输出；
- 替换与恢复都失败时，错误提供备份路径。

- [ ] **步骤 7：编写常见错误、Windows/Git Bash 示例和速查表**

常见错误表至少包含：

| 错误 | 原因 | 处理建议 |
|---|---|---|
| `--out must be json or json5` | 未指定或格式非法 | 使用 `--out json` 或 `--out json5` |
| `--indent must be an integer from 0 to 8` | 缩进越界 | 使用整数 `0–8` |
| `input and output are the same file` | 尝试原地转换 | 使用不同输出路径 |
| `non-finite number ... is not valid JSON` | JSON5 包含 Infinity/NaN | 转换前改为标准 JSON 可表示值 |
| `invalid UTF-8 in string` | 字符串编码非法 | 将输入修复为有效 UTF-8 |
| `refuse to replace non-regular output` | 输出是目录或 symlink | 使用普通文件路径 |
| `unterminated raw string` | 反引号字符串未闭合 | 添加结束反引号 |

加入 Windows PowerShell 示例：

```powershell
go run ./cmd/json-convert --out json5 --indent 2 `
  "C:\data\input.json" "C:\data\output.json5"
```

加入 Git Bash 示例：

```bash
go run ./cmd/json-convert --out json5 --indent 2 \
  /c/data/input.json /c/data/output.json5
```

结尾增加能力速查表，至少覆盖结构、顺序、重复键、数字精度、注释、raw string、非有限数字、无效 UTF-8、原地转换、stdin/stdout、原始排版。

- [ ] **步骤 8：实际运行文档中的核心转换示例**

在临时目录创建步骤 3 的 `input.json`，运行：

```bash
go run ./cmd/json-convert --out json5 --indent 2 input.json output.json5
```

逐字节比较 `output.json5` 与 README 预期输出，确认字符串没有多余反斜杠，`1.2300` 保持原文，文件末尾是一个 LF。

再创建带前置/行尾注释、扩展数字和尾随逗号的 `input.json5`，运行：

```bash
go run ./cmd/json-convert --out json --indent 2 input.json5 output.json
```

使用标准库验证：

```bash
go run <临时验证程序>
```

临时验证程序必须调用 `encoding/json.Valid` 检查输出，并逐字节核对 README 示例。临时文件不得提交。

- [ ] **步骤 9：执行文档与仓库验证**

运行：

```bash
git diff --check
go test ./...
git status --short
```

使用真实 Python 检查中文文档编码：

```bash
"/c/Users/tony/.local/bin/python.exe" -c "from pathlib import Path; p=Path('cmd/json-convert/README.md'); s=p.read_text(encoding='utf-8'); print('U+FFFD:', s.count(chr(0xfffd))); print('NUL:', s.count(chr(0)))"
```

预期：

- `git diff --check` 无输出；
- `go test ./...` 通过；
- `U+FFFD: 0`、`NUL: 0`；
- `git status --short` 只显示 `?? cmd/json-convert/README.md`；
- `git diff --name-only` 不包含任何 `.go`、根 README、`go.mod` 或 `go.sum`。

- [ ] **步骤 10：自审并提交帮助文档**

逐项对照以下实现事实：

- `main.go`：参数、退出码、同文件检查和输出文件类型；
- `parser.go`：两种输入语法、数字、字符串和注释；
- `writer.go`：反引号策略、JSON 转义、数字和 UTF-8；
- `comments.go`：注释清理及 `_hint`；
- 测试：错误与安全边界。

确认没有承诺实现不存在的 stdin/stdout、原地转换、原始排版或全部注释保留。

提交：

```bash
git add cmd/json-convert/README.md
git commit -m "docs(转换器): 添加中文使用手册" -m "说明命令参数、双向转换规则、注释提示、能力限制、文件安全、常见错误和操作示例。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

- [ ] **步骤 11：提交后重新验证并推送**

运行：

```bash
go test ./...
git status --short --branch
git show --stat --oneline HEAD
git push origin main
```

预期：测试通过；工作树干净；最新提交只包含 `cmd/json-convert/README.md`；普通 push 成功。
