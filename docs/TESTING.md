# AegisCore 测试说明

## 1. 验证入口

| 命令 | 用途 |
|---|---|
| `make test` | 运行 `common` 和 `user-service` 的 Go 测试 |
| `make test-containers` | 显式运行 `common` 和 `user-service` 的全部 Docker-backed 测试 |
| `make coverage` | 生成覆盖率报告并检查 `common` 与 user-service 手写包覆盖率基线 |
| `make lint` | 运行各模块 `golangci-lint` |
| `make user-service-architecture-lint` | 检查 user-service 架构边界、生成物 drift 和 OPSX 文档语言约束 |
| `make user-service-openapi-generate` | 生成 OpenAPI 3 文档 |
| `make user-service-image-verify` | 校验 user-service Distroless 镜像静态链接、UID/GID、基础数据和禁止工具 |
| `make compose-dashboard-check` | 检查 Compose Grafana dashboard 是否与通用 dashboard 一致 |
| `make verify` | 运行 lint、架构 lint、测试、OpenAPI 生成和 `git diff --exit-code` |

## 2. 单元测试

Go 单元测试位于对应包内，以 `_test.go` 结尾。常见覆盖范围：

- `common/contract/`：错误、响应和分页契约。
- `common/http/`：binding、middleware、OpenAPI 和 response helper。
- `common/runtime/`：config、datastore、logger、metrics、scheduler、workerpool、timezone。
- `common/security/`：JWT verifier、token version validator 契约、Casbin authorizer、password；user-service token issuer 和 claims 测试位于 `user-service/internal/features/auth/`。
- `user-service/internal/features/*/domain`：领域对象和错误。
- `user-service/internal/features/*/application`：command、query、validator、service 和 seed。
- `user-service/internal/features/*/transport/http`：controller、input、mapper 和 routes。
- `user-service/internal/bootstrap` 与 `internal/providers`：通过 `bootstrap.AppOptions` 或显式 supply 同源 service/runtime config 验证 composition root、provider 依赖和 Fx lifecycle timeout，不在测试中复制 Nacos loader 或二次读取配置来源。

运行单模块测试：

```bash
make common-test
make user-service-test
```

### 覆盖率门禁

CI 的 coverage job 和本地 `make coverage` 使用同一个仓库级脚本 `scripts/check-coverage.sh`。脚本输出并上传三个 profile：

- `coverage/common.out` 与 `coverage/common.txt`：`common` 全量 statement coverage，默认最低基线为 `COMMON_MIN_COVERAGE=75.0`。
- `coverage/user-service.out` 与 `coverage/user-service.txt`：user-service 全量 statement coverage，仅用于可见性，数值会包含 Ent/OpenAPI 生成代码。
- `coverage/user-service-handwritten.out` 与 `coverage/user-service-handwritten.txt`：user-service 手写包覆盖率门禁，排除 `user-service/docs/openapi.go` 和 `user-service/internal/persistence/ent/` 下除 `schema/` 外的 Ent 生成物，默认最低基线为 `USER_SERVICE_HANDWRITTEN_MIN_COVERAGE=75.0`。
- changed-code coverage：在 PR 或设置 `CHANGED_COVERAGE_BASE` 时检查 `common` 与 user-service 手写 Go 文件变更行，默认最低基线为 `CHANGED_CODE_MIN_COVERAGE=80.0`；只统计落入 coverage block 的可执行行。

调整基线时必须显式覆盖环境变量并同步记录原因，例如：

```bash
COMMON_MIN_COVERAGE=76.0 USER_SERVICE_HANDWRITTEN_MIN_COVERAGE=76.0 make coverage
CHANGED_COVERAGE_BASE=origin/main CHANGED_CODE_MIN_COVERAGE=85.0 make coverage
```

Localcache 行为或消费边界变化后，需用 race detector 覆盖固定 TTL、容量驱逐、singleflight、caller 取消、loader timeout、强失效和 fail-closed 场景：

