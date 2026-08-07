#!/usr/bin/env bash
set -euo pipefail

# 校验仓库级架构边界和生成物状态。
#
# 用法：
#   ./user-service/scripts/architecture/lint.sh
#   ./user-service/scripts/architecture/lint.sh --repo-root /path/to/repo
#   make user-service-architecture-lint
#
# 执行前提：
#   - 在 Git 工作区内运行，且仓库包含 common、user-service、tools/openapi-convert 等模块。
#   - 本机需要安装 ripgrep（rg）；脚本依赖 rg 执行正则扫描。
#   - 需要可用的常见 Unix 工具 awk、sed、head、find、wc、tr 和 git。
#   - 部分检查会调用 git diff --name-only，用于发现 Ent/OpenAPI 生成物漂移。
#
# 行为：
#   - 检查 Go toolchain 版本一致，并防止 CI 重复触发标准 lint 与普通单测。
#   - 检查 Atlas dev PostgreSQL 镜像配置在 Dockerfile、Compose、atlas.hcl 和迁移脚本中一致。
#   - 扫描 common 与 user-service 的架构边界，避免共享包、feature 分层、Ent 根级暴露和 DI 元数据越界。
#   - 检查 mock 生成文件 build tag、测试专用符号、OpenSpec 中文模板和生成物未提交漂移。
#   - 收集所有违规项后统一退出；任一违规会以非 0 状态结束。
#
# 注意事项：
#   - 本脚本只做静态扫描和 Git diff 检查，不会修改源码或生成物。
#   - 新增目录或架构边界时，应同步扩展本脚本和 architecture/lint-test.sh 的 fixture 覆盖。
# 该脚本使用 Bash 数组、进程替换和 BASH_SOURCE；保持 Bash shebang，避免被 sh 误执行。
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

# 解析可选参数，允许测试脚本把 lint 根目录切到临时 fixture 仓库。
while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-root)
      if [[ $# -lt 2 || -z "$2" ]]; then
        printf 'architecture-lint: --repo-root requires a path\n' >&2
        exit 2
      fi
      repo_root="$(cd "$2" && pwd)"
      shift 2
      ;;
    -h|--help)
      printf 'Usage: %s [--repo-root <path>]\n' "$0"
      exit 0
      ;;
    *)
      printf 'architecture-lint: unknown argument: %s\n' "$1" >&2
      exit 2
      ;;
  esac
done

service_dir="${repo_root}/user-service"

# 让未匹配的 glob 展开为空列表，避免不存在的 feature 子目录被当作字面路径传给 rg。
shopt -s nullglob

# 先校验核心扫描工具是否存在；缺少 rg 时后续架构规则无法可靠执行。
if ! command -v rg >/dev/null 2>&1; then
  printf 'architecture-lint: required command not found: rg; install ripgrep before running this check\n' >&2
  exit 1
fi

failures=0

# 累积违规数量并输出完整上下文，最后统一决定是否失败，方便一次修复多类问题。
report() {
  printf 'architecture-lint: %s\n' "$*" >&2
  failures=$((failures + 1))
}

run_rg() {
  # 针对 Go 文件执行违规模式扫描；rg 退出码 1 表示无匹配，不属于失败。
  local description="$1"
  local pattern="$2"
  shift 2
  if [[ "$#" -eq 0 ]]; then
    return
  fi

  local output status
  set +e
  output="$(rg -n --glob '*.go' "$pattern" "$@" 2>&1)"
  status=$?
  set -e

  case "${status}" in
    0)
      report "${description}"$'\n'"${output}"
      ;;
    1)
      return
      ;;
    *)
      report "${description}: rg execution failed"$'\n'"${output}"
      ;;
  esac
}

