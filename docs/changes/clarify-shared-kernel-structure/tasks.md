# Tasks

- [x] 检查 `deployments/k8s/user-services/` 当前内容，确认只有 README 边界说明，没有 Kubernetes YAML 或业务规格。
- [x] 检查 `user-service/internal/shared/identity/status.go`，确认文件只承载 `UserStatus`、解析和账号生命周期判断。
- [x] 检查 `user-service/internal/shared/` 当前子包，确认只有 `identity` 和 `rbacbaseline` 两个已登记共享内核。
- [x] 形成迁移结论：`deployments/k8s/user-services/` 当前没有内容需要迁移到 `internal/shared`。
- [x] 形成命名结论：将 `identity/status.go` 重命名为 `identity/user_status.go`，为未来公共枚举建立 subject-specific 文件命名规则。
- [x] 形成结构结论：不按 feature 模块对 `internal/shared` 重新分类，继续按稳定共享业务内核子域分类。
- [x] 更新 `AGENTS.md`，补充部署资产不得迁入 `internal/shared`，并禁止根级 `shared/errors`、`shared/enums`、`shared/types`、`shared/utils`、`shared/helpers` 兜底包。
- [x] 更新 `docs/ARCHITECTURE.md`，补充 Kubernetes/deployments 与 shared kernel 的边界，以及公共错误、枚举和值对象的放置规则。
- [x] 将 `user-service/internal/shared/identity/status.go` 重命名为 `user-service/internal/shared/identity/user_status.go`。
- [x] 将 `user-service/internal/shared/identity/status_test.go` 重命名为 `user-service/internal/shared/identity/user_status_test.go`。
- [x] 新增 `user-service/internal/shared/README.md`，记录 shared 目录和文件命名规则。
- [x] 新增 `identity/doc.go` 与 `rbacbaseline/doc.go`，让 shared 子包自描述职责。
- [x] 更新 `user-service/scripts/architecture-lint.sh`，拦截根级 shared 兜底包和旧 `identity/status.go` 文件名。
- [x] 运行 `make architecture-lint`。
