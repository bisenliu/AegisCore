## 1. HTTP 契约调整

- [x] 1.1 修改 `user-service/internal/features/permission/transport/http/request.go`，让 `ListPermissionsRequest` 只包含 `module` 和 `http_method` 查询参数。
- [x] 1.2 修改 `user-service/internal/features/permission/transport/http/input.go`，删除 cursor UUID 解析和 `pagination.NormalizePageSize`，只 trim `module` 与 `http_method`。
- [x] 1.3 修改 permission HTTP controller 和 response doc，返回 `data.items` 且不包含 `data.pagination`。
- [x] 1.4 更新 Swagger 注释，删除 `cursor`、`page_size` 参数，并将权限列表描述改为完整权限目录查询。

## 2. Application 与 Store 调整

- [x] 2.1 修改 permission application 查询模型，删除 `Cursor`、`PageSize`、`Limit`、`NextCursor` 和 `HasNext`，让 `ListPermissionsResult` 只包含 `Items`。
- [x] 2.2 修改 permission store port，使 `List(ctx, input)` 返回 `[]permissiondomain.Permission`，并让 `ListPermissionsInput` 只包含 `Module` 和 `HTTPMethod`。
- [x] 2.3 修改 PostgreSQL permission store，删除 permission ID cursor predicate、`Limit(input.Limit + 1)` 和 hasNext 计算，查询全部匹配权限并继续按 `permission_id` 稳定排序。
- [x] 2.4 更新 permission mapper，删除 pagination mapper/import，并保持权限响应 items 字段映射不变。
- [x] 2.5 更新所有受 store port 签名变化影响的调用点和 mockgen 生成 mock。

## 3. 测试与脚本

- [x] 3.1 更新 permission controller 测试，覆盖无 pagination 响应、`module` 过滤、`http_method` 过滤和非法 `http_method` 返回 400。
- [x] 3.2 更新 permission input、application query、store 和 mapper 单元测试，删除 cursor/page_size/hasNext 断言并新增完整集合与稳定排序断言。
- [x] 3.3 确认用户列表和角色列表分页测试保持通过，避免误删共享 pagination 能力。
- [x] 3.4 更新压测脚本，删除权限列表查询中的 `page_size`，并将权限错误查询改为 `GET /api/v1/permissions?http_method=not-a-method`。

## 4. OpenAPI 与规格同步

- [x] 4.1 运行 `make user-service-openapi-generate`，更新 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`。
- [x] 4.2 检查 OpenAPI diff，确认 `GET /api/v1/permissions` 只公开 `module`、`http_method` 参数，成功响应 schema 不包含 `pagination`。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认架构与规格边界检查通过。

## 5. 验证与收尾

- [x] 5.1 运行 permission 相关 Go 包测试，并运行受影响的 role 调用或 mock 相关测试。
- [x] 5.2 将本次预期代码、OpenAPI 生成物和 OpenSpec artifacts 加入暂存区，避免最终 `make verify` 被未暂存预期变更阻塞。
- [x] 5.3 运行 `make lint` 并确认通过。
- [x] 5.4 运行 `make verify` 并确认通过。
- [x] 5.5 检查 `git diff --cached`，确认未包含 Casbin、seed、权限表 schema 或数据库 migration 的非预期变更。
