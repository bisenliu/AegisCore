## 1. Ent README

- [x] 1.1 在 `user-services/ent/README.md` 说明 `schema/` 和 `generate.go` 的职责，以及其余文件和目录属于 Ent 生成代码。
- [x] 1.2 在 `user-services/ent/README.md` 记录重新生成命令：在 `user-services/` 模块执行 `go generate ./ent`。
- [x] 1.3 在 `user-services/ent/README.md` 添加不要手动修改生成文件的警告。
- [x] 1.4 在 `user-services/ent/README.md` 说明新增 Entity Schema 的基本流程和注意事项，包括 codegen、Atlas migration、SQL review 与 `atlas.sum`。

## 2. Development Guide

- [x] 2.1 在 `docs/DEVELOPMENT.md` 的 Ent 生成命令或编码规范附近添加 `user-services/ent/README.md` 引用。
- [x] 2.2 确认新增引用能引导开发者查阅生成代码边界、重新生成命令和新增 Entity Schema 流程。

## 3. Verification

- [x] 3.1 验证 `user-services/ent/README.md` 存在且覆盖规格中的全部场景。
- [x] 3.2 验证 `docs/DEVELOPMENT.md` 已包含到 `user-services/ent/README.md` 的引用。
- [x] 3.3 确认本次实现不修改 Ent schema、Ent 生成代码、migration SQL 或 `atlas.sum`。
