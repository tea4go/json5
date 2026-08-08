package main

import (
	"reflect"
	"testing"
)

func TestCleanComment(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"line", "// 服务名称", "服务名称"},
		{"one line block", "/* one line */", "one line"},
		{"star block LF", "/*\n * first\n * second\n */", "first\nsecond"},
		{"star block CRLF", "/*\r\n\t* first\r\n\t* second\r\n\t*/", "first\nsecond"},
		{"star block CR", "/*\r  * first\r  * second\r  */", "first\nsecond"},
		{"common indentation", "/*\n    first\n      second\n    third\n*/", "first\n  second\nthird"},
		{"trim blank edges keep internal newline", "/*\n\n  first\n\n  second\n\n*/", "first\n\nsecond"},
		{"empty", "/**/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanComment([]byte(tt.raw)); got != tt.want {
				t.Fatalf("cleanComment(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestAddHintMembersPreservesOrderDuplicatesAndRecurses(t *testing.T) {
	input := []byte(`{
// first

/* second */
name: "demo", // trailing
name_hint: "existing",
// duplicate
name: "again",
nested: {/* child */ enabled: true},
items: [
  {/* array child */ port: 1},
  // array comment ignored
  2
]
}`)
	doc, err := parseDocument(input, modeJSON5)
	if err != nil {
		t.Fatal(err)
	}

	got := addHintMembers(doc)
	if keys := memberKeys(got); !reflect.DeepEqual(keys, []string{"name_hint", "name", "name_hint", "name_hint", "name", "nested", "items"}) {
		t.Fatalf("top-level keys = %#v", keys)
	}
	if text := string(got.object[0].value.text); text != "first\nsecond\ntrailing" {
		t.Fatalf("first hint = %q", text)
	}
	if text := string(got.object[3].value.text); text != "duplicate" {
		t.Fatalf("duplicate hint = %q", text)
	}
	if keys := memberKeys(got.object[5].value); !reflect.DeepEqual(keys, []string{"enabled_hint", "enabled"}) {
		t.Fatalf("nested keys = %#v", keys)
	}
	if keys := memberKeys(got.object[6].value.array[0]); !reflect.DeepEqual(keys, []string{"port_hint", "port"}) {
		t.Fatalf("array object keys = %#v", keys)
	}
	if len(got.object[6].value.array) != 2 {
		t.Fatalf("array length = %d", len(got.object[6].value.array))
	}
}

func TestAddHintMembersSkipsEmptyCommentsAndDoesNotHintTopLevelOrArrayComments(t *testing.T) {
	doc, err := parseDocument([]byte("// top\n[/* array */{/* */ value:1}]"), modeJSON5)
	if err != nil {
		t.Fatal(err)
	}
	got := addHintMembers(doc)
	if got.kind != kindArray || len(got.array) != 1 {
		t.Fatalf("result = %#v", got)
	}
	if keys := memberKeys(got.array[0]); !reflect.DeepEqual(keys, []string{"value"}) {
		t.Fatalf("keys = %#v", keys)
	}
}

func TestAddHintMembersHintsExistingHintKey(t *testing.T) {
	doc, err := parseDocument([]byte("{/* describe hint */ name_hint:'old'}"), modeJSON5)
	if err != nil {
		t.Fatal(err)
	}
	got := addHintMembers(doc)
	if keys := memberKeys(got); !reflect.DeepEqual(keys, []string{"name_hint_hint", "name_hint"}) {
		t.Fatalf("keys = %#v", keys)
	}
}

func TestCommentOwnershipAcrossValueAndCommaLines(t *testing.T) {
	doc, err := parseDocument([]byte("{\n"+
		"raw:`first\nsecond` /* before comma */, // after comma\n"+
		"/* next leading */ next:2,\n"+
		"port:1\n"+
		", /* comma line */ host:3,\n"+
		"same:4, /* same line wins */ other:5\n"+
		"}"), modeJSON5)
	if err != nil {
		t.Fatal(err)
	}
	got := addHintMembers(doc)
	wantKeys := []string{"raw_hint", "raw", "next_hint", "next", "port_hint", "port", "host", "same_hint", "same", "other"}
	if keys := memberKeys(got); !reflect.DeepEqual(keys, wantKeys) {
		t.Fatalf("keys = %#v, want %#v", keys, wantKeys)
	}
	wantHints := map[int]string{
		0: "before comma\nafter comma",
		2: "next leading",
		4: "comma line",
		7: "same line wins",
	}
	for index, want := range wantHints {
		if text := string(got.object[index].value.text); text != want {
			t.Errorf("hint at %d = %q, want %q", index, text, want)
		}
	}
}

func TestAddHintMembersUsesTrailingCommentsAfterMultilineCompositeValues(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		field     string
		wantHint  string
		wantValue string
	}{
		{
			name:      "object value",
			input:     "{\nconfig: {\n  enabled: true\n}, // object hint\nafter: 1\n}",
			field:     "config",
			wantHint:  "object hint",
			wantValue: "config_hint",
		},
		{
			name:      "array value",
			input:     "{\nitems: [\n  1,\n  2\n], // array hint\nafter: 1\n}",
			field:     "items",
			wantHint:  "array hint",
			wantValue: "items_hint",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parseDocument([]byte(tt.input), modeJSON5)
			if err != nil {
				t.Fatal(err)
			}
			got := addHintMembers(doc)
			if keys := memberKeys(got); !reflect.DeepEqual(keys, []string{tt.wantValue, tt.field, "after"}) {
				t.Fatalf("keys = %#v, want [%s %s after]", keys, tt.wantValue, tt.field)
			}
			if text := string(got.object[0].value.text); text != tt.wantHint {
				t.Fatalf("hint = %q, want %q", text, tt.wantHint)
			}
		})
	}
}

func TestObjectEndCommentOwnership(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantKeys []string
		wantHint string
	}{
		{"detached without comma is discarded", "{last:1\n// object tail\n}", []string{"last"}, ""},
		{"detached after trailing comma is discarded", "{last:1,\n// object tail\n}", []string{"last"}, ""},
		{"same line without comma is trailing", "{last:1 /* trailing */\n}", []string{"last_hint", "last"}, "trailing"},
		{"same line after comma is trailing", "{last:1, // trailing\n}", []string{"last_hint", "last"}, "trailing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parseDocument([]byte(tt.input), modeJSON5)
			if err != nil {
				t.Fatal(err)
			}
			got := addHintMembers(doc)
			if keys := memberKeys(got); !reflect.DeepEqual(keys, tt.wantKeys) {
				t.Fatalf("keys = %#v, want %#v", keys, tt.wantKeys)
			}
			if tt.wantHint != "" && string(got.object[0].value.text) != tt.wantHint {
				t.Fatalf("hint = %q, want %q", got.object[0].value.text, tt.wantHint)
			}
		})
	}
}

func TestArrayEndAndDocumentEndCommentsAreRemoved(t *testing.T) {
	doc, err := parseDocument([]byte("// top\n[{value:1}, // array end\n]\n// document end"), modeJSON5)
	if err != nil {
		t.Fatal(err)
	}
	got := addHintMembers(doc)
	if got.kind != kindArray || len(got.array) != 1 {
		t.Fatalf("result = %#v", got)
	}
	if keys := memberKeys(got.array[0]); !reflect.DeepEqual(keys, []string{"value"}) {
		t.Fatalf("keys = %#v, want [value]", keys)
	}
}

func memberKeys(v value) []string {
	keys := make([]string, len(v.object))
	for i := range v.object {
		keys[i] = v.object[i].key
	}
	return keys
}
