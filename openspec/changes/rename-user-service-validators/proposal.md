## Why

`common/validation/` 与 `user-services/internal/validation/` 目录同名但职责不同：前者是共享校验核心，后者是用户服务内的请求输入清洗、解析和服务特定校验规则集合。随着未来 team/toll team 等请求对象校验逻辑增加，继续使用 `internal/validation/` 会增加语义冲突和维护成本。

本变更通过将用户服务本地校验边界重命名为 `user-services/internal/validators/`，明确它是一组服务内校验器组件，而不是共享校验引擎，同时为后续按领域拆分 user/auth/team 校验逻辑建立稳定目录结构。

## What Changes

- 将 `user-services/internal/validation/` 重命名为 `user-services/internal/validators/`。
- 将 Go package 从 `validation` 调整为 `validators`，controller 直接通过 `validators.NormalizeCreateUser`、`validators.NormalizeLogin` 等调用服务内校验器。
- 将当前聚合的用户与认证校验逻辑拆分为：
  - `user-services/internal/validators/user.go`
  - `user-services/internal/validators/user_test.go`
  - `user-services/internal/validators/auth.go`
  - `user-services/internal/validators/auth_test.go`
- 保持 `common/validation/` 作为共享 validator engine、tag rule、错误归一化和 DTO hook 的能力边界。
- 保持现有 HTTP API、请求字段、响应信封、错误码和公开错误语义兼容。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-service-validation-boundary`: 明确用户服务本地校验边界使用 `user-services/internal/validators/` 承载服务内校验器集合，可包含 Normalize、Validate、Parse 等 API 入参和内部对象进入 Service 前的清洗、校验和转换逻辑。
- `request-validation`: 保持 `common/validation/` 的共享校验核心定位，并明确服务特定校验器不得因目录重命名上移到 `common/validation/`。

## Impact

- 影响代码：`user-services/internal/validation/`、`user-services/internal/controller/user_controller.go`、`user-services/internal/controller/auth_controller.go` 及相关测试引用。
- 影响文档/规格：更新涉及 `user-services/internal/validation` 的 OpenSpec 规格和能力说明。
- API 兼容性：不改变 HTTP 路由、请求/响应字段、响应信封、错误码或公开 message。
- 数据兼容性：不涉及 Ent schema、Atlas migration 或数据库数据模型。
- 依赖影响：不新增第三方依赖。
