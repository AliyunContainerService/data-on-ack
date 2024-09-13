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
import com.aliyun.kubeai.model.k8s.UserGroup;
import com.aliyun.kubeai.model.k8s.UserGroupList;
import com.aliyun.kubeai.model.k8s.usergroup.Status;
import com.google.common.base.Strings;
import com.google.common.cache.Cache;
import com.google.common.cache.CacheBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.fabric8.kubernetes.client.dsl.base.CustomResourceDefinitionContext;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Component;

import java.io.InputStream;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;

@Slf4j
@Component
public class K8sUserGroupDao {
    public static final String KUBEAI_USER_GROUPS_CRD_NAME = "usergroups.data.kubeai.alibabacloud.com";
    public static final String DEFAULT_USER_GROUP_NAMESPACE = "kube-ai";

    @Autowired
    public KubeClient kubeClient;

    private Cache<String, UserGroup> localCache = CacheBuilder.newBuilder()
            .expireAfterAccess(3600, TimeUnit.MINUTES)
            .build();

    private UserGroup setNullWithDefaultValue(UserGroup userGroup) {
        if (userGroup.getSpec().getDefaultClusterRoles() == null) {
            userGroup.getSpec().setDefaultClusterRoles(new ArrayList<>());
        }
        if (userGroup.getSpec().getDefaultRoles() == null) {
            userGroup.getSpec().setDefaultRoles(new ArrayList<>());
        }
        if (userGroup.getStatus() == null) {
            userGroup.setStatus(new Status());
        }
        return userGroup;
    }

