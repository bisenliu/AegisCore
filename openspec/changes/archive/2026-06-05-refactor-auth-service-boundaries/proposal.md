## Why

`user-services/internal/service/auth_service.go` 当前同时承载用户认证、密码校验、token 签发、Refresh Token 会话管理、token version 校验和认证上下文解析等职责，认证需求继续扩展时会使 `AuthService` 演化为难以测试和维护的“上帝服务”。现在需要在保持 `user-session-control` 既有行为语义不变的前提下，将 token、session、credential 等策略从用例编排中拆出，降低模块耦合并提升后续扩展性。

## What Changes

- 将 `AuthService` 保留为登录、改密、刷新、退出当前设备和退出全部设备的统一用例入口，仅负责高层流程编排和错误语义衔接。
- 从 `AuthService` 中拆出凭证校验组件，负责按用户名读取认证资料、使用共享密码 helper 校验密码，并统一映射无效凭证语义。
- 从 `AuthService` 中拆出 token 签发与 token claims 校验组件，负责 Access/Refresh/Password-Change token 的 TTL 兜底、签发、解析和 subject/user_id/token_version 基础校验。
- 从 `AuthService` 中拆出会话管理组件，负责 Redis Refresh Token 会话创建、读取校验、轮转删除、当前设备退出、全部会话删除和 token version 缓存失效。
- 保持 `/api/v1/auth/login`、`/api/v1/auth/refresh`、`/api/v1/auth/logout`、`/api/v1/auth/logout-all` 和 `/api/v1/auth/change-password` 的 HTTP 契约、响应信封、错误映射、Redis key 行为、token claims 和 token_version 行为不变。
- 不引入新的认证协议、授权模型、数据库字段或外部配置项。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 调整认证会话控制能力的内部职责边界，要求 token 签发、会话管理和凭证校验从 `AuthService` 的用例编排中分离，但不改变外部 API 与安全语义。

## Impact

- 主要影响 `user-services/internal/service/auth_service.go` 及同包新增认证组件文件。
- 可能需要调整 `AuthServiceParams` 的 Fx 注入组合方式，但不改变 controller、router、DTO、repository 抽象和 Redis repository 对外契约。
- 不涉及 Ent schema、Atlas migration、HTTP 路由、错误码、响应信封、配置字段或 Swagger 契约变更。
- 测试影响集中在 `user-services/internal/service/auth_service_test.go` 和新增组件的单元测试覆盖；现有认证流程测试应继续通过。
