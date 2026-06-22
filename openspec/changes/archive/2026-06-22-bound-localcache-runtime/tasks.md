## 1. OpenSpec 与依赖准备

- [x] 1.1 确认 `bound-localcache-runtime` proposal、design 和 delta specs 通过 OpenSpec 状态检查。
- [x] 1.2 在 `common/go.mod` 中新增 `github.com/dgraph-io/ristretto/v2` 依赖，并运行 `go mod tidy` 更新 `common/go.sum`。

## 2. common/runtime/localcache 核心实现

- [x] 2.1 删除旧 `sync.Map + ttl` 实现和旧 `New(ttl)` API，不保留兼容代码。
- [x] 2.2 实现基于 Ristretto v2 的 `Cache`、`Config`、`Loader`、`CloneFunc`、`Stats` 和错误类型。
- [x] 2.3 实现 `Get`、`GetOrLoad`、内部 `lookup`、`Set`、`Delete`、`Clear`、`Stats` 和 `Close`。
- [x] 2.4 在 `GetOrLoad` 中封装 `singleflight.DoChan`、double-check、loader 错误不缓存、`LoadTimeout` 和关闭后拒绝新请求。
- [x] 2.5 增加 localcache 单元测试，覆盖无效配置、容量、TTL、singleflight 合并、loader 错误不缓存、Set/Delete/Clear、Close、clone 隔离和统计不污染 hit ratio。

## 3. 配置与服务组装

- [x] 3.1 在 `common/runtime/config` 增加 `LocalCacheConfig` 和 `LocalCacheInstanceConfig`，支持 `auth_token_version` 与 `rbac_user_roles` 配置。
- [x] 3.2 为 localcache 配置增加 validation，校验容量、TTL、load timeout、缓存名和默认值。
- [x] 3.3 更新 `user-service/configs/config.yaml`、loader 测试、bootstrap 测试和 e2e 测试配置。
- [x] 3.4 在 user-service Fx/provider 层组装 auth token version 本地缓存和 RBAC user role 本地缓存，并注册 `Close` 生命周期。

## 4. auth 与 RBAC 调用点迁移

- [x] 4.1 重构 auth `TokenVersionValidator` 使用 `localcache.Cache[string,int64].GetOrLoad`，删除本地 `singleflight.Group` 和旧 localcache API 调用。
- [x] 4.2 保持 token version 本地失效语义，`InvalidateTokenVersion` 调用新缓存 `Delete`。
- [x] 4.3 重构 RBAC `entUserRoleResolver` 使用 `localcache.Cache[uuid.UUID,[]uuid.UUID].GetOrLoad`，删除手写 `singleflight.Group` 和缓存指针替换。
- [x] 4.4 保持 RBAC 用户角色缓存 clone、单用户失效和全量 `Clear` 语义。

## 5. 观测指标

- [x] 5.1 在 `common/runtime/observability/metrics` 增加 localcache collector，读取 `StatsProvider` 并导出低基数 Prometheus 指标。
- [x] 5.2 在 user-service runtime dependency metrics 注册 auth 与 RBAC localcache collectors。
- [x] 5.3 增加 metrics 单元测试，验证指标名称、标签和值，并覆盖 metrics disabled 时不注册 collector。

## 6. 验证与收尾

- [x] 6.1 运行 `go test ./...` 于 `common` 模块。
- [x] 6.2 运行相关 user-service 包测试，至少覆盖 auth validators、permission casbin、providers metrics/config。
- [x] 6.3 运行 `make user-service-architecture-lint`。
- [x] 6.4 如配置或生成物存在 drift，运行必要生成或格式化命令并检查 `git diff`。
- [x] 6.5 更新 tasks checkbox 状态并确认 `openspec status --change bound-localcache-runtime` apply-ready。