```bash
cd common
go test -race ./runtime/localcache ./runtime/observability/metrics

cd ../user-service
go test -race ./internal/features/auth/... ./internal/features/permission/...
```

容量统计只断言 `aegiscore_localcache_capacity_evictions_total`；TTL 到期、`Invalidate` 和 `InvalidateAll` 不得增加该指标。并发失效测试应使用 channel 或 barrier 控制 loader 发布顺序，证明首次竞态透明重试、连续竞态返回 `ErrInvalidated`，不得使用固定 `time.Sleep` 推测竞态窗口。

### 测试替身与生成

- 生成 mock 的入口放在消费测试所在 package 的 `mock_generate.go`，文件必须使用 `//go:build generate`，使 `go generate ./...` 可以发现指令而普通构建不会编译测试生成入口。
- 测试 command tree、provider 或其他依赖集合时，使用位于 `_test.go` 的完整 dependency fixture。fixture 对未声明的依赖调用应立即失败，单个测试只覆盖自身目标依赖，避免生产代码通过 nil/default 分支补齐测试输入。
- 外部协作者 port 的失败注入、调用次数和顺序优先使用消费包本地生成的 gomock；真实算法行为继续使用真实轻量 service。
- 协议 server、数据库 driver、并发记录器、通道同步 executor、OTel exporter、只读 stats source 等具有协议、并发或状态语义的测试替身可以保留，不应机械替换为 gomock。
- 不要新增跨 feature testing facade、共享断言 wrapper、全局可变 hook、`ForTest` 正式 API 或仅为测试服务的兼容 adapter。

## 3. 集成测试和 e2e

集成和 e2e 测试使用真实依赖时优先复用 `common/testing/containers/`：

- `common/testing/containers/postgres.go`
- `common/testing/containers/redis.go`

user-service e2e 位于 `user-service/tests/e2e/`，覆盖 HTTP flow、migration 和测试 harness。HTTP harness 在启动 Fx App 前加载一次 service config，并复用 `bootstrap.AppOptions`，使测试与正式 App 使用相同的 service/runtime config 和 composition root 接线。CI 的阻塞式 `container-test` job 运行根 `make test-containers`：根 target 分别调用 common 与 user-service 的模块 target，由模块 target 显式传递 `-aegiscore.testcontainers` flag，覆盖 common PostgreSQL/Redis fixture、permission/role PostgreSQL 集成测试和 user-service HTTP E2E；普通单测只由复用质量 workflow 的 `unit` job 运行一次。

配置 fixture 必须使用最终严格契约：核心路径为 `app/server/log/observability`，服务资源位于 `resources.redis.cache_redis` 和 `resources.postgres.primary_db`，feature cache 位于 `auth.token_version_cache` 与 `rbac.user_role_cache`。Redis 正向 fixture 必须显式配置 `mode`；`cluster` 使用 `addrs`、`timeout` 和可选 `cluster.max_redirects`，`standalone` 使用 `addr` 和 `timeout`，两种 mode 均固定使用 Redis 0 号库且不暴露 `db` 配置。旧路径只允许出现在 strict decoder 负向测试中，用于证明未知字段会被拒绝；正向 fixture 必须能够通过 Nacos 分层配置合成后的 strict decode。

版本化本地 Nacos 配置位于 `deployments/nacos/local-host/` 与 `deployments/nacos/local-docker/`。测试必须分别严格解码两个目录中的完整三文档，断言公共字段一致，并确认 Namespace、dataId 顺序、Compose DNS 和宿主映射端口与 Compose 一致。`tools/nacos-config-seed` 测试还必须覆盖目录文档在网络写入前完成读取与基本校验。

运行：

```bash
make test-containers
```

`make test-containers` 是完整真实依赖门禁的唯一仓库入口。common 与 user-service 的模块 target 使用 `-v -count=1`，在日志中记录实际执行的测试名和耗时，并禁止 Go test cache 代替真实执行。Docker daemon、镜像拉取、容器启动、连接、migration 或配置失败时必须使测试失败；已传入 `-aegiscore.testcontainers` 后 Docker-backed 测试不得静默 skip。`AEGISCORE_TEST_CONTAINERS` 不是受支持的开关。

