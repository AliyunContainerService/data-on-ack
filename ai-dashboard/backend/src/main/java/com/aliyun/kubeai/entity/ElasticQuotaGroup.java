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
    
package com.aliyun.kubeai.entity;

import com.aliyun.kubeai.utils.K8sUtil;
import com.google.common.base.Strings;
import lombok.Data;
import lombok.extern.slf4j.Slf4j;

import java.util.List;

/**
 * 组名，树状结果，可支持分层quota管理，叶子节点为namespace
 */
@Data
@Slf4j
public class ElasticQuotaGroup {
    @Data
    public static class ResourceQuota {
        private String resourceName;
        private String min;
        private String max;
        //dynamic
        private Integer using;
        private Integer allocated;
    }

    public boolean isValid() throws Exception {
        if (subGroupNames.isEmpty() || Strings.isNullOrEmpty(name)) {
            log.info("sub group name empty or name empty");
            return false;
        }
        for (String subGroupName : subGroupNames) {
            if (Strings.isNullOrEmpty(subGroupName)) {
                log.info("sub group name empty");
                return false;
            }
        }
        for (ResourceQuota quota : quotaList) {
            String resourceName = quota.getResourceName();
            Double min = null;
            if (!Strings.isNullOrEmpty(quota.getMin())) {
                min = K8sUtil.parseResourceToNumber(quota.getMin());
            }
            Double max = null;
            if (!Strings.isNullOrEmpty(quota.getMax())) {
                max = K8sUtil.parseResourceToNumber(quota.getMax());
            }
            if (min != null && max != null && min > max) {
                throw new Exception(String.format("resource min>max name:%s min:%s max:%s", resourceName, quota.getMin(), quota.getMax()));
            }
        }
        return true;
    }

    private String name;
    /**
     * sub group maybe namespace or group
     * v1: only one namespace in subGroupNames
     */
    private List<String> subGroupNames;
    private List<ResourceQuota> quotaList;
    private String createTime;
    private String updateTime;
}
