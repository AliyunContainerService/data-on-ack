# Accessing Multiple RayCluster Dashboards by Path via Ingress

When several RayClusters run in the same ACK cluster, this guide shows how to reach each Ray Dashboard through a shared entrypoint, routed by path:

```
http://<domain_url>/<namespace>/<ray_cluster_name>/
```

e.g. `http://alb-xxx.cn-hangzhou.alb.aliyuncsslb.com/default/raycluster-a/` opens the dashboard of `raycluster-a` in namespace `default`.

This document covers **Approach 1: `RayCluster.spec.headGroupSpec.enableIngress`** (KubeRay auto-generates the Ingress). Approach 2 (Gateway API) was researched and the conclusion is at the end: **the current ALB Gateway API implementation does not support URL rewriting, so it cannot serve this scenario yet**.

## Prerequisites

- An [ACK managed cluster](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2) with the ALB Ingress Controller installed (an `alb` IngressClass exists).
- The ACK KubeRay Operator, in a version that supports `headGroupSpec.ingressOptions` (`path` / `pathType`) and copies RayCluster annotations onto the generated Ingress.

## Approach 1: enableIngress + ingressOptions

### Required configuration on the RayCluster

See [ray-cluster-ingress.yaml](./ray-cluster-ingress.yaml); replace `<NAMESPACE>`, `<RAY_CLUSTER_NAME>` and `<IMAGE_REGISTRY>`, then `kubectl apply -f ray-cluster-ingress.yaml`. All relevant settings live on the RayCluster:

```yaml
metadata:
  annotations:
    kubernetes.io/ingress.class: alb          # 1. ingress class
    alb.ingress.kubernetes.io/use-regex: "true"        # 2. allow regex in path
    alb.ingress.kubernetes.io/rewrite-target: /${2}    # 3. strip URL prefix before forwarding
spec:
  headGroupSpec:
    enableIngress: true                       # 4. operator generates the Ingress
    ingressOptions:
      path: /<NAMESPACE>/<RAY_CLUSTER_NAME>(/|$)(.*)   # 5. custom (regex) path
      pathType: Prefix
```

What each item does:

1. **`kubernetes.io/ingress.class: alb`**: read by the operator and set as `spec.ingressClassName` of the generated Ingress.
2. **`alb.ingress.kubernetes.io/use-regex: "true"`** and **`rewrite-target: /${2}`**: the operator copies every RayCluster annotation except `kubernetes.io/ingress.class` onto the generated Ingress. These two annotations implement ALB path rewriting (see [Advanced ALB Ingress configurations](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/advanced-alb-ingress-configurations)): `use-regex` permits a regex `path`; `${2}` in `rewrite-target` references the second capture group.
3. **`enableIngress: true`**: makes the operator create `<cluster-name>-head-ingress`, backed by the head service's dashboard port (8265).
4. **`ingressOptions.path`**: overrides the operator's default path. In the regex, `(/|$)` is capture group 1 (the slash after the prefix, or end of path) and `(.*)` is group 2 (the remainder). With `rewrite-target: /${2}`, `/<ns>/<name>/api/version` is rewritten to `/api/version` before being forwarded — i.e. `/<ns>/<name>/(.*)` is passed through as `/(.*)`.

> Why not use upstream KubeRay defaults? The upstream-generated path (older versions: `/<name>/(.*)` with `PathType: Exact`) is rejected by ALB/nginx admission webhooks (wildcards are not allowed in exact paths), while a plain `/` prefix path makes multiple clusters sharing one ALB collide. `ingressOptions` declaratively sets a regex path with `Prefix` type, avoiding both problems.

### Behavior and caveats

- **Do not edit the generated Ingress manually.** This operator version re-syncs the Ingress labels/annotations/spec on every reconcile; manual edits are reverted. All customization (path, rewrite annotations) must be written on the RayCluster.
- **Always use a trailing slash**: `http://<domain>/<ns>/<name>/`. The Ray Dashboard is a React SPA whose static assets and API calls use relative paths (`./static/...`, `api/...`) and whose client routing is hash-based (`#/overview`). The browser resolves relative URLs against the current URL's "directory": with a trailing slash the base is `/<ns>/<name>/` and requests stay inside the rewrite rule; without it the base is `/`, so `api/...` resolves to `/api/...` outside the prefix and fails with 404/503 (see [ray-project/ray#8432](https://github.com/ray-project/ray/issues/8432); the new dashboard works under subpaths thanks to relative paths + hash routing, but only with the trailing slash).
- Each RayCluster gets **its own Ingress**; multiple clusters can share one ALB instance, distinguished by path.