run_rg_any() {
  # 针对任意文件执行违规模式扫描，供 YAML、Markdown、Git diff 输出等非 Go 内容复用。
  local description="$1"
  local pattern="$2"
  shift 2
  if [[ "$#" -eq 0 ]]; then
    return
  fi

  local output status
  set +e
  output="$(rg -n "$pattern" "$@" 2>&1)"
  status=$?
  set -e

  case "${status}" in
    0)
      report "${description}"$'\n'"${output}"
      ;;
    1)
      return
      ;;
    *)
      report "${description}: rg execution failed"$'\n'"${output}"
      ;;
  esac
}

go_mod_version() {
  local file="$1"
  awk '$1 == "go" { print $2; exit }' "${file}"
}

toolchain_version() {
  local file="$1"
  awk '$1 == "toolchain" { sub(/^go/, "", $2); print $2; exit }' "${file}"
}

workflow_go_version() {
  local file="$1"
  sed -nE "s/^[[:space:]]*go-version:[[:space:]]*['\"]?([0-9]+\\.[0-9]+\\.[0-9]+)['\"]?.*/\\1/p" "${file}" | head -n 1
}

check_go_toolchain_version() {
  # common/go.mod 是当前仓库 Go 版本的单一参照；go.work、服务模块和 CI 配置必须跟随它。
  local expected
  expected="$(go_mod_version "${repo_root}/common/go.mod")"
  if [[ -z "${expected}" ]]; then
    report "common/go.mod missing go version"
    return
  fi

  local label version
  while IFS=$'\t' read -r label version; do
    if [[ -z "${version}" ]]; then
      report "${label} missing Go toolchain version; expected ${expected}"
    elif [[ "${version}" != "${expected}" ]]; then
      report "${label} has Go version ${version}; expected ${expected}"
    fi
  done < <(
    printf 'go.work go\t%s\n' "$(go_mod_version "${repo_root}/go.work")"
    printf 'go.work toolchain\t%s\n' "$(toolchain_version "${repo_root}/go.work")"
    for mod in common/go.mod user-service/go.mod tools/openapi-convert/go.mod; do
      printf '%s go\t%s\n' "${mod}" "$(go_mod_version "${repo_root}/${mod}")"
    done
    for workflow in .github/workflows/ci.yml .github/workflows/lint.yml; do
      printf '%s go-version\t%s\n' "${workflow}" "$(workflow_go_version "${repo_root}/${workflow}")"
    done
  )

  for mod in common/go.mod user-service/go.mod tools/openapi-convert/go.mod; do
    version="$(toolchain_version "${repo_root}/${mod}")"
    if [[ -n "${version}" && "${version}" != "${expected}" ]]; then
      report "${mod} toolchain has Go version ${version}; expected ${expected}"
    fi
  done
}

check_ci_quality_workflow() {
  # 顶层 CI 唯一拥有事件触发；复用 workflow 只定义一次标准 lint 与普通单测，容器门禁只调用根入口。
  local ci_workflow="${repo_root}/.github/workflows/ci.yml"
  local quality_workflow="${repo_root}/.github/workflows/lint.yml"
  local caller_count lint_count unit_count container_count service_container_count

  caller_count="$( (rg -n 'uses:[[:space:]]+\./\.github/workflows/lint\.yml[[:space:]]*$' "${ci_workflow}" || true) | wc -l | tr -d ' ' )"
  if [[ "${caller_count}" -ne 1 ]]; then
    report "ci workflow must call .github/workflows/lint.yml exactly once; found ${caller_count}"
  fi

  if ! rg -q '^  workflow_call:[[:space:]]*$' "${quality_workflow}"; then
    report "quality workflow must expose workflow_call"
  fi
  if rg -q '^  (pull_request|push):[[:space:]]*$' "${quality_workflow}"; then
    report "quality workflow must not directly trigger pull_request or push"
  fi

  lint_count="$( (rg -n '^[[:space:]]+(-[[:space:]]+)?(run:[[:space:]]+)?make lint[[:space:]]*$' "${ci_workflow}" "${quality_workflow}" || true) | wc -l | tr -d ' ' )"
  if [[ "${lint_count}" -ne 1 ]]; then
    report "CI standard lint command must appear exactly once; found ${lint_count}"
  fi

  unit_count="$( (rg -n '^[[:space:]]+(-[[:space:]]+)?(run:[[:space:]]+)?make test[[:space:]]*$' "${ci_workflow}" "${quality_workflow}" || true) | wc -l | tr -d ' ' )"
  if [[ "${unit_count}" -ne 1 ]]; then
    report "CI standard unit test command must appear exactly once; found ${unit_count}"
  fi

  container_count="$( (rg -n '^[[:space:]]+(-[[:space:]]+)?(run:[[:space:]]+)?make test-containers[[:space:]]*$' "${ci_workflow}" "${quality_workflow}" || true) | wc -l | tr -d ' ' )"
  if [[ "${container_count}" -ne 1 ]]; then
    report "CI Docker-backed test command must call root make test-containers exactly once; found ${container_count}"
  fi

  service_container_count="$( (rg -n 'make[[:space:]]+-C[[:space:]]+(common|user-service)[[:space:]]+test-containers' "${ci_workflow}" "${quality_workflow}" || true) | wc -l | tr -d ' ' )"
  if [[ "${service_container_count}" -ne 0 ]]; then
    report "CI must not bypass root make test-containers with module-local container targets; found ${service_container_count}"
  fi
}

