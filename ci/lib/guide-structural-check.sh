#!/usr/bin/env bash
# 对单个指南目录做结构校验：
#   - README.md 存在（调用方已保证）
#   - README 中以相对路径引用的本地文件都存在
#   - 目录内的 *.yaml/*.yml manifest 能被解析（kubectl --dry-run=client）
#   - README 中引用的镜像引用格式合法（粗检）
# 返回 0 表示通过，非零表示失败。
set -uo pipefail

guide_dir="${1:?usage: guide-structural-check.sh <guide-dir>}"
[[ -d "${guide_dir}" ]] || { echo "指南目录不存在: ${guide_dir}"; exit 1; }
readme="${guide_dir}/README.md"
[[ -f "${readme}" ]] || { echo "缺少 README.md: ${readme}"; exit 1; }

fail=0

# 1) README 相对路径引用的本地文件是否存在
#    匹配 [text](path)，排除 http(s)/# 开头与纯锚点。
while IFS= read -r ref; do
  # 去掉可能的 #fragment
  ref="${ref%%#*}"
  [[ -z "${ref}" ]] && continue
  target="${guide_dir}/${ref}"
  if [[ ! -e "${target}" ]]; then
    echo "  [结构] README 引用的文件不存在: ${ref}"
    fail=1
  fi
done < <(grep -oE '\]\(([^)]+)\)' "${readme}" | sed -E 's/^\]\(//; s/\)$//' | grep -vE '^(https?:)?//' | grep -vE '^#')

# 2) 目录内 manifest 可解析（纯离线校验：YAML 语法 + apiVersion/kind/metadata 齐备。
#    不使用 kubectl dry-run，因其需要连接 API Server 拉取 openapi 校验）
shopt -s nullglob
for m in "${guide_dir}"/*.yaml "${guide_dir}"/*.yml; do
  if ! python3 - "${m}" <<'PYEOF' 2>/dev/null
import sys, yaml
path = sys.argv[1]
with open(path) as f:
    docs = list(yaml.safe_load_all(f))
bad = False
for d in docs:
    if d is None:
        continue
    if not isinstance(d, dict) or not all(k in d for k in ("apiVersion", "kind", "metadata")):
        bad = True
sys.exit(1 if bad else 0)
PYEOF
  then
    echo "  [结构] manifest 无法解析或缺少 apiVersion/kind/metadata: ${m}"
    fail=1
  fi
done

# 3) 镜像引用粗检：允许约定的替换占位符 <your-registry>（e2e 阶段会用
#    image_registry 变量替换），但其余未替换的占位符/变量视为问题。
if grep -RhoE 'image:[[:space:]]*[^[:space:]]+' "${guide_dir}" --include='*.yaml' --include='*.yml' 2>/dev/null \
     | sed -E 's/<your-registry>//g' \
     | grep -qE '<[^>]+>|\$\{'; then
  echo "  [结构] manifest 中 image 存在未替换的占位符/变量（<your-registry> 除外），请替换为可用镜像"
  fail=1
fi

exit "${fail}"
