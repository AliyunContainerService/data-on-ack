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
 * 数据集，pvc
 */
@Data
public class Dataset extends BaseEntity {
    private String name; //数据集名称
    private String sourceName; //来源名称
    private String sourceType; //来源类型 pvc或storage
    private String accessConf; //secret name
    private String namespace;
    private boolean accelerated; //是否加速
    private String acceleratedName; //加速后数据集名
    private String runtimeConf; //runtime conf for fluid
}
