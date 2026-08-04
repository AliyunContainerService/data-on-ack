package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestNormalizeUserID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"user@example.com", "user-example.com"},
		{"User@Example.COM", "user-example.com"},
		{"foo_bar", "foo-bar"},
		{"already-clean", "already-clean"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeUserID(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeUserID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenAppName(t *testing.T) {
	got := GenAppName("c123abc")
	if got != "c123abc-kube-ai-dashboard" {
		t.Errorf("GenAppName = %q, want %q", got, "c123abc-kube-ai-dashboard")
	}
}

func TestParsePodID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ack-ai-dashboard-admin-ui-855469c4-tgh8t", "tgh8t"},
		{"pod-123", "123"},
		{"", ""},
		{"nopod", "nopod"},
	}
	for _, tt := range tests {
		got := parsePodID(tt.input)
		if got != tt.want {
			t.Errorf("parsePodID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetLoginURL(t *testing.T) {
	cfg := &config.AppConfig{IsIntlAccount: false}
	oauth := OAuthInfo{
		AppID:      "test-app-id",
		AppSecret:  "test-secret",
		RedirectURI: "http://localhost/login/aliyun",
	}
	url := GetLoginURL(cfg, oauth, "http://localhost/login/aliyun")

	if url == "" {
		t.Fatal("GetLoginURL returned empty URL")
	}
	if !contains(url, "client_id=test-app-id") {
		t.Errorf("URL missing client_id: %s", url)
	}
	if !contains(url, "redirect_uri=") {
		t.Errorf("URL missing redirect_uri: %s", url)
	}
	if !contains(url, "response_type=code") {
		t.Errorf("URL missing response_type: %s", url)
	}
	if !contains(url, "signin.aliyun.com") {
		t.Errorf("URL should use domestic signin domain: %s", url)
	}
}

func TestGetLoginURL_Intl(t *testing.T) {
	cfg := &config.AppConfig{IsIntlAccount: true}
	oauth := OAuthInfo{AppID: "intl-id", AppSecret: "s", RedirectURI: "http://host/login/aliyun"}
	url := GetLoginURL(cfg, oauth, "http://host/login/aliyun")
	if !contains(url, "signin.alibabacloud.com") {
		t.Errorf("intl URL should use alibabacloud domain: %s", url)
	}
}

func TestCheckAuth_PublicPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := cookie.NewStore()
	r.Use(sessions.Sessions("test", store))

	r.Use(CheckAuth("/health", "/login"))
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.GET("/protected", func(c *gin.Context) { c.JSON(200, gin.H{"secret": true}) })

	// /health should be accessible without session
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("health status = %d, want 200", w.Code)
	}

	// /protected should redirect to /login (302)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusFound {
		t.Errorf("protected status = %d, want 302", w.Code)
	}
}

func TestCheckAuth_StaticAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := cookie.NewStore()
	r.Use(sessions.Sessions("test", store))

	r.Use(CheckAuth("/health"))
	r.GET("/app.js", func(c *gin.Context) { c.String(200, "js") })
	r.GET("/style.css", func(c *gin.Context) { c.String(200, "css") })

	for _, path := range []string{"/app.js", "/style.css"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("%s status = %d, want 200 (static asset bypass)", path, w.Code)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && indexOf(s, substr) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
