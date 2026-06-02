## 1. 命名审查与风险分类

- [x] 1.1 扫描 `common/`、`user-services/`、`docs/`、`openspec/specs/` 和当前 `openspec/changes/` 中的目录名、文件名、Go 包名、类型名、函数名、方法名、变量名和 capability 名。
- [x] 1.2 将候选命名问题按外部契约、公共 Go API、内部 Go API、文档/规格表达、工具链或迁移历史分类，并记录所属 capability 与风险。
- [x] 1.3 确认不纳入本次改名的高风险名称，包括 `user-services` module path、HTTP 路由、JSON 字段、响应码数值、`X-Trace-ID`、配置 key、环境变量、Swagger import path 和已存在 migration 文件名。

## 2. 低风险代码命名标准化

- [x] 2.1 重命名可控的内部 Go 类型、函数、方法、参数或 helper，使其语义更清晰且符合 Go 命名风格。
- [x] 2.2 同步更新所有 Go 引用、imports、测试引用和文档引用，确保 controller/service/repository 分层职责不变。
- [x] 2.3 对修改过的 Go 文件运行 `gofmt`，不修改 `user-services/ent/` 生成代码。

## 3. 文档与规格命名一致性

- [x] 3.1 更新 `docs/opsx/CAPABILITY_MAP.md`，使 capability 列表、状态和 future candidates 与当前 `openspec/specs/` 保持一致。
- [x] 3.2 统一 OpenSpec 和文档中的响应码命名表达，明确对外响应 `code` 仍为当前数字枚举。
- [x] 3.3 补充或修正 trace-id 边界命名说明，保持 `X-Trace-ID`、`trace_id` 和 `trace-id` 的既有契约不变。
- [x] 3.4 记录未来 migration 文件名应使用清晰语义名称，但不重命名既有 migration 文件或修改 `atlas.sum`。

## 4. 验证与结果输出

- [x] 4.1 在 `common/` 运行 `go test ./...`，修复因命名变更导致的编译或测试失败。
- [x] 4.2 在 `user-services/` 运行 `go test ./...`，确认 HTTP runtime、用户 profile、认证、validation、Swagger 和 migration 相关测试不因命名变更失败。
- [x] 4.3 复查 `openspec status --change review-and-standardize-names`，确认变更 artifacts 状态正常。
- [x] 4.4 输出修改结果清单，逐项说明每处命名修改的原因、影响范围、所属 capability，以及保留高风险名称的原因。
