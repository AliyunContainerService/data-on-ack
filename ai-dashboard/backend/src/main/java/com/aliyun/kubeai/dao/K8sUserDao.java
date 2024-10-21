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
import com.aliyun.kubeai.entity.JobEntity;
import com.aliyun.kubeai.entity.JobSqlRequest;
import com.aliyun.kubeai.entity.NotebookEntity;
import com.aliyun.kubeai.mapper.JobMapper;
import com.aliyun.kubeai.mapper.NotebookMapper;
import com.aliyun.kubeai.model.k8s.user.Status;
import com.aliyun.kubeai.model.k8s.user.User;
import com.aliyun.kubeai.model.k8s.user.UserList;
import com.aliyun.kubeai.vo.ApiRole;
import com.google.common.base.Strings;
import com.google.common.cache.Cache;
import com.google.common.cache.CacheBuilder;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.base.CustomResourceDefinitionContext;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Component;

import java.util.*;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

@Slf4j
@Component
public class K8sUserDao {
    public static final String KUBEAI_USER_CRD_NAME = "users.data.kubeai.alibabacloud.com";
    private static final String DEFAULT_USER_NAMESPACE = "kube-ai";
    public static final String USER_TYPE_ADMIN = "admin";
    public static final String USER_TYPE_RESEARCHER = "researcher";
    public static final String ADMIN_DEFAULT_CLUSTER_ROLE = "kubeai-admin-clusterrole";
    public static final String RESEARCHER_DEFAULT_CLUSTER_ROLE = "kubeai-researcher-clusterrole";
    public static final String RESEARCHER_DEFAULT_ROLE = "kubeai-researcher-role";

    @Autowired
    public JobMapper jobMapper;

    @Autowired
    public NotebookMapper notebookMapper;

    @Autowired
    public KubeClient kubeClient;

    public Set<String> getRolesByUserType(List<String> apiRoles, boolean isGetClusterRole) {
        Set<String> res = new HashSet<>();
        boolean tmp = isGetClusterRole ? res.add(RESEARCHER_DEFAULT_CLUSTER_ROLE) : res.add(RESEARCHER_DEFAULT_ROLE);
        if (apiRoles.contains(USER_TYPE_ADMIN)) {
            tmp = isGetClusterRole ? res.add(ADMIN_DEFAULT_CLUSTER_ROLE) : false;
        }
        return res;
    }

    private Cache<String, User> localCache = CacheBuilder.newBuilder()
            .expireAfterAccess(3600, TimeUnit.MINUTES)
            .build();

    public boolean isUserHasRunningResourceInNamespace(User user, Set<String> namespaces) throws Exception {
        if (namespaces == null || namespaces.isEmpty()) {
            return false;
        }
        JobSqlRequest req = new JobSqlRequest();
        req.setNamespaces(namespaces);
        req.setUserIds(new HashSet<>(Arrays.asList(user.getMetadata().getName())));
        req.setStatuses(new HashSet<>(Arrays.asList("Running")));
        log.info("find running job req:{}", req);
        List<JobEntity> runningJobs = jobMapper.findJob(req);
        log.info("found running job res size:{}", runningJobs.size());
        if (runningJobs != null && !runningJobs.isEmpty()) {
            throw new Exception("user has running job");
        }

        JobSqlRequest notebookReq = new JobSqlRequest();
        notebookReq.setNamespaces(namespaces);
        notebookReq.setUserIds(new HashSet<>(Arrays.asList(user.getSpec().getUserName())));
        notebookReq.setStatuses(new HashSet<>(Arrays.asList("Running")));
        List<NotebookEntity> runningNotebooks = notebookMapper.findNotebook(notebookReq);
        if (runningNotebooks != null && !runningNotebooks.isEmpty()) {
            throw new Exception("user has running notebook");
        }
        return false;
    }

