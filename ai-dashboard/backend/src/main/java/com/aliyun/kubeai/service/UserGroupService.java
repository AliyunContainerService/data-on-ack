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
import com.aliyun.kubeai.dao.K8sUserDao;
import com.aliyun.kubeai.dao.K8sUserGroupDao;
import com.aliyun.kubeai.model.common.Pagination;
import com.aliyun.kubeai.model.k8s.UserGroup;
import com.aliyun.kubeai.model.k8s.eqtree.ElasticQuotaTreeWithPrefix;
import com.aliyun.kubeai.model.k8s.user.K8sServiceAccount;
import com.aliyun.kubeai.model.k8s.user.User;
import com.aliyun.kubeai.model.k8s.usergroup.Spec;
import com.google.common.base.Strings;
import com.google.common.collect.Sets;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.api.model.ServiceAccount;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.*;
import java.util.stream.Collectors;

import static com.aliyun.kubeai.model.k8s.UserGroup.StringListEqual;
import static com.aliyun.kubeai.utils.StringUtil.deserializeNodeName;
import static com.aliyun.kubeai.utils.StringUtil.serializeNodeName;

@Slf4j
@Service
public class UserGroupService {
    @Resource
    K8sService k8sService;

    @Resource
    K8sUserDao userDao;

    @Resource
    QuotaGroupService quotaGroupService;

    @Autowired
    K8sUserGroupDao userGroupDao;

    public Map<String, List<String>> getGroupNsByMetaName(List<String> groupSpecNames) throws Exception{
        Map<String, String> groupNameToQuotaName = new HashMap<>();
        Map<String, List<String>> groupNamespaceIndex = new HashMap<>();
        groupNamespaceIndex.put("default-group", Arrays.asList(new String[]{"default-group"})); // add default-group
        List<UserGroup> allUserGroups = userGroupDao.listUserGroupByGroupName(null, null, false);
        if (null == allUserGroups) {
            return groupNamespaceIndex;
        }
        for (UserGroup ug : allUserGroups) {
            if(ug.getSpec().getQuotaNames() != null && !ug.getSpec().getQuotaNames().isEmpty()) {
                groupNameToQuotaName.put(ug.getSpec().getGroupName(), ug.getSpec().getQuotaNames().get(0));
            }
//            groupNameToQuotaName.put(ug.getSpec().getGroupName(),
//                    ug.getSpec().getQuotaNames() == null || ug.getSpec().getQuotaNames().isEmpty() ? null : ug.getSpec().getQuotaNames().get(0));
        }
        List<List<String>> quotaNameLists = new ArrayList<>();
        if (groupSpecNames != null) {
            quotaNameLists = allUserGroups.stream().filter(x->groupSpecNames.contains(x.getSpec().getGroupName()))
                    .map(x->x.getSpec().getQuotaNames()).filter(x->x!=null).collect(Collectors.toList());
        } else {
            quotaNameLists = allUserGroups.stream().map(x->x.getSpec().getQuotaNames()).filter(x->x!=null).collect(Collectors.toList());
        }
        List<String> quotaNames = new ArrayList<>();
        for (List<String> quotaNameList: quotaNameLists) {
            quotaNames.addAll(quotaNameList);
        }
        Map<String, List<String>> quotaNamespacesIndex = quotaGroupService.getQuotaNamespacesIndexByName(null, quotaNames);
        for (Map.Entry<String, String> kv : groupNameToQuotaName.entrySet()) {
            List<String> value = quotaNamespacesIndex.get(kv.getValue());
            groupNamespaceIndex.put(kv.getKey(), value);
        }
        return groupNamespaceIndex;
    }

