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
    
package com.aliyun.kubeai.model.k8s;

import com.aliyun.kubeai.model.k8s.usergroup.Spec;
import com.aliyun.kubeai.model.k8s.usergroup.Status;
import io.fabric8.kubernetes.api.model.Namespaced;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.model.annotation.Group;
import io.fabric8.kubernetes.model.annotation.Version;
import lombok.Data;

import java.util.List;

@Version("v1")
@Group("data.kubeai.alibabacloud.com")
@Data
public class UserGroup extends CustomResource<Spec, Status> implements Namespaced, Comparable<UserGroup> {
    private String crdName;

    @Override
    public int compareTo(UserGroup o) {
        return getSpec().getGroupName().compareTo(o.getSpec().getGroupName());
    }

    public static boolean StringListEqual(List<String> a, List<String> b) {
        if (null == a && null == b) {
            return true;
        }
        if (null != a && b == null) {
            return false;
        }
        if (null != b && a == null) {
            return false;
        }
        return a.equals(b);
    }

    public boolean deepEqual(UserGroup o) {
        if (!getMetadata().getName().equals(o.getMetadata().getName())) {
            return false;
        }
        if (!getMetadata().getNamespace().equals(o.getMetadata().getNamespace())) {
            return false;
        }
        if (!StringListEqual(getSpec().getDefaultClusterRoles(), o.getSpec().getDefaultClusterRoles())) {
            return false;
        }
        if (!StringListEqual(getSpec().getDefaultRoles(), o.getSpec().getDefaultRoles())) {
            return false;
        }
        if (!getSpec().getGroupName().equals(o.getSpec().getGroupName())) {
            return false;
        }
        if (!StringListEqual(getSpec().getQuotaNames(), o.getSpec().getQuotaNames())) {
            return false;
        }
        return true;
    }
}
