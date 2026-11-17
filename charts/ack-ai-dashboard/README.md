# ack-ai-dashboard

Cluster operations console for the ACK Cloud-Native AI Suite: cluster overview,
GPU/node monitoring (embedded Grafana), node shell & batch commands, elastic
quota trees, unified workload view, user/group management, datasets, model
registry and platform settings.

## Prerequisites

- ACK Pro cluster (Kubernetes >= 1.20) with `kubectl` / Helm 3 configured.
- **Nginx Ingress Controller** installed (the console is exposed via Ingress).
- Cloud-Native AI Suite / `ack-arena` installed (for `arena_*` monitoring metrics).
- Namespace `kube-ai` (created below).

## Quick start (login-free, out of the box)

The default `admin-ui.auth.mode` is `none`: no login, every request runs as the
built-in `admin`. The chart automatically provisions the admin identity, an
in-cluster Prometheus for monitoring, AC2 default development images, and (if
the cluster has no default StorageClass) marks
`alicloud-disk-topology-alltype` as default.

```bash
kubectl create namespace kube-ai

helm install ack-ai-dashboard charts/ack-ai-dashboard -n kube-ai \
  --set grafana.adminPassword='<GRAFANA_PASSWORD>' \
  --set 'admin-ui.dashboard.ingress.hosts[0].host=ai-dashboard.example.com' \
  --set 'admin-ui.dashboard.ingress.hosts[0].paths[0]=/'
```

> Use the release name `ack-ai-dashboard` so the default Grafana proxy target
> (`ack-ai-dashboard-grafana.kube-ai:80`) resolves correctly.

### Access

Point the Ingress host at the Nginx Ingress SLB IP (e.g. in `/etc/hosts`):

```bash
kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
# <SLB_IP> ai-dashboard.example.com
```

Then open `http://ai-dashboard.example.com/`.

> ⚠️ **Security**: in `none` mode anyone who can reach the console URL has full
> admin access. Use it only on intranet / PoC environments or behind an access
> control layer. For production switch to `aliyun` or `custom` (below), or
> restrict the Ingress/SLB (ACL whitelist, intranet SLB).

## Authentication modes

| Mode | Description | Aliyun AK/SK |
|------|-------------|--------------|
| `none` (default) | Login-free, built-in admin, full access | No |
| `aliyun` | RAM OAuth2 SSO; main account becomes admin | Yes |
| `custom` | Trust identity headers from an upstream gateway | No |

For `aliyun`, create the credentials Secret and grant IMS permissions (see
`docs/deploy-guide.md`):

```bash
kubectl create secret generic ai-dashboard-credentials -n kube-ai \
  --from-literal=accessKeyId=<AK> --from-literal=accessKeySecret=<SK>
helm upgrade ack-ai-dashboard charts/ack-ai-dashboard -n kube-ai \
  --set admin-ui.auth.mode=aliyun --reuse-values
```

## Monitoring

By default the chart deploys an in-cluster Prometheus (`prometheus.enabled=true`)
scraping `arena-exporter` and the ARMS GPU exporter, and wires Grafana to it —
monitoring works without ARMS. To use ARMS Prometheus instead:

```bash
helm upgrade ack-ai-dashboard charts/ack-ai-dashboard -n kube-ai --reuse-values \
  --set prometheus.enabled=false \
  --set 'grafana.datasources.datasources\.yaml.datasources[0].url=<ARMS_INTRANET_URL>'
```

## Offline / cross-region clusters

Images default to `registry-cn-hangzhou.ack.aliyuncs.com/acs/`. If your nodes
cannot reach that registry, sync the images to a reachable registry and remap
the AC2 development images via the dev-console chart's `imageRewrites` (the
console image itself can be overridden with `admin-ui.image.repository`).

## Key values

| Parameter | Default | Description |
|-----------|---------|-------------|
| `admin-ui.image.repository` | `registry-cn-hangzhou.ack.aliyuncs.com/acs/ai-dashboard` | Console image |
| `admin-ui.image.tag` | `v3.3.0` | Console image tag |
| `admin-ui.auth.mode` | `none` | Auth mode (`none`/`aliyun`/`custom`) |
| `admin-ui.dashboard.ingress.hosts[0].host` | `ai-dashboard.local` | Ingress domain |
| `grafana.adminPassword` | `""` | Grafana admin password (set this) |
| `prometheus.enabled` | `true` | Deploy in-cluster Prometheus |
| `platformConfig.enabled` | `true` | Seed AC2 default images on first install |
| `ensureDefaultStorageClass` | `true` | Ensure a default StorageClass exists |
