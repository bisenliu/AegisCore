#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
readonly repo_root
readonly workflow_dir="${repo_root}/.github/workflows"
readonly sha_ref_regex='^[0-9a-f]{40}$'

# 派生项目可以移除 GitHub 配置，此时没有需要校验的 Action。
if [[ ! -d "${workflow_dir}" ]]; then
  printf 'github-actions-pins-check: skipped (no workflows directory)\n'
  exit 0
fi

# 先收集文件，让扫描失败直接中止，避免被进程替换掩盖为成功。
workflow_files="$(mktemp)"
trap 'rm -f "${workflow_files}"' EXIT
find "${workflow_dir}" -type f \( -name '*.yml' -o -name '*.yaml' \) -print0 > "${workflow_files}"

failed=0

while IFS= read -r -d '' workflow; do
  line_number=0
  while IFS= read -r line || [[ -n "${line}" ]]; do
    line_number=$((line_number + 1))
    uses_value="${line#*uses: }"
    if [[ "${uses_value}" == "${line}" ]]; then
      continue
    fi

    uses_value="${uses_value%%#*}"
    uses_value="${uses_value#\'}"
    uses_value="${uses_value%\'}"
    uses_value="${uses_value#\"}"
    uses_value="${uses_value%\"}"
    uses_value="${uses_value//[[:space:]]/}"

    if [[ "${uses_value}" == ./* ]]; then
      continue
    fi

    if [[ "${uses_value}" != *@* ]]; then
      printf '%s:%d uses action without explicit ref: %s\n' "${workflow#"${repo_root}/"}" "${line_number}" "${uses_value}" >&2
      failed=1
      continue
    fi

    action_ref="${uses_value##*@}"
    if [[ ! "${action_ref}" =~ ${sha_ref_regex} ]]; then
      printf '%s:%d external action must pin a 40-character commit SHA: %s\n' "${workflow#"${repo_root}/"}" "${line_number}" "${uses_value}" >&2
      failed=1
    fi
  done < "${workflow}"
done < "${workflow_files}"

exit "${failed}"
