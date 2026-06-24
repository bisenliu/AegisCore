## 1. 公共 Fx 图能力

- [x] 1.1 在 `common/runtime/fxgraph` 或等价 runtime primitive 包中新增业务中立的 Fx 依赖图生成 API。
- [x] 1.2 实现 DOT 文本输出和文件写入能力，确保输出不包含 user-service 业务语义。
- [x] 1.3 为公共 helper 添加单元测试，覆盖重复生成稳定输出、无效输出路径和基础 DOT 内容。

## 2. user-service 生成入口

- [x] 2.1 在 user-service 自有边界新增薄生成入口或脚本，组装 `bootstrap.AppModule` 并调用 `common` helper。
- [x] 2.2 确定并生成提交用 DOT 文件位置，例如 `user-service/docs/fx-dependency-graph.dot`。
- [x] 2.3 确保生成入口不启动 HTTP server lifecycle、不连接真实 PostgreSQL/Redis，也不改变服务运行时 provider 行为。

## 3. 交付命令

- [x] 3.1 在 `user-service/Makefile` 新增服务内 Fx 依赖图生成目标和中文 help 文案。
- [x] 3.2 在仓库根 `Makefile` 新增 `user-service-` 前缀目标，委托 user-service 目标。
- [x] 3.3 评估是否把生成目标纳入 `verify`；若不纳入，提供专用 check 目标或明确通过 `git diff --exit-code` 检查 drift。

## 4. 验证

- [x] 4.1 执行公共 helper 相关 Go 测试，例如 `make common-test` 或精确包测试。
- [x] 4.2 执行 user-service 相关 Go 测试，例如 `make user-service-test` 或精确包测试。
- [x] 4.3 执行 `make user-service-architecture-lint`，确认架构边界和 OPSX 文档约束通过。
- [x] 4.4 执行 Fx 依赖图生成命令，并通过 `git diff --exit-code` 或专用 check 命令确认生成物 drift 可见。
