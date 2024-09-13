package com.aliyun.kubeai.cluster;

import com.alibaba.fastjson.JSON;
import com.alibaba.fastjson.JSONArray;
import com.alibaba.fastjson.JSONObject;
import com.aliyun.kubeai.entity.ElasticQuotaGroup;
import com.aliyun.kubeai.entity.FluidDataset;
import com.aliyun.kubeai.entity.K8sPvc;
import com.aliyun.kubeai.model.k8s.K8sFluidDataset;
import com.aliyun.kubeai.model.k8s.K8sFluidDatasetList;
import com.aliyun.kubeai.model.k8s.dataset.EncryptOption;
import com.aliyun.kubeai.model.k8s.dataset.MatchExpression;
import com.aliyun.kubeai.model.k8s.dataset.Mount;
import com.google.common.base.Strings;
import io.fabric8.kubernetes.api.model.*;
import io.fabric8.kubernetes.api.model.apiextensions.v1beta1.CustomResourceDefinition;
import io.fabric8.kubernetes.api.model.rbac.*;
import io.fabric8.kubernetes.client.*;
import io.fabric8.kubernetes.client.Config;
import io.fabric8.kubernetes.client.ConfigBuilder;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.fabric8.kubernetes.client.dsl.base.CustomResourceDefinitionContext;
import io.fabric8.kubernetes.client.utils.Serialization;
import io.kubernetes.client.util.Yaml;
import lombok.extern.slf4j.Slf4j;
import org.apache.commons.lang3.tuple.Triple;
import org.junit.Test;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.*;

import static java.util.stream.Collectors.toList;

/**
 * https://help.aliyun.com/document_detail/134903.html
 */
@Slf4j
public class KubernetesClientTest {

    public KubernetesClient getClient() {
        Config config = new ConfigBuilder().build();
        return new DefaultKubernetesClient(config);
    }

    @Test
    public void testList() {
        KubernetesClient client = getClient();

        PodList podList = client.pods().inNamespace("kube-system").list();
        podList.getItems().forEach((obj) -> {
            System.out.printf("meta:%s\n status:%s\n", obj.getMetadata(), obj.getStatus());
        });
    }

    @Test
    public void testGetSeldon() {
        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext.Builder()
                .withGroup("machinelearning.seldon.io")
                .withKind("SeldonDeployment")
                .withVersion("v1")
                .withScope("Namespaced")
                .withPlural("seldondeployments")
                .build();

        Map<String, Object> result = null;
        try {
            result = getClient().customResource(ctx).get("default", "fashion-mnist");
        } catch (Exception e) {
            log.warn("no SeldonDeployment found, {}", e.getMessage());
        }

        System.out.println(JSON.toJSONString(result));
    }

    @Test
    public void testGetClusterId() {
        KubernetesClient client = getClient();

        final String namespace = "kube-system";
        final String name = "ack-cluster-profile";
        Resource<ConfigMap> configMapResource = client.configMaps().inNamespace(namespace).withName(name);

        ConfigMap configMap = configMapResource.get();
        Map<String, String> data = configMap.getData();
        log.info("========= clusterId: " + data.get("clusterid"));
    }

    @Test
    public void bindingClusterRole() {
        String namespace = "dev1";
        String userName = "bob";
        String clusterRoleName = "arena-topnode";
        KubernetesClient client = getClient();
        RoleRef roleRef = new RoleRefBuilder().withApiGroup("rbac.authorization.k8s.io")
                .withKind("ClusterRole")
                .withName(clusterRoleName).build();
        ObjectMeta metaData = new ObjectMetaBuilder().withName(userName + "-" + clusterRoleName).withNamespace(namespace).build();
        Subject subject = new SubjectBuilder().withKind("ServiceAccount").withName(userName).withNamespace(namespace).build();
        ClusterRoleBinding clusterRoleBinding = new ClusterRoleBindingBuilder()
                .withKind("ClusterRoleBinding")
                .withApiVersion("rbac.authorization.k8s.io/v1")
                .withMetadata(metaData)
                .withSubjects(subject)
                .withRoleRef(roleRef).build();
        ClusterRoleBinding bindRes = client.rbac().clusterRoleBindings().inNamespace(namespace).createOrReplace(clusterRoleBinding);
        log.info("bindRes:{}", JSON.toJSONString(bindRes));
    }

