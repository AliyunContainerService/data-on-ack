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
import com.aliyun.kubeai.dao.K8sUserDao;
import com.aliyun.kubeai.dao.K8sUserGroupDao;
import com.aliyun.kubeai.entity.K8sNamespace;
import com.aliyun.kubeai.entity.K8sPvc;
import com.aliyun.kubeai.entity.K8sSecret;
import com.aliyun.kubeai.model.k8s.UserGroup;
import com.aliyun.kubeai.model.k8s.eqtree.ElasticQuotaTreeWithPrefix;
import com.aliyun.kubeai.model.k8s.user.K8sRoleBinding;
import com.aliyun.kubeai.model.k8s.user.K8sServiceAccount;
import com.aliyun.kubeai.model.k8s.user.User;
import com.google.common.base.Strings;
import io.fabric8.kubernetes.api.model.*;
import io.fabric8.kubernetes.api.model.rbac.*;
import io.fabric8.kubernetes.client.KubernetesClient;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.io.InputStream;
import java.util.*;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;
import java.util.stream.Collectors;

import static com.aliyun.kubeai.utils.DateUtil.transUTCTime;
import static com.aliyun.kubeai.utils.K8sUtil.k8sVersion;

@Slf4j
@Service
public class K8sService {
    public static final String DEFAULT_USER_NAMESPACE = "kube-ai";
    public static final String DEFAULT_USER_GROUP_NAMESPACE = "kube-ai";
    public static Map<String, String> roleToDefination = new HashMap<>();
    public static final String ADMIN_DEFAULT_CLUSTER_ROLE_FILE = "defaultAdminClusterRole.yaml";
    public static final String DEFAULT_CLUSTER_ROLE = "kubeai-researcher-clusterrole";
    public static final String DEFAULT_CLUSTER_ROLE_FILE = "defaultResearcherClusterRole.yaml";
    public static final String DEFAULT_ROLE = "kubeai-researcher-role";
    public static final String DEFAULT_ROLE_FILE = "defaultResearcherRole.yaml";
    public static final String ADMIN_DEFAULT_CLUSTER_ROLE = "kubeai-admin-clusterrole";

    @Resource
    private KubeClient kubeClient;

    @Autowired
    K8sUserGroupDao userGroupDao;

    @Autowired
    K8sUserDao userDao;

    @Resource
    QuotaGroupService quotaGroupService;

    public K8sService() {
        if (roleToDefination.isEmpty()) {
            roleToDefination.put(ADMIN_DEFAULT_CLUSTER_ROLE, ADMIN_DEFAULT_CLUSTER_ROLE_FILE);
            roleToDefination.put(DEFAULT_CLUSTER_ROLE, DEFAULT_CLUSTER_ROLE_FILE);
            roleToDefination.put(DEFAULT_ROLE, DEFAULT_ROLE_FILE);
        }
    }

    public String getK8sVersion() {
        return k8sVersion(kubeClient);
    }

    private K8sPvc parseK8sPvcToPvc(PersistentVolumeClaim k8sPvc) {
        K8sPvc pvc = new K8sPvc();
        pvc.setName(k8sPvc.getMetadata().getName());
        pvc.setNamespace(k8sPvc.getMetadata().getNamespace());
        pvc.setStatus(k8sPvc.getStatus().getPhase());
        pvc.setCreateTime(transUTCTime(k8sPvc.getMetadata().getCreationTimestamp()));
        pvc.setUpdateTime(transUTCTime(k8sPvc.getMetadata().getDeletionTimestamp()));
        return pvc;
    }

    public String genRoleBindingName(String serviceAccountName, String roleName, String namespace) {
        boolean isClusterRole = Strings.isNullOrEmpty(namespace);
        String roleBindingName = isClusterRole ? String.format("%s:clusterRole:%s", serviceAccountName, roleName) :
                String.format("%s:%s:%s", serviceAccountName, namespace, roleName);
        return roleBindingName;
    }


