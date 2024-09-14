/*
Copyright 2019 The Alibaba Authors.

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

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ResourcesRange the container resources range
type ResourcesRange struct {
	// The container name in pod spec
	ContainerName string `json:"containerName"`

	// The minimum required resources for the container
	Min v1.ResourceRequirements `json:"min"`

	// Maximum resources that can be applied for by the container
	Max v1.ResourceRequirements `json:"max"`
}

// Schedule the Schedule of paraset
type Schedule struct {
	// The crontab expression e.g. "* * * * ?"
	/*
	   	A cron expression represents a set of times, using 5 space-separated fields.

	   	Field name   | Mandatory? | Allowed values  | Allowed special characters
	   	----------   | ---------- | --------------  | --------------------------
	   	Minutes      | Yes        | 0-59            | * / , -
	   	Hours        | Yes        | 0-23            | * / , -
	   	Day of month | Yes        | 1-31            | * / , - ?
	   	Month        | Yes        | 1-12 or JAN-DEC | * / , -
	   	Day of week  | Yes        | 0-6 or SUN-SAT  | * / , - ?

	   Month and Day-of-week field values are case insensitive.  "SUN", "Sun", and
	   "sun" are equally accepted.
	*/
	Cron string `json:"cron,omitempty"`

	// The max number of pods that belongs to this paraset in one node.
	MaxReplicaPerNode *int32 `json:"maxReplicaPerNode,omitempty"`

	// The resources range for each container
	ResourcesRanges []ResourcesRange `json:"resourcesRanges"`
}

// ParaSetSpec defines the desired state of ParaSet
type ParaSetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//expression
	Schedules []Schedule `json:"schedules"`

	// A label query over pods that are managed by the para set.
	// Must match in order to be controlled.
	// It must match the pod template's labels.
	Selector *metav1.LabelSelector `json:"selector"`

	// An object that describes the pod that will be created.
	// The paraSet will create exactly one copy of this pod on every node
	// that matches the template's node selector (or on every node if no node
	// selector is specified).
	Template v1.PodTemplateSpec `json:"template"`
}

// ParaSetStatus defines the observed state of ParaSet
type ParaSetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// The number of nodes that are running at least 1
	// paraset pod and are supposed to run the para pod.
	NumberNodesScheduled int32 `json:"numberNodesScheduled"`

	// The total number of nodes that should be running the para
	// pod (including nodes correctly running the para pod).
	NumberNodesDesiredScheduled int32 `json:"numberNodesDesiredScheduled"`

	// The number of pods are running and available.
	NumberPodsAvailable int32 `json:"numberPodsAvailable"`

	// The total number of pods that should be running and
	// available (including nodes correctly running the para pod).
	NumberPodsDesiredAvailable int32 `json:"numberPodsDesiredAvailable"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ParaSet is the Schema for the parasets API
type ParaSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ParaSetSpec   `json:"spec,omitempty"`
	Status ParaSetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ParaSetList contains a list of ParaSet
type ParaSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ParaSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ParaSet{}, &ParaSetList{})
}
