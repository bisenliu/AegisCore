# Design

## Overview

本变更只定义边界，不实现 gRPC 能力。目标是把未来本服务入站 gRPC transport 的位置与职责讲清楚，避免它和现有 HTTP transport、外部系统 gRPC integration 混用。

目标关系：

```text
features/<feature>/transport/http
  -> inbound HTTP request parsing, HTTP DTOs, Gin routes/controllers
  -> application command/query

features/<feature>/transport/grpc
  -> future inbound gRPC handler and server-side protobuf mapping
  -> application command/query

internal/integration/grpc
  -> outbound client adapter for external gRPC services
  -> feature-owned application ports
```

Application 层继续表达 use case 和 command/query，不接收 HTTP request DTO，也不接收未来 gRPC/protobuf DTO。HTTP 和 gRPC transport 都只在边界层完成协议解析、输入规范化、错误语义映射和 application 输入映射。

## Target Boundary

未来若某个 feature 暴露真实 gRPC API，推荐布局为：

```text
user-service/internal/features/<feature>/transport/
  http/
    controller.go
    request.go
    response.go
    routes.go
    validation.go
  grpc/
    README.md or doc.go
    handler.go
    mapper.go
    validation.go
```

本次没有真实 gRPC API，因此不创建 handler、mapper、validation 或 generated code。若实现者选择新增占位目录，只能添加 README 或 package doc，说明准入条件和禁止事项。

## `transport/grpc` Responsibilities

`features/<feature>/transport/grpc` 只用于本服务入站 gRPC API：

- gRPC handler 或 service implementation。
- server-side protobuf request/response 与 application command/query/result 的映射。
- 传输边界输入校验和规范化。
- gRPC status code、metadata、deadline/cancellation 与 application 错误语义的边界适配。
- 与 feature-local application use case 的调用。

禁止放置：

- 外部 gRPC service client adapter。
- 未被真实 API 使用的 proto 或 generated code。
- feature 业务编排、跨 store 事务、密码校验、token 签发或持久化访问。
- Gin HTTP controller、HTTP response envelope、Swagger model 或 `/api/v1` route。
- 服务级 gRPC server lifecycle、listener、interceptor 或 Fx provider。未来如需 runtime wiring，应作为单独变更放在服务级 providers/bootstrap 边界中设计。

## Relationship To `transport/http`

`transport/http` 继续是当前唯一已实现的入站 API transport。它保留 Gin controller、route registration、HTTP request/response DTO、Swagger model、HTTP validation 和 response envelope 输出。

未来 `transport/grpc` 与 `transport/http` 应并列在同一 feature 的 `transport/` 下，并分别处理各自协议 DTO。二者都应映射到 application command/query，而不是互相调用对方 controller 或复用对方 request/response DTO。

可以共享的逻辑应优先放在 transport-neutral 的 application validators 或 domain/value object 中，而不是让 HTTP 和 gRPC transport 互相导入。

## Relationship To `internal/integration/grpc`

`internal/integration/grpc` 已经表示出站 gRPC 防腐层，服务于用户服务调用外部 gRPC service。该目录可以在未来承载 generated client 包装、外部 protobuf DTO 转换、metadata/deadline/status code 映射，并实现 feature-owned application ports。

它不承载本服务暴露的 gRPC API。入站 gRPC handler 属于对应 feature 的 `transport/grpc`，不是 integration。

## Documentation Updates

更新长期规则文档：

- `docs/ARCHITECTURE.md`
  - 在 feature 分层表中将 `transport/http` 描述扩展为当前 HTTP transport，并补充未来 `transport/grpc` 准入规则。
  - 在依赖规则中补充 `transport/grpc` 行，允许依赖 application、gRPC/protobuf 边界类型和 feature-local validation，禁止依赖 Ent、Redis、SQL、HTTP response envelope 和 external client adapter。
  - 在 integration 边界说明中强调 `internal/integration/grpc` 是出站 external client adapter，不是本服务 gRPC server transport。
  - 在 current constraints 中说明当前没有真实 gRPC API、proto、generated code 或 gRPC server runtime。
- `AGENTS.md`
  - 同步 Repository Shape、Repository Rules 或依赖表中关于 feature transport 的规则。

不要新增 `openspec/` 或 `docs/opsx/`。

## Verification Strategy

实现后检查：

```bash
rg -n "transport/grpc|integration/grpc|gRPC" docs/ARCHITECTURE.md AGENTS.md
find user-service/internal/features -path "*/transport/grpc/*" -type f -print | sort
rg -n "google.golang.org/grpc|\\.proto|protoc|buf.yaml|grpc.NewServer" .
git diff -- docs/ARCHITECTURE.md AGENTS.md user-service/internal/features docs/changes/add-user-service-grpc-transport-boundary
```

如果只修改 Markdown 或 README，不需要运行 Go 测试。若新增任何 Go package doc，则在 `user-service/` 运行：

```bash
go test ./...
```

## Risks And Mitigations

### Inbound and outbound gRPC confusion

Risk: future code places server handlers under `internal/integration/grpc`.

Mitigation: document that `integration/grpc` is outbound-only and `transport/grpc` is inbound-only.

### Premature generated code

Risk: adding proto/generated code before an API exists creates unused contracts and maintenance burden.

Mitigation: this change forbids proto generation and allows only docs/package docs until a real API is designed.

### Transport DTO leakage

Risk: application services start depending on protobuf or HTTP DTOs.

Mitigation: both transports must map into transport-neutral application command/query types before invoking use cases.
