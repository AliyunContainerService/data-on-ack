# data-on-ack

data-on-ack 是基于阿里云容器服务 ACK 的开源云原生 AI/ML 平台，为数据科学家和 ML 工程师提供模型开发、训练、推理的全流程工具。

## 快速开始

**前提条件**：ACK Pro 集群 (>= 1.20) + Nginx Ingress Controller + 阿里云 AK/SK

```bash
# 1. 创建凭据 Secret
kubectl create ns kube-ai
kubectl create secret generic ai-dashboard-credentials \
  --from-literal=accessKeyId=<AK> --from-literal=accessKeySecret=<SK> -n kube-ai
kubectl create secret generic ai-dev-console-credentials \
  --from-literal=accessKeyId=<AK> --from-literal=accessKeySecret=<SK> -n kube-ai

# 2. 安装运维控制台
helm install ai-dashboard charts/ack-ai-dashboard -n kube-ai \
  --set 'admin-ui.dashboard.ingress.hosts[0].host=<YOUR_DOMAIN>' \
  --set 'admin-ui.dashboard.ingress.hosts[0].paths[0]=/' \
  --set grafana.adminPassword=<PASSWORD>

# 3. 安装开发控制台
helm install ai-dev-console charts/ack-ai-dev-console -n kube-ai \
  --set 'dev-console.console.ingress.hosts[0].host=<YOUR_DOMAIN>' \
  --set 'dev-console.console.ingress.hosts[0].paths[0]=/'
```

详细步骤请参阅 [部署指南](docs/deploy-guide.md)。

## 文档

| 文档 | 说明 |
|------|------|
| [部署指南](docs/deploy-guide.md) | 端到端安装配置（RAM 授权、Secret 创建、Helm 安装、域名配置） |
| [功能使用指南](docs/user-guide.md) | 运维控制台和开发控制台的功能介绍和操作方法 |
| [运维指南](docs/ops-guide.md) | 升级、扩缩容、故障排查、安全加固、卸载 |

## 项目结构

```
data-on-ack/
├── ai-dashboard/          # 运维控制台（Go + React）
├── ai-dev-console/        # 开发控制台（Go + React）
├── commit-agent/          # Notebook 代码同步 Agent（gRPC sidecar）
├── notebook-controller/   # Notebook CRD Controller
├── charts/                # Helm Charts
│   ├── ack-ai-dashboard/    # 运维控制台 + Grafana
│   └── ack-ai-dev-console/  # 开发控制台 + Notebook Controller
└── docs/                  # 文档
```

## 组件说明

### [ai-dashboard](./ai-dashboard) — 运维控制台

面向集群管理员，提供：
- 用户/用户组/配额管理（基于 ACK ElasticQuotaTree）
- GPU 集群监控（内嵌 Grafana）
- 数据集管理（PVC/Fluid）
- Node Shell 远程运维
- 模型注册中心

### [ai-dev-console](./ai-dev-console) — 开发控制台

面向 ML 工程师，提供：
- Notebook 管理（Jupyter Lab / VS Code Server）
- 分布式训练任务（PyTorch / TensorFlow / XGBoost）
- 模型推理服务部署
- 实验跟踪与对比
- 数据集 & 模型注册

### [commit-agent](./commit-agent)

运行在 Notebook Pod 中的 gRPC sidecar，支持 Docker/containerd 运行时，提供代码同步能力。

### [notebook-controller](./notebook-controller)

Kubernetes Controller，管理 `Notebook` CRD 的生命周期（Pod/Service 创建、停止、恢复）。

### [charts](./charts)

可直接部署的 Helm Charts，预构建镜像已发布：
- `registry-cn-hangzhou.ack.aliyuncs.com/dev/ai-dashboard:v3.0.1`
- `registry-cn-hangzhou.ack.aliyuncs.com/dev/ai-dev-console:v3.0.0`

## 前提条件

- 阿里云 ACK Pro 集群 (Kubernetes >= 1.20)
- 集群已安装 Nginx Ingress Controller
- 阿里云 RAM 账号（需 IMS API 权限用于 OAuth 登录）

## 许可证

本项目基于 Apache License 2.0 开源，详见 [LICENSE](./LICENSE)。
