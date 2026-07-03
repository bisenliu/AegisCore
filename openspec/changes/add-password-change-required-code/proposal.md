## Why

当前强制改密登录路径以 `200 OK` 加 `password_change_required: true` 表达业务状态，前端需要依赖响应数据字段区分普通登录成功和受限改密 token。该语义属于认证状态分支，应纳入统一错误码体系，避免成功响应承载需要客户端特殊处理的受限状态。

## What Changes

- 新增共享错误码 `CodePasswordChangeRequired Code = 20006`，用于表达凭据校验通过但账号必须先修改密码。
- 调整登录 HTTP 响应：当用户需要强制改密时，返回带受限 `password_change` token 的统一 envelope，并使用 `CodePasswordChangeRequired`，而不是 `CodeOK` 加 `password_change_required: true` 作为成功分支。
- 保持安全语义不变：强制改密登录仍只签发 subject 为 `password_change` 的受限 token，不创建普通 refresh session，不返回 refresh token。
- 更新 OpenAPI、单元测试和 e2e 覆盖，使客户端可通过稳定错误码识别强制改密分支。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`：共享错误契约新增认证类错误码 `CodePasswordChangeRequired = 20006`，并要求公共响应 helper 能渲染该错误码。
- `auth-session-management`：登录强制改密分支从 `CodeOK` 加业务 flag 调整为携带 `CodePasswordChangeRequired` 的受限 token 响应。

## Impact

- 受影响代码：`common/contract/errors`、`common/contract/response` 使用侧、`user-service/internal/features/auth/application/command`、`user-service/internal/features/auth/transport/http`、auth controller/use case 测试和 e2e 测试。
- API 契约：`POST /api/v1/auth/login` 的强制改密响应 envelope code 变更为 `20006`，响应数据仍需要携带受限 access token、token type、过期时间，且不得携带 refresh token。
- OpenAPI：错误码枚举和登录接口响应说明需要重新生成。
- 数据库、部署和外部依赖：不涉及 schema、migration、Redis key、部署资产或新增依赖。
