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
| 4 | [支持数万 Rollout 并发](2-advanced/4-rolloutserver-high-concurrency/README_ZH.md) | 将 RolloutServer 扩展到 1 万–5 万并发：分布式架构、部署模式、配额/调度器/存储调优、沙箱容量、验证方法 |
| 5 | [RolloutServer 同时支持 Evaluation 和 ReinforceLearning](2-advanced/5-rolloutserver-eval-rl/README_ZH.md) | 一个 Server 同时服务评测与 RL：模型端点/凭据按请求配置、供应商 Secret 注入、Backend/Queue 隔离 |

更多进阶主题（镜像缓存、P2P 加速环境 setup）规划中。

## 前置条件（通用）

- 带 GPU 节点的 [ACK 托管集群](https://help.aliyun.com/zh/ack/)。
- 集群可访问的容器镜像仓库（阿里云 ACR）。
- 工作机上有 `kubectl`、Helm 3、Docker 与 `aliyun` CLI。
