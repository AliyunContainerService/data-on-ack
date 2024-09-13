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

import com.aliyun.kubeai.cluster.AliyunClient;
import com.aliyun.kubeai.cluster.KubeClient;
import com.aliyun.kubeai.entity.JobInstance;
import com.aliyun.kubeai.entity.ServingJob;
import com.aliyun.kubeai.entity.TrainingJob;
import com.aliyun.kubeai.mapper.JobInstanceMapper;
import com.aliyun.kubeai.mapper.ServingJobMapper;
import com.aliyun.kubeai.mapper.TrainingJobMapper;
import com.aliyun.kubeai.model.InstanceInfo;
import com.aliyun.kubeai.utils.DateUtil;
import com.github.kubeflow.arena.client.ArenaClient;
import com.github.kubeflow.arena.enums.ServingJobType;
import com.github.kubeflow.arena.enums.TrainingJobType;
import com.github.kubeflow.arena.exceptions.ArenaException;
import com.github.kubeflow.arena.model.serving.ServingJobInfo;
import com.github.kubeflow.arena.model.training.TrainingJobInfo;
import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import javax.annotation.Resource;
import java.io.IOException;
import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.Date;
import java.util.List;

@Slf4j
@Service
public class JobSyncService {

    @Resource
    private TrainingJobMapper trainingJobMapper;

    @Resource
    private ServingJobMapper servingJobMapper;

    @Resource
    private JobInstanceMapper jobInstanceMapper;

    @Resource
    private AliyunClient aliyunClient;

    @Resource
    private KubeClient kubeClient;

    @Scheduled(fixedDelay = 30000)
    private void checkJobs() throws ArenaException, IOException {
        checkRunningJobs();
        checkFinishedJobs();
        checkFinishedInstances();
    }

    private void checkFinishedInstances() {
        List<JobInstance> jobInstances = jobInstanceMapper.findRunningInstance();
        if (jobInstances != null && !jobInstances.isEmpty()) {
            for (JobInstance jobInstance : jobInstances) {
                if (!kubeClient.isPodExist(jobInstance.getNamespace(), jobInstance.getName())) {
                    resetTrainingJobInstance(jobInstance);
                    jobInstance.setStatus("Terminated");
                    boolean success = jobInstanceMapper.updateJobInstance(jobInstance) > 0;
                    if (success) {
                        log.info("terminate pod");
                    }
                }
            }
        }
    }

    private void checkRunningJobs() throws ArenaException, IOException {
        ArenaClient client = new ArenaClient();
        // training job
        List<TrainingJobInfo> trainingJobInfos = client.training().list(TrainingJobType.AllTrainingJob, true);
        if (!trainingJobInfos.isEmpty()) {
            for (TrainingJobInfo info : trainingJobInfos) {
                TrainingJob trainingJob = trainingJobMapper.findByJobId(info.getUuid());
                if (trainingJob == null) {
                    trainingJob = buildTrainingJob(info);
                    trainingJobMapper.createTrainingJob(trainingJob);
                }

                String jobId = trainingJob.getJobId();
                String namespace = info.getNamespace();
                com.github.kubeflow.arena.model.training.Instance[] instances = info.getInstances();
                for (com.github.kubeflow.arena.model.training.Instance instance : instances) {
                    JobInstance jobInstance = jobInstanceMapper.findJobInstance(jobId, namespace, instance.getName());
                    if (jobInstance != null) {
                        resetTrainingJobInstance(instance, jobInstance);
                        jobInstanceMapper.updateJobInstance(jobInstance);
                    } else {
                        jobInstance = buildTrainingJobInstance(instance, jobId);
                        if (jobInstance != null) {
                            jobInstance.setNamespace(namespace);
                            jobInstanceMapper.createJobInstance(jobInstance);
                        }
                    }
                }

                resetTrainingJob(info, trainingJob);
                trainingJobMapper.updateTrainingJob(trainingJob);
            }
        }

        // serving job
//        List<ServingJobInfo> servingJobInfos = client.serving().list(ServingJobType.AllServingJob, true);
//        if (!servingJobInfos.isEmpty()) {
//            for (ServingJobInfo info : servingJobInfos) {
//                ServingJob servingJob = servingJobMapper.findByJobId(info.getUuid());
//                if (servingJob == null) {
//                    servingJob = buildServingJob(info);
//                    servingJobMapper.createServingJob(servingJob);
//                }
//
//                String jobId = servingJob.getJobId();
//                String namespace = info.getNamespace();
//
//                com.github.kubeflow.arena.model.serving.Instance[] instances = info.getInstances();
//                for (com.github.kubeflow.arena.model.serving.Instance instance : instances) {
//                    JobInstance jobInstance = jobInstanceMapper.findJobInstance(jobId, namespace, instance.getName());
//                    if (jobInstance != null) {
//                        resetServingJobInstance(instance, jobInstance);
//                        jobInstanceMapper.updateJobInstance(jobInstance);
//                    } else {
//                        jobInstance = buildServingJobInstance(instance, jobId);
//                        if (jobInstance != null) {
//                            jobInstance.setNamespace(namespace);
//                            jobInstanceMapper.createJobInstance(jobInstance);
//                        }
//                    }
//                }
//
//                resetServingJob(info, servingJob);
//                servingJobMapper.updateServingJob(servingJob);
//            }
//        }
    }


