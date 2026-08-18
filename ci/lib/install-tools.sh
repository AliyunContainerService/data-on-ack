#!/usr/bin/env bash
# 安装验证所需的命令行工具：kubectl 与 helm。
# aone-hosted runner 默认镜像不保证带这些工具，这里显式安装到 /usr/local/bin。
set -euo pipefail

KUBECTL_VERSION="${KUBECTL_VERSION:-v1.30.5}"
HELM_VERSION="${HELM_VERSION:-v3.15.4}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo ">>> installing kubectl ${KUBECTL_VERSION}"
  curl -fsSL -o /tmp/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
  chmod +x /tmp/kubectl
  mv /tmp/kubectl /usr/local/bin/kubectl
fi

if ! command -v helm >/dev/null 2>&1; then
  echo ">>> installing helm ${HELM_VERSION}"
  curl -fsSL -o /tmp/helm.tgz "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz"
  tar -xzf /tmp/helm.tgz -C /tmp
  mv /tmp/linux-amd64/helm /usr/local/bin/helm
  rm -rf /tmp/helm.tgz /tmp/linux-amd64
fi

# guide-structural-check.sh 用 python3 + PyYAML 做离线 manifest 解析
if ! python3 -c 'import yaml' >/dev/null 2>&1; then
  echo ">>> installing PyYAML"
  pip3 install --quiet PyYAML
fi

kubectl version --client --output=yaml
helm version --short
