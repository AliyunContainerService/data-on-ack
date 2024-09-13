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
import com.aliyun.kubeai.exception.AIException;
import com.aliyun.kubeai.model.common.Pagination;
import com.aliyun.kubeai.model.k8s.UserGroup;
import com.aliyun.kubeai.model.k8s.user.K8sRoleBinding;
import com.aliyun.kubeai.model.k8s.user.K8sServiceAccount;
import com.aliyun.kubeai.model.k8s.user.Spec;
import com.aliyun.kubeai.model.k8s.user.User;
import com.aliyun.kubeai.vo.ApiRole;
import com.google.common.base.Strings;
import com.google.common.collect.Sets;
import io.fabric8.kubernetes.api.model.*;
import io.fabric8.kubernetes.client.utils.Serialization;
import lombok.extern.slf4j.Slf4j;
import org.apache.commons.codec.binary.Base64;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.io.ByteArrayOutputStream;
import java.nio.charset.StandardCharsets;
import java.util.*;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

import static com.aliyun.kubeai.utils.DateUtil.transUTCTime;


@Slf4j
@Service
public class UserService {
    public static final String DEFAULT_QUOTA_NAMESPACE = "default-group";
    public static final String DEFAULT_USER_NAMESPACE = "kube-ai";
    public static final String ADMIN_DEFAULT_CLUSTER_ROLE = "kubeai-admin-clusterrole";
    public static final String DEFAULT_KUBERNETES_ENDPOINT_NAME = "kubernetes";
    public static final String DEFAULT_CLUSTER_NAME = "kubernetes";
    public static final String ENV_DASHBOARD_ADMIN_UID = "DASHBOARD_ADMINUID";


    @Autowired
    K8sUserDao userDao;

    @Autowired
    K8sUserGroupDao userGroupDao;

    @Resource
    K8sService k8sService;

    @Resource
    QuotaGroupService quotaGroupService;

    @Autowired
    KubeClient client;

    /**
     * replace with spring security
     *
     * @param userName
     * @param password
     * @return
     */
    public User login(String userName, String password) {
        if (Strings.isNullOrEmpty(userName)) {
            throw new AIException("用户名不能为空");
        }
        if (Strings.isNullOrEmpty(password)) {
            throw new AIException("密码不能为空");
        }

        User user = userDao.findUserById(userName); // cant login by userName by now, it's not unique
        if (user == null || !user.getSpec().getPassword().equals(password)) {
            throw new AIException("登录失败，用户名或密码错误");
        }
        return user;
    }

    public ServiceAccount findServiceAccountByName(String name, String namespace) throws Exception {
        ServiceAccount serviceAccount = null;
        try {
            serviceAccount = client.getClient().serviceAccounts().inNamespace(namespace).withName(name).get();
        } catch (Exception e) {
            log.info("get service account failed {} in {}, wait 1 sec:", name, namespace, e.getMessage());
            TimeUnit.SECONDS.sleep(1);  // k8s create sc in async, so wait for 1s here
            serviceAccount = client.getClient().serviceAccounts().inNamespace(namespace).withName(name).get();
        }
        return serviceAccount;
    }

    public boolean namespaceAuthentication(String namespace, User user) {
        if (Strings.isNullOrEmpty(namespace)) {
            return false;
        }

        Spec spec = user.getSpec();
        if (spec.getApiRoles().contains(ApiRole.ADMIN.toString())) {
            return true;
        }

        List<K8sRoleBinding> clusterRoleConfigs = spec.getK8sServiceAccount().getClusterRoleBindings();
        if (null != clusterRoleConfigs) {
            List<String> clusterRoles = clusterRoleConfigs.stream().map(x -> x.getRoleName()).collect(Collectors.toList());
            if (!clusterRoles.isEmpty()) {
                return true;
            }
        }

        List<K8sRoleBinding> roleConfigs = spec.getK8sServiceAccount().getRoleBindings();
        if (null != roleConfigs) {
            List<String> roleNamespaces = roleConfigs.stream().map(x -> x.getNamespace()).collect(Collectors.toList());
            return roleNamespaces.contains(namespace);
        }
        return false;
    }