check_atlas_postgres_version() {
  # 当前本地交付配置默认跟随最新 PostgreSQL；正式环境建议固定具体版本或镜像 digest。
  local expected="latest"
  local dockerfile="${repo_root}/deployments/docker/atlas-postgres-pgtrgm.Dockerfile"
  local migrate_script="${service_dir}/scripts/migrate-diff.sh"
  local atlas_config="${service_dir}/migrations/atlas.hcl"
  local compose_file="${repo_root}/deployments/compose/docker-compose.yml"

  if ! rg -q "^FROM postgres:${expected}$" "${dockerfile}"; then
    report "Atlas pg_trgm Dockerfile must use postgres:${expected}"
  fi
  if ! rg -q "^ATLAS_DEV_IMAGE=\"aegiscore-atlas-postgres-pgtrgm:${expected}\"$" "${migrate_script}"; then
    report "migration diff script must build aegiscore-atlas-postgres-pgtrgm:${expected}"
  fi
  if ! rg -q "^[[:space:]]+default = \"docker\\+postgres://_/aegiscore-atlas-postgres-pgtrgm:${expected}/dev\\?search_path=public\"$" "${atlas_config}"; then
    report "Atlas dev URL must use aegiscore-atlas-postgres-pgtrgm:${expected}"
  fi
  if ! rg -q "^[[:space:]]+image: postgres:${expected}$" "${compose_file}"; then
    report "Compose PostgreSQL service must use postgres:${expected}"
  fi
}

check_ent_internal_persistence_boundary() {
  # Ent 是 user-service 私有持久化实现，必须位于 internal 下，禁止重新暴露为模块根级包。
  if [[ -d "${service_dir}/ent" ]]; then
    report "root-level user-service/ent package is forbidden; use user-service/internal/persistence/ent"
  fi

  run_rg "root-level user-service Ent import is forbidden" \
    'github\.com/aegiscore/user-service/ent(/|")' \
    "${repo_root}/common" \
    "${repo_root}/tools" \
    "${service_dir}"
}

