# 通过 Ingress 按路径访问多个 RayCluster 的 Dashboard

当同一个 ACK 集群中运行多个 RayCluster 时，本文描述如何让每个集群的 Ray Dashboard 通过统一入口按路径访问：

```
http://<domain_url>/<namespace>/<ray_cluster_name>/
```

例如 `http://alb-xxx.cn-hangzhou.alb.aliyuncsslb.com/default/raycluster-a/` 访问 `default` 命名空间下 `raycluster-a` 的 Dashboard。

本文先给出**方案一：基于 `RayCluster.spec.headGroupSpec.enableIngress`**（KubeRay 自动生成 Ingress）。方案二（基于 Gateway API）的调研结论见文末：**当前 ALB 的 Gateway API 实现尚不支持 URL 重写，暂时无法满足本场景**。

## 前提条件

- [ACK 托管集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)，并已安装 ALB Ingress Controller（集群中存在 `ingressclass` 为 `alb`）。
- 已安装 ACK 版 KubeRay Operator，且版本支持 `headGroupSpec.ingressOptions`（`path` / `pathType` 字段），并具备"将 RayCluster 注解透传到生成的 Ingress"的行为。

## 方案一：enableIngress + ingressOptions

### 在 RayCluster 上需要的配置

见 [ray-cluster-ingress.yaml](./ray-cluster-ingress.yaml)，将 `<NAMESPACE>`、`<RAY_CLUSTER_NAME>`、`<IMAGE_REGISTRY>` 替换为实际值后 `kubectl apply -f ray-cluster-ingress.yaml`。关键配置全部集中在 RayCluster 上：

```yaml
metadata:
  annotations:
    kubernetes.io/ingress.class: alb          # ① 指定 ingress class
    alb.ingress.kubernetes.io/use-regex: "true"        # ② 允许 path 使用正则
    alb.ingress.kubernetes.io/rewrite-target: /${2}    # ③ 转发前剥掉 URL 前缀
spec:
  headGroupSpec:
    enableIngress: true                       # ④ 让 operator 自动生成 Ingress
    ingressOptions:
      path: /<NAMESPACE>/<RAY_CLUSTER_NAME>(/|$)(.*)   # ⑤ 自定义路径（正则）
      pathType: Prefix
```

各配置项的作用：

1. **`kubernetes.io/ingress.class: alb`**：operator 读取该注解并设置为生成 Ingress 的 `spec.ingressClassName`。
2. **`alb.ingress.kubernetes.io/use-regex: "true"`** 与 **`rewrite-target: /${2}`**：除 `kubernetes.io/ingress.class` 外，operator 会把 RayCluster 上的其余注解原样复制到生成的 Ingress 上。这两个注解是 ALB 的路径重写语义（见 [ALB Ingress 服务高级用法](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/advanced-alb-ingress-configurations)）：`use-regex` 允许 `path` 写正则；`rewrite-target` 中的 `${2}` 引用第二捕获组。
3. **`enableIngress: true`**：触发 operator 创建名为 `<cluster-name>-head-ingress` 的 Ingress，后端为 head service 的 dashboard 端口（8265）。
4. **`ingressOptions.path`**：覆盖 operator 默认路径。路径正则中 `(/|$)` 是第一捕获组（匹配前缀后的斜杠或结尾），`(.*)` 是第二捕获组（剩余路径）。配合 `rewrite-target: /${2}`，`/<ns>/<name>/api/version` 会被重写为 `/api/version` 再转发给 Dashboard —— 即"把 `/<ns>/<name>/(.*)` 透传到 `/(.*)`"。

> 为什么不直接用 KubeRay 上游默认行为？上游默认生成的路径（旧版本为 `/<name>/(.*)` + `PathType: Exact`）会被 ALB/nginx 的校验 webhook 拒绝（exact 路径不允许通配符）；而默认 `/` 前缀路径在多集群共享一个 ALB 时又会互相冲突。`ingressOptions` 允许声明式地指定正则路径与 `Prefix` 类型，绕开这两个问题。

### 行为与注意事项

