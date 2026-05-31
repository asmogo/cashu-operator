#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cluster_name="${CLUSTER_NAME:-cashu-operator-dev}"
cluster_context="${CLUSTER_CONTEXT:-k3d-${cluster_name}}"
processor_dir="${ARKADE_PROCESSOR_DIR:-/Users/asm/git/cashu-arkade-lightning-procesor}"
image="${ARKADE_PROCESSOR_IMAGE:-cashu-arkade-lightning-procesor:dev}"
dockerfile="${ARKADE_PROCESSOR_DOCKERFILE:-${repo_root}/hack/arkade-processor-local.Dockerfile}"

if [[ ! -d "${processor_dir}" ]]; then
  echo "Arkade processor repo not found: ${processor_dir}" >&2
  exit 1
fi

node_arch="$(kubectl --context "${cluster_context}" get node -o jsonpath='{.items[0].status.nodeInfo.architecture}' 2>/dev/null || true)"
case "${node_arch}" in
  amd64 | arm64)
    platform="linux/${node_arch}"
    ;;
  *)
    platform="${ARKADE_PROCESSOR_PLATFORM:-linux/arm64}"
    ;;
esac

git -C "${processor_dir}" submodule update --init --recursive
docker build --platform "${platform}" -t "${image}" -f "${dockerfile}" "${processor_dir}"
k3d image import "${image}" -c "${cluster_name}"
