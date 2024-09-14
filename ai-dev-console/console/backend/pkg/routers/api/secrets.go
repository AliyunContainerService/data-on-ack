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
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/gin-gonic/gin"
)

func NewSecretsAPIController() *SecretsAPIController {
	return &SecretsAPIController{secretHandler: handlers.NewSecretHandler()}
}

type SecretsAPIController struct {
	secretHandler *handlers.SecretHandler
}

func (sc *SecretsAPIController) RegisterRoutes(routes *gin.RouterGroup) {

}
