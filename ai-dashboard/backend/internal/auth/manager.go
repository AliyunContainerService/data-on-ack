package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/aliyun"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/k8s"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

const (
	dashboardFullName    = "ack-ai-dashboard-admin-ui"
	dashboardNamespace   = "kube-ai"
	envIngressEnabled    = "DASHBOARD_INGRESS_ENABLE"
	envDashboardHost     = "DASHBOARD_HOST"
	defaultFilterURL     = "/login/aliyun"
)

// Manager handles OAuth2 SSO authentication lifecycle.
type Manager struct {
	cfg         *config.AppConfig
	kubeClient  *k8s.Client
	imsClient   *aliyun.IMSClient
	webAppMgr   *WebAppManager
	oauthInfo   *OAuthInfo
}

func NewManager(cfg *config.AppConfig, kubeClient *k8s.Client, imsClient *aliyun.IMSClient) (*Manager, error) {
	m := &Manager{
		cfg:        cfg,
		kubeClient: kubeClient,
		imsClient:  imsClient,
		webAppMgr:  NewWebAppManager(imsClient),
	}

	if cfg.IsCreateWebApp {
		if err := m.initWebApp(); err != nil {
			return nil, fmt.Errorf("init OAuth web app: %w", err)
		}
	}

	return m, nil
}

func (m *Manager) initWebApp() error {
	clusterID, _ := m.kubeClient.GetClusterID()
	appName := GenAppName(clusterID)
	redirectURI, err := m.resolveRedirectURI()
	if err != nil {
		return fmt.Errorf("resolve redirect URI: %w", err)
	}

	oauth, err := m.webAppMgr.EnsureApp(appName, redirectURI)
	if err != nil {
		return err
	}

	m.oauthInfo = oauth
	logrus.Infof("OAuth web app ready: app=%s redirect=%s", appName, redirectURI)
	return nil
}

func (m *Manager) resolveRedirectURI() (string, error) {
	if m.cfg.IsCreateWebApp {
		// Priority 1: explicit DASHBOARD_HOST env var (set by Helm chart)
		envHost := getEnvOrEmpty(envDashboardHost)
		if envHost != "" {
			return fmt.Sprintf("http://%s%s", envHost, defaultFilterURL), nil
		}

		// Priority 2: lookup from cluster resources
		ingressEnabled := getEnvOrEmpty(envIngressEnabled)
		isIngress := ingressEnabled == "true"
		var host string
		var err error
		if isIngress {
			host, err = m.kubeClient.GetIngressHost(dashboardFullName, dashboardNamespace)
		} else {
			host, err = m.kubeClient.GetServiceClusterIP(dashboardFullName, dashboardNamespace)
		}
		if err != nil {
			return "", err
		}
		if host != "" {
			return fmt.Sprintf("http://%s%s", host, defaultFilterURL), nil
		}
	}

	return fmt.Sprintf("http://localhost%s", defaultFilterURL), nil
}

// HandleLoginCallback processes the OAuth2 authorization code callback at /login/aliyun.
func (m *Manager) HandleLoginCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing authorization code"})
		return
	}

	if m.oauthInfo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth not configured"})
		return
	}

	// Anti-CSRF: the state echoed back by the provider must match the nonce
	// we generated in HandleLoginRedirect and stored in the session.
	if !verifyAndConsumeOAuthState(c, c.Query("state")) {
		logrus.Warn("OAuth callback rejected: state mismatch or missing")
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid OAuth state"})
		return
	}

	callbackURL := m.callbackURL(c)
	accessToken, err := ExchangeToken(m.cfg, *m.oauthInfo, callbackURL, code)
	if err != nil {
		logrus.Errorf("OAuth token exchange failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token exchange failed"})
		return
	}

	userInfo, err := FetchUserInfo(m.cfg, accessToken)
	if err != nil {
		logrus.Errorf("OAuth userinfo fetch failed: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "userinfo fetch failed"})
		return
	}

	// Determine login name: sub-account uses upn, main account uses login_name
	loginName := userInfo.Upn
	if loginName == "" {
		loginName = userInfo.LoginName
	}

	// Determine role: main account (no upn) is admin, sub-account is researcher.
	// A sub-account explicitly configured via --admin-uid/DASHBOARD_ADMINUID is also admin.
	role := RoleResearcher
	if userInfo.Upn == "" || (m.cfg.AdminUID != "" && userInfo.Uid == m.cfg.AdminUID) {
		role = RoleAdmin
	}

	session := sessions.Default(c)
	session.Set(SessionKeyAccountID, userInfo.Aid)
	session.Set(SessionKeyUserID, userInfo.Uid)
	session.Set(SessionKeyLoginName, loginName)
	session.Set(SessionKeyRole, role)
	session.Set(SessionKeyToken, accessToken)
	if err := session.Save(); err != nil {
		logrus.Errorf("save session: %v", err)
	}

	c.Redirect(http.StatusFound, "/")
}

