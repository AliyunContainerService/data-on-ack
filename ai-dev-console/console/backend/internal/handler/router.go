package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewRouter(
	cfg *config.AppConfig,
	authManager *auth.Manager,
	notebookService *service.NotebookService,
	trainingService *service.TrainingService,
	servingService *service.ServingService,
	datasetService *service.DatasetService,
	modelService *service.ModelService,
	metricsService *service.MetricsService,
	experimentService *service.ExperimentService,
) *gin.Engine {
	gin.SetMode(cfg.GinMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	store := cookie.NewStore([]byte("ai-dev-console-secret-key"))
	r.Use(sessions.Sessions("ai-dev-console-session", store))

	// --- Public routes ---
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, response.Result{Code: response.CodeSuccess, Message: "ok"})
	})
	r.GET("/api/v1/login/aliyun", authManager.HandleLoginRedirect)
	r.GET("/api/v1/login/aliyun/callback", authManager.HandleLoginCallback)
	r.POST("/api/v1/logout", authManager.HandleLogout)
	r.POST("/api/v1/login/token", authManager.HandleTokenLogin)

	// --- Auth-protected API routes ---
	api := r.Group("/api/v1", auth.CheckAuth(
		"/health", "/api/v1/login", "/api/v1/logout",
	))

	// User info
	api.GET("/user/info", authManager.HandleGetUserInfo)

	// Notebooks (namespace-scoped operations require access check)
	nbHandler := &NotebookHandler{svc: notebookService}
	api.GET("/notebooks", nbHandler.List)
	api.POST("/notebooks", nbHandler.Create)
	api.GET("/notebooks/:namespace/:name", checkNamespaceAccess, nbHandler.Get)
	api.DELETE("/notebooks/:namespace/:name", checkNamespaceAccess, nbHandler.Delete)
	api.PUT("/notebooks/:namespace/:name/stop", checkNamespaceAccess, nbHandler.Stop)
	api.PUT("/notebooks/:namespace/:name/start", checkNamespaceAccess, nbHandler.Start)
	api.PUT("/notebooks/:namespace/:name/resize", checkNamespaceAccess, nbHandler.Resize)
	api.GET("/notebooks/:namespace/:name/ssh", checkNamespaceAccess, nbHandler.SSHInfo)

	// Training Jobs (namespace-scoped operations require access check)
	jobHandler := &TrainingHandler{svc: trainingService}
	api.GET("/training-jobs", jobHandler.List)
	api.POST("/training-jobs", jobHandler.Create)
	api.GET("/training-jobs/:namespace/:name", checkNamespaceAccess, jobHandler.Get)
	api.DELETE("/training-jobs/:namespace/:name", checkNamespaceAccess, jobHandler.Delete)
	api.GET("/training-jobs/:namespace/:name/pods", checkNamespaceAccess, jobHandler.Pods)
	api.GET("/training-jobs/:namespace/:name/logs", checkNamespaceAccess, jobHandler.Logs)
	api.GET("/training-jobs/:namespace/:name/events", checkNamespaceAccess, jobHandler.Events)
	api.GET("/training-jobs/:namespace/:name/yaml", checkNamespaceAccess, jobHandler.YAML)
	api.GET("/training-jobs/:namespace/:name/checkpoints", checkNamespaceAccess, jobHandler.Checkpoints)

	// GPU Metrics
	metricsHandler := &MetricsHandler{svc: metricsService}
	api.GET("/training-jobs/:namespace/:name/metrics", checkNamespaceAccess, metricsHandler.Metrics)
	api.GET("/metrics/available", metricsHandler.AvailableMetrics)

	// Experiments
	expHandler := &ExperimentHandler{svc: experimentService}
	api.GET("/experiments", expHandler.List)
	api.GET("/experiments/:name", expHandler.Get)
	api.POST("/experiments", expHandler.Create)
	api.POST("/experiments/:name/runs", expHandler.AddRun)
	api.DELETE("/experiments/:name", expHandler.Delete)

	// Cluster capabilities detection
	api.GET("/capabilities", func(c *gin.Context) {
		caps := map[string]interface{}{
			"raySupport": trainingService.DetectRaySupport(),
		}
		response.OK(c, caps)
	})

	// Ray clusters listing (for Ray Dashboard embed)
	api.GET("/ray-clusters", func(c *gin.Context) {
		namespaces := getUserNamespaces(c)
		gvr := k8s.CRDGVR("rayclusters")
		var clusters []map[string]interface{}
		for _, ns := range namespaces {
			list, err := notebookService.AdminClient().Dynamic().Resource(gvr).Namespace(ns).List(
				c.Request.Context(), metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, item := range list.Items {
				clusters = append(clusters, map[string]interface{}{
					"name":      item.GetName(),
					"namespace": item.GetNamespace(),
					"dashboardURL": fmt.Sprintf("/ray/%s/%s/", item.GetNamespace(), item.GetName()),
				})
			}
		}
		response.OK(c, clusters)
	})

	// Datasets (PVC management)
	dsHandler := &DatasetHandler{svc: datasetService}
	api.GET("/datasets", dsHandler.List)
	api.POST("/datasets", dsHandler.Create)
	api.DELETE("/datasets/:namespace/:name", checkNamespaceAccess, dsHandler.Delete)

	// Model registry
	mdlHandler := &ModelHandler{svc: modelService}
	api.GET("/models", mdlHandler.List)
	api.POST("/models", mdlHandler.Register)
	api.DELETE("/models/:namespace/:name", checkNamespaceAccess, mdlHandler.Delete)

	// Serving/Inference
	srvHandler := &ServingHandler{svc: servingService}
	api.GET("/serving", srvHandler.List)
	api.POST("/serving", srvHandler.Create)
	api.GET("/serving/:namespace/:name", checkNamespaceAccess, srvHandler.Get)
	api.DELETE("/serving/:namespace/:name", checkNamespaceAccess, srvHandler.Delete)
	api.POST("/serving/:namespace/:name/test", checkNamespaceAccess, srvHandler.Test)
	api.GET("/serving/:namespace/:name/pods", checkNamespaceAccess, srvHandler.Pods)
	api.GET("/serving/:namespace/:name/logs", checkNamespaceAccess, srvHandler.Logs)

	// Overview dashboard
	api.GET("/overview", func(c *gin.Context) {
		userName := auth.GetCurrentUser(c)
		namespaces := getUserNamespaces(c)
		overview := buildOverview(notebookService, trainingService, servingService, namespaces, userName)
		response.OK(c, overview)
	})

	// --- Reverse proxies to notebook services (must be before SPA fallback) ---
	// Each proxy type has its own path rewrite mode:
	//   notebook: keep full prefix (Jupyter uses NB_PREFIX=/notebook/{ns}/{name})
	//   vscode:   strip prefix (code-server serves from /)
	//   tensorboard: strip prefix (TB serves from /)
	r.Any("/notebook/*path", proxyAuthCheck, NewServiceProxy(ProxyConfig{
		RoutePrefix: "notebook",
		Mode:        ProxyModeKeepPrefix,
	}))
	r.Any("/vscode/*path", proxyAuthCheck, NewServiceProxy(ProxyConfig{
		RoutePrefix: "vscode",
		Mode:        ProxyModeStripToRoot,
	}))
	r.Any("/tensorboard/*path", proxyAuthCheck, NewServiceProxy(ProxyConfig{
		RoutePrefix: "tensorboard",
		Mode:        ProxyModeStripToRoot,
		TargetPort:  6006,
	}))
	// Ray Dashboard: /ray/{namespace}/{cluster-name}/* → port 8265
	r.Any("/ray/*path", proxyAuthCheck, NewServiceProxy(ProxyConfig{
		RoutePrefix: "ray",
		Mode:        ProxyModeStripToRoot,
		TargetPort:  8265,
	}))

	// --- Frontend static files + SPA fallback ---
	distDir := cfg.FrontendDir
	if _, err := os.Stat(distDir); err == nil {
		r.Static("/assets", filepath.Join(distDir, "assets"))
		r.StaticFile("/favicon.ico", filepath.Join(distDir, "favicon.ico"))
		r.StaticFile("/vite.svg", filepath.Join(distDir, "vite.svg"))

		indexFile := filepath.Join(distDir, "index.html")
		apiPrefixes := []string{"/api/", "/health"}
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

func getUserNamespaces(c *gin.Context) []string {
	// Dev mode: return all relevant namespaces
	if os.Getenv("DISABLE_AUTH") == "true" {
		return []string{"default", "default-group", "kube-ai"}
	}

	session := sessions.Default(c)
	nsJSON, ok := session.Get(auth.SessionKeyUserNS).(string)
	if !ok || nsJSON == "" {
		return []string{"default"}
	}
	var namespaces []string
	if err := json.Unmarshal([]byte(nsJSON), &namespaces); err != nil {
		return []string{"default"}
	}
	if len(namespaces) == 0 {
		return []string{"default"}
	}
	return namespaces
}

// isNamespaceAllowed checks if a namespace is in the user's allowed list.
// Used by Create handlers to validate the target namespace from request body.
func isNamespaceAllowed(c *gin.Context, ns string) bool {
	if os.Getenv("DISABLE_AUTH") == "true" {
		return true
	}
	if auth.GetCurrentRole(c) == auth.RoleAdmin {
		return true
	}
	if ns == "" {
		return true
	}
	allowed := getUserNamespaces(c)
	for _, a := range allowed {
		if a == ns {
			return true
		}
	}
	return false
}

// proxyAuthCheck enforces authentication and namespace access for reverse proxy routes.
// URL pattern: /{prefix}/{namespace}/{name}/...
// Unauthenticated → 302 /login; authenticated but namespace mismatch → 403.
// WebSocket upgrade requests are authenticated but skip namespace check.
func proxyAuthCheck(c *gin.Context) {
	// Dev mode: skip all auth
	if os.Getenv("DISABLE_AUTH") == "true" {
		c.Next()
		return
	}

	// Check session authentication
	session := sessions.Default(c)
	name := session.Get(auth.SessionKeyName)
	if name == nil || name == "" {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	// Set user context for downstream
	c.Set("userName", name)
	c.Set("userRole", session.Get(auth.SessionKeyRole))

	// WebSocket upgrade: authenticate but skip namespace check
	if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
		c.Next()
		return
	}

	// Admin bypasses namespace check
	if auth.GetCurrentRole(c) == auth.RoleAdmin {
		c.Next()
		return
	}

	// Extract namespace from path: /{prefix}/{namespace}/{name}/...
	pathParam := c.Param("path")
	parts := strings.SplitN(strings.TrimPrefix(pathParam, "/"), "/", 3)
	if len(parts) >= 2 {
		ns := parts[0]
		allowed := getUserNamespaces(c)
		for _, a := range allowed {
			if a == ns {
				c.Next()
				return
			}
		}
		response.FailedWithCode(c, 40300, "access denied: namespace '"+ns+"' is not in your allowed scope")
		c.Abort()
		return
	}

	c.Next()
}

// checkNamespaceAccess verifies the :namespace param is within the user's allowed namespaces.
// Admin role bypasses the check.
func checkNamespaceAccess(c *gin.Context) {
	if os.Getenv("DISABLE_AUTH") == "true" {
		c.Next()
		return
	}
	// Admin bypasses
	if auth.GetCurrentRole(c) == auth.RoleAdmin {
		c.Next()
		return
	}
	ns := c.Param("namespace")
	if ns == "" {
		c.Next()
		return
	}
	allowed := getUserNamespaces(c)
	for _, a := range allowed {
		if a == ns {
			c.Next()
			return
		}
	}
	response.FailedWithCode(c, 40300, "access denied: namespace '"+ns+"' is not in your allowed scope")
	c.Abort()
}
