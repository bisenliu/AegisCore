#!/usr/bin/env bash
set -euo pipefail

# 验证 architecture/lint.sh 能发现关键架构违规，并且不会误报允许的测试或生成文件。
#
# 用法：
#   ./user-service/scripts/architecture/lint-test.sh
#
# 执行前提：
#   - 本机需要安装 architecture/lint.sh 依赖的 ripgrep（rg）和常见 Unix 工具。
#   - 脚本会在系统临时目录创建一个最小 Git fixture 仓库，结束时自动删除。
#
# 行为：
#   - 构造 common、user-service、tools/openapi-convert、CI workflow、deployments 和 openspec 的最小目录。
#   - 写入一组故意违规的 Go 文件和配置，用来覆盖 mock build tag、测试钩子、默认 logger、Fx/Dig 元数据等检查。
#   - Go import 分层规则由 `.golangci.yml` 的 depguard 覆盖，不在本 fixture 中断言。
#   - 通过 --repo-root 让 architecture/lint.sh 针对 fixture 运行，而不是扫描真实仓库。
#   - 断言预期违规都会出现在输出中，并断言测试文件、生成目录、组合入口等白名单不会产生误报。
#
# 注意事项：
#   - 该脚本只验证 lint 规则的代表性路径，不替代真实仓库的 architecture-lint 扫描。
#   - 更新 architecture/lint.sh 的规则、报错文本或排除列表时，应同步更新这里的 fixture 和断言。
# 步骤 1：定位被测 architecture/lint.sh 所在目录，确保后续调用使用当前仓库脚本版本。
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 步骤 2：创建临时 fixture 仓库根目录，隔离测试输入，避免真实工作区状态影响断言。
fixture_root="$(mktemp -d)"

# 步骤 3：注册退出清理逻辑，测试结束或中途失败时都删除临时 fixture。
trap 'rm -rf "${fixture_root}"' EXIT

# 步骤 4：构造最小仓库布局；目录只包含触发或排除规则所需的文件。
mkdir -p \
  "${fixture_root}/.github/workflows" \
  "${fixture_root}/common" \
  "${fixture_root}/common/runtime/observability/metrics" \
  "${fixture_root}/common/runtime/example" \
  "${fixture_root}/common/testing/example" \
  "${fixture_root}/deployments/docker" \
  "${fixture_root}/deployments/compose" \
  "${fixture_root}/deployments/helm/aegiscore-user-service/templates" \
  "${fixture_root}/docs/opsx" \
  "${fixture_root}/openspec/changes" \
  "${fixture_root}/openspec/specs" \
  "${fixture_root}/tools/openapi-convert" \
  "${fixture_root}/user-service/ent" \
  "${fixture_root}/user-service/docs" \
  "${fixture_root}/user-service/internal/persistence/ent/schema" \
  "${fixture_root}/user-service/internal/features/auth/application" \
  "${fixture_root}/user-service/internal/features/auth/infrastructure" \
  "${fixture_root}/user-service/internal/features/role" \
  "${fixture_root}/user-service/internal/features/role/infrastructure" \
  "${fixture_root}/user-service/internal/features/user" \
  "${fixture_root}/user-service/internal/features/user/infrastructure/postgres" \
  "${fixture_root}/user-service/internal/features/user/transport/http" \
  "${fixture_root}/user-service/internal/shared" \
  "${fixture_root}/user-service/migrations" \
  "${fixture_root}/user-service/scripts"

# 步骤 5：写入 common/go.mod，作为 Go 版本一致性检查的参照模块。
cat > "${fixture_root}/common/go.mod" <<'EOF'
module github.com/aegiscore/common

go 1.26.5
EOF

# 步骤 6：写入 go.work，覆盖 workspace go 版本、toolchain 和模块列表检查。
cat > "${fixture_root}/go.work" <<'EOF'
go 1.26.5

toolchain go1.26.5

use (
	./common
	./tools/openapi-convert
	./user-service
)
EOF

# 步骤 7：写入 tools/openapi-convert/go.mod，覆盖仓库工具模块版本检查。
cat > "${fixture_root}/tools/openapi-convert/go.mod" <<'EOF'
module github.com/aegiscore/tools/openapi-convert

