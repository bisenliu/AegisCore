## ADDED Requirements

### Requirement: user-service 部署终止预算一致性

user-service 的原生 Kubernetes manifest 与 Helm 默认 values MUST 使用一致的 `terminationGracePeriodSeconds`，且该值 MUST 大于默认 `runtime.lifecycle.stop_timeout`。默认部署 grace MUST 至少等于 Fx Stop 总预算加 30 秒平台余量；平台余量 MUST 用于覆盖 kubelet 进程调度、信号传递、受控 `preStop` 和网络抖动，不得以单个组件的 shutdown timeout 替代 Fx 全部逆序串行 `OnStop` hook 的总预算。

仓库 MUST 提供可由 CI 执行的结构化配置一致性测试，读取 user-service 默认配置、原生 Kubernetes Deployment 和 Helm 默认 values，并在预算不足、字段缺失、值无效或两个部署入口漂移时稳定失败。

#### Scenario: 默认部署预算覆盖应用与平台余量

- **WHEN** user-service 默认 `runtime.lifecycle.stop_timeout` 为 120 秒
- **THEN** 原生 Kubernetes 与 Helm 默认 `terminationGracePeriodSeconds` MUST 同为 150 秒
- **AND** 默认 deployment grace MUST 满足 `150s >= 120s + 30s`
- **AND** 正常关闭提前完成时 Pod MUST NOT 等待完整 150 秒才退出

#### Scenario: 拒绝小于约束值的 grace

- **WHEN** 配置一致性测试发现任一默认 `terminationGracePeriodSeconds` 小于默认 Fx Stop 总预算加 30 秒平台余量
- **THEN** 测试 MUST 失败并指出不满足约束的部署入口及预算值

#### Scenario: 检测原生 Kubernetes 与 Helm 漂移

- **WHEN** 原生 Kubernetes manifest 与 Helm values 的默认 `terminationGracePeriodSeconds` 不一致
- **THEN** 配置一致性测试 MUST 失败
- **AND** 测试 MUST NOT 通过正则或未解析的文本片段把注释、其他资源或同名无关字段误认为目标值

#### Scenario: 变更应用预算或 preStop

- **WHEN** 协作者调整默认 `runtime.lifecycle.stop_timeout` 或新增、延长 `preStop`
- **THEN** 同一变更 MUST 重新核对 30 秒平台余量是否仍能覆盖平台阶段
- **AND** 若余量不足，原生 Kubernetes 与 Helm 默认 grace、自动校验和预算说明 MUST 同步提高

#### Scenario: 发布期间优雅终止

- **WHEN** Kubernetes 因滚动发布、缩容、驱逐或故障退出终止 user-service Pod
- **THEN** 默认部署 MUST 为进程提供完成 Fx Stop 全链路及平台余量的时间
- **AND** kubelet MUST 只在 termination grace 耗尽且进程仍未退出时执行强制终止

#### Scenario: 保持应用和数据契约

- **WHEN** 仅对齐 user-service 部署终止预算并增加一致性门禁
- **THEN** 系统 MUST NOT 改变业务 API、OpenAPI、认证/RBAC 语义、数据库 schema、Atlas migration 或 Fx `OnStop` 逆序串行语义