    private boolean updateGroupUser(String userName, List<String> newGroupNames, List<String> oldGroupNames) throws Exception{
        User userToUpdate = k8sService.findUserByName(userName);
        if (null == userToUpdate) {
            log.warn("user not found name:{}", userName);
            return false;
        }
        List<String> k8sGroups = userToUpdate.getSpec().getGroups();
        if (!StringListEqual(k8sGroups, oldGroupNames)) {
            log.warn("user's group not match in userName:{} ui:{} k8s:{}", userName, oldGroupNames, k8sGroups);
        }

        if (StringListEqual(newGroupNames, k8sGroups)) {
            log.info("update group user with group not change");
            return true;
        }
        Set<String> oldGroupsSet = new HashSet<>();
        if (null != k8sGroups) {
            oldGroupsSet.addAll(k8sGroups);
        }
        Set<String> newGroupsSet = new HashSet<>();
        if (null != newGroupNames) {
            newGroupsSet.addAll(newGroupNames);
        }

        ServiceAccount serviceAccount = k8sService.findServiceAccountByUser(userToUpdate);
        if (null == serviceAccount) {
            log.warn("service account not found for user:{}", userToUpdate.getSpec().getUserName());
            return false;
        }

        List<UserGroup> oldGroups = oldGroupsSet.stream()
                .map(x->k8sService.findUesrGroupByMetaName(x, null))
                .filter(Objects::nonNull)
                .collect(Collectors.toList());

        List<UserGroup> newGroups = newGroupsSet.stream()
                .map(x->k8sService.findUesrGroupByMetaName(x, null))
                .filter(Objects::nonNull)
                .collect(Collectors.toList());

        if (!genAndUpdateUserRoleBindingsByUserGroups(serviceAccount, oldGroups, newGroups, userToUpdate)){
            log.warn("delete user group role by user failed");
            return false;
        }
        // update group to user
        userToUpdate.getSpec().setGroups(newGroupNames);
        userToUpdate = cleanUser(userToUpdate);
        if (!userDao.updateUser(userToUpdate, null)) {
            log.warn("update user failed:%s", userName);
            return false;
        }
        return true;
    }

    public Pagination<UserGroup> listUserGroup(int page, int limit, String userGroupName) {
        Pagination<UserGroup> res = new Pagination();
        List<UserGroup> totalItems = userGroupDao.listUserGroupByGroupName(userGroupName, null, false);
        int total = totalItems.size();
        Collections.sort(totalItems);
        List<UserGroup> resK8sItems = totalItems;
        if (page * limit <= total) {
            resK8sItems = totalItems.subList((page - 1) * limit, page * limit);
        } else if ((page - 1) * limit >= total) {
            resK8sItems = Arrays.asList();
        }
        res.setItems(resK8sItems);
        res.setTotal(total);
        return res;
    }

    private boolean isUserGroupNameExist(String groupName) {
        if (Strings.isNullOrEmpty(groupName)) {
            return false;
        }
        UserGroup foundGroup = k8sService.findUserGroupByName(groupName, null);
        if (null != foundGroup) {
            return true;
        }
        foundGroup = k8sService.findUesrGroupByMetaName(groupName, null);
        return null != foundGroup;
    }

    private boolean isQuotaNodeExistAndNotBindByOtherGroup(List<String> quotaNames) throws Exception {
        if (null == quotaNames) return true;
        List<UserGroup> userGroups = userGroupDao.listUserGroupByGroupName(null, null, false);
        Set<String> bindedQuotaNames = new HashSet<>();
        userGroups.stream().map(x->bindedQuotaNames.addAll(x.getSpec().getQuotaNames()));
        ElasticQuotaTreeWithPrefix tree = quotaGroupService.getElasticQuotaTree(null, null);
        for (String quotaName: quotaNames) {
            if (bindedQuotaNames.contains(quotaName)) {
                throw new Exception(String.format("quota node already binded name:{}", quotaName));
            }
            List<String> nodeNameList = deserializeNodeName(quotaName);
            if (null == quotaGroupService.findElasticNodeByName(tree.getSpec().getRoot(), nodeNameList.get(1), nodeNameList.get(0))) {
                throw new Exception(String.format("quota node not found, name:%s", quotaName));
            }
        }
        return true;
    }