    // isClusterRole != quotaNamespace.isEmpty(), because need to gen empty role bindings for empty quotaNamespaces when add
    public List<K8sRoleBinding> genK8sRoleBindings(List<String> roles, ServiceAccount sa, Set<String> quotaNamespaces, boolean isClusterRole) throws Exception{
        if (roles == null || roles.isEmpty()) {
            return null;
        }
        if (!isClusterRole && (null == quotaNamespaces || quotaNamespaces.isEmpty())) {
            return null;
        }
        String saName = sa.getMetadata().getName();
        List<K8sRoleBinding> foundRoleBindings = new ArrayList<>();
        if (isClusterRole) {
            for (String role : roles) {
                K8sRoleBinding k8sRoleBinding = new K8sRoleBinding();
                k8sRoleBinding.setRoleName(role);
                k8sRoleBinding.setName(genRoleBindingName(saName, role, null));
                foundRoleBindings.add(k8sRoleBinding);
            }
            return foundRoleBindings;
        }
        for (String role: roles) {
            for (String roleNamespace : quotaNamespaces) {
                K8sRoleBinding k8sRoleBinding = new K8sRoleBinding();
                k8sRoleBinding.setRoleName(role);
                k8sRoleBinding.setNamespace(roleNamespace);
                k8sRoleBinding.setName(genRoleBindingName(saName, role, roleNamespace));
                foundRoleBindings.add(k8sRoleBinding);
            }
        }
        return foundRoleBindings;
    }


    // @params user for change user.spec.k8sRolebinding
    public boolean genK8sRoleBindingsByRolesAndNamespaces(ServiceAccount serviceAccount,
                                                          Set<String> roles,
                                                          Set<String> clusterRoles,
                                                          Set<String> quotaNamespaces,
                                                          K8sServiceAccount k8sServiceAccount// output
                                       ) throws Exception{
        if (k8sServiceAccount == null) {
            k8sServiceAccount = new K8sServiceAccount();
        }
        String saName = serviceAccount.getMetadata().getName();
        log.info("update role binding for sa:{} namespace:{} roles:{} clusterroles:{}",
                saName, quotaNamespaces, roles, clusterRoles);
        //boolean isNamespaceChanged = !oldQuotaNamespaces.equals(newQuotaNamespaces);
        //boolean isRoleChanged = !oldRoles.equals(newRoles);
        //boolean isClusterRoleChanged = !oldClusterRoles.equals(newClusterRoles);
        //if (!isNamespaceChanged && !isRoleChanged && !isClusterRoleChanged) {
        //    log.warn("role bindings no change");
        //    return true;
        //}

        // delete role binding
        List<K8sRoleBinding> rolebindings = new ArrayList<>();
        if (null != roles) {
            rolebindings = genK8sRoleBindings(new ArrayList<>(roles), serviceAccount, quotaNamespaces, false);
        }
        k8sServiceAccount.setRoleBindings(rolebindings);

        List<K8sRoleBinding> clusterRoleBindings = null;
        if (null != clusterRoles) {
            clusterRoleBindings = genK8sRoleBindings(new ArrayList<>(clusterRoles), serviceAccount, null, true);
        }
        k8sServiceAccount.setClusterRoleBindings(clusterRoleBindings);

        return true;
    }

    public Set<String> genRoleBindingInfoByGroups(List<UserGroup> groups, ElasticQuotaTreeWithPrefix tree) throws Exception{
        Set<String> quotaNamespaces = new HashSet<>();
        if (null == groups || groups.isEmpty()) {
            return quotaNamespaces;
        }
        Set<String> groupQuotaNamespaces = null;
        for (UserGroup group : groups) {
            //TODO
            log.info("find group's namespace name:{}", JSON.toJSONString(group));
            if (tree != null) {
                groupQuotaNamespaces = quotaGroupService.getQuotaNamespaceByNameInTree(tree, group.getSpec().getQuotaNames());
            } else {
                groupQuotaNamespaces = quotaGroupService.getQuotaNamespacesByName(null, group.getSpec().getQuotaNames());
            }
            if (null != groupQuotaNamespaces) {
                quotaNamespaces.addAll(groupQuotaNamespaces);
            }
        }
        return quotaNamespaces;
    }

    public List<User> listUserByGroupMetaName(String userGroupName) {
        if(Strings.isNullOrEmpty(userGroupName)) {
            return null;
        }
        List<User> allUsers = userDao.findUserByName(null, null, false);
        return allUsers.stream()
                .filter(x->x.getSpec().getGroups()!=null)
                .filter(x->x.getSpec().getGroups().contains(userGroupName))
                .collect(Collectors.toList());
    }

