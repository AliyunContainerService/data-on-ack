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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ElasticQuotaTree mirrors the ACK scheduler CRD (scheduling.sigs.k8s.io/v1beta1).
type ElasticQuotaTree struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ElasticQuotaTreeSpec `json:"spec,omitempty"`
}

type ElasticQuotaTreeSpec struct {
	Root *ElasticQuotaNode `json:"root,omitempty"`
}

// ElasticQuotaNode is a node in the elastic quota tree. Leaves carry the
// namespaces they govern.
type ElasticQuotaNode struct {
	Name       string              `json:"name"`
	Min        map[string]string   `json:"min,omitempty"`
	Max        map[string]string   `json:"max,omitempty"`
	Children   []*ElasticQuotaNode `json:"children,omitempty"`
	Namespaces []string            `json:"namespaces,omitempty"`
}

type ElasticQuotaTreeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ElasticQuotaTree `json:"items"`
}

// NamespaceQuotaView is the per-namespace answer to "is my quota enough /
// why am I pending".
type NamespaceQuotaView struct {
	Namespace string            `json:"namespace"`
	QuotaNode string            `json:"quotaNode,omitempty"`
	Min       map[string]string `json:"min,omitempty"`
	Max       map[string]string `json:"max,omitempty"`
	// Used is computed from pod requests in the namespace.
	Used map[string]string `json:"used"`
}
