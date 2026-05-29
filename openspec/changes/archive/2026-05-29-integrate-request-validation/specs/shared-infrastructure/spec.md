## ADDED Requirements

### Requirement: Provide reusable validation dependency

系统必须允许服务通过 Fx 获取共享请求校验器依赖，并由需要 HTTP 请求校验的服务显式引入该依赖。

#### Scenario: Provide validation module
- **Given** 服务 Fx app 引入共享 validation module
- **When** Fx 解析依赖
- **Then** 系统必须提供共享请求校验器实例
- **Then** controller 必须能够注入该实例并用于请求绑定和校验

#### Scenario: Do not validate runtime config in config loader
- **Given** `common/config.Load` 加载 YAML 和 `AEGISCORE_` 环境变量
- **When** 共享请求校验能力被引入服务
- **Then** `common/config.Load` 仍不得执行 required、optional、字段存在性或基础取值范围校验
- **Then** 请求 DTO 校验与运行时配置加载必须保持职责分离

#### Scenario: Do not connect extra runtime dependencies
- **Given** 服务引入共享 validation module
- **When** Fx app 启动
- **Then** validation module 不得创建 Redis client、PostgreSQL 连接池、Ent client 或 HTTP server
- **Then** validation module 初始化失败时必须返回错误并阻止服务以不完整状态启动
