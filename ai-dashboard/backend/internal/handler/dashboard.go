package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/response"
	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	cfg        *config.AppConfig
	kubeClient *k8s.Client
	proxy      *httputil.ReverseProxy
}

func newDashboardHandler(cfg *config.AppConfig, kubeClient *k8s.Client) *DashboardHandler {
	target := cfg.GrafanaProxyTarget
	targetURL, _ := url.Parse(fmt.Sprintf("http://%s", target))
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		// Rewrite path: /grafana/foo → /grafana/foo (Grafana is served from subpath)
		// The Grafana service already expects /grafana/ prefix, so no rewrite needed
		req.Header.Set("X-Forwarded-Host", req.Host)
	}

	return &DashboardHandler{cfg: cfg, kubeClient: kubeClient, proxy: proxy}
}

func (h *DashboardHandler) RegisterRoutes(r *gin.Engine, rg *gin.RouterGroup) {
	rg.GET("/dashboard/url", h.GetDashboardURL)

	// Grafana reverse proxy — must be on the main engine (not the auth-protected group)
	// to match the original Zuul route: /grafana/** → arena-exporter-grafana:80/grafana/
	r.Any("/grafana/*path", h.GrafanaProxy)
}

func (h *DashboardHandler) GetDashboardURL(c *gin.Context) {
	clusterID, _ := h.kubeClient.GetClusterID()
	host := h.cfg.GrafanaProxyTarget
	url := fmt.Sprintf("http://%s/grafana/d/kube-ai-cluster-details?orgId=1&refresh=10s&var-cluster=%s",
		host, clusterID)
	response.OK(c, url)
}

func (h *DashboardHandler) GrafanaProxy(c *gin.Context) {
	path := c.Param("path")
	// Ensure the path starts with /grafana/
	c.Request.URL.Path = "/grafana/" + strings.TrimPrefix(path, "/")
	h.proxy.ServeHTTP(c.Writer, c.Request)
}
