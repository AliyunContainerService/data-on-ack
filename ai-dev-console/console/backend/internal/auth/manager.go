package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/k8s"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/model"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/response"
	ims "github.com/alibabacloud-go/ims-20190815/v2/client"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SessionKeyAccountID = "accountId"
	SessionKeyLoginID   = "loginId"
	SessionKeyName      = "name"
	SessionKeyLoginName = "loginName"
	SessionKeyUserNS    = "namespaces"
	SessionKeyToken     = "token"
	SessionKeyRole      = "role"

	RoleAdmin      = "admin"
	RoleResearcher = "researcher"

	defaultAppName = "kube-ai-dev-console"
	envMyPodName   = "MY_POD_NAME"
	oauthPath      = "/api/v1/login/aliyun/callback"
)

// Manager handles OAuth2 SSO login/logout and RAM Web App lifecycle.
type Manager struct {
	cfg        *config.AppConfig
	kubeClient *k8s.Client
	imsClient  *ims.Client
	oauthApp   *model.OAuthApp
}

func NewManager(cfg *config.AppConfig, kubeClient *k8s.Client, imsClient *ims.Client) (*Manager, error) {
	m := &Manager{
		cfg:        cfg,
		kubeClient: kubeClient,
		imsClient:  imsClient,
	}

	if cfg.IsCreateWebApp {
		if err := m.initOAuthApp(); err != nil {
			return nil, fmt.Errorf("init oauth app: %w", err)
		}
	}

	return m, nil
}

// HandleLoginRedirect redirects the user to RAM OAuth2 authorize endpoint.
func (m *Manager) HandleLoginRedirect(c *gin.Context) {
	if m.oauthApp == nil {
		response.Failed(c, "OAuth app not initialized")
		return
	}

	redirectURI := m.getRedirectURI(c)
	loginURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&access_type=offline",
		m.cfg.RamSigninURL(),
		url.QueryEscape(m.oauthApp.AppID),
		url.QueryEscape(redirectURI),
	)
	c.Redirect(http.StatusFound, loginURL)
}

// HandleLoginCallback handles the OAuth2 callback with authorization code.
func (m *Manager) HandleLoginCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Failed(c, "missing authorization code")
		return
	}

	// Exchange code for token
	redirectURI := m.getRedirectURI(c)
	token, err := m.exchangeToken(code, redirectURI)
	if err != nil {
		logrus.Errorf("exchange token failed: %v", err)
		response.Failed(c, "token exchange failed")
		return
	}

	// Get RAM user info
	ramUser, err := m.getUserInfo(token)
	if err != nil {
		logrus.Errorf("get user info failed: %v", err)
		response.Failed(c, "get user info failed")
		return
	}

	// Determine role and normalize name
	isAdmin := ramUser.Upn == ""
	var loginName, normalizedName string
	if isAdmin {
		loginName = ramUser.LoginName
		normalizedName = normalizeUserID(ramUser.Aid)
	} else {
		loginName = ramUser.Upn
		normalizedName = normalizeUserID(ramUser.Upn)
	}

	// Get user namespaces from User CRD
	namespaces := m.getUserNamespaces(normalizedName)

	// Get K8s SA token for the user
	k8sToken, err := m.kubeClient.GetServiceAccountToken(normalizedName, k8s.KubeAINamespace)
	if err != nil {
		logrus.Warnf("get k8s token for user %s failed: %v", normalizedName, err)
		// Not fatal — user may not have SA yet
	}

	// Save session
	session := sessions.Default(c)
	session.Set(SessionKeyAccountID, ramUser.Aid)
	session.Set(SessionKeyLoginID, ramUser.Uid)
	session.Set(SessionKeyName, normalizedName)
	session.Set(SessionKeyLoginName, loginName)
	session.Set(SessionKeyToken, k8sToken)
	if isAdmin {
		session.Set(SessionKeyRole, RoleAdmin)
	} else {
		session.Set(SessionKeyRole, RoleResearcher)
	}
	if len(namespaces) > 0 {
		nsJSON, _ := json.Marshal(namespaces)
		session.Set(SessionKeyUserNS, string(nsJSON))
	}
	if err := session.Save(); err != nil {
		logrus.Errorf("save session failed: %v", err)
	}

	// Redirect to frontend
	c.Redirect(http.StatusFound, "/")
}