go 1.26.5
EOF

# 步骤 8：写入 user-service/go.mod，覆盖服务模块版本检查。
cat > "${fixture_root}/user-service/go.mod" <<'EOF'
module github.com/aegiscore/user-service

go 1.26.5
EOF

# 步骤 9：写入故意违规的 CI workflow，触发重复 lint、重复单测和模块本地容器测试入口检查。
cat > "${fixture_root}/.github/workflows/ci.yml" <<'EOF'
name: ci
on:
  pull_request:
jobs:
  quality:
    uses: ./.github/workflows/lint.yml
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v7
        with:
          go-version: '1.26.5'
      - run: |
          make lint
          make test
  container-test:
    runs-on: ubuntu-latest
    steps:
      - run: make -C user-service test-containers
EOF

# 步骤 10：写入故意违规的质量 workflow，触发复用 workflow 不应直接监听 pull_request 的检查。
cat > "${fixture_root}/.github/workflows/lint.yml" <<'EOF'
name: quality
on:
  workflow_call:
  pull_request:
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v7
        with:
          go-version: '1.26.5'
      - run: make lint
  unit:
    runs-on: ubuntu-latest
    steps:
      - run: make test
EOF

# 步骤 11：写入 Atlas pg_trgm Dockerfile fixture，覆盖 Atlas PostgreSQL 镜像一致性检查。
cat > "${fixture_root}/deployments/docker/atlas-postgres-pgtrgm.Dockerfile" <<'EOF'
FROM postgres:latest
EOF

# 步骤 12：写入迁移 diff 脚本 fixture，覆盖 Atlas dev 镜像名称一致性检查。
cat > "${fixture_root}/user-service/scripts/migrate-diff.sh" <<'EOF'
ATLAS_DEV_IMAGE="aegiscore-atlas-postgres-pgtrgm:latest"
EOF

# 步骤 13：写入 atlas.hcl fixture，覆盖 Atlas dev URL 镜像一致性检查。
cat > "${fixture_root}/user-service/migrations/atlas.hcl" <<'EOF'
  default = "docker+postgres://_/aegiscore-atlas-postgres-pgtrgm:latest/dev?search_path=public"
EOF

# 步骤 14：写入故意违规的 Compose fixture，触发本地完整配置入口和 --config 检查。
cat > "${fixture_root}/deployments/compose/docker-compose.yml" <<'EOF'
services:
  postgres:
    image: postgres:latest
  user-service:
    image: aegiscore/user-service:local
    command:
      - serve
      - --config
EOF

# 步骤 15：写入故意违规的 Helm production values，触发必须显式 image.ref 的检查。
cat > "${fixture_root}/deployments/helm/aegiscore-user-service/values.yaml" <<'EOF'
image:
  repository: aegiscore-user-service
  tag: latest
  pullPolicy: IfNotPresent
EOF

# 步骤 16：写入故意违规的 Helm local values，触发禁止 latest image ref 的检查。
cat > "${fixture_root}/deployments/helm/aegiscore-user-service/values-local.yaml" <<'EOF'
image:
  ref: aegiscore-user-service:latest
EOF

# 步骤 17：写入故意违规的 Helm Chart，触发 appVersion latest fallback 检查。
cat > "${fixture_root}/deployments/helm/aegiscore-user-service/Chart.yaml" <<'EOF'
apiVersion: v2
name: aegiscore-user-service
type: application
version: 0.1.0
appVersion: "latest"
EOF

# 步骤 18：写入故意违规的 Helm helper，触发未强制 image.ref 和未使用不可变 allowlist 的检查。
cat > "${fixture_root}/deployments/helm/aegiscore-user-service/templates/_helpers.tpl" <<'EOF'
{{- define "aegiscore-user-service.image" -}}
{{- printf "%s:%s" .Values.image.repository (.Values.image.tag | default .Chart.AppVersion) -}}
{{- end -}}
EOF

# 步骤 19：写入故意违规的 Helm Deployment，触发未使用集中 image helper 的检查。
cat > "${fixture_root}/deployments/helm/aegiscore-user-service/templates/deployment.yaml" <<'EOF'
image: {{ .Values.image.ref | quote }}
EOF

