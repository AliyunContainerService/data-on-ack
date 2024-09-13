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
    
package com.aliyun.kubeai.controller;

import com.alibaba.fastjson.JSON;
import com.aliyun.kubeai.cluster.KubeClient;
import com.aliyun.kubeai.exception.K8sCRDNotFoundException;
import com.aliyun.kubeai.model.common.RequestResult;
import com.aliyun.kubeai.model.common.ResultCode;
import com.aliyun.kubeai.model.k8s.UserGroup;
import com.aliyun.kubeai.model.k8s.eqtree.ElasticQuotaNodeWithPrefix;
import com.aliyun.kubeai.model.k8s.eqtree.ElasticQuotaTreeWithPrefix;
import com.aliyun.kubeai.model.k8s.user.K8sRoleBinding;
import com.aliyun.kubeai.model.k8s.user.K8sServiceAccount;
import com.aliyun.kubeai.model.k8s.user.User;
import com.aliyun.kubeai.service.K8sService;
import com.aliyun.kubeai.service.QuotaGroupService;
import com.aliyun.kubeai.service.UserGroupService;
import com.aliyun.kubeai.service.UserService;
import com.aliyun.kubeai.utils.K8sUtil;
import com.google.common.base.Strings;
import com.google.common.collect.Sets;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Quantity;
import io.fabric8.kubernetes.api.model.QuantityBuilder;
import io.fabric8.kubernetes.api.model.ServiceAccount;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import java.util.*;
import java.util.stream.Collectors;

import static java.lang.Math.max;

@Slf4j
@RestController
@RequestMapping("/group")
public class ElasticQuotaController {

    private final static Integer MAX_QUOTA = 2147483647;
    private final static String UI_MAX_QUOTA = "N/A";
    private final static String CreateTree = "createTree";
    public final static String AddNode = "addNode";
    public final static String DeleteNode = "deleteNode";
    public final static String DeleteTree = "deleteTree";
    public final static String UpdateNode = "updateNode";
    public final static String UpdateResourceType = "updateResourceType";
    // PodPending means the pod has been accepted by the system, but one or more of the containers
    // has not been started. This includes time before being bound to a node, as well as time spent
    // pulling images onto the host.
    private final static String PodPending = "Pending";
    // PodRunning means the pod has been bound to a node and all of the containers have been started.
    // At least one container is still running or is in the process of being restarted.
    private final static String PodRunning = "Running";
    // PodSucceeded means that all containers in the pod have voluntarily terminated
    // with a container exit code of 0, and the system is not going to restart any of these containers.
    private final static String PodSucceeded = "Succeeded";
    // PodFailed means that all containers in the pod have terminated, and at least one container has
    // terminated in a failure (exited with a non-zero exit code or was stopped by the system).
    private final static String PodFailed = "Failed";
    // PodUnknown means that for some reason the state of the pod could not be obtained, typically due
    // to an error in communicating with the host of the pod.
    // Deprecated in v1.21: It isn't being set since 2015 (74da3b14b0c0f658b3bb8d2def5094686d0e9095)
    private final static String PodUnknown = "Unknown";


    @Resource
    private QuotaGroupService groupService;

    @Resource
    private UserGroupService userGroupService;

    @Resource
    private K8sService k8sService;

    @Resource
    private UserService userService;

    @Resource
    private KubeClient kubeClient;

    @GetMapping("/list")
    public RequestResult<ElasticQuotaTreeWithPrefix> listGroup(@RequestParam(name = "name", required = false) String name,
                                                               @RequestParam(name = "namespace", required = false) String namespace,
                                                               @RequestParam(name="page", required = false) Integer page,
                                                               @RequestParam(name="limit", required = false) Integer limit) {
        log.info("list group, page:{}, limit:{}, name:{}", page, limit, name);
        RequestResult<ElasticQuotaTreeWithPrefix> result = new RequestResult<>();
        try {
            ElasticQuotaTreeWithPrefix tree = groupService.getElasticQuotaTree(name, namespace);
            if (null != tree) {
                deserializeTree(tree.getSpec().getRoot());
                result.setData(tree);
            }
        } catch (K8sCRDNotFoundException e) {
            log.warn("elastic quota crd not found exception:{}", e.getMessage());
            result.setFailed(ResultCode.GROUP_CRD_NOT_FOUND_EXCEPTION, ResultCode.GROUP_CRD_NOT_FOUND_MESSAGE);
        } catch (Exception e) {
            log.error("list group exception", e);
            result.setFailed(ResultCode.LIST_GROUP_FAILED, "获取配额组异常");
        }
        log.info("list group result:{}", JSON.toJSONString(result));
        return result;
    }

    private int getKubernetesVersion() {
        String k8sVersion = k8sService.getK8sVersion();
        String[] version = k8sVersion.split("\\.");
        return Integer.parseInt(version[1]);
    }

