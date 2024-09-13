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
import com.aliyun.kubeai.model.CreateUserGroupRequest;
import com.aliyun.kubeai.model.UpdateUserGroupRequest;
import com.aliyun.kubeai.model.common.Pagination;
import com.aliyun.kubeai.model.common.RequestResult;
import com.aliyun.kubeai.model.common.ResultCode;
import com.aliyun.kubeai.model.k8s.UserGroup;
import com.aliyun.kubeai.model.k8s.user.User;
import com.aliyun.kubeai.service.K8sService;
import com.aliyun.kubeai.service.UserGroupService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import java.util.List;
import java.util.Map;

@Slf4j
@RestController
@RequestMapping("/user_group")
public class UserGroupController {
    @Resource
    UserGroupService userGroupService;

    @Resource
    K8sService k8sService;

    @GetMapping("/get_group_namespaces")
    public RequestResult<Map<String, List<String>>> getGroupNamespaces(@RequestParam(name="group_names", required = false) List<String> groupMetaNames) {
        RequestResult<Map<String, List<String>>> result = new RequestResult<>();
        try {
            Map<String, List<String>> resData = userGroupService.getGroupNsByMetaName(groupMetaNames);
            if (null != resData) {
                result.setData(resData);
            }
        } catch (Exception e) {
            log.error("getGroupNamespaces exception", e);
            result.setFailed(ResultCode.GET_GROUP_NAMESPACES_EXCEPTION, String.format("获取Group Namespace异常:%s", e.getMessage()));
        }
        log.info("getGroupNamespaces result: {}", JSON.toJSONString(result));
        return result;
    }

    @GetMapping("/list")
    public RequestResult<Pagination<UserGroup>> listUserGroup(@RequestParam(name = "userGroupName", required = false) String userGroupName,
                                                              @RequestParam(name = "page", required = false) Integer page,
                                                              @RequestParam(name = "limit", required = false) Integer limit) {
        log.info("list researcher group, page:{}, limit:{}, name:{}", page, limit, userGroupName);
        if (page == null || page < 1) {
            page = 1;
        }
        if (limit == null || limit < 1) {
            limit = 20;
        }
        RequestResult<Pagination<UserGroup>> result = new RequestResult<>();
        Pagination<UserGroup> pagination = userGroupService.listUserGroup(page, limit, userGroupName);
        result.setData(pagination);
        log.info("list researcher result:{}", JSON.toJSONString(result));
        return result;
    }

    @PostMapping("create")
    public RequestResult<UserGroup> createUserGroup(@RequestBody CreateUserGroupRequest userGroupReq) {
        log.info("create user group: {}", JSON.toJSONString(userGroupReq));
        RequestResult<UserGroup> result = new RequestResult<>();
        UserGroup userGroup = userGroupReq.getUserGroup();
        List<String> userNames = userGroupReq.getUserNames();
        try {
            boolean success = userGroupService.createUserGroup(userGroup, userNames);
            if (!success) {
                result.setFailed(ResultCode.CREATE_USER_GROUP_FAILED, "创建userGroup失败");
            }
        } catch (Exception e) {
            log.error("create user group exception", e);
            result.setFailed(ResultCode.CREATE_USER_GROUP_EXCEPTION, String.format("创建userGroup异常:%s", e.getMessage()));
        }
        log.info("create user group result: {}", JSON.toJSONString(result));
        return result;
    }

    //@PutMapping("update_users")
    //public RequestResult<Void> updateGroupUsers(@RequestBody List<UpdateGroupUserRequest> userUpdateRequest) {
    //    log.info("update group users req: {}", JSON.toJSONString(userUpdateRequest));
    //    RequestResult<Void> result = new RequestResult<>();
    //    try {
    //        boolean success = userGroupService.updateGroupUsers(userUpdateRequest);
    //        if (!success) {
    //            result.setFailed(ResultCode.UPDATE_USER_GROUP_FAILED, "更新User失败");
    //        }
    //    } catch (Exception e) {
    //        log.error("update user group exception", e);
    //        result.setFailed(ResultCode.UPDATE_USER_GROUP_EXCEPTION, String.format("更新userGroup异常:%s", e.getMessage()));
    //    }
    //    log.info("update group users result: {}", JSON.toJSONString(result));
    //    return result;
    //}

    @PutMapping("update")
    public RequestResult<Void> updateUserGroup(@RequestBody UpdateUserGroupRequest userGroupReq) {
        log.info("update user group: {}", JSON.toJSONString(userGroupReq));
        UserGroup userGroup = userGroupReq.getUserGroup();
        List<User> newUsers = userGroupReq.getUsers();
        RequestResult<Void> result = new RequestResult<>();
        try {
            UserGroup oldUserGroup = k8sService.findUesrGroupByMetaName(userGroup.getMetadata().getName(),
                    userGroup.getMetadata().getNamespace());
            if (oldUserGroup == null) {
                result.setFailed(ResultCode.UPDATE_USER_GROUP_NOT_FOUND, "更新的userGroup不存在");
            } else {
                boolean success = userGroupService.updateUserGroup(oldUserGroup, userGroup, newUsers);
                if (!success) {
                    result.setFailed(ResultCode.UPDATE_USER_GROUP_FAILED, "更新userGroup失败");
                }
            }
        } catch (Exception e) {
            log.error("update user group exception", e);
            result.setFailed(ResultCode.UPDATE_USER_GROUP_EXCEPTION, String.format("更新userGroup异常:%s", e.getMessage()));
        }
        log.info("update user group result: {}", JSON.toJSONString(result));
        return result;
    }

    @PutMapping("delete")
    public RequestResult<Void> deleteUserGroup(@RequestBody UserGroup userGroup) {
        log.info("delete user group: {}", JSON.toJSONString(userGroup));
        RequestResult<Void> result = new RequestResult<>();
        try {
            boolean success = userGroupService.deleteUserGroup(userGroup);
            if (!success) {
                result.setFailed(ResultCode.DELETE_USER_GROUP_FAILED, "删除userGroup失败");
            }
        } catch (Exception e) {
            log.error("delete user group exception", e);
            result.setFailed(ResultCode.DELETE_USER_GROUP_EXCEPTION, String.format("删除userGroup异常:%s", e.getMessage()));
        }
        log.info("delete user group result: {}", JSON.toJSONString(result));
        return result;
    }
}
