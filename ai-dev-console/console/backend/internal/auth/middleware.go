package auth

import (
	"os"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// CheckAuth is a middleware that verifies user is authenticated.
// Paths in publicPrefixes are excluded from auth check.
// When DISABLE_AUTH env is set to "true", all requests are allowed (dev mode).
func CheckAuth(publicPrefixes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Dev mode: skip all auth
		if os.Getenv("DISABLE_AUTH") == "true" {
			c.Set("userName", "dev-user")
			c.Set("userRole", RoleAdmin)
			c.Next()
			return
		}

		// Skip public paths
		for _, prefix := range publicPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		// Skip static assets
		if isStaticAsset(path) {
			c.Next()
			return
		}

		session := sessions.Default(c)
		name := session.Get(SessionKeyName)
		if name == nil || name == "" {
			response.Unauthorized(c)
			c.Abort()
			return
		}

		// Set user info in context for downstream handlers
		c.Set("userName", name)
		c.Set("userRole", session.Get(SessionKeyRole))
		c.Set("userToken", session.Get(SessionKeyToken))

		var namespaces []string
		if nsJSON, ok := session.Get(SessionKeyUserNS).(string); ok && nsJSON != "" {
			// Stored as JSON string
			c.Set("userNamespaces", nsJSON)
		}
		_ = namespaces

		c.Next()
	}
}

func isStaticAsset(path string) bool {
	staticExts := []string{".js", ".css", ".png", ".jpg", ".ico", ".svg", ".woff", ".woff2", ".ttf"}
	for _, ext := range staticExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	if strings.HasPrefix(path, "/assets/") {
		return true
	}
	return false
}

// GetCurrentUser extracts the current user name from gin context.
func GetCurrentUser(c *gin.Context) string {
	name, _ := c.Get("userName")
	if s, ok := name.(string); ok {
		return s
	}
	return ""
}

// GetCurrentRole extracts the current user role from gin context.
func GetCurrentRole(c *gin.Context) string {
	role, _ := c.Get("userRole")
	if s, ok := role.(string); ok {
		return s
	}
	return ""
}