    private void checkFinishedJobs() throws ArenaException, IOException {
        ArenaClient arenaClient = new ArenaClient();

        boolean success = false;

        // training job
        List<TrainingJob> trainingJobs = trainingJobMapper.findRunningJob();
        for (TrainingJob trainingJob : trainingJobs) {
            TrainingJobType jobType = TrainingJobType.getByAlias(trainingJob.getType());
            TrainingJobInfo trainingJobInfo = arenaClient.training().namespace(trainingJob.getNamespace()).get(trainingJob.getName(), jobType);
            if (trainingJobInfo == null) {
                List<JobInstance> jobInstances = jobInstanceMapper.findByJobId(trainingJob.getJobId());
                for (JobInstance jobInstance : jobInstances) {
                    if (!kubeClient.isPodExist(jobInstance.getNamespace(), jobInstance.getName())) {
                        resetTrainingJobInstance(jobInstance);
                        jobInstance.setStatus("Terminated");
                        success = jobInstanceMapper.updateJobInstance(jobInstance) > 0;
                        if (!success) {
                            log.error("update training job instance failed");
                        }
                    }
                }

                resetTrainingJob(trainingJob);
                trainingJob.setStatus("Terminated");
                success = trainingJobMapper.updateTrainingJob(trainingJob) > 0;
                if (!success) {
                    log.error("update training job to terminated failed");
                }
            }
        }

        // serving job
//        List<ServingJob> servingJobs = servingJobMapper.findRunningJob();
//        for (ServingJob servingJob : servingJobs) {
//            ServingJobType jobType = ServingJobType.getByAlias(servingJob.getType());
//            ServingJobInfo servingJobInfo = arenaClient.serving().namespace(servingJob.getNamespace()).get(servingJob.getName(), jobType, null);
//            if (servingJobInfo == null) {
//                List<JobInstance> jobInstances = jobInstanceMapper.findByJobId(servingJob.getJobId());
//                for (JobInstance jobInstance : jobInstances) {
//                    if (!kubeClient.isPodExist(jobInstance.getNamespace(), jobInstance.getName())) {
//                        resetTrainingJobInstance(jobInstance);
//                        jobInstance.setStatus("Terminated");
//                        success = jobInstanceMapper.updateJobInstance(jobInstance) > 0;
//                        if (!success) {
//                            log.error("update job instance failed");
//                        }
//                    }
//                }
//
//                resetServingJob(servingJob);
//                servingJob.setStatus("Terminated");
//                success = servingJobMapper.updateServingJob(servingJob) > 0;
//                if (!success) {
//                    log.error("update serving job to terminated failed");
//                }
//            }
//        }
    }