    public String getDefaultCurrentNamespace(User user) {
        Spec spec = user.getSpec();
        List<K8sRoleBinding> roleConfigs = spec.getK8sServiceAccount().getRoleBindings();
        if (null != roleConfigs) {
            List<String> roleNamespaces = roleConfigs.stream().filter(x -> x.getNamespace() != null && !x.getNamespace().isEmpty()).map(x -> x.getNamespace()).collect(Collectors.toList());
            if (!roleNamespaces.isEmpty()) {
                return roleNamespaces.get(0);
            }
        }

        List<K8sRoleBinding> clusterRoleConfigs = spec.getK8sServiceAccount().getClusterRoleBindings();
        if (null != clusterRoleConfigs) {
            List<String> roleNamespaces = clusterRoleConfigs.stream().filter(x -> x.getNamespace() != null && !x.getNamespace().isEmpty()).map(x -> x.getNamespace()).collect(Collectors.toList());
            if (!roleNamespaces.isEmpty()) {
                return roleNamespaces.get(0);
            }
            List<String> roleNames = clusterRoleConfigs.stream().map(x -> x.getRoleName()).collect(Collectors.toList());
            if (!roleNames.isEmpty()) {
                return "default";
            }
        }

        return null;
    }

    public Set<String> getRolesByUserType(List<String> apiRoles, boolean isGetClusterRole) {
        return userDao.getRolesByUserType(apiRoles, isGetClusterRole);
    }

    public String getBearerTokenByUserId(String userId) throws Exception {
        User user = userDao.findUserById(userId);
        if (null == user) {
            log.warn("gen kube config not found user id:{}", userId);
            return null;
        }
        String serviceAccountName = user.getSpec().getK8sServiceAccount().getName();
        String serviceAccountNamespace = user.getSpec().getK8sServiceAccount().getNamespace();
        ServiceAccount serviceAccount = findServiceAccountByName(serviceAccountName, serviceAccountNamespace);
        if (serviceAccount == null) {
            log.warn("can't find service account by name:{}", serviceAccountNamespace);
            return null;
        }

        String secretName = serviceAccount.getSecrets().get(0).getName();
        Secret secret = client.getClient().secrets().inNamespace(serviceAccountNamespace).withName(secretName).get();
        if (null == secret) {
            log.warn("can't find secret by sa:{}", secretName);
            return null;
        }
        String secretTokenBase64 = secret.getData().get("token");
        String secretToken = new String(Base64.decodeBase64(secretTokenBase64.getBytes()), StandardCharsets.UTF_8);
        return secretToken;
    }

