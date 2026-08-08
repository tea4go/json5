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

func TestParseJSON5ObjectPreservesMembersAndLexemes(t *testing.T) {
	input := []byte("// top\r\n{unquoted:`raw\r\nbytes`,'single-key':'single\\nvalue',hex:-0xFF,plus:+12,leading:.5,trailing:1.,}")
	doc, err := parseDocument(input, modeJSON5)
	if err != nil {
		t.Fatalf("parseDocument returned error: %v", err)
	}
	if doc.kind != kindObject || len(doc.object) != 6 {
		t.Fatalf("document = kind %v with %d members, want object with 6", doc.kind, len(doc.object))
	}
	wantKeys := []string{"unquoted", "single-key", "hex", "plus", "leading", "trailing"}
	wantTexts := [][]byte{[]byte("raw\r\nbytes"), []byte("single\nvalue"), []byte("-0xFF"), []byte("+12"), []byte(".5"), []byte("1.")}
	for i := range wantKeys {
		if doc.object[i].key != wantKeys[i] {
			t.Errorf("member %d key = %q, want %q", i, doc.object[i].key, wantKeys[i])
		}
		if !bytes.Equal(doc.object[i].value.text, wantTexts[i]) {
			t.Errorf("member %d text = %q, want %q", i, doc.object[i].value.text, wantTexts[i])
		}
	}
}

func TestParseJSON5StringsEscapesContinuationsAndRawBytes(t *testing.T) {
	input := []byte("[\"double\\n\\u0041\\uD83D\\uDE00\\\ncontinued\",'single\\t\\u0042\\\r\ncontinued\\\rmore',`")
	input = append(input, 'a', 0x01, 0xff, '\r', '\n', 'b', '`', ']')
	doc, err := parseDocument(input, modeJSON5)
	if err != nil {
		t.Fatalf("parseDocument returned error: %v", err)
	}
	want := [][]byte{[]byte("double\nA😀continued"), []byte("single\tBcontinuedmore"), {'a', 0x01, 0xff, '\r', '\n', 'b'}}
	if len(doc.array) != len(want) {
		t.Fatalf("items = %d, want %d", len(doc.array), len(want))
	}
	for i := range want {
		if !bytes.Equal(doc.array[i].text, want[i]) {
			t.Errorf("item %d = %q, want %q", i, doc.array[i].text, want[i])
		}
	}
}

func TestParseJSON5ObjectKeys(t *testing.T) {
	doc, err := parseDocument([]byte(`{$a:1,_b2:2,A9$:3,'quoted-key':4,"double":5}`), modeJSON5)
	if err != nil {
		t.Fatalf("valid keys returned error: %v", err)
	}
	want := []string{"$a", "_b2", "A9$", "quoted-key", "double"}
	for i := range want {
		if doc.object[i].key != want[i] {
			t.Errorf("key %d = %q, want %q", i, doc.object[i].key, want[i])
		}
	}

	invalid := []string{"{`raw`:1}", "{9abc:1}", "{a-b:1}", "{é:1}"}
	for _, input := range invalid {
		_, err := parseDocument([]byte(input), modeJSON5)
		if err == nil || !strings.Contains(err.Error(), "object key") {
			t.Errorf("parseDocument(%q) error = %v, want object key error", input, err)
		}
	}
}

func TestParseJSON5NumbersPreserveTokens(t *testing.T) {
	valid := []string{"0", "0.5", "0e2", "0xFF", "+0x10", "-0Xf", "+12", "-12", ".5", "-.5", "+.5", "1.", "1.e2", "1E+2", "Infinity", "+Infinity", "-Infinity", "NaN"}
	for _, input := range valid {
		doc, err := parseDocument([]byte(input), modeJSON5)
		if err != nil {
			t.Errorf("parseDocument(%q) returned error: %v", input, err)
			continue
		}
		if doc.kind != kindNumber || string(doc.text) != input {
			t.Errorf("parseDocument(%q) = kind %v text %q", input, doc.kind, doc.text)
		}
	}

	invalid := []string{"01", "-01", "00", "0x", "+", ".", "1e", "1e+", "Infinityx", "NaNx", "0xFFz", "+true", "--1"}
	for _, input := range invalid {
		if _, err := parseDocument([]byte(input), modeJSON5); err == nil {
			t.Errorf("parseDocument(%q) succeeded, want error", input)
		}
	}
}