### RBAC policy sync 故障注入

RBAC 写后同步 unit harness 位于 permission application、Redis watcher 和 Casbin engine 相关测试包，使用 `_test.go` 内 fake 控制 loader 阻塞、Redis publish 失败、dispatcher 重试、watcher 消息乱序和 user-role cache 解析延迟。Docker-backed 验收位于 role PostgreSQL adapter 和 `user-service/tests/e2e/rbac_sync_recovery_test.go`，穿透真实 PostgreSQL transaction、生产 outbox store/dispatcher/Redis publisher/watcher 和两个 Casbin engine。测试必须通过 channel、barrier、`require.Eventually` 或明确 deadline 等待状态谓词，不使用固定 `time.Sleep` 作为状态变化的主要判断。

覆盖场景和风险：

- Redis publish 或 Pub/Sub 故障后恢复：验证数据库 revision 已提交但通知链路失败时，dispatcher 重试和 watcher 数据库 revision 补偿可在没有新 RBAC 写入的情况下使两个副本 lag 归零，并通过恢复前 deny、恢复后 allow 证明 Casbin projection 已更新。
- Watcher 自恢复状态机：使用可控 subscriber 分别注入初始订阅确认失败、运行期 Receive 终止、持续重连和恢复后消息，断言带抖动退避不超过配置上限、成功确认只清除 subscription 当前错误，并且订阅退避期间 PostgreSQL revision 周期校准继续推进 `LastReconcileSuccessAt`。
- Watcher 停止阶段：分别在订阅确认、Receive、退避和 payload 数据库查询阻塞时取消根 context，断言停止后不再订阅、每个 PubSub 恰好关闭一次、正常取消不记录故障，并以 `go test -race` 检查 goroutine 生命周期和状态竞争。
- reload 乱序完成：验证后发 revision 先完成、先发 revision 后完成时，旧 projection 不会覆盖最新 applied revision，授权 allow/deny 结果必须对应最新数据库状态。
- Add/Remove/Replace 重放：验证 dispatcher 重试、重复投递和乱序 watcher 事件不会丢通知，也不会因非幂等副作用破坏最终 projection 或 cache 失效语义。
- 100 并发 RBAC 写：使用真实 PostgreSQL mutation 验证业务行、commit-ordered revision 和 pending outbox 一一对应且 revision 连续唯一；授权收敛由独立的 Redis 故障恢复 e2e 断言覆盖。

运行 unit harness：

```bash
cd user-service
go test -race ./internal/features/permission/application ./internal/features/permission/infrastructure/redis ./internal/features/permission/infrastructure/casbin
go test -race ./internal/features/permission/infrastructure/redis ./internal/features/permission ./internal/providers/observability
```

运行真实提交顺序、100 并发和 Redis 恢复验收：

```bash
cd user-service
go test ./internal/features/role/infrastructure/postgres -run TestPostgresPolicyRevisionFollowsCommitOrderAndHandlesConcurrentWrites -count=1 -args -aegiscore.testcontainers
go test ./tests/e2e -run TestRBACOutboxRedisRecoveryConvergesAllProjectionsWithoutNewWrite -count=1 -args -aegiscore.testcontainers
```

完整真实 PostgreSQL/Redis 集成门禁使用仓库根入口：

```bash
make test-containers
```

## 4. 性能和容量验收

本地 Compose 环境的可重复性能入口是 `deployments/compose/scripts/generate-real-metrics-load.sh`。脚本覆盖管理员登录、refresh、认证异常、用户创建/列表、角色启停、RBAC 授权和 watcher revision 检查，并采集 user-service `/metrics` 与 Prometheus 查询快照。

默认预算：

- `CONCURRENCY=8`、`DURATION=60`，用于覆盖常规并发流量。
- `MIN_REQUESTS=100`，防止空跑或流量不足。
- `MAX_P95_SECONDS=1.0`，限制客户端观测 HTTP p95。
- `MAX_ERROR_RATE_PERCENT=1.0`，限制 `000` 和 `5xx` 请求比例。

