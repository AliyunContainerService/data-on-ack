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
    
package routers

import (
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/routers/api"
)

func DefaultAPIV1Controllers(logHandler *handlers.LogHandler, jobHandler *handlers.JobHandler, cronHandler *handlers.CronHandler, loginAuth auth.Auth,
	dataHandler *handlers.DataHandler, dataSourceHandler *handlers.DataSourceHandler, codeSourceHandler *handlers.CodeSourceHandler, notebookController *api.NotebookAPIsController,
	evaluateHandler *handlers.EvaluateHandler, modelsHandler *handlers.ModelsHandler) []APIController {
	return []APIController{
		api.NewHealthAPIsController(),
		api.NewJobAPIsController(jobHandler),
		api.NewAuthAPIsController(loginAuth),
		api.NewLogsAPIsController(logHandler),
		api.NewCronAPIsController(cronHandler),
		api.NewDLCAPIsController(),
		api.NewDataAPIsController(dataHandler),
		api.NewDataSourceAPIsController(dataSourceHandler),
		api.NewCodeSourceAPIsController(codeSourceHandler),
		api.NewSecretsAPIController(),
		api.NewEvaluateAPIsController(evaluateHandler),
		notebookController,
		api.NewModelsAPIscontroller(modelsHandler),
	}
}