    public boolean createUserGroup(UserGroup userGroup, List<String> userNames) throws Exception {
        log.info("create userGroup:{}", JSON.toJSONString(userGroup));
        Spec spec = userGroup.getSpec();
        String groupName = spec.getGroupName();
        if (Strings.isNullOrEmpty(groupName)) {
            log.warn("user group name is required");
            return false;
        }
        if (isUserGroupNameExist(groupName)) {
            throw new Exception("userGroup already exist");
        }

        // check quota node exist
        if (null != userGroup.getSpec().getQuotaNames()) {
            if (!isQuotaNodeExistAndNotBindByOtherGroup(userGroup.getSpec().getQuotaNames())) {
                log.warn("quota node illeagl: not exist or binded by others");
                return false;
            }
        }
        log.info("create user group with name:{}", groupName);
        String metaName = String.format("%s", groupName.toLowerCase().replace("@", "-")).replace("_", "-");
        userGroup.getMetadata().setName(metaName);
        if (!userGroupDao.createUserGroup(userGroup)) {
            log.warn("create k8s user group failed");
            return false;
        }

        if (null != userNames && !userNames.isEmpty()) {
            for (String userName : userNames) {
                User user = k8sService.findUserByName(userName);
                if (null == user) {
                    log.warn("user not found to join group:{} userName:{}", groupName, userName);
                    return false;
                }
                List<String> oldGroups = user.getSpec().getGroups();
                List<String> newGroups = new ArrayList<>();
                if (oldGroups != null) {
                    newGroups = new ArrayList<>(oldGroups);
                }
                newGroups.add(metaName);
                if(!updateGroupUser(userName, newGroups, oldGroups)) {
                    log.warn("update k8s user's groups failed:{}", userName);
                    return false;
                }
            }
        }

        return true;
    }

    private User cleanUser(User u) {
        User resUser = new User();
        u.getSpec().getK8sServiceAccount().setAdditionalProperty("additionalProperties", null);
        u.getSpec().getK8sServiceAccount().setName(k8sService.genServiceAccountName(null, u.getSpec().getUserId()));
        u.getSpec().getK8sServiceAccount().setNamespace(u.getMetadata().getNamespace());
        resUser.setSpec(u.getSpec());
        ObjectMeta meta = new ObjectMetaBuilder().withNamespace(u.getMetadata().getNamespace()).withName(u.getMetadata().getName()).build();
        resUser.setMetadata(meta);
        return resUser;
    }

    public boolean updateUserGroup(UserGroup oldUserGroup, UserGroup newUserGroup, List<User> newUsers) throws Exception {
        // NOTE user refer group by group.meta.name, not group.spec.name
        List<User> oldGroupUsers = k8sService.listUserByGroupMetaName(oldUserGroup.getMetadata().getName());
        //boolean isRoleNotChange = null != newUserGroup && oldUserGroup.deepEqual(newUserGroup);
        //update role bindings and user group

        Set<String> oldUserNames = new HashSet<>();
        if (null != oldGroupUsers) {
            oldUserNames = oldGroupUsers.stream().map(x->x.getSpec().getUserName()).collect(Collectors.toSet());
        }
        Set<String> newUserNames = new HashSet<>();
        if (null != newUsers) {
            newUserNames = newUsers.stream().map(x -> x.getSpec().getUserName()).collect(Collectors.toSet());
        }
        log.info("old users:{} new users:{}", oldUserNames, newUserNames);
        Set<String> userNamesToDelete = com.google.common.collect.Sets.difference(oldUserNames, newUserNames);
        Set<String> userNamesToAdd = com.google.common.collect.Sets.difference(newUserNames, oldUserNames);
        Set<String> userNamesToUpdate = com.google.common.collect.Sets.intersection(oldUserNames, newUserNames);
        List<User> usersToDelete = new ArrayList<>();
        List<User> usersToUpdate = new ArrayList<>();
        List<User> usersToAdd = new ArrayList<>();

        if (null != userNamesToDelete && !userNamesToDelete.isEmpty()) {
            usersToDelete = oldGroupUsers.stream().filter(x -> userNamesToDelete.contains(x.getSpec().getUserName())).collect(Collectors.toList());
        }
        if (null != userNamesToAdd && !userNamesToAdd.isEmpty()) {
            usersToAdd = newUsers.stream().filter(x -> userNamesToAdd.contains(x.getSpec().getUserName())).collect(Collectors.toList());
        }
        if (null != userNamesToUpdate && !userNamesToUpdate.isEmpty()) {
            usersToUpdate = newUsers.stream().filter(x -> userNamesToUpdate.contains(x.getSpec().getUserName())).collect(Collectors.toList());
        }
        if(!this.updateUserRoleBindingsAndUserGroup(oldUserGroup, newUserGroup, usersToUpdate, usersToDelete, usersToAdd)){
            log.warn("updateUserGroupAndRoles failed oldUserGroup:{} newUserGroup:{} newUserNames:{}", oldGroupUsers, newUserGroup, newUsers);
            return false;
        }

        log.info("users to leave group:{} users:{}", oldUserGroup.getSpec().getGroupName(), userNamesToDelete);
        for (User u : usersToDelete) {
            if (null == u.getSpec().getGroups()) {
                u.getSpec().setGroups(new ArrayList<>());
            }
            List<String> newGroups = new ArrayList<>();
            if (null != u.getSpec().getGroups()) {
                newGroups = u.getSpec().getGroups().stream().filter(x -> !x.equals(oldUserGroup.getMetadata().getName())).collect(Collectors.toList());
            }
            u.getSpec().setGroups(newGroups);
            u = cleanUser(u);
            if (!userDao.updateUser(u, u)) {
                log.warn("delete group for users failed  user:{} group:{}", u.getSpec().getUserName(), newGroups);
                return false;
            }
        }
        log.info("users to join group:{} users:{}", newUserGroup.getSpec().getGroupName(), userNamesToAdd);
        for (User u : usersToAdd) {
            if (null == u.getSpec().getGroups()) {
                u.getSpec().setGroups(new ArrayList<>());
            }
            u.getSpec().getGroups().add(newUserGroup.getMetadata().getName());
            u = cleanUser(u);
            if (!userDao.updateUser(u, u)) {
                log.warn("add group to users failed user:{} groups:{}", u.getSpec().getUserName(), u.getSpec().getGroups());
                return false;
            }
        }
        log.info("users to update role bindings group:{} users:{}", newUserGroup.getMetadata().getName(), userNamesToUpdate);
        //if (!isRoleNotChange) {
        for (User u : usersToUpdate) {
            u = cleanUser(u);
            if (!userDao.updateUser(u, u)) {
                log.warn("update role binding for users failed user:{} group:{}", u.getSpec().getUserName(), u.getSpec().getGroups());
                return false;
            }
        }
        //}
        return true;
    }

