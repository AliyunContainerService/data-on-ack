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
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/gin-gonic/gin"
)

func NewDataAPIsController(dataHandler *handlers.DataHandler) *DataAPIsController {
	return &DataAPIsController{
		dataHandler: dataHandler,
	}
}

type DataAPIsController struct {
	dataHandler *handlers.DataHandler
}

func (dc *DataAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	overview := routes.Group("/data")
	overview.GET("/total", dc.getClusterTotal)
	overview.GET("/request/:podPhase", dc.getClusterRequest)
	overview.GET("/nodeInfos", dc.getClusterNodeInfos)
	overview.GET("/podRangeInfo", dc.getClusterPodRangeInfo)
}

func (dc *DataAPIsController) getClusterTotal(c *gin.Context) {
	clusterTotal, err := dc.dataHandler.GetClusterTotalResource()
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to getClusterTotal, err=%v", err))
		return
	}
	utils.Succeed(c, clusterTotal)
}

func (dc *DataAPIsController) getClusterRequest(c *gin.Context) {
	podPhase := c.Param("podPhase")
	if podPhase == "" {
		podPhase = string(corev1.PodRunning)
	}
	clusterRequest, err := dc.dataHandler.GetClusterRequestResource(podPhase)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to getClusterRequest, err=%v", err))
		return
	}
	utils.Succeed(c, clusterRequest)
}

func (dc *DataAPIsController) getClusterNodeInfos(c *gin.Context) {
	searchParam := c.Query("searchParam")
	clusterNodeInfos, err := dc.dataHandler.GetClusterNodeInfos(searchParam)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to getClusterNodeInfos, err=%v", err))
		return
	}
	utils.Succeed(c, clusterNodeInfos)
}

func (dc *DataAPIsController) getClusterPodRangeInfo(c *gin.Context) {
	query := c.Query("query")
	start := c.Query("start")
	end := c.Query("end")
	step := c.Query("step")

	clusterPodRangeInfo, err := dc.dataHandler.QueryRange(query, start, end, step)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to getClusterPodRangeInfo, err=%v", err))
		return
	}
	utils.Succeed(c, clusterPodRangeInfo)
}
