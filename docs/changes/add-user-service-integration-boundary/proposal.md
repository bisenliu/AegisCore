# Add user-service integration boundary

## What

新增 `user-service/internal/integration` 作为用户服务访问外部系统的防腐层边界，并在长期架构文档中明确其职责、子目录规则和禁止事项。

目标结构：

```text
user-service/internal/
  integration/
    README.md
    http/
      doc.go or README.md
    grpc/
      doc.go or README.md
    events/
      doc.go or README.md
```

`integration` 用于承载用户服务对外部 HTTP、gRPC 和事件系统的 client adapter、协议 DTO 映射、外部错误归一化、重试/超时等调用边界规则。当前没有真实外部系统调用，因此本变更只建立目录边界、package docs 或 README 占位，并同步更新 `docs/ARCHITECTURE.md`。

实现时应清理或迁移当前空的 `user-service/internal/integrations` 占位目录，避免 singular/plural 两套边界并存。

## Why

用户服务当前已经按 feature-first 分层组织业务代码，`providers` 承载服务级运行时组装，`common` 承载跨服务稳定基础能力。但随着后续接入订单、支付、通知、消息总线或其他外部系统，代码容易出现两类漂移：

- 外部 client 被直接放进 feature 的 `infra`，导致 feature adapter 同时承担本服务持久化和外部系统协议转换。
- 外部调用被误放进 `providers`、`common`、`internal/shared` 或临时横向目录，模糊运行时组装、跨服务基础能力和业务编排的边界。

提前建立 `internal/integration` 边界，可以给外部系统防腐层一个明确落点，同时不承诺任何尚不存在的 order/payment client、消息 broker 依赖或跨服务契约。

## Scope

包括：

- 新增 `user-service/internal/integration` 边界说明。
- 建立 `integration/http`、`integration/grpc`、`integration/events` 的目录规则。
- 在没有真实外部系统调用的前提下，只添加 README 或 package doc 占位，避免未使用代码。
- 明确 `integration` 不属于 feature 内部业务编排，不拥有用例流程、登录状态机、跨 store 事务或 HTTP controller 行为。
- 明确 feature app service 仍通过消费侧 ports 表达外部能力需求，integration adapter 只在确有真实外部调用时实现这些最小接口。
- 更新 `docs/ARCHITECTURE.md`，把 `internal/integration` 加入 user-service 目录边界和依赖规则。
- 视需要同步更新根目录 `AGENTS.md`，保持长期规则一致。
- 清理当前空的 `user-service/internal/integrations` 目录，统一使用 singular `integration`。

不包括：

- 不实现 order、payment、notification 或其他真实外部 client。
- 不引入 Kafka、RabbitMQ、NATS 或其他 broker 依赖。
- 不新增 gRPC 代码生成、protobuf schema 或 OpenAPI client 生成。
- 不修改现有 user/auth feature 行为。
- 不改变 HTTP API、响应信封、认证中间件、配置 key、Ent schema 或 Atlas migration。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- 存在 `user-service/internal/integration`，并清晰说明其作为外部系统防腐层的责任边界。
- `integration/http`、`integration/grpc`、`integration/events` 的用途和准入条件被文档化。
- 当前没有真实外部调用时，只存在 README 或 package doc 占位，不新增会触发 unused warning 的 Go declarations。
- 不存在 singular `integration` 与 plural `integrations` 两套边界并存的问题。
- `integration` 文档明确不承载 feature 业务编排，也不替代 feature-owned app ports。
- `docs/ARCHITECTURE.md` 同步说明 user-service integration boundary、依赖方向和禁止事项。
- 如 `AGENTS.md` 中的仓库入口规则需要同步，也已更新。
- `user-service/` 下 `go test ./...` 通过，或本次仅文档/README 变更时说明无需运行 Go 测试的原因。