    // update one user's role binding by userGroups it belongs
    private boolean genAndUpdateUserRoleBindingsByUserGroups(ServiceAccount serviceAccount,
                                                             List<UserGroup> oldGroups,
                                                             List<UserGroup> newGroups,
                                                             User user) throws Exception {
        Set<String> allOldQuotaNamespaces = new HashSet<>();
        Set<String> userRoles = userDao.getRolesByUserType(user.getSpec().getApiRoles(), false);
        Set<String> userCusterRoles = userDao.getRolesByUserType(user.getSpec().getApiRoles(), true);
        Set<String> allNewQuotaNamespaces = new HashSet<>();
        if (null != oldGroups) {
            allOldQuotaNamespaces = k8sService.genRoleBindingInfoByGroups(oldGroups, null);
        }
        if (null != newGroups) {
            allNewQuotaNamespaces = k8sService.genRoleBindingInfoByGroups(newGroups, null);
        }

        if (!genAndUpdateRoleBindingsForUserByUserGroups(serviceAccount, userRoles, userRoles, userCusterRoles, userCusterRoles,
                allOldQuotaNamespaces, allNewQuotaNamespaces, user)){
            log.warn("update role binding failed for user:{}", user);
            return false;
        }
        return true;
    }

     // gen and update role bindings for user
     public boolean genAndUpdateRoleBindingsForUserByUserGroups(ServiceAccount serviceAccount,
                                                                Set<String> oldRoles, Set<String> newRoles,
                                                                Set<String> oldClusterRoles, Set<String> newClusterRoles,
                                                                Set<String> oldQuotaNamespaces, Set<String> newQuotaNamespaces,
                                                                User user) throws Exception {
        log.info("updateRoleBindingsForGroups sa:{} namespace:{}->{} roles:{}->{} clusterroles:{}->{}",
                serviceAccount.getMetadata().getName(), oldQuotaNamespaces, newQuotaNamespaces, oldRoles, newRoles, oldClusterRoles, newClusterRoles);

        K8sServiceAccount oldK8sServiceAccount = new K8sServiceAccount();
        if(!k8sService.genK8sRoleBindingsByRolesAndNamespaces(serviceAccount, oldRoles, oldClusterRoles, oldQuotaNamespaces, oldK8sServiceAccount)) {
            log.warn("gen k8s roleBinding for old groups failed, sa:{} namespace:{} roles:{} clusterRoles:{}",
                    serviceAccount.getMetadata().getName(), oldQuotaNamespaces, oldRoles, oldClusterRoles);
            return false;
        }
        K8sServiceAccount newK8sServiceAccount = new K8sServiceAccount();
        if(!k8sService.genK8sRoleBindingsByRolesAndNamespaces(serviceAccount, newRoles, newClusterRoles, newQuotaNamespaces, newK8sServiceAccount)) {
            log.warn("gen k8s roleBinding for new groups failed, sa:{} namespace:{} roles:{} clusterRoles:{}",
                    serviceAccount.getMetadata().getName(), newQuotaNamespaces, newRoles, newClusterRoles);
            return false;
        }
        if (null != user) {
             user.getSpec().setK8sServiceAccount(newK8sServiceAccount);
        }
        if (!k8sService.updateRoleBinding(serviceAccount, newK8sServiceAccount.getRoleBindings(), oldK8sServiceAccount.getRoleBindings(), false)){
            log.warn("update role binding failed for sa:{} namespace:{}->{} roles:{}->{}", serviceAccount.getMetadata().getName(),
                    oldQuotaNamespaces, newQuotaNamespaces, oldRoles, newRoles);
            return false;
        }
        if (!k8sService.updateRoleBinding(serviceAccount, newK8sServiceAccount.getClusterRoleBindings(), oldK8sServiceAccount.getClusterRoleBindings(), true)){
            log.warn("update cluster role binding failed for sa:{} cluster roles:{}->{}", serviceAccount.getMetadata().getName(), oldClusterRoles, newClusterRoles);
            return false;
        }
        return true;
    }

