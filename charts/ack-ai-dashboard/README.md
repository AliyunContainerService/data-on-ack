ack-ai-dashboard是一个面向集群管理员的运维管控台，提供集群监控大盘、数据集管理加速、用户级资源Quota分配、任务列表及费用预估等能力，帮助用户在Kubernetes集群中快速搭建管理机器学习工作环境。

### 前提条件
在安装ack-ai-dashboard之前，如下信息需要明确：
- 该组件不能单独工作，依赖**ack-arena**和**ack-arena-exporter**。
- **任务列表**监控页面需要单独配置权限。
- 集群类型：**ACK Pro版集群**。

### 安装
ack-ai-dashboard的安装步骤如下：

1. [容器服务kubernetes版控制台](https://cs.console.aliyun.com/#/k8s/cluster/list)。
2. 选择Ack Pro版集群，在左侧导航栏选择**应用** - **AI&大数据**。
3. 点击**一键部署**按钮。
4. 勾选**AI-Dashboard**控制台，在弹出的对话框中根据指引配置权限，并绑定管理员云账号UID。具体步骤见本文**安装AI-Dashboard控制台**一节。
5. 勾选**运维组件**，点击页面最下方**部署KubeAI**按钮。

## 安装Ai-Dashboard控制台。

为了控制ack-ai-dashboard的访问安全，这里接入了阿里云Ram认证体系。ack-ai-dashboard作为Ram的WebAPP，通过提供Ram颁发的client身份，获取OAuth的访问token。
所以需要安装前提供：

   1. 具有访问权限的管理员账号aliuid。
   1. 提供ram为webApp颁发的clientID和clientSecret。

具体获取这些信息的步骤如下：

   - 在[ram用户管理界面](https://ram.console.aliyun.com/users)，创建管理员账号。

   - 在ram[应用管理界面](https://ram.console.aliyun.com/applications)，创建应用，具体参考[ram官方文档](https://help.aliyun.com/document_detail/93693.html?spm=a2c8b.12215442.0.dexternal.18fb336aT9CAz4)。

   - 在应用详情页，添加aliuid授权范围。

   - 为ack-ai-dashboard创建访问ram的secret。至此，ack-ai-dashboard就可以安装了。

   - 安装完成，配置完ack-ai-dashboard的访问方式后。还需要在ram控制台，配置WebApp的回调地址。

### 访问方式

目前提供四种参考访问方式，具体参考[ack-ai-dashboard访问方式配置文档](https://yuque.antfin.com/op0cg2/dw3nil/du1a0r)




