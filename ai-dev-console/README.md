# ai-dev-console — 开发控制台

基于 Go (Gin) + React (Vite) 的 ACK AI 模型开发控制台。

## 功能

- Notebook 管理（Jupyter Lab / VS Code Server / TensorBoard）
- 分布式训练任务（PyTorch / TensorFlow / XGBoost / MPI）
- 模型推理服务部署与测试
- 实验跟踪与对比
- 数据集管理（PVC）
- 模型注册中心
- Ray Dashboard 代理
- RAM SSO 登录（OAuth2）

## 构建

```bash
# 完整 Docker 构建（含前端）
docker build -f Dockerfile.console-new -t ai-dev-console:latest .

# 仅构建后端（本地开发）
cd console/backend && go build -o server ./cmd/server/

# 仅构建前端
cd console/frontend-new && npm ci && npm run build
```

## 本地开发

```bash
# 后端（需 kubeconfig）
export DISABLE_AUTH=true
cd console/backend && go run ./cmd/server/ --listen-addr=:9090 --disable-oauth

# 前端
cd console/frontend-new && npm run dev
```

## 部署

参见 [部署指南](../docs/deploy-guide.md)。

```bash
helm install ai-dev-console ../charts/ack-ai-dev-console -n kube-ai \
  --set 'dev-console.console.ingress.hosts[0].host=<domain>' \
  --set 'dev-console.console.ingress.hosts[0].paths[0]=/'
```

## 配置

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `KUBE_DL_INGRESS_ENABLE` | 是否启用 Ingress 发现 | `true` |
| `DEV_CONSOLE_HOST` | OAuth 回调域名 | 从 Ingress 自动发现 |
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
- **Notebook Controller**: controller-runtime (独立 subchart)
- **Commit Agent**: gRPC DaemonSet (独立 subchart)
