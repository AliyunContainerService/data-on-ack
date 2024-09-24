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
	training "github.com/AliyunContainerService/data-on-ack/ai-dev-console/apis/training/v1alpha1"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/gin-contrib/sessions"
	"github.com/spf13/pflag"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/gin-gonic/gin"
)

func init() {
	pflag.BoolVar(&detectJobInNS, "detect-job-in-ns", false, "detect jobs in namespace when do listing and return a map")
}

var (
	detectJobInNS bool
)

func NewDLCAPIsController() *DLCAPIsController {
	return &DLCAPIsController{
		handler: handlers.NewDLCHandler(),
	}
}

func (dc *DLCAPIsController) RegisterRoutes(routes *gin.RouterGroup) {
	dlcAPIs := routes.Group("/dlc")
	dlcAPIs.GET("/common-config", dc.getCommonConfig)
	dlcAPIs.GET("/namespaces", dc.getAvailableNamespaces)
}

type DLCAPIsController struct {
	handler *handlers.DLCHandler
}

func (dc *DLCAPIsController) getCommonConfig(c *gin.Context) {
	dlcCfg, err := dc.handler.GetDLCConfig()
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to marshal dlc common config, err: %v", err))
		return
	}

	utils.Succeed(c, dlcCfg)
}

func (dc *DLCAPIsController) getAvailableNamespaces(c *gin.Context) {
	session := sessions.Default(c)
	loginUserName, _ := session.Get(auth.SessionKeyLoginName).(string)

	availableNS, err := dc.handler.ListAvailableNamespaces(loginUserName)
	if err != nil {
		handleErr(c, fmt.Sprintf("failed to list avaliable namespaces, error: %v", err))
		return
	}
	if !detectJobInNS {
		utils.Succeed(c, availableNS)
		return
	}

	nsWithJob := make(map[string]bool)
	for _, ns := range availableNS {
		nsWithJob[ns] = dc.handler.DetectJobsInNS(loginUserName, ns, training.TFJobKind) ||
			dc.handler.DetectJobsInNS(loginUserName, ns, training.PyTorchJobKind)
	}
	utils.Succeed(c, nsWithJob)
}