    public String genKubeConfig(String userId, String curNamespace) throws Exception {
        log.info("gen kube config for user id:{}", userId);
        User user = userDao.findUserById(userId);
        if (null == user) {
            log.warn("gen kube config not found user id:{}", userId);
            return null;
        }
        String serviceAccountName = user.getSpec().getK8sServiceAccount().getName();
        String serviceAccountNamespace = user.getSpec().getK8sServiceAccount().getNamespace();
        ServiceAccount serviceAccount = k8sService.findServiceAccountByUser(user);
        if (serviceAccount == null) {
            log.warn("can't find service account by name:{}", serviceAccountNamespace);
            return null;
        }

        if (Strings.isNullOrEmpty(curNamespace)) {
            curNamespace = getDefaultCurrentNamespace(user);
        }
        if (!namespaceAuthentication(curNamespace, user)) {
            log.warn("namespace:{} not authorize to user:{}", curNamespace, user);
            return null;
        }

        String secretName = serviceAccount.getSecrets().get(0).getName();
        Secret secret = client.getClient().secrets().inNamespace(serviceAccountNamespace).withName(secretName).get();
        String secretCaCrtBase64 = secret.getData().get("ca.crt");
        String secretNamespaceBase64 = secret.getData().get("namespace");
        String secretNamespace = new String(Base64.decodeBase64(secretNamespaceBase64.getBytes()), StandardCharsets.UTF_8);
        if (Strings.isNullOrEmpty(curNamespace)) {
            curNamespace = secretNamespace;
        }
        String secretTokenBase64 = secret.getData().get("token");
        String secretToken = new String(Base64.decodeBase64(secretTokenBase64.getBytes()), StandardCharsets.UTF_8);

        log.info("config secret done");
        Endpoints endpoints = client.getClient().endpoints().inNamespace("default").withName(DEFAULT_KUBERNETES_ENDPOINT_NAME).get();
        String apiVersion = endpoints.getApiVersion();
        List<EndpointSubset> endpointSubsets = endpoints.getSubsets();
        if (endpointSubsets == null || endpointSubsets.isEmpty()) {
            log.warn("endpoints name:{} not found in default", DEFAULT_KUBERNETES_ENDPOINT_NAME);
            return null;
        }
        String portName = endpointSubsets.get(0).getPorts().get(0).getName();
        Integer portNum = endpointSubsets.get(0).getPorts().get(0).getPort();
        EndpointAddress epAddr = endpoints.getSubsets().get(0).getAddresses().get(0);

        String serverAddrs = String.format("%s://%s:%d/", portName, epAddr.getIp(), portNum);
        log.info("config serverAddrs:{}", serverAddrs);

        String clusterName = DEFAULT_CLUSTER_NAME;
        Context context = new ContextBuilder().withCluster(clusterName).withNamespace(curNamespace).withUser(serviceAccountName).build();
        NamedContext namedContext = new NamedContextBuilder().withContext(context).withName(serviceAccountName).build();

        Cluster cluster = new ClusterBuilder().withCertificateAuthorityData(secretCaCrtBase64).withServer(serverAddrs).build();
        NamedCluster namedCluster = new NamedClusterBuilder().withCluster(cluster).withName(clusterName).build();

        AuthInfo userAuth = new AuthInfoBuilder().withNewToken(secretToken).build();
        NamedAuthInfo authInfo = new NamedAuthInfoBuilder().withName(serviceAccountName).withUser(userAuth).build();

        io.fabric8.kubernetes.api.model.Config modelConfig = new io.fabric8.kubernetes.api.model.ConfigBuilder()
                .withApiVersion(apiVersion)
                .withKind("Config")
                .withCurrentContext(serviceAccountName)
                .withClusters(namedCluster)
                .withContexts(namedContext)
                .withUsers(authInfo)
                .build();
        ByteArrayOutputStream stream = new ByteArrayOutputStream();
        String kubeConfig = "";
        try {
            Serialization.yamlMapper().writeValue(stream, modelConfig);
            kubeConfig = new String(stream.toByteArray());
            log.info("write kube config value:{}", kubeConfig);
        } catch (Exception e) {
            log.error("write kube config value exception:{}", e);
            throw new Exception(e);
        }

        log.info("gen k8s config:{}", kubeConfig);
        //String base64String = Base64.encodeBase64String(kubeConfig.getBytes());
        return kubeConfig;
    }

    private boolean isUserNameExist(String userName) {
        if (Strings.isNullOrEmpty(userName)) {
            return false;
        }
        List<User> foundUser = userDao.findUserByName(userName, null, true);
        return null != foundUser && !foundUser.isEmpty();
    }

