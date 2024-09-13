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
    
package com.aliyun.kubeai.dao;

import com.alibaba.fastjson.JSON;
import com.aliyun.kubeai.cluster.KubeClient;
import com.aliyun.kubeai.model.k8s.eqtree.*;
import com.google.common.base.Strings;
import io.fabric8.kubernetes.client.CustomResourceList;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.fabric8.kubernetes.client.dsl.base.CustomResourceDefinitionContext;
import lombok.extern.slf4j.Slf4j;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Component;

import java.io.InputStream;
import java.util.List;
import java.util.Map;

import static com.aliyun.kubeai.utils.StringUtil.deserializeNodeName;

@Slf4j
@Component
public class EqTreeDao {
    public final static String DEFAULT_QUOTA_TREE_NAMESPACE = "kube-system";
    public final static String ELASTIC_QUOTA_TREE_CRD_NAME = "elasticquotatrees.scheduling.sigs.k8s.io";
    public final static String DEFAULT_QUOTA_TREE_NAME = "elasticquotatree";

    @javax.annotation.Resource
    private KubeClient kubeClient;

    private ElasticQuotaTree setNullWithDefaultValue(ElasticQuotaTree eqtree) {
        if (eqtree.getStatus() == null) {
            eqtree.setStatus(new Status()); // compatiable to k8s api 1.18
        }
        return eqtree;
    }

    public void createEqTreeFromFile(String yamlFileName, boolean isReplace) throws Exception {
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(ELASTIC_QUOTA_TREE_CRD_NAME);
        MixedOperation<ElasticQuotaTree, ElasticQuotaTreeList,
                Resource<ElasticQuotaTree>> client =
                kubeClient.getClient().customResources(ctx, ElasticQuotaTree.class, ElasticQuotaTreeList.class);
        InputStream defaultYamlStream = new ClassPathResource(yamlFileName).getInputStream();
        ElasticQuotaTree eqtree = client.load(defaultYamlStream).get();
        if (Strings.isNullOrEmpty(eqtree.getMetadata().getNamespace())) {
            eqtree.getMetadata().setNamespace(DEFAULT_QUOTA_TREE_NAMESPACE);
        }
        if (!isReplace) {
            ElasticQuotaTreeWithPrefix foundEqtree = this.getElasticQuotaTree(eqtree.getMetadata().getName(), eqtree.getMetadata().getNamespace());
            if (null != foundEqtree) {
                return;
            }
        }
        eqtree = setNullWithDefaultValue(eqtree);
        client.inNamespace(eqtree.getMetadata().getNamespace()).createOrReplace(eqtree);
        return;
    }

    public ElasticQuotaTree createOrReplaceEqTree(ElasticQuotaTree tree) throws Exception{
        if (tree == null) {
            return null;
        }
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(ELASTIC_QUOTA_TREE_CRD_NAME);
        tree = setNullWithDefaultValue(tree);
        String objJsonString = JSON.toJSONString(tree).replace("cRDName", "crdName");
        Map<String, Object> res = kubeClient.getClient().customResource(ctx)
                .createOrReplace(DEFAULT_QUOTA_TREE_NAMESPACE, objJsonString);
        if (null == res) {
            return null;
        }
        return tree;
    }

    public boolean deleteElasticQuotaTree(String name, String namespace) throws Exception{
        KubernetesClient client = kubeClient.getClient();
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(ELASTIC_QUOTA_TREE_CRD_NAME);
        if (Strings.isNullOrEmpty(namespace)) {
            namespace = DEFAULT_QUOTA_TREE_NAMESPACE;
        }
        Map<String, Object> res = client.customResource(ctx).delete(namespace, name);
        res.entrySet().stream().forEach(x->log.info("delete elastic quota tree res {}:{}", x.getKey(), x.getValue()));
        return true;
    }

    public ElasticQuotaTreeWithPrefix getElasticQuotaTree(String treeName, String namespace) {
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(ELASTIC_QUOTA_TREE_CRD_NAME);
        MixedOperation<ElasticQuotaTreeWithPrefix, ElasticQuotaTreeWithPrefixList,
                Resource<ElasticQuotaTreeWithPrefix>> eqTreeClient =
                kubeClient.getClient().customResources(ctx, ElasticQuotaTreeWithPrefix.class, ElasticQuotaTreeWithPrefixList.class);
        CustomResourceList<ElasticQuotaTreeWithPrefix> res;
        if (Strings.isNullOrEmpty(treeName)) {
            treeName = DEFAULT_QUOTA_TREE_NAME;
        }
        if (!Strings.isNullOrEmpty(namespace)) {
            res = eqTreeClient.inNamespace(namespace).list();
        } else {
            res = eqTreeClient.inAnyNamespace().list();
        }

        for (ElasticQuotaTreeWithPrefix t : res.getItems()) {
            if (!Strings.isNullOrEmpty(treeName)) {
                if (t.getMetadata().getName().equals(treeName)) {
                    log.info("got tree:{}", JSON.toJSONString(t));
                    return parseToNodeTree(t);
                }
                continue;
            } else {
                log.info("got tree:{}", JSON.toJSONString(t));
                return parseToNodeTree(t); //return the first one, TODO sort by create time
            }
        }
        return null;
    }

    private ElasticQuotaTreeWithPrefix parseToNodeTree(ElasticQuotaTreeWithPrefix tree) {
        ElasticQuotaNodeWithPrefix root = tree.getSpec().getRoot();
        ElasticQuotaNodeWithPrefix newRoot = parseNodeWithPrefix(root);
        tree.getSpec().setRoot(newRoot);
        return tree;
    }

    private ElasticQuotaNodeWithPrefix parseNodeWithPrefix(ElasticQuotaNodeWithPrefix root) {
        String nodeName = root.getName();
        String nodePrefix = root.getPrefix();
        List<String> deserializeNodeNames = deserializeNodeName(nodeName);
        root.setName(deserializeNodeNames.get(1));
        if (null != nodePrefix) {
            root.setPrefix(nodePrefix);
        } else {
            root.setPrefix(deserializeNodeNames.get(0));
        }

        if (null != root.getChildren()) {
            for (ElasticQuotaNodeWithPrefix child: root.getChildren()) {
                parseNodeWithPrefix(child);
            }
        }
        return root;
    }
}
