#!/usr/bin/env bash
#
# 从通用可观测性 dashboard 生成 Docker Compose 使用的 Grafana dashboard。
#
# 通用 dashboard 使用 ${DS_PROMETHEUS} 作为 Prometheus datasource uid，
# 方便普通 Grafana 导入时选择数据源。Compose provisioning 创建的数据源
# uid 固定为 "prometheus"，所以生成产物只改写 Prometheus datasource 引用，
# 不改查询表达式和面板结构。
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SOURCE="${SOURCE:-$ROOT_DIR/deployments/observability/grafana/user-service-overview.json}"
TARGET="${TARGET:-$ROOT_DIR/deployments/compose/grafana/dashboards/user-service-overview.json}"
DATASOURCE_UID="${DATASOURCE_UID:-prometheus}"
MODE="write"

usage() {
  cat <<USAGE
从通用 dashboard 生成本地 Docker Compose Grafana dashboard。

Usage:
  $(basename "$0") [--check]

Environment:
  SOURCE          通用 dashboard 路径。默认: $SOURCE
  TARGET          生成的 Compose dashboard 路径。默认: $TARGET
  DATASOURCE_UID  Compose Prometheus datasource uid。默认: $DATASOURCE_UID

Options:
  --check         TARGET 未同步时失败。
  -h, --help      显示帮助。
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)
      MODE="check"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! command -v jq >/dev/null 2>&1; then
  echo "生成 Compose Grafana dashboard 需要 jq。" >&2
  exit 1
fi

if [[ ! -f "$SOURCE" ]]; then
  echo "通用 dashboard 不存在: $SOURCE" >&2
  exit 1
fi

tmp="$(mktemp "${TMPDIR:-/tmp}/aegiscore-compose-dashboard.XXXXXX.json")"
trap 'rm -f "$tmp"' EXIT

jq --arg uid "$DATASOURCE_UID" '
  def walk(f):
    . as $in
    | if type == "object" then
        reduce keys_unsorted[] as $key ({}; . + {($key): ($in[$key] | walk(f))}) | f
      elif type == "array" then
        map(walk(f)) | f
      else
        f
      end;

  walk(
    if type == "object"
      and (.datasource? | type == "object")
      and (.datasource.type? == "prometheus")
    then .datasource.uid = $uid
    else .
    end
  )
' "$SOURCE" > "$tmp"

jq empty "$tmp" >/dev/null

if [[ "$MODE" == "check" ]]; then
  if [[ ! -f "$TARGET" ]]; then
    echo "generated dashboard is missing: $TARGET" >&2
    exit 1
  fi

  if cmp -s "$tmp" "$TARGET"; then
    echo "Compose Grafana dashboard 已同步。"
    exit 0
  fi

  echo "Compose Grafana dashboard 未同步。请执行: make compose-dashboard-generate" >&2
  exit 1
fi

mkdir -p "$(dirname "$TARGET")"
mv "$tmp" "$TARGET"
trap - EXIT

echo "已从 $SOURCE 生成 $TARGET"
