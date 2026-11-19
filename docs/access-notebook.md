---
title: 访问 Notebook 开发环境
layout: default
parent: Notebook开发环境
nav_order: 3
description: 通过 kubectl port-forward 或 ALB Ingress 访问 JupyterLab 和 Code Server。
---

# 访问 Notebook 开发环境

Notebook Controller 为每个 Notebook 创建一个同名的 ClusterIP Service。本节介绍两种访问方式：本地临时访问使用 `kubectl port-forward`；需要共享入口时使用 ALB Ingress。

> **提示**：Notebook Controller 不会创建 Ingress，也不提供访问入口的用户认证和 namespace 授权。需要由控制台统一代理 Notebook 流量并进行身份与权限校验时，请参考[开发控制台功能使用指南中的 Notebook 访问](development-console-user-guide.html#访问-notebook)和[部署指南中的认证模式](development-console-deploy-guide.html#认证模式)。

以下命令假设已经按照[创建 Notebook 开发环境](create-notebook.html)在 `notebook-demo` namespace 中创建了 `jupyter-sample` 和 `code-server-sample`。

## 方式一：kubectl port-forward

`kubectl port-forward` 不会创建公网入口，适合个人调试和确认开发环境是否正常。

### 访问 JupyterLab

在一个终端中执行：

```bash
kubectl port-forward \
  --namespace notebook-demo \
  service/jupyter-sample \
  8888:80
```

保持命令运行，然后在浏览器中访问：

```text
http://127.0.0.1:8888/notebook/notebook-demo/jupyter-sample/
```

JupyterLab 配置了 `/notebook/notebook-demo/jupyter-sample` base path，因此本地访问时也需要保留该路径前缀。

### 访问 Code Server

在另一个终端中执行：

```bash
kubectl port-forward \
  --namespace notebook-demo \
  service/code-server-sample \
  8080:80
```

然后访问：

```text
http://127.0.0.1:8080/
```

`port-forward` 直接连接 Code Server Service，没有入口代理，因此使用根路径 `/`，不需要执行 rewrite。

## 方式二：ALB Ingress

ALB Ingress 适合为多个用户提供统一的 HTTP/HTTPS 入口。开始前确认：

- ACK 集群已经安装 ALB Ingress Controller。
- 集群中存在可用的 `alb` IngressClass 和对应 AlbConfig。
- Notebook Pod 使用 Terway 等 ALB 可以直接访问 Pod IP 的网络模式。
- 已经根据环境配置访问认证、HTTPS 和网络访问控制。

```bash
kubectl get ingressclass alb
kubectl get albconfig --all-namespaces
```

Notebook Controller 创建的是 ClusterIP Service。为 ALB 使用 IP 类型服务器组，给两个 Service 添加注解：

```bash
kubectl annotate service jupyter-sample \
  --namespace notebook-demo \
  alb.ingress.kubernetes.io/server-group-type=Ip \
  --overwrite

kubectl annotate service code-server-sample \
  --namespace notebook-demo \
  alb.ingress.kubernetes.io/server-group-type=Ip \
  --overwrite
```

JupyterLab 能直接处理完整的 Notebook base path，而 Code Server 需要入口代理删除路径前缀。由于 `alb.ingress.kubernetes.io/rewrite-target` 对整个 Ingress 生效，两类 Notebook 应分别使用不同的 Ingress。

### 创建 JupyterLab Ingress

将以下内容保存为 `jupyter-ingress.yaml`。该 Ingress 不执行 rewrite，完整请求路径会转发给 JupyterLab：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: notebook-jupyter
  namespace: notebook-demo
spec:
  ingressClassName: alb
  rules:
    - http:
        paths:
          - path: /notebook/notebook-demo/jupyter-sample
            pathType: Prefix
            backend:
              service:
                name: jupyter-sample
                port:
                  number: 80
```

```bash
kubectl apply -f jupyter-ingress.yaml
```

### 创建 Code Server Ingress

将以下内容保存为 `code-server-ingress.yaml`。请求中的 `/notebook/notebook-demo/code-server-sample` 前缀会在转发前被删除：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: notebook-code-server
  namespace: notebook-demo
  annotations:
    alb.ingress.kubernetes.io/use-regex: "true"
    alb.ingress.kubernetes.io/rewrite-target: /${2}
spec:
  ingressClassName: alb
  rules:
    - http:
        paths:
          - path: /notebook/notebook-demo/code-server-sample(/|$)(.*)
            pathType: Prefix
            backend:
              service:
                name: code-server-sample
                port:
                  number: 80
```

```bash
kubectl apply -f code-server-ingress.yaml
```

ALB rewrite 的正则路径和 `${2}` 捕获组语法请参见[阿里云 ALB Ingress 高级配置文档](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/advanced-alb-ingress-configurations)。

### 确认 Ingress 就绪

等待两个 Ingress 的 `ADDRESS` 字段出现 ALB 地址：

```bash
kubectl get ingress \
  --namespace notebook-demo \
  --watch
```

也可以检查 Ingress 事件和后端 Endpoint：

```bash
kubectl describe ingress notebook-jupyter \
  --namespace notebook-demo

kubectl describe ingress notebook-code-server \
  --namespace notebook-demo

kubectl get endpoints jupyter-sample code-server-sample \
  --namespace notebook-demo
```

获取 ALB 地址：

```bash
ALB_ADDRESS=$(kubectl get ingress notebook-jupyter \
  --namespace notebook-demo \
  --output=jsonpath='{.status.loadBalancer.ingress[0].hostname}')

echo "http://${ALB_ADDRESS}/notebook/notebook-demo/jupyter-sample/"
echo "http://${ALB_ADDRESS}/notebook/notebook-demo/code-server-sample/"
```

最终两类开发环境使用统一的外部路径格式：

```text
/notebook/<namespace>/<name>/
```

区别仅在于入口层是否执行 rewrite：JupyterLab 保留完整路径，Code Server 删除 Notebook 前缀后再转发到根路径。

> **安全提示**：示例 Notebook 关闭了应用内认证。不要直接使用公网 ALB 暴露这些实例。生产环境应使用内网 ALB，或在入口层配置身份认证、HTTPS、访问控制和租户授权后再开放访问。