check_helm_user_service_immutable_image() {
  # 生产 Helm chart 必须由发布流程显式传入不可变 image.ref，禁止 latest fallback。
  local chart_dir="${repo_root}/deployments/helm/aegiscore-user-service"
  local values_file="${chart_dir}/values.yaml"
  local local_values_file="${chart_dir}/values-local.yaml"
  local chart_file="${chart_dir}/Chart.yaml"
  local helpers_file="${chart_dir}/templates/_helpers.tpl"
  local deployment_file="${chart_dir}/templates/deployment.yaml"
  local seed_job_file="${chart_dir}/templates/rbac-seed-job.yaml"

  if [[ ! -d "${chart_dir}" ]]; then
    return
  fi

  if ! rg -q '^[[:space:]]+ref:[[:space:]]+""[[:space:]]*#' "${values_file}"; then
    report "Helm production values must require explicit image.ref"
  fi
  if rg -q '^[[:space:]]+(repository|tag):' "${values_file}"; then
    report "Helm production values must not retain image.repository or image.tag"
  fi
  if rg -q 'appVersion:[[:space:]]+"latest"|appVersion:[[:space:]]+latest' "${chart_file}"; then
    report "Helm Chart.appVersion must not use latest fallback"
  fi
  if ! rg -q 'required "image\.ref is required' "${helpers_file}"; then
    report "Helm image helper must require image.ref"
  fi
  if ! rg -q 'fail "image\.ref must be immutable and must not use latest tag"' "${helpers_file}"; then
    report "Helm image helper must fail on latest image ref"
  fi
  if rg -q 'image\.repository|image\.tag' "${helpers_file}"; then
    report "Helm image helper must not retain repository/tag fallback"
  fi
  if rg -q '^[[:space:]]+ref:[[:space:]]+.*:latest([[:space:]]|$)' "${local_values_file}"; then
    report "Helm local values must not use latest image ref"
  fi
  if ! rg -q 'image:[[:space:]]+\{\{ include "aegiscore-user-service\.image" \. \| quote \}\}' "${deployment_file}"; then
    report "Helm Deployment must use centralized immutable image helper"
  fi
  if ! rg -q 'image:[[:space:]]+\{\{ include "aegiscore-user-service\.image" \. \| quote \}\}' "${seed_job_file}"; then
    report "Helm RBAC seed Job must use centralized immutable image helper"
  fi
}

check_mock_generate_build_tags() {
  # mock_generate.go 只允许在 generate build tag 下参与 mockgen，避免进入常规编译路径。
  local file
  while IFS= read -r file; do
    if [[ "$(head -n 1 "${file}")" != "//go:build generate" ]]; then
      report "mock_generate.go must use //go:build generate: ${file#"${repo_root}/"}"
    fi
  done < <(find "${repo_root}/common" "${service_dir}" -type f -name 'mock_generate.go' -print)
}

check_test_only_production_symbols() {
  # ForTest/testHook 命名只允许出现在测试、测试基础设施或生成物中，防止测试钩子进入生产路径。
  local pattern='(^|[^[:alnum:]_])([[:alpha:]_][[:alnum:]_]*ForTest|testHook[[:alnum:]_]*)([^[:alnum:]_]|$)'
  local file output status
  while IFS= read -r file; do
    set +e
    output="$(rg -n "${pattern}" "${file}" 2>&1)"
    status=$?
    set -e
    case "${status}" in
      0)
        report "test-only symbol must not enter production Go files: ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
      1)
        ;;
      *)
        report "test-only symbol scan failed for ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
    esac
  done < <(
    find "${repo_root}/common" "${service_dir}" \
      -type f -name '*.go' \
      ! -name '*_test.go' \
      ! -name 'mock_generate.go' \
      ! -path "${repo_root}/common/testing/*" \
      ! -path "${service_dir}/internal/persistence/ent/*" \
      ! -path "${service_dir}/docs/*" \
      -print
  )
}

check_feature_default_logger_dependencies() {
  # feature 主路径应通过依赖注入获得 logger，避免依赖包级默认 logger 或 context.Background()。
  local pattern='logger\.SetDefault\(|logger\.(FromContext|Info|Warn|Error|Debug)\(context\.Background\(\)|logger\.NamedComponent\(nil,'
  local file output status
  while IFS= read -r file; do
    set +e
    output="$(rg -n "${pattern}" "${file}" 2>&1)"
    status=$?
    set -e
    case "${status}" in
      0)
        report "feature production code must not use package-level default logger as main-path dependency: ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
      1)
        ;;
      *)
        report "feature default logger dependency scan failed for ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
    esac
  done < <(
    find "${service_dir}/internal/features" \
      -type f -name '*.go' \
      ! -name '*_test.go' \
      ! -name 'mock_generate.go' \
      \( -path '*/application/*' -o -path '*/infrastructure/*' \) \
      -print
  )
}

