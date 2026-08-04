## MODIFIED Requirements

### Requirement: 角色、权限与用户绑定

系统 MUST 提供角色创建、更新、启停、详情、列表和角色权限绑定，以及用户角色绑定的查询、添加、移除和完整替换能力。公开写接口 MUST NOT 允许创建或篡改系统角色；绑定 MUST 只引用存在的代码基线权限、未软删除用户和启用角色，完整替换 MUST 保持事务性。角色权限完整替换 MUST 在 application 层批量校验去重后的完整权限集合，且权限校验查询次数 MUST NOT 随权限 ID 数量增长。

#### Scenario: 角色目录写入和查询

- **WHEN** 授权调用方提交合法角色信息和存在的权限集合
- **THEN** 系统 MUST 创建非系统角色并写入角色权限绑定，成功响应 MUST 返回新建角色实体
- **WHEN** 授权调用方更新、启用或停用存在的普通角色
- **THEN** 系统 MUST 持久化变更，成功响应 MUST NOT 包含更新后的角色实体
- **WHEN** 授权调用方查询角色详情或分页查询角色
- **THEN** 系统 MUST 返回角色数据、权限摘要和共享 pagination 信息

#### Scenario: 系统角色与权限绑定保护

- **WHEN** 普通角色接口尝试创建、修改或停用系统角色
- **THEN** 系统 MUST 拒绝操作并保持系统角色及其基线语义不变
- **WHEN** 角色权限写请求引用不存在或不属于当前代码基线投影的权限
- **THEN** 系统 MUST 拒绝写入并保持已有关系不变
- **AND** role application MUST 通过 permission application 拥有的最小查询端口校验权限，MUST NOT 导入 permission infrastructure
- **WHEN** 调用方把任意现存基线权限绑定给普通角色
- **THEN** 系统 MUST 允许绑定且 MUST NOT 检查权限 active 或 system 状态
- **AND** Permission 状态语义的移除 MUST NOT 删除或改变 `Role.Active` 与 `Role.IsSystem`

#### Scenario: 角色权限完整替换的 application 批量校验

- **WHEN** 调用方以一个或多个 permission ID 完整替换角色权限
- **THEN** role application MUST 先按首次出现顺序去重，并通过 permission application 查询端口一次性校验整个权限集合
- **AND** permission PostgreSQL store MUST 只执行一次 `WHERE permission_id IN (...)` 查询，100 个与 1000 个 permission ID 的 permission lookup SQL 查询次数 MUST 均为 1
- **AND** 系统 MUST NOT 依赖 PostgreSQL `IN` 查询的自然返回顺序，成功结果及传给绑定替换的权限顺序 MUST 与去重后的输入顺序一致
- **WHEN** 批量校验的任一 permission ID 不存在
- **THEN** 系统 MUST 返回 `permissiondomain.ErrPermissionNotFound` 语义且 MUST NOT 返回部分成功结果
- **AND** role application MUST NOT 调用角色权限绑定替换或发送 policy change 通知

#### Scenario: 空角色权限集合的批量校验

- **WHEN** 批量权限查询收到空 permission ID 集合
- **THEN** 系统 MUST 返回非 nil 空权限集合且 MUST NOT 访问数据库
- **AND** role application MUST 继续以空集合执行合法的角色权限完整替换

#### Scenario: 角色权限替换、停用与 seed

- **WHEN** 调用方以合法权限集合完整替换角色权限
- **THEN** 系统 MUST 在同一事务中重新批量校验完整权限集合、删除旧绑定并批量写入新绑定，任一非幂等错误 MUST 回滚全部变更
- **AND** application 批量校验与事务写入之间任一权限变为不存在时，事务内重校验 MUST 拒绝替换并保持已有关系不变
- **WHEN** 角色被停用
- **THEN** 该角色 MUST NOT 出现在用户有效角色、有效权限或 Casbin policy 中
- **WHEN** seed 补齐或同步系统角色权限绑定
- **THEN** 系统 MUST 批量维护绑定并将已有绑定视为幂等成功，非唯一冲突错误 MUST 使本次事务失败

#### Scenario: 用户角色绑定与完整替换

- **WHEN** 调用方将存在且启用的角色绑定给存在且未软删除的用户
- **THEN** 系统 MUST 写入用户角色关系并使后续授权能够使用该角色
- **WHEN** 写请求引用不存在或已软删除的用户、不存在的角色或已停用角色
- **THEN** 系统 MUST 拒绝写入并返回明确错误
- **AND** 系统 MUST NOT 改变已有关系、失效缓存或发送 policy change 通知
- **WHEN** 调用方以全部合法且启用的角色集合完整替换用户角色
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定
- **AND** 任一角色不可用或任一写入失败时系统 MUST 回滚全部变更

#### Scenario: 有效权限聚合

- **WHEN** 系统或授权调用方查询用户有效权限
- **THEN** 系统 MUST 聚合该用户当前启用角色及其绑定的存在权限并返回去重集合
- **AND** 响应 MUST NOT 包含权限 `active`、`is_system` 或 `system`
- **AND** 角色、权限、用户角色和角色权限 MUST 使用外部 UUID 作为稳定业务标识，join 表内部标识 MUST NOT 暴露给 application 或 transport
- **WHEN** 已认证用户没有有效角色绑定并访问受保护路由
- **THEN** 系统 MUST 拒绝访问
