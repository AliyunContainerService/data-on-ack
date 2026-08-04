package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/aliyun"
	ims "github.com/alibabacloud-go/ims-20190815/v2/client"
)

const (
	envMyPodName    = "MY_POD_NAME"
	defaultScopes   = "aliuid;profile"
	webAppType      = "WebApp"
)

// WebAppManager manages the RAM OAuth2 web application lifecycle.
type WebAppManager struct {
	imsClient *aliyun.IMSClient
	mu        sync.RWMutex
	cached    *OAuthInfo
}

func NewWebAppManager(imsClient *aliyun.IMSClient) *WebAppManager {
	return &WebAppManager{imsClient: imsClient}
}

// EnsureApp creates or updates the RAM web app and returns OAuth credentials.
func (m *WebAppManager) EnsureApp(appName, redirectURI string) (*OAuthInfo, error) {
	m.mu.RLock()
	cached := m.cached
	m.mu.RUnlock()
	if cached != nil {
		return cached, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cached != nil {
		return m.cached, nil
	}

	client, err := m.imsClient.GetClient()
	if err != nil {
		return nil, fmt.Errorf("get IMS client: %w", err)
	}

	app, err := getOrCreateApplication(client, appName, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("get/create web app: %w", err)
	}

	secret, err := getOrCreateAppSecret(client, app.AppID)
	if err != nil {
		return nil, fmt.Errorf("get/create app secret: %w", err)
	}

	m.cached = &OAuthInfo{
		AppID:      app.AppID,
		AppSecret:  secret.AppSecretValue,
		RedirectURI: redirectURI,
	}
	return m.cached, nil
}

// DeleteApp removes the RAM web app on shutdown.
func (m *WebAppManager) DeleteApp(appName string) error {
	client, err := m.imsClient.GetClient()
	if err != nil {
		return fmt.Errorf("get IMS client: %w", err)
	}
	return deleteApplication(client, appName)
}

// GenAppName generates the web app name based on cluster ID.
func GenAppName(clusterID string) string {
	return fmt.Sprintf("%s-kube-ai-dashboard", clusterID)
}

// GenDisplayName generates a display name with pod ID suffix.
func GenDisplayName(appName string) string {
	podName := os.Getenv(envMyPodName)
	podID := parsePodID(podName)
	if podID == "" {
		return appName
	}
	return fmt.Sprintf("%s-%s", appName, podID)
}

func parsePodID(podName string) string {
	if podName == "" {
		return ""
	}
	parts := strings.Split(podName, "-")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func getOrCreateApplication(client *ims.Client, appName, redirectURI string) (*Application, error) {
	existing, err := findApplication(client, appName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := updateApplication(client, existing.AppID, redirectURI, appName); err != nil {
			return existing, err
		}
		return existing, nil
	}

	displayName := GenDisplayName(appName)
	req := &ims.CreateApplicationRequest{}
	req.SetAppName(appName)
	req.SetAppType(webAppType)
	req.SetDisplayName(displayName)
	req.SetRedirectUris(redirectURI)
	req.SetPredefinedScopes(defaultScopes)

	resp, err := client.CreateApplication(req)
	if err != nil {
		return nil, fmt.Errorf("IMS CreateApplication: %w", err)
	}

	app := &Application{}
	if resp.Body.Application != nil {
		if err := deserializeIms(resp.Body.Application, app); err != nil {
			return nil, err
		}
	}
	return app, nil
}

func getOrCreateAppSecret(client *ims.Client, appID string) (*AppSecret, error) {
	listReq := &ims.ListAppSecretIdsRequest{}
	listReq.SetAppId(appID)
	listResp, err := client.ListAppSecretIds(listReq)
	if err != nil {
		return nil, fmt.Errorf("IMS ListAppSecretIds: %w", err)
	}

	if len(listResp.Body.AppSecrets.AppSecret) > 0 {
		secretID := ""
		if listResp.Body.AppSecrets.AppSecret[0].AppSecretId != nil {
			secretID = *listResp.Body.AppSecrets.AppSecret[0].AppSecretId
		}
		getReq := &ims.GetAppSecretRequest{}
		getReq.SetAppId(appID)
		getReq.SetAppSecretId(secretID)
		getResp, err := client.GetAppSecret(getReq)
		if err != nil {
			return nil, fmt.Errorf("IMS GetAppSecret: %w", err)
		}
		secret := &AppSecret{AppSecretID: secretID}
		if getResp.Body.AppSecret != nil {
			if v := getResp.Body.AppSecret.AppSecretValue; v != nil {
				secret.AppSecretValue = *v
			}
		}
		return secret, nil
	}

	createReq := &ims.CreateAppSecretRequest{}
	createReq.SetAppId(appID)
	createResp, err := client.CreateAppSecret(createReq)
	if err != nil {
		return nil, fmt.Errorf("IMS CreateAppSecret: %w", err)
	}
	secret := &AppSecret{}
	if createResp.Body.AppSecret != nil {
		if v := createResp.Body.AppSecret.AppSecretId; v != nil {
			secret.AppSecretID = *v
		}
		if v := createResp.Body.AppSecret.AppSecretValue; v != nil {
			secret.AppSecretValue = *v
		}
	}
	return secret, nil
}

func findApplication(client *ims.Client, appName string) (*Application, error) {
	resp, err := client.ListApplications()
	if err != nil {
		return nil, fmt.Errorf("IMS ListApplications: %w", err)
	}

	for _, app := range resp.Body.Applications.Application {
		if app.AppName == nil {
			continue
		}
		if *app.AppName == appName {
			res := &Application{}
			if err := deserializeIms(app, res); err != nil {
				return nil, err
			}
			return res, nil
		}
	}
	return nil, nil
}

func updateApplication(client *ims.Client, appID, redirectURI, appName string) error {
	displayName := GenDisplayName(appName)
	req := &ims.UpdateApplicationRequest{}
	req.SetAppId(appID)
	req.SetNewDisplayName(displayName)
	req.SetNewRedirectUris(redirectURI)

	if _, err := client.UpdateApplication(req); err != nil {
		return fmt.Errorf("IMS UpdateApplication: %w", err)
	}
	return nil
}

func deleteApplication(client *ims.Client, appName string) error {
	app, err := findApplication(client, appName)
	if err != nil {
		return err
	}
	if app == nil {
		return nil
	}

	podName := os.Getenv(envMyPodName)
	if podName != "" && parsePodIDFromDisplayName(app.DisplayName) != parsePodID(podName) {
		return nil
	}

	req := &ims.DeleteApplicationRequest{}
	req.SetAppId(app.AppID)
	_, err = client.DeleteApplication(req)
	return err
}

func parsePodIDFromDisplayName(displayName string) string {
	return parsePodID(displayName)
}

func deserializeIms(src interface{}, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
