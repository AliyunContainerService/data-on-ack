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

import lombok.extern.slf4j.Slf4j;

@Slf4j
public class ResourceQuotaType {
    public static final String CPU = "cpu";
    public static final String Memory = "memory";
    public static final String NvidiaGPU = "nvidia.com/gpu";
    public static final String AliyunGPU = "aliyun.com/gpu";
    public static final String RequestPrefix = "requests";
    public static final String LimitPrefix = "limits";


    public static boolean IsRequestKey(String resourceType) {
        return resourceType.startsWith(RequestPrefix);
    }

    public static boolean IsLimitKey(String resourceType) {
        return resourceType.startsWith(LimitPrefix);
    }

    public static String resourceName(String resourceType) {
        String prefixType = LimitPrefix;
        if (IsRequestKey(resourceType)) {
            prefixType = RequestPrefix;
        }
        return resourceType.substring(resourceType.indexOf(prefixType) + prefixType.length() + 1); // +1 for '.'
    }

    public static String reqeustKey(String resourceType) {
        return RequestPrefix + "." + resourceType;
    }

    public static String limitKey(String resourceType) {
        return LimitPrefix + "." + resourceType;
    }
}
