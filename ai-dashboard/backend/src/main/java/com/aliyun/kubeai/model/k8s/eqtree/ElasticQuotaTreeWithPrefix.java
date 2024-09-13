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
    
package com.aliyun.kubeai.model.k8s.eqtree;

import io.fabric8.kubernetes.api.model.Namespaced;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.model.annotation.Group;
import io.fabric8.kubernetes.model.annotation.Version;
import lombok.Data;


@Version("v1beta1")
@Group("scheduling.sigs.k8s.io")
@Data
public class ElasticQuotaTreeWithPrefix extends CustomResource<SpecWithPrefix, Status> implements Namespaced, Comparable<ElasticQuotaTreeWithPrefix> {
    private String crdName;

    @Override
    public int compareTo(ElasticQuotaTreeWithPrefix o) {
        return getMetadata().getCreationTimestamp().compareTo(o.getMetadata().getCreationTimestamp());
    }
}


