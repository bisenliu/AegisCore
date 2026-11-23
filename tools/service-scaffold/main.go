package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// exitOK 表示命令执行成功的退出状态码。
	exitOK = 0
	// exitError 表示命令执行遇到错误或参数校验失败的退出状态码。
	exitError = 1
)

// main 是服务脚手架 CLI 的总入口。
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run 解析命令行参数并执行生成逻辑，返回进程退出码。
func run(args []string, stdout io.Writer, stderr io.Writer) int {
	var opts ScaffoldOptions

	flags := flag.NewFlagSet("service-scaffold", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&opts.ServiceName, "name", "", "新业务服务的名称（必须为小写 kebab-case，例如 order-service）")
	flags.IntVar(&opts.Port, "port", 8081, "新服务默认监听的 HTTP 服务端口（1-65535）")
	flags.StringVar(&opts.RepoRoot, "repo-root", "", "AegisCore 仓库根目录路径（留空时自动向上推导）")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "试运行模式：仅打印将要生成的文件和目录变更，不写入磁盘")
	flags.BoolVar(&opts.Force, "force", false, "强制模式：允许覆盖已存在的目标目录与文件")

	flags.Usage = func() {
		_, _ = fmt.Fprintf(stderr, "用法：service-scaffold -name <service-name> [选项]\n\n选项：\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		return exitError
	}

	if opts.ServiceName == "" {
		_, _ = fmt.Fprintln(stderr, "错误：必须通过 -name 指定新服务名称（例如 -name order-service）")
		flags.Usage()
		return exitError
	}

	if opts.RepoRoot == "" {
		resolvedRoot, err := findRepoRoot()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "定位仓库根目录失败：%v\n", err)
			return exitError
		}
		opts.RepoRoot = resolvedRoot
	}

	if err := GenerateService(opts, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "生成服务失败：%v\n", err)
		return exitError
	}

	if !opts.DryRun {
		_, _ = fmt.Fprintf(stdout, "\n业务服务 %q 生成成功！\n", opts.ServiceName)
		_, _ = fmt.Fprintln(stdout, "后续建议步骤：")
		_, _ = fmt.Fprintf(stdout, "1. 在根 Makefile 的 BUSINESS_SERVICES 中追加 %s 以便纳入全局 make build/test/lint。\n", opts.ServiceName)
		_, _ = fmt.Fprintf(stdout, "2. 运行 make %s-build 构建服务。\n", opts.ServiceName)
		_, _ = fmt.Fprintf(stdout, "3. 运行 make %s-test 运行服务测试。\n", opts.ServiceName)
	}

	return exitOK
}

// findRepoRoot 尝试自当前工作目录向上搜寻包含 go.work 的目录作为仓库根路径。
func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	curr := cwd
	for {
		if _, err := os.Stat(filepath.Join(curr, "go.work")); err == nil {
			return curr, nil
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}

	return cwd, nil
}
