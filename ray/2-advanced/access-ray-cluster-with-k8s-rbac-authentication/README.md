# Access a Ray Cluster with Kubernetes RBAC Authentication

This guide shows how to access a Ray cluster using Kubernetes RBAC authentication, verified on an ACK managed cluster with the managed KubeRay operator (v1.5.1) and Ray 2.55.0.

## Background

A Ray head exposes three ports — job submission API / dashboard (8265), Ray Client (10001) and GCS (6379) — and **none of them is authenticated by default**. Anyone who can reach port 8265 can submit a job, which is equivalent to running arbitrary code inside your worker pods, where that code inherits the pods' ServiceAccount token and any RAM role bound through RRSA.

Ray 2.55.0 and later can delegate token validation to Kubernetes: on every request it asks the ACK API server two questions, so "who may use which Ray cluster" becomes an ordinary Kubernetes RBAC question.

| Kubernetes API | Stage | Question answered |
|---|---|---|
| `TokenReview` (`authentication.k8s.io`) | Authentication | Which identity does this token belong to? |
| `SubjectAccessReview` (`authorization.k8s.io`) | Authorization | May that identity use this RayCluster? |

Authorization is expressed with the custom verb `ray:write`, applied to a single `RayCluster` resource:

```yaml
rules:
- apiGroups: ["ray.io"]
  resources: ["rayclusters"]
  verbs: ["ray:write"]
```

`SubjectAccessReview` does not require verbs to come from the standard Kubernetes verb set; it simply matches `(identity, apiGroup, resource, resourceName, verb)` against the RBAC rules. So `ray:write` is independent of `get` / `list` / `create`: a user who can run `kubectl get raycluster` does **not** thereby gain the right to submit Ray jobs, and vice versa.

**Two identities** are involved, both of which need authorization — mixing them up is the most common configuration mistake. The Ray cluster's own ServiceAccount needs permission to call the two review APIs above (plus `ray:write` on its own cluster, for head↔worker authentication); the caller's ServiceAccount only needs `ray:write` on the target cluster.

## Prerequisites

- [ACK managed cluster](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2), Kubernetes ≥ 1.24 (needs the TokenRequest API and projected volumes)
- [KubeRay Operator](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components) installed in the cluster
- Ray image version **≥ 2.55.0**
- Permission to create ClusterRoles and ClusterRoleBindings (`kubectl auth can-i create clusterroles` returns `yes`)

## 1. Deploy a Ray Cluster with Authentication Enabled

[`ray-cluster-auth.yaml`](ray-cluster-auth.yaml) contains the RBAC objects plus the RayCluster:

| Object | Kind | Purpose |
|---|---|---|
| `ray-cluster-auth` | ServiceAccount | Identity of all Ray containers |
| `ray-authenticator` | ClusterRole + ClusterRoleBinding | Allows Ray to call `TokenReview` / `SubjectAccessReview` |
| `ray-writer` | ClusterRole | Defines the `ray:write` verb on `rayclusters` |
| `ray-cluster-auth` | RoleBinding | Grants the cluster's own SA `ray:write` on itself, for head↔worker auth |
| `ray-cluster-auth` | RayCluster | Head + one worker group, both with auth enabled |

The two ClusterRoles are cluster-scoped and can be shared by multiple Ray clusters — re-applying them is harmless. Everything else is namespaced; the sample uses `default`. The Ray containers read their namespace from the Downward API (see below) and need no edit; to deploy into another namespace, change the ClusterRoleBinding in two places: **its subject's namespace** and **its own name**. The first because a ClusterRoleBinding is cluster-scoped but its subject is a namespaced ServiceAccount, whose full identity is `system:serviceaccount:<namespace>:<name>` — subjects have no namespace wildcard. The second because cluster-scoped names are global. Also adjust the `-n` / `--as=system:serviceaccount:<namespace>:...` arguments in the commands below.

```bash
kubectl apply -f ray-cluster-auth.yaml
kubectl wait --for=condition=Ready pod -l ray.io/cluster=ray-cluster-auth --timeout=300s
kubectl get raycluster,pod
```

