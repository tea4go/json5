# json-convert 双语 README 设计

## 目标

将 `cmd/json-convert` 当前中文参考手册调整为中英文双语文档：

- `README.md`：英文完整参考手册；
- `README_zh.md`：中文完整参考手册。

两份文档内容、规则、表格、示例和警告保持等价，并提供相互切换的语言链接。

## 文件变更

```text
cmd/json-convert/README.md     # 由中文改为英文完整手册
cmd/json-convert/README_zh.md  # 由当前中文 README 重命名而来
```

不修改：

- `cmd/json-convert/*.go`；
- 测试；
- 根目录 `README.md`；
- `go.mod`、`go.sum`；
- 命令行为和 `--help`。

## 中文版

将当前 `cmd/json-convert/README.md` 完整保留为 `README_zh.md`，只在标题下加入语言导航：

```markdown
[English](README.md) | 中文
```

除导航外，不删减或修改现有规则、示例和注意事项。

## 英文版

新 `README.md` 是中文版的完整等价翻译，标题下加入：

```markdown
English | [中文](README_zh.md)
```

章节与中文版一一对应：

1. Quick Start；
2. Command Syntax and Options；
3. Process Behavior；
4. JSON → JSON5；
5. JSON5 → JSON；
6. Comment-to-`_hint` Conversion；
7. Backtick Raw String Notes；
8. Ordering, Duplicate Keys, and Downstream Risks；
9. File Safety；
10. Common Errors；
11. PowerShell and Git Bash Examples；
12. Capability and Limitation Summary。

## 翻译规则

- 使用自然、专业的英文技术写作，不逐字硬译；
- 英文版不删减中文版的功能、限制、错误和警告；
- 命令、文件名、代码标识符、错误文本和程序输出保持原样；
- 中文示例中的注释和 `_hint` 内容保持中文，使两份文档示例输出逐字节一致；
- 两份文档的标题层级、表格、列表和代码块数量保持对应；
- 统一使用以下术语：
  - `strict JSON`
  - `extended JSON5`
  - `backtick raw string`
  - `hint field`
  - `duplicate keys`
  - `atomic replacement`
  - `non-regular output`
  - `non-finite number`
- 保留反引号扩展不是标准 JSON5 的醒目警告；
- 保留 stdin/stdout、原地转换、原始排版和全部注释保留等能力限制。

## 验证

- 两份语言导航链接指向存在文件；
- 对比一级至三级标题，确保章节一一对应；
- 对比表格和代码围栏数量，确保没有漏译；
- 抽查所有命令、错误文本、数字表和示例输出，确保两版一致；
- 实际运行 JSON → JSON5、JSON5 → JSON 和紧凑输出代表命令；
- 示例输出与两份文档一致；
- 两份文件均为有效 UTF-8，无 `U+FFFD`、NUL；
- Markdown 围栏配对；
- `git diff --check` 通过；
- `go test ./...` 通过；
- 变更范围只包含 `cmd/json-convert/README.md` 和 `README_zh.md`。

## 提交

使用单一文档提交，标题建议：

```text
docs(转换器): 添加英文使用手册
```

正文说明保留中文手册、增加英文完整翻译和语言切换链接。