    public UserGroup createUserGroupFromFile(String yamlFileName, boolean isReplace) throws Exception {
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_GROUPS_CRD_NAME);
        MixedOperation<UserGroup, UserGroupList,
                Resource<UserGroup>> client =
                kubeClient.getClient().customResources(ctx, UserGroup.class, UserGroupList.class);
        InputStream defaultYamlStream = new ClassPathResource(yamlFileName).getInputStream();
        UserGroup userGroup = client.load(defaultYamlStream).get();
        if (Strings.isNullOrEmpty(userGroup.getMetadata().getNamespace())) {
            userGroup.getMetadata().setNamespace(DEFAULT_USER_GROUP_NAMESPACE);
        }
        if (!isReplace) {
            UserGroup foundUserGroup = this.getUserGroupByMetaName(userGroup.getMetadata().getName(), userGroup.getMetadata().getNamespace());
            if (null != foundUserGroup) {
                return foundUserGroup;
            }
        }
        userGroup = setNullWithDefaultValue(userGroup);
        return client.inNamespace(userGroup.getMetadata().getNamespace()).createOrReplace(userGroup);
    }

    public UserGroup loadUserGroup(String userGroupCrdInJson) {
        if (Strings.isNullOrEmpty(userGroupCrdInJson)) {
            return null;
        }
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_GROUPS_CRD_NAME);
        MixedOperation<UserGroup, UserGroupList,
                Resource<UserGroup>> userClient =
                kubeClient.getClient().customResources(ctx, UserGroup.class, UserGroupList.class);
        return userClient.load(userGroupCrdInJson).get();
    }

    public UserGroup getUserGroupByMetaName(String userGroupMetaName, String namespace) {
        if (Strings.isNullOrEmpty(userGroupMetaName)) {
            return null;
        }
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_GROUPS_CRD_NAME);
        MixedOperation<UserGroup, UserGroupList,
                Resource<UserGroup>> userClient =
                kubeClient.getClient().customResources(ctx, UserGroup.class, UserGroupList.class);
        if (Strings.isNullOrEmpty(namespace)) {
            namespace = DEFAULT_USER_GROUP_NAMESPACE;
        }
        return userClient.inNamespace(namespace).withName(userGroupMetaName).get();
    }

    public List<UserGroup> listUserGroupByGroupName(String userGroupName, String namespace, boolean strictMatch) {
        List<UserGroup> res = new ArrayList<>();

        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_GROUPS_CRD_NAME);
        MixedOperation<UserGroup, UserGroupList,
                Resource<UserGroup>> userClient =
                kubeClient.getClient().customResources(ctx, UserGroup.class, UserGroupList.class);
        if (Strings.isNullOrEmpty(namespace)) {
            namespace = DEFAULT_USER_GROUP_NAMESPACE;
        }
        UserGroupList k8sUserGroupList = userClient.inNamespace(namespace).list();
        for (UserGroup k8sUserGroup : k8sUserGroupList.getItems()) {
            //updateCache(k8sUserGroup);
            if (!Strings.isNullOrEmpty(userGroupName)) {
                if (!k8sUserGroup.getSpec().getGroupName().contains(userGroupName)) {
                    continue;
                }
                if (strictMatch) {
                    if (!k8sUserGroup.getSpec().getGroupName().equals(userGroupName)) {
                        continue;
                    }
                }
            }
            res.add(k8sUserGroup);
        }
        return res;
    }

    public boolean createUserGroup(UserGroup userGroup) throws Exception {
        log.info("create user group:{}", JSON.toJSONString(userGroup));
        if (userGroup == null) {
            log.warn("create user group with empty value");
            return false;
        }
        if (Strings.isNullOrEmpty(userGroup.getMetadata().getNamespace())) {
            userGroup.getMetadata().setNamespace(DEFAULT_USER_GROUP_NAMESPACE);
        }
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_GROUPS_CRD_NAME);
        MixedOperation<UserGroup, UserGroupList,
                        io.fabric8.kubernetes.client.dsl.Resource<UserGroup>> client =
                                kubeClient.getClient().customResources(ctx, UserGroup.class, UserGroupList.class);
        if (Strings.isNullOrEmpty(userGroup.getMetadata().getNamespace())) {
            userGroup.getMetadata().setNamespace(DEFAULT_USER_GROUP_NAMESPACE);
        }
        userGroup = setNullWithDefaultValue(userGroup);
        UserGroup res = client.inNamespace(userGroup.getMetadata().getNamespace()).create(userGroup);
        log.info("update or create user group done res:{}", res);
        return true;
    }

     public boolean createOrReplaceUserGroup(UserGroup userGroup) {
         CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_GROUPS_CRD_NAME);
         KubernetesClient client = kubeClient.getClient();
         String ns = userGroup.getMetadata().getNamespace();
         if (Strings.isNullOrEmpty(ns)) {
             ns = DEFAULT_USER_GROUP_NAMESPACE;
         }
         try {
             userGroup = setNullWithDefaultValue(userGroup);
             Map<String, Object> res = client.customResource(ctx).createOrReplace(ns, JSON.toJSONString(userGroup));
             log.info("update user group res:{}", JSON.toJSONString(res));
         } catch (Exception e) {
             log.error("update usergroup exception:", e);
             return false;
         }
        return true;
    }

    public boolean deleteUserGroup(UserGroup userGroup) {
        CustomResourceDefinitionContext ctx = kubeClient.buildCrdContext(KUBEAI_USER_GROUPS_CRD_NAME);
        //MixedOperation<UserGroup, UserGroupList,
        //        Resource<UserGroup>> userGroupClient =
        //        kubeClient.getClient().customResources(ctx, UserGroup.class, UserGroupList.class);
         KubernetesClient client = kubeClient.getClient();
         String ns = userGroup.getMetadata().getNamespace();
         if (Strings.isNullOrEmpty(ns)) {
             ns = DEFAULT_USER_GROUP_NAMESPACE;
         }
         try {
             Map<String, Object> res = client.customResource(ctx).delete(ns, userGroup.getMetadata().getName());
             log.info("delete user group res:{}", JSON.toJSONString(res));
         } catch (Exception e) {
             log.error("delete user group exception:", e);
             return false;
         }
        return true;
    }

    //private void updateCache(UserGroup userGroup) {
    //    String userGroupId = userGroup.getMetadata().getName();
    //    log.info("update cache user group uid:{}", userGroupId);
    //    localCache.put(userGroupId, userGroup);
    //    return;
    //}
}
