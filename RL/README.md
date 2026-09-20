# RL on ACK

Guides for running agentic **Reinforcement Learning (RL)** post-training on Alibaba Cloud Container
Service for Kubernetes (ACK): dataset preparation, launching an RL job, and a rollout control plane
for trajectory management. Examples use the **slime + harbor** stack (see
[0-related-repositories](1-user-guide/0-related-repositories/README.md)); a verl-based path is
covered in the run guide. Each guide ships an English `README.md` and a Chinese `README_ZH.md`.

## 1. User Guide (basics)

| # | Guide | What you learn |
|---|-------|----------------|
| 0 | [Related repositories](1-user-guide/0-related-repositories/README.md) | The three repos this stack builds on (slime / harbor / verl-recipe) and what Alibaba contributes upstream |
| 1 | [Prepare a dataset](1-user-guide/1-prepare-dataset/README.md) | ACR setup (aliyun CLI), build/push task sandbox images, pull secret, prompt manifest |
| 2 | [Run an RL task](1-user-guide/2-run-an-rl-task/README.md) | NAS + KubeRay + workspace image + RayCluster, then launch slime GRPO (or the verl recipe), monitor, read results |

## 2. Advanced

| # | Guide | Problem it solves |
|---|-------|-------------------|
| 3 | [Manage trajectories with RolloutServer](2-advanced/3-rolloutserver-trajectory-management/README.md) | Deploy a rollout control plane: run trials with fault tolerance/quota, persist trajectories, serve RL training data |

More advanced topics (image caching, P2P-accelerated environment setup) are planned.

## Prerequisites (shared)

- An [ACK managed cluster](https://help.aliyun.com/zh/ack/) with GPU nodes.
- A container registry (Alibaba Cloud ACR) reachable from the cluster.
- `kubectl`, Helm 3, Docker, and the `aliyun` CLI on your workstation.