    public boolean createUser(User user) throws Exception {
        log.info("create user:{}", JSON.toJSONString(user));
        Spec spec = user.getSpec();
        String userId = spec.getUserId();
        String userNamespace = k8sService.DEFAULT_USER_NAMESPACE;
        if (isUserNameExist(spec.getUserName())) {
            throw new Exception("user name already exist");
        }

        if (Strings.isNullOrEmpty(userId)) {
            // cannot use userName from ram with @, for k8s serviceaccount format check
            userId = String.format("%s", spec.getUserName().toLowerCase().replace("@", "-")).replace("_", "-");//, UUID.randomUUID().toString(), spec.getUserName());
            spec.setUserId(userId);
        }
        log.info("create user with uid:{}", userId);
        user.getMetadata().setName(userId);
        user.getMetadata().setNamespace(userNamespace);


        String serviceAccountName = k8sService.genServiceAccountName(spec.getUserName(), userId);
        K8sServiceAccount serviceAccountConfig = spec.getK8sServiceAccount();
        if (null == serviceAccountConfig) {
            log.warn("create user with namespace empty");
            return false;
        }
        ServiceAccount serviceAccount = client.createServiceAccount(userNamespace, serviceAccountName);
        if (serviceAccount == null) {
            log.warn("create service account failed");
            return false;
        }
        boolean createdSuccess = client.createSecretForServiceAccount(userNamespace, serviceAccount);
        if (!createdSuccess) {
            log.warn("create secret for {} failed", serviceAccount.getMetadata().getName());
            return false;
        }
        serviceAccountConfig.setNamespace(serviceAccount.getMetadata().getNamespace());
        serviceAccountConfig.setName(serviceAccount.getMetadata().getName());
        log.info("create service account {}", serviceAccount);
        updateSpecByApiRoles(spec.getApiRoles(), spec);

        if (!updateRoleBindingForUser(user, null)) {
            log.warn("update role binding failed for create user:{}", user.getSpec().getUserName());
            return false;
        }

        if (!userDao.createUser(user)) {
            log.warn("create k8s user failed");
            return false;
        }
        return true;
    }

    public List<User> findUserByName(String userName) {
        return userDao.findUserByName(userName, k8sService.DEFAULT_USER_NAMESPACE, false);
    }

    public Pagination<User> listUser(int page, int limit, String userName) {
        Pagination<User> res = new Pagination();
        List<User> totalItems = userDao.findUserByName(userName, DEFAULT_USER_NAMESPACE, false);
        totalItems.forEach(x->x.getMetadata().setCreationTimestamp(transUTCTime(x.getMetadata().getCreationTimestamp())));
        int total = totalItems.size();
        Collections.sort(totalItems);
        List<User> resK8sItems = totalItems;
        if (page * limit <= total) {
            resK8sItems = totalItems.subList((page - 1) * limit, page * limit);
        } else if ((page - 1) * limit >= total) {
            resK8sItems = Arrays.asList();
        }
        res.setItems(resK8sItems);
        res.setTotal(total);
        return res;
    }

    List<K8sRoleBinding> filterRoleBindings(List<K8sRoleBinding> roleBindings, String roleName, boolean includeAfterFilter) {
        if (null == roleBindings) {
            roleBindings = new ArrayList<>();
        }
        List<K8sRoleBinding> filteredClusterRoleBindings;
        if (includeAfterFilter) {
            filteredClusterRoleBindings = roleBindings.stream().filter(x -> !Strings.isNullOrEmpty(x.getRoleName()) && x.getRoleName().equals(roleName)).collect(Collectors.toList());
            if (!filteredClusterRoleBindings.isEmpty()) {
                return roleBindings;
            }
            K8sRoleBinding roleBinding = new K8sRoleBinding();
            roleBinding.setRoleName(roleName);
            roleBindings.add(roleBinding);
        } else {
            roleBindings = roleBindings.stream().filter(x -> !Strings.isNullOrEmpty(x.getRoleName()) && !x.getRoleName().equals(roleName)).collect(Collectors.toList());
        }
        return roleBindings;
    }

    public void updateSpecByApiRoles(List<String> apiRoles, Spec userSpec) {
        K8sServiceAccount serviceAccountConfig = userSpec.getK8sServiceAccount();
        List<K8sRoleBinding> clusterRoleBindings = serviceAccountConfig.getClusterRoleBindings();
        List<K8sRoleBinding> roleBindings = serviceAccountConfig.getRoleBindings();
        if (apiRoles.contains(ApiRole.ADMIN.toString())) {
            clusterRoleBindings = filterRoleBindings(clusterRoleBindings, k8sService.ADMIN_DEFAULT_CLUSTER_ROLE, true);
        } else {
            clusterRoleBindings = filterRoleBindings(clusterRoleBindings, k8sService.ADMIN_DEFAULT_CLUSTER_ROLE, false);
            userSpec.setAliuid(null); // reset aliuid for researcher because they can't login webui
        }
        serviceAccountConfig.setClusterRoleBindings(clusterRoleBindings);
        serviceAccountConfig.setRoleBindings(roleBindings);
        return;
    }

