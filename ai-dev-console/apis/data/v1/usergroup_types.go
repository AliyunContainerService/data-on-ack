package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserGroupSpec defines the desired state of UserGroup
type UserGroupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of User. Edit User_types.go to remove/update
	QuotaNames          []string `json:"quotaNames,omitempty"`
	DefaultRoles        []string `json:"defaultRoles,omitempty"`
	DefaultClusterRoles []string `json:"defaultClusterRoles,omitempty"`
}

// UserStatus defines the observed state of User
type UserGroupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// User is the Schema for the users API
type UserGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserGroupSpec   `json:"spec,omitempty"`
	Status UserGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserGroupList contains a list of UserGroup
type UserGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UserGroup{}, &UserGroupList{})
}
