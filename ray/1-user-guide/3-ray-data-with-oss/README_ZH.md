# 使用 Ray Data 读取 OSS 数据

本文介绍如何在 RayJob 中运行一个 Ray Data 流水线：从阿里云 OSS 读取数据、处理、并将结果写回 OSS。

访问 OSS 中数据的两种方式：

1. **通过 S3 兼容接口访问（`pyarrow.fs.S3FileSystem`）** — 本文已实现
2. **通过 OSS 存储卷挂载访问** — TODO（待补充）

## 前提条件

- 已创建 [ACK 托管版集群](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- 集群中已安装 [KubeRay Operator 组件](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components)
- 一个 OSS Bucket，以及对该 Bucket 有读写权限的 AccessKey（例如 `AliyunOSSFullAccess` 权限）

## 方法一：通过 S3 兼容接口访问

Ray Job 的入口命令是 `python /home/ray/job/ray_data_oss_sample.py`，使用 Iris 数据集实现一个读取 -> 处理 -> 写入 Pipeline：

1. **读取**：从镜像内置的 `iris.csv` 加载（`/home/ray/iris.csv`）
2. **写入**：将数据集以 **parquet** 格式写入 OSS（通过 Ray Data 的 S3 兼容接口 `pyarrow.fs.S3FileSystem`）
3. **读回**：从 OSS 读取该 parquet 文件，构建新的 Ray Data Dataset
4. **聚合**：过滤 `sepal_length > 5.0` 的行，然后按 `species` 分组，计算各数值列的平均值

> parquet 读写通过 OSS 的 S3 兼容接口（`pyarrow.fs.S3FileSystem`）。访问 OSS 需要设置 `force_virtual_addressing=True` 以使用 virtual hosted style 的域名格式；任务只在首次运行时写入 parquet，之后若检测到 parquet 输出已存在，则跳过写入、直接读取（只写一次，后续只读）。

### 1. 测试数据

经典的 Iris 数据集已包含在本目录中（`iris.csv`，150 条样本）。它通过 `Dockerfile` 中的 `COPY iris.csv /home/ray/iris.csv` 打进镜像，Pipeline 从 `/home/ray/iris.csv` 读取。OSS Bucket 用于存放 parquet 输出。

### 2. 构建包含示例数据的镜像

`Dockerfile` 基于 Ray 镜像（已自带示例所需的 `pandas` 和 `pyarrow`），并把 `iris.csv` 拷贝进镜像（构建上下文为本目录，包含该文件）：

构建并推送到你的镜像仓库，将<IMAGE_REGISTRY>替换为你的镜像仓库地址。

> 集群在 VPC 内时可使用同一 ACR 的 `registry-vpc` 地址加速拉取。

```bash
cd ray/1-user-guide/3-ray-data-with-oss/
docker build -f Dockerfile -t <IMAGE_REGISTRY>/ray:2.56.1-py312-with-iris .
docker push <IMAGE_REGISTRY>/ray:2.56.1-py312-with-iris
```

### 3. 创建 ConfigMap 和 Secret

将示例代码存入 ConfigMap（data key 为文件名 `ray_data_oss_sample.py`）：

```bash
kubectl create configmap ray-job-code --from-file=ray_data_oss_sample.py
```

预期输出：

```
configmap/ray-job-code created
```

将 OSS 凭证存入 Secret（RayJob 通过 `secretKeyRef` 引用）：

```bash
kubectl create secret generic oss-credential \
  --from-literal=akId=<your-access-key-id> \
  --from-literal=akSecret=<your-access-key-secret>
```

### 4. 提交 RayJob

应用 `ray-data-oss.yaml` 前，先修改：

- `spec.rayClusterSpec.headGroupSpec.template.spec.containers[0].image`：你推送的镜像地址
- `env.OSS_ENDPOINT`：OSS 所在地域的 Endpoint（默认为 `https://oss-cn-beijing.aliyuncs.com`）
- `env.OSS_BUCKET`：你的 Bucket 名称
- `env.OSS_PARQUET_DIR`：Bucket 中 parquet 输出的目录（默认为 `data/iris_parquet/`）

然后提交：

```bash
kubectl apply -f ray-data-oss.yaml
```

预期输出：

```
rayjob.ray.io/rayjob-data-oss created
```

等待任务完成：

```bash
kubectl wait --for=condition=complete job/rayjob-data-oss --timeout=300s
kubectl get rayjob rayjob-data-oss
```

预期输出：

```
NAME              JOB STATUS   DEPLOYMENT STATUS   RAY CLUSTER NAME         START TIME             END TIME               AGE
rayjob-data-oss   SUCCEEDED    Complete            rayjob-data-oss-xxxxx     ...                    ...                    ...
```

查看任务输出：

```bash
kubectl logs job/rayjob-data-oss
```

预期输出（节选）：

```
Loaded 150 rows from /home/ray/iris.csv
Wrote parquet to oss://<bucket>/data/iris_parquet/
Read back 150 rows from oss://<bucket>/data/iris_parquet/
Rows with sepal_length > 5.0: 118
Job 'rayjob-data-oss-xxxxx' succeeded
```

> 如果 parquet 输出已存在（例如重复提交任务），上面日志中的写入行会变为 `Parquet output already exists at oss://<bucket>/data/iris_parquet/, skip writing`。

任务打印的聚合结果（过滤 `sepal_length > 5.0` 后，按 species 分组各数值列的平均值）：

```
     species  sepal_length  sepal_width  petal_length  petal_width
      setosa      5.313636     3.713636      1.509091     0.277273
  versicolor      5.997872     2.804255      4.317021     1.346809
   virginica      6.622449     2.983673      5.573469     2.032653
```

可以检查写入 OSS 的 parquet 文件：

```bash
ossutil ls oss://<your-bucket>/data/iris_parquet/
```

由于 `shutdownAfterJobFinishes` 为 `true`，任务成功后 RayCluster 会自动清理。

## 方法二：通过 OSS 存储卷挂载访问

TODO（待补充）：如何在 Ray Job 中通过 `volumes` / `volumeMounts` 挂载 OSS 存储卷，以及访问挂载路径的 Ray Data 示例代码。

## 清理

```bash
kubectl delete -f ray-data-oss.yaml
kubectl delete configmap ray-job-code
kubectl delete secret oss-credential
```