Expected output (`STATUS` is `ready`, all pods `1/1 Running` with `RESTARTS` at 0):

```
NAME                                  DESIRED WORKERS   AVAILABLE WORKERS   STATUS   AGE
raycluster.ray.io/ray-cluster-auth    1                 1                   ready    2m

NAME                                              READY   STATUS    RESTARTS   AGE
pod/ray-cluster-auth-head-b7ktw                   1/1     Running   0          2m
pod/ray-cluster-auth-workergroup-worker-x9vqd     1/1     Running   0          2m
```

Confirm the cluster's own ServiceAccount may call the review APIs:

```bash
kubectl auth can-i create tokenreviews --as=system:serviceaccount:default:ray-cluster-auth
kubectl auth can-i create subjectaccessreviews --as=system:serviceaccount:default:ray-cluster-auth
```

Both must print `yes`.

### Environment variables

The head **and every worker group** must set all three — none is optional. They are already in the sample YAML:

| Variable | Value | Purpose |
|---|---|---|
| `RAY_AUTH_MODE` | `token` | Enables token authentication |
| `RAY_ENABLE_K8S_TOKEN_AUTH` | `true` | Delegates validation to Kubernetes RBAC |
| `RAY_CLUSTER_NAMESPACE` | namespace | Namespace for the `SubjectAccessReview`. **KubeRay v1.5.1 does not inject this one** |

`RAY_CLUSTER_NAME` (the resource name for the `SubjectAccessReview`) **does not need to be set explicitly**: KubeRay already injects it through the Downward API from the `ray.io/cluster` pod label, and the cluster was verified to work with it omitted.

Prefer the Downward API for `RAY_CLUSTER_NAMESPACE` as well, rather than a hardcoded string — Ray pods always run in the RayCluster's own namespace, so `metadata.namespace` is always the correct value:

```yaml
- name: RAY_CLUSTER_NAMESPACE
  valueFrom:
    fieldRef:
      fieldPath: metadata.namespace
```

> Omitting `RAY_CLUSTER_NAMESPACE` does not merely "weaken authentication" — it makes the cluster **completely unable to start**. Without the namespace, GCS cannot build the `SubjectAccessReview`, so it fails the authorization check before sending any request and rejects **every** request, including the head's own connection to its local GCS. `ray start --head` then retries for about 20 s and exits, the head pod enters `CrashLoopBackOff`, and workers stay stuck in `Init:0/1`.


## 2. Grant a User Access

One ServiceAccount per team or per pipeline is recommended, scoped with `--resource-name` to exactly the clusters it may access:

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

Verify the authorization boundary:

```bash
kubectl auth can-i ray:write rayclusters/ray-cluster-auth --as=system:serviceaccount:default:ray-user
# yes

kubectl auth can-i ray:write rayclusters/other-cluster --as=system:serviceaccount:default:ray-user
# no
```

## 3. Verify That Delegation Works

Forward the dashboard port:

```bash
kubectl port-forward svc/ray-cluster-auth-head-svc 8265:8265 &
```

Submitting a job without a token is rejected:

```bash
ray job submit --address http://localhost:8265 -- python -c "print('hello')"
```

Expected output:

```
ray.exceptions.AuthenticationError: Authentication required: Unauthorized: Missing authentication token
```

Submitting with the token of an authorized identity succeeds:

```bash
export RAY_AUTH_MODE=token
export RAY_AUTH_TOKEN=$(kubectl create token ray-user)

ray job submit --address http://localhost:8265 \
  -- python -c "import ray; ray.init(); print(ray.cluster_resources())"
```

Expected output:

```
Job 'raysubmit_xxxxxxxx' submitted successfully
...
{'CPU': 4.0, 'memory': ..., 'node:__internal_head__': 1.0, ...}
Job 'raysubmit_xxxxxxxx' succeeded
```

Hit the same endpoint with three identities and compare: only if all three results differ can you conclude that Ray is querying Kubernetes RBAC per identity

