// Package commentcheck 检查手写 Go 代码的中文文档注释。
//
// 检查范围覆盖函数、方法以及导出的类型、常量和变量。Ent、OpenAPI、Mock 等生成物
// 与测试文件由生成器或测试框架维护，不纳入该门禁。
package commentcheck

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// IssueMissingFunctionComment 表示函数或方法没有紧邻的中文文档注释。
	IssueMissingFunctionComment = "函数或方法缺少紧邻的中文文档注释"
	// IssueMissingExportedComment 表示导出声明没有紧邻的中文文档注释。
	IssueMissingExportedComment = "导出声明缺少紧邻的中文文档注释"
)

// Issue 描述一处缺失或不合规的文档注释。
type Issue struct {
	File   string
	Line   int
	Name   string
	Reason string
}

// CheckRoots 检查多个目录，并按遍历顺序返回全部注释问题。
func CheckRoots(roots []string) ([]Issue, error) {
	var issues []Issue
	for _, root := range roots {
		found, err := CheckRoot(root)
		if err != nil {
			return nil, err
		}
		issues = append(issues, found...)
	}
	return issues, nil
}

// CheckRoot 递归检查单个目录中的手写 Go 源文件。
func CheckRoot(root string) ([]Issue, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	var issues []Issue
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !shouldCheckFile(path) {
			return nil
		}
		found, parseErr := checkFile(path)
		if parseErr != nil {
			return parseErr
		}
		issues = append(issues, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return issues, nil
}

// shouldSkipDir 跳过依赖目录、生成目录和版本控制目录，这些目录不手写业务注释。
func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules", "testdata":
		return true
	default:
		return false
	}
}

// shouldCheckFile 只检查手写生产 Go 文件，测试文件和生成文件不在范围内。
func shouldCheckFile(path string) bool {
	if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return false
	}
	normalized := filepath.ToSlash(path)
	// Ent 与 OpenAPI 生成物由生成器维护，不要求手写中文文档注释。
	if strings.Contains(normalized, "/internal/persistence/ent/") && !strings.Contains(normalized, "/internal/persistence/ent/schema/") {
		return false
	}
	if strings.HasSuffix(normalized, "/docs/openapi.go") {
		return false
	}
	return true
}

// checkFile 解析单个 Go 代码文件，检查其中的函数、方法和对外公开声明是否都编写了紧邻的中文说明。
func checkFile(path string) ([]Issue, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if isGenerated(source) {
		return nil, nil
	}

	var issues []Issue
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			if typed.Name == nil || !hasChineseDoc(typed.Doc) {
				issues = append(issues, newIssue(fileSet, path, typed.Pos(), functionName(typed), IssueMissingFunctionComment))
			}
		case *ast.GenDecl:
			if typed.Tok == token.IMPORT {
				continue
			}
			for _, spec := range typed.Specs {
				name, exported := exportedName(spec)
				if !exported {
					continue
				}
				if !hasChineseDoc(specDoc(typed, spec)) {
					issues = append(issues, newIssue(fileSet, path, spec.Pos(), name, IssueMissingExportedComment))
				}
			}
		}
	}
	return issues, nil
}

// newIssue 记录一处缺注释的位置和原因。
func newIssue(fileSet *token.FileSet, path string, pos token.Pos, name string, reason string) Issue {
	position := fileSet.Position(pos)
	return Issue{File: path, Line: position.Line, Name: name, Reason: reason}
}

// functionName 拼接完整的函数名称；如果是属于某个类型的方法，则带上类型名前缀（如 Type.Method），方便在报错时准确指出是哪一个方法。
func functionName(decl *ast.FuncDecl) string {
	if decl.Recv == nil || len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	return receiverName(decl.Recv.List[0].Type) + "." + decl.Name.Name
}

// receiverName 从方法接收者里取出类型名。
func receiverName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.StarExpr:
		return receiverName(typed.X)
	case *ast.Ident:
		return typed.Name
	case *ast.IndexExpr:
		return receiverName(typed.X)
	case *ast.IndexListExpr:
		return receiverName(typed.X)
	default:
		return "receiver"
	}
}

// exportedName 取出导出的类型或变量名；未导出的声明不需要对外文档注释。
func exportedName(spec ast.Spec) (string, bool) {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		return typed.Name.Name, typed.Name.IsExported()
	case *ast.ValueSpec:
		for _, name := range typed.Names {
			if name.IsExported() {
				return name.Name, true
			}
		}
	}
	return "", false
}

// specDoc 找到紧挨着声明的注释；成组声明没有逐项注释时，使用整组前面的说明。
func specDoc(decl *ast.GenDecl, spec ast.Spec) *ast.CommentGroup {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		if typed.Doc != nil {
			return typed.Doc
		}
	case *ast.ValueSpec:
		if typed.Doc != nil {
			return typed.Doc
		}
		// 同一组常量里，前面的中文分组说明可以覆盖后面没有单独注释的导出项。
		if previous := previousGroupComment(decl, spec); previous != nil {
			return previous
		}
	}
	// 单独声明使用声明前注释；成组声明没有逐项注释时，允许使用整组前面的说明。
	return decl.Doc
}

// previousGroupComment 向前查找同一组里最近的中文说明，让一组常量不用每行重复注释。
func previousGroupComment(decl *ast.GenDecl, spec ast.Spec) *ast.CommentGroup {
	var previous *ast.CommentGroup
	if containsChinese(commentText(decl.Doc)) {
		previous = decl.Doc
	}
	for _, item := range decl.Specs {
		if item == spec {
			return previous
		}
		if doc := directSpecDoc(item); containsChinese(commentText(doc)) {
			previous = doc
		}
	}
	return nil
}

// directSpecDoc 返回写在单个声明正上方的注释。
func directSpecDoc(spec ast.Spec) *ast.CommentGroup {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		return typed.Doc
	case *ast.ValueSpec:
		return typed.Doc
	default:
		return nil
	}
}

// commentText 取出注释正文；没有注释时返回空字符串。
func commentText(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	return group.Text()
}

// hasChineseDoc 判断紧邻注释里有没有汉字。
func hasChineseDoc(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}
	return containsChinese(group.Text())
}

// containsChinese 判断文本里有没有汉字。
func containsChinese(value string) bool {
	for _, r := range value {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// maxGeneratedHeaderBytes 是检测代码文件是否包含自动生成标记时读取的最大头部字节数（4KB）。
const maxGeneratedHeaderBytes = 4096

// isGenerated 识别带生成标记的文件，这些文件不要求手写中文注释。
func isGenerated(source []byte) bool {
	// 生成器标记只认文件头部，避免正文里的普通说明被误判为生成代码。
	head := source
	if len(head) > maxGeneratedHeaderBytes {
		head = head[:maxGeneratedHeaderBytes]
	}
	if !utf8.Valid(head) {
		return false
	}
	text := string(head)
	return strings.Contains(text, "Code generated") || strings.Contains(text, "DO NOT EDIT")
}
