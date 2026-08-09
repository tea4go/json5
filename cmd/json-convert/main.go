package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type options struct {
	out    outputMode
	indent int
	input  string
	output string
}

var (
	renameFile = os.Rename
	removeFile = os.Remove
)

func parseOptions(args []string, stderr io.Writer) (options, error) {
	var opts options
	var out string
	flags := flag.NewFlagSet("json-convert", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&out, "out", "", "output format: json, json5, or golang")
	flags.IntVar(&opts.indent, "indent", 2, "indentation spaces (0..8)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: json-convert --out json|json5|golang [--indent 0..8] INPUT [OUTPUT]")
	}
	if err := flags.Parse(args); err != nil {
		flags.Usage()
		return options{}, err
	}
	switch out {
	case "json":
		opts.out = outputJSON
	case "json5":
		opts.out = outputJSON5
	case "golang":
		opts.out = outputGolang
	default:
		flags.Usage()
		return options{}, fmt.Errorf("--out must be json, json5, or golang")
	}
	if opts.indent < 0 || opts.indent > 8 {
		flags.Usage()
		return options{}, fmt.Errorf("--indent must be an integer from 0 to 8")
	}
	if flags.NArg() < 1 || flags.NArg() > 2 {
		flags.Usage()
		return options{}, fmt.Errorf("expected one or two file paths")
	}
	opts.input = flags.Arg(0)
	if flags.NArg() == 2 {
		opts.output = flags.Arg(1)
	}
	if opts.output == "" {
		opts.output = defaultOutputPath(opts.input, opts.out)
	}
	return opts, nil
}

func convertFile(opts options) error {
	same, err := sameFile(opts.input, opts.output)
	if err != nil {
		return err
	}
	if same {
		return fmt.Errorf("convert %q to %q: input and output are the same file", opts.input, opts.output)
	}

	data, err := os.ReadFile(opts.input)
	if err != nil {
		return fmt.Errorf("read input %q: %w", opts.input, err)
	}
	mode := modeJSON
	if opts.out == outputJSON || opts.out == outputGolang {
		mode = modeJSON5
	}
	value, err := parseDocument(data, mode)
	if err != nil {
		return fmt.Errorf("parse input %q: %w", opts.input, err)
	}
	switch opts.out {
	case outputJSON:
		value = addHintMembers(value)
		data, err = writeDocument(value, opts.out, opts.indent)
	case outputJSON5:
		data, err = writeDocument(value, opts.out, opts.indent)
	case outputGolang:
		data, err = generateGoDefinitions(value, goPackageName(opts.output), rootTypeName(opts.input))
	default:
		err = fmt.Errorf("unsupported output mode")
	}
	if err != nil {
		return fmt.Errorf("convert input %q: %w", opts.input, err)
	}
	if err := writeFileAtomic(opts.output, data); err != nil {
		return fmt.Errorf("write output %q: %w", opts.output, err)
	}
	return nil
}

func defaultOutputPath(input string, out outputMode) string {
	dir := filepath.Dir(input)
	base := filepath.Base(input)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		name = strings.TrimLeft(base, ".")
	}
	if name == "" {
		name = "output"
	}
	return filepath.Join(dir, name+"_convert"+defaultOutputExt(out))
}

func defaultOutputExt(out outputMode) string {
	switch out {
	case outputJSON:
		return ".json"
	case outputJSON5:
		return ".json5"
	case outputGolang:
		return ".go"
	default:
		return ".out"
	}
}

func goPackageName(output string) string {
	dir := filepath.Base(filepath.Dir(output))
	name := goIdentifier(dir, false)
	if name == "" {
		return "main"
	}
	return name
}

func rootTypeName(input string) string {
	base := filepath.Base(input)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return goIdentifier(name, true)
}

func sameFile(input, output string) (bool, error) {
	inputAbs, err := filepath.Abs(filepath.Clean(input))
	if err != nil {
		return false, fmt.Errorf("compare input path %q: %w", input, err)
	}
	outputAbs, err := filepath.Abs(filepath.Clean(output))
	if err != nil {
		return false, fmt.Errorf("compare output path %q: %w", output, err)
	}
	if inputAbs == outputAbs || runtime.GOOS == "windows" && strings.EqualFold(inputAbs, outputAbs) {
		return true, nil
	}
	inputInfo, inputErr := os.Stat(inputAbs)
	outputInfo, outputErr := os.Stat(outputAbs)
	if inputErr == nil && outputErr == nil {
		return os.SameFile(inputInfo, outputInfo), nil
	}
	return false, nil
}

func writeFileAtomic(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".json-convert-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() {
		temp.Close()
		removeFile(tempName)
	}()

	if _, err = temp.Write(data); err != nil {
		return err
	}
	if err = temp.Sync(); err != nil {
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}

	outputInfo, statErr := os.Lstat(path)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if statErr == nil && !outputInfo.Mode().IsRegular() {
		return fmt.Errorf("refuse to replace non-regular output %q", path)
	}

	if err = renameFile(tempName, path); err == nil {
		return nil
	}
	if errors.Is(statErr, os.ErrNotExist) {
		return err
	}

	backup, err := os.CreateTemp(dir, ".json-convert-backup-*")
	if err != nil {
		return err
	}
	backupName := backup.Name()
	if err = backup.Close(); err != nil {
		os.Remove(backupName)
		return err
	}
	if err = removeFile(backupName); err != nil {
		return err
	}
	if err = renameFile(path, backupName); err != nil {
		return err
	}
	if replaceErr := renameFile(tempName, path); replaceErr != nil {
		if restoreErr := renameFile(backupName, path); restoreErr != nil {
			return fmt.Errorf("replace output: %v; restore backup %q: %w", replaceErr, backupName, restoreErr)
		}
		return replaceErr
	}
	if err = removeFile(backupName); err != nil {
		return err
	}
	return nil
}

func run(args []string, stderr io.Writer) int {
	opts, err := parseOptions(args, stderr)
	if err != nil {
		return 2
	}
	if err := convertFile(opts); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}
