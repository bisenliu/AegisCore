## Context

`common` 当前承载跨服务契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力，但实际代码中已经包含 user-service 私有语义：`common/runtime/config.Config` 包含 `Auth` 与 `Ent`，`common/runtime/resources` 包含 `NameUserDB`，`common/security/auth.JWTService` 同时提供 access、refresh、password-change token 的签发和解析，并固定 `user_id`、`token_version`、`session_id` claims。

这些实现让共享层既是 verifier 又是 issuer，也让通用配置 loader 强制理解用户认证会话策略。后续新增服务复用 `common` 时，会被迫继承 user-service 认证模型，或者在只需要验签时获得签发 token 的能力，违反最小权限原则。

本变更是跨 `common` 与 `user-service` 的破坏性边界收敛，不保留旧 API、旧 alias 或旧 constructor。目标是在编译期暴露所有误用，避免兼容层继续扩大共享层安全面。

## Goals / Non-Goals

**Goals:**

- `common/runtime/config` 仅提供通用 runtime config 和 loader primitive，不再声明或校验 user-service auth/ent 配置。
- `common/runtime/resources` 不再声明用户数据库资源名，服务资源名由服务私有包拥有。
- `common/security/auth` 只提供通用 JWT 验签、issuer/audience 校验和 token 字符串 helper，不提供签发 API 和 user-service claims。
- `user-service` 私有拥有认证配置、JWT issuer、access/refresh/password-change subject、用户会话 claims、TTL fallback 和认证策略校验。
- 共享 HTTP auth middleware 依赖最小 verifier 接口，避免依赖具备签发能力的 concrete type。
- 保持 HTTP API、OpenAPI、数据库 schema 和部署配置字段名不变，只改变 Go 代码归属与边界。

**Non-Goals:**

- 不重新设计登录、refresh、退出、强制改密、token version 或 RBAC 授权业务流程。
- 不引入新加密算法、KMS、JWKS、非 HMAC token 或多 issuer 支持。
- 不改数据库 schema，不新增 Atlas migration。
- 不改变外部 HTTP 契约和 OpenAPI 文档内容。
- 不保留旧 API 的兼容 alias、deprecated wrapper 或资源名转发。

## Decisions

### 1. user-service 定义私有根配置

决策：新增 user-service 私有配置包，定义服务根配置并嵌入或组合 `common/runtime/config` 的通用配置块；`AuthConfig`、`JWTConfig`、`PasswordKDFConfig`、`EntConfig` 和认证策略校验全部下沉到该包。

理由：配置字段是否必需、TTL 关系、password KDF 预算、Ent SQL debug 都是 user-service 当前运行策略，不属于跨服务 runtime primitive。

备选方案：继续在 `common` 保留 `AuthConfig`，仅移动 issuer。该方案会继续让所有服务共享 auth 配置结构，无法解决 loader 和校验层的边界问题，因此不采用。

### 2. common loader 提供泛型/目标结构加载能力

决策：将 `common/runtime/config.Load` 的核心能力抽象为加载任意目标结构的 primitive，并让 common 自身只校验通用字段；user-service 通过私有 `Load` 调用共享 loader 并执行服务私有 `Validate`。

理由：保留配置文件读取、环境变量覆盖、duration decode 等跨服务通用能力，同时让每个服务拥有自己的 schema 与校验规则。

备选方案：为每个服务复制 loader。该方案会增加重复实现并削弱跨服务一致性，因此不采用。

### 3. JWT 签发只存在于 user-service auth feature

决策：`common/security/auth` 删除 `SignAccessToken`、`SignRefreshToken`、`SignPasswordChangeToken`、`SignInput` 和 user-service 专属 `Claims`；user-service auth token 包定义 issuer、claims、subject 和 TTL fallback。

理由：签发能力是高敏感服务私有能力，不应由共享包暴露给所有依赖者。refresh/password-change 是认证会话业务语义，也应跟随 auth feature。