# 步骤 20：写入故意违规的 Helm RBAC seed Job，触发未使用集中 image helper 的检查。
cat > "${fixture_root}/deployments/helm/aegiscore-user-service/templates/rbac-seed-job.yaml" <<'EOF'
image: {{ .Values.image.ref | quote }}
EOF

# 步骤 20.1：写入故意违规的 Kustomize 聚合，触发默认资源包含 seed Job 的检查。
mkdir -p "${fixture_root}/deployments/k8s/user-service"
cat > "${fixture_root}/deployments/k8s/user-service/kustomization.yaml" <<'EOF'
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deployment.yaml
  - rbac-seed-job.yaml
EOF

# 步骤 21：写入 application 反向导入 HTTP transport 的违规样例，覆盖 feature 分层检查。
cat > "${fixture_root}/user-service/internal/features/auth/application/violating_import.go" <<'EOF'
package application

import _ "github.com/aegiscore/user-service/internal/features/auth/transport/http"
EOF

# 步骤 22：写入缺少 generate build tag 的 mock_generate.go，覆盖 mock 生成入口检查。
cat > "${fixture_root}/common/runtime/example/mock_generate.go" <<'EOF'
package example

//go:generate go tool mockgen -destination=mock_test.go -package=example example.org/project Port
EOF

# 步骤 23：写入 root-level Ent import 违规样例，覆盖 Ent 私有边界检查。
cat > "${fixture_root}/common/runtime/example/root_ent_import.go" <<'EOF'
package example

import _ "github.com/aegiscore/user-service/ent"
EOF

# 步骤 24：写入 common metrics 业务语义违规样例，覆盖 common 观测 primitive 边界检查。
cat > "${fixture_root}/common/runtime/observability/metrics/casbin_reload.go" <<'EOF'
package metrics

const casbinReloadsMetricName = "aegiscore_casbin_policy_reloads_total"
EOF

# 步骤 25：写入生产文件中的 testHook 违规样例，覆盖测试专用符号检查。
cat > "${fixture_root}/user-service/internal/features/auth/application/test_hook.go" <<'EOF'
package application

func setClockForTest() {}
EOF

# 步骤 26：写入 application 层 Fx 元数据违规样例，覆盖 application/domain 框架无关检查。
cat > "${fixture_root}/user-service/internal/features/auth/application/fx_metadata.go" <<'EOF'
package application

import "go.uber.org/fx"

type Deps struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

# 步骤 27：写入 role feature 组合入口白名单样例，确认 fx.go 不被 Fx 元数据规则误报。
cat > "${fixture_root}/user-service/internal/features/role/fx.go" <<'EOF'
package role

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

# 步骤 28：写入 role infrastructure Fx 元数据违规样例，覆盖 role feature 分层元数据检查。
cat > "${fixture_root}/user-service/internal/features/role/infrastructure/store.go" <<'EOF'
package infrastructure

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

# 步骤 29：写入 user infrastructure Fx 元数据违规样例，覆盖 user feature 分层元数据检查。
cat > "${fixture_root}/user-service/internal/features/user/infrastructure/postgres/fx_metadata.go" <<'EOF'
package postgres

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

# 步骤 30：写入 user feature 组合入口白名单样例，确认 fx.go 不被 Fx 元数据规则误报。
cat > "${fixture_root}/user-service/internal/features/user/fx.go" <<'EOF'
package user

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

# 步骤 31：写入固定 route_registrar.go 违规样例，覆盖路由集中总装检查。
cat > "${fixture_root}/user-service/internal/features/user/route_registrar.go" <<'EOF'
package user
EOF

# 步骤 32：写入测试文件白名单样例，确认测试代码中的 Fx 元数据不会被生产规则误报。
cat > "${fixture_root}/user-service/internal/features/user/infrastructure/postgres/store_test.go" <<'EOF'
package postgres

import "go.uber.org/fx"

type TestParams struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

# 步骤 33：写入带 generate build tag 的 mock_generate.go 白名单样例，确认生成入口不会被误报。
cat > "${fixture_root}/user-service/internal/features/user/transport/http/mock_generate.go" <<'EOF'
//go:build generate

