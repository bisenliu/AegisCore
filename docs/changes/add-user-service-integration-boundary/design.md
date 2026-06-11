# Design

## Overview

本变更新增 `user-service/internal/integration`，用于表达用户服务访问外部系统的防腐层。它是服务内边界，不是跨服务共享模块，也不是 feature 业务编排层。

目标责任划分：

```text
features/<feature>/app
  -> feature-owned ports
  -> integration/* adapter when a real external system exists

features/<feature>/transport/http
  -> app command/query

features/<feature>/infra/postgres|redis
  -> service-owned persistence resources

providers
  -> service runtime wiring only

integration
  -> external system protocol/client boundary only
```

`integration` 的核心作用是隔离外部系统协议、传输细节和错误语义，使 feature app service 只依赖自己拥有的最小 port，而不直接依赖第三方 SDK、HTTP client、gRPC generated client、broker producer 或外部 DTO。

## Target Package Layout

```text
user-service/internal/integration/
  README.md
  http/
    README.md or doc.go
  grpc/
    README.md or doc.go
  events/
    README.md or doc.go
```

当前没有真实外部系统调用，因此推荐使用 README 占位。若实现者选择 `doc.go`，文件应只包含 package comment 和 `package <name>`，不定义未使用常量、变量、接口或 struct。

推荐 package names：

- `integration/http` -> `package httpintegration`
- `integration/grpc` -> `package grpcintegration`
- `integration/events` -> `package eventsintegration`

也可以只放 README，等真实 adapter 出现时再创建 Go package。若使用 Go package docs，避免包名与标准库 `net/http` 或 `google.golang.org/grpc` 在调用处产生歧义。

## Directory Semantics

### `integration/http`

用于外部 HTTP API client adapter：

- 外部 request/response DTO。
- 外部状态码和错误体到 domain/app 错误的转换。
- per-system base URL、timeout、auth header、idempotency key、retry/backoff 等传输边界。
- 对 feature app port 的实现。

不得承载 Gin controller、用户服务 HTTP route、`common/http/response` 输出、或本服务 HTTP API DTO。

### `integration/grpc`

用于外部 gRPC service client adapter：

- generated client 包装。
- metadata、deadline、status code 映射。
- protobuf DTO 和 feature app command/result 的转换。
- 对 feature app port 的实现。

不得承载本服务 gRPC server 入口、未使用的 protobuf 生成代码或未来设想的服务定义。

### `integration/events`

用于外部事件系统的 producer/consumer adapter 边界：

- 事件 envelope 与外部 topic/subject/stream 映射。
- broker-specific producer/consumer wrapper。
- 重试、幂等、投递结果和反序列化错误归一化。
- 对 feature app port 或应用事件处理入口的适配。

当前不引入 Kafka、RabbitMQ、NATS 或其他 broker 依赖。没有真实 broker 时只保留文档占位。

## Relationship To Feature Layers

Feature app service 仍拥有消费侧 ports。例如未来用户资料 feature 需要调用外部画像系统，应在 `internal/features/user/app/ports.go` 定义最小接口，再由 `internal/integration/http/<system>` 或 `internal/integration/grpc/<system>` 的 adapter 实现。

`integration` 不应定义为了 adapter 自身方便而扩张的大接口。接口归消费侧 feature app 层所有，adapter 只实现接口。

Feature `infra/postgres` 和 `infra/redis` 继续表示用户服务拥有的持久化资源访问。外部系统调用不应混入这些目录，除非它们只是为该 feature 自己的数据库或 Redis adapter 做内部存储访问。

## Relationship To Providers

`internal/providers` 仍只负责服务级 Fx provider：

- Gin engine。
- HTTP route registration。
- JWT service。
- PostgreSQL/Redis named resources。
- Ent clients。

当未来出现真实 external client 时，可以由 `providers` 或 feature module 负责 Fx wiring，但 client 实现和协议转换应放在 `integration`。`providers` 不承载外部 API DTO、状态码映射或调用状态机。

## Relationship To Common

`common` 只承载跨服务稳定契约和基础能力。外部系统 client adapter 通常是用户服务特定集成，不应因为未来可能复用而放入 `common`。

只有当某个外部系统契约已经成为多个服务共享的稳定平台能力，并且没有业务语义泄露时，才可以讨论迁入 `common` 的对应分类目录。该判断不属于本变更。

## Existing Placeholder Directories

当前工作树中存在空目录：

```text
user-service/internal/integrations/
user-service/internal/jobs/
user-service/internal/messaging/
```

本变更只处理 integration boundary。实现时应删除或不再保留空的 `internal/integrations`，统一使用 `internal/integration`。`jobs` 与 `messaging` 如果仍为空且不在当前架构文档中声明，可以在实现时保持不动或作为单独清理事项，不要把本变更扩大成后台任务或消息架构设计。

`user-service/internal/messages` 当前承载用户可见消息常量，不属于本次 integration boundary。

## Documentation Updates

Update long-lived docs:

- `docs/ARCHITECTURE.md`
  - 在 module boundaries 或 feature organization 之后补充 `user-service/internal/integration` 责任。
  - 在 dependency rules 中加入 integration 行，说明允许依赖外部 SDK/client、feature app ports、domain 值对象和 common runtime primitives，禁止依赖 Gin response、Ent、feature service 业务编排和 service-owned persistence adapter。
  - 在 infrastructure 或 constraints 中说明当前没有 order/payment client 或 broker dependency。
- `AGENTS.md`
  - 如仓库入口规则需要同步，加入 `internal/integration` 目录说明和禁止事项。

不要新增 `openspec/` 或 `docs/opsx/`。

## Verification Strategy

Implementation should verify:

```bash
find user-service/internal/integration -maxdepth 3 -type f -print | sort
test ! -d user-service/internal/integrations
rg -n "internal/integration|integration/" docs/ARCHITECTURE.md AGENTS.md
rg -n "Kafka|RabbitMQ|NATS|order|payment" user-service/internal/integration docs/ARCHITECTURE.md AGENTS.md
```

The last scan is not expected to be completely empty because docs may mention non-goals, but it should not reveal real client implementation or dependencies.

If only Markdown/README files are added, Go tests are optional. If any Go package docs are added, run:

```bash
cd user-service
go test ./...
```

## Risks And Mitigations

### Boundary name conflict

Risk: both `internal/integration` and `internal/integrations` remain and future contributors choose different locations.

Mitigation: remove the empty plural directory during implementation and document singular `integration` as the only supported boundary.

### Premature abstraction

Risk: adding interfaces or generic clients before a real external system exists creates dead code.

Mitigation: this change only allows README/package docs. Real clients must arrive with a concrete feature port and tests.

### Feature business logic drift

Risk: external adapter code starts owning use case decisions because it is outside feature directories.

Mitigation: architecture docs must state that `integration` maps protocol and errors; feature app services own business orchestration and command/query handling.
