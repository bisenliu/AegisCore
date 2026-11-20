package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type importInfo struct {
	name string
	path string
}

type methodSpec struct {
	name      string
	signature string
}

// main 命令行工具入口，负责启动代码生成逻辑并在出错时打印提示。
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "metrics nopgen: %v\n", err)
		os.Exit(1)
	}
}

// run 解析命令行输入的接口文件路径和输出目标，自动生成对应的空操作代码。
func run() error {
	// nopgen 从接口声明生成无返回值 metrics 的 no-op 实现；source、type 和 output 是最小必填输入。
	source := flag.String("source", "", "Go source file containing the interface")
	typeName := flag.String("type", "", "interface type name")
	output := flag.String("output", "", "output file")
	structName := flag.String("struct", "nopMetrics", "generated no-op struct name")
	funcName := flag.String("func", "", "constructor function name")
	comment := flag.String("comment", "", "constructor comment")
	flag.Parse()

	if strings.TrimSpace(*source) == "" {
		return errors.New("-source is required")
	}
	if strings.TrimSpace(*typeName) == "" {
		return errors.New("-type is required")
	}
	if strings.TrimSpace(*output) == "" {
		return errors.New("-output is required")
	}
	if strings.TrimSpace(*funcName) == "" {
		*funcName = "Nop" + strings.TrimSpace(*typeName)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, *source, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	imports, err := collectImports(file)
	if err != nil {
		return err
	}
	methods, usedImportNames, err := findInterfaceMethods(fset, file, *typeName)
	if err != nil {
		return err
	}

	generated, err := renderFile(file.Name.Name, *typeName, *structName, *funcName, *comment, methods, filterImports(imports, usedImportNames))
	if err != nil {
		return err
	}
	return os.WriteFile(*output, generated, 0o644)
}

// collectImports 分析并提取原始代码文件中引入的所有外部包，供生成的代码复用。
func collectImports(file *ast.File) (map[string]importInfo, error) {
	imports := make(map[string]importInfo)
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		name := path.Base(importPath)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = importInfo{name: name, path: importPath}
	}
	return imports, nil
}

// findInterfaceMethods 从源文件中定位指定的接口声明，并提取其中的所有方法。
func findInterfaceMethods(fset *token.FileSet, file *ast.File, typeName string) ([]methodSpec, map[string]struct{}, error) {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != typeName {
				continue
			}
			iface, ok := typeSpec.Type.(*ast.InterfaceType)
			if !ok {
				return nil, nil, fmt.Errorf("%s is not an interface", typeName)
			}
			return interfaceMethods(fset, iface)
		}
	}
	return nil, nil, fmt.Errorf("interface %s not found", typeName)
}

// interfaceMethods 提取接口中定义的所有方法名称、参数与返回值结构。
func interfaceMethods(fset *token.FileSet, iface *ast.InterfaceType) ([]methodSpec, map[string]struct{}, error) {
	// 仅支持接口内显式声明的方法，不支持嵌入接口；通过 SelectorExpr 收集外部包名，特殊别名或 dot import 不在支持范围内。
	methods := make([]methodSpec, 0, len(iface.Methods.List))
	usedImportNames := make(map[string]struct{})
	for _, field := range iface.Methods.List {
		if len(field.Names) != 1 {
			return nil, nil, errors.New("embedded interfaces are not supported")
		}
		funcType, ok := field.Type.(*ast.FuncType)
		if !ok {
			return nil, nil, fmt.Errorf("%s is not a method", field.Names[0].Name)
		}
		ast.Inspect(funcType, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			usedImportNames[ident.Name] = struct{}{}
			return true
		})
		signature, err := renderSignature(fset, funcType)
		if err != nil {
			return nil, nil, err
		}
		methods = append(methods, methodSpec{name: field.Names[0].Name, signature: signature})
	}
	return methods, usedImportNames, nil
}

