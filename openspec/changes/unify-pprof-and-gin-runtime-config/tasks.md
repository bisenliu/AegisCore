## 1. 共享配置契约

- [x] 1.1 在 `common/runtime/config` 增加 `RuntimeConfig.Gin`、`GinConfig`、`ObservabilityConfig.Pprof` 和 `PprofConfig`，字段映射为 `runtime.gin.mode`、`observability.pprof.enabled`、`observability.pprof.addr`。
- [x] 1.2 在共享默认配置中设置 `runtime.gin.mode=release`、`observability.pprof.enabled=false`、`observability.pprof.addr=127.0.0.1:6060`，并在 `setCoreDefaults` 注册对应 key。
- [x] 1.3 在共享配置校验中增加 Gin mode 枚举校验、pprof `host:port` 校验和生产类环境 loopback 校验，错误路径必须指向完整配置字段。
- [x] 1.4 更新 `common/runtime/config` 测试，覆盖默认值、`AEGISCORE_RUNTIME_GIN_MODE`、`AEGISCORE_OBSERVABILITY_PPROF_ENABLED`、`AEGISCORE_OBSERVABILITY_PPROF_ADDR`、非法 Gin mode、非法 pprof 地址和生产类环境非 loopback 地址。

## 2. user-service 配置与 pprof lifecycle

- [x] 2.1 更新 `user-service/configs/config.yaml`，新增 `runtime.gin.mode` 和 `observability.pprof` 示例配置及简短说明。
- [x] 2.2 调整 `user-service/internal/bootstrap/pprof.go`，让 `NewPprofServer` 只读取 `params.Config.Observability.Pprof`，删除 `os.LookupEnv`、裸 `PPROF_*` 常量和环境读取 helper。
- [x] 2.3 保留或迁移 pprof 地址解析与 loopback helper 到配置校验归属，避免 `NewPprofServer` 重复执行配置加载阶段已完成的校验。
- [x] 2.4 更新 `user-service/internal/bootstrap/pprof_test.go`，使用构造的 `config.Config` 表达启停和地址，不再通过 `t.Setenv(PPROF_*)` 驱动 pprof provider 行为。

## 3. Gin mode 显式初始化

- [x] 3.1 新增显式 Gin mode 配置 provider 和 `GinModeConfigured` marker，基于 `params.Config.Runtime.Gin.Mode` 调用一次 `gin.SetMode`。
- [x] 3.2 将 Gin mode provider 接入 user-service bootstrap/provider graph，并让 `GinParams` 显式依赖 `GinModeConfigured`，确保 Fx graph 表达 Gin mode 先于 `NewGinEngine` 构造完成。
- [x] 3.3 删除 `NewGinEngine` 中的 `gin.SetMode(gin.ReleaseMode)`，保持 engine constructor 只创建 engine、设置可信代理和安装 middleware。
- [x] 3.4 更新相关 Gin provider 测试，验证 `NewGinEngine` 不再覆盖已有 Gin mode，并验证正式 graph 会按配置设置 mode。

## 4. OpenSpec 与文档同步

- [x] 4.1 确认本 change 的 `proposal.md`、`design.md`、`tasks.md` 和 `specs/*/spec.md` 与最终实现一致。
- [x] 4.2 运行 `make user-service-architecture-lint`，验证架构边界、OpenSpec 和文档约束。

## 5. 验证与收尾

- [x] 5.1 运行 `go test ./runtime/config`（workdir: `common`），验证共享配置加载和校验。
- [x] 5.2 运行 `go test ./internal/bootstrap ./internal/providers ./internal/config`（workdir: `user-service`），验证 pprof、Gin mode 和服务配置。
- [x] 5.3 运行 `make test`，验证仓库相关测试集合。
- [x] 5.4 将本次预期代码、配置、测试和 OpenSpec 工件变更加到暂存区。
- [x] 5.5 运行 `make lint`，失败时修复后重新运行。
- [x] 5.6 运行 `make verify`，失败时修复后重新运行；不得在验证失败或未运行时标记本 change 完成。
