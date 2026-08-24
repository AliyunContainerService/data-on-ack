# Ray GCS Fault Tolerance on Alibaba Cloud

The Global Control Service (GCS) is Ray's cluster-level metadata service. It runs inside the head pod and owns node registration, the actor registry, placement groups, job metadata and resource management. **By default it keeps all of that in the head process's memory, with no replica anywhere**. Once the head process exits, the cluster state goes with it, and a single process-level failure escalates into the loss of the entire cluster:

```
GCS process dies  → raylets on the workers lose their connection to GCS
                  → after RAY_gcs_rpc_server_reconnect_timeout_s (60s on the head) the head exits
                  → the head restarts, but the in-memory cluster state is gone
                  → the new head does not recognize the old workers and terminates them
                  → the cluster is rebuilt, not recovered
```

With GCS fault tolerance enabled, GCS writes its state to an external Redis as it runs and reloads it after a restart, while the workers' reconnect timeout is stretched to 600 seconds — long enough to sit out a head restart. A head failure is downgraded from "rebuild the cluster" to "a control-plane blip of about a minute".

```
Ray head pod                       External Redis                Worker pod
  GCS process ──── ① sync writes ───→  RAY<ns>@NODE            never restarts
  RAY_REDIS_ADDRESS                    RAY<ns>@ACTOR           RAY_gcs_rpc_server_
  REDIS_PASSWORD                       RAY<ns>@KV ...            reconnect_timeout_s=600
  RAY_external_storage_namespace              │                        │
       ↑                                      │                        │
  new GCS ←──── ② full reload on restart ─────┘                        │
       └────────────────── ③ raylet reconnects ──────────────────────→ ┘
```

Three values decide how long a failure simulation takes:

| Setting | Value | Who sets it |
|---|---|---|
| `RAY_gcs_rpc_server_reconnect_timeout_s` (head) | **60** | Ray's default; KubeRay does not inject it |
| `RAY_gcs_rpc_server_reconnect_timeout_s` (worker) | **600** | Injected by KubeRay; must exceed the head's value |
| `RAY_external_storage_namespace` | the RayCluster **UID** | Injected by KubeRay; used as the Redis key prefix |

This guide was verified on an ACK managed cluster with the managed KubeRay operator (v1.5.1) and Ray 2.51.0.

## Prerequisites

