# AI Dashboard

ACK 上的 AI 基础设施管理平台，提供集群监控、弹性配额管理、用户/用户组管理、数据集管理等功能。

## 架构

- **后端**: Go 1.22 + Gin + client-go + IMS SDK
- **前端**: React 18 + Ant Design 5 + Vite 5 + React Router 6 + Zustand
- **部署**: 单一 Docker 镜像（Go 二进制 + 前端静态文件），无 MySQL/arena 依赖

## 功能

- 集群概览（Grafana 嵌入）
- 弹性配额树管理（ElasticQuotaTree CRUD）
- 用户管理（RAM 用户同步、K8s User CRD、RBAC、kubeconfig 下载）
- 用户组管理（UserGroup CRD、配额绑定、命名空间解析）
- 数据集管理（Fluid Dataset + Runtime CRD）
- K8s 资源查看（PVC、Secret、Namespace）
- OAuth2 SSO 登录（阿里云 RAM）

## 快速开始

### 本地开发

```bash
# 后端
make build-backend && ./bin/ai-dashboard

# 前端
cd frontend && npm install && npm run dev
```

### 构建 Docker 镜像

```bash
make docker-build
```

### 部署

```bash
helm install ai-dashboard charts/ack-ai-dashboard
```

## 凭证配置

支持两种 IMS API 凭证模式：

1. **静态 AK/SK**（`CREDENTIAL_MODE=static`）: 通过环境变量 `AK_ACCESS_KEY_ID` / `AK_ACCESS_KEY_SECRET` 配置
2. **RRSA**（`CREDENTIAL_MODE=rrsa`）: 通过 OIDC token → STS 自动获取临时凭证

## 项目结构

```
ai-dashboard/
├── backend/
│   ├── cmd/server/main.go          # 入口
│   └── internal/
│       ├── aliyun/                 # IMS 客户端
│       ├── auth/                   # OAuth2 SSO + 中间件
│       ├── config/                 # 配置
│       ├── credential/             # 凭证 provider (static/rrsa)
│       ├── handler/                # HTTP handler
│       ├── k8s/                     # K8s client + CRD 操作
│       ├── model/                  # CRD 模型 + DTO
│       ├── response/               # 统一响应
│       └── service/                # 业务逻辑 + 嵌入 YAML 清单
├── frontend/
│   └── src/
│       ├── api/                    # API 客户端
│       ├── i18n/                   # 中英双语
│       ├── layouts/                # antd 布局
│       ├── pages/                  # 页面组件
│       ├── router/                 # 路由
│       └── store/                  # Zustand 状态管理
├── go.mod
├── Dockerfile
└── Makefile
```