package http

import "go.uber.org/fx"

type Params struct {
	fx.In
	Store any `name:"primary_db"`
}
EOF

# 步骤 34：写入默认 logger 依赖违规样例，覆盖 feature 主路径 logger 注入检查。
cat > "${fixture_root}/user-service/internal/features/auth/infrastructure/default_logger.go" <<'EOF'
package infrastructure

import (
	"context"

	"github.com/aegiscore/common/runtime/logger"
)

func useDefaultLogger() {
	logger.Info(context.Background(), "default logger dependency")
}
EOF

# 步骤 35：写入旧环境变量配置入口违规样例，覆盖本地完整配置入口清理检查。
cat > "${fixture_root}/user-service/internal/features/auth/infrastructure/env_config.go" <<'EOF'
package infrastructure

import "os"

func loadEnvConfig() string {
	return os.Getenv("USER_SERVICE_CONFIG")
}
EOF

# 步骤 36：写入测试文件中允许的测试专用符号，确认 ForTest/testHook 规则不会误报测试文件。
cat > "${fixture_root}/user-service/internal/features/auth/application/allowed_test.go" <<'EOF'
package application

func setRequestForTest() {}
EOF

# 步骤 37：写入 common/testing 白名单样例，确认测试基础设施中的 ForTest 命名不会误报。
cat > "${fixture_root}/common/testing/example/helper.go" <<'EOF'
package example

func NewStoreForTest() {}
EOF

# 步骤 38：写入 Ent 生成目录白名单样例，确认生成物目录不会被测试专用符号规则误报。
cat > "${fixture_root}/user-service/internal/persistence/ent/schema/generated.go" <<'EOF'
package schema

func testHookGenerated() {}
EOF

# 步骤 39：写入 OpenAPI 生成目录白名单样例，确认 docs 生成物不会被测试专用符号规则误报。
cat > "${fixture_root}/user-service/docs/openapi.go" <<'EOF'
package docs

func NewDocumentForTest() {}
EOF

# 步骤 40：初始化 fixture 为 Git 仓库，使 architecture/lint.sh 中依赖 git diff 的规则可运行。
git -C "${fixture_root}" init -q

# 步骤 41：执行被测 lint 脚本；临时关闭 errexit 以捕获预期失败输出和退出码。
set +e
output="$("${script_dir}/lint.sh" --repo-root "${fixture_root}" 2>&1)"
status=$?
set -e

# 步骤 42：确认 fixture 中的故意违规确实让 lint 失败，避免测试在无效输入下误判通过。
if [[ "${status}" -eq 0 ]]; then
  printf 'architecture-lint-test: expected fixture violation to fail\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 43：断言 mock_generate.go build tag 违规被报告。
if [[ "${output}" != *"mock_generate.go must use //go:build generate"* ]]; then
  printf 'architecture-lint-test: expected mock generate build tag violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 44：断言生产文件中的测试专用符号违规被报告。
if [[ "${output}" != *"test-only symbol must not enter production Go files"* ]]; then
  printf 'architecture-lint-test: expected test-only production symbol violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 45：断言 feature 主路径默认 logger 依赖违规被报告。
if [[ "${output}" != *"feature production code must not use package-level default logger as main-path dependency"* ]]; then
  printf 'architecture-lint-test: expected default logger dependency violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 46：断言旧本地配置入口的 Go 代码违规被报告。
if [[ "${output}" != *"production Go code must not retain local config path entrypoints"* ]]; then
  printf 'architecture-lint-test: expected Go local config path violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 47：断言 root-level user-service/ent 目录违规被报告。
if [[ "${output}" != *"root-level user-service/ent package is forbidden"* ]]; then
  printf 'architecture-lint-test: expected root-level Ent directory violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 48：断言 root-level Ent import 违规被报告。
if [[ "${output}" != *"root-level user-service Ent import is forbidden"* ]]; then
  printf 'architecture-lint-test: expected root-level Ent import violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 49：断言 Compose 中旧本地配置入口违规被报告。
