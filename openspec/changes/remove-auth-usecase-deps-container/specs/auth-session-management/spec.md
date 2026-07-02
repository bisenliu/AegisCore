## ADDED Requirements

### Requirement: 认证 command use case 最小依赖边界

认证 command use case MUST 通过自身 constructor 声明最小依赖，并且结构体 MUST 只保存该 use case 实际需要的 collaborator。系统 MUST NOT 通过跨多个 command use case 的共享依赖容器向单个 use case 暴露无关的 credential、token、session、metrics 或配置依赖。

#### Scenario: 退出当前会话不能访问无关凭证依赖

- **WHEN** 实现或维护退出当前会话 use case
- **THEN** 该 use case MUST 只注入撤销当前 refresh session 和记录退出指标所需的依赖
- **AND** 该 use case MUST NOT 通过共享依赖容器访问 credential verifier、token issuer、refresh token rotation 配置或其他无关 collaborator

#### Scenario: 登录与刷新复用签发逻辑不扩大依赖面

- **WHEN** 登录或刷新 use case 复用 access token、refresh token 和 refresh session 创建逻辑
- **THEN** 复用逻辑 MUST 以显式参数或窄 helper 表达所需的 token issuer 与 session lifecycle
- **AND** 复用逻辑 MUST NOT 要求调用方持有覆盖其他 use case 的公共依赖容器

#### Scenario: Fx 装配表达 use case 真实依赖

- **WHEN** user-service 装配 auth command use case
- **THEN** Fx provider MUST 直接提供各 use case constructor 所需的最小参数结构
- **AND** 系统 MUST NOT 继续 provide 或消费 `UseCaseDeps` 作为 auth command use case 的公共装配入口

#### Scenario: 测试 fixture 不隐藏依赖边界

- **WHEN** command 包测试构造登录、刷新、改密、退出当前会话或退出全部会话 use case
- **THEN** 测试 MUST 按被测 use case 的最小 constructor 参数直接提供 mock collaborator
- **AND** 测试 MUST NOT 通过公共 `UseCaseDeps` fixture 隐藏单个 use case 的真实依赖面