    @Test
    public void bindingRole() {
        String namespace = "dev1";
        String userName = "bob";
        String roleName = "arena";
        KubernetesClient client = getClient();
        RoleRef roleRef = new RoleRefBuilder().withApiGroup("rbac.authorization.k8s.io")
                .withKind("Role")
                .withName(roleName).build();
        ObjectMeta metaData = new ObjectMetaBuilder().withName(userName + "-" + roleName).withNamespace(namespace).build();
        Subject subject = new SubjectBuilder().withKind("ServiceAccount").withName(userName).withNamespace(namespace).build();
        RoleBinding roleBinding = new RoleBindingBuilder()
                .withKind("RoleBinding")
                .withApiVersion("rbac.authorization.k8s.io/v1")
                .withMetadata(metaData)
                .withSubjects(subject)
                .withRoleRef(roleRef).build();
        RoleBinding bindRes = client.rbac().roleBindings().inNamespace(namespace).createOrReplace(roleBinding);
        log.info("bindRes:{}", JSON.toJSONString(bindRes));
    }

    @Test
    public void createServiceAccount() {
        String namespace = "default";
        String userName = "bob";
        KubernetesClient client = getClient();
        ServiceAccount createdServiceAccount = null;

        try {
            ServiceAccount serviceAccount = new ServiceAccountBuilder()
                    .withNewMetadata().withName(userName).endMetadata()
                    .withAutomountServiceAccountToken(false)
                    .build();
            createdServiceAccount = client.serviceAccounts().inNamespace(namespace).create(serviceAccount);
            //client.serviceAccounts().inNamespace(namespace).delete(serviceAccount);
        } catch (Exception e) {
            createdServiceAccount = client.serviceAccounts().inNamespace(namespace).withName(userName).get();
        }
        log.info("created service account:{}", createdServiceAccount);
    }

    @Test
    public void createKubernetesConfigForUser() {
        String namespace = "dev1";
        String userName = "bob";
        KubernetesClient client = getClient();

        ServiceAccount serviceAccount = client.serviceAccounts().inNamespace(namespace).withName(userName).get();
        String secretName = serviceAccount.getSecrets().get(0).getName();
        Secret secret = client.secrets().inNamespace(namespace).withName(secretName).get();
        String secretCaCrtBase64 = secret.getData().get("ca.crt");
        //String secretCaCrt = new String(Base64.getDecoder().decode(secretCaCrtBase64.getBytes()), StandardCharsets.UTF_8);
        String secretNamespaceBase64 = secret.getData().get("namespace");
        String secretNamespace = new String(Base64.getDecoder().decode(secretNamespaceBase64.getBytes()), StandardCharsets.UTF_8);
        String secretTokenBase64 = secret.getData().get("token");
        String secretToken = new String(Base64.getDecoder().decode(secretTokenBase64.getBytes()), StandardCharsets.UTF_8);
        Config config = client.getConfiguration();
        String curContextName = config.getCurrentContext().getName();
        List<NamedContext> namedContexts = config.getContexts();
        NamedContext curContext = null;
        for (NamedContext c : namedContexts) {
            if (c.getName().equals(curContextName)) {
                curContext = c;
                break;
            }
        }

        String clusterName = curContext.getContext().getCluster();
        String serverAddrs = config.getMasterUrl();

        Context context = new ContextBuilder().withCluster(clusterName).withNamespace(secretNamespace).withUser(userName).build();
        NamedContext namedContext = new NamedContextBuilder().withContext(context).withName(userName).build();

        Cluster cluster = new ClusterBuilder().withCertificateAuthorityData(secretCaCrtBase64).withServer(serverAddrs).build();
        NamedCluster namedCluster = new NamedClusterBuilder().withCluster(cluster).withName(clusterName).build();

        AuthInfo userAuth = new AuthInfoBuilder().withNewToken(secretToken).build();
        NamedAuthInfo authInfo = new NamedAuthInfoBuilder().withName(userName).withUser(userAuth).build();

        io.fabric8.kubernetes.api.model.Config modelConfig = new io.fabric8.kubernetes.api.model.ConfigBuilder()
                .withApiVersion(config.getApiVersion())
                .withKind("Config")
                .withCurrentContext(userName)
                .withClusters(namedCluster)
                .withContexts(namedContext)
                .withUsers(authInfo)
                .build();
        log.info("config for user:{}", JSON.toJSONString(modelConfig.toString()));
        ByteArrayOutputStream stream = new ByteArrayOutputStream();
        String res = "";
        try {
            Serialization.yamlMapper().writeValue(stream, modelConfig);
            res = new String(stream.toByteArray());
            log.info("write value:{}", res);
        } catch (Exception e) {
            log.warn("write value exception:{}", e.getMessage());
        }
        return;
    }

