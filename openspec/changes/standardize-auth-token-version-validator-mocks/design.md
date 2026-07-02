## Context

`user-service/internal/features/auth/application/validators` 已有 `mock_generate.go` 生成的 `UserTokenVersionStore` 与 `TokenVersionCache` gomock mock，但 token version validator 测试仍保留手写 store/cache 测试替身。当前测试能够覆盖本地缓存、Redis miss、PostgreSQL 回源、singleflight 合并和缓存失效重载，但依赖交互隐藏在替身内部，和同包其他 gomock 风格不一致。

本变更只调整测试实现。生产 validator、auth middleware、Redis adapter、`common/runtime/localcache`、Ent schema、OpenAPI、部署和观测资产均不变。

## Goals / Non-Goals

**Goals:**

- 使用已有 gomock 生成物替换 token version validator 测试中的手写 user store 和 token cache。
- 删除 `tokenVersionUserTestStore` 与 `tokenVersionSessionTestStore`，不保留兼容层或双路径测试替身。
- 用 gomock expectation 明确表达 Redis cache hit/miss、PostgreSQL 回源、Redis 回填、错误不缓存、失效重载和 singleflight 合并行为。
- 保留真实 `localcache` 实例，继续验证实例内本地缓存容量、命中和失效语义。
- 保持测试范围限定在 `user-service/internal/features/auth/application/validators` 包内。

**Non-Goals:**

- 不修改 token version validator 生产代码或 port 定义。
- 不修改 `localcache` primitive、auth middleware、provider 装配或配置语义。
- 不改 Redis integration/miniredis 测试，不新增全局测试 helper 或跨 feature mock。
- 不改变 HTTP API、OpenAPI、数据库 schema、migration、部署清单或观测资产。

## Decisions

- 使用 `mock_generate.go` 已生成的 gomock mock，而不是继续维护手写替身。理由是 validator 依赖已经有明确 port mock，expectation 能直接表达调用次数、参数、返回值和错误路径。备选方案是保留手写替身并只清理命名，但这会继续隐藏 port 交互并造成测试风格分裂。
- 保留真实 `localcache`，不 mock 本地缓存。理由是本地缓存是 token version validator 的核心行为之一，真实实例能覆盖缓存命中、失效和 singleflight 与回源路径的组合。备选方案是 mock localcache，但会把关键缓存语义退化为测试自我实现。
- singleflight 并发场景在测试内部通过 `DoAndReturn`、channel、mutex 或 atomic 计数控制阻塞和释放。理由是并发合并需要可观测地限制回源调用次数，同时避免引入跨测试共享状态。备选方案是使用 sleep 等时间等待，但会增加 flake 风险。
- 按测试用例本地化构造 mock expectation，不新增全局 helper。理由是本次范围仅为 validators 包内测试，局部 helper 可以存在于测试文件内，但不应引入跨 feature 或共享测试抽象。备选方案是创建共享 helper，但会扩大边界且不符合“不新增全局测试 helper”的约束。

## Risks / Trade-offs

- gomock expectation 过细可能让测试对实现调用顺序过度敏感。缓解方式：只对安全语义和缓存路径必要的调用次数、参数和返回值设置 expectation，避免无意义的严格顺序约束。
- 并发测试如果依赖时间等待可能不稳定。缓解方式：使用 channel、mutex 或 atomic 计数同步 goroutine，不以 sleep 作为主要断言机制。
- 删除手写替身后，部分测试可读性可能因 gomock 设置增多而下降。缓解方式：在测试内部使用小型局部构造函数减少重复，但不抽象出跨包 helper。
- mockgen drift 可能说明 mock 生成物未同步。缓解方式：执行 `make user-service-generate` 并确认无生成差异。

## Migration Plan

- 在 `token_version_validator_test.go` 中逐步替换手写替身实例为 gomock controller 和对应 mock。
- 删除 `tokenVersionUserTestStore` 与 `tokenVersionSessionTestStore` 定义及其辅助状态。
- 调整 singleflight、按用户隔离和失效重载测试，使依赖调用通过 expectation 和并发同步原语表达。
- 执行 `make user-service-generate`，确认 mock 生成物无 drift。
- 执行 `cd user-service && go test ./internal/features/auth/application/validators`。
- 执行 `make user-service-architecture-lint`。

回滚方式为恢复该测试文件到变更前版本；由于不涉及生产代码、数据库或部署资产，无运行时迁移和线上回滚步骤。

## Open Questions

- 无。
