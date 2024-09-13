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
    
package com.aliyun.kubeai.model.auth;

import com.alibaba.fastjson.annotation.JSONField;
import lombok.Data;

@Data
public class RoleAuth {

    @JSONField(name = "Code")
    private String code;

    @JSONField(name = "AccessKeyId")
    private String accessKeyId;

    @JSONField(name = "AccessKeySecret")
    private String accessKeySecret;

    @JSONField(name = "SecurityToken")
    private String securityToken;

    @JSONField(name = "Expiration")
    private String expiration;

    @JSONField(name = "LastUpdated")
    private String lastUpdated;

}
