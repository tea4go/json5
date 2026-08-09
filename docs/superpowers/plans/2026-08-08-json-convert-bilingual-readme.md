# json-convert 双语 README 实施计划

> **面向 AI 代理的工作者：** 必需子技能：使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 将现有中文 `cmd/json-convert/README.md` 保留为 `README_zh.md`，并创建内容完整等价、表达自然的英文 `README.md`。

**架构：** 中文版仅增加语言切换链接，英文版逐节完整翻译中文手册。命令、错误文本、输入输出示例和代码块保持一致，通过结构对比、实际命令、编码检查和完整测试验证交付。

**技术栈：** Markdown、UTF-8、Go 1.19、Git。

---

### 任务 1：创建并验证双语参考手册

**文件：**
- 修改：`cmd/json-convert/README.md`
- 创建：`cmd/json-convert/README_zh.md`

- [ ] **步骤 1：保存中文手册并添加语言导航**

复制当前 `cmd/json-convert/README.md` 为 `cmd/json-convert/README_zh.md`，在一级标题后加入：

```markdown
[English](README.md) | 中文
```

除语言导航外，中文版的规则、示例、表格和警告保持不变。

- [ ] **步骤 2：编写完整英文手册**

将 `cmd/json-convert/README.md` 改为英文完整翻译，在一级标题后加入：

```markdown
English | [中文](README_zh.md)
```

英文版必须完整对应以下章节：

```text
Quick Start
Command Syntax and Options
Process Behavior
JSON → JSON5
JSON5 → JSON
Comment-to-_hint Conversion
Backtick Raw String Notes
Ordering, Duplicate Keys, and Downstream Risks
File Safety
Common Errors
PowerShell and Git Bash Examples
Capability and Limitation Summary
```

统一使用 `strict JSON`、`extended JSON5`、`backtick raw string`、`hint field`、`duplicate keys`、`atomic replacement`、`non-regular output` 和 `non-finite number`。命令、错误文本、JSON/JSON5 示例及中文示例数据保持原样。

- [ ] **步骤 3：检查双语结构完整性**

使用脚本提取两份文档的标题、表格分隔行和代码围栏数量，确认：

- 两份文档的章节一一对应；
- 表格数量一致；
- 代码围栏均成对；
- `README.md` 链接到 `README_zh.md`；
- `README_zh.md` 链接到 `README.md`。

- [ ] **步骤 4：实际验证代表性命令和示例**

在临时目录执行：

```bash
go run ./cmd/json-convert --out json5 --indent 2 input.json output.json5
go run ./cmd/json-convert --out json5 --indent 0 input.json compact.json5
go run ./cmd/json-convert --out json --indent 2 input.json5 output.json
```

逐字节核对两份 README 中对应输出；用 `encoding/json.Valid` 验证严格 JSON 输出。临时文件不得提交。

- [ ] **步骤 5：执行编码、格式和测试验证**

运行：

```bash
git diff --check
go test ./...
```

用 UTF-8 脚本检查两份 README：

```text
U+FFFD = 0
NUL = 0
```

确认变更范围只包含：

```text
cmd/json-convert/README.md
cmd/json-convert/README_zh.md
```

不得修改 Go 代码、测试、根 README、`go.mod` 或 `go.sum`，也不得包含工作区中与本任务无关的既有改动。

- [ ] **步骤 6：自审并提交**

逐节对照中英文内容，确认没有漏译、误译、矛盾或新增未实现承诺。提交：

```bash
git add cmd/json-convert/README.md cmd/json-convert/README_zh.md
git commit -m "docs(转换器): 添加英文使用手册" -m "保留中文完整手册，新增等价英文翻译和双向语言切换链接。" -m "Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

- [ ] **步骤 7：提交后验证并推送**

运行：

```bash
go test ./...
git show --stat --oneline HEAD
git status --short --branch
git push origin main
```

预期：测试通过；提交只包含两份 `cmd/json-convert` README；普通 push 成功。