    private Map<String, String> serializeMinMaxNan(Map<String, String> minOrMaxQuota) {
        Map<String, String> res = new HashMap<>();
        for (Map.Entry<String, String> kv: minOrMaxQuota.entrySet()) {
            String key = kv.getKey();
            String value = kv.getValue();
            Quantity q = Quantity.parse(value);
            if (q.getAmount().equals(UI_MAX_QUOTA) || Double.parseDouble(q.getAmount()) >= MAX_QUOTA) {
                q.setAmount(String.format("%d", MAX_QUOTA));
            }
            res.put(key, q.toString());
        }
        return res;
    }

    private Map<String, String> deserializeMinMaxNan(Map<String, String> minOrMaxQuota) {
        Map<String, String> res = new HashMap<>();
        for (Map.Entry<String, String> kv: minOrMaxQuota.entrySet()) {
            String key = kv.getKey();
            String value = kv.getValue();
            if (value == null) {
                value = UI_MAX_QUOTA;
            }
            Quantity q = Quantity.parse(value);
            if (Double.parseDouble(q.getAmount()) >= MAX_QUOTA) {
                q.setAmount(UI_MAX_QUOTA);
            }
            res.put(key, q.toString());
        }
        return res;
    }

    private void serializeTree(ElasticQuotaNodeWithPrefix rootNode) {
        rootNode.setMin(serializeMinMaxNan(rootNode.getMin()));
        rootNode.setMax(serializeMinMaxNan(rootNode.getMax()));
        List<ElasticQuotaNodeWithPrefix> childs = rootNode.getChildren();
        if (null == childs) {
            return;
        }
        for (ElasticQuotaNodeWithPrefix child: childs) {
            serializeTree(child);
        }
        return;
    }

    private void deserializeTree(ElasticQuotaNodeWithPrefix rootNode) {
        rootNode.setMin(deserializeMinMaxNan(rootNode.getMin()));
        rootNode.setMax(deserializeMinMaxNan(rootNode.getMax()));
        List<ElasticQuotaNodeWithPrefix> childs = rootNode.getChildren();
        if (null == childs) {
            return;
        }
        for (ElasticQuotaNodeWithPrefix child: childs) {
            deserializeTree(child);
        }
        return;
    }

    @PostMapping("create")
    public RequestResult<Void> createGroup(@RequestBody ElasticQuotaTreeWithPrefix tree,
                                           @RequestParam(name="name", required = false) String name,
                                           @RequestParam(name="namespace", required = false) String namespace) {
        log.info("create group tree: {}", JSON.toJSONString(tree));
        ElasticQuotaNodeWithPrefix rootNode = tree.getSpec().getRoot();
        RequestResult<Void> result = new RequestResult<>();
        try {
            serializeTree(rootNode);
            this.preCreateCheck(rootNode);
            boolean success = groupService.createElasticQuotaTree(rootNode, name, namespace);
            if (!success) {
                result.setFailed(ResultCode.CREATE_GROUP_FAILED, "创建配额组失败");
            }
            if (getKubernetesVersion() < 20) {
                success = groupService.createResourceQuotaFromElasticQuotaTree(tree);
                if (!success) {
                    result.setFailed(ResultCode.CREATE_GROUP_FAILED, "创建ResourceQuota失败");
                }
            }
        } catch (K8sCRDNotFoundException e) {
            log.warn("elastic quota crd not found exception:{}", e.getMessage());
            result.setFailed(ResultCode.GROUP_CRD_NOT_FOUND_EXCEPTION, ResultCode.GROUP_CRD_NOT_FOUND_MESSAGE);
        } catch (Exception e) {
            log.error("create group exception", e);
            result.setFailed(ResultCode.CREATE_DATASET_EXCEPTION, String.format("创建配额组异常, %s", e.getMessage()));
        }
        log.info("create group result: {}", JSON.toJSONString(result));
        return result;
    }

    private List<String> deserializeNodeName(String nodeNameWithPrefix) {
        List<String> listName = Arrays.asList(nodeNameWithPrefix.split("\\."));
        int prefixEndIndex = listName.size() - 2;
        String prefix = null;
        if (prefixEndIndex >= 0) {
            prefix = String.join(".", listName.subList(0, prefixEndIndex + 1));
        }
        return Arrays.asList(prefix, listName.get(listName.size() - 1));
    }

    private String serializeNodeName(String prefix, String nodeNameWithoutPrefix) {
        if (Strings.isNullOrEmpty(prefix)) {
            return nodeNameWithoutPrefix;
        }
        return prefix + "." + nodeNameWithoutPrefix;
    }


    private boolean updateSubNodeName(ElasticQuotaNodeWithPrefix node, String prefix, String newNodeName) {
        if (Strings.isNullOrEmpty(newNodeName)) {
            return false;
        }
        String childNewPrefix = newNodeName;
        if (!Strings.isNullOrEmpty(prefix)) {
            childNewPrefix = prefix + "." + newNodeName;
        }
        node.setName(newNodeName);
        node.setPrefix(prefix);
        List<ElasticQuotaNodeWithPrefix> childs  = node.getChildren();
        if (null == childs || childs.isEmpty()) {
            return true;
        }
        for (ElasticQuotaNodeWithPrefix child: childs) {
            updateSubNodeName(child, childNewPrefix, child.getName());
        }
        return true;
    }

