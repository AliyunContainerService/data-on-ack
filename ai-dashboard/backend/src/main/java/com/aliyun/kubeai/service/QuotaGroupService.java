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

import com.alibaba.fastjson.JSON;
import com.aliyun.kubeai.cluster.KubeClient;
import com.aliyun.kubeai.dao.EqTreeDao;
import com.aliyun.kubeai.entity.ElasticQuotaGroup;
import com.aliyun.kubeai.entity.ResourceQuotaType;
import com.aliyun.kubeai.model.k8s.eqtree.*;
import com.aliyun.kubeai.utils.K8sUtil;
import com.google.common.base.Strings;
import io.fabric8.kubernetes.api.model.*;
import io.fabric8.kubernetes.client.KubernetesClient;
import lombok.extern.slf4j.Slf4j;
import org.apache.commons.lang3.StringUtils;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.*;
import java.util.stream.Collectors;

import static com.aliyun.kubeai.dao.EqTreeDao.DEFAULT_QUOTA_TREE_NAME;
import static com.aliyun.kubeai.dao.EqTreeDao.DEFAULT_QUOTA_TREE_NAMESPACE;
import static com.aliyun.kubeai.utils.StringUtil.deserializeNodeName;
import static com.aliyun.kubeai.utils.StringUtil.serializeNodeName;

@Slf4j
@Service
public class QuotaGroupService {
    private final static String PipelineRunner = "pipeline-runner";

    private final static String PipelineRunnerRoleFile = "pipelineRunnerRole.yaml";
    public static final String PipelineMinioArtifactSecretFile = "pipelineMinioArtifactSecret.yaml";

    private final static String PipelineRunnerRole = PipelineRunner;

    public static Map<String, String> resourceDefaultUnitMap = new HashMap<>();

    @Resource
    private EqTreeDao eqTreeDao;

    @Resource
    private KubeClient kubeClient;

    @Resource
    private K8sService k8sService;

    public QuotaGroupService() {
        if (resourceDefaultUnitMap.isEmpty()) {
            resourceDefaultUnitMap.put(K8sUtil.RESOUCE_TYPE_MEMORY, "M");
            resourceDefaultUnitMap.put(K8sUtil.RESOUCE_TYPE_ALIYUN_GPU_MEMORY, "G");
        }
    }

    public Map<String, List<String>> getQuotaNamespaceIndexByNameInTree(ElasticQuotaTreeWithPrefix tree, List<String> quotaNames) throws Exception {
        Map<String, List<String>> resNamespaces = new HashMap<>();
        for (String quotaName : quotaNames) {
            List<String> quotaNameList = deserializeNodeName(quotaName);
            String quotaNameWithoutPrefix = quotaNameList.get(1);
            String quotaNamePrefix = quotaNameList.get(0);
            List<String> oneQuotaNamesapces = findNamespaceByChangedNode(tree, quotaNameWithoutPrefix, quotaNamePrefix);
            if (null == oneQuotaNamesapces) {
                throw new Exception(String.format("quota node:%s not found in tree:%s", quotaName, tree.getMetadata().getName()));
            }
            resNamespaces.put(quotaName, oneQuotaNamesapces);
        }
        return resNamespaces;
    }

    public Set<String> getQuotaNamespaceByNameInTree(ElasticQuotaTreeWithPrefix tree, List<String> quotaNames) throws Exception {
        Set<String> resNamesapces = new HashSet<>();
        for (String quotaName : quotaNames) {
            List<String> quotaNameList = deserializeNodeName(quotaName);
            String quotaNameWithoutPrefix = quotaNameList.get(1);
            String quotaNamePrefix = quotaNameList.get(0);
            List<String> oneQuotaNamesapces = findNamespaceByChangedNode(tree, quotaNameWithoutPrefix, quotaNamePrefix);
            if (null == oneQuotaNamesapces) {
                log.warn(String.format("quota node:%s not found in tree:%s", quotaName, tree.getMetadata().getName()));
                continue;
            }
            resNamesapces.addAll(oneQuotaNamesapces);
        }
        return resNamesapces;
    }

    public Map<String, List<String>> getQuotaNamespacesIndexByName(String quotaTreeName, List<String> quotaNames) throws Exception{
        Map<String, List<String>> resNamespaces = new HashMap<>();
        if (null == quotaNames || quotaNames.isEmpty()) {
            return resNamespaces;
        }
        ElasticQuotaTreeWithPrefix oldTree = this.getElasticQuotaTree(quotaTreeName, null);
        if (null == oldTree) {
            throw new Exception(String.format("quota tree not found:%s", quotaTreeName));
        }
        return this.getQuotaNamespaceIndexByNameInTree(oldTree, quotaNames);
    }

