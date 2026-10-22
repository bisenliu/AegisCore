## 1. 统一 Go 工具链

- [x] 1.1 核实 Go 1.27 最新补丁版及 `golang:1.27.3-alpine3.24` 不可变镜像 digest；将 `go.work` 和四个 `go.mod` 固定到 1.27.3，按需更新 `go.work.sum`。
- [x] 1.2 更新 `.github/workflows/ci.yml`、`.github/workflows/lint.yml` 与两个 `deployments/docker/*Dockerfile`；扩展 `user-service/scripts/architecture/lint.sh` 至全部四个 module，更新 `lint-test.sh` fixture，并运行架构 lint fixture 测试确认版本漂移可检出。

## 2. 迁移共享与服务 UUID 类型

- [x] 2.1 将 `common/runtime/id` 及 `common` 其他直接 UUID 用法迁移到标准库 `uuid`，保持默认 v7 和现有导出函数签名；更新 v7、规范字符串及非法解析的相关单元测试。
- [x] 2.2 将 user-service 的 user/auth/role/permission、router、shared 和测试中的直接 UUID 用法迁移到标准库；逐项处理 `Nil`、`NewString`、`NewRandom` 和 `Version` API 差异，并运行受影响包测试。
- [x] 2.3 在 `user-service/internal/persistence/ent/schema/` 适配标准库 UUID 的 Ent 值读写，保留 PostgreSQL `uuid` 列类型；将 `golang.org/x/tools` 更新至支持 Go 1.27 export data v5 的版本，运行 `make user-service-generate`，审查 Ent 生成物中 UUID 类型及生成 diff，不手工编辑生成代码。
- [x] 2.4 清理 `common/go.mod`、`user-service/go.mod` 的旧 UUID 直接依赖并运行各 module 的 `go mod tidy`；确认一方源码和生成物不再直接导入 `github.com/google/uuid`，仅允许第三方传递依赖保留该模块。
- [x] 2.5 将一方非生成源码的 `uuid` 导入并入标准库导入组，保留现有 `goimports` formatter 与本地模块前缀配置；同步 lint 文档，验证格式检查和生成物无 drift。

## 3. 验证契约与生成物

- [x] 3.1 运行共享 UUID、用户/认证/RBAC 相关测试，并验证 v4/v7、JSON/JWT/Redis 字符串、非法 UUID 输入及 SQLite/真实 PostgreSQL Ent UUID 读写与现有契约一致。
- [x] 3.2 运行 `make user-service-migrate-diff name=verify-stdlib-uuid` 和 `make user-service-migrate-validate`，审查 SQL/`atlas.sum` diff；确认 PostgreSQL UUID 列无结构变化时不保留空 migration。
- [x] 3.3 运行 `make user-service-openapi-generate` 并检查 OpenAPI 生成物 diff；运行 `make user-service-architecture-lint` 和 Docker builder 镜像构建检查，确认工具链、架构与构建配置一致。
- [x] 3.4 同步 `README.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和 `openspec/config.yaml`；扫描非历史、非生成文件中的旧 Go 版本声明，审查本 change 的 proposal、design 与规格 delta 一致，并运行 `openspec validate upgrade-go-127-and-stdlib-uuid`。

## 4. 最终质量门禁

- [x] 4.1 检查 `git status` 和预期生成物 diff，仅暂存本 change 的代码、依赖、文档与 OpenSpec 文件；若生成命令再次运行，使用 `git diff --exit-code` 确认无意外 drift。
- [x] 4.2 在全部实现、规格与文档任务完成且预期变更已暂存后运行 `make lint` 和 `make verify`；两者通过后再将对应任务标为完成。