    private List<String> genNewNodeNameByOldNodeName(ElasticQuotaNodeWithPrefix oldChildNode, ElasticQuotaNodeWithPrefix oldRootNode, ElasticQuotaNodeWithPrefix newRootNode) throws Exception{
        String oldChildNodeNameWithPrefix = serializeNodeName(oldChildNode.getPrefix(), oldChildNode.getName());
        String oldRootNodeNameWithPrefix = serializeNodeName(oldRootNode.getPrefix(), oldRootNode.getName());
        int suffixStartIndx = oldChildNodeNameWithPrefix.indexOf(oldRootNodeNameWithPrefix);
        if (suffixStartIndx < 0) {
            throw new Exception("node name not available");
        }
        suffixStartIndx += oldRootNodeNameWithPrefix.length();
        String newNodeNameWithPrefix = serializeNodeName(newRootNode.getPrefix(), newRootNode.getName()) + oldChildNodeNameWithPrefix.substring(suffixStartIndx);
        return deserializeNodeName(newNodeNameWithPrefix);
    }

    private boolean updateQuotaNameInUserGroup(ElasticQuotaTreeWithPrefix tree, ElasticQuotaNodeWithPrefix oldLeafNode, ElasticQuotaNodeWithPrefix newLeafNode) {
        if (oldLeafNode == null || newLeafNode == null) {
            log.info("update quota name with new or old leaf node empty");
            return false;
        }
        String oldQuotaNameWithPrefix = serializeNodeName(oldLeafNode.getPrefix(), oldLeafNode.getName());
        String newQuotaNameWithPrefix = serializeNodeName(newLeafNode.getPrefix(), newLeafNode.getName());
        List<UserGroup> quotaUserGroups = userGroupService.findUserGroupsByQuotaName(tree, oldLeafNode.getName(), oldLeafNode.getPrefix(), true);
        if (null == quotaUserGroups) {
            log.info("no user group found for quota:{}", oldQuotaNameWithPrefix);
            return true;
        }
        for (UserGroup group: quotaUserGroups) {
            final String tempOldQuotaName = oldQuotaNameWithPrefix;
            List<String> newQuotaNames = group.getSpec().getQuotaNames().stream().filter(x->!x.equals(tempOldQuotaName)).collect(Collectors.toList());
            if (!Strings.isNullOrEmpty(newQuotaNameWithPrefix)) {
                newQuotaNames = Arrays.asList(serializeNodeName(newLeafNode.getPrefix(), newLeafNode.getName()));
            }
            group.getSpec().setQuotaNames(newQuotaNames);
            if(!userGroupService.createOrReplaceUserGroup(group)){
                log.warn("update user group quota names failed");
                return false;
            }
        }
        return true;
    }

    private boolean updateUserGroupByEqTree(ElasticQuotaTreeWithPrefix oldTree, ElasticQuotaTreeWithPrefix newTree, ElasticQuotaNodeWithPrefix oldNode, ElasticQuotaNodeWithPrefix newNode) throws Exception{
        // update user group if quota name changed
        if (oldNode == null && newNode == null) {
            log.warn("no node to change");
            return true;
        }
        String oldQuotaNameWithPrefix = null;
        String newQuotaNameWithPrefix = null;
        if (null != oldNode) {
            oldQuotaNameWithPrefix = serializeNodeName(oldNode.getPrefix(), oldNode.getName());
        }
        if (null != newNode) {
            newQuotaNameWithPrefix = serializeNodeName(newNode.getPrefix(), newNode.getName());
        }
        if ((oldQuotaNameWithPrefix != null && oldQuotaNameWithPrefix.equals(newQuotaNameWithPrefix))
                ||(newQuotaNameWithPrefix != null && newQuotaNameWithPrefix.equals(oldQuotaNameWithPrefix))) {
            log.info("no name change for node old:{} new:{}", oldQuotaNameWithPrefix, newQuotaNameWithPrefix);
            return true;
        }

        if (Strings.isNullOrEmpty(oldQuotaNameWithPrefix)) {
            log.info("old quota name empty may new node newNode:{}", newQuotaNameWithPrefix);
            return true;
        }

        // get all leaf quota nodes
        List<ElasticQuotaNodeWithPrefix> changedNodes = groupService.findLeafNodeByNode(oldTree, oldNode.getName(), oldNode.getPrefix());
        for(ElasticQuotaNodeWithPrefix changedOldLeafNode: changedNodes) {
            // delete node action
            if (newNode == null) {
                List<UserGroup> quotaUserGroups = userGroupService.findUserGroupsByQuotaName(oldTree, changedOldLeafNode.getName(), changedOldLeafNode.getPrefix(), true);
                if (quotaUserGroups != null && quotaUserGroups.size() > 0) {
                    String userGroupInUsingName = quotaUserGroups.get(0).getSpec().getGroupName();
                    log.warn("delete tree leaf node:{} but user group:{} still using it", oldQuotaNameWithPrefix, userGroupInUsingName);
                    throw new Exception(String.format("Please delete user group first userGroup name:%s", userGroupInUsingName));
                }
            } else {
                List<String> newNodeNames = genNewNodeNameByOldNodeName(changedOldLeafNode, oldNode, newNode);
                ElasticQuotaNodeWithPrefix newLeafNode = groupService.findElasticNodeByName(newTree.getSpec().getRoot(), newNodeNames.get(1), newNodeNames.get(0));
                if (!updateQuotaNameInUserGroup(oldTree, changedOldLeafNode, newLeafNode)) {
                    log.info("update quota names in user group failed");
                    return false;
                }
            }
        }

        return true;
    }

