package json5

import (
	"bytes"
	"strings"
	"testing"
)

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

func TestCheckValidBacktickStringValues(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "top level", data: []byte("`raw string`")},
		{name: "object value", data: []byte("{key:`raw string`}")},
		{name: "array value", data: []byte("[`raw string`]")},
		{name: "newlines", data: []byte("`CRLF\r\nLF\nCR\r`")},
		{name: "control bytes", data: []byte{'`', 0x00, 0x01, 0x1f, '`'}},
		{name: "byte ff", data: []byte{'`', 0xff, '`'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkValid(tt.data, new(scanner)); err != nil {
				t.Fatalf("checkValid(%q) returned error: %v", tt.data, err)
			}
		})
	}
}

func TestCheckValidRejectsBacktickObjectKey(t *testing.T) {
	data := []byte("{`key`:1}")
	err := checkValid(data, new(scanner))
	if err == nil {
		t.Fatal("checkValid accepted a backtick object key")
	}

	syntaxErr, ok := err.(*SyntaxError)
	if !ok {
		t.Fatalf("error type = %T, want *SyntaxError", err)
	}
	if syntaxErr.msg != "invalid character '`' looking for beginning of object key" {
		t.Errorf("error message = %q, want %q", syntaxErr.msg, "invalid character '`' looking for beginning of object key")
	}
	if syntaxErr.Offset != 2 {
		t.Errorf("error offset = %d, want 2", syntaxErr.Offset)
	}
}

func TestCheckValidRejectsUnclosedBacktickStringAtEOF(t *testing.T) {
	data := []byte("`unclosed")
	err := checkValid(data, new(scanner))
	if err == nil {
		t.Fatal("checkValid accepted an unclosed backtick string")
	}

	syntaxErr, ok := err.(*SyntaxError)
	if !ok {
		t.Fatalf("error type = %T, want *SyntaxError", err)
	}
	if syntaxErr.msg != "unexpected end of JSON input" {
		t.Errorf("error message = %q, want %q", syntaxErr.msg, "unexpected end of JSON input")
	}
	if syntaxErr.Offset != int64(len(data)) {
		t.Errorf("error offset = %d, want %d", syntaxErr.Offset, len(data))
	}
}

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

func TestBacktickStringUnmarshalInterfaces(t *testing.T) {
	input := []byte("`line 1\r\n\\n'\"line 2`")
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

func TestBacktickStringFirstBacktickTerminatesString(t *testing.T) {
	input := []byte("{value:`a`b}")
	err := checkValid(input, new(scanner))
	if err == nil {
		t.Fatal("checkValid accepted content after the closing backtick")
	}

	syntaxErr, ok := err.(*SyntaxError)
	if !ok {
		t.Fatalf("error type = %T, want *SyntaxError", err)
	}
	want := "invalid character 'b' after object key:value pair"
	if syntaxErr.msg != want {
		t.Fatalf("error message = %q, want %q", syntaxErr.msg, want)
	}
}

func TestUnmarshalBacktickStringValuePositions(t *testing.T) {
	const want = "raw string"
	tests := []struct {
		name   string
		input  string
		decode func([]byte) (string, error)
	}{
		{
			name:  "top level",
			input: "`raw string`",
			decode: func(data []byte) (string, error) {
				var got string
				err := Unmarshal(data, &got)
				return got, err
			},
		},
		{
			name:  "object value",
			input: "{value:`raw string`}",
			decode: func(data []byte) (string, error) {
				var got struct {
					Value string
				}
				err := Unmarshal(data, &got)
				return got.Value, err
			},
		},
		{
			name:  "array element",
			input: "[`raw string`]",
			decode: func(data []byte) (string, error) {
				var got []string
				err := Unmarshal(data, &got)
				if err != nil || len(got) != 1 {
					return "", err
				}
				return got[0], nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.decode([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("decoded value = %q, want %q", got, want)
			}
		})
	}
}

func TestUnmarshalBacktickStringPreservesEveryByte(t *testing.T) {
	content := []byte{
		'\'', '"', '\\', 'n', '\\', 't', '\\', 'u', '4', 'e', '2', 'd', '\\', '\\',
		'\n', '\n', ' ', '\t', 'x', '\r', '\n', '\r',
		0x00, 0x01, 0x1f, 0x7f, 0xff,
	}
	input := append([]byte{'`'}, content...)
	input = append(input, '`')

	var got string
	if err := Unmarshal(input, &got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(got), content) {
		t.Fatalf("decoded bytes = %v, want %v", []byte(got), content)
	}
}
