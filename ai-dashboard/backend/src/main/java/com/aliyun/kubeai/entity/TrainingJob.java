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
    
package com.aliyun.kubeai.entity;

import lombok.Data;

/**
 * 训练任务
 */
@Data
public class TrainingJob extends BaseEntity {
    /**
     * 作业ID
     */
    private String jobId;
    /**
     * 作业名称
     */
    private String name;
    /**
     * 命名空间
     */
    private String namespace;
    /**
     * 运行时间
     */
    private String duration;
    /**
     * 作业类型
     */
    private String type;
    /**
     * 请求gpu数量
     */
    private int requestGpus;
    /**
     * 分配gpu数量
     */
    private int allocatedGpus;
    /**
     * 实际费用
     */
    private float tradeCost;
    /**
     * 按量费用
     */
    private float onDemandCost;
    /**
     * 节省比率
     */
    private float savedCost;
    /**
     * cpu核时
     */
    private float coreHour;
    /**
     * gpu卡时
     */
    private float gpuHour;

    /**
     * 非数据库字段
     */
    private String formatTime;
}
