## Context

`GET /api/v1/permissions` 当前沿用通用分页契约，HTTP request 接收 `cursor` 和 `page_size`，application query/store 通过 cursor predicate 和 `Limit(input.Limit + 1)` 计算 `pagination`。但权限目录的业务来源是 `internal/shared/rbacbaseline.DefaultPermissions()`，数据库表只是只读投影；权限数量由代码基线控制，不是用户生成的无限增长列表。

本次变更仅收敛 permission list 查询契约：保留 `items` 包装、保留 `module` 和 `http_method` 过滤、保留稳定排序和 HTTP method validator，不改变权限基线、seed、Casbin policy、数据库 schema、用户列表或角色列表分页。

## Goals / Non-Goals

**Goals:**

- 删除权限列表请求中的 `cursor` 和 `page_size`，避免 API 展示不存在的业务分页语义。
- 删除权限列表响应中的 `data.pagination`，成功响应保持 `data.items` 包装。
- 简化 permission application query/result、store port 和 PostgreSQL adapter，让列表返回完整匹配权限集合。
- 保持 `module` 和 `http_method` 过滤，非法 HTTP method 继续由 application validator 返回 `400 Bad Request`。
- 保持按 `permission_id` 稳定排序，避免附带改变现有顺序契约。
- 同步更新 OpenAPI、测试、mock 和压测脚本。

**Non-Goals:**

- 不修改用户列表或角色列表分页契约。
- 不修改 Casbin 授权、policy sync、RBAC seed、权限基线或超级管理员语义。
- 不修改 Ent schema、Atlas SQL migration 或权限表字段。
- 不新增 common helper、shared 业务内核或跨服务契约。
- 不恢复权限创建、更新、启停、详情或 route diff 公开接口。

## Decisions

1. 权限列表返回完整匹配集合，而不是保留后端分页但隐藏响应。

   理由：权限目录来自代码基线，返回完整集合更符合业务模型；如果继续在 store 内分页但不暴露 pagination，会让调用方无法获得完整目录，并产生隐式截断风险。备选方案是固定大 `page_size` 或继续内部 limit，但这会把旧分页语义隐藏在实现中。

2. request 和 input preparer 只保留 `module`、`http_method` trim，不在 transport 层验证 HTTP method。

   理由：当前 HTTP method 合法性属于 application validator 责任，继续保留该边界可以避免 transport 与 application 重复校验。备选方案是在 input preparer 拒绝非法 method，但会改变现有错误来源和测试边界。

3. application query/result 和 store port 同步删除 pagination 字段。

   理由：分页不再是权限列表契约，保留 `Cursor`、`PageSize`、`Limit`、`NextCursor` 或 `HasNext` 会造成死字段和误用风险。备选方案是仅修改 controller 响应，但 application/store 仍保留分页复杂度，不符合契约收敛目标。

4. PostgreSQL adapter 删除 cursor predicate 和 limit，但继续按 `permission_id` 排序。

   理由：稳定排序是可见契约，保留排序可以降低响应变化范围；删除 cursor 和 limit 后，查询结果即为完整匹配集合。备选方案是不排序或改按创建时间排序，但会引入无关行为变化。

5. OpenAPI 使用新的 `PermissionListResponseDoc`，不复用 pagination mapper。

   理由：文档需要明确表达 `data.items` 且不存在 `data.pagination`；继续引用共享 pagination 类型会让生成物与实际响应不一致。

## Risks / Trade-offs

- 客户端仍发送 `cursor` 或 `page_size` 可能不会再看到文档支持。缓解：OpenAPI 删除这些参数，测试覆盖目标契约“不接受或展示 cursor/page_size”，实现时根据现有 binding 行为确保无效旧参数不会影响查询。
- 一次返回完整权限集合会增加单次响应大小。缓解：权限集合由代码基线控制，数量稳定且有限；保留 `module` 和 `http_method` 过滤。
- 删除 pagination 可能影响依赖旧响应的客户端。缓解：保留 `items` 包装，减少结构变化；在 proposal 中标记为 breaking change。
- store port 签名变化会影响 role 或测试 mock。缓解：更新消费侧调用、mockgen 生成物和相关单元测试，确保 role 绑定权限校验不受影响。

## Migration Plan

1. 修改 permission HTTP request、input preparer、controller、response doc 和 Swagger 注释。
2. 修改 permission application query/result、mapper、store port 和 PostgreSQL adapter，删除 cursor/limit/hasNext 逻辑并保留稳定排序。
3. 更新 controller、input、query、store、mapper 测试和 mockgen 生成 mock。
4. 更新压测脚本中的权限列表请求和错误查询请求。
5. 重新生成 OpenAPI 生成物。
6. 更新 `rbac-access-control` 规格 delta 并在归档时合并到主规格。
7. 验证相关包测试、`make user-service-openapi-generate`、`make user-service-architecture-lint` 和 `make verify`。

回滚方式：恢复 permission list 分页字段、store cursor/limit 查询和 OpenAPI 文档，再重新生成 OpenAPI；不需要数据库回滚或 seed 回滚。

## Open Questions

无。
