## Context

仓库已有明确规则：正式代码不得暴露仅为测试服务的构造函数、hook、flag、可替换入口或兼容分支，测试应优先使用依赖注入、生成 mock、局部 fixture 和可观察状态。历史 change 已移除全局 CLI stub、localcache `setNowForTest` 和冗余 watcher constructor，但本次扫描仍确认以下残留：

| 类别 | 位置 | 识别依据 | 处理方向 |
|---|---|---|---|
| 测试生成入口参与正常编译 | `common/http/middleware/mock_generate.go`、`common/security/casbin/mock_generate.go`；`user-service/cmd/mock_generate.go`；auth 下 6 个、permission 下 7 个、role 下 4 个、user 下 3 个 `mock_generate.go`，实施前共 23 个 | 文件只包含 package clause 和 `go:generate mockgen`，无运行时职责；实施前无 build tag，因此进入正常 package `GoFiles` | 保留生成文件位置和指令，增加 `//go:build generate`，从正常构建排除；auth 根包为 Fx wiring 测试新增 1 个同样隔离的本地生成入口 |
| CLI 测试默认补齐 | `user-service/cmd/main.go` 的 `rootCommandDependencies.withDefaults` | 生产 `main` 已传完整依赖；所有部分 struct 调用都来自测试；逐字段 nil fallback 隐藏测试缺失依赖 | 删除自动补齐，测试统一通过完整 dependency fixture 构造后仅覆盖目标字段 |
| serve nil factory 兜底 | `user-service/cmd/main.go` 的 `runServe` | 生产 command 总是传 `appFactory`，测试也可显式传入；nil 分支不对应 CLI 行为 | 删除 nil fallback，保留 `lifecycleAppFactory` 依赖注入和生命周期测试 |
| 仅测试使用的共享导出构造器 | `common/runtime/workerpool.NewUnmanaged` | 仓库唯一直接消费者是 `pool_test.go`；主规格约束 `Pool.Stop` 和 Fx 生命周期，不要求公开 unmanaged constructor | 改为包内 `newUnmanaged`，`New` 继续注册 Fx hook，测试 fixture 在同包调用包内构造器 |
| 仅测试使用的 feature 导出构造器 | `user-service/internal/features/permission/infrastructure/redis.NewStoreWithInstance` | 唯一消费者是同包 watcher 测试，用于固定 payload 的 instance ID；生产只使用 `NewStore` | 删除导出入口，由同包测试使用 `newStore` 与 `MustKeyCatalog` 组合 |
| 宽接口测试替身 | `user-service/internal/features/auth/application/credentials/verifier_test.go` 的 `stubPasswordService`；`user-service/internal/features/auth/fx_test.go` 的 `fakeTokenVersionStore`、`fakeAuthStore` | 替身为失败注入或 Fx 构造实现无关方法，存在 `not implemented`/`unexpected` 分支，未直接表达端口 expectation | 为消费包补充或复用生成 mock，测试 fixture 明确期望和返回值；保留真实 `password.Service` 覆盖 Argon2id 行为 |

以下命中具有真实运行时或稳定规格语义，不进入清理：

- `common/http/middleware` 与 permission HTTP 的 Casbin 白名单、wildcard method 和 `OPTIONS` 旁路已由 `rbac-access-control` 主规格定义。
- refresh token 接受裸 token 或 Bearer 值属于当前输入契约；认证 subject 隔离、token version fallback 和撤销失败关闭均为安全边界。
- request ID、metrics unmatched route、logger `Sync`、Redis/PostgreSQL health、pprof handler 和 session purge TTL 等 fallback 处理真实运行时异常或运维场景。
- `common/http/response` 的 `ValidationFailed`、`Conflict`、`NotFound` 等 wrapper 已由 `shared-platform-primitives` 主规格定义，即使当前业务 controller 多使用 `response.Fail` 也不能按“单仓库无调用”删除。
- `entObservabilityDriver.now` 是实例内 SQL latency 观测依赖，不是全局 test hook；测试对其局部替换不改变正式 API，当前保留。
- datastore 协议模拟器、scheduler 并发记录器、OTel exporter、同步 purge executor 和只读 metrics stats fake 用于验证协议、并发或真实轻量 adapter 行为，不机械替换为 gomock。
- `testLifecycleApp`、测试 claims/envelope、`testAuthUseCases` 等只在 `_test.go` 中组合真实生命周期接口、协议载荷或已生成 mock，不进入正式构建，也不替代生产端口 expectation。

## Goals / Non-Goals

**Goals:**

