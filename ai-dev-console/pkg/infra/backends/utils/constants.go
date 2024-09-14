/*
Copyright 2021 The Alibaba Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import apiv1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"

const (
	// Expanded object statuses only used in persistent layer.
	PodStopped = "Stopped"
	// JobStopped means the job has been stopped manually by user,
	// a job can be stopped only when job has not reached a final
	// state(Succeed/Failed).
	JobStopped apiv1.JobConditionType = "Stopped"
	// JobStopping is a intermediate state before truly Stopped.
	JobStopping apiv1.JobConditionType = "Stopping"
)
