## 1. 调用点确认

- [x] 1.1 使用 `rg` 重新确认 `commoncasbin.Authorize`、`commoncasbin.Enforce`、`middleware.Auth` 和 `AuthWithTokenVersionValidator` 的仓库内生产调用点。
- [x] 1.2 根据调用点结果确认 package-level `casbin.Authorize` 是移除还是仅标记 `Deprecated`，并记录迁移方式。

## 2. Casbin helper 调整

- [x] 2.1 调整 `common/security/casbin/authorizer.go`，保留 `Enforce`，让 `Authorizer.Authorize` 直接基于 `Enforce` 处理 denied 语义。
- [x] 2.2 移除或废弃 package-level `casbin.Authorize`，确保 `ErrNotConfigured`、`ErrDenied` 和 enforcer error wrap 语义不变。
- [x] 2.3 更新 `common/security/casbin/authorizer_test.go`，覆盖 nil authorizer、nil enforcer、允许、拒绝、context canceled 和 enforcer error 场景。

## 3. JWT middleware helper 调整

- [x] 3.1 在 `common/http/middleware/auth.go` 中将 `Auth()` 标记为 `Deprecated`，说明推荐使用 `AuthWithTokenVersionValidator(log, jwtService, cfg, nil)`。
- [x] 3.2 确认 `AuthWithTokenVersionValidator` 的 token version validator 测试覆盖不降低；如需保留 wrapper 行为测试，仅验证其兼容委托语义。

## 4. 验证

- [x] 4.1 运行 `go test ./security/casbin ./http/middleware` 于 `common/` 模块，确认共享认证授权 helper 测试通过。
- [x] 4.2 运行 user-service 相关授权与路由测试，至少覆盖 `./internal/features/permission/...` 和 `./internal/router/...`。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认边界约束未被破坏。
- [x] 4.4 运行 `make verify`；若耗时或环境阻塞，记录未运行原因和已完成的替代验证。
- [x] 4.5 使用 `git diff --exit-code -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml user-service/ent` 确认未产生不应有的生成物 drift。
