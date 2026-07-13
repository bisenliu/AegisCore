## MODIFIED Requirements

### Requirement: 权限目录管理

系统 MUST 提供权限目录创建、更新、启停、查询、列表和路由差异分析能力，用于描述可授权的 HTTP 资源和动作。权限创建 MUST 返回新建权限实体；权限更新、启用和停用成功后 MUST 返回无实体成功响应，调用方如需最新实体 MUST 使用查询接口读取。公开权限创建 API MUST NOT 接收或使用调用方提供的系统权限标记，普通创建路径 MUST 创建非系统权限。

#### Scenario: 创建权限

- **WHEN** 授权调用方提供合法权限标识、方法、路径和描述
- **THEN** 系统 MUST 创建权限记录，并使其可参与后续角色绑定和授权判断
- **AND** 系统 MUST 返回新建权限实体
- **AND** 新建权限 MUST 为非系统权限
- **AND** 普通权限创建路径 MUST NOT 允许调用方制造系统权限

#### Scenario: 创建权限忽略或拒绝公开系统标记

- **WHEN** 授权调用方通过公开权限创建 API 提交 `system` 字段
- **THEN** 系统 MUST NOT 创建系统权限
- **AND** 若 HTTP JSON 绑定启用未知字段拒绝，系统 MUST 返回请求错误
- **AND** 若 HTTP JSON 绑定忽略未知字段，系统 MUST 创建非系统权限

#### Scenario: 更新权限不返回实体

- **WHEN** 授权调用方更新存在的权限目录记录且输入合法
- **THEN** 系统 MUST 持久化权限元数据变更
- **AND** 成功响应 MUST NOT 包含权限实体响应体
- **AND** 持久化层 MUST NOT 为构造成功响应而在更新后重新查询该权限实体

#### Scenario: 启停权限不返回实体

- **WHEN** 授权调用方启用或停用存在的权限目录记录
- **THEN** 系统 MUST 持久化权限启用状态变更
- **AND** 成功响应 MUST NOT 包含权限实体响应体
- **AND** 持久化层 MUST NOT 为构造成功响应而在更新后重新查询该权限实体

#### Scenario: 权限输入非法

- **WHEN** 权限方法、路径、标识或描述不满足 domain validation
- **THEN** 系统 MUST 拒绝创建或更新，并返回一致的校验错误

#### Scenario: 路由差异分析

- **WHEN** 系统扫描已注册 HTTP 路由并与权限目录比较
- **THEN** 系统 MUST 能识别 missing、stale 或不一致的权限定义，且 MUST NOT 创建权限、修改权限状态或绑定角色

#### Scenario: 可授权路由发现

- **WHEN** 系统构造 route diff 诊断输入
- **THEN** 系统 MUST 排除 `OPTIONS`、`/api/v1/` 外路径和认证公开或会话控制路由，且 application 层 MUST NOT 直接依赖 Gin engine

### Requirement: 角色与权限绑定

系统 MUST 提供角色创建、更新、查询、列表和角色权限绑定能力，并保证绑定引用的权限存在且状态可用。角色创建 MUST 返回新建角色实体；角色更新、启用和停用成功后 MUST 返回无实体成功响应。角色权限绑定的替换、系统绑定补齐和系统绑定同步 MUST 使用批量写入方式新增多条绑定，并保持事务性和错误语义。公开角色创建 API MUST NOT 接收或使用调用方提供的系统角色标记，普通创建路径 MUST 创建非系统角色。

#### Scenario: 创建角色并绑定权限

- **WHEN** 授权调用方创建角色并指定合法权限集合
- **THEN** 系统 MUST 持久化角色、写入角色权限绑定，并使授权策略可同步使用
- **AND** 新建角色 MUST 为非系统角色
- **AND** 普通角色创建路径 MUST NOT 允许调用方制造系统角色

#### Scenario: 创建角色忽略或拒绝公开系统标记

- **WHEN** 授权调用方通过公开角色创建 API 提交 `system` 字段
- **THEN** 系统 MUST NOT 创建系统角色
- **AND** 若 HTTP JSON 绑定启用未知字段拒绝，系统 MUST 返回请求错误
- **AND** 若 HTTP JSON 绑定忽略未知字段，系统 MUST 创建非系统角色

#### Scenario: 更新角色不返回实体

