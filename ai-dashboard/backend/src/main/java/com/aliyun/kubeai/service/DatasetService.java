/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
*/
    
package com.aliyun.kubeai.service;

import com.aliyun.kubeai.cluster.KubeClient;
import com.aliyun.kubeai.entity.FluidDataset;
import com.aliyun.kubeai.model.k8s.K8sFluidDataset;
import com.aliyun.kubeai.model.k8s.K8sFluidDatasetList;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.base.Strings;
import io.fabric8.kubernetes.client.CustomResourceList;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.base.CustomResourceDefinitionContext;
import io.fabric8.kubernetes.client.utils.Serialization;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.io.ByteArrayInputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static com.aliyun.kubeai.utils.DateUtil.transUTCTime;

@Slf4j
@Service
public class DatasetService {
    private final static String DATASET_CRD_NAME = "datasets.data.fluid.io";
    private final static String ALLUXIO_RUNTIME_CRD_NAME = "alluxioruntimes.data.fluid.io";
    private final static String JINDO_RUNTIME_CRD_NAME = "jindoruntimes.data.fluid.io";

    @Resource
    private KubeClient kubeClient;

    private String getRunTimeCrdName(FluidDataset dataset) throws Exception{
        ObjectMapper yamlReader = Serialization.yamlMapper();
        Object obj = yamlReader.readValue(dataset.getRuntimeConf(), Object.class);
        if (null == obj) {
            throw new Exception("runtime type empty");
        }
        String runtimeKind = ((LinkedHashMap<String, Object>)obj).get("kind").toString();
        if (!runtimeKind.equals("AlluxioRuntime")) {
            return JINDO_RUNTIME_CRD_NAME;
        }
        return ALLUXIO_RUNTIME_CRD_NAME;
    }

    public boolean createFluidDataset(FluidDataset dataset) throws Exception {
        KubernetesClient client = kubeClient.getClient();
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(DATASET_CRD_NAME);
        CustomResourceDefinitionContext runtimeCtx = kubeClient.buildCrdContext(getRunTimeCrdName(dataset));

        K8sFluidDataset k8sFluidDataset = Serialization.unmarshal(dataset.getDatasetConf(), K8sFluidDataset.class);
        String namespace = k8sFluidDataset.getMetadata().getNamespace();
        try {
            InputStream datasetInputStream = new ByteArrayInputStream(dataset.getDatasetConf().getBytes(StandardCharsets.UTF_8));
            Map<String, Object> res = client.customResource(ctx).createOrReplace(namespace, datasetInputStream);
            res.entrySet().stream().forEach(x -> log.info("create dataset res {}", x));

            InputStream inputStream = new ByteArrayInputStream(dataset.getRuntimeConf().getBytes(StandardCharsets.UTF_8));
            res = client.customResource(runtimeCtx).createOrReplace(namespace, inputStream);
            res.entrySet().stream().forEach(x -> log.info("create runtime res {}", x));
            return true;
        } catch (Exception e) {
            log.error("create dataset failed", e);
            throw new Exception(e);
        }
    }

    public List<K8sFluidDataset> listFluidDatasets(String name, String namespace, boolean isStrictMatch) throws Exception{
        log.info("list fluid dataset name:[{}] ns:[{}]", name, namespace);
        KubernetesClient client = kubeClient.getClient();

        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(DATASET_CRD_NAME);

        MixedOperation<K8sFluidDataset, K8sFluidDatasetList,
                io.fabric8.kubernetes.client.dsl.Resource<K8sFluidDataset>> datasetClient =
                client.customResources(ctx, K8sFluidDataset.class, K8sFluidDatasetList.class);
        CustomResourceList<K8sFluidDataset> res;
        if (Strings.isNullOrEmpty(namespace)) {
             res = datasetClient.inAnyNamespace().list();
        } else {
            res = datasetClient.inNamespace(namespace).list();
        }
        List<K8sFluidDataset> datasets = res.getItems();
        List<K8sFluidDataset> resDatasets = new ArrayList<>();
        for (K8sFluidDataset o : datasets) {
            if (!Strings.isNullOrEmpty(name)) {
                if (!o.getMetadata().getName().contains(name)) {
                    continue;
                }
                if (isStrictMatch) {
                    if (!o.getMetadata().getName().equals(name)) {
                        continue;
                    }
                }
            }
            o.getMetadata().setCreationTimestamp(transUTCTime(o.getMetadata().getCreationTimestamp()));
            o.getMetadata().getManagedFields().stream().forEach(x->x.setTime(transUTCTime(x.getTime())));
            resDatasets.add(o);
        }
        return resDatasets;
    }

    private void deleteFluidDatasetRuntime(String namespace, String name, String runtimeCrdName) {
        log.info("delete runtime name:{} namespace:{}", name, namespace);

        KubernetesClient client = kubeClient.getClient();
        try {
            CustomResourceDefinitionContext runtimeCtx = kubeClient.buildCrdContext(runtimeCrdName);
            Map<String, Object> res = client.customResource(runtimeCtx).delete(namespace, name);
            res.entrySet().stream().forEach(x -> log.info("delete runtime res {}:{}", x.getKey(), x.getValue()));
        } catch (Exception e) {
            log.error("delete fluid runtime exception", e);
            log.warn("delete runtime failed:{}", e.getMessage());
        }
        return;
    }


    public boolean deleteFluidDataset(String namespace, String name) throws Exception {
        log.info("delete dataset by namespace:{} name:{}", namespace, name);
        KubernetesClient client = kubeClient.getClient();

        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(DATASET_CRD_NAME);
        try {
            Map<String, Object> res = client.customResource(ctx).delete(namespace, name);
            res.entrySet().stream().forEach(x -> log.info("delete dataset res {}:{}", x.getKey(), x.getValue()));
        } catch (Exception e) {
            log.info("delete dataset exception:{}", e.getMessage());
            throw new Exception(e);
        } finally {
            deleteFluidDatasetRuntime(namespace, name, ALLUXIO_RUNTIME_CRD_NAME);
            deleteFluidDatasetRuntime(namespace, name, JINDO_RUNTIME_CRD_NAME);
        }

        return true;
    }
}
