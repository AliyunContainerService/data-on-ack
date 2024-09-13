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

import com.aliyun.kubeai.cluster.KubeClient;
import com.aliyun.kubeai.cluster.MetadataClient;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;

@Slf4j
@Service
public class DashboardService {

    private static final String GRAFANA_URL = "http://grafana.%s.%s.alicontainer.com";

    @Resource
    MetadataClient metadataClient;

    @Resource
    private KubeClient kubeClient;

    /**
     * 获得各种资源的统计数据
     *
     * @return
     */
    public String getGrafanaUrl() {
        String clusterId = kubeClient.getClusterId();
        String regionId = metadataClient.getRegionId();
        if (Strings.isNullOrEmpty(clusterId) || Strings.isNullOrEmpty(regionId)) {
            log.error("get grafana url failed");
            return null;
        }

        return String.format(GRAFANA_URL, clusterId, regionId);
    }

}
