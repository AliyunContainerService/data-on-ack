ack-ai-dev-console 是云原生AI套件中的一个组件，是一个面向算法工程师的模型开发训练控制台，可帮助用户在Kubernetes集群中快速搭建深度学习工作环境。主要包含以下能力：
- 集群资源概览
- 数据集管理
- 代码管理
- 单机训练
- 分布式训练
- 定时任务
- 任务历史记录

### 前提条件
在安装ack-ai-dev-console之前，如下信息需要明确：
- 该组件不能单独工作，依赖**ack-arena**。
- 集群类型：**ACK Pro版集群**, **ACK@Edge集群**。

### 安装

ack-ai-dev-console组件提供以下两种安装方式，用户可根据需要从ACK控制台或应用目录安装。

#### 通过ACK控制台安装

ack-ai-dev-console的安装步骤如下：

1. 登录阿里云，进入[容器服务kubernetes版控制台](https://cs.console.aliyun.com/#/k8s/cluster/list)。
2. 选择Ack Pro版集群，在左侧导航栏选择**应用** - **AI工程加速**。
![image1.jpg](https://img.alicdn.com/imgextra/i1/O1CN015ukvmc1KVRg3VCoxT_!!6000000001169-0-tps-3426-1528.jpg)
3. 如果首次安装云原生AI套件，可以看到**一键部署**按钮，点击该按钮，进入组件选择安装页面。
![image2.jpg](https://img.alicdn.com/imgextra/i4/O1CN01S1xeIC21RLbtqCwCP_!!6000000006981-0-tps-3432-1520.jpg)
4. 在交互方式部分勾选**开发控制台**，在弹出的对话框中根据指引配置权限，具体步骤见本文**安装云原生AI套件**一节，并选择访问方式。
![image3.jpg](https://img.alicdn.com/imgextra/i2/O1CN0193iHCQ1Gh7J7RwIl6_!!6000000000653-0-tps-1186-690.jpg)
5. 点击**部署云原生AI套件**按钮，即可完成安装。
6. 如果之前安装过云原生AI套件的其他组件，可以在组件列表页面，找到ack-ai-dev-console，点击**安装**即可。
![image4.jpg](https://img.alicdn.com/imgextra/i3/O1CN01S0PH8L1JDIMffvSNY_!!6000000000994-0-tps-2980-1282.jpg)

#### 通过Helm安装

1. 登录阿里云，进入[容器服务kubernetes版控制台](https://cs.console.aliyun.com/#/k8s/cluster/list)。
2. 在左侧导航栏选择**市场**-**应用目录**，在右侧选中**ack-ai-dev-console**。
3. 在**应用目录**-**ack-ai-dev-console**页面上，切换到参数页面，查看和更新您所需要配置的参数。
4. 在右侧的**创建**面板中选择集群和命名空间，并单击**创建**。

其中可配置参数列表如下：

| Parameter                 | Description                                             | Default                                                     |
| ------------------------- | ------------------------------------------------------- | ----------------------------------------------------------- |
| replicaCount              | Replicas of ack-ai-dev-console deployment               | 1                                                           |
| image.repository          | Repository for image                                    | registry.cn-beijing.aliyuncs.com/acs/kubeai-dev-console |
| image.tag                 | Tag for image                                           | 1.0.0                                                       |
| image.pullPolicy          | Image pull policy                                       | IfNotPresent                                                |
| resources.limits.cpu      | CPU resource limit of ack-ai-dev-console                | 2000m                                                       |
| resources.limits.memory   | Memory resource limit of ack-ai-dev-console             | 500Mi                                                       |
| resources.requests.cpu    | CPU resource requests of ack-ai-dev-console             | 500m                                                        |
| resources.requests.memory | Memory resource requests of ack-ai-dev-console          | 100Mi                                                       |
| service.type              | Service type                                            | NodePort                                                    |
| service.port              | Service port                                            | 80                                                          |
| service.nodePort          | Service node port if specifies service type as NodePort | 31102                                                       |
| console.host              | Host of ack-ai-dev-console                              |                                                             |
| console.adminUid          | Aliyun account uid                                      |                                                             |
| console.ingress.enabled   | Enable ingress or not                                   | False                                                       |
| nodeSelector              | Node labels for kruise-manager pod                      | {}                                                          |
| tolerations               | Tolerations for kruise-manager pod                      | []                                                          |
| affinity                  | Node affinity policy for kruise-manager pod             | {}                                                          |



### 访问方式

1. 登录阿里云，进入[容器服务kubernetes版控制台](https://cs.console.aliyun.com/#/k8s/cluster/list)。
2. 选择Ack Pro版集群，在左侧导航栏选择**应用** - **AI工程加速**。

![image5.jpg](https://img.alicdn.com/imgextra/i3/O1CN01Z2RIGm1eZpmsueLaZ_!!6000000003886-0-tps-2998-618.jpg)

3. 在云原生AI套件组件列表页面左上角可以看到**开发控制台**链接，点击该链接，即可打开开发控制台。

![image6.jpg](https://img.alicdn.com/imgextra/i4/O1CN01LPYZeV1JZHNQwWDI1_!!6000000001042-0-tps-3424-1932.jpg)

4. 如果要授权给其他人访问，可以创建RAM子账号，把开发控制台链接分享给子账号用户，用子账号登录后就可以访问。
5. 如果是通过Helm安装的，则需要用户通过kubectl得到ack-ai-dev-console service地址或配置ingress来访问开发控制台。


