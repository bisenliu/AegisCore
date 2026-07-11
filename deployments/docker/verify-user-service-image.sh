#!/usr/bin/env bash
set -euo pipefail

image="${1:-aegiscore-user-services:latest}"
tmpdir="$(mktemp -d)"
container=""

cleanup() {
  if [[ -n "${container}" ]]; then
    docker rm -f "${container}" >/dev/null 2>&1 || true
  fi
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

fail() {
  printf 'user-service image verification failed: %s\n' "$*" >&2
  exit 1
}

user="$(docker image inspect "${image}" --format '{{.Config.User}}')"
[[ "${user}" == "65532:65532" ]] || fail "expected image user 65532:65532, got ${user:-<empty>}"

container="$(docker create "${image}" help)"
docker export "${container}" -o "${tmpdir}/rootfs.tar"
tar -tf "${tmpdir}/rootfs.tar" > "${tmpdir}/rootfs.txt"

required_paths=(
  "app/user-service/bin/user-services"
  "etc/ssl/certs/ca-certificates.crt"
  "usr/share/zoneinfo/Asia/Shanghai"
  "tmp/"
)
for path in "${required_paths[@]}"; do
  grep -Fxq "${path}" "${tmpdir}/rootfs.txt" || fail "required runtime path missing: /${path}"
done

for path in \
  "bin/sh" \
  "busybox/sh" \
  "sbin/apk" \
  "usr/bin/wget" \
  "usr/bin/curl" \
  "bin/grep" \
  "usr/bin/grep" \
  "usr/local/bin/atlas"; do
  if grep -Fxq "${path}" "${tmpdir}/rootfs.txt"; then
    fail "forbidden runtime tool exists: /${path}"
  fi
done

docker cp "${container}:/app/user-service/bin/user-services" "${tmpdir}/user-services"
file_output="$(file "${tmpdir}/user-services")"
if [[ "${file_output}" != *"statically linked"* && "${file_output}" != *"static"* ]]; then
  fail "service binary is not statically linked: ${file_output}"
fi

docker run --rm --read-only --tmpfs /tmp:uid=65532,gid=65532,mode=1777 "${image}" help >/dev/null
docker run --rm --read-only --tmpfs /tmp:uid=65532,gid=65532,mode=1777 "${image}" serve --help >/dev/null
docker run --rm --read-only --tmpfs /tmp:uid=65532,gid=65532,mode=1777 "${image}" rbac --help >/dev/null
docker run --rm --read-only --tmpfs /tmp:uid=65532,gid=65532,mode=1777 "${image}" fxgraph --help >/dev/null
docker run --rm --read-only --tmpfs /tmp:uid=65532,gid=65532,mode=1777 "${image}" healthcheck --help >/dev/null

printf 'user-service image verification passed: %s\n' "${image}"