    @Test
    public void listNamespaces() {
        KubernetesClient client = getClient();
        NamespaceList namespaceList = client.namespaces().list();
        List<String> res = namespaceList.getItems().stream().map(x -> x.getMetadata().getName()).collect(toList());
        res.forEach(x -> System.out.println(x));
    }

    @Test
    public void listDataset() {
        KubernetesClient client = getClient();
        CustomResourceDefinitionContext ctx = buildCRDContextByName("datasets.data.fluid.io");
        Map<String, Object> namespaceList = client.customResource(ctx).list();
        namespaceList.forEach((x, y) -> {
            System.out.println(x);
            System.out.println(y);
        });
        //res.forEach(x->System.out.println(x));
    }

    private CustomResourceDefinitionContext buildCRDContextByName(String crdName) {
        KubernetesClient client = getClient();
        CustomResourceDefinition crd = client.customResourceDefinitions().withName(crdName).get();
        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext().fromCrd(crd);
        return ctx;
    }

    @Test
    public void listElasticQuota() {
        KubernetesClient client = getClient();
        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext.Builder()
                .withGroup("scheduling.sigs.k8s.io")
                .withKind("ElasticQuota")
                .withVersion("v1alpha1")
                .withScope("Namespaced")
                .withPlural("elasticquotas")
                .build();

        String namespace = "default";
        String groupName = "jackwg-test-group0";

        Map<String, Object> namespaceList = client.customResource(ctx).list();
        //Map<String, Object> namespaceList = client.customResource(ctx).get(namespace, groupName);
        //Map<String, Object> namespaceList = client.customResource(ctx).get(groupName);
        namespaceList.forEach((x, y) -> {
            System.out.println("x:" + x);
            System.out.println("y:" + y);
        });
        //res.forEach(x->System.out.println(x));
    }

    @Test
    public void patchElasticQuota() {
        KubernetesClient client = getClient();
        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext.Builder()
                .withGroup("scheduling.sigs.k8s.io")
                .withKind("ElasticQuota")
                .withVersion("v1alpha1")
                .withScope("Namespaced")
                .withPlural("elasticquotas")
                .build();

        List<Triple<String, Integer, Integer>> quotaConfigs = new ArrayList<>();
        //aliyun.com/gpu-mem,aliyun.com/gpu,nvidia.com/gpu
        quotaConfigs.add(Triple.of("gpu", 2, 3));

        String groupName = "jackwg-test-group";
        String namespace = "default";
        JSONObject metadataNode = new JSONObject();
        metadataNode.put("name", groupName);
        metadataNode.put("namespace", namespace);

        JSONObject minNode = new JSONObject();
        JSONObject maxNode = new JSONObject();
        for (Triple<String, Integer, Integer> qc : quotaConfigs) {
            String resourceName = qc.getLeft();
            assert resourceName != null;
            if (qc.getMiddle() != null) {
                minNode.put(resourceName, qc.getMiddle());
            }
            if (qc.getRight() != null) {
                maxNode.put(resourceName, qc.getRight());
            }
        }
        JSONObject specNode = new JSONObject();
        specNode.put("min", minNode);
        specNode.put("max", maxNode);

        JSONObject root = new JSONObject();
        root.put("apiVersion", "scheduling.sigs.k8s.io/v1alpha1");
        root.put("kind", "ElasticQuota");
        root.put("metadata", metadataNode);
        root.put("spec", specNode);

        log.info("patch req:{}", root.toJSONString());
        try {
            //CustomResourceDefinition patchRes = client.customResourceDefinitions().withName(groupName).patch(crd);
            Map<String, Object> patchRes = client.customResource(ctx).createOrReplace(namespace, root.toJSONString());
            log.info("patch quota res len:{}", patchRes.size());
            patchRes.entrySet().stream().forEach(x -> log.info("patch elastic quota res {}:{}", x.getKey(), x.getValue()));
        } catch (Exception e) {
            log.info("exception:{}", e);
        }
    }

