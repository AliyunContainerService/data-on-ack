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
	"context"
	"net/http/httputil"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/pkg/infra/backends/clientmgr"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

func NewPipelineAPIsController() *PipelineAPIsController {
	reverseProxy, err := utils.NewKubeflowProxy()
	if err != nil {
		klog.Error("fail to NewProxy:" + err.Error())
		return nil
	}
	klog.Info("success create NewPipelineAPIsController")
	return &PipelineAPIsController{reverseProxy: reverseProxy}
}

type PipelineAPIsController struct {
	reverseProxy *httputil.ReverseProxy
}

func (pc *PipelineAPIsController) RegisterRoutes(routes *gin.Engine) {
	routes.GET("/pipelinesession", pc.GetSession)
	routes.GET("/pipelineCheckInstall", pc.CheckInstall)

	jobAPI := routes.Group("/pipeline")
	jobAPI.GET("/*path", pc.PipelineReverseProxy)
	jobAPI.POST("/*path", pc.PipelineReverseProxy)
	jobAPI.PUT("/*path", pc.PipelineReverseProxy)
	jobAPI.DELETE("/*path", pc.PipelineReverseProxy)
	jobAPI.PATCH("/*path", pc.PipelineReverseProxy)
	jobAPI.HEAD("/*path", pc.PipelineReverseProxy)
	klog.Info("success register NewPipelineAPIsController")

}
func (pc *PipelineAPIsController) PipelineReverseProxy(c *gin.Context) {
	// construct pipeline server url
	c.Request.URL.Path = strings.Replace(c.Request.URL.Path, "/pipeline", "", 1)

	// request pipeline server
	pc.reverseProxy.ServeHTTP(c.Writer, c.Request)
}
func (pc *PipelineAPIsController) GetSession(c *gin.Context) {

	session := sessions.Default(c)
	utils.Succeed(c, map[string]interface{}{
		"accountId":  session.Get(auth.SessionKeyAccountID),
		"loginId":    session.Get(auth.SessionKeyLoginID),
		"name":       session.Get(auth.SessionKeyName),
		"loginName":  session.Get(auth.SessionKeyLoginName),
		"namespaces": session.Get(auth.SessionKeyUserNS),
		"token":      session.Get(auth.SessionKeyToken),
	})
}

func (pc *PipelineAPIsController) CheckInstall(c *gin.Context) {
	// check ml-pipeline deployment install
	kubeclient := clientmgr.GetKubeClient()
	_, err := kubeclient.AppsV1().Deployments("kube-ai").Get(context.TODO(), "ml-pipeline", metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			klog.Info("kubeflow pipeline not install")
		} else {
			klog.Error("fail to get ml-pipeline")
		}
		utils.Succeed(c, map[string]interface{}{
			"install": false,
		})
	} else {
		klog.Info("kubeflow pipeline has install")
		utils.Succeed(c, map[string]interface{}{
			"install": true,
		})
	}

}
