package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	pflag "github.com/spf13/pflag"
	logs "github.com/tea4go/gh/log4go"
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

func filepathJoin(elem ...string) string {
	path := filepath.Join(elem...)
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(path, "\\", "/")
	}
	return path
}
func parseOptions(args []string, stderr io.Writer) (options, error) {
	var opts options
	out := pflag.StringP("out", "", "", "output format: json, json5, or golang")
	indent := pflag.IntP("indent", "", 2, "indentation spaces (0..8)")
	pflag.Usage = func() {
		fmt.Fprintln(stderr, "Usage: json-convert --out json|json5|golang [--indent 0..8] INPUT [OUTPUT]")
		pflag.PrintDefaults()
	}
	if err := pflag.CommandLine.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return options{}, err
		}
		pflag.Usage()
		return options{}, err
	}
	log_name := os.Getenv("log_name")
	if log_name == "" {
		log_name = "json-convert"
	}
	// 标准程序块
	logsFileName := filepathJoin(os.TempDir(), "ulog_"+log_name+".txt")
	logs.SetLogger("file", `{"filename":"`+logsFileName+`", "perm": "0666","level":5}`)
	logs.StartLogger()
	// 标准程序块

	opts.indent = *indent

	switch *out {
	case "json":
		opts.out = outputJSON
	case "json5":
		opts.out = outputJSON5
	case "golang":
		opts.out = outputGolang
	default:
		pflag.Usage()
		return options{}, fmt.Errorf("--out must be json, json5, or golang")
	}
	if opts.indent < 0 || opts.indent > 8 {
		pflag.Usage()
		return options{}, fmt.Errorf("--indent must be an integer from 0 to 8")
	}
	if pflag.NArg() < 1 || pflag.NArg() > 2 {
		pflag.Usage()
		return options{}, fmt.Errorf("expected one or two file paths")
	}
	opts.input = pflag.Arg(0)
	if pflag.NArg() == 2 {
		opts.output = pflag.Arg(1)
	}
	if opts.output == "" {
		opts.output = defaultOutputPath(opts.input, opts.out)
	}
	return opts, nil
}

func convertFile(opts options) error {
	logs.Notice("开始转换")
	logs.Notice("= 模式: %s", outputModeName(opts.out))
	logs.Notice("= 输入: %s", opts.input)
	logs.Notice("= 输出: %s", opts.output)
	logs.Notice("= 缩进: %d", opts.indent)

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
	logs.Info("读取输入文件成功: %s (%d bytes)", opts.input, len(data))
	mode := modeJSON
	if opts.out == outputJSON || opts.out == outputGolang {
		mode = modeJSON5
	}
	logs.Info("解析输入模式: %s", parseModeName(mode))
	value, err := parseDocument(data, mode)
	if err != nil {
		return fmt.Errorf("parse input %q: %w", opts.input, err)
	}
	logs.Info("解析输入完成")

	switch opts.out {
	case outputJSON:
		logs.Info("附加注释 hint 字段")
	case outputJSON5:
		data, err = writeDocument(value, opts.out, opts.indent)
	case outputGolang:
		logs.Info("生成 Go 对象定义")
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
	logs.Notice("转换完成: %s", opts.output)
	return nil
}

func outputModeName(out outputMode) string {
	switch out {
	case outputJSON:
		return "json"
	case outputJSON5:
		return "json5"
	case outputGolang:
		return "golang"
	default:
		return "unknown"
	}
}

func parseModeName(mode parseMode) string {
	switch mode {
	case modeJSON:
		return "json"
	case modeJSON5:
		return "json5"
	default:
		return "unknown"
	}
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
		if errors.Is(err, pflag.ErrHelp) {
			return 0
		}
		logs.Error("命令行参数错误: %v", err)
		return 2
	}
	logs.Info("参数解析完成")
	if err := convertFile(opts); err != nil {
		logs.Error("执行失败: %v", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}
