package credential

import (
	"fmt"
	"os"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/config"
	"github.com/aliyun/credentials-go/credentials"
)

// RRSAProvider obtains STS credentials via OIDC token -> AssumeRoleWithOIDC.
// Uses credentials-go's built-in "oidc_role_arn" type which handles STS refresh.
type RRSAProvider struct {
	cred credentials.Credential
}

func NewRRSAProvider(cfg *config.AppConfig) (*RRSAProvider, error) {
	if cfg.OIDCProviderARN == "" {
		return nil, fmt.Errorf("rrsa credential mode requires OIDC_PROVIDER_ARN env var")
	}

	// Set environment variables that credentials-go reads for OIDC
	os.Setenv("ALIBABA_CLOUD_OIDC_PROVIDER_ARN", cfg.OIDCProviderARN)
	os.Setenv("ALIBABA_CLOUD_OIDC_TOKEN_FILE", cfg.OIDCTokenFile)
	os.Setenv("ALIBABA_CLOUD_ROLE_ARN", cfg.OIDCProviderARN)

	credConfig := new(credentials.Config).SetType("oidc_role_arn")
	cred, err := credentials.NewCredential(credConfig)
	if err != nil {
		return nil, fmt.Errorf("create rrsa credential: %w", err)
	}
	return &RRSAProvider{cred: cred}, nil
}

func (p *RRSAProvider) GetAKInfo() (*AKInfo, error) {
	akID, err := p.cred.GetAccessKeyId()
	if err != nil {
		return nil, fmt.Errorf("get access key id: %w", err)
	}
	akSecret, err := p.cred.GetAccessKeySecret()
	if err != nil {
		return nil, fmt.Errorf("get access key secret: %w", err)
	}
	token, err := p.cred.GetSecurityToken()
	if err != nil {
		return nil, fmt.Errorf("get security token: %w", err)
	}

	ak := &AKInfo{
		AccessKeyID:     *akID,
		AccessKeySecret: *akSecret,
		SecurityToken:   *token,
		// 5-minute TTL so the Manager cache refreshes periodically.
		Expiration: time.Now().UTC().Add(5 * time.Minute).Format("2006-01-02T15:04:05Z"),
	}
	return ak, nil
}
