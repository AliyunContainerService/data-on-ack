---
title: 运维指南
layout: default
parent: 云原生AI套件
nav_order: 3
---

# 运维指南

本文介绍 data-on-ack AI 平台的日常运维操作，包括升级、扩缩容、故障排查和备份恢复。

## 组件清单

| 组件 | Deployment 名称 | 端口 | 健康检查 |
|------|-----------------|------|----------|
| 运维控制台 | ack-ai-dashboard-admin-ui | 8080 | `/health` |
| 开发控制台 | ack-ai-dev-console | 9090 | `/health` |
| Grafana | ack-ai-dashboard-grafana | 3000 | `/api/health` |
| Notebook Controller | notebook-controller-deployment | 8443 | `/healthz` |
| Commit Agent | ack-commit-agent (DaemonSet) | 9090 | `ack-commit-ctl version` |

## 升级

### Helm 升级

```bash
# 升级运维控制台到新版本
helm upgrade ai-dashboard charts/ack-ai-dashboard -n kube-ai \
  --set admin-ui.image.tag=<NEW_TAG> \
  --reuse-values

# 升级开发控制台到新版本
helm upgrade ai-dev-console charts/ack-ai-dev-console -n kube-ai \
  --set dev-console.image.tag=<NEW_TAG> \
  --reuse-values
```

### 自行构建新版本

```bash
# 拉取最新代码
git pull origin main

# 构建新镜像
cd ai-dashboard && docker build -t <registry>/ai-dashboard:<tag> .
cd ai-dev-console && docker build -f Dockerfile.console-new -t <registry>/ai-dev-console:<tag> .

# 推送并升级
docker push <registry>/ai-dashboard:<tag>
docker push <registry>/ai-dev-console:<tag>
helm upgrade ... --set admin-ui.image.tag=<tag>
```

## 扩缩容

```bash
# 控制台副本数
kubectl scale deploy ack-ai-dashboard-admin-ui -n kube-ai --replicas=2
kubectl scale deploy ack-ai-dev-console -n kube-ai --replicas=2
```

> **注意**：多副本时必须配置 `auth.existingSecret`，否则每个 Pod 生成随机 Session Key 导致登录状态不一致。

## 日志查看

```bash
# 运维控制台日志
kubectl logs -f deploy/ack-ai-dashboard-admin-ui -n kube-ai

# 开发控制台日志
kubectl logs -f deploy/ack-ai-dev-console -n kube-ai

# Notebook Controller 日志
kubectl logs -f deploy/notebook-controller-deployment -n kube-ai
```

## 故障排查

### Pod CrashLoopBackOff

```bash
kubectl logs <pod-name> -n kube-ai --previous
```

**常见原因：**

| 错误信息 | 原因 | 解决方案 |
|----------|------|----------|
| `failed to init credential provider` | AK/SK Secret 缺失 | 创建 `ai-dashboard-credentials` Secret |
| `IMS UpdateApplication: InvalidParameter` | OAuth redirect URI 为空 | 检查 Ingress host 配置 |
| `failed to init k8s client` | ServiceAccount 无权限 | 检查 RBAC Role/ClusterRole |
| `token not authenticated` | Token 过期 | 重新登录 |

### Ingress 访问异常

```bash
# 检查 Ingress 是否创建
kubectl get ingress -n kube-ai

# 检查 Nginx Ingress Controller
kubectl get pods -n kube-system | grep nginx-ingress

# 查看 Ingress Controller 日志
kubectl logs -f deploy/nginx-ingress-controller -n kube-system | grep <domain>
```

### OAuth 登录失败

1. 确认 RAM 权限策略已授权（IMS 相关 Action）
2. 确认 AK/SK 有效且未过期
3. 查看控制台日志中的 IMS 报错信息
4. 确认 Ingress 域名可正确解析到 SLB IP

### Notebook 无法打开

```bash
# 检查 Notebook Pod 状态
kubectl get pods -n <notebook-namespace> | grep <notebook-name>

# 查看 Notebook Controller 日志
kubectl logs deploy/notebook-controller-deployment -n kube-ai | grep <notebook-name>
```

## Session 管理

### 启用持久化 Session

```bash
kubectl create secret generic ai-dashboard-auth \
  --from-literal=sessionSecret=$(openssl rand -hex 32) \
  -n kube-ai

helm upgrade ai-dashboard charts/ack-ai-dashboard -n kube-ai \
  --set admin-ui.auth.existingSecret=ai-dashboard-auth \
  --reuse-values
```

### 轮换 Session Secret

```bash
# 更新 Secret
kubectl create secret generic ai-dashboard-auth \
  --from-literal=sessionSecret=$(openssl rand -hex 32) \
  -n kube-ai --dry-run=client -o yaml | kubectl apply -f -

# 重启 Pod 使新 Secret 生效（所有用户需重新登录）
kubectl rollout restart deploy/ack-ai-dashboard-admin-ui -n kube-ai
```

## 安全加固

### 启用 HTTPS

1. 准备 TLS 证书（可使用 cert-manager 自动签发或手动上传）：
   ```bash
   kubectl create secret tls ai-dashboard-tls \
     --cert=tls.crt --key=tls.key -n kube-ai
   ```

2. Helm upgrade 启用 TLS：
   ```bash
   helm upgrade ai-dashboard charts/ack-ai-dashboard -n kube-ai \
     --set 'admin-ui.dashboard.ingress.tls[0].secretName=ai-dashboard-tls' \
     --set 'admin-ui.dashboard.ingress.tls[0].hosts[0]=<domain>' \
     --set admin-ui.auth.secureCookie=true \
     --reuse-values
   ```

### 轮换 AK/SK

```bash
kubectl create secret generic ai-dashboard-credentials \
  --from-literal=accessKeyId=<NEW_AK> \
  --from-literal=accessKeySecret=<NEW_SK> \
  -n kube-ai --dry-run=client -o yaml | kubectl apply -f -

kubectl rollout restart deploy/ack-ai-dashboard-admin-ui -n kube-ai
```

## 卸载

```bash
# 卸载开发控制台
helm uninstall ai-dev-console -n kube-ai

# 卸载运维控制台
helm uninstall ai-dashboard -n kube-ai

# 清理 Secret（可选）
kubectl delete secret ai-dashboard-credentials ai-dev-console-credentials \
  ai-dashboard-auth ai-dev-console-auth -n kube-ai

# 清理 CRD（谨慎操作，会删除所有 Notebook/User/UserGroup 资源）
kubectl delete crd notebooks.kubeflow.org
kubectl delete crd users.data.kubeai.alibabacloud.com
kubectl delete crd usergroups.data.kubeai.alibabacloud.com
kubectl delete crd elasticquotatrees.scheduling.sigs.k8s.io
```

## 监控指标

Grafana 面板通过运维控制台的 `/grafana/` 路径访问，默认包含：

| 面板 | 内容 |
|------|------|
| Cluster Details | 集群资源总览、节点分布 |
| Node Details | 单节点资源利用率详情 |
| Resource Quota | 弹性配额使用情况 |
| Training Jobs | 训练任务资源消耗 |
