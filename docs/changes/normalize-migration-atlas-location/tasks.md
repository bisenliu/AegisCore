# Tasks

## Implementation

- [x] 使用 `git mv user-service/atlas.hcl user-service/migrations/atlas.hcl` 移动 Atlas 配置文件。
- [x] 检查 `user-service/migrations/atlas.hcl` 内容，确认 `src = "ent://ent/schema"` 和 `dir = "file://migrations"` 在脚本从 `user-service/` 执行时仍正确解析。
- [x] 更新 `user-service/scripts/migrate-diff.sh`，为 `atlas migrate diff` 增加 `--config file://migrations/atlas.hcl`。
- [x] 更新 `user-service/scripts/migrate-diff.sh` 注释，将 `./atlas.hcl` 改为 `./migrations/atlas.hcl`。
- [x] 更新 `user-service/scripts/migrate-apply.sh`，为 `atlas migrate apply` 增加 `--config file://migrations/atlas.hcl`。
- [x] 更新 `user-service/scripts/migrate-apply.sh` 注释，将 `./atlas.hcl` 改为 `./migrations/atlas.hcl`。
- [x] 检查 `user-service/scripts/migrate-validate.sh`，保留 `atlas migrate validate --dir file://migrations`，并更新注释说明校验对象为 `user-service/migrations/`。
- [x] 更新 `user-service/ent/migrate/main.go` 中的 `atlas migrate diff` 调用，增加 `--config file://migrations/atlas.hcl`。
- [x] 更新 `user-service/Dockerfile`，删除 `COPY user-service/atlas.hcl /app/user-service/atlas.hcl`，确认 `COPY user-service/migrations /app/user-service/migrations` 会包含 `migrations/atlas.hcl`。
- [x] 增加或更新 CI migration validation，使 CI 通过 `make migrate-validate` 或脚本入口验证迁移目录。
- [x] 更新 `AGENTS.md` 中 Atlas 配置路径、migration 目录说明和关键入口列表。
- [x] 更新 `docs/ARCHITECTURE.md` 的 Database Migrations 章节，将 Atlas 配置位置改为 `user-service/migrations/atlas.hcl`。
- [x] 更新 `docs/DEVELOPMENT.md` 的 migration directory layout、迁移流程和路径说明。
- [x] 更新 `docs/TESTING.md` 中涉及 migration validation 的路径说明。
- [x] 更新 `user-service/ent/README.md` 中涉及 Atlas 配置或 migration 验证的路径说明。
- [x] 使用 `rg` 检查旧路径引用，确认除本变更记录描述历史位置外没有残留 `user-service/atlas.hcl` 或 `./atlas.hcl`。

## Verification

- [x] 运行 `make migrate-validate`，确认通过。
- [x] 运行 `make migrate-diff`，确认缺少 `name` 时仍输出用法并失败。
- [x] 在本地 Atlas dev database 可用时运行 `make migrate-diff name=verify_atlas_location`，确认命令能读取 `migrations/atlas.hcl`；如生成临时 SQL，删除验证产物并恢复 `atlas.sum`。
- [x] 在可用非生产数据库上设置 `DATABASE_URL` 后运行 `make migrate-apply`，确认命令能读取 `migrations/atlas.hcl`。
- [x] 如新增或更新 CI workflow，运行或静态检查 workflow，确认命令不硬编码旧路径。
- [x] 检查 Dockerfile 构建上下文，确认运行镜像包含 `/app/user-service/migrations/atlas.hcl`。
- [x] 检查 `git diff -- user-service/migrations user-service/scripts user-service/Dockerfile .github AGENTS.md docs user-service/ent/README.md user-service/ent/migrate/main.go`，确认没有修改历史 SQL 内容、Ent schema、Ent generated code、HTTP API 或业务逻辑。

`make migrate-diff name=verify_atlas_location` 已确认能读取 `migrations/atlas.hcl`。该验证在当前 schema 状态下生成了一个临时 comment-only SQL diff，已删除临时 SQL 并恢复 `atlas.sum`，本变更不提交 SQL 语义变化。

## Review Notes

- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
- [x] 确认 `user-service/migrations/*.sql` 无内容变化。
- [x] 确认 `user-service/migrations/atlas.sum` 没有因配置文件移动被不必要重写。
- [x] 确认根目录 Makefile 迁移入口保持不变。
- [x] 确认文档与脚本中的 Atlas 配置路径一致。
