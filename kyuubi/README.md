# Kyuubi on ACK

## Build Kyuubi Docker Image (with Alibaba Cloud OSS Hadoop-Compatible SDK)

```
docker build -f ./docker/kyuubi.Dockerfile -t {KYUUBI_IMAGE}
docker push {KYUUBI_IMAGE}
```

## Build Spark Image (with Alibaba Cloud OSS Hadoop-Compatible SDK)

```
docker build -f ./docker/spark.Dockerfile -t {SPARK_IMAGE}
docker push {SPARK_IMAGE}
```


## Configure Kyuubi and Spark 

Create Kyuubi namespace. This is where we deploy Kyuubi related resources.

```bash
kubectl create namespace kyuubi
```

Create Spark namespace. Kyuubi starts spark engines via `spark-submit`, spark driver pods and executor pods will be created in this namespace.

```bash
kbectl create namespace spark
```

Before deploying Kyuubi into the cluster, you can double-check all the configurations stored in [`deploy/conf`](./deploy/conf/) dir. You can 
find the configuration reference for [Spark](https://spark.apache.org/docs/3.5.8/configuration.html) and [Kyuubi](https://kyuubi.readthedocs.io/en/v1.10.2/configuration/settings.html). Some of the mandatory fields you need to specify are:

- `./deploy/conf/oss-cred.env`: Modify it with your valid OSS credentials. Both Kyuubi and Spark use the credentials to access your OSS Bucket. The credentials will be transformed into a Kubernetes Secret and applied to `kyuubi` and `spark` namespace.
- `./deploy/conf/spark-defaults.conf`:
    - `spark.kubernetes.container.image`: replace it with your own {SPARK_IMAGE} url.
    - `spark.kubernetes.file.upload.path`: a upload path needed for spark engine running in cluster deploy mode. See ["Running Spark on Kubernetes - Dependency Management"](https://spark.apache.org/docs/3.5.8/running-on-kubernetes.html#dependency-management) for more information.

```bash
kubectl apply -k ./deploy/
```

The expected result is like:
```
serviceaccount/spark-driver created
role.rbac.authorization.k8s.io/spark-submitter created
rolebinding.rbac.authorization.k8s.io/kyuubi-spark created
rolebinding.rbac.authorization.k8s.io/spark-driver created
configmap/kyuubi-conf created
configmap/spark-conf created
secret/hadoop-oss created
secret/hadoop-oss created
```

## Deploy Kyuubi

Next, we deploy Kyuubi with [Kyuubi Helm Chart](https://github.com/apache/kyuubi/tree/v1.10.2/charts/kyuubi). Firstly, modify `./values.yaml` and replace 
the `{KYUUBI_IMAGE_REPO}` and `{KYUUBI_IMAGE_TAG}` with your own Kyuubi image info.

```
image:
  repository: {KYUUBI_IMAGE_REPO}  # replace it with your Kyuubi image info
  tag: {KYUUBI_IMAGE_TAG}          # replace it with your Kyuubi image info
  pullPolicy: Always
...
```

Install kyuubi. Make sure you've successfully installed [Helm](https://helm.sh/) before this step.

```bash
mkdir {YOUR_PATH_TO_KYUUBI} && cd {YOUR_PATH_TO_KYUUBI}
git clone -b v1.10.2 https://github.com/apache/kyuubi.git

helm upgrade kyuubi {YOU_PATH_TO_KYUUBI}/kyuubi/charts/kyuubi \
    --install \
    --namespace kyuubi \
    --create-namespace \
    --values ./values.yaml
```

If everything goes well, you will see kyuubi pods start and run in `kyuubi` namespace:
```
kubectl get pod -n kyuubi
```

Exepected results are like:
```
NAME       READY   STATUS    RESTARTS   AGE
kyuubi-0   1/1     Running   0          101s
kyuubi-1   1/1     Running   0          40s
```

## Connect to Kyuubi

Kyuubi exposes JDBC/ODBC interface and allows users to connect it via a compatible client (e.g. a thrift client). For a full list about "How to integrate X with Kyuubi", please see [Kyuubi documentation](https://kyuubi.readthedocs.io/en/v1.10.2/client/index.html) for more details.

In the following steps, we show how to connect to Kyuubi with the [Kyuubi Beeline CLI](https://kyuubi.readthedocs.io/en/v1.10.2/client/cli/kyuubi_beeline.html). This is an easy way to check if Kyuubi is working well in your Kubernetes cluster.

```
kubectl exec -it kyuubi-0 -n kyuubi -- bash -c '${KYUUBI_HOME}/bin/kyuubi-beeline -u jdbc:kyuubi://localhost:10009'
```
The command logins you into `kyuubi-0` pod and try to connect to Kyuubi JDBC server with the `kyuubi-beeline` CLI. For the first time you run this command, you may wait for a while for the Spark engine pods to start up (The Spark engine pods run in `spark` namespace in our configuration). 

After the Spark engine is ready, you'll see an interacive SQL query executor.

An example SQL query is showed here:
```sql
CREATE DATABASE IF NOT EXISTS demo;
USE demo;

CREATE TABLE IF NOT EXISTS demo_table (
    id INT,
    name STRING,
    age INT
) USING parquet 
location 'oss://{OSS_BUCKET}/{PATH_TO_DATABASE_DATA}';

INSERT INTO demo_table VALUES (1, 'Alice', 25);

SELECT * FROM demo_table;
```

> [!NOTE]
> By default, Kyuubi launches Spark SQL engines using a dummy embedded Apache Derby-based metastore. This means the database schema will NOT be persisted. In a real production environment, consider configure [Kyuubi with hive metastore](https://kyuubi.readthedocs.io/en/v1.10.2/deployment/hive_metastore.html) as a persistent metastore choice.

## Clean up environments

1. Check if any Spark engine pods are still running in `spark` namespace

2. Uninstall Kyuubi
    ```
    helm del -n kyuubi kyuubi
    ```

3. Delete Kyuubi and Spark configurations

    ```
    kubectl delete -k ./deploy
    ```

