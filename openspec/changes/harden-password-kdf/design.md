## Context

`common/security/password` 是 `common-credentials` capability 的共享密码凭证原语，目前负责 Argon2id hash 生成、encoded hash 解析和 constant-time 校验。当前实现缺少 `context.Context` 取消入口，也没有限制同一进程内 Argon2id KDF 的并发数量和等待队列长度；在 Argon2id `m=65536` 参数下，突发注册或登录请求会直接转化为高内存 KDF 并发。

该变更只影响 `common/security/password` 包，不进入 `user-services` controller/service/infra 分层，不新增配置读取、Fx provider、Redis/PostgreSQL 连接、HTTP route 或响应契约。

## Goals / Non-Goals

**Goals:**

- 删除旧的 `Hash` 和 `Verify` API，统一使用 `HashContext` 和 `VerifyContext`，避免调用方绕过取消语义。
- 让所有密码 hash 和校验调用都在等待队列或执行槽位时尊重 context 取消。
- 在包内限制 Argon2id 执行并发和总排队数量，队列满时快速返回 `ErrPasswordKDFBusy`。
- 对明文密码、encoded hash、Argon2id 参数、salt 长度和 key 长度做明确边界校验。
- 保持 hash 输出格式、默认参数和既有错误语义兼容，但不保持旧 Go API 兼容。

**Non-Goals:**

- 不新增运行时配置项或动态调参能力。
- 不修改用户服务认证业务语义、HTTP API、错误码映射或 Swagger 文档；只在编译层面迁移 Go 调用点。
- 不引入外部限流组件、Redis 分布式队列或 worker pool 依赖。
- 不改变已有数据库 schema、migration 或 Ent 生成代码。

## Decisions

- 使用包级 buffered channel 实现 KDF gate 和 queue。理由：密码包应保持 side-effect free 且无外部依赖，channel 可用最少代码表达单进程并发和排队容量。备选方案是引入 worker pool 或 `x/sync/semaphore`；前者增加生命周期管理，后者增加依赖且收益有限。
- 删除 `Hash`/`Verify`，只保留 `HashContext`/`VerifyContext`。理由：密码 KDF 是高成本安全边界，统一强制 context-aware API 可以避免新增调用继续使用不可取消路径。备选方案是保留兼容 wrapper；这会降低迁移压力，但会留下绕过取消和排队治理语义的旧入口。
- 在进入 Argon2id 前完成输入校验和 hash 参数策略校验。理由：不可信 encoded hash 和超长明文密码不应消耗高成本 KDF 资源。备选方案是接受 encoded hash 自带参数；这会允许攻击者提交更高成本参数放大资源消耗。
- 固定默认 KDF 并发为 2、总队列为 16。理由：按当前 `64MiB` 内存参数，默认并发 2 可将 Argon2 工作内存控制在约 `128MiB`，队列限制避免 goroutine 无界堆积。备选方案是完全串行；资源更保守但吞吐过低。
- 不拆分 `password.go`，除非实现后文件明显变得难以维护。理由：该包仍只有一个聚合职责，拆分为 `hash.go`、`parse.go`、`gate.go` 会增加导航成本；测试可通过函数和用例分组表达边界。

## Risks / Trade-offs

- 删除 `Hash`/`Verify` 会造成编译期破坏性变更 → 实现阶段必须同步迁移仓库内所有调用点，并通过测试确保无旧 API 引用。
- KDF queue 满会让上层更早收到 `ErrPasswordKDFBusy` → 调用方需要按现有错误映射策略决定是否返回可重试错误；本变更只定义 password 包错误语义。
- 包级 gate 只限制单进程内并发 → 多副本部署仍需要网关、服务层或基础设施限流；本变更不承诺分布式限流。
- 固定并发和队列大小可能不适合所有机器规格 → 当前先选择安全默认值，避免新增配置面；未来如有真实部署差异可另提变更增加配置化能力。
- `context.Context` 无法中断已经进入 `argon2.IDKey` 的 CPU/内存计算 → 设计只保证等待队列和等待槽位期间可取消，执行中取消需等待当前 KDF 返回。

## Migration Plan

- 直接更新 `common/security/password/password.go`，删除 `Hash`/`Verify` 兼容入口，只提供 `HashContext`/`VerifyContext`。
- 迁移仓库内所有 `password.Hash` 和 `password.Verify` 调用点，传入已有 request/service context。
- 扩展 `common/security/password/password_test.go`，覆盖新增错误和边界行为。
- 在 `common/` 模块运行 `go test ./...` 验证共享包。
- 无数据库、Ent、Atlas、Redis、配置或 HTTP 部署迁移步骤；回滚方式是恢复旧 API 并回滚调用点迁移。

## Open Questions

- 无阻塞问题。默认并发和队列常量先作为包内策略固定，后续如出现部署侧调参需求再独立提案。
