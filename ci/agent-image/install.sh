#!/usr/bin/env bash
# 在镜像构建阶段安装 prepare-context.sh 预下载的工具，并校验 Agent CLI 仍可用。
# 该脚本在 Dockerfile 的 RUN 中执行，不访问网络。
set -euo pipefail

src="/tmp/agent-image-bin"

# RUN 以基础镜像默认（非 root）用户执行，/usr/local/bin 不可写。
# 装到 claude CLI 所在目录（如 /opt/node/bin）——该目录由当前用户拥有、
# 本就可写且已在 PATH 上；claude 不存在时退回用户级 ~/.local/bin。
if command -v claude >/dev/null 2>&1; then
  dst="$(dirname "$(command -v claude)")"
else
  dst="$HOME/.local/bin"
fi
mkdir -p "$dst"
echo ">>> 安装目标目录: $dst"

install_bin() {
  local name="$1"
  if [ -f "$src/$name" ]; then
    install -m 0755 "$src/$name" "$dst/$name"
    echo ">>> installed $name -> $dst/$name"
  else
    echo ">>> skip $name（构建上下文中不存在）"
  fi
}

install_bin kubectl
install_bin helm

echo "=== 版本校验 ==="
kubectl version --client --output=yaml || true
helm version --short || true
command -v jq >/dev/null 2>&1 && jq --version || echo "jq: 未安装"
python3 -c 'import yaml; print("PyYAML", yaml.__version__)' || echo "PyYAML: 未安装"

# Agent CLI 必须存在，否则该镜像无法用于 Agentic Pipeline 的 agent-execution 阶段。
echo "=== Agent CLI 校验 ==="
if command -v claude >/dev/null 2>&1; then
  claude --version || true
else
  echo "错误：镜像内找不到 claude CLI，Agentic Pipeline 会在 agent-execution 阶段失败" >&2
  exit 1
fi
