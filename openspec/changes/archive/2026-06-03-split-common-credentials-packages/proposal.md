## Why

当前 `common/credentials` 同时承载密码 hash、JWT token、认证传输常量和认证上下文 helper，包职责过宽，调用方需要通过同一个包理解两类不同共享原语。将其拆分为 `common/password` 与 `common/auth` 可以让密码凭证能力和 HTTP/JWT 认证边界更清晰，也为后续认证与会话逻辑复用提供更稳定的包结构。

## What Changes

- **BREAKING** 删除 `common/credentials` 目录，不再保留旧包路径兼容层。
- 新增 `common/password` 包，提供密码 hash 与校验能力，公开 API 调整为 `Hash()` 与 `Verify()`。
- 新增 `common/auth` 包，提供认证上下文 helper、Authorization/Bearer 常量、JWT claims、JWT service、token subject 和签发/解析能力。
- 更新依赖共享凭证原语的中间件、用户会话业务和测试，改为导入 `github.com/aegiscore/common/password` 或 `github.com/aegiscore/common/auth`。
- 保持密码 hash 格式、Argon2id 默认参数、JWT claims、签名算法、issuer/audience 校验、subject 校验、`user_id`/`token_version`/`session_id` 校验和认证响应语义不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `common-credentials`: 将共享凭证原语的规范包路径从单一 `common/credentials` 拆分为 `common/password` 与 `common/auth`，并更新密码公开 API 名称。
- `user-authentication`: 将认证中间件和认证会话业务复用共享认证边界常量、JWT service 与认证上下文 helper 的规范来源从 `common/credentials` 更新为 `common/auth`。
- `user-session-control`: 将登录、刷新和 token 枚举复用的共享包来源从 `common/credentials` 更新为 `common/password` 与 `common/auth`。

## Impact

- 受影响代码：`common/credentials/` 将被删除并拆分到 `common/password/`、`common/auth/`；现有导入 `github.com/aegiscore/common/credentials` 的代码需要迁移。
- API 兼容性：Go 包路径和密码函数名发生破坏性变更；运行时 HTTP API、错误码、响应 envelope 和 token 载荷不应发生可观察变化。
- 依赖影响：不新增第三方依赖，继续使用 `golang.org/x/crypto/argon2`、`github.com/golang-jwt/jwt/v5` 和现有 `common/config` 认证配置。
- 数据模型与配置：不修改 Ent schema、Atlas migration、YAML 配置结构或环境变量覆盖规则。
