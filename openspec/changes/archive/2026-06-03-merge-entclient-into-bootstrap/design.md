## Context

用户服务当前在 `user-services/internal/bootstrap` 中集中组装 HTTP runtime、Gin engine、JWT service、Redis client 和 PostgreSQL 连接池，但 Ent client provider 位于独立的 `user-services/internal/entclient` 包。该包只有一个 `provider.go`，对外只服务于 `bootstrap.UserServiceModule` 的 Fx wiring。

`shared-infrastructure` 主规格已经要求用户服务基于具名 `user_db` 和 `common_db` PostgreSQL 连接池创建具名 Ent clients，并在 Fx app 停止时关闭 Ent clients。本变更只调整服务内代码组织，不改变配置、数据库连接、Ent schema、HTTP API 或 repository 注入语义。

## Goals / Non-Goals

**Goals:**

- 将 Ent client provider 合入 `user-services/internal/bootstrap`，与 Redis/PostgreSQL runtime provider 放在同一服务启动装配边界内。
- 移除 `user-services/internal/entclient` 包和 `bootstrap` 对该包的导入。
- 保持 `user_db` 与 `common_db` Ent client 的 Fx name、生命周期关闭行为和 repository 注入行为不变。
- 补充或调整测试，证明用户服务模块仍能解析具名 Ent clients，且不连接未声明的 datastore。

**Non-Goals:**

- 不修改 `common/infrastructure` 的公共 helper、配置加载策略或命名实例契约。
- 不修改 Ent schema、生成代码、Atlas migration 或数据库结构。
- 不改变 controller/service/repository 分层、HTTP 路由、响应信封、错误码或认证行为。
- 不新增用户、支付或健康检查等业务能力。

## Decisions

- 将 provider 代码迁移为 `bootstrap` 包内实现，而不是在 `common` 中新增通用 Ent helper。

  理由：当前 Ent client 使用 `github.com/aegiscore/user-services/ent` 生成包，是用户服务特定依赖；放入 `common` 会反向依赖服务模块或引入泛化抽象，破坏模块边界。

- 保留 `NewNamedClients`、`ClientParams` 和 `NamedClients` 的最小语义，但将包名从 `entclient` 改为 `bootstrap`。

  理由：这能让 Fx provider 替换最小化，仅更新 `UserServiceModule` 中的 provider 引用，避免额外命名和行为改动。

- 继续使用 `entsql.OpenDB(dialect.Postgres, db)` 基于既有 `*sql.DB` 创建 Ent driver。

  理由：该路径复用已有 PostgreSQL lifecycle、连接池和 ping 行为，不引入新的数据库连接或 DSN 构造逻辑。

- Ent client 关闭仍通过 Fx `OnStop` hook 统一处理。

  理由：现有行为要求 Fx app 停止时关闭 Ent clients；迁移包位置不应改变资源释放时机或错误返回顺序。

## Risks / Trade-offs

- [Risk] 删除包后仍存在旧 import 或文档引用导致编译或规格不一致。→ Mitigation：使用内容搜索确认 `internal/entclient` 引用全部迁移，并运行 `go test ./...`。
- [Risk] Ent client provider 与 PostgreSQL provider 同包后文件职责变大。→ Mitigation：将 Ent provider 放在独立的 `ent.go` 文件中，保持 `bootstrap.go` 只做高层 Fx module wiring。
- [Risk] Fx named dependency tag 被误改会导致 repository 注入失败。→ Mitigation：保留 struct tag `name:"user_db"` 和 `name:"common_db"`，并用 Fx validation 或现有测试覆盖依赖图解析。
- [Risk] 关闭 Ent client 与关闭底层 `*sql.DB` 的顺序可能被误改。→ Mitigation：迁移时只复制现有 hook 逻辑，不重排 PostgreSQL provider lifecycle。