    public Set<String> getQuotaNamespacesByName(String quotaTreeName, List<String> quotaNames) throws Exception{
        Set<String> resNamesapces = new HashSet<>();
        if (null == quotaNames || quotaNames.isEmpty()) {
            return resNamesapces;
        }
        ElasticQuotaTreeWithPrefix oldTree = this.getElasticQuotaTree(quotaTreeName, null);
        if (null == oldTree) {
            throw new Exception(String.format("quota tree not found:%s", quotaTreeName));
        }
        resNamesapces = getQuotaNamespaceByNameInTree(oldTree, quotaNames);
        return resNamesapces;
    }

    private List<ElasticQuotaNodeWithPrefix> findAncenstorNode(ElasticQuotaTreeWithPrefix tree, String nodeNameWithoutPrefix, String nodeNamePrefix) {
        List<ElasticQuotaNodeWithPrefix> res = new ArrayList<>();
        ElasticQuotaNodeWithPrefix rootNode = tree.getSpec().getRoot();
        ElasticQuotaNodeWithPrefix targetNode = findElasticNodeByName(rootNode, nodeNameWithoutPrefix, nodeNamePrefix);
        if (targetNode == null) {
            return null;
        }

        ElasticQuotaNodeWithPrefix parentNode = findElasticNodeParentByName(rootNode, nodeNameWithoutPrefix, nodeNamePrefix);
        while(null != parentNode) {
            res.add(parentNode);
            parentNode = findElasticNodeParentByName(rootNode, parentNode.getName(), parentNode.getPrefix());
        }
        return res;
    }

    public List<String> findAncenstorNodeNamesInTree(ElasticQuotaTreeWithPrefix tree, String nodeName, String prefix) {
        String nodeNameWithPrefix = serializeNodeName(prefix, nodeName);
        if (Strings.isNullOrEmpty(nodeNameWithPrefix)) {
            return null;
        }
        List<ElasticQuotaNodeWithPrefix> nodes = findAncenstorNode(tree, nodeName, prefix);
        List<String> res = new ArrayList<>();
        if (null != nodes) {
            res.addAll(nodes.stream().map(x->serializeNodeName(x.getPrefix(), x.getName())).collect(Collectors.toList()));
        }
        res.add(nodeNameWithPrefix);
        return res;
    }

    public List<ElasticQuotaNodeWithPrefix> getLeafNodes(ElasticQuotaNodeWithPrefix rootNode, List<ElasticQuotaNodeWithPrefix> res) {
        if (res == null)  {
            res = new ArrayList<>();
        }
        if (rootNode.getChildren() == null || rootNode.getChildren().isEmpty()) {
            res.add(rootNode);
        }
        if (null != rootNode.getChildren()) {
            for (ElasticQuotaNodeWithPrefix child : rootNode.getChildren()) {
                getLeafNodes(child, res);
            }
        }
        return res;
    }

    public List<ElasticQuotaNodeWithPrefix> findLeafNodeByNode(ElasticQuotaTreeWithPrefix tree, String nodeName, String prefix) {
        ElasticQuotaNodeWithPrefix changedNode = findElasticNodeByName(tree.getSpec().getRoot(), nodeName, prefix);
        return getLeafNodes(changedNode, null);
    }

    public List<String> findAncenstorNodeNames(String quotaTreeName, String nodeName, String prefix) {
        String nodeNameWithPrefix = serializeNodeName(prefix, nodeName);
        if (Strings.isNullOrEmpty(nodeNameWithPrefix)) {
            return null;
        }
        ElasticQuotaTreeWithPrefix oldTree = this.getElasticQuotaTree(quotaTreeName, null);
        return findAncenstorNodeNamesInTree(oldTree, nodeName, prefix);
    }

    private ElasticQuotaNodeWithPrefix findElasticNodeParentByName(ElasticQuotaNodeWithPrefix root, String toFindName, String toFindPrefix) {
        if (root == null || Strings.isNullOrEmpty(root.getName())) {
            return null;
        }
        List<ElasticQuotaNodeWithPrefix> childs = root.getChildren();
        if (null == childs || childs.isEmpty()) {
            return null;
        }
        for (ElasticQuotaNodeWithPrefix child : childs) {
            if (child.getName().equals(toFindName)) {
                return root;
            }
            ElasticQuotaNodeWithPrefix subRes = findElasticNodeByName(child, toFindName, toFindPrefix);
            if (null != subRes) {
                return child;
            }
        }
        return null;
    }