- [ACK managed cluster](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- [KubeRay Operator](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components) **v1.3.0 or later** installed in the cluster (`gcsFaultToleranceOptions` was introduced in v1.3.0)
- Ray image **2.0.0 or later**.
- Redis. A single-shard Redis Cluster or Redis Sentinel, with one or more replicas

There are two ways to provide that Redis: an **in-cluster single-replica Redis** (option A, for validation only) and **ApsaraDB for Redis / Tair** (option B, for production). Only the preparation in step 1 differs; every later step gives the commands and expected results for both.

## Configuration: `gcsFaultToleranceOptions`

Adding the field is what enables fault tolerance — `gcsFaultToleranceOptions: {}` is already a valid, enabled configuration:

```yaml
spec:
  gcsFaultToleranceOptions:
    # Option A takes a Kubernetes Service name, option B the Tair VPC endpoint
    redisAddress: "redis:6379"      # bare host:port, no redis:// prefix
    redisPassword:
      valueFrom:
        secretKeyRef:
          name: redis-password-secret
          key: password
```

| Field | Purpose |
|---|---|
| `redisAddress` | Redis endpoint as a bare `host:port`. Writing `redis://host:port` makes Ray treat the whole string as a hostname and the head fails to start with `resolve: Host not found` |
| `redisPassword` | Password, read from a Secret. KubeRay appends `--redis-password=$REDIS_PASSWORD` to the head's start command for you — do not write it into `rayStartParams` as well |
| `externalStorageNamespace` | Redis key prefix. **Leave it unset** |

Leaving `externalStorageNamespace` unset lets KubeRay default it to the RayCluster UID, which is globally unique and never reused. Several RayClusters can therefore share one Redis without interfering, and a cluster that is deleted and recreated under the same name never picks up the previous generation's state. Setting it by hand breaks exactly those properties: two clusters sharing a value overwrite each other's metadata, and a fixed value on a `RayService` makes the old and new RayClusters collide during an upgrade.

## 1. Prepare Redis

Pick one of the two options. This step produces the two things the later steps share: a Secret named `redis-password-secret` holding the password, and a pod that can run `redis-cli`.

### Option A: in-cluster Redis (validation)

```bash
kubectl apply -f redis.yaml
kubectl wait --for=condition=available deploy/redis --timeout=120s
export REDIS_POD=$(kubectl get pod -l app=redis -o jsonpath='{.items[0].metadata.name}')
```

Expected output — `deployment.apps/redis condition met`, and the pod is ready:

```
NAME                     READY   STATUS    RESTARTS   AGE
redis-6b9d5f8c4d-hq2xn   1/1     Running   0          15s
```

[`redis.yaml`](redis.yaml) runs a single Redis replica with AOF persistence and `maxmemory-policy noeviction`, with the password `5241590000000000`. **It is fit for validation only**: if a single replica loses its data, the recovered GCS loses the cluster state with it.

### Option B: ApsaraDB for Redis / Tair (production)

[ApsaraDB for Redis / Tair](https://help.aliyun.com/zh/redis/) gives you automatic failover and managed backups.

**① Instance selection**

The topologies Ray's documentation lists as supported are a **single-shard** Redis Cluster or Redis Sentinel (with one or more replicas), which on Tair corresponds to the **standard (primary/replica) architecture** — the one this guide was verified against.

GCS metadata is small — the 1 worker plus 1 actor below produce only seven keys, with the data packed into the fields of those hashes, and the key count does not grow with the cluster. Size the instance against your own actor and placement-group scale, and set alerts on memory usage, connection count and latency.

**② Network and whitelist**

The instance must sit in the **same region and the same VPC** as the ACK cluster, with the whitelist allowing the cluster's CIDR. This guide was run with the cluster's VPC CIDR allowed.

**③ Create the Secret and a long-lived redis-cli pod**

There is no Redis pod in the cluster, so the `redis-cli` needs somewhere to run: the same pod serves as the pre-deployment connectivity check and as the client used to query Redis in steps 3 and 5. Taking the password from the Secret you just created keeps it out of the command line and the shell history:
This pod is optional — it only makes inspecting the data more convenient. You can also connect to the instance directly, as described in [Connect to an ApsaraDB for Redis instance](https://help.aliyun.com/zh/redis/getting-started/step-3-connect-to-an-apsaradb-for-redis-instance).

```bash
export TAIR_HOST=r-xxxxxxxx.redis.<region>.rds.aliyuncs.com
kubectl create secret generic redis-password-secret --from-literal=password='<Tair instance password>'

kubectl run redis-client --image=redis:7.4-alpine --restart=Never \
  --env="REDISCLI_AUTH=$(kubectl get secret redis-password-secret -o go-template='{{.data.password | base64decode}}')" \
  --command -- sleep infinity
kubectl wait --for=condition=Ready pod/redis-client --timeout=60s
kubectl exec redis-client -- redis-cli -h ${TAIR_HOST} PING
```

Expected output:

```
PONG
```

> Create the Secret with `--from-literal`: creating it from a file stores the file's trailing newline too, and the head then fails to start with `WRONGPASS`.
>
> If the check fails, fix the whitelist, the security group or the password before deploying — a GCS that cannot reach Redis shows up as a head pod stuck in `CrashLoopBackOff`, which costs far more to diagnose after the fact than the check costs to run.

Finally point `redisAddress` in [`raycluster-gcs-ft.yaml`](raycluster-gcs-ft.yaml) at the instance endpoint; that is option B's only change to the RayCluster:

```yaml
    redisAddress: "r-xxxxxxxx.redis.<region>.rds.aliyuncs.com:6379"
```

## 2. Deploy a RayCluster with Fault Tolerance Enabled

[`raycluster-gcs-ft.yaml`](raycluster-gcs-ft.yaml) contains the `gcsFaultToleranceOptions` block shown above, plus a ConfigMap with the two verification scripts. The head runs with `num-cpus: "0"` so that actors are always scheduled onto a worker.

```bash
kubectl apply -f raycluster-gcs-ft.yaml
kubectl wait --for=condition=Ready pod -l ray.io/cluster=raycluster-gcs-ft --timeout=300s
kubectl get raycluster raycluster-gcs-ft
kubectl get pod -l ray.io/cluster=raycluster-gcs-ft
```

Expected output, identical for both options (`STATUS: ready`, both pods `1/1 Running` with `RESTARTS 0`):

```
NAME                DESIRED WORKERS   AVAILABLE WORKERS   CPUS   MEMORY   GPUS   STATUS   AGE
raycluster-gcs-ft   1                 1                   2      4Gi      0      ready    66s

NAME                                         READY   STATUS    RESTARTS   AGE
raycluster-gcs-ft-head-v6wxq                 1/1     Running   0          66s
raycluster-gcs-ft-small-group-worker-z5qgc   1/1     Running   0          66s
```

**A ready cluster does not prove fault tolerance is active.** Run all three checks below; on a cluster without fault tolerance every one of them comes back empty.

```bash
# A namespace may hold several Ray clusters, so the selectors must include ray.io/cluster or they may return a pod from a different cluster
export HEAD_POD=$(kubectl get pod -l ray.io/cluster=raycluster-gcs-ft,ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
export WORKER_POD=$(kubectl get pod -l ray.io/cluster=raycluster-gcs-ft,ray.io/group=small-group -o jsonpath='{.items[0].metadata.name}')
```

**① KubeRay attached the Redis cleanup finalizer**

```bash
kubectl get raycluster raycluster-gcs-ft -o jsonpath='{.metadata.finalizers}{"\n"}'
# ["ray.io/gcs-ft-redis-cleanup-finalizer"]
```

**② The head received the three environment variables and the `--redis-password` argument**

```bash
kubectl get pod ${HEAD_POD} \
  -o jsonpath='{range .spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' \
  | grep -iE "redis|external_storage"
```

Expected output — four lines. The first three are the injected environment variables (`REDIS_PASSWORD` prints empty because its value comes from a `secretKeyRef`); the fourth is KubeRay's generated start command, matched by the same `grep` because its value contains `redis` (arguments unrelated to fault tolerance are elided below). With option B only the `RAY_REDIS_ADDRESS` line differs, showing the instance's VPC endpoint:

```
RAY_external_storage_namespace=cbf3aa87-0a34-45de-8b64-0f4f8bda7b4c    # = RayCluster UID
RAY_REDIS_ADDRESS=redis:6379                                           # option B: r-xxxxxxxx.redis.<region>.rds.aliyuncs.com:6379
REDIS_PASSWORD=
KUBERAY_GEN_RAY_START_CMD=ray start --head --block ... --num-cpus=0 --redis-password=$REDIS_PASSWORD
```

`--redis-password` is appended by the operator, not written by hand. Confirm it reached the container's arguments:

```bash
kubectl get pod ${HEAD_POD} -o jsonpath='{.spec.containers[0].args}' | tr ' ' '\n' | grep redis
# --redis-password=$REDIS_PASSWORD
```

**③ The worker received the 600-second reconnect timeout, and the head did not**

```bash
kubectl get pod ${WORKER_POD} \
  -o jsonpath='{range .spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' | grep reconnect
# RAY_gcs_rpc_server_reconnect_timeout_s=600

kubectl get pod ${HEAD_POD} -o jsonpath='{.spec.containers[0].env}' | grep -c reconnect
# 0    the head keeps Ray's 60s default, as intended
```

## 3. Create a Detached Actor and Inspect Redis

`detached_actor.py` creates a counter actor named `counter_actor`. An ordinary actor's lifetime is tied to the driver that created it, and the actor is reclaimed as soon as that driver exits; an actor created with `lifetime="detached"` outlives its driver, and the name → actor mapping is held by GCS (written to Redis under the `@ACTOR` key once fault tolerance is on), so any later driver can retrieve it with `ray.get_actor("counter_actor")`.

Every run of `increment_counter.py` is a new driver. Whether it can **look the actor up by name** shows whether GCS reloaded its metadata from Redis after the head restart; whether the **count keeps going up** shows whether the actor process itself was dragged into that restart (it runs on a worker, so the in-memory count survives as long as the worker does). Without fault tolerance, the new head's GCS holds no such registration and `ray.get_actor` fails outright, unable to find `counter_actor`.

```bash
kubectl exec ${HEAD_POD} -- python3 /home/ray/samples/detached_actor.py
kubectl exec ${HEAD_POD} -- python3 /home/ray/samples/increment_counter.py
```

Expected output:

```
1
```

Now look at what GCS wrote to Redis. `RAY_UID` is the key prefix, and you **must export it before deleting the RayCluster** — once the cluster is gone, so is the UID:

```bash
export RAY_UID=$(kubectl get raycluster raycluster-gcs-ft -o jsonpath='{.metadata.uid}')

# Option A: this database holds nothing but this cluster, so listing all of it is the clearest view
kubectl exec ${REDIS_POD} -- env REDISCLI_AUTH="5241590000000000" redis-cli KEYS "*"

# Option B: KEYS is off limits on a production instance (it blocks Redis until the walk finishes) — iterate with a cursor and filter by prefix
kubectl exec redis-client -- redis-cli -h ${TAIR_HOST} --scan --pattern "RAY${RAY_UID}*"
```

Expected output, identical for both options — every key carries the `RAY<UID>` prefix:

```
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@KV                 # internal KV, holding assorted metadata from Ray's components
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@NODE               # node registry: each raylet's address, resources and liveness
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@JOB                # job metadata: every job a driver submitted, and its state
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@JobCounter         # auto-incrementing job ID counter
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@ACTOR              # actor registry, where counter_actor's name → ID mapping lives
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@ACTOR_TASK_SPEC    # the actor creation task spec, needed to recreate an actor
RAYcbf3aa87-0a34-45de-8b64-0f4f8bda7b4c@WORKERS            # worker process info, including abnormal-exit records
```

To read one of the hashes, `HGETALL` is written the same way for both options; only the pod it runs in changes:

```bash
kubectl exec ${REDIS_POD} -- env REDISCLI_AUTH="5241590000000000" redis-cli HGETALL "RAY${RAY_UID}@NODE"   # option A
kubectl exec redis-client -- redis-cli -h ${TAIR_HOST} HGETALL "RAY${RAY_UID}@NODE"                        # option B
```

## 4. Simulate a Head Failure

This step is the same for both options: Redis only stores the metadata, and the recovery timeline is set by the 60-second timeout on the head. Kill the GCS process inside the head pod and watch both pods:

```bash
kubectl exec ${HEAD_POD} -- pkill gcs_server

for i in $(seq 1 10); do
  echo "=== T+$((i*10))s ==="
  kubectl get pod -l ray.io/cluster=raycluster-gcs-ft --no-headers \
    -o 'custom-columns=NAME:.metadata.name,READY:.status.containerStatuses[0].ready,RESTARTS:.status.containerStatuses[0].restartCount'
  sleep 10
done
```

> Keep `custom-columns=...` quoted: zsh treats the `[0]` inside it as a glob pattern and fails with `no matches found`.

Expected timeline:

| Time | Head | Worker |
|---|---|---|
| T+0s ~ T+40s | ready=true, restarts=0 | ready=true, restarts=0 |
| **T+50s** | ready=**false** | ready=true, restarts=0 |
| **T+60s** | container restarts, restarts=**1** | ready=true, restarts=**0** |
| **T+70s** | ready=**true** | ready=true, restarts=0 |

**The worker stays `ready=true` with `RESTARTS=0` throughout — that is the effect of GCS fault tolerance.** The 60-second mark is the head's default `RAY_gcs_rpc_server_reconnect_timeout_s`. Without fault tolerance the worker gets no 600-second override and exits at the same moment as the head.

Reach the detached actor again:

```bash
kubectl exec ${HEAD_POD} -- python3 /home/ray/samples/increment_counter.py
```

Expected output — the count continues instead of starting over, because the actor never left the worker pod:

```
2
```

## 5. Delete the Cluster and Verify Redis Cleanup

Counting the keys before and after the deletion is what proves the cleanup actually happened, and the two options need different criteria: option A's database holds nothing but this cluster, so `DBSIZE` (the number of keys in the current database) is enough; option B's instance usually also hosts other Ray clusters or other applications, so `DBSIZE` never drops to zero and the count has to be scoped to the UID prefix.

```bash
# Before the deletion — option A
kubectl exec ${REDIS_POD} -- env REDISCLI_AUTH="5241590000000000" redis-cli DBSIZE
# Before the deletion — option B
kubectl exec redis-client -- redis-cli -h ${TAIR_HOST} --scan --pattern "RAY${RAY_UID}*" | wc -l

kubectl delete raycluster raycluster-gcs-ft
kubectl get job --field-selector metadata.name=raycluster-gcs-ft-redis-cleanup

# After the deletion — repeat whichever counting command matches your option
```

Expected output — KubeRay runs a cleanup Job that removes this cluster's keys before releasing the finalizer, and the Job is reclaimed afterwards by `ttlSecondsAfterFinished`:

```
NAME                              STATUS     COMPLETIONS   DURATION   AGE
raycluster-gcs-ft-redis-cleanup   Complete   1/1           5s         6s
```

| Option | Counted with | Before | After |
|---|---|---|---|
| A: in-cluster Redis | `DBSIZE` | `(integer) 7` | `(integer) 0` |
| B: Tair | count of the `RAY<uid>*` prefix | `7` | `0` |

Cleanup has been best-effort since KubeRay v1.1.0 (v1.0.0 blocked deletion until the Job succeeded): if the Job fails, the finalizer is still released and the keys stay behind, to be removed by hand using the UID prefix:

```bash
# Option A
kubectl exec ${REDIS_POD} -- sh -c \
  "redis-cli -a 5241590000000000 --scan --pattern 'RAY${RAY_UID}*' | xargs -r redis-cli -a 5241590000000000 DEL"
# Option B
kubectl exec redis-client -- sh -c \
  "redis-cli -h ${TAIR_HOST} --scan --pattern 'RAY${RAY_UID}*' | xargs -r redis-cli -h ${TAIR_HOST} DEL"
```

## Cleanup

```bash
kubectl delete -f raycluster-gcs-ft.yaml --ignore-not-found

# Option A
kubectl delete -f redis.yaml

# Option B: the Tair instance stays; only the client pod and the Secret go
kubectl delete pod redis-client --ignore-not-found
kubectl delete secret redis-password-secret --ignore-not-found
```

## References

- [GCS fault tolerance in KubeRay](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/kuberay-gcs-ft.html)
- [Tuning Redis for a persistent fault tolerant GCS](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/kuberay-gcs-persistent-ft.html)
- [Ray Serve end-to-end fault tolerance](https://docs.ray.io/en/latest/serve/production-guide/fault-tolerance.html)
- [Ray Core: GCS fault tolerance](https://docs.ray.io/en/latest/ray-core/fault_tolerance/gcs.html)
- [Best practices for Ray clusters on ACK](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ray-cluster-best-practices/)
- [Key eviction from Redis](https://redis.io/docs/latest/develop/reference/eviction/)