- 从正常构建和公开 API 中移除已确认的测试适配残留，不改变外部业务行为。
- 让 CLI 测试显式声明完整依赖，避免测试因隐式生产默认值而误执行真实 runner。
- 让关键端口测试通过生成 mock 或窄 fixture 直接表达调用、失败和顺序。
- 为 `mock_generate.go` 和明显 test-only 正式代码建立可重复门禁。
- 对所有保留边界给出可追溯理由，避免关键词扫描误删容错、安全或协议行为。

**Non-Goals:**

- 不删除或修改 Casbin 白名单、`OPTIONS` 旁路、Bearer refresh 输入、response wrapper 或任何已进入主规格的兼容/边界行为。
- 不改变 HTTP API、OpenAPI、CLI command/flag/env、数据库 schema、Atlas migration、Redis key/payload、Casbin policy、metrics、日志字段或部署资产。
- 不把所有手写 fake/stub 一律替换为 gomock；只有生成 mock 能更清晰表达端口契约时才替换。
- 不新增跨 feature testing facade、全局 clock、共享断言 wrapper、测试配置开关或 `ForTest` API。
- 不手写 Ent 或 OpenAPI 生成物。

## Decisions

### Decision: 使用四重证据判定测试适配债务

候选代码只有同时满足以下条件才删除或收紧：

1. 生产调用图中没有真实消费者，或调用只为转调同一实现而存在。
2. 当前主规格和架构文档没有要求该 API、分支或兼容行为。
3. Git 历史、命名、注释或使用方式表明其主要用于测试便利性或生成测试资产。
4. `_test.go` fixture、显式依赖注入、生成 mock、真实轻量 adapter 或可观察状态可以覆盖原测试目标。

仅命中关键词或仅在当前服务无调用都不足以删除共享 API。备选方案是依据 `rg` 结果批量删除，但会误伤公共契约和安全边界，因此不采用。

### Decision: `mock_generate.go` 使用 `generate` build tag

实施前的 23 个文件继续保留在消费 mock 的 package 内，以符合现有 `delivery-operations` 生成路径要求；文件头增加 `//go:build generate`。auth 根包因 Fx wiring 测试需要消费本包未导出构造逻辑，新增 1 个 generate-only 入口生成最小 application port mock。Go generate 会启用 `generate` build tag，`make common-generate` 和 `make user-service-generate` 仍能执行指令，而普通 `go list`、`go build`、`go test` 的非测试包编译不会把生成入口计入 `GoFiles`。

备选方案一是把指令移动到 `_test.go`，但 package 模式下的 `go generate ./...` 不应依赖测试文件扫描语义，且生成文件可能覆盖承载指令的文件。备选方案二是集中到仓库脚本，会破坏“mock 生成入口归消费包所有”的现有约束。因此选择 generate-only 源文件。

### Decision: 保留显式依赖注入，删除隐式默认补齐

`rootCommandDependencies`、runner function type 和 `lifecycleAppFactory` 是替代进程级可变 stub 的正确依赖注入边界，应保留。`defaultRootCommandDependencies` 只由生产入口创建完整依赖；测试新增 `testRootCommandDependencies`，先提供全部 inert dependency，再按测试目标覆盖一个 runner。`newRootCommand` 和 `runServe` 不再修复 nil 依赖。

备选方案是恢复包级可变函数或让每个测试手写完整 struct；前者重新引入全局状态和并发污染，后者重复且易漏。完整 fixture 在 `_test.go` 内集中维护更符合边界。

### Decision: 仅测试消费的构造器降为包内实现

`workerpool.NewUnmanaged` 改为 `newUnmanaged`，`New` 仍是共享 runtime 的正式入口并保持 Fx hook 可选注册、`Stop` drain 和错误语义。permission Redis 测试直接调用已有 `newStore(client, keys, instanceID, log)`，删除 `NewStoreWithInstance` 包装层。二者均不提供 deprecated alias，因为仓库调用图无生产消费者，保留 alias 只会延续无价值 API。

如果未来出现真实手动生命周期 consumer，应以独立 change 说明 owner、关闭责任和稳定 API，再决定是否公开构造器，而不是以测试用例作为公开理由。

### Decision: 生成 mock 只替换隐藏端口契约的宽接口 fake

`PasswordService` mock 用于验证 dummy hash 调用、KDF busy 和异常传播；auth Fx 测试用 application port mock 或更窄的状态 fixture 表达读取/回填 expectation。生成指令仍归消费测试 package 所有。具有协议模拟、并发协调、同步执行或真实算法覆盖价值的 double 保留，并在实现审计中记录理由。

