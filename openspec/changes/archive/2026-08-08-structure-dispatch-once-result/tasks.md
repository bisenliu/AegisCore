## 1. 结构化返回类型与流程拆分

- [x] 1.1 在 permission application 内定义 `DispatcherDispatchResult`、结构化错误和必要的私有结果类型。
- [x] 1.2 将 `DispatchOnce` 拆分为 claim batch、dispatch batch、refresh backlog/status、finalize result 私有步骤。
- [x] 1.3 适配后台 `run` 循环和现有直接调用方到新的 `DispatchOnce(ctx) (DispatcherDispatchResult, error)` 签名。

## 2. 单元测试覆盖

- [x] 2.1 更新成功投递、部分 publish 失败、ack 失败和 claim lost 测试，断言结构化计数与错误分类。
- [x] 2.2 新增 claim 失败、backlog/status 失败和 `ctx` canceled 的结构化返回语义测试。
- [x] 2.3 扩展 fake store 支持 claim、ack、fail、backlog 错误注入，不改变生产 store 端口。

## 3. 验证与收尾

- [x] 3.1 运行 `openspec validate structure-dispatch-once-result --strict`。
- [x] 3.2 运行 `go test ./user-service/internal/features/permission/application -run 'TestDispatcherDispatchOnce|TestDispatcherRecords|TestDispatcherCancellation|TestDispatcherStatus'`。
- [x] 3.3 运行 `make user-service-architecture-lint`。
- [x] 3.4 暂存本次预期代码和 OpenSpec 变更后运行 `make lint`。
- [x] 3.5 暂存本次预期代码和 OpenSpec 变更后运行 `make verify`，并排除 runtime 自动文件导致的工作区噪声。
