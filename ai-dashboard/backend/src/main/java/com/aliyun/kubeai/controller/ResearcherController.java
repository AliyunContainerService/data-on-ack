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
import com.aliyun.kubeai.model.common.Pagination;
import com.aliyun.kubeai.model.common.RequestResult;
import com.aliyun.kubeai.model.common.ResultCode;
import com.aliyun.kubeai.model.k8s.user.User;
import com.aliyun.kubeai.service.UserService;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.apache.commons.compress.utils.IOUtils;
import org.apache.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.context.request.RequestContextHolder;
import org.springframework.web.context.request.ServletRequestAttributes;

import javax.annotation.Resource;
import javax.servlet.http.HttpServletResponse;
import java.io.ByteArrayInputStream;
import java.io.InputStream;
import java.nio.charset.StandardCharsets;

@Slf4j
@RestController
@RequestMapping("/researcher")
public class ResearcherController {
    @Resource
    private UserService researcherService;

    @ModelAttribute
    void setHeader(HttpServletResponse response) {
        response.addHeader("Content-type", "application/octet-stream");
        response.addHeader("Content-Disposition", "attachment;filename=kuebeconfig.yaml");
        response.addHeader("Access-Control-Expose-Headers", "X-Suggested-Filename");
        response.addHeader("X-Suggested-Filename", "kubeconfig.yaml");
    }

    @GetMapping("getBearerToken")
    public RequestResult<String> getBearerToken(@RequestParam(name="userId", required = true) String userId) {
        log.info("get bearer token by userId: {}", userId);
        RequestResult<String> result = new RequestResult<>();
        try {
            String token = researcherService.getBearerTokenByUserId(userId);
            if (Strings.isNullOrEmpty(token)) {
                result.setFailed(ResultCode.GET_RESEARCHER_TOKEN_FAILED, "获取BearerToken失败");
            } else {
                result.setData(token);
            }
        } catch (Exception e) {
            log.error("get bearer token exception", e);
            result.setFailed(ResultCode.GET_RESEARCHER_TOKEN_EXCEPTION, String.format("获取BearerToken异常:%s", e.getMessage()));
        }
        log.info("get bearer token result: {}", JSON.toJSONString(result));
        return result;
    }

    @GetMapping("download/kubeconfig")
    public void downloadKubeConfig(@RequestParam(name = "userId") String userId,
                                   @RequestParam(name = "namespace", required = false) String curNamespace) {
        HttpServletResponse response = ((ServletRequestAttributes) RequestContextHolder.getRequestAttributes()).getResponse();
        setHeader(response);
        log.info("download kubeconfig researcher:{} namespace:{}", userId, curNamespace);
        try {
            String kubeConfig = researcherService.genKubeConfig(userId, curNamespace);
            log.info("kubeconfig length:{}", kubeConfig.length());
            InputStream inputStream = new ByteArrayInputStream(kubeConfig.getBytes(StandardCharsets.UTF_8));
            IOUtils.copy(inputStream, response.getOutputStream());
            response.flushBuffer();
        } catch (Exception e) {
            log.error("download kube config exception", e);
            response.setStatus(HttpStatus.SC_SERVICE_UNAVAILABLE);
        }
    }

    @GetMapping("/list")
    public RequestResult<Pagination<User>> listResearcher(@RequestParam(name = "userName", required = false) String userName,
                                                          @RequestParam(name = "page", required = false) Integer page,
                                                          @RequestParam(name = "limit", required = false) Integer limit) {
        log.info("list researcher, page:{}, limit:{}, name:{}", page, limit, userName);
        if (page == null || page < 1) {
            page = 1;
        }
        if (limit == null || limit < 1) {
            limit = 20;
        }
        RequestResult<Pagination<User>> result = new RequestResult<>();
        Pagination<User> pagination = researcherService.listUser(page, limit, userName);
        result.setData(pagination);
        log.info("list researcher result:{}", JSON.toJSONString(result));
        return result;
    }

    @PostMapping("create")
    public RequestResult<User> createRearcher(@RequestBody User researcher) {
        log.info("create researcher: {}", JSON.toJSONString(researcher));
        RequestResult<User> result = new RequestResult<>();
        try {
            boolean success = researcherService.createUser(researcher);
            if (!success) {
                result.setFailed(ResultCode.CREATE_RESEARCHER_FAILED, "创建user失败");
            } else {
                result.setData(researcher);
            }
        } catch (Exception e) {
            log.error("create researcher exception", e);
            result.setFailed(ResultCode.CREATE_RESEARCHER_FAILED, String.format("创建user异常:%s", e.getMessage()));
        }
        log.info("create researcher result: {}", JSON.toJSONString(result));
        return result;
    }

    @PutMapping("update")
    public RequestResult<Void> updateResearcher(@RequestBody User researcher) {
        log.info("update researcher: {}", JSON.toJSONString(researcher));
        RequestResult<Void> result = new RequestResult<>();
        try {
            boolean success = researcherService.updateUser(researcher);
            if (!success) {
                result.setFailed(ResultCode.UPDATE_RESEARCHER_FAILED, "更新user失败");
            }
        } catch (Exception e) {
            log.error("update researcher exception", e);
            result.setFailed(ResultCode.UPDATE_RESEARCHER_EXCEPTION, String.format("更新user异常:%s", e.getMessage()));
        }
        log.info("update researcher result: {}", JSON.toJSONString(result));
        return result;
    }

    @PutMapping("delete")
    public RequestResult<Void> deleteRearcher(@RequestBody User researcher) {
        log.info("delete researcher: {}", JSON.toJSONString(researcher));
        RequestResult<Void> result = new RequestResult<>();
        try {
            if (!researcherService.deleteUser(researcher)) {
                result.setFailed(ResultCode.DELETE_RESEARCHER_FAILED, "删除user失败");
            }
        } catch (Exception e) {
            log.error("delete researcher exception", e);
            result.setFailed(ResultCode.DELETE_RESEARCHER_EXCEPTION, String.format("删除user异常:%s", e.getMessage()));
        }
        log.info("delete researcher result: {}", JSON.toJSONString(result));
        return result;
    }
}