### Verification

```bash
kubectl get ingress <RAY_CLUSTER_NAME>-head-ingress

# API-level check (expect 200)
curl -o /dev/null -w "%{http_code}\n" \
  "http://<domain_url>/<NAMESPACE>/<RAY_CLUSTER_NAME>/api/version"

# Browser (note the trailing slash)
# http://<domain_url>/<NAMESPACE>/<RAY_CLUSTER_NAME>/
```

## Approach 2: Gateway API (research conclusion: not feasible today)

The goal is the same `/<namespace>/<ray_cluster_name>/` path-based access. ACK clusters expose two ALB-backed GatewayClasses:

- `alb` (controller `gateways.alibabacloud.com/alb/v1`, ALB Standard edition)
- `alb-extensible` (controller `gateways.alibabacloud.com/alb-extensible/v1`, ALB Extensible edition)

**Conclusion: the current implementation cannot serve this scenario.** Mounting the Ray Dashboard under a subpath requires URL rewriting (`/<ns>/<name>/(.*)` → `/(.*)` before forwarding), and ALB's Gateway API does not support any form of path rewrite today.

### Test observations

**1. `alb` class (Standard): Gateway provisions fine, but URLRewrite is unsupported**

The Gateway + HTTPRoute are accepted and applied (ALB instance, listener and forwarding rule are all created):

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

but the HTTPRoute gets a `PartiallyInvalid` condition:

```
Dropped Rule: Route path type ReplacePrefixMatch is not supported.
```

The controller (cloud-controller-manager-alb, tested with v3.1.0) drops the rewrite filter; the generated ALB rule has an empty `RewriteConfig`. Requests reach the Dashboard with the original `/<ns>/<name>/...` path and get 404.

ALB-Ingress-style rewrite annotations on the HTTPRoute (`alb.ingress.kubernetes.io/rewrite-target` / `rewrite-path`) are also ignored by the Gateway API controller. Path conditions only accept literal prefixes — regex paths (e.g. `/prefix/(.*)`) are rejected by the ALB API (`PathConfig.Values illegal`).

**2. `alb-extensible` class (Extensible): regex semantics, but instances cannot be created**

This class conversely **rejects** `PathPrefix` matches:

```
Dropped Rule: Route path match type PathPrefix is not supported on Extension edition (alb-extensible GatewayClass).
```

It expects `type: RegularExpression` path matches, which combined with `ReplaceFullPath: /${2}` should map onto ALB's native "regex path condition + Rewrite action (`${1}`/`${2}` capture groups)" capability. In practice, however, no Extensible-edition ALB instance could be created: `CreateLoadBalancer --LoadBalancerEdition Extensible` fails with `IllegalParam.ZoneId` for every availability zone in cn-hangzhou (including zones where vSwitches were created specifically for this test), meaning the Extensible edition is not available in this region (not launched / not whitelisted), so end-to-end verification was impossible.

### Capability comparison

| Capability | Approach 1 (Ingress) | Gateway API `alb` | Gateway API `alb-extensible` |
|---|---|---|---|
| Regex path matching | ✅ (`use-regex`) | ❌ (literal prefix only) | ✅ (RegularExpression) |
| Path rewrite | ✅ (`rewrite-target`) | ❌ (URLRewrite dropped) | Not verifiable (instance creation fails) |
| Declarative multi-cluster path routing | ✅ | ⚠️ routes but cannot rewrite | — |

### Recommendation

- **Use Approach 1 for now.**
- The ALB forwarding-rule API itself already supports "regex path condition + Rewrite action (capture groups)". Once the Gateway API controller supports the `URLRewrite` filter, or the Extensible edition becomes available in the target region, re-validate Approach 2. The recommended HTTPRoute shape (Extensible semantics) would be:

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

> Either way, the trailing-slash requirement stands: browsers must use `http://<domain>/<ns>/<name>/` (see "Behavior and caveats" in Approach 1).
