package config

import (
	"fmt"
	"net"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"
)

// AppConfig holds all configuration for ai-dev-console.
type AppConfig struct {
	ListenAddr     string
	GinMode        string
	CredentialMode string // "static" or "rrsa"
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
	// Frontend
	FrontendDir string
}

var cfg *AppConfig

func Parse() {
	var (
		listenAddr string
		ginMode    string
		credMode   string
		isIntl     bool
		disableOAuth bool
		adminUID   string
	)

	pflag.StringVar(&listenAddr, "listen-addr", ":9090", "HTTP listen address")
	pflag.StringVar(&ginMode, "gin-mode", gin.ReleaseMode, "Gin mode (debug/release/test)")
	pflag.StringVar(&credMode, "credential-mode", "", "Credential mode: static or rrsa (auto-detect if empty)")
	pflag.BoolVar(&isIntl, "intl-account", false, "International account (signin.alibabacloud.com)")
	pflag.BoolVar(&disableOAuth, "disable-oauth", false, "Disable OAuth (for local dev)")
	pflag.StringVar(&adminUID, "admin-uid", "", "Admin user aliuid")
	pflag.Parse()

	cfg = &AppConfig{
		ListenAddr:      listenAddr,
		GinMode:         ginMode,
		CredentialMode:  resolveCredMode(credMode),
		AccessKeyID:     os.Getenv("AK_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("AK_ACCESS_KEY_SECRET"),
		OIDCProviderARN: os.Getenv("OIDC_PROVIDER_ARN"),
		OIDCTokenFile:   getEnvDefault("OIDC_TOKEN_FILE", "/var/run/secrets/tokens/oidc-token"),
		RoleARN:         os.Getenv("OIDC_ROLE_ARN"),
		IsIntlAccount:   isIntl || os.Getenv("INTL_ACCOUNT") == "true",
		IsCreateWebApp:  !disableOAuth,
		AdminUID:        getEnvDefault("CONSOLE_ADMINUID", adminUID),
		FrontendDir:     getEnvDefault("FRONTEND_DIR", "./dist"),
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

// RamOAuthDomain returns the best available OAuth domain (prefer VPC proxy).
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

// RamIMSDomain returns the best available IMS endpoint.
func (c *AppConfig) RamIMSDomain() string {
	domain := "ims.vpc-proxy.aliyuncs.com"
	if !isDomainAvailable(domain) {
		return "ims.aliyuncs.com"
	}
	return domain
}

func isDomainAvailable(domain string) bool {
	if host, _, err := net.SplitHostPort(domain); err == nil {
		domain = host
	}
	_, err := net.LookupHost(domain)
	return err == nil
}
