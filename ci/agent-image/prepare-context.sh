#!/usr/bin/env bash
# 在 runner 侧准备镜像构建上下文：把 kubectl / helm 二进制下载到
# ci/agent-image/bin/，供 Dockerfile COPY 使用（构建机可能无公网出口）。
#
# 用法：bash ci/agent-image/prepare-context.sh
# 环境变量：KUBECTL_VERSION / HELM_VERSION 可覆盖版本。
set -euo pipefail

# 基础镜像（Debian 13）已自带 jq 与 PyYAML，这里只需补 kubectl 与 helm。
KUBECTL_VERSION="${KUBECTL_VERSION:-v1.30.5}"
HELM_VERSION="${HELM_VERSION:-v3.15.4}"

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bin_dir="$script_dir/bin"
mkdir -p "$bin_dir"

# 依次尝试多个下载源，任一成功即返回。
fetch() {
  local out="$1"
  shift
  local url
  for url in "$@"; do
    echo ">>> try: $url"
    if curl -fsSL --connect-timeout 10 --max-time 600 -o "$out.tmp" "$url"; then
      mv "$out.tmp" "$out"
      echo ">>> ok: $url"
      return 0
    fi
    rm -f "$out.tmp"
  done
  return 1
}

echo "=== kubectl ${KUBECTL_VERSION} ==="
fetch "$bin_dir/kubectl" \
  "https://files.m.daocloud.io/dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
  "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
  "https://storage.googleapis.com/kubernetes-release/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
chmod +x "$bin_dir/kubectl"

echo "=== helm ${HELM_VERSION} ==="
fetch "$bin_dir/helm.tgz" \
  "https://mirrors.huaweicloud.com/helm/${HELM_VERSION}/helm-${HELM_VERSION}-linux-amd64.tar.gz" \
  "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz"
tar -xzf "$bin_dir/helm.tgz" -C "$bin_dir" linux-amd64/helm
mv "$bin_dir/linux-amd64/helm" "$bin_dir/helm"
rm -rf "$bin_dir/linux-amd64" "$bin_dir/helm.tgz"
chmod +x "$bin_dir/helm"

echo "=== 构建上下文内容 ==="
ls -lh "$bin_dir"
