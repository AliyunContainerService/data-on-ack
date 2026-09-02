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

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/alanfokco/agentscope-go/v2/pkg/agentscope/tool"
)

// maxLogBytes bounds how much log text a single tool result may carry.
// Truncation must happen here (first hop), not in the model context: the
// framework's default per-tool-result budget is far larger than any sensible
// prompt share.
const maxLogBytes = 32 * 1024

// toolEnv is the per-user tool execution environment. Every tool call is
// validated against AllowedNamespaces independently of any upstream check:
// the console read path uses an admin client, so RBAC is NOT a backstop for
// reads (design §3.2 / security review B).
type toolEnv struct {
	UserName          string
	AllowedNamespaces []string

	Notebooks *service.NotebookService
	Training  *service.TrainingService
	Serving   *service.ServingService
	Metrics   *service.MetricsService
	Quota     *service.QuotaService
}

func (e *toolEnv) allowNS(ns string) error {
	for _, a := range e.AllowedNamespaces {
		if a == ns {
			return nil
		}
	}
	return fmt.Errorf("namespace %q is not in your allowed scope", ns)
}

func strArg(input map[string]any, key string) string {
	if v, ok := input[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func intArg(input map[string]any, key string, def int) int {
	switch v := input[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

func schema(props string, required ...string) json.RawMessage {
	req := ""
	if len(required) > 0 {
		b, _ := json.Marshal(required)
		req = `,"required":` + string(b)
	}
	return json.RawMessage(`{"type":"object","properties":{` + props + `}` + req + `}`)
}

// NewReadOnlyToolkit builds the P1 read-only console toolkit bound to one
// user's identity and namespace scope. Write tools are intentionally NOT
// included yet (P2, gated by the confirmation-ticket mechanism).
func NewReadOnlyToolkit(env *toolEnv) *tool.Toolkit {
	tools := []tool.Tool{
		tool.NewFunctionTool(
			"list_training_jobs",
			"List the user's training jobs (TFJob/PyTorchJob/MPIJob/XGBoostJob/RayJob) across the allowed namespaces. Optional kind filter.",
			schema(`"kind":{"type":"string","description":"optional: TFJob|PyTorchJob|MPIJob|XGBoostJob|RayJob"}`),
			func(ctx context.Context, input map[string]any) (any, error) {
				return env.Training.List(env.AllowedNamespaces, strArg(input, "kind"))
			},
		),
		tool.NewFunctionTool(
			"get_training_job",
			"Get one training job's detail (status, conditions, replica specs).",
			schema(`"namespace":{"type":"string"},"name":{"type":"string"},"kind":{"type":"string"}`,
				"namespace", "name", "kind"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, name, kind := strArg(input, "namespace"), strArg(input, "name"), strArg(input, "kind")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				return env.Training.Get(name, ns, kind)
			},
		),
		tool.NewFunctionTool(
			"list_job_pods",
			"List the pods belonging to a training job.",
			schema(`"namespace":{"type":"string"},"name":{"type":"string"},"kind":{"type":"string"}`,
				"namespace", "name", "kind"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, name, kind := strArg(input, "namespace"), strArg(input, "name"), strArg(input, "kind")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				return env.Training.ListPods(ns, name, kind)
			},
		),
		tool.NewFunctionTool(
			"get_job_events",
			"Get Kubernetes events of a training job (scheduling failures, image pull errors, OOM kills...).",
			schema(`"namespace":{"type":"string"},"name":{"type":"string"}`, "namespace", "name"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, name := strArg(input, "namespace"), strArg(input, "name")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				return env.Training.GetEvents(ns, name)
			},
		),
		tool.NewFunctionTool(
			"get_job_logs",
			"Read logs of a training job pod. The pod must belong to the given job (verified server-side). Output is truncated.",
			schema(`"namespace":{"type":"string"},"job_name":{"type":"string"},"pod":{"type":"string"},`+
				`"container":{"type":"string","description":"optional container name"},`+
				`"tail":{"type":"integer","description":"tail lines, default 200, max 2000"}`,
				"namespace", "job_name", "pod"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, jobName, pod := strArg(input, "namespace"), strArg(input, "job_name"), strArg(input, "pod")
				container := strArg(input, "container")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				// Ownership check identical to the HTTP logs endpoint.
				if err := env.Training.EnsurePodBelongsToJob(ns, jobName, pod); err != nil {
					return nil, err
				}
				// tail <= 0 would omit TailLines entirely and stream the whole
				// pod log (memory amplification, review finding B6).
				tail := intArg(input, "tail", 200)
				if tail <= 0 {
					tail = 200
				}
				if tail > 2000 {
					tail = 2000
				}
				logs, err := env.Training.GetPodLogs(ns, pod, container, int64(tail))
				if err != nil {
					return nil, err
				}
				if len(logs) > maxLogBytes {
					logs = logs[len(logs)-maxLogBytes:] // keep the tail (failures are at the end)
					logs = "...[truncated]...\n" + logs
				}
				return map[string]any{"pod": pod, "logs": logs}, nil
			},
		),
		tool.NewFunctionTool(
			"get_job_metrics",
			"Get GPU metrics (utilization/memory/power/temperature) of a training job's pods, if DCGM exporter is available.",
			schema(`"namespace":{"type":"string"},"name":{"type":"string"},"kind":{"type":"string"},`+
				`"metric_name":{"type":"string","description":"optional metric name; use list_available_metrics to discover"}`,
				"namespace", "name", "kind"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, name, kind := strArg(input, "namespace"), strArg(input, "name"), strArg(input, "kind")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				return env.Metrics.GetTrainingJobMetrics(ns, name, kind, strArg(input, "metric_name"))
			},
		),
		tool.NewFunctionTool(
			"list_available_metrics",
			"List metric names available for get_job_metrics.",
			schema(``),
			func(ctx context.Context, input map[string]any) (any, error) {
				return env.Metrics.GetAvailableMetrics(), nil
			},
		),
		tool.NewFunctionTool(
			"list_notebooks",
			"List the user's notebooks (Jupyter/VSCode) across the allowed namespaces.",
			schema(``),
			func(ctx context.Context, input map[string]any) (any, error) {
				return env.Notebooks.List(env.AllowedNamespaces)
			},
		),
		tool.NewFunctionTool(
			"get_notebook",
			"Get one notebook's detail (status, resources, mounts).",
			schema(`"namespace":{"type":"string"},"name":{"type":"string"}`, "namespace", "name"),
			func(ctx context.Context, input map[string]any) (any, error) {
				ns, name := strArg(input, "namespace"), strArg(input, "name")
				if err := env.allowNS(ns); err != nil {
					return nil, err
				}
				return env.Notebooks.Get(name, ns)
			},
		),
		tool.NewFunctionTool(
			"list_serving",
			"List the user's inference services across the allowed namespaces.",
			schema(``),
			func(ctx context.Context, input map[string]any) (any, error) {
				return env.Serving.List(env.AllowedNamespaces)
			},
		),
		tool.NewFunctionTool(
			"get_quota",
			"Get the user's resource quota (ElasticQuotaTree leaves for the allowed namespaces) and current usage, to answer 'why pending / is my quota enough' questions.",
			schema(``),
			func(ctx context.Context, input map[string]any) (any, error) {
				return env.Quota.GetForNamespaces(env.AllowedNamespaces)
			},
		),
	}
	return tool.NewToolkit(tools...)
}
