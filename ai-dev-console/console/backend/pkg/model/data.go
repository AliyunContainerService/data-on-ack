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
    
package model

type ClusterTotalResource struct {
	TotalCPU    int64 `json:"totalCPU"`
	TotalMemory int64 `json:"totalMemory"`
	TotalGPU    int64 `json:"totalGPU"`
}

type ClusterRequestResource struct {
	RequestCPU    int64 `json:"requestCPU"`
	RequestMemory int64 `json:"requestMemory"`
	RequestGPU    int64 `json:"requestGPU"`
}

type ClusterNodeInfo struct {
	NodeName      string `json:"nodeName"`
	InstanceType  string `json:"instanceType"`
	GPUType       string `json:"gpuType"`
	TotalCPU      int64  `json:"totalCPU"`
	TotalMemory   int64  `json:"totalMemory"`
	TotalGPU      int64  `json:"totalGPU"`
	RequestCPU    int64  `json:"requestCPU"`
	RequestMemory int64  `json:"requestMemory"`
	RequestGPU    int64  `json:"requestGPU"`
}

type ClusterNodeInfoList struct {
	Items []ClusterNodeInfo `json:"items,omitempty"`
}

type ArmsInfo struct {
	ClusterId  string `json:"clusterId,omitempty"`
	ArmsUrl    string `json:"armsUrl,omitempty"`
	ArmsRegion string `json:"armsRegion,omitempty"`
	UserId     string `json:"aid,omitempty"`
}
