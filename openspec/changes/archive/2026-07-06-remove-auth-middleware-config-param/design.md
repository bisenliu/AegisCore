## Context

`common/http/middleware.AuthWithTokenVersionValidator` 是共享 JWT 认证 middleware 的导出入口，当前签名包含 `log`、`jwtService`、`config.AuthConfig` 和 `TokenVersionValidator`。实际实现中 `config.AuthConfig` 使用空白标识符接收，JWT 配置已在 `auth.NewJWTService(config.AuthConfig)` 构造阶段被消费，middleware 运行时只依赖 `JWTService` 解析 token，并在存在 validator 时校验 token version。

user-service 通过 `user-service/internal/router/router.go` 挂载受保护路由，并由 `user-service/internal/providers/routes.go` 将服务配置适配为 router 参数。当前 `RouteParams.AuthConfig` 只用于继续传给未使用的 middleware 参数，导致 common API、router 参数和测试 fixture 都暴露了不存在的行为依赖。

## Goals / Non-Goals

**Goals:**

- 移除 `AuthWithTokenVersionValidator` 中未使用的 `config.AuthConfig` 参数。
- 同步收敛 user-service router/provider 的参数传递和测试构造，避免继续传播无效依赖。
- 保持 JWT 解析、认证失败响应、token version 校验、日志字段、Gin context 写入和受保护路由挂载行为不变。
- 通过单元测试覆盖 common middleware、router 注册和 provider 路由装配的编译与行为回归。

**Non-Goals:**

- 不改变 `auth.NewJWTService(config.AuthConfig)` 的构造方式。
- 不改变 `config.AuthConfig` 结构体、配置文件格式或配置校验规则。
- 不改变 HTTP 外部响应契约、OpenAPI 文档、数据库 schema、migration、部署清单或观测资产。
- 不保留兼容重载、废弃 wrapper 或临时适配函数。

## Decisions

- 决策：直接修改 `AuthWithTokenVersionValidator` 函数签名，删除 `config.AuthConfig` 参数。
  备选方案：保留参数并仅改名或添加注释。该方案仍会让调用方误判配置参与认证行为，无法消除 API 噪音，因此不采用。

- 决策：删除 `router.RouteParams.AuthConfig` 字段，并更新 provider 适配层不再填充该字段。
  备选方案：仅更新 middleware 调用但保留 router 参数。该方案会在 router API 中继续保留无消费者配置，形成新的噪音，因此不采用。

- 决策：测试只同步新签名和参数结构，不新增生产分支或兼容 helper。
  备选方案：新增 wrapper 让旧调用继续编译。该变更仅影响仓库内调用方，且目标是最终不保留兼容方案，因此不采用。

- 决策：规格归属 `shared-platform-primitives`，因为变更对象是 `common/http/middleware` 和共享认证 helper API 治理。
  备选方案：归入 `auth-session-management`。该能力描述 user-service 认证会话行为，本次不改变会话或 token version 行为，因此不采用。

## Risks / Trade-offs

- [Risk] 函数签名 breaking change 会导致未同步调用点编译失败。→ Mitigation：使用全仓搜索 `AuthWithTokenVersionValidator(`，同步更新 common 测试和 user-service router 调用，并运行相关包测试。
- [Risk] 删除 router 参数时遗漏 provider 或测试 fixture 字段。→ Mitigation：搜索 `AuthConfig:`、`params.AuthConfig` 和 `RouteParams{`，运行 `go test ./user-service/internal/router ./user-service/internal/providers`。
- [Risk] 误删 JWT 配置真实消费路径。→ Mitigation：仅删除 middleware 的未使用参数，保留所有 `auth.NewJWTService(config.AuthConfig)` 构造调用。
- [Risk] 规格误表达为认证行为变更。→ Mitigation：delta spec 只描述共享 helper API 治理，明确运行时认证语义不变。

## Migration Plan

1. 修改 common middleware 签名并删除多余 import。
2. 更新 user-service router/provider 参数结构和调用点。
3. 更新 common middleware、router registration 和 provider routes 相关测试构造。
4. 运行 `go test ./common/http/middleware ./user-service/internal/router ./user-service/internal/providers`。
5. 若需要回滚，恢复函数签名、router 参数字段和调用点；由于不涉及数据或外部 HTTP 契约，无数据库或部署回滚步骤。

## Open Questions

无。