    public ElasticQuotaNodeWithPrefix findElasticNodeByName(ElasticQuotaNodeWithPrefix root, String nodeName, String prefix) {
        if (root == null || Strings.isNullOrEmpty(root.getName())) {
            return null;
        }
        if (root.getName().equals(nodeName)) {
            if (Strings.isNullOrEmpty(root.getPrefix()) && Strings.isNullOrEmpty(prefix)) {
                return root;
            }
            if (root.getPrefix().equals(prefix)) {
                return root;
            }
        }
        if (null != root.getChildren()) {
            for (ElasticQuotaNodeWithPrefix child : root.getChildren()) {
                ElasticQuotaNodeWithPrefix res = findElasticNodeByName(child, nodeName, prefix);
                if (res != null) {
                    return res;
                }
            }
        }
        return null;
    }

    public List<String> getAllNamespaces(ElasticQuotaNodeWithPrefix root, List<String> res) {
        if (null != root.getNamespaces() && !root.getNamespaces().isEmpty()) {
            res.addAll(root.getNamespaces());
        }
        if (null != root.getChildren()) {
            for (ElasticQuotaNodeWithPrefix child : root.getChildren()) {
                getAllNamespaces(child, res);
            }
        }
        return res;
    }

    private List<String> getAllNamepaces(ElasticQuotaNodeWithPrefix root, List<String> res) {
        if (null != root.getNamespaces() && !root.getNamespaces().isEmpty()) {
            res.addAll(root.getNamespaces());
        }
        if (null != root.getChildren()) {
            for (ElasticQuotaNodeWithPrefix child : root.getChildren()) {
                getAllNamepaces(child, res);
            }
        }
        return res;
    }

    public List<String> findNamespaceByChangedNode(ElasticQuotaTreeWithPrefix tree, String changedNodeName, String changedNodePrefix) throws Exception{
        ElasticQuotaNodeWithPrefix root = tree.getSpec().getRoot();
        if (root == null || Strings.isNullOrEmpty(root.getName())) {
            return null;
        }
        ElasticQuotaNodeWithPrefix changedNode = findElasticNodeByName(root, changedNodeName, changedNodePrefix);
        if (changedNode == null) {
            return null;
        }
        List<String> res = new ArrayList<>();
        res = getAllNamepaces(changedNode, res);
        return res;
    }

    public boolean createElasticQuotaTree(ElasticQuotaNodeWithPrefix rootNode, String name, String namespace) throws Exception {
        ElasticQuotaTreeWithPrefix tree = new ElasticQuotaTreeWithPrefix();
        SpecWithPrefix spec = new SpecWithPrefix();
        tree.setSpec(spec);

        ObjectMeta metaData = new ObjectMeta();
        tree.setMetadata(metaData);

        if (rootNode != null) {
            spec.setRoot(rootNode);
        }
        metaData.setName(name);
        if (Strings.isNullOrEmpty(name)) {
            metaData.setName(DEFAULT_QUOTA_TREE_NAME);
        }
        metaData.setNamespace(namespace);
        if (Strings.isNullOrEmpty(namespace)) {
            metaData.setNamespace(DEFAULT_QUOTA_TREE_NAMESPACE);
        }
        return this.createOrReplaceElasticQuotaTree(tree);
    }

    public ElasticQuotaTreeWithPrefix getElasticQuotaTree(String treeName, String namespace) {
        return eqTreeDao.getElasticQuotaTree(treeName, namespace);
    }

    public boolean deleteElasticQuotaTree(String namespace, String treeName) throws Exception {
        return eqTreeDao.deleteElasticQuotaTree(treeName, namespace);
    }

    public boolean createOrReplaceElasticQuotaTree(ElasticQuotaTreeWithPrefix newTree) throws Exception {
        log.info("client got tree:{}", JSON.toJSONString(newTree));
        ElasticQuotaTree treeWithoutPrefix = parseToTreeWithoutPrefix(newTree);
        ElasticQuotaTree res = eqTreeDao.createOrReplaceEqTree(treeWithoutPrefix);
        if (null == res) {
            log.info("client create or replace tree res null");
            return false;
        }
        log.info("create or replace quota tree res:{}", res);
        return true;
    }

