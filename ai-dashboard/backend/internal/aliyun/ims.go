package aliyun

import (
	"fmt"
	"sync"
	"time"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/credential"
	openapi "github.com/alibabacloud-go/darabonba-openapi/client"
	ims "github.com/alibabacloud-go/ims-20190815/v2/client"
)

// IMSClient wraps the IMS SDK client with credential auto-refresh.
type IMSClient struct {
	credManager *credential.Manager
	cfg         *config.AppConfig
	mu          sync.Mutex
	client      *ims.Client
}

func NewIMSClient(credManager *credential.Manager, cfg *config.AppConfig) (*IMSClient, error) {
	c := &IMSClient{credManager: credManager, cfg: cfg}
	if _, err := c.GetClient(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *IMSClient) GetClient() (*ims.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ak, err := c.credManager.Get()
	if err != nil {
		return nil, fmt.Errorf("get credentials: %w", err)
	}

	// Rebuild client if nil or if credentials changed (STS token expired)
	if c.client != nil && !needsRefresh(ak) {
		return c.client, nil
	}

	endpoint := c.cfg.RamIMSDomain()
	conf := &openapi.Config{}
	conf.AccessKeyId = &ak.AccessKeyID
	conf.AccessKeySecret = &ak.AccessKeySecret
	conf.SecurityToken = &ak.SecurityToken
	conf.Endpoint = &endpoint

	client, err := ims.NewClient(conf)
	if err != nil {
		return nil, fmt.Errorf("create IMS client: %w", err)
	}
	c.client = client
	return c.client, nil
}

func needsRefresh(ak *credential.AKInfo) bool {
	if ak.Expiration == "" {
		return false
	}
	layout := "2006-01-02T15:04:05Z"
	t, err := time.Parse(layout, ak.Expiration)
	if err != nil {
		return true
	}
	return t.Before(time.Now())
}

// RamUser represents a RAM user from IMS API.
type RamUser struct {
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
	DisplayName string `json:"displayName"`
	CreateDate  string `json:"createDate"`
	UpdateDate  string `json:"updateDate"`
}

// ListUsers lists all RAM users via IMS API.
func (c *IMSClient) ListUsers() ([]RamUser, error) {
	client, err := c.GetClient()
	if err != nil {
		return nil, err
	}

	req := &ims.ListUsersRequest{}
	resp, err := client.ListUsers(req)
	if err != nil {
		return nil, fmt.Errorf("IMS ListUsers: %w", err)
	}

	users := make([]RamUser, 0)
	for _, u := range resp.Body.Users.User {
		ru := RamUser{}
		if u.UserId != nil {
			ru.UserID = *u.UserId
		}
		if u.UserPrincipalName != nil {
			ru.UserName = *u.UserPrincipalName
		}
		if u.DisplayName != nil {
			ru.DisplayName = *u.DisplayName
		}
		if u.CreateDate != nil {
			ru.CreateDate = *u.CreateDate
		}
		if u.UpdateDate != nil {
			ru.UpdateDate = *u.UpdateDate
		}
		users = append(users, ru)
	}
	return users, nil
}
