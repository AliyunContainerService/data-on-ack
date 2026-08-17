# Read Data from OSS with Ray Data

This guide shows how to run a Ray Data pipeline in a RayJob that reads data from Alibaba Cloud OSS, processes it, and writes the result back.

Two ways to access OSS data:

1. **S3-compatible interface (`pyarrow.fs.S3FileSystem`)** — implemented in this guide
2. **OSS storage volume mount** — TODO (to be added)

## Prerequisites

- [ACK managed cluster](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/create-an-ack-managed-cluster-2)
- [KubeRay Operator](https://help.aliyun.com/zh/ack/cloud-native-ai-suite/use-cases/ack-install-kuberay-components) installed in the cluster
- An OSS bucket, and an AccessKey with read/write permission on it (e.g. `AliyunOSSFullAccess`)

## Method 1: Access OSS via the S3-compatible interface

The Ray Job entrypoint runs `python /home/ray/job/ray_data_oss_sample.py`, which implements a read -> process -> write pipeline with the Iris dataset:

1. **Read**: load `iris.csv` baked into the Docker image (`/home/ray/iris.csv`)
2. **Write**: store the dataset as **parquet** on OSS (via Ray Data's S3-compatible interface, `pyarrow.fs.S3FileSystem`)
3. **Read back**: read the parquet from OSS into a new Ray Data Dataset
4. **Aggregate**: filter rows with `sepal_length > 5.0`, then group by `species` and compute the mean of each numeric column

> The parquet read/write goes through OSS's S3-compatible interface (`pyarrow.fs.S3FileSystem`). Accessing OSS requires `force_virtual_addressing=True` to use virtual-hosted-style domain names; the job writes the parquet output only on its first run — if parquet files already exist, it skips the write and reads them directly (write once, read-only afterwards).

### 1. Test data

The classic Iris dataset is included in this directory (`iris.csv`, 150 samples). It is copied into the Docker image by the `Dockerfile` (`COPY iris.csv /home/ray/iris.csv`), and the pipeline reads it from `/home/ray/iris.csv`. The OSS bucket is used for the parquet output.

### 2. Build the image with the sample data

The `Dockerfile` is based on the Ray image (which already bundles `pandas` and `pyarrow` used by the sample) and copies `iris.csv` into the image (the build context is this directory, which contains the file):

Build and push it to your image registry, replacing `<IMAGE_REGISTRY>` with your registry address.

> If the cluster is inside a VPC, you can use the `registry-vpc` endpoint of the same ACR for faster pulls.

```bash
cd ray/1-user-guide/3-ray-data-with-oss/
docker build -f Dockerfile -t <IMAGE_REGISTRY>/ray:2.56.1-py312-with-iris .
docker push <IMAGE_REGISTRY>/ray:2.56.1-py312-with-iris
```

### 3. Create the ConfigMap and the Secret

Store the sample code in a ConfigMap (the data key is the file name `ray_data_oss_sample.py`):

```bash
kubectl create configmap ray-job-code --from-file=ray_data_oss_sample.py
```

Expected output:

```
configmap/ray-job-code created
```

Store the OSS credentials in a Secret (referenced by the RayJob via `secretKeyRef`):

```bash
kubectl create secret generic oss-credential \
  --from-literal=akId=<your-access-key-id> \
  --from-literal=akSecret=<your-access-key-secret>
```

### 4. Submit the RayJob

Edit `ray-data-oss.yaml` before applying:

- `spec.rayClusterSpec.headGroupSpec.template.spec.containers[0].image`: your pushed image
- `env.OSS_ENDPOINT`: your OSS region endpoint (default `https://oss-cn-beijing.aliyuncs.com`)
- `env.OSS_BUCKET`: your bucket name
- `env.OSS_PARQUET_DIR`: parquet output directory inside the bucket (default `data/iris_parquet/`)

Then submit:

```bash
kubectl apply -f ray-data-oss.yaml
```

Expected output:

```
rayjob.ray.io/rayjob-data-oss created
```

Wait for the job to finish:

```bash
kubectl wait --for=condition=complete job/rayjob-data-oss --timeout=300s
kubectl get rayjob rayjob-data-oss
```

Expected output:

```
NAME              JOB STATUS   DEPLOYMENT STATUS   RAY CLUSTER NAME         START TIME             END TIME               AGE
rayjob-data-oss   SUCCEEDED    Complete            rayjob-data-oss-xxxxx     ...                    ...                    ...
```

Check the job output:

```bash
kubectl logs job/rayjob-data-oss
```

Expected output (abbreviated):

```
Loaded 150 rows from /home/ray/iris.csv
Wrote parquet to oss://<bucket>/data/iris_parquet/
Read back 150 rows from oss://<bucket>/data/iris_parquet/
Rows with sepal_length > 5.0: 118
Job 'rayjob-data-oss-xxxxx' succeeded
```

> If the parquet output already exists (e.g. you re-submit the job), the write line above is replaced by `Parquet output already exists at oss://<bucket>/data/iris_parquet/, skip writing`.

The aggregated result printed by the job (mean of each numeric column per species, rows with `sepal_length > 5.0`):

```
     species  sepal_length  sepal_width  petal_length  petal_width
      setosa      5.313636     3.713636      1.509091     0.277273
  versicolor      5.997872     2.804255      4.317021     1.346809
   virginica      6.622449     2.983673      5.573469     2.032653
```

You can verify the parquet written to OSS:

```bash
ossutil ls oss://<your-bucket>/data/iris_parquet/
```

Because `shutdownAfterJobFinishes` is `true`, the RayCluster is cleaned up automatically after the job succeeds.

## Method 2: Access OSS via a storage volume mount

TODO (to be added): how to mount an OSS storage volume in the Ray Job via `volumes` / `volumeMounts`, and a Ray Data sample that reads the mount path.

## Cleanup

```bash
kubectl delete -f ray-data-oss.yaml
kubectl delete configmap ray-job-code
kubectl delete secret oss-credential
```
