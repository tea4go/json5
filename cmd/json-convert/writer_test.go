package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	json5 "github.com/titanous/json5"
)

func TestWriteJSON5PreservesStringsMembersAndNumberLexemes(t *testing.T) {
	input := []byte(`{"z":"line\n\"double\" and 'single'","a":"has ` + "`" + ` tick","z":1.2300}`)
	value, err := parseDocument(input, modeJSON)
	if err != nil {
		t.Fatal(err)
	}

	got, err := writeDocument(value, outputJSON5, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n" +
		"  \"z\": `line\n\"double\" and 'single'`,\n" +
		"  \"a\": \"has ` tick\",\n" +
		"  \"z\": 1.2300\n" +
		"}\n"
	if !bytes.Equal(got, []byte(want)) {
		t.Fatalf("writeDocument output = %q, want %q", got, want)
	}
}

func TestWriteJSON5CompactObjectsArraysAndEmptyStructures(t *testing.T) {
	value, err := parseDocument([]byte(`{"object":{"items":[1,"x"]},"emptyObject":{},"emptyArray":[]}`), modeJSON)
	if err != nil {
		t.Fatal(err)
	}

	got, err := writeDocument(value, outputJSON5, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"object\":{\"items\":[1,`x`]},\"emptyObject\":{},\"emptyArray\":[]}\n"
	if string(got) != want {
		t.Fatalf("writeDocument output = %q, want %q", got, want)
	}
}

func TestWriteJSON5IndentationOneThroughEight(t *testing.T) {
	value, err := parseDocument([]byte(`{"nested":[{"raw":"left\n   middle\n\tright"},[],{}]}`), modeJSON)
	if err != nil {
		t.Fatal(err)
	}

	for indent := 1; indent <= 8; indent++ {
		t.Run(strings.Repeat("space", indent), func(t *testing.T) {
			got, err := writeDocument(value, outputJSON5, indent)
			if err != nil {
				t.Fatal(err)
			}
			s1 := strings.Repeat(" ", indent)
			s2 := strings.Repeat(" ", indent*2)
			s3 := strings.Repeat(" ", indent*3)
			want := "{\n" + s1 + "\"nested\": [\n" + s2 + "{\n" + s3 +
				"\"raw\": `left\n   middle\n\tright`\n" + s2 + "},\n" + s2 + "[],\n" + s2 + "{}\n" + s1 + "]\n}\n"
			if string(got) != want {
				t.Fatalf("writeDocument output = %q, want %q", got, want)
			}
		})
	}
}

func TestWriteJSON5RoundTrip(t *testing.T) {
	value, err := parseDocument([]byte(`{"message":"line\n\"quoted\"","array":[true,false,null,12.50],"object":{"x":"y"}}`), modeJSON)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := writeDocument(value, outputJSON5, 2)
	if err != nil {
		t.Fatal(err)
	}

	var got struct {
		Message string
		Array   []interface{}
		Object  map[string]string
	}
	if err := json5.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json5.Unmarshal(%q): %v", encoded, err)
	}
	if got.Message != "line\n\"quoted\"" || len(got.Array) != 4 || got.Array[0] != true || got.Array[1] != false || got.Array[2] != nil || got.Array[3] != 12.5 || got.Object["x"] != "y" {
		t.Fatalf("round-trip value = %#v", got)
	}
}

func TestWriteJSON5RawStringRoundTripsControlAndInvalidUTF8Bytes(t *testing.T) {
	text := []byte{'a', 0x00, 0x1f, 0xff, '\n', 'b'}
	value := value{kind: kindString, text: text, pos: position{line: 3, column: 5}}
	encoded, err := writeDocument(value, outputJSON5, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{'`'}, text...)
	want = append(want, '`', '\n')
	if !bytes.Equal(encoded, want) {
		t.Fatalf("writeDocument output = %q, want %q", encoded, want)
	}
	var decoded string
	if err := json5.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json5.Unmarshal(%q): %v", encoded, err)
	}
	if !bytes.Equal([]byte(decoded), text) {
		t.Fatalf("round-trip bytes = %q, want %q", []byte(decoded), text)
	}
}