### Decision: 门禁检查结构特征而非猜测业务语义

架构/交付脚本至少验证：所有 `mock_generate.go` 含 `//go:build generate`；正常 package `GoFiles` 不包含这些文件；正式 Go 文件不新增 `ForTest`、`set*ForTest`、`testHook` 等显式测试语义 API。对“仅测试引用导出符号”仍保留人工复核，因为自动规则无法判断跨仓库公共 API 和未来插件消费者。

## 实施复扫结论

- 实施后共有 24 个 `mock_generate.go`，包含 23 个既有入口和 1 个 auth Fx wiring 本地入口；全部使用 `//go:build generate`，普通 package `GoFiles` 不包含这些文件。
- `rootCommandDependencies.withDefaults`、nil `appFactory` fallback、`NewUnmanaged`、`NewStoreWithInstance`、`stubPasswordService`、`fakeTokenVersionStore`、`fakeAuthStore` 和 `countingTokenVersionStore` 已无定义或引用，也未保留 alias。
- 正式 Go 文件中的测试专用注释已改写为 pprof debug server 复用、Ristretto 写入可见性和 stdout/stderr fsync 平台差异等纯运行时语义。
- 剩余 fallback/兼容命中分别处理 SQL operation 空语句、request ID 生成、未匹配 metrics route、默认 logger/timezone、Argon2id 格式校验、认证 session/token TTL、撤销缓存清理和已规格化 Bearer refresh 输入，均具有真实运行时或主规格依据。
- datastore 协议模拟器、scheduler 并发记录器、OTel exporter、同步 purge executor、只读 stats source、测试 claims/envelope 和生命周期 fixture 继续只存在于 `_test.go` 或 `common/testing`，不进入正式构建。
- 全量生成未产生 Ent、Atlas、OpenAPI 或部署资产变化。

## Risks / Trade-offs

- [Risk] `generate` build tag 配置错误导致 mock 无法刷新 -> Mitigation：分别运行 `make common-generate`、`make user-service-generate`，删除或改动一个生成物后验证可重建，并检查最终 drift。
- [Risk] CLI 完整 fixture 的 inert runner 被意外调用却返回成功 -> Mitigation：默认 runner 使用会立即返回明确错误或触发测试失败的函数，目标测试必须显式覆盖会执行的依赖。
- [Risk] 公共导出 API 的仓库外消费者不可见 -> Mitigation：本仓库没有发布兼容承诺或外部模块引用证据，且两个目标 API 注释明确包含测试用途；在 release note/变更摘要中标明 Go API 收紧，不提供无消费者 alias。
- [Risk] mock 过度使用降低并发和协议测试可读性 -> Mitigation：只替换宽接口、失败注入和调用顺序场景；保留通道、atomic、协议 server、真实 service 和同步 executor fixture。
- [Risk] 关键词门禁产生误报 -> Mitigation：自动门禁只覆盖明确命名和 build tag 结构，运行时 fallback、compatibility 和 hook 仍依赖人工规格复核。
- [Risk] 清理时误改认证或 RBAC 安全路径 -> Mitigation：对 auth、permission、role、router、cmd、workerpool 运行聚焦测试并执行 `make test`、`make user-service-architecture-lint`、`make lint`、`make verify`。

## Migration Plan

1. 记录当前 `mock_generate.go` 列表、生产/测试引用和需保留边界，作为实施基线。
2. 为实施前的 23 个生成入口增加 `generate` build tag，并为 auth Fx wiring 测试新增 1 个 generate-only 本地入口，先验证生成命令和 mock drift。
3. 引入 CLI 完整测试 fixture，再删除 `withDefaults` 与 nil factory 分支，运行 cmd 聚焦测试。
4. 将 `NewUnmanaged`、`NewStoreWithInstance` 降为包内实现并更新同包测试。
5. 替换确认的宽接口 auth fake/stub，重新生成 mock，并逐项检查 expectation 是否覆盖原错误和安全路径。
6. 更新架构 lint、测试文档和对应 fixture，运行 OpenSpec、相关 package、生成、lint 和全量 verify。

本 change 不需要数据库、部署或数据迁移。回滚时可整体恢复对应生产和测试提交；不得只恢复旧测试适配生产 API 而保留依赖它的新测试。

## Open Questions

- 实施扫描若发现新的高置信候选，只有满足四重证据且不扩大 capability 范围时才纳入；否则记录为后续 change，不在本次顺带删除。
- `entObservabilityDriver.now` 当前明确保留；若后续需要统一 clock abstraction，应由独立观测设计证明多个运行时消费者，而不是以测试便利性推动共享抽象。
