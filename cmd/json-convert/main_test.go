package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseOptions(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want options
	}{
		{"json defaults", []string{"--out", "json", "in.json5", "out.json"}, options{out: outputJSON, indent: 2, input: "in.json5", output: "out.json"}},
		{"json5 and indent", []string{"--indent", "0", "--out=json5", "in.json", "out.json5"}, options{out: outputJSON5, indent: 0, input: "in.json", output: "out.json5"}},
		{"flags reversed", []string{"--out", "json5", "--indent", "8", "in.json", "out.json5"}, options{out: outputJSON5, indent: 8, input: "in.json", output: "out.json5"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			got, err := parseOptions(tt.args, &stderr)
			if err != nil {
				t.Fatalf("parseOptions() error = %v, stderr = %q", err, stderr.String())
			}
			if got != tt.want {
				t.Fatalf("parseOptions() = %#v, want %#v", got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestParseOptionsRejectsInvalidArgumentsAndPrintsUsage(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"missing out", []string{"in", "out"}},
		{"invalid out", []string{"--out", "yaml", "in", "out"}},
		{"indent negative", []string{"--out", "json", "--indent", "-1", "in", "out"}},
		{"indent too large", []string{"--out", "json", "--indent", "9", "in", "out"}},
		{"indent noninteger", []string{"--out", "json", "--indent", "two", "in", "out"}},
		{"missing positional", []string{"--out", "json", "in"}},
		{"extra positional", []string{"--out", "json", "in", "out", "extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if _, err := parseOptions(tt.args, &stderr); err == nil {
				t.Fatal("parseOptions() error = nil")
			}
			if !strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("stderr = %q, want usage", stderr.String())
			}
		})
	}
}

func TestConvertFileJSONToJSON5(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "output.json5")
	mustWrite(t, input, `{"message":"hello","number":1.20}`)

	if err := convertFile(options{out: outputJSON5, indent: 2, input: input, output: output}); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, output)
	if !strings.Contains(got, "`hello`") || !strings.Contains(got, "1.20") {
		t.Fatalf("output = %q", got)
	}
	assertOneTrailingLF(t, got)
}

func TestConvertFileJSON5ToJSON(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json5")
	output := filepath.Join(dir, "output.json")
	mustWrite(t, input, "{\n  // useful note\n  answer: 0x2a,\n}\n")

	if err := convertFile(options{out: outputJSON, indent: 2, input: input, output: output}); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, output)
	if !json.Valid([]byte(got)) {
		t.Fatalf("output is not valid JSON: %q", got)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["answer_hint"] != "useful note" || decoded["answer"] != float64(42) {
		t.Fatalf("decoded = %#v", decoded)
	}
	assertOneTrailingLF(t, got)
}

func TestConvertFileRejectsSamePathBeforeReading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	err := convertFile(options{out: outputJSON, indent: 2, input: path, output: path})
	if err == nil || !strings.Contains(err.Error(), "same file") {
		t.Fatalf("error = %v, want same file", err)
	}
}

func TestConvertFileRejectsHardLink(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "output.json")
	mustWrite(t, input, `{}`)
	if err := os.Link(input, output); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if err := convertFile(options{out: outputJSON, indent: 2, input: input, output: output}); err == nil || !strings.Contains(err.Error(), "same file") {
		t.Fatalf("error = %v, want same file", err)
	}
}

func TestConvertFileRejectsSymlinkToInput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "output.json")
	mustWrite(t, input, `{}`)
	if err := os.Symlink(input, output); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := convertFile(options{out: outputJSON, indent: 2, input: input, output: output}); err == nil || !strings.Contains(err.Error(), "same file") {
		t.Fatalf("error = %v, want same file", err)
	}
}

func TestConvertFileFailurePreservesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "output.json5")
	mustWrite(t, input, `{invalid`)
	mustWrite(t, output, "original")

	if err := convertFile(options{out: outputJSON5, indent: 2, input: input, output: output}); err == nil {
		t.Fatal("convertFile() error = nil")
	}
	if got := mustRead(t, output); got != "original" {
		t.Fatalf("output = %q, want original", got)
	}
}

func TestConvertFileOverwritesExistingOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "output.json5")
	mustWrite(t, input, `{"new":true}`)
	mustWrite(t, output, "old")

	if err := convertFile(options{out: outputJSON5, indent: 0, input: input, output: output}); err != nil {
		t.Fatal(err)
	}
	if got := mustRead(t, output); got != "{\"new\":true}\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestConvertFileReportsOutputDirectoryError(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	mustWrite(t, input, `{}`)
	output := filepath.Join(dir, "missing", "output.json")
	if err := convertFile(options{out: outputJSON, indent: 2, input: input, output: output}); err == nil || !strings.Contains(err.Error(), "write") {
		t.Fatalf("error = %v, want write error", err)
	}
}

func TestRunReportsParseError(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "output.json5")
	mustWrite(t, input, `{invalid`)
	var stderr bytes.Buffer
	if code := run([]string{"--out", "json5", input, output}, &stderr); code == 0 {
		t.Fatal("run() = 0, want failure")
	}
	if !strings.Contains(stderr.String(), "parse input") || !strings.Contains(stderr.String(), filepath.Base(input)) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunSuccessHasNoStderr(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	output := filepath.Join(dir, "output.json5")
	mustWrite(t, input, `{}`)
	var stderr bytes.Buffer
	if code := run([]string{"--out", "json5", input, output}, &stderr); code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertOneTrailingLF(t *testing.T, content string) {
	t.Helper()
	if !strings.HasSuffix(content, "\n") || strings.HasSuffix(content, "\n\n") {
		t.Fatalf("output must end in exactly one LF: %q", content)
	}
	if runtime.GOOS == "windows" && strings.HasSuffix(content, "\r\n") {
		t.Fatalf("output ends in CRLF, want LF: %q", content)
	}
}
