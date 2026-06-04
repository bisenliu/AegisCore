## Context

当前 `request-validation` 能力由 `common/validation` 提供，包内同时包含 validator 初始化、结构体校验、字段名解析、错误归一化、反射 binder、Gin request binding、失败响应和日志出口。Controller 依赖共享 validation 是合理边界，但 `common/validation` 直接依赖 Gin、`common/response` 和 `common/logger`，使纯校验消费者也被动继承 HTTP transport 依赖。

现有外部契约必须保持不变：controller 请求绑定失败仍返回 `common/response.Envelope`，validator tag 失败仍返回 HTTP 400、业务码 `10001` 和字段级 `errors` 明细，非字段级 binding 错误仍返回 HTTP 400 与通用请求错误码。

## Goals / Non-Goals

**Goals:**

- 将 Gin request binding 与 `BindOrAbort` HTTP 适配逻辑迁移到新的 `common/ginvalidation` 包。
- 保持 `common/validation` 专注于 validator 初始化、结构体校验、字段名解析、错误归一化和自定义校验规则。
- 保持 controller/service/repository 分层，HTTP 解析和响应仍位于 controller 边界。
- 保持现有 HTTP 响应信封、错误码、公开消息、日志字段和测试行为兼容。

**Non-Goals:**

- 不更换 Gin、不引入新的 HTTP 框架抽象。
- 不改变用户服务路由、DTO JSON 字段、业务错误码或数据模型。
- 不调整 Ent schema、Atlas migration、数据库连接、Redis 连接或服务启动依赖。
- 不把用户服务特定请求清洗规则迁入 `common/validation`。

## Decisions

1. 新增扁平包 `common/ginvalidation`，而不是创建 `common/validation/gin` 子目录。

   理由：Gin 绑定和 abort 行为属于 transport adapter，不是纯 validation core 的内部实现。扁平包路径让 controller 显式表达依赖 HTTP/Gin 适配层。

   替代方案：使用 `common/validation/gin`。该方案会让 Gin 适配看起来仍隶属于核心 validation，降低边界清晰度。

2. `common/validation` 保留错误归一化，向适配层暴露错误分类 API。

   理由：JSON 类型错误、validator tag 错误、字段显示名解析和 enum 翻译都依赖 validator 内部语义，应继续由核心包负责。`common/ginvalidation` 只根据分类决定输出 validation failed envelope 还是 bad request envelope。

   替代方案：把错误归一化也迁到 `common/ginvalidation`。该方案会让非 Gin 调用方难以复用一致的校验错误模型。

3. Controller 依赖 `*validation.Validator` 和 `ginvalidation` binder 函数，而不是复制绑定逻辑。

   理由：维持现有 Fx 注入方式，减少构造函数和 bootstrap 改动，同时避免在每个 controller 中重复日志、响应和 abort 逻辑。

   替代方案：新增 `ginvalidation.Binder` struct 并通过 Fx 注入。该方案更面向对象，但会扩大依赖注入改动范围；当前拆分目标可用函数式适配更小改动完成。

## Risks / Trade-offs

- [Risk] 包迁移导致 controller 或测试遗漏旧 import。→ 通过 `go test ./...` 在 `common` 和 `user-services` 模块验证。
- [Risk] 错误分类 API 暴露过多 validation 内部细节。→ 暴露聚合的 `Failure`/`ClassifyError`，避免导出多个内部 helper。
- [Risk] `common/ginvalidation` 仍依赖 `common/response` 和 Gin。→ 这是适配层的明确职责，核心风险是避免该依赖留在 `common/validation`。
- [Risk] 现有 capability 文档描述“共享校验器 BindOrAbort”时路径发生变化。→ 在 delta spec 中要求核心校验与 Gin 适配分离，并保持行为兼容。