func TestParseJSON5CommentsCaptureRawPositionsAndObjectOwnership(t *testing.T) {
	input := []byte("// top\r\n{/* lead */a:1 /* trail */,\r\n// next\r\nb:2}")
	doc, err := parseDocument(input, modeJSON5)
	if err != nil {
		t.Fatalf("parseDocument returned error: %v", err)
	}
	if len(doc.object) != 2 {
		t.Fatalf("members = %d, want 2", len(doc.object))
	}
	lead := doc.object[0].leadingComments
	trail := doc.object[0].trailingComments
	next := doc.object[1].leadingComments
	if len(lead) != 1 || string(lead[0].raw) != "/* lead */" || lead[0].start.offset != 9 || lead[0].end.offset != 19 || lead[0].startLine != 2 || lead[0].endLine != 2 {
		t.Errorf("first leading comments = %+v", lead)
	}
	if len(trail) != 1 || string(trail[0].raw) != "/* trail */" || trail[0].startLine != 2 || trail[0].endLine != 2 {
		t.Errorf("first trailing comments = %+v", trail)
	}
	if len(next) != 1 || string(next[0].raw) != "// next" || next[0].startLine != 3 || next[0].endLine != 3 {
		t.Errorf("second leading comments = %+v", next)
	}
}

func TestParseJSON5CommentAfterCommaOnMemberLineIsTrailing(t *testing.T) {
	doc, err := parseDocument([]byte("{port:8080, /* c */ name:`demo`}"), modeJSON5)
	if err != nil {
		t.Fatalf("parseDocument returned error: %v", err)
	}
	port := doc.object[0]
	name := doc.object[1]
	if len(port.trailingComments) != 1 || string(port.trailingComments[0].raw) != "/* c */" {
		t.Fatalf("port trailing comments = %+v, want /* c */", port.trailingComments)
	}
	if len(name.leadingComments) != 0 {
		t.Fatalf("name leading comments = %+v, want none", name.leadingComments)
	}
}

func TestParseJSON5CommentAfterCommaOnNextLineIsLeading(t *testing.T) {
	doc, err := parseDocument([]byte("{port:8080,\n /* c */ name:`demo`}"), modeJSON5)
	if err != nil {
		t.Fatalf("parseDocument returned error: %v", err)
	}
	port := doc.object[0]
	name := doc.object[1]
	if len(port.trailingComments) != 0 {
		t.Fatalf("port trailing comments = %+v, want none", port.trailingComments)
	}
	if len(name.leadingComments) != 1 || string(name.leadingComments[0].raw) != "/* c */" {
		t.Fatalf("name leading comments = %+v, want /* c */", name.leadingComments)
	}
}

func TestParseWhitespaceFormFeedByMode(t *testing.T) {
	if _, err := parseDocument([]byte("{a:\f1}"), modeJSON5); err != nil {
		t.Fatalf("JSON5 form feed returned error: %v", err)
	}
	if _, err := parseDocument([]byte("{\"a\":\f1}"), modeJSON); err == nil {
		t.Fatal("JSON form feed succeeded, want error")
	}
}

func TestParseJSON5ReportsUnterminatedTokensAtPosition(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		message string
	}{
		{"raw", []byte("\r\n`abc"), "line 2, column 5: unterminated raw string"},
		{"single", []byte("\r\n'abc"), "line 2, column 5: unterminated string"},
		{"double", []byte("\r\n\"abc"), "line 2, column 5: unterminated string"},
		{"block comment", []byte("\r\n/* abc"), "line 2, column 7: unterminated block comment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDocument(tt.data, modeJSON5)
			if err == nil || err.Error() != tt.message {
				t.Fatalf("error = %v, want %q", err, tt.message)
			}
		})
	}
}

func TestParseJSON5RejectsUnescapedNewlinesInQuotedStrings(t *testing.T) {
	for _, input := range [][]byte{[]byte("'a\nb'"), []byte("\"a\rb\"")} {
		if _, err := parseDocument(input, modeJSON5); err == nil {
			t.Errorf("parseDocument(%q) succeeded, want error", input)
		}
	}
}
