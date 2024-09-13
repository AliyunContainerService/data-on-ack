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
    
package com.aliyun.kubeai.vo;

import com.aliyun.kubeai.entity.JobInstance;
import lombok.Data;

import java.util.List;

@Data
public class JobCost {
    /**
     * 运行时长
     */
    private String duration;
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
     * 核时
     */
    private float coreHour;
    /**
     * 非数据库字段
     */
    private String formatTime;
    /**
     * Pod列表
     */
    private List<JobInstance> instances;
}
