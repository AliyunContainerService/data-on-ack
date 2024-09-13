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

import com.aliyun.kubeai.model.common.RequestResult;
import com.aliyun.kubeai.model.common.ResultCode;
import com.aliyun.kubeai.service.DashboardService;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.ResponseBody;
import org.springframework.web.bind.annotation.RestController;

import javax.annotation.Resource;

@Slf4j
@RestController
@RequestMapping("/dashboard")
public class DashboardController {

    @Resource
    private DashboardService dashboardService;

    @Deprecated
    @GetMapping("/url")
    @ResponseBody
    public RequestResult<String> getResourceCount() {
        log.info("get grafana url");
        RequestResult<String> result = new RequestResult<>();
        String grafanaUrl = dashboardService.getGrafanaUrl();
        log.info("grafana url: {}", grafanaUrl);
        if (Strings.isNullOrEmpty(grafanaUrl)) {
            result.setFailed(ResultCode.GET_GRAFANA_FAILED, "get grafana url failed");
        } else {
            result.setData(grafanaUrl);
        }

        return result;
    }

}
