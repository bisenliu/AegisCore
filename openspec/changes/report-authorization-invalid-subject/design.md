## Context

当前 RBAC 授权链路由 HTTP middleware 解析认证上下文，permission HTTP adapter 调用 application authorizer，再委托内存授权 engine 执行 Casbin 判断。`authorization.Enforce` 接收字符串形式的认证用户 ID，并在内部转换为 `uuid.UUID` 供 engine 使用。

现状中，用户 ID 解析失败会返回 `(false, nil)`。该路径能做到 fail closed，但会让上层 adapter 把解析失败误判为普通策略拒绝，导致内部错误语义、测试断言和后续观测都无法区分非法 subject 与无权限访问。

该变更只涉及 user-service permission feature 的授权语义和测试，不引入新外部依赖，不改变数据库、OpenAPI、部署清单或 Casbin policy 数据模型。

## Goals / Non-Goals

**Goals:**

- 让 permission application authorizer 在认证用户 ID 非法时返回明确错误。
- 保持授权链路 fail closed，非法 subject 不得进入底层 engine，也不得放行业务请求。
- 让 HTTP adapter 不再把非法 subject 折叠为普通 `commoncasbin.ErrDenied`。
- 用单元测试覆盖 application 层错误语义和 HTTP 层响应映射。

**Non-Goals:**

- 不改变 JWT 签发、解析或 token version 校验规则。
- 不改变权限目录、角色绑定、policy loader、policy sync 或 seed 行为。
- 不新增 common 共享错误类型；该错误属于 user-service permission 授权应用语义。
- 不修改数据库 schema、migration、OpenAPI 生成物、部署资产或观测 dashboard。

## Decisions

- 在 `user-service/internal/features/permission/application/authorization` 内定义 permission 专属错误。理由：非法 subject 是该授权服务对入站认证上下文的应用层约束，不属于 `common/security/casbin` 的无业务三元组能力，也不需要进入 `common` 形成跨服务契约。备选方案是在 common casbin 包新增错误，但会把 user-service 的 UUID subject 约束泄露到通用 Casbin primitive 中。
- `uuid.Parse` 失败时返回 `false` 和可通过 `errors.Is` 识别的错误。理由：`false` 保持 fail closed，错误值保留诊断语义，调用方可以按场景映射。备选方案是继续返回 `(false, nil)`，但无法满足本次规格要求。
- HTTP adapter 将非法 subject 映射为认证上下文无效路径，而不是 `ErrDenied`。理由：该场景不是 Casbin 策略拒绝，而是认证后上下文不满足授权服务前置条件；对外仍应拒绝请求，对内应保留错误分类。备选方案是返回 InternalError，但标准 HTTP 链路中非法 user ID 正常会被 JWT 认证提前拦截，若到达授权层更接近认证上下文异常，使用认证失败路径更符合边界语义。
- 测试直接基于现有 fake engine 和 fake authorizer 扩展，不新增生产接口或测试专用分支。理由：现有代码已经具备可测试性，额外抽象会增加无业务价值的复杂度。

## Risks / Trade-offs

- 非标准测试或自定义 middleware 直接写入非法 `user_id` 时，HTTP 响应可能从 `403 Forbidden` 变为认证上下文无效响应。→ 通过测试明确该行为，并在 proposal/spec 中标注这是错误语义修正，不改变正常 JWT 链路。
- 如果错误只用格式化字符串而不暴露哨兵错误，adapter 无法稳定识别。→ 使用包内导出的哨兵错误，并用 `fmt.Errorf("%w", ...)` 包装底层解析错误上下文。
- 如果把错误放入 common 包，会扩大共享契约。→ 错误定义保留在 permission application authorization 包内，HTTP adapter 作为同 feature 边界消费者进行映射。

## Migration Plan

实现时先更新 application authorizer 错误返回和对应单元测试，再更新 HTTP adapter 错误映射和 transport 测试。无需数据迁移、配置迁移或部署顺序调整；回滚可以恢复原有 `(false, nil)` 逻辑和测试断言，但会重新引入错误语义吞噬问题。

验证方式包括运行 permission 授权相关 Go 测试和 `make user-service-architecture-lint`。若仅改 Go 代码且不触及 OpenAPI 注解，无需运行 OpenAPI 生成。

## Open Questions

无。
