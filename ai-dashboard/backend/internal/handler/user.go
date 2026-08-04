package handler

import (
	"net/http"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	authManager *auth.Manager
	ramService  *service.RamService
	userService *service.UserService
	kubeClient  *k8s.Client
}

func newUserHandler(ramSvc *service.RamService, userSvc *service.UserService, authMgr *auth.Manager, kubeClient *k8s.Client) *UserHandler {
	return &UserHandler{
		authManager: authMgr,
		ramService:  ramSvc,
		userService: userSvc,
		kubeClient:  kubeClient,
	}
}

func (h *UserHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/user/list/ramUsers", h.ListRamUsers)
	rg.GET("/user/get", h.GetUserByAliuid)
	rg.GET("/user/info", h.UserInfo)
	rg.POST("/user/logout", h.Logout)
}

func (h *UserHandler) ListRamUsers(c *gin.Context) {
	users, err := h.ramService.ListRamUsers()
	if err != nil {
		response.Failed(c, response.CodeUserNotFound, "list ram users failed: "+err.Error())
		return
	}
	resp := make([]model.RamUserResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, model.RamUserResponse{
			UserID:      u.UserID,
			UserName:    u.UserName,
			DisplayName: u.DisplayName,
		})
	}
	response.OK(c, resp)
}

func (h *UserHandler) GetUserByAliuid(c *gin.Context) {
	aliuid := c.Query("aliuid")
	if aliuid == "" {
		response.Failed(c, response.CodeUserNotFound, "aliuid is required")
		return
	}
	user, err := h.userService.FindUserByAliuid(aliuid)
	if err != nil {
		response.Failed(c, response.CodeUserNotFound, "user not found: "+err.Error())
		return
	}
	response.OK(c, user)
}

func (h *UserHandler) UserInfo(c *gin.Context) {
	_, _, loginName, _, ok := h.authManager.GetCurrentUser(c)
	if !ok {
		response.Failed(c, response.CodeUserNotLogin, "user not login")
		return
	}

	version, _ := h.kubeClient.GetK8sVersion()
	token := ""

	user, err := h.userService.FindUserByAliuid(loginName)
	if err != nil || user == nil {
		users, _ := h.userService.ListUsers(loginName)
		for _, u := range users {
			if u.Spec.UserName == loginName || u.Spec.Aliuid == loginName {
				user = &u
				break
			}
		}
	}

	response.OK(c, model.UserInfoResponse{
		User:       user,
		Token:       token,
		K8sVersion:  version,
	})
}

func (h *UserHandler) Logout(c *gin.Context) {
	h.authManager.HandleLogout(c)
}

var _ = http.StatusOK
