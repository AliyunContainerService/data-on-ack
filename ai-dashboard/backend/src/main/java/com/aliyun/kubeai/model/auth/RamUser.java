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
public class RamUser {
    //"userId": "229587511043049496",
    @JSONField(name = "userId")
    private String userId;
    //"userName": "aidashboard",
    @JSONField(name = "userName")
    private String userName;
    //"displayName": "aidashboard",
    @JSONField(name = "displayName")
    private String displayName;
    //"comments": "",
    @JSONField(name = "comments")
    private String comments;
    //"createDate": "2021-01-19T07:57:29Z",
    @JSONField(name = "createDate")
    private String createDate;
    //"updateDate": "2021-04-25T07:05:27Z"
    @JSONField(name = "updateDate")
    private String updateDate;
}