check_feature_application_domain_fx_metadata() {
  # application/domain 层保持框架无关，不携带 Fx import、fx.In 或 name/optional 注入标签。
  local pattern='go\.uber\.org/fx|fx\.In|`[^`]*(name|optional):"'
  local file output status
  while IFS= read -r file; do
    set +e
    output="$(rg -n "${pattern}" "${file}" 2>&1)"
    status=$?
    set -e
    case "${status}" in
      0)
        report "feature application/domain production code must not carry Fx DI metadata: ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
      1)
        ;;
      *)
        report "feature application/domain Fx metadata scan failed for ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
    esac
  done < <(
    find "${service_dir}/internal/features" \
      -type f -name '*.go' \
      ! -name '*_test.go' \
      ! -name 'mock_generate.go' \
      \( -path '*/application/*' -o -path '*/domain/*' \) \
      -print
  )
}

check_user_feature_framework_metadata() {
  # user feature 除组合入口 fx.go 外，不在业务分层结构中暴露 Fx/Dig 元数据。
  local pattern='go\.uber\.org/(fx|dig)|(^|[^[:alnum:]_])(fx|dig)\.(In|Out)([^[:alnum:]_]|$)|`[^`]*(name|optional):"'
  local file output status
  while IFS= read -r file; do
    set +e
    output="$(rg -n "${pattern}" "${file}" 2>&1)"
    status=$?
    set -e
    case "${status}" in
      0)
        report "user feature production code must not carry Fx/Dig DI metadata outside composition: ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
      1)
        ;;
      *)
        report "user feature Fx/Dig metadata scan failed for ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
    esac
  done < <(
    find "${service_dir}/internal/features/user" \
      -type f -name '*.go' \
      ! -name '*_test.go' \
      ! -name 'fx.go' \
      ! -name 'mock_generate.go' \
      \( -path '*/application/*' -o -path '*/domain/*' -o -path '*/infrastructure/*' -o -path '*/transport/*' \) \
      -print
  )
}

check_role_feature_framework_metadata() {
  # role feature 除组合入口 fx.go 外，不在业务分层结构中暴露 Fx/Dig 元数据。
  local pattern='go\.uber\.org/(fx|dig)|(^|[^[:alnum:]_])(fx|dig)\.(In|Out)([^[:alnum:]_]|$)|`[^`]*(name|optional):"'
  local file output status
  while IFS= read -r file; do
    set +e
    output="$(rg -n "${pattern}" "${file}" 2>&1)"
    status=$?
    set -e
    case "${status}" in
      0)
        report "role feature production code must not carry Fx/Dig DI metadata outside composition: ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
      1)
        ;;
      *)
        report "role feature Fx/Dig metadata scan failed for ${file#"${repo_root}/"}"$'\n'"${output}"
        ;;
    esac
  done < <(
    find "${service_dir}/internal/features/role" \
      -type f -name '*.go' \
      ! -name '*_test.go' \
      ! -name 'fx.go' \
      ! -name 'mock_generate.go' \
      \( -path '*/application/*' -o -path '*/domain/*' -o -path '*/infrastructure/*' -o -path '*/transport/*' \) \
      -print
  )
}