// HandleTokenLogin authenticates a user with a Kubernetes Bearer Token.
func (m *Manager) HandleTokenLogin(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		response.Failed(c, "token is required")
		return
	}

	// Validate the token by making a TokenReview or SelfSubjectReview
	userName, err := m.validateK8sToken(req.Token)
	if err != nil {
		logrus.Warnf("token login failed: %v", err)
		response.Failed(c, "invalid token: authentication failed")
		return
	}

	// Get user namespaces
	normalizedName := normalizeUserID(userName)
	namespaces := m.getUserNamespaces(normalizedName)

	// Determine role from User CRD apiRoles field
	role := m.getUserRoleFromCRD(normalizedName)

	// Save session
	session := sessions.Default(c)
	session.Set(SessionKeyAccountID, "token-auth")
	session.Set(SessionKeyLoginID, userName)
	session.Set(SessionKeyName, normalizedName)
	session.Set(SessionKeyLoginName, userName)
	session.Set(SessionKeyRole, role)
	session.Set(SessionKeyToken, req.Token)
	if len(namespaces) > 0 {
		nsJSON, _ := json.Marshal(namespaces)
		session.Set(SessionKeyUserNS, string(nsJSON))
	}
	if err := session.Save(); err != nil {
		logrus.Errorf("save session: %v", err)
		response.Failed(c, "failed to save session")
		return
	}

	response.OK(c, model.UserInfo{
		Aid:        "token-auth",
		Uid:        userName,
		Name:       normalizedName,
		LoginName:  userName,
		Role:       role,
		Namespaces: namespaces,
	})
}

func (m *Manager) validateK8sToken(tokenStr string) (string, error) {
	// Use admin client's TokenReview to validate the token
	review := &authv1.TokenReview{
		Spec: authv1.TokenReviewSpec{
			Token: tokenStr,
		},
	}
	result, err := m.kubeClient.Typed().AuthenticationV1().TokenReviews().Create(
		context.TODO(), review, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("token review: %w", err)
	}
	if !result.Status.Authenticated {
		errMsg := result.Status.Error
		if errMsg == "" {
			errMsg = "token not authenticated"
		}
		return "", fmt.Errorf(errMsg)
	}
	return result.Status.User.Username, nil
}

// HandleLogout clears the session.
func (m *Manager) HandleLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	_ = session.Save()
	response.OKMsg(c, "logged out")
}

// HandleGetUserInfo returns current user info from session.
func (m *Manager) HandleGetUserInfo(c *gin.Context) {
	// Dev mode: return mock user
	if os.Getenv("DISABLE_AUTH") == "true" {
		response.OK(c, model.UserInfo{
			Aid:        "dev",
			Uid:        "dev",
			Name:       "dev-admin",
			LoginName:  "dev-admin",
			Role:       RoleAdmin,
			Namespaces: []string{"default", "default-group", "kube-ai"},
		})
		return
	}

	session := sessions.Default(c)
	name := session.Get(SessionKeyName)
	if name == nil {
		response.Unauthorized(c)
		return
	}

	role, _ := session.Get(SessionKeyRole).(string)
	var namespaces []string
	if nsJSON, ok := session.Get(SessionKeyUserNS).(string); ok && nsJSON != "" {
		_ = json.Unmarshal([]byte(nsJSON), &namespaces)
	}

	response.OK(c, model.UserInfo{
		Aid:        getSessionStr(session, SessionKeyAccountID),
		Uid:        getSessionStr(session, SessionKeyLoginID),
		Name:       getSessionStr(session, SessionKeyName),
		LoginName:  getSessionStr(session, SessionKeyLoginName),
		Role:       role,
		Namespaces: namespaces,
	})
}

