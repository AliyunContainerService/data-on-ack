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
limitations under the License.
*/

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/tool"
)

// WriteToolNames lists every mutating tool. The permission engine registers
// Ask rules for exactly these names, so the framework emits a confirmation
// request before execution (design §3.3, P2).
var WriteToolNames = []string{
	"create_notebook",
	"stop_notebook",
	"start_notebook",
	"submit_training_job",
	"stop_training_job",
}

func isWriteTool(name string) bool {
	for _, n := range WriteToolNames {
		if n == name {
			return true
		}
	}
	return false
}

func strMapArg(input map[string]any, key string) map[string]string {
	raw, ok := input[key].(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// NewWriteToolkit builds the P2 mutating tools. Every call still passes the
// same independent ownership validation as read tools; the HITL confirmation
// is enforced by the permission engine BEFORE these functions run.
func NewWriteToolkit(env *toolEnv) []tool.Tool {
	return []tool.Tool{
		tool.NewFunctionTool(
			"create_notebook",
			"Create a Jupyter/VSCode notebook for the user in one of the allowed namespaces. "+
				"gpu is an integer count; cpu/memory are Kubernetes quantities (e.g. \"4\", \"16Gi\").",
			schema(`"namespace":{"type":"string"},`+
				`"name":{"type":"string","description":"lowercase RFC1123 name"},`+
				`"image":{"type":"string"},`+
				`"cpu":{"type":"string","description":"e.g. 4"},`+
				`"memory":{"type":"string","description":"e.g. 16Gi"},`+
				`"gpu":{"type":"integer"},`+
				`"gpuType":{"type":"string","description":"optional GPU model selector"},`+
				`"storage":{"type":"string","description":"optional workdir PVC size, e.g. 50Gi"},`+
				`"env":{"type":"object","description":"optional environment variables"}`,
				"namespace", "name", "image", "cpu", "memory"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns := strArg(input, "namespace")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				name := strArg(input, "name")
				if !isRFC1123Name(name) {
					return nil, fmt.Errorf("invalid notebook name %q (must be lowercase RFC1123)", name)
				}
				spec := &model.NotebookSpec{
					Name:      name,
					Namespace: ns,
					Image:     strArg(input, "image"),
					CPU:       strArg(input, "cpu"),
					Memory:    strArg(input, "memory"),
					GPU:       intArg(input, "gpu", 0),
					GPUType:   strArg(input, "gpuType"),
					Storage:   strArg(input, "storage"),
					Env:       strMapArg(input, "env"),
				}
				if spec.Image == "" || spec.CPU == "" || spec.Memory == "" {
					return nil, fmt.Errorf("image, cpu and memory are required")
				}
				if err := env.Notebooks.Create(spec, env.UserName); err != nil {
					return nil, err
				}
				return map[string]any{
					"created": true, "namespace": ns, "name": name,
					"hint": "notebook is being provisioned; use get_notebook to poll its status",
				}, nil
			},
		),
		tool.NewFunctionTool(
			"stop_notebook",
			"Stop (scale down) the user's notebook. The notebook object and data are kept.",
			schema(`"namespace":{"type":"string"},"name":{"type":"string"}`, "namespace", "name"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, name := strArg(input, "namespace"), strArg(input, "name")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				if err := env.Notebooks.Stop(name, ns, env.UserName); err != nil {
					return nil, err
				}
				return map[string]any{"stopped": true, "namespace": ns, "name": name}, nil
			},
		),
		tool.NewFunctionTool(
			"start_notebook",
			"Start (resume) a previously stopped notebook.",
			schema(`"namespace":{"type":"string"},"name":{"type":"string"}`, "namespace", "name"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, name := strArg(input, "namespace"), strArg(input, "name")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				if err := env.Notebooks.Start(name, ns, env.UserName); err != nil {
					return nil, err
				}
				return map[string]any{"started": true, "namespace": ns, "name": name}, nil
			},
		),
		tool.NewFunctionTool(
			"submit_training_job",
			"Submit a distributed training job (TFJob/PyTorchJob/MPIJob/RayJob) for the user. "+
				"Use this both for new jobs and for RESUBMITTING a fixed version of a failed job "+
				"(choose a new name, e.g. <old-name>-fix1).",
			schema(`"namespace":{"type":"string"},`+
				`"name":{"type":"string","description":"lowercase RFC1123 name"},`+
				`"kind":{"type":"string","description":"TFJob|PyTorchJob|MPIJob|RayJob"},`+
				`"image":{"type":"string"},`+
				`"command":{"type":"string","description":"container command line"},`+
				`"script":{"type":"string","description":"optional multi-line script content mounted via ConfigMap"},`+
				`"workerCount":{"type":"integer"},`+
				`"workerCpu":{"type":"string"},`+
				`"workerMemory":{"type":"string"},`+
				`"workerGpu":{"type":"integer"},`+
				`"psCount":{"type":"integer","description":"parameter servers (TF only)"},`+
				`"psCpu":{"type":"string"},"psMemory":{"type":"string"},`+
				`"env":{"type":"object"},`+
				`"shmSize":{"type":"string","description":"e.g. 8Gi"},`+
				`"queue":{"type":"string","description":"optional ElasticQuota queue"}`,
				"namespace", "name", "kind", "image", "command", "workerCount", "workerCpu", "workerMemory"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns := strArg(input, "namespace")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				name := strArg(input, "name")
				if !isRFC1123Name(name) {
					return nil, fmt.Errorf("invalid job name %q (must be lowercase RFC1123)", name)
				}
				kind := strArg(input, "kind")
				switch kind {
				case "TFJob", "PyTorchJob", "MPIJob", "RayJob":
				default:
					return nil, fmt.Errorf("unsupported job kind %q", kind)
				}
				spec := &model.TrainingJobSpec{
					Name:         name,
					Namespace:    ns,
					Kind:         kind,
					Image:        strArg(input, "image"),
					Command:      strArg(input, "command"),
					Script:       strArg(input, "script"),
					WorkerCount:  int32(intArg(input, "workerCount", 1)),
					WorkerCPU:    strArg(input, "workerCpu"),
					WorkerMemory: strArg(input, "workerMemory"),
					WorkerGPU:    intArg(input, "workerGpu", 0),
					PSCount:      int32(intArg(input, "psCount", 0)),
					PSCPU:        strArg(input, "psCpu"),
					PSMemory:     strArg(input, "psMemory"),
					Env:          strMapArg(input, "env"),
					ShmSize:      strArg(input, "shmSize"),
					Queue:        strArg(input, "queue"),
				}
				if spec.Image == "" || spec.Command == "" {
					return nil, fmt.Errorf("image and command are required")
				}
				if spec.WorkerCount < 1 {
					spec.WorkerCount = 1
				}
				if err := env.Training.Create(spec, env.UserName); err != nil {
					return nil, err
				}
				return map[string]any{
					"submitted": true, "namespace": ns, "name": name, "kind": kind,
					"hint": "use get_training_job / get_job_events to follow progress",
				}, nil
			},
		),
		tool.NewFunctionTool(
			"stop_training_job",
			"Delete (stop) a training job and its pods.",
			schema(`"namespace":{"type":"string"},"name":{"type":"string"},"kind":{"type":"string"}`,
				"namespace", "name", "kind"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, name, kind := strArg(input, "namespace"), strArg(input, "name"), strArg(input, "kind")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				if err := env.Training.Delete(name, ns, kind, env.UserName); err != nil {
					return nil, err
				}
				return map[string]any{"stopped": true, "namespace": ns, "name": name}, nil
			},
		),
	}
}

// isRFC1123Name guards resource names against anything outside the safe set,
// so model-proposed names cannot smuggle path/label-injection payloads.
func isRFC1123Name(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' && i != 0:
		default:
			return false
		}
	}
	return !strings.HasSuffix(s, "-")
}

var _ = json.Marshal