```bash
TOKEN_OK=$(kubectl create token ray-user)   # has ray:write
TOKEN_NO=$(kubectl create token default)    # has no ray:write
URL=http://localhost:8265/api/version

curl -s -o /dev/null -w "no token      HTTP %{http_code}\n" ${URL}
curl -s -o /dev/null -w "ray:write     HTTP %{http_code}\n" -H "Authorization: Bearer ${TOKEN_OK}" ${URL}
curl -s -o /dev/null -w "no ray:write  HTTP %{http_code}\n" -H "Authorization: Bearer ${TOKEN_NO}" ${URL}
```

| Output | Meaning |
|---|---|
| `401` `200` `403` | Delegation is fully working — Ray decides per identity |
| All three `200` | Auth is not active: the image is older than 2.55.0, or the environment variables are incomplete |

### Client-side configuration

| Variable | Purpose |
|---|---|
| `RAY_AUTH_MODE=token` | Tells the Ray CLI to attach an `Authorization: Bearer` header to requests |
| `RAY_AUTH_TOKEN` | The token itself; takes precedence |
| `RAY_AUTH_TOKEN_PATH` | File to read the token from when `RAY_AUTH_TOKEN` is unset; defaults to `~/.ray/auth_token` |

`kubectl create token` issues a token valid for 1 hour by default. Long-running jobs or pipelines can extend it (the cluster may enforce an upper bound):

```bash
export RAY_AUTH_TOKEN=$(kubectl create token ray-user --duration=8h)
```

Clients running inside the same ACK cluster can use the service DNS name `ray-cluster-auth-head-svc.default:8265` directly, with no port-forward.

## 4. Revoke Access

```bash
kubectl delete rolebinding ray-user

export RAY_AUTH_TOKEN=$(kubectl create token ray-user)
ray job submit --address http://localhost:8265 -- python -c "print('hello')"
```

Expected result: the submission fails with `AuthenticationError`.

> Revocation takes **up to 5 minutes**, because Ray caches tokens that have already passed validation. During incident response, restart the head pod to force the cache to clear (`kubectl delete pod -l ray.io/node-type=head`), or delete the head service outright to cut off the entry point.

Re-create the RoleBinding to restore that user's access:

```bash
kubectl create rolebinding ray-user --role=ray-user --serviceaccount=default:ray-user
```

## Notes

- **Only one permission level.** `ray:write` is the only verb, and **there is no read-only tier**: whoever gets access can submit and stop jobs. All authorized identities on one cluster share its resources and can see each other's jobs.
- **Granularity is per cluster.** Scope every Role with `--resource-name`, and give each team and each pipeline its own ServiceAccount so that revoking one does not affect the others. Review the RoleBinding list periodically.
- **Access auditing.** With ACK cluster auditing enabled, the audit logs collected into SLS contain a record of every `subjectaccessreviews` call, which lets you trace who accessed which Ray cluster and when.

### Simplified form on KubeRay v1.6.0+

Once the managed operator is upgraded to v1.6.0 or later, the three required environment variables and the projected volume can be replaced with:

```yaml
spec:
  rayVersion: '2.55.0'
  authOptions:
    mode: 'token'
    enableK8sTokenAuth: true
```

The operator then injects `RAY_AUTH_MODE`, `RAY_ENABLE_K8S_TOKEN_AUTH`, `RAY_CLUSTER_NAME` and `RAY_CLUSTER_NAMESPACE` into all Ray containers and mounts the projected token automatically. The RBAC objects stay unchanged. The explicit form used in this guide **still works** on v1.6.0, so you can migrate at your own pace.

## Cleanup

```bash
kubectl delete -f ray-cluster-auth.yaml
kubectl delete rolebinding ray-user --ignore-not-found
kubectl delete role ray-user --ignore-not-found
kubectl delete serviceaccount ray-user
```

## References

- [Configure Ray clusters to use Kubernetes RBAC authentication](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/kuberay-auth-rbac.html)
- [Configure Ray clusters to use token authentication](https://docs.ray.io/en/latest/cluster/kubernetes/user-guides/kuberay-auth.html)
- [Kubernetes: Authenticating (TokenReview)](https://kubernetes.io/docs/reference/access-authn-authz/authentication/) / [Authorization (SubjectAccessReview)](https://kubernetes.io/docs/reference/access-authn-authz/authorization/)
