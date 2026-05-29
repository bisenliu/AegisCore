## 1. Atlas 与 Ent 迁移基础

- [x] 1.1 确认 Atlas Ent schema source 所需依赖，并保持模块依赖可解析
- [x] 1.2 在 `user-services/ent/migrate/main.go` 封装 Ent schema inspect/diff，输出 Atlas 可读取的 PostgreSQL schema source
- [x] 1.3 在 `user-services/atlas.hcl` 配置 `data.external_schema.ent`、本地开发环境、部署环境和 `file://migrations` migration directory
- [x] 1.4 确认迁移生成流程不调用 `client.Schema.Create(ctx)`，并不修改 `user-services/ent/` 下生成代码

## 2. 迁移目录与生成脚本

- [x] 2.1 创建 `user-services/migrations/` 并生成或初始化 `atlas.sum`
- [x] 2.2 新增 `user-services/scripts/migrate-diff.sh`，封装 `atlas migrate diff <name> --env local` 和 `atlas migrate hash --dir file://migrations`
- [x] 2.3 新增 `user-services/scripts/migrate-validate.sh`，校验 migration directory 与 `atlas.sum` 一致
- [x] 2.4 基于现有 `user-services/ent/schema/user.go` 生成首个 baseline SQL migration，并提交 SQL 与 `atlas.sum`

## 3. 人工审查与安全 SQL 调整

- [x] 3.1 在开发文档或脚本注释中说明人工修改 SQL 后必须运行 `atlas migrate hash --dir file://migrations`
- [x] 3.2 提供 PostgreSQL `CREATE INDEX CONCURRENTLY` 的安全调整示例，并标注不能在事务中执行的注意事项
- [x] 3.3 验证修改 SQL 但不更新 `atlas.sum` 时，`migrate-validate.sh` 或 Atlas 校验会失败

## 4. 部署与容器集成

- [x] 4.1 新增 `user-services/scripts/migrate-apply.sh`，使用 `DATABASE_URL` 和 `atlas migrate apply --env deploy` 执行已提交 migration
- [x] 4.2 新增或更新用户服务 entrypoint，使迁移成功后才启动 `./user-services/cmd serve`，迁移失败时直接退出
- [x] 4.3 更新 Dockerfile 或提供 Dockerfile 示例，打包 Atlas CLI、用户服务二进制、配置文件和 `user-services/migrations/`
- [x] 4.4 说明推荐 CI/CD 独立 migration job 执行方式，以及容器启动前执行方式的适用场景和并发注意事项

## 5. 验证与文档

- [x] 5.1 运行 `go generate ./ent` 验证 Ent 生成流程仍可用且未手写生成代码
- [x] 5.2 分别在 `common/` 和 `user-services/` 运行 `go test ./...`
- [x] 5.3 在可用测试数据库上执行 Atlas migration validate/apply 流程，确认 baseline migration 可应用
- [x] 5.4 更新 `docs/DEVELOPMENT.md` 或相关开发文档，记录服务内迁移目录方案、生成命令、人工 review 规则和部署执行步骤
- [x] 5.5 如实现新增长期能力，更新 `docs/opsx/CAPABILITY_MAP.md` 将 `database-schema-migrations` 映射到实现文件与主规格