    private boolean isAliuidExist(User newUser, User oldUser) {
        String newAliuid = newUser.getSpec().getAliuid();
        String oldAliuid = oldUser.getSpec().getAliuid();
        if (Strings.isNullOrEmpty(newAliuid)) {
            return false;
        } else if (Strings.isNullOrEmpty(oldAliuid)) {
            return false;
        } else {
            if (!newAliuid.equals(oldAliuid)) {
                return null != findUserByAliuid(newAliuid);
            }
        }
        return false;
    }

    // foundUser may be null when create user
    private boolean updateRoleBindingForUser(User user, User foundUser) throws Exception{
        ServiceAccount serviceAccount = null;
        if (null != foundUser) {
            serviceAccount = k8sService.findServiceAccountByUser(foundUser);
        } else {
            serviceAccount = k8sService.findServiceAccountByUser(user);
        }
        if (null == serviceAccount) {
            log.warn("find service account failed for user:{}", user.getSpec().getUserName());
            return false;
        }

        List<K8sRoleBinding> newK8sRoleBindings = user.getSpec().getK8sServiceAccount().getRoleBindings();
        List<K8sRoleBinding> oldK8sRoleBindings = null;
        if (foundUser != null) {
            oldK8sRoleBindings = foundUser.getSpec().getK8sServiceAccount().getRoleBindings();
        }
        if (!k8sService.updateRoleBinding(serviceAccount, newK8sRoleBindings, oldK8sRoleBindings, false)) {
            log.warn("update user role binding failed sa:{} roleBinding:{}->{}",
                    serviceAccount.getMetadata().getName(), oldK8sRoleBindings, newK8sRoleBindings);
            return false;
        }

        // update cluster role bindings
        List<K8sRoleBinding> newK8sClusterRoleBindings = user.getSpec().getK8sServiceAccount().getClusterRoleBindings();
        List<K8sRoleBinding> oldK8sClusterRoleBindings = null;
        if (foundUser != null) {
            oldK8sClusterRoleBindings = foundUser.getSpec().getK8sServiceAccount().getClusterRoleBindings();
        }
        if (!k8sService.updateRoleBinding(serviceAccount, newK8sClusterRoleBindings, oldK8sClusterRoleBindings, true)) {
            log.warn("update user cluster role binding failed sa:{} roleBinding:{}->{}", oldK8sClusterRoleBindings, newK8sClusterRoleBindings);
            return false;
        }

        return true;
    }

    public boolean createOrReplaceUser(User user) throws Exception {
        String uid = user.getMetadata().getName();
        User foundUser = userDao.findUserById(uid);
        if (user.getSpec() != null && user.getSpec().getK8sServiceAccount() != null) {
            user.getSpec().getK8sServiceAccount().setAdditionalProperty("additionalProperties", null);
        }
        if (null == foundUser) {
            log.warn("update user not found id:{}", uid);
            return userDao.createUser(user);
        }
        return userDao.updateUser(user, user);
    }

    private Set<String> parseUserRolebindingNamespaces(User user) throws Exception {
        Set<String> res = new HashSet<>();
        if (null == user.getSpec().getGroups()) {
            return res;
        }
        if (user.getSpec().getK8sServiceAccount() == null || user.getSpec().getK8sServiceAccount().getRoleBindings() == null) {
            return res;
        }
        return user.getSpec().getK8sServiceAccount().getRoleBindings().stream().map(x->x.getNamespace()).collect(Collectors.toSet());
    }

    private Set<String> getNamespaceByUser(User user) throws Exception{
        if (null == user.getSpec().getGroups()) {
            return new HashSet<>();
        }
        Set<String> namespace = new HashSet<>();
        List<UserGroup> userGroups = user.getSpec().getGroups().stream().map(x->k8sService.findUesrGroupByMetaName(x, null))
                .filter(x->x!=null).collect(Collectors.toList());
        if (userGroups != null) {
            for (UserGroup userGroup: userGroups) {
                namespace.addAll(quotaGroupService.getQuotaNamespacesByName(null, userGroup.getSpec().getQuotaNames()));
            }
        }
        return namespace;
    }

