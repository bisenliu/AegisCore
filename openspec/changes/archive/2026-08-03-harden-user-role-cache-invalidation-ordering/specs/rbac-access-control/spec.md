## ADDED Requirements

### Requirement: 用户角色缓存失效顺序门禁

系统 MUST 将 user-role cache 的回源写入与 per-user generation、全量 generation 或等价 revision 绑定。用户角色缓存失效 MUST 先提升对应 generation/revision，再删除或清空缓存项；任何在失效前开始、失效后完成的旧回源结果 MUST NOT 写入缓存。该门禁 MUST 位于 permission feature 的 resolver 或 cache wrapper 边界内，MUST NOT 将 RBAC 业务 revision 语义下沉到 `common/runtime/localcache` 公共 API。cache disabled 模式 MUST 继续直接回源并保持 fail-closed。

#### Scenario: 单用户失效抑制旧 load 写回

- **WHEN** 用户角色 cache miss 已经开始为某个用户回源，且该用户的 Add、Remove 或 Replace 用户角色绑定成功后调用 `InvalidateUserRole`
- **THEN** 系统 MUST 先提升该用户的 generation/revision，再删除该用户缓存项
- **AND** 失效前开始但失效后完成的旧回源结果 MUST NOT 写入该用户缓存
- **AND** 后续授权 MUST 重新回源并使用失效后的最终角色集合

#### Scenario: 全量失效抑制所有旧 load 写回
- **WHEN** 一个或多个用户角色 cache miss 已经开始回源，且系统调用 `InvalidateAllUserRoles`
- **THEN** 系统 MUST 先提升全量 generation/revision，再清空用户角色缓存
- **AND** 全量失效前开始但失效后完成的任一旧回源结果 MUST NOT 写入缓存
- **AND** 后续授权 MUST 重新回源并使用全量失效后的最终角色集合

#### Scenario: 失效竞态保持 fail-closed
- **WHEN** `RolesForUser` 的回源结果因 generation/revision 过期而被抑制
- **THEN** 当前授权请求 MUST fail-closed，MUST NOT 使用旧角色集合产生允许结果
- **AND** loader 错误、context 取消或过期回源结果 MUST NOT 写入缓存

#### Scenario: cache disabled 模式保持直接回源
- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 不创建通用 loading cache，并逐次从 PostgreSQL 回源当前启用角色
- **AND** `InvalidateUserRole` 与 `InvalidateAllUserRoles` MUST 保持安全且不得引入旧 load 写回路径
- **AND** 回源成功 MUST 返回独立角色 ID slice，回源错误或 context 取消 MUST 保持 fail-closed
