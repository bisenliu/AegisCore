## Context

`common/middleware/auth.go` 已经将认证失败响应统一为 `unauthenticatedMessage`，并将缺失 header、header 格式错误、空 token、token 校验失败等日志提升为 error。现有 OpenSpec 主规格只要求未认证场景返回 HTTP 401、统一信封和 `CodeUnauthenticated`，没有约束具体认证失败文案或日志级别，因此该已完成实现变更不需要同步修改主规格。

`common/jwt.Service.ParseToken` 当前基于 `github.com/golang-jwt/jwt/v5` 解析 HMAC JWT，并启用 expiration required、可选 issuer 和 audience 校验。认证中间件目前把所有缺失、格式错误、签名错误、过期、issuer/audience 不匹配和缺少 user_id 的场景统一映射为 `CodeUnauthenticated`。

## Goals / Non-Goals

**Goals:**

- 在 `common/response` 中新增当前认证流程能够真实产生并可靠返回的 token 细分错误码。
- 在认证中间件中区分缺失认证信息、token 非法与 token 过期，仍保持 HTTP 401 和统一对外文案。
- 保持错误码定义、Gin helper、应用错误构造函数和测试一致。
- 更新 `api-response-contract` delta，确保标准错误码契约包含新增认证错误码。

**Non-Goals:**

- 不实现 refresh token、token 黑名单、会话撤销、MFA、账号冻结或账号封禁能力。
- 不新增 `CodeTokenRevoked`、`CodeMFARequired`、`CodeUserAccountLocked`，直到对应运行时流程和数据模型存在。
- 不改变路由鉴权范围、白名单配置、JWT 签发逻辑或数据模型。
- 不暴露底层 JWT 解析错误详情给 API 调用方。

## Decisions

1. 只新增 `CodeTokenInvalid = 20001` 与 `CodeTokenExpired = 20002`。

   理由：当前代码能直接识别 header/token 格式错误、签名或 claims 校验失败，以及 `jwt/v5` 的过期错误。`Revoked/MFA/Locked` 需要黑名单、MFA 状态机或账号状态字段支持，仓库目前没有对应能力，提前定义会导致契约先于行为存在。

   备选方案：一次性添加五个错误码。该方案被拒绝，因为会引入未使用常量和误导前端以为这些场景已经可返回。

2. HTTP status 保持 `401 Unauthorized`，业务 code 细分。

   理由：缺失认证信息、token 非法和 token 过期都属于认证失败，HTTP 语义不需要拆分；前端需要差异化处理时读取响应信封 `code` 即可。

   备选方案：过期 token 使用其他 HTTP status。该方案被拒绝，因为会偏离通用认证失败语义并增加调用方兼容成本。

3. API message 继续使用统一 `unauthenticatedMessage`。

   理由：统一文案满足当前产品体验，并避免泄露签名、issuer、audience、claims 缺失等具体校验细节。细分行为由数字 code 表达。

   备选方案：按错误类型返回不同文案。该方案被拒绝，因为用户已明确统一未认证响应信息，且文案不是前端自动处理的稳定依据。

4. 缺失 `Authorization` header 继续使用 `CodeUnauthenticated = 20000`。

   理由：缺失认证信息不是 token 格式错误、非法或过期；保留通用未认证错误码能维持现有语义，并把 `CodeTokenInvalid` 限定为已提供但不能通过解析或校验的 token。

   备选方案：缺失 header 也返回 `CodeTokenInvalid`。该方案被拒绝，因为会扩大 token invalid 的语义并降低错误码可读性。

5. JWT 错误分类放在 `common/middleware` 或 `common/jwt` 的共享层，不放到 `user-services`。

   理由：认证中间件和 JWT service 均属于 `common` 共享基础能力，`user-services` 应只消费统一响应契约，避免重复实现错误映射。

## Risks / Trade-offs

- [Risk] 已有测试或调用方断言所有认证失败 code 都是 `20000` → Mitigation：更新仓库内测试；对外说明 2xxxx 为认证错误族，前端可按 `20002` 触发刷新逻辑。
- [Risk] `jwt/v5` 错误包装方式变化导致过期错误识别失败 → Mitigation：使用 `errors.Is(err, jwtv5.ErrTokenExpired)` 并增加过期 token 单元测试。
- [Risk] 缺少 secret 属于服务配置错误但发生在认证请求路径中 → Mitigation：继续返回 401 对外安全文案，内部日志保留 error 和底层原因；不把配置错误暴露给调用方。
- [Risk] 未来增加 revoked、MFA、locked 后需要扩展 code → Mitigation：预留连续 20003-20005 语义，但不在本变更中发布公共契约。

## Migration Plan

实现时先更新 `common/response` 错误码和构造函数，再更新认证中间件映射和测试，最后运行 `go test ./...` 验证 `common` 与 `user-services`。回滚时可恢复中间件统一使用 `CodeUnauthenticated`，并移除新增错误码及 OpenSpec delta。

## Open Questions

无。本变更只覆盖当前可由 JWT 解析流程直接判断的认证失败细分。
