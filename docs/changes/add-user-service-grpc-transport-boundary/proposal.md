# Add user-service gRPC transport boundary

## What

为未来用户服务提供入站 gRPC transport 建立清晰边界，明确 feature-local `transport/grpc` 与现有 `transport/http`、`internal/integration/grpc` 的关系。

目标是在长期架构文档中说明：

- `user-service/internal/features/<feature>/transport/http` 承载当前 Gin HTTP controller、route、HTTP DTO、Swagger model 和 HTTP 边界校验。
- `user-service/internal/features/<feature>/transport/grpc` 是未来本服务暴露入站 gRPC API 时的放置位置。
- `user-service/internal/integration/grpc` 继续只表示用户服务调用外部 gRPC service 的 client adapter，不承载本服务 gRPC server、handler 或 route 逻辑。

当前没有真实 gRPC API，因此本变更只应新增文档或最小 package doc，不引入 gRPC dependency、不生成 proto、不实现 gRPC server。

## Why

仓库已经建立 feature-first 分层，并且现有外部系统防腐层使用 `internal/integration/grpc` 表示出站 gRPC client adapter。后续如果用户服务需要暴露 gRPC API，容易出现两个边界混淆：

- 把本服务入站 gRPC handler 放入 `internal/integration/grpc`，导致出站防腐层承载 server transport。
- 把 gRPC DTO 或 handler 放入 HTTP transport 目录，导致 `transport/http` 同时表达两种协议。

提前声明 `features/<feature>/transport/grpc` 的准入条件，可以让未来 gRPC API 与 HTTP API 一样贴近 feature 边界，同时保持 application command/query 不依赖具体传输协议。

## Scope

包括：

- 更新 `docs/ARCHITECTURE.md`，说明 feature transport 可以按协议拆分为 `transport/http` 和未来 `transport/grpc`。
- 明确 `transport/grpc` 仅用于本服务入站 gRPC API 的 handler、server-side DTO/proto mapping、validation 和 application command/query 映射。
- 明确没有真实 gRPC API 时，只允许 README 或 package doc 占位，不新增空 handler、空 interface、空 service 或未使用 Go declaration。
- 明确 `internal/integration/grpc` 与 `features/<feature>/transport/grpc` 的区别：前者是出站 external client adapter，后者是入站 service transport。
- 如需要，同步更新 `AGENTS.md`，保持仓库入口规则和长期架构文档一致。

不包括：

- 不引入 `google.golang.org/grpc` 或其他 gRPC runtime 依赖。
- 不新增 `.proto` 文件、buf 配置、protobuf generated code 或 code generation 流程。
- 不实现 gRPC server lifecycle、listener、reflection、health service、interceptor 或 Fx provider。
- 不改变现有 Gin HTTP route、middleware、controller、Swagger、response envelope 或 `/api/v1` 行为。
- 不新增任何真实 gRPC API。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `docs/ARCHITECTURE.md` 明确记录 feature-local `transport/grpc` 的未来放置规则。
- `docs/ARCHITECTURE.md` 明确区分入站 `features/<feature>/transport/grpc` 和出站 `internal/integration/grpc`。
- 如创建 `transport/grpc` 占位，只包含 README 或 package doc，不包含业务代码、空 service、空 handler 或未使用 declarations。
- `AGENTS.md` 如涉及 feature transport 规则，已同步更新。
- 没有新增 gRPC/protobuf dependency、proto 文件、generated code、server provider 或 runtime 配置。
- 现有 HTTP 服务不受影响；本次若仅文档/README 变更，可说明无需运行 Go 测试。
