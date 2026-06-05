## 1. 符号重命名

- [x] 1.1 在 `common/runtime/config/config.go` 中将 `PostgresDatabaseConfig` 重命名为 `PostgresDBConfig`。
- [x] 1.2 将 `Config.PostgresDatabase(name string)` 重命名为 `Config.PostgresDatabaseConfig(name string)`，并确保返回类型为 `(PostgresDBConfig, bool)`。
- [x] 1.3 确认 `PostgresDatabaseConfig{}` 空值返回和正常返回字面量均改为 `PostgresDBConfig{}`，且 DSN、连接池参数和 ping timeout 赋值不变。

## 2. 引用迁移

- [x] 2.1 全局搜索 `PostgresDatabaseConfig` 并更新代码、测试、注释和文档中的旧类型名引用。
- [x] 2.2 全局搜索 `PostgresDatabase(` 并将项目内调用迁移为 `PostgresDatabaseConfig(`。
- [x] 2.3 检查 `common/runtime/datastorefx` 和 `user-services/internal/bootstrap` 的 PostgreSQL provider wiring，确认仍只连接调用方声明的命名实例。

## 3. 验证

- [x] 3.1 对修改过的 Go 文件运行 `gofmt`。
- [x] 3.2 在 `common/` 运行 `go test ./...`，确认共享配置和 datastore 相关包编译通过。
- [x] 3.3 在 `user-services/` 运行 `go test ./...`，确认用户服务 bootstrap 和依赖注入引用编译通过。
- [x] 3.4 再次搜索旧符号，确认仓库中不存在 `PostgresDatabaseConfig` 类型引用和 `PostgresDatabase(` 方法调用。
