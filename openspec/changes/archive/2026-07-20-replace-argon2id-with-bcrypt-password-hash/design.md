## Context

当前 `common/security/password` 提供 Argon2id 密码 KDF 原语，并要求调用方通过 `password.Options` 提供并发和队列上限。user-service 在 `auth.password_kdf.argon2_concurrency` 和 `auth.password_kdf.argon2_queue_size` 中暴露这些预算，认证登录、dummy verification、用户创建、强制改密和 RBAC 超级管理员创建都经由该服务生成或验证 `users.password_hash`。

本次变更是跨 `common` 与 `user-service` 的破坏性安全变更。最终策略已经确定为直接切换到 `golang.org/x/crypto/bcrypt`，不保留旧 Argon2id 验证、兼容、迁移或 fallback。既有 Argon2id 哈希在发布后会被视为无效哈希或无效凭据。

## Goals / Non-Goals

**Goals:**

- 使用 `golang.org/x/crypto/bcrypt` 作为唯一密码哈希算法，固定 bcrypt cost 初始值为 `12`。
- 删除 Argon2id 相关实现、配置、测试、dummy hash 和 OpenSpec 语义。
- 删除 password KDF 资源繁忙错误和认证流程中的 `password_kdf_busy` 特殊分支。
- 保持 `HashContext` 和 `VerifyContext` 对上层 application 的最小接口，避免认证、用户和 RBAC 业务逻辑扩散修改。
- 保持 `users.password_hash` 字段不变，不引入 Ent schema 或 Atlas migration。

**Non-Goals:**

- 不验证旧 Argon2id hash。
- 不实现旧 hash 的在线迁移、批量迁移、登录后 rehash 或双算法兼容。
- 不新增 `auth.password_hash`、`bcrypt_cost`、并发或队列配置。
- 不新增 bcrypt 运行时资源指标或部署限流机制。
- 不改变 HTTP 路径、OpenAPI 请求响应结构、JWT、refresh session、token version 或 RBAC 授权语义。

## Decisions

### 使用固定 bcrypt cost

决策：`common/security/password` 内部定义固定 `defaultBcryptCost = 12`，`NewService` 不从 user-service 配置读取 cost。

理由：当前需求明确不需要配置块，固定 cost 能减少配置面、部署差异和错误校验分支。`golang.org/x/crypto v0.54.0` 已在 `common/go.mod` 中直接依赖，无需新增 module。

备选方案：暴露 `auth.password_hash.bcrypt_cost`。该方案被放弃，因为会增加服务私有配置、测试矩阵和运维决策点，且当前没有多环境调优需求。

### 删除 Argon2id 资源门控和 busy 契约

决策：删除 `Concurrency`、`QueueSize`、Argon2 queue/gate、`ErrPasswordKDFBusy`、`password_kdf_busy` 和认证 `503` 特殊分支。

理由：这些语义来自 Argon2id 高内存 KDF 预算模型。切换到 bcrypt 后继续保留会形成误导性配置和错误契约。bcrypt 运算失败不应被表达为 Argon2/KDF 队列繁忙。

备选方案：保留通用 password hash 并发门控。该方案被放弃，因为最终方案已要求不需要配置，且当前没有观测或运维契约要求保留该运行时资源池。

### 保留 password service 的调用接口

决策：保留 `HashContext(ctx, plain)` 和 `VerifyContext(ctx, plain, encodedHash)` 方法签名；`ctx` 在 bcrypt 调用前检查取消状态，但 bcrypt 库调用本身不可中断。

理由：auth、user 和 RBAC 调用方已经依赖该最小接口，保留接口能把算法替换限制在 `common/security/password` 和组合层。上下文参数仍能在进入 CPU 密集操作前 fail fast。

备选方案：改为无 context 的 `Hash`/`Verify`。该方案会扩大调用方改动，且对本次算法替换没有必要。

### 明文密码长度限制收敛到 bcrypt 上限

决策：将明文密码最大长度限制为 bcrypt 安全输入上限 72 字节，并继续通过 `ErrPasswordTooLong` 表达。

理由：bcrypt 对超过 72 字节的密码存在安全语义边界。显式拒绝可避免用户输入被截断或底层库错误泄露。

备选方案：保留当前 256 字节限制并依赖 bcrypt 返回错误。该方案错误边界不如显式校验清晰。

### 不修改数据库字段

决策：保留 `users.password_hash` 字段和 Ent schema，不生成 Atlas migration。

理由：bcrypt hash 约 60 字符，当前 `password_hash` 为非空字符串且 SQL 类型是 `character varying NOT NULL`，字段容量和语义均满足需求。

备选方案：增加算法字段或 hash version 字段。该方案服务于兼容和迁移，但本次明确不保留旧算法，因此不需要。

## Risks / Trade-offs

- 旧用户无法登录 → 这是预期破坏性行为；发布前通过业务侧重置密码、重新创建账号或重建环境数据处理，不在代码中实现迁移。
- bcrypt cost 固定后无法按环境调优 → 当前以安全策略一致性优先；未来如确有生产调优需求，应通过独立 OpenSpec change 引入配置。
- 删除 `password_kdf_busy` 会改变登录失败 metrics 分类和 `503` 分支 → 同步删除相关 metrics reason 和测试，认证失败仍保持统一无效凭据或内部错误边界。
- bcrypt 运算无法响应执行中的 context cancel → 在调用前检查 `ctx.Err()`，不承诺执行中取消。
- 旧配置文件包含 `auth.password_kdf` 将启动失败 → 这是严格删除旧配置的预期结果；同步更新默认配置、测试配置和部署示例。

## Migration Plan

1. 先合并 OpenSpec change，明确旧 Argon2id hash 和 `auth.password_kdf` 配置不再被接受。
2. 实现 `common/security/password` bcrypt 替换，删除 Argon2id parser、KDF、资源门控和 busy 错误。
3. 更新 user-service 配置结构、默认配置、provider 和 RBAC CLI 依赖构造。
4. 更新 auth dummy bcrypt hash、登录错误分类和相关 metrics/test。
5. 运行相关 Go 测试和 `make user-service-architecture-lint`。
6. 发布前清理环境中的旧 `auth.password_kdf` 配置；对已有用户执行业务侧密码重置、账号重建或环境数据重建。

回滚策略：若新版本发布失败，可回滚到上一代码版本和上一配置；回滚不会恢复已由新版本写入的 bcrypt hash 对旧版本 Argon2id 验证的兼容性。因此生产回滚前必须确认是否已有 bcrypt hash 写入，必要时继续执行密码重置或账号重建流程。

## Verification

- `go test ./...` in `common/`
- `go test ./...` in `user-service/`
- `make user-service-architecture-lint`
- 最终合并前运行 `make lint` 和 `make verify`

## Open Questions

无。