    private List<K8sRoleBinding> regenerateUserRoleBindings(ServiceAccount serviceAccount,
                                                            Set<String> userRoles,
                                                            Set<String> userClusterRoles,
                                                            List<UserGroup> userGroups,
                                                            ElasticQuotaTreeWithPrefix oldTree,
                                                            ElasticQuotaNodeWithPrefix oldNode,
                                                            ElasticQuotaNodeWithPrefix newNode,
                                                            List<K8sRoleBinding> oldRoleBindings) throws Exception{ //output
        Set<String> oldQuotaNamespaces = k8sService.genRoleBindingInfoByGroups(userGroups, oldTree);
        K8sServiceAccount regeneretedK8sServiceAccount = new K8sServiceAccount();
        if(!k8sService.genK8sRoleBindingsByRolesAndNamespaces(serviceAccount, userRoles, userClusterRoles, oldQuotaNamespaces, regeneretedK8sServiceAccount)) {
            log.warn("gen k8s roleBinding for old groups failed, sa:{} namespace:{} roles:{} clusterRoles:{}",
                    serviceAccount.getMetadata().getName(), oldQuotaNamespaces, userRoles, userClusterRoles);
            return null;
        }
        List<K8sRoleBinding> regeneretedRoleBindings = regeneretedK8sServiceAccount.getRoleBindings();
        if (oldRoleBindings != null && regeneretedRoleBindings != null && oldRoleBindings.size() < regeneretedRoleBindings.size()) {
           log.warn("old rolebinding size error %d should be %d", oldRoleBindings.size(), regeneretedK8sServiceAccount.getRoleBindings().size());
        }

        // update ns for user groups
        K8sServiceAccount newK8sServiceAccount = new K8sServiceAccount();
        Set<String> newQuotaNamespaces = new HashSet<>(oldQuotaNamespaces);
        Set<String> oldNs = getAllNamespaces(oldNode, new ArrayList<>());
        Set<String> newNs = getAllNamespaces(newNode, new ArrayList<>());
        if (oldNs.equals(newNs)) {
            log.info("update user by eqtree node without namespace change ns old:{} new:{}", oldNs, newNs);
            return null;
        }
        Set<String> nsToAdd = Sets.difference(newNs, oldNs);
        Set<String> nsToDelete = Sets.difference(oldNs, newNs);
        log.info("ns to add:{} delete:{}", nsToAdd, nsToDelete);
        newQuotaNamespaces.removeAll(nsToDelete);
        if(!k8sService.genK8sRoleBindingsByRolesAndNamespaces(serviceAccount, userRoles, userClusterRoles, newQuotaNamespaces, newK8sServiceAccount)) {
            log.warn("gen k8s roleBinding for old groups failed, sa:{} namespace:{} roles:{} clusterRoles:{}",
                    serviceAccount.getMetadata().getName(), newQuotaNamespaces, userRoles, userClusterRoles);
            return null;
        }
        List<K8sRoleBinding> newRoleBindings = newK8sServiceAccount.getRoleBindings();
        if (newRoleBindings == null) {
            newRoleBindings = new ArrayList<>();
        }
        log.info("old role binding size:{}, new size:{}", oldRoleBindings == null ? 0 : oldRoleBindings.size(), newRoleBindings.size());
        return newRoleBindings;
    }