    private TrainingJob buildTrainingJob(TrainingJobInfo info) {
        TrainingJob job = new TrainingJob();
        job.setJobId(info.getUuid());
        job.setName(info.getName());
        job.setNamespace(info.getNamespace());
        job.setStatus(formatStatus(info.getStatus().alias()));
        job.setType(info.getTrainer().alias());
        job.setRequestGpus(info.getRequestGPUs());
        job.setAllocatedGpus(info.getAllocatedGPUs());

        Date createTime = DateUtil.unixTimestampToDate(info.getCreationTimestamp());
        String duration = DateUtil.getFormatDuration(createTime);
        job.setDuration(duration);

        job.setCreateTime(createTime);
        job.setModifyTime(createTime);
        return job;
    }

    private ServingJob buildServingJob(ServingJobInfo info) {
        ServingJob job = new ServingJob();
        job.setJobId(info.getUuid());
        job.setName(info.getName());
        job.setNamespace(info.getNamespace());

        job.setType(info.getType().alias());
        job.setStatus("Running");
        job.setReplicas(info.getAvailableInstances());

        Date createTime = DateUtil.unixTimestampToDate(info.getCreationTimestamp());
        String duration = DateUtil.getFormatDuration(createTime);
        job.setDuration(duration);
        job.setCreateTime(createTime);
        job.setModifyTime(createTime);

        return job;
    }

    private void resetTrainingJob(TrainingJobInfo info, TrainingJob trainingJob) {
        trainingJob.setStatus(formatStatus(info.getStatus().alias()));
        String jobDuration = DateUtil.getFormatDuration(trainingJob.getCreateTime());
        trainingJob.setDuration(jobDuration);
        trainingJob.setModifyTime(new Date());

        float tradeCost = 0f;
        float onDemandCost = 0f;
        float coreHour = 0f;
        float gpuHour = 0f;
        List<JobInstance> jobInstances = jobInstanceMapper.findByJobId(trainingJob.getJobId());
        for (JobInstance instance : jobInstances) {
            tradeCost += instance.getTradeCost();
            onDemandCost += instance.getOnDemandCost();
            float hour = getJobRunningHour(instance.getCreateTime());
            coreHour += (instance.getCpuCore() * hour);
            gpuHour += (instance.getGpu() * hour);
        }

        float savedCost = calculateSavedCost(tradeCost, onDemandCost);
        trainingJob.setTradeCost(tradeCost);
        trainingJob.setOnDemandCost(onDemandCost);
        trainingJob.setSavedCost(savedCost);
        trainingJob.setCoreHour(coreHour);
        trainingJob.setGpuHour(gpuHour);
    }

    private void resetTrainingJob(TrainingJob trainingJob) {
        String formatDuration = DateUtil.getFormatDuration(trainingJob.getCreateTime());
        trainingJob.setDuration(formatDuration);
        trainingJob.setAllocatedGpus(0);
        trainingJob.setModifyTime(new Date());

        float tradeCost = 0f;
        float onDemandCost = 0f;
        List<JobInstance> jobInstances = jobInstanceMapper.findByJobId(trainingJob.getJobId());
        for (JobInstance instance : jobInstances) {
            tradeCost += instance.getTradeCost();
            onDemandCost += instance.getOnDemandCost();
        }

        float savedCost = calculateSavedCost(tradeCost, onDemandCost);
        trainingJob.setTradeCost(tradeCost);
        trainingJob.setOnDemandCost(onDemandCost);
        trainingJob.setSavedCost(savedCost);
    }

