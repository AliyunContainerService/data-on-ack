---
title: 安装 Notebook Controller
layout: default
parent: Notebook开发环境
nav_order: 1
description: 使用 Helm 安装 Notebook Controller，并确认 CRD 和 controller 已就绪。
---

# 安装 Notebook Controller

Notebook Controller 监听集群中的 `Notebook` 自定义资源，并为每个 Notebook 创建一个 StatefulSet 和一个 ClusterIP Service。

> **提示**：Notebook Controller 只负责调谐 Notebook 及其工作负载，不包含可视化管理、用户认证和租户授权能力。需要这些能力时，请参考[开发控制台部署指南](development-console-deploy-guide.html)和[功能使用指南中的 Notebook 管理](development-console-user-guide.html#notebook-管理)。

## 前提条件

- 已安装 Helm 3。
- 已安装 `kubectl`，且当前 context 指向目标 Kubernetes 集群。
- 当前用户可以创建 CRD、ClusterRole 等集群级资源。
- 集群节点能够拉取 `registry-cn-beijing.ack.aliyuncs.com` 中的 controller 镜像。

先确认当前操作的集群，避免安装到错误的环境：

```bash
kubectl config current-context
kubectl cluster-info
```

## 使用 Helm 安装

克隆 Data on ACK 仓库：

```bash
git clone https://github.com/AliyunContainerService/data-on-ack.git
cd data-on-ack
```

安装 Notebook Controller chart：

```bash
helm upgrade --install notebook-controller ./charts/notebook-controller \
  --namespace notebook-controller-system \
  --create-namespace \
  --wait \
  --timeout 5m
```

一个集群只需要安装一个 Notebook Controller。Controller 会监听所有 namespace 中的 `Notebook` 资源。

## 确认安装就绪

等待 Notebook CRD 建立完成：

```bash
kubectl wait \
  --for=condition=Established \
  crd/notebooks.kubeflow.org \
  --timeout=60s
```

等待 controller Deployment 完成滚动发布：

```bash
kubectl rollout status \
  deployment/notebook-controller-deployment \
  --namespace notebook-controller-system \
  --timeout=2m
```

查看 Helm release 和 controller Pod：

```bash
helm status notebook-controller \
  --namespace notebook-controller-system

kubectl get deployment,pod,service \
  --namespace notebook-controller-system
```

正常情况下，Helm release 的状态为 `deployed`，Deployment 的 `READY` 为 `1/1`，controller Pod 的状态为 `Running`。

最后确认 API Server 已提供 Notebook API：

```bash
kubectl api-resources --api-group=kubeflow.org | grep -w Notebook
```

预期可以看到 `notebooks` 资源以及 `kubeflow.org/v1` API 版本。接下来可以按照[创建 Notebook 开发环境](create-notebook.html)创建开发环境。

## 排查安装问题

如果 Deployment 未就绪，查看 Pod 状态、事件和 controller 日志：

```bash
kubectl get pods \
  --namespace notebook-controller-system \
  --output=wide

kubectl describe deployment notebook-controller-deployment \
  --namespace notebook-controller-system

kubectl logs \
  deployment/notebook-controller-deployment \
  --namespace notebook-controller-system \
  --tail=200
```

常见原因包括镜像无法拉取、当前用户没有创建集群级 RBAC 的权限，或者集群中已经运行了另一个 Notebook Controller。
