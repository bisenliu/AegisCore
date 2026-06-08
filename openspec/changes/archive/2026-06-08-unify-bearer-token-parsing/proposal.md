## Why

当前受保护 API 的 Gin 认证中间件对 `Authorization: Bearer <token>` 前缀执行大小写敏感校验，而 `common/security/auth.StripBearerPrefix` 已按大小写无关方式处理 Bearer 前缀。不同认证入口对同一 HTTP Bearer 传输语义的兼容性不一致，可能导致客户端在刷新、密码变更和受保护 API 之间遇到不一致认证行为。

## What Changes

- 在 `common/security/auth` 中确立单一 Bearer authorization 解析入口，用于从 Authorization header 中提取 token 并区分缺失前缀、空 token 和有效 token。
- 调整 Gin 认证中间件使用 shared auth 包的解析逻辑，不再自行组合 `strings.HasPrefix` 与 `strings.TrimPrefix`。
- 统一 Bearer 前缀匹配语义：`Bearer ` 前缀按大小写无关方式匹配，token 内容仍保持原样，仅去除首尾空白。
- 保持现有 HTTP 401、业务错误码、公开认证失败文案、JWT 校验、token version 校验和认证上下文传播语义不变。
- 不引入新的认证方式、授权规则、配置项或数据模型变更。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `common-credentials`: 明确 `common/security/auth` 提供统一 Bearer authorization 解析函数，并规定大小写无关的 Bearer 前缀匹配、空 token 和缺失前缀处理语义。
- `user-authentication`: 要求 Gin 认证中间件复用 shared auth Bearer 解析逻辑，并接受大小写不同但语义等价的 Bearer 前缀。

## Impact

- 影响代码：`common/security/auth/token.go`、`common/http/middleware/auth.go` 及相关单元测试。
- 外部 API：受保护 API 将接受 `bearer <token>`、`BEARER <token>` 等大小写不同的 Bearer 前缀；原本合法的 `Bearer <token>` 行为保持不变。
- 错误响应：缺少 Authorization header、格式错误、空 token、非法 token 和过期 token 的 HTTP status、业务码与公开 message 保持兼容。
- 依赖与数据：不新增第三方依赖，不修改配置、数据库 schema、Redis key 或 Ent 生成代码。
