## 1. Scheduler 默认值与执行流程

- [x] 1.1 在 `common/runtime/scheduler` 中新增包内私有常量 `defaultLockUnlockTimeout` 和 `defaultLockRenewTimeout`，并让 `executor.go`、`renew.go`、`validation.go` 统一引用。
- [x] 1.2 将 `runJob()` 的运行状态收敛到私有状态结构，保持原有 cleanup 顺序和错误记录语义。
- [x] 1.3 将本地 overlap gate、全局并发 gate 和分布式锁获取逻辑拆分为私有函数，保持 skip reason、metrics 和日志不变。
- [x] 1.4 将任务 context/续租准备、执行后 cleanup 和结果记录拆分为私有函数，保持 panic、失败、成功和未启动路径语义不变。
- [x] 1.5 补充或调整 scheduler 测试，覆盖锁释放默认超时、续租默认超时、跳过、失败、panic、续租失败和 gate 归还行为。
- [x] 1.6 运行 `rg '5\s*\*\s*time\.Second' common/runtime/scheduler`，确认目标内联默认值不再残留。

## 2. PostgreSQL 测试容器 helper

- [x] 2.1 在 `common/testing/containers/postgres.go` 中新增 Docker mapped port 探测间隔私有常量，替换端口探测循环中的 `time.Sleep(100 * time.Millisecond)`。
- [x] 2.2 确认 PostgreSQL 测试容器启动超时、Docker 命令、mapped port 解析和 readiness probe 语义保持不变。
- [x] 2.3 运行 `rg '100\s*\*\s*time\.Millisecond' common/testing/containers/postgres.go`，确认目标内联探测间隔不再残留。

## 3. common 模块依赖整理

- [x] 3.1 在 `common` 目录运行 `GOWORK=off go mod why -m github.com/quic-go/quic-go github.com/swaggo/swag gopkg.in/yaml.v2 go.mongodb.org/mongo-driver/v2 go.yaml.in/yaml/v2`，记录哪些依赖仍有真实导入链。
- [x] 3.2 在 `common` 目录运行 `GOWORK=off go mod tidy`，只提交 Go 工具链实际整理出的 `common/go.mod` 和 `common/go.sum` 变更。
- [x] 3.3 运行 `GOWORK=off go mod tidy -diff`，确认 `common` 模块无剩余 tidy diff。
- [x] 3.4 不手工删除 `user-service` 中由 Gin、Swagger UI、Prometheus 或其他真实导入链需要的间接依赖。

## 4. 验证与收尾

- [x] 4.1 运行 `go test ./runtime/scheduler ./testing/containers` 于 `common` 目录，确认相关包测试通过。
- [x] 4.2 运行 `make common-test`，确认 common 模块测试通过。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认 OpenSpec 和架构边界检查通过。
- [x] 4.4 将本次预期代码、依赖和 OpenSpec artifact 变更加到暂存区。
- [x] 4.5 运行 `make lint`，确认 lint 通过。
- [x] 4.6 运行 `make verify`，确认完整验证通过且无未暂存预期变更导致 drift 检查失败。
