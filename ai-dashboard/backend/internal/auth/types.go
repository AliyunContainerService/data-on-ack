package auth

// OAuthInfo holds the RAM OAuth2 web app credentials.
type OAuthInfo struct {
	AppID      string
	AppSecret  string
	RedirectURI string
}

// RamUserInfo is the user profile returned by the RAM OAuth2 userinfo endpoint.
type RamUserInfo struct {
	Aid       string `json:"aid"`        // Alibaba Cloud account ID
	Uid       string `json:"uid"`        // RAM sub-account ID
	Upn       string `json:"upn"`        // User Principal Name (sub-account)
	LoginName string `json:"login_name"` // Login name (main account)
}

// Application represents a RAM OAuth2 web application.
type Application struct {
	AppID        string      `json:"AppId"`
	AppName      string      `json:"AppName"`
	DisplayName  string      `json:"DisplayName"`
	RedirectUris interface{} `json:"RedirectUris"`
	AppType      string      `json:"AppType"`
}

// AppSecret holds the secret for a RAM OAuth2 web application.
type AppSecret struct {
	AppSecretID    string
	AppSecretValue string
}

// Session keys
const (
	SessionKeyAccountID = "accountId"
	SessionKeyUserID     = "userId"
	SessionKeyLoginName  = "loginName"
	SessionKeyRole       = "role"
	SessionKeyToken      = "token"
)

const (
	RoleAdmin      = "admin"
	RoleResearcher = "researcher"
)
