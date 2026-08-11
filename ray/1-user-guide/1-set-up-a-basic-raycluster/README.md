# Set Up and Use a Basic RayCluster

This guide shows how to deploy a basic RayCluster (1 head + 1 worker) on an ACK cluster via KubeRay, and run a simple Ray Actor sample program on it.

## Prerequisites

- [ACK managed cluster](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- [KubeRay Operator](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components) installed in the cluster

## 1. Deploy the RayCluster

```bash
kubectl apply -f raycluster.yaml
```

Expected output:

```
raycluster.ray.io/demo-ray-cluster created
```

Wait for the pods to be ready:

```bash
kubectl get pods
```

Expected output (1 head pod and 1 worker pod, both Running):

```
NAME                                          READY   STATUS    RESTARTS   AGE
demo-ray-cluster-head-mgdnb                   1/1     Running   0          2m16s
demo-ray-cluster-worker-group-worker-lvbr2    1/1     Running   0          2m16s
```

## 2. Check Ray Cluster Status

Log in to the head pod and check the cluster status with `ray status`:

```bash
HEAD_POD=$(kubectl get pod -l ray.io/node-type=head -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $HEAD_POD -- ray status
```

Expected output (the head node has `num-cpus` set to 0 so it does not take compute tasks; the cluster has 2 CPUs in total):

```
======== Autoscaler status: 2026-08-10 06:08:43.387414 ========
Node status
---------------------------------------------------------------
Active:
 (no active nodes)
Idle:
 1 worker-group
 1 headgroup
Pending:
 (no pending nodes)
Recent failures:
 (no failures)

Resources
---------------------------------------------------------------
Total Usage:
 0.0/2.0 CPU
 0B/6.00GiB memory
 0B/1.52GiB object_store_memory

From request_resources:
 (none)
Pending Demands:
 (no resource demands)
```

## 3. Run a Ray Sample Program

The script connects to the local Ray cluster via `ray.init(address="auto")`: a `RandIntActor` draws a random integer between 1 and 100, then an `AddActor` adds 5 to it and returns the result. First copy it into the head pod:

```bash
kubectl cp ray_actor_rand_and_sum.py $HEAD_POD:/tmp/ray_actor_rand_and_sum.py
```

### Method 1: Run directly in the head pod

```bash
kubectl exec -it $HEAD_POD -- python /tmp/ray_actor_rand_and_sum.py
```

Expected output (the random number varies on each run):

```
Random number: 61
Final result: 66
```

### Method 2: Submit via the Ray Job CLI

`ray job submit` submits the program as a Ray Job managed by the cluster, which fits tasks that need background scheduling. Submit and wait for the job to finish:

```bash
kubectl exec -it $HEAD_POD -- ray job submit -- python /tmp/ray_actor_rand_and_sum.py
```

Expected output (the job ID varies on each run):

```
Random number: 1
Final result: 6
Job 'raysubmit_phQgB2Qrh6VKFYd9' succeeded
```

Check the job list:

```bash
kubectl exec -it $HEAD_POD -- ray job list
```

Expected output (abbreviated, the submitted job shows status SUCCEEDED):

```
Job submission server address: http://10.246.1.67:8265
[JobDetails(..., submission_id='raysubmit_phQgB2Qrh6VKFYd9', status=<JobStatus.SUCCEEDED: 'SUCCEEDED'>, entrypoint='python /tmp/ray_actor_rand_and_sum.py', ...)]
```

View the job logs:

```bash
kubectl exec -it $HEAD_POD -- ray job logs raysubmit_phQgB2Qrh6VKFYd9
```

Expected output:

```
Job submission server address: http://10.246.1.67:8265
...
Random number: 1
Final result: 6
```

## Cleanup

```bash
kubectl delete -f raycluster.yaml
```
