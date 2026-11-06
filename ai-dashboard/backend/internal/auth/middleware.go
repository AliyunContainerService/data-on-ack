package auth

import (
	"net/http"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// CheckAuth returns a gin middleware that enforces session-based authentication.
// Paths in publicPaths are allowed without authentication.
//
// Behavior depends on the configured auth mode (AUTH_MODE / --auth-mode):
//   - aliyun: require a session established by RAM OAuth2 SSO login
//   - none:   allow all requests (built-in admin identity)
//   - custom: trust identity headers injected by an upstream gateway
func CheckAuth(publicPaths ...string) gin.HandlerFunc {
	prefixSet := make(map[string]bool)
	for _, p := range publicPaths {
		prefixSet[p] = true
	}

	return func(c *gin.Context) {
		// Auth disabled: skip all auth checks
		if AuthDisabled() {
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

		// Custom mode: identity comes from trusted gateway headers
		if CurrentMode() == ModeCustom {
			if authenticateViaHeaders(c) {
				c.Next()
				return
			}
			rejectUnauthenticated(c)
			return
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

// authenticateViaHeaders populates the session from trusted identity headers
// injected by an upstream authenticating gateway (custom auth mode).
// Returns false when the user header is absent or empty.
func authenticateViaHeaders(c *gin.Context) bool {
	cfg := config.Get()
	user := strings.TrimSpace(c.GetHeader(cfg.AuthUserHeader))
	if user == "" {
		return false
	}

	role := RoleResearcher
	if userInList(user, cfg.AuthAdminUsers) || groupsIntersect(c.GetHeader(cfg.AuthGroupsHeader), cfg.AuthAdminGroups) {
		role = RoleAdmin
	}

	session := sessions.Default(c)
	session.Set(SessionKeyAccountID, user)
	session.Set(SessionKeyUserID, user)
	session.Set(SessionKeyLoginName, user)
	session.Set(SessionKeyRole, role)
	if err := session.Save(); err != nil {
		logrus.Errorf("custom auth: save session: %v", err)
	}
	return true
}

// rejectUnauthenticated handles requests without a trusted identity in custom mode.
// Browser navigations are redirected to the configured login URL (if any);
// API requests get a 401 carrying the login URL for the frontend to consume.
func rejectUnauthenticated(c *gin.Context) {
	cfg := config.Get()
	loginURL := ""
	if cfg != nil {
		loginURL = cfg.AuthLoginURL
	}
	if loginURL != "" && strings.Contains(c.GetHeader("Accept"), "text/html") {
		c.Redirect(http.StatusFound, loginURL)
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":     10101,
			"message":  "authentication required",
			"loginUrl": loginURL,
		})
	}
	c.Abort()
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