    private boolean isUserHasRunningJobInOldGroup(User user, User foundUser) throws Exception {
        if (foundUser == null) {
            return false;
        }

        Set<String> newQuotaNamespace = new HashSet<>();
        if (null != user) {
            newQuotaNamespace = parseUserRolebindingNamespaces(user);
        } else {
            user = foundUser;
        }
        Set<String> oldQuotaNamespace = parseUserRolebindingNamespaces(foundUser);
        Set<String> nsToLeave = Sets.difference(oldQuotaNamespace, newQuotaNamespace);
        if (nsToLeave != null && !nsToLeave.isEmpty()) {
            return userDao.isUserHasRunningResourceInNamespace(user, nsToLeave);
        }
        return false;
    }

    public boolean updateUser(User user) throws Exception {
        String uid = user.getMetadata().getName();
        User foundUser = userDao.findUserById(uid);
        if (null == foundUser) {
            log.warn("update user not found id:{}", uid);
            return false;
        }
        Spec spec = user.getSpec();
        //user name uniq
        if (!foundUser.getSpec().getUserName().equals(spec.getUserName())) {
            if (isUserNameExist(spec.getUserName())) {
                throw new Exception("user name exist");
            }
        }
        // aliuid uniq
        if (this.isAliuidExist(user, foundUser)) {
            throw new Exception("aliuid exist");
        }

        Spec foundSpec = foundUser.getSpec();
        String userName = spec.getUserName();
        if (Strings.isNullOrEmpty(userName)) {
            spec.setUserName(foundSpec.getUserName()); // reset user name if empty
        }
        spec.setUserId(uid); // not editable, reset
        if (null != foundSpec.getDeletable() && !foundSpec.getDeletable()) {
            spec.setDeletable(foundSpec.getDeletable()); // reset deletable
            List<String> apiRoles = spec.getApiRoles(); // reset admin role
            if (!apiRoles.contains(ApiRole.ADMIN.toString())) {
                apiRoles.add(ApiRole.ADMIN.toString());
            }
            spec.setApiRoles(apiRoles);
        }

        K8sServiceAccount foundServiceAccount = foundSpec.getK8sServiceAccount();
        String serviceAccountName = foundServiceAccount.getName();

        // update api roles
        K8sServiceAccount serviceAccountConfig = spec.getK8sServiceAccount();
        updateSpecByApiRoles(spec.getApiRoles(), spec);
        //sync service account
        serviceAccountConfig.setNamespace(foundServiceAccount.getNamespace());
        serviceAccountConfig.setName(serviceAccountName);

        // is user has running job in leaving namespace
        if (isUserHasRunningJobInOldGroup(user, foundUser)) {
            log.warn("user has running job, can not delete");
            return false;
        }
        //update user role bindings
        if (!updateRoleBindingForUser(user, foundUser)) {
            log.warn("update role binding failed for user:{}", user.getSpec().getUserName());
            return false;
        }

        try {
            if (!userDao.updateUser(user, foundUser)) {
                return false;
            }
        } catch (Exception e) {
            log.error("update user error", e);
            return false;
        }

        return true;
    }

    public boolean deleteUser(User user) throws Exception {
        log.info("to delete user:{}", JSON.toJSONString(user));

        String uid = user.getMetadata().getName();
        User foundUser = userDao.findUserById(uid);
        if (null == foundUser) {
            log.warn("delete user not found");
            return false;
        }
        Spec spec = foundUser.getSpec();
        if (null != spec.getDeletable() && false == spec.getDeletable()) {
            throw new Exception("user not deletable");
        }

        K8sServiceAccount serviceAccountConfig = spec.getK8sServiceAccount();
        ServiceAccount serviceAccount = k8sService.findServiceAccountByUser(foundUser);
        if (serviceAccount == null) {
            log.warn("delete user with service account not found");
            return false;
        }

        if (isUserHasRunningJobInOldGroup(null, foundUser)) {
            log.warn("user has running job, can not delete");
            return false;
        }
        //delete clusterRoleBindings
        List<K8sRoleBinding> clusterRoleBindings = serviceAccountConfig.getClusterRoleBindings();
        if (!k8sService.updateRoleBinding(serviceAccount, null, clusterRoleBindings, true)) {
            log.warn("delete cluster role binding failed");
            return false;
        }
        //delete roleBindings
        List<K8sRoleBinding> roleBindings = spec.getK8sServiceAccount().getRoleBindings();
        if (!k8sService.updateRoleBinding(serviceAccount, null, roleBindings, false)) {
            log.warn("delete role binding failed");
            return false;
        }
        //delete user
        try {
            //delete service account
            if (!client.deleteServiceAccount(serviceAccountConfig.getNamespace(), serviceAccountConfig.getName())) {
                log.warn("delete user service account failed");
                return false;
            }
            if (!userDao.deleteUser(foundUser)) {
                log.warn("delete user failed {}", foundUser);
                return false;
            }
        } catch (Exception e) {
            log.error("delete user exception", e);
            return false;
        }
        return true;
    }

