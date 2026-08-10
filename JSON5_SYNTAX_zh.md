# JSON5 语法说明

本文说明本仓库中 `json5` 解码器与 `json-convert` 工具涉及的 JSON5 格式语法。为避免歧义，文中会区分：

- 根包 `github.com/tea4go/json5` 的解码能力；
- `cmd/json-convert` 在 `--out json` 时接受的 JSON5 子集；
- 本仓库额外支持的非标准反引号 raw string 扩展。

> [!IMPORTANT]
> 反引号 raw string 是本仓库私有扩展，不属于标准 JSON5。和其他实现交换数据时，请优先使用标准字符串或直接输出严格 JSON。

## 1. 顶层结构

JSON5 文档的顶层仍然是一个值：

```text
document = ws value ws
```

这里的 `value` 可以是：

- 对象 `object`
- 数组 `array`
- 字符串 `string`
- 数字 `number`
- `true`
- `false`
- `null`

根值之后不允许再有额外内容。

## 2. 空白与注释

本仓库支持的常见空白包括：

- 空格
- Tab
- LF
- CR
- Form Feed

支持两类注释：

```javascript
// 行注释

/* 块注释 */
```

示例：

```javascript
{
  // 服务名
  name: 'demo',
  /* 监听端口 */
  port: 8080,
}
```

## 3. 对象

对象语法与 JSON 类似，但更宽松：

```text
object = "{" [ member *( "," member ) [ "," ] ] "}"
member = key ":" value
```

支持以下对象键：

- 双引号字符串键：`"name"`
- 单引号字符串键：`'name'`
- 未加引号键：`name`

示例：

```javascript
{
  "double": 1,
  'single': 2,
  unquoted: 3,
}
```

说明：

- 允许尾随逗号；
- 根包解码器支持 JSON5 风格未加引号键；
- `json-convert` 对未加引号键的支持更严格，只接受以 ASCII 字母、`_`、`$` 开头，后续可跟数字的键；
- 反引号 raw string 不能作为对象键。

## 4. 数组

数组同样允许尾随逗号：

```text
array = "[" [ value *( "," value ) [ "," ] ] "]"
```

示例：

```javascript
[
  1,
  2,
  3,
]
```

## 5. 字符串

### 5.1 标准 JSON5 字符串

本仓库支持：

- 双引号字符串
- 单引号字符串

示例：

```javascript
"hello"
'world'
'single\'quote'
"double\"quote"
```

双引号和单引号字符串支持常见转义，并支持反斜杠续行：

```javascript
'line one\
line two'
```

这类续行不会把换行本身保留到结果里。

### 5.2 反引号 raw string 扩展

本仓库额外支持反引号包裹的原始字符串：

```javascript
{
  script: `printf "line one\n"
C:\tmp\line-two`
}
```

规则如下：

- 只允许作为值，不允许作为对象键；
- 开始反引号之后遇到的第一个反引号就是结束符；
- 内容中不能再出现反引号；
- 没有任何转义语义；
- 引号、反斜杠、控制字节、实际换行和无效 UTF-8 都按原字节保留。

因此下面两个值含义不同：

```javascript
{
  escaped: "a\nb",
  raw: `a\nb`,
}
```

- `escaped` 解码后包含实际换行；
- `raw` 解码后包含两个普通字符：反斜杠和 `n`。

## 6. 数字

本仓库支持的 JSON5 数字形式包括：

- 普通十进制整数：`0`、`12`、`-12`
- 带前导 `+`：`+12`
- 小数：`0.5`、`12.34`
- 省略整数位的小数：`.5`、`-.5`
- 尾随小数点：`1.`、`+1.`
- 指数：`1e3`、`1E+3`、`-1.e1`
- 十六进制：`0xFF`、`-0x2a`、`+0X10`
- 非有限数：`Infinity`、`+Infinity`、`-Infinity`、`NaN`

示例：

```javascript
{
  dec: 12,
  plus: +12,
  leading: .5,
  trailing: 1.,
  exp: -1.e1,
  hex: 0xFF,
  inf: Infinity,
  nan: NaN,
}
```

限制：

- 不允许前导零十进制，如 `01`；
- `+NaN` 和 `-NaN` 不属于本仓库接受的数字形式；
- 不完整写法如 `1e`、`0x`、`.` 都会报错。

## 7. 字面量

支持标准字面量：

```javascript
true
false
null
```

## 8. 一个完整示例

```javascript
{
  // 服务配置
  name: 'demo',
  enabled: true,
  ratio: .5,
  timeout: 1.,
  limit: 0x20,
  note: `line one
line two`,
  tags: [
    'a',
    "b",
  ],
}
```

## 9. `json-convert` 的 JSON5 子集

`cmd/json-convert` 在 `--out json` 时不会复用根包解码器，而是使用自己的解析器。因此它的 JSON5 支持范围需要单独看待。

它支持：

- 注释；
- 单引号、双引号字符串；
- 反引号 raw string；
- 双引号键、单引号键、部分未加引号键；
- 尾随逗号；
- 十六进制、前导 `+`、`.5`、`1.`、`Infinity`、`NaN`；
- 对象成员顺序与重复键保留。

它额外有这些明确限制：

- 未加引号键只接受 ASCII 标识符子集；
- 不接受反引号对象键；
- 输出严格 JSON 时，`Infinity`、`NaN` 不能被写出，会直接报错；
- raw string 若含无效 UTF-8，也无法写成严格 JSON。

## 10. 与严格 JSON 的主要差异

相对严格 JSON，本仓库支持这些扩展：

- 注释；
- 单引号字符串；
- 未加引号对象键；
- 尾随逗号；
- 更宽松的数字形式；
- `Infinity` 和 `NaN`；
- 非标准反引号 raw string。

## 11. 兼容性建议

如果你希望数据能被大多数 JSON 工具接受，建议遵循下面的最小兼容集合：

- 只使用双引号字符串和双引号对象键；
- 不写注释；
- 不使用尾随逗号；
- 不使用十六进制、`.5`、`1.`、`Infinity`、`NaN`；
- 不使用反引号 raw string。

需要和其他系统交换数据时，可以使用：

```bash
go run ./cmd/json-convert --out json input.json5 output.json
```

把 JSON5 转成严格 JSON。