// Cleanup deletes the OAuth web app on shutdown.
func (m *Manager) Cleanup() {
	if m.oauthApp == nil || m.imsClient == nil {
		return
	}
	if err := m.deleteOAuthApp(); err != nil {
		logrus.Errorf("cleanup oauth app failed: %v", err)
	} else {
		logrus.Info("oauth app deleted successfully")
	}
}

// --- Internal helpers ---

func (m *Manager) initOAuthApp() error {
	if m.imsClient == nil {
		return fmt.Errorf("IMS client not available")
	}

	appName := genAppName()
	displayName := genDisplayName(appName)

	// Try to find existing app
	app, appID, err := m.findApp(appName)
	if err != nil {
		return err
	}

	if app == nil {
		// Create new app
		appID, err = m.createApp(appName, displayName)
		if err != nil {
			return err
		}
	}

	// Update redirect URI if host is known (from env or ingress)
	if redirectURI := m.resolveStartupRedirectURI(); redirectURI != "" {
		if err := m.updateAppRedirectURI(appID, redirectURI); err != nil {
			logrus.Warnf("update app redirect URI: %v", err)
		}
	}

	// Get or create app secret
	secret, err := m.getOrCreateSecret(appID)
	if err != nil {
		return err
	}

	m.oauthApp = &model.OAuthApp{
		AppID:     appID,
		AppSecret: secret,
		AppName:   appName,
	}

	logrus.Infof("OAuth app ready: %s (ID: %s)", appName, appID)
	return nil
}

func (m *Manager) resolveStartupRedirectURI() string {
	// Check DEV_CONSOLE_HOST env var first
	if host := os.Getenv("DEV_CONSOLE_HOST"); host != "" {
		return fmt.Sprintf("http://%s%s", host, oauthPath)
	}
	// Try ingress host
	if os.Getenv("KUBE_DL_INGRESS_ENABLE") == "true" {
		host, err := m.kubeClient.GetIngressHostByRules("ack-ai-dev-console", k8s.KubeAINamespace)
		if err == nil && host != "" {
			return fmt.Sprintf("http://%s%s", host, oauthPath)
		}
	}
	return ""
}

func (m *Manager) updateAppRedirectURI(appID, redirectURI string) error {
	req := &ims.UpdateApplicationRequest{}
	req.SetAppId(appID)
	req.SetNewRedirectUris(redirectURI)
	_, err := m.imsClient.UpdateApplication(req)
	if err != nil {
		return fmt.Errorf("IMS UpdateApplication: %w", err)
	}
	logrus.Infof("OAuth app redirect URI updated: %s", redirectURI)
	return nil
}

func (m *Manager) findApp(appName string) (*string, string, error) {
	listReq := &ims.ListApplicationsRequest{}
	listResp, err := m.imsClient.ListApplications(listReq)
	if err != nil {
		return nil, "", err
	}
	for _, app := range listResp.Body.Applications.Application {
		if app.AppName != nil && *app.AppName == appName {
			return app.AppName, *app.AppId, nil
		}
	}
	return nil, "", nil
}

func (m *Manager) createApp(appName, displayName string) (string, error) {
	req := &ims.CreateApplicationRequest{}
	req.SetAppName(appName)
	req.SetAppType("WebApp")
	req.SetDisplayName(displayName)
	req.SetPredefinedScopes("openid;aliuid;profile")
	resp, err := m.imsClient.CreateApplication(req)
	if err != nil {
		return "", fmt.Errorf("create application: %w", err)
	}
	return *resp.Body.Application.AppId, nil
}

