## 1. Response Package Refactor

- [x] 1.1 拆分 `common/response/response.go`，将响应信封和 Gin 成功/失败 helper 保持在聚焦文件中。
- [x] 1.2 将分页类型、分页参数规范化和分页数据构造移动到独立文件，并保持导出 API 与 JSON 结构不变。
- [x] 1.3 将标准响应消息常量和错误响应 helper 组织到职责明确的文件中，保持业务码、HTTP status 和对外消息不变。
- [x] 1.4 运行 `gofmt` 格式化 `common/response` 相关 Go 文件。

## 2. Bootstrap Responsibility Split

- [x] 2.1 拆分 `user-services/internal/bootstrap/bootstrap.go`，将 `NewApp` 和 `UserServiceModule` 移动到 `app.go` 或 `fx.go` 等职责命名文件，并移除泛化的 `bootstrap.go` 聚合文件。
- [x] 2.2 将 Gin engine 创建、中间件注册和 trusted proxies 配置移动到 HTTP engine 聚焦文件，保持中间件顺序不变。
- [x] 2.3 将 JWT service provider 与路由注册 wiring 移动到聚焦文件，保持认证配置、logger、controller 和 token version validator 注入不变。
- [x] 2.4 将 HTTP server 创建和 Fx lifecycle hook 移动到 server 聚焦文件，保持 timeout、graceful shutdown 和 `http.ErrServerClosed` 处理不变。
- [x] 2.5 保持 Redis/PostgreSQL/Ent runtime wiring 在 bootstrap 边界显式表达，不新增未声明数据源连接。
- [x] 2.6 运行 `gofmt` 格式化 `user-services/internal/bootstrap` 相关 Go 文件。

## 3. Ent Schema Domain Classification

- [x] 3.1 在 `user-services/ent/schema/` 下增加用户领域分类路径，并将当前 `User` schema 实现移动到该分类中。
- [x] 3.2 在根 `schema` package 保留 Ent codegen 和 Atlas source 可读取的聚合入口，继续暴露 `User` schema。
- [x] 3.3 确认本次变更未修改 Ent `User` 字段、索引或注释语义，且不生成新的 Atlas migration。

## 4. Verification

- [x] 4.1 在 `common/` 运行 `go test ./...`，验证响应包拆分后编译和测试通过。
- [x] 4.2 在 `user-services/` 运行 `go test ./...`，验证 bootstrap 拆分后 Fx wiring、路由和运行时相关测试通过。
- [x] 4.3 在 `user-services/` 运行 `go generate ./ent`，验证分类后的 schema 可被 Ent codegen 读取，且不得手写 `user-services/ent/` 生成代码。
- [x] 4.4 运行 Atlas schema source 验证或 migration validate，确认分类后的 schema source 可读取且未因字段/索引语义变化产生新的 migration。
- [x] 4.5 审查变更 diff，确认 HTTP API、响应信封、错误码、配置键、运行时资源名称和数据库 schema 均保持兼容。
