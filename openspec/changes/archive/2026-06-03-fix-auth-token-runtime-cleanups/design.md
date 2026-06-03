## Context

本变更横跨 `common` 与 `user-services` 两个 Go module，但均属于低风险清理和退出稳定性修复。当前 logger stop hook 只忽略部分 stdout/stderr `Sync()` 错误，macOS 终端场景可能出现 `ENOTTY` 并影响服务退出状态；Bearer 前缀剥离逻辑分散在 `auth_controller.go` 和 `auth_service.go`，而 `common/auth` 已承担认证传输常量和上下文 helper 的共享职责。

受影响能力为 `shared-infrastructure`、`common-credentials`、`user-session-control` 与 `http-service-runtime`。变更不涉及 Ent schema、Atlas migration、路由契约、响应信封、配置项或数据库结构。

## Goals / Non-Goals

**Goals:**

- 修复 logger `Sync()` 在 stdout/stderr 不可同步设备上返回 `ENOTTY` 时导致退出异常的问题。
- 在 `common/auth` 提供统一 Bearer 前缀剥离 helper，并替换用户服务 controller/service 中的重复逻辑。
- 用命名常量表达密码 hash 长度上限、HTTP shutdown 默认值、CLI start/stop 超时。
- 保持现有 API、错误映射、Redis/PostgreSQL/Fx wiring 和配置加载行为不变。

**Non-Goals:**

- 不移动 `authenticatedSession()` 到 `common/auth`，因为当前仅用户会话 service 使用，移动会扩大公共 API 面。
- 不新增配置项或允许用户通过配置修改 CLI start/stop 超时。
- 不修改认证 token claims、JWT 签名规则、Redis session key、路由或响应 envelope。
- 不修改 Ent schema、迁移脚本或生成代码。

## Decisions

1. `StripBearerPrefix` 放入 `common/auth`

`common/auth` 已包含 `AuthorizationHeader`、`TokenTypeBearer`、`TokenPrefix` 和认证上下文 helper，Bearer 传输层字符串处理属于同一职责范围。相比继续在 user-services 中保留 controller/service 本地 helper，将剥离逻辑放入 common 能避免大小写、空白处理和前缀规则漂移。

替代方案：只在 user-services 内新建私有 helper。该方案能减少公共 API，但无法支撑后续其他服务或中间件复用，也与现有认证传输常量的 common 边界不一致。

2. helper 只剥离可选 `Bearer ` 前缀

函数先 `strings.TrimSpace(token)`，再对固定长度前缀使用 `strings.EqualFold` 做大小写无关匹配。没有前缀时返回 trim 后原 token。该行为保持刷新请求体和改密 token 对可选 Bearer 前缀的兼容，同时不会把其他 token 内容误解析为 Bearer token。

替代方案：解析任意空白分隔的 authorization scheme。该方案更复杂，可能改变当前 `Bearer ` 固定前缀语义，本次不采用。

3. logger stop hook 忽略 `EINVAL` 和 `ENOTTY`

Zap 对 stdout/stderr 的 `Sync()` 在不同 OS 或终端设备上可能返回不可同步类错误，这类错误不代表日志资源关闭失败。stop hook 继续返回其他 `Sync()` 错误，以保留真实文件或底层 writer 同步失败的可见性。

替代方案：忽略所有 `Sync()` 错误。该方案会掩盖真实持久化日志失败，不采用。

4. 暂不移动 `authenticatedSession()`

`authenticatedSession()` 当前负责把 common/auth context helper 的结果映射到用户会话 service 的业务错误处理。把它移到 `common/auth` 会要求 common 暴露更高层 session 聚合 helper，或者引入 service 侧错误映射适配。由于本次目标是小修和去重 Bearer 处理，保留私有 helper 更符合最小改动原则。

替代方案：在 `common/auth` 增加 `AuthenticatedSession(ctx) (userID, sessionID string, ok bool)`。该方案合理但属于公共 API 扩展，后续有跨服务复用时再引入。

5. 命名常量只表达现有策略

`common/password` 的 encoded hash 长度上限、bootstrap 的默认 graceful shutdown 超时、CLI start/stop 超时均保持现有值，仅通过常量命名语义。启动和停止超时拆分为两个常量，即使当前值相同，也方便后续独立调整。

## Risks / Trade-offs

- `StripBearerPrefix` 成为公共 API 后需要稳定维护。缓解：函数语义保持窄范围，仅处理固定 Bearer 前缀和 trim，不引入复杂解析。
- 忽略 `ENOTTY` 可能隐藏少数非终端 writer 的同名错误。缓解：只忽略明确 syscall 错误，其他错误仍返回。
- 不移动 `authenticatedSession()` 不能完全消除 service 中所有认证上下文胶水代码。缓解：本次只去重已扩散的 Bearer 前缀处理，避免过早扩大 common API。
- 常量提取主要提升可读性，行为验证依赖测试覆盖现有路径。缓解：运行 common 和 user-services 相关测试，确保无行为回归。