func (m *Manager) getOrCreateSecret(appID string) (string, error) {
	listReq := &ims.ListAppSecretIdsRequest{}
	listReq.SetAppId(appID)
	listResp, err := m.imsClient.ListAppSecretIds(listReq)
	if err != nil {
		return "", err
	}

	if len(listResp.Body.AppSecrets.AppSecret) > 0 {
		getReq := &ims.GetAppSecretRequest{}
		getReq.SetAppId(appID)
		getReq.SetAppSecretId(*listResp.Body.AppSecrets.AppSecret[0].AppSecretId)
		getResp, err := m.imsClient.GetAppSecret(getReq)
		if err != nil {
			return "", err
		}
		return *getResp.Body.AppSecret.AppSecretValue, nil
	}

	// Create new secret
	createReq := &ims.CreateAppSecretRequest{}
	createReq.SetAppId(appID)
	createResp, err := m.imsClient.CreateAppSecret(createReq)
	if err != nil {
		return "", err
	}
	return *createResp.Body.AppSecret.AppSecretValue, nil
}

func (m *Manager) deleteOAuthApp() error {
	if m.oauthApp == nil {
		return nil
	}
	_, appID, err := m.findApp(m.oauthApp.AppName)
	if err != nil || appID == "" {
		return err
	}
	req := &ims.DeleteApplicationRequest{}
	req.SetAppId(appID)
	_, err = m.imsClient.DeleteApplication(req)
	return err
}

func (m *Manager) exchangeToken(code, redirectURI string) (string, error) {
	if m.oauthApp == nil {
		return "", fmt.Errorf("oauth app not initialized")
	}

	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {redirectURI},
		"client_id":    {m.oauthApp.AppID},
		"client_secret": {m.oauthApp.AppSecret},
	}

	resp, err := http.PostForm(m.cfg.RamTokenURL(), data)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("token error: %s", tokenResp.Error)
	}
	return tokenResp.AccessToken, nil
}

func (m *Manager) getUserInfo(token string) (*model.RamUserInfo, error) {
	req, _ := http.NewRequest("GET", m.cfg.RamUserInfoURL(), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var userInfo model.RamUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}
	return &userInfo, nil
}

// getUserRoleFromCRD looks up the User CRD's spec.apiRoles field.
// If apiRoles contains "admin", returns RoleAdmin; otherwise RoleResearcher.
// If the CRD is not found or apiRoles field doesn't exist, defaults to RoleResearcher.
func (m *Manager) getUserRoleFromCRD(userName string) string {
	gvr := k8s.CRDGVR("users")
	obj, err := m.kubeClient.Dynamic().Resource(gvr).Namespace(k8s.KubeAINamespace).Get(
		context.TODO(), userName, metav1Options())
	if err != nil {
		logrus.Debugf("getUserRoleFromCRD: user CRD %q not found: %v", userName, err)
		return RoleResearcher
	}
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return RoleResearcher
	}
	apiRoles, ok := spec["apiRoles"].([]interface{})
	if !ok {
		return RoleResearcher
	}
	for _, r := range apiRoles {
		if s, ok := r.(string); ok && s == "admin" {
			return RoleAdmin
		}
	}
	return RoleResearcher
}

