# Design

## Overview

当前转换工具在 `user-service/internal/tools/openapi-convert/main.go` 中完成以下流程：

1. 从 `swag init` 生成的 Swagger 2 JSON 文件读取输入。
2. 使用 `kin-openapi` 转换为 OpenAPI 3。
3. 固定设置 OpenAPI version、global server、健康检查 path-level server 和 BearerAuth security scheme。
4. 校验 OpenAPI 3 文档。
5. 输出 JSON、YAML 和内嵌 JSON 的 Go 源码文件。

目标是把第 1、2、4、5 步和可参数化的第 3 步抽到 `common`，让所有服务复用同一转换与渲染实现。用户服务继续拥有自己的源码扫描范围、输出位置和 OpenAPI 语义参数。

## Package Layout

新增共享包：

```text
common/http/openapi/
  convert.go
  render.go
  options.go
  convert_test.go
  render_test.go
```

该包属于 `common/http`，因为它服务于 HTTP API 文档生成，且可以依赖 OpenAPI 相关第三方库；它不进入 `common/contract`，避免把构建期文档转换工具误认为运行时 API 契约 DTO。

用户服务保留薄命令入口：

```text
user-service/internal/tools/openapi-convert/
  main.go
```

该入口只解析 flags、构造 options、调用 `common/http/openapi`、写文件和输出摘要。它可以继续位于 `user-service/internal/tools`，因为它承载用户服务默认值和脚本兼容性。未来新服务可以复制一个薄 wrapper，或在后续变更中引入 workspace 级共享命令入口。

## Shared API

建议共享包提供以下 API：

```go
package openapi

type ConvertOptions struct {
    OpenAPIVersion string
    Servers        []Server
    PathServers    map[string][]Server
    SecuritySchemes map[string]SecurityScheme
}

type Server struct {
    URL         string
    Description string
}

type SecurityScheme struct {
    Type         string
    Scheme       string
    BearerFormat string
    Description  string
}

type Document struct {
    OpenAPI string
    PathCount int
    JSON []byte
    YAML []byte
}

func ConvertSwagger2JSON(ctx context.Context, data []byte, opts ConvertOptions) (*Document, error)
func ConvertSwagger2File(ctx context.Context, inputPath string, opts ConvertOptions) (*Document, error)
```

渲染 Go embed 文件：

```go
type GoDocumentOptions struct {
    PackageName string
    FunctionName string
    ConstName string
    GeneratedBy string
}

func RenderGoDocument(jsonData []byte, opts GoDocumentOptions) ([]byte, error)
```

默认值必须是无业务语义的：

- `OpenAPIVersion` 空值可默认 `3.0.3`。
- `FunctionName` 空值可默认 `ReadOpenAPI`。
- `ConstName` 空值可默认 `openAPIDocument`。
- `PackageName` 必须由调用方提供，避免共享包猜测服务输出包。
- `Servers`、`PathServers` 和 `SecuritySchemes` 空值表示不注入。

## Normalization Rules

共享包只执行调用方显式要求的规范化：

- 设置 `doc.OpenAPI` 为 `ConvertOptions.OpenAPIVersion` 或默认 `3.0.3`。
- 当 `Servers` 非空时，设置 `doc.Servers`。
- 当 `PathServers` 中存在某个 path 时，为该 path 设置 path-level servers。
- 当 `SecuritySchemes` 非空时，确保 `doc.Components` 和 `doc.Components.SecuritySchemes` 初始化，并写入调用方声明的 scheme。

共享包不得内置以下用户服务规则：

- `/api/v1`
- `/livez`
- `/readyz`
- `/startupz`
- `BearerAuth`
- 中文 Bearer token 描述
- `scripts/openapi-generate.sh`

这些都应由用户服务 wrapper 或脚本传入。

## User Service Wrapper

`user-service/internal/tools/openapi-convert/main.go` 保留现有 CLI 的基本兼容性：

```text
-input <path>
-json <path>
-yaml <path>
-go <path>
-package <name>
```

并可以新增服务侧可配置 flags：

```text
-openapi-version 3.0.3
-server /api/v1
-root-server /
-root-path /livez
-root-path /readyz
-root-path /startupz
-bearer-auth-name BearerAuth
-bearer-auth-description "输入 Bearer token，格式为：Bearer <token>"
-generated-by scripts/openapi-generate.sh
```

为保持脚本简单，wrapper 可继续把这些值作为用户服务默认值；但默认值必须只存在于 wrapper，不进入 `common` 包。若未来引入 `common/cmd/openapi-convert`，这些值必须改由每个服务脚本显式传参。

## Script Updates

`user-service/scripts/openapi-generate.sh` 继续负责：

- 从 `user-service/` 模块目录运行。
- 调用 `swag init`。
- 维护 `-d ./cmd,./internal/router,...` 扫描范围。
- 输出到 `docs/openapi.json`、`docs/openapi.yaml` 和 `docs/openapi.go`。

转换命令仍可调用本服务 wrapper：

```sh
go run ./internal/tools/openapi-convert \
  -input "$tmp_dir/swagger/swagger.json" \
  -json docs/openapi.json \
  -yaml docs/openapi.yaml \
  -go docs/openapi.go \
  -package docs
```

如果 wrapper 默认值被移除，则脚本必须显式传入 server、root path 和 BearerAuth 参数，避免服务语义藏在共享包中。

## Dependency Management

`common/go.mod` 需要新增直接依赖：

- `github.com/getkin/kin-openapi v0.140.0`
- `gopkg.in/yaml.v3 v3.0.1`

`user-service/go.mod` 是否继续直接依赖 `github.com/getkin/kin-openapi` 取决于迁移后的服务模块是否仍有直接 import。若只有 `common` 使用，运行 `go mod tidy` 后该依赖应从用户服务直接依赖中移除或降为间接依赖。

## Documentation Updates

长期规则文档需要同步说明：

- `common/http/openapi` 是跨服务 OpenAPI 文档转换与渲染辅助能力，不承载服务路径、认证方案或源码扫描范围。
- 每个服务自己的 `scripts/openapi-generate.sh` 或薄 wrapper 拥有服务侧 OpenAPI 生成参数。
- `user-service` 继续拥有运行时 OpenAPI 路由和生成文档文件。

至少更新：

- `AGENTS.md`
- `docs/ARCHITECTURE.md`
- `docs/DEVELOPMENT.md`

## Verification Strategy

先运行共享包测试：

```bash
cd common && go test ./http/openapi
```

再运行用户服务生成链路：

```bash
make openapi-generate
```

确认生成结果可解析：

```bash
cd user-service && go test ./internal/router
```

最后运行边界和相关模块测试：

```bash
make architecture-lint
make test-common
make test-user-service
```

如果 `make openapi-generate` 改写了 `user-service/docs/openapi.*`，需要 review diff，确认变化只来自等价格式化或生成注释参数化，不改变 API 语义。

## Risk

主要风险是把用户服务约定不小心硬编码进 `common`，导致后续服务复用时继承错误 server、健康探针路径或认证描述。通过 options 强制服务侧显式声明这些语义，可以控制该风险。

第二个风险是 `common` 引入较重的 OpenAPI 转换依赖。该依赖只应被共享 OpenAPI 工具包使用，不应扩散到运行时 HTTP middleware、response、binding 或业务契约包。

第三个风险是生成文件产生非预期 diff。实施时应在运行生成后检查 `user-service/docs/openapi.json`、`openapi.yaml` 和 `openapi.go`，确保输出语义保持一致。
