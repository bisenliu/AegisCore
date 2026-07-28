## ADDED Requirements

### Requirement: Helm 生产镜像不可变发布

user-service Helm 生产发布 MUST 使用不可变镜像引用。Deployment、RBAC seed Job、安全扫描、SBOM、镜像身份记录和 Helm release MUST 指向同一个已构建并已推送的 user-service 镜像工件。生产 Helm chart MUST NOT 默认渲染 `:latest`，也 MUST NOT 通过 `Chart.appVersion`、空 tag fallback、环境 values 或命令行覆盖接受 `latest` 作为发布镜像。

#### Scenario: 默认 Helm 渲染禁止 latest

- **WHEN** 渲染 user-service Helm chart 的生产基线 values
- **THEN** chart MUST 要求调用方显式提供不可变 image ref
- **AND** 渲染结果 MUST NOT 包含 `image: *:latest`
- **AND** 缺少 image ref 时 Helm lint 或 Helm template MUST 失败

#### Scenario: 显式 latest 覆盖被拒绝

- **WHEN** 发布方通过 values 文件或 `--set` 将 user-service 镜像设置为 `latest` tag
- **THEN** Helm lint 或 Helm template MUST 失败
- **AND** 系统 MUST NOT 生成 Deployment 或 RBAC seed Job manifest

#### Scenario: 同一发布工件贯穿 CI 与 Helm

- **WHEN** CI/CD 构建准备发布的 user-service 镜像
- **THEN** pipeline MUST 推送 `sha-<commit>` tag 或解析 registry digest，并记录该镜像身份
- **AND** 漏洞扫描、SBOM、镜像内容断言和 Helm 发布 MUST 使用同一 image ID、digest 或等价不可变引用
- **AND** Deployment 与 RBAC seed Job MUST 使用完全相同的不可变 image ref

#### Scenario: 回滚使用不可变历史引用

- **WHEN** Helm release 需要回滚 user-service 镜像
- **THEN** 发布方 MUST 回退到上一版已记录的不可变 image ref
- **AND** 回滚流程 MUST NOT 将镜像改回 `latest` 或依赖 registry 当前 tag 指向
