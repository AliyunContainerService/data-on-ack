---
title: 创建 Notebook 开发环境
layout: default
parent: Notebook开发环境
nav_order: 2
description: 使用 Notebook CR 创建 JupyterLab 或 Code Server 开发环境。
---

# 创建 Notebook 开发环境

Notebook Controller 使用 `Notebook` CR 中的 PodSpec 启动开发环境，并自动创建同名的 StatefulSet 和 ClusterIP Service。下面的两个示例统一使用 `/notebook/<namespace>/<name>/` 作为外部访问路径。

> **提示**：Notebook Controller 不包含开发环境创建页面、镜像模板、资源规格预设或 PVC 创建流程。需要通过页面选择镜像、资源和存储配置时，请参考[开发控制台功能使用指南中的 Notebook 管理](development-console-user-guide.html#notebook-管理)。

示例使用临时容器文件系统，删除 Notebook 后数据不会保留。生产环境应另外挂载 PVC。

## 创建 namespace

```bash
kubectl create namespace notebook-demo
```

## 创建 JupyterLab

示例镜像来自阿里云 AC2 公共镜像仓库，适合在 ACK 集群中使用。将以下内容保存为 `jupyter.yaml`：

```yaml
apiVersion: kubeflow.org/v1
kind: Notebook
metadata:
  name: jupyter-sample
  namespace: notebook-demo
spec:
  template:
    spec:
      containers:
        - name: jupyter-sample
          image: ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/jupyter:4.0.5-3.2304
          imagePullPolicy: IfNotPresent
          command:
            - /bin/sh
            - -c
          args:
            - >-
              exec jupyter-lab
              --notebook-dir=/home/jovyan
              --ip=0.0.0.0
              --no-browser
              --allow-root
              --port=8888
              --ServerApp.token=''
              --ServerApp.password=''
              --ServerApp.allow_origin='*'
              --ServerApp.base_url=${NB_PREFIX}
          ports:
            - name: notebook-port
              containerPort: 8888
          resources:
            requests:
              cpu: 500m
              memory: 1Gi
            limits:
              cpu: "2"
              memory: 4Gi
```

创建 JupyterLab：

```bash
kubectl apply -f jupyter.yaml
```

Notebook Controller 会自动注入：

```text
NB_PREFIX=/notebook/notebook-demo/jupyter-sample
```

JupyterLab 使用该值作为 `base_url`，因此能够直接处理带 Notebook 前缀的请求。

## 创建 Code Server

将以下内容保存为 `code-server.yaml`：

```yaml
apiVersion: kubeflow.org/v1
kind: Notebook
metadata:
  name: code-server-sample
  namespace: notebook-demo
  annotations:
    notebooks.kubeflow.org/http-rewrite-uri: /
spec:
  template:
    spec:
      containers:
        - name: code-server-sample
          image: ac2-registry.cn-hangzhou.cr.aliyuncs.com/ac2/vscode:4.22.1-3.2304
          imagePullPolicy: IfNotPresent
          command:
            - /bin/sh
            - -c
          args:
            - >-
              exec code-server
              --bind-addr 0.0.0.0:8080
              --auth none
              --disable-telemetry
              --disable-update-check
          ports:
            - name: notebook-port
              containerPort: 8080
          resources:
            requests:
              cpu: 500m
              memory: 1Gi
            limits:
              cpu: "2"
              memory: 4Gi
```

创建 Code Server：

```bash
kubectl apply -f code-server.yaml
```

Code Server 在根路径 `/` 提供服务。`notebooks.kubeflow.org/http-rewrite-uri: /` 表示入口代理应在转发前去掉 `/notebook/notebook-demo/code-server-sample` 前缀；具体的 ALB Ingress 配置请参见[访问 Notebook 开发环境](access-notebook.html)。

## 确认开发环境就绪

根据已创建的开发环境执行对应命令：

```bash
# JupyterLab
kubectl rollout status statefulset/jupyter-sample \
  --namespace notebook-demo \
  --timeout=10m

# Code Server
kubectl rollout status statefulset/code-server-sample \
  --namespace notebook-demo \
  --timeout=10m
```

查看 Notebook、StatefulSet、Pod 和 Service：

```bash
kubectl get notebooks,statefulsets,pods,services \
  --namespace notebook-demo
```

就绪后可以看到：

- Notebook Controller 创建了与 Notebook 同名的 StatefulSet。
- StatefulSet 的 `READY` 为 `1/1`，Pod 状态为 `Running`。
- Controller 创建了同名的 ClusterIP Service，Service 端口为 `80`。

如果 Pod 未启动，使用下面的命令查看事件和容器日志：

```bash
kubectl describe pod \
  --namespace notebook-demo \
  --selector notebook-name=jupyter-sample

kubectl logs \
  --namespace notebook-demo \
  --selector notebook-name=jupyter-sample \
  --tail=200
```

将命令中的 `jupyter-sample` 替换为 `code-server-sample` 即可排查 Code Server。

> **安全提示**：为了突出 Notebook CR 和访问路径，示例关闭了 JupyterLab token 和 Code Server 内置认证。请勿将这些示例直接暴露到公网；生产环境必须在入口层配置身份认证和访问控制。