    @Test
    public void testGetPVC() {
        String name = "";
        String namespace = "default";
        KubernetesClient client = getClient();
        PersistentVolumeClaimList pvcList = null;
        PersistentVolumeClaim resPvc = null;
        if (!Strings.isNullOrEmpty(name) && !Strings.isNullOrEmpty(namespace)) {
            resPvc = client.persistentVolumeClaims().inNamespace(namespace).withName(name).get();
        } else if (!Strings.isNullOrEmpty(namespace)) {
            pvcList = client.persistentVolumeClaims().inNamespace(namespace).list();
        } else {
            pvcList = client.persistentVolumeClaims().list();
        }
        //PersistentVolumeClaimList pvcList = client.persistentVolumeClaims().inNamespace("default").list();
        List<K8sPvc> res = new ArrayList<>();
        for (PersistentVolumeClaim pvcItem : pvcList.getItems()) {
            K8sPvc pvc = new K8sPvc();
            pvc.setName(pvcItem.getMetadata().getName());
            pvc.setNamespace(pvcItem.getMetadata().getNamespace());
            res.add(pvc);
        }
        log.info("pvc parsed:{}", res);
    }

    @Test
    public void createElasticQuota() {
        ElasticQuotaGroup group = new ElasticQuotaGroup();
        group.setName("jackwg-test-group0");

        List<String> subGroupNames = new ArrayList<>(Arrays.asList("default"));
        group.setSubGroupNames(subGroupNames);

        List<ElasticQuotaGroup.ResourceQuota> elasticResourceQuotas = new ArrayList<>();
        List<Triple<String, Integer, Integer>> quotaConfigs = new ArrayList<>();
        //aliyun.com/gpu-mem,aliyun.com/gpu,nvidia.com/gpu
        quotaConfigs.add(Triple.of("cpu", 1, 2));
        quotaConfigs.add(Triple.of("gpu", 1, 2));
        quotaConfigs.add(Triple.of("gpuMem", 1, 2));
        quotaConfigs.add(Triple.of("gpuTopology", 1, 2));
        quotaConfigs.add(Triple.of("npu", 1, 2));
        for (Triple<String, Integer, Integer> t : quotaConfigs) {
            ElasticQuotaGroup.ResourceQuota rq = new ElasticQuotaGroup.ResourceQuota();
            rq.setMin(String.valueOf(t.getMiddle()));
            rq.setMax(String.valueOf(t.getRight()));
            rq.setResourceName(t.getLeft());
            elasticResourceQuotas.add(rq);
        }
        group.setQuotaList(elasticResourceQuotas);

        KubernetesClient client = getClient();

        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext.Builder()
                .withGroup("scheduling.sigs.k8s.io")
                .withKind("ElasticQuota")
                .withVersion("v1alpha1")
                .withScope("Namespaced")
                .withPlural("elasticquotas")
                .build();

        String name = group.getName();
        String namespace = group.getSubGroupNames().get(0); // only one namespace for one ElasticQuota now

        JSONObject metadataNode = new JSONObject();
        metadataNode.put("name", name);
        metadataNode.put("namespace", namespace);

        JSONObject minNode = new JSONObject();
        JSONObject maxNode = new JSONObject();
        for (ElasticQuotaGroup.ResourceQuota rq : group.getQuotaList()) {
            String resourceName = rq.getResourceName();
            minNode.put(resourceName, rq.getMin());
            maxNode.put(resourceName, rq.getMin());
        }
        JSONObject specNode = new JSONObject();
        specNode.put("min", minNode);
        specNode.put("max", maxNode);

        JSONObject root = new JSONObject();
        root.put("apiVersion", "scheduling.sigs.k8s.io/v1alpha1");
        root.put("kind", "ElasticQuota");
        root.put("metadata", metadataNode);
        root.put("spec", specNode);

        String jsonBody = root.toJSONString();
        log.info("elastic quota: {}", jsonBody);

        try {
            Map<String, Object> res = client.customResource(ctx).create(namespace, jsonBody);
            log.info("create quota res len:{}", res.size());
            res.entrySet().stream().forEach(x -> log.info("create elastic quota res {}:{}", x.getKey(), x.getValue()));
        } catch (IOException e) {
            log.error("create elastic quota failed", e);
        }
    }

