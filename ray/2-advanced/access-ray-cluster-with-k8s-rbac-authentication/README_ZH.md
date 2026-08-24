# 使用 Kubernetes RBAC 认证访问 Ray 集群

本文介绍在 ACK 托管集群上验证使用Kubernetes RBAC 认证访问 Ray 集群 ，使用托管的 KubeRay Operator（v1.5.1）与 Ray 2.55.0。

## 背景

Ray Head 暴露的三个端口 —— 作业提交 API / Dashboard（8265）、Ray Client（10001）、GCS（6379）—— **默认都不做认证**。能访问 8265 就能提交作业，等同于在 Worker Pod 内执行任意代码，且这些代码会继承 Pod 的 ServiceAccount Token 与通过 RRSA 绑定的 RAM 角色。

Ray 2.55.0 起可将 Token 校验委托给 Kubernetes：每次请求都向 ACK API Server 问两个问题，于是"谁能用哪个 Ray 集群"就变成了普通的 Kubernetes RBAC 问题。

| Kubernetes API | 阶段 | 回答的问题 |
|---|---|---|
| `TokenReview`（`authentication.k8s.io`） | 认证 | 这个 Token 属于哪个身份？ |
| `SubjectAccessReview`（`authorization.k8s.io`） | 授权 | 该身份可以使用这个 RayCluster 吗？ |

授权用自定义动词 `ray:write` 表达，作用在单个 `RayCluster` 资源上：

```yaml
rules:
- apiGroups: ["ray.io"]
  resources: ["rayclusters"]
  verbs: ["ray:write"]
```

`SubjectAccessReview` 不要求动词取自 Kubernetes 标准动词集，它只是拿 `(身份, apiGroup, resource, resourceName, verb)` 去匹配 RBAC 规则。因此 `ray:write` 与 `get` / `list` / `create` 彼此独立：能执行 `kubectl get raycluster` 的用户并不因此获得提交 Ray 作业的权限，反之亦然。

机制中涉及**两个身份**，都需要授权，混淆二者是最常见的配置错误：Ray 集群自身的 ServiceAccount 需要调用上述两个校验接口的权限（外加对本集群的 `ray:write`，用于 Head ↔ Worker 认证）；调用方的 ServiceAccount 只需要目标集群上的 `ray:write`。

## 前提条件

- [ACK 托管版集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)，Kubernetes ≥ 1.24（需支持 TokenRequest API 与投射卷）
- 集群中已安装 [KubeRay Operator 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)
- Ray 镜像版本 **≥ 2.55.0**
- 当前账号可创建 ClusterRole 与 ClusterRoleBinding（`kubectl auth can-i create clusterroles` 返回 `yes`）

## 1. 部署启用认证的 Ray 集群

[`ray-cluster-auth.yaml`](ray-cluster-auth.yaml) 包含 RBAC 对象与 RayCluster：

| 对象 | 类型 | 作用 |
|---|---|---|
| `ray-cluster-auth` | ServiceAccount | 所有 Ray 容器使用的身份 |
| `ray-authenticator` | ClusterRole + ClusterRoleBinding | 允许 Ray 调用 `TokenReview` / `SubjectAccessReview` |
| `ray-writer` | ClusterRole | 定义 `rayclusters` 上的 `ray:write` 动词 |
| `ray-cluster-auth` | RoleBinding | 授予集群自身 SA 对本集群的 `ray:write`，用于 Head ↔ Worker 认证 |
| `ray-cluster-auth` | RayCluster | 一个 Head 加一个 Worker 组，均已启用认证 |

两个 ClusterRole 为集群级资源，多个 Ray 集群可共用，重复 apply 不会产生副作用。其余对象均为命名空间级，示例使用 `default` 命名空间。Ray 容器的命名空间由 Downward API 取得（见下文），无需修改；如需部署到其他命名空间，只需改 ClusterRoleBinding 的两处：**subject 的命名空间**与**它自己的名称**。前者是因为 ClusterRoleBinding 虽是集群级对象，其 subject 却是命名空间级的 ServiceAccount，完整身份为 `system:serviceaccount:<命名空间>:<名称>`，subject 中不存在命名空间通配符；后者是因为集群级对象名称全局唯一。另需相应调整下文命令中的 `-n` 与 `--as=system:serviceaccount:<命名空间>:...` 参数。

```bash
kubectl apply -f ray-cluster-auth.yaml
kubectl wait --for=condition=Ready pod -l ray.io/cluster=ray-cluster-auth --timeout=300s
kubectl get raycluster,pod
```

