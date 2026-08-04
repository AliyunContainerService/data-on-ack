package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func TestOK(t *testing.T) {
	r := setupRouter()
	r.GET("/test", func(c *gin.Context) {
		OK(c, map[string]string{"hello": "world"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var res Result
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.Code != CodeSuccess {
		t.Errorf("code = %d, want %d", res.Code, CodeSuccess)
	}
	if res.Message != "ok" {
		t.Errorf("message = %q, want %q", res.Message, "ok")
	}
}

func TestFailed(t *testing.T) {
	r := setupRouter()
	r.GET("/test", func(c *gin.Context) {
		Failed(c, CodeUserNotFound, "user not found")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	var res Result
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if res.Code != CodeUserNotFound {
		t.Errorf("code = %d, want %d", res.Code, CodeUserNotFound)
	}
	if res.Message != "user not found" {
		t.Errorf("message = %q, want %q", res.Message, "user not found")
	}
}

func TestPagination(t *testing.T) {
	p := Pagination{Total: 5, Items: []string{"a", "b", "c"}}
	if p.Total != 5 {
		t.Errorf("Total = %d, want 5", p.Total)
	}
}
