// Command commentcheck 是仓库级中文文档注释门禁入口。
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aegiscore/platform/testing/commentcheck"
)

// main 是进程入口，负责把命令行参数交给根命令执行。
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run 执行本次命令；参数不合法或执行失败时给出明确错误。
func run(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("commentcheck", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var roots multiString
	flags.Var(&roots, "root", "要检查的模块或目录，可重复指定")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if len(roots) == 0 {
		roots = append(roots, ".")
	}

	issues, err := commentcheck.CheckRoots(roots)
	if err != nil {
		return writeLine(stderr, fmt.Sprintf("commentcheck: %v", err))
	}
	for _, item := range issues {
		if code := writeLine(stdout, fmt.Sprintf("%s:%d: %s: %s", item.File, item.Line, item.Name, item.Reason)); code != 0 {
			return code
		}
	}
	if len(issues) > 0 {
		return writeLine(stderr, fmt.Sprintf("commentcheck: failed with %d issue(s)", len(issues)))
	}
	return writeLine(stdout, "commentcheck: ok")
}

// writeLine 输出一行结果；输出失败时返回非零退出码。
func writeLine(writer io.Writer, message string) int {
	if _, err := fmt.Fprintln(writer, message); err != nil {
		return 1
	}
	return 0
}

type multiString []string

// String 返回这个参数当前拼好的文本。
func (m *multiString) String() string {
	return strings.Join(*m, ",")
}

// Set 接收命令行中多次传入的同名参数（例如多个 -root 目录），并将它们追加到目录列表中。
func (m *multiString) Set(value string) error {
	*m = append(*m, value)
	return nil
}
