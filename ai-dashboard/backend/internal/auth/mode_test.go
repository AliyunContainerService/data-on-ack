package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/AliyunContainerService/data-on-ack/ai-dashboard/backend/internal/config"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	config.Parse()
	os.Exit(m.Run())
}

func setAuthMode(t *testing.T, mode string) {
	t.Helper()
	old := config.Get().AuthMode
	config.Get().AuthMode = mode
	t.Cleanup(func() { config.Get().AuthMode = old })
}

func TestUserInList(t *testing.T) {
	tests := []struct {
		user, list string
		want       bool
	}{
		{"alice", "alice,bob", true},
		{"bob", " alice , bob ", true},
		{"carol", "alice,bob", false},
		{"alice", "", false},
		{"", "alice", false},
	}
	for _, tt := range tests {
		if got := userInList(tt.user, tt.list); got != tt.want {
			t.Errorf("userInList(%q, %q) = %v, want %v", tt.user, tt.list, got, tt.want)
		}
	}
}

func TestGroupsIntersect(t *testing.T) {
	tests := []struct {
		header, adminGroups string
		want                bool
	}{
		{"dev,ops", "ops", true},
		{"dev,ops", "ops,admin", true},
		{"dev", "ops,admin", false},
		{"dev", "", false},
		{"", "ops", false},
	}
	for _, tt := range tests {
		if got := groupsIntersect(tt.header, tt.adminGroups); got != tt.want {
			t.Errorf("groupsIntersect(%q, %q) = %v, want %v", tt.header, tt.adminGroups, got, tt.want)
		}
	}
}

func setupAuthRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	store := cookie.NewStore([]byte("test-secret"))
	r.Use(sessions.Sessions("test-session", store))
	r.Use(CheckAuth("/health"))
	r.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/protected", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func TestCheckAuthNoneMode(t *testing.T) {
	setAuthMode(t, ModeNone)
	r := setupAuthRouter()

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("none mode: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCheckAuthCustomMode(t *testing.T) {
	setAuthMode(t, ModeCustom)
	cfg := config.Get()
	oldUsers, oldGroups, oldLoginURL := cfg.AuthAdminUsers, cfg.AuthAdminGroups, cfg.AuthLoginURL
	cfg.AuthAdminUsers = "boss@example.com"
	cfg.AuthAdminGroups = "platform-admins"
	cfg.AuthLoginURL = "https://sso.example.com/login"
	t.Cleanup(func() {
		cfg.AuthAdminUsers, cfg.AuthAdminGroups, cfg.AuthLoginURL = oldUsers, oldGroups, oldLoginURL
	})

	t.Run("with user header", func(t *testing.T) {
		r := setupAuthRouter()
		r.GET("/whoami", func(c *gin.Context) {
			m := &Manager{cfg: config.Get()}
			_, _, loginName, role, ok := m.GetCurrentUser(c)
			c.JSON(http.StatusOK, gin.H{"loginName": loginName, "role": role, "ok": ok})
		})
		req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
		req.Header.Set("X-Forwarded-User", "boss@example.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if body := w.Body.String(); !strings.Contains(body, `"role":"admin"`) {
			t.Errorf("expected admin role, got %s", body)
		}
	})

	t.Run("group-based admin", func(t *testing.T) {
		r := setupAuthRouter()
		r.GET("/whoami", func(c *gin.Context) {
			m := &Manager{cfg: config.Get()}
			_, _, _, role, _ := m.GetCurrentUser(c)
			c.String(http.StatusOK, role)
		})
		req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
		req.Header.Set("X-Forwarded-User", "carol@example.com")
		req.Header.Set("X-Forwarded-Groups", "dev,platform-admins")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || w.Body.String() != "admin" {
			t.Errorf("status=%d role=%q, want 200 admin", w.Code, w.Body.String())
		}
	})

	t.Run("plain user is researcher", func(t *testing.T) {
		r := setupAuthRouter()
		r.GET("/whoami", func(c *gin.Context) {
			m := &Manager{cfg: config.Get()}
			_, _, _, role, _ := m.GetCurrentUser(c)
			c.String(http.StatusOK, role)
		})
		req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
		req.Header.Set("X-Forwarded-User", "dev@example.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK || w.Body.String() != RoleResearcher {
			t.Errorf("status=%d role=%q, want 200 researcher", w.Code, w.Body.String())
		}
	})

	t.Run("missing header returns 401 with loginUrl", func(t *testing.T) {
		r := setupAuthRouter()
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
		if !strings.Contains(w.Body.String(), "sso.example.com") {
			t.Errorf("expected loginUrl in body, got %s", w.Body.String())
		}
	})

	t.Run("missing header redirects browsers to loginUrl", func(t *testing.T) {
		r := setupAuthRouter()
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Accept", "text/html,application/xhtml+xml")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusFound {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
		}
		if loc := w.Header().Get("Location"); loc != "https://sso.example.com/login" {
			t.Errorf("Location = %q, want SSO login URL", loc)
		}
	})
}
