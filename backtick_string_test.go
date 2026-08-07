package json5

import "testing"

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