100 并发验收和容量记录示例：

```bash
CONCURRENCY=100 DURATION=120 MIN_REQUESTS=1000 MAX_P95_SECONDS=2.0 MAX_ERROR_RATE_PERCENT=1.0 \
  ./deployments/compose/scripts/generate-real-metrics-load.sh
```

执行后必须保留脚本输出的三个 artifact 路径，并在验收记录中说明：

- HTTP 请求数、p95 和错误率是否满足预算。
- `aegiscore_postgres_pool_open_connections`、Redis 健康指标、`process_resident_memory_bytes`、`go_goroutines` 和 CPU/内存采样是否接近 Compose、Kubernetes 或 Helm 中配置的上限。
- HPA 最大副本、单副本数据库连接池上限和 PostgreSQL `max_connections` 是否满足 `maxReplicas * 单副本最大连接数 + 管理/迁移预留连接 <= 数据库可用连接`。
- Redis 连接数、Pub/Sub watcher 和 RBAC 写入收敛是否在 100 并发场景下无持续 lag 或重试堆积。

## 5. 断言和失败处理

测试断言与失败处理优先使用 `testify/require`，通过立即失败机制减少后续空指针、错误状态级联和手写判断样板。测试应优先使用能够准确表达意图的语义化断言，而不是通过 `True`、`False`、手写 `if` 或组合多个基础断言来表达同一语义。

当 `testify` 已提供更具体的语义化断言时，应优先使用对应断言，从而让失败信息包含具体差异、缺失项或冲突项，而不是只输出模糊的 `Expected true, got false`。常见检查应使用语义化断言方法：

- 错误返回值：`require.NoError`、`require.Error`、`require.ErrorIs`、`require.ErrorContains`。
- 对象和值：`require.Equal`、`require.NotEqual`、`require.Nil`、`require.NotNil`。
- 数值与范围：`require.Greater`、`require.Less`、`require.GreaterOrEqual`、`require.LessOrEqual`。
- 集合和字符串：`require.Len`、`require.Empty`、`require.NotEmpty`、`require.Contains`、`require.ElementsMatch`。
- 专属类型断言：当知晓 `testify` 已提供更精确匹配的专属方法时，优先使用 `require.JSONEq`、`require.Regexp`、`require.WithinDuration`、`require.Panics` 等方法，避免组合多个基础断言拼凑复杂自定义逻辑。
- 布尔状态：只有断言本身就是布尔状态，或没有更具体的语义化方法时，才使用 `require.True` 或 `require.False`。

不要把手写失败判断机械替换成 `require.FailNow`、`require.FailNowf`、`require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`。存在明确语义化断言时，必须优先使用对应的 `require` 或 `assert` 方法。

可接受示例：

```go
got, err := service.Handle(ctx, input)
require.NoError(t, err)
require.NotNil(t, got)
require.Equal(t, wantID, got.ID)
require.Len(t, got.Items, 2)
require.ElementsMatch(t, wantTags, got.Tags)
require.Greater(t, got.Score, 0)
require.True(t, cache.IsReady())
```

避免示例：

```go
if err != nil {
    t.Fatalf("handle failed: %v", err)
}
if got.ID != wantID {
    t.Errorf("id = %s, want %s", got.ID, wantID)
}
require.True(t, len(got.Items) == 2)
require.True(t, got.Score > 0)
require.True(t, strings.Contains(err.Error(), "timeout"))
```

当一个测试需要在单次执行中收集多个相互独立的断言失败时，可以使用 `testify/assert`。初始化失败、前置条件失败，或后续检查依赖当前结果时，仍然使用 `require` 立即终止当前测试。

