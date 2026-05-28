## 1. 配置加载实现调整

- [x] 1.1 移除 `common/config/config.go` 中所有 `validate` tag，使配置结构体不声明 required/optional 或范围规则。
- [x] 1.2 移除 `common/config/loader.go` 中的显式 required key 校验流程，包括 `validateRequiredKeys` 和 `explicitRequiredKeys`。
- [x] 1.3 移除 `common/config/loader.go` 中的结构体验证流程，包括 validator 初始化、错误格式化和相关辅助函数。
- [x] 1.4 保留 Viper YAML 读取、`AEGISCORE_` 环境变量覆盖绑定和 `Unmarshal` 反序列化能力。
- [x] 1.5 检查 `go.mod`/`go.sum`，如 `go-playground/validator` 不再被使用则移除直接依赖并整理模块校验文件。

## 2. 测试更新

- [x] 2.1 删除或改写 `common/config/loader_test.go` 中缺失必填配置会失败的断言。
- [x] 2.2 删除或改写基础范围非法值会在 `Load` 阶段失败的断言。
- [x] 2.3 新增测试验证缺失主要配置字段时 `Load` 仍成功并产生对应零值配置。
- [x] 2.4 保留配置文件读取、环境变量覆盖、可选字段省略和 PostgreSQL 命名 DSN 测试。

## 3. 启动失败边界确认

- [x] 3.1 确认 Redis ping 仍在 `common/infrastructure/redis.go` 的 Fx lifecycle 启动阶段执行并返回错误。
- [x] 3.2 确认 PostgreSQL `user_db` 和 `common_db` ping 仍在 `common/infrastructure/postgres.go` 的 Fx lifecycle 启动阶段执行并返回带名称的错误。
- [x] 3.3 确认 `common/config.Load` 不新增任何外部 I/O 或启动前连接探测。

## 4. 验证与整理

- [x] 4.1 对修改过的 Go 文件运行 `gofmt -w`。
- [x] 4.2 在 `common/` 模块运行 `go test ./...`。
- [x] 4.3 在 `user-services/` 模块运行 `go test ./...`，验证共享配置变更未破坏服务装配编译。
- [x] 4.4 确认 OpenSpec delta 与实现方向一致，所有任务完成后再进入归档。
