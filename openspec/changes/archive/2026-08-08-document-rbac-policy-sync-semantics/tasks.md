## 1. 统一规格基线

- [x] 1.1 对照 dispatcher、watcher、subscriber 和 enforcer 相关 change，确认 `rbac-access-control` delta 完整覆盖 batch partial success、lifecycle root context、watcher final state、panic observability 与 race/stress 语义
- [x] 1.2 检查统一 delta 未新增 capability、未保留旧无参 `Start()`、background root、error-only `DispatchOnce` 或单 waiter 控制共享 reload flight 的兼容行为
- [x] 1.3 确认本 change 仅包含 `openspec/changes/document-rbac-policy-sync-semantics/` 下的规格产物，未修改 Go 代码、测试、API、数据库、部署或生成物

## 2. 规格与架构验证

- [x] 2.1 运行 `openspec validate document-rbac-policy-sync-semantics --strict` 并修复所有 proposal、design、spec delta 和 tasks 校验错误
- [x] 2.2 运行 `make user-service-architecture-lint`，确认文档与架构边界检查通过
- [x] 2.3 检查 `git status --short -- openspec/changes/document-rbac-policy-sync-semantics`，确认本次变更只包含该 change 目录
- [x] 2.4 检查 `git diff -- openspec/changes/document-rbac-policy-sync-semantics`，确认没有 Go 代码、测试、API、数据库、部署或生成物 drift
