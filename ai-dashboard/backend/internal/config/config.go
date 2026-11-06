package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

type AppConfig struct {
	ListenAddr       string
	GinMode          string
	CredentialMode   string // "static" or "rrsa"
	// Auth
	AuthMode         string // "aliyun" (RAM OAuth2 SSO), "none" (no login, full access), "custom" (external user system via trusted headers)
	AuthUserHeader   string // custom mode: header carrying the authenticated username
	AuthGroupsHeader string // custom mode: header carrying comma-separated groups
	AuthAdminUsers   string // custom mode: comma-separated usernames granted the admin role
	AuthAdminGroups  string // custom mode: comma-separated groups granted the admin role
	AuthLoginURL     string // custom mode: where to redirect unauthenticated browser requests (e.g. customer SSO)
	// Static credential
	AccessKeyID     string
	AccessKeySecret string
	// RRSA credential
	OIDCProviderARN string
	OIDCTokenFile   string
	RoleARN         string
	// OAuth
	IsIntlAccount  bool
	IsCreateWebApp bool
	AdminUID       string
	// Grafana
	GrafanaProxyTarget string
	// Frontend
	FrontendDir string
}

var (
	cfg      *AppConfig
	listenAddr  string
	ginMode     string
	credMode    string
	authMode    string
	isIntl      bool
	disableOAuth bool
	adminUID    string
)

func Parse() {
	pflag.StringVar(&listenAddr, "listen-addr", ":8080", "HTTP listen address")
	pflag.StringVar(&ginMode, "gin-mode", gin.ReleaseMode, "Gin mode (debug/release/test)")
	pflag.StringVar(&credMode, "credential-mode", "", "Credential mode: static or rrsa (default: from env CREDENTIAL_MODE)")
	pflag.StringVar(&authMode, "auth-mode", "", "Auth mode: aliyun, none, custom (default: from env AUTH_MODE, or aliyun)")
	pflag.BoolVar(&isIntl, "intl-account", false, "International account (signin.alibabacloud.com)")
	pflag.BoolVar(&disableOAuth, "disable-oauth", false, "Disable OAuth (for local dev)")
	pflag.StringVar(&adminUID, "admin-uid", "", "Admin user aliuid")
	pflag.Parse()

	resolvedAuthMode := resolveAuthMode(authMode)

	cfg = &AppConfig{
		ListenAddr:         listenAddr,
		GinMode:            ginMode,
		CredentialMode:     resolveCredMode(credMode),
		AuthMode:           resolvedAuthMode,
		AuthUserHeader:     getEnvDefault("AUTH_USER_HEADER", "X-Forwarded-User"),
		AuthGroupsHeader:   getEnvDefault("AUTH_GROUPS_HEADER", "X-Forwarded-Groups"),
		AuthAdminUsers:     os.Getenv("AUTH_ADMIN_USERS"),
		AuthAdminGroups:    os.Getenv("AUTH_ADMIN_GROUPS"),
		AuthLoginURL:       os.Getenv("AUTH_LOGIN_URL"),
		AccessKeyID:        os.Getenv("AK_ACCESS_KEY_ID"),
		AccessKeySecret:   os.Getenv("AK_ACCESS_KEY_SECRET"),
		OIDCProviderARN:    os.Getenv("OIDC_PROVIDER_ARN"),
		OIDCTokenFile:      getEnvDefault("OIDC_TOKEN_FILE", "/var/run/secrets/tokens/oidc-token"),
		RoleARN:            os.Getenv("OIDC_ROLE_ARN"),
		IsIntlAccount:      isIntl || os.Getenv("INTL_ACCOUNT") == "true",
		IsCreateWebApp:     !disableOAuth && resolvedAuthMode == "aliyun",
		AdminUID:           getEnvDefault("DASHBOARD_ADMINUID", adminUID),
		GrafanaProxyTarget: getEnvDefault("GRAFANA_PROXY_TARGET", "arena-exporter-grafana.kube-ai:80"),
		FrontendDir:        getEnvDefault("FRONTEND_DIR", "./dist"),
	}
}

func Get() *AppConfig {
	return cfg
}

func resolveCredMode(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return getEnvDefault("CREDENTIAL_MODE", "static")
}

// resolveAuthMode determines the auth mode from the --auth-mode flag, the
// AUTH_MODE env var, or the legacy DISABLE_AUTH env var. Defaults to "aliyun".
func resolveAuthMode(flagVal string) string {
	mode := strings.ToLower(strings.TrimSpace(flagVal))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(os.Getenv("AUTH_MODE")))
	}
	if mode == "" && os.Getenv("DISABLE_AUTH") == "true" {
		logrus.Warn("DISABLE_AUTH is deprecated, please use AUTH_MODE=none instead")
		mode = "none"
	}
	switch mode {
	case "", "aliyun":
		return "aliyun"
	case "none", "custom":
		return mode
	default:
		logrus.Warnf("unknown AUTH_MODE %q, falling back to %q", mode, "aliyun")
		return "aliyun"
	}
}

func getEnvDefault(key, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	return val
}

// RamSigninURL returns the OAuth2 authorize endpoint for RAM.
// Can be overridden via OAUTH_SIGNIN_URL env var.
func (c *AppConfig) RamSigninURL() string {
	if url := os.Getenv("OAUTH_SIGNIN_URL"); url != "" {
		return url
	}
	if c.IsIntlAccount {
		return "https://signin.alibabacloud.com/oauth2/v1/auth"
	}
	return "https://signin.aliyun.com/oauth2/v1/auth"
}

func (c *AppConfig) RamOAuthDomain() string {
	domain := "oauth.vpc-proxy.aliyuncs.com"
	if c.IsIntlAccount {
		domain = "oauth-intl.vpc-proxy.aliyuncs.com"
	}
	if !isDomainAvailable(domain) {
		if c.IsIntlAccount {
			return "oauth.alibabacloud.com"
		}
		return "oauth.aliyun.com"
	}
	return domain
}

// RamTokenURL returns the OAuth2 token endpoint.
// Can be overridden via OAUTH_TOKEN_URL env var.
func (c *AppConfig) RamTokenURL() string {
	if url := os.Getenv("OAUTH_TOKEN_URL"); url != "" {
		return url
	}
	return fmt.Sprintf("https://%s/v1/token", c.RamOAuthDomain())
}

// RamUserInfoURL returns the OAuth2 userinfo endpoint.
// Can be overridden via OAUTH_USERINFO_URL env var.
func (c *AppConfig) RamUserInfoURL() string {
	if url := os.Getenv("OAUTH_USERINFO_URL"); url != "" {
		return url
	}
	return fmt.Sprintf("https://%s/v1/userinfo", c.RamOAuthDomain())
}

func (c *AppConfig) RamIMSDomain() string {
	domain := "ims.vpc-proxy.aliyuncs.com"
	if !isDomainAvailable(domain) {
		return "ims.aliyuncs.com"
	}
	return domain
}

func isDomainAvailable(domain string) bool {
	// Strip port if present
	if host, _, err := net.SplitHostPort(domain); err == nil {
		domain = host
	}
	// Try DNS resolution; if it resolves, assume the VPC endpoint is reachable.
	_, err := net.LookupHost(domain)
	return err == nil
}