check_environment_variable_config_removed() {
  # 环境变量只允许选择 Nacos 配置来源；旧本地完整配置文件入口不得回流。
  run_rg_any "production Go code must not retain local config path entrypoints" \
    '\bConfigPath\b|USER_SERVICE_CONFIG|configs/config\.yaml' \
    --glob '*.go' \
    --glob '!*_test.go' \
    --glob '!common/testing/**' \
    --glob '!user-service/internal/persistence/ent/**' \
    --glob '!user-service/docs/**' \
    --glob '!cmd/rbac_bootstrap_super_admin.go' \
    "${repo_root}/common" \
    "${service_dir}"

  run_rg_any "Docker Compose runtime config must not mount local full config or pass --config" \
    'user-service\.local\.yaml|configs/config\.yaml|^[[:space:]]*-[[:space:]]*--config[[:space:]]*$' \
    "${repo_root}/deployments/compose/docker-compose.yml"

  local k8s_manifest_files=()
  local k8s_file
  for k8s_file in \
    "${repo_root}/deployments/k8s/user-service/deployment.yaml" \
    "${repo_root}/deployments/k8s/user-service/rbac-seed-job.yaml"; do
    if [[ -e "${k8s_file}" ]]; then
      k8s_manifest_files+=("${k8s_file}")
    fi
  done
  if [[ "${#k8s_manifest_files[@]}" -gt 0 ]]; then
    run_rg_any "Kubernetes user-service manifests must not mount local full config or pass --config" \
      'runtime-config|config/config\.yaml|^[[:space:]]*-[[:space:]]*--config[[:space:]]*$' \
      "${k8s_manifest_files[@]}"
  fi

  local helm_and_workflow_paths=()
  if [[ -d "${repo_root}/deployments/helm/aegiscore-user-service/templates" ]]; then
    helm_and_workflow_paths+=("${repo_root}/deployments/helm/aegiscore-user-service/templates")
  fi
  if [[ -d "${repo_root}/.github/workflows" ]]; then
    helm_and_workflow_paths+=("${repo_root}/.github/workflows")
  fi
  if [[ "${#helm_and_workflow_paths[@]}" -gt 0 ]]; then
    run_rg_any "Helm user-service templates must not mount local full config or pass --config" \
      'runtime-config|config/config\.yaml|^[[:space:]]*-[[:space:]]*--config[[:space:]]*$|\$\{\{[[:space:]]*env\.' \
      "${helm_and_workflow_paths[@]}"
  fi
}

check_common_metrics_business_semantics() {
  # common metrics 只承载跨服务观测 primitive，不拥有 user-service 或 RBAC 业务指标。
  local common_metrics_dir="${repo_root}/common/runtime/observability/metrics"
  if [[ ! -d "${common_metrics_dir}" ]]; then
    return
  fi
  run_rg "common runtime metrics must not contain service or RBAC business metrics semantics" \
    'Casbin|casbin|permission|role|rbac|user-service|aegiscore_casbin' \
    "${common_metrics_dir}"
}

# 步骤 1：校验 Go 版本在 go.work、各模块和 CI workflow 中保持一致。
check_go_toolchain_version

# 步骤 2：校验 CI 质量门禁只保留一个标准 lint、单测和容器测试入口。
check_ci_quality_workflow

# 步骤 3：校验 Atlas dev PostgreSQL 镜像引用在 Docker、Compose、atlas.hcl 和迁移脚本中一致。
check_atlas_postgres_version

# 步骤 4：校验 Ent 生成代码只暴露在 user-service/internal/persistence/ent 私有边界内。
check_ent_internal_persistence_boundary

# 步骤 5：校验 Helm chart 生产镜像必须使用显式不可变 image.ref，禁止 latest fallback。
check_helm_user_service_immutable_image

# 步骤 6：校验旧本地完整配置文件入口没有回流到 Go 代码、Compose、Kubernetes、Helm 或 workflow。
check_environment_variable_config_removed

# 步骤 7：校验 common metrics 只承载跨服务 primitive，不包含 user-service 或 RBAC 业务语义。
check_common_metrics_business_semantics

# 步骤 8：校验 mock_generate.go 只在 generate build tag 下参与 mockgen。
check_mock_generate_build_tags

# 步骤 9：校验 ForTest/testHook 等测试专用符号不会进入生产 Go 文件。
check_test_only_production_symbols

# 步骤 10：校验 feature 主路径不依赖包级默认 logger 或 context.Background logger。
check_feature_default_logger_dependencies

# 步骤 11：校验 feature application/domain 层不携带 Fx DI 框架元数据。
check_feature_application_domain_fx_metadata

