## Context

根 `make test` 已覆盖 `common`、`user-service` 和两个工具 module，但 `make lint` 仅调用前两个 module。CI 复用质量 workflow 执行根 lint，因此同样漏掉工具；主 CI 的 `govulncheck` 和 `gosec` matrix 也只列出两个一层目录。`gosec` 当前输出路径依赖 `../security`，直接把二层工具路径加入 matrix 会把报告写到 `tools/security`，并且斜杠不适合作为 artifact 名称。

两个工具的 `run` 都接收 `io.Writer` 以便测试，但成功消息使用未检查的 `fmt.Fprintf`。失败诊断同样未显式处理 writer error。成功 writer 失败会错误返回 `exitOK`；失败诊断 writer 失败虽然不应改变既有非零状态，但必须显式表达该语义以通过 `errcheck`。

`tools/openapi-convert` 直接复用 `common/http/openapi`，其 `go.mod` 使用不可从远端解析的占位版本 `github.com/aegiscore/common v0.0.0`。Go workspace 会替换该依赖，所以日常 build 正常；`GOWORK=off go mod tidy` 则失败。仓库中的 `user-service` 已通过相邻目录 `replace` 表达相同源码依赖边界。

## Goals / Non-Goals

**Goals:**

- 四个 Go module 执行同一根 golangci 配置，并全部进入 `govulncheck` 与 `gosec`。
- 工具成功输出不可写时稳定返回非零退出码，失败路径继续稳定返回非零退出码。
- 四个 module 在完整仓库 checkout 中均可执行 `GOWORK=off go mod tidy -diff`，且不产生 metadata drift。
- CI job、SARIF 文件和 artifact 名称保持稳定、无路径分隔符，并从任意深度 module 写到仓库根安全目录。

**Non-Goals:**

- 不把工具纳入 race 或 coverage matrix；根测试入口已覆盖其普通单测，本 change 只修复报告指出的 lint 与安全门禁缺口。
- 不发布 `common` 的远端 `v0.0.0` 版本，也不承诺从仓库外单独获取或构建 `tools/openapi-convert`。
- 不改变工具 flag、正常 stdout 文本、既有 stderr 文本、生成文件格式或 Nacos 发布协议。
- 不修改业务代码、HTTP API、数据库 migration、OpenAPI 生成物、部署或观测资产。

## Decisions

### Decision: 根 Makefile 显式聚合四个 module lint

两个工具 Makefile 各增加 `lint`，统一执行 `golangci-lint run ./...`。根 Makefile 增加带工具上下文的 `tools-openapi-convert-lint` 与 `tools-nacos-config-seed-lint`，并把它们加入 `lint` 依赖。

不在仓库根直接运行一次 `golangci-lint run ./...`，因为根目录不是 Go module，显式 module 入口也与现有测试和 lint 组织一致。

### Decision: 安全 matrix 分离稳定名称与工作目录

`govulncheck` 和 `gosec` matrix 使用 `include` 项，分别声明无斜杠的 `name` 与真实 `path`。job 和 artifact 使用 `name`，`working-directory` 使用 `path`。`gosec` 报告直接输出到 `${{ github.workspace }}/security`，避免对 module 目录深度作假设。

不直接把 `tools/openapi-convert` 字符串追加到现有标量 matrix，因为它会污染 artifact 名称，并破坏相对报告路径。

### Decision: 成功输出错误决定退出状态，失败诊断保持原错误状态

两个工具在写成功消息时检查 `fmt.Fprintf` 返回值；写入失败后尝试向 stderr 输出带上下文的诊断并返回 `exitError`。`failf` 仍服务于已经确定失败的路径，显式丢弃诊断 writer error，因为调用方此时已经返回非零状态，且没有第三条可靠输出通道。

不扩大 `run` 的返回类型或引入新的 writer abstraction；现有 `int` 退出码契约足以准确表达进程结果，测试可用最小失败 writer 覆盖异常路径。

### Decision: openapi-convert 使用仓库相邻 common replace

在 `tools/openapi-convert/go.mod` 增加 `replace github.com/aegiscore/common => ../../common`。这定义该工具可在完整仓库 checkout 中关闭 workspace 后独立执行 Go module 命令，与 `user-service` 的仓库内源码依赖策略一致。

不复制 OpenAPI 实现到工具 module，也不虚构可发布的 `common` 版本。相对 `replace` 意味着仓库目录结构仍是该内部交付工具的正式前提。

### Decision: 用 GOWORK=off 验证每个 module metadata

对四个 module 分别执行 `GOWORK=off go mod tidy` 并提交结果，验证时再执行 `GOWORK=off go mod tidy -diff`。这样能发现被 workspace 隐藏的依赖解析和 checksum 缺口。

## Risks / Trade-offs

- [Risk] 扩大 lint 和安全扫描会增加 CI 时长 → Mitigation：沿用并行 matrix 与 Go cache，工具 module 依赖规模较小。
- [Risk] 相对 `replace` 让 `openapi-convert` 无法脱离仓库目录独立发布 → Mitigation：该工具本就复用未发布的内部 `common`，规格明确其支持边界是完整仓库 checkout。
- [Risk] stderr 本身失败时无法打印 stdout 写失败诊断 → Mitigation：进程仍返回非零退出码；不存在可依赖的第三输出通道。
- [Trade-off] 没有把工具加入 race/coverage → 保持本 change 聚焦已确认的质量和安全缺口，工具普通测试继续由根 `make test` 执行。

## Migration Plan

1. 合并工具 lint、输出错误处理测试、CI matrix 和 module metadata。
2. CI 自动开始生成四个 module 的 lint、`govulncheck` 与 `gosec` 结果；不需要运行时迁移或部署顺序调整。
3. 若新增门禁暴露依赖漏洞或静态问题，修复对应 module 后再合并，不提供跳过工具扫描的兼容开关。
4. 回滚时整体回退本 change；不会影响已部署服务或数据，但会恢复原有工具质量覆盖缺口。

## Open Questions

无。

## Verification

- `GOWORK=off go mod tidy -diff`（分别在四个 module 中执行）
- `go test ./tools/openapi-convert ./tools/nacos-config-seed`
- `make tools-openapi-convert-lint tools-nacos-config-seed-lint`
- `make user-service-architecture-lint`
- `openspec validate harden-repository-tool-quality`
- 预期变更暂存后运行 `make lint` 和 `make verify`
