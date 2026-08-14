# Run a Basic Ray Job

This guide shows how to run a Ray Job on an ACK cluster via KubeRay, in two ways:

1. Submit a Ray Job to an **existing RayCluster**
2. Create a **new RayCluster** to run a Ray Job (the cluster is cleaned up automatically after the job finishes)

The sample code is stored in a Kubernetes ConfigMap and mounted into the Ray pods, so no image rebuild is needed.

## Prerequisites

- [ACK managed cluster](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- [KubeRay Operator](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components) installed in the cluster

## 1. Store the Sample Code in a ConfigMap

The Ray Job entrypoint runs `python /home/ray/job/ray_job_sample.py`. The script connects to the Ray cluster via `ray.init(address="auto")`, computes squares of `[1, 2, 3, 4, 5]` with Ray remote tasks, then sums them with a Ray actor.

Create the ConfigMap from the local file (the data key is the file name `ray_job_sample.py`):

```bash
kubectl create configmap ray-job-code --from-file=ray_job_sample.py
```

Expected output:

```
configmap/ray-job-code created
```

The ConfigMap is mounted into the Ray pods at `/home/ray/job`, so the script is available at `/home/ray/job/ray_job_sample.py`.

## 2. Method 1: Submit to an Existing RayCluster

`ray-job-existing-cluster.yaml` contains two parts:

1. A RayCluster named `demo-ray-cluster`, with the sample code ConfigMap mounted at `/home/ray/job` on the head pod, and labeled `ray.io/cluster: demo-ray-cluster`
2. A RayJob that targets this cluster via `clusterSelector` (no new RayCluster is created)

If you already have a RayCluster running, keep only the RayJob part and make sure the ConfigMap is mounted on its head pod and the cluster carries the label used by `clusterSelector`.

```bash
kubectl apply -f ray-job-existing-cluster.yaml
```

Expected output:

```
raycluster.ray.io/demo-ray-cluster created
rayjob.ray.io/rayjob-existing-cluster created
```

The RayJob stays `Initializing` until the cluster is ready, then submits the job:

```bash
kubectl get rayjob rayjob-existing-cluster
```

Expected output (the job ID varies on each run):

```
NAME                      JOB STATUS   DEPLOYMENT STATUS   RAY CLUSTER NAME   START TIME             END TIME               AGE
rayjob-existing-cluster   SUCCEEDED    Complete            demo-ray-cluster   2026-08-14T07:43:21Z   2026-08-14T07:45:23Z   2m3s
```

The submitter pod logs show the job output:

```bash
kubectl logs job/rayjob-existing-cluster
```

Expected output (abbreviated):

```
numbers: [1, 2, 3, 4, 5]
squares: [1, 4, 9, 16, 25]
total: 55
Job 'rayjob-existing-cluster-bq5pj' succeeded
```

Because `shutdownAfterJobFinishes` is `false`, the existing cluster keeps running after the job finishes.

## 3. Method 2: Create a New RayCluster to Run the Ray Job

`ray-job-new-cluster.yaml` embeds the whole cluster spec in the RayJob (`rayClusterSpec`). The operator creates a RayCluster (1 head + 1 worker) named after the RayJob, submits the job, and shuts the cluster down when the job finishes.

```bash
kubectl apply -f ray-job-new-cluster.yaml
```

Expected output:

```
rayjob.ray.io/rayjob-new-cluster created
```

While the cluster is being provisioned, the RayJob shows `Initializing`:

```bash
kubectl get rayjob rayjob-new-cluster
```

Expected output:

```
NAME                 JOB STATUS   DEPLOYMENT STATUS   RAY CLUSTER NAME           START TIME             END TIME   AGE
rayjob-new-cluster                Initializing        rayjob-new-cluster-4jw7m   2026-08-14T07:45:49Z              5s
```

Wait for the job to finish and check the result:

```bash
kubectl wait --for=condition=complete job/rayjob-new-cluster --timeout=300s
kubectl get rayjob rayjob-new-cluster
```

Expected output:

```
NAME                 JOB STATUS   DEPLOYMENT STATUS   RAY CLUSTER NAME           START TIME             END TIME               AGE
rayjob-new-cluster   SUCCEEDED    Complete            rayjob-new-cluster-4jw7m   2026-08-14T07:45:49Z   2026-08-14T07:46:25Z   37s
```

Since `shutdownAfterJobFinishes` is `true`, the RayCluster pods are deleted automatically after the job succeeds:

```bash
kubectl get pods -l ray.io/cluster=rayjob-new-cluster-4jw7m
```

Expected output:

```
No resources found in default namespace.
```

## 4. Check the Ray Job Status

View all RayJobs and their statuses:

```bash
kubectl get rayjobs
```

View the detailed status of a RayJob (dashboard address, submission ID, cluster name, etc.):

```bash
kubectl describe rayjob rayjob-existing-cluster
```

```
Status:
  Dashboard URL:         demo-ray-cluster-head-svc.default.svc.cluster.local:8265
  Job Deployment Status: Complete
  Job Id:                rayjob-existing-cluster-bq5pj
  Job Status:            SUCCEEDED
  Message:               Job finished successfully.
  Ray Cluster Name:      demo-ray-cluster
```

View the job from inside the Ray cluster (head pod):

```bash
HEAD_POD=$(kubectl get pod -l ray.io/node-type=head -l ray.io/cluster=demo-ray-cluster -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $HEAD_POD -- ray job list
kubectl exec -it $HEAD_POD -- ray job logs rayjob-existing-cluster-bq5pj
```

## Cleanup

```bash
kubectl delete -f ray-job-existing-cluster.yaml
kubectl delete -f ray-job-new-cluster.yaml
kubectl delete configmap ray-job-code
```
