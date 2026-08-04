## MODIFIED Requirements

### Requirement: 角色、权限与用户绑定

系统 MUST 提供角色创建、更新、启停、详情、列表和角色权限绑定，以及用户角色绑定的查询、添加、移除和完整替换能力。公开写接口 MUST NOT 允许创建或篡改系统角色；普通角色 metadata、状态和权限绑定写端口 MUST 在同一数据库事务内锁定目标角色并基于最新 `IsSystem` 拒绝系统角色。绑定 MUST 只引用存在的代码基线权限、未软删除用户和启用角色，完整替换 MUST 保持事务性。角色权限完整替换 MUST 在 application 层批量校验去重后的完整权限集合，且权限校验查询次数 MUST NOT 随权限 ID 数量增长。

#### Scenario: 角色目录写入和查询

- **WHEN** 授权调用方提交合法角色信息和存在的权限集合
- **THEN** 系统 MUST 创建非系统角色并写入角色权限绑定，成功响应 MUST 返回新建角色实体
- **WHEN** 授权调用方更新、启用或停用存在的普通角色
- **THEN** 系统 MUST 持久化变更，成功响应 MUST NOT 包含更新后的角色实体
- **WHEN** 授权调用方查询角色详情或分页查询角色
- **THEN** 系统 MUST 返回角色数据、权限摘要和共享 pagination 信息

#### Scenario: 系统角色 metadata 与状态保护

- **WHEN** 普通角色接口尝试修改系统角色的 `Name`、`Description` 或 `Active`，或提交与当前值相同的 metadata 或状态
- **THEN** 系统 MUST 返回 `roledomain.ErrSystemRoleProtected` 语义且 HTTP 接口 MUST 返回 `409 Conflict`
- **AND** 系统 MUST 保持该角色全部字段、角色权限绑定和系统基线语义不变
- **WHEN** 公开角色创建接口创建角色
- **THEN** 持久化记录的 `IsSystem` MUST 为 `false`，公开输入 MUST NOT 提供提升为系统角色的方式

#### Scenario: 系统角色权限绑定保护

- **WHEN** 角色权限 add、replace 或 remove 普通写请求以系统角色为目标并引用合法基线权限
- **THEN** 系统 MUST 返回 `roledomain.ErrSystemRoleProtected` 语义且 HTTP 接口 MUST 返回 `409 Conflict`
- **AND** 系统 MUST 保持该系统角色的已有权限绑定集合不变
- **WHEN** 角色权限写请求引用不存在或不属于当前代码基线投影的权限
- **THEN** 系统 MUST 拒绝写入并保持已有关系不变
- **AND** role application MUST 通过 permission application 拥有的最小查询端口校验权限，MUST NOT 导入 permission infrastructure
- **WHEN** 调用方把任意现存基线权限绑定给普通角色
- **THEN** 系统 MUST 允许绑定且 MUST NOT 检查权限 active 或 system 状态
- **AND** Permission 状态语义的移除 MUST NOT 删除或改变 `Role.Active` 与 `Role.IsSystem`

#### Scenario: 系统角色保护的事务原子性

- **WHEN** 普通 metadata、状态或角色权限 store 开始目标角色写事务
- **THEN** store MUST 在任何角色或绑定 mutation 前以 PostgreSQL `FOR UPDATE` 锁定目标角色，并以锁定后的最新 `IsSystem` 判定是否允许写入
- **AND** metadata UPDATE MUST 额外强制 `is_system=false`，application 层事务外读取 MUST NOT 作为系统角色保护的权威判断
- **WHEN** store 判定目标为系统角色并返回 `ErrSystemRoleProtected`
- **THEN** 本次事务 MUST NOT 改变角色、角色权限绑定、policy revision counter、policy revision 或 outbox event
- **AND** application MUST NOT 发送 policy change 通知或触发本实例 reload

#### Scenario: 系统角色保护与并发 seed

- **WHEN** RBAC seed 正在更新目标系统角色并持有该角色数据库行锁，普通 metadata、状态或角色权限写请求并发到达
- **THEN** 普通写请求 MUST 等待 seed transaction 结束并读取提交后的最新角色状态
- **AND** seed 提交 `IsSystem=true` 后普通写请求 MUST 返回 `ErrSystemRoleProtected`
- **AND** 普通写请求 MUST NOT 覆盖 seed metadata、改变系统角色绑定、推进 revision 或创建 outbox event
- **WHEN** 系统维护代码写入系统角色或其基线权限绑定
- **THEN** 系统 MUST 只使用 `SeedRoleStore` 或 `SeedRolePermissionStore` 受信端口，普通 HTTP use case MUST NOT 获得绕过参数、兼容开关或受信端口依赖

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

- **WHEN** 调用方以合法权限集合完整替换普通角色权限
- **THEN** 系统 MUST 在同一事务中删除旧绑定并批量写入新绑定，任一非幂等错误 MUST 回滚全部变更
- **AND** application 批量校验与事务写入之间任一权限变为不存在时，事务内重校验 MUST 拒绝替换并保持已有关系不变
- **WHEN** 普通角色被停用
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
