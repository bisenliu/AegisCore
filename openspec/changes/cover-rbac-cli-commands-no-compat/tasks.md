## 1. RBAC CLI 可测试性

- [x] 1.1 在 `user-service/cmd/rbac.go` 中引入最小未导出依赖工厂 seam，使测试可注入 RBAC seed、用户创建、凭据和 password fake dependency。
- [x] 1.2 保持 `runRBACSeedCommand`、`runAssignSuperAdminCommand`、`runCreateSuperAdminCommand` 和 `createSuperAdmin` 的外部业务语义不变，不新增旧 CLI alias、旧 flag、旧 env 或旧 Make target 兼容路径。

## 2. RBAC CLI 命令测试

- [x] 2.1 在 `user-service/cmd` 同包测试中覆盖 `runRBACSeedCommand` 的成功、seed error、依赖初始化 error 和 cleanup error 合并语义。
- [x] 2.2 覆盖 `runAssignSuperAdminCommand` 的新增绑定、已存在绑定、服务错误、依赖初始化错误和 cleanup 执行语义。
- [x] 2.3 覆盖 `runCreateSuperAdminCommand` 的成功输出、`createSuperAdmin` 错误传播、依赖初始化错误和 cleanup 错误合并语义。
- [x] 2.4 覆盖 `createSuperAdmin` 的不存在用户创建、已存在用户绑定、已存在用户重置密码、hash 错误、凭据更新错误和角色绑定错误。
- [x] 2.5 覆盖 `normalizeCreateSuperAdminOptions`、`normalizeUsername` 和 `chainCleanup` 的当前归一化、缺失参数和执行顺序语义。
- [x] 2.6 调整既有命令测试断言，使 CLI 错误、参数缺失、依赖初始化和 cleanup 断言优先使用 `require` 及更具体语义化断言。

## 3. 验证和收尾

- [x] 3.1 运行 `go test -cover ./user-service/cmd`，确认 RBAC CLI 相关函数不再全部为 0%。
- [x] 3.2 运行 `openspec validate cover-rbac-cli-commands-no-compat`。
- [x] 3.3 运行 `make user-service-architecture-lint`。
- [x] 3.4 暂存本次预期源码、测试和 `openspec/changes/cover-rbac-cli-commands-no-compat/` 产物，排除 Multica runtime 文件。
- [x] 3.5 运行 `make lint` 和 `make verify`；如果被非本次文件或 runtime 文件阻塞，记录原因并只在验证可归因通过后标记完成。