// HandleLoginRedirect handles both the login redirect and the OAuth callback.
// If the request has a "code" query parameter, it processes the callback;
// otherwise it redirects to the RAM SSO login page.
func (m *Manager) HandleLoginRedirect(c *gin.Context) {
	// If code is present, this is the OAuth callback from RAM
	if code := c.Query("code"); code != "" {
		m.HandleLoginCallback(c)
		return
	}

	if m.oauthInfo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OAuth not configured"})
		return
	}
	// Generate an anti-CSRF state nonce, persist it in the session and attach
	// it to the authorization URL. It is validated (and consumed) in the callback.
	state, err := newOAuthState()
	if err != nil {
		logrus.Errorf("generate OAuth state: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate OAuth state"})
		return
	}
	session := sessions.Default(c)
	session.Set(SessionKeyOAuthState, state)
	if err := session.Save(); err != nil {
		logrus.Errorf("save session: %v", err)
	}

	callbackURL := m.callbackURL(c)
	loginURL := GetLoginURL(m.cfg, *m.oauthInfo, callbackURL, state)
	c.Redirect(http.StatusFound, loginURL)
}

// HandleLogout clears the session and redirects to login.
func (m *Manager) HandleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Delete(SessionKeyAccountID)
	session.Delete(SessionKeyUserID)
	session.Delete(SessionKeyLoginName)
	session.Delete(SessionKeyRole)
	session.Delete(SessionKeyToken)
	session.Delete(SessionKeyOAuthState)
	session.Save()
	c.Redirect(http.StatusFound, "/login")
}

// GetCurrentUser returns the logged-in user info from the session.
func (m *Manager) GetCurrentUser(c *gin.Context) (accountID, userID, loginName, role string, ok bool) {
	// Dev mode: return a mock admin user
	if os.Getenv("DISABLE_AUTH") == "true" {
		return "dev-account", "dev-user", "dev-admin", "admin", true
	}
	session := sessions.Default(c)
	accountID, _ = session.Get(SessionKeyAccountID).(string)
	userID, _ = session.Get(SessionKeyUserID).(string)
	loginName, _ = session.Get(SessionKeyLoginName).(string)
	role, _ = session.Get(SessionKeyRole).(string)
	ok = accountID != ""
	return
}

// Cleanup deletes the RAM web app on shutdown.
func (m *Manager) Cleanup() {
	if m.cfg.IsCreateWebApp && m.oauthInfo != nil {
		clusterID, _ := m.kubeClient.GetClusterID()
		appName := GenAppName(clusterID)
		if err := m.webAppMgr.DeleteApp(appName); err != nil {
			logrus.Errorf("delete OAuth web app: %v", err)
		} else {
			logrus.Info("OAuth web app deleted")
		}
	}
}

func (m *Manager) callbackURL(c *gin.Context) string {
	return fmt.Sprintf("http://%s%s", c.Request.Host, defaultFilterURL)
}

// newOAuthState generates a random hex-encoded nonce for the OAuth state parameter.
func newOAuthState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// verifyAndConsumeOAuthState checks the callback state against the value stored
// in the session and clears it afterwards so it cannot be replayed.
func verifyAndConsumeOAuthState(c *gin.Context, state string) bool {
	session := sessions.Default(c)
	defer func() {
		session.Delete(SessionKeyOAuthState)
		_ = session.Save()
	}()

	expected, _ := session.Get(SessionKeyOAuthState).(string)
	if state == "" || expected == "" || state != expected {
		return false
	}
	return true
}

func getEnvOrEmpty(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}