    public User findUserByName(String userName) {
        if (Strings.isNullOrEmpty(userName)) {
            return null;
        }
        List<User> allUsers = userDao.findUserByName(userName, null, true);
        if (null == allUsers || allUsers.isEmpty()) {
            return null;
        }
        if (allUsers.size() > 1) {
            log.warn("user name duplicated name:{} found user size:{}", userName, allUsers.size());
        }
        return allUsers.get(0);
    }

    public String genServiceAccountName(String userName, String uid) {
        return String.format("%s", uid); //only uid for service account name don't changing with user name
    }

    public Object findRoleByName(String roleName, boolean isClusterRole, String namespace) {
        Object ret;
        if (isClusterRole) {
            ret = kubeClient.getClient().rbac().clusterRoles().withName(roleName).get();
        } else {
            if (Strings.isNullOrEmpty(roleName) || Strings.isNullOrEmpty(namespace)) {
                return null;
            }
            ret = kubeClient.getClient().rbac().roles().inNamespace(namespace).withName(roleName).get();
        }
        return ret;
    }

    public boolean deleteRoleBinding(String name, boolean isClusterRole, String namespace) {
        boolean res;
        // TODO if role or cluster role no binding then delete it
        if (isClusterRole) {
            res = kubeClient.getClient().rbac().clusterRoleBindings().withName(name).delete();
        } else {
            res = kubeClient.getClient().rbac().roleBindings().inNamespace(namespace).withName(name).delete();
        }
        return res;
    }

    public Object findRoleBindingByName(String name, String namespace, boolean isClusterRole) {
        log.info("find role binding by name:{} namespace:{} isClusterRole:{}", name, namespace, isClusterRole);
        try {
            if (isClusterRole) {
                return kubeClient.getClient().rbac().clusterRoleBindings().withName(name).get();
            }
            if (Strings.isNullOrEmpty(namespace)) {
                RoleBindingList roleBindingList = kubeClient.getClient().rbac().roleBindings().inAnyNamespace().list();
                for (RoleBinding roleBinding : roleBindingList.getItems()) {
                    if (roleBinding.getMetadata().getName().equals(name)) {
                        return roleBinding;
                    }
                }
            } else {
                return kubeClient.getClient().rbac().roleBindings().inNamespace(namespace).withName(name).get();
            }
        } catch (Exception e) {
            log.warn("find role binding by name error", e);
            return null;
        }
        return null;
    }


    public Object createRole(String roleFileName, String roleName, boolean isClusterRole, String roleNamespace) {
        log.info("create role name:{} yaml file:{} namespace:{} isClusterRole:{}", roleName, roleFileName, roleNamespace, isClusterRole);
        Object res = null;
        try {
            InputStream resource = new ClassPathResource(roleFileName).getInputStream();
            Object cr = null;
            String roleNameFromYaml = null;
            if (isClusterRole) {
                ClusterRole clusterRole = kubeClient.getClient().rbac().clusterRoles().load(resource).get();
                roleNameFromYaml = clusterRole.getMetadata().getName();
                cr = clusterRole;
            } else {
                Role role = kubeClient.getClient().rbac().roles().load(resource).get();
                if (Strings.isNullOrEmpty(roleNamespace)) {
                    log.warn("create role with namespace empty");
                    return null;
                }
                ObjectMeta objectMeta = new ObjectMetaBuilder()
                        .withName(role.getMetadata().getName())
                        .withNamespace(roleNamespace).build();
                role.setMetadata(objectMeta);
                roleNameFromYaml = role.getMetadata().getName();
                cr = role;
            }
            if (cr == null || !roleNameFromYaml.equals(roleName)) {
                log.warn("role name not match:{} yaml:{} isCluster:{}", roleName, roleNameFromYaml, isClusterRole);
                return null;
            }
            if (isClusterRole) {
                res = kubeClient.getClient().rbac().clusterRoles().create((ClusterRole) cr);
            } else {
                //BUGFIX:operation namespace not always null, https://github.com/fabric8io/kubernetes-client/issues/1835
                res = kubeClient.getClient().rbac().roles().inNamespace(roleNamespace).create((Role) cr);
            }
        } catch (Exception e) {
            log.error("create role exception", e);
            return null;
        }
        return res;
    }


