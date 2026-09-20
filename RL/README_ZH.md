# ACK 上的 RL

在阿里云容器服务 ACK 上进行智能体**强化学习（RL）**后训练的系列指南：数据集准备、启动 RL 任务，以及管理轨迹的
rollout 管控面。示例采用 **slime + harbor** 技术栈（见
[0-related-repositories](1-user-guide/0-related-repositories/README_ZH.md)），运行指南中也覆盖基于 verl 的路径。
每篇指南都提供英文 `README.md` 与中文 `README_ZH.md`。

## 1. 用户指南（基础）

| # | 指南 | 你将学到 |
|---|------|----------|
| 0 | [相关代码库介绍](1-user-guide/0-related-repositories/README_ZH.md) | 本栈依赖的三个仓库（slime / harbor / verl-recipe）及阿里向上游贡献了什么 |
| 1 | [数据集准备](1-user-guide/1-prepare-dataset/README_ZH.md) | ACR 配置（aliyun CLI）、构建/推送任务沙箱镜像、pull secret、prompt 清单 |
| 2 | [执行 RL 任务](1-user-guide/2-run-an-rl-task/README_ZH.md) | NAS + KubeRay + workspace 镜像 + RayCluster，然后启动 slime GRPO（或 verl 配方）、监控、读结果 |

## 2. 进阶

| # | 指南 | 解决的问题 |
|---|------|-----------|
| 3 | [用 RolloutServer 管理轨迹](2-advanced/3-rolloutserver-trajectory-management/README_ZH.md) | 部署 rollout 管控面：带容错/配额地运行 trial、持久化轨迹、提供 RL 训练数据 |

更多进阶主题（镜像缓存、P2P 加速环境 setup）规划中。

## 前置条件（通用）

- 带 GPU 节点的 [ACK 托管集群](https://help.aliyun.com/zh/ack/)。
- 集群可访问的容器镜像仓库（阿里云 ACR）。
- 工作机上有 `kubectl`、Helm 3、Docker 与 `aliyun` CLI。
