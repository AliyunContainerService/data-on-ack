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

import com.aliyun.kubeai.entity.ServingJob;
import com.aliyun.kubeai.entity.TrainingJob;
import com.aliyun.kubeai.model.common.Pagination;
import com.aliyun.kubeai.model.common.RequestResult;
import com.aliyun.kubeai.service.JobService;
import com.aliyun.kubeai.vo.JobCost;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import javax.annotation.Resource;

@Slf4j
@RestController
@RequestMapping("/job")
public class JobController {

    @Resource
    private JobService jobService;

    @GetMapping("/training")
    public RequestResult<Pagination<TrainingJob>> listTrainingJob(@RequestParam(name = "name", required = false) String name,
                                                                  @RequestParam(name = "page", required = false) Integer page,
                                                                  @RequestParam(name = "limit", required = false) Integer limit) {
        log.info("list training job, name:{} page:{} limit:{}", name, page, limit);
        RequestResult<Pagination<TrainingJob>> result = new RequestResult<>();
        Pagination<TrainingJob> pagination = jobService.listTrainingJobByPage(name, page, limit);
        result.setData(pagination);
        return result;
    }

    @GetMapping("/serving")
    public RequestResult<Pagination<ServingJob>> listServingJob(@RequestParam(name = "name", required = false) String name,
                                                                @RequestParam(name = "page", required = false) Integer page,
                                                                @RequestParam(name = "limit", required = false) Integer limit) {
        log.info("list serving job, name:{} page:{} limit:{}", name, page, limit);
        RequestResult<Pagination<ServingJob>> result = new RequestResult<>();
        Pagination<ServingJob> pagination = jobService.listServingJobByPage(name, page, limit);
        result.setData(pagination);
        return result;
    }

    @GetMapping("/cost")
    public RequestResult<JobCost> getJobCost(@RequestParam String jobId,
                                             @RequestParam String jobType) {
        log.info("get job cost, jobId:{} jobType:{}", jobId, jobType);
        RequestResult<JobCost> result = new RequestResult<>();
        JobCost jobCost = jobService.getJobCost(jobId, jobType);
        result.setData(jobCost);
        return result;
    }
}
