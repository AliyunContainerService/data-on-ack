package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
)

// ExchangeToken exchanges the authorization code for an access token.
func ExchangeToken(cfg *config.AppConfig, oauth OAuthInfo, redirectURL, code string) (string, error) {
	params := url.Values{}
	params.Set("code", code)
	params.Set("client_id", oauth.AppID)
	params.Set("redirect_uri", redirectURL)
	params.Set("grant_type", "authorization_code")
	params.Set("client_secret", oauth.AppSecret)

	tokenURL := cfg.RamTokenURL()
	resp, err := http.PostForm(tokenURL, params)
	if err != nil {
		return "", fmt.Errorf("exchange token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}

	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("access_token not found in response: %s", string(body))
	}
	return token, nil
}

// FetchUserInfo calls the RAM userinfo endpoint with the access token.
func FetchUserInfo(cfg *config.AppConfig, accessToken string) (*RamUserInfo, error) {
	userInfoURL := cfg.RamUserInfoURL()
	req, err := http.NewRequest(http.MethodGet, userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch user info: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read userinfo response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var info RamUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("parse userinfo: %w", err)
	}
	return &info, nil
}

// GetLoginURL builds the RAM SSO authorization URL for the user to visit.
func GetLoginURL(cfg *config.AppConfig, oauth OAuthInfo, callbackURL string) string {
	vals := url.Values{}
	vals.Set("client_id", oauth.AppID)
	vals.Set("redirect_uri", callbackURL)
	vals.Set("response_type", "code")
	authURL, _ := url.Parse(cfg.RamSigninURL())
	authURL.RawQuery = vals.Encode()
	return authURL.String()
}

// NormalizeUserID converts a RAM user principal name to a K8s-safe name:
// lowercase, replace @ with -, replace _ with -.
func NormalizeUserID(upn string) string {
	s := strings.ToLower(upn)
	s = strings.ReplaceAll(s, "@", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}
