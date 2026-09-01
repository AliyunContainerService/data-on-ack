package handler

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type UserGroupHandler struct {
	userGroupService *service.UserGroupService
	quotaService     *service.QuotaService
}

func newUserGroupHandler(ugSvc *service.UserGroupService, qSvc *service.QuotaService) *UserGroupHandler {
	return &UserGroupHandler{userGroupService: ugSvc, quotaService: qSvc}
}

func (h *UserGroupHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// User group management is a cluster-admin capability.
	rg.GET("/user_group/list", adminOnly, h.List)
	rg.GET("/user_group/get_group_namespaces", adminOnly, h.GetGroupNamespaces)
	rg.POST("/user_group/create", adminOnly, h.Create)
	rg.PUT("/user_group/update", adminOnly, h.Update)
	rg.PUT("/user_group/delete", adminOnly, h.Delete)
}

func (h *UserGroupHandler) List(c *gin.Context) {
	groups, err := h.userGroupService.ListUserGroups()
	if err != nil {
		response.Failed(c, response.CodeGroupError, "list user groups failed: "+err.Error())
		return
	}
	response.OK(c, groups)
}

func (h *UserGroupHandler) GetGroupNamespaces(c *gin.Context) {
	groupName := c.Query("groupName")
	if groupName == "" {
		response.Failed(c, response.CodeGroupError, "groupName is required")
		return
	}
	namespaces, err := h.userGroupService.GetGroupNamespaces(groupName, h.quotaService)
	if err != nil {
		response.Failed(c, response.CodeGroupError, "get namespaces failed: "+err.Error())
		return
	}
	response.OK(c, namespaces)
}

func (h *UserGroupHandler) Create(c *gin.Context) {
	var req model.CreateUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeGroupError, "invalid request: "+err.Error())
		return
	}
	ug := &model.UserGroup{}
	ug.Name = req.GroupName
	ug.Spec = model.UserGroupSpec{
		GroupName:           req.GroupName,
		QuotaNames:          req.QuotaNames,
		DefaultRoles:        req.DefaultRoles,
		DefaultClusterRoles: req.DefaultClusterRoles,
	}
	if err := h.userGroupService.CreateUserGroup(ug); err != nil {
		response.Failed(c, response.CodeGroupError, "create user group failed: "+err.Error())
		return
	}
	response.OKMsg(c, "user group created", ug)
}

func (h *UserGroupHandler) Update(c *gin.Context) {
	var req model.UpdateUserGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeGroupError, "invalid request: "+err.Error())
		return
	}
	ug := &model.UserGroup{}
	ug.Name = req.GroupName
	ug.Spec = model.UserGroupSpec{
		GroupName:           req.GroupName,
		QuotaNames:          req.QuotaNames,
		DefaultRoles:        req.DefaultRoles,
		DefaultClusterRoles: req.DefaultClusterRoles,
	}
	if err := h.userGroupService.CreateUserGroup(ug); err != nil {
		response.Failed(c, response.CodeGroupError, "update user group failed: "+err.Error())
		return
	}
	response.OKMsg(c, "user group updated", ug)
}

func (h *UserGroupHandler) Delete(c *gin.Context) {
	groupName := c.Query("groupName")
	if groupName == "" {
		response.Failed(c, response.CodeGroupError, "groupName is required")
		return
	}
	if err := h.userGroupService.DeleteUserGroup(groupName); err != nil {
		response.Failed(c, response.CodeGroupError, "delete user group failed: "+err.Error())
		return
	}
	response.OKMsg(c, "user group deleted", nil)
}
