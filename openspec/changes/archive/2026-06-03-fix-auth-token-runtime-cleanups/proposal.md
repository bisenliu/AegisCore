## Why

当前用户服务存在两类低风险但影响运行稳定性和可维护性的问题：Zap logger 在 macOS 或终端 stdout/stderr 上执行 `Sync()` 时可能因 `ENOTTY` 导致退出异常；Bearer token 前缀剥离逻辑分散在 controller 和 service 中，容易产生规则漂移。

同时，若干运行时与安全边界使用裸字面量表达，降低了配置策略和输入边界的可读性，需要通过命名常量固化语义。

## What Changes

- 在 logger 停止流程中同时忽略 `syscall.EINVAL` 与 `syscall.ENOTTY`，避免 stdout/stderr `Sync()` 的终端相关错误影响服务退出状态。
- 在 `common/auth` 新增统一的 `StripBearerPrefix(token string) string` helper，集中处理 trim 与大小写无关的可选 `Bearer ` 前缀剥离。
- 将用户会话刷新、密码变更 token 校验和认证 controller 中的本地 Bearer 清洗逻辑替换为 `common/auth` helper，并移除重复 helper。
- 在 `common/password` 中用命名常量表达 encoded hash 最大长度边界。
- 在用户服务 bootstrap 与 CLI 中用命名常量表达默认关闭超时、启动超时和停止超时。
- 不移动 `authenticatedSession()` 到 common 层，除非后续出现跨服务复用需求；本次保持最小改动，避免扩大依赖边界。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-infrastructure`: logger 停止 hook 必须忽略 stdout/stderr `Sync()` 常见不可同步错误，包括 `EINVAL` 和 `ENOTTY`。
- `common-credentials`: 共享认证 helper 必须提供统一的可选 Bearer 前缀剥离能力。
- `user-session-control`: refresh token 和密码变更 token 的 Bearer 前缀处理必须复用共享 helper。
- `http-service-runtime`: 服务启动、停止和默认关闭超时策略必须使用具名常量表达。

## Impact

- 影响代码：`common/infrastructure/logger.go`、`common/auth/`、`common/password/password.go`、`user-services/internal/controller/auth_controller.go`、`user-services/internal/service/auth_service.go`、`user-services/internal/bootstrap/bootstrap.go`、`user-services/cmd/main.go`。
- API 兼容性：HTTP API 路径、请求/响应结构、错误码和认证语义保持兼容。
- 配置兼容性：不新增或修改配置项。
- 数据兼容性：不修改 Ent schema、迁移或持久化数据结构。
- 依赖影响：不新增第三方依赖。
