## 1. Constructor 与装配

- [x] 1.1 修改 `user-service/internal/features/user/infrastructure/postgres/user_store.go`，删除 `UserStoreParams`、`fx.In`、`name:"primary_db"` 和 Fx import，并将 `NewUserStore` 改为显式接收 `*ent.Client`。
- [x] 1.2 修改 `user-service/internal/features/user/fx.go`，通过 `fx.ParamTags(`name:"primary_db"`)` 在 composition 层适配命名 Ent client，并继续以 `UserProfileStore` port 提供 adapter。
- [x] 1.3 更新 user feature 中所有直接调用 `NewUserStore` 的测试和辅助代码，改为传入普通 `*ent.Client` 参数，不保留旧 constructor 或兼容 wrapper。

## 2. 架构检查

- [x] 2.1 扩展 `user-service/scripts/architecture-lint.sh`，禁止 user feature 的 domain、application、infrastructure 和 transport 生产包导入 `go.uber.org/fx`、`go.uber.org/dig` 或声明 `fx.In`、`fx.Out`、`dig.In`、`dig.Out` 以及 Fx/Dig 命名 struct tag。
- [x] 2.2 更新 `user-service/scripts/architecture-lint-test.sh` fixture，覆盖 user feature 违规样例，并确认 `fx.go`、`fx_test.go`、`*_test.go` 和生成辅助文件不会误报。

## 3. 验证

- [x] 3.1 运行 `cd user-service && go test ./internal/features/user/... -count=1`，确认用户资料 feature 测试通过。
- [x] 3.2 运行 `rg -n 'go\.uber\.org/(fx|dig)|fx\.(In|Out)' user-service/internal/features/user --glob '*.go' --glob '!fx.go' --glob '!fx_test.go'`，确认无输出。
- [x] 3.3 运行 `openspec validate remove-fx-from-user-adapters`，确认 change specs 通过校验。
- [x] 3.4 运行 `make user-service-architecture-lint`，确认架构检查通过。
- [x] 3.5 暂存本次预期代码、OpenSpec 和文档变更后运行 `make lint`，确认 lint 通过。
- [x] 3.6 保持本次预期变更已暂存后运行 `make verify`，确认完整验证通过且无未暂存预期 drift。
