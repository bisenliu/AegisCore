## 1. Common PostgreSQL 基础能力

- [x] 1.1 将 `common/infrastructure/postgres.go` 中固定输出 `user_db` 和 `common_db` 的 `PostgresPools`/`NewPostgresPools` 调整为单个 PostgreSQL 连接池创建与生命周期注册 API。
- [x] 1.2 保留连接池参数设置、启动 `PingContext`、停止 `Close`、日志字段和错误包装行为，并确保单库 API 可被服务侧 provider 复用。
- [x] 1.3 更新 `common/infrastructure/module.go`，使共享模块继续提供配置、日志和 Redis client，但不再固定提供所有已知业务数据库连接池。

## 2. 用户服务数据库装配

- [x] 2.1 在 `user-services` 模块中新增服务侧 PostgreSQL provider，显式输出具名 `user_db` 与 `common_db` 的 `*sql.DB`。
- [x] 2.2 将用户服务 Fx module 注册新增 provider，保持 `entclient.NewClients` 继续通过 `name:"user_db"` 和 `name:"common_db"` 注入。
- [x] 2.3 确认 controller/service/repository 分层不变，HTTP API、响应信封和 Ent client 注入名不变。

## 3. 测试与验证

- [x] 3.1 为 common PostgreSQL 单库创建/生命周期注册能力补充或更新单元测试，覆盖缺失数据库配置、连接池参数设置和生命周期注册行为。
- [x] 3.2 为用户服务 Fx 装配补充或更新测试，验证用户服务提供 `user_db` 和 `common_db`，且 `common/infrastructure.Module` 不再隐式提供固定数据库集合。
- [x] 3.3 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 3.4 分别在 `common/` 和 `user-services/` 运行 `go test ./...`，记录并修复与本变更相关的失败。
