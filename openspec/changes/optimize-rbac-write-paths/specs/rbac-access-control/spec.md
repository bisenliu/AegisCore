## MODIFIED Requirements

### Requirement: 权限目录管理

系统 MUST 提供权限目录创建、更新、启停、查询、列表和路由差异分析能力，用于描述可授权的 HTTP 资源和动作。权限创建 MUST 返回新建权限实体；权限更新、启用和停用成功后 MUST 返回无实体成功响应，调用方如需最新实体 MUST 使用查询接口读取。

#### Scenario: 创建权限

- **WHEN** 授权调用方提供合法权限标识、方法、路径和描述
- **THEN** 系统 MUST 创建权限记录，并使其可参与后续角色绑定和授权判断
- **AND** 系统 MUST 返回新建权限实体

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

系统 MUST 提供角色创建、更新、查询、列表和角色权限绑定能力，并保证绑定引用的权限存在且状态可用。角色创建 MUST 返回新建角色实体；角色更新、启用和停用成功后 MUST 返回无实体成功响应。角色权限绑定的替换、系统绑定补齐和系统绑定同步 MUST 使用批量写入方式新增多条绑定，并保持事务性和错误语义。

#### Scenario: 创建角色并绑定权限

- **WHEN** 授权调用方创建角色并指定合法权限集合
- **THEN** 系统 MUST 持久化角色、写入角色权限绑定，并使授权策略可同步使用

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

### Requirement: 用户角色绑定

系统 MUST 支持将角色绑定到用户，并为授权判断提供用户有效权限查询能力。用户角色替换 MUST 使用批量写入方式新增多条绑定，并保持事务性和错误语义。

#### Scenario: 绑定角色给用户

- **WHEN** 授权调用方把存在的角色绑定给存在用户
- **THEN** 系统 MUST 写入用户角色关系，并使该用户后续访问权限生效

#### Scenario: 用户或角色不存在

- **WHEN** 用户角色绑定请求引用不存在的用户或角色
- **THEN** 系统 MUST 拒绝绑定并返回明确错误

#### Scenario: 查询用户有效权限

- **WHEN** 系统或调用方查询某用户有效权限
- **THEN** 系统 MUST 聚合用户角色和角色权限，返回该用户当前可访问的权限集合

#### Scenario: 用户无角色

- **WHEN** 已认证用户没有有效角色绑定并访问 RBAC 保护路由
- **THEN** 系统 MUST 拒绝访问

#### Scenario: 批量替换用户角色绑定

- **WHEN** 授权调用方使用合法角色集合替换用户的完整角色绑定
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一新增绑定失败时，系统 MUST 回滚本次删除和新增

### Requirement: 策略同步

系统 MUST 在在线 RBAC 写操作成功后触发本实例策略刷新，并通过 Redis policy version、Pub/Sub 和定时版本补偿同步其他副本。写操作成功响应是否包含实体 MUST NOT 影响 policy reload、用户角色缓存失效或跨副本同步语义。

#### Scenario: 在线角色绑定变更

- **WHEN** 用户角色绑定通过 HTTP API 变更成功
- **THEN** 本实例 MUST 执行策略刷新或用户角色缓存失效，并通知其他副本

#### Scenario: 在线权限策略变更

- **WHEN** 在线 RBAC 管理接口修改权限、角色启停或角色权限绑定并提交成功
- **THEN** 本实例 MUST 执行 policy reload，并通过 Redis policy version 和 Pub/Sub 通知其他副本；其他副本 MUST 通过 Pub/Sub 和周期性版本补偿感知变更

#### Scenario: 授权热路径

- **WHEN** 业务请求进入 RBAC 授权中间件
- **THEN** 授权 MUST 使用本实例内存 Casbin enforcer 和本地可用的用户角色解析结果，MUST NOT 每请求读取 Redis policy version 做强一致门控

#### Scenario: 写响应契约不影响同步

- **WHEN** 权限、角色、用户角色绑定或角色权限绑定写操作成功且响应不包含更新后实体
- **THEN** 系统 MUST 继续按既有规则触发本实例 policy reload、用户角色缓存失效和跨副本 policy version 通知