    public Object bindRole(ServiceAccount serviceAccount, String roleName, boolean isClusterRole, String roleNamespace) {
        log.info("bind role:{} to serviceAccount:{} in namespace:{} isCluster:{}", roleName, serviceAccount.getMetadata().getName(), roleNamespace, isClusterRole);
        if (Strings.isNullOrEmpty(roleName) || serviceAccount == null) {
            log.warn("bind role with empty service account or role");
            return null;
        }
        if (!isClusterRole && Strings.isNullOrEmpty(roleNamespace)) {
            log.warn("bind role with empty namespace");
            return null;
        }
        Object res;
        String userNamespace = serviceAccount.getMetadata().getNamespace();
        String userName = serviceAccount.getMetadata().getName();
        String roleKind = isClusterRole ? "ClusterRole" : "Role";
        RoleRef roleRef = new RoleRefBuilder().withApiGroup("rbac.authorization.k8s.io")
                .withKind(roleKind)
                .withName(roleName).build(); // TODO need set namespace here if role namespace and roleBinding namespace not equal?
        String roleBindingName = genRoleBindingName(userName, roleName, roleNamespace);
        ObjectMeta metaData = null;
        if (isClusterRole) {
            metaData = new ObjectMetaBuilder().withName(roleBindingName).build();
        } else {
            metaData = new ObjectMetaBuilder().withName(roleBindingName).withNamespace(roleNamespace).build();
        }
        Subject subject = new SubjectBuilder().withKind("ServiceAccount").withName(userName).withNamespace(userNamespace).build();
        String roleBindingKind = isClusterRole ? "ClusterRoleBinding" : "RoleBinding";
        if (isClusterRole) {
            ClusterRoleBinding clusterRoleBinding = new ClusterRoleBindingBuilder()
                    .withKind(roleBindingKind)
                    .withApiVersion("rbac.authorization.k8s.io/v1")
                    .withMetadata(metaData)
                    .withSubjects(subject)
                    .withRoleRef(roleRef).build();
            res = kubeClient.getClient().rbac().clusterRoleBindings().createOrReplace(clusterRoleBinding);
        } else {
            RoleBinding roleBinding = new RoleBindingBuilder()
                    .withKind(roleBindingKind)
                    .withApiVersion("rbac.authorization.k8s.io/v1")
                    .withMetadata(metaData)
                    .withSubjects(subject)
                    .withRoleRef(roleRef).build();
            res = kubeClient.getClient().rbac().roleBindings().inNamespace(roleNamespace).createOrReplace(roleBinding);
        }
        log.info("create role bindRes:{}", JSON.toJSONString(res));

        return res;
    }

