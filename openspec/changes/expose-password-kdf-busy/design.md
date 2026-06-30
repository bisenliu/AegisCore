## Context

`common/security/password.Service` 通过实例级 `gate` 和 `queue` 限制 Argon2id KDF 的执行并发与总排队数量。user-service auth provider 为认证功能注入单个共享 password service，登录、强制改密和用户创建等路径都会消耗该资源预算。

当前登录凭据校验在 `VerifyContext` 返回任意错误时统一转换为 `authdomain.ErrInvalidCredentials`。这会把 `password.ErrPasswordKDFBusy` 伪装成密码错误，导致客户端无法按服务繁忙语义重试，登录失败 metrics 也无法区分安全失败和资源耗尽。

本 change 采用不兼容方案：KDF busy 保留为系统过载错误并在登录 HTTP 边界返回 `503 Service Unavailable`。真实用户名不存在、密码不匹配和用户状态不允许登录仍保持原认证失败语义。

## Goals / Non-Goals

**Goals:**

- 登录凭据校验遇到 `password.ErrPasswordKDFBusy` 时 MUST 保留该错误原因，不得转换为无效凭据。
- `POST /api/v1/auth/login` 在 KDF busy 时 MUST 返回 `503 Service Unavailable`，并在 OpenAPI 中声明该响应。
- 登录失败 metrics MUST 能独立标识 KDF busy，避免污染 credential invalid 统计。
- 共享错误契约 MUST 提供业务中立的服务不可用错误构造，供 user-service HTTP 边界复用。
- 保留服务内 Argon2 并发和队列门控作为最后资源保护线。

**Non-Goals:**

- 不取消或放宽默认 `argon2_concurrency` / `argon2_queue_size`。
- 不新增网关、Ingress、WAF、Kubernetes 或 Helm 限流配置。
- 不改变密码哈希参数、编码格式、数据库 schema、Redis key schema 或 refresh session 策略。
- 不把 KDF busy 映射为 `429 Too Many Requests`；429 留给按调用方、IP、用户、租户或设备维度的频率限制。
- 不引入新外部依赖或后台队列。

## Decisions

### 使用 503 表达 KDF 资源池繁忙

KDF busy 是服务实例本地资源池耗尽，不是某个调用方超过配额。HTTP 边界使用 `503 Service Unavailable` 更准确，客户端和网关可以把它识别为临时服务繁忙并按退避策略重试。

备选方案是返回 `429 Too Many Requests`。该方案被拒绝，因为 429 更适合业务限流或网关限流，会错误暗示请求方本身违反频率限制。

### 在 credentials 边界保留 busy 原因

`application/credentials` 是密码凭据校验的 owning boundary，必须在这里区分 KDF 资源错误与凭据错误。实现应对 `password.ErrPasswordKDFBusy` 单独记录 warn 级日志并原样返回；其他密码解析、输入或哈希异常仍可按无效凭据处理，避免泄露密码哈希状态。

备选方案是在 HTTP mapper 中识别包裹后的 `ErrInvalidCredentials`。该方案不可行，因为当前转换会丢失 busy 原因，且会继续污染 command 层 metrics reason。

### 在 common 提供业务中立服务不可用错误契约

`common/contract/errors` 已承载跨服务错误分类和 HTTP status 映射。新增服务不可用 helper 应保持业务中立，不包含 auth 专用消息；auth HTTP mapper 负责提供认证服务繁忙的公开消息。

备选方案是在 auth HTTP transport 内手写 `contracterrors.Error`。该方案会绕过共享契约，增加后续服务间错误码漂移风险。

### 保留服务内 KDF 门控

网关限流不能替代进程内资源预算。服务内门控用于限制单 Pod 同时执行 Argon2 的 CPU 和内存占用，并在网关漏配、内网绕过、重试风暴或副本流量不均时保护进程。

备选方案是取消队列或并发限制，只依赖网关。该方案会允许 handler goroutine 和 Argon2 工作内存无限增长，存在 OOM 和尾延迟失控风险。

### 更新 OpenAPI 生成物

登录接口对外响应语义发生不兼容变化，HTTP 注解和生成物必须同步更新。实现完成后运行 `make user-service-openapi-generate` 并检查生成物 diff。

## Risks / Trade-offs

- [Risk] 客户端当前只处理 401 登录失败，可能未处理 503。→ Mitigation：OpenAPI 明确声明 503，发布说明标注不兼容响应语义；客户端按临时失败退避重试。
- [Risk] 503 会暴露服务端局部资源繁忙状态。→ Mitigation：公开消息仅说明认证服务繁忙，不包含队列长度、并发槽位、用户名存在性或密码匹配信息。
- [Risk] KDF busy 指标增加后告警阈值需要重新设定。→ Mitigation：独立 metrics reason 使用稳定英文值，便于仪表盘和告警按 reason 过滤。
- [Risk] 如果只改 HTTP mapper 而不改 credentials，busy 原因仍会丢失。→ Mitigation：任务要求覆盖 credentials 单元测试、command metrics 测试和 controller HTTP 映射测试。
- [Risk] 新增共享错误 helper 可能影响所有服务可用错误码集合。→ Mitigation：保持 helper 业务中立，现有错误码和响应 envelope 不变。

## Migration Plan

1. 在 common 错误契约中新增业务中立的服务不可用错误构造和测试。
2. 在 auth credentials 边界保留 `password.ErrPasswordKDFBusy`，并补充日志和单元测试。
3. 在 login command metrics reason 中独立记录 KDF busy。
4. 在 auth HTTP mapper 和 controller 测试中将 KDF busy 映射为 503。
5. 更新登录 OpenAPI 注解并重新生成 OpenAPI 文档。
6. 运行相关 Go 测试、`make user-service-architecture-lint`、`make lint` 和 `make verify`。

回滚方式：恢复 credentials 对 KDF 错误统一映射无效凭据，移除 HTTP 503 映射和 KDF busy metrics reason，重新生成 OpenAPI。回滚不涉及数据库、Redis、部署清单或数据迁移。

## Open Questions

无。