直接使用 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf` 仅限于无法通过现有语义化断言清晰表达的自定义测试控制流、特殊诊断输出，或测试辅助工具不适合依赖 `testify` 的场景。保留此类用法时，应让原因在代码上下文中保持清晰。

## 6. 架构边界测试

Go import 分层边界由根 `.golangci.yml` 的 `depguard` 规则随 `make lint` 检查。架构检查脚本位于 `user-service/scripts/architecture/lint.sh`，fixture 自测位于 `user-service/scripts/architecture/lint-test.sh`，覆盖其余跨文件或非 Go import 约束：

- 禁止旧 RBAC baseline import。
- 检查 `go.work`、各 `go.mod` 的 `go` 版本和 GitHub Actions 的 Go toolchain 版本一致；`go.mod` 中存在 `toolchain` 行时也必须一致。
- 检查主 CI 只调用一次复用质量 workflow，且 lint/unit workflow 不直接监听重复 PR/push 事件、标准 lint 与普通单测命令各只出现一次。
- 检查 OpenAPI 和 Ent 生成物 drift。
- 检查 `openspec/specs/`、`openspec/changes/` 和 `docs/opsx/` 下 Markdown 是否保留默认英文模板内容。
- 检查 `common/` 与 `user-service/` 的 `mock_generate.go` 是否使用 `generate` build tag。
- 检查人工维护的正式 Go 文件是否新增 `*ForTest` 或 `testHook*` 等明确测试语义 symbol；`_test.go`、`common/testing`、Ent 和 OpenAPI 生成物不在该扫描范围内。
- 人工搜索和审查应默认排除 `user-service/internal/persistence/ent/` 下除 `schema/` 外的 Ent 生成物；生成物 drift 仍由 `make user-service-architecture-lint`、`make user-service-generate` 和 `make verify` 暴露。

运行：

```bash
make user-service-architecture-lint
```

## 7. OpenAPI drift

API 注解、路由、request、response 或共享 OpenAPI helper 变化后，执行：

```bash
make user-service-openapi-generate
git diff -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml
```

若生成物有变化，应随代码一起提交。

## 8. Ent 和 migration 验证

Ent schema 变化后执行：

```bash
make user-service-generate
make user-service-migrate-diff name=<migration-name>
make user-service-migrate-validate
```

Ent 生成物不是人工修改入口。审查重点应放在 `user-service/internal/persistence/ent/schema/`、SQL migration 和 `atlas.sum`，并通过生成命令确认 `user-service/internal/persistence/ent/` 生成输出可复现。

进入环境或发布流程前，确认 SQL migration 和 `atlas.sum` 已提交到 Git，并通过 DBA 工单或受控发布平台执行。`users.nickname` substring 模糊查询必须由 `pg_trgm` 提供的 GIN `gin_trgm_ops` 索引支撑，不得保留普通索引、无扩展 fallback 或双索引兼容分支；测试记录必须说明目标库创建 `pg_trgm` 是否需要 DBA 权限或前置动作。

部署资产变更还应检查 Compose、Kubernetes 和 Helm 渲染结果不包含自动执行 `atlas migrate apply` 的 Job、service、command 或 args；普通 user-service 运行时镜像应确认不包含 `/usr/local/bin/atlas`。

Distroless 运行时镜像变更还应执行：

```bash
docker buildx build -f deployments/docker/user-service.Dockerfile -t aegiscore-user-service:latest --load .
IMAGE=aegiscore-user-service:latest make user-service-image-verify
```

该验证检查静态链接、UID/GID `65532`、CA certificates、`Asia/Shanghai` timezone、`/tmp`、CLI help，以及 shell、`apk`、`wget`、`curl`、`grep` 和 Atlas 均不存在。

## 9. 观测资产验证

通用 Grafana dashboard 变化后执行：

```bash
make compose-dashboard-generate
make compose-dashboard-check
```

Prometheus alert 或 dashboard 变更需要同时检查 `deployments/observability/` 和 `deployments/compose/` 中的对应资产。

## 10. OPSX 文档和规格验证

变更 OPSX 文档或 OpenSpec specs 后执行：

```bash
openspec list --specs
openspec validate --specs
make user-service-architecture-lint
```

主规格应包含 `Requirement` 和 `Scenario`，并覆盖主流程、异常流程或边界条件。
