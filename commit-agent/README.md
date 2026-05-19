# Commit Agent

Commit Agent is a gRPC-based sidecar/DaemonSet that runs on the Kubernetes node and exposes a Unix-domain-socket API so notebook pods can snapshot their running container into a new image and push it to a registry. For end-user usage, see the [official documentation](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/user-guide/create-and-use-a-jupyter-notebook).

The agent auto-detects whether the host runs Docker or containerd (override with `--runtime` or the `CONTAINER_RUNTIME` env var) and exposes:

- `v1beta1.ImageService/Version`
- `v1beta1.ImageService/CommitImage`
- `v1beta1.ImageService/PushImage`
- `grpc.health.v1.Health/{Check,Watch}` for liveness/readiness probing

## Build

```shell
go mod tidy && go mod vendor
make build           # docker image (linux/amd64 by default)
make buildx          # multi-arch (linux/amd64,linux/arm64) via buildx
make build-client    # CLI: bin/ack-commit-ctl
make test            # unit tests
```

## Generate gRPC code

```shell
cd v1beta1
protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    service.proto
```

## Server flags

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--socket-address` | `/host/run/commit-agent/commit-agent.sock` | UDS path the agent listens on |
| `--runtime` | `""` (auto) | Force `docker` or `containerd` |
| `--containerd-namespace` | `k8s.io` | Namespace used to look up containers |
| `--max-concurrent` | `4` | Max parallel commit/push RPCs |
| `--request-timeout` | `30m` | Default per-RPC timeout (push of large images can be long) |
| `--shutdown-timeout` | `30s` | Graceful shutdown deadline on SIGTERM |
| `--log-level` | `info` | `panic\|fatal\|error\|warn\|info\|debug\|trace` |
| `--log-format` | `text` | `text` or `json` |
| `--version` | – | Print version and exit |

## CLI flags (`ack-commit-ctl`)

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--server-socket` | `/mnt/commit-agent/commit-agent.sock` | UDS path (mount the host socket into the notebook pod) |
| `--timeout` | `30m` | Per-RPC deadline |
| `--dial-timeout` | `5s` | Time to establish the gRPC connection |
| `--log-level` | `info` | Log level |

`push` reads `--username`/`--password` or, if unset, the `ACR_USERNAME`/`ACR_PASSWORD` environment variables so credentials don't end up in shell history.

## Stability features

- **Graceful shutdown** on SIGINT/SIGTERM with a configurable deadline before forcing stop.
- **Panic recovery interceptor** keeps a single bad request from crashing the agent.
- **Request-ID + structured logging** interceptor: every RPC logs `method`, `request_id`, `latency_ms`, `code`.
- **Per-container mutex** serialises concurrent commits against the same container; different containers commit in parallel.
- **Bounded concurrency** via `--max-concurrent` so the node isn't OOM'd by simultaneous pushes.
- **gRPC health service** for `kubectl`/probe integration (`grpc.health.v1.Health`).
- **cgroup v1 + v2 support** in the CLI's container-ID detection (works on modern systemd-cgroup hosts).
- **Larger push-stream buffer** so very large registries don't trip `bufio.Scanner: token too long`.

## Deploy

`DaemonSet.yaml` is a hardened example: drops `hostNetwork`/`hostIPC`/`hostPID`, sets resource requests/limits, defines startup/liveness probes that verify the UDS, and uses `system-node-critical`.
