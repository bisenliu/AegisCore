## Context

`common/security/auth` 是 shared credential primitive 的拥有边界，当前 `JWTService.parse` 在成功完成 JWT 签名、过期时间、issuer 和 audience 校验后，会先检查 `claims.UserID == ""`，再调用 `uuid.Parse(claims.UserID)` 校验外部用户 ID 格式。空值和非 UUID 字符串当前都会返回 `ErrMissingUserID`，导致调用方和日志无法区分 claim 缺失与 claim 格式非法。

该变更只影响 common 模块的 JWT 凭证错误语义，不改变 `user-services` controller/service/infra 分层，不涉及 Ent、Redis、PostgreSQL、Fx 启动依赖、HTTP 响应信封或 runtime 配置。

## Goals / Non-Goals

**Goals:**

- 在 `common/security/auth` 中为非法 UUID 用户 ID 提供明确错误常量 `ErrInvalidUserID`。
- 保持缺失 `user_id` 继续返回 `ErrMissingUserID`。
- 让 `ParseToken`、`ParseRefreshToken`、`ParsePasswordChangeToken` 通过共享 `parse` 路径一致区分 `user_id` 缺失与格式非法。
- 用单元测试覆盖非法 UUID claim 的错误语义，避免回归。

**Non-Goals:**

- 不改变 JWT claim 字段、subject、签名算法、issuer/audience/expiration 校验方式。
- 不改变 HTTP 认证中间件对外返回的 HTTP 401、响应信封、业务错误码或公开错误文案。
- 不引入新的配置项、数据库变更、Redis key 变更或外部依赖。
- 不调整用户服务认证状态机、token version 校验或 session 管理逻辑。

## Decisions

- 在 `common/security/auth/jwt.go` 新增导出的 sentinel error `ErrInvalidUserID`。理由：该包已经通过 `ErrMissingSecret`、`ErrMissingUserID`、`ErrMissingTokenVersion` 等导出错误表达调用方可识别的凭证校验失败类型，新增同层级错误最小且符合现有模式。替代方案是在 UUID 解析错误外层 `fmt.Errorf` 包装原错误，但会迫使调用方依赖第三方 UUID 错误文本或类型，不利于稳定契约。
- 将 `uuid.Parse(claims.UserID)` 失败映射为 `ErrInvalidUserID`，空字符串仍映射为 `ErrMissingUserID`。理由：字段存在但格式非法与字段缺失是不同输入类别，错误命名应准确反映根因。替代方案是继续复用 `ErrMissingUserID`，但这正是本次修复要消除的误导。
- 保持 `sign` 路径也对非 UUID 输入返回 `ErrInvalidUserID`。理由：签发输入中的用户 ID 同样可能存在但格式非法，解析和签发应保持一致的共享凭证校验语义。替代方案是只修改 `ParseToken`，但会留下同一个包中同类输入错误语义不一致的问题。
- 测试放在 `common/security/auth/jwt_test.go`，使用现有 `signTestToken` helper 构造带非法 `user_id` claim 的签名 token。理由：该文件已经覆盖 JWT sign/parse 和 sentinel error 判断，新增 case 能以最小改动验证回归点。

## Risks / Trade-offs

- [Risk] 已有 Go 调用方可能把非法 UUID token 的错误与 `ErrMissingUserID` 做 `errors.Is` 匹配。→ Mitigation：HTTP 中间件对外仍归类为认证失败；内部 Go 错误语义修正属于更准确的共享凭证契约，proposal 明确兼容边界。
- [Risk] 新增导出错误常量增加公共 API 表面积。→ Mitigation：常量位于已有错误常量组内，名称和语义窄化到 JWT 用户 ID 格式非法，不引入新抽象。
- [Risk] 只改 common 测试可能遗漏 HTTP 层行为回归。→ Mitigation：实现不改变 middleware 错误映射；运行 common 测试验证凭证原语，必要时运行 user-services 测试确认下游兼容。
