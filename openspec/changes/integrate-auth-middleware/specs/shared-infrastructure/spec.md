## ADDED Requirements

### Requirement: Load authentication configuration

系统 MUST 从 YAML 配置和 `AEGISCORE_` 环境变量覆盖中加载 `auth` 配置到共享配置对象。认证配置 MUST 至少支持 JWT secret、issuer、audience、token 过期配置和认证白名单路径。配置加载器 MUST 只负责读取、覆盖和反序列化这些字段，不得在 `common/config.Load` 阶段执行 required、字段存在性或基础取值范围校验。

#### Scenario: Load auth config from YAML
- **Given** YAML 配置包含 `auth.jwt.secret`、`auth.jwt.issuer`、`auth.jwt.audience` 和 `auth.whitelist`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 将这些字段反序列化到 `config.Config` 的认证配置中
- **Then** 白名单路径 MUST 保持 YAML 中的顺序

#### Scenario: Override auth config with environment variable
- **Given** YAML 配置包含 auth 配置
- **Given** 环境变量提供 `AEGISCORE_AUTH_JWT_SECRET` 或 `AEGISCORE_AUTH_JWT_ISSUER`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 使用环境变量覆盖后的 auth 配置值

#### Scenario: Missing auth config is not rejected by config loader
- **Given** YAML 和环境变量未显式提供 auth 配置
- **When** `common/config.Load` 被调用
- **Then** 配置加载 MUST 成功反序列化配置对象
- **Then** 配置加载器 MUST NOT 因 auth 字段缺失、为空或零值而返回校验错误

#### Scenario: Auth config does not create datastore dependencies
- **Given** 配置中存在 auth 配置
- **When** `common/config.Load` 或共享基础设施 module 初始化
- **Then** 系统 MUST NOT 因 auth 配置存在而创建 Redis client、PostgreSQL 连接池、Ent client 或 HTTP server
