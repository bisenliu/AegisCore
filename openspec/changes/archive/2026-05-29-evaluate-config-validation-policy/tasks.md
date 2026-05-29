## 1. 配置加载实现调整

- [x] 1.1 移除 `common/config/loader.go` 中的 `cfg.Validate()` 调用，使 `Load` 只返回读取、反序列化或环境绑定错误。
- [x] 1.2 删除 `common/config/validate.go`，移除当前 required、范围、duration 和连接池校验实现。
- [x] 1.3 保留 `common/config/config.go` 中命名 Redis/PostgreSQL 配置结构、`RedisConfig(name)` 和 `Postgres(name)` 查询能力。
- [x] 1.4 保留 Viper YAML 读取、`AEGISCORE_` 环境变量覆盖绑定和 `Unmarshal` 反序列化能力。

## 2. 测试更新

- [x] 2.1 改写 `common/config/loader_test.go` 中缺失主要配置字段会失败的断言，验证 `Load` 成功并产生零值配置。
- [x] 2.2 改写基础范围非法值会在 `Load` 阶段失败的断言，验证 `Load` 成功并保留原始非法值。
- [x] 2.3 保留并更新命名 Redis/PostgreSQL 配置加载、YAML merge、环境变量覆盖和 PostgreSQL DSN 测试。
- [x] 2.4 确认 `common/config` 测试不再依赖字段级校验错误文案。

## 3. 启动失败边界确认

- [x] 3.1 确认 Redis ping 仍在 `common/infrastructure/redis.go` 的 Fx lifecycle 启动阶段执行并返回带实例名的错误。
- [x] 3.2 确认 PostgreSQL `user_db` 和 `common_db` ping 仍在 `common/infrastructure/postgres.go` 的 Fx lifecycle 启动阶段执行并返回带名称的错误。
- [x] 3.3 确认 `common/config.Load` 不新增任何外部 I/O 或启动前连接探测。

## 4. 验证与整理

- [x] 4.1 对修改过的 Go 文件运行 `gofmt -w`。
- [x] 4.2 在 `common/` 模块运行 `go test ./...`。
- [x] 4.3 在 `user-services/` 模块运行 `go test ./...`，验证共享配置变更未破坏服务装配编译。