    private void resetServingJob(ServingJobInfo info, ServingJob servingJob) {
        String jobDuration = DateUtil.getFormatDuration(servingJob.getCreateTime());
        servingJob.setDuration(jobDuration);
        servingJob.setModifyTime(new Date());

        float tradeCost = 0f;
        float onDemandCost = 0f;
        float coreHour = 0f;
        float gpuHour = 0f;
        List<JobInstance> jobInstances = jobInstanceMapper.findByJobId(servingJob.getJobId());
        for (JobInstance instance : jobInstances) {
            tradeCost += instance.getTradeCost();
            onDemandCost += instance.getOnDemandCost();
            float hour = getJobRunningHour(instance.getCreateTime());
            coreHour += (instance.getCpuCore() * hour);
            gpuHour += (instance.getGpu() * hour);
        }

        float savedCost = calculateSavedCost(tradeCost, onDemandCost);
        servingJob.setTradeCost(tradeCost);
        servingJob.setOnDemandCost(onDemandCost);
        servingJob.setSavedCost(savedCost);
        servingJob.setCoreHour(coreHour);
        servingJob.setGpuHour(gpuHour);
    }

    private void resetServingJob(ServingJob servingJob) {
        String formatDuration = DateUtil.getFormatDuration(servingJob.getCreateTime());
        servingJob.setDuration(formatDuration);
        servingJob.setModifyTime(new Date());

        float tradeCost = 0f;
        float onDemandCost = 0f;
        List<JobInstance> jobInstances = jobInstanceMapper.findByJobId(servingJob.getJobId());
        for (JobInstance instance : jobInstances) {
            tradeCost += instance.getTradeCost();
            onDemandCost += instance.getOnDemandCost();
        }

        float savedCost = calculateSavedCost(tradeCost, onDemandCost);
        servingJob.setTradeCost(tradeCost);
        servingJob.setOnDemandCost(onDemandCost);
        servingJob.setSavedCost(savedCost);
    }

    private JobInstance buildTrainingJobInstance(com.github.kubeflow.arena.model.training.Instance training, String jobId) {
        if (training.getStatus().equalsIgnoreCase("Pending")) {
            return null;
        }

        String nodeIp = training.getNodeIP();
        if(nodeIp == null || nodeIp.equals("N/A")) {
            return null;
        }

        JobInstance instance = new JobInstance();
        instance.setJobId(jobId);
        instance.setName(training.getName());
        instance.setNamespace(training.getNamespace());
        instance.setStatus(training.getStatus());

        instance.setNodeName(training.getNode());
        instance.setNodeIp(nodeIp);

        InstanceInfo instanceInfo = aliyunClient.getEcsInstance(training.getNode(), nodeIp);
        if (instanceInfo == null) {
            log.warn("ecs instance not found, name:{} ip:{}", training.getNode(), nodeIp);
            return null;
        }

        instance.setInstanceType(instanceInfo.getInstanceType());
        instance.setResourceType(instanceInfo.getResourceType());
        instance.setSpot(instanceInfo.isSpot());

        int gpu = instanceInfo.getGpu();
        int requestGpus = training.getRequestGPUs();

        float tradePrice = instanceInfo.getTradePrice();
        float onDemandPrice = instanceInfo.getOnDemandPrice();
        if (requestGpus > 0) {
            tradePrice = tradePrice * ((float)requestGpus / gpu);
            onDemandPrice = onDemandPrice * ((float)requestGpus / gpu);
        }

        instance.setTradePrice(tradePrice);
        instance.setOnDemandPrice(onDemandPrice);

        long createTimestamp = training.getCreationTimestamp();
        long durationSecond = System.currentTimeMillis() / 1000 - createTimestamp;
        Date createTime = DateUtil.unixTimestampToDate(createTimestamp);
        String formatDuration = DateUtil.getFormatDuration(createTime);
        float tradeCost = formatDecimal(tradePrice * durationSecond);
        float onDemandCost = formatDecimal(onDemandPrice * durationSecond);
        float savedCost = calculateSavedCost(tradeCost, onDemandCost);

        instance.setTradeCost(tradeCost);
        instance.setOnDemandCost(onDemandCost);
        instance.setSavedCost(savedCost);

        instance.setDuration(formatDuration);
        instance.setCreateTime(createTime);
        instance.setModifyTime(createTime);

        instance.setCpuCore(instanceInfo.getCpuCore());
        instance.setGpu(instanceInfo.getGpu());

        return instance;
    }

