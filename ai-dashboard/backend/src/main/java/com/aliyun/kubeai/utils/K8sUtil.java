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
    
package com.aliyun.kubeai.utils;

import com.aliyun.kubeai.cluster.KubeClient;
import com.google.common.base.Strings;
import io.fabric8.kubernetes.api.model.Quantity;
import lombok.extern.slf4j.Slf4j;

@Slf4j
public class K8sUtil {
    public static final String RESOUCE_TYPE_MEMORY = "memory";
    public static final String RESOUCE_TYPE_ALIYUN_GPU_MEMORY = "aliyun.com/gpu-mem";
    public static final String ENV_K8S_VERSION = "CLUSTER_VERSION"; // 1.20.4-aliyun.1
    public static final String DEFAULT_ELASTIC_QUOTA_TREE_VERSION = "1.20.0";
    private static final String DEFAULT_NO_ELASTIC_QUOTA_TREE_VERSION = "1.18.0";
    private final static Integer MAX_QUOTA = 2147483647;
    private final static String UI_MAX_QUOTA = "N/A";

    public static Double parseResourceToNumber(String resource) {
        if (Strings.isNullOrEmpty(resource)) {
            resource = "0";
        }
        Quantity q = Quantity.parse(resource);
        if (q.getAmount().equals(UI_MAX_QUOTA) || Double.parseDouble(q.getAmount()) >= MAX_QUOTA) {
            q.setAmount(String.format("%d", MAX_QUOTA));
        }
        return Quantity.getAmountInBytes(q).doubleValue();
    }

    public static String k8sVersion(KubeClient kubeClient) {
        String k8sVersion = System.getenv(ENV_K8S_VERSION);
        int startIdx = 0;
        if (Strings.isNullOrEmpty(k8sVersion)) {
            k8sVersion = kubeClient.getClient().getVersion().getGitVersion();
            if (k8sVersion.toLowerCase().startsWith("v")) {
                startIdx = 1; // skip 'v'
            }
        }
        int endIndex = k8sVersion.indexOf('-');
        if (endIndex < 0) {
            endIndex = k8sVersion.length();
        }
        k8sVersion = k8sVersion.substring(startIdx, endIndex);
        log.info("k8sVersion:{}", k8sVersion);
        return k8sVersion;
    }
}
