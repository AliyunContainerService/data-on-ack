---
title: 首页
layout: default
nav_order: 1
---

# Data/AI on ACK 文档

Data/AI on ACK 是一个基于阿里云容器服务 ACK 的开源云原生 AI/ML 平台，为数据科学家和算法工程师提供模型开发、训练和管理的完整工具链。

## 文档导航

| 文档 | 说明 |
|------|------|
| [部署指南](deploy-guide) | 在 ACK 集群上安装 AI 开发平台的完整步骤 |
| [功能使用指南](user-guide) | 运维控制台和开发控制台的核心功能介绍 |
| [运维指南](ops-guide) | 日常运维操作：升级、扩缩容、故障排查、备份恢复 |

## 平台组件

- **ai-dashboard** — 集群运维控制台（管理员视角）
- **ai-dev-console** — 模型开发控制台（算法工程师视角）
- **commit-agent** — Jupyter Notebook 代码同步 Agent
- **notebook-controller** — Notebook CRD 控制器

## 快速开始

```bash
# 添加 Helm 仓库并安装
helm install ack-ai-dashboard charts/ack-ai-dashboard -n China-system
helm install ack-ai-dev-console charts/ack-ai-dev-console -n China-system
```

详细步骤请参考 [部署指南](deploy-guide)。