    private JobInstance buildServingJobInstance(com.github.kubeflow.arena.model.serving.Instance serving, String jobId) {
        String nodeIp = serving.getNodeIP();
        if(nodeIp == null || nodeIp.equals("N/A")) {
            return null;
        }

        JobInstance instance = new JobInstance();
        instance.setJobId(jobId);
        instance.setName(serving.getName());
        instance.setNamespace(serving.getNamespace());
        instance.setStatus(serving.getStatus());

        instance.setNodeName(serving.getNodeName());
        instance.setNodeIp(serving.getNodeIP());

        InstanceInfo instanceInfo = aliyunClient.getEcsInstance(serving.getNodeName(), nodeIp);
        if (instanceInfo == null) {
            log.warn("ecs instance not found, name:{} ip:{}", serving.getNodeName(), nodeIp);
            return null;
        }

        instance.setInstanceType(instanceInfo.getInstanceType());
        instance.setResourceType(instanceInfo.getResourceType());
        instance.setSpot(instanceInfo.isSpot());

        int gpu = instanceInfo.getGpu();
        float cpuCore = instanceInfo.getCpuCore();
        int requestGpus = serving.getRequestGPUs();
        float requestCpus = serving.getRequestCPUs();
        float tradePrice = instanceInfo.getTradePrice();
        float onDemandPrice = instanceInfo.getOnDemandPrice();
        if (requestGpus > 0) {
            tradePrice = tradePrice * ((float)requestGpus / gpu);
            onDemandPrice = onDemandPrice * ((float)requestGpus / gpu);
        } else if (requestCpus > 0f) {
            tradePrice = tradePrice * (requestCpus / cpuCore);
            onDemandPrice = onDemandPrice * (requestCpus / cpuCore);
        }

        instance.setTradePrice(tradePrice);
        instance.setOnDemandPrice(onDemandPrice);

        long createTimestamp = serving.getCreationTimestamp();
        long durationSecond = System.currentTimeMillis() / 1000 - createTimestamp;
        Date createTime = DateUtil.unixTimestampToDate(createTimestamp);
        String formatDuration = DateUtil.getFormatDuration(createTime);
        float tradeCost = formatDecimal(tradePrice * durationSecond);
        float onDemandCost = formatDecimal(onDemandPrice * durationSecond);
        float savedCost = calculateSavedCost(tradeCost, onDemandCost);

        instance.setDuration(formatDuration);
        instance.setTradeCost(tradeCost);
        instance.setOnDemandCost(onDemandCost);
        instance.setSavedCost(savedCost);

        instance.setCreateTime(createTime);
        instance.setModifyTime(createTime);

        instance.setCpuCore(instanceInfo.getCpuCore());
        instance.setGpu(instanceInfo.getGpu());

        return instance;
    }

    private void resetTrainingJobInstance(JobInstance instance) {
        Date now = new Date();
        long durationSecond = DateUtil.getDurationSecond(instance.getCreateTime(), now);
        float tradeCost = formatDecimal(instance.getTradePrice() * durationSecond);
        float onDemandCost = formatDecimal(instance.getOnDemandPrice() * durationSecond);
        float savedCost = calculateSavedCost(tradeCost, onDemandCost);

        String formatDuration = DateUtil.getFormatDuration(instance.getCreateTime());
        instance.setDuration(formatDuration);
        instance.setTradeCost(tradeCost);
        instance.setOnDemandCost(onDemandCost);
        instance.setSavedCost(savedCost);
        instance.setModifyTime(now);
    }