预期输出（`STATUS` 为 `ready`，所有 Pod `1/1 Running` 且 `RESTARTS` 为 0）：

```
NAME                                  DESIRED WORKERS   AVAILABLE WORKERS   STATUS   AGE
raycluster.ray.io/ray-cluster-auth    1                 1                   ready    2m

NAME                                              READY   STATUS    RESTARTS   AGE
pod/ray-cluster-auth-head-b7ktw                   1/1     Running   0          2m
pod/ray-cluster-auth-workergroup-worker-x9vqd     1/1     Running   0          2m
```

确认集群自身的 ServiceAccount 有权调用校验接口：

```bash
kubectl auth can-i create tokenreviews --as=system:serviceaccount:default:ray-cluster-auth
kubectl auth can-i create subjectaccessreviews --as=system:serviceaccount:default:ray-cluster-auth
```

两条均须输出 `yes`。

### 环境变量

Head 与**每一个** Worker 组都必须配置以下三项，缺一不可。示例 YAML 中已包含：

| 变量 | 值 | 作用 |
|---|---|---|
| `RAY_AUTH_MODE` | `token` | 启用 Token 认证 |
| `RAY_ENABLE_K8S_TOKEN_AUTH` | `true` | 将校验委托给 Kubernetes RBAC |
| `RAY_CLUSTER_NAMESPACE` | 命名空间 | `SubjectAccessReview` 的命名空间。**KubeRay v1.5.1 不会自动注入这一项** |

`RAY_CLUSTER_NAME`（`SubjectAccessReview` 的资源名）**不必显式配置**：KubeRay 已通过 Downward API 从 Pod 标签 `ray.io/cluster` 注入，实测省略后集群工作正常。

`RAY_CLUSTER_NAMESPACE` 同样建议用 Downward API 取，而不是写死字符串 —— Ray Pod 总是运行在 RayCluster 所属的命名空间中，`metadata.namespace` 必然是正确值：

```yaml
- name: RAY_CLUSTER_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
```

> 遗漏 `RAY_CLUSTER_NAMESPACE` 的后果不是"认证变弱"，而是**集群完全无法启动**。缺少命名空间时 GCS 无法构造 `SubjectAccessReview`，会在发出请求前直接判定授权失败并拒绝**一切**请求，包括 Head 自身连接本地 GCS 的那次调用。于是 `ray start --head` 重试约 20 秒后退出，Head Pod 进入 `CrashLoopBackOff`，Worker 卡在 `Init:0/1`。


## 2. 为用户授权

推荐一个团队 / 一条流水线对应一个 ServiceAccount，并用 `--resource-name` 精确限定可访问的集群：

```bash
kubectl create serviceaccount ray-user

kubectl create role ray-user \
  --verb=ray:write \
  --resource=rayclusters \
  --resource-name=ray-cluster-auth

kubectl create rolebinding ray-user \
  --role=ray-user \
  --serviceaccount=default:ray-user
```

验证授权边界：

```bash
kubectl auth can-i ray:write rayclusters/ray-cluster-auth --as=system:serviceaccount:default:ray-user
# yes

kubectl auth can-i ray:write rayclusters/other-cluster --as=system:serviceaccount:default:ray-user
# no
```

## 3. 验证委托机制已生效

转发 Dashboard 端口：

```bash
kubectl port-forward svc/ray-cluster-auth-head-svc 8265:8265 &
```

不携带 Token 提交作业会被拒绝：

```bash
ray job submit --address http://localhost:8265 -- python -c "print('hello')"
```

预期输出：

```
ray.exceptions.AuthenticationError: Authentication required: Unauthorized: Missing authentication token
```

携带已授权身份的 Token 则提交成功：

```bash
export RAY_AUTH_MODE=token
export RAY_AUTH_TOKEN=$(kubectl create token ray-user)

ray job submit --address http://localhost:8265 \
  -- python -c "import ray; ray.init(); print(ray.cluster_resources())"
```

预期输出：

```
Job 'raysubmit_xxxxxxxx' submitted successfully
...
{'CPU': 4.0, 'memory': ..., 'node:__internal_head__': 1.0, ...}
Job 'raysubmit_xxxxxxxx' succeeded
```

**用三种身份访问同一接口做对照，三种结果互不相同，证明 Ray 正在逐身份查询 Kubernetes RBAC：**

