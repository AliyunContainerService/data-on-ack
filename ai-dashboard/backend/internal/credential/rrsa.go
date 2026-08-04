package credential

import (
	"fmt"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/aliyun/credentials-go/credentials"
)

// RRSAProvider obtains STS credentials via OIDC token → AssumeRoleWithOIDC.
// The credentials-go library handles STS token auto-refresh internally.
type RRSAProvider struct {
	cred credentials.Credential
}

func NewRRSAProvider(cfg *config.AppConfig) (*RRSAProvider, error) {
	if cfg.OIDCProviderARN == "" {
		return nil, fmt.Errorf("rrsa credential mode requires OIDC_PROVIDER_ARN env var")
	}

	credConfig := &credentials.Config{}
	credConfig.SetType("oidc_role_arn")
	credConfig.SetOIDCProviderArn(cfg.OIDCProviderARN)
	credConfig.SetOIDCTokenFilePath(cfg.OIDCTokenFile)
	credConfig.SetRoleArn(cfg.OIDCProviderARN)

	cred, err := credentials.NewCredential(credConfig)
	if err != nil {
		return nil, fmt.Errorf("create rrsa credential: %w", err)
	}
	return &RRSAProvider{cred: cred}, nil
}

func (p *RRSAProvider) GetAKInfo() (*AKInfo, error) {
	model, err := p.cred.GetCredential()
	if err != nil {
		return nil, fmt.Errorf("get credential from rrsa: %w", err)
	}

	ak := &AKInfo{}
	if model.AccessKeyId != nil {
		ak.AccessKeyID = *model.AccessKeyId
	}
	if model.AccessKeySecret != nil {
		ak.AccessKeySecret = *model.AccessKeySecret
	}
	if model.SecurityToken != nil {
		ak.SecurityToken = *model.SecurityToken
	}

	// credentials-go handles refresh internally; use a 5-minute TTL
	// so the Manager cache refreshes periodically.
	ak.Expiration = time.Now().UTC().Add(5 * time.Minute).Format("2006-01-02T15:04:05Z")

	return ak, nil
}
