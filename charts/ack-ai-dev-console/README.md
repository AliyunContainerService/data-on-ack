# ack-ai-dev-console

`ack-ai-dev-console` is a component of the Cloud-Native AI Suite. It is a model development and training console for algorithm engineers, helping users quickly set up a deep learning environment on Kubernetes clusters. Key features include:

- Cluster resource overview
- Dataset management
- Code management
- Single-machine training
- Distributed training
- Scheduled jobs
- Job history

## Prerequisites

Before installing `ack-ai-dev-console`, ensure the following:

- This component cannot work independently. It depends on **ack-arena**.
- Cluster type: **ACK Pro cluster** or **ACK@Edge cluster**.

## Installation

`ack-ai-dev-console` supports two installation methods. Choose the one that best fits your needs.

### Via ACK Console

1. Log in to Alibaba Cloud and go to the [ACK Console](https://cs.console.aliyun.com/#/k8s/cluster/list).
2. Select an ACK Pro cluster and navigate to **Applications** → **AI Engineering Acceleration** in the left sidebar.
3. If this is a fresh installation, click the **One-click Deploy** button to enter the component selection page. If the Cloud-Native AI Suite has been installed before, find `ack-ai-dev-console` in the component list and click **Install**.
4. Under the interaction mode section, select **Dev Console**. In the pop-up dialog, configure permissions as instructed and select an access method.
5. Click **Deploy Cloud-Native AI Suite** to complete the installation.

### Via Helm

1. Log in to Alibaba Cloud and go to the [ACK Console](https://cs.console.aliyun.com/#/k8s/cluster/list).
2. In the left sidebar, select **Marketplace** → **App Catalog** and choose **ack-ai-dev-console**.
3. On the **ack-ai-dev-console** page, switch to the **Parameters** tab to review and update the configuration values.
4. In the **Create** panel on the right, select the target cluster and namespace, then click **Create**.

#### Configuration Parameters

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
2. Select an ACK Pro cluster and navigate to **Applications** → **AI Engineering Acceleration** in the left sidebar.
3. Click the **Dev Console** link in the upper-left area of the Cloud-Native AI Suite component list page to open the dev console.
4. To grant access to other users, create a RAM sub-account and share the console link. Sub-account users can log in with their own credentials.
5. If deployed via Helm, obtain the service address using `kubectl` or configure an Ingress to access the console.

