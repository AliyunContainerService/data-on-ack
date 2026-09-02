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

package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var eqTreeGVR = schema.GroupVersionResource{
	Group: "scheduling.sigs.k8s.io", Version: "v1beta1", Resource: "elasticquotatrees",
}

const eqTreeNamespace = "kube-system"

// QuotaService answers quota questions (C3): it reads ElasticQuotaTree leaves
// and computes real usage from pod requests. Read-only by design.
type QuotaService struct {
	adminClient *k8s.Client
}

func NewQuotaService(adminClient *k8s.Client) *QuotaService {
	return &QuotaService{adminClient: adminClient}
}

// GetForNamespaces returns quota + usage views for the given namespaces.
func (s *QuotaService) GetForNamespaces(namespaces []string) ([]model.NamespaceQuotaView, error) {
	trees, err := s.listTrees()
	if err != nil {
		return nil, fmt.Errorf("list elasticquotatrees: %w", err)
	}

	views := make([]model.NamespaceQuotaView, 0, len(namespaces))
	for _, ns := range namespaces {
		view := model.NamespaceQuotaView{Namespace: ns, Used: map[string]string{}}
		if leaf := findLeafForNamespace(trees, ns); leaf != nil {
			view.QuotaNode = leaf.Name
			view.Min = leaf.Min
			view.Max = leaf.Max
		}
		if used, err := s.namespaceUsage(ns); err == nil {
			view.Used = used
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *QuotaService) listTrees() ([]model.ElasticQuotaTree, error) {
	raw, err := s.adminClient.Dynamic().Resource(eqTreeGVR).Namespace(eqTreeNamespace).
		List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var list model.ElasticQuotaTreeList
	b, err := raw.MarshalJSON()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// namespaceUsage sums pod requests (cpu/memory/GPU) in one namespace.
func (s *QuotaService) namespaceUsage(namespace string) (map[string]string, error) {
	// Include Pending pods: "why am I pending / is my quota enough" questions
	// must account for already-requested-but-not-running resources (review
	// finding B9). Succeeded/Failed pods are excluded via the phase filter
	// below; the list is capped to keep this read cheap.
	pods, err := s.adminClient.Typed().CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		Limit: 1000,
	})
	if err != nil {
		return nil, err
	}

	cpuMillis := int64(0)
	memBytes := int64(0)
	gpu := int64(0)
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}
		for j := range pod.Spec.Containers {
			req := pod.Spec.Containers[j].Resources.Requests
			if v, ok := req[corev1.ResourceCPU]; ok {
				cpuMillis += v.MilliValue()
			}
			if v, ok := req[corev1.ResourceMemory]; ok {
				memBytes += v.Value()
			}
			if v, ok := req["nvidia.com/gpu"]; ok {
				gpu += v.Value()
			}
		}
	}

	used := map[string]string{
		"cpu":    fmt.Sprintf("%dm", cpuMillis),
		"memory": fmt.Sprintf("%dMi", memBytes/(1024*1024)),
	}
	if gpu > 0 {
		used["nvidia.com/gpu"] = fmt.Sprintf("%d", gpu)
	}
	return used, nil
}

func findLeafForNamespace(trees []model.ElasticQuotaTree, ns string) *model.ElasticQuotaNode {
	for i := range trees {
		if leaf := walkForNamespace(trees[i].Spec.Root, ns); leaf != nil {
			return leaf
		}
	}
	return nil
}

func walkForNamespace(node *model.ElasticQuotaNode, ns string) *model.ElasticQuotaNode {
	if node == nil {
		return nil
	}
	for _, n := range node.Namespaces {
		if n == ns {
			return node
		}
	}
	for _, child := range node.Children {
		if found := walkForNamespace(child, ns); found != nil {
			return found
		}
	}
	return nil
}
