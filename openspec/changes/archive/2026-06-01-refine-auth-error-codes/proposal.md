## Why

当前认证失败场景统一返回 `CodeUnauthenticated`，前端无法区分 token 格式非法、过期、失效拉黑等可执行不同处理策略的原因。用户已将 `common/middleware/auth.go` 的未认证响应文案统一为 `unauthenticatedMessage` 并将相关 warn 日志提升为 error；该变更只影响实现细节和可观测性，不改变现有 OpenSpec 已声明的响应契约，因此无需单独同步主规格。

## What Changes

- 评估并引入必要的认证细分错误码：`CodeTokenInvalid`、`CodeTokenExpired`，用于当前已有 JWT 解析流程可可靠识别的非法 token 与过期 token 场景；缺失认证信息仍保留为通用 `CodeUnauthenticated`。
- 暂不引入 `CodeTokenRevoked`、`CodeMFARequired`、`CodeUserAccountLocked`，因为当前仓库尚无 token 黑名单、MFA 挑战、账号冻结或封禁流程；提前添加会制造未被实现路径使用的契约。
- 保持所有认证失败 HTTP status 为 `401 Unauthorized`，只细分统一响应信封中的业务 `code`。
- 保持对外未认证文案统一为 `登录状态无效或已过期，请重新登录`，避免向调用方暴露签名、issuer、audience 等验证细节。
- 更新 `api-response-contract` 的 OpenSpec delta，明确标准错误码和 JWT 认证失败映射。
- 更新认证中间件、响应构造函数、相关测试和必要文档，使错误码语义与返回行为一致。

## Capabilities

### New Capabilities

无。本变更不引入完整认证、授权、会话或令牌管理能力，只细化现有共享响应契约中的认证失败错误码。

### Modified Capabilities

- `api-response-contract`: 扩展标准数字业务码，要求 JWT 认证失败在统一失败信封中区分 token 非法与 token 过期。

## Impact

- 影响 capability：`api-response-contract`。
- 主要代码位置：`common/response/error.go`、`common/response/response.go`、`common/middleware/auth.go`、`common/jwt/service.go` 及对应测试。
- API 行为影响：HTTP status 仍为 401；响应信封 `success`、`message` 结构保持不变；部分已携带但无效或过期的 JWT 失败场景业务 `code` 将从 `20000` 变为 `20001` 或 `20002`；缺失认证信息仍为 `20000`。
- 兼容性影响：依赖 `code == 20000` 判断所有未认证失败的调用方需要调整为按 2xxxx 认证错误族或明确处理 `20001`、`20002`。
- 文档影响：`common/middleware/auth.go` 的统一未认证文案和日志级别变更不需要更新现有 OpenSpec 主规格；新增认证细分错误码属于响应契约变更，需要通过 `api-response-contract` delta 更新。