- **WHEN** 授权调用方更新存在的角色记录且输入合法
- **THEN** 系统 MUST 持久化角色元数据变更
- **AND** 成功响应 MUST NOT 包含角色实体响应体
- **AND** 持久化层 MUST NOT 为构造成功响应而在更新后重新查询该角色实体

#### Scenario: 启停角色不返回实体

- **WHEN** 授权调用方启用或停用存在的角色记录
- **THEN** 系统 MUST 持久化角色启用状态变更
- **AND** 成功响应 MUST NOT 包含角色实体响应体
- **AND** 持久化层 MUST NOT 为构造成功响应而在更新后重新查询该角色实体

#### Scenario: 绑定不存在权限

- **WHEN** 角色绑定请求引用不存在或不可用的权限
- **THEN** 系统 MUST 拒绝绑定并保持已有角色权限关系不被破坏

#### Scenario: 角色通过权限端口校验

- **WHEN** 角色 application 需要校验权限 ID
- **THEN** 角色 feature MUST 通过权限 application 查询端口校验，不得导入 permission infrastructure

#### Scenario: 停用角色

- **WHEN** 角色已停用
- **THEN** 通过该角色形成的绑定 MUST NOT 在有效权限查询或 Casbin policy 加载中授予访问权限

#### Scenario: 查询角色列表

- **WHEN** 授权调用方分页查询角色
- **THEN** 系统 MUST 返回角色列表、权限摘要和共享 pagination 信息

#### Scenario: 批量替换角色权限绑定

- **WHEN** 授权调用方使用合法权限集合替换角色的完整权限绑定
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一新增绑定发生非幂等错误时，系统 MUST 回滚本次删除和新增

#### Scenario: 批量维护系统角色权限绑定

- **WHEN** RBAC seed 补齐或同步系统角色权限绑定
- **THEN** 系统 MUST 批量新增缺失绑定
- **AND** 已存在绑定的唯一冲突 MUST 保持幂等成功语义
- **AND** 非唯一冲突错误 MUST 使本次操作失败并保持既有事务回滚语义

### Requirement: RBAC 系统数据引导

系统 MUST 提供 CLI 能力初始化系统角色、系统权限、系统绑定，并支持为用户分配或创建超级管理员。系统角色和系统权限 MUST 仅由 RBAC seed port 根据代码基线写入或更新，普通公开 API、普通 command 和普通 store create 路径 MUST NOT 写入系统角色或系统权限。

#### Scenario: 初始化 RBAC 系统数据

- **WHEN** 运维执行 `aegiscore-user-services rbac seed`
- **THEN** 系统 MUST 创建或更新默认系统角色、权限和绑定，并输出插入、更新、绑定增删统计；seed MUST NOT 自动创建真实业务用户或为任意业务用户分配超级管理员角色
- **AND** RBAC seed port 写入的默认角色和默认权限 MUST 标记为系统数据
- **AND** 默认系统角色和默认系统权限 MUST 来自代码基线

#### Scenario: 只有 seed port 可写系统角色和系统权限

- **WHEN** 非 seed 的角色或权限创建路径写入数据
- **THEN** 系统 MUST 固定写入非系统数据
- **AND** 系统 MUST NOT 从普通 command、普通 store create input 或公开 HTTP 请求读取系统标记

#### Scenario: 分配超级管理员

- **WHEN** 运维执行 `rbac assign-super-admin --user-id <uuid>`
- **THEN** 系统 MUST 为指定已存在用户绑定内置超级管理员角色

#### Scenario: 创建超级管理员

- **WHEN** 运维执行 `create-super-admin` 并提供管理员密码环境变量
- **THEN** 系统 MUST 创建或复用管理员用户并绑定内置超级管理员角色；已有管理员默认 MUST NOT 重置密码，只有显式传入 `--reset-password` 或 `ADMIN_RESET_PASSWORD=true` 时才允许重置密码

#### Scenario: 缺少管理员密码

- **WHEN** 创建超级管理员时缺少配置的密码环境变量或密码为空
- **THEN** 系统 MUST 拒绝执行并返回明确错误

#### Scenario: 运行中执行离线 RBAC 命令

- **WHEN** HTTP 副本已经运行时执行 `rbac seed`、`rbac assign-super-admin` 或 `rbac create-super-admin`
- **THEN** 命令只修改持久化数据，不得被视为运行期 policy refresh；运维 MUST 滚动重启副本或触发在线 RBAC 刷新
