package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// serviceNameRegex 约束服务名必须以小写字母开头，中间可包含小写字母、数字和连字符，并以小写字母或数字结尾。
var serviceNameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// reservedServiceNames 声明不可用作新业务服务名称的系统保留目录与关键组件名称。
var reservedServiceNames = map[string]struct{}{
	"platform":    {},
	"tools":       {},
	"deployments": {},
	"docs":        {},
	"scripts":     {},
	"internal":    {},
	"openspec":    {},
	"bin":         {},
	"coverage":    {},
	"test":        {},
	"tests":       {},
}

// ScaffoldOptions 封装服务骨架生成所需的全部参数配置。
type ScaffoldOptions struct {
	// RepoRoot 是当前项目仓库的绝对或相对根路径。
	RepoRoot string
	// ServiceName 是待创建的业务服务名称（如 order-service）。
	ServiceName string
	// Port 是新服务默认监听的 HTTP 服务端口。
	Port int
	// DryRun 为 true 时仅打印将要生成的目录与文件，不产生实际磁盘写入。
	DryRun bool
	// Force 为 true 时允许覆盖已经存在的目标目录与文件。
	Force bool
}

// TemplateData 承载注入到各代码与配置文件模板中的渲染变量。
type TemplateData struct {
	ServiceName string
	ModuleName  string
	BinaryName  string
	CommandName string
	TitleName   string
	PackageName string
	Port        int
}

// Validate 校验脚手架输入参数的合法性与安全性。
func (o *ScaffoldOptions) Validate() error {
	name := strings.TrimSpace(o.ServiceName)
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if !serviceNameRegex.MatchString(name) {
		return fmt.Errorf("invalid service name %q: must match lowercase kebab-case pattern ^[a-z][a-z0-9-]*[a-z0-9]$", name)
	}
	if _, isReserved := reservedServiceNames[name]; isReserved {
		return fmt.Errorf("service name %q is a reserved system directory name", name)
	}
	if o.Port < 1 || o.Port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", o.Port)
	}
	if o.RepoRoot == "" {
		return fmt.Errorf("repo root cannot be empty")
	}

	targetDir := filepath.Join(o.RepoRoot, name)
	if stat, err := os.Stat(targetDir); err == nil && stat.IsDir() && !o.Force {
		return fmt.Errorf("target service directory %s already exists (use -force to overwrite)", targetDir)
	}

	return nil
}

// GenerateService 根据传入的配置选项执行完整的文件生成与工作区注册流程。
func GenerateService(opts ScaffoldOptions, stdout io.Writer) error {
	if err := opts.Validate(); err != nil {
		return err
	}

	data := buildTemplateData(opts)

	// 第一阶段：生成所有服务文件与配置模板
	for _, tpl := range scaffoldTemplates {
		resolvedPath, err := renderString(tpl.Path, data)
		if err != nil {
			return fmt.Errorf("render path %s: %w", tpl.Path, err)
		}

		fullPath := filepath.Join(opts.RepoRoot, resolvedPath)

		resolvedContent, err := renderString(tpl.Content, data)
		if err != nil {
			return fmt.Errorf("render content for %s: %w", resolvedPath, err)
		}

		// 对生成的 Go 代码文件自动执行 gofmt 规范格式化
		contentBytes := []byte(resolvedContent)
		if strings.HasSuffix(resolvedPath, ".go") {
			formatted, formatErr := format.Source(contentBytes)
			if formatErr == nil {
				contentBytes = formatted
			}
		}

		if opts.DryRun {
			_, _ = fmt.Fprintf(stdout, "[dry-run] will create file: %s\n", resolvedPath)
			continue
		}

		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory %s: %w", dir, err)
		}

		if err := os.WriteFile(fullPath, contentBytes, 0644); err != nil {
			return fmt.Errorf("write file %s: %w", fullPath, err)
		}
		_, _ = fmt.Fprintf(stdout, "created file: %s\n", resolvedPath)
	}

	// 第二阶段：更新根目录 go.work
	if err := updateGoWork(opts, stdout); err != nil {
		return fmt.Errorf("update go.work: %w", err)
	}

	return nil
}

// buildTemplateData 构造模板渲染所需的上下文数据。
func buildTemplateData(opts ScaffoldOptions) TemplateData {
	name := strings.TrimSpace(opts.ServiceName)
	parts := strings.Split(name, "-")
	titleParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			titleParts = append(titleParts, strings.ToUpper(part[:1])+part[1:])
		}
	}
	titleName := strings.Join(titleParts, " ")

	return TemplateData{
		ServiceName: name,
		ModuleName:  "github.com/aegiscore/" + name,
		BinaryName:  name,
		CommandName: "aegiscore-" + name,
		TitleName:   titleName,
		PackageName: strings.ReplaceAll(name, "-", ""),
		Port:        opts.Port,
	}
}

// renderString 使用 text/template 渲染单个文本字符串。
func renderString(content string, data TemplateData) (string, error) {
	t, err := template.New("scaffold").Parse(content)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// updateGoWork 确保新服务模块被登记到仓库根目录的 go.work 中。
func updateGoWork(opts ScaffoldOptions, stdout io.Writer) error {
	goWorkPath := filepath.Join(opts.RepoRoot, "go.work")
	contentBytes, err := os.ReadFile(goWorkPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 如果不存在 go.work 则跳过追加
			return nil
		}
		return err
	}

	content := string(contentBytes)
	entry := fmt.Sprintf("./%s", opts.ServiceName)

	// 检查是否已经存在该模块
	if strings.Contains(content, entry) {
		_, _ = fmt.Fprintf(stdout, "module %s already present in go.work\n", entry)
		return nil
	}

	if opts.DryRun {
		_, _ = fmt.Fprintf(stdout, "[dry-run] will update go.work: add %s\n", entry)
		return nil
	}

	// 定位 use ( ... ) 闭合括号并在末尾追加新条目
	useIdx := strings.Index(content, "use (")
	if useIdx == -1 {
		// 没有括号格式的 use 块，回退直接末尾追加
		newContent := content + fmt.Sprintf("\nuse %s\n", entry)
		return os.WriteFile(goWorkPath, []byte(newContent), 0644)
	}

	closeParenIdx := strings.Index(content[useIdx:], ")")
	if closeParenIdx == -1 {
		return fmt.Errorf("malformed go.work: missing closing parenthesis in use block")
	}
	absoluteCloseIdx := useIdx + closeParenIdx

	// 在闭合括号前插入新行并保持制表符缩进
	insertion := fmt.Sprintf("\t%s\n", entry)
	newContent := content[:absoluteCloseIdx] + insertion + content[absoluteCloseIdx:]

	if err := os.WriteFile(goWorkPath, []byte(newContent), 0644); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(stdout, "registered %s in go.work\n", entry)
	return nil
}