    private boolean updateUserByEqTree(ElasticQuotaTreeWithPrefix oldTree, ElasticQuotaTreeWithPrefix newTree,
                                       ElasticQuotaNodeWithPrefix oldNode, ElasticQuotaNodeWithPrefix newNode) throws Exception{
        if (oldNode == null && newNode == null) {
            log.warn("no node to change");
            return true;
        }
        // update k8s role for users binding to this quotaGroup
        String oldQuotaNameWithPrefix = null;
        String newQuotaNameWithPrefix = null;
        if (null != oldNode) {
            oldQuotaNameWithPrefix = serializeNodeName(oldNode.getPrefix(), oldNode.getName());
        }
        if (null != newNode) {
            newQuotaNameWithPrefix = serializeNodeName(newNode.getPrefix(), newNode.getName());
        }

        if (Strings.isNullOrEmpty(oldQuotaNameWithPrefix)) {
            log.info("old quota name empty may new node newNode:{}", newQuotaNameWithPrefix);
            return true;
        }

        if (newNode != null && oldNode != null && oldNode.getNamespaces().equals(newNode.getNamespaces())) {
            log.info("update node without namespace change node new:{} old:{}", newQuotaNameWithPrefix, oldQuotaNameWithPrefix);
            return true;
        }

        List<UserGroup> quotaUserGroups = userGroupService.findUserGroupsByQuotaName(oldTree, oldNode.getName(), oldNode.getPrefix(),  true);
        if (null == quotaUserGroups) {
            log.info("no user group found for quota:{}", oldQuotaNameWithPrefix);
            return true;
        }
        List<User> allUsers = userService.findUserByName(null);
        // update user roleBinding
        for (User user: allUsers) {
            if (null == user.getSpec().getGroups()) {
                log.info("user no belong to quota Node:{} user:{}", oldQuotaNameWithPrefix, user.getMetadata().getName());
                continue;
            }
            List<UserGroup> changedUserGroups = quotaUserGroups.stream().filter(x->user.getSpec().getGroups().contains(x.getMetadata().getName())).collect(Collectors.toList());
            if (changedUserGroups == null || changedUserGroups.isEmpty()) {
                log.info("user no belong to quota Node:{} user:{}", oldQuotaNameWithPrefix, user.getMetadata().getName());
                continue;
            }
            List<UserGroup> userGroups = user.getSpec().getGroups().stream().map(x->k8sService.findUesrGroupByMetaName(x, null)).collect(Collectors.toList());
            ServiceAccount sa = k8sService.findServiceAccountByUser(user);
            if (null == sa) {
                throw new Exception(String.format("user note found:%s", user.getMetadata().getName()));
            }
            List<K8sRoleBinding>  oldRoleBindings = user.getSpec().getK8sServiceAccount().getRoleBindings();

            Set<String> userRoles = userService.getRolesByUserType(user.getSpec().getApiRoles(), false);
            Set<String> userClusterRoles = userService.getRolesByUserType(user.getSpec().getApiRoles(), true);
            // regenereate rolebindings by user's groups
            List<K8sRoleBinding> newRoleBindings = regenerateUserRoleBindings(sa, userRoles, userClusterRoles, userGroups, oldTree, oldNode, newNode, oldRoleBindings);
            if (null == newRoleBindings) {
                log.warn("regenerate role bindings for user failed:{}", user.getMetadata().getName());
                return false;
            }

            if(!k8sService.updateRoleBinding(sa, newRoleBindings, oldRoleBindings, false)){
                log.warn("update role binding by quota failed");
                return false;
            }
            if (null == user.getSpec().getK8sServiceAccount()) {
                user.getSpec().setK8sServiceAccount(new K8sServiceAccount());
            }
            user.getSpec().getK8sServiceAccount().setRoleBindings(newRoleBindings);
            if(!userService.createOrReplaceUser(user)){
                log.warn("createOrReplaceUser user failed by eqTree name:{}", user.getMetadata().getName());
                return false;
            }
        }
        return true;
    }

    private boolean updateEqTreeName(ElasticQuotaNodeWithPrefix newNode, String oldNodeNameWithoutPrefix, String newNodeNameWithoutPrefix, String prefix) throws Exception{
        log.info("update node name from:{} to:{} prefix:{}", oldNodeNameWithoutPrefix, newNodeNameWithoutPrefix, prefix);
        // if addNode then add prefix for node name
        if (!oldNodeNameWithoutPrefix.equals(newNodeNameWithoutPrefix)) {
            if(!updateSubNodeName(newNode, prefix, newNodeNameWithoutPrefix)){
                log.warn("update sub node name failed prefix new:{} old:{}", prefix, newNodeNameWithoutPrefix);
                return false;
            }
        }
        return true;
    }


