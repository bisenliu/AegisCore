## 1. Bootstrap 基础 App options

- [x] 1.1 在 `user-service/internal/bootstrap/app.go` 增加接收已解析 `*serviceconfig.Config` 的无 I/O 基础 Fx options 构建入口，统一 supply service config 与单次派生的共享 runtime config，并接入 logger、`AppModule`、`fx.StartTimeout` 和 `fx.StopTimeout`。
- [x] 1.2 将 `bootstrap.NewApp` 改为消费已解析配置和基础 options，移除 `ConfigPath`、`serviceconfig.NewConfig` 与 `serviceconfig.NewRuntimeConfig` 的第二套 provider 链，保持现有 module、provider、invoke 和 lifecycle hook 顺序。
- [x] 1.3 新增或更新 `user-service/internal/bootstrap` 装配测试，验证 service config 指针身份、runtime config 投影、`App.StartTimeout()`/`App.StopTimeout()` 与依赖图解析均来自同一配置，并确认基础 options 可追加测试 option 而无需配置 I/O。
- [x] 1.4 更新 `user-service/internal/providers` 相关装配测试，使其复用基础 options 或显式 supply 已解析的同源 service/runtime config，且不建立 `ConfigPath -> NewConfig` provider 链。

## 2. Serve 配置所有权与生命周期接线

- [x] 2.1 修改 `user-service/cmd/serve.go` 的 `lifecycleAppFactory` 和正式 factory，使 `runServe` 唯一加载并校验 service config 后把同一个配置对象传给 `bootstrap.NewApp`，同时继续从该对象建立 Start/Stop context。
- [x] 2.2 保留外部 context、`App.Wait()`、非零 shutdown code、单次 `App.Stop()` 和未取消上游 context value 的现有语义，确认 App 顶层 Fx timeout 与 CLI 显式 context 使用相同配置值但不形成累加预算。
- [x] 2.3 更新 `user-service/cmd` 的局部 App factory 与 lifecycle 替身测试，断言配置错误时 factory 不被调用、成功时 factory 收到同一已解析对象，并覆盖 Start/Stop deadline、内部/外部退出和 Stop 只调用一次。
- [x] 2.4 全仓迁移受 `bootstrap.NewApp` 与 `lifecycleAppFactory` 签名影响的正式和测试调用点；保持 `user-service/cmd/fxgraph.go` 行为不变，不在本 change 迁移其配置或依赖图接线。

## 3. 文档、注释与回归检查

- [x] 3.1 更新 `common/runtime/config` lifecycle 注释、`user-service/configs/config.yaml` 注释及相关 architecture/development/testing 文档，明确配置在 `fx.New` 前解析、`start_timeout` 约束 `App.Start` lifecycle 阶段且不限制 `fx.New` 同步构造。
- [x] 3.2 扫描 `user-service/cmd/serve.go`、`internal/bootstrap` 和 `internal/providers`，确认正式 serve/App 装配只有一次 `serviceconfig.NewConfig` 调用、没有第二套 `ConfigPath` provider 链，且基础 options 未进入 `common`、feature、`internal/shared` 或 `internal/integration`。
- [x] 3.3 对本 change 修改的 Go 文件执行 `gofmt`，然后运行 `cd user-service && go test ./cmd ./internal/bootstrap ./internal/providers -count=1`，修复所有配置同源与装配回归。

## 4. OpenSpec 与架构验证

- [x] 4.1 核对 proposal、design、`shared-platform-primitives` 与 `runtime-observability` delta specs 和实现一致，并运行 `openspec validate unify-user-service-app-configuration`。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认 composition root、配置与文档归属符合仓库边界。
- [x] 4.3 确认未修改配置字段、默认值、环境变量契约、OpenAPI/Ent 生成物、数据库 migration、部署资产、观测资产或认证/RBAC 业务行为。

## 5. 最终交付门禁

- [x] 5.1 在实现、测试、规格和文档任务全部完成后，使用 `git add` 仅暂存本 change 的预期代码、测试、文档和 `openspec/changes/unify-user-service-app-configuration/`，再运行 `git diff --cached --check` 与 `git diff --cached --name-only` 检查暂存范围。
- [x] 5.2 在预期变更已暂存后运行 `make lint`；未通过时修复并重新暂存，未通过前不得勾选本任务。
- [x] 5.3 在预期变更已暂存且 lint 通过后运行 `make verify`，通过其最终 `git diff --exit-code` 暴露生成物 drift 或未暂存意外变更；未通过时修复并重新执行，未通过前不得将 change 视为完成。
