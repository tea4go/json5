package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
		{"golang defaults output", []string{"--out", "golang", "config.json5"}, options{out: outputGolang, indent: 2, input: "config.json5", output: "config_convert.go"}},
		{"empty output uses default", []string{"--out", "json", "123.json", ""}, options{out: outputJSON, indent: 2, input: "123.json", output: "123_convert.json"}},
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
		{"no arguments", nil},
		{"missing out", []string{"in", "out"}},
		{"out missing value", []string{"--out"}},
		{"invalid out", []string{"--out", "yaml", "in", "out"}},
		{"indent missing value", []string{"--out", "json", "--indent"}},
		{"indent negative", []string{"--out", "json", "--indent", "-1", "in", "out"}},
		{"indent too large", []string{"--out", "json", "--indent", "9", "in", "out"}},
		{"indent noninteger", []string{"--out", "json", "--indent", "two", "in", "out"}},
		{"unknown flag", []string{"--out", "json", "--unknown", "in", "out"}},
		{"missing positional", []string{"--out", "json"}},
		{"too many positional", []string{"--out", "json", "in", "out", "extra"}},
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

func TestConvertFileJSON5ToGolang(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "config.json5")
	outputDir := filepath.Join(dir, "pkgdemo")
	if err := os.Mkdir(outputDir, 0o700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(outputDir, "config.go")
	mustWrite(t, input, "{\n  // service host\n  host: `localhost`,\n  ports: [80, 443],\n  tls: {enabled: true},\n}\n")

	if err := convertFile(options{out: outputGolang, indent: 2, input: input, output: output}); err != nil {
		t.Fatal(err)
	}
	got := mustRead(t, output)
	if !strings.Contains(got, "package pkgdemo") {
		t.Fatalf("output = %q, want package name based on directory", got)
	}
	if !strings.Contains(got, "type Config struct") {
		t.Fatalf("output = %q, want root struct", got)
	}
	if !strings.Contains(got, "// service host") || !strings.Contains(got, "json:\"host\"") || !strings.Contains(got, "Host") || !strings.Contains(got, "string") {
		t.Fatalf("output = %q, want host field and comment", got)
	}
	if !strings.Contains(got, "json:\"ports\"") || !strings.Contains(got, "[]int64") {
		t.Fatalf("output = %q, want ports slice", got)
	}
	if !strings.Contains(got, "json:\"tls\"") || !strings.Contains(got, "type ConfigTls struct") {
		t.Fatalf("output = %q, want nested struct", got)
	}
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

func TestConvertFileRejectsSymlinkOutputToAnotherFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	target := filepath.Join(dir, "target.json5")
	output := filepath.Join(dir, "output.json5")
	mustWrite(t, input, `{"new":true}`)
	mustWrite(t, target, "original")
	if err := os.Symlink(target, output); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := convertFile(options{out: outputJSON5, indent: 0, input: input, output: output}); err == nil {
		t.Fatal("convertFile() error = nil, want symlink output error")
	}
	info, err := os.Lstat(output)
	if err != nil {
		t.Fatalf("lstat output: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("output mode = %v, want symlink", info.Mode())
	}
	if got, err := os.Readlink(output); err != nil || got != target {
		t.Fatalf("readlink output = %q, %v; want %q", got, err, target)
	}
	if got := mustRead(t, target); got != "original" {
		t.Fatalf("target = %q, want original", got)
	}
	assertNoAtomicArtifacts(t, dir)
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

func TestWriteFileAtomicRejectsEmptyOutputDirectory(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.json")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}

	err := writeFileAtomic(output, []byte("new"))
	if err == nil {
		t.Fatal("writeFileAtomic() error = nil, want directory error")
	}
	info, statErr := os.Stat(output)
	if statErr != nil {
		t.Fatalf("output directory was removed: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf("output = %q, want directory", output)
	}
	assertNoAtomicArtifacts(t, dir)
}

func TestWriteFileAtomicRejectsNonEmptyOutputDirectory(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.json")
	if err := os.Mkdir(output, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(output, "keep.txt")
	mustWrite(t, marker, "keep")

	err := writeFileAtomic(output, []byte("new"))
	if err == nil {
		t.Fatal("writeFileAtomic() error = nil, want directory error")
	}
	if got := mustRead(t, marker); got != "keep" {
		t.Fatalf("marker = %q, want keep", got)
	}
	assertNoAtomicArtifacts(t, dir)
}

func TestWriteFileAtomicRestoresOldFileAfterReplacementFailure(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.json")
	mustWrite(t, output, "old")

	originalRename := renameFile
	originalRemove := removeFile
	t.Cleanup(func() {
		renameFile = originalRename
		removeFile = originalRemove
	})

	calls := 0
	renameFile = func(oldPath, newPath string) error {
		calls++
		switch calls {
		case 1:
			return errors.New("direct replacement failed")
		case 2:
			return os.Rename(oldPath, newPath)
		case 3:
			return errors.New("fallback replacement failed")
		case 4:
			return os.Rename(oldPath, newPath)
		default:
			t.Fatalf("unexpected rename %q -> %q", oldPath, newPath)
			return nil
		}
	}
	removeFile = os.Remove

	err := writeFileAtomic(output, []byte("new"))
	if err == nil || !strings.Contains(err.Error(), "fallback replacement failed") {
		t.Fatalf("error = %v, want replacement failure", err)
	}
	if got := mustRead(t, output); got != "old" {
		t.Fatalf("output = %q, want restored old content", got)
	}
	assertNoAtomicArtifacts(t, dir)
}

func TestWriteFileAtomicKeepsBackupWhenRestoreFails(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "output.json")
	mustWrite(t, output, "old")

	originalRename := renameFile
	originalRemove := removeFile
	t.Cleanup(func() {
		renameFile = originalRename
		removeFile = originalRemove
	})

	calls := 0
	renameFile = func(oldPath, newPath string) error {
		calls++
		switch calls {
		case 1:
			return errors.New("direct replacement failed")
		case 2:
			return os.Rename(oldPath, newPath)
		case 3:
			return errors.New("fallback replacement failed")
		case 4:
			return errors.New("restore failed")
		default:
			t.Fatalf("unexpected rename %q -> %q", oldPath, newPath)
			return nil
		}
	}
	removeFile = os.Remove

	err := writeFileAtomic(output, []byte("new"))
	if err == nil || !strings.Contains(err.Error(), "fallback replacement failed") || !strings.Contains(err.Error(), "restore failed") {
		t.Fatalf("error = %v, want replacement and restore failures", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(dir, ".json-convert-backup-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) != 1 {
		t.Fatalf("backup files = %v, want one", matches)
	}
	if got := mustRead(t, matches[0]); got != "old" {
		t.Fatalf("backup = %q, want old content", got)
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

func TestRunHelpPrintsUsageOnce(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"--help"}, &stderr); code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if count := strings.Count(stderr.String(), "Usage:"); count != 1 {
		t.Fatalf("stderr usage count = %d, want 1; stderr = %q", count, stderr.String())
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

func assertNoAtomicArtifacts(t *testing.T, dir string) {
	t.Helper()
	for _, pattern := range []string{".json-convert-*", ".json-convert-backup-*"} {
		matches, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("atomic artifacts = %v, want none", matches)
		}
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