    private ResourceQuota parseQuotaGroupToResourceQuota(ElasticQuotaGroup quotaGroup) throws Exception{
        ResourceQuota resourceQuota = new ResourceQuota();
        ObjectMeta meta = new ObjectMetaBuilder().withName(quotaGroup.getName()).withNamespace(quotaGroup.getSubGroupNames().get(0)).build();
        resourceQuota.setMetadata(meta);
        Map<String, Quantity> hard = new HashMap<>();
        for (ElasticQuotaGroup.ResourceQuota quota : quotaGroup.getQuotaList()) {
            String resourceName = quota.getResourceName();
            String maxQuota = quota.getMax();
            if (resourceDefaultUnitMap.containsKey(resourceName.toLowerCase()) && StringUtils.isNumeric(maxQuota)) {
                maxQuota += resourceDefaultUnitMap.get(resourceName);
            }
            String requestResourceName = ResourceQuotaType.reqeustKey(resourceName);
            hard.put(requestResourceName, Quantity.parse(maxQuota));
        }
        ResourceQuotaSpec spec = new ResourceQuotaSpecBuilder().withHard(hard).build();
        resourceQuota.setSpec(spec);
        return resourceQuota;
    }
    private ElasticQuotaTree parseToTreeWithoutPrefix(ElasticQuotaTreeWithPrefix tree) {
        ElasticQuotaNodeWithPrefix treeRoot = tree.getSpec().getRoot();
        ElasticQuotaNode resultRoot = ConverterElasticQuotaNodeToElasticQuotaNodeWithoutPrefix(treeRoot);

        ElasticQuotaTree result = new ElasticQuotaTree();
        Spec spec = new Spec();
        spec.setRoot(resultRoot);
        result.setSpec(spec);
        result.setMetadata(new ObjectMetaBuilder()
                .withNamespace(tree.getMetadata().getNamespace())
                .withName(tree.getMetadata().getName()).build());
        return result;
    }