func TestWriteJSON5QuotesBacktickStringsWithStandardJSONEscapes(t *testing.T) {
	value := value{
		kind: kindObject,
		object: []member{{
			key:   "control\x00key",
			value: value{kind: kindString, text: []byte{'a', '`', 0x00, 0x1f}},
		}},
	}
	encoded, err := writeDocument(value, outputJSON5, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\"control\\u0000key\":\"a`\\u0000\\u001f\"}\n"
	if string(encoded) != want {
		t.Fatalf("writeDocument output = %q, want %q", encoded, want)
	}
}

func TestWriteJSON5RejectsInvalidUTF8WhenBacktickForcesQuotedString(t *testing.T) {
	value := value{kind: kindString, text: []byte{'a', '`', 0xff}, pos: position{line: 4, column: 7}}
	_, err := writeDocument(value, outputJSON5, 0)
	if err == nil || !strings.Contains(err.Error(), "line 4, column 7") || !strings.Contains(err.Error(), "invalid UTF-8") {
		t.Fatalf("writeDocument error = %v, want positioned invalid UTF-8 error", err)
	}
}

func TestWriteJSONBasicValues(t *testing.T) {
	for _, input := range []string{`null`, `true`, `false`, `12.50`, `"line\nquoted"`} {
		value, err := parseDocument([]byte(input), modeJSON)
		if err != nil {
			t.Fatal(err)
		}
		got, err := writeDocument(value, outputJSON, 2)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != input+"\n" {
			t.Errorf("writeDocument(%s) = %q", input, got)
		}
	}
}

func TestWriteJSONNormalizesJSON5NumbersWithoutLosingPrecision(t *testing.T) {
	const hugeHex = "0x123456789abcdef0123456789abcdef"
	tests := []struct {
		input string
		want  string
	}{
		{"0xFF", "255"},
		{"+0Xff", "255"},
		{"-0xFF", "-255"},
		{hugeHex, "1512366075204170929049582354406559215"},
		{"+12", "12"},
		{".5", "0.5"},
		{"+.5", "0.5"},
		{"-.5", "-0.5"},
		{"1.", "1.0"},
		{"+1.", "1.0"},
		{"1.e2", "1.0e2"},
		{"1.2300", "1.2300"},
		{"1E+6", "1E+6"},
		{"-0", "-0"},
		{"-0x0", "-0"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			value, err := parseDocument([]byte(tt.input), modeJSON5)
			if err != nil {
				t.Fatal(err)
			}
			got, err := writeDocument(value, outputJSON, 0)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want+"\n" {
				t.Fatalf("writeDocument(%s) = %q, want %q", tt.input, got, tt.want+"\n")
			}
			if !json.Valid(bytes.TrimSuffix(got, []byte("\n"))) {
				t.Fatalf("writeDocument(%s) produced invalid JSON: %q", tt.input, got)
			}
		})
	}
}

func TestWriteJSONRejectsNonFiniteNumbersWithPosition(t *testing.T) {
	for _, input := range []string{"Infinity", "+Infinity", "-Infinity", "NaN", "+NaN", "-NaN"} {
		t.Run(input, func(t *testing.T) {
			value := value{kind: kindNumber, text: []byte(input), pos: position{line: 2, column: 3}}
			_, err := writeDocument(value, outputJSON, 2)
			if err == nil || !strings.Contains(err.Error(), "line 2, column 3") || !strings.Contains(err.Error(), input) {
				t.Fatalf("writeDocument error = %v, want positioned error containing %q", err, input)
			}
		})
	}
}

func TestWriteJSONRejectsInvalidUTF8InStringsKeysAndHints(t *testing.T) {
	tests := []struct {
		name  string
		value value
		pos   string
	}{
		{"string", value{kind: kindString, text: []byte{0xff}, pos: position{line: 2, column: 4}}, "line 2, column 4"},
		{"key", value{kind: kindObject, object: []member{{key: string([]byte{0xff}), keyPos: position{line: 3, column: 6}, value: value{kind: kindNull}}}}, "line 3, column 6"},
		{"hint", value{kind: kindObject, object: []member{{key: "name_hint", keyPos: position{line: 5, column: 2}, value: value{kind: kindString, text: []byte{0xff}, pos: position{line: 5, column: 2}}}}}, "line 5, column 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := writeDocument(tt.value, outputJSON, 0)
			if err == nil || !strings.Contains(err.Error(), tt.pos) || !strings.Contains(err.Error(), "invalid UTF-8") {
				t.Fatalf("writeDocument error = %v, want %s invalid UTF-8 error", err, tt.pos)
			}
		})
	}
}

func TestWriteJSONUsesStandardQuotingPreservesDuplicateKeysAndSupportsIndent(t *testing.T) {
	value, err := parseDocument([]byte(`{"x":"line\n\"quoted\"\\slash\t","x":""}`), modeJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		indent int
		want   string
	}{
		{0, "{\"x\":\"line\\n\\\"quoted\\\"\\\\slash\\t\",\"x\":\"\"}\n"},
		{2, "{\n  \"x\": \"line\\n\\\"quoted\\\"\\\\slash\\t\",\n  \"x\": \"\"\n}\n"},
	} {
		got, err := writeDocument(value, outputJSON, tt.indent)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tt.want {
			t.Fatalf("indent %d output = %q, want %q", tt.indent, got, tt.want)
		}
		if !json.Valid(bytes.TrimSuffix(got, []byte("\n"))) {
			t.Fatalf("indent %d produced invalid JSON: %q", tt.indent, got)
		}
		if bytes.Count(got, []byte(`"x"`)) != 2 {
			t.Fatalf("indent %d did not preserve duplicate keys: %q", tt.indent, got)
		}
	}
}
