# RolloutServer Token 管理

本文介绍 RolloutServer 的 API 鉴权 token 体系：普通 token 与 admin token 的权限边界、配置与 Secret 注入、多 token 平滑轮换，以及 batch controller 的独立鉴权。模型 API key（如 `OPENAI_API_KEY`）不属于本文范围，它们按请求配置，见 [Evaluation 与 RL 双模式](../5-rolloutserver-eval-rl/README_ZH.md)。

## 两级 token 体系

| Token 类型 | 配置字段 | 权限 |
|---|---|---|
| 普通 token | `auth_tokens`（列表） | 提交、查询、取消等所有常规端点 |
| admin token | `admin_tokens`（列表） | 普通 token 的全部权限 + 配额写操作 |

admin-only 端点（写操作）：

- `PUT /api/v1/quotas/{queue}`（运行时修改队列配额）
- `POST /api/v1/quotas/reconcile`（手动配额对账）
- `PUT /api/v1/job-quotas`、`PUT/DELETE /api/v1/job-quotas/{job_id}`

同路径的 GET 查询只需普通 token。admin token 可以同时当普通 token 使用。

免鉴权端点：`/health`、`/metrics`、`/help`、`/api/v1/help`、`/api/config`、`/api/health`、`/debug/*`。其余端点（含提交、取消、Viewer 的 job/trial 查询）都需要 token。

**未配置 `auth_tokens` 时，所有端点放行**（启动日志显示 auth=disabled）。生产环境必须配置。只配置 `admin_tokens` 时，配额写操作仍受 admin 校验，其余端点放行。

## 携带 token

按以下顺序识别（任选其一）：

1. `Authorization: Bearer <token>`（推荐）
2. `X-Auth-Token: <token>` 请求头
3. `?token=<token>` 查询参数
4. `rollout_token` cookie（浏览器访问 UI 时，用 `?token=` 打开页面会自动种 7 天 HttpOnly cookie）

```bash
curl -s $SERVER/api/v1/quotas -H "Authorization: Bearer $TOKEN"
```

缺失 token 返回 401，无效 token 返回 403。校验使用常数时间比较（`hmac.compare_digest`），防时序攻击。

## 配置与 Secret 注入

### 生产：existingSecret（推荐）

创建 Secret，包含两个 key：

```bash
kubectl create secret generic kube-rl-auth -n kube-rl \
  --from-literal=tokens='<token1>,<token2>' \
  --from-literal=admin-tokens='<admin-token>'
```

values 中引用：

```yaml
auth:
  existingSecret: kube-rl-auth
```

行为：

- ConfigMap 不再包含 token 明文；server 各角色（单体 / ingress / viewer）通过环境变量 `KUBE_RL_AUTH_TOKENS`（key `tokens`）和 `KUBE_RL_ADMIN_TOKENS`（key `admin-tokens`，可选）注入。
- 环境变量优先于配置文件同名键。
- batch controller 自动使用 Secret 中 `tokens` 的**第一个** token 作为出站请求凭据。
- worker 不注入 token（无 HTTP 面）。

### 测试：inline tokens

```yaml
auth:
  tokens: ["test-token"]
  adminTokens: ["test-admin"]
```

inline 方式会把 token 写进**明文 ConfigMap**，仅用于测试。生产使用 existingSecret。

## 多 token 平滑轮换

`auth_tokens` / `admin_tokens` 都是列表，天然支持新旧 token 并存。轮换步骤：

1. 在 Secret 的 `tokens` 中**追加**新 token（保留旧 token）。
2. 重启生效：`kubectl -n kube-rl rollout restart deploy/kube-rl-ingress`（单体模式为 `deploy/kube-rl`；viewer 同理）。
3. 客户端逐步切换到新 token。
4. 确认无旧 token 流量后，从 Secret 移除旧 token，再次重启。

注意：

- token 只在进程启动时加载，**没有热加载**。
- 修改 existingSecret 内容**不会**自动触发滚动更新（Pod 注解只含 ConfigMap checksum），必须手动 `rollout restart`。
- inline tokens 走 `helm upgrade` 会因 ConfigMap 变化自动滚动。

## Batch Controller 的独立鉴权

batch controller（8082）保护自身 API 的 token 与 server 独立：

- 通过 `batchController.extraEnv` 注入 `BATCH_CONTROLLER_API_TOKENS`（逗号分隔列表）：

```yaml
batchController:
  extraEnv:
    - name: BATCH_CONTROLLER_API_TOKENS
      valueFrom:
        secretKeyRef:
          name: kube-rl-auth
          key: tokens
```

- 配置后，其 `/api/v1/*` 端点要求 `Authorization: Bearer`（仅 header，不支持 query/cookie）；`/health`、`/api/v1/help` 免鉴权。
- 未配置时其 API 全放行。
- chart 不会自动渲染该变量，必须显式配置。

## 安全检查清单

- [ ] 生产环境配置了 `auth.existingSecret`，ConfigMap 中无 token 明文。
- [ ] 配置了 `admin_tokens`，配额写操作与日常提交权限分离。
- [ ] batch controller 配置了 `BATCH_CONTROLLER_API_TOKENS`。
- [ ] 客户端统一使用 `Authorization: Bearer`，不在 URL 中传 token（避免日志泄漏）。
- [ ] 轮换流程验证过：追加新 token → 重启 → 切换 → 移除旧 token。
- [ ] 模型 API key 不在 server 全局环境或 `env_profiles` 中（按请求配置，见双模式指南）。