    @Test
    public void testCreateDataset() {
        FluidDataset fluidDataset = new FluidDataset();
        String datasetConf = "apiVersion: data.fluid.io/v1alpha1\n" +
                "kind: Dataset\n" +
                "metadata:\n" +
                "  name: kubeflow-oss-mnist\n" +
                "spec:\n" +
                "  mounts:\n" +
                "  - mountPoint: oss://<OSS_BUCKET>/<OSS_DIRECTORY>/\n" +
                "    name: mydata\n" +
                "    options:\n" +
                "      fs.oss.endpoint: <OSS_ENDPOINT>\n" +
                "    encryptOptions:\n" +
                "      - name: fs.oss.accessKeyId\n" +
                "        valueFrom:\n" +
                "          secretKeyRef:\n" +
                "            name: mysecret\n" +
                "            key: fs.oss.accessKeyId\n" +
                "      - name: fs.oss.accessKeySecret\n" +
                "        valueFrom:\n" +
                "          secretKeyRef:\n" +
                "            name: mysecret\n" +
                "            key: fs.oss.accessKeySecret\n" +
                "  nodeAffinity:\n" +
                "    required:\n" +
                "      nodeSelectorTerms:\n" +
                "        - matchExpressions:\n" +
                "          - key: aliyun.accelerator/nvidia_name\n" +
                "            operator: In\n" +
                "            values:\n" +
                "            - Tesla-V100-SXM2-16GB\n";
        fluidDataset.setDatasetConf(datasetConf);

        String runtimeConfig = String.join("\n", "apiVersion: data.fluid.io/v1alpha1",
                "kind: AlluxioRuntime",
                "metadata:",
                "  name: kubeflow-oss-mnist",
                "spec:",
                "  replicas: 4",
                "  data:",
                "    replicas: 1",
                "#  alluxioVersion:",
                "#    image: registry.cn-huhehaote.aliyuncs.com/alluxio/alluxio",
                "#    imageTag: \"2.3.0-SNAPSHOT-bbce37a\"",
                "#    imagePullPolicy: Always",
                "  tieredstore:",
                "    levels:",
                "      - mediumtype: SSD",
                "        path: /var/lib/docker/alluxio",
                "        quota: 50Gi",
                "        high: \"0.99\"",
                "        low: \"0.8\"",
                "  properties:",
                "    # alluxio fuse",
                "    alluxio.fuse.jnifuse.enabled: \"true\"",
                "    alluxio.fuse.debug.enabled: \"false\"",
                "    alluxio.fuse.cached.paths.max: \"1000000\"",
                "    alluxio.fuse.logging.threshold: 1000ms",
                "    # alluxio master",
                "    alluxio.master.metastore: ROCKS",
                "    alluxio.master.journal.folder: /journal",
                "    alluxio.master.journal.type: UFS",
                "    alluxio.master.metastore.inode.cache.max.size: \"10000000\"",
                "    alluxio.master.journal.log.size.bytes.max: 500MB",
                "    alluxio.master.metadata.sync.concurrency.level: \"128\"",
                "    alluxio.master.metadata.sync.executor.pool.size: \"128\"",
                "    alluxio.master.metadata.sync.ufs.prefetch.pool.size: \"128\"",
                "    alluxio.master.rpc.executor.max.pool.size: \"1024\"",
                "    alluxio.master.rpc.executor.core.pool.size: \"128\"",
                "    # alluxio worker",
                "    alluxio.worker.allocator.class: alluxio.worker.block.allocator.GreedyAllocator",
                "    alluxio.worker.network.reader.buffer.size: 32MB",
                "    alluxio.worker.file.buffer.size: 320MB",
                "    alluxio.worker.block.master.client.pool.size: \"1024\"",
                "    # alluxio user",
                "    alluxio.user.block.worker.client.pool.min: \"512\"",
                "    alluxio.user.file.writetype.default: MUST_CACHE",
                "    alluxio.user.ufs.block.read.location.policy: alluxio.client.block.policy.LocalFirstAvoidEvictionPolicy",
                "    alluxio.user.block.write.location.policy.class: alluxio.client.block.policy.LocalFirstAvoidEvictionPolicy",
                "    alluxio.user.block.size.bytes.default: 16MB",
                "    alluxio.user.streaming.reader.chunk.size.bytes: 32MB",
                "    alluxio.user.local.reader.chunk.size.bytes: 32MB",
                "    alluxio.user.metrics.collection.enabled: \"false\"",
                "    alluxio.user.update.file.accesstime.disabled: \"true\"",
                "    alluxio.user.file.passive.cache.enabled: \"false\"",
                "    alluxio.user.block.avoid.eviction.policy.reserved.size.bytes: 2GB",
                "    alluxio.user.block.master.client.pool.gc.threshold: 2day",
                "    alluxio.user.file.master.client.threads: \"1024\"",
                "    alluxio.user.block.master.client.threads: \"1024\"",
                "    alluxio.user.file.readtype.default: CACHE",
                "    alluxio.user.metadata.cache.enabled: \"true\"",
                "    alluxio.user.metadata.cache.expiration.time: 2day",
                "    alluxio.user.metadata.cache.max.size: \"1000000\"",
                "    alluxio.user.direct.memory.io.enabled: \"true\"",
                "    alluxio.user.worker.list.refresh.interval: 2min",
                "    alluxio.user.logging.threshold: 1000ms",
                "    # other alluxio configurations",
                "    alluxio.web.ui.enabled: \"false\"",
                "    alluxio.security.stale.channel.purge.interval: 365d",
                "    alluxio.job.worker.threadpool.size: \"164\"",
                "  master:",
                "    jvmOptions:",
                "      - \"-Xmx6G\"",
                "      - \"-XX:+UnlockExperimentalVMOptions\"",
                "      - \"-XX:ActiveProcessorCount=8\"",
                "  worker:",
                "    jvmOptions:",
                "      - \"-Xmx12G\"",
                "      - \"-XX:+UnlockExperimentalVMOptions\"",
                "      - \"-XX:MaxDirectMemorySize=32g\"",
                "      - \"-XX:ActiveProcessorCount=8\"",
                "    resources:",
                "      limits:",
                "        cpu: 8",
                "  fuse:",
                "#    image: registry.cn-huhehaote.aliyuncs.com/alluxio/alluxio-fuse",
                "#    imageTag: \"2.3.0-SNAPSHOT-bbce37a\"",
                "#    imagePullPolicy: Always",
                "    env:",
                "      MAX_IDLE_THREADS: \"32\"",
                "    jvmOptions:",
                "      - \"-Xmx16G\"",
                "      - \"-Xms16G\"",
                "      - \"-XX:+UseG1GC\"",
                "      - \"-XX:MaxDirectMemorySize=32g\"",
                "      - \"-XX:+UnlockExperimentalVMOptions\"",
                "      - \"-XX:ActiveProcessorCount=24\"",
                "    resources:",
                "      limits:",
                "        cpu: 16",
                "    args:",
                "      - fuse",
                "      - --fuse-opts=kernel_cache,ro,max_read=131072,attr_timeout=7200,entry_timeout=7200,nonempty");
        fluidDataset.setRuntimeConf(runtimeConfig);
        KubernetesClient client = getClient();
        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext.Builder()
                .withGroup("data.fluid.io")
                .withKind("Dataset")
                .withScope("Namespaced")
                .withVersion("v1alpha1")
                .withPlural("datasets")
                .build();

        CustomResourceDefinitionContext runtimeCtx = new CustomResourceDefinitionContext.Builder()
                .withVersion("v1alpha1")
                .withKind("AlluxioRuntime")
                .withPlural("alluxioruntimes")
                .withScope("Namespaced")
                .withGroup("data.fluid.io")
                .build();

        try {
            K8sFluidDataset dataset = JSON.parseObject(fluidDataset.getDatasetConf(), K8sFluidDataset.class);
            InputStream datasetInputStream = new ByteArrayInputStream(fluidDataset.getDatasetConf().getBytes(StandardCharsets.UTF_8));
            Map<String, Object> res = client.customResource(ctx).create(dataset.getMetadata().getNamespace(), datasetInputStream);
            res.entrySet().stream().forEach(x -> log.info("create dataset res {}", x));

            InputStream inputStream = new ByteArrayInputStream(fluidDataset.getRuntimeConf().getBytes(StandardCharsets.UTF_8));
            res = client.customResource(runtimeCtx).createOrReplace(dataset.getMetadata().getNamespace(), inputStream);
            res.entrySet().stream().forEach(x -> log.info("create runtime res {}", x));
        } catch (IOException e) {
            log.error("create elastic quota failed", e);
        }
    }

