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
    
package com.aliyun.kubeai.model.common;

public interface ResultCode {
    int OK = 10000;

    // login
    int LOGIN_FAILED = 10101;
    int ILLEGAL_TOKEN = 10102;
    int TOKEN_EXPIRED = 10103;
    int USER_NOT_LOGIN = 10104;
    int USER_NOT_FOUND = 10105;
    int USER_AUTH_FAILED = 10106;
    int USER_LIST_RAM_EXCEPTION = 10107;

    // group
    int CREATE_GROUP_FAILED = 10201;
    int UPDATE_GROUP_FAILED = 10202;
    int DELETE_GROUP_FAILED = 10203;
    int DELETE_GROUP_EXCEPTION = 10204;
    int LIST_GROUP_FAILED = 10205;
    int GROUP_REQUSET_INVALID = 10206;
    int GROUP_CRD_NOT_FOUND_EXCEPTION = 10207;
    String GROUP_CRD_NOT_FOUND_MESSAGE = "请先升级集群版本>=1.20";

    // dataset
    int CREATE_DATASET_FAILED = 10301;
    int DELETE_DATASET_FAILED = 10302;
    int LIST_DATASET_FAILED = 10303;
    int CREATE_DATASET_EXCEPTION = 10304;
    int DATASET_CRD_NOT_FOUND_EXCEPTION = 10305;
    String DATASET_CRD_NOT_FOUND_MESSAGE = "请先安装ack-fluid组件";

    // researcher
    int CREATE_RESEARCHER_FAILED = 10401;
    int UPDATE_RESEARCHER_FAILED = 10402;
    int DELETE_RESEARCHER_FAILED = 10403;
    int UPDATE_RESEARCHER_EXCEPTION = 10404;
    int DELETE_RESEARCHER_EXCEPTION = 10405;
    int GET_RESEARCHER_TOKEN_FAILED = 10406;
    int GET_RESEARCHER_TOKEN_EXCEPTION = 10407;


    // k8s
    int LIST_PVC_FAILED = 10501;
    int LIST_SECRET_FAILED = 10502;
    int LIST_NAMESPACE_FAILED = 10503;

    // user group
    int CREATE_USER_GROUP_FAILED = 10601;
    int CREATE_USER_GROUP_EXCEPTION = 10602;
    int UPDATE_USER_GROUP_NOT_FOUND = 10603;
    int UPDATE_USER_GROUP_FAILED = 10611;
    int UPDATE_USER_GROUP_EXCEPTION = 10612;
    int DELETE_USER_GROUP_FAILED = 10621;
    int DELETE_USER_GROUP_EXCEPTION = 10622;
    int GET_GROUP_NAMESPACES_EXCEPTION = 10623;


    // dashboard
    int GET_GRAFANA_FAILED = 10701;
}
