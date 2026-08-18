#!/usr/bin/env bash
# 端到端验证 ray/（tutorials）下的全部指南。
#
# 对每个指南目录依次执行：
#   1. 结构校验（必过）：README 存在、引用的 manifest/脚本/镜像引用合法、YAML 可解析。
#   2. e2e 部署验证：优先运行 ci/guides/<slug>.sh 专用钩子；否则走通用流程
#      （apply 指南内 manifest -> 等待关键资源就绪 -> 冒烟 -> 清理）。
#      若集群缺少该指南所需能力（GPU / eRDMA / OSS 等），标记 SKIP 并说明原因，
#      不计为失败；其余任何失败都会使本脚本以非零退出，从而让流水线失败。
#
# 依赖：kubectl 已安装、KUBECONFIG 已就绪（见 ci/lib/*.sh）。
set -uo pipefail

GUIDE_ROOT="${GUIDE_ROOT:-ray}"
CI_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${CI_DIR}/lib"
GUIDES_HOOK_DIR="${CI_DIR}/guides"

# 结果统计
declare -A RESULT   # guide -> PASS/FAIL/SKIP
declare -A REASON   # guide -> 说明

log()  { printf '\n\033[1;34m== %s\033[0m\n' "$*"; }
ok()   { printf '  \033[32m[PASS]\033[0m %s\n' "$*"; }
bad()  { printf '  \033[31m[FAIL]\033[0m %s\n' "$*"; }
skip() { printf '  \033[33m[SKIP]\033[0m %s\n' "$*"; }

# ---- 集群能力探测（用于判定指南前置条件） -------------------------------
HAS_GPU="false"; HAS_ERDMA="false"
if kubectl get nodes -o json 2>/dev/null | grep -q '"nvidia.com/gpu"'; then HAS_GPU="true"; fi
if kubectl get nodes -o json 2>/dev/null | grep -q '"aliyun/erdma"'; then HAS_ERDMA="true"; fi
log "集群能力探测: GPU=${HAS_GPU} eRDMA=${HAS_ERDMA}"

# ---- 发现全部指南目录（含 README.md 的目录） ---------------------------
mapfile -t GUIDES < <(find "${GUIDE_ROOT}" -name README.md -type f | sort)
if [[ ${#GUIDES[@]} -eq 0 ]]; then
  echo "未在 ${GUIDE_ROOT} 下发现任何指南（README.md）"; exit 1
fi
log "发现 ${#GUIDES[@]} 个指南"

# ---- 逐个指南验证 -------------------------------------------------------
for readme in "${GUIDES[@]}"; do
  guide_dir="$(dirname "${readme}")"
  slug="${guide_dir//\//_}"          # e.g. ray_2-advanced_gpu-sharing-with-cgpu
  name="${guide_dir#${GUIDE_ROOT}/}" # e.g. 2-advanced/gpu-sharing-with-cgpu
  log "指南: ${name}"

  # 1) 结构校验
  if ! bash "${LIB_DIR}/guide-structural-check.sh" "${guide_dir}"; then
    RESULT[$slug]="FAIL"; REASON[$slug]="结构校验未通过"; bad "${name} 结构校验失败"; continue
  fi
  ok "${name} 结构校验通过"

  # 2) e2e 部署验证
  hook="${GUIDES_HOOK_DIR}/${slug}.sh"
  if [[ -f "${hook}" ]]; then
    # 专用钩子：钩子自身负责前置条件判定，返回 77 表示 SKIP
    set +e; bash "${hook}"; rc=$?; set -e 2>/dev/null || set +e
    case "${rc}" in
      0)  RESULT[$slug]="PASS"; REASON[$slug]="专用钩子验证通过"; ok "${name} e2e 通过" ;;
      77) RESULT[$slug]="SKIP"; REASON[$slug]="钩子判定前置条件不满足"; skip "${name} 跳过（前置条件不满足）" ;;
      *)  RESULT[$slug]="FAIL"; REASON[$slug]="专用钩子失败 rc=${rc}"; bad "${name} e2e 失败(rc=${rc})" ;;
    esac
    continue
  fi

  # 通用 e2e：apply -> 等待 -> 清理
  set +e; bash "${LIB_DIR}/guide-e2e-generic.sh" "${guide_dir}" "${HAS_GPU}" "${HAS_ERDMA}"; rc=$?; set +e
  case "${rc}" in
    0)  RESULT[$slug]="PASS"; REASON[$slug]="通用 e2e 通过"; ok "${name} e2e 通过" ;;
    77) RESULT[$slug]="SKIP"; REASON[$slug]="集群缺少所需能力"; skip "${name} 跳过（集群缺少所需能力）" ;;
    *)  RESULT[$slug]="FAIL"; REASON[$slug]="通用 e2e 失败 rc=${rc}"; bad "${name} e2e 失败(rc=${rc})" ;;
  esac
done

# ---- 汇总 ---------------------------------------------------------------
log "验证汇总"
pass=0; fail=0; skipped=0
for slug in "${!RESULT[@]}"; do
  case "${RESULT[$slug]}" in
    PASS) pass=$((pass+1));;
    FAIL) fail=$((fail+1));;
    SKIP) skipped=$((skipped+1));;
  esac
done
echo "PASS=${pass}  FAIL=${fail}  SKIP=${skipped}  (共 ${#RESULT[@]})"
for slug in "${!RESULT[@]}"; do
  printf '  %-8s %s — %s\n' "${RESULT[$slug]}" "${slug}" "${REASON[$slug]}"
done

if [[ ${fail} -gt 0 ]]; then
  echo "存在失败的指南，流水线判定为失败"; exit 1
fi
echo "全部指南验证通过（SKIP 不计为失败）"; exit 0
