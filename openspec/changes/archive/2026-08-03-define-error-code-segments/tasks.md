## 1. 规格与注释实现

- [x] 1.1 核对 `openspec/changes/define-error-code-segments/specs/shared-platform-primitives/spec.md` 的错误码段规则与 `design.md` 决策一致。
- [x] 1.2 更新 `common/contract/errors/code.go` 顶部注释，明确 `0`、`10xxx`、`20xxx`、`30xxx`、`40xxx`、`50xxx`、`60xxx`、`70xxx-89xxx` 和 `90xxx` 的语义边界。
- [x] 1.3 在注释中明确 `Code` 不驱动 HTTP status，HTTP status 只由 `Kind` 推导；新增 `Kind` 必须同步 `common/http/response.statusCode` 和响应测试。
- [x] 1.4 确认不新增错误码常量、不修改现有错误码数值、不引入运行时校验或兼容分支。

## 2. 验证

- [x] 2.1 运行 `go test ./common/contract/errors ./common/http/response`，确认现有错误构造和 HTTP 映射仍通过。
- [x] 2.2 运行 `make user-service-architecture-lint`，确认 OpenSpec 与架构约束通过。
- [x] 2.3 将本次预期代码和 OpenSpec 变更加到暂存区。
- [x] 2.4 运行 `make lint`，确认 lint 通过。
- [x] 2.5 运行 `make verify`，确认完整验证通过且没有未暂存预期 diff 阻塞。