# 步骤 12：校验 user feature 除组合入口外不携带 Fx/Dig 注入元数据。
check_user_feature_framework_metadata

# 步骤 13：校验 role feature 除组合入口外不携带 Fx/Dig 注入元数据。
check_role_feature_framework_metadata

# 步骤 14：校验 feature 不再保留固定 route_registrar.go，路由统一由 router 总装。
if [[ -n "$(find "${service_dir}/internal/features" -type f -name 'route_registrar.go' -print -quit)" ]]; then
  report "fixed feature route_registrar.go files are forbidden; register auth, permission, role and user routes centrally in user-service/internal/router"
fi

# 步骤 15：校验旧 RBAC baseline 包目录已清理，统一使用 internal/shared/rbacbaseline。
if [[ -d "${service_dir}/internal/features/permission/application/rbacbaseline" ]]; then
  report "old permission/application/rbacbaseline package still exists"
fi

# 步骤 16：校验仓库中没有集中式 mock 目录，mock 应放在消费测试附近。
for forbidden_mock_dir in \
  "${repo_root}/mocks" \
  "${repo_root}/testmocks" \
  "${repo_root}/common/mocks" \
  "${repo_root}/common/testmocks" \
  "${service_dir}/mocks" \
  "${service_dir}/testmocks"; do
  if [[ -d "${forbidden_mock_dir}" ]]; then
    report "central mock repository is forbidden: ${forbidden_mock_dir}; put generated mocks next to the consuming feature tests"
  fi
done

# 步骤 17：校验旧 permission/application/rbacbaseline import 已全部迁移。
run_rg "old RBAC baseline import remains" \
  'github\.com/aegiscore/user-service/internal/features/permission/application/rbacbaseline' \
  "${service_dir}/internal"

# 步骤 18：校验旧 user/domain 状态枚举引用已迁移到 shared/identity。
run_rg "old user-domain status reference remains" \
  'userdomain\.UserStatus|github\.com/aegiscore/user-service/internal/features/user/domain.*UserStatus' \
  "${service_dir}/internal/features" "${service_dir}/internal/persistence/ent/schema"

# 步骤 19：校验旧 user/domain/user_status.go 文件不存在，避免状态枚举回到 user feature 私有域。
if [[ -e "${service_dir}/internal/features/user/domain/user_status.go" ]]; then
  report "old user/domain/user_status.go still exists; use internal/shared/identity"
fi

# 步骤 20：校验 auth feature 不直接导入 user domain，用户状态与身份错误通过 shared/identity 消费。
run_rg "auth feature must not import user domain" \
  'github\.com/aegiscore/user-service/internal/features/user/domain' \
  "${service_dir}/internal/features/auth"

# 步骤 21：校验 role feature 不直接导入 user domain，用户身份错误通过 shared/identity 消费。
run_rg "role feature must not import user domain; use shared/identity for user identity errors" \
  'github\.com/aegiscore/user-service/internal/features/user/domain' \
  "${service_dir}/internal/features/role"

# 步骤 22：校验 shared 包不反向依赖任何 feature 包，保持服务内共享内核稳定。
run_rg "shared packages must not import feature packages" \
  'github\.com/aegiscore/user-service/internal/features/' \
  "${service_dir}/internal/shared"

# 步骤 23：校验服务组合层不直接依赖 permission infrastructure 具体实现。
for service_composition_dir in \
  "${service_dir}/internal/providers" \
  "${service_dir}/internal/bootstrap" \
  "${service_dir}/internal/router" \
  "${service_dir}/cmd"; do
  if [[ -d "${service_composition_dir}" ]]; then
    run_rg "service composition must not import permission infrastructure concrete; use permission public contracts" \
      'github\.com/aegiscore/user-service/internal/features/permission/infrastructure/(casbin|redis)' \
      "${service_composition_dir}"
  fi
done

