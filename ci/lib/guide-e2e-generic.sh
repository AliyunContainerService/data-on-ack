#!/usr/bin/env bash
# 单个指南的通用 e2e 验证：
#   apply 指南内全部 manifest -> 等待关键工作负载就绪 -> 清理。
# 若 manifest 需要 GPU / eRDMA 而集群不具备，则返回 77（SKIP）。
# 返回 0 成功；77 跳过；其余非零为失败。
#
# 用法: guide-e2e-generic.sh <guide-dir> <HAS_GPU:true|false> <HAS_ERDMA:true|false>
set -uo pipefail

guide_dir="${1:?usage: guide-e2e-generic.sh <guide-dir> <HAS_GPU> <HAS_ERDMA>}"
HAS_GPU="${2:-false}"
HAS_ERDMA="${3:-false}"
WAIT_TIMEOUT="${GUIDE_WAIT_TIMEOUT:-15m}"

# 收集 manifest（仅 k8s 资源，跳过 Dockerfile/脚本）
mapfile -t MANIFESTS < <(find "${guide_dir}" -maxdepth 1 \( -name '*.yaml' -o -name '*.yml' \) -type f | sort)
if [[ ${#MANIFESTS[@]} -eq 0 ]]; then
  echo "  [e2e] 指南无可部署 manifest，跳过部署验证"
  exit 77
fi

# ---- 前置能力判定 -------------------------------------------------------
needs_gpu="false"; needs_erdma="false"
for m in "${MANIFESTS[@]}"; do
  grep -q 'nvidia.com/gpu' "${m}" && needs_gpu="true"
  grep -qE 'aliyun/erdma|aliyun\.com/erdma' "${m}" && needs_erdma="true"
done
if [[ "${needs_gpu}" == "true" && "${HAS_GPU}" != "true" ]]; then
  echo "  [e2e] 指南需要 GPU，但集群无 GPU 节点 -> SKIP"; exit 77
fi
if [[ "${needs_erdma}" == "true" && "${HAS_ERDMA}" != "true" ]]; then
  echo "  [e2e] 指南需要 eRDMA，但集群无 eRDMA 节点 -> SKIP"; exit 77
fi

# ---- 镜像占位符替换 -----------------------------------------------------
# manifest 中的 <your-registry> 由 IMAGE_REGISTRY（CI 变量 image_registry）替换。
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT
APPLY_MANIFESTS=()
needs_sub="false"
for m in "${MANIFESTS[@]}"; do grep -q '<your-registry>' "${m}" && needs_sub="true"; done
if [[ "${needs_sub}" == "true" && -z "${IMAGE_REGISTRY:-}" ]]; then
  echo "  [e2e] manifest 引用 <your-registry> 但未配置 IMAGE_REGISTRY -> SKIP"; exit 77
fi
for m in "${MANIFESTS[@]}"; do
  base="$(basename "${m}")"
  if [[ "${needs_sub}" == "true" ]]; then
    sed "s#<your-registry>#${IMAGE_REGISTRY}#g" "${m}" > "${WORK_DIR}/${base}"
  else
    cp "${m}" "${WORK_DIR}/${base}"
  fi
  APPLY_MANIFESTS+=("${WORK_DIR}/${base}")
done

# ---- imagePullSecret ------------------------------------------------------
# 引用私有镜像仓库的指南需要拉取凭证。凭证优先级：
#   1. 集群内预置的 docker-registry secret（tutorials-regcred）——推荐，凭证
#      一次性存入测试集群，不经过 CI/Agent 上下文；
#   2. 环境变量 IMAGE_REGISTRY_USERNAME / IMAGE_REGISTRY_PASSWORD（YAML 兜底
#      流水线经 secrets envs 注入时使用），存在时创建/刷新同名 secret。
# 两者都没有则 SKIP。secret 最终挂到 default ServiceAccount 上，指南 manifest
# 无需自带 imagePullSecrets。
PULL_SECRET_NAME="tutorials-regcred"
if [[ "${needs_sub}" == "true" ]]; then
  if [[ -n "${IMAGE_REGISTRY_USERNAME:-}" && -n "${IMAGE_REGISTRY_PASSWORD:-}" ]]; then
    registry_host="${IMAGE_REGISTRY%%/*}"
    echo "  [e2e] 用环境变量凭证创建/刷新 imagePullSecret ${PULL_SECRET_NAME} (server=${registry_host})"
    if ! kubectl create secret docker-registry "${PULL_SECRET_NAME}" \
          --docker-server="${registry_host}" \
          --docker-username="${IMAGE_REGISTRY_USERNAME}" \
          --docker-password="${IMAGE_REGISTRY_PASSWORD}" \
          --dry-run=client -o yaml | kubectl apply -f - >/dev/null; then
      echo "  [e2e] imagePullSecret 创建失败"; exit 1
    fi
  elif ! kubectl get secret "${PULL_SECRET_NAME}" >/dev/null 2>&1; then
    echo "  [e2e] 集群内不存在预置的 imagePullSecret ${PULL_SECRET_NAME}，且未提供环境变量凭证 -> SKIP"
    exit 77
  else
    echo "  [e2e] 使用集群内预置的 imagePullSecret ${PULL_SECRET_NAME}"
  fi
  # 绑定到 default ServiceAccount（幂等：merge patch 重复执行结果一致）
  if ! kubectl patch serviceaccount default --type merge \
        -p "{\"imagePullSecrets\":[{\"name\":\"${PULL_SECRET_NAME}\"}]}" >/dev/null; then
    echo "  [e2e] 绑定 imagePullSecret 到 default ServiceAccount 失败"; exit 1
  fi
fi

# ---- 部署 ---------------------------------------------------------------
echo "  [e2e] apply: ${MANIFESTS[*]##*/}"
if ! kubectl apply -f "${APPLY_MANIFESTS[@]}"; then
  echo "  [e2e] kubectl apply 失败"; exit 1
fi

# ---- 等待关键工作负载就绪 ----------------------------------------------
rc=0
# RayService / RayCluster
while IFS= read -r kind_name; do
  [[ -z "${kind_name}" ]] && continue
  echo "  [e2e] 等待 ${kind_name} 就绪 (timeout=${WAIT_TIMEOUT})"
  if ! kubectl wait --for=condition=Ready "${kind_name}" --timeout="${WAIT_TIMEOUT}"; then
    echo "  [e2e] ${kind_name} 未在时限内就绪"; rc=1
  fi
done < <(kubectl get rayservice,raycluster,deployment,statefulset -o jsonpath='{range .items[*]}{.kind}/{.metadata.name}{"\n"}{end}' 2>/dev/null)

# 由本批 manifest 直接创建的裸 Pod（无 owner）
while IFS= read -r pod; do
  [[ -z "${pod}" ]] && continue
  echo "  [e2e] 等待 Pod ${pod} 就绪"
  if ! kubectl wait --for=condition=Ready "pod/${pod}" --timeout="${WAIT_TIMEOUT}"; then
    echo "  [e2e] Pod ${pod} 未就绪"; rc=1
  fi
done < <(kubectl get pods -o jsonpath='{range .items[?(@.metadata.ownerReferences==null)]}{.metadata.name}{"\n"}{end}' 2>/dev/null)

# ---- 清理 ---------------------------------------------------------------
echo "  [e2e] 清理已部署资源"
kubectl delete -f "${APPLY_MANIFESTS[@]}" --ignore-not-found=true --wait=false >/dev/null 2>&1 || true

exit "${rc}"
