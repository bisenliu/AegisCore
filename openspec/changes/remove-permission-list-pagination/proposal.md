## Why

权限目录由代码基线定义并同步为数据库只读投影，当前列表接口继续暴露 `cursor`、`page_size` 和 `data.pagination` 会让调用方误以为权限目录是可分页资源。该变更将权限列表收敛为完整匹配集合，减少前后端处理分支，并保持 `items` 包装以降低响应结构变化。

## What Changes

- **BREAKING**: `GET /api/v1/permissions` 不再接受或展示 `cursor`、`page_size` 分页参数。
- **BREAKING**: 权限列表响应删除 `data.pagination`，响应数据调整为 `data.items`。
- 权限列表继续支持 `module` 和 `http_method` 过滤，非法 HTTP method 仍返回 `400 Bad Request`。
- 权限列表按稳定 `permission_id` 排序返回所有匹配权限。
- 用户列表和角色列表分页契约保持不变。
- Casbin、RBAC seed、权限表结构和数据库 migration 不发生变化。
- OpenAPI 文档和压测脚本同步反映权限列表不分页的契约。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `rbac-access-control`: 权限目录列表从分页查询改为返回完整匹配权限集合，且响应不包含 `pagination`。

## Impact

- API: `GET /api/v1/permissions` 请求参数和成功响应结构变化；`GET /api/v1/permissions?module=user` 与 `GET /api/v1/permissions?http_method=GET` 继续可用。
- 代码: 调整 permission HTTP request、input preparer、controller、response doc、application query/result、store port、PostgreSQL adapter、mapper 和相关测试。
- OpenAPI: 重新生成 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`。
- 测试与脚本: 更新 permission controller、input、query、store、mapper 测试，更新 mockgen 生成 mock，压测脚本删除权限查询 `page_size` 并使用非法 `http_method` 覆盖错误查询。
- 规格: 更新 `rbac-access-control` 权限目录场景，明确权限列表返回完整匹配集合且不包含 pagination。
- 数据库与运行时授权: 不涉及 Ent schema、Atlas migration、Casbin policy、seed 数据语义或权限基线变更。
