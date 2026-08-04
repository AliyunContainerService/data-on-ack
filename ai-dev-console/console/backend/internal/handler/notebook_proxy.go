package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// proxyCache caches reverse proxy instances keyed by "{type}/{namespace}/{name}".
var proxyCache sync.Map

// ProxyMode defines how a reverse proxy rewrites paths.
type ProxyMode int

const (
	// ProxyModeKeepPrefix keeps the full prefix in the forwarded path.
	// e.g. /notebook/ns/name/lab → backend receives /notebook/ns/name/lab
	// Used by Jupyter (NB_PREFIX aware).
	ProxyModeKeepPrefix ProxyMode = iota

	// ProxyModeStripToRoot strips the route prefix and forwards only the remainder.
	// e.g. /vscode/ns/name/foo → backend receives /foo
	// Used by code-server, TensorBoard, etc.
	ProxyModeStripToRoot
)

// ProxyConfig describes a reverse proxy route.
type ProxyConfig struct {
	// RoutePrefix is the URL prefix that triggers this proxy (e.g. "notebook", "vscode", "tensorboard").
	RoutePrefix string
	// Mode determines path rewriting behavior.
	Mode ProxyMode
	// TargetPort is the port on the per-notebook ClusterIP Service (default 80).
	TargetPort int
}

// NewServiceProxy creates a gin handler that reverse-proxies requests to
// per-notebook K8s Services based on the given config.
//
// Route pattern: /{prefix}/{namespace}/{name}/*
//   - Resolves target: http://{name}.{namespace}.svc:{port}
//   - Rewrites path per Mode
//   - Supports WebSocket upgrade
//   - Caches proxy instances per notebook
func NewServiceProxy(cfg ProxyConfig) gin.HandlerFunc {
	if cfg.TargetPort == 0 {
		cfg.TargetPort = 80
	}
	prefix := "/" + cfg.RoutePrefix + "/"

	return func(c *gin.Context) {
		fullPath := c.Request.URL.Path
		if !strings.HasPrefix(fullPath, prefix) {
			c.String(http.StatusBadRequest, "path does not match proxy prefix")
			return
		}

		// Parse: /{prefix}/{namespace}/{name}/remainder
		remainder := strings.TrimPrefix(fullPath, prefix)
		parts := strings.SplitN(remainder, "/", 3)
		if len(parts) < 2 {
			c.String(http.StatusBadRequest, "expected /"+cfg.RoutePrefix+"/{namespace}/{name}/...")
			return
		}
		namespace := parts[0]
		name := parts[1]
		subPath := ""
		if len(parts) == 3 {
			subPath = "/" + parts[2]
		}

		// Get or create cached proxy
		cacheKey := fmt.Sprintf("%s/%s/%s", cfg.RoutePrefix, namespace, name)
		proxy := getOrBuildProxy(cacheKey, name, namespace, cfg.TargetPort)
		if proxy == nil {
			c.String(http.StatusBadGateway, "failed to create proxy for %s", cacheKey)
			return
		}

		// Rewrite request path based on mode
		var targetPath string
		switch cfg.Mode {
		case ProxyModeKeepPrefix:
			// Forward full path: /{prefix}/{ns}/{name}/subpath
			targetPath = fullPath
		case ProxyModeStripToRoot:
			// Forward only the sub-path (strip prefix + ns + name)
			if subPath == "" {
				targetPath = "/"
			} else {
				targetPath = subPath
			}
		}

		c.Request.URL.Path = targetPath
		c.Request.RequestURI = targetPath
		if c.Request.URL.RawQuery != "" {
			c.Request.RequestURI += "?" + c.Request.URL.RawQuery
		}
		c.Writer.Header().Del("Content-Type")

		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// getOrBuildProxy returns a cached proxy or creates a new one.
func getOrBuildProxy(cacheKey, name, namespace string, port int) *httputil.ReverseProxy {
	if cached, ok := proxyCache.Load(cacheKey); ok {
		return cached.(*httputil.ReverseProxy)
	}

	target := fmt.Sprintf("http://%s.%s.svc:%d", name, namespace, port)
	remote, err := url.Parse(target)
	if err != nil {
		logrus.Errorf("proxy: invalid target %s: %v", target, err)
		return nil
	}

	logrus.Infof("new proxy: %s -> %s", cacheKey, target)
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.Host = remote.Host
			// Forward original host for code-server / Jupyter
			if fwdHost := req.Header.Get("X-Forwarded-Host"); fwdHost == "" {
				req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
			}
			req.Header.Set("X-Forwarded-Proto", "http")
			// Preserve WebSocket upgrade headers
			if upgrade := req.Header.Get("Upgrade"); upgrade != "" {
				req.Header.Set("Connection", "Upgrade")
			}
		},
		// Flush immediately for streaming (kernel output, terminal, SSE)
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			// Suppress noise from normal client disconnects
			if !strings.Contains(err.Error(), "context canceled") &&
				!strings.Contains(err.Error(), "connection reset") {
				logrus.Debugf("proxy %s error: %v", cacheKey, err)
			}
			w.WriteHeader(http.StatusBadGateway)
		},
	}

	proxyCache.Store(cacheKey, proxy)
	return proxy
}

// InvalidateProxy removes cached proxy for a notebook.
func InvalidateProxy(proxyType, namespace, name string) {
	proxyCache.Delete(fmt.Sprintf("%s/%s/%s", proxyType, namespace, name))
}