- **不要手工修改生成的 Ingress**。该版本的 operator 每次 reconcile 都会将 Ingress 的 labels/annotations/spec 同步回期望状态，手工修改会被回滚。所有定制（路径、重写注解）都必须写在 RayCluster 上，由 operator 落到 Ingress。
- **访问 URL 必须带末尾斜杠**：`http://<domain>/<ns>/<name>/`。原因是 Ray Dashboard 是 React SPA，静态资源与 API 调用均使用相对路径（`./static/...`、`api/...`），客户端路由使用 hash（`#/overview`）。浏览器按当前 URL 的"目录"解析相对路径：带末尾斜杠时基准是 `/<ns>/<name>/`，请求保持在重写规则内；不带末尾斜杠时基准变成 `/`，`api/...` 会解析成 `/api/...` 飞出前缀导致 404/503（参见 [ray-project/ray#8432](https://github.com/ray-project/ray/issues/8432)，新版 dashboard 通过相对路径 + hash 路由使子路径部署可用，但前提就是末尾斜杠）。
- 每个 RayCluster 生成**各自独立的 Ingress**，多个集群可以共用同一个 ALB 实例，靠路径区分。

### 验证

```bash
kubectl get ingress <RAY_CLUSTER_NAME>-head-ingress

# API 级验证（应返回 200）
curl -o /dev/null -w "%{http_code}\n" \
  "http://<domain_url>/<NAMESPACE>/<RAY_CLUSTER_NAME>/api/version"

# 浏览器访问（注意末尾斜杠）
# http://<domain_url>/<NAMESPACE>/<RAY_CLUSTER_NAME>/
```

## 方案二：Gateway API（调研结论：当前暂不可行）

目标同样是 `/<namespace>/<ray_cluster_name>/` 多集群按路径访问。调研基于 ACK 集群中 ALB 提供的两个 GatewayClass：

- `alb`（controller `gateways.alibabacloud.com/alb/v1`，对应 ALB 标准版）
- `alb-extensible`（controller `gateways.alibabacloud.com/alb-extensible/v1`，对应 ALB 弹性版）

**结论：当前实现无法满足本场景**。关键点在于 Ray Dashboard 挂在子路径下必须做 URL 重写（把 `/<ns>/<name>/(.*)` 重写为 `/(.*)` 再转发），而 ALB 的 Gateway API 目前不支持任何形式的路径重写。

### 测试过程与现象

**1. `alb` class（标准版）：Gateway 可正常创建，但不支持 URLRewrite**

Gateway + HTTPRoute 可以被接受并下发（ALB 实例、监听、转发规则均成功创建）：

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: ray-dash
  namespace: default
spec:
  parentRefs:
  - name: ray-gw
  rules:
  - matches:
    - path: {type: PathPrefix, value: /default/raycluster-a}
    filters:
    - type: URLRewrite
      urlRewrite:
        path: {type: ReplacePrefixMatch, replacePrefixMatch: /}
    backendRefs:
    - name: raycluster-a-head-svc
      port: 8265
```

但 HTTPRoute 上出现 `PartiallyInvalid` 条件：

```
Dropped Rule: Route path type ReplacePrefixMatch is not supported.
```

即 controller（cloud-controller-manager-alb，测试版本 v3.1.0）丢弃了重写 filter，生成的 ALB 转发规则中 `RewriteConfig` 为空。请求以原始路径 `/<ns>/<name>/...` 到达 Dashboard，返回 404。

也尝试过在 HTTPRoute 上加 ALB Ingress 风格的重写注解（`alb.ingress.kubernetes.io/rewrite-target` / `rewrite-path`），Gateway API controller 不解析这些注解，行为不变。另外 path 条件只支持字面前缀，正则路径（如 `/prefix/(.*)`）会被 ALB API 以 `PathConfig.Values illegal` 拒绝。

**2. `alb-extensible` class（弹性版）：语义上支持正则，但当前无法创建实例**

该 class 反过来**拒绝** `PathPrefix` 匹配：

```
Dropped Rule: Route path match type PathPrefix is not supported on Extension edition (alb-extensible GatewayClass).
```

需要改用 `type: RegularExpression` 的路径匹配。结合 `ReplaceFullPath: /${2}` 理论上可映射到 ALB 原生的"正则路径条件 + Rewrite 动作（`${1}`/`${2}` 捕获组引用）"能力。但实测中弹性版 ALB 实例无法创建：对 cn-hangzhou 的所有可用区（含专门新建 vSwitch 的可用区），`CreateLoadBalancer --LoadBalancerEdition Extensible` 均报 `IllegalParam.ZoneId`，说明弹性版当前在该地域不可用（未开服或未加白），无法完成端到端验证。

### 能力对比

| 能力 | 方案一（Ingress） | Gateway API `alb` | Gateway API `alb-extensible` |
|---|---|---|---|
| 正则路径匹配 | ✅（`use-regex`） | ❌（仅字面前缀） | ✅（RegularExpression） |
| 路径重写 | ✅（`rewrite-target`） | ❌（URLRewrite 被丢弃） | 未能验证（实例无法创建） |
| 声明式多集群按路径 | ✅ | ⚠️ 可路由但不可重写 | — |

### 建议

- **现阶段使用方案一**。
- ALB 转发规则 API 本身已支持"正则路径条件 + Rewrite 动作（捕获组）"，待 Gateway API controller 支持 `URLRewrite` filter、或弹性版在目标地域可用后，可重新验证方案二。届时推荐的 HTTPRoute 形态（弹性版语义）：

```yaml
rules:
- matches:
  - path:
      type: RegularExpression
      value: ^/<NAMESPACE>/<RAY_CLUSTER_NAME>(/|$)(.*)
  filters:
  - type: URLRewrite
    urlRewrite:
      path:
        type: ReplaceFullPath
        replaceFullPath: /${2}
  backendRefs:
  - name: <RAY_CLUSTER_NAME>-head-svc
    port: 8265
```

> 无论哪种方案，末尾斜杠的要求不变：浏览器访问必须使用 `http://<domain>/<ns>/<name>/`（原因见方案一"行为与注意事项"）。