    private boolean updateUserAndUserGroupByTree(ElasticQuotaTreeWithPrefix oldTree, ElasticQuotaTreeWithPrefix newTree,
                                                 String action, String oldNodeName, String newNodeName, String prefix) throws Exception {
        if (action.equals(UpdateResourceType)) {
            log.info("no change to update for action:{}", UpdateResourceType);
            return true;
        }

        ElasticQuotaNodeWithPrefix oldNode = null;
        if (null != oldTree && !Strings.isNullOrEmpty(oldNodeName)) {
            ElasticQuotaNodeWithPrefix oldRoot = oldTree.getSpec().getRoot();
            oldNode = groupService.findElasticNodeByName(oldRoot, oldNodeName, prefix);
            if (oldNode == null) {
                throw new Exception(String.format("old node not found:%s", oldNodeName));
            }
        }
        ElasticQuotaNodeWithPrefix newNode = null;
        if (null != newTree && !Strings.isNullOrEmpty(newNodeName) && !action.equals(DeleteNode)) {
            ElasticQuotaNodeWithPrefix newRoot = newTree.getSpec().getRoot();
            newNode = groupService.findElasticNodeByName(newRoot, newNodeName, prefix);
            if (newNode == null) {
                throw new Exception(String.format("new node not found:%s", newNodeName));
            }
        }
        if (newNode == null && oldNode == null) {
            log.warn("no node change");
            return true;
        }

        if (newNode != null) {
            groupService.updatePipelineRBAC(newNode);
        }
        if (null != newNode && !action.equals(AddNode) &&!updateEqTreeName(newNode, oldNodeName, newNodeName, prefix)) {
            log.warn("update eqTree name failed");
            return false;
        }

        if(!updateUserByEqTree(oldTree, newTree, oldNode, newNode)){
            log.warn("update user by eqTree node:{} new:{} failed", oldNode, newNode);
            return false;
        }

        if(!updateUserGroupByEqTree(oldTree, newTree, oldNode, newNode)) {
            log.warn("update user group by eqTree node:{} new:{} failed", oldNode, newNode);
            return false;
        }

        return true;
    }

    @PutMapping("update")
    public RequestResult<Void> updateGroup(@RequestBody ElasticQuotaTreeWithPrefix tree,
                                           @RequestParam(name="oldNodeName", required = true) String oldNodeName,
                                           @RequestParam(name="action", required = true) String action,
                                           @RequestParam(name="prefix", required = true) String prefix,
                                           @RequestParam(name="newNodeName", required = true) String newNodeName) {
        log.info("update action:{} prefix:{} oldNode:{} newNode:{} tree: {}", action, prefix, oldNodeName, newNodeName, JSON.toJSONString(tree));
        RequestResult<Void> result = new RequestResult<>();
        try {
            String treeName = tree.getMetadata().getName();
            ElasticQuotaTreeWithPrefix oldTree = groupService.getElasticQuotaTree(treeName, null);
            if (oldTree == null) {
                throw new Exception(String.format("elastic quota tree not found name:%s", treeName));
            }
            if(!this.preUpdateCheck(oldTree, tree, action, oldNodeName, newNodeName, prefix)){
                log.warn("pre update check failed");
                throw new Exception(String.format("update check failed"));
            }
            serializeTree(tree.getSpec().getRoot());
            if (action.equals(AddNode)) {
                oldNodeName = null;
            }

            if(!this.updateUserAndUserGroupByTree(oldTree, tree, action, oldNodeName, newNodeName, prefix)){
                log.warn("update tree node name failed");
                throw new Exception(String.format("update tree node name failed"));
            }
            boolean success = groupService.createOrReplaceElasticQuotaTree(tree);
            if (!success) {
                result.setFailed(ResultCode.UPDATE_GROUP_FAILED, "更新配额组失败");
            }
            if (getKubernetesVersion() < 20) {
                success = groupService.createResourceQuotaFromElasticQuotaTree(tree);
                if (!success) {
                    result.setFailed(ResultCode.CREATE_GROUP_FAILED, "创建ResourceQuota失败");
                }
            }

        } catch (K8sCRDNotFoundException e) {
            log.warn("elastic quota crd not found exception:{}", e.getMessage());
            result.setFailed(ResultCode.GROUP_CRD_NOT_FOUND_EXCEPTION, ResultCode.GROUP_CRD_NOT_FOUND_MESSAGE);
        } catch (Exception e) {
            log.error("update group exception", e);
            result.setFailed(ResultCode.UPDATE_GROUP_FAILED, e.getMessage());
        }

        log.info("update group result: {}", JSON.toJSONString(result));
        return result;
    }

    private Set<String> getAllNamespaces(ElasticQuotaNodeWithPrefix root, List<String> res) {
        Set<String> resSet = new HashSet<>();
        if (root == null) {
            resSet.addAll(res);
            return resSet;
        }
        if (null != root.getNamespaces() && !root.getNamespaces().isEmpty()) {
            res.addAll(root.getNamespaces());
        }
        if (null != root.getChildren()) {
            for (ElasticQuotaNodeWithPrefix child : root.getChildren()) {
                getAllNamespaces(child, res);
            }
        }
        return resSet;
    }

    private boolean preCreateCheck(ElasticQuotaNodeWithPrefix rootNode) throws Exception {
        List<String> namespaces = new ArrayList<>();
        getAllNamespaces(rootNode, namespaces);
        if(!isNamespaceNoRunningPod(namespaces)) {
            return false;
        }
        return validateQuotaInTree(rootNode);
    }

    private boolean addNodeCheck(ElasticQuotaTreeWithPrefix newTree, String changedNodeName, String prefix) throws Exception {
        List<String> addNamespaces = groupService.findNamespaceByChangedNode(newTree, changedNodeName, prefix);
        log.info("add nodeName:{} namespaced:{}", changedNodeName, addNamespaces);
        if (null == addNamespaces || addNamespaces.isEmpty()) {
            return true;
        }

        if (!addNamespaces.isEmpty()) {
            return isNamespaceNoRunningPod(addNamespaces);
        }
        return true;
    }

