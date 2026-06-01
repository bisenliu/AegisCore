## Why

`common/` 承载配置、日志、中间件、响应、校验和基础设施装配等跨服务能力，但当前存在硬编码运行时策略、部分边界条件未被防护、以及验证和日志实现文件职责过宽的问题。现在对共享层做系统性审查与重构，可以降低后续服务复用时的生产误配置风险，并减少公共能力演进的回归半径。

## What Changes

- 为 Redis 增加可配置 ping timeout，移除基础设施初始化中的隐藏 `5s` 策略，并与 PostgreSQL ping timeout 行为对齐。
- 为 CORS 与 trace-id 中间件增加可配置选项，集中维护 HTTP header、Gin context key、日志字段、响应消息和 validation tag/rule 等包内常量。
- 加固 `common/response` 的 nil error 处理，避免失败响应路径发生 panic。
- 拆分 `common/validation` 和 `common/logger` 中职责过宽的实现文件，在保持公开 API 稳定的前提下提升可读性和可测试性。
- 加固 JSON binding、反射 binding、默认 logger 并发访问和 trace-id 输入边界等代码质量问题。
- 增加显式 opt-in 的命名 Redis/PostgreSQL Fx provider helper，减少服务侧重复 wiring，但不自动连接未声明实例。
- 非目标：不引入认证、授权、支付、健康检查聚合或新的业务 API；不修改 `user-services/ent/` 生成代码；不增加配置文件字段校验或默认值设置能力。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 增加 Redis ping timeout、命名 datastore provider helper、logger 并发安全与文件组织约束。
- `api-response-contract`: 明确失败响应 helper 对 nil error 的安全行为，并保留统一信封和标准业务码契约。
- `request-validation`: 增强 binding 边界条件、字段解析能力和 validation 包内模块化组织要求。

## Impact

- 影响代码：`common/config/`、`common/infrastructure/`、`common/logger/`、`common/middleware/`、`common/response/`、`common/validation/` 及对应测试。
- 兼容性：公开 Go API 优先保持兼容；新增 helper 和 options 采用 opt-in；默认中间件行为应保持现有可用性，严格化行为需通过选项启用或明确记录。
- 外部可观察行为：不改变 `common/config.Load` 的职责，不增加配置字段校验或默认值设置；CORS、trace-id、validation 和响应错误码需保持既有 API 契约兼容。
- 依赖与系统：不新增外部运行时依赖；继续使用 Gin、Fx、Viper、Zap、Redis、PostgreSQL 和现有 validator 生态。
