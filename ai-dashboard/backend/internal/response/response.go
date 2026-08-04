package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CodeSuccess        = 10000
	CodeUserNotLogin   = 10101
	CodeUserAuthFailed = 10102
	CodeUserNotFound   = 10103
	CodeGroupError     = 10201
	CodeDatasetError   = 10301
	CodeResearcherErr  = 10401
	CodeQuotaError     = 10501
	CodeK8sError       = 10601
)

type Result struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Result{
		Code:    CodeSuccess,
		Message: "ok",
		Data:    data,
	})
}

func OKMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, Result{
		Code:    CodeSuccess,
		Message: msg,
		Data:    data,
	})
}

func Failed(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Result{
		Code:    code,
		Message: msg,
	})
}

func FailedWithData(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, Result{
		Code:    code,
		Message: msg,
		Data:    data,
	})
}

// Pagination is a generic paginated response wrapper.
type Pagination struct {
	Total int64       `json:"total"`
	Items interface{} `json:"items"`
}
