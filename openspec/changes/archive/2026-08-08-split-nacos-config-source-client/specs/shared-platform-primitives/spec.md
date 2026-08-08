## MODIFIED Requirements

### Requirement: 显式配置来源与加载管线

系统 MUST 将配置文档来源、文档合成、严格解码和服务配置策略表达为显式边界。`common/runtime/config` MUST 只拥有业务中立的配置 schema、document contract、deep merge、raw digest、strict decode、encode、render、redact 和通用校验原语；具体 Nacos 环境、认证、client、failover 和文档读取 MUST 位于 Nacos source adapter。服务 MUST 显式组合 defaults、normalize 和 validate，shared loader MUST NOT 通过服务类型的隐式接口自动发现这些行为。本地配置目录与 Namespace 的发布选择 MUST 留在 Compose 初始化服务和仓库级发布工具边界，runtime Nacos source MUST NOT 读取或解释仓库目录。

#### Scenario: Nacos source adapter 职责边界

- **WHEN** Nacos source adapter 通过 Nacos v3 HTTP API 读取配置文档
- **THEN** adapter MUST 将 server endpoint 解析、认证 token 获取与复用、HTTP 请求/响应处理、按声明顺序 failover 和单个 dataId 读取保持在 `common/runtime/config/nacos` package 内
- **AND** adapter MUST 继续实现业务中立 `DocumentSource` contract，使调用方能通过 `config.LoadSource` 合成多 dataId 文档并生成 `SourceMetadata`
- **AND** adapter MUST NOT 把 Nacos 认证、endpoint、failover、watch、长轮询、SDK fallback、服务发现或仓库目录映射能力加入 `common/runtime/config` 核心包

#### Scenario: Nacos source 失败语义

- **WHEN** Nacos source 依次尝试多个 server 读取同一 dataId
- **THEN** adapter MUST 按声明顺序尝试 server，并在总 timeout 预算内为剩余 server 分配尝试预算，单个故障 server MUST NOT 独占全部预算
- **AND** 全部 server 失败时错误 MUST 聚合每个已尝试 server 的 origin 和原始失败原因
- **AND** Nacos envelope、配置结果、HTTP 状态、JSON 解码、响应体读取和响应体超限错误 MUST 保持可诊断但不包含配置文档内容、password 或 access token
- **AND** 认证启用时 adapter MUST 使用 `Bearer` token 调用配置接口并复用已取得 token；未启用认证时 MUST NOT 发送认证 header

#### Scenario: Nacos source 本地示例

- **WHEN** 示例或测试展示 Nacos source 与配置加载管线的组合
- **THEN** 示例 MUST 使用本地 `httptest.Server` 模拟 Nacos v3 响应，MUST NOT 连接真实 Nacos、依赖外部网络或读取仓库部署目录
- **AND** 示例 MUST 展示多个 dataId 按声明顺序加载、后者覆盖前者、metadata 保留 provider、service、namespace、group、dataId 顺序和 digest
