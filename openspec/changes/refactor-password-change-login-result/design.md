## Context

强制改密登录是一个受限认证流程：凭据已经校验通过，系统会签发 subject 为 `password_change` 的短期受限 token，并创建一次性 password change session。它不同于无效凭据或系统错误，但也不是普通登录成功，调用方必须先引导用户完成改密，不能进入普通会话。

此前方案尝试把该分支表达为登录响应 data 中的 `status` 枚举。最终决策改为使用响应信封的稳定业务码：普通登录返回 `success=true/code=0`，强制改密登录返回 `success=false/code=20006`，data 只携带受限 token 载荷。这样前端按统一 envelope code 分支处理，不需要解析 token DTO 内的登录状态枚举。

## Goals / Non-Goals

**Goals:**

- 让 `TokenResult` 只表达 token 载荷，移除 `PasswordChangeRequired` 或等价 token 业务状态字段。
- 让登录 use case 返回 `LoginResult.PasswordChangeRequired`，该字段只用于 application 到 transport 的分支编排，不作为对外响应枚举。
- 让普通登录返回 HTTP `200 OK`、`success=true`、`CodeOK` 和普通 token 响应。
- 让强制改密登录返回 HTTP `200 OK`、`success=false`、`CodePasswordChangeRequired=20006`、`PasswordChangeRequired` 提示消息和受限 token 响应。
- 保留并规范化 `CodePasswordChangeRequired=20006` 作为认证类稳定业务码，不新增对应 reason 或错误构造函数。
- 更新 OpenAPI 注解和生成物，准确表达普通登录与强制改密登录的 envelope 差异。

**Non-Goals:**

- 不改变 password change token 的 subject、签发参数、TTL、解析校验或一次性 session 存储。
- 不改变强制改密后的改密 endpoint、token version 提升、refresh/logout 或 RBAC 行为。
- 不提供旧 `status` 响应枚举或旧 `password_change_required` 兼容字段。
- 不把强制改密登录改成 HTTP `401`、`403`、`409` 或 `202`。
- 不新增数据库 migration、部署清单、观测 dashboard 或跨模块错误映射注册表。

## Decisions

1. 登录分支归属于 `command.LoginResult`，不归属于 token issuer。
   - 选择：`LoginUseCase.Login` 返回 `*LoginResult`，其中包含 `PasswordChangeRequired bool` 和 `Tokens *authtokens.TokenResult`。
   - 理由：是否需要强制改密是登录流程结果，不是 JWT 签发器的 token 载荷属性。使用 bool 足以表达当前唯一分支，避免对外字符串枚举滞留在 application 层。
   - 备选：使用 `LoginStatus` 字符串枚举。该方案会继续保留 `authenticated/password_change_required` 枚举，容易再次泄漏到 HTTP DTO。

2. HTTP 登录响应复用 token DTO。
   - 选择：删除 `LoginResponse`，普通登录、强制改密和 refresh 共用 `TokenResponse`，字段为 `access_token`、`refresh_token,omitempty`、`token_type` 和 `expires_in`。
   - 理由：响应 data 只承载 token 载荷，流程分支由 envelope code 表达，重复 DTO 会造成 OpenAPI 和 mapper 维护成本。
   - 备选：保留无状态字段的 `LoginResponse`。该方案与 `TokenResponse` 完全重复，没有额外语义价值。

3. 强制改密登录使用稳定业务码 `CodePasswordChangeRequired=20006`。
   - 选择：`common/contract/errors` 定义 `CodePasswordChangeRequired Code = 20006`，但不新增 error reason 或 factory；controller 直接构造专用 envelope。
   - 理由：该分支需要被客户端稳定识别，但不是 `response.Fail` 错误出口。保留 code 而不引入 error constructor 可以避免把它误用成通用错误。
   - 备选：使用 `CodeOK` 配合 data status。该方案让前端需要解析业务载荷才能区分普通登录和强制改密。

4. 强制改密 envelope 使用 `success=false`。
   - 选择：`toPasswordChangeRequiredEnvelope` 返回 `success=false`、`code=20006`、`message=messages.PasswordChangeRequired`、`data=toTokenResponse(tokens)`。
   - 理由：调用方不能把该响应当作普通成功继续进入系统；`success=false` 与非 `CodeOK` 一致，便于统一拦截和流程分支。
   - 备选：`success=true/code=20006`。该方案会让 `success` 与 code 语义不一致，增加前端判断复杂度。

5. 登录失败仍通过 error 和 `response.Fail` 处理。
   - 选择：凭证错误、用户状态拒绝、KDF busy、token/session 创建失败继续返回 error。
   - 理由：这些是真正失败路径，应保留统一错误映射、HTTP status 和 message。

6. OpenAPI 通过注解和生成物同步，不手写生成文件。
   - 选择：修改 auth controller 注解和 DTO 后运行 `make user-service-openapi-generate`，由脚本更新 `user-service/docs/openapi.go/json/yaml`。
   - 理由：OpenAPI 生成物是仓库约定产物，手写会造成 drift。

## Risks / Trade-offs

- [Risk] 强制改密返回 `success=false` 但仍携带 data，可能与部分“失败无 data”的通用假设不一致。-> Mitigation：OpenAPI、规格和 E2E 明确该业务码可携带受限 token；该分支不走 `response.Fail`。
- [Risk] 删除 `status` 字段会影响已按枚举判断的调用方。-> Mitigation：这是本变更明确的 breaking change；调用方应迁移为判断 envelope `code=20006`。
- [Risk] `CodePasswordChangeRequired` 位于 common，但语义偏 user-service auth。-> Mitigation：认证类 200xx code 已包含 MFA、账号锁定等登录流程状态，本 code 保持在认证码段，并且不新增通用 error factory。
- [Risk] OpenAPI 只能声明 envelope schema，无法精确表达同一 HTTP 200 下 code 差异。-> Mitigation：在接口描述和测试中固化 `success=false/code=20006` 语义。

## Migration Plan

1. 更新 OpenSpec proposal、design、tasks 和 delta，使其描述 `success=false/code=20006` 最终方案。
2. 修改 `common/contract/errors`，新增或保留 `CodePasswordChangeRequired=20006` 和测试期望。
3. 修改 auth token result 与 issuer，使 `TokenResult` 只表达 token 载荷。
4. 修改登录 use case 接口、实现和测试，返回 `LoginResult.PasswordChangeRequired`。
5. 修改 auth HTTP DTO、mapper、controller 和测试，删除响应枚举，使用 `toPasswordChangeRequiredEnvelope` 渲染强制改密登录。
6. 更新 OpenAPI 注解并运行 `make user-service-openapi-generate`。
7. 运行 `gofmt`、相关 Go 测试、E2E 编译检查和架构/OpenAPI 验证。

回滚方式：恢复登录响应 data `status` 或旧强制改密 envelope 语义，并同步回滚 OpenAPI 与测试。由于不涉及数据库或部署资产，回滚不需要数据迁移。

## Open Questions

无。
