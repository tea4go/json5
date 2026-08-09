package main

import (
	goparser "go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateGoDefinitionsFormatsRootArray(t *testing.T) {
	value, err := parseDocument([]byte(`[{name:"a"},{name:"b"}]`), modeJSON5)
	if err != nil {
		t.Fatal(err)
	}

	got, err := generateGoDefinitions(value, "demo", "items")
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if !strings.Contains(text, "package demo") {
		t.Fatalf("output = %q, want package clause", text)
	}
	if !strings.Contains(text, "type Items []ItemsItem") {
		t.Fatalf("output = %q, want root array type", text)
	}
	if !strings.Contains(text, "type ItemsItem struct") {
		t.Fatalf("output = %q, want item struct type", text)
	}
	if _, err := goparser.ParseFile(token.NewFileSet(), "generated.go", got, goparser.AllErrors); err != nil {
		t.Fatalf("generated Go does not parse: %v\n%s", err, text)
	}
}

func TestGenerateGoDefinitionsMergesMixedArrayNumbersToFloat(t *testing.T) {
	value, err := parseDocument([]byte(`{values:[1,2.5]}`), modeJSON5)
	if err != nil {
		t.Fatal(err)
	}

	got, err := generateGoDefinitions(value, "demo", "config")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Values []float64 `json:\"values\"`") {
		t.Fatalf("output = %q, want float slice", got)
	}
}
