## Context

AegisCore 当前是 Go workspace，`common` 承载共享配置、基础设施、认证、响应和中间件，`user-services` 承载 CLI、Fx bootstrap、Gin 路由、业务 service/repository 和 Ent schema。常量天然横跨多个边界：配置 key 和资源名是外部契约，路由和错误码是 API 契约，DTO 校验和 Ent schema 是业务/数据契约，测试中的端口和 timeout 则只是场景数据。

这次审查发现最突出的混淆点是关闭超时：`cmd/main.go` 中 `stopTimeout = 15s` 限制整个 Fx app stop，而 `bootstrap/server.go` 中 `defaultShutdownTimeout = 10s` 只限制 HTTP server graceful shutdown，示例配置又声明 `http.shutdown_timeout = 25s`。当外层 stop context 只有 15s 时，HTTP 配置的 25s 无法完整生效，维护者也难以从命名判断二者关系。

## Goals / Non-Goals

**Goals:**

- 明确关闭超时的两层语义：CLI/Fx app lifecycle budget 与 HTTP server graceful shutdown budget。
- 让外层 Fx stop timeout 不小于示例配置中的 HTTP shutdown timeout，避免默认配置被外层 context 提前截断。
- 建立常量分层原则，避免项目后续把所有值塞进单一 `constants` 包。
- 收敛跨模块契约常量和明显重复默认值，同时保留测试数据、局部格式字符串和包内实现细节的就近定义。
- 保持 HTTP API、YAML key、环境变量、错误码、资源名、数据库 schema 和 Fx named injection 契约兼容。

**Non-Goals:**

- 不重新设计配置加载器，不在 `common/config.Load` 增加 required/range 校验。
- 不引入全局 `common/constants` 或 `user-services/constants` 聚合包。
- 不重命名服务目录、Go module path、CLI 名、路由前缀或数据库字段。
- 不修改 Ent 生成代码；若涉及 schema 常量，只改 `user-services/ent/schema/` 后运行生成。
- 不新增支付、授权、管理后台等当前没有代码基础的业务能力。

## Decisions

### Decision 1: 分离并显式命名两层关闭超时

将 CLI 的 `stopTimeout` 重命名为能表达 Fx app 停止预算的名称，例如 `appStopTimeout` 或 `fxStopTimeout`。HTTP server fallback 保持为 HTTP 语义名称，例如 `defaultHTTPShutdownTimeout`。外层默认停止预算必须大于或等于 `user-services/configs/config.yaml` 中的 `http.shutdown_timeout`，建议默认改为 `30s`，以覆盖当前示例配置 `25s` 并保留少量资源关闭余量。

替代方案是把 HTTP fallback 从 `10s` 改为 `15s` 或 `25s`，但这不能解决外层 15s 截断示例配置 25s 的问题。另一个方案是从配置中动态计算 CLI stop context，但 CLI 在 Fx app 启动前并不直接持有加载后的配置，强行读取会重复配置加载职责。

### Decision 2: 不做“一刀切”集中 constants 包

常量按能力和边界归属，而不是按“是否是常量”归属。跨模块可观察契约放在对应 common 包中，例如认证传输常量在 `common/auth`，资源名在 `common/infrastructure`，响应码在 `common/response`，trace header 在 `common/middleware`。用户服务业务常量放在 `user-services/internal/domain` 或就近 DTO/schema，运行时私有默认值放在 `cmd` 或 `bootstrap` 的聚焦文件内。

替代方案是创建全局 `constants` 包。该方案短期减少字面量搜索结果，但会制造高耦合依赖、模糊 capability 边界，并容易让业务常量、配置契约、测试数据和格式字符串混杂。

### Decision 3: 将重复默认值分为“契约重复”和“场景重复”

DTO 校验上限与 Ent schema 上限属于同一业务约束在 HTTP 边界和数据边界的重复表达，应通过同一 domain/schema 常量或测试保证一致。示例 YAML 与代码 fallback 的重复默认值必须要么保持一致，要么明确说明一个是部署示例、一个是缺失配置时的安全 fallback。测试端口、短 timeout、示例 URL 和 Swagger examples 属于场景数据，不强制集中。

### Decision 4: 资源名常量保持 common/infrastructure 边界

`user_db`、`common_db`、`cache_redis` 已在 `common/infrastructure/resource_names.go` 有公共常量，继续作为跨模块 datastore/Ent wiring 契约来源。Fx struct tag 无法直接引用常量，保留必要字面量并用测试或邻近注释降低漂移风险，不做大规模反射或 tag 生成重构。

### Decision 5: 认证和会话 TTL 默认值按 owner 收敛

Access/refresh token TTL fallback 属于 `user-services/internal/service/auth_service.go` 的认证业务默认值；Bearer、Authorization、JWT subject/claim 属于 `common/auth`；Redis session key 和 session TTL fallback 属于 Redis auth session repository。实现时优先统一命名和文档，只有当示例配置、DTO example 与 fallback 明显冲突时才调整示例或测试，避免改变运行时安全策略。

## Risks / Trade-offs

- 外层 stop timeout 增大可能让默认退出等待变长。缓解方式：仅增大默认预算到覆盖示例 HTTP shutdown 的合理值，不改变接收信号后立即开始停止的行为。
- 过度集中 DTO/schema 常量可能引入 Ent schema 到 service/domain 的反向依赖。缓解方式：只在无循环依赖的位置定义共享业务约束，或用测试验证一致性。
- 保留部分字面量可能看起来“不彻底”。缓解方式：用常量分层原则明确哪些值不集中，例如 Fx tag、Swagger example、测试场景数据和局部格式字符串。
- 修改默认值可能影响依赖超时测试。缓解方式：补充针对命名和预算关系的单元测试，并保持现有 public contract 不变。

## Migration Plan

1. 重命名 CLI/Fx lifecycle timeout 常量并调整默认 stop budget，使其不小于示例 HTTP shutdown timeout。
2. 重命名或保留 HTTP server fallback 常量，但确保名称体现 HTTP graceful shutdown，而不是整个服务停止。
3. 按分类整理明显跨模块常量：资源名、认证传输、响应码、trace-id、配置默认路径、业务阈值。
4. 对 DTO 与 Ent schema 的长度限制、用户默认状态、认证 TTL、session TTL 和示例配置进行一致性检查。
5. 运行 `go test ./...`，分别在 `common/` 和 `user-services/` 模块验证。
6. 如修改 Ent schema，运行 `go generate ./ent` 并按 Atlas workflow 生成/校验 migration；本变更预计不需要数据库结构变更。

## Open Questions

- 是否将示例配置中的 `auth.jwt.access_token_ttl: 2h` 调整为与 fallback/DTO example `15m` 一致，还是保留为本地开发示例并在注释或文档中说明差异。
- DTO 与 Ent schema 长度限制应抽到 `internal/domain` 常量，还是保留重复并增加测试防漂移，取决于实现时是否会产生不合理依赖。
