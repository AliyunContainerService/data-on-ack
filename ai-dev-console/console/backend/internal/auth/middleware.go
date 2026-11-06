package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// CheckAuth is a middleware that verifies user is authenticated.
// Paths in publicPrefixes are excluded from auth check.
//
// Behavior depends on the configured auth mode (AUTH_MODE / --auth-mode):
//   - aliyun: require a session established by RAM OAuth2 SSO (or K8s token) login
//   - none:   allow all requests (built-in admin identity)
//   - custom: trust identity headers injected by an upstream gateway
func CheckAuth(m *Manager, publicPrefixes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Auth disabled: everyone is the built-in admin
		if AuthDisabled() {
			c.Set("userName", NoneModeUser)
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

		// Custom mode: identity comes from trusted gateway headers
		if CurrentMode() == ModeCustom {
			if m != nil && m.AuthenticateViaHeaders(c) {
				c.Next()
				return
			}
			m.RejectUnauthenticated(c)
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

// AuthenticateViaHeaders populates the session from trusted identity headers
// injected by an upstream authenticating gateway (custom auth mode).
// Role comes from the configured admin user/group lists, falling back to the
// User CRD apiRoles field; namespace scope comes from the User CRD.
// Returns false when the user header is absent or empty.
func (m *Manager) AuthenticateViaHeaders(c *gin.Context) bool {
	cfg := m.cfg
	user := strings.TrimSpace(c.GetHeader(cfg.AuthUserHeader))
	if user == "" {
		return false
	}

	normalizedName := normalizeUserID(user)

	role := RoleResearcher
	if userInList(user, cfg.AuthAdminUsers) || userInList(normalizedName, cfg.AuthAdminUsers) ||
		groupsIntersect(c.GetHeader(cfg.AuthGroupsHeader), cfg.AuthAdminGroups) {
		role = RoleAdmin
	} else if crdRole := m.getUserRoleFromCRD(normalizedName); crdRole != "" {
		role = crdRole
	}

	namespaces := m.getUserNamespaces(normalizedName)

	session := sessions.Default(c)
	session.Set(SessionKeyAccountID, user)
	session.Set(SessionKeyLoginID, user)
	session.Set(SessionKeyName, normalizedName)
	session.Set(SessionKeyLoginName, user)
	session.Set(SessionKeyRole, role)
	if len(namespaces) > 0 {
		nsJSON, _ := json.Marshal(namespaces)
		session.Set(SessionKeyUserNS, string(nsJSON))
	} else {
		session.Delete(SessionKeyUserNS)
	}
	if err := session.Save(); err != nil {
		logrus.Errorf("custom auth: save session: %v", err)
	}

	c.Set("userName", normalizedName)
	c.Set("userRole", role)
	return true
}

// RejectUnauthenticated handles requests without a trusted identity in custom mode.
// Browser navigations are redirected to the configured login URL (if any);
// API requests get a 401 carrying the login URL for the frontend to consume.
func (m *Manager) RejectUnauthenticated(c *gin.Context) {
	loginURL := ""
	if m != nil && m.cfg != nil {
		loginURL = m.cfg.AuthLoginURL
	}
	if loginURL != "" && strings.Contains(c.GetHeader("Accept"), "text/html") {
		c.Redirect(http.StatusFound, loginURL)
	} else {
		c.JSON(http.StatusUnauthorized, response.Result{
			Code:    10401,
			Message: "authentication required",
			Data:    map[string]string{"loginUrl": loginURL},
		})
	}
	c.Abort()
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