    private boolean deleteNodeCheck(ElasticQuotaTreeWithPrefix oldTree, String changedNodeName, String changedNodePrefix) throws Exception {
        List<String> deleteNamespaces = groupService.findNamespaceByChangedNode(oldTree, changedNodeName, changedNodePrefix);
        log.info("delete root:{} namespaced:{}", changedNodeName, deleteNamespaces);
        if (deleteNamespaces.isEmpty()) {
            return true;
        }
        return isNamespaceNoRunningPod(deleteNamespaces);
    }

    private boolean updateResourceTypeCheck(ElasticQuotaTreeWithPrefix tree, String changedNodeName) throws Exception {
        return true;
    }

    private boolean isNamespaceNoRunningPod(List<String> namespaces) throws Exception {
        for (String ns : namespaces) {
            List<Pod> allPods = k8sService.getPod(ns);
            log.info("ns:{} pod size:{}", ns, allPods.size());
            for (Pod pod : allPods) {
                String podPhase = pod.getStatus().getPhase();
                if (podPhase.equals(PodFailed) || podPhase.equals(PodPending) || podPhase.equals(PodSucceeded) || podPhase.equals(PodUnknown)) {
                    continue;
                }
                throw new Exception(String.format("namespace:%s pod:%s in running", ns, pod.getMetadata().getName()));
            }
        }
        return true;
    }

    private String htmlMessage(String var) {
       return String.format("<strong><i>%s</i></strong><br>", var);
    }

    private boolean validateQuotaInTree(ElasticQuotaNodeWithPrefix rootNode) throws Exception {
        if (rootNode == null) {
            return true;
        }
        String noteName = rootNode.getName();
        String notePrefix = rootNode.getPrefix();
        Set<String> maxResourceTypes = rootNode.getMax().keySet();
        Set<String> minResourceTypes = rootNode.getMin().keySet();
        for (String resourceType : minResourceTypes) {
            String rootMinStr = rootNode.getMin().get(resourceType);
            String rootMaxStr = rootNode.getMax().get(resourceType);
            Double rootMin = K8sUtil.parseResourceToNumber(rootMinStr);
            Double rootMax = K8sUtil.parseResourceToNumber(rootMaxStr);
            if (rootMin > rootMax) {
                throw new Exception(String.format("<br>node: %sresource: %swant: %sgiven: %s", htmlMessage(notePrefix + "." + noteName),
                        htmlMessage(resourceType), htmlMessage("min&lt;max"), htmlMessage(String.format("%s&gt;%s", rootMinStr, rootMaxStr))));
            }
        }

        List<ElasticQuotaNodeWithPrefix> childs = rootNode.getChildren();
        if (childs != null && !childs.isEmpty()) {
            for (String resourceType : minResourceTypes) {
                String rootMinStr = rootNode.getMin().get(resourceType);
                Double rootMin = K8sUtil.parseResourceToNumber(rootMinStr);
                Double sumOfChildsMin = childs.stream().map(x -> K8sUtil.parseResourceToNumber(x.getMin().get(resourceType))).collect(Collectors.reducing(0.0D, (x, y) -> x + y));
                Quantity q = new QuantityBuilder().withAmount(String.format("%.1f", sumOfChildsMin)).build();
                if (sumOfChildsMin > rootMin) {
                    String humanReadableQuota = sumOfChildsMin >= MAX_QUOTA ? UI_MAX_QUOTA : q.toString();
                    throw new Exception(String.format("<br>node: %sresource: %swant: %sgiven: %s", htmlMessage(noteName),
                            htmlMessage(resourceType), htmlMessage("min&lt;sum(child.min)"), htmlMessage(String.format("%s&gt;%s", humanReadableQuota, rootMinStr))));
                }
            }

            for (String resourceType : maxResourceTypes) {
                String rootMaxStr = rootNode.getMax().get(resourceType);
                Double rootMax = K8sUtil.parseResourceToNumber(rootMaxStr);
                Double maxOfChildsMax = childs.stream().map(x -> K8sUtil.parseResourceToNumber(x.getMax().get(resourceType))).collect(Collectors.reducing(0.0D, (x, y) -> max(x, y)));
                Quantity q = new QuantityBuilder().withAmount(String.format("%.1f", maxOfChildsMax)).build();
                if (maxOfChildsMax > rootMax && rootMax < MAX_QUOTA) {
                    String humanReadableQuota = maxOfChildsMax >= MAX_QUOTA ? UI_MAX_QUOTA : q.toString();
                    throw new Exception(String.format("<br>node: %sresource: %swant: %sgiven: %s", htmlMessage(noteName), htmlMessage(resourceType),
                            htmlMessage("max&lt;max(child.max)"), htmlMessage(String.format("%s&gt;%s", humanReadableQuota, rootMaxStr))));
                }
            }

            for (ElasticQuotaNodeWithPrefix child : childs) {
                if (!validateQuotaInTree(child)) {
                    return false;
                }
            }
        }


        return true;
    }

