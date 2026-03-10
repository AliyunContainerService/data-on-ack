# data-on-ack

data-on-ack is an open-source cloud-native AI/ML platform built on Alibaba Cloud Container Service for Kubernetes (ACK). It provides a set of tools and components to help data scientists and ML engineers build, train, and manage machine learning workloads on Kubernetes clusters.

## Projects

### [ai-dashboard](./ai-dashboard)

A cluster operations dashboard for administrators. It provides:
- Cluster monitoring overview
- Dataset management and acceleration
- User-level resource quota allocation
- Job list and cost estimation

The backend is built with Spring Boot (JDK 11) and the frontend with Vue.js and Element UI.

### [ai-dev-console](./ai-dev-console)

A model development and training console for algorithm engineers. It enables users to quickly set up a deep learning environment on Kubernetes clusters. Key features include:
- Cluster resource overview
- Dataset management
- Code management
- Single-machine training
- Distributed training
- Scheduled jobs
- Job history

### [commit-agent](./commit-agent)

A gRPC-based agent that runs inside Jupyter Notebook pods to synchronize code changes. It supports both Docker and containerd container runtimes and provides a client/server interface for code commit operations.

### [notebook-controller](./notebook-controller)

A Kubernetes controller that manages `Notebook` custom resources (Jupyter notebooks). It watches for Notebook CRDs and reconciles the corresponding Kubernetes resources (pods, services, etc.).

### [charts](./charts)

Helm charts for deploying the following components:
- **ack-ai-dashboard** – Helm chart for the AI Dashboard component
- **ack-ai-dev-console** – Helm chart for the AI Dev Console component

## Prerequisites

- Kubernetes cluster (ACK Pro edition recommended)
- [ack-arena](https://github.com/kubeflow/arena) installed in the cluster

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](./LICENSE) file for details.