    @Test
    public void testCreatePVC() {
        String pvcName = "oss-pvc";

        KubernetesClient client = getClient();

        Map<String, Quantity> requests = new HashMap<>();
        requests.put("storage", new Quantity("5Gi"));

        PersistentVolumeClaim pvc = new PersistentVolumeClaimBuilder()
                .withNewMetadata().withName(pvcName).endMetadata()
                .withNewSpec()
                .withAccessModes("ReadWriteMany")
                .withResources(new ResourceRequirementsBuilder()
                        .withRequests(requests)
                        .build())
                .endSpec()
                .build();

        client.persistentVolumeClaims().inNamespace("default").create(pvc);
    }

    @Test
    public void testDeletePVC() {
        String pvcName = "fashion-mnist-sample";
        KubernetesClient client = getClient();
        boolean success = client.persistentVolumeClaims().inNamespace("default").withName(pvcName).delete();
        System.out.println(success);
    }

    @Test
    public void testDeletePV() {
        String pvName = "fashion-mnist-sample";
        KubernetesClient client = getClient();
        boolean success = client.persistentVolumes().withName(pvName).delete();
        System.out.println(success);
    }

    @Test
    public void testDeleteCRD() {
        KubernetesClient client = getClient();

        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext.Builder()
                .withGroup("scheduling.sigs.k8s.io")
                .withKind("ElasticQuota")
                .withVersion("v1alpha1")
                .withScope("Namespaced")
                .withPlural("elasticquotas")
                .build();


        //JSONObject metadataNode = new JSONObject();
        //metadataNode.put("namespace", "default");

        try {
            Map<String, Object> res = client.customResource(ctx).delete("default", "jackwg-test-group0");
            res.entrySet().stream().forEach(x -> log.info("create elastic quota res {}:{}", x.getKey(), x.getValue()));
        } catch (Exception e) {
            log.info("delete exception:{}", e.getMessage());
        }
    }

    @Test
    public void testPatchCRD() {
        KubernetesClient client = getClient();

        CustomResourceDefinitionContext ctx = new CustomResourceDefinitionContext.Builder()
                .withGroup("scheduling.sigs.k8s.io")
                .withKind("ElasticQuota")
                .withVersion("v1alpha1")
                .withScope("Namespaced")
                .withPlural("elasticquotas")
                .build();


        //JSONObject metadataNode = new JSONObject();
        //metadataNode.put("namespace", "default");

        Map<String, Object> patchRequest = new HashMap<String, Object>();
        try {
            Map<String, Object> res = client.customResource(ctx).createOrReplace(patchRequest);
            res.entrySet().stream().forEach(x -> log.info("create elastic quota res {}:{}", x.getKey(), x.getValue()));
        } catch (Exception e) {
            log.info("delete exception:{}", e.getMessage());
        }
    }

}
