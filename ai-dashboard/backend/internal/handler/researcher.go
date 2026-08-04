package handler

import (
	"net/http"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ResearcherHandler struct {
	userService *service.UserService
}

func newResearcherHandler(userSvc *service.UserService) *ResearcherHandler {
	return &ResearcherHandler{userService: userSvc}
}

func (h *ResearcherHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/researcher/list", h.List)
	rg.POST("/researcher/create", h.Create)
	rg.PUT("/researcher/update", h.Update)
	rg.PUT("/researcher/delete", h.Delete)
	rg.GET("/researcher/getBearerToken", h.GetBearerToken)
	rg.GET("/researcher/download/kubeconfig", h.DownloadKubeConfig)
}

func (h *ResearcherHandler) List(c *gin.Context) {
	userName := c.Query("userName")
	users, err := h.userService.ListUsers(userName)
	if err != nil {
		response.Failed(c, response.CodeResearcherErr, "list users failed: "+err.Error())
		return
	}
	response.OK(c, response.Pagination{
		Total: int64(len(users)),
		Items: users,
	})
}

func (h *ResearcherHandler) Create(c *gin.Context) {
	var req model.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeResearcherErr, "invalid request: "+err.Error())
		return
	}
	user := buildUserFromCreateReq(&req)
	if err := h.userService.CreateUser(user); err != nil {
		response.Failed(c, response.CodeResearcherErr, "create user failed: "+err.Error())
		return
	}
	response.OKMsg(c, "user created", user)
}

func (h *ResearcherHandler) Update(c *gin.Context) {
	var req model.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Failed(c, response.CodeResearcherErr, "invalid request: "+err.Error())
		return
	}
	user := buildUserFromUpdateReq(&req)
	if err := h.userService.UpdateUser(user); err != nil {
		response.Failed(c, response.CodeResearcherErr, "update user failed: "+err.Error())
		return
	}
	response.OKMsg(c, "user updated", user)
}

func (h *ResearcherHandler) Delete(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		response.Failed(c, response.CodeResearcherErr, "userId is required")
		return
	}
	if err := h.userService.DeleteUser(userID); err != nil {
		response.Failed(c, response.CodeResearcherErr, "delete user failed: "+err.Error())
		return
	}
	response.OKMsg(c, "user deleted", nil)
}

func (h *ResearcherHandler) GetBearerToken(c *gin.Context) {
	userID := c.Query("userId")
	if userID == "" {
		response.Failed(c, response.CodeResearcherErr, "userId is required")
		return
	}
	token, err := h.userService.GetBearerToken(userID)
	if err != nil {
		response.Failed(c, response.CodeResearcherErr, "get bearer token failed: "+err.Error())
		return
	}
	response.OK(c, token)
}

func (h *ResearcherHandler) DownloadKubeConfig(c *gin.Context) {
	userID := c.Query("userId")
	namespace := c.Query("namespace")
	if userID == "" {
		response.Failed(c, response.CodeResearcherErr, "userId is required")
		return
	}
	kubeconfig, err := h.userService.GenKubeConfig(userID, namespace)
	if err != nil {
		response.Failed(c, response.CodeResearcherErr, "gen kubeconfig failed: "+err.Error())
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+userID+"-kubeconfig")
	c.Data(http.StatusOK, "application/yaml", []byte(kubeconfig))
}

func buildUserFromCreateReq(req *model.CreateUserRequest) *model.User {
	u := &model.User{}
	u.Kind = "User"
	u.APIVersion = "data.kubeai.alibabacloud.com/v1"
	u.Name = req.UserName
	u.Spec.UserName = req.UserName
	u.Spec.ApiRoles = req.ApiRoles
	u.Spec.Groups = req.Groups
	u.Spec.Aliuid = req.Aliuid
	if req.K8sServiceAccount != nil {
		u.Spec.K8sServiceAccount = &model.K8sServiceAccount{
			Name:               req.K8sServiceAccount.Name,
			Namespace:          req.K8sServiceAccount.Namespace,
			RoleBindings:       req.K8sServiceAccount.RoleBindings,
			ClusterRoleBindings: req.K8sServiceAccount.ClusterRoleBindings,
		}
	}
	return u
}

func buildUserFromUpdateReq(req *model.UpdateUserRequest) *model.User {
	u := buildUserFromCreateReq(&model.CreateUserRequest{
		UserName:          req.UserName,
		ApiRoles:          req.ApiRoles,
		Groups:            req.Groups,
		Aliuid:            req.Aliuid,
		K8sServiceAccount: req.K8sServiceAccount,
	})
	return u
}

var _ = metav1.CreateOptions{}
