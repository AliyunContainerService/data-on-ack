package aliyun

import (
	"fmt"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/internal/credential"
	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	ims "github.com/alibabacloud-go/ims-20190815/v2/client"
	"github.com/alibabacloud-go/tea/tea"
)

// NewIMSClient creates an Alibaba Cloud IMS client using credential manager.
func NewIMSClient(credMgr *credential.Manager, cfg *config.AppConfig) (*ims.Client, error) {
	ak, err := credMgr.Get()
	if err != nil {
		return nil, fmt.Errorf("get credentials for IMS: %w", err)
	}

	endpoint := cfg.RamIMSDomain()
	imsConfig := &openapi.Config{
		AccessKeyId:     tea.String(ak.AccessKeyID),
		AccessKeySecret: tea.String(ak.AccessKeySecret),
		Endpoint:        tea.String(endpoint),
	}
	if ak.SecurityToken != "" {
		imsConfig.SecurityToken = tea.String(ak.SecurityToken)
	}

	client, err := ims.NewClient(imsConfig)
	if err != nil {
		return nil, fmt.Errorf("create IMS client: %w", err)
	}
	return client, nil
}
