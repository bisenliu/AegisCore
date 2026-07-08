## 1. 共享错误模型

- [x] 1.1 在 `common/contract/errors` 中定义稳定 `Kind`、`Reason` 与新的应用错误结构，保留 `Code`、`Message`、可选 `Cause`，移除 `HTTPStatus` 字段。
- [x] 1.2 移除接收 HTTP status 的旧 `NewError`、`Wrap` 或等价 factory API，改为语义驱动构造入口和必要的业务中立 helper。
- [x] 1.3 更新 `FromError`、`Error()`、`Unwrap()`、`errors.Is` 和 `errors.As` 行为，覆盖 nil error、未知 error、wrapped application error 和内部错误脱敏。
- [x] 1.4 更新 `common/contract/errors` 单元测试，断言不再存在 `HTTPStatus` 行为，并锁定 `Kind`、`Reason`、`Code`、`Message`、`Cause` 与错误链语义。

## 2. HTTP 渲染责任迁移

- [x] 2.1 在 `common/http/response` 中实现唯一 HTTP status 推导函数，由应用错误 `Kind` 映射 `400`、`401`、`403`、`404`、`409`、`500`、`503`。
- [x] 2.2 调整 `Fail`、`WriteError`、错误 response helper、validation response helper 和 span error 标注，使其使用 HTTP 层状态码推导并保持统一 envelope。
- [x] 2.3 更新 `common/http/response` 单元测试，覆盖 nil error、未知 error、wrapped application error、validation error、内部错误脱敏和 span status 标注。

## 3. 共享调用方迁移

- [x] 3.1 调整 `common/validation` 的错误结构、分类和测试，使字段校验失败与 bad request 使用语义 `Kind`/`Reason`，并保留结构化字段明细。
- [x] 3.2 调整 `common/http/binding` 中空 body、尾随 JSON body、绑定失败等路径，移除对旧错误构造方式和旧 code-only 分类的依赖。
- [x] 3.3 调整 `common/http/middleware` 中 auth、Casbin、recovery 等共享中间件错误路径，迁移到新的语义错误构造或 response helper。
- [x] 3.4 按编译结果迁移 user-service 中调用共享错误 factory 的 feature mapper 或集成点，但不迁移领域 sentinel error、不删除 `toXHTTPError` mapper、不调整登录强制改密响应结构。

## 4. 规格和验证

- [x] 4.1 运行 `openspec validate reshape-shared-error-contract`，确认 proposal、design、tasks 和 `shared-platform-primitives` delta 合法。
- [x] 4.2 运行 `go test ./common/...`，确认共享契约、HTTP response、binding、middleware 和 validation 测试通过。
- [x] 4.3 运行 `make user-service-architecture-lint`，确认 common 与 user-service 边界仍符合架构规则。
- [x] 4.4 检查本次变更未手写 Ent/OpenAPI 生成物且未引入数据库、部署或观测资产漂移；如实际修改 OpenAPI 注解，再运行 `make user-service-openapi-generate` 并检查生成物 diff。
- [x] 4.5 将本次预期代码、测试和 OpenSpec 变更加到暂存区后运行 `make lint` 和 `make verify`；如 Multica runtime 文件导致工作区不干净，验证时排除项目根 `AGENTS.md`、`CLAUDE.md`、`.multica/project/resources.json` 和 `.multica/**`，并记录实际阻塞原因。
