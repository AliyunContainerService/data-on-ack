# 部署并使用基础 Ray Service

本文介绍如何在 ACK 集群上通过 KubeRay 的 `RayService` 自定义资源部署一个基础的 Ray Serve 应用，以及如何访问和更新它。

`RayService` 会为你管理两件事：

- 承载 Serve 应用的 `RayCluster`。
- Serve 应用本身，通过 `serveConfigV2` 声明式描述。KubeRay 负责把应用部署到集群上、监控其健康状态，并在配置变更时执行零中断升级。

## 前提条件

- 已创建 [ACK 托管版集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- 集群中已安装 [KubeRay Operator 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)

## 示例说明

示例部署一个最简单的 HTTP 应用。Serve 应用代码放在 ConfigMap（`ray-serve-app`）中，同时挂载到 head 和 worker Pod，并通过 `PYTHONPATH` 指向挂载路径，使 Serve 能够导入它。这样示例完全自包含：既不需要构建自定义镜像，也不需要下载外部 `working_dir`。

```python
from ray import serve
from starlette.requests import Request


@serve.deployment
class Greeter:
    async def __call__(self, request: Request) -> str:
        name = request.query_params.get("name", "Ray")
        return f"Hello {name} from Ray Serve!"


app = Greeter.bind()
```

`serveConfigV2` 告诉 KubeRay 把这个模块作为一个应用部署，并使用 2 个副本：

```yaml
serveConfigV2: |
  applications:
    - name: greeter
      import_path: greeter:app
      route_prefix: /
      deployments:
        - name: Greeter
          num_replicas: 2
          ray_actor_options:
            num_cpus: 0.5
```

> 生产环境推荐把应用代码打进自定义容器镜像，或使用 `runtime_env.working_dir` 指向 OSS 上的代码包，而不是使用 ConfigMap。

## 1. 部署 RayService

```bash
kubectl apply -f ray-service.yaml
```

预期输出：

```
configmap/ray-serve-app created
rayservice.ray.io/demo-ray-service created
```

等待 RayService 就绪（这一步会创建 RayCluster、启动 Serve 并部署应用，通常需要几分钟）：

```bash
kubectl get rayservice demo-ray-service -w
```

预期输出（等到 `SERVICE STATUS` 变为 `Running`）：

```
NAME               SERVICE STATUS   NUM SERVE ENDPOINTS
demo-ray-service   WaitForServeDeploymentReady
demo-ray-service   Running          2
```

查看 KubeRay 创建的 Pod 和 Service：

```bash
kubectl get pods -l ray.io/serve=true
kubectl get svc | grep demo-ray-service
```

预期输出（`-rq2wc` 这类后缀由 KubeRay 生成，每次都不同）：

```
NAME                                                 READY   STATUS    RESTARTS   AGE
demo-ray-service-rq2wc-head-jvhf2                    1/1     Running   0          11m
demo-ray-service-rq2wc-worker-group-worker-2kv2v     1/1     Running   0          11m

demo-ray-service-head-svc         ClusterIP   None             <none>   10001/TCP,8265/TCP,6379/TCP,8080/TCP,8000/TCP   2m
demo-ray-service-rq2wc-head-svc   ClusterIP   None             <none>   10001/TCP,8265/TCP,6379/TCP,8080/TCP,8000/TCP   11m
demo-ray-service-serve-svc        ClusterIP   192.168.46.223   <none>   8000/TCP                                        2m
```

注意 KubeRay 会额外创建一个 **serve service**（`demo-ray-service-serve-svc`），它只把流量转发到 Serve 代理健康的 Pod。访问应用时请始终使用这个 Service，而不是 head Service。

## 2. 查看 Serve 应用状态

Serve 应用的状态会反映在 RayService 资源上：

```bash
kubectl describe rayservice demo-ray-service
```

预期输出（已省略部分内容，应用和 deployment 分别为 `RUNNING`/`HEALTHY`）：

```
Status:
  Active Service Status:
    Application Statuses:
      greeter:
        Serve Deployment Statuses:
          Greeter:
            Status:  HEALTHY
        Status:      RUNNING
```

也可以进入 head Pod 用 Serve CLI 查看：

```bash
HEAD_POD=$(kubectl get pod -l ray.io/node-type=head,ray.io/serve=true -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $HEAD_POD -- serve status
```

预期输出：

```
proxies:
  ...: HEALTHY
applications:
  greeter:
    status: RUNNING
    deployments:
      Greeter:
        status: HEALTHY
        replica_states:
          RUNNING: 2
```

## 3. 访问 Ray Service

在集群内通过 serve service 访问应用：

```bash
kubectl run curl-test --rm -it --restart=Never --image=curlimages/curl -- \
  curl -s "http://demo-ray-service-serve-svc.default.svc.cluster.local:8000/?name=ACK"
```

预期输出：

```
Hello ACK from Ray Serve!
```

或者转发到本地访问：

```bash
kubectl port-forward svc/demo-ray-service-serve-svc 8000:8000
curl -s "http://127.0.0.1:8000/?name=ACK"
```

如需在集群外暴露服务，可以把 Service 改为 LoadBalancer 类型或创建 Ingress，参见[在 ACK 中暴露应用](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/expose-applications)。

## 4. 更新 Serve 应用（原地更新）

修改 `serveConfigV2` 会触发应用的原地更新，不会重建 RayCluster。例如把副本数从 2 改为 3：

```bash
kubectl patch rayservice demo-ray-service --type merge -p '
spec:
  serveConfigV2: |
    applications:
      - name: greeter
        import_path: greeter:app
        route_prefix: /
        deployments:
          - name: Greeter
            num_replicas: 3
            ray_actor_options:
              num_cpus: 0.5
'
```

确认新的副本数：

```bash
kubectl exec -it $HEAD_POD -- serve status | grep -A2 replica_states
```

预期输出：

```
        replica_states:
          RUNNING: 3
```

如果修改的是 `rayClusterConfig`（例如镜像），KubeRay 会执行**零中断升级**：先创建新的 RayCluster，等应用在新集群上就绪后切换流量，最后销毁旧集群。

## 清理

```bash
kubectl delete -f ray-service.yaml
```
