/*
Copyright 2020 The Alibaba Authors.

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

package v1alpha1

import v1 "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"

const (
	DefaultContainerName                 = "mars"
	DefaultPortName                      = "mars-port"
	DefaultPort                          = 11111
	DefaultCacheSizePercentage     int32 = 45
	DefaultCacheMountPath                = "/dev/shm"
	DefaultSchedulerRestartPolicy        = v1.RestartPolicyNever
	DefaultWebServiceRestartPolicy       = v1.RestartPolicyAlways
	DefaultWorkerRestartPolicy           = v1.RestartPolicyExitCode
)
