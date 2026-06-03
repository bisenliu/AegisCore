## 1. Feedback Drafting

- [x] 1.1 整理 `user-services/internal/errmsg/` 包命名评审反馈，按“问题说明、原因分析、建议改法”输出中文文本。
- [x] 1.2 整理 `common/infrastructure/` 目录组织评审反馈，按“问题说明、原因分析、建议改法”输出中文文本。
- [x] 1.3 在包命名反馈中明确 Go package name 应短小、全小写、语义明确，并给出 `errmsg` 等符合规范的建议。
- [x] 1.4 在基础设施目录反馈中说明单目录平铺的可维护性风险，并给出按基础设施类型或职责分层的目录组织示例。

## 2. Consistency Review

- [x] 2.1 检查反馈文本不得承诺改变 HTTP API、错误码、响应信封、配置 key、数据库 schema 或运行时行为。
- [x] 2.2 检查基础设施目录建议明确保留 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例和 Fx named injection 行为。
- [x] 2.3 检查反馈文本与 `project-naming-consistency` 和 `shared-infrastructure` 两个 capability 的规格要求一致。

## 3. Validation

- [x] 3.1 运行 OpenSpec 状态检查，确认 proposal、design、specs 和 tasks 均已完成且 change apply-ready。
- [x] 3.2 如后续实现涉及代码或文档文件落地，运行相关格式化、测试或文档校验命令。
