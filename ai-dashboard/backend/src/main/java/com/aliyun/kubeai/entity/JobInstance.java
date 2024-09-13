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
 * 任务实例
 */
@Data
public class JobInstance extends BaseEntity {
    /**
     * 作业ID
     */
    private String jobId;
    /**
     * Pod
     */
    private String name;
    /**
     * 命名空间
     */
    private String namespace;
    /**
     * Pod运行时长
     */
    private String duration;
    /**
     * 节点名称
     */
    private String nodeName;
    /**
     * 节点IP
     */
    private String nodeIp;
    /**
     * 实例规格
     */
    private String instanceType;
    /**
     * 资源类型，ecs/eci
     */
    private String resourceType;
    /**
     * 是否spot
     */
    private boolean isSpot;
    /**
     * 实际价格
     */
    private float tradePrice;
    /**
     * 按量付费价格
     */
    private float onDemandPrice;
    /**
     * 实际费用
     */
    private float tradeCost;
    /**
     * 按量付费费用
     */
    private float onDemandCost;
    /**
     * 节省比率
     */
    private float savedCost;
    /**
     * cpu核数
     */
    private float cpuCore;
    /**
     * gpu卡数
     */
    private int gpu;

    /**
     * 非数据库字段
     */
    private String formatTime;
}
