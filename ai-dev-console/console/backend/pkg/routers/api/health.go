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
	"net/http"

	"github.com/gin-gonic/gin"
)

func NewHealthAPIsController() *HealthAPIsController {
	return &HealthAPIsController{}
}

type HealthAPIsController struct {
}

func (hc *HealthAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	//overview := routes.Group("/health")
	routes.GET("/health", hc.checkHealth)
}

func (hc *HealthAPIsController) checkHealth(c *gin.Context) {
	c.JSONP(http.StatusOK, gin.H{
		"code": "200",
		"data": "ok",
	})
}
