package model

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- User CRD (data.kubeai.alibabacloud.com/v1) ---

type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UserSpec `json:"spec,omitempty"`
	Status            UserStatus `json:"status,omitempty"`
}

type UserSpec struct {
	Aliuid           string           `json:"aliuid,omitempty"`
	UserName         string           `json:"userName,omitempty"`
	UserId           string           `json:"userId,omitempty"`
	Password         string           `json:"password,omitempty"`
	Groups           []string         `json:"groups,omitempty"`
	ApiRoles         []string         `json:"apiRoles,omitempty"`
	Deletable        *bool            `json:"deletable,omitempty"`
	K8sServiceAccount *K8sServiceAccount `json:"k8sServiceAccount,omitempty"`
}

type K8sServiceAccount struct {
	Name               string        `json:"name,omitempty"`
	Namespace          string        `json:"namespace,omitempty"`
	RoleBindings       []RoleBinding `json:"roleBindings,omitempty"`
	ClusterRoleBindings []RoleBinding `json:"clusterRoleBindings,omitempty"`
}

type RoleBinding struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	RoleName  string `json:"roleName,omitempty"`
}

type UserStatus struct{}

type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// --- UserGroup CRD (data.kubeai.alibabacloud.com/v1) ---

type UserGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              UserGroupSpec `json:"spec,omitempty"`
}

type UserGroupSpec struct {
	GroupName           string   `json:"groupName,omitempty"`
	QuotaNames         []string `json:"quotaNames,omitempty"`
	DefaultRoles       []string `json:"defaultRoles,omitempty"`
	DefaultClusterRoles []string `json:"defaultClusterRoles,omitempty"`
}

type UserGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserGroup `json:"items"`
}
