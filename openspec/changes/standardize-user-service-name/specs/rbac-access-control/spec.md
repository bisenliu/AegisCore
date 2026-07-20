## MODIFIED Requirements

### Requirement: RBAC 系统数据与运维 CLI

系统 MUST 提供带服务上下文的 `rbac seed`、`rbac assign-super-admin` 和 `rbac create-super-admin` 命令，用于维护系统角色、系统权限、默认绑定和超级管理员。系统数据 MUST 只由 seed port 根据 `internal/shared/rbacbaseline` 写入。RBAC 运维 CLI MUST 通过 `aegiscore-user-service` 根命令调用，旧 `aegiscore-user-services` 根命令 MUST NOT 作为 RBAC 兼容入口保留。

#### Scenario: 初始化系统基线

- **WHEN** 运维执行 `aegiscore-user-service rbac seed`
- **THEN** 系统 MUST 幂等创建或更新基线角色、权限和绑定并输出变更统计
- **AND** 系统角色和权限 MUST 标记为系统数据
- **AND** seed MUST NOT 创建业务用户或自动分配超级管理员
- **AND** 非 seed 的角色或权限 command、store create 或公开 HTTP 路径 MUST 固定写入非系统数据并 MUST NOT 接收系统标记

#### Scenario: 超级管理员维护

- **WHEN** 运维执行 `rbac assign-super-admin --user-id <uuid>`
- **THEN** 系统 MUST 为指定存在用户幂等绑定内置超级管理员角色
- **WHEN** 运维执行 `rbac create-super-admin` 并提供合法 username 和密码
- **THEN** 系统 MUST 创建或复用用户并绑定内置超级管理员角色
- **AND** username MUST trim 后转为小写，空 nickname MUST 回退为归一化 username
- **AND** 未显式指定 password env 时系统 MUST 从 `ADMIN_PASSWORD` 读取非空密码
- **AND** 已有用户的密码 MUST NOT 默认重置，只有显式 `--reset-password` 或 `ADMIN_RESET_PASSWORD=true` 时系统才 MUST 更新密码
- **AND** 必需输入缺失时命令 MUST 返回明确错误

#### Scenario: 离线命令不等同在线刷新

- **WHEN** HTTP 副本运行期间执行 seed、assign-super-admin 或 create-super-admin
- **THEN** 命令 MUST 只修改持久化数据并 MUST NOT 宣称已触发运行期 policy refresh
- **AND** 运维 MUST 滚动重启副本或触发在线 RBAC 刷新使运行实例收敛

### Requirement: RBAC 可观测性

系统 MUST 为 RBAC 授权判定和正式模块执行的 route diff 提供低基数 metrics，并使用显式注入的 logger 记录加载和同步异常。观测失败 MUST NOT 改变授权或策略同步结果。RBAC policy sync Redis key prefix、Pub/Sub channel、metrics `service` label 和 route diff 观测输出 MUST 使用 `aegiscore-user-service`，旧 `aegiscore-user-services` prefix 或兼容 channel MUST NOT 被读取、发布或订阅。

#### Scenario: 授权 metrics 的低基数与敏感数据约束

- **WHEN** permission authorization service 完成一次 RBAC Enforce 判定
- **THEN** counter MUST 记录 `result="allow"`、`result="deny"` 或 `result="error"`
- **AND** histogram MUST 记录本次判定耗时
- **AND** 标签 MUST 只使用 `result`、HTTP method 和 route template
- **AND** 指标 MUST NOT 包含用户、角色、权限、token、trace、IP、账号、Redis key、SQL、原始错误或 raw path
- **AND** user-service 默认 `service` label MUST 为 `aegiscore-user-service`

#### Scenario: route diff 和日志观测

- **WHEN** 正式 permission 模块执行 route diff
- **THEN** 系统 MUST 记录本次 missing、stale 和不一致结果
- **AND** route diff metrics MUST 使用当前单数服务名

#### Scenario: RBAC policy sync key 和 channel

- **WHEN** permission Redis adapter 生成 policy version key 或 policy refresh channel
- **THEN** key 和 channel prefix MUST 来自当前 `app.name` 并归一化为 `aegiscore-user-service`
- **AND** adapter MUST NOT 读取、发布、订阅或迁移旧 `aegiscore-user-services` prefix 下的 policy version key 或 Pub/Sub channel
- **AND** 发布后副本收敛 MUST 依赖新 prefix 下的 version key、Pub/Sub channel 和周期性补偿
