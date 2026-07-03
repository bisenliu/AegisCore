## Context

强制改密登录目前沿用普通登录成功响应：HTTP controller 调用登录 use case 后统一 `response.OK`，`TokenResponse` 通过 `password_change_required` 字段区分受限 token。这个分支的认证凭据有效，但账号尚未获得普通会话能力，因此它不是普通登录成功，也不应要求前端检查业务 flag 才能识别下一步动作。

本变更跨越 `common` 和 `user-service`：`common/contract/errors` 提供稳定错误码枚举；`user-service/internal/features/auth` 拥有强制改密判断、受限 token 签发和 HTTP 响应映射。变更不涉及 Ent schema、migration、Redis key、RBAC policy、部署清单或观测资产。

## Goals / Non-Goals

**Goals:**

- 在共享错误契约中新增 `CodePasswordChangeRequired = 20006`，保持认证类错误码连续且可被 OpenAPI 枚举。
- 登录强制改密分支返回携带 `CodePasswordChangeRequired` 的响应，并继续携带受限 `password_change` token 数据。
- 让客户端以 envelope `code` 识别强制改密状态，不再依赖 `password_change_required` 字段作为判定依据。
- 覆盖错误码值、auth controller 映射、登录 use case 语义和 e2e 登录改密流程测试，并重新生成 OpenAPI。

**Non-Goals:**

- 不改变账号状态模型、密码策略、JWT subject、token TTL、refresh session 生命周期或 token version 校验逻辑。
- 不新增数据库字段、migration、Redis key schema、外部依赖或部署流程。
- 不把 user-service 的强制改密业务语义放入 `common/http/response`、`common/security/auth` 或 `user-service/internal/shared`。

## Decisions

1. `CodePasswordChangeRequired` 放在 `common/contract/errors` 的 200xx 认证错误码段。

   备选方案是把该码放在 auth feature 私有常量中，但登录响应 envelope 和 OpenAPI 错误码枚举依赖共享契约；私有常量会让客户端契约分散。放入 `common` 时只增加业务中立的认证状态码，不放入 user-service 专用响应逻辑。

2. 强制改密分支由 auth HTTP 边界映射为专用 envelope code。

   登录 use case 继续负责校验凭据并签发受限 token，使用现有 `TokenResult.PasswordChangeRequired` 或等价内部标记向 transport 表达分支；HTTP controller 根据该分支写出 HTTP `200 OK`、`success: false`、`code: CodePasswordChangeRequired` 和受限 token 数据。备选方案是在 use case 返回 application error，但普通失败 helper 不携带 token 数据，会让受限 token 难以通过统一 envelope 返回。

3. 响应数据保留受限 token 必需字段，`password_change_required` 不再作为外部判定依据。

   客户端应读取 envelope `code == 20006` 进入改密流程；响应数据只承载 `access_token`、`token_type`、`expires_in`，不得携带 `refresh_token`。如为兼容短期保留字段，也不能作为 OpenAPI 和测试中的主判定条件。

4. OpenAPI 通过现有生成链路更新。

   更新 swagger 注解或 DTO 后运行 `make user-service-openapi-generate`，同步 `user-service/docs/openapi.go`、`openapi.json` 和 `openapi.yaml`。不手写生成物。

## Risks / Trade-offs

- 客户端仍按 `CodeOK + password_change_required` 判断可能无法进入改密流程 → 在 API 变更说明、OpenAPI 和 e2e 中固定 `CodePasswordChangeRequired`，推动前端改为读取 code。
- 如果将该状态渲染成通用失败响应，token 数据可能被遗漏 → auth HTTP 边界需要专门测试强制改密响应包含 access token 且不包含 refresh token。
- 如果把专用 writer 做进 `common/http/response`，会污染共享边界 → 仅在 auth transport 内组合 envelope，`common` 只增加错误码和必要构造测试。
- OpenAPI 枚举遗漏新 code 会导致客户端生成物滞后 → 实现任务必须包含 OpenAPI 生成和 diff 检查。

## Migration Plan

1. 新增共享错误码和测试，保证 `CodePasswordChangeRequired` 的数值稳定为 `20006`。
2. 调整 auth 登录 HTTP 响应映射和 DTO/OpenAPI 注解，确保强制改密分支携带新 code、受限 token、无 refresh token。
3. 更新单元测试和 e2e 流程，测试从 envelope code 判断强制改密。
4. 运行相关测试、OpenAPI 生成和架构 lint。

回滚时恢复登录强制改密响应映射和 OpenAPI 生成物，并移除未发布使用的错误码；如果错误码已经对外发布，应保留常量以避免客户端兼容风险，只回滚 controller 映射。

## Open Questions

- 无。
