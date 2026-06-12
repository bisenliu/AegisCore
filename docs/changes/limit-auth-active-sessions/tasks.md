# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更不创建 `openspec/` 或 `docs/opsx/`。
- [x] 检查 `user-service/internal/features/auth/application/command/login.go`、`refresh.go`、`sessions.go`、`tokens.go` 当前登录和 refresh rotation 写 session 流程。
- [x] 检查 `user-service/internal/features/auth/infrastructure/redis/session_store.go` 当前 `CreateSession`、`RotateSession`、`DeleteSession`、`DeleteAllUserSessions` 和 Lua 脚本。
- [x] 检查 `common/runtime/config` 的 config struct、loader、validation 和测试写法。
- [x] 确认本次不修改 HTTP API、JWT claim、Ent schema、Atlas migration、Redis key schema、eventbus 或 outbox。

## Configuration

- [x] 在 `common/runtime/config/config.go` 的 auth 配置中新增 `MaxActiveSessionsPerUser int`。
- [x] 在 `common/runtime/config/validation.go` 中校验 `auth.max_active_sessions_per_user >= 0`。
- [x] 更新 `user-service/configs/config.yaml`，在 `auth` 下新增 `max_active_sessions_per_user: 5` 和中文注释。
- [x] 更新 config loader 测试，覆盖 YAML 加载和 `AEGISCORE_AUTH_MAX_ACTIVE_SESSIONS_PER_USER` 环境变量覆盖。
- [x] 新增或更新配置校验测试，确认负数配置失败，`0` 配置通过。

## Application Policy

- [x] 在 auth application lifecycle 中持有 `maxActiveSessionsPerUser` session 写入策略值。
- [x] 更新 auth application-owned `AuthSessionStore` port，使 `CreateSession` 和 `RotateSession` 接收 session 上限值。
- [x] 更新 `AuthSessionLifecycle`，让 `CreateTokenSession` 和 `RotateTokenSession` 传递上限值。
- [x] 更新 `UseCaseDeps`，从 `params.Config.Auth.MaxActiveSessionsPerUser` 读取并保存上限值。
- [x] 更新 `issueTokenPair`，创建 refresh session 时传入上限值。
- [x] 更新 refresh rotation 路径，轮转 session 时传入相同上限值。
- [x] 确认 application 层不导入 Redis client、workerpool、Lua 细节或 `config.Config` 以外的 infrastructure 类型。

## Redis Adapter

- [x] 将 `CreateSession` 的 pipeline 改为 Lua 脚本，原子执行 session payload 写入、过期索引清理、索引写入、上限裁剪和索引 TTL 刷新。
- [x] 为 create 脚本传入 `AuthSessionPrefix(userID)`，用于删除被裁剪 session payload key。
- [x] `max_active_sessions_per_user > 0` 时按 sorted-set 最旧 score 裁剪超限 session。
- [x] `max_active_sessions_per_user == 0` 时不执行上限裁剪，但保留过期索引懒清理。
- [x] 调整 `RotateSession` Lua 脚本，在旧 session 删除和新 session 写入后执行同样的上限裁剪。
- [x] 保持 `RotateSession` 的 not found 和 mismatch 返回码语义不变。
- [x] 如脚本返回裁剪数量，更新调用方解析和日志字段；如不返回，至少保持错误语义清晰。
- [x] 保持 Redis key schema 不变。
- [x] 保持 `DeleteSession` 行为不变。
- [x] 保持 `DeleteAllUserSessions` 使用 `auth_session_purge_pool` 异步执行批量物理清理。

## Workerpool Boundary

- [x] 确认没有把登录 session 上限裁剪提交到 workerpool。
- [x] 确认 `common/runtime/workerpool` 不新增 auth session 业务规则、Redis key 规则或 DTO。
- [x] 确认 auth application 不直接依赖 workerpool。
- [x] 保留 `auth_session_purge_pool` 仅用于 detached session 批量物理清理。

## Tests

- [x] 更新 Redis session store 测试 helper，支持传入 session 上限值。
- [x] 新增测试：连续创建 6 个 session，上限 5，最终只保留最近 5 个。
- [x] 新增测试：被裁剪的最旧 session `GetSession` 返回 `ErrAuthSessionNotFound`。
- [x] 新增测试：用户 session sorted-set `ZCARD` 不超过上限。
- [x] 新增测试：上限为 0 时连续创建 session 不裁剪。
- [x] 新增测试：过期 sorted-set 项仍会被懒清理。
- [x] 新增测试：并发创建 session 后最终不超过上限。
- [x] 新增测试：`RotateSession` 成功后旧 session 不存在，新 session 存在，总数不超过上限。
- [x] 保留并通过 `RotateSession` not found、mismatch 和并发 rotation 相关测试。
- [x] 更新 auth command tests，确认 login 和 refresh rotation 会传递配置中的 session 上限值。
- [x] 更新 auth command tests，确认 create/rotate session 失败时不返回 token 响应。
- [x] 回归测试 `DeleteAllUserSessions`、purge workerpool、`RevokeAllUserSessions` 和 `DeleteSession`。

## Documentation

- [x] 更新 `docs/ARCHITECTURE.md`，说明认证 Redis refresh session 支持每用户活跃上限。
- [x] 更新 `docs/ARCHITECTURE.md`，明确 session 上限裁剪同步执行，workerpool 只用于批量物理清理。
- [x] 更新 `AGENTS.md` Current Feature Areas 或 Repository Shape，补充认证会话受控多会话治理。
- [x] 更新 `AGENTS.md` Repository Rules，补充影响 token 可续期能力的 session 策略必须同步落地。
- [x] 确认文档没有新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 common 配置相关测试：

```bash
cd common
go test ./runtime/config
```

- [x] 运行 auth command 测试：

```bash
cd user-service
go test ./internal/features/auth/application/command
```

- [x] 运行 auth Redis adapter 测试：

```bash
cd user-service
go test ./internal/features/auth/infrastructure/redis
```

- [x] 运行变更范围测试：

```bash
make test-common
make test-user-service
```

- [x] 运行结构扫描，确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 运行依赖扫描，确认 application 层没有引入 Redis client 或 workerpool：

```bash
rg -n "redis/go-redis|runtime/workerpool" user-service/internal/features/auth/application
```

## Review Notes

- [x] 确认默认上限为 5，且 0 明确表示不限制。
- [x] 确认超限裁剪同步、原子执行。
- [x] 确认被裁剪 refresh session 无法再刷新 token。
- [x] 确认被裁剪设备的既有 access token 不会立即失效，这是当前 access token 不查 session key 的既有边界。
- [x] 确认 logout-all 和强制改密仍通过 token version 实现立即失效。
- [x] 确认 workerpool 只用于批量物理清理，不参与 session 上限安全策略。
