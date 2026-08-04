package credential

import (
	"errors"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
)

// NewProvider creates the credential provider based on config.
func NewProvider(cfg *config.AppConfig) (Provider, error) {
	switch cfg.CredentialMode {
	case "rrsa":
		return NewRRSAProvider(cfg)
	case "static":
		return NewStaticProvider(cfg)
	default:
		// Auto-detect: if RRSA env vars are set, use RRSA; otherwise static
		if cfg.OIDCProviderARN != "" {
			return NewRRSAProvider(cfg)
		}
		return NewStaticProvider(cfg)
	}
}

// StaticProvider reads AK/SK from environment variables.
type StaticProvider struct {
	accessKeyID     string
	accessKeySecret string
}

func NewStaticProvider(cfg *config.AppConfig) (*StaticProvider, error) {
	if cfg.AccessKeyID == "" || cfg.AccessKeySecret == "" {
		return nil, errors.New("static credential mode requires AK_ACCESS_KEY_ID and AK_ACCESS_KEY_SECRET env vars")
	}
	return &StaticProvider{
		accessKeyID:     cfg.AccessKeyID,
		accessKeySecret: cfg.AccessKeySecret,
	}, nil
}

func (p *StaticProvider) GetAKInfo() (*AKInfo, error) {
	return &AKInfo{
		AccessKeyID:     p.accessKeyID,
		AccessKeySecret: p.accessKeySecret,
		// SecurityToken and Expiration are empty for static keys
	}, nil
}