```bash
TOKEN_OK=$(kubectl create token ray-user)   # 有 ray:write
TOKEN_NO=$(kubectl create token default)    # 无 ray:write
URL=http://localhost:8265/api/version

curl -s -o /dev/null -w "无凭证        HTTP %{http_code}\n" ${URL}
curl -s -o /dev/null -w "有 ray:write  HTTP %{http_code}\n" -H "Authorization: Bearer ${TOKEN_OK}" ${URL}
curl -s -o /dev/null -w "无 ray:write  HTTP %{http_code}\n" -H "Authorization: Bearer ${TOKEN_NO}" ${URL}
```

| 输出 | 含义 |
|---|---|
| `401` `200` `403` | 委托机制完全生效，Ray 正按身份逐一判定 |
| 三档全为 `200` | 认证未启用：镜像版本低于 2.55.0，或环境变量不全 |

### 客户端配置

| 变量 | 作用 |
|---|---|
| `RAY_AUTH_MODE=token` | 指示 Ray CLI 在请求中附加 `Authorization: Bearer` 头 |
| `RAY_AUTH_TOKEN` | Token 内容，优先级最高 |
| `RAY_AUTH_TOKEN_PATH` | 未设置 `RAY_AUTH_TOKEN` 时从该文件读取，默认 `~/.ray/auth_token` |

`kubectl create token` 默认签发有效期为 1 小时，长时间运行的作业或流水线可以延长（集群可能配置了上限）：

```bash
export RAY_AUTH_TOKEN=$(kubectl create token ray-user --duration=8h)
```

运行在同一 ACK 集群内的客户端可直接使用 Service 域名 `ray-cluster-auth-head-svc.default:8265`，无需 port-forward。

## 4. 吊销访问权限

```bash
kubectl delete rolebinding ray-user

export RAY_AUTH_TOKEN=$(kubectl create token ray-user)
ray job submit --address http://localhost:8265 -- python -c "print('hello')"
```

预期结果：提交失败，报 `AuthenticationError`。

> 吊销存在**最多 5 分钟延迟**，Ray 会缓存已通过校验的 Token。安全事件响应时应重启 Head Pod 强制清空缓存（`kubectl delete pod -l ray.io/node-type=head`），或直接删除 Head Service 切断访问入口。

重新创建 RoleBinding 即可恢复该用户的访问：

```bash
kubectl create rolebinding ray-user --role=ray-user --serviceaccount=default:ray-user
```

## 说明

- **只有一个权限级别。** `ray:write` 是唯一的动词，**不存在只读级别**：获得访问权即可提交与终止作业；同一集群上所有获授权身份共享资源，可相互查看作业。
- **授权粒度到集群。** 每个 Role 都用 `--resource-name` 限定范围，每个团队、每条流水线使用独立 ServiceAccount，吊销其中一个不影响其他。建议定期审查 RoleBinding 清单。
- **访问审计。** 开启 ACK 集群审计功能后，采集到 SLS 的审计日志中包含每一次 `subjectaccessreviews` 调用记录，可追溯何人在何时访问了哪个 Ray 集群。

### KubeRay v1.6.0 上的简化写法

托管 Operator 升级到 v1.6.0 及以上后，三个必需环境变量与投射卷可以替换为：

```yaml
spec:
  rayVersion: '2.55.0'
  authOptions:
    mode: 'token'
    enableK8sTokenAuth: true
```

Operator 会自动为所有 Ray 容器注入 `RAY_AUTH_MODE`、`RAY_ENABLE_K8S_TOKEN_AUTH`、`RAY_CLUSTER_NAME` 与 `RAY_CLUSTER_NAMESPACE`，并自动挂载投射 Token。RBAC 对象保持不变。本文的显式写法在 v1.6.0 上**依然有效**，可从容安排迁移。

## 清理

```bash
kubectl delete -f ray-cluster-auth.yaml
kubectl delete rolebinding ray-user --ignore-not-found
kubectl delete role ray-user --ignore-not-found
kubectl delete serviceaccount ray-user
```

## 参考资料

- [Configure Ray clusters to use Kubernetes RBAC authentication](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/kuberay-auth-rbac.html)
- [Configure Ray clusters to use token authentication](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/kuberay-auth.html)
- [Kubernetes 认证（TokenReview）](https://kubernetes.io/docs/reference/access-authn-authz/authentication/) / [授权（SubjectAccessReview）](https://kubernetes.io/docs/reference/access-authn-authz/authorization/)
