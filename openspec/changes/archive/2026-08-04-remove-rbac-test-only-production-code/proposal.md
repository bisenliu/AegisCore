## Why

当前 RBAC 生产源码保留了只被测试调用的 watcher 构造器、默认角色权限 helper 和授权白名单 wrapper，并在默认角色 catalog 中保留未来角色伪代码模板。这些内容不参与生产运行，却扩大了生产 API 和维护面，也使生产调用图静态分析产生可避免的 unreachable 报告。

## What Changes

- 从 permission Redis watcher 生产文件删除测试专用构造器，将 fake store 所需组装收敛到 `_test.go`。
- 从 `internal/shared/rbacbaseline` 删除未被生产调用的 `permissionIDs`、对应 helper 自测和未来默认角色注释模板。
- 从 permission HTTP transport 删除 feature-local 白名单 rule/option alias 和 wrapper，直接使用 `common/http/middleware` 已有 Casbin option 类型与构造函数。
- 保持 `NewWatcher`、`DefaultRoles()`、`DefaultPermissions()`、`DefaultRolePermissions()`、`Authorize` 的生产行为以及显式授权白名单语义不变，不保留旧名称、兼容 alias、转发 wrapper 或备用分支。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `rbac-access-control`：新增生产源码与测试辅助代码边界要求，禁止仅为测试保留生产 helper、无服务语义的 common wrapper 和未来伪代码模板；运行时行为不变。

## Impact

- 代码：`user-service/internal/features/permission/infrastructure/redis/`、`user-service/internal/features/permission/transport/http/`、`user-service/internal/shared/rbacbaseline/` 及对应测试。
- API 与安全：HTTP 路由、响应、认证授权顺序、白名单匹配和 fail-closed 语义不变；删除项均无生产消费者。
- 数据与交付：不修改 Ent schema、Atlas migration、OpenAPI、配置、部署或观测资产，不新增依赖。
- 验证：运行相关包测试、生产调用图 deadcode 检查、架构 lint、lint 和 verify。