    private void resetTrainingJobInstance(com.github.kubeflow.arena.model.training.Instance training, JobInstance instance) {
        Date now = new Date();
        instance.setStatus(training.getStatus());

        if(instance.getTradePrice() == 0 || instance.getOnDemandPrice() == 0) {
            InstanceInfo instanceInfo = aliyunClient.getEcsInstance(training.getNode(), training.getNodeIP());
            if (instanceInfo != null) {
                int gpu = instanceInfo.getGpu();
                int requestGpus = training.getRequestGPUs();

                float tradePrice = instanceInfo.getTradePrice();
                float onDemandPrice = instanceInfo.getOnDemandPrice();

                if (requestGpus > 0) {
                    tradePrice = tradePrice * ((float)requestGpus / gpu);
                    onDemandPrice = onDemandPrice * ((float)requestGpus / gpu);
                }

                instance.setTradePrice(tradePrice);
                instance.setOnDemandPrice(onDemandPrice);
            }
        }

        String formatDuration = DateUtil.getFormatDuration(instance.getCreateTime());
        long durationSecond = DateUtil.getDurationSecond(instance.getCreateTime(), now);
        float tradeCost = formatDecimal(instance.getTradePrice() * durationSecond);
        float onDemandCost = formatDecimal(instance.getOnDemandPrice() * durationSecond);
        float savedCost = calculateSavedCost(tradeCost, onDemandCost);

        instance.setDuration(formatDuration);
        instance.setTradeCost(tradeCost);
        instance.setOnDemandCost(onDemandCost);
        instance.setSavedCost(savedCost);
        instance.setModifyTime(now);
    }

    private void resetServingJobInstance(com.github.kubeflow.arena.model.serving.Instance serving, JobInstance instance) {
        Date now = new Date();
        instance.setStatus(serving.getStatus());

        if(instance.getTradePrice() == 0 || instance.getOnDemandPrice() == 0) {
            InstanceInfo instanceInfo = aliyunClient.getEcsInstance(serving.getNodeName(), serving.getNodeIP());
            if (instanceInfo != null) {
                int gpu = instanceInfo.getGpu();
                int requestGpus = serving.getRequestGPUs();

                float tradePrice = instanceInfo.getTradePrice();
                float onDemandPrice = instanceInfo.getOnDemandPrice();
                if (requestGpus > 0) {
                    tradePrice = tradePrice * ((float)requestGpus / gpu);
                    onDemandPrice = onDemandPrice * ((float)requestGpus / gpu);
                }

                instance.setTradePrice(tradePrice);
                instance.setOnDemandPrice(onDemandPrice);
            }
        }

        String formatDuration = DateUtil.getFormatDuration(instance.getCreateTime());
        long durationSecond = DateUtil.getDurationSecond(instance.getCreateTime(), now);
        float tradeCost = formatDecimal(instance.getTradePrice() * durationSecond);
        float onDemandCost = formatDecimal(instance.getOnDemandPrice() * durationSecond);
        float savedCost = calculateSavedCost(tradeCost, onDemandCost);

        instance.setDuration(formatDuration);
        instance.setTradeCost(tradeCost);
        instance.setOnDemandCost(onDemandCost);
        instance.setSavedCost(savedCost);
        instance.setModifyTime(new Date());
    }

    private float calculateSavedCost(float tradeCost, float onDemandCost) {
        if (onDemandCost <= 0f) {
            return 0f;
        }
        float save = (onDemandCost - tradeCost) / onDemandCost;
        BigDecimal b = new BigDecimal(save);
        return b.setScale(2, RoundingMode.HALF_UP).floatValue();
    }

    private String formatStatus(String status) {
        String firstLetter = status.substring(0, 1).toUpperCase();
        String restLetters = status.substring(1).toLowerCase();
        return firstLetter + restLetters;
    }

    private float formatDecimal(float f) {
        BigDecimal bd = new BigDecimal(f);
        return bd.setScale(6, RoundingMode.HALF_UP).floatValue();
    }

    private float getJobRunningHour(Date createTime) {
        long deltaSeconds = (new Date().getTime() - createTime.getTime()) / 1000;
        float secondsPerHour = 3600f;
        return deltaSeconds / secondsPerHour;
    }

}
