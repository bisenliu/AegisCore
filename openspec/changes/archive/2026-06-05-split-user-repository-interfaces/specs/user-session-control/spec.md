## ADDED Requirements

### Requirement: Authentication uses credential and token version repository interfaces

用户会话控制能力 SHALL 将认证凭证访问与 token version 访问拆分为独立仓储接口。认证凭证组件 MUST 依赖凭证仓储接口读取认证所需用户资料并更新凭证；认证会话组件 MUST 依赖 token version 仓储接口读取和原子递增用户 token version。认证服务和认证组件 MUST NOT 为登录、改密、刷新或退出全部设备流程依赖包含用户资料创建、用户列表查询和其他无关方法的完整用户仓储大接口。

#### Scenario: Credential component declares credential repository dependency
- **Given** 登录或修改密码流程需要读取用户认证资料或更新用户凭证
- **When** 认证凭证组件声明仓储依赖
- **Then** 认证凭证组件 MUST 依赖凭证仓储接口
- **Then** 该接口 MUST 覆盖按用户名读取认证资料和更新凭证所需方法
- **Then** 认证凭证组件 MUST NOT 依赖用户列表查询或 token version 原子递增能力

#### Scenario: Session component declares token version repository dependency
- **Given** 刷新、退出全部设备或 token version 校验流程需要读取或递增用户 token version
- **When** 认证会话组件声明仓储依赖
- **Then** 认证会话组件 MUST 依赖 token version 仓储接口
- **Then** 该接口 MUST 覆盖读取 token version 和原子递增 token version 所需方法
- **Then** 认证会话组件 MUST NOT 依赖用户资料创建、用户列表查询或凭证更新能力

#### Scenario: Auth service construction injects separated repository capabilities
- **Given** Fx 构造认证服务及其内部凭证、token 和会话组件
- **When** 依赖注入容器提供 PostgreSQL 用户仓储实现
- **Then** 同一个底层 PostgreSQL 用户仓储实例 MUST 能以凭证仓储接口和 token version 仓储接口身份注入
- **Then** Fx 装配 MUST NOT 为不同小接口重复创建多个语义独立的 PostgreSQL 用户仓储实例
- **Then** `AuthService` MUST 继续只保存认证凭证组件、认证 token 组件、认证会话组件和必要编排策略

#### Scenario: Authentication behavior remains compatible
- **Given** PostgreSQL 用户仓储实现通过小接口提供认证凭证和 token version 能力
- **When** 系统处理登录、修改密码、刷新 token、退出当前设备或退出全部设备请求
- **Then** token 签发、Refresh Token 会话、token version 校验与递增、Redis 会话清理和统一响应行为 MUST 与迁移前保持一致
- **Then** service 层 MUST 继续负责领域错误到认证失败、token 无效、not found 或内部错误响应的映射

#### Scenario: Authentication tests use narrow fakes
- **Given** 单元测试只验证登录凭证校验流程
- **When** 测试构造认证凭证组件的仓储替身
- **Then** 测试替身 MUST 只需要实现凭证读取或凭证更新相关方法
- **Then** 测试替身 MUST NOT 为用户资料创建、用户列表查询或 token version 递增提供无关空实现

#### Scenario: Token version tests use narrow fakes
- **Given** 单元测试只验证刷新、改密凭据校验或退出全部设备中的 token version 行为
- **When** 测试构造认证会话组件的仓储替身
- **Then** 测试替身 MUST 只需要实现 token version 读取和递增相关方法
- **Then** 测试替身 MUST NOT 为用户资料创建、用户列表查询、按用户名读取或凭证更新提供无关空实现
