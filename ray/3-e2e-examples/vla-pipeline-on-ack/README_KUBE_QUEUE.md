# Manage VLA jobs with ack-kube-queue on ACK

English | [简体中文](./README_KUBE_QUEUE_ZH.md)

This guide uses a real VLA data processing job that converts egocentric video into a LeRobot dataset. It shows how to queue a RayJob with ack-kube-queue on Alibaba Cloud Container Service for Kubernetes (ACK), admit it by adjusting its quota, and inspect the result. Follow the existing [VLA pipeline guide](./README.md) to prepare the VLA operators, models, container image, and OSS volumes. This guide focuses on job queueing.

## Workflow

1. Prepare an ACK cluster, GPU node, VLA image, models, and input data.
2. Install ack-kube-queue from **Applications → Cloud-native AI Suite** in the ACK console and enable RayJob queueing.
3. Assign a leaf quota in ElasticQuotaTree to the VLA namespace.
4. Submit a RayJob with `spec.suspend: true` and observe it waiting in the queue.
5. Increase the available quota, observe the job dequeue and execute, and verify its LeRobot output.

## Prerequisites

- An ACK cluster that supports ack-kube-queue, the managed KubeRay Operator, and `kubectl` access. Check the supported cluster types and versions in the [ack-kube-queue component documentation](https://help.aliyun.com/en/ack/cloud-native-ai-suite/product-overview/ack-kube-queue).
- At least one schedulable GPU node, plus enough CPU and memory for a Ray head. The sample RayJob requests **8 CPU, 32 GiB of memory, and one GPU** in total. Allow additional capacity for system components.
- Input video, MoGe, HaWoR, and MANO models, and a container image with Ray, Data-Juicer, and the VLA dependencies, prepared as described in the [VLA pipeline guide](./README.md). The Data-Juicer version in the image must match the version required by the script.
- An input and code PVC readable by every Ray pod, and a writable output PVC. Use separate OSS prefixes for input and output. Create any private image pull Secret in the VLA job namespace. Do not place AccessKeys or tokens in the RayJob manifest.
- Permission to install and configure Cloud-native AI Suite components in the ACK console, and Kubernetes RBAC permissions to create namespaces, ElasticQuotaTree, and RayJobs.

The example uses the `vla-demo` namespace. Replace the image and PVC names with your own. If the cluster already has an ElasticQuotaTree, add a leaf to that tree instead of creating another one. ACK currently supports only one ElasticQuotaTree in the `kube-system` namespace.

## Step 1 — Install ack-kube-queue in the ACK console

1. Sign in to the [ACK console](https://cs.console.aliyun.com/), open **Clusters**, and select your cluster.
2. In the cluster navigation, choose **Applications → Cloud-native AI Suite**.
3. If the suite is not installed, select **Kube-Queue** on its deployment page. If the suite is already installed, find **ack-kube-queue** in the component list and click **Deploy**. See [Manage AI/ML workloads with ack-kube-queue](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/use-ack-kube-queue-to-manage-job-queues) for the console workflow.
4. Enable **RayJob** queueing in the component parameters. By default, ack-kube-queue enables queueing for native Jobs only. If the component is already installed, update its parameters on the same page.

Confirm that the console shows the component as deployed, then check its workloads and CRD:

```bash
kubectl -n kube-queue get deploy
kubectl get crd queueunits.scheduling.x-k8s.io
```

RayJob also requires the managed KubeRay Operator. See [Install KubeRay on ACK](https://help.aliyun.com/en/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components).

## Step 2 — Create the VLA namespace and queue quota

```bash
kubectl create namespace vla-demo
```

The following tree associates its leaf quota only with `vla-demo`. The initial leaf quota is 1 CPU and 1 GiB, below the RayJob's request, so you can observe queueing. If your cluster already has a quota tree, have the cluster administrator merge this leaf into the existing tree and size the root quota for the cluster.

```yaml
# vla-quota.yaml
apiVersion: scheduling.sigs.k8s.io/v1beta1
kind: ElasticQuotaTree
metadata:
  name: vla-demo
  namespace: kube-system
spec:
  root:
    name: root
    max:
      cpu: "16"
      memory: 64Gi
      nvidia.com/gpu: "1"
    min:
      cpu: "16"
      memory: 64Gi
      nvidia.com/gpu: "1"
    children:
      - name: root.vla-demo
        namespaces: [vla-demo]
        max:
          cpu: "1"
          memory: 1Gi
        min:
          cpu: "1"
          memory: 1Gi
```

```bash
kubectl apply -f vla-quota.yaml
kubectl get elasticquotatree -n kube-system
kubectl get queue -A
```

ack-kube-queue creates a queue for each ElasticQuotaTree leaf and routes RayJobs from the associated namespace to that queue. See [ElasticQuotaTree and job queues](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/improve-resource-utilization-with-elasticquotatree-and-task-queues).

## Step 3 — Submit the VLA RayJob

First, build the image and deploy the input and output PVCs as described in the [VLA pipeline guide](./README.md). The image must contain the Data-Juicer version expected by the script. **This example has only one GPU worker**, so set the pipeline script to a one-sample, one-GPU configuration: `LIMIT=1`, `GPU_BATCH_SIZE=1`, `MOGE_ACTOR_MIN=MOGE_ACTOR_MAX=1`, and `COMBINED_ACTOR_MIN=COMBINED_ACTOR_MAX=1`. Keep `MOGE_GPUS=0.25` and `COMBINED_GPUS=0.25`. The guide's default number of GPU actors may otherwise wait indefinitely for more GPUs. Replace the image, PVC, Ray version, and entrypoint below with your own values. `/data/oss/code/vla_pipeline.py` is the real video-processing and LeRobot-export script.

```yaml
# vla-rayjob.yaml
apiVersion: ray.io/v1
kind: RayJob
metadata:
  name: vla-egodex-demo
  namespace: vla-demo
spec:
  suspend: true
  submissionMode: K8sJobMode
  entrypoint: python /data/oss/code/vla_pipeline.py
  shutdownAfterJobFinishes: true
  ttlSecondsAfterFinished: 3600
  submitterPodTemplate:
    spec:
      restartPolicy: Never
      imagePullSecrets:
        - name: vla-acr-pull
      containers:
        - name: ray-job-submitter
          image: <ACR_REGISTRY>/<NAMESPACE>/<VLA_IMAGE>:<TAG>
  rayClusterSpec:
    rayVersion: "2.56.0" # Match the Ray version in the image.
    headGroupSpec:
      rayStartParams:
        num-cpus: "0"
        dashboard-host: "0.0.0.0"
      template:
        spec:
          imagePullSecrets:
            - name: vla-acr-pull
          containers:
            - name: ray-head
              image: <ACR_REGISTRY>/<NAMESPACE>/<VLA_IMAGE>:<TAG>
              resources:
                requests: {cpu: "2", memory: 8Gi}
                limits: {cpu: "3", memory: 12Gi}
              env:
                - name: PYTHONPATH
                  value: /data/oss/code
              volumeMounts:
                - {name: input, mountPath: /data/oss, readOnly: true}
                - {name: output, mountPath: /data/output}
          volumes:
            - name: input
              persistentVolumeClaim: {claimName: vla-input-pvc, readOnly: true}
            - name: output
              persistentVolumeClaim: {claimName: vla-output-pvc}
    workerGroupSpecs:
      - groupName: gpu-worker
        replicas: 1
        minReplicas: 1
        maxReplicas: 1
        rayStartParams:
          num-cpus: "6"
        template:
          spec:
            imagePullSecrets:
              - name: vla-acr-pull
            containers:
              - name: ray-worker
                image: <ACR_REGISTRY>/<NAMESPACE>/<VLA_IMAGE>:<TAG>
                resources:
                  requests: {cpu: "6", memory: 24Gi, nvidia.com/gpu: "1"}
                  limits: {cpu: "6", memory: 28Gi, nvidia.com/gpu: "1"}
                env:
                  - name: PYTHONPATH
                    value: /data/oss/code
                volumeMounts:
                  - {name: input, mountPath: /data/oss, readOnly: true}
                  - {name: output, mountPath: /data/output}
            volumes:
              - name: input
                persistentVolumeClaim: {claimName: vla-input-pvc, readOnly: true}
              - name: output
                persistentVolumeClaim: {claimName: vla-output-pvc}
```

```bash
kubectl apply -f vla-rayjob.yaml
kubectl get rayjob vla-egodex-demo -n vla-demo
kubectl get queueunit -n vla-demo
kubectl describe queueunit vla-egodex-demo-ray-qu -n vla-demo
```

The RayJob requests 8 CPU, 32 GiB, and one GPU, exceeding the initial leaf quota. The QueueUnit should remain pending with `Insufficient quota` in `status.message`, while the RayJob's `spec.suspend` remains `true`. The component generates the QueueUnit name; if it differs from this example, use the name shown by `kubectl get queueunit -n vla-demo`.

## Step 4 — Increase the quota and observe execution

In `vla-quota.yaml`, change the leaf's `max` and `min` to the values below, leaving the root quota unchanged, and apply the file again:

```yaml
max:
  cpu: "16"
  memory: 64Gi
  nvidia.com/gpu: "1"
min:
  cpu: "16"
  memory: 64Gi
  nvidia.com/gpu: "1"
```

```bash
kubectl apply -f vla-quota.yaml
kubectl get queueunit -n vla-demo -w
kubectl get rayjob vla-egodex-demo -n vla-demo -w
kubectl get pods -n vla-demo -o wide
```

The QueueUnit should become `Dequeued`. ack-kube-queue then sets the RayJob's `spec.suspend` to `false`, and KubeRay creates the Ray head, GPU worker, and submitter pods. The RayJob should eventually report `jobDeploymentStatus: Complete` and `jobStatus: SUCCEEDED`. If the cluster has only one 8-CPU GPU node, use `headGroupSpec.template.spec.nodeSelector` or node affinity to place the Ray head on a CPU node, leaving enough CPU for the GPU worker.

## Step 5 — Inspect logs and output

```bash
kubectl get rayjob vla-egodex-demo -n vla-demo \
  -o jsonpath='{.status.jobDeploymentStatus}{" "}{.status.jobStatus}{"\n"}'
kubectl logs -n vla-demo job/vla-egodex-demo --tail=100
```

On success, the application logs include the input row count, output row count, and LeRobot dataset path. Check the OSS prefix behind the output PVC for episode files under `lerobot_dataset/data/`, `meta/`, and `videos/`. The real VLA job performs frame extraction, MoGe camera calibration, HaWoR/MegaSAM hand and scene reconstruction, action computation, captioning, and LeRobot export. Exact output depends on the input video and script parameters.

## Troubleshooting

| Symptom | What to check |
| --- | --- |
| No QueueUnit appears | Verify that RayJob queueing is enabled, the RayJob has `spec.suspend: true`, and its namespace is mapped to an ElasticQuotaTree leaf. |
| QueueUnit keeps waiting | Inspect `kubectl describe queueunit` and ensure the CPU, memory, and GPU requests fit the available quota. |
| Pods remain Pending after dequeue | Check free node resources, the GPU device plugin, node affinity, and volume events. Quota admission does not guarantee that a node can fit the pods. |
| `ImagePullBackOff` or 401 | Check the image address and whether the pull Secret in this namespace can access the target ACR repository. |
| Entrypoint reports a dependency version mismatch | Align the Ray, Data-Juicer, and VLA script versions in the image, then rebuild it. |

## References

- [VLA pipeline demo](./README.md)
- [Manage AI/ML workloads with ack-kube-queue](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/use-ack-kube-queue-to-manage-job-queues)
- [ElasticQuotaTree](https://help.aliyun.com/en/ack/ack-managed-and-ack-dedicated/user-guide/elasticquotatree)
