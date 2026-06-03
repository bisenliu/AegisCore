## MODIFIED Requirements

### Requirement: Load authentication configuration

系统 MUST 从 YAML 配置和 `AEGISCORE_` 环境变量覆盖中加载 `auth` 配置到共享配置对象。认证配置 MUST 支持 JWT secret、issuer、audience 和 token 过期配置。配置加载器 MUST 只负责读取、覆盖和反序列化这些字段，不得在 `common/config.Load` 阶段执行 required、字段存在性或基础取值范围校验。认证配置 MUST NOT 包含 `auth.whitelist` 字段，服务 MUST NOT 通过共享认证配置声明公开路径或认证豁免路径。

#### Scenario: Load auth config from YAML
- **Given** YAML 配置包含 `auth.jwt.secret`、`auth.jwt.issuer` 和 `auth.jwt.audience`
- **When** `common/config.Load` 反序列化配置
- **Then** 系统 MUST 将这些字段反序列化到 `config.Config` 的认证配置中
- **Then** 认证配置对象 MUST NOT 暴露白名单路径集合

#### Scenario: Override auth config with environment variable
- **Given** YAML 配置包含 auth 配置
- **When** 环境变量通过 `AEGISCORE_` 前缀覆盖 auth JWT 或 token 会话相关配置
- **Then** 系统 MUST 使用环境变量覆盖后的 auth 配置值
- **Then** 系统 MUST NOT 支持通过环境变量配置认证白名单路径

#### Scenario: Missing auth config is not rejected by config loader
- **Given** YAML 和环境变量未显式提供 auth 配置
- **When** `common/config.Load` 反序列化配置
- **Then** 配置加载 MUST 成功反序列化配置对象
- **Then** 配置加载器 MUST NOT 因 auth 字段缺失、为空或零值而返回校验错误

#### Scenario: Auth config does not create infrastructure clients
- **Given** 配置中存在 auth 配置
- **When** `common/config.Load` 反序列化配置
- **Then** 系统 MUST NOT 因 auth 配置存在而创建 Redis client、PostgreSQL 连接池、Ent client 或 HTTP server

#### Scenario: Whitelist config is not part of the contract
- **Given** 用户服务示例配置和共享配置结构
- **When** 调用方查看认证配置契约
- **Then** `auth.whitelist` MUST NOT 出现在示例 YAML 中
- **Then** `config.AuthConfig` MUST NOT 包含白名单字段
