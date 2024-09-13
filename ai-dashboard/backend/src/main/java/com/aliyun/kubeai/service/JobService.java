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
    
package com.aliyun.kubeai.service;

import com.aliyun.kubeai.entity.JobInstance;
import com.aliyun.kubeai.entity.ServingJob;
import com.aliyun.kubeai.entity.TrainingJob;
import com.aliyun.kubeai.mapper.JobInstanceMapper;
import com.aliyun.kubeai.mapper.ServingJobMapper;
import com.aliyun.kubeai.mapper.TrainingJobMapper;
import com.aliyun.kubeai.model.common.Pagination;
import com.aliyun.kubeai.vo.JobCost;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.util.List;

@Slf4j
@Service
public class JobService {

    private static final String TRAINING_JOB = "training";

    @Resource
    private TrainingJobMapper trainingJobMapper;

    @Resource
    private ServingJobMapper servingJobMapper;

    @Resource
    private JobInstanceMapper jobInstanceMapper;

    public Pagination<TrainingJob> listTrainingJobByPage(String name, int page, int limit) {
        Pagination<TrainingJob> pagination = new Pagination<>();
        long count = trainingJobMapper.countTrainingJob(name);
        pagination.setTotal(count);
        if (count > 0) {
            long offset = (page - 1) * limit;
            List<TrainingJob> list = trainingJobMapper.findTrainingJobByPage(name, offset, limit);
            for (TrainingJob job : list) {
                job.setFormatTime(job.getDuration());
            }
            pagination.setItems(list);
        }
        return pagination;
    }

    public Pagination<ServingJob> listServingJobByPage(String name, int page, int limit) {
        Pagination<ServingJob> pagination = new Pagination<>();
        long count = servingJobMapper.countServingJob(name);
        pagination.setTotal(count);
        if (count > 0) {
            long offset = (page - 1) * limit;
            List<ServingJob> list = servingJobMapper.findServingJobByPage(name, offset, limit);
            for (ServingJob job : list) {
                job.setFormatTime(job.getDuration());
            }
            pagination.setItems(list);
        }
        return pagination;
    }

    public JobCost getJobCost(String jobId, String jobType) {
        JobCost jobCost = new JobCost();

        if (jobType.equals(TRAINING_JOB)) {
            TrainingJob trainingJob = trainingJobMapper.findByJobId(jobId);
            jobCost.setDuration(trainingJob.getDuration());
            jobCost.setTradeCost(trainingJob.getTradeCost());
            jobCost.setOnDemandCost(trainingJob.getOnDemandCost());
            jobCost.setSavedCost(trainingJob.getSavedCost());
            jobCost.setCoreHour(trainingJob.getCoreHour());
        } else {
            ServingJob servingJob = servingJobMapper.findByJobId(jobId);
            jobCost.setDuration(servingJob.getDuration());
            jobCost.setTradeCost(servingJob.getTradeCost());
            jobCost.setOnDemandCost(servingJob.getOnDemandCost());
            jobCost.setSavedCost(servingJob.getSavedCost());
            jobCost.setCoreHour(servingJob.getCoreHour());
        }

        jobCost.setFormatTime(jobCost.getDuration());

        List<JobInstance> jobInstances = jobInstanceMapper.findByJobId(jobId);
        for (JobInstance jobInstance : jobInstances) {
            jobInstance.setFormatTime(jobCost.getDuration());
        }
        jobCost.setInstances(jobInstances);
        return jobCost;
    }


}
