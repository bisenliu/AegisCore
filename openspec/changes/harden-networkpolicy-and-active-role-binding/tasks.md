## 1. 部署网络策略

- [x] 1.1 更新 `deployments/k8s/user-services/networkpolicy.yaml`，将 ingress 默认来源改为明确 `namespaceSelector` 与 `podSelector` 组合，并移除 `namespaceSelector: {}` 的生产默认用法。
- [x] 1.2 更新 `deployments/k8s/user-services/networkpolicy.yaml`，将 PostgreSQL、Redis、OTLP Collector egress 拆分为带 `to` 目的地约束的独立规则，保留 DNS 目的地约束。
- [x] 1.3 更新 `deployments/helm/aegiscore-user-services/values.yaml`，使默认 `networkPolicy.ingress` 与 `networkPolicy.egress` 符合显式来源和显式目的地约束。
- [x] 1.4 新增或更新 deployments 下的 admission policy 资产或说明，限制 user-service 网络准入标签只能由受信任 namespace 或受控 workload 使用。
- [x] 1.5 更新 `deployments/helm/aegiscore-user-services/README.md` 或环境覆盖示例，说明外部 PostgreSQL、Redis、OTLP Collector 必须通过精确 `ipBlock` 或等价目的地覆盖，不得删除 `to` 恢复任意目的端口放行。

## 2. RBAC 用户角色绑定

- [x] 2.1 在 role domain 中新增明确的停用角色绑定错误，并确保错误命名和注释符合现有领域错误风格。
- [x] 2.2 更新 `roleCommandService.AddUserRole`，在写入前拒绝 `Active=false` 的角色，并确保拒绝时不调用 `UserRoleStore.Add`、不发送 policy change 通知。
- [x] 2.3 更新 `roleCommandService.ReplaceUserRoles`，在写入前检查所有目标角色均启用，任一停用时整体拒绝，并确保旧绑定保持不变且不发送 policy change 通知。
- [x] 2.4 更新 HTTP 错误映射或 role transport 错误处理，使停用角色绑定返回明确客户端错误，而不是内部错误。

## 3. 测试与验证覆盖

- [x] 3.1 为 `role/application/command` 增加 AddUserRole 绑定停用角色的单元测试，断言写入和通知均未发生。
- [x] 3.2 为 `role/application/command` 增加 ReplaceUserRoles 包含停用角色的单元测试，断言替换和通知均未发生。
- [x] 3.3 为 role HTTP transport 或错误映射增加测试，覆盖停用角色绑定错误的 HTTP 响应语义。
- [x] 3.4 运行相关 Go 包测试：`go test ./user-service/internal/features/role/application/command ./user-service/internal/features/role/transport/http`。
- [x] 3.5 运行 Helm 验证：`helm lint deployments/helm/aegiscore-user-services` 和 `helm template aegiscore-user-services deployments/helm/aegiscore-user-services`，检查渲染结果不包含任意 namespace 入站放行或无 `to` 的业务 egress 规则。

## 4. OpenSpec 与交付验证

- [x] 4.1 运行 `openspec status --change "harden-networkpolicy-and-active-role-binding"`，确认 proposal、design、specs 和 tasks 均完成且可 apply。
- [x] 4.2 运行 `make user-service-architecture-lint`，确认 OpenSpec 和文档语言、架构边界检查通过。
- [x] 4.3 完成全部实现、规格和文档任务后，先将本次预期代码和文档变更加到暂存区。
- [x] 4.4 暂存后运行 `make lint`，失败时修复后重新运行。
- [x] 4.5 暂存后运行 `make verify`，用最终 drift 检查暴露未同步生成物或意外变更。
