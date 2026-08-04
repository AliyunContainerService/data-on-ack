package handler

import (
	"crypto/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func NewRouter(
	cfg *config.AppConfig,
	kubeClient *k8s.Client,
	authManager *auth.Manager,
	userService *service.UserService,
	quotaService *service.QuotaService,
	userGroupService *service.UserGroupService,
	datasetService *service.DatasetService,
	ramService *service.RamService,
	costService *service.CostService,
	modelAdminService *service.ModelAdminService,
) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	store := cookie.NewStore(getSessionSecret())
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		Secure:   true,
	})
	r.Use(sessions.Sessions("ai-dashboard-session", store))

	// --- Public routes ---
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Result{Code: response.CodeSuccess, Message: "ok"})
	})
	r.GET("/login/aliyun", authManager.HandleLoginRedirect)
	r.GET("/login/aliyun/callback", authManager.HandleLoginCallback)
	r.POST("/logout", authManager.HandleLogout)

	// --- Auth-protected API routes ---
	api := r.Group("", auth.CheckAuth(
		"/health", "/login", "/login/aliyun", "/logout", "/grafana/",
	))

	// Register all handlers
	newUserHandler(ramService, userService, authManager, kubeClient).RegisterRoutes(api)
	newResearcherHandler(userService).RegisterRoutes(api)
	newQuotaHandler(quotaService).RegisterRoutes(api)
	newUserGroupHandler(userGroupService, quotaService).RegisterRoutes(api)
	newDatasetHandler(datasetService, kubeClient).RegisterRoutes(api)
	newK8sHandler(kubeClient).RegisterRoutes(api)
	newOpsHandler(kubeClient).RegisterRoutes(api)
	newSettingsHandler(kubeClient).RegisterRoutes(api)
	newNodeShellHandler(kubeClient).RegisterRoutes(api)
	newCostHandler(costService).RegisterRoutes(api)
	newModelAdminHandler(modelAdminService).RegisterRoutes(api)

	// --- Grafana reverse proxy + dashboard URL ---
	dashHandler := newDashboardHandler(cfg, kubeClient)
	dashHandler.RegisterRoutes(r, api)

	// --- Frontend static files + SPA fallback ---
	distDir := cfg.FrontendDir
	if _, err := os.Stat(distDir); err == nil {
		r.Static("/assets", filepath.Join(distDir, "assets"))
		if _, err := os.Stat(filepath.Join(distDir, "favicon.ico")); err == nil {
			r.StaticFile("/favicon.ico", filepath.Join(distDir, "favicon.ico"))
		}
		if _, err := os.Stat(filepath.Join(distDir, "vite.svg")); err == nil {
			r.StaticFile("/vite.svg", filepath.Join(distDir, "vite.svg"))
		}
		// SPA fallback: serve index.html for non-API routes
		indexFile := filepath.Join(distDir, "index.html")
		apiPrefixes := []string{"/user/", "/researcher/", "/group/", "/user_group/",
			"/dataset/", "/k8s/", "/dashboard/", "/grafana/", "/health", "/login/aliyun", "/logout", "/api/", "/ops/"}
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			for _, prefix := range apiPrefixes {
				if strings.HasPrefix(path, prefix) {
					c.JSON(http.StatusNotFound, response.Result{Code: 404, Message: "not found"})
					return
				}
			}
			c.File(indexFile)
		})
	}

	return r
}

// getSessionSecret reads the cookie encryption key from SESSION_SECRET env var.
// If not set, generates a random 32-byte key and logs a warning.
func getSessionSecret() []byte {
	if s := os.Getenv("SESSION_SECRET"); s != "" {
		return []byte(s)
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	logrus.Warn("SESSION_SECRET not set, using random key (sessions will not survive pod restart)")
	return b
}
