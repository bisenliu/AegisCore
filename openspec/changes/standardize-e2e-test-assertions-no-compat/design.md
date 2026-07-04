## 背景

`user-service/tests/e2e` 是跨 feature 的端到端测试包：`harness_test.go` 负责 Testcontainers、配置文件、Fx/Gin runtime 启动和 response envelope 辅助；`migrations_test.go` 负责查找并应用 user-service SQL migration；`http_flow_test.go` 负责认证、强制改密、用户创建/查询、缺失认证、登出和 refresh 失败的完整 HTTP flow。

这些测试当前失败判断集中在 `t.Fatal` / `t.Fatalf` 和复合 if。迁移目标是让断言表达符合 `docs/TESTING.md` 的语义化优先规则，同时保持端到端流程、测试数据、容器前置条件和生产行为完全不变。

## 目标与非目标

目标：

- 将 `user-service/tests/**/*_test.go` 内可语义化表达的历史失败判断迁移到 `testify/require` 或必要时的 `testify/assert`。
- 让 HTTP status、错误码、响应 envelope、集合长度、无序集合、JSON 响应和时间相关检查优先使用更具体断言。
- 对完整 HTTP flow 中多个互相独立的响应字段检查，允许使用 `assert` 收集独立失败；对前置条件、解码、依赖后续检查的结果继续使用 `require`。
- 通过残留扫描列明任何符合 `docs/TESTING.md` 特殊例外规则的直接 `testing.T` 失败调用，或正则扫描 false positive。
- 运行 OpenSpec 校验，并在实现阶段运行目标测试或记录容器环境前置条件。

非目标：

- 不修改 E2E 流程覆盖的生产 API、controller、middleware、认证、授权、用户业务逻辑或 response envelope 运行时行为。
- 不修改 Ent schema、Atlas SQL migration、OpenAPI 生成物、部署资产、RBAC seed 或测试数据语义。
- 不迁移 `common`、feature、router、provider、cmd 或 tools 测试。
- 不新增旧 API 响应、旧 migration 行为、旧测试数据兼容断言。
- 不新增机械 `Fail` / `Failf` / `FailNow` / `FailNowf` 替换，也不新增旧手写断言兼容 helper。

## 决策

### 决策一：直接在 E2E 测试中使用 testify

`user-service/go.mod` 已直接声明 `github.com/stretchr/testify`，E2E 测试应直接导入 `require` 和必要的 `assert`。迁移不新增共享断言 helper，也不把历史手写 if 包装成兼容函数。

备选方案是新增本包专用 `must*` helper 隐藏 `require` 调用。该方案会保留旧断言风格的间接层，降低失败信息清晰度，不符合本次统一断言目标。

### 决策二：依赖后续执行的检查使用 require

配置写入、空闲端口解析、PostgreSQL 打开、password KDF 创建、密码 hash、migration 文件读取、SQL 语句拆分、HTTP response envelope 解码和 response data 解码均是后续检查的前置条件。它们应使用 `require.NoError`、`require.NotEmpty`、`require.NotNil`、`require.Greater` 或等价语义化断言立即终止当前测试。

备选方案是在这些辅助函数中使用 `assert` 继续收集失败。该方案会让后续步骤在无效前置条件下继续执行，容易产生级联错误。

### 决策三：完整 HTTP flow 的独立字段可使用 assert

当同一个 HTTP 响应中多个字段互相独立，且后续逻辑不依赖某个字段检查成功时，可使用 `assert.Equal`、`assert.NotEmpty`、`assert.Empty` 或等价断言收集多个字段差异。例如 response envelope 的 status、success、code、message，以及 token metadata 的 token type、expires_in 和 refresh token 存在性可以按依赖关系选择 `assert` 或 `require`。

备选方案是全部使用 `require`。该方案简单但可能在完整 flow 响应中只报告第一个字段差异，诊断信息少于 `docs/TESTING.md` 允许的独立失败收集。

### 决策四：migration parser 的 error return 不属于 testing.T 失败方法

`splitSQLStatements` 返回的 `fmt.Errorf(...)` 是 SQL 解析 helper 的普通 error return，不是 `testing.T` 失败调用。实施时若验收正则因为 `fmt.Errorf(` 包含 `t.Errorf(` 子串而命中这些行，应在 tasks 的剩余例外中列为扫描 false positive；不得为了消除 false positive 而改变 parser 错误语义或引入无意义包装。

备选方案是重写 helper 错误构造以规避正则。该方案属于机械性规避，不提升断言语义，且会给 migration parser 引入无关 churn。

### 决策五：保持 E2E 环境开关和容器前置条件不变

`requireE2EEnabled` 现有语义保持不变：显式 `AEGISCORE_TEST_E2E` 启用时设置 Testcontainers 开关；通用容器测试开关已启用时直接运行；否则跳过。迁移断言不得改变容器启动、migration 应用、Fx app 启停或请求构造的前置条件。

## 风险与权衡

- 风险：E2E 测试需要 Docker 或兼容容器运行时，验证环境缺失时 `go test ./user-service/tests/...` 可能跳过或无法完整运行。缓解：tasks 明确运行前置条件；无法运行时记录环境原因，并至少完成静态扫描和 OpenSpec 校验。
- 风险：将复合 if 拆成多个断言可能改变失败停止位置。缓解：只在后续逻辑不依赖该字段成功时使用 `assert`；依赖后续执行的前置条件使用 `require`。
- 风险：残留扫描正则会命中 `fmt.Errorf` false positive。缓解：tasks 中显式列明 false positive，保留 parser error return 语义。
- 风险：迁移过程中可能顺手调整测试数据或 E2E flow。缓解：任务范围明确禁止修改流程、测试数据构造和运行前置条件。

## 实施计划

1. 基线扫描 `user-service/tests/**/*_test.go` 的历史失败调用和 testify 使用点。
2. 迁移 `harness_test.go` 中配置、端口、请求构造、envelope、data 和 PostgreSQL helper 断言。
3. 迁移 `migrations_test.go` 中 migration 文件枚举、读取、拆分、应用和 user-service 根目录定位断言。
4. 迁移 `http_flow_test.go` 中 seed、登录、强制改密、用户创建/查询、改密、登出和失败响应断言。
5. 运行 gofmt、残留扫描、testify 使用扫描、目标测试和 OpenSpec 校验；若 E2E 容器前置条件缺失，在 tasks 中记录替代验证。

## 回滚方式

本 change 只应修改 E2E 测试断言和 OpenSpec artifacts。若需要回滚，恢复 `user-service/tests/e2e/*.go` 到迁移前断言写法并删除 `openspec/changes/standardize-e2e-test-assertions-no-compat/` 即可，不需要生产数据或 migration 回滚。

## 验证方式

- `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/tests --glob "*_test.go"`
- `rg "github.com/stretchr/testify/(require|assert)" user-service/tests --glob "*_test.go"`
- `gofmt` 覆盖修改过的 Go 测试文件
- `go test ./user-service/tests/...`
- `openspec validate standardize-e2e-test-assertions-no-compat`
- 实现完成并暂存本次预期变更后，按仓库流程运行 `make lint` 和 `make verify`；若 E2E 容器环境或非本次变更阻塞，记录具体原因和已完成替代验证。
