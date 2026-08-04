package auth

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// CheckAuth returns a gin middleware that enforces session-based authentication.
// Paths in publicPaths are allowed without authentication.
// When DISABLE_AUTH=true, all requests are allowed (dev mode).
func CheckAuth(publicPaths ...string) gin.HandlerFunc {
	prefixSet := make(map[string]bool)
	for _, p := range publicPaths {
		prefixSet[p] = true
	}

	return func(c *gin.Context) {
		// Dev mode: skip all auth
		if os.Getenv("DISABLE_AUTH") == "true" {
			c.Next()
			return
		}

		path := c.Request.URL.Path

		// Skip static assets
		if isStaticAsset(path) {
			c.Next()
			return
		}

		// Check public paths
		for prefix := range prefixSet {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}

		session := sessions.Default(c)
		accountID := session.Get(SessionKeyAccountID)
		if accountID == nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		c.Next()
	}
}

func isStaticAsset(path string) bool {
	suffixes := []string{".js", ".css", ".png", ".ico", ".jpg", ".svg", ".woff", ".woff2", ".ttf", ".map"}
	for _, s := range suffixes {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}
