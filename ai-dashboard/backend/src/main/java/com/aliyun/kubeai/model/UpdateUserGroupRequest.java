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
    
package com.aliyun.kubeai.model;

import com.alibaba.fastjson.annotation.JSONField;
import com.aliyun.kubeai.model.k8s.UserGroup;
import com.aliyun.kubeai.model.k8s.user.User;
import lombok.Data;

import java.util.List;

@Data
public class UpdateUserGroupRequest {
    @JSONField(name = "userGroup")
    private UserGroup userGroup;

    @JSONField(name = "users")
    private List<User> users;
}
