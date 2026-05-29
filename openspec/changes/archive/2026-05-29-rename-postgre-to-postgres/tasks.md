## 1. 配置契约实现

- [x] 1.1 将 `common/config.Config` 中 PostgreSQL 配置 map 字段从 `Postgre` 改为 `PostgresConfigs`，并将 mapstructure tag 从 `postgre` 改为 `postgres`。
- [x] 1.2 更新 `Config.Postgres(name)` lookup 逻辑，确保从 `Config.PostgresConfigs` map 读取命名实例并保持返回 `PostgresDatabaseConfig` 的现有语义。
- [x] 1.3 将 `user-services/configs/config.yaml` 中 `.postgre_base`、`&postgre_base`、`*postgre_base` 和 `postgre:` 统一改为 `postgres` 命名。

## 2. 测试与调用点更新

- [x] 2.1 更新 `common/config/loader_test.go` 中所有 YAML fixture、字段断言和环境变量覆盖测试，使用 `postgres` 与 `AEGISCORE_POSTGRES_...`。
- [x] 2.2 更新 `common/infrastructure` PostgreSQL provider 测试中的配置结构字段名，确保缺失配置、连接池设置和 lifecycle 行为仍覆盖。
- [x] 2.3 更新 `user-services/internal/bootstrap` PostgreSQL pool 测试中的配置结构字段名，确保只声明并连接 `user_db` 和 `common_db`，不连接 `pay_db`。
- [x] 2.4 全仓搜索 `postgre` / `Postgre`，将运行时代码、测试 fixture、开发文档和主规格中仍表示当前配置契约的引用改为 `postgres` / `Postgres`。

## 3. 迁移工具约定同步

- [x] 3.1 检查 `user-services/scripts/` 与 `user-services/atlas.hcl` 是否存在从项目配置读取 PostgreSQL 命名实例的逻辑，如存在则改为读取 `postgres.user_db`。
- [x] 3.2 保持 `DATABASE_URL` 迁移路径可用，确认迁移脚本不启动 Fx app、Redis client、HTTP server 或 Ent runtime client。
- [x] 3.3 不生成新的 Ent 代码或 Atlas SQL migration；如实现过程中发现 Ent schema 未变化，应跳过 `go generate ./ent` 和 migration diff。

## 4. 验证

- [x] 4.1 在 `common/` 运行 `go test ./...`，确认配置加载和 PostgreSQL provider 测试通过。
- [x] 4.2 在 `user-services/` 运行 `go test ./...`，确认 bootstrap、Ent client 和用户服务测试通过。
- [x] 4.3 如修改了 shell 脚本或 Atlas 配置，运行 `user-services/scripts/migrate-validate.sh` 或说明因本地 Atlas/PostgreSQL 环境缺失无法运行。
- [x] 4.4 运行 `openspec status --change "rename-postgre-to-postgres"`，确认 change 处于 apply-ready 状态。
