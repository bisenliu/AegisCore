## 1. 配置模型更新

- [x] 1.1 将 `common/config.Config` 中的 PostgreSQL 配置从 `map[string]PostgresConfig` 调整为共享连接参数结构，包含 `host`、`port`、`username`、`password`、`user_db_name`、`pay_db_name`、`common_db_name` 和连接池配置字段
- [x] 1.2 更新 `common/config` 的默认值逻辑，为 PostgreSQL driver、连接池大小、连接生命周期和 ping timeout 保留现有默认行为
- [x] 1.3 更新 `common/config` 的校验逻辑，确保 `host`、`port`、`username`、`user_db_name`、`common_db_name` 缺失或无效时返回清晰错误
- [x] 1.4 使用标准库 URL 构造方式实现命名数据库连接配置生成，避免用户名或密码特殊字符导致 DSN 转义错误

## 2. PostgreSQL 基础设施更新

- [x] 2.1 调整 `common/infrastructure/postgres.go`，使 `user_db` 和 `common_db` 连接池从新 PostgreSQL 配置生成连接信息
- [x] 2.2 保持 Fx 输出的 `user_db` 和 `common_db` 命名 `*sql.DB` 不变，避免影响现有 Ent client 装配
- [x] 2.3 确认 `pay_db_name` 仅作为配置字段可读取，不新增支付 API、支付 repository、支付 Ent client 或额外启动依赖

## 3. 示例配置与文档一致性

- [x] 3.1 将 `user-services/configs/config.yaml` 的 `database.postgres` 更新为新格式：`host`、`port`、`username`、`password`、`user_db_name`、`pay_db_name`、`common_db_name`
- [x] 3.2 搜索仓库中旧 `database.postgres.<name>.dsn` 示例或说明，并更新为新的 PostgreSQL 配置格式
- [x] 3.3 确认能力地图仍将支付服务或 `pay_db` 业务列为未来候选能力，避免文档暗示本变更已实现支付能力

## 4. 测试与验证

- [x] 4.1 为 `common/config` 添加或更新配置加载测试，覆盖新 PostgreSQL 格式、默认值应用和缺失必填字段错误
- [x] 4.2 为 DSN 构造添加测试，覆盖 `user_db_name`、`common_db_name` 和用户名/密码特殊字符转义
- [x] 4.3 如修改 Go 源码，运行 `gofmt -w` 格式化相关文件
- [x] 4.4 在 `common/` 目录运行 `go test ./...` 并确认通过
- [x] 4.5 在 `user-services/` 目录运行 `go test ./...` 并确认通过
- [x] 4.6 确认未修改 Ent schema 或 `user-services/ent/` 生成代码，因此无需运行 `go generate ./ent`
- [x] 4.7 运行 `openspec status --change "update-postgres-db-config"`，确认 change 处于可应用状态
