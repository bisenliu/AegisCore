# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`AGENTS.md` 和本 change 的 `proposal.md`、`design.md`，确认本次只新增外部系统防腐层边界，不实现真实外部 client。
- [x] 新增 `user-service/internal/integration/README.md`，说明 integration 是用户服务访问外部系统的防腐层。
- [x] 新增 `user-service/internal/integration/http/README.md` 或最小 package doc，说明外部 HTTP client adapter 的准入条件和禁止事项。
- [x] 新增 `user-service/internal/integration/grpc/README.md` 或最小 package doc，说明外部 gRPC client adapter 的准入条件和禁止事项。
- [x] 新增 `user-service/internal/integration/events/README.md` 或最小 package doc，说明外部事件系统 adapter 的准入条件和禁止事项。
- [x] 清理空的 `user-service/internal/integrations` 目录，确保只保留 singular `integration` 边界。
- [x] 避免新增未使用 Go declarations；如使用 `doc.go`，只保留 package comment 和 package declaration。
- [x] 不新增 order、payment、notification 或其他真实外部系统 client。
- [x] 不引入 Kafka、RabbitMQ、NATS、protobuf、OpenAPI client generation 或其他外部集成依赖。

## Documentation

- [x] 更新 `docs/ARCHITECTURE.md`，在 user-service 目录职责中加入 `internal/integration`。
- [x] 更新 `docs/ARCHITECTURE.md`，说明 `integration/http`、`integration/grpc`、`integration/events` 的边界与准入条件。
- [x] 更新 `docs/ARCHITECTURE.md` 的 dependency rules，明确 integration 可以依赖和禁止依赖的内容。
- [x] 更新 `docs/ARCHITECTURE.md`，明确 integration 不承载 feature 业务编排，feature app ports 仍由消费侧 feature 拥有。
- [x] 如 `AGENTS.md` 中的 Repository Shape、Repository Rules 或 Key Entry Points 需要同步，补充 `internal/integration` 规则。
- [x] 确认长期规则文档仍声明不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `find user-service/internal/integration -maxdepth 3 -type f -print | sort`，确认 README 或 package docs 存在。
- [x] 运行 `test ! -d user-service/internal/integrations`，确认没有 plural boundary 残留。
- [x] 运行 `rg -n "internal/integration|integration/" docs/ARCHITECTURE.md AGENTS.md`，确认长期规则文档引用新边界。
- [x] 运行 `rg -n "Kafka|RabbitMQ|NATS|order|payment" user-service/internal/integration docs/ARCHITECTURE.md AGENTS.md`，确认只存在非目标说明，不存在真实 client 或 broker 依赖。
- [x] 如新增任何 Go package doc，在 `user-service/` 运行 `go test ./...`。
- [x] 检查 `git diff -- user-service/internal/integration user-service/internal/integrations docs/ARCHITECTURE.md AGENTS.md docs/changes/add-user-service-integration-boundary`，确认没有 HTTP API、配置 key、Ent schema、migration、generated code 或真实外部依赖变更。

## Review Notes

- [x] 确认 `integration` 没有导入 Gin response、Ent、Redis store、feature service implementation 或 provider internals。
- [x] 确认 feature app ports 仍由消费侧 feature 拥有，没有在 integration 下定义通用大接口。
- [x] 确认 `providers` 没有新增外部系统协议 DTO 或业务调用逻辑。
- [x] 确认 `common` 没有新增用户服务特定 external client helper。
- [x] 确认本变更没有新增 `openspec/` 或 `docs/opsx/`。
