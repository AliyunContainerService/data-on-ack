/*


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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// UserSpec defines the desired state of User
type UserSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of User. Edit User_types.go to remove/update
	UserName          string            `json:"userName,omitempty"`
	UserId            string            `json:"userId,omitempty"`
	Password          string            `json:"password,omitempty"`
	Aliuid            string            `json:"aliuid,omitempty"`
	ExternalUser      ExternalUser      `json:"externalUser,omitempty"`
	ApiRoles          []string          `json:"apiRoles,omitempty"`
	Groups            []string          `json:"groups,omitempty"`
	K8sServiceAccount K8sServiceAccount `json:"k8sServiceAccount,omitempty"`
	Deletable         bool              `json:"deletable,omitempty"`
}

//type K8sMetaData struct {
//	Name      string `json:"name,omitempty"`
//	Namespace string `json:"namespace,omitempty"`
//}

type ExternalUser struct {
	AuthType string `json:"authType,omitempty"`
	Uid      string `json:"uid,omitempty"`
	Name     string `json:"name,omitempty"`
}

type K8sServiceAccount struct {
	metav1.ObjectMeta   `json:",inline"`
	RoleBindings        []K8sRoleBinding `json:"roleBindings,omitempty"`
	ClusterRoleBindings []K8sRoleBinding `json:"clusterRoleBindings,omitempty"`
}

type K8sRoleBinding struct {
	metav1.ObjectMeta `json:",inline"`
	RoleName          string `json:"roleName,omitempty"`
}

// UserStatus defines the observed state of User
type UserStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}