    private ElasticQuotaNode ConverterElasticQuotaNodeToElasticQuotaNodeWithoutPrefix(ElasticQuotaNodeWithPrefix node) {
        ElasticQuotaNode result = new ElasticQuotaNode();
        result.setName(serializeNodeName(node.getPrefix(), node.getName()));
        result.setMax(node.getMax());
        result.setMin(node.getMin());
        result.setNamespaces(node.getNamespaces());

        if (null != node.getChildren() && !node.getChildren().isEmpty()) {
            List<ElasticQuotaNode> children = new ArrayList<ElasticQuotaNode>();
            for (ElasticQuotaNodeWithPrefix item : node.getChildren()){
                children.add(ConverterElasticQuotaNodeToElasticQuotaNodeWithoutPrefix(item));
            }
            result.setChildren(children);
        }
        return result;
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

    public ElasticQuotaGroup ConverterElasticQuotaNodeToElasticQuotaGroup(ElasticQuotaNodeWithPrefix elasticQuotaNode) {
        ElasticQuotaGroup elasticQuotaGroup = new ElasticQuotaGroup();
        elasticQuotaGroup.setSubGroupNames(elasticQuotaNode.getNamespaces());
        elasticQuotaGroup.setName(elasticQuotaNode.getName().toLowerCase());

        Map<String, String> minMap = elasticQuotaNode.getMin();
        Map<String, String> maxMap = elasticQuotaNode.getMax();
        List<ElasticQuotaGroup.ResourceQuota> resourceQuotas = new ArrayList<ElasticQuotaGroup.ResourceQuota>();

        for (Map.Entry<String, String> item : minMap.entrySet()) {
            ElasticQuotaGroup.ResourceQuota resourceQuota = new ElasticQuotaGroup.ResourceQuota();
            resourceQuota.setResourceName(item.getKey());
            resourceQuota.setMin(item.getValue());
            resourceQuota.setMax(maxMap.get(item.getKey()));
            resourceQuotas.add(resourceQuota);
        }
        elasticQuotaGroup.setQuotaList(resourceQuotas);
        return elasticQuotaGroup;
    }

    public boolean createResourceQuotaFromElasticQuotaTree(ElasticQuotaTreeWithPrefix elasticQuotaTree) throws Exception {
        ElasticQuotaNodeWithPrefix root = elasticQuotaTree.getSpec().getRoot();
        return createResourceQuota(root);
    }

    public boolean createResourceQuota(ElasticQuotaNodeWithPrefix elasticQuotaNode) throws Exception {
        if (elasticQuotaNode.getChildren() == null || elasticQuotaNode.getChildren().size() == 0){
            KubernetesClient client = kubeClient.getClient();
            ElasticQuotaGroup elasticQuotaGroup = ConverterElasticQuotaNodeToElasticQuotaGroup(elasticQuotaNode);
            if (elasticQuotaGroup.getSubGroupNames() == null || elasticQuotaGroup.getSubGroupNames().isEmpty()) {
                log.info("skip node name:{}", elasticQuotaNode.getName());
                return true;
            }
            if (elasticQuotaGroup.getSubGroupNames() != null && elasticQuotaGroup.getSubGroupNames().size() > 1){
                log.error("resource quota namespaces > 1.");
                return false;
            }
            ResourceQuota resourceQuota = parseQuotaGroupToResourceQuota(elasticQuotaGroup);
            String resourceQuotaJson = JSON.toJSONString(resourceQuota);
            log.info("create or replace resource quota:{}", resourceQuotaJson);
            try {
                ResourceQuota res = client.resourceQuotas().inNamespace(resourceQuota.getMetadata().getNamespace()).withName(elasticQuotaGroup.getName()).createOrReplace(resourceQuota);
                log.info("create quota res:{}", JSON.toJSONString(res));
            } catch (Exception e) {
                log.error("create elastic quota failed", e);
                return false;
            }
            return true;
        }else {
            boolean flag = true;
            for (ElasticQuotaNodeWithPrefix node : elasticQuotaNode.getChildren()) {
                flag = flag && createResourceQuota(node);
            }
            return flag;
        }
    }

    public boolean deleteResourceQuota(String namespace, String groupName) throws Exception {
        if (Strings.isNullOrEmpty(namespace) || Strings.isNullOrEmpty(groupName)) {
            log.warn("delete group without name:{} or namespace:{}", groupName, namespace);
            return false;
        }
        KubernetesClient client =  kubeClient.getClient();
        ResourceQuota rq = new ResourceQuota();
        ObjectMeta meta = new ObjectMetaBuilder().withNamespace(namespace).withName(groupName).build();
        rq.setMetadata(meta);
        try {
            return client.resourceQuotas().inNamespace(namespace).delete(rq);
        } catch (Exception e) {
            log.info("delete exception:{}", e.getMessage());
            throw new Exception(e);
        }
    }

    // update/create pipeline runner service/role/rolebindings for quota
    public boolean updatePipelineRBAC(ElasticQuotaNodeWithPrefix node) throws Exception {
        List<String> namespaces = node.getNamespaces();
        if (namespaces == null){
            return true;
        }
        for(String namespace: namespaces) {
            updatePipelineRBAC(namespace);
        }
        List<ElasticQuotaNodeWithPrefix> childs  = node.getChildren();
        if (null == childs || childs.isEmpty()) {
            return true;
        }
        for (ElasticQuotaNodeWithPrefix child: childs) {
            updatePipelineRBAC(child);
        }
        return true;
    }
    private boolean updatePipelineRBAC(String namespace) throws Exception {
        // check pipeline runner service account exist
        ServiceAccount pipeline_sa = k8sService.getServiceAccount(PipelineRunner, namespace);
        if (pipeline_sa == null) {
            log.info(String.format("create pipeline runner service account in %s",namespace));

            // create pipeline runner service account
            pipeline_sa = kubeClient.createServiceAccountWithAutoMount(namespace, PipelineRunner);
            if (pipeline_sa == null) {
                log.error(" fail to pipeline runner serviceaccount in " + namespace);
                throw new Exception("fail to create service account");
            }

            // create role
            if (null == k8sService.createRole(PipelineRunnerRoleFile, PipelineRunnerRole, false, namespace)) {
                log.error("fail to create pipeline runner role in " + namespace);
                throw  new Exception("fail to create pipeline runner role");
            }

            // create rolebinding
            if (null == k8sService.bindRole(pipeline_sa, PipelineRunnerRole, false, namespace)) {
                log.error("fail to create pipeline runner rolebinding in " + namespace);
                throw new Exception("fail to create pipeline runner role binding");
            }

            // create mlpipeline-minio-artifact secret
            k8sService.createSecret(PipelineMinioArtifactSecretFile, namespace);
            log.info(String.format("success create pipeline runner RBAC in %s",namespace));
        }

        return true;
    }
    public void updateResearchRole(ElasticQuotaNodeWithPrefix node) throws Exception {
        List<String> namespaces = node.getNamespaces();
        if (namespaces == null){
            return;
        }
        for(String namespace: namespaces) {
            k8sService.updateResearcherRole(namespace);
        }
        List<ElasticQuotaNodeWithPrefix> childs  = node.getChildren();
        if (null == childs || childs.isEmpty()) {
            return;
        }
        for (ElasticQuotaNodeWithPrefix child: childs) {
            updateResearchRole(child);
        }
    }


}
