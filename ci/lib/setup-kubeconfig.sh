#!/usr/bin/env bash
# 从 CI secret 注入的 KUBECONFIG_B64 还原 kubeconfig，供后续 kubectl 使用。
# KUBECONFIG_B64 是测试集群 kubeconfig 文件的 base64 单行编码，
# 由 pipeline 通过 ${{secrets.TUTORIALS_KUBECONFIG_B64}} 注入为本环境变量。
set -euo pipefail

: "${KUBECONFIG_B64:?KUBECONFIG_B64 未设置（需在 CI secret TUTORIALS_KUBECONFIG_B64 中配置 base64 单行 kubeconfig）}"

KUBECONFIG_DIR="${HOME}/.kube"
mkdir -p "${KUBECONFIG_DIR}"
echo "${KUBECONFIG_B64}" | base64 -d > "${KUBECONFIG_DIR}/config"
chmod 600 "${KUBECONFIG_DIR}/config"
export KUBECONFIG="${KUBECONFIG_DIR}/config"

# 验证连通性
kubectl cluster-info >/dev/null
echo ">>> 测试集群连通正常"
kubectl get nodes --no-headers | awk '{print "node:", $1, $2}'