    public User findUserByAliuid(String aliuid) {
        if (Strings.isNullOrEmpty(aliuid)) {
            throw new AIException("find by empty uid");
        }
        User foundUser = userDao.findUserByAliuid(aliuid);
        if (null != foundUser) {
            log.debug("find user in k8s by aliuid:{}", aliuid);
            return foundUser;
        }
        String adminUidsStr = System.getenv(ENV_DASHBOARD_ADMIN_UID);
        if (Strings.isNullOrEmpty(adminUidsStr)) {
            return null;
        }
        String[] adminUids = adminUidsStr.split(",");
        log.info("get admin uid from env:{}", adminUids);
        if (adminUids == null || adminUids.length < 1) {
            return null;
        }

        int uidIndex = 0;
        for (String uid : adminUids) {
            uidIndex++;
            if (null != userDao.findUserByAliuid(uid)) {
                log.info("user registered id:{}", uid);
                continue;
            }
            User newAdmin = new User();
            Spec spec = new Spec();
            String userName = uid;
            K8sServiceAccount serviceAccount = new K8sServiceAccount();
            serviceAccount.setName(k8sService.genServiceAccountName(null, uid));
            serviceAccount.setNamespace(DEFAULT_USER_NAMESPACE);

            //cluster role binding
            List<K8sRoleBinding> clusterRoledindings = new ArrayList<>();
            K8sRoleBinding defaultAdminClusterRoleBinding = new K8sRoleBinding();
            K8sRoleBinding defaultReseracherClusterRoleBinding = new K8sRoleBinding();
            defaultAdminClusterRoleBinding.setRoleName(ADMIN_DEFAULT_CLUSTER_ROLE);
            clusterRoledindings.add(defaultAdminClusterRoleBinding);
            defaultReseracherClusterRoleBinding.setRoleName(k8sService.DEFAULT_CLUSTER_ROLE);
            clusterRoledindings.add(defaultReseracherClusterRoleBinding);

            List<K8sRoleBinding> roleBindings = new ArrayList<>();
            if (k8sService.getNamespaceList(DEFAULT_QUOTA_NAMESPACE).size() > 0) {
                K8sRoleBinding roleBinding = new K8sRoleBinding();
                roleBinding.setRoleName(k8sService.DEFAULT_ROLE);
                roleBinding.setNamespace(DEFAULT_QUOTA_NAMESPACE);
                roleBindings.add(roleBinding);
            }
            spec.setApiRoles(Arrays.asList(ApiRole.ADMIN.toString()));
            spec.setUserName(userName);
            spec.setPassword("123456");
            spec.setGroups(Arrays.asList("defaultUserGroup"));
            spec.setAliuid(uid);
            spec.setDeletable(false);
            serviceAccount.setClusterRoleBindings(clusterRoledindings);
            serviceAccount.setRoleBindings(roleBindings);
            spec.setK8sServiceAccount(serviceAccount);
            newAdmin.setSpec(spec);
            log.info("create admin uid:{}", uid);
            try {
                if (!createUser(newAdmin)) {
                    log.warn("create admin user failed");
                    return null;
                }
            } catch (Exception e) {
                log.error("create admin user exception", e);
                continue;
            }
        }
        return userDao.findUserByAliuid(aliuid);
    }

}
