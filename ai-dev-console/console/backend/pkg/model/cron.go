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

type CronInfo struct {
	Name              string `json:"name,omitempty"`
	Namespace         string `json:"namespace,omitempty"`
	Kind              string `json:"type,omitempty"`
	Schedule          string `json:"schedule,omitempty"`
	ConcurrencyPolicy string `json:"concurrencyPolicy,omitempty"`
	Suspend           string `json:"suspend,omitempty"`
	Status            string `json:"status,omitempty"`
	Deadline          string `json:"deadline,omitempty"`
	HistoryLimit      int64  `json:"historyLimit,omitempty"`
	LastScheduleTime  string `json:"lastScheduleTime,omitempty"`
	CreateTime        string `json:"createTime,omitempty"`
}

type CronHistoryInfo struct {
	Name       string `json:"name,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Kind       string `json:"type,omitempty"`
	CreateTime string `json:"createTime,omitempty"`
}
