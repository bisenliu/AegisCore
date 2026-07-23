## MODIFIED Requirements

### Requirement: Go 模块依赖声明

`common` 模块 MUST 将生产代码直接 import 的第三方模块声明为 direct require，MUST NOT 将直接 import 的运行时依赖标记为 `// indirect`。

#### Scenario: PostgreSQL tracing direct dependency

- **WHEN** `common/runtime/datastore/postgres.go` 直接 import `github.com/XSAM/otelsql`
- **THEN** `common/go.mod` MUST 在 direct require 组声明 `github.com/XSAM/otelsql`
- **AND** 该 require MUST NOT 标记为 `// indirect`