    private boolean updateUserRoleBindingsByUserGroup(List<User> users, UserGroup oldUserGroup, UserGroup newUserGroup) throws Exception{
        if (null == users || users.isEmpty()) {
            log.info("no user to update");
            return true;
        }

        if (oldUserGroup != null && newUserGroup != null && oldUserGroup.getMetadata().getName().equals(newUserGroup.getMetadata().getName())) {
            log.info("update user group without name change, no user need to update");
            return true;
        }

        for (User user: users) {
            String saName = k8sService.genServiceAccountName(user.getSpec().getUserName(), user.getSpec().getUserId());
            log.info("update user CR name:{} userName:{}", user.getMetadata().getName(), saName);
            if (null == user.getSpec().getK8sServiceAccount()) {
                user.getSpec().setK8sServiceAccount(new K8sServiceAccount());
            }
            // role update
            ServiceAccount serviceAccount = k8sService.findServiceAccountByUser(user);
            if (null == serviceAccount) {
                log.warn("service account not found for user:{}", user.getSpec().getUserName());
                return false;
            }
            List<UserGroup> newUserGroups = new ArrayList<>();
            List<UserGroup> oldUserGroups = new ArrayList<>();
            if (null != user.getSpec().getGroups()){
                oldUserGroups = user.getSpec().getGroups().stream()
                        .map(x->k8sService.findUesrGroupByMetaName(x, null))
                        .filter(Objects::nonNull)
                        .collect(Collectors.toList());
                if(null != oldUserGroups) {
                    if (null != oldUserGroup) {
                        newUserGroups = oldUserGroups.stream()
                                .filter(x -> !x.getMetadata().getName().equals(oldUserGroup.getMetadata().getName()))
                                .filter(Objects::nonNull)
                                .collect(Collectors.toList());
                    } else {
                        newUserGroups.addAll(oldUserGroups);
                    }
                }
            }

            if (null != newUserGroup) {
                log.info("add or update group to user group:{} user:{}", newUserGroup.getMetadata().getName(), user.getMetadata().getName());
                newUserGroups.add(newUserGroup);
            }
            if(!genAndUpdateUserRoleBindingsByUserGroups(serviceAccount, oldUserGroups, newUserGroups, user)){
                log.warn("update user groups role failed");
                return false;
            }
        }
        return true;
    }

    public boolean createOrReplaceUserGroup(UserGroup userGroup) {
        return userGroupDao.createOrReplaceUserGroup(userGroup);
    }