if [[ "${output}" != *"Docker Compose runtime config must not mount local full config or pass --config"* ]]; then
  printf 'architecture-lint-test: expected Compose local config path violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 50：断言 Helm production values 未强制 image.ref 的违规被报告。
if [[ "${output}" != *"Helm production values must require explicit image.ref"* ]]; then
  printf 'architecture-lint-test: expected Helm image.ref required violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 51：断言 Helm image helper 未使用不可变 allowlist 的违规被报告。
if [[ "${output}" != *"Helm image helper must allow only repository digest refs"* || "${output}" != *"Helm image helper must allow only full sha commit tags"* || "${output}" != *"Helm image helper must reject mutable image refs"* ]]; then
  printf 'architecture-lint-test: expected Helm immutable allowlist violation reports\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 51.1：断言 RBAC seed Job 默认发布门禁违规被报告。
if [[ "${output}" != *"Helm RBAC seed Job must be disabled by default for runtime manifests"* || "${output}" != *"Helm RBAC seed Job name must support release-unique suffixes"* || "${output}" != *"Kustomize default user-service resources must not include RBAC seed Job"* ]]; then
  printf 'architecture-lint-test: expected RBAC seed release gate violation reports\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 52：断言 application/domain 层 Fx 元数据违规被报告。
if [[ "${output}" != *"feature application/domain production code must not carry Fx DI metadata"* ]]; then
  printf 'architecture-lint-test: expected application/domain Fx metadata violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 53：断言 common metrics 中的服务或 RBAC 业务语义违规被报告。
if [[ "${output}" != *"common runtime metrics must not contain service or RBAC business metrics semantics"* ]]; then
  printf 'architecture-lint-test: expected common metrics business semantics violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 54：断言 CI 质量门禁相关违规被完整报告。
if [[ "${output}" != *"quality workflow must not directly trigger pull_request or push"* || "${output}" != *"CI standard lint command must appear exactly once"* || "${output}" != *"CI standard unit test command must appear exactly once"* || "${output}" != *"CI Docker-backed test command must call root make test-containers exactly once"* || "${output}" != *"CI must not bypass root make test-containers with module-local container targets"* ]]; then
  printf 'architecture-lint-test: expected duplicate CI quality gate violation reports\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 55：断言 user feature 分层 Fx/Dig 元数据违规被报告。
if [[ "${output}" != *"user feature production code must not carry Fx/Dig DI metadata outside composition"* ]]; then
  printf 'architecture-lint-test: expected user feature Fx/Dig metadata violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 56：断言 role feature 分层 Fx/Dig 元数据违规被报告。
if [[ "${output}" != *"role feature production code must not carry Fx/Dig DI metadata outside composition"* ]]; then
  printf 'architecture-lint-test: expected role feature Fx/Dig metadata violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 57：断言固定 route_registrar.go 违规被报告。
if [[ "${output}" != *"fixed feature route_registrar.go files are forbidden"* ]]; then
  printf 'architecture-lint-test: expected fixed feature route registrar violation report\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 58：断言白名单文件不进入违规结果；覆盖测试文件、测试 helper、Ent/OpenAPI 生成目录和 feature 组合入口。
if [[ "${output}" == *"allowed_test.go"* || "${output}" == *"common/testing/example/helper.go"* || "${output}" == *"user-service/internal/persistence/ent/schema/generated.go"* || "${output}" == *"user-service/docs/openapi.go"* || "${output}" == *"user-service/internal/features/role/fx.go"* || "${output}" == *"user-service/internal/features/user/fx.go"* || "${output}" == *"user-service/internal/features/user/infrastructure/postgres/store_test.go"* || "${output}" == *"user-service/internal/features/user/transport/http/mock_generate.go"* ]]; then
  printf 'architecture-lint-test: excluded test or generated file produced a false positive\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 59：断言 rg 没有执行错误；如果失败通常意味着规则路径或 fixture 目录布局需要同步调整。
if [[ "${output}" == *"rg execution failed"* ]]; then
  printf 'architecture-lint-test: unexpected rg execution failure\n%s\n' "${output}" >&2
  exit 1
fi

# 步骤 60：全部断言通过后输出成功标记，供调用方和 CI 识别。
printf 'architecture-lint-test: ok\n'
