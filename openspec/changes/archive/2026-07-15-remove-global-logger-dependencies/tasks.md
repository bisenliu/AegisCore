## 1. Logger primitive 行为

- [x] 1.1 修改 `common/runtime/logger/factory.go`，移除 `newLogger` 对 `SetDefault` 的隐式调用，并更新 `NewWithConfig`、`New` 相关注释，说明构造函数只返回调用方拥有的 logger。
- [x] 1.2 保留 `common/runtime/logger/fx.go` 中 `NewLogger` 的 Fx `OnStop` Sync 责任，并补齐或更新测试验证 `Sync` 错误忽略规则不变。
- [x] 1.3 更新 `common/runtime/logger` 单元测试，覆盖 `New`、`NewWithConfig` 和 `NewLogger` 单独调用均不改变进程级默认 logger；仅默认 fallback 测试显式调用 `SetDefault` 并在 cleanup 中恢复。
- [x] 1.4 审查 `common/http/...` 与 `common/runtime/logger` 测试，迁移不必要的默认 logger 修改为 `logger.ToContext`、显式 base logger 或局部 nop logger，保持 request ID、trace ID、span ID 字段语义不变。

## 2. user-service 显式注入迁移

- [x] 2.1 审查 `user-service/internal/providers/` 和 feature composition provider 的 logger 装配路径，确保服务级 `*zap.Logger` 被显式传给需要日志的 user、auth、role、permission component。
- [x] 2.2 迁移 `user-service/internal/features/user/` application、HTTP 边界和关键 PostgreSQL infrastructure 的日志依赖，使用 constructor 注入、注入 logger 派生组件 logger或 request context logger，移除正式主路径 package-level 默认 logger 依赖。
- [x] 2.3 迁移 `user-service/internal/features/auth/` command、credentials、sessions、tokens、validators 以及 Redis/PostgreSQL 等关键 infrastructure 的日志依赖，保持安全撤销、token version、强制改密相关日志字段和敏感信息过滤不变。
- [x] 2.4 迁移 `user-service/internal/features/permission/` application、authorization、policy sync、Casbin adapter、Redis watcher 和 PostgreSQL adapter 的日志依赖，保持授权 fail-closed、policy reload、用户角色缓存失效和 Redis policy version 发布语义不变。
- [x] 2.5 迁移 `user-service/internal/features/role/` command/query、RBAC seed、用户角色绑定、角色权限绑定和 PostgreSQL adapter 的日志依赖，保持角色、权限、绑定、seed 和超级管理员业务结果不变。

## 3. 测试 fixture 与架构约束

- [x] 3.1 更新 user、auth、permission、role application/infrastructure 测试 fixture 和 mock 构造，显式提供局部 logger 或 context logger，确保并行构造多个 fixture 时日志实例互不覆盖。
- [x] 3.2 增加或扩展 `make user-service-architecture-lint` 覆盖的静态检查，禁止 feature application 和关键 infrastructure 生产代码以 `logger.SetDefault`、`logger.FromContext(context.Background())`、包级 `logger.Info/Warn/Error/Debug` 或 `logger.NamedComponent(nil, ...)` 作为正式主路径日志来源。
- [x] 3.3 为新增架构检查补齐测试或 golden 覆盖，验证违规生产代码会失败，测试 fixture 的显式局部 logger 或 context logger 不被误判。

## 4. 针对性验证

- [x] 4.1 执行 `cd common && go test ./runtime/logger ./http/... -count=1`，确认 logger 构造、context logger 和共享 HTTP helper 行为通过。
- [x] 4.2 执行 user feature application/infrastructure 相关测试，至少覆盖 `cd user-service && go test ./internal/features/user/... -count=1` 或更窄且能覆盖本次修改的包集合。
- [x] 4.3 执行 auth feature application/infrastructure 相关测试，至少覆盖 `cd user-service && go test ./internal/features/auth/... -count=1` 或更窄且能覆盖本次修改的包集合。
- [x] 4.4 执行 permission feature application/infrastructure 相关测试，至少覆盖 `cd user-service && go test ./internal/features/permission/... -count=1` 或更窄且能覆盖本次修改的包集合。
- [x] 4.5 执行 role feature application/infrastructure 相关测试，至少覆盖 `cd user-service && go test ./internal/features/role/... -count=1` 或更窄且能覆盖本次修改的包集合。
- [x] 4.6 执行 `make user-service-architecture-lint`，确认新增静态约束和既有架构边界均通过。
- [x] 4.7 执行 `openspec validate remove-global-logger-dependencies`，确认 change artifacts 和 delta specs 可解析且满足 OpenSpec 规则。

## 5. 收尾验证与 drift 检查

- [x] 5.1 检查本次变更不产生 Ent、Atlas、OpenAPI、Prometheus/Grafana 或部署资产漂移；如未涉及生成物，记录 `git diff -- user-service/docs deployments user-service/migrations user-service/ent` 无非预期变更。
- [x] 5.2 暂存本次预期代码、规格和文档变更，执行 `git status --short` 确认只包含预期文件。
- [x] 5.3 在暂存预期变更后执行 `make lint`，通过后才能勾选本任务。
- [x] 5.4 在暂存预期变更后执行 `make verify`，通过后才能勾选本任务；若失败，修复后重新暂存并重跑。