// renderSignature 将方法的参数和返回值重新拼接为可写入代码文件的声明文本。
func renderSignature(fset *token.FileSet, funcType *ast.FuncType) (string, error) {
	params, err := renderFieldList(fset, funcType.Params, true)
	if err != nil {
		return "", err
	}
	if funcType.Results == nil {
		return params, nil
	}
	if len(funcType.Results.List) == 1 && len(funcType.Results.List[0].Names) == 0 {
		result, err := renderNode(fset, funcType.Results.List[0].Type)
		if err != nil {
			return "", err
		}
		return params + " " + result, nil
	}
	results, err := renderFieldList(fset, funcType.Results, true)
	if err != nil {
		return "", err
	}
	return params + " " + results, nil
}

// renderFieldList 将参数列表或返回值列表格式化为代码文本。
func renderFieldList(fset *token.FileSet, list *ast.FieldList, parens bool) (string, error) {
	if list == nil {
		if parens {
			return "()", nil
		}
		return "", nil
	}
	fields := make([]string, 0, len(list.List))
	for _, field := range list.List {
		fieldType, err := renderNode(fset, field.Type)
		if err != nil {
			return "", err
		}
		if len(field.Names) == 0 {
			fields = append(fields, fieldType)
			continue
		}
		names := make([]string, 0, len(field.Names))
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
		fields = append(fields, strings.Join(names, ", ")+" "+fieldType)
	}
	rendered := strings.Join(fields, ", ")
	if parens {
		return "(" + rendered + ")", nil
	}
	return rendered, nil
}

// renderNode 将语法解析树中的节点直接还原为原始代码文本。
func renderNode(fset *token.FileSet, node ast.Node) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// filterImports 剔除未被使用的多余包引入，确保生成的代码干净且能直接编译。
func filterImports(imports map[string]importInfo, usedImportNames map[string]struct{}) []importInfo {
	filtered := make([]importInfo, 0, len(usedImportNames))
	for name := range usedImportNames {
		info, ok := imports[name]
		if ok {
			filtered = append(filtered, info)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].path < filtered[j].path
	})
	return filtered
}

// renderFile 拼接生成完整的 Go 语言源文件内容。
func renderFile(packageName, typeName, structName, funcName, comment string, methods []methodSpec, imports []importInfo) ([]byte, error) {
	// 当前模板生成空方法体，适用于无返回值 metrics 接口；接口未来增加返回值时需要先扩展生成策略。
	var buf bytes.Buffer
	buf.WriteString("// Code generated by metrics nopgen; DO NOT EDIT.\n\n")
	buf.WriteString("package ")
	buf.WriteString(packageName)
	buf.WriteString("\n\n")
	writeImports(&buf, imports)
	fmt.Fprintf(&buf, "type %s struct{}\n\n", structName)
	if strings.TrimSpace(comment) != "" {
		fmt.Fprintf(&buf, "// %s\n", strings.TrimSpace(comment))
	}
	fmt.Fprintf(&buf, "func %s() %s {\n\treturn %s{}\n}\n\n", funcName, typeName, structName)
	for _, method := range methods {
		fmt.Fprintf(&buf, "func (%s) %s%s {}\n", structName, method.name, method.signature)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, err
	}
	return formatted, nil
}

// writeImports 将需要保留的包引入声明写入到生成的代码头部。
func writeImports(buf *bytes.Buffer, imports []importInfo) {
	if len(imports) == 0 {
		return
	}
	if len(imports) == 1 && imports[0].name == path.Base(imports[0].path) {
		fmt.Fprintf(buf, "import %q\n\n", imports[0].path)
		return
	}
	buf.WriteString("import (\n")
	for _, info := range imports {
		if info.name == path.Base(info.path) {
			fmt.Fprintf(buf, "\t%q\n", info.path)
			continue
		}
		fmt.Fprintf(buf, "\t%s %q\n", info.name, info.path)
	}
	buf.WriteString(")\n\n")
}
