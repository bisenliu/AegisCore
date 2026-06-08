## 1. 注释缺口盘点

- [x] 1.1 扫描 `common/` 和 `user-services/` 中的手写 Go 源码，列出缺少 godoc 的导出类型、函数、方法、接口、常量和变量。
- [x] 1.2 识别复杂业务逻辑、关键分支、边界条件、默认值、超时预算、TTL、限制阈值和其他策略参数的注释缺口。
- [x] 1.3 明确排除 `user-services/ent/` 下生成代码的手工修改范围，仅将 schema 或手写代码纳入整改。

## 2. godoc 注释补齐

- [x] 2.1 为 `common/contract`、`common/http`、`common/runtime`、`common/security` 和 `common/validation` 中导出的手写 Go 标识符补充以标识符名称开头的 godoc 注释。
- [x] 2.2 为 `user-services/cmd`、`user-services/internal/bootstrap`、`user-services/internal/router`、`user-services/internal/controller`、`user-services/internal/service`、`user-services/internal/repository`、`user-services/internal/domain` 和 `user-services/internal/dto` 中导出的手写 Go 标识符补充 godoc 注释。
- [x] 2.3 为 `user-services/ent/schema` 中手写 schema 类型和导出方法补充 godoc 注释，保持 Ent 生成代码不变。
- [x] 2.4 校对新增 godoc，确保注释描述当前真实行为、参数、返回结果和重要错误，不写入未实现功能。

## 3. 复杂逻辑与策略参数注释

- [x] 3.1 为错误映射、认证上下文、请求校验、响应转换、配置 fallback、资源生命周期和 graceful shutdown 等关键分支补充解释“为什么”的注释。
- [x] 3.2 为 not found、唯一性冲突、缺失配置、空值、默认实例名和外部输入等边界条件补充必要注释。
- [x] 3.3 为魔法数字、特殊阈值、默认值、超时预算、TTL、容量限制和策略参数补充语义说明，并遵循现有常量 owner 边界。
- [x] 3.4 删除或改写空泛、重复代码表面含义、与当前实现不一致或容易误导维护者的注释。

## 4. 格式化与验证

- [x] 4.1 对修改过的 Go 文件运行 `gofmt`。
- [x] 4.2 在仓库适用位置运行 `golangci-lint`，修复注释缺失、godoc 风格和注释一致性相关问题。
- [x] 4.3 如 `golangci-lint` 暴露非注释类历史问题，记录其与本变更的关系并避免无关扩散。
- [x] 4.4 分别在 `common/` 和 `user-services/` 运行 `go test ./...`，确认注释整改未引入行为回归。

## 5. 结果复核

- [x] 5.1 复核本 change 的 `go-comment-governance` requirements，确认实现覆盖导出 API、复杂逻辑、策略参数和 lint 验证场景。
- [x] 5.2 汇总修改文件、验证命令和任何剩余风险，说明外部 HTTP API、配置、数据库 schema、生成代码和运行时行为保持不变。
