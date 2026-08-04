package model

// --- DTOs for API requests/responses ---

// CreateUserRequest is the body for POST /researcher/create.
type CreateUserRequest struct {
	UserName           string        `json:"userName"`
	ApiRoles           []string      `json:"apiRoles"`
	Groups             []string      `json:"groups"`
	Aliuid             string        `json:"aliuid"`
	K8sServiceAccount  *K8sServiceAccount `json:"k8sServiceAccount"`
}

// UpdateUserRequest is the body for PUT /researcher/update.
type UpdateUserRequest struct {
	UserName           string        `json:"userName"`
	ApiRoles           []string      `json:"apiRoles"`
	Groups             []string      `json:"groups"`
	Aliuid             string        `json:"aliuid"`
	K8sServiceAccount  *K8sServiceAccount `json:"k8sServiceAccount"`
}

// CreateQuotaNodeRequest is the body for POST /group/create.
type CreateQuotaNodeRequest struct {
	Name       string            `json:"name"`
	Parent     string            `json:"parent"`
	Min        map[string]string `json:"min,omitempty"`
	Max        map[string]string `json:"max,omitempty"`
	Namespaces []string          `json:"namespaces,omitempty"`
}

// UpdateQuotaTreeRequest is the body for PUT /group/update.
type UpdateQuotaTreeRequest struct {
	Action      string            `json:"action"` // add, delete, update
	OldNodeName string           `json:"oldNodeName,omitempty"`
	NewNodeName string           `json:"newNodeName,omitempty"`
	Prefix      string           `json:"prefix,omitempty"`
	Node        *ElasticQuotaNode `json:"node,omitempty"`
}

// CreateUserGroupRequest is the body for POST /user_group/create.
type CreateUserGroupRequest struct {
	GroupName           string   `json:"groupName"`
	QuotaNames          []string `json:"quotaNames"`
	DefaultRoles        []string `json:"defaultRoles,omitempty"`
	DefaultClusterRoles []string `json:"defaultClusterRoles,omitempty"`
	UserNames           []string `json:"userNames,omitempty"`
}

// UpdateUserGroupRequest is the body for PUT /user_group/update.
type UpdateUserGroupRequest struct {
	GroupName           string   `json:"groupName"`
	QuotaNames          []string `json:"quotaNames"`
	DefaultRoles        []string `json:"defaultRoles,omitempty"`
	DefaultClusterRoles []string `json:"defaultClusterRoles,omitempty"`
	Users               []string `json:"users,omitempty"`
}

// CreateDatasetRequest is the body for POST /dataset/create.
type CreateDatasetRequest struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace"`
	RuntimeConf    string `json:"runtimeConf"`
	DatasetConf    string `json:"datasetConf"`
}

// RamUserResponse is the response for GET /user/list/ramUsers.
type RamUserResponse struct {
	UserID      string `json:"userId"`
	UserName    string `json:"userName"`
	DisplayName string `json:"displayName"`
}

// UserInfoResponse is the response for GET /user/info.
type UserInfoResponse struct {
	User        *User   `json:"user"`
	Token       string  `json:"token"`
	K8sVersion  string  `json:"k8sVersion"`
}

// K8sResourceList is a generic list response for PVC/Secret/Namespace.
type K8sResourceList struct {
	Items []map[string]interface{} `json:"items"`
	Total int                      `json:"total"`
}
