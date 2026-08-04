package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeSuccess = 10000
	CodeFailed  = 50000
	CodeUnauth  = 40100
)

// Result is the unified API response structure.
type Result struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Pagination wraps paginated results.
type Pagination struct {
	Total int64       `json:"total"`
	Items interface{} `json:"items"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Result{
		Code:    CodeSuccess,
		Message: "ok",
		Data:    data,
	})
}

func OKMsg(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Result{
		Code:    CodeSuccess,
		Message: msg,
	})
}

func OKPagination(c *gin.Context, total int64, items interface{}) {
	c.JSON(http.StatusOK, Result{
		Code:    CodeSuccess,
		Message: "ok",
		Data: Pagination{
			Total: total,
			Items: items,
		},
	})
}

func Failed(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Result{
		Code:    CodeFailed,
		Message: msg,
	})
}

func FailedWithCode(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Result{
		Code:    code,
		Message: msg,
	})
}

func Unauthorized(c *gin.Context) {
	c.JSON(http.StatusUnauthorized, Result{
		Code:    CodeUnauth,
		Message: "unauthorized",
	})
}
