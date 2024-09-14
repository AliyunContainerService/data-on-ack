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

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/dmo"
)

type EvaluateJobInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Image        string `json:"image"`
	ModelName    string `json:"modelName"`
	ModelVersion string `json:"modelVersion"`
	Metrics      string `json:"metrics"`

	Status       string `json:"jobStatus"`
	CreateTime   string `json:"createTime"`
	ModifiedTime string `json:"ModifyTime"`
}

type SubmitEvaluateJobArgs struct {
	dmo.SubmitEvaluateJobInfo `json:",inline"`
}