    public User loadUser(String userCrdInJson) {
        if (Strings.isNullOrEmpty(userCrdInJson)){
            return null;
        }
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_CRD_NAME);
        MixedOperation<User, UserList,
                io.fabric8.kubernetes.client.dsl.Resource<User>> userClient =
                kubeClient.getClient().customResources(ctx, User.class, UserList.class);
        return userClient.load(userCrdInJson).get();
    }

    public List<User> findUserByName(String userName, String namespace, boolean isStrictMatch) {
        List<User> res = new ArrayList<>();

        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_CRD_NAME);
        MixedOperation<User, UserList,
                io.fabric8.kubernetes.client.dsl.Resource<User>> userClient =
                kubeClient.getClient().customResources(ctx, User.class, UserList.class);
        if (Strings.isNullOrEmpty(namespace)) {
            namespace = DEFAULT_USER_NAMESPACE; // found user in kube-ai namespace by default
        }
        UserList k8sUserList = userClient.inNamespace(namespace).list();
        for (User k8sUser : k8sUserList.getItems()) {
//            updateCache(k8sUser);
            if (!Strings.isNullOrEmpty(userName)) {
                if (!k8sUser.getSpec().getUserName().contains(userName)) {
                    continue;
                }
                if (isStrictMatch) {
                    if (!k8sUser.getSpec().getUserName().equals(userName)) {
                        continue;
                    }
                }
                res.add(k8sUser);
            } else {
                res.add(k8sUser);
            }
        }
        return res;
    }

    public User findUserByAliuid(String aliuid) {
        if (Strings.isNullOrEmpty(aliuid)) {
            return null;
        }
        User ret = localCache.getIfPresent(aliuid);
        if (null != ret && ret.getSpec().getApiRoles().contains(ApiRole.ADMIN.toString())) {
            log.info("found user in cache by aliuid:{}", aliuid);
            return ret;
        }

        List<User> userList = this.findUserByName(null, null, true);
        if (null == userList || userList.isEmpty()) {
            return null;
        }

        List<User> adminList = userList.stream().filter(
                x -> x.getSpec().getApiRoles().contains(ApiRole.ADMIN.toString()) && x.getSpec().getAliuid() != null && x.getSpec().getAliuid().equals(aliuid)
        ).collect(Collectors.toList());
        if (!adminList.isEmpty()) {
            log.info("found user by aliuid:{}", aliuid);
            return adminList.get(0);
        }
        return null;
    }

    public User findUserById(String userId) {
        User res = localCache.getIfPresent(userId);
        if (res != null) {
            log.info("found user in cache by id:{}", userId);
            return res;
        }

        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_CRD_NAME);


        MixedOperation<User, UserList,
                io.fabric8.kubernetes.client.dsl.Resource<User>> userClient =
                kubeClient.getClient().customResources(ctx, User.class, UserList.class);
        res = userClient.inNamespace(DEFAULT_USER_NAMESPACE).withName(userId).get();
        if (res != null) {
            log.info("find user in k8s by id:{} {}", userId, res);
//            updateCache(res);
        }
        return res;
    }

    public boolean updateUser(User user, User oldUser) throws Exception {
        log.info("update user:{}", JSON.toJSONString(user));
        if (user == null) {
            log.warn("update user with empty value");
            return false;
        }
        if (Strings.isNullOrEmpty(user.getMetadata().getNamespace())) {
            user.getMetadata().setNamespace(DEFAULT_USER_NAMESPACE);
        }
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_CRD_NAME);
        user = setNullWithDefaultValue(user);
        String objJsonString = JSON.toJSONString(user).replace("cRDName", "crdName");
        Map<String, Object> res = kubeClient.getClient().customResource(ctx).createOrReplace(DEFAULT_USER_NAMESPACE, objJsonString);
        if (res == null) {
            log.warn("update user failed res null");
            return false;
        }
        if (null == oldUser) {
            oldUser = user;
        }
//        invalidateCache(oldUser);
//        updateCache(user);
        log.info("update user done req:{}", objJsonString);
        return true;
    }

    private User setNullWithDefaultValue(User user) {
        if (user.getSpec().getPassword() == null) {
            user.getSpec().setPassword("");
        }
        if (user.getSpec().getGroups() == null) {
            user.getSpec().setGroups(new ArrayList<>());
        }
        if (user.getSpec().getApiRoles() == null) {
            user.getSpec().setApiRoles(new ArrayList<>());
        }
        if (user.getSpec().getExternalUsers() == null) {
            user.getSpec().setExternalUsers(new ArrayList<>());
        }
        if (user.getSpec().getDeletable() == null) {
            user.getSpec().setDeletable(true);
        }
        if (user.getSpec().getAliuid() == null) {
            user.getSpec().setAliuid("");
        }
        if (user.getStatus() == null) {
            user.setStatus(new Status());
        }
        return user;
    }

    public boolean createUser(User user) throws Exception {
        log.info("create user:{}", JSON.toJSONString(user));
        if (user == null) {
            log.warn("create user with empty value");
            return false;
        }
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_CRD_NAME);
        MixedOperation<User, UserList,
                        io.fabric8.kubernetes.client.dsl.Resource<User>> client =
                kubeClient.getClient().customResources(ctx, User.class, UserList.class);
        if (Strings.isNullOrEmpty(user.getMetadata().getNamespace())) {
            user.getMetadata().setNamespace(DEFAULT_USER_NAMESPACE);
        }
        user = setNullWithDefaultValue(user);
        User res = client.inNamespace(user.getMetadata().getNamespace()).create(user);
//        updateCache(user);
        log.info("create user done value:{}", res);
        return true;
    }

    public boolean deleteUser(User user) throws Exception {
        log.info("delete user value:{}", user);
        String userName = user.getMetadata().getName();
        if (Strings.isNullOrEmpty(userName)) {
            log.info("delete invalid user");
            return false;
        }
        //delete user crd
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_CRD_NAME);
        kubeClient.getClient().customResource(ctx).delete(DEFAULT_USER_NAMESPACE, userName);
//        invalidateCache(user);
        return true;
    }

    private void invalidateCache(User user) {
        String userId = user.getMetadata().getName();
        localCache.invalidate(userId);
        log.info("invalidate cache user uid:{}", userId);
        String aliuid = user.getSpec().getAliuid();
        if (!Strings.isNullOrEmpty(aliuid)) {
            log.info("invalidate cache user aliuid:{}", aliuid);
            localCache.invalidate(aliuid);
        }
        return;
    }

    private void updateCache(User user) {
        String userId = user.getMetadata().getName();
        log.info("update cache user uid:{}", userId);
        localCache.put(userId, user);
        String aliuid = user.getSpec().getAliuid();
        if (!Strings.isNullOrEmpty(aliuid)) {
            log.info("update cache user aliuid:{}", aliuid);
            localCache.put(aliuid, user);
        }
        return;
    }
}
