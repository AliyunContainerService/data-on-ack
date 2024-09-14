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

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/gin-gonic/gin"
	"k8s.io/klog"
)

func NewMLMetadataController() *MLMetadataController {
	reverseProxy, err := utils.NewKubeflowProxy()

	if err != nil {
		klog.Error("fail to NewProxy:" + err.Error())
		return nil
	}
	klog.Info("success create NewMLMetadataController")
	return &MLMetadataController{reverseProxy: reverseProxy}
}

type MLMetadataController struct {
	reverseProxy *httputil.ReverseProxy
}

func (mc *MLMetadataController) RegisterRoutes(routes *gin.Engine) {
	metadataAPI := routes.Group("/ml_metadata.MetadataStoreService")
	metadataAPI.GET("/*path", mc.MLMetadataReverseProxy)
	metadataAPI.POST("/*path", mc.MLMetadataReverseProxy)
	metadataAPI.PUT("/*path", mc.MLMetadataReverseProxy)
	metadataAPI.DELETE("/*path", mc.MLMetadataReverseProxy)
	metadataAPI.PATCH("/*path", mc.MLMetadataReverseProxy)
	metadataAPI.HEAD("/*path", mc.MLMetadataReverseProxy)
	klog.Info("success register NewPipelineAPIsController")

}

func (mc *MLMetadataController) MLMetadataReverseProxy(c *gin.Context) {
	klog.Info("metadata proxy:" + c.Request.URL.Path)
	mc.reverseProxy.ServeHTTP(c.Writer, c.Request)
}
