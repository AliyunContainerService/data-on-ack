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

package model

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AgentSession persists one agentic-console conversation (design §3.1/§4).
// It lives in the same CRD family as User/UserGroup. Only metadata and a
// compacted context summary are stored: raw message streams never enter etcd.
type AgentSession struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentSessionSpec   `json:"spec,omitempty"`
	Status AgentSessionStatus `json:"status,omitempty"`
}

type AgentSessionSpec struct {
	// OwnerUser is written server-side from the login identity; clients can
	// never set it for someone else.
	OwnerUser string `json:"ownerUser"`
	AgentType string `json:"agentType"` // copilot|diagnose
	Title     string `json:"title,omitempty"`
	Locale    string `json:"locale,omitempty"`
}

type AgentSessionStatus struct {
	// ContextSummary is the compacted conversation context (bounded, see
	// service.AgentSessionStore).
	ContextSummary string      `json:"contextSummary,omitempty"`
	LastActive     metav1.Time `json:"lastActive,omitempty"`
	RunsTotal      int         `json:"runsTotal,omitempty"`
}

type AgentSessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentSession `json:"items"`
}

// AgentSessionGVR identifies the AgentSession custom resource.
func AgentSessionGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "data.kubeai.alibabacloud.com",
		Version:  "v1",
		Resource: "agentsessions",
	}
}