    public List<UserGroup> findUserGroupsByQuotaName(ElasticQuotaTreeWithPrefix tree, String quotaName, String quotaNamePrefix, boolean isRecursive) {
        String quotaWithPrefix = serializeNodeName(quotaNamePrefix, quotaName);
        if (Strings.isNullOrEmpty(quotaWithPrefix)) {
            return null;
        }
        List<UserGroup> allUserGroups = userGroupDao.listUserGroupByGroupName(null, null, false);
        if (null == allUserGroups) {
            return null;
        }
        if (!isRecursive) {
            return allUserGroups.stream().filter(x -> x.getSpec().getQuotaNames().contains(quotaWithPrefix)).collect(Collectors.toList());
        }
        Set<String> ancenstors = new HashSet<>(quotaGroupService.findAncenstorNodeNamesInTree(tree, quotaName, quotaNamePrefix));
        return allUserGroups.stream()
                .filter(x->x.getSpec().getQuotaNames()!=null)
                .filter(x-> !Sets.intersection(
                        new HashSet<>(x.getSpec().getQuotaNames()),
                        ancenstors).isEmpty()
                ).collect(Collectors.toList());
    }

    private boolean updateUserRoleBindingsAndUserGroup(UserGroup oldUserGroup, UserGroup newUserGroup,
                                                       List<User> usersToUpdate, List<User> usersToDelete, List<User> usersToAdd) throws Exception{
        Set<String> oldQuotaNamespaces = quotaGroupService.getQuotaNamespacesByName(null, oldUserGroup.getSpec().getQuotaNames());
        boolean isRoleNotChange = null != newUserGroup && oldUserGroup.deepEqual(newUserGroup);
        if ((isRoleNotChange && (null == usersToUpdate || usersToUpdate.isEmpty()))
                && (null == usersToDelete || usersToDelete.isEmpty())
                && (null == usersToAdd || usersToAdd.isEmpty())) {
            log.info("update user group without change");
            return true;
        }

        if (null != usersToDelete) {
            Set<String> oldNamespaces = quotaGroupService.getQuotaNamespacesByName(null, oldUserGroup.getSpec().getQuotaNames());
            for (User user: usersToDelete) {
                userDao.isUserHasRunningResourceInNamespace(user, oldNamespaces);
            }
        }

        boolean isDeleteUserGroup = null == newUserGroup;
        Set<String> newQuotaNamespaces = new HashSet<>();
        if (!isDeleteUserGroup) {
            newQuotaNamespaces = quotaGroupService.getQuotaNamespacesByName(null, newUserGroup.getSpec().getQuotaNames());
        }
        log.info("update user group with user oldNs:{} newNs:{}", oldQuotaNamespaces, newQuotaNamespaces);

        //if (!isRoleNotChange) {
        if (!updateUserRoleBindingsByUserGroup(usersToUpdate, oldUserGroup, newUserGroup)) {
            log.warn("update userGroup/users roles failed");
            return false;
        }
        //}
        if (!updateUserRoleBindingsByUserGroup(usersToAdd, null, newUserGroup)){
            log.warn("add users to userGroup roles");
            return false;
        }
        if (!updateUserRoleBindingsByUserGroup(usersToDelete, oldUserGroup, null)){
            log.warn("delete userGroup roles failed");
            return false;
        }
        // update userGroup
        if (isDeleteUserGroup) {
            return userGroupDao.deleteUserGroup(oldUserGroup);
        }
        if (!isRoleNotChange) {
            return userGroupDao.createOrReplaceUserGroup(newUserGroup);
        }
        return true;
    }

    public boolean deleteUserGroup(UserGroup userGroup) throws Exception{
        String gname = userGroup.getMetadata().getName();
        String gns = userGroup.getMetadata().getNamespace();
        UserGroup ug = k8sService.findUesrGroupByMetaName(gname, gns);
        if (null == ug) {
            log.info("delete user group not found name:%s ns:%s", gname, gns);
            return true;
        }
        // NOTE user refer group by group.meta.name, not group.spec.name
        List<User> oldGroupUsers = k8sService.listUserByGroupMetaName(userGroup.getMetadata().getName());
        if(!this.updateUserRoleBindingsAndUserGroup(userGroup, null, null, oldGroupUsers, null)) {
            log.warn("update user group role bindings failed");
            return false;
        }
        // update user group names
        for (User u: oldGroupUsers) {
            List<String> newGroups = u.getSpec().getGroups().stream().filter(x->!x.equals(gname)).collect(Collectors.toList());
            u.getSpec().setGroups(newGroups);
            u = cleanUser(u);
            if(!userDao.updateUser(u, u)){
                log.warn("update user's group failed {}", JSON.toJSONString(u));
                return false;
            }
        }
        return true;
    }


}
