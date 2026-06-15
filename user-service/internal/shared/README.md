# User Service Shared Kernel

`internal/shared` 只承载用户服务内稳定共享业务内核。新增内容必须已经被至少两个 feature 真实消费，且不能归入跨服务无业务语义的 `common`。

## 目录规则

- 按稳定业务内核子域建包，例如 `identity`、`rbacbaseline`。
- 不新增根级 `errors`、`enums`、`types`、`utils` 或 `helpers` 包。
- 公共错误放在 owning shared 子包的 `errors.go`。
- 公共枚举按业务语义命名为 `<subject>_status.go`、`<subject>_type.go` 或 `<subject>_kind.go`。
- 系统内置规格或目录数据放在 owning shared 子包的 `catalog.go` 或更具体的 `<subject>_catalog.go`。

## 禁止事项

- 不放 Gin、Ent、Redis、SQL、Fx provider、controller、transport DTO、store port 或 use case。
- 不读取配置、不写日志、不访问数据库或缓存、不调用外部系统。
- 不迁入 `deployments/` 下的 Docker、Compose、Kubernetes 或 Helm 资产。