备选方案：把签发方法拆成 common `Issuer` 接口但继续放在 shared package。该方案仍会给共享层定义签发模型和业务 subject，因此不采用。

### 4. common 只提供 verifier primitive

决策：`common/security/auth` 保留 HMAC JWT 验签、算法限制、expiration required、可选 issuer/audience 校验、Bearer 前缀处理等通用能力。调用方传入自己的 claims 结构并自行校验 subject 与业务字段。

理由：验签是跨服务可复用安全 primitive，但 claims 内容和 subject 语义由服务拥有。

备选方案：common 提供统一 `Claims` 并用 map 扩展。该方案会把 claims schema 继续中心化到共享层，容易再次吸收业务字段，因此不采用。

### 5. 共享 middleware 依赖最小 verifier 接口

决策：共享 HTTP auth middleware 不接收 concrete `JWTService`，而接收只包含 access token 验证能力的接口；user-service 提供适配器完成 token 解析、subject 检查、token version/session 字段校验和上下文注入。

理由：middleware 只需要验证请求身份，不应获得签发能力。接口化后 shared middleware 可以复用，但敏感认证语义留在服务内。

备选方案：将整个 auth middleware 下沉到 user-service。该方案安全边界最强，但会丢失 HTTP helper 复用；当前只需通过最小接口即可达到最小权限，因此不采用。

### 6. 服务资源名全部服务私有

决策：`NameUserDB` 从 `common/runtime/resources` 删除并移动到 user-service 私有资源名包。user-service 的 Fx provider、RBAC CLI 和测试使用私有常量。

理由：`user_db` 是 user-service 拥有的数据库实例命名，通用 datastore 只应接受调用方传入的名字，不应声明具体业务资源。

备选方案：在 common 中维护所有服务资源名。该方案会让 common 成为服务目录，违背 primitive 边界，因此不采用。

## Risks / Trade-offs

- [Risk] 破坏性 API 删除会导致大量编译错误。→ Mitigation：按 common primitive、user-service config、issuer/middleware、资源名、测试的顺序迁移，使用 `go test` 和 architecture lint 逐步收敛。
- [Risk] loader 泛型化后环境变量覆盖行为可能回归。→ Mitigation：保留原 viper 设置、key replacer、duration decode，并迁移现有 loader 测试覆盖 auth 字段下沉后的 user-service loader。
- [Risk] JWT claims 迁移可能影响 access/refresh/password-change token 兼容。→ Mitigation：不改变 token 字段名、subject 字符串、issuer/audience 配置值和 HMAC 算法，只改变代码归属。
- [Risk] middleware 接口变化可能影响 token version 校验链路。→ Mitigation：user-service 适配器保持现有 claims 到 auth context 的映射，并运行 auth、router、provider 测试。
- [Risk] 删除 `NameUserDB` 后资源名散落。→ Mitigation：统一放入 user-service 私有资源名包，禁止在 common 测试中引用 user-service 资源名。

## Migration Plan

1. 新增 user-service 私有配置与资源名包，迁移 auth/ent 配置类型和校验逻辑。
2. 调整 common loader/config，删除 auth/ent 字段和校验，保留通用配置读取能力。
3. 调整 user-service bootstrap、providers、RBAC CLI 和测试，全部依赖服务私有配置。
4. 收敛 `common/security/auth` 为 verifier-only，并将 issuer、claims、subject 下沉到 user-service auth token 包。
5. 调整共享 HTTP middleware 的最小 verifier 接口和 user-service 适配器。
6. 删除 `common/runtime/resources.NameUserDB` 并迁移所有引用。
7. 运行相关 Go 测试、`make user-service-architecture-lint`、`make lint` 和 `make verify`。

回滚策略：该变更不涉及数据库和外部 HTTP 契约，回滚为代码级回滚。若发布后发现认证失败，可回滚应用镜像到变更前版本；配置字段名保持不变，不需要配置回滚或数据迁移。

## Open Questions

无。当前方案明确采用不保留兼容层的破坏性边界收敛。
