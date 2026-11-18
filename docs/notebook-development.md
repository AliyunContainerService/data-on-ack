---
title: Notebook开发环境
layout: default
nav_order: 3
has_children: true
---

# Notebook开发环境

本部分介绍如何在 Kubernetes 集群中安装 Notebook Controller，通过 `Notebook` 自定义资源创建 JupyterLab 或 Code Server 开发环境，并配置开发环境的访问入口。

- [安装 Notebook Controller](notebook-controller-install.html) — 使用 Helm 安装 controller 并确认组件就绪
- [创建 Notebook 开发环境](create-notebook.html) — 使用 `kubectl` 创建 JupyterLab 或 Code Server 开发环境
- [访问 Notebook 开发环境](access-notebook.html) — 使用 `kubectl port-forward` 或 ALB Ingress 访问开发环境
