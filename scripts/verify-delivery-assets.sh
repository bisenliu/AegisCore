#!/usr/bin/env bash
set -euo pipefail

readonly repo_root="$(git rev-parse --show-toplevel)"
readonly chart_dir="${repo_root}/deployments/helm/aegiscore-user-service"
readonly k8s_dir="${repo_root}/deployments/k8s/user-service"
readonly compose_file="${repo_root}/deployments/compose/docker-compose.yml"
readonly alerts_dir="${repo_root}/deployments/observability/prometheus"
readonly image_ref="example.com/aegiscore-user-service:sha-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

# require_tool 校验外部命令是否存在，缺失时立即失败并提示本地/CI 需要安装的工具。
require_tool() {
  local tool="$1"
  if ! command -v "${tool}" >/dev/null 2>&1; then
    printf 'delivery verification requires %s on PATH\n' "${tool}" >&2
    exit 127
  fi
}

# render_helm_runtime 渲染不包含 RBAC seed Job 的 Helm runtime manifest。
render_helm_runtime() {
  local output="$1"
  helm template aegiscore-user-service "${chart_dir}" \
    --values "${chart_dir}/values.yaml" \
    --set-string "image.ref=${image_ref}" \
    --set rbacSeedJob.enabled=false \
    > "${output}"
}

# render_helm_seed 渲染发布前置 RBAC seed Job manifest，用于 schema 和契约校验。
render_helm_seed() {
  local output="$1"
  helm template aegiscore-user-service "${chart_dir}" \
    --values "${chart_dir}/values.yaml" \
    --set-string "image.ref=${image_ref}" \
    --set rbacSeedJob.enabled=true \
    --set-string rbacSeedJob.nameSuffix=rbac-seed-ci \
    --show-only templates/rbac-seed-job.yaml \
    > "${output}"
}

# assert_contains 校验文件内容包含指定正则；用于表达必须存在的部署契约。
assert_contains() {
  local file="$1"
  local pattern="$2"
  if ! grep -Eq -- "${pattern}" "${file}"; then
    printf '%s does not contain required pattern: %s\n' "${file#"${repo_root}/"}" "${pattern}" >&2
    exit 1
  fi
}

# assert_not_contains 校验文件内容不包含指定正则；用于阻断 runtime manifest 混入发布 Job。
assert_not_contains() {
  local file="$1"
  local pattern="$2"
  if grep -Eq -- "${pattern}" "${file}"; then
    printf '%s contains forbidden pattern: %s\n' "${file#"${repo_root}/"}" "${pattern}" >&2
    exit 1
  fi
}

require_tool docker
require_tool helm
require_tool kubeconform
require_tool kubectl
require_tool promtool

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

readonly helm_runtime="${tmp_dir}/helm-runtime.yaml"
readonly helm_seed="${tmp_dir}/helm-rbac-seed.yaml"
readonly kustomize_runtime="${tmp_dir}/kustomize-runtime.yaml"
readonly k8s_seed="${k8s_dir}/rbac-seed-job.yaml"

printf '==> helm lint\n'
helm lint "${chart_dir}" --set-string "image.ref=${image_ref}" --set rbacSeedJob.enabled=true

printf '==> helm template\n'
render_helm_runtime "${helm_runtime}"
render_helm_seed "${helm_seed}"
assert_not_contains "${helm_runtime}" '^kind: Job$'
assert_contains "${helm_seed}" '^kind: Job$'

printf '==> kustomize render\n'
kubectl kustomize "${k8s_dir}" > "${kustomize_runtime}"

printf '==> kubernetes schema validation\n'
kubeconform -strict -summary \
  -schema-location default \
  -schema-location 'https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/{{.NormalizedKubernetesVersion}}-standalone-strict/{{.ResourceKind}}{{.KindSuffix}}.json' \
  "${helm_runtime}" "${helm_seed}" "${kustomize_runtime}" "${k8s_seed}"

printf '==> static and helm contract checks\n'
for manifest in "${helm_runtime}" "${kustomize_runtime}"; do
  assert_contains "${manifest}" 'kind: Deployment'
  assert_contains "${manifest}" 'replicas: 2|minReplicas: 2'
  assert_contains "${manifest}" 'name: GOMEMLIMIT'
  assert_contains "${manifest}" 'value: "?384MiB"?'
  assert_contains "${manifest}" 'topologyKey: kubernetes.io/hostname'
  assert_contains "${manifest}" 'topologyKey: topology.kubernetes.io/zone'
  assert_contains "${manifest}" 'kind: PodDisruptionBudget'
  assert_contains "${manifest}" 'maxUnavailable: 1'
  assert_contains "${manifest}" 'kind: HorizontalPodAutoscaler'
  assert_contains "${manifest}" 'maxReplicas: 6'
done

for manifest in "${helm_seed}" "${k8s_seed}"; do
  assert_contains "${manifest}" 'kind: Job'
  assert_contains "${manifest}" 'name: GOMEMLIMIT'
  assert_contains "${manifest}" 'value: "?384MiB"?'
done

printf '==> docker compose config\n'
docker compose -f "${compose_file}" config --quiet

printf '==> prometheus rules\n'
(
  cd "${alerts_dir}"
  promtool check rules user-service-alerts.yaml
  promtool test rules user-service-alerts.test.yaml
)

printf '==> dashboard drift\n'
"${repo_root}/deployments/compose/scripts/generate-grafana-dashboard.sh" --check

printf 'delivery assets verification: ok\n'
