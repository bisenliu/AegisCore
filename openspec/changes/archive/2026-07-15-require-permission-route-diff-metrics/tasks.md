## 1. 收紧 Permission 查询服务依赖

- [x] 1.1 将 `NewPermissionQueryService` 的 Metrics 参数改为必选单值 `permissionapplication.Metrics`，删除 variadic 选择和构造器内部 no-op fallback，并更新中文构造器注释。
- [x] 1.2 更新 permission application 内全部直接构造调用点：需要观察指标的测试传入 mock/spy，其余测试显式传入 `permissionapplication.NopMetrics()`。
- [x] 1.3 检查 `features/permission/fx.go` 的正式 provider 接线，确保 `newPermissionMetrics` 是唯一单值 Metrics 输出，query service 不使用 variadic、optional 或 slice/group annotation，且不重组无关 provider。

## 2. 固定正式 Fx 模块行为

- [x] 2.1 增加基于正式 `permission.Module` 的测试，按 `permissionapplication.Metrics` 精确接口注入 spy，从容器取得 `PermissionQueryService`，执行 route diff 并断言 `RouteDiffObserved` 收到准确的 missing、stale 数量。
- [x] 2.2 在正式 module 测试中取得并检查 Fx/DOT 图，证明 `PermissionQueryService` 构造节点存在明确的 `permissionapplication.Metrics` 输入边，且测试未通过手工构造 query service 绕过生产接线。
- [x] 2.3 覆盖 metrics 启用和禁用配置的正式构图：启用时使用现有 Prometheus recorder 并更新既有 route diff metric，禁用时使用现有 `NopMetrics()` 且不注册 permission Prometheus 指标。
- [x] 2.4 运行 `cd user-service && go test ./internal/features/permission/... -count=1`，确认 permission feature 全部测试通过。

## 3. 规格与架构验证

- [x] 3.1 复核实现与 `rbac-access-control`、`runtime-observability` delta 一致，并确认未修改 route diff 判定、HTTP response、权限目录、Casbin policy、指标名称、backend、dashboard 或无关 Fx provider。
- [x] 3.2 运行 `openspec validate require-permission-route-diff-metrics`，确认 proposal、design、两份 spec delta 和 tasks 均通过 OpenSpec 校验。
- [x] 3.3 运行 `make user-service-architecture-lint`，确认 feature 分层、Metrics 归属和 Fx provider 边界符合仓库规则。

## 4. 最终交付门禁

- [x] 4.1 审查工作区 diff，仅将本 change 的预期代码、测试和 OpenSpec 文件加入暂存区，并运行 `git diff --cached --check` 确认暂存内容无格式问题。
- [x] 4.2 在预期变更已暂存后运行 `make lint`；只有命令通过后才完成本任务。
- [x] 4.3 在预期变更已暂存后运行 `make verify`，确认全仓测试、生成物 drift 与最终 `git diff --exit-code` 门禁均通过；只有命令通过后才完成本任务和 change。
