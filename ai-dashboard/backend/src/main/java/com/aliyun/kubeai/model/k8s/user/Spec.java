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
    
package com.aliyun.kubeai.model.k8s.user;

import com.fasterxml.jackson.databind.JsonDeserializer;
import com.fasterxml.jackson.databind.annotation.JsonDeserialize;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import lombok.Data;

import java.util.List;


@JsonDeserialize(using = JsonDeserializer.None.class)
@Data
public class Spec implements KubernetesResource {
    private String userName;
    private String userId;
    private String password;
    private String aliuid;
    private List<ExternalUser> externalUsers;
    private List<String> apiRoles;
    private List<String> groups;
    private K8sServiceAccount k8sServiceAccount;
    private Boolean deletable;
}