# 步骤 24：校验 internal/shared 下不存在 errors/enums/types/utils/helpers 这类兜底包。
for forbidden_shared_dir in errors enums types utils helpers; do
  if [[ -d "${service_dir}/internal/shared/${forbidden_shared_dir}" ]]; then
    report "internal/shared/${forbidden_shared_dir} is a forbidden root-level catch-all package; put shared errors/enums/types in the owning shared kernel package"
  fi
done

# 步骤 25：校验 shared identity 枚举文件命名保持主体明确，避免泛化为 status.go。
if [[ -e "${service_dir}/internal/shared/identity/status.go" ]]; then
  report "internal/shared/identity/status.go should be named user_status.go to keep shared enum files subject-specific"
fi

# 步骤 26：校验 shared 包不引入运行时、传输层、数据库、Redis、Fx 等禁止依赖。
run_rg "shared packages must not import forbidden runtime or transport dependencies" \
  'github\.com/gin-gonic/gin|github\.com/aegiscore/user-service/internal/persistence/ent(/|")|github\.com/redis/go-redis|database/sql|github\.com/jackc/pgx|go\.uber\.org/fx|github\.com/aegiscore/common/http/response|github\.com/aegiscore/common/contract/response|github\.com/aegiscore/common/runtime/(config|logger|datastore)' \
  "${service_dir}/internal/shared"

# 步骤 27：校验 application/domain/infrastructure 不反向导入 feature HTTP transport DTO 或 controller。
run_rg "application/domain/infrastructure must not import feature HTTP transport DTO/controller packages" \
  'github\.com/aegiscore/user-service/internal/features/.*/transport/http' \
  "${service_dir}"/internal/features/*/application \
  "${service_dir}"/internal/features/*/domain \
  "${service_dir}"/internal/features/*/infrastructure

# 步骤 28：校验 gRPC transport 不复用 HTTP transport 包，避免协议适配层互相耦合。
run_rg "gRPC transport must not import feature HTTP transport packages" \
  'github\.com/aegiscore/user-service/internal/features/.*/transport/http' \
  "${service_dir}"/internal/features/*/transport/grpc

# 步骤 29：校验 OpenAPI 生成物没有未提交漂移，确保注解变更后重新生成。
run_rg_any "OpenAPI generated files have uncommitted drift" \
  '^user-service/docs/(openapi\.go|openapi\.json|openapi\.yaml)$' \
  <(cd "${repo_root}" && git diff --name-only -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml)

# 步骤 30：校验 OpenSpec/OPSX 文档没有遗留默认英文模板或占位内容。
run_rg_any "OpenSpec/OPSX markdown must use Simplified Chinese instead of default English templates" \
  '(<!--|-->|What problem does this solve|Describe what will change|brief description|condition|expected outcome|Overview|Summary|Acceptance Criteria|Migration Notes|Implementation Plan|Rollout Plan|Out of Scope)' \
  "${repo_root}/openspec/specs" \
  "${repo_root}/openspec/changes" \
  "${repo_root}/docs/opsx"

# 步骤 31：校验 Ent 生成物没有未提交漂移，确保 schema 变更后重新生成。
run_rg_any "Ent generated files have uncommitted drift; run make generate and commit generated output" \
  '^user-service/internal/persistence/ent/(client|ent|mutation|runtime|tx|user|user_create|user_delete|user_query|user_update|permission|permission_create|permission_delete|permission_query|permission_update|role|role_create|role_delete|role_query|role_update|rolepermission|rolepermission_create|rolepermission_delete|rolepermission_query|rolepermission_update|userrole|userrole_create|userrole_delete|userrole_query|userrole_update)\.go$|^user-service/internal/persistence/ent/(enttest|hook|migrate|predicate|runtime|user|permission|role|rolepermission|userrole)/' \
  <(cd "${repo_root}" && git diff --name-only -- user-service/internal/persistence/ent)

# 汇总所有步骤发现的违规；只有执行完全部检查后才统一返回失败状态。
if [[ "${failures}" -gt 0 ]]; then
  printf 'architecture-lint: failed with %d issue(s)\n' "${failures}" >&2
  exit 1
fi

printf 'architecture-lint: ok\n'
