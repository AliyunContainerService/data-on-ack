package service

import (
	"fmt"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var userGroupGVR = schema.GroupVersionResource{
	Group: "data.kubeai.alibabacloud.com", Version: "v1", Resource: "usergroups",
}

type UserGroupService struct {
	kubeClient *k8s.Client
	crdClient  *k8s.CRDClient
}

func NewUserGroupService(kubeClient *k8s.Client) *UserGroupService {
	return &UserGroupService{
		kubeClient: kubeClient,
		crdClient:  k8s.NewCRDClient(kubeClient),
	}
}

func (s *UserGroupService) ListUserGroups() ([]model.UserGroup, error) {
	var list model.UserGroupList
	if err := s.crdClient.List(userGroupGVR, UserNamespace, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (s *UserGroupService) GetUserGroup(name string) (*model.UserGroup, error) {
	var ug model.UserGroup
	if err := s.crdClient.Get(userGroupGVR, UserNamespace, name, &ug); err != nil {
		return nil, err
	}
	return &ug, nil
}

func (s *UserGroupService) CreateUserGroup(ug *model.UserGroup) error {
	ug.Namespace = UserNamespace
	return s.crdClient.CreateOrReplace(userGroupGVR, UserNamespace, ug)
}

func (s *UserGroupService) DeleteUserGroup(name string) error {
	return s.crdClient.Delete(userGroupGVR, UserNamespace, name)
}

// GetGroupNamespaces returns all namespaces associated with a user group's quota names.
func (s *UserGroupService) GetGroupNamespaces(groupName string, quotaService *QuotaService) ([]string, error) {
	ug, err := s.GetUserGroup(groupName)
	if err != nil {
		return nil, err
	}
	if ug == nil {
		return nil, fmt.Errorf("user group %s not found", groupName)
	}
	return quotaService.GetQuotaNamespaces(ug.Spec.QuotaNames)
}
