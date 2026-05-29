## Context

AegisCore 目前在 `user-services` 中使用 Ent schema 描述用户表结构，并通过 `common` 与 `user-services/internal/entclient` 提供 PostgreSQL 连接池和 Ent client。当前仓库缺少标准化的数据库 schema 迁移流程，如果继续依赖运行时 `client.Schema.Create(ctx)`，数据库结构变更会绕过代码审查、难以在 CI/CD 中复现，也不利于生产环境执行可控的变更。

本变更引入 Atlas 作为 migration engine，使用 Ent schema 作为期望状态来源，生成可提交、可审查、可人工微调的 SQL migration 文件。迁移执行发生在 CI/CD 或容器启动前，不进入 Gin controller、service、repository 分层，也不改变现有 HTTP API 或响应契约。

## Goals / Non-Goals

**Goals:**

- 建立 `user-services` 首个基于 Ent 与 Atlas 的 SQL migration 工作流。
- 支持开发者从 `user-services/ent/schema` 生成 `user-services/migrations/*.sql` 与 `atlas.sum`。
- 支持人工审查和安全微调生成的 SQL，并通过 Atlas 校验和机制保持迁移目录完整性。
- 支持 Docker 镜像打包迁移文件，并在 CI/CD 或服务启动前通过 Atlas CLI 执行迁移。
- 为未来多服务接入提供可复制的服务内迁移目录约定。

**Non-Goals:**

- 不在服务运行时调用 `client.Schema.Create(ctx)` 或自动建表。
- 不修改 `user-services/ent/` 下生成代码；Ent schema 变更后仍通过 `go generate ./ent` 生成代码。
- 不新增业务表、用户写接口、认证授权、支付能力或数据库管理 API。
- 不将迁移执行集成进 HTTP 请求链路、controller/service/repository 分层或统一响应信封。

## Decisions

### 采用方案 B：服务内独立维护迁移目录

迁移文件放在各服务目录内，例如 `user-services/migrations/`。每个服务拥有自己的 `atlas.hcl`、Ent schema loader、生成脚本和部署脚本。

选择原因：

- 服务解耦：每个服务的 schema、迁移历史和部署节奏由服务自身维护，避免根目录集中迁移把所有服务绑定到同一个发布节奏。
- CI/CD 独立性：服务镜像只需要打包自身 migrations，发布用户服务时不必携带支付服务或其他服务迁移。
- 版本控制清晰：Ent schema、业务代码、迁移 SQL 和 `atlas.sum` 在同一个服务变更中 review，便于追踪因果关系。
- 多服务协同更安全：当多个服务共享同一个数据库时，应通过明确的数据 ownership 或共享库规格解决，而不是把迁移目录集中化作为默认方案。

方案 A 的优点是全局可见、适合单体数据库或平台团队统一治理，但在微服务演进中容易造成跨服务耦合、镜像打包膨胀和发布权限边界不清。因此本项目默认采用方案 B，仅在未来出现跨服务共享数据库且由平台团队统一治理时再评估集中目录。

### Atlas 配置放在服务目录

`user-services/atlas.hcl` 定义本服务的 migration directory、Ent schema source 和目标数据库 URL。建议目录结构：

```text
user-services/
  atlas.hcl
  ent/
    schema/
    migrate/
      main.go
  migrations/
    atlas.sum
    20260529120000_initial.sql
  scripts/
    migrate-diff.sh
    migrate-apply.sh
    entrypoint.sh
```

`ent/migrate/main.go` 封装 Atlas CLI 对 Ent schema 的读取和 diff 操作。当前 Atlas CLI 已支持 `ent://ent/schema` schema source，因此 `atlas.hcl` 直接将 `env.local.src` 配置为 `ent://ent/schema`，并在脚本中设置 `GOWORK=off` 避免 Go workspace 模式与 Atlas 内部 `-mod=mod` 调用冲突。

### 生成与执行命令分离

开发期生成命令只负责 diff 和 hash：

```bash
atlas migrate diff <name> --env local
atlas migrate hash --dir file://migrations
```

部署期执行命令只负责 apply：

```bash
atlas migrate apply --env deploy
```

这样可以避免生产容器在启动时重新生成迁移，确保部署执行的是已提交并审查过的 SQL。

### 人工微调 SQL 后重算 atlas.sum

开发者允许将生成 SQL 调整为 PostgreSQL 更安全的形式，例如把普通索引改成 `CREATE INDEX CONCURRENTLY`。任何手动修改都必须在提交前运行 `atlas migrate hash --dir file://migrations`，并同时提交更新后的 `atlas.sum`。CI 必须运行 `atlas migrate validate --dir file://user-services/migrations` 或等价校验，防止 SQL 与校验和不一致。

### 迁移配置复用运行时数据库信息但不依赖 Fx

迁移脚本可以读取 `DATABASE_URL`，或从现有 YAML 配置派生 PostgreSQL DSN。为了降低部署复杂度，Atlas 的 `deploy` 环境优先使用 `env("DATABASE_URL")`。如需复用 `AEGISCORE_` 配置，可在脚本中组装 URL，但迁移执行不应启动 Fx app、Redis client 或 HTTP server。

## Risks / Trade-offs

- [Risk] 服务内迁移目录会让跨服务共享库的 schema ownership 变得敏感 → Mitigation: 每张表必须有明确服务 owner；共享数据库变更需要单独 proposal 和规格约束。
- [Risk] 人工修改 SQL 后忘记更新 `atlas.sum` 会导致部署失败 → Mitigation: 生成脚本和 CI 都执行 Atlas hash/validate，PR 必须提交 SQL 与 `atlas.sum`。
- [Risk] `CREATE INDEX CONCURRENTLY` 不能在事务中执行 → Mitigation: 对需要 concurrent index 的 migration 文件添加 Atlas 支持的非事务指令或拆分迁移，并在 review checklist 中检查。
- [Risk] 容器启动前执行迁移会延长启动时间或多个副本并发执行 → Mitigation: 优先在 CI/CD release job 中执行迁移；若在 entrypoint 执行，依赖 Atlas migration lock，并限制迁移 job/副本策略。
- [Risk] Atlas CLI 增加构建和运行依赖 → Mitigation: 在开发文档和 Dockerfile 中固定安装方式，运行镜像可用多阶段构建只复制 `atlas` 二进制和迁移目录。

## Migration Plan

1. 在 `user-services` 增加 `atlas.hcl`、Ent schema loader、`migrations/` 目录和脚本。
2. 为现有 `user-services/ent/schema/user.go` 生成首个 baseline migration，并提交 SQL 与 `atlas.sum`。
3. 在 CI 中增加 migration directory 校验，确保手动修改后的 SQL 与 checksum 一致。
4. 更新 Dockerfile/entrypoint 或部署 job，使镜像包含 `user-services/migrations` 并在服务启动前执行 `atlas migrate apply`。
5. 验证 `go test ./...`、`go generate ./ent` 和 Atlas validate/apply dry-run 或测试库执行流程。

回滚策略：数据库迁移默认按向前修复处理；对于可逆变更，可在后续 migration 中补偿。部署失败时应停止服务启动并保留数据库当前版本，由运维或 release job 根据 Atlas 状态和 SQL review 决定重试或补偿。

## Open Questions

- 生产部署最终由 CI/CD 独立 migration job 执行，还是由容器 entrypoint 在启动前执行，需要结合现有发布平台确定。
- 是否需要为未来共享数据库引入跨服务 schema ownership 文档和 code owner 规则，可在多服务接入时单独设计。
