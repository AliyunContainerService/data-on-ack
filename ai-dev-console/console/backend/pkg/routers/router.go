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
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/routers/api"
	"github.com/spf13/pflag"

	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/auth"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/constants"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/handlers"
	md "github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/middleware"
	"github.com/AliyunContainerService/data-on-ack/ai-dev-console/console/backend/pkg/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"

	"k8s.io/klog"
)

func init() {
	pflag.StringVar(&eventStorage, "event-storage", "arena", "event storage backend plugin name, persist events into backend if it's specified")
	pflag.StringVar(&objectStorage, "object-storage", "arena", "object storage backend plugin name, persist jobs and pods into backend if it's specified")
	pflag.StringVar(&clientType, "client-type", "arena", "client type name, support apiserver and arena")
}

var (
	eventStorage  string
	objectStorage string
	clientType    string
)

type APIController interface {
	RegisterRoutes(routes *gin.RouterGroup)
}

func InitRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	r.Use(
		gin.Recovery(),
		func(context *gin.Context) {
			context.Header("Cache-Control", "no-store,no-cache")
		},
	)

	r.NoRoute(
		utils.Redirect500,
		utils.Redirect403,
		utils.Redirect404,
		utils.Redirect1000,
	)

	store := cookie.NewStore([]byte("secret"))
	store.Options(sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	r.Use(
		sessions.Sessions("loginSession", store),
	)

	aliCloudAuth, err := auth.NewAliCloudAuth()
	if err != nil {
		panic(err)
	}
	if md.EnableAuth() {
		r.Use(
			md.CheckAuthMiddleware(aliCloudAuth),
		)
	}
	// notebook after auth but must before others or browser will render dev-console index.html
	notebookHandler, err := handlers.NewNotebookHandler(objectStorage)
	if err != nil {
		klog.Errorf("NewNotebookHandler error: %v", err)
		panic(err)
	}
	jupyterReserveProxy := r.Group("/notebook")
	notebookController := api.NewNotebookAPIsController(notebookHandler)

	vscodeReserveProxy := r.Group("/vscode")
	sdReserveProxy := r.Group("/sd")
	commonProxy := r.Group("/common")

	notebookController.RegisterAllReverseProxy(jupyterReserveProxy, notebookController.JupyterReverseProxy)
	notebookController.RegisterAllReverseProxy(vscodeReserveProxy, notebookController.VSCodeReverseProxy)
	notebookController.RegisterAllReverseProxy(sdReserveProxy, notebookController.SdReverseProxy)
	notebookController.RegisterAllReverseProxy(commonProxy, notebookController.CommonReverseProxy)

	klog.Info("Success create NotebookAPIsController")
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	distDir := path.Join(wd, "dist")
	klog.Infof("kubedl-console working dir: %s", wd)
	r.LoadHTMLFiles(distDir + "/index.html")
	r.Use(static.Serve("/", static.LocalFile(distDir, true)))
	r.Use(func(context *gin.Context) {

		if !strings.HasPrefix(context.Request.URL.Path, "/api/v1") &&
			!strings.HasPrefix(context.Request.URL.Path, "/ml_metadata.MetadataStoreService") &&
			!strings.HasPrefix(context.Request.URL.Path, "/pipeline") &&
			!strings.HasPrefix(context.Request.URL.Path, "/mlflow") {
			context.HTML(http.StatusOK, "index.html", gin.H{})
		}

	})
	klog.Info("Success Load index.html")

	logHandler, err := handlers.NewLogHandler(eventStorage)
	if err != nil {
		klog.Error("Fail to new log handler:" + err.Error())
		panic(err)
	}

	jobHandler, err := handlers.NewJobHandler(objectStorage, clientType, logHandler)
	if err != nil {
		klog.Error("Fail to NewJobHandler:" + err.Error())
		panic(err)
	}

	cronHandler, err := handlers.NewCronHandler(objectStorage, clientType)
	if err != nil {
		klog.Error("Fail to NewCronHandlerr:" + err.Error())
		panic(err)
	}

	dataHandler := handlers.NewDataHandler()

	dataSourceHandler := handlers.NewDataSourceHandler()

	codeSourceHandler := handlers.NewCodeSourceHandler()

	evaluateHandler, err := handlers.NewEvaluateHandler(objectStorage, clientType)
	if err != nil {
		klog.Error("Fail to NewEvaluateHandler:" + err.Error())
		panic(err)
	}

	modelsHandler, err := handlers.NewModelsHandler(objectStorage)
	if err != nil {
		klog.Error("Fail to NewModelsHandler:" + err.Error())
		panic(err)
	}

	mlMetadataController := api.NewMLMetadataController()
	mlMetadataController.RegisterRoutes(r)

	pipelineController := api.NewPipelineAPIsController()
	pipelineController.RegisterRoutes(r)

	mlflowController := api.NewMlflowAPIsController()
	mlflowController.RegisterRoutes(r)

	apiV1Routes := r.Group(constants.ApiV1Routes)

	ctrls := DefaultAPIV1Controllers(logHandler, jobHandler, cronHandler, aliCloudAuth, dataHandler, dataSourceHandler, codeSourceHandler, notebookController, evaluateHandler, modelsHandler)

	for _, ctrl := range ctrls {
		ctrl.RegisterRoutes(apiV1Routes)
	}
	return r
}
