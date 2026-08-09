# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 常用命令

```bash
# 运行与 CI 相同的完整测试套件
go test -v ./...

# 运行根包或转换器包的测试
go test ./
go test ./cmd/json-convert

# 运行单个测试（将名称替换为目标 Test 函数）
go test ./ -run '^TestUnmarshal$' -v
go test ./cmd/json-convert -run '^TestConvertFileJSON5ToJSON$' -v

# 构建全部包，或单独构建转换器
go build ./...
go build -o json-convert.exe ./cmd/json-convert

# 直接运行文件转换命令
go run ./cmd/json-convert --out json input.json5 output.json
go run ./cmd/json-convert --out json5 input.json output.json5

# 格式与静态检查（仓库没有单独的 lint 配置）
gofmt -w <changed-go-files>
go vet ./...
```

`go.mod` 声明 Go 1.19；GitHub Actions 使用当前 stable Go，并只执行 `go test -v ./...`。`go vet ./...` 当前会在 `decode_test.go:1211` 报告未导出字段 `m2` 带 JSON tag；这是现有测试代码中的已知静态检查结果，CI 不运行 vet。

## 架构概览

### 核心 `json5` 解码包

根目录是仅解码的 `github.com/titanous/json5` 包，公开入口主要是 `Unmarshal`、流式 `Decoder`、`Number` 和 `RawMessage`；仓库没有公共 `Marshal`、`Valid`、`Compact` 或 `Indent` API。实现源自 Go `encoding/json` 的解码架构并扩展 JSON5 语法：

- `scanner.go` 是逐字节状态机，统一负责语法验证、值边界识别，以及注释、单引号、JSON5 数字、未加引号键和反引号 raw string 的词法状态。
- `decode.go` 在 scanner 验证后，通过反射把值写入 struct、map、slice 和自定义 `Unmarshaler`；解码到空接口时走非反射快速路径。修改一种字面量通常需要同时检查反射路径、interface 快速路径和 `unquoteBytes`。
- `stream.go` 的 `Decoder` 复用同一 scanner，从缓冲区切出一个完整顶层值后交给 `decodeState`，因此 scanner 变更同时影响 `Unmarshal` 和流式解码。
- `tags.go`、`fold.go` 和 `types.go` 处理与 `encoding/json` 兼容的 struct tag、大小写折叠和嵌入字段选择，并缓存反射字段元数据。

反引号 raw string 是本 fork 默认启用的非标准扩展：只允许作为值，第一个反引号终止内容，中间字节不做转义、UTF-8 修复或换行归一化；不要把它误当成标准 JSON5，也不要允许它作为对象键。

### `json-convert` 独立转换器

`cmd/json-convert` 不复用核心 decoder，也不向根包增加编码 API。它有一条独立、保真优先的数据流：

1. `main.go` 解析 CLI 参数，按输出格式选择严格 JSON 或扩展 JSON5 输入模式，协调读取、转换和原子文件替换。
2. `parser.go` 使用递归下降解析器构造专用有序语法树。对象使用 `[]member` 而不是 map，以保留成员顺序和重复键；数字保留原始 token；注释携带位置和成员归属信息。
3. JSON5 → JSON 时，`comments.go` 递归把对象成员的关联注释转换成紧邻原成员之前的 `<key>_hint` 成员；顶层注释和数组元素注释不保留。
4. `writer.go` 输出严格 JSON 或本项目 JSON5。严格 JSON 模式会规范化十六进制、前导 `+` 和省略整数/小数位等数字形式，并拒绝 `Infinity`、`NaN` 或无法输出为 JSON 的无效 UTF-8；JSON5 模式优先使用反引号值，内容含反引号时回退到标准双引号。

转换器的重要不变量是保留顺序、重复键和数字精度。不要用 `map[string]any` 或 `float64` 替换其专用语法树。输出只写指定文件，不支持 stdin/stdout 或原地覆盖；失败时必须保留已有输出。

## 测试组织

- 根包的 `*_test.go` 覆盖 scanner、反射解码、流式解码、数字、tag 和反引号扩展。
- `json5_test.go` 遍历 `testdata/`：`.json` 与标准库结果比较，`.json5` 与 Otto 的 ES5 求值结果比较，`.js`/`.txt` 验证拒绝路径。新增非标准扩展应写专门测试，而不是假设 Otto 接受它。
- `cmd/json-convert/*_test.go` 按 parser、writer、注释归属、CLI 与原子文件替换分层。转换语法或保真规则变化时，通常需要同时覆盖解析 token、最终输出和文件失败路径。
