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
    
package com.aliyun.kubeai.mapper;

import com.aliyun.kubeai.entity.TrainingJob;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;

import java.util.List;

@Mapper
public interface TrainingJobMapper {

    int createTrainingJob(TrainingJob trainingJob);

    int updateTrainingJob(TrainingJob trainingJob);

    TrainingJob findByJobId(String jobId);

    List<TrainingJob> findByName(String jobName);

    List<TrainingJob> findTrainingJob(@Param("namespace") String namespace, @Param("jobName") String jobName);

    long countTrainingJob(String jobName);

    List<TrainingJob> findTrainingJobByPage(@Param("jobName") String jobName,
                                            @Param("offset") long offset,
                                            @Param("limit") int limit);

    List<TrainingJob> findByStatus(String status);

    List<TrainingJob> findRunningJob();
}