    public boolean updateRoleBinding(ServiceAccount serviceAccount, List<K8sRoleBinding> roleBindingConfigs,
                                     List<K8sRoleBinding> foundRoleBindingConfigs, boolean isClusterRoleBinding) {
        if (null == serviceAccount) {
            log.warn("update role binding with sa empty");
            return false;
        }

        if (null != roleBindingConfigs) {
            for (K8sRoleBinding roleBindingConfig : roleBindingConfigs) {
                if (Strings.isNullOrEmpty(roleBindingConfig.getRoleName())) {
                    log.warn("role binding name empty isCluster:{}", isClusterRoleBinding);
                    return false;
                }
                if (!isClusterRoleBinding && Strings.isNullOrEmpty(roleBindingConfig.getNamespace())) {
                    log.warn("role binding namespace empty isCluster:{}", isClusterRoleBinding);
                    return false;
                }
                if (Strings.isNullOrEmpty(roleBindingConfig.getName())) {
                    roleBindingConfig.setName(genRoleBindingName(serviceAccount.getMetadata().getName(), roleBindingConfig.getRoleName(), roleBindingConfig.getNamespace()));
                }
            }
        }

        String serviceAccountName = serviceAccount.getMetadata().getName();
        // gen role binding name

        Map<String, ObjectMeta> oldRoleBindingIndex = null;
        if (null != foundRoleBindingConfigs) {
            oldRoleBindingIndex = foundRoleBindingConfigs.stream().distinct().collect(Collectors.toMap(ObjectMeta::getName, Function.identity()));
        }
        Map<String, ObjectMeta> newRoleBindingIndex = null;
        if (null != roleBindingConfigs) {
            newRoleBindingIndex = roleBindingConfigs.stream().distinct().collect(Collectors.toMap(ObjectMeta::getName, Function.identity()));
        }
        if (null != roleBindingConfigs) {
            for (K8sRoleBinding roleBindingConfig : roleBindingConfigs) {
                String roleBindingName = roleBindingConfig.getName();
                //if (null != oldRoleBindingIndex && oldRoleBindingIndex.containsKey(roleBindingName)) {
                //    log.debug("rolebinding already exist name:{}", roleBindingName);
                //    continue;
                //}
                //add namespace
                String roleName = roleBindingConfig.getRoleName();
                String roleNamespace = roleBindingConfig.getNamespace();
                String roleFileName = roleToDefination.getOrDefault(roleName, null);
                if (Strings.isNullOrEmpty(roleFileName)) {
                    log.warn("role file not found name:{} isCluster:{}", roleName, isClusterRoleBinding);
                    return false;
                }
                if (null == findRoleByName(roleName, isClusterRoleBinding, roleNamespace)) {
                    log.info("role not exist roleName:{} namespace:{} isClusterRole:{}", roleName, roleNamespace, isClusterRoleBinding);
                    if (null == createRole(roleFileName, roleName, isClusterRoleBinding, roleNamespace)) {
                        log.warn("create cluster role:{} in namespace:{} failed", roleName, roleNamespace);
                        return false;
                    }
                }
                Object roleBinding = bindRole(serviceAccount, roleName, isClusterRoleBinding, roleNamespace);
                if (null == roleBinding) {
                    log.warn("bind role:{} to user:{} failed in namespace:{} isClusterRole:{}", roleName, serviceAccountName, roleNamespace, isClusterRoleBinding);
                    return false;
                }
            }
        }
        if (null != foundRoleBindingConfigs) {
            for (K8sRoleBinding roleBindingConfig : foundRoleBindingConfigs) {
                String roleBindingName = roleBindingConfig.getName();
                String namespace = roleBindingConfig.getNamespace();
                if (null != newRoleBindingIndex && newRoleBindingIndex.containsKey(roleBindingName)) {
                    log.debug("rolebinding to reserve name:{}", roleBindingName);
                    continue;
                }
                //remove role binding
                if (null == findRoleBindingByName(roleBindingName, namespace, isClusterRoleBinding)) {
                    log.info("rolebinding already been deleted name:{} namespace:{}", roleBindingName, namespace);
                    continue;
                }
                if (!deleteRoleBinding(roleBindingConfig.getName(), isClusterRoleBinding, namespace)) {
                    log.warn("delete roleBinding:{} in namespace:{} for user:{} failed", roleBindingName, namespace, serviceAccountName);
                    return false;
                }
                //TODO remove role not refered by any binding
            }
        }
        return true;
    }

    public UserGroup findUesrGroupByMetaName(String userGroupMetaName, String userGroupNamespace) {
        if (Strings.isNullOrEmpty(userGroupMetaName)) {
            log.warn("find userGroup by empty metaName");
            return null;
        }
        if (Strings.isNullOrEmpty(userGroupNamespace)) {
            userGroupNamespace = DEFAULT_USER_GROUP_NAMESPACE;
        }
        return userGroupDao.getUserGroupByMetaName(userGroupMetaName, userGroupNamespace);
    }

    public UserGroup findUserGroupByName(String userGroupname, String userGroupNamespace) {
        if (Strings.isNullOrEmpty(userGroupNamespace)) {
            userGroupNamespace = DEFAULT_USER_GROUP_NAMESPACE;
        }
        List<UserGroup> res = userGroupDao.listUserGroupByGroupName(userGroupname, userGroupNamespace, true);
        if (res != null && !res.isEmpty()) {
            if (res.size() > 1) {
                log.warn("user group name duplicated");
            }
            return res.get(0);
        }

        return null;
    }

