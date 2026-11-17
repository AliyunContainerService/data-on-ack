# ack-ai-dev-console

Model development console for the ACK Cloud-Native AI Suite: interactive
notebooks (JupyterLab / VS Code), distributed training jobs (PyTorch / TensorFlow
/ MPI / Ray), inference serving with an API playground, datasets and a model
registry. Includes the notebook-controller and the commit-agent DaemonSet.

## Prerequisites

- ACK Pro cluster (Kubernetes >= 1.20) with `kubectl` / Helm 3 configured.
- **Nginx Ingress Controller** installed.
- Cloud-Native AI Suite / `ack-arena` installed (training operators).
- Install `ack-ai-dashboard` alongside (provides user management and the admin
  User record); the dev-console provisions the admin ServiceAccount/token used
  to submit workloads in `none` mode.

## Quick start (login-free, out of the box)

The default `dev-console.auth.mode` is `none`. The chart provisions the `admin`
ServiceAccount, token Secret and cluster-admin binding automatically, so
notebook / training / serving submission works immediately.

```bash
kubectl create namespace kube-ai

helm install ack-ai-dev-console charts/ack-ai-dev-console -n kube-ai \
  --set 'dev-console.console.ingress.hosts[0].host=ai-dev.example.com' \
  --set 'dev-console.console.ingress.hosts[0].paths[0]=/'
```

### Access

```bash
kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
# <SLB_IP> ai-dev.example.com
```

Open `http://ai-dev.example.com/`.

> ⚠️ **Security**: `none` mode grants full admin access to anyone who can reach
> the URL. Use only on intranet / PoC or behind access control; for production
> use `aliyun` or `custom`, or restrict the Ingress/SLB.

## Development images

Notebook and training image defaults use the public **AC2 container images**
(Alibaba Cloud Linux, designed for the ACK AI suite), e.g.
`ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/jupyter`,
`.../jupyter-pytorch-cuda`, `.../vscode`, `.../pytorch`.

### Offline / cross-region clusters

If nodes cannot pull the AC2 registry directly, sync the AC2 images to a
reachable registry and configure prefix rewrites (applied to the built-in image
defaults at runtime):

```bash
helm upgrade ack-ai-dev-console charts/ack-ai-dev-console -n kube-ai --reuse-values \
  --set 'dev-console.imageRewrites[0].from=ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/' \
  --set 'dev-console.imageRewrites[0].to=registry-vpc.cn-beijing.aliyuncs.com/<ns>/'
```

The console / notebook-controller / commit-agent images themselves default to
`registry-cn-hangzhou.ack.aliyuncs.com/acs/` and can be overridden via
`dev-console.image.repository`, `notebook.image.repository` and
`dev-console.image.commitAgentImageName`.

## Authentication modes

`none` (default, login-free), `aliyun` (RAM OAuth2 SSO, needs the
`ai-dev-console-credentials` Secret + IMS permissions), `custom` (trusted gateway
headers). See `docs/deploy-guide.md`.

## Key values

| Parameter | Default | Description |
|-----------|---------|-------------|
| `dev-console.image.repository` | `registry-cn-hangzhou.ack.aliyuncs.com/acs/ai-dev-console` | Console image |
| `dev-console.image.tag` | `v3.2.0` | Console image tag |
| `dev-console.image.commitAgentImageName` | `registry-cn-hangzhou.ack.aliyuncs.com/acs/commit-agent` | commit-agent image |
| `dev-console.image.commitAgentImageTag` | `v0.2.0` | commit-agent tag |
| `notebook.image.repository` | `registry-cn-hangzhou.ack.aliyuncs.com/acs/notebook-controller` | notebook-controller image |
| `notebook.image.tag` | `v1.1.0` | notebook-controller tag |
| `dev-console.auth.mode` | `none` | Auth mode |
| `dev-console.console.ingress.hosts[0].host` | `ai-dev-console.local` | Ingress domain |
| `dev-console.imageRewrites` | `[]` | AC2 image registry prefix rewrites |
