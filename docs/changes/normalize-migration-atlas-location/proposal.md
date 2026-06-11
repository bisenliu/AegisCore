# Normalize migration Atlas location

## What

统一用户服务 Atlas 配置和 migration 目录的位置说明与调用方式。

本变更建议将 `user-service/atlas.hcl` 迁移到 `user-service/migrations/atlas.hcl`，让 Atlas 配置与其管理的 SQL migration directory 同目录归档，同时更新脚本、Dockerfile、文档和 CI 校验入口，保证根目录 Makefile 迁移命令继续可用。

包括：

- 移动 Atlas 配置文件到 `user-service/migrations/atlas.hcl`。
- 更新 migration 脚本，使 `migrate-diff`、`migrate-validate` 和 `migrate-apply` 显式使用新的 Atlas 配置位置。
- 更新 Dockerfile，使运行镜像包含新的配置位置。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md` 和相关说明中的路径。
- 增加或更新 CI migration validation，使 `migrate-validate` 在新布局下可被自动验证。

本变更只调整 Atlas 配置文件位置、路径引用和校验入口，不修改历史 SQL 内容、`atlas.sum`、Ent schema、数据库 schema、HTTP API、业务逻辑或生产迁移执行策略。

## Why

当前 Atlas 配置位于 `user-service/atlas.hcl`，migration 文件位于 `user-service/migrations/`。这在脚本已经统一切换到 `user-service/` 目录的前提下可工作，但配置与被管理的 migration directory 分离，容易在文档、容器复制路径和 CI 命令中形成两个需要同步维护的位置。

将 `atlas.hcl` 放入 `user-service/migrations/` 可以让迁移资产形成一个更清晰的边界：SQL 文件、`atlas.sum` 和 Atlas migration 配置都在同一目录下。这样更便于 Docker 镜像打包、CI 校验、迁移目录审查和后续多服务迁移目录扩展。

## Scope

包括：

- 评估保留 `user-service/atlas.hcl` 与迁移到 `user-service/migrations/atlas.hcl` 的取舍，并采用迁移方案。
- 使用 `git mv user-service/atlas.hcl user-service/migrations/atlas.hcl` 保留文件移动语义。
- 更新 `user-service/scripts/migrate-diff.sh`：
  - 从 `user-service/` 目录执行。
  - 使用 `atlas migrate diff <name> --config file://migrations/atlas.hcl --env local`。
  - 继续使用 `atlas migrate hash --dir file://migrations` 更新 migration hash。
- 更新 `user-service/scripts/migrate-validate.sh`：
  - 继续校验 `user-service/migrations/`。
  - 如需要读取 Atlas 配置，使用新路径；否则保持显式 migration dir 校验。
- 更新 `user-service/scripts/migrate-apply.sh`：
  - 使用 `atlas migrate apply --config file://migrations/atlas.hcl --env deploy`。
  - 保持 `DATABASE_URL` 必需校验。
- 更新 `user-service/Dockerfile`：
  - 不再复制根部 `user-service/atlas.hcl`。
  - 继续复制完整 `user-service/migrations/`，使镜像中包含 `migrations/atlas.hcl`。
- 更新 CI：
  - 如果已有 migration validation job，改用新脚本或新配置路径。
  - 如果没有 migration validation job，新增一个轻量 workflow/job 运行 `make migrate-validate`。
- 更新文档和代理规则中的 Atlas 配置位置与命令说明。

不包括：

- 不修改 `user-service/migrations/*.sql` 的历史 SQL 内容。
- 不修改 `user-service/migrations/atlas.sum`，除非 Atlas 因配置文件进入 migration dir 而要求校验策略调整；若需要调整，必须在设计和任务中明确原因。
- 不修改 Ent schema 或生成 Ent 代码。
- 不生成新的数据库 migration。
- 不执行生产迁移。
- 不改变 `make migrate-diff`、`make migrate-validate`、`make migrate-apply` 的用户入口。
- 不新增 OpenSpec/OPSX 工件。

## Acceptance Criteria

- Atlas 配置文件位于 `user-service/migrations/atlas.hcl`。
- 仓库中不再引用旧路径 `user-service/atlas.hcl`，除非是在变更记录中描述历史位置。
- `make migrate-validate` 可以通过。
- 在本地 Atlas dev database 可用时，`make migrate-diff name=<name>` 能使用新配置路径生成迁移。
- 在设置 `DATABASE_URL` 后，`make migrate-apply` 能使用新配置路径执行已提交迁移。
- Dockerfile 打包出的运行镜像包含 `user-service/migrations/atlas.hcl`。
- CI 中存在 migration validation 入口，且调用路径与新布局一致。
- 文档、脚本注释和代理规则中的 Atlas 配置位置与 migration 目录说明一致。
- `user-service/migrations/*.sql`、Ent schema、HTTP API、业务逻辑和生产迁移执行策略无语义变化。
