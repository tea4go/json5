# json5 [![GoDoc](https://godoc.org/github.com/tea4go/json5?status.svg)](https://godoc.org/github.com/tea4go/json5) [![Build Status](https://github.com/tea4go/json5/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/tea4go/json5/actions/workflows/ci.yml)

这是一个用于实现 [JSON5](https://github.com/json5/json5) 解码的 Go 包。用法请参阅[文档](https://godoc.org/github.com/tea4go/json5)。

## 反引号原始字符串

这个分支支持以反引号包裹的原始字符串，作为一种非标准的 JSON5 扩展。它们只能作为值使用，不能作为对象键使用。分隔符之间的每一个字节都会被原样保留：引号、反斜杠、类似转义的序列、控制字符和换行都不会被解释或规范化。

开头分隔符之后遇到的第一个反引号就会结束字符串，因此内容中不能包含反引号。比如，shell 变量可以在不转义原始字符串值的情况下保存一个多行 JSON5 文档：

```sh
payload='{
  command: `printf "line one\n"
C:\tmp\line-two`
}'
```

其他 JSON5 实现可能会拒绝这种扩展。

## JSON 转换命令

将 JSON5 转换为标准 JSON：

```sh
go run ./cmd/json-convert --out json input.json5 output.json
```

将标准 JSON 转换为 JSON5：

```sh
go run ./cmd/json-convert --out json5 input.json output.json5
```

从 JSON5 生成 Go struct 定义：

```sh
go run ./cmd/json-convert --out golang config.json5 config.go
```

当 `OUTPUT` 省略或显式传入空字符串时，`json-convert` 会根据 `--out`
自动生成 `<input>_convert.json`、`<input>_convert.json5` 或
`<input>_convert.go`。对 JSON / JSON5 输出可使用 `--indent 0`
生成紧凑格式。转换过程会保留对象成员顺序和重复键。将 JSON5 转换为
JSON 时，附着在对象成员上的注释会变成位于其前面的 `<name>_hint` 成员。
