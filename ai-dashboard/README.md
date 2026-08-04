# ai-dashboard — 运维控制台

基于 Go (Gin) + React (Vite) 的 ACK AI 集群运维控制台。

## 功能

- 用户/用户组管理（关联 ACK ElasticQuotaTree）
- GPU 集群监控（Grafana 面板代理）
- 数据集管理（PVC 生命周期）
- Node Shell 远程运维（仅 admin）
- 模型注册中心
- RAM SSO 登录（OAuth2）

## 构建

```bash
# 完整 Docker 构建（含前端）
docker build -t ai-dashboard:latest .

# 仅构建后端（本地开发）
cd backend && go build -o server ./cmd/server/

# 仅构建前端
cd frontend && npm ci && npm run build
```

## 本地开发

```bash
# 后端（需 kubeconfig）
export DISABLE_AUTH=true
cd backend && go run ./cmd/server/ --listen-addr=:8080 --disable-oauth

# 前端
cd frontend && npm run dev
```

## 部署

参见 [部署指南](../docs/deploy-guide.md)。

```bash
helm install ai-dashboard ../charts/ack-ai-dashboard -n kube-ai \
  --set 'admin-ui.dashboard.ingress.hosts[0].host=<domain>' \
  --set 'admin-ui.dashboard.ingress.hosts[0].paths[0]=/' \
  --set grafana.adminPassword=<password>
```

## 配置

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `DASHBOARD_INGRESS_ENABLE` | 是否启用 Ingress 发现 | `true` |
| `DASHBOARD_HOST` | OAuth 回调域名 | 从 Ingress 自动发现 |
| `CREDENTIAL_MODE` | 凭据模式 (static/rrsa) | `static` |
| `AK_ACCESS_KEY_ID` | 阿里云 AccessKey ID | - |
| `AK_ACCESS_KEY_SECRET` | 阿里云 AccessKey Secret | - |
| `SESSION_SECRET` | Cookie 加密密钥 | 随机生成 |
| `ENABLE_SECURE_COOKIE` | Cookie Secure 标志 | `false` |
| `DISABLE_AUTH` | 禁用认证（开发模式） | `false` |
| `FRONTEND_DIR` | 前端静态文件目录 | `./dist` |

## 技术栈

- **后端**: Go 1.22 + Gin + client-go + IMS SDK
- **前端**: React 18 + TypeScript + Vite + Ant Design
- **监控**: Grafana (subchart)
