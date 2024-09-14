/*
*Copyright (c) 2021, Alibaba Group;
*Licensed under the Apache License, Version 2.0 (the "License");
*you may not use this file except in compliance with the License.
*You may obtain a copy of the License at

*   http://www.apache.org/licenses/LICENSE-2.0

*Unless required by applicable law or agreed to in writing, software
*distributed under the License is distributed on an "AS IS" BASIS,
*WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*See the License for the specific language governing permissions and
*limitations under the License.
*/
    
package api

import (
	"net/http/httputil"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"k8s.io/klog"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
)

type MlflowAPIsController struct {
	reverseProxy *httputil.ReverseProxy
}

func NewMlflowAPIsController() *MlflowAPIsController {
	reverseProxy, err := utils.NewMlflowProxy()
	if err != nil {
		klog.Error("failed to create reverse proxy for mlflow tracking server")
		return nil
	}
	klog.Info("successfully create mlflow APIs controller")
	return &MlflowAPIsController{reverseProxy: reverseProxy}
}

func (ctrl *MlflowAPIsController) RegisterRoutes(routes *gin.Engine) {
	routes.GET("/mlflow-session", ctrl.GetSession)

	group := routes.Group("/mlflow")
	group.Any("/*path", ctrl.ReverseProxyMiddleware)
	klog.Info("successfully register mlflow APIs controller")
}

func (ctrl *MlflowAPIsController) GetSession(ctx *gin.Context) {
	session := sessions.Default(ctx)
	utils.Succeed(ctx, map[string]interface{}{
		"accountId":  session.Get(auth.SessionKeyAccountID),
		"loginId":    session.Get(auth.SessionKeyLoginID),
		"name":       session.Get(auth.SessionKeyName),
		"loginName":  session.Get(auth.SessionKeyLoginName),
		"namespaces": session.Get(auth.SessionKeyUserNS),
		"token":      session.Get(auth.SessionKeyToken),
	})
}

func (ctrl *MlflowAPIsController) BasicAuthMiddleware(ctx *gin.Context) {
	session := sessions.Default(ctx)
	username, _ := session.Get(auth.SessionKeyLoginName).(string)
	password, _ := session.Get(auth.SessionKeyToken).(string)
	ctx.Request.SetBasicAuth(username, password)
}

func (ctrl *MlflowAPIsController) ReverseProxyMiddleware(ctx *gin.Context) {
	// Construct MLflow request URL
	ctx.Request.URL.Path = strings.Replace(ctx.Request.URL.Path, "/mlflow", "", 1)

	// Route request to MLflow tracking server
	ctrl.reverseProxy.ServeHTTP(ctx.Writer, ctx.Request)
}
