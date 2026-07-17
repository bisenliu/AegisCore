## 1. Role Store Constructor 边界

- [x] 1.1 修改 `user-service/internal/features/role/infrastructure/postgres/role_store.go`，移除 `go.uber.org/fx` import、`RoleStoreParams` 和 `name:"primary_db"` tag，将 `NewRoleStore` 改为直接接收 `*ent.Client`。
- [x] 1.2 修改 `user-service/internal/features/role/infrastructure/postgres/role_permission_store.go`，移除 `go.uber.org/fx` import、`RolePermissionStoreParams` 和 `name:"primary_db"` tag，将 `NewRolePermissionStore` 改为直接接收 `*ent.Client`，不新增兼容 wrapper。
- [x] 1.3 修改 `user-service/internal/features/role/infrastructure/postgres/user_role_store.go`，移除 `go.uber.org/fx` import、`UserRoleStoreParams` 和 `name:"primary_db"` tag，将 `NewUserRoleStore` 改为直接接收 `*ent.Client`。

## 2. Fx Composition 适配

- [x] 2.1 修改 `user-service/internal/features/role/fx.go`，在 composition 层把具名 `primary_db` Ent client 适配到 `NewRoleStore`、`NewRolePermissionStore` 和 `NewUserRoleStore` 的普通 Go constructor，并继续 `fx.As` 到 role application ports。
- [x] 2.2 确认 `PermissionLookup` 仍通过 `roleapplication.PermissionLookup` 窄 port 注入 application service，且 role infrastructure 不导入 permission feature infrastructure 以外的新宽接口。
- [x] 2.3 确认 role feature Fx module 保留，且不改变 controller、application service、RBAC seed、watcher、Casbin initial load 或 Redis policy sync 生命周期。

## 3. 测试更新

- [x] 3.1 更新 role infrastructure store 测试中的 `NewRoleStore`、`NewRolePermissionStore`、`NewUserRoleStore` 调用，改为直接传入 Ent test client。
- [x] 3.2 更新 role seed、command、query 或 composition 测试中受 constructor 签名影响的调用点，确保不保留旧 Params 类型引用。
- [x] 3.3 运行 `cd user-service && go test ./internal/features/role/... -count=1`，并修复 role feature 测试失败。

## 4. 架构检查

- [x] 4.1 在 `user-service-architecture-lint` 对应脚本或规则中增加 role feature Fx/Dig 禁止检查，覆盖 domain、application、infrastructure 和 transport 生产 Go 文件，排除 `fx.go`、`fx_test.go` 与 `*_test.go`。
- [x] 4.2 确认规则检查 `go.uber.org/fx`、`go.uber.org/dig`、`fx.In`、`fx.Out`、`dig.In`、`dig.Out` 和仅服务于 DI 的 tag。
- [x] 4.3 运行 `rg -n 'go\.uber\.org/(fx|dig)|fx\.(In|Out)' user-service/internal/features/role --glob '*.go' --glob '!fx.go' --glob '!fx_test.go'`，确认无输出。

## 5. OpenSpec 与全量验证

- [x] 5.1 运行 `openspec validate remove-fx-from-role-adapters`，修复 proposal、design、tasks 或 spec delta 校验问题。
- [x] 5.2 运行 `make user-service-architecture-lint`，确认新增架构规则和既有规则通过。
- [x] 5.3 检查本 change 不产生 Ent、Atlas migration、OpenAPI 或部署观测生成物；如出现非预期生成物 drift，移除或解释并补充对应验证。
- [x] 5.4 暂存本次预期代码、测试、架构 lint 和 OpenSpec artifact 变更。
- [x] 5.5 运行 `make lint`，修复 lint 失败。
- [x] 5.6 运行 `make verify`，修复 verify 失败并确认最终 diff 状态符合仓库验证要求。
