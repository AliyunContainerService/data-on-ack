// Copyright 2018 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	common "github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/job_controller/api/v1"
)

const (
	TFJobKind = "TFJob"
	// DefaultPortName is name of the port used to communicate between PS and
	// workers.
	TFJobDefaultPortName = "tfjob-port"
	// DefaultContainerName is the name of the TFJob container.
	TFJobDefaultContainerName = "tensorflow"
	// DefaultPort is default value of the port.
	TFJobDefaultPort = 2222
	// DefaultRestartPolicy is default RestartPolicy for TFReplicaSpec.
	TFJobDefaultRestartPolicy = common.RestartPolicyExitCode
)