    public ServiceAccount findServiceAccountByUser(User user) throws Exception {
        String saName = genServiceAccountName(user.getSpec().getUserName(), user.getSpec().getUserId());
        String saNamespace = user.getSpec().getK8sServiceAccount().getNamespace();
        if (Strings.isNullOrEmpty(saNamespace)) {
            saNamespace = DEFAULT_USER_NAMESPACE;
        }
        ServiceAccount serviceAccount = null;
        try {
            serviceAccount = kubeClient.getClient().serviceAccounts().inNamespace(saNamespace).withName(saName).get();
        } catch (Exception e) {
            log.info("get service account failed, wait 1 sec:", e.getMessage());
            TimeUnit.SECONDS.sleep(1);  // k8s create sc in async, so wait for 1s here
            serviceAccount = kubeClient.getClient().serviceAccounts().inNamespace(saName).withName(saName).get();
        }
        return serviceAccount;
    }

    public ServiceAccount getServiceAccount(String saName, String saNamespace) throws Exception {
        ServiceAccount serviceAccount = null;
        try {
            serviceAccount = kubeClient.getClient().serviceAccounts().inNamespace(saNamespace).withName(saName).get();
        } catch (Exception e) {
            log.info("get service account failed, wait 1 sec:", e.getMessage());
            TimeUnit.SECONDS.sleep(1);  // k8s create sc in async, so wait for 1s here
            try {
                serviceAccount = kubeClient.getClient().serviceAccounts().inNamespace(saNamespace).withName(saName).get();
            } catch (Exception e1) {
                log.info("fail to get service accout:" + e1.getMessage());
            }


        }
        return serviceAccount;
    }

    private K8sSecret parseK8sSecretToSecret(Secret k8sSecret) {
        K8sSecret secret = new K8sSecret();
        secret.setName(k8sSecret.getMetadata().getName());
        secret.setNamespace(k8sSecret.getMetadata().getNamespace());
        List<String> keys = new ArrayList<>();
        if (k8sSecret.getData() != null) {
            keys = k8sSecret.getData().entrySet().stream().map(x -> x.getKey()).collect(Collectors.toList());
        }
        secret.setKeys(keys);
        return secret;
    }

    private K8sNamespace parseK8sNamespace(Namespace namespace) {
        K8sNamespace resNamespace = new K8sNamespace();
        resNamespace.setName(namespace.getMetadata().getName());
        return resNamespace;
    }

    public List<K8sNamespace> getNamespaceList(String name) {
        log.info("get namespace by:{}", name);
        List<K8sNamespace> resList = new ArrayList<>();
        KubernetesClient client = kubeClient.getClient();
        if (!Strings.isNullOrEmpty(name)) {
            Namespace namespace = client.namespaces().withName(name).get();
            if (namespace != null) {
                K8sNamespace np = parseK8sNamespace(namespace);
                resList.add(np);
            }
            return resList;
        }
        NamespaceList namespaceList = client.namespaces().list();
        for (Namespace np : namespaceList.getItems()) {
            K8sNamespace tmp = new K8sNamespace();
            tmp.setName(np.getMetadata().getName());
            resList.add(tmp);
        }
        log.info("got namespace:{}", resList);
        return resList;
    }

    public List<K8sPvc> getPvcList(String name, String namespace, boolean isStrictMatch) {
        KubernetesClient client = kubeClient.getClient();
        PersistentVolumeClaimList pvcList = null;
        PersistentVolumeClaim targetPvc = null;
        List<K8sPvc> res = new ArrayList<>();
        if (!Strings.isNullOrEmpty(name) && !Strings.isNullOrEmpty(namespace) && isStrictMatch) {
            targetPvc = client.persistentVolumeClaims().inNamespace(namespace).withName(name).get();
            if (targetPvc != null) {
                K8sPvc resPvc = parseK8sPvcToPvc(targetPvc);
                res.add(resPvc);
            }
            return res;
        }

        if (!Strings.isNullOrEmpty(namespace)) {
            pvcList = client.persistentVolumeClaims().inNamespace(namespace).list();
        } else {
            pvcList = client.persistentVolumeClaims().inAnyNamespace().list();
        }

        for (PersistentVolumeClaim pvcItem : pvcList.getItems()) {
            if (!Strings.isNullOrEmpty(name)) {
                if (!pvcItem.getMetadata().getName().contains(name)) {
                    continue;
                }
                if (isStrictMatch) {
                    if (!pvcItem.getMetadata().getName().equals(name)) {
                        continue;
                    }
                }
            }
            K8sPvc tmpPvc = parseK8sPvcToPvc(pvcItem);
            res.add(tmpPvc);
        }
        log.info("pvc parsed:{}", res);
        return res;
    }

