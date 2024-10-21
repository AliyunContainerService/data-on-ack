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
import com.aliyun.kubeai.model.auth.RamUser;
import com.aliyun.kubeai.model.common.RequestResult;
import com.aliyun.kubeai.model.common.ResultCode;
import com.aliyun.kubeai.model.k8s.user.Spec;
import com.aliyun.kubeai.model.k8s.user.User;
import com.aliyun.kubeai.service.RamService;
import com.aliyun.kubeai.service.UserService;
import com.aliyun.kubeai.vo.ApiRole;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.security.core.Authentication;
import org.springframework.security.oauth2.provider.OAuth2Authentication;
import org.springframework.security.oauth2.provider.authentication.OAuth2AuthenticationDetails;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import java.security.Principal;
import java.util.*;

import static com.aliyun.kubeai.utils.K8sUtil.k8sVersion;


@Slf4j
@RestController
@RequestMapping("/user")
public class AdminController {

    @Resource
    private UserService userService;

    @Resource
    private RamService ramService;

    @Resource
    private KubeClient kubeClient;

    private static final String RAM_TOKEN_KEY = "ram_access_token";
    private static final String RAM_REFRESH_KEY = "ram_refresh_token";

    @Value("${oauth.is-closing:false}")
    private boolean isTesting;

    @GetMapping("/list/ramUsers")
    public RequestResult<List<RamUser>> listRamUser() {
        RequestResult<List<RamUser>> result = new RequestResult<>();
        try {
            List<RamUser> ramUsers = new ArrayList<>();
            // mock for local test
            if (isTesting) {
                RamUser ramUser = new RamUser();
                ramUser.setUserName("hella@1983706117860305.onaliyun.com");
                ramUser.setUserId("261305311888729924");
                ramUsers.add(ramUser);
            } else {
                ramUsers = ramService.listRamUser();
            }
            result.setData(ramUsers);
        } catch (Exception e) {
            log.error("list ram user exception", e);
            result.setFailed(ResultCode.USER_LIST_RAM_EXCEPTION, "list ram user exception");
        }
        log.info("list ram user result:{}", JSON.toJSONString(result));
        return result;
    }

    @GetMapping("/get")
    public RequestResult<User> getAdmin(@RequestParam(name = "aliuid", required = false) String aliuid) {
        log.info("get user aliuid:{}", aliuid);
        RequestResult<User> result = new RequestResult<>();
        try {
            User pagination = userService.findUserByAliuid(aliuid);
            result.setData(pagination);
        } catch (Exception e) {
            log.error("get admin error", e);
            result.setFailed(ResultCode.USER_NOT_FOUND, "user not found");
        }
        log.info("get user aliuid result:{}", JSON.toJSONString(result));
        return result;
    }

    @GetMapping("info")
    public RequestResult<Map<String, Object>> user(Principal principal) {
        RequestResult<Map<String, Object>> result = new RequestResult<>();
        OAuth2Authentication myPrincipal = (OAuth2Authentication) principal;
        Map<String, Object> userInfo = new HashMap<>();
        String clusterVersion = k8sVersion(kubeClient);
        userInfo.put("k8sVersion", clusterVersion);
        User user = new User();

        String token = UUID.randomUUID().toString().replace("-", "");
        if (myPrincipal == null) {
            if (isTesting) {
                Spec spec = new Spec();
                spec.setApiRoles(Arrays.asList(ApiRole.ADMIN.toString()));
                spec.setUserName("admin");
                user.setSpec(spec);
                userInfo.put("user", user);
                userInfo.put("token", token);
                result.setData(userInfo);
                log.info("user/info res:{}", JSON.toJSONString(userInfo));
                return result;
            }
            result.setFailed(ResultCode.USER_NOT_LOGIN, "user not login");
            return result;
        }
        Authentication userAuthentication = myPrincipal.getUserAuthentication();
        log.info("user authen res:{}", JSON.toJSONString(userAuthentication));

        OAuth2AuthenticationDetails auth2AuthenticationDetails = (OAuth2AuthenticationDetails) myPrincipal.getDetails();
        log.info("user authen details:{}", JSON.toJSONString(auth2AuthenticationDetails));

        String aliyunAccessToken = auth2AuthenticationDetails.getTokenValue();
        Map<String, String> userProfileDetails = (Map<String, String>) userAuthentication.getDetails();
        log.info("accessToken:{}", aliyunAccessToken);
        String aliuid = userProfileDetails.get("uid");
        String ramUserPrincipleName = userProfileDetails.get("upn"); // aidashboard@1323.com
        if (Strings.isNullOrEmpty(ramUserPrincipleName)) {
            // main account only has login_name, sub account has upn(User Principal Name)
            ramUserPrincipleName = userProfileDetails.get("login_name"); //jackwg@1323.com
        }
        // update ram token, refresh token
        if (Strings.isNullOrEmpty(aliuid)) {
            result.setFailed(ResultCode.USER_AUTH_FAILED, "user auth failed");
            return result;
        }

        log.info("found user by id:{} ramUserPrincipleName:{}", aliuid, ramUserPrincipleName);
        try {
            user = userService.findUserByAliuid(aliuid);

            if (user == null) {
                result.setCode(ResultCode.USER_NOT_FOUND);
                result.setMessage("user not login");
                return result;
            } else {
                // set user name
                String userName = user.getSpec().getUserName();
                if (Strings.isNullOrEmpty(userName) || !userName.equals(ramUserPrincipleName)) {
                    log.info("update user name from:{} to:{}", userName, ramUserPrincipleName);
                    user.getSpec().setUserName(ramUserPrincipleName);
                    if (!userService.updateUser(user)) {
                        log.warn("upadate user name failed userName from:{} to:{}", userName, ramUserPrincipleName);
                    }
                }
            }
            log.info("find user name:{}", JSON.toJSONString(user));
        } catch (Exception e) {
            log.error("get user exception", e);
            result.setFailed(ResultCode.USER_NOT_FOUND, "found user exception:" + e.getMessage());
            return result;
        }
        // save to mysql
        log.info("user/info res:{}", JSON.toJSONString(userInfo));
        userInfo.put("user", user);
        userInfo.put("token", token);
        result.setData(userInfo);
        return result;
    }

    @PostMapping("/logout")
    public RequestResult<Void> logout(@RequestBody Map<String, Object> logoutInfo) {
        log.info("logout: {}", JSON.toJSONString(logoutInfo));
        RequestResult<Void> result = new RequestResult<>();
        //String token = (String) logoutInfo.get("token");
        return result;
    }
}
