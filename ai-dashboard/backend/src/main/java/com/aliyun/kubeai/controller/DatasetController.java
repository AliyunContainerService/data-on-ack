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

import com.aliyun.kubeai.entity.FluidDataset;
import com.aliyun.kubeai.exception.K8sCRDNotFoundException;
import com.aliyun.kubeai.model.common.RequestResult;
import com.aliyun.kubeai.model.common.ResultCode;
import com.aliyun.kubeai.model.k8s.K8sFluidDataset;
import com.aliyun.kubeai.service.DatasetService;
import com.aliyun.kubeai.utils.JsonUtil;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;

import javax.annotation.Resource;
import java.util.List;

@Slf4j
@RestController
@RequestMapping("/dataset")
public class DatasetController {

    @Resource
    private DatasetService datasetService;

    @GetMapping("/list")
    public RequestResult<List<K8sFluidDataset>> getDatasets(@RequestParam(name = "name", required = false) String name,
                                                         @RequestParam(name = "namespace", required = false) String namespace) {
        log.info("list datasets by name:{}, ns:{}", name, namespace);
        RequestResult<List<K8sFluidDataset>> result = new RequestResult<>();
        try {
            List<K8sFluidDataset> datasets = datasetService.listFluidDatasets(name, namespace, false);
            log.info("got datasets len:{}", datasets.size());
            result.setData(datasets);
        } catch (K8sCRDNotFoundException e) {
            log.warn("dataset crd not found exception:{}", e.getMessage());
            result.setFailed(ResultCode.DATASET_CRD_NOT_FOUND_EXCEPTION, ResultCode.DATASET_CRD_NOT_FOUND_MESSAGE);
        } catch (Exception e) {
            log.error("get dataset failed:", e);
            result.setFailed(ResultCode.LIST_DATASET_FAILED, "获取数据集异常");
        }
        return result;
    }


    @PostMapping("/create")
    public RequestResult<Void> createDataset(@RequestBody FluidDataset dataset) {
        log.info("create dataset: {}", JsonUtil.getGson().toJson(dataset));
        RequestResult<Void> result = new RequestResult<>();
        try {
            boolean success = datasetService.createFluidDataset(dataset);
            if (!success) {
                result.setFailed(ResultCode.CREATE_DATASET_FAILED, "创建数据集失败");
            }
        } catch (K8sCRDNotFoundException e) {
            log.warn("dataset crd not found exception:{}", e.getMessage());
            result.setFailed(ResultCode.DATASET_CRD_NOT_FOUND_EXCEPTION, ResultCode.DATASET_CRD_NOT_FOUND_MESSAGE);
        } catch (Exception e) {
            log.error("create dataset exception:", e);
            result.setFailed(ResultCode.CREATE_DATASET_FAILED, "创建数据异常");
        }

        return result;
    }

    @PutMapping("/delete")
    public RequestResult<Void> deleteDataset(@RequestParam(name="name", required = true) String name,
                                             @RequestParam(name="namespace", required = true) String namespace) {
        log.info("delete dataset, name:{} namespace:{}", name, namespace);
        RequestResult<Void> result = new RequestResult<>();
        try {
            boolean success = datasetService.deleteFluidDataset(namespace, name);
            if (!success) {
                result.setFailed(ResultCode.DELETE_DATASET_FAILED, "删除数据集失败");
            }
        } catch (K8sCRDNotFoundException e) {
            log.warn("dataset crd not found exception:{}", e.getMessage());
            result.setFailed(ResultCode.DATASET_CRD_NOT_FOUND_EXCEPTION, ResultCode.DATASET_CRD_NOT_FOUND_MESSAGE);
        } catch (Exception e) {
            log.error("delete dataset exception:{}", e);
            result.setFailed(ResultCode.DELETE_DATASET_FAILED, "删除数据集异常");
        }
        return result;
    }
}
