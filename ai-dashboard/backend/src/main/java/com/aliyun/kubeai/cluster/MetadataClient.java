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
    
package com.aliyun.kubeai.cluster;

import com.alibaba.fastjson.JSON;
import com.aliyun.kubeai.model.auth.RoleAuth;
import com.aliyun.kubeai.utils.HttpUtil;
import com.google.common.base.Strings;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;

@Component
@Slf4j
public class MetadataClient {
    @Value("${oauth.is-closing:false}")
    private boolean isTesting;

    private static final String METADATA_ENDPOINT = "http://100.100.100.200";

    public String getRegionId() {
        String url = String.format("%s/latest/meta-data/region-id", METADATA_ENDPOINT);
        if (isTesting) {
            return null;
        }
        try {
            return HttpUtil.get(url);
        } catch (IOException e) {
            log.error("get regionId failed", e);
        }

        return null;
    }

    public String getRoleName() {
        String url = String.format("%s/latest/meta-data/ram/security-credentials/", METADATA_ENDPOINT);
        if (isTesting) {
            return null;
        }

        try {
            return HttpUtil.get(url);
        } catch (IOException e) {
            log.error("get role name failed", e);
        }

        return null;
    }

    public RoleAuth getRoleAuth(String roleName) {
        if (isTesting) {
            return null;
        }

        String url = String.format("%s/latest/meta-data/ram/security-credentials/%s", METADATA_ENDPOINT, roleName);
        String payload = null;
        try {
            payload = HttpUtil.get(url);
        } catch (IOException e) {
            log.error("get role name failed", e);
            return null;
        }

        if (!Strings.isNullOrEmpty(payload)) {
            return JSON.parseObject(payload, RoleAuth.class);
        }
        return null;
    }


}
