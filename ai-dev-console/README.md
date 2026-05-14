# ACK AI Dev Console

ACK AI Dev Console is a component of the Cloud-Native AI Suite that provides a model development and training console for algorithm engineers. It helps users quickly set up a deep learning environment on Kubernetes clusters.

## Features

- Cluster resource overview
- Dataset management
- Code management
- Single-machine training
- Distributed training
- Scheduled jobs
- Job history

## Prerequisites

- ACK Pro cluster or ACK@Edge cluster
- [ack-arena](https://github.com/kubeflow/arena) installed in the cluster

## Build

```bash
# Build the backend binary
make build

# Build and push the Docker image
make image
```

## Deployment

### Via ACK Console

1. Log in to Alibaba Cloud and go to the [ACK Console](https://cs.console.aliyun.com/#/k8s/cluster/list).
2. Select an ACK Pro cluster and navigate to **Applications** → **AI Engineering Acceleration** in the left sidebar.
3. Click **One-click Deploy** if this is a fresh installation, or click **Install** next to `ack-ai-dev-console` in the component list.
4. In the installation dialog, select **Dev Console** under the interaction mode, configure access permissions, and choose an access method.
5. Click **Deploy Cloud-Native AI Suite** to complete the installation.

### Via Helm

1. Log in to Alibaba Cloud and go to the [ACK Console](https://cs.console.aliyun.com/#/k8s/cluster/list).
2. In the left sidebar, select **Marketplace** → **App Catalog** and then choose **ack-ai-dev-console**.
3. Switch to the **Parameters** tab to review and customize the configuration values.
4. In the **Create** panel on the right, select the target cluster and namespace, then click **Create**.

#### Helm Parameters

| Parameter                 | Description                                             | Default                                                      |
| ------------------------- | ------------------------------------------------------- | ------------------------------------------------------------ |
| replicaCount              | Number of replicas for the deployment                   | 1                                                            |
| image.repository          | Docker image repository                                 | registry.cn-beijing.aliyuncs.com/acs/kubeai-dev-console      |
| image.tag                 | Docker image tag                                        | 1.0.0                                                        |
| image.pullPolicy          | Image pull policy                                       | IfNotPresent                                                 |
| resources.limits.cpu      | CPU limit                                               | 2000m                                                        |
| resources.limits.memory   | Memory limit                                            | 500Mi                                                        |
| resources.requests.cpu    | CPU request                                             | 500m                                                         |
| resources.requests.memory | Memory request                                          | 100Mi                                                        |
| service.type              | Kubernetes service type                                 | NodePort                                                     |
| service.port              | Service port                                            | 80                                                           |
| service.nodePort          | Node port (when service type is NodePort)               | 31102                                                        |
| console.host              | Hostname of the dev console                             |                                                              |
| console.adminUid          | Alibaba Cloud account UID for admin access              |                                                              |
| console.ingress.enabled   | Whether to enable Ingress                               | false                                                        |
| nodeSelector              | Node selector for the pod                               | {}                                                           |
| tolerations               | Tolerations for the pod                                 | []                                                           |
| affinity                  | Affinity policy for the pod                             | {}                                                           |

## Accessing the Console

1. Log in to Alibaba Cloud and go to the [ACK Console](https://cs.console.aliyun.com/#/k8s/cluster/list).
2. Select the ACK Pro cluster and navigate to **Applications** → **AI Engineering Acceleration** in the left sidebar.
3. Click the **Dev Console** link in the upper-left area of the Cloud-Native AI Suite component list page.
4. To grant access to other users, create a RAM sub-account and share the console link. Sub-account users can log in with their own credentials.
5. If deployed via Helm, obtain the service address using `kubectl` or configure an Ingress to access the console.
