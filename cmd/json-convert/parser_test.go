package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestParseJSONPreservesObjectOrderDuplicatesAndLexemes(t *testing.T) {
	doc, err := parseDocument([]byte(`{"z":1.2300,"a":123456789012345678901234567890,"z":"line\n\"q\""}`), modeJSON)
	if err != nil {
		t.Fatalf("parseDocument returned error: %v", err)
	}
	if doc.kind != kindObject {
		t.Fatalf("kind = %v, want object", doc.kind)
	}
	if len(doc.object) != 3 {
		t.Fatalf("members = %d, want 3", len(doc.object))
	}
	for i, want := range []string{"z", "a", "z"} {
		if doc.object[i].key != want {
			t.Errorf("member %d key = %q, want %q", i, doc.object[i].key, want)
		}
	}
	if got := doc.object[0].value.text; !bytes.Equal(got, []byte("1.2300")) {
		t.Errorf("decimal text = %q, want %q", got, "1.2300")
	}
	if got := doc.object[1].value.text; !bytes.Equal(got, []byte("123456789012345678901234567890")) {
		t.Errorf("integer text = %q", got)
	}
	if got := doc.object[2].value.text; !bytes.Equal(got, []byte("line\n\"q\"")) {
		t.Errorf("decoded string = %q", got)
	}
}

func TestParseJSONRejectsJSON5Syntax(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"unquoted key", `{a:1}`},
		{"single quoted value", `{"a":'x'}`},
		{"backtick value", "{\"a\":`x`}"},
		{"trailing object comma", `{"a":1,}`},
		{"trailing array comma", `[1,]`},
		{"leading plus", `+1`},
		{"leading decimal point", `.5`},
		{"line comment", "// no\n1"},
		{"block comment", `/* no */1`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseDocument([]byte(tt.data), modeJSON); err == nil {
				t.Fatalf("parseDocument(%q) succeeded, want error", tt.data)
			}
		})
	}
}

func TestParseJSONReportsLineAndColumn(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{"LF", "{\n  \"a\":1,\n  \"b\": @\n}", "line 3, column 8: expected value"},
		{"CRLF", "{\r\n  \"a\":1,\r\n  \"b\": @\r\n}", "line 3, column 8: expected value"},
		{"CR", "{\r  \"a\":1,\r  \"b\": @\r}", "line 3, column 8: expected value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDocument([]byte(tt.data), modeJSON)
			if err == nil {
				t.Fatal("parseDocument succeeded, want error")
			}
			var parseErr *parseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("error type = %T, want *parseError", err)
			}
			if got := err.Error(); got != tt.want {
				t.Fatalf("error = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJSONDecodesUnicodeEscapes(t *testing.T) {
	input := []byte{'"', '\\', 'u', '0', '0', '4', '1', '=', 'A', ' ', 's', 'm', 'i', 'l', 'e', '=', '\\', 'u', 'D', '8', '3', 'D', '\\', 'u', 'D', 'E', '0', '0', '"'}
	doc, err := parseDocument(input, modeJSON)
	if err != nil {
		t.Fatalf("parseDocument returned error: %v", err)
	}
	if got, want := doc.text, []byte("A=A smile=😀"); !bytes.Equal(got, want) {
		t.Fatalf("decoded string = %q, want %q", got, want)
	}
}

func TestParseJSONRejectsInvalidStrings(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"invalid escape", []byte(`"\x20"`)},
		{"raw control", []byte{'"', 'a', 0x01, '"'}},
		{"lone high surrogate", []byte(`"\uD800"`)},
		{"lone low surrogate", []byte(`"\uDC00"`)},
		{"high followed by non-low", []byte(`"\uD800A"`)},
		{"invalid utf8", []byte{'"', 0xff, '"'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseDocument(tt.data, modeJSON); err == nil {
				t.Fatal("parseDocument succeeded, want error")
			}
		})
	}
}

func TestParseJSONNumberGrammar(t *testing.T) {
	valid := []string{"0", "-0", "1", "-12", "1.0", "1e2", "1E+2", "1e-2", "1e400"}
	for _, input := range valid {
		t.Run("valid_"+input, func(t *testing.T) {
			doc, err := parseDocument([]byte(input), modeJSON)
			if err != nil {
				t.Fatalf("parseDocument(%q) returned error: %v", input, err)
			}
			if doc.kind != kindNumber || string(doc.text) != input {
				t.Fatalf("number = kind %v text %q", doc.kind, doc.text)
			}
		})
	}

	invalid := []string{"01", "-01", "1.", "1e", "1e+", "--1", "1 2", "true false", "nullx", "[]x"}
	for _, input := range invalid {
		name := strings.NewReplacer(" ", "_", "/", "_").Replace(input)
		t.Run("invalid_"+name, func(t *testing.T) {
			if _, err := parseDocument([]byte(input), modeJSON); err == nil {
				t.Fatalf("parseDocument(%q) succeeded, want error", input)
			}
		})
	}
}