func (m *Manager) getUserNamespaces(userName string) []string {
	// Look up User CRD for namespace bindings
	gvr := k8s.CRDGVR("users")
	obj, err := m.kubeClient.Dynamic().Resource(gvr).Namespace(k8s.KubeAINamespace).Get(
		context.TODO(), userName, metav1Options())
	if err != nil {
		return nil
	}
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil
	}

	var namespaces []string

	// Method 1: Direct spec.namespaces field
	if nsList, ok := spec["namespaces"].([]interface{}); ok {
		for _, ns := range nsList {
			if s, ok := ns.(string); ok {
				namespaces = append(namespaces, s)
			}
		}
	}

	// Method 2: Extract from k8sServiceAccount.roleBindings[].namespace
	if saSpec, ok := spec["k8sServiceAccount"].(map[string]interface{}); ok {
		if bindings, ok := saSpec["roleBindings"].([]interface{}); ok {
			for _, rb := range bindings {
				if rbMap, ok := rb.(map[string]interface{}); ok {
					if ns, ok := rbMap["namespace"].(string); ok && ns != "" {
						namespaces = append(namespaces, ns)
					}
				}
			}
		}
	}

	// Method 3: Resolve from groups → UserGroup → quotaNames → ElasticQuotaTree leaf → namespaces
	if groups, ok := spec["groups"].([]interface{}); ok && len(namespaces) == 0 {
		for _, g := range groups {
			groupName, _ := g.(string)
			if groupName == "" {
				continue
			}
			nsFromGroup := m.resolveGroupNamespaces(groupName)
			namespaces = append(namespaces, nsFromGroup...)
		}
	}

	// Deduplicate
	return uniqueStrings(namespaces)
}

func (m *Manager) resolveGroupNamespaces(groupName string) []string {
	// Get UserGroup CRD → quotaNames → resolve from ElasticQuotaTree
	ugGVR := k8s.CRDGVR("usergroups")
	obj, err := m.kubeClient.Dynamic().Resource(ugGVR).Namespace(k8s.KubeAINamespace).Get(
		context.TODO(), groupName, metav1Options())
	if err != nil {
		return nil
	}
	spec, ok := obj.Object["spec"].(map[string]interface{})
	if !ok {
		return nil
	}
	quotaNames, ok := spec["quotaNames"].([]interface{})
	if !ok {
		return nil
	}

	// For each quota name, find the leaf node in ElasticQuotaTree and get its namespaces
	eqGVR := k8s.CRDGVR("elasticquotatrees")
	trees, err := m.kubeClient.Dynamic().Resource(eqGVR).Namespace("kube-system").List(
		context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil
	}

	var namespaces []string
	for _, tree := range trees.Items {
		treeSpec, _ := tree.Object["spec"].(map[string]interface{})
		if treeSpec == nil {
			continue
		}
		root, _ := treeSpec["root"].(map[string]interface{})
		if root == nil {
			continue
		}
		for _, qn := range quotaNames {
			qnStr, _ := qn.(string)
			if qnStr == "" {
				continue
			}
			if nsList := findNodeNamespaces(root, qnStr); len(nsList) > 0 {
				namespaces = append(namespaces, nsList...)
			}
		}
	}
	return namespaces
}

func findNodeNamespaces(node map[string]interface{}, targetName string) []string {
	name, _ := node["name"].(string)
	if name == targetName {
		var nsList []string
		if ns, ok := node["namespaces"].([]interface{}); ok {
			for _, n := range ns {
				if s, ok := n.(string); ok {
					nsList = append(nsList, s)
				}
			}
		}
		return nsList
	}
	if children, ok := node["children"].([]interface{}); ok {
		for _, child := range children {
			childMap, _ := child.(map[string]interface{})
			if childMap == nil {
				continue
			}
			if result := findNodeNamespaces(childMap, targetName); len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range input {
		if !seen[s] && s != "" {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func (m *Manager) getRedirectURI(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, oauthPath)
}

// --- Utility functions ---

func normalizeUserID(id string) string {
	s := strings.ToLower(id)
	s = strings.ReplaceAll(s, "@", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

func genAppName() string {
	return defaultAppName
}

func genDisplayName(appName string) string {
	podName := os.Getenv(envMyPodName)
	if podName == "" {
		return appName
	}
	parts := strings.Split(podName, "-")
	if len(parts) > 0 {
		return fmt.Sprintf("%s-%s", appName, parts[len(parts)-1])
	}
	return appName
}

func getSessionStr(session sessions.Session, key string) string {
	val := session.Get(key)
	if val == nil {
		return ""
	}
	s, _ := val.(string)
	return s
}
