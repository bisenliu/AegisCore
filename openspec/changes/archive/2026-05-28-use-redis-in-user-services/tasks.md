## 1. 用户服务 Redis Provider

- [x] 1.1 在 `user-services/internal/bootstrap` 新增 Redis provider 文件，定义 `cache_redis` 实例名常量。
- [x] 1.2 实现用户服务 Redis provider，调用 `common/infrastructure.NewRedisClient` 创建 `cache_redis`。
- [x] 1.3 使用 `fx.Out` 提供具名 `cache_redis` `*redis.Client`，供用户服务内部组件注入。
- [x] 1.4 将 Redis provider 注册到 `user-services/internal/bootstrap.Module` 的 `fx.Provide`。

## 2. 依赖声明与运行时行为

- [x] 2.1 确认 `user-services/go.mod` 对 `github.com/redis/go-redis/v9` 的依赖声明与直接代码引用一致。
- [x] 2.2 确保用户服务只声明 `cache_redis`，不声明或连接 `queue_redis`。
- [x] 2.3 保持现有 controller/service/repository 不注入 Redis，除非后续业务能力明确需要。

## 3. 测试

- [x] 3.1 更新 `user-services/internal/bootstrap` 测试，验证 Fx app 可以解析具名 `cache_redis` client。
- [x] 3.2 更新 Redis lifecycle 测试，验证用户服务启动时 ping `cache_redis`、停止时 close。
- [x] 3.3 更新负向测试，验证用户服务不提供 `queue_redis`。
- [x] 3.4 更新启动失败测试，验证 `cache_redis` 配置缺失或不可用时返回清晰错误。

## 4. 验证

- [x] 4.1 对修改过的 Go 文件运行 `gofmt -w`。
- [x] 4.2 在 `user-services/` 运行 `go test ./...`。
- [x] 4.3 在 `common/` 运行 `go test ./...`，确认共享基础设施未回归。
