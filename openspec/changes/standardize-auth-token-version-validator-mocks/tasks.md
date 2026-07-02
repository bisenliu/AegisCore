## 1. 测试现状梳理

- [x] 1.1 检查 `user-service/internal/features/auth/application/validators/token_version_validator_test.go` 中 `tokenVersionUserTestStore` 与 `tokenVersionSessionTestStore` 的使用点。
- [x] 1.2 确认 `mock_generate.go` 已提供 `MockUserTokenVersionStore` 与 `MockTokenVersionCache`，并记录各测试需要表达的调用参数、返回值和错误路径。
- [x] 1.3 确认 token version validator 测试仍使用真实 `localcache` 实例，不引入 mock localcache。

## 2. gomock 改造

- [x] 2.1 将本地缓存命中、Redis cache hit、Redis cache miss、PostgreSQL 回源和 Redis 回填相关测试改为 gomock expectation。
- [x] 2.2 将错误路径测试改为 gomock expectation，并确认错误结果不会被本地缓存。
- [x] 2.3 删除 `tokenVersionUserTestStore` 与 `tokenVersionSessionTestStore` 定义及其辅助状态，不保留兼容测试替身。
- [x] 2.4 清理改造后不再使用的 import、辅助函数和测试状态字段。

## 3. 并发与失效场景

- [x] 3.1 使用 `DoAndReturn`、channel、mutex 或 atomic 计数重写同一用户 singleflight 合并测试，断言并发回源只执行一次。
- [x] 3.2 使用 gomock expectation 重写不同用户并发测试，断言不同用户之间不共享 singleflight 结果。
- [x] 3.3 使用 gomock expectation 重写缓存失效重载测试，断言失效后会重新读取 Redis 或 PostgreSQL 当前值。

## 4. 验证

- [x] 4.1 执行 `make user-service-generate`，确认已有 mock 生成物无 drift。
- [x] 4.2 执行 `cd user-service && go test ./internal/features/auth/application/validators` 并通过。
- [x] 4.3 执行 `make user-service-architecture-lint` 并通过。
- [x] 4.4 暂存本次预期代码和 OpenSpec 变更后执行 `make lint` 并通过。
- [x] 4.5 在 `make lint` 通过且预期变更已暂存后执行 `make verify` 并通过。
