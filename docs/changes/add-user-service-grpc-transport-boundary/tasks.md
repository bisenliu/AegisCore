# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`AGENTS.md` 和本 change 的 `proposal.md`、`design.md`，确认本次只建立未来入站 gRPC transport 边界。
- [x] 不引入 gRPC runtime、protobuf、buf、protoc、generated code 或服务级 gRPC server wiring。
- [x] 如决定创建 feature-level `transport/grpc` 占位，只添加 README 或 package doc；不得添加空 handler、空 interface、空 service 或未使用 declarations。
- [x] 保持现有 `transport/http` controller、routes、request/response DTO、Swagger 和 `/api/v1` 行为不变。
- [x] 保持 `internal/integration/grpc` 作为出站外部 gRPC client adapter 边界，不向其中添加本服务 gRPC server 逻辑。

## Documentation

- [x] 更新 `docs/ARCHITECTURE.md` 的 feature 分层说明，明确 `transport/http` 是当前已实现 HTTP transport，未来 `transport/grpc` 可用于本服务入站 gRPC API。
- [x] 更新 `docs/ARCHITECTURE.md`，说明 `transport/grpc` 的职责：gRPC handler、server-side protobuf mapping、边界 validation、application command/query 映射。
- [x] 更新 `docs/ARCHITECTURE.md`，说明 `transport/grpc` 的禁止事项：外部 gRPC client、业务编排、持久化访问、HTTP controller/response、未使用 proto/generated code。
- [x] 更新 `docs/ARCHITECTURE.md` 的依赖规则，补充 `transport/grpc` 可以依赖和禁止依赖的内容。
- [x] 更新 `docs/ARCHITECTURE.md`，明确 `internal/integration/grpc` 是出站 external client adapter，不是本服务 gRPC server transport。
- [x] 更新 `docs/ARCHITECTURE.md` 的当前约束，说明当前没有真实 gRPC API、proto、generated code 或 gRPC server runtime。
- [x] 如 `AGENTS.md` 中的 Repository Shape、Repository Rules 或依赖表需要同步，补充 feature-local `transport/grpc` 规则。
- [x] 确认长期规则文档仍声明不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `rg -n "transport/grpc|integration/grpc|gRPC" docs/ARCHITECTURE.md AGENTS.md`，确认长期规则文档覆盖新边界。
- [x] 运行 `find user-service/internal/features -path "*/transport/grpc/*" -type f -print | sort`，确认如有占位则只有 README 或 package doc。
- [x] 运行 `rg -n "google.golang.org/grpc|\\.proto|protoc|buf.yaml|grpc.NewServer" .`，确认没有引入 gRPC/protobuf/server runtime。
- [x] 检查 `git diff -- docs/ARCHITECTURE.md AGENTS.md user-service/internal/features docs/changes/add-user-service-grpc-transport-boundary`，确认没有 HTTP API、配置 key、Ent schema、migration、generated code 或真实 gRPC API 变更。
- [x] 如新增任何 Go package doc，在 `user-service/` 运行 `go test ./...`；如仅 Markdown/README 变更，记录无需运行 Go 测试。

## Review Notes

- [x] 确认 application command/query 不接收 HTTP request DTO 或未来 protobuf DTO。
- [x] 确认 HTTP 与未来 gRPC transport 不互相导入对方 DTO/controller。
- [x] 确认 `internal/integration/grpc` 没有承载入站 server handler。
- [x] 确认本变更没有新增 `openspec/` 或 `docs/opsx/`。