    private boolean updateNodeCheck(ElasticQuotaTreeWithPrefix oldTree, ElasticQuotaTreeWithPrefix newTree, String oldNodeName, String newNodeName, String prefix) throws Exception {
        log.info("update node old name:{} new:{}", oldNodeName, newNodeName);
        List<String> oldNamespaces = groupService.findNamespaceByChangedNode(oldTree, oldNodeName, prefix);
        List<String> newNamespaces = groupService.findNamespaceByChangedNode(newTree, newNodeName, prefix);
        List<String> addNamespaces = new ArrayList<>();
        List<String> deleteNamespaces = new ArrayList<>();
        if (null != newNamespaces) {
            addNamespaces = newNamespaces;
        }
        if (null != oldNamespaces) {
            deleteNamespaces = oldNamespaces;
        }
        if (null != oldNamespaces) {
            addNamespaces = addNamespaces.stream().filter(x -> oldNamespaces.indexOf(x) < 0).collect(Collectors.toList());
        }
        if (null != newNamespaces) {
            deleteNamespaces = deleteNamespaces.stream().filter(x -> newNamespaces.indexOf(x) < 0).collect(Collectors.toList());
        }
        log.info("update namespaced add:{} delete:{}", addNamespaces, deleteNamespaces);
        if ((null == addNamespaces || addNamespaces.isEmpty()) && (null == deleteNamespaces || deleteNamespaces.isEmpty())) {
            return true;
        }

        if (!addNamespaces.isEmpty()) {
            isNamespaceNoRunningPod(addNamespaces);
        }

        if (!deleteNamespaces.isEmpty()) {
            isNamespaceNoRunningPod(deleteNamespaces);
        }

        return true;
    }

    private boolean preUpdateCheck(ElasticQuotaTreeWithPrefix oldTree, ElasticQuotaTreeWithPrefix tree, String action, String oldNodeName, String newNodeName, String prefix) throws Exception {
        if (action.equals(UpdateNode)) {
            if(!validateQuotaInTree(tree.getSpec().getRoot()))  {
                return false;
            }
            return this.updateNodeCheck(oldTree, tree, oldNodeName, newNodeName, prefix);
        } else if (action.equals(DeleteNode)) {
            return this.deleteNodeCheck(oldTree, oldNodeName, prefix);
        } else if (action.equals(AddNode)) {
            if(!validateQuotaInTree(tree.getSpec().getRoot()))  {
                return false;
            }
            return this.addNodeCheck(tree, newNodeName, prefix);
        } else if (action.equals(UpdateResourceType)) {
            if (!validateQuotaInTree(tree.getSpec().getRoot())) {
                return false;
            }
            return this.updateResourceTypeCheck(tree, oldNodeName);
        } else {
            throw new Exception(String.format("unknown action"));
        }
    }


    @PutMapping("delete")
    public RequestResult<Void> deleteGroup(@RequestParam(name = "name", required = false) String name,
                                           @RequestParam(name = "namespace", required = false) String namespace) {
        log.info("delete group name: {}", name);
        RequestResult<Void> result = new RequestResult<>();
        try {
            ElasticQuotaTreeWithPrefix tree = this.groupService.getElasticQuotaTree(name, namespace);
            if (tree == null) {
                throw new Exception("elastic quota tree not found");
            }
            String rootName = tree.getSpec().getRoot().getName();
            String rootPrefix = tree.getSpec().getRoot().getPrefix();
            this.deleteNodeCheck(tree, rootName, rootPrefix);
        } catch (K8sCRDNotFoundException e) {
            log.warn("elastic quota crd not found exception:{}", e.getMessage());
            result.setFailed(ResultCode.GROUP_CRD_NOT_FOUND_EXCEPTION, ResultCode.GROUP_CRD_NOT_FOUND_MESSAGE);
        } catch (Exception e) {
            result.setFailed(ResultCode.GROUP_REQUSET_INVALID, e.getMessage());
            return result;
        }

        try {
            boolean success = groupService.deleteElasticQuotaTree(namespace, name);
            if (!success) {
                result.setFailed(ResultCode.DELETE_GROUP_FAILED, "删除配额组失败");
            }
            if (getKubernetesVersion() < 20) {
                success = groupService.deleteResourceQuota(namespace, name.toLowerCase());
                if (!success) {
                    result.setFailed(ResultCode.DELETE_GROUP_FAILED, "删除ResourceQuota失败");
                }
            }
        } catch (Exception e) {
            result.setFailed(ResultCode.DELETE_GROUP_EXCEPTION, "删除配额组异常");
        }

        log.info("delete group result: {}", JSON.toJSONString(result));
        return result;

    }


}
