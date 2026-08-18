#!/usr/bin/env bash
# 验证通过后：把验证提交的 ray/（指南）目录树整体镜像到外部（GitHub）
# 不含 CI 配置的分支，并向 upstream 官方仓提交一个 MR。
#
# 不使用 cherry-pick 搬单个提交：CI checkout 为浅克隆时触发提交会被视为
# 根提交，cherry-pick 会把全仓库当新增应用并与外部分支冲突；且外部分支
# 从未同步过历史提交，单提交增量也不完整。改为直接用
# `git checkout <commit> -- ray/` 镜像整个 ray/ 树，无冲突、不依赖历史。
#
# 只搬运 ray/ 路径，确保 .aoneci/ 与 ci/ 等内部 CI 配置不会泄漏到外部分支。
#
# 需要的环境变量（由 pipeline 注入）：
#   COMMIT_ID        触发流水线的提交（${{git.commitId}}）
#   EXTERNAL_BRANCH  外部承接 cherry-pick 的目标分支（${{params.external_branch}}）
#   UPSTREAM_REPO    MR 目标仓库，如 AliyunContainerService/data-on-ack
#   GITHUB_TOKEN     具备 public_repo 权限的 GitHub PAT（${{secrets.TUTORIALS_GITHUB_TOKEN}}）
#   GITHUB_USER      GitHub 用户名 / fork 属主（${{secrets.TUTORIALS_GITHUB_USER}}）
set -euo pipefail

: "${COMMIT_ID:?COMMIT_ID 未设置}"
: "${EXTERNAL_BRANCH:?EXTERNAL_BRANCH 未设置}"
: "${UPSTREAM_REPO:?UPSTREAM_REPO 未设置}"
: "${GITHUB_TOKEN:?GITHUB_TOKEN 未设置（需配置 Aone secret TUTORIALS_GITHUB_TOKEN）}"
: "${GITHUB_USER:?GITHUB_USER 未设置（需配置 Aone secret TUTORIALS_GITHUB_USER）}"

GUIDE_PATH="ray"
REPO_NAME="${UPSTREAM_REPO##*/}"          # data-on-ack
FORK_REPO="${GITHUB_USER}/${REPO_NAME}"   # <user>/data-on-ack
TS="$(date +%Y%m%d%H%M%S)"
WORK_BRANCH="tutorials-sync-${COMMIT_ID:0:8}-${TS}"

# Aone runner 直连 github.com 不稳定（连接易被重置）。publish Job 里的
# setup-github-proxy 组件已向全局 git 配置写入 github.com 专用代理，
# git fetch/push 自动生效；curl 不读 git 配置，这里取出同一代理供
# api.github.com 调用使用。代理地址内嵌凭证，禁止打印。
GH_PROXY="$(git config --global --get http.https://github.com.proxy || true)"
if [ -z "${GH_PROXY}" ]; then
  echo "警告：未检测到 GitHub 代理配置（setup-github-proxy 未生效？），API 调用将直连"
fi

git config user.name  "tutorials-ci"
git config user.email "tutorials-ci@localhost"

# 1) 添加 GitHub fork 远端（带 token），拉取外部分支
FORK_URL="https://${GITHUB_USER}:${GITHUB_TOKEN}@github.com/${FORK_REPO}.git"
git remote add github "${FORK_URL}"
git fetch github "${EXTERNAL_BRANCH}" --depth 50
# 同时确保能访问到触发提交（checkout 已带内部历史）
git cat-file -e "${COMMIT_ID}" || { echo "找不到提交 ${COMMIT_ID}"; exit 1; }

# 2) 基于外部分支建工作分支
git checkout -b "${WORK_BRANCH}" "github/${EXTERNAL_BRANCH}"

# 3) 把验证提交的 ray/ 树整体镜像到工作分支（含新增/修改/删除），
#    只触碰 ray/ 路径，确保 CI 配置不外泄。
git rm -rq --ignore-unmatch "${GUIDE_PATH}/"
git checkout "${COMMIT_ID}" -- "${GUIDE_PATH}/"
git add -A "${GUIDE_PATH}/"
if git diff --cached --quiet; then
  echo "外部分支 ${GUIDE_PATH}/ 已与提交 ${COMMIT_ID} 一致，无需同步"; exit 0
fi
git commit -m "docs(tutorials): sync guide updates from internal (${COMMIT_ID:0:8})"

# 4) 推送到 fork
git push github "${WORK_BRANCH}"

# 5) 通过 GitHub API 创建 MR：fork/WORK_BRANCH -> upstream/EXTERNAL_BRANCH
TITLE="docs(tutorials): sync guide updates (${COMMIT_ID:0:8})"
BODY=$(cat <<EOF
由内部 tutorials-validate 流水线在端到端验证通过后自动同步。

- 源提交（内部）: \`${COMMIT_ID}\`
- 同步范围: \`${GUIDE_PATH}/\`
- 验证: 全部指南端到端验证通过
EOF
)
PAYLOAD="$(python3 - "$TITLE" "$BODY" "${GITHUB_USER}:${WORK_BRANCH}" "$EXTERNAL_BRANCH" <<'PY'
import json,sys
title,body,head,base=sys.argv[1:5]
print(json.dumps({"title":title,"body":body,"head":head,"base":base}))
PY
)"
RESP="$(curl -sS ${GH_PROXY:+--proxy "${GH_PROXY}"} -X POST \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -d "${PAYLOAD}" \
  "https://api.github.com/repos/${UPSTREAM_REPO}/pulls")"
PR_URL="$(echo "${RESP}" | python3 -c 'import sys,json;print(json.load(sys.stdin).get("html_url",""))')"
if [ -z "${PR_URL}" ]; then
  echo "创建 MR 失败，GitHub API 返回："; echo "${RESP}"; exit 1
fi
echo ">>> MR 已创建: ${PR_URL}"
