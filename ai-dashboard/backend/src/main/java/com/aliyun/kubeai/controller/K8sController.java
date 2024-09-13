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
    
package com.aliyun.kubeai.controller;

import com.aliyun.kubeai.entity.K8sNamespace;
import com.aliyun.kubeai.entity.K8sPvc;
import com.aliyun.kubeai.entity.K8sSecret;
import com.aliyun.kubeai.model.common.RequestResult;
import com.aliyun.kubeai.model.common.ResultCode;
import com.aliyun.kubeai.service.K8sService;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import javax.annotation.Resource;
import java.util.List;

@Slf4j
@RestController
@RequestMapping("/k8s")
public class K8sController {
    @Resource
    private K8sService k8sService;

    @GetMapping("/pvc/list")
    public RequestResult<List<K8sPvc>> getPvcs(@RequestParam(name = "name", required = false) String name,
                                               @RequestParam(name = "namespace", required = false) String namespace) {
        log.info("list pvc by name:{} namespace:{}", name, namespace);
        RequestResult<List<K8sPvc>> result = new RequestResult<>();
        try {
            List<K8sPvc> pvcs = k8sService.getPvcList(name, namespace, false);
            log.info("got pvc len:{}", pvcs.size());
            result.setData(pvcs);
        } catch (Exception e) {
            log.error("get pvc failed:", e);
            result.setFailed(ResultCode.LIST_DATASET_FAILED, "获取PVC异常");
        }
        return result;
    }

    @GetMapping("/secret/list")
    public RequestResult<List<K8sSecret>> getSecrets(@RequestParam(name = "name", required = false) String name,
                                                     @RequestParam(name = "namespace", required = false) String namespace) {
        log.info("list K8sSecret by name:{} namespace:{}", name, namespace);
        RequestResult<List<K8sSecret>> result = new RequestResult<>();
        try {
            List<K8sSecret> K8sSecrets = k8sService.getSecretList(name, namespace);
            log.info("got K8sSecrets len:{}", K8sSecrets.size());
            result.setData(K8sSecrets);
        } catch (Exception e) {
            log.error("get K8sSecrets failed:{}", e);
            result.setFailed(ResultCode.LIST_SECRET_FAILED, "获取secret异常");
        }
        return result;
    }

    @GetMapping("/namespace/list")
    public RequestResult<List<K8sNamespace>> getNamespaces(@RequestParam(name = "name", required = false) String name) {
        log.info("list K8sNamespace by name:{} namespace:{}", name);
        RequestResult<List<K8sNamespace>> result = new RequestResult<>();
        try {
            List<K8sNamespace> k8sNamespaces = k8sService.getNamespaceList(name);
            log.info("got k8sNamespaces len:{}", k8sNamespaces.size());
            result.setData(k8sNamespaces);
        } catch (Exception e) {
            log.error("get k8sNamespaces failed:", e);
            result.setFailed(ResultCode.LIST_NAMESPACE_FAILED, "获取namespace异常");
        }
        return result;
    }
}