    public List<Pod> getPod(String namespace) {
        KubernetesClient client = kubeClient.getClient();
        PodList podList = null;
        if (Strings.isNullOrEmpty(namespace)) {
            podList = client.pods().inAnyNamespace().list();
        } else {
            podList = client.pods().inNamespace(namespace).list();
        }
        return podList.getItems();
    }

    public List<K8sSecret> getSecretList(String name, String namespace) {
        KubernetesClient client = kubeClient.getClient();
        SecretList secretList = null;
        Secret targetSecret = null;
        List<K8sSecret> res = new ArrayList<>();
        if (!Strings.isNullOrEmpty(name) && !Strings.isNullOrEmpty(namespace)) {
            targetSecret = client.secrets().inNamespace(namespace).withName(name).get();
            if (targetSecret != null) {
                K8sSecret resSecret = parseK8sSecretToSecret(targetSecret);
                res.add(resSecret);
            }
            return res;
        }

        if (!Strings.isNullOrEmpty(namespace)) {
            secretList = client.secrets().inNamespace(namespace).list();
        } else {
            secretList = client.secrets().inAnyNamespace().list();
        }

        for (Secret item : secretList.getItems()) {
            if (!Strings.isNullOrEmpty(name)) {
                if (!name.equals(item.getMetadata().getName())) {
                    continue;
                }
            }
            K8sSecret tmpSecret = parseK8sSecretToSecret(item);
            res.add(tmpSecret);
        }
        log.info("secret parsed:{}", res);
        return res;
    }

    public void updateResearcherRole(String namespace) throws Exception {
        if (Strings.isNullOrEmpty(namespace)) {
            throw new Exception("invalid namespace");
        }
        // check kubeai-researcher-role exist
        Role role =  kubeClient.getClient().rbac().roles().inNamespace(namespace).withName(DEFAULT_ROLE).get();
        if (role != null) {
            // delete old kubeai-researcher-role
            if (!kubeClient.getClient().rbac().roles().inNamespace(namespace).withName(DEFAULT_ROLE).delete()) {
                throw new Exception("fail to delete " + DEFAULT_ROLE + " in " + namespace);
            }
            log.info("success to delete " + DEFAULT_ROLE + " in " + namespace);

            // create new kubeai-researcher-role
            if (null == this.createRole(DEFAULT_ROLE_FILE, DEFAULT_ROLE, false, namespace)) {
                throw new Exception("fail to create role " + DEFAULT_ROLE + " in " + namespace);
            }
            log.info("success to delete " + DEFAULT_ROLE + " in " + namespace);
        }

    }
    public void updateResearchClusterRole() throws Exception {
        ClusterRole role = kubeClient.getClient().rbac().clusterRoles().withName(DEFAULT_CLUSTER_ROLE).get();
        if (role != null) {
            if (!kubeClient.getClient().rbac().clusterRoles().withName(DEFAULT_CLUSTER_ROLE).delete()) {
                throw new Exception("fail to delete clusterrole: " + DEFAULT_CLUSTER_ROLE);
            }
            log.info("success to delete clusterrole:" + DEFAULT_CLUSTER_ROLE );
            if (null == this.createRole(DEFAULT_CLUSTER_ROLE_FILE, DEFAULT_CLUSTER_ROLE, true, "mockns")) {
                throw new Exception("fail to create clusterrole " + DEFAULT_CLUSTER_ROLE );
            }
            log.info("success to create clusterrole:" + DEFAULT_CLUSTER_ROLE );
        }

    }

    public void createSecret(String filePath, String namespace) throws Exception {
        if (Strings.isNullOrEmpty(filePath) || Strings.isNullOrEmpty(namespace)) {
            throw  new Exception("some param is null ");
        }
        InputStream resource = new ClassPathResource(filePath).getInputStream();
        Secret secret = kubeClient.getClient().secrets().load(resource).get();
        kubeClient.getClient().secrets().inNamespace(namespace).createOrReplace(secret);
        log.info("success to create secret in " + namespace);
    }
}